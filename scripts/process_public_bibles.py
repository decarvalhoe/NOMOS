#!/usr/bin/env python3
"""Process the PUBLIC Nomos canonical bibles with the read-only corpus pipeline.

RCP-010 (#196) asks to process the public and licensed bibles with Nomos. The
licensed half (ISO 13485, ISO/IEC/IEEE 12207, ISO/IEC 25010, ISPE GAMP 5) is
blocked on licensed-document procurement and must never be committed as full text
(doctrine §2.6). This script does the **public half**, which is actionable today:

1. classify the external reference register into public vs licensed bibles
   (reusing scripts/regulated_reference_canon.py);
2. snapshot the in-repo public reference corpus into a dedicated, push-free git
   checkout (so the corpus is never the live repo);
3. run `nomos corpus scan -> manifest -> feed -> attest` over it, producing
   atomization reports, manifests, and an attestation;
4. prove zero source mutation via the corpus read-only guard AND a before/after
   git-status check on the snapshot corpus.

It emits a compact processing summary and returns non-zero if the public half is
not cleanly processable (e.g. any source mutation, or a licensed bible leaking
into the processed set).
"""

from __future__ import annotations

import argparse
import importlib.util
import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any

ROOT_DEFAULT = Path(__file__).resolve().parents[1]


def load_canon(root: Path):
    spec = importlib.util.spec_from_file_location(
        "reference_canon", root / "scripts" / "regulated_reference_canon.py"
    )
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


def classify_bibles(root: Path) -> dict[str, Any]:
    canon = load_canon(root)
    report = canon.build_report(root, licensed_root=None, allow_public_surrogates=False)
    public, licensed = [], []
    for bible in report.get("bibles", []):
        if bible.get("source_class") == "public":
            public.append(bible["id"])
        else:
            licensed.append({"id": bible["id"], "source_class": bible["source_class"]})
    return {"public": public, "licensed": licensed, "canon_status": report.get("status")}


def public_corpus_docs(root: Path) -> list[Path]:
    """The in-repo public reference corpus: the reference-basis markdown that
    describes the public bibles. Markdown only, so atomization is meaningful."""
    base = root / "docs" / "regulated" / "reference-basis"
    docs = [base / "README.md", base / "nomos-bible-corpus-policy.md"]
    return [d for d in docs if d.exists()]


def git(args: list[str], cwd: Path) -> str:
    return subprocess.run(
        ["git", "-c", "core.autocrlf=false", *args],
        cwd=cwd,
        check=True,
        capture_output=True,
        text=True,
    ).stdout.strip()


def run_nomos(nomos_bin: str, args: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run([nomos_bin, *args], capture_output=True, text=True)


def build_corpus(root: Path, workdir: Path) -> Path:
    corpus = workdir / "public-bibles-corpus"
    corpus.mkdir(parents=True)
    for doc in public_corpus_docs(root):
        shutil.copy2(doc, corpus / doc.name)
    git(["init", "-q"], corpus)
    git(["add", "-A"], corpus)
    git(["-c", "user.email=nomos@local", "-c", "user.name=nomos", "commit", "-qm", "public bibles snapshot"], corpus)
    return corpus


def process(root: Path, nomos_bin: str) -> dict[str, Any]:
    split = classify_bibles(root)
    with tempfile.TemporaryDirectory() as tmp:
        workdir = Path(tmp)
        corpus = build_corpus(root, workdir)
        evidence = workdir / "evidence"
        evidence.mkdir()

        before = git(["status", "--porcelain"], corpus)
        head_before = git(["rev-parse", "HEAD"], corpus)

        steps: dict[str, Any] = {}
        snapshot = evidence / "snapshot.json"
        manifest = evidence / "source-manifest.yaml"
        feed = evidence / "feed.json"
        attestation = evidence / "attestation.json"

        r = run_nomos(nomos_bin, ["corpus", "scan", "--root", str(corpus), "--out", str(snapshot)])
        steps["scan"] = r.returncode
        if r.returncode == 0:
            r = run_nomos(nomos_bin, ["corpus", "manifest", "--snapshot", str(snapshot), "--out", str(manifest), "--domain", "public-reference-basis"])
            steps["manifest"] = r.returncode
        if steps.get("manifest") == 0:
            r = run_nomos(nomos_bin, ["corpus", "feed", "--root", str(corpus), "--snapshot", str(snapshot), "--manifest", str(manifest), "--out", str(feed)])
            steps["feed"] = r.returncode
        if steps.get("scan") == 0:
            r = run_nomos(nomos_bin, ["corpus", "attest", "--snapshot", str(snapshot), "--corpus-id", "public-bibles", "--project-id", "nomos", "--out", str(attestation)])
            steps["attest"] = r.returncode

        after = git(["status", "--porcelain"], corpus)
        head_after = git(["rev-parse", "HEAD"], corpus)
        source_mutation = (before != after) or (head_before != head_after)

        snap = json.loads(snapshot.read_text(encoding="utf-8")) if snapshot.exists() else {}
        processed_files = [s["path"] for s in snap.get("sources", [])]
        licensed_ids = {entry["id"] for entry in split["licensed"]}
        leaked = [p for p in processed_files for lid in licensed_ids if lid.lower() in p.lower()]

        return {
            "schema_version": "nomos-public-policy-fixture-v2",
            "claim_boundary": (
                "Two in-repo public policy documents exercise the pipeline; "
                "public_classified_count is registry classification, not the number "
                "of external public bibles processed. Licensed bibles are excluded."
            ),
            "bible_split": {
                "public_classified_count": len(split["public"]),
                "external_public_sources_processed": 0,
                "licensed_blocked": split["licensed"],
                "canon_status": split["canon_status"],
            },
            "processed_corpus": {
                "files": processed_files,
                "file_count": len(processed_files),
            },
            "pipeline_steps": steps,
            "artifacts_present": {
                "snapshot": snapshot.exists(),
                "manifest": manifest.exists(),
                "atomization_feed": feed.exists(),
                "attestation": attestation.exists(),
            },
            "read_only_guard": "pass" if not source_mutation else "fail",
            "source_mutation": "none" if not source_mutation else "DETECTED",
            "licensed_leak": leaked,
        }


def acceptance_ok(summary: dict[str, Any]) -> bool:
    artifacts = summary["artifacts_present"]
    steps = summary["pipeline_steps"]
    return (
        summary["read_only_guard"] == "pass"
        and summary["source_mutation"] == "none"
        and not summary["licensed_leak"]
        and all(artifacts.get(name) for name in ("snapshot", "manifest", "atomization_feed", "attestation"))
        and all(steps.get(name) == 0 for name in ("scan", "manifest", "feed", "attest"))
    )


def main() -> int:
    parser = argparse.ArgumentParser(description="Process the public Nomos bibles read-only.")
    parser.add_argument("--root", default=str(ROOT_DEFAULT))
    parser.add_argument("--nomos-bin", required=True, help="Path to the built nomos binary.")
    parser.add_argument("--out", default="", help="Optional summary JSON output path.")
    args = parser.parse_args()

    summary = process(Path(args.root).resolve(), args.nomos_bin)
    text = json.dumps(summary, indent=2, sort_keys=True)
    if args.out:
        Path(args.out).write_text(text + "\n", encoding="utf-8")
    print(text)
    return 0 if acceptance_ok(summary) else 1


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""Process the PUBLIC canonical references with the read-only corpus pipeline.

#644 (RCP public slice of #196). Before this, the script picked two in-repo
policy documents, ran four of the six pipeline stages, wrote the artifacts into
a temporary directory and deleted them on exit — then a summary said "public
bibles processed". The register said 23; the truth was two in-repo Markdown
files and no external source at all. That gap is closed here in the only honest
direction: by counting less, and proving what is counted.

What "processed" means now
--------------------------
A registered public reference counts as PROCESSED only when ALL of these hold:

  1. the register classifies it public (scripts/regulated_reference_canon.py —
     the same classifier the licence gate uses, never a hardcoded list);
  2. `public-source-snapshots.yaml` carries a capture entry for it: official
     URL, capture date, sha256, size, licence/policy note, version identity.
     No entry, or an entry marked blocked, is an explicit BLOCKED state — not a
     missing row someone forgot;
  3. a local capture whose sha256 EQUALS the entry's is present in
     --captures-dir (outside the repository: public does not mean the full
     text may be committed — see the licence register's
     no_committed_third_party_full_text);
  4. scan, manifest, feed, body ledger, attestation and the strict gate ALL exit
     0 over that capture.

Anything short of that is reported by name: `blocked` with its reason.

What is retained
----------------
Every stage's artifact is written to --retain-dir under
<ref_id>/<sha256[:16]>/ and CONTENT-ADDRESSED: a receipt.json records the
sha256 of each artifact, the source capture's sha256, the version identity and
the exit code of every stage. The receipts are committed under
docs/regulated/reference-basis/public-bibles-processing/receipts/; the
artifacts themselves stay outside the repository, because a feed and a body
ledger carry the atomised text of a third-party document. A receipt lets anyone
re-run the chain over the same capture and compare digests byte for byte.

Change one byte of a capture and its sha256 changes: it no longer matches the
snapshot entry (blocked: capture_hash_mismatch), and any receipt keyed by the
old digest is STALE — reported as such, never silently reused.

The in-repo fixture corpus (two policy documents) still exists, still exercises
the pipeline offline, and is counted under its own name: `fixture_documents`.
It is never counted as an external public source.

Modes
-----
  --nomos-bin BIN                        required: the built binary
  --captures-dir DIR                     local captures, <ref_id>.<ext>, outside the repo
  --retain-dir DIR                       where artifacts are retained (default .nomos/retained-public-sources)
  --capture-live                         fetch each public URL, record sha256 + date in the snapshots file
                                         (network; hash only, nothing stored) — skippable
  --out summary.json
"""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
import shutil
import subprocess
import sys
import tempfile
import urllib.request
from datetime import date, datetime, timezone
from pathlib import Path
from typing import Any

import yaml

ROOT_DEFAULT = Path(__file__).resolve().parents[1]
SNAPSHOTS_PATH = Path("docs/regulated/reference-basis/public-source-snapshots.yaml")
RECEIPTS_DIR = Path("docs/regulated/reference-basis/public-bibles-processing/receipts")
SNAPSHOTS_SCHEMA = "nomos-public-source-snapshots-v1"
RECEIPT_SCHEMA = "nomos-public-source-receipt-v1"
SUMMARY_SCHEMA = "nomos-public-source-processing-v3"
STAGES = ("scan", "manifest", "feed", "body_ledger", "attest", "strict")
FIXTURE_DOCS = ("README.md", "nomos-bible-corpus-policy.md")

CLAIM_BOUNDARY = (
    "external_public_sources_processed counts registered public references whose local "
    "capture matches the recorded sha256 and completed scan, manifest, feed, body ledger, "
    "attestation and the strict gate. public_classified_count is register classification. "
    "fixture_documents are two in-repo policy documents and are never counted as external. "
    "Retained artifacts live outside the repository; committed receipts content-address them."
)


# ----------------------------------------------------------------- helpers

def load_canon(root: Path):
    spec = importlib.util.spec_from_file_location(
        "reference_canon", root / "scripts" / "regulated_reference_canon.py"
    )
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return "sha256:" + h.hexdigest()


def git(args: list[str], cwd: Path) -> str:
    return subprocess.run(
        ["git", "-c", "core.autocrlf=false", *args], cwd=cwd, check=True, capture_output=True, text=True
    ).stdout.strip()


def run_nomos(nomos_bin: str, args: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run([nomos_bin, *args], capture_output=True, text=True)


def load_yaml(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    return data if isinstance(data, dict) else {}


# ----------------------------------------------------------------- register

def classify_bibles(root: Path) -> dict[str, Any]:
    """Public vs licensed, from the canon classifier — never a hardcoded list."""
    canon = load_canon(root)
    report = canon.build_report(root, licensed_root=None, allow_public_surrogates=False)
    public, licensed = [], []
    for bible in report.get("bibles", []):
        if bible.get("source_class") == "public":
            public.append(bible["id"])
        else:
            licensed.append({"id": bible["id"], "source_class": bible["source_class"]})
    return {"public": public, "licensed": licensed, "canon_status": report.get("status")}


def register_entries(root: Path) -> dict[str, dict[str, Any]]:
    register = load_yaml(root / "docs/regulated/reference-basis/external-reference-register.yaml")
    return {str(r.get("id")): r for r in register.get("references") or [] if isinstance(r, dict)}


def load_snapshots(root: Path) -> dict[str, Any]:
    data = load_yaml(root / SNAPSHOTS_PATH)
    if data and data.get("schema_version") != SNAPSHOTS_SCHEMA:
        raise SystemExit(f"{SNAPSHOTS_PATH}: schema_version {data.get('schema_version')!r} != {SNAPSHOTS_SCHEMA!r}")
    return data


def snapshot_entry(snapshots: dict[str, Any], ref_id: str) -> dict[str, Any] | None:
    for entry in snapshots.get("sources") or []:
        if isinstance(entry, dict) and entry.get("reference_id") == ref_id:
            return entry
    return None


# ----------------------------------------------------------------- live capture

def capture_live(root: Path, ref_ids: list[str], timeout: int) -> dict[str, Any]:
    """Fetch each public URL, hash it, record date/size/sha256. Stores NOTHING.

    A failed fetch is recorded as blocked with its reason — a network refusal is
    a fact about today, not a reason to invent a hash.
    """
    entries = register_entries(root)
    snapshots = load_snapshots(root) or {"schema_version": SNAPSHOTS_SCHEMA, "sources": []}
    by_id = {e["reference_id"]: e for e in snapshots.get("sources") or [] if isinstance(e, dict)}
    today = date.today().isoformat()
    results = {}
    for ref_id in ref_ids:
        reg = entries.get(ref_id, {})
        url = str(reg.get("url", "")).strip()
        entry = by_id.setdefault(ref_id, {"reference_id": ref_id})
        entry["official_url"] = url
        entry["version_identity"] = str(reg.get("version_or_date", "")).strip() or "unspecified"
        entry.setdefault("licence_or_policy", "public official source; hash only, no full text committed")
        entry.setdefault("storage_policy", "hash_only_no_full_text")
        try:
            h = hashlib.sha256()
            size = 0
            req = urllib.request.Request(url, headers={"User-Agent": "Nomos-PublicSourceCapture/0.1 (read-only)"})
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                content_type = resp.headers.get("Content-Type", "")
                while True:
                    chunk = resp.read(65536)
                    if not chunk:
                        break
                    h.update(chunk)
                    size += len(chunk)
            entry.update({
                "status": "captured_hash_only",
                "captured_on": today,
                "sha256": "sha256:" + h.hexdigest(),
                "size_bytes": size,
                "content_type": content_type,
            })
            entry.pop("blocked_reason", None)
            results[ref_id] = "captured_hash_only"
        except Exception as exc:  # noqa: BLE001 — the reason is the evidence
            entry.update({"status": "blocked", "blocked_reason": f"fetch_failed: {type(exc).__name__}: {str(exc)[:120]}", "checked_on": today})
            results[ref_id] = "blocked"
    snapshots["sources"] = sorted(by_id.values(), key=lambda e: e["reference_id"])
    snapshots["schema_version"] = SNAPSHOTS_SCHEMA
    snapshots.setdefault("claim_boundary", "Hash-only captures of public official sources, dated. A hash proves what the URL served that day; it is not a copy and it is not a statement about the content.")
    (root / SNAPSHOTS_PATH).write_text(yaml.safe_dump(snapshots, sort_keys=False, allow_unicode=True), encoding="utf-8")
    return results


# ----------------------------------------------------------------- pipeline

def run_chain(nomos_bin: str, corpus: Path, evidence: Path, corpus_id: str) -> dict[str, int]:
    """All six stages. A stage is skipped (not run) when a prerequisite failed,
    and skipped stages are recorded as -1 so a reader never mistakes "not run"
    for "passed"."""
    steps: dict[str, int] = {s: -1 for s in STAGES}
    snapshot = evidence / "snapshot.json"
    manifest = evidence / "source-manifest.yaml"
    feed = evidence / "feed.json"
    ledger = evidence / "body-ledger.json"
    attestation = evidence / "attestation.json"
    strict_report = evidence / "strict-gate.json"

    r = run_nomos(nomos_bin, ["corpus", "scan", "--root", str(corpus), "--out", str(snapshot)])
    steps["scan"] = r.returncode
    if steps["scan"] == 0:
        r = run_nomos(nomos_bin, ["corpus", "manifest", "--snapshot", str(snapshot), "--out", str(manifest), "--domain", "public-reference-basis"])
        steps["manifest"] = r.returncode
    if steps["manifest"] == 0:
        r = run_nomos(nomos_bin, ["corpus", "feed", "--root", str(corpus), "--snapshot", str(snapshot), "--manifest", str(manifest), "--out", str(feed)])
        steps["feed"] = r.returncode
        r = run_nomos(nomos_bin, ["corpus", "body-ledger", "--root", str(corpus), "--manifest", str(manifest), "--out", str(ledger)])
        steps["body_ledger"] = r.returncode
    if steps["scan"] == 0:
        args = ["corpus", "attest", "--snapshot", str(snapshot), "--corpus-id", corpus_id, "--project-id", "nomos", "--out", str(attestation)]
        if steps["body_ledger"] == 0:
            args += ["--corpus-body-ledger", str(ledger)]
        r = run_nomos(nomos_bin, args)
        steps["attest"] = r.returncode
    if steps["feed"] == 0 and steps["body_ledger"] == 0:
        r = run_nomos(nomos_bin, ["strict", "--corpus-integrity-source", str(corpus), "--corpus-integrity-feed", str(feed), "--corpus-body-ledger", str(ledger), "--format", "json"])
        strict_report.write_text(r.stdout, encoding="utf-8")
        steps["strict"] = r.returncode
    return steps


def chain_complete(steps: dict[str, int], source_mutation: str) -> bool:
    """The one rule that turns a run into "processed": every stage — the strict
    gate included — exited 0, and the source did not move. Kept as a function so
    the rule is tested directly rather than only through a run in which strict
    happens to pass."""
    return source_mutation == "none" and all(steps.get(s) == 0 for s in STAGES)


def make_corpus(files: list[Path], workdir: Path, name: str) -> Path:
    corpus = workdir / name
    corpus.mkdir(parents=True)
    for f in files:
        shutil.copy2(f, corpus / f.name)
    git(["init", "-q"], corpus)
    git(["add", "-A"], corpus)
    git(["-c", "user.email=nomos@local", "-c", "user.name=nomos", "commit", "-qm", "snapshot"], corpus)
    return corpus


def retain(evidence: Path, retain_root: Path, ref_id: str, source_sha: str) -> tuple[Path, dict[str, str]]:
    """Copy the artifacts to <retain>/<ref_id>/<sha[:16]>/ and content-address them."""
    dest = retain_root / ref_id / source_sha.split(":", 1)[-1][:16]
    if dest.exists():
        shutil.rmtree(dest)
    dest.mkdir(parents=True)
    digests: dict[str, str] = {}
    for f in sorted(evidence.iterdir()):
        if f.is_file():
            shutil.copy2(f, dest / f.name)
            digests[f.name] = sha256_file(dest / f.name)
    return dest, digests


def find_capture(captures_dir: Path | None, ref_id: str) -> Path | None:
    if captures_dir is None or not captures_dir.is_dir():
        return None
    matches = sorted(p for p in captures_dir.iterdir() if p.is_file() and p.stem == ref_id)
    return matches[0] if matches else None


def process_public_source(root: Path, nomos_bin: str, ref_id: str, entry: dict[str, Any] | None,
                          captures_dir: Path | None, retain_root: Path, workdir: Path) -> dict[str, Any]:
    result: dict[str, Any] = {"reference_id": ref_id, "status": "blocked"}
    if entry is None:
        result["blocked_reason"] = "no_snapshot_entry: not registered in public-source-snapshots.yaml"
        return result
    if entry.get("status") == "blocked":
        result["blocked_reason"] = str(entry.get("blocked_reason", "blocked in snapshots file"))
        return result
    declared = str(entry.get("sha256", "")).strip().lower()
    if not declared:
        result["blocked_reason"] = "no_recorded_sha256"
        return result
    capture = find_capture(captures_dir, ref_id)
    if capture is None:
        result["blocked_reason"] = "no_local_capture: --captures-dir holds no file for this reference (hash-only capture recorded, content not present)"
        result["captured_hash_only"] = True
        return result
    actual = sha256_file(capture)
    result["capture_sha256"] = actual
    if actual != declared:
        result["blocked_reason"] = f"capture_hash_mismatch: local {actual[:23]}… != recorded {declared[:23]}… — the capture changed; re-record it or the recorded hash is stale"
        # Stale relative to the bytes that exist NOW, not to the recorded hash:
        # a receipt made from a capture that no longer exists cannot be reused.
        result["stale_receipts"] = stale_receipts(root, ref_id, actual)
        return result

    corpus = make_corpus([capture], workdir, f"public-{ref_id}")
    evidence = workdir / f"evidence-{ref_id}"
    evidence.mkdir()
    before = git(["status", "--porcelain"], corpus)
    steps = run_chain(nomos_bin, corpus, evidence, f"public-{ref_id}")
    after = git(["status", "--porcelain"], corpus)
    result["pipeline_steps"] = steps
    result["source_mutation"] = "none" if before == after else "DETECTED"
    if chain_complete(steps, result["source_mutation"]):
        dest, digests = retain(evidence, retain_root, ref_id, actual)
        receipt = {
            "schema_version": RECEIPT_SCHEMA,
            "reference_id": ref_id,
            "official_url": entry.get("official_url"),
            "version_identity": entry.get("version_identity"),
            "captured_on": entry.get("captured_on"),
            "capture_sha256": actual,
            "capture_size_bytes": capture.stat().st_size,
            "pipeline_steps": steps,
            "artifacts": digests,
            "retained_at": str(dest),
            "processed_on": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "claim_boundary": "Six stages completed over a capture whose sha256 matches the recorded one; the artifacts are content-addressed above and retained outside the repository.",
        }
        rdir = root / RECEIPTS_DIR / ref_id
        rdir.mkdir(parents=True, exist_ok=True)
        (rdir / f"{actual.split(':')[1][:16]}.receipt.json").write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        result["status"] = "processed"
        result["receipt"] = str((rdir / f"{actual.split(':')[1][:16]}.receipt.json").relative_to(root))
        result["artifacts"] = digests
    else:
        failed = [s for s in STAGES if steps[s] != 0]
        result["blocked_reason"] = f"pipeline_incomplete: {', '.join(failed)}" if result["source_mutation"] == "none" else "source_mutation_detected"
    return result


def stale_receipts(root: Path, ref_id: str, current_sha: str) -> list[str]:
    rdir = root / RECEIPTS_DIR / ref_id
    if not rdir.is_dir():
        return []
    prefix = current_sha.split(":", 1)[-1][:16]
    return sorted(str(p.relative_to(root)) for p in rdir.glob("*.receipt.json") if not p.name.startswith(prefix))


def process(root: Path, nomos_bin: str, captures_dir: Path | None, retain_root: Path) -> dict[str, Any]:
    split = classify_bibles(root)
    snapshots = load_snapshots(root)
    with tempfile.TemporaryDirectory() as tmp:
        workdir = Path(tmp)

        # --- the in-repo fixture corpus, named as such, never counted as external
        base = root / "docs" / "regulated" / "reference-basis"
        fixture_files = [base / n for n in FIXTURE_DOCS if (base / n).exists()]
        fixture_corpus = make_corpus(fixture_files, workdir, "fixture-policy-docs")
        fixture_evidence = workdir / "fixture-evidence"
        fixture_evidence.mkdir()
        before = git(["status", "--porcelain"], fixture_corpus)
        fixture_steps = run_chain(nomos_bin, fixture_corpus, fixture_evidence, "public-policy-fixture")
        fixture_mutation = "none" if before == git(["status", "--porcelain"], fixture_corpus) else "DETECTED"

        # --- registered public sources, one by one
        per_source = []
        for ref_id in split["public"]:
            per_source.append(process_public_source(root, nomos_bin, ref_id, snapshot_entry(snapshots, ref_id), captures_dir, retain_root, workdir))

        processed = [r for r in per_source if r["status"] == "processed"]
        hash_only = [r for r in per_source if r.get("captured_hash_only")]
        blocked = [r for r in per_source if r["status"] == "blocked"]
        licensed_ids = {e["id"] for e in split["licensed"]}

        return {
            "schema_version": SUMMARY_SCHEMA,
            "claim_boundary": CLAIM_BOUNDARY,
            "bible_split": {
                "public_classified_count": len(split["public"]),
                "external_public_sources_processed": len(processed),
                "external_public_sources_captured_hash_only": len(hash_only),
                "external_public_sources_blocked": len(blocked),
                "licensed_blocked": split["licensed"],
                "canon_status": split["canon_status"],
            },
            "fixture_documents": {
                "files": [f.name for f in fixture_files],
                "pipeline_steps": fixture_steps,
                "source_mutation": fixture_mutation,
                "counted_as_external": False,
            },
            "public_sources": per_source,
            "licensed_leak": [r["reference_id"] for r in per_source if r["reference_id"] in licensed_ids],
            "retain_dir": str(retain_root),
        }


def acceptance_ok(summary: dict[str, Any]) -> bool:
    """Green means: the fixture chain ran clean, nothing licensed leaked, and no
    source is in a contradictory state. Blocked sources are an honest state and
    do not fail the run — they are simply not counted."""
    fx = summary["fixture_documents"]
    if fx["source_mutation"] != "none" or any(fx["pipeline_steps"][s] != 0 for s in STAGES):
        return False
    if summary["licensed_leak"]:
        return False
    for r in summary["public_sources"]:
        if r["status"] == "processed" and not r.get("receipt"):
            return False
        if r["status"] == "blocked" and not r.get("blocked_reason"):
            return False
    return True


def main() -> int:
    parser = argparse.ArgumentParser(description="Process the public canonical references read-only, and say exactly what was processed.")
    parser.add_argument("--root", default=str(ROOT_DEFAULT))
    parser.add_argument("--nomos-bin", required=True, help="Path to the built nomos binary.")
    parser.add_argument("--captures-dir", default="", help="Local captures <ref_id>.<ext>, OUTSIDE the repository.")
    parser.add_argument("--retain-dir", default=".nomos/retained-public-sources", help="Where retained artifacts go (outside docs/).")
    parser.add_argument("--capture-live", action="store_true", help="Fetch each public URL and record its sha256 and date (network; nothing stored).")
    parser.add_argument("--timeout", type=int, default=30)
    parser.add_argument("--out", default="", help="Summary JSON output path.")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    if args.capture_live:
        results = capture_live(root, classify_bibles(root)["public"], args.timeout)
        print(json.dumps({"capture_live": results}, indent=2, sort_keys=True), file=sys.stderr)

    retain_root = Path(args.retain_dir)
    if not retain_root.is_absolute():
        retain_root = root / retain_root
    captures = Path(args.captures_dir).resolve() if args.captures_dir else None
    summary = process(root, args.nomos_bin, captures, retain_root)
    text = json.dumps(summary, indent=2, sort_keys=True)
    if args.out:
        Path(args.out).write_text(text + "\n", encoding="utf-8")
    print(text)
    return 0 if acceptance_ok(summary) else 1


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""VRC-36 (#573, doc 45 §6 E-1) — the consumer conformance kit.

« Dépendre de l'artefact, jamais du code » (doc 43 §1) — and consuming the
artifact means PASSING THIS KIT. The reference importer every conformant
consumer replays before trusting a Canonical Knowledge Bundle:

1. `schema_version` is exactly `ckm-bundle-v1` — an unknown version is a
   refusal, never a best-effort parse.
2. Structural + facet-vocabulary invariants (the ckm_bundle_validate.py
   nucleus, imported, not duplicated): ≥1 feed, ≥1 node per feed, unique
   node_ids, no orphan rag_metadata, every facet value inside the controlled
   vocabulary generated from specs/facets.cue.
3. The in-toto attestation digest is RECOMPUTED over the feeds payload and
   compared — one mutated byte anywhere in the feeds drifts the digest and
   the bundle is rejected. (Canonicalization mirrors Go's json.Marshal:
   compact separators, raw UTF-8, HTML escapes for < > & and the JS-unsafe
   U+2028/U+2029.)
4. Every node's source_hash has the real-digest FORM (sha256:64hex) and all
   nodes of one source_path agree on it — a forged or inconsistent hash is
   a refusal.

Exit 0 only when every rung passes; the JSON verdict names each finding.
"""
from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
import re
import sys
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
EXPECTED_SCHEMA = "ckm-bundle-v1"
SHA256_FORM = re.compile(r"^sha256:[a-f0-9]{64}$")


def _load_nucleus():
    """Import the existing bundle validator as a module (single source of
    truth for the structural + facet rules — the kit composes, it does not
    fork)."""
    spec = importlib.util.spec_from_file_location(
        "ckm_bundle_validate", ROOT / "scripts" / "ckm_bundle_validate.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def go_canonical_json(value: Any) -> bytes:
    """Byte-compatible mirror of Go's json.Marshal for the feeds payload:
    compact separators, raw UTF-8, and Go's mandatory escapes. Safe as a
    text-level replace because raw < > & U+2028 U+2029 can only occur inside
    JSON strings."""
    s = json.dumps(value, separators=(",", ":"), ensure_ascii=False)
    s = s.replace("<", "\u003c").replace(">", "\u003e").replace("&", "\u0026")
    s = s.replace(chr(0x2028), "\u2028").replace(chr(0x2029), "\u2029")
    return s.encode("utf-8")


def finding(code: str, path: str, message: str) -> dict[str, str]:
    return {"code": code, "severity": "error", "path": path, "message": message}


def verify_attestation(bundle: dict[str, Any]) -> list[dict[str, str]]:
    att = bundle.get("attestation") or {}
    subjects = att.get("subject") or []
    if not subjects:
        return [finding("KIT_ATTESTATION_ABSENT", "attestation.subject",
                        "The bundle carries no attestation subject — nothing binds the payload.")]
    declared = (subjects[0].get("digest") or {}).get("sha256", "")
    if not re.fullmatch(r"[a-f0-9]{64}", declared or ""):
        return [finding("KIT_ATTESTATION_DIGEST_MALFORMED", "attestation.subject[0].digest.sha256",
                        f"Attestation digest {declared!r} is not a sha256 hex digest.")]
    recomputed = hashlib.sha256(go_canonical_json(bundle.get("feeds"))).hexdigest()
    if recomputed != declared:
        return [finding("KIT_ATTESTATION_DIGEST_MISMATCH", "attestation.subject[0].digest.sha256",
                        f"Recomputed feeds digest {recomputed} != attested {declared} — "
                        "the payload was altered after emission.")]
    return []


def verify_hash_forms(bundle: dict[str, Any]) -> list[dict[str, str]]:
    findings: list[dict[str, str]] = []
    by_source: dict[str, str] = {}
    for fi, feed in enumerate(bundle.get("feeds") or []):
        for ni, node in enumerate(feed.get("nodes") or []):
            path = f"feeds[{fi}].nodes[{ni}]"
            source_hash = node.get("source_hash", "")
            if not SHA256_FORM.match(source_hash):
                findings.append(finding("KIT_SOURCE_HASH_FORM", f"{path}.source_hash",
                                        f"source_hash {source_hash!r} is not a real sha256 digest form."))
                continue
            src = node.get("source_path", "")
            if src in by_source and by_source[src] != source_hash:
                findings.append(finding("KIT_SOURCE_HASH_INCONSISTENT", f"{path}.source_hash",
                                        f"Nodes of {src} disagree on source_hash — a partial tamper."))
            by_source.setdefault(src, source_hash)
    return findings


def run_kit(bundle: dict[str, Any], vocab_path: Path | None = None) -> dict[str, Any]:
    findings: list[dict[str, str]] = []

    schema = bundle.get("schema_version")
    if schema != EXPECTED_SCHEMA:
        findings.append(finding("KIT_SCHEMA_VERSION", "schema_version",
                                f"schema_version {schema!r} is not {EXPECTED_SCHEMA!r} — refuse, never best-effort."))

    nucleus = _load_nucleus()
    vocab = nucleus.load_facets_vocab(vocab_path) if vocab_path else nucleus.load_facets_vocab()
    structural = nucleus.validate_bundle(bundle, vocab)
    findings.extend(structural["findings"])

    findings.extend(verify_attestation(bundle))
    findings.extend(verify_hash_forms(bundle))

    return {
        "schema_version": "nomos-consumer-kit-verdict-v1",
        "status": "fail" if findings else "pass",
        "summary": {**structural["summary"], "findings": len(findings)},
        "findings": findings,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bundle", required=True, type=Path)
    parser.add_argument("--vocab", type=Path, default=None,
                        help="facet vocabulary artifact (default: specs/generated/facets-vocab.json)")
    args = parser.parse_args()
    try:
        bundle = json.loads(args.bundle.read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001
        print(json.dumps({"status": "fail", "findings": [finding("KIT_PARSE_FAILED", str(args.bundle), str(exc))]},
                         indent=2, sort_keys=True))
        return 1
    verdict = run_kit(bundle, args.vocab)
    print(json.dumps(verdict, indent=2, sort_keys=True, ensure_ascii=False))
    return 0 if verdict["status"] == "pass" else 1


if __name__ == "__main__":
    raise SystemExit(main())

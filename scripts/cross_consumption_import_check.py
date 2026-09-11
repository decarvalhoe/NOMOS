#!/usr/bin/env python3
"""NRT-029 (#702) — the cross-consumption import proof.

A consumer indexed what `nomos rag export` handed out. This check answers,
with evidence rather than trust, one question: "is what you indexed exactly
what NOMOS exported?" — chunk by chunk, against the index manifest of
`nomos rag manifest` (nomos-rag-index-manifest-v1):

1. every manifest chunk is in the index, once, and no index record is outside
   the manifest (a hit on an unknown chunk could not be cited);
2. per chunk, `source_hash` is the manifest's, and the embedding text and body
   the consumer stored hash to the manifest's `embedding_hash` / `body_hash`
   (recomputed from the text when the record carries it, compared as given
   when it carries only the hashes);
3. per source, the chunk count is the manifest's;
4. the index digest recomputed from the consumer's own records — the engine's
   grammar, `chunk_id NUL source_hash NUL embedding_hash_hex NUL` in chunk_id
   order — equals the manifest's `chunk_digest`.

With `--citations <answers.yaml>` (repeatable) it also cross-checks the
answer records a consumer hands to `nomos answer gate`: every cited
`chunk_id` exists in the manifest and carries the manifest's `source_hash`
and `source_id`, so an answer cannot cite a chunk the bundle does not contain.

Accepted index record shapes, one JSON object per line:
  - the neutral export record (nomos-rag-chunk-v1): chunk_id, embedding_text,
    body_text, provenance.source_hash;
  - the langchain / llamaindex projections: metadata.chunk_id,
    metadata.source_hash, metadata.body_text, page_content or text;
  - a flat consumer dump: chunk_id, source_hash, then either embedding_text
    and body_text, or embedding_hash and body_hash.
A line that is none of these is a named finding, never a skipped line.

Exit 0 = the index is the export (and the citations resolve); 1 = a finding,
named in the JSON verdict and on stderr; 2 = usage (unreadable input).
Claim boundary: identity and fingerprint preservation only — no retrieval
quality, no ranking, no answer quality, nothing about a deployment.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for the cross-consumption import check.", file=sys.stderr)
    raise SystemExit(2) from exc

VERDICT_SCHEMA = "nomos-cross-consumption-import-verdict-v1"
MANIFEST_SCHEMA = "nomos-rag-index-manifest-v1"
FIELDS = ("source_hash", "embedding_hash", "body_hash")
CLAIM_BOUNDARY = (
    "The index is (or is not) the export NOMOS handed out: identities and fingerprints "
    "preserved, citations resolvable. No retrieval-quality, ranking, answer-quality or "
    "deployment claim."
)


class UsageError(RuntimeError):
    pass


def sha256_of(text: str) -> str:
    return "sha256:" + hashlib.sha256(text.encode("utf-8")).hexdigest()


def with_prefix(value: Any) -> str:
    """Normalise a digest to the `sha256:<hex>` form the manifest uses."""
    s = str(value or "").strip()
    if not s or s.startswith("sha256:"):
        return s
    return "sha256:" + s


def normalise(obj: Any) -> dict[str, str] | str:
    """Reduce one index record to its fingerprint, or say why it cannot be."""
    if not isinstance(obj, dict):
        return "record is not a JSON object"
    meta = obj.get("metadata") if isinstance(obj.get("metadata"), dict) else {}
    prov = obj.get("provenance") if isinstance(obj.get("provenance"), dict) else {}
    chunk_id = str(obj.get("chunk_id") or meta.get("chunk_id") or obj.get("id_") or "").strip()
    if not chunk_id:
        return "record carries no chunk_id"
    source_hash = with_prefix(prov.get("source_hash") or obj.get("source_hash") or meta.get("source_hash"))
    if not source_hash:
        return f"{chunk_id}: record carries no source_hash"
    embedding_text = obj.get("embedding_text")
    if embedding_text is None:
        embedding_text = obj.get("page_content") if "page_content" in obj else obj.get("text")
    body_text = obj.get("body_text") if "body_text" in obj else meta.get("body_text")
    embedding_hash = sha256_of(embedding_text) if isinstance(embedding_text, str) else with_prefix(obj.get("embedding_hash"))
    body_hash = sha256_of(body_text) if isinstance(body_text, str) else with_prefix(obj.get("body_hash"))
    if not embedding_hash or not body_hash:
        return f"{chunk_id}: record carries neither the texts (embedding_text/body_text) nor their hashes (embedding_hash/body_hash)"
    return {"chunk_id": chunk_id, "source_hash": source_hash, "embedding_hash": embedding_hash, "body_hash": body_hash}


def read_index(path: Path) -> tuple[list[dict[str, str]], list[str]]:
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except (OSError, UnicodeDecodeError) as exc:
        raise UsageError(f"index unreadable: {path}: {exc}") from exc
    records: list[dict[str, str]] = []
    errors: list[str] = []
    for number, line in enumerate(lines, 1):
        if not line.strip():
            continue
        try:
            obj = json.loads(line)
        except ValueError as exc:
            errors.append(f"index line {number}: not JSON ({exc}) — a record nobody can read is a record nobody indexed")
            continue
        got = normalise(obj)
        if isinstance(got, str):
            errors.append(f"index line {number}: {got}")
        else:
            records.append(got)
    return records, errors


def read_manifest(path: Path) -> dict[str, Any]:
    try:
        doc = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeDecodeError, ValueError) as exc:
        raise UsageError(f"manifest unreadable: {path}: {exc}") from exc
    if not isinstance(doc, dict):
        raise UsageError(f"manifest is not a JSON object: {path}")
    return doc


def digest_of(records: list[dict[str, str]]) -> str:
    """The engine's index digest, recomputed from the consumer's records."""
    h = hashlib.sha256()
    for r in sorted(records, key=lambda rec: rec["chunk_id"]):
        h.update(r["chunk_id"].encode("utf-8"))
        h.update(b"\0")
        h.update(r["source_hash"].encode("utf-8"))
        h.update(b"\0")
        h.update(r["embedding_hash"].removeprefix("sha256:").encode("utf-8"))
        h.update(b"\0")
    return "sha256:" + h.hexdigest()


def manifest_chunks(manifest: dict[str, Any]) -> dict[str, dict[str, Any]]:
    out: dict[str, dict[str, Any]] = {}
    for chunk in manifest.get("chunks") or []:
        if isinstance(chunk, dict) and str(chunk.get("chunk_id") or "").strip():
            out[str(chunk["chunk_id"])] = chunk
    return out


def check_index(manifest: dict[str, Any], records: list[dict[str, str]], line_errors: list[str]) -> tuple[list[str], dict[str, Any]]:
    findings: list[str] = list(line_errors)
    if manifest.get("schema_version") != MANIFEST_SCHEMA:
        findings.append(f"manifest schema_version {manifest.get('schema_version')!r} is not {MANIFEST_SCHEMA!r} — refuse, never best-effort")
    expected = manifest_chunks(manifest)
    if not expected:
        findings.append("the manifest lists no chunk — nothing to compare an index against")
    if not records:
        findings.append("the index is empty — nothing was ingested")
    seen: dict[str, dict[str, str]] = {}
    for r in records:
        if r["chunk_id"] in seen:
            findings.append(f"{r['chunk_id']}: indexed twice — identity is not unique")
            continue
        seen[r["chunk_id"]] = r
    missing = sorted(set(expected) - set(seen))
    extra = sorted(set(seen) - set(expected))
    findings += [f"{cid}: in the manifest, not in the index" for cid in missing]
    findings += [f"{cid}: in the index, not in the manifest — a hit on it could not be cited" for cid in extra]
    mismatched: list[str] = []
    for cid in sorted(set(expected) & set(seen)):
        for field in FIELDS:
            want = with_prefix(expected[cid].get(field))
            got = seen[cid][field]
            if want != got:
                findings.append(f"{cid}: {field} {got} != manifest {want}")
                if cid not in mismatched:
                    mismatched.append(cid)
    counts: dict[str, int] = {}
    for cid in seen:
        if cid in expected:
            source_id = str(expected[cid].get("source_id") or "")
            counts[source_id] = counts.get(source_id, 0) + 1
    for source in manifest.get("sources") or []:
        if not isinstance(source, dict):
            continue
        source_id = str(source.get("source_id") or "")
        want = int(source.get("chunk_count") or 0)
        if counts.get(source_id, 0) != want:
            findings.append(f"source {source_id}: {counts.get(source_id, 0)} chunk(s) indexed, manifest says {want}")
    recomputed = digest_of(list(seen.values())) if seen else ""
    declared = str(manifest.get("chunk_digest") or "")
    if recomputed != declared:
        findings.append(f"index digest {recomputed or '(empty)'} != manifest chunk_digest {declared or '(absent)'} — the index is not the export")
    detail = {
        "manifest_chunks": len(expected),
        "index_records": len(records),
        "matched": len(set(expected) & set(seen)) - len(mismatched),
        "missing": missing,
        "extra": extra,
        "mismatched": mismatched,
        "index_digest": recomputed,
        "manifest_chunk_digest": declared,
    }
    return findings, detail


def check_citations(manifest: dict[str, Any], fixtures_path: Path) -> tuple[list[str], dict[str, Any]]:
    try:
        doc = yaml.safe_load(fixtures_path.read_text(encoding="utf-8"))
    except (OSError, UnicodeDecodeError, yaml.YAMLError) as exc:
        raise UsageError(f"citations unreadable: {fixtures_path}: {exc}") from exc
    answers = doc.get("answers") if isinstance(doc, dict) else None
    label = str(fixtures_path)
    if not isinstance(answers, list) or not answers:
        return [f"{label}: no answers to cross-check (answers: [...] expected)"], {"file": label, "answers": 0, "references_checked": 0}
    expected = manifest_chunks(manifest)
    findings: list[str] = []
    checked = 0
    for index, answer in enumerate(answers):
        if not isinstance(answer, dict):
            findings.append(f"{label}: answers[{index}] is not a mapping")
            continue
        answer_id = str(answer.get("answer_id") or f"answers[{index}]")
        for kind in ("source_spans", "citations", "retrieved_chunks"):
            for position, ref in enumerate(answer.get(kind) or []):
                if not isinstance(ref, dict):
                    findings.append(f"{answer_id}: {kind}[{position}] is not a mapping")
                    continue
                chunk_id = str(ref.get("chunk_id") or "").strip()
                if not chunk_id:
                    findings.append(f"{answer_id}: {kind}[{position}] names no chunk_id — a citation the index cannot resolve")
                    continue
                checked += 1
                chunk = expected.get(chunk_id)
                if chunk is None:
                    findings.append(f"{answer_id}: {kind} cites {chunk_id}, which is not in the manifest")
                    continue
                source_hash = with_prefix(ref.get("source_hash"))
                if source_hash and source_hash != with_prefix(chunk.get("source_hash")):
                    findings.append(f"{answer_id}: {kind} {chunk_id} carries source_hash {source_hash}, manifest {with_prefix(chunk.get('source_hash'))}")
                source_id = str(ref.get("source_id") or "").strip()
                if source_id and source_id != str(chunk.get("source_id") or ""):
                    findings.append(f"{answer_id}: {kind} {chunk_id} carries source_id {source_id!r}, manifest {str(chunk.get('source_id') or '')!r}")
    return findings, {"file": label, "answers": len(answers), "references_checked": checked}


def main() -> int:
    parser = argparse.ArgumentParser(description="Prove that a consumer's index is the export NOMOS handed out, chunk by chunk, against the index manifest.")
    parser.add_argument("--manifest", required=True, help="index manifest written by `nomos rag manifest`")
    parser.add_argument("--index", required=True, help="the consumer's index dump, one JSON record per line")
    parser.add_argument("--citations", action="append", default=[], help="answer records YAML (answers: [...]) whose citations must resolve in the manifest; repeatable")
    parser.add_argument("--report", default=None, help="write the JSON verdict here as well as stdout")
    args = parser.parse_args()
    try:
        manifest = read_manifest(Path(args.manifest))
        records, line_errors = read_index(Path(args.index))
        findings, detail = check_index(manifest, records, line_errors)
        citations = []
        for path in args.citations:
            problems, summary = check_citations(manifest, Path(path))
            findings += problems
            citations.append(summary)
    except UsageError as exc:
        print(f"cross-consumption import check: {exc}", file=sys.stderr)
        return 2
    verdict = {
        "schema_version": VERDICT_SCHEMA,
        "status": "fail" if findings else "pass",
        "claim_boundary": CLAIM_BOUNDARY,
        "manifest": str(args.manifest),
        "index": str(args.index),
        "index_check": detail,
        "citations": citations,
        "findings": findings,
    }
    encoded = json.dumps(verdict, indent=2, sort_keys=True, ensure_ascii=False)
    if args.report:
        Path(args.report).write_text(encoded + "\n", encoding="utf-8")
    print(encoded)
    if findings:
        for problem in findings:
            print(f"cross-consumption import check: {problem}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

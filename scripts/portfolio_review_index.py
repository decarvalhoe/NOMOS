#!/usr/bin/env python3
"""NRT-021 (#669) — index and guard the periodic QMS review records.

The committed records under docs/regulated/operations/records/ (management
review, internal audit, role assignment) and records/capa/ (deviation/CAPA)
are human acts. This sidecar makes them COUNTABLE and CHECKABLE without ever
producing one:

  index   docs/regulated/operations/records/index.json
          (`nomos-review-record-index-v1`): one entry per record with its
          type, date, sha256, decisions / actions / findings / assignments
          counts, action statuses, cited artifact paths and their existence.
  guard   hard failures (exit 1): a record without record_id, record_type or
          date; a decision without id; an action without id or owner; a CAPA
          without status/severity/opened; a cited artifact path that does not
          exist in the tree.
  check   `--check`: the committed index must equal a fresh build (exit 4 on
          drift, files named).

Counted, not failed: actions whose status is not recorded (`status_unrecorded`)
— the current records track actions by `tracking:` issue references; the
count is reported so a future record format can be held to it.

Claim boundary: an index of records is not a record. Nothing here approves,
closes or performs a review.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import sys
from pathlib import Path
from typing import Any

import yaml

ROOT_DEFAULT = Path(__file__).resolve().parents[1]
RECORDS_DIR = Path("docs/regulated/operations/records")
INDEX_PATH = RECORDS_DIR / "index.json"
INDEX_SCHEMA = "nomos-review-record-index-v1"
REVIEW_TYPES = {"management_review", "internal_audit", "role_assignment"}
CAPA_TYPE = "deviation_capa"
CLAIM_BOUNDARY = (
    "An index of committed QMS records: counts, dates, hashes, cited artifacts and their existence. "
    "It is not a record, performs no review, closes no CAPA and approves nothing."
)
# A cited artifact is a repository path; prose around it is ignored.
# A file path ends with a known extension; a directory citation ends with "/".
# "docs/45" (a document number) is prose, not a path.
PATH_RE = re.compile(
    r"(?<![\w/])((?:docs|scripts|specs|cli|tests|templates|\.vrc-wiring-matrix|\.github)/"
    r"(?:[A-Za-z0-9_.-]+/)*(?:[A-Za-z0-9_.-]+\.(?:md|yaml|yml|json|go|py|cue|ttl|sh)|(?=\s|$)))"
)


class GuardError(Exception):
    pass


def sha256_file(p: Path) -> str:
    return "sha256:" + hashlib.sha256(p.read_bytes()).hexdigest()


def cited_paths(*texts: Any) -> list[str]:
    found: set[str] = set()
    for t in texts:
        if isinstance(t, str):
            for m in PATH_RE.findall(t):
                m = m.rstrip(".,;:)")
                if m and not m.endswith("/") or m.endswith("/"):
                    found.add(m)
        elif isinstance(t, list):
            for x in t:
                found.update(cited_paths(x))
        elif isinstance(t, dict):
            for x in t.values():
                found.update(cited_paths(x))
    return sorted(found)


def date_str(v: Any) -> str:
    return str(v)[:10] if v is not None else ""


def index_record(root: Path, rel: Path, failures: list[str]) -> dict[str, Any] | None:
    doc = yaml.safe_load((root / rel).read_text(encoding="utf-8"))
    if not isinstance(doc, dict):
        return None
    rtype = str(doc.get("record_type", ""))
    if rtype not in REVIEW_TYPES and rtype != CAPA_TYPE:
        return None
    rid = str(doc.get("record_id", "")).strip()
    entry: dict[str, Any] = {
        "record_id": rid,
        "record_type": rtype,
        "path": rel.as_posix(),
        "sha256": sha256_file(root / rel),
    }
    where = rel.as_posix()
    if not rid:
        failures.append(f"{where}: record_id missing")
    if rtype == CAPA_TYPE:
        entry["date"] = date_str(doc.get("opened"))
        for field in ("status", "severity", "opened"):
            if not str(doc.get(field, "")).strip():
                failures.append(f"{where}: CAPA field {field} missing")
        entry["status"] = str(doc.get("status", ""))
        entry["severity"] = str(doc.get("severity", ""))
        entry["closed"] = date_str(doc.get("closed")) or None
        entry["retro_documented"] = bool(doc.get("retro_documented", False))
        ev = doc.get("effectiveness_verification") or {}
        entry["effectiveness_verified"] = bool(ev.get("verified", False)) if isinstance(ev, dict) else False
        entry["corrective_actions"] = len(doc.get("corrective_actions") or [])
        entry["preventive_actions"] = len(doc.get("preventive_actions") or [])
        if entry["status"] == "closed" and not entry["closed"]:
            failures.append(f"{where}: closed CAPA without a closed date")
        cited = cited_paths(doc.get("corrective_actions"), doc.get("preventive_actions"), ev, doc.get("references"))
    else:
        entry["date"] = date_str(doc.get("date"))
        if not entry["date"]:
            failures.append(f"{where}: date missing")
        decisions = doc.get("decisions") or []
        actions = doc.get("actions") or []
        findings = doc.get("findings") or []
        assignments = doc.get("assignments") or []
        for i, d in enumerate(decisions):
            if not isinstance(d, dict) or not str(d.get("id", "")).strip():
                failures.append(f"{where}: decision #{i + 1} has no id")
        statuses: dict[str, int] = {}
        for i, a in enumerate(actions):
            if not isinstance(a, dict) or not str(a.get("id", "")).strip():
                failures.append(f"{where}: action #{i + 1} has no id")
                continue
            if not str(a.get("owner", "")).strip():
                failures.append(f"{where}: action {a.get('id')} has no owner")
            status = str(a.get("status", "")).strip() or "status_unrecorded"
            statuses[status] = statuses.get(status, 0) + 1
        for i, f in enumerate(findings):
            if not isinstance(f, dict) or not str(f.get("id", "")).strip() or not str(f.get("severity", "")).strip():
                failures.append(f"{where}: finding #{i + 1} lacks id or severity")
        entry.update({
            "decisions": len(decisions),
            "actions": len(actions),
            "action_statuses": dict(sorted(statuses.items())),
            "findings": len(findings),
            "assignments": len(assignments),
            "next_review_due": date_str(doc.get("next_review_due")) or None,
        })
        cited = cited_paths(doc.get("inputs"), doc.get("decisions"), doc.get("actions"), doc.get("findings"), doc.get("references"), doc.get("evidence"))
    existence = []
    for c in cited:
        exists = (root / c).exists()
        existence.append({"path": c, "exists": exists})
        if not exists:
            failures.append(f"{where}: cites {c} which does not exist in the tree")
    entry["cited_artifacts"] = existence
    return entry


def build_index(root: Path) -> tuple[dict[str, Any], list[str]]:
    failures: list[str] = []
    records: list[dict[str, Any]] = []
    base = root / RECORDS_DIR
    if not base.is_dir():
        raise GuardError(f"{RECORDS_DIR} does not exist")
    for p in sorted(base.rglob("*.yaml")):
        rel = p.relative_to(root)
        try:
            entry = index_record(root, rel, failures)
        except yaml.YAMLError as exc:
            failures.append(f"{rel.as_posix()}: not valid YAML: {exc}")
            continue
        if entry:
            records.append(entry)
    ids = [r["record_id"] for r in records if r["record_id"]]
    for dup in sorted({i for i in ids if ids.count(i) > 1}):
        failures.append(f"record_id {dup} appears more than once")
    by_type: dict[str, int] = {}
    for r in records:
        by_type[r["record_type"]] = by_type.get(r["record_type"], 0) + 1
    index = {
        "schema_version": INDEX_SCHEMA,
        "generated_by": "scripts/portfolio_review_index.py",
        "records_dir": RECORDS_DIR.as_posix(),
        "total": len(records),
        "by_type": dict(sorted(by_type.items())),
        "open_capa": sum(1 for r in records if r["record_type"] == CAPA_TYPE and r.get("status") != "closed"),
        "actions_status_unrecorded": sum(r.get("action_statuses", {}).get("status_unrecorded", 0) for r in records),
        "cited_artifacts_missing": sum(1 for r in records for c in r["cited_artifacts"] if not c["exists"]),
        "records": records,
        "claim_boundary": CLAIM_BOUNDARY,
    }
    return index, failures


def render(index: dict[str, Any]) -> str:
    return json.dumps(index, indent=2, ensure_ascii=False, sort_keys=False) + "\n"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--root", default=str(ROOT_DEFAULT))
    parser.add_argument("--check", action="store_true", help="compare the committed index with a fresh build (exit 4 on drift)")
    parser.add_argument("--out", default="", help="write the index here instead of the committed path")
    args = parser.parse_args()
    root = Path(args.root).resolve()
    try:
        index, failures = build_index(root)
    except GuardError as exc:
        print(f"review-index: REFUSED — {exc}", file=sys.stderr)
        return 1
    if failures:
        print("review-index: GUARD FAILED:", file=sys.stderr)
        for f in failures:
            print("  -", f, file=sys.stderr)
        return 1
    text = render(index)
    if args.check:
        committed = root / INDEX_PATH
        if not committed.exists() or committed.read_text(encoding="utf-8") != text:
            print(f"review-index: DRIFT — {INDEX_PATH} is not a fresh build; regenerate with scripts/portfolio_review_index.py", file=sys.stderr)
            return 4
        print(f"review-index: check OK — {index['total']} record(s), {index['open_capa']} open CAPA, {index['actions_status_unrecorded']} action(s) without recorded status")
        return 0
    out = Path(args.out) if args.out else root / INDEX_PATH
    out.write_text(text, encoding="utf-8")
    print(f"review-index: wrote {out} — {index['total']} record(s): {index['by_type']}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

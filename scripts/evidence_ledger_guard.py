#!/usr/bin/env python3
"""NRT-033 (#716) — the evidence ledger is a GENERATED index, checked in CI.

Decision (docs/51): `status: effective` on docs/regulated/evidence-index/
evidence-ledger.yaml means "index in force, computed from the tree and checked
in CI". It says nothing about the effectiveness of the documents it indexes:
their statuses are OBSERVED from the files (a `status:` marker in the first
lines of each .md/.yaml/.json) and recounted next to the declaration, never
softened.

  --write   recompute every category's `observed` block, set
            `status: effective` and today's `generated_at`; keep the claim
            boundary "Missing evidence is not assumed".
  --check   refuse: observed drift (files changed, ledger not regenerated), a
            declared presence whose location is missing, a `present`/`effective`
            declaration over drafts, `generated_by_workflow_when_run` on a path
            that is not a generated one, a category with a status outside the
            vocabulary, a malformed gap, and a ledger older than the portfolio
            freshness policy (90 days) — staleness is a finding; regenerating
            the index is the recurring DevOps action declared in the README.

Exit codes: 0 ok · 1 red · 2 usage. Categories measured by another guard
(repeated-ci-evidence/ → scripts/repeated_ci_evidence.py) keep their declared
status; this guard observes their files like the others.
"""

from __future__ import annotations

import argparse
import re
import sys
from datetime import date, datetime
from pathlib import Path

import yaml

ROOT_DEFAULT = Path(__file__).resolve().parents[1]
LEDGER = Path("docs/regulated/evidence-index/evidence-ledger.yaml")
STALE_AFTER_DAYS = 90  # portfolio freshness policy (cli/internal/portfolio DefaultStaleAfterDays)
GENERATED_PREFIXES = (".regulated-doc-gate/", ".regulated-evidence-pack/")
DECLARED_STATUSES = {"present", "present_draft", "present_measured", "draft_not_effective", "generated_by_workflow_when_run", "requires_evidence"}
PRESENT_LIKE = {"present", "present_draft", "present_measured", "draft_not_effective"}
EFFECTIVE_MARKERS = {"effective", "approved"}
DRAFT_MARKERS = {"draft", "template", "pending_approval", "not_effective", "draft_not_effective"}
GAP_FIELDS = ("id", "description", "severity", "status", "blocks_claims")
GAP_STATUSES = {"open", "closed", "mitigated"}
GAP_SEVERITIES = {"minor", "major", "critical"}
MARKER_RE = re.compile(r"^\s*(?:status|Status)\s*:\s*\**\s*([A-Za-z_][A-Za-z0-9_-]*)", re.M)
CLAIM_BOUNDARY_REQUIRED = "Missing evidence is not assumed"


class Loader(yaml.SafeLoader):
    pass


Loader.add_constructor("tag:yaml.org,2002:timestamp", lambda loader, node: loader.construct_scalar(node))


def load(root: Path) -> dict:
    p = root / LEDGER
    if not p.exists():
        raise SystemExit(f"ledger-guard: {LEDGER} is missing")
    doc = yaml.load(p.read_text(encoding="utf-8"), Loader=Loader)
    if not isinstance(doc, dict):
        raise SystemExit(f"ledger-guard: {LEDGER} is not a mapping")
    return doc


def marker_of(path: Path) -> str:
    try:
        head = "\n".join(path.read_text(encoding="utf-8", errors="replace").splitlines()[:40])
    except OSError:
        return ""
    m = MARKER_RE.search(head)
    return m.group(1).lower() if m else ""


def observe(root: Path, location: str) -> dict:
    """What the tree says about one expected_location — deterministic."""
    p = root / location
    obs: dict = {"exists": p.exists(), "kind": "absent", "generated_path": location.startswith(GENERATED_PREFIXES), "files": 0, "statuses": {}, "unmarked": 0}
    if not p.exists():
        return obs
    files = [p] if p.is_file() else sorted(f for f in p.rglob("*") if f.is_file() and f.suffix in (".md", ".yaml", ".yml", ".json"))
    obs["kind"] = "file" if p.is_file() else "directory"
    obs["files"] = len(files)
    counts: dict[str, int] = {}
    for f in files:
        mk = marker_of(f)
        if mk:
            counts[mk] = counts.get(mk, 0) + 1
        else:
            obs["unmarked"] += 1
    obs["statuses"] = dict(sorted(counts.items()))
    return obs


def check_category(cat: dict, obs: dict) -> list[str]:
    cid, declared, loc = cat.get("id", "?"), str(cat.get("current_status", "")), str(cat.get("expected_location", ""))
    errs = []
    if declared not in DECLARED_STATUSES:
        errs.append(f"{cid}: current_status {declared!r} is not in the ledger vocabulary {sorted(DECLARED_STATUSES)}")
        return errs
    if not str(cat.get("claim_allowed", "")).strip():
        errs.append(f"{cid}: claim_allowed is required ('none' when nothing is claimable)")
    if declared in PRESENT_LIKE and not obs["exists"]:
        errs.append(f"{cid}: declared {declared} but {loc} does not exist — a missing location is requires_evidence")
    if declared == "generated_by_workflow_when_run" and not obs["generated_path"]:
        errs.append(f"{cid}: generated_by_workflow_when_run is only for generated paths ({', '.join(GENERATED_PREFIXES)}), not {loc}")
    if declared in ("present", "present_measured") and obs["exists"]:
        drafts = {k: v for k, v in obs["statuses"].items() if k in DRAFT_MARKERS}
        if drafts:
            errs.append(f"{cid}: declared {declared} but the files carry draft markers {drafts} — a draft is not present evidence")
    if not obs["exists"] and declared == "requires_evidence" and obs["generated_path"]:
        pass  # a generated path may be declared requires_evidence: conservative, allowed
    return errs


def check_gaps(gaps: list) -> list[str]:
    errs, seen = [], set()
    for i, g in enumerate(gaps or []):
        where = f"blocking_gaps[{i}]"
        if not isinstance(g, dict):
            errs.append(f"{where}: not a mapping"); continue
        missing = [f for f in GAP_FIELDS if f not in g or g[f] in (None, "", [])]
        if missing:
            errs.append(f"{where} ({g.get('id', '?')}): missing {', '.join(missing)}")
        if g.get("id") in seen:
            errs.append(f"{where}: duplicate gap id {g.get('id')}")
        seen.add(g.get("id"))
        if g.get("status") not in GAP_STATUSES:
            errs.append(f"{where} ({g.get('id', '?')}): status {g.get('status')!r} not in {sorted(GAP_STATUSES)}")
        if g.get("severity") not in GAP_SEVERITIES:
            errs.append(f"{where} ({g.get('id', '?')}): severity {g.get('severity')!r} not in {sorted(GAP_SEVERITIES)}")
    return errs


def recompute(root: Path, doc: dict) -> dict:
    out = dict(doc)
    cats = []
    for cat in doc.get("evidence_categories") or []:
        c = dict(cat)
        c["observed"] = observe(root, str(cat.get("expected_location", "")))
        cats.append(c)
    out["evidence_categories"] = cats
    return out


def parse_day(v) -> date | None:
    try:
        return date.fromisoformat(str(v)[:10])
    except ValueError:
        return None


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--root", default=str(ROOT_DEFAULT))
    ap.add_argument("--check", action="store_true")
    ap.add_argument("--write", action="store_true")
    ap.add_argument("--today", default="", help="ISO date (tests); default today")
    args = ap.parse_args()
    if args.check == args.write:
        print("ledger-guard: choose exactly one of --check or --write", file=sys.stderr)
        return 2
    root = Path(args.root).resolve()
    today = date.fromisoformat(args.today) if args.today else date.today()
    doc = load(root)
    errs: list[str] = []
    if CLAIM_BOUNDARY_REQUIRED not in str(doc.get("claim_boundary", "")):
        errs.append(f"claim_boundary must keep the rule {CLAIM_BOUNDARY_REQUIRED!r}")
    cats = doc.get("evidence_categories") or []
    if not cats:
        errs.append("evidence_categories is empty")
    ids = [c.get("id") for c in cats]
    if len(ids) != len(set(ids)):
        errs.append("duplicate evidence category ids")
    fresh = recompute(root, doc)
    for cat, new in zip(cats, fresh["evidence_categories"]):
        errs += check_category(cat, new["observed"])
    errs += check_gaps(doc.get("blocking_gaps") or [])
    if args.write:
        if errs:
            print("ledger-guard: RED — fix the declarations before regenerating", file=sys.stderr)
            for e in errs:
                print("  -", e, file=sys.stderr)
            return 1
        fresh["status"] = "effective"
        fresh["generated_at"] = today.isoformat()
        fresh["index_rule"] = ("status: effective means this INDEX is in force — computed from the tree by scripts/evidence_ledger_guard.py and "
                               "checked in CI. It says nothing about the effectiveness of the indexed documents: their statuses are observed "
                               "and recounted under each category's observed block, never softened.")
        (root / LEDGER).write_text(yaml.safe_dump(fresh, sort_keys=False, allow_unicode=True, width=100), encoding="utf-8")
        print(f"ledger-guard: wrote {LEDGER} — {len(cats)} categories observed, status effective, generated_at {today}")
        return 0
    # --check
    if doc.get("status") != "effective":
        errs.append(f"status is {doc.get('status')!r}; the index in force is written by --write (status: effective)")
    day = parse_day(doc.get("generated_at"))
    if day is None:
        errs.append("generated_at is not an ISO date")
    elif (today - day).days > STALE_AFTER_DAYS:
        errs.append(f"generated_at {day} is older than {STALE_AFTER_DAYS} days — stale index; regenerate with --write (recurring DevOps action)")
    for cat, new in zip(cats, fresh["evidence_categories"]):
        if cat.get("observed") != new["observed"]:
            errs.append(f"{cat.get('id')}: observed block drifts from the tree (declared {cat.get('observed')}, tree {new['observed']}) — regenerate with --write")
    if "index_rule" not in doc:
        errs.append("index_rule is missing — the ledger must say what effective means here")
    if errs:
        print("ledger-guard: RED", file=sys.stderr)
        for e in errs:
            print("  -", e, file=sys.stderr)
        return 1
    print(f"ledger-guard: OK — index effective, generated_at {day}, {len(cats)} categories observed and in agreement with the tree, {len(doc.get('blocking_gaps') or [])} gap(s) well-formed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

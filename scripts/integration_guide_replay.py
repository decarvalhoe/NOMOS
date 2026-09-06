#!/usr/bin/env python3
"""NRT-027 (#680) — the customer integration guide is EXECUTED, not trusted.

Every fenced ```bash block of the guide that follows a `<!-- replay -->`
marker is run, in order, as one bash script (`set -euo pipefail`) with:

  $NOMOS  the CLI binary built once from cli/ (or --nomos)
  $REPO   the repository root
  $WORK   a scratch directory shared by all blocks of one run (cwd)

`<!-- replay expects: a.json, out/b.yaml -->` names artifacts (relative to
$WORK) that must exist after the block. A block that exits non-zero, or an
expected artifact that is missing, is red and named.

A markdown table that follows `<!-- contracts -->` names the contracts the
guide relies on with their stability and version; each row is checked
against specs/contract-registry.yaml (the registry of NRT-023), so a
stability the registry does not confirm is red.

Exit codes: 0 all blocks ran and the table agrees · 1 red · 2 usage.
What this proves: the guide runs against the repository's fixtures on this
toolchain. What it does not prove: that a customer validated it.
"""

from __future__ import annotations

import argparse
import os
import re
import shutil
import subprocess
import sys
import tempfile
from dataclasses import dataclass, field
from pathlib import Path

import yaml

ROOT_DEFAULT = Path(__file__).resolve().parents[1]
GUIDE_DEFAULT = Path("docs/48-customer-integration-guide.md")
REGISTRY = Path("specs/contract-registry.yaml")
REPLAY_RE = re.compile(r"^\s*<!--\s*replay(?:\s+expects:\s*(?P<expects>[^>]*?))?\s*-->\s*$")
CONTRACTS_RE = re.compile(r"^\s*<!--\s*contracts\s*-->\s*$")
MIN_BLOCKS = 5


@dataclass
class Block:
    index: int
    line: int
    script: str
    expects: list[str] = field(default_factory=list)


@dataclass
class ContractRow:
    line: int
    id: str
    stability: str
    version: str


def parse_guide(text: str) -> tuple[list[Block], list[ContractRow]]:
    lines = text.splitlines()
    blocks: list[Block] = []
    rows: list[ContractRow] = []
    i = 0
    while i < len(lines):
        m = REPLAY_RE.search(lines[i])
        if m:
            expects = [e.strip() for e in (m.group("expects") or "").split(",") if e.strip()]
            j = i + 1
            while j < len(lines) and not lines[j].strip():
                j += 1
            if j >= len(lines) or not lines[j].strip().startswith("```"):
                raise SystemExit(f"guide:{i + 1}: replay marker is not followed by a fenced block")
            k = j + 1
            body = []
            while k < len(lines) and not lines[k].strip().startswith("```"):
                body.append(lines[k])
                k += 1
            if k >= len(lines):
                raise SystemExit(f"guide:{j + 1}: unterminated fenced block")
            blocks.append(Block(len(blocks) + 1, j + 1, "\n".join(body) + "\n", expects))
            i = k + 1
            continue
        if CONTRACTS_RE.search(lines[i]):
            j = i + 1
            while j < len(lines) and not lines[j].strip():
                j += 1
            header = lines[j] if j < len(lines) else ""
            if not header.strip().startswith("|"):
                raise SystemExit(f"guide:{i + 1}: contracts marker is not followed by a table")
            j += 2  # header + separator
            while j < len(lines) and lines[j].strip().startswith("|"):
                cells = [c.strip().strip("`") for c in lines[j].strip().strip("|").split("|")]
                if len(cells) < 3:
                    raise SystemExit(f"guide:{j + 1}: contracts table row needs id, stability, version")
                rows.append(ContractRow(j + 1, cells[0], cells[1], cells[2].replace("-", "") if cells[2] in ("-", "—") else cells[2]))
                j += 1
            i = j
            continue
        i += 1
    return blocks, rows


def check_contracts(root: Path, rows: list[ContractRow]) -> list[str]:
    reg = yaml.safe_load((root / REGISTRY).read_text(encoding="utf-8"))
    by = {c["id"]: c for c in reg["contracts"]}
    errs = []
    for r in rows:
        c = by.get(r.id)
        if c is None:
            errs.append(f"guide:{r.line}: contract `{r.id}` is not in {REGISTRY}")
            continue
        if c["stability"] != r.stability:
            errs.append(f"guide:{r.line}: `{r.id}` is declared {r.stability} in the guide but {c['stability']} in the registry")
        if r.version and str(c.get("schema_version") or "") and str(c["schema_version"]) != r.version:
            errs.append(f"guide:{r.line}: `{r.id}` version {r.version} in the guide, {c['schema_version']} in the registry")
    return errs


def build_nomos(root: Path, work: Path) -> Path:
    binary = work / "bin" / "nomos"
    binary.parent.mkdir(parents=True, exist_ok=True)
    r = subprocess.run(["go", "build", "-o", str(binary), "."], cwd=root / "cli", capture_output=True, text=True)
    if r.returncode != 0:
        raise SystemExit(f"replay: go build failed:\n{r.stderr}")
    return binary


def run_blocks(root: Path, work: Path, nomos: Path, blocks: list[Block]) -> list[str]:
    env = dict(os.environ, NOMOS=str(nomos), REPO=str(root), WORK=str(work))
    errs = []
    for b in blocks:
        r = subprocess.run(["bash", "-euo", "pipefail", "-c", b.script], cwd=work, env=env, capture_output=True, text=True)
        head = b.script.strip().splitlines()[0] if b.script.strip() else "(empty)"
        if r.returncode != 0:
            tail = "\n".join((r.stderr or r.stdout).strip().splitlines()[-6:])
            errs.append(f"block {b.index} (guide:{b.line}, starts `{head}`) exited {r.returncode}:\n{tail}")
            break  # later blocks depend on earlier artifacts; one red is the verdict
        missing = [e for e in b.expects if not (work / e).exists()]
        if missing:
            errs.append(f"block {b.index} (guide:{b.line}) ran but did not produce: {', '.join(missing)}")
            break
        print(f"replay: block {b.index} ok — {head}" + (f" → {', '.join(b.expects)}" if b.expects else ""))
    return errs


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--root", default=str(ROOT_DEFAULT))
    ap.add_argument("--guide", default=str(GUIDE_DEFAULT))
    ap.add_argument("--work", default="", help="scratch directory (default: a fresh temp dir, removed unless --keep)")
    ap.add_argument("--keep", action="store_true")
    ap.add_argument("--nomos", default="", help="use this binary instead of building cli/")
    ap.add_argument("--min-blocks", type=int, default=MIN_BLOCKS)
    args = ap.parse_args()
    root = Path(args.root).resolve()
    guide = root / args.guide if not Path(args.guide).is_absolute() else Path(args.guide)
    if not guide.exists():
        print(f"replay: guide not found: {guide}", file=sys.stderr)
        return 2
    blocks, rows = parse_guide(guide.read_text(encoding="utf-8"))
    errs = []
    if len(blocks) < args.min_blocks:
        errs.append(f"the guide has {len(blocks)} replay block(s); at least {args.min_blocks} are required — a guide without commands proves nothing")
    if not rows:
        errs.append("the guide names no contract under a <!-- contracts --> table")
    errs += check_contracts(root, rows)
    if errs:
        print("replay: RED", file=sys.stderr)
        for e in errs:
            print("  -", e, file=sys.stderr)
        return 1
    work = Path(args.work).resolve() if args.work else Path(tempfile.mkdtemp(prefix="nomos-guide-"))
    work.mkdir(parents=True, exist_ok=True)
    (work / "out").mkdir(exist_ok=True)
    try:
        nomos = Path(args.nomos).resolve() if args.nomos else build_nomos(root, work)
        errs = run_blocks(root, work, nomos, blocks)
        if errs:
            print("replay: RED", file=sys.stderr)
            for e in errs:
                print("  -", e, file=sys.stderr)
            print(f"replay: work dir kept for inspection: {work}", file=sys.stderr)
            return 1
        print(f"replay: OK — {len(blocks)} block(s) ran against the fixtures, {len(rows)} contract row(s) agree with {REGISTRY}")
        return 0
    finally:
        if not errs and not args.keep and not args.work:
            shutil.rmtree(work, ignore_errors=True)


if __name__ == "__main__":
    raise SystemExit(main())

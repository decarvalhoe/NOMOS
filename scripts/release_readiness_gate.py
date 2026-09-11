#!/usr/bin/env python3
"""NRT-034 (#717) — the release-readiness verdict as a candidate GATE.

Runs `nomos portfolio release-readiness` on the tree, re-verifies the written
verdict (digest, criteria, status binding) and exits 0 only when the verdict
is `ready`. Anything else — not_ready, unreadable, forged — is exit 1 with the
unmet checks named. Used by scripts/release_candidate_gates.py under the id
`release-readiness`, which the v1.0.0-BETA.1 candidate spec requires: no beta
candidate is assembled on a `not_ready` verdict. Not a release, not an approval.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import tempfile
from pathlib import Path


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--root", default=".")
    ap.add_argument("--out", default="", help="keep the verdict here (default: temp file)")
    ap.add_argument("--verdict-file", default="", help="gate an already written verdict instead of computing (tests)")
    args = ap.parse_args()
    root = Path(args.root).resolve()
    out = Path(args.out) if args.out else Path(tempfile.mkdtemp(prefix="nomos-readiness-")) / "release-readiness.json"
    if args.verdict_file:
        out = Path(args.verdict_file)
    else:
        r = subprocess.run(["go", "run", ".", "portfolio", "release-readiness", "--repo-root", str(root), "--out", str(out)], cwd=root / "cli", capture_output=True, text=True)
        if r.returncode != 0:
            print("release-readiness gate: the verdict could not be computed:\n" + (r.stderr or r.stdout).strip(), file=sys.stderr)
            return 1
    v = subprocess.run(["go", "run", ".", "portfolio", "release-readiness", "--repo-root", str(root), "--verify", str(out)], cwd=root / "cli", capture_output=True, text=True)
    if v.returncode != 0:
        print("release-readiness gate: the verdict does not re-verify — " + (v.stderr or v.stdout).strip(), file=sys.stderr)
        return 1
    doc = json.loads(out.read_text(encoding="utf-8"))
    if doc.get("verdict") != "ready":
        print(f"release-readiness gate: verdict is {doc.get('verdict')!r}; a beta candidate is not assembled on this tree", file=sys.stderr)
        for u in doc.get("unmet") or []:
            print("  -", u, file=sys.stderr)
        return 1
    print(f"release-readiness gate: ready — {len(doc.get('criteria') or [])} criteria met, bound to {doc.get('status_digest')}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

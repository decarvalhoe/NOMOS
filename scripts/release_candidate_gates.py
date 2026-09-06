#!/usr/bin/env python3
"""#639 — run the gate set for a release candidate and RECORD what happened.

Emits `nomos-release-candidate-gates-v1` JSON: the commit the gates ran on,
when, and for each gate its command, exit code and status. `nomos release
candidate` then refuses a candidate unless every required gate is present here
with status pass and exit 0 on the SAME commit. Nothing here interprets
results — a failing gate is written as failing, and the candidate is refused
downstream.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
PY = sys.executable

GATES: list[tuple[str, list[str], Path]] = [
    ("claim-boundary-guard", [PY, "scripts/claim_boundary_guard.py", "--root", "."], ROOT),
    ("vrc-wiring-matrix", [PY, "scripts/vrc_wiring_matrix.py", "--root", "."], ROOT),
    ("roadmap-lane-guard", [PY, "scripts/roadmap_lane_guard.py", "--root", "."], ROOT),
    ("repeated-ci-evidence", [PY, "scripts/repeated_ci_evidence.py", "--root", "."], ROOT),
    ("training-competence-gate", [PY, "scripts/training_competence_gate.py", "--root", "."], ROOT),
    ("go-vet", ["go", "vet", "./..."], ROOT / "cli"),
    ("go-test", ["go", "test", "./..."], ROOT / "cli"),
    # NRT-034 (#717): the v1.0 readiness verdict, ready or the beta candidate is refused.
    ("release-readiness", [PY, "scripts/release_readiness_gate.py", "--root", "."], ROOT),
]


def run_gate(gate_id: str, cmd: list[str], cwd: Path) -> dict:
    t0 = time.monotonic()
    try:
        r = subprocess.run(cmd, cwd=cwd, capture_output=True, text=True)
        code = r.returncode
        tail = (r.stderr or r.stdout).strip().splitlines()[-3:]
    except FileNotFoundError as exc:
        code, tail = 127, [str(exc)]
    return {
        "id": gate_id,
        "command": " ".join(cmd if cmd[0] != PY else ["python3", *cmd[1:]]),
        "exit_code": code,
        "status": "pass" if code == 0 else "fail",
        "seconds": round(time.monotonic() - t0, 2),
        "tail": tail,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out", required=True)
    parser.add_argument("--only", default="", help="comma-separated gate ids to run (default: all)")
    parser.add_argument("--commit", default="", help="commit the gates run on (default: git HEAD)")
    args = parser.parse_args()
    commit = args.commit or subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT, capture_output=True, text=True, check=True).stdout.strip()
    only = {g for g in args.only.split(",") if g}
    results = [run_gate(gid, cmd, cwd) for gid, cmd, cwd in GATES if not only or gid in only]
    for r in results:
        print(f"[gates] {r['status']:4} {r['id']} (exit {r['exit_code']}, {r['seconds']}s)")
    doc = {
        "schema_version": "nomos-release-candidate-gates-v1",
        "commit": commit,
        "measured_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "gates": results,
        "claim_boundary": "Exit codes of the named gates on the named commit, as measured. A gate absent here was not run.",
    }
    Path(args.out).parent.mkdir(parents=True, exist_ok=True)
    Path(args.out).write_text(json.dumps(doc, indent=2) + "\n", encoding="utf-8")
    failed = [r["id"] for r in results if r["status"] != "pass"]
    print(f"[gates] wrote {args.out}; {len(results) - len(failed)}/{len(results)} pass" + (f"; FAILED: {', '.join(failed)}" if failed else ""))
    return 0  # recording succeeded; the candidate command judges the content


if __name__ == "__main__":
    raise SystemExit(main())

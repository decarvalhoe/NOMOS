"""NRT-034 (#717) — the release-readiness gate: ready or the beta candidate is refused."""

from __future__ import annotations

import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
GATE = ROOT / "scripts" / "release_readiness_gate.py"
GATES = ROOT / "scripts" / "release_candidate_gates.py"
BETA_SPEC = ROOT / "docs" / "regulated" / "release" / "v1.0.0-BETA.1-candidate.yaml"


class ReleaseReadinessGateTests(unittest.TestCase):
    def test_beta_spec_requires_the_gate_and_alpha_does_not(self) -> None:
        import yaml
        beta = yaml.safe_load(BETA_SPEC.read_text(encoding="utf-8"))
        alpha = yaml.safe_load((ROOT / "docs/regulated/release/v0.2.0-ALPHA-candidate.yaml").read_text(encoding="utf-8"))
        self.assertIn("release-readiness", beta["gates"]["required"])
        self.assertNotIn("release-readiness", alpha["gates"]["required"])
        self.assertEqual(beta["approval_status"], "pending")
        self.assertEqual(beta["approvals"], [])
        self.assertFalse(beta["release_executed"])
        self.assertEqual(beta["version"], "v1.0.0-BETA.1")

    def test_gate_is_green_on_the_real_tree_and_measured_by_the_gate_set(self) -> None:
        if not shutil.which("go"):
            self.skipTest("go required")
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp) / "gates.json"
            r = subprocess.run([sys.executable, str(GATES), "--only", "release-readiness", "--out", str(out), "--commit", "test"], capture_output=True, text=True)
            self.assertEqual(r.returncode, 0, r.stderr + r.stdout)
            doc = json.loads(out.read_text(encoding="utf-8"))
            gate = [g for g in doc["gates"] if g["id"] == "release-readiness"][0]
            self.assertEqual((gate["status"], gate["exit_code"]), ("pass", 0), gate)

    def test_forged_or_not_ready_verdict_is_refused(self) -> None:
        if not shutil.which("go"):
            self.skipTest("go required")
        with tempfile.TemporaryDirectory() as tmp:
            verdict = Path(tmp) / "release-readiness.json"
            r = subprocess.run([sys.executable, str(GATE), "--root", str(ROOT), "--out", str(verdict)], capture_output=True, text=True)
            self.assertEqual(r.returncode, 0, r.stderr)
            doc = json.loads(verdict.read_text(encoding="utf-8"))
            doc["verdict"] = "not_ready"  # edited after computation: the digest no longer matches
            verdict.write_text(json.dumps(doc), encoding="utf-8")
            r = subprocess.run([sys.executable, str(GATE), "--root", str(ROOT), "--verdict-file", str(verdict)], capture_output=True, text=True)
            self.assertEqual(r.returncode, 1)
            self.assertIn("does not re-verify", r.stderr)


if __name__ == "__main__":
    unittest.main()

"""NRT-019 (#667) — the portfolio status is a computed, closed, self-consistent view.

1. CUE contract (when cue is installed): the valid fixture passes, a narrative
   field and a non-pending candidate fail.
2. The committed valid fixture is byte-identical to a fresh computation over the
   synthetic mini-repo at the fixed clock (when go is installed).
3. On the real repository the status is self-consistent: section availability
   counts match, the capabilities section agrees with the committed matrix, and
   every section key is present.
"""

from __future__ import annotations

import json
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CUE = ROOT / "specs" / "portfolio-status.cue"
VALID = ROOT / "specs" / "examples" / "portfolio-status.valid.json"
INVALID = sorted((ROOT / "specs" / "examples").glob("portfolio-status.invalid-*.json"))
SECTIONS = ["capabilities", "roadmap", "gaps", "capa", "reviews", "repeated_ci", "praxis_gate", "competence", "domain_packs", "public_sources", "release_candidate"]


@unittest.skipUnless(shutil.which("cue"), "cue not installed")
class Contract(unittest.TestCase):
    def vet(self, p: Path) -> int:
        return subprocess.run(["cue", "vet", str(CUE), str(p), "-d", "#PortfolioStatus"], capture_output=True, text=True).returncode

    def test_contract_refuses_narrative_and_non_pending_candidate(self) -> None:
        self.assertEqual(self.vet(VALID), 0)
        self.assertEqual(len(INVALID), 2)
        for p in INVALID:
            self.assertNotEqual(self.vet(p), 0, p.name)


@unittest.skipUnless(shutil.which("go"), "go not installed")
class Engine(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.tmp = tempfile.mkdtemp()
        cls.bin = str(Path(cls.tmp) / "nomos")
        r = subprocess.run(["go", "build", "-o", cls.bin, "."], cwd=ROOT / "cli", capture_output=True, text=True)
        if r.returncode != 0:
            raise unittest.SkipTest("nomos build failed: " + r.stderr[-400:])

    def status(self, *args: str) -> dict:
        r = subprocess.run([self.bin, "portfolio", "status", *args], capture_output=True, text=True, cwd=ROOT)
        self.assertEqual(r.returncode, 0, r.stderr)
        return json.loads(r.stdout)

    def test_valid_fixture_is_a_fresh_computation(self) -> None:
        r = subprocess.run([self.bin, "portfolio", "status", "--repo-root", "cli/internal/portfolio/testdata/minirepo", "--now", "2026-09-06T00:00:00Z",
                            "--release-candidate", "candidate-manifest.json"], capture_output=True, text=True, cwd=ROOT)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(r.stdout, VALID.read_text(encoding="utf-8"), "committed fixture drifted from the engine output")

    def test_findings_and_reviews_on_the_real_repository(self) -> None:
        # NRT-020 (#668): while the Praxis gate is blocked its unmet requirements are findings;
        # every finding names a source hash; filters are exact; the review index is non-empty.
        r = subprocess.run([self.bin, "portfolio", "findings", "--repo-root", str(ROOT)], capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stderr)
        rep = json.loads(r.stdout)
        self.assertEqual(rep["schema_version"], "nomos-portfolio-findings-v1")
        self.assertEqual(rep["sources_unavailable"], [])
        kinds = {f["kind"] for f in rep["findings"]}
        self.assertIn("praxis_requirement_unmet", kinds)
        self.assertIn("evidence_gap", kinds)
        for f in rep["findings"]:
            self.assertTrue(f["source"]["sha256"].startswith("sha256:"), f)
            self.assertTrue(f["id"].split(":")[0], f)
        self.assertEqual(rep["total"], len(rep["findings"]))
        self.assertEqual(rep["consistency_findings"], sum(1 for f in rep["findings"] if f["consistency"]))
        r = subprocess.run([self.bin, "portfolio", "findings", "--repo-root", str(ROOT), "--kind", "evidence_gap", "--lane", "regulated"], capture_output=True, text=True)
        sub = json.loads(r.stdout)
        self.assertTrue(all(f["kind"] == "evidence_gap" for f in sub["findings"]))
        self.assertEqual(sub["total"], len(sub["findings"]))
        r = subprocess.run([self.bin, "portfolio", "reviews", "--repo-root", str(ROOT)], capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stderr)
        reviews = json.loads(r.stdout)
        self.assertGreaterEqual(reviews["total"], 3)
        self.assertTrue(all(r_["cited_artifacts"] is not None for r_ in reviews["records"]))

    def test_real_repository_status_is_self_consistent(self) -> None:
        st = self.status("--repo-root", ".")
        self.assertEqual(st["schema_version"], "nomos-portfolio-status-v1")
        for key in SECTIONS:
            self.assertIn(key, st)
        unavailable = sum(1 for k in SECTIONS if st[k].get("available") is False)
        self.assertEqual(unavailable, st["sections_unavailable"])
        stale = 0
        for k in SECTIONS:
            sec = st[k]
            if sec.get("available") is False:
                continue
            for v in sec.values():
                if isinstance(v, dict) and v.get("freshness") == "stale":
                    stale += 1
            for item in (sec.get("records") or sec.get("packs") or []):
                if isinstance(item, dict) and (item.get("source") or {}).get("freshness") == "stale":
                    stale += 1
        self.assertEqual(stale, st["sections_stale"])
        caps = st["capabilities"]
        matrix = json.loads((ROOT / ".vrc-wiring-matrix" / "wiring-matrix.json").read_text(encoding="utf-8"))
        self.assertTrue(caps["expected_vs_computed_agree"])
        self.assertEqual(caps["total"], matrix["summary"]["capabilities"])
        self.assertEqual(caps["mismatches"], 0)
        self.assertEqual(st["praxis_gate"]["status"], "blocked")
        self.assertIn("lifts no claim", st["claim_boundary"])


if __name__ == "__main__":
    unittest.main()

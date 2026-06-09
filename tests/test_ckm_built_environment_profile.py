from __future__ import annotations

import subprocess
import shutil
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
PROFILE = ROOT / "specs/examples/nomos-domain-profile.built-environment.valid.yaml"


class CKMBuiltEnvironmentProfileTests(unittest.TestCase):
    def test_profile_passes_domain_profile_contract(self) -> None:
        if shutil.which("cue") is None:
            self.skipTest("cue not available")
        result = subprocess.run(
            [
                "cue",
                "vet",
                "specs/nomos-domain-profile.cue",
                "specs/examples/nomos-domain-profile.built-environment.valid.yaml",
                "-d",
                "#DomainProfile",
            ],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )

        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_source_authority_register_and_golden_corpus_are_declared(self) -> None:
        profile = yaml.safe_load(PROFILE.read_text(encoding="utf-8"))
        self.assertEqual(profile["domain_profile"], "built-environment")

        artifacts = {artifact["id"]: artifact for artifact in profile["required_artifacts"]}
        self.assertIn("ch-source-authority-register", artifacts)
        self.assertIn("vd-lausanne-golden-corpus", artifacts)

        fixture_path = ROOT / artifacts["vd-lausanne-golden-corpus"]["path"]
        fixture = yaml.safe_load(fixture_path.read_text(encoding="utf-8"))
        self.assertEqual(fixture["domain_profile"], "built-environment")
        self.assertIn("no building compliance certification", fixture["claim_boundary"].lower())

        register = fixture["source_authority_register"]
        self.assertGreaterEqual(len(register), 3)
        for source in register:
            with self.subTest(source=source["source_id"]):
                self.assertIn(source["level"], {"confederation", "canton", "commune"})
                self.assertTrue(source["theme"])
                self.assertIn(source["access_policy"], {"repository_public", "licensed_reference_only"})
                self.assertIn(source["machine_readability"], {"structured", "semi_structured", "pdf_text", "human_only"})

        golden_cases = {case["case_id"]: case for case in fixture["golden_corpus"]}
        self.assertIn("VD-LAUSANNE-PERMIT-001", golden_cases)
        self.assertIn("VD-LAUSANNE-FIRE-001", golden_cases)
        for case in golden_cases.values():
            with self.subTest(case=case["case_id"]):
                self.assertEqual(case["feed_status"], "pass")
                self.assertEqual(case["toc_status"], "pass")
                self.assertEqual(case["gate_status"], "pass")
                self.assertIn(case["jurisdiction"], {"VD", "Lausanne"})


if __name__ == "__main__":
    unittest.main()

from __future__ import annotations

import shutil
import subprocess
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
PROFILE = ROOT / "specs/examples/nomos-domain-profile.business-operations.valid.yaml"
METIER_ATOM = ROOT / "specs/examples/facets.business-metier.valid.yaml"
FIXTURE = ROOT / "docs/regulated/domain-packs/business-operations/business-operations-fixture.yaml"


class CKMBusinessBibleTests(unittest.TestCase):
    def test_business_profile_accepts_business_bible_source_class(self) -> None:
        self.assertTrue(PROFILE.exists(), f"Missing business profile: {PROFILE}")
        profile = yaml.safe_load(PROFILE.read_text(encoding="utf-8"))

        self.assertEqual(profile["domain_profile"], "business-operations")
        references = {item["id"]: item for item in profile["references"]}
        self.assertEqual(references["BIZ-OPS-BIBLE"]["authority_type"], "business_bible")
        self.assertEqual(profile["claim_ladder"]["current_level"], "mapped")

        if shutil.which("cue") is None:
            self.skipTest("cue is not installed")

        result = subprocess.run(
            [
                "cue",
                "vet",
                "specs/nomos-domain-profile.cue",
                "specs/examples/nomos-domain-profile.business-operations.valid.yaml",
                "-d",
                "#DomainProfile",
            ],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_metier_nature_validates_as_faceted_atom(self) -> None:
        self.assertTrue(METIER_ATOM.exists(), f"Missing metier atom fixture: {METIER_ATOM}")

        if shutil.which("cue") is None:
            self.skipTest("cue is not installed")

        result = subprocess.run(
            [
                "cue",
                "vet",
                "specs/atomization-spine.cue",
                "specs/facets.cue",
                "specs/examples/facets.business-metier.valid.yaml",
                "-d",
                "#FacetedAtom",
            ],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_non_aec_golden_corpus_uses_same_core_mechanics(self) -> None:
        self.assertTrue(FIXTURE.exists(), f"Missing business fixture: {FIXTURE}")
        fixture = yaml.safe_load(FIXTURE.read_text(encoding="utf-8"))

        self.assertEqual(fixture["pack_id"], "business-operations")
        self.assertFalse(fixture["golden_corpus"]["aec_domain"])
        self.assertEqual(fixture["source_authority_register"][0]["source_class"], "business_bible")
        self.assertEqual(fixture["source_authority_register"][0]["nature"], "metier")
        self.assertEqual(
            fixture["golden_corpus"]["core_mechanics"],
            {
                "feed": "green",
                "certified_toc": "green",
                "body_ledger": "green",
                "release_gate": "green",
            },
        )
        self.assertIn("same_core_mechanics_different_vocabularies", fixture["demonstration"])


if __name__ == "__main__":
    unittest.main()

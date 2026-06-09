from __future__ import annotations

import json
import shutil
import subprocess
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
PROMOTION_SPEC = ROOT / "specs/canon-promotion.cue"
VALID_FIXTURE = ROOT / "specs/examples/canon-promotion.valid.yaml"
INVALID_FIXTURE = ROOT / "specs/examples/canon-promotion.invalid-shared.yaml"
VALIDATOR = ROOT / "scripts/ckm_canon_promotion_validate.py"


class CKMCanonPromotionTests(unittest.TestCase):
    def test_contract_uses_customer_source_review_state_and_certificate(self) -> None:
        self.assertTrue(PROMOTION_SPEC.exists(), f"Missing promotion contract: {PROMOTION_SPEC}")
        self.assertTrue(VALID_FIXTURE.exists(), f"Missing valid fixture: {VALID_FIXTURE}")

        fixture = yaml.safe_load(VALID_FIXTURE.read_text(encoding="utf-8"))
        self.assertEqual(fixture["record_type"], "ckm_canon_promotion_bundle")
        self.assertEqual(fixture["source"]["authority_type"], "customer_source")
        self.assertEqual(fixture["source"]["access_policy"], "customer_confidential")
        self.assertEqual(fixture["atoms"][0]["review_state"], "approved")
        self.assertEqual(fixture["certificates"][0]["revoked"], False)

        if shutil.which("cue") is None:
            self.skipTest("cue is not installed")

        result = subprocess.run(
            [
                "cue",
                "vet",
                "specs/atomization-spine.cue",
                "specs/facets.cue",
                "specs/canon-promotion.cue",
                "specs/examples/canon-promotion.valid.yaml",
                "-d",
                "#CanonPromotionBundle",
            ],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_confidential_user_promoted_canon_stays_siloed(self) -> None:
        report = self._validate(VALID_FIXTURE)

        self.assertEqual(report["status"], "pass")
        self.assertEqual(report["shared_catalog_exposed_source_ids"], [])
        self.assertEqual(report["siloed_source_ids"], ["SRC-CUSTOMER-CLAIMS"])
        self.assertEqual(report["promoted_atoms"]["ATOM-CANON-PROMOTED-001"]["provenance"], "user_promoted")
        self.assertEqual(report["promoted_atoms"]["ATOM-CANON-PROMOTED-001"]["trust_tier"], "indicative")

    def test_shared_export_or_certified_trust_is_rejected(self) -> None:
        self.assertTrue(INVALID_FIXTURE.exists(), f"Missing invalid fixture: {INVALID_FIXTURE}")
        result = subprocess.run(
            ["python", str(VALIDATOR), "--bundle", str(INVALID_FIXTURE)],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )

        self.assertNotEqual(result.returncode, 0)
        self.assertIn("customer_confidential_source_exposed", result.stdout)
        self.assertIn("user_promoted_cannot_be_certified", result.stdout)

    def _validate(self, fixture: Path) -> dict:
        self.assertTrue(VALIDATOR.exists(), f"Missing validator: {VALIDATOR}")
        result = subprocess.run(
            ["python", str(VALIDATOR), "--bundle", str(fixture)],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        return json.loads(result.stdout)


if __name__ == "__main__":
    unittest.main()

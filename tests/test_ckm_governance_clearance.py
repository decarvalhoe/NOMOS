from __future__ import annotations

import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
IP_ROOT = ROOT / "docs/regulated/ip-governance"
WORDMARK = IP_ROOT / "nomos-wordmark-clearance.md"
FTO = IP_ROOT / "nomos-fto-note.md"
LICENSE_REGISTER = IP_ROOT / "license-register.yaml"
PUBLIC_CLAIM_BOUNDARY = ROOT / "docs/public-claim-boundary.md"


class CKMGovernanceClearanceTests(unittest.TestCase):
    def test_wordmark_clearance_records_crowding_and_blocks_public_claims(self) -> None:
        self.assertTrue(WORDMARK.exists(), f"Missing wordmark note: {WORDMARK}")
        content = WORDMARK.read_text(encoding="utf-8")

        for required in (
            "NOMOS Canonical Knowledge Mesh",
            "not legal advice",
            "WIPO Global Brand Database",
            "EUIPO eSearch",
            "USPTO Trademark Search",
            "NOMOS AI",
            "clearance_required_before_public_claim",
        ):
            with self.subTest(required=required):
                self.assertIn(required, content)

    def test_fto_note_scopes_patent_risk_for_rag_governance(self) -> None:
        self.assertTrue(FTO.exists(), f"Missing FTO note: {FTO}")
        content = FTO.read_text(encoding="utf-8")

        for required in (
            "preliminary FTO screen",
            "not a freedom-to-operate opinion",
            "patent counsel",
            "RAG governance",
            "claim chart required",
            "do not implement from patent text",
        ):
            with self.subTest(required=required):
                self.assertIn(required, content)

    def test_license_register_isolates_agpl_risk_dependencies(self) -> None:
        self.assertTrue(
            LICENSE_REGISTER.exists(),
            f"Missing license register: {LICENSE_REGISTER}",
        )
        register = yaml.safe_load(LICENSE_REGISTER.read_text(encoding="utf-8"))
        self.assertEqual(register["record_type"], "ckm_license_register")
        self.assertEqual(register["policy"]["agpl_boundary"], "process_api_boundary")

        dependencies = {item["id"]: item for item in register["dependencies"]}
        for dependency_id in ("openfisca-core", "lexnlp"):
            with self.subTest(dependency_id=dependency_id):
                dependency = dependencies[dependency_id]
                self.assertEqual(dependency["integration_policy"], "process_api_boundary")
                self.assertFalse(dependency["may_vendor_code"])
                self.assertFalse(dependency["may_link_in_process"])
                self.assertIn("AGPL", dependency["risk_basis"])

        self.assertIn("license_review_required_before_integration", register["blocked_claims"])

    def test_public_claim_boundary_blocks_ip_clearance_claims(self) -> None:
        content = PUBLIC_CLAIM_BOUNDARY.read_text(encoding="utf-8")

        for required in (
            "trademark clearance",
            "freedom-to-operate opinion",
            "third-party license clearance",
        ):
            with self.subTest(required=required):
                self.assertIn(required, content)


if __name__ == "__main__":
    unittest.main()

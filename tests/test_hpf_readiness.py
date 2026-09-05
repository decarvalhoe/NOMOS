from __future__ import annotations

import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
MATRIX_PATH = ROOT / "docs/roadmap/high-potential-pilot-readiness.yaml"
DOC_PATH = ROOT / "docs/roadmap/high-potential-pilot-readiness.md"


REQUIRED_LANES = {
    "gxp_csv",
    "ai_rag_governance",
    "medical_samd",
    "finance_regtech",
    "cyber_supplier_assurance",
    "legal_ediscovery",
    "verifiable_evidence",
    "github_app_workflow",
    "control_plane_portfolio",
}

VALID_STATUSES = {"go", "wait", "blocked"}

DOMAIN_PROFILE_EXPECTATIONS = {
    "ai_rag_governance": {
        "path": "specs/examples/nomos-domain-profile.ai.valid.yaml",
        "domain_profile": "ai-governance",
        "blocked_terms": ("certified", "regulatory compliance"),
    },
    "gxp_csv": {
        "path": "specs/examples/nomos-domain-profile.gxp.valid.yaml",
        "domain_profile": "gxp-csv",
        "blocked_terms": ("gxp compliant", "part 11"),
    },
    "finance_regtech": {
        "path": "specs/examples/nomos-domain-profile.finance.valid.yaml",
        "domain_profile": "finance-regtech",
        "blocked_terms": ("financial regulatory compliance", "disclosures"),
    },
    "medical_samd": {
        "path": "specs/examples/nomos-domain-profile.medical-samd.valid.yaml",
        "domain_profile": "medical-samd",
        "blocked_terms": ("medical-device", "iso 13485", "clinical"),
    },
    "cyber_supplier_assurance": {
        "path": "specs/examples/nomos-domain-profile.cyber-supplier-assurance.valid.yaml",
        "domain_profile": "cyber-supplier-assurance",
        "blocked_terms": ("certifies supplier security", "supplier security compliance"),
    },
    "verifiable_evidence": {
        "path": "specs/examples/nomos-domain-profile.verifiable-evidence.valid.yaml",
        "domain_profile": "verifiable-evidence",
        "blocked_terms": ("regulated validation", "legal compliance"),
    },
    "legal_ediscovery": {
        "path": "specs/examples/nomos-domain-profile.legal.valid.yaml",
        "domain_profile": "legal-ediscovery",
        "blocked_terms": ("legal advice", "legal sufficiency"),
    },
}


class HPFReadinessTests(unittest.TestCase):
    def load_matrix(self) -> dict[str, object]:
        return yaml.safe_load(MATRIX_PATH.read_text(encoding="utf-8"))

    def test_readiness_matrix_has_required_governance_boundary(self) -> None:
        matrix = self.load_matrix()

        self.assertEqual(matrix["schema_version"], "0.2.0")
        self.assertEqual(matrix["decision_date"], "2026-09-05")
        self.assertEqual(matrix["matrix_id"], "HPF-001")
        self.assertEqual(matrix["record_type"], "high_potential_pilot_readiness_matrix")
        boundary = str(matrix["claim_boundary"]).lower()

        for blocked_claim in (
            "no certification",
            "no compliance claim",
            "no customer validation",
            "no legal sufficiency",
        ):
            self.assertIn(blocked_claim, boundary)

        self.assertEqual(matrix["first_two_pilot_lanes"], ["ai_rag_governance", "gxp_csv"])
        wait_semantics = matrix["status_semantics"]["wait"].lower()
        self.assertIn("licensed references gate only their clause-level use/claim", wait_semantics)
        self.assertNotIn("licensed references must land", wait_semantics)

    def test_each_lane_has_status_gate_evidence_and_existing_artifacts(self) -> None:
        matrix = self.load_matrix()
        lanes = matrix["lanes"]
        self.assertIsInstance(lanes, list)
        by_id = {lane["id"]: lane for lane in lanes}

        self.assertTrue(REQUIRED_LANES.issubset(by_id))
        self.assertEqual(len({lane["rank"] for lane in lanes}), len(lanes))

        for lane_id, lane in by_id.items():
            with self.subTest(lane=lane_id):
                self.assertIn(lane["status"], VALID_STATUSES)
                self.assertRegex(lane["issue"], r"^#\d+$")
                self.assertGreaterEqual(int(lane["rank"]), 1)
                self.assertTrue(str(lane["evidence_gate_or_dependency"]).strip())
                self.assertTrue(lane["verification_commands"])

                artifacts = lane["evidence_artifacts"]
                self.assertIsInstance(artifacts, list)
                self.assertGreaterEqual(len(artifacts), 3)
                for artifact in artifacts:
                    artifact_path = ROOT / artifact
                    self.assertTrue(
                        artifact_path.exists(),
                        f"{lane_id} references missing artifact {artifact}",
                    )

                claim_impact = lane["claim_impact"]
                self.assertTrue(claim_impact["allowed"])
                self.assertTrue(claim_impact["prohibited"])
                self.assertTrue(claim_impact["blocked_levels"])

    def test_first_two_pilot_lanes_are_go_and_evidence_backed(self) -> None:
        matrix = self.load_matrix()
        by_id = {lane["id"]: lane for lane in matrix["lanes"]}

        for lane_id in matrix["first_two_pilot_lanes"]:
            with self.subTest(lane=lane_id):
                lane = by_id[lane_id]
                self.assertEqual(lane["status"], "go")
                self.assertGreaterEqual(len(lane["evidence_artifacts"]), 5)

        self.assertLess(by_id["ai_rag_governance"]["rank"], by_id["gxp_csv"]["rank"])

    def test_claim_dependencies_do_not_block_bounded_planning(self) -> None:
        matrix = self.load_matrix()
        by_id = {lane["id"]: lane for lane in matrix["lanes"]}

        medical = by_id["medical_samd"]
        self.assertEqual(medical["status"], "go")
        self.assertIn("#192", medical["claim_dependencies"])
        self.assertIn("#193", medical["claim_dependencies"])
        self.assertNotIn("external_dependencies", medical)

        legal = by_id["legal_ediscovery"]
        self.assertEqual(legal["status"], "blocked")
        self.assertIn("customer_counsel_review", legal["external_dependencies"])

        control_plane = by_id["control_plane_portfolio"]
        self.assertEqual(control_plane["status"], "wait")
        self.assertIn(
            "docs/regulated/control-plane/multi-corpus-roadmap.yaml",
            control_plane["evidence_artifacts"],
        )

    def test_markdown_summary_tracks_machine_readable_record(self) -> None:
        matrix = self.load_matrix()
        doc = DOC_PATH.read_text(encoding="utf-8")

        self.assertIn("no certification", doc.lower())
        self.assertIn("no compliance", doc.lower())
        for lane in matrix["lanes"]:
            self.assertIn(lane["id"], doc)
            self.assertIn(lane["issue"], doc)

    def test_hpf_domain_profiles_keep_blocked_claims_explicit(self) -> None:
        for lane_id, expectation in DOMAIN_PROFILE_EXPECTATIONS.items():
            with self.subTest(lane=lane_id):
                profile = yaml.safe_load((ROOT / expectation["path"]).read_text(encoding="utf-8"))
                self.assertEqual(profile["domain_profile"], expectation["domain_profile"])

                not_authorized = " ".join(profile["intended_use"]["not_authorized"]).lower()
                blocked_claims = " ".join(
                    str(claim.get("statement", "")) + " " + str(claim.get("reason", ""))
                    for claim in profile["claim_ladder"]["blocked_claims"]
                ).lower()
                profile_boundary = f"{not_authorized} {blocked_claims}"

                for blocked_term in expectation["blocked_terms"]:
                    self.assertIn(blocked_term, profile_boundary)


if __name__ == "__main__":
    unittest.main()

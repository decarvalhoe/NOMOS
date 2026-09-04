"""VRC-16 (#562) — competence status is computed from signed records, or it is not claimed.

Doctrine §2.3: the proof is the failure. Every test here breaks one rule and
proves the gate refuses to call it competence — an unsigned record, a failed
assessment recorded as a pass, an expired one, a self-assessment without a
recorded waiver, a competence id that resolves to nothing, a role handed to a
human with no defined training, and published statuses that drift from the
records.

The shipped tree is pinned too: zero attestations, zero established roles.
"""

from __future__ import annotations

import copy
import importlib.util
import subprocess
import sys
import tempfile
import unittest
from datetime import date
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "training_competence_gate.py"
TRAINING_DIR = ROOT / "docs" / "regulated" / "operations" / "training-records"

_SPEC = importlib.util.spec_from_file_location("training_competence_gate", SCRIPT)
tcg = importlib.util.module_from_spec(_SPEC)
assert _SPEC.loader is not None
_SPEC.loader.exec_module(tcg)

AS_OF = date(2026, 9, 4)
PERSON = "someone (someone@example.com)"
OTHER = "second assessor (second@example.com)"

MATRIX = {
    "schema_version": "0.1.0",
    "roles": [
        {
            "role_id": "quality_owner",
            "status": "requires_evidence",
            "required_competences": [
                {"id": "comp-qo-001", "name": "One"},
                {"id": "comp-qo-002", "name": "Two"},
            ],
        }
    ],
}

CROSSWALK = {
    "schema_version": "nomos-role-crosswalk-v1",
    "assigned_roles": [
        {
            "assigned_role": "quality_owner",
            "matrix_role_id": "quality_owner",
            "sop_role": "Quality owner",
            "disposition": "mapped",
        }
    ],
}

ASSIGNMENTS = {
    "record_id": "TEST-ROLE-001",
    "assignments": [{"role": "quality_owner", "assignee": PERSON}],
}

CONTROL_MATRIX = {
    "controls": [{"control_id": "CTL-QS-004", "current_status": "not_qualified"}]
}

SOP = (
    "# Training And Competence SOP\n\n"
    "| Role | Required training | Evidence status |\n"
    "|---|---|---|\n"
    "| Quality owner | QMS, document control. | requires_evidence |\n"
)


def _attestation(record_id: str, competence_id: str, **overrides) -> dict:
    """A fully valid, independently assessed attestation."""
    record = {
        "record_id": record_id,
        "assessee": {"name": PERSON, "role_id": "quality_owner"},
        "assessor": {"name": OTHER, "role_id": "quality_unit"},
        "competence": {"id": competence_id, "name": "One", "required_level": "proficient"},
        "assessment": {
            "date": "2026-08-01",
            "method": "written_assessment",
            "result": "pass",
        },
        "decision": {
            "competent": True,
            "signed_by_assessor": True,
            "signed_by_assessee": True,
            "signed_at": "2026-08-01T10:00:00Z",
        },
        "approval": {"approved_by": OTHER, "approved_at": "2026-08-02T09:00:00Z"},
        "validity": {"effective_from": "2026-08-01", "expires_at": None},
    }
    for dotted, value in overrides.items():
        section, _, field = dotted.partition("__")
        if field:
            record[section][field] = value
        else:
            record[section] = value
    return record


class GateTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = Path(self._tmp.name) / "repo"

        self.training = self.root / "docs" / "regulated" / "operations" / "training-records"
        self.attestations = self.training / "attestations"
        self.attestations.mkdir(parents=True)
        (self.root / "docs" / "regulated" / "operations" / "records").mkdir(parents=True)
        (self.root / "docs" / "regulated" / "quality-system").mkdir(parents=True)
        (self.root / "docs" / "regulated" / "control-matrix").mkdir(parents=True)

        self._dump(self.training / "training-matrix.yaml", MATRIX)
        self._dump(self.training / "role-crosswalk.yaml", CROSSWALK)
        self._dump(
            self.root / "docs/regulated/operations/records/2026-06-11-role-assignment-record.yaml",
            ASSIGNMENTS,
        )
        self._dump(
            self.root / "docs/regulated/control-matrix/nomos-control-matrix.yaml", CONTROL_MATRIX
        )
        (self.root / "docs/regulated/quality-system/training-and-competence-sop.md").write_text(
            SOP, encoding="utf-8"
        )
        self._dump(
            self.training / "independence-waiver.yaml",
            {"schema_version": "nomos-independence-waiver-v1", "waived_records": []},
        )

    @staticmethod
    def _dump(path: Path, data) -> None:
        path.write_text(yaml.safe_dump(data, sort_keys=False), encoding="utf-8")

    def _write_attestation(self, record: dict) -> Path:
        path = self.attestations / f"{record['record_id']}.yaml"
        self._dump(path, record)
        return path

    def _evaluate(self):
        return tcg.evaluate(self.root, AS_OF)

    def _failures(self) -> set[str]:
        checks, _ = self._evaluate()
        return {c["check"] for c in checks if c["status"] == "fail"}

    def _status(self, role: str = "quality_owner") -> str:
        _, summary = self._evaluate()
        return summary["roles"][role]["status"]

    def _publish_established(self) -> None:
        """Move every published status to `established`, as a real promotion would."""
        matrix = copy.deepcopy(MATRIX)
        matrix["roles"][0]["status"] = "established"
        self._dump(self.training / "training-matrix.yaml", matrix)
        self._dump(
            self.root / "docs/regulated/control-matrix/nomos-control-matrix.yaml",
            {"controls": [{"control_id": "CTL-QS-004", "current_status": "qualified"}]},
        )
        (self.root / "docs/regulated/quality-system/training-and-competence-sop.md").write_text(
            SOP.replace("requires_evidence", "established"), encoding="utf-8"
        )

    # --- the honest empty state -------------------------------------------

    def test_no_attestation_means_requires_evidence_never_established(self) -> None:
        self.assertEqual(self._failures(), set())
        self.assertEqual(self._status(), "requires_evidence")

    # --- the only way to `established` ------------------------------------

    def test_full_coverage_establishes_the_role(self) -> None:
        self._write_attestation(_attestation("REC-1", "comp-qo-001"))
        self._write_attestation(_attestation("REC-2", "comp-qo-002"))
        # The records alone are not enough: the published statuses must follow.
        self.assertEqual(self._failures(), {"published_status"})
        self.assertEqual(self._status(), "established")

        self._publish_established()
        self.assertEqual(self._failures(), set())

    def test_one_missing_competence_keeps_the_whole_role_short(self) -> None:
        # ADVERSARIAL: partial coverage is not competence.
        self._write_attestation(_attestation("REC-1", "comp-qo-001"))
        self.assertEqual(self._status(), "requires_evidence")
        _, summary = self._evaluate()
        self.assertEqual(summary["roles"]["quality_owner"]["missing"], ["comp-qo-002"])

    # --- forged or incomplete records -------------------------------------

    def test_unsigned_record_is_refused(self) -> None:
        for field in ("signed_by_assessor", "signed_by_assessee"):
            with self.subTest(field=field):
                self.attestations.mkdir(exist_ok=True)
                for stale in self.attestations.glob("*.yaml"):
                    stale.unlink()
                record = _attestation("REC-1", "comp-qo-001")
                record["decision"][field] = False
                self._write_attestation(record)
                self.assertIn("attestation_records", self._failures())
                self.assertEqual(self._status(), "requires_evidence")

    def test_failed_assessment_is_not_evidence_of_competence(self) -> None:
        for result in ("fail", "conditional", ""):
            with self.subTest(result=result):
                for stale in self.attestations.glob("*.yaml"):
                    stale.unlink()
                record = _attestation("REC-1", "comp-qo-001")
                record["assessment"]["result"] = result
                self._write_attestation(record)
                self.assertIn("attestation_records", self._failures())

    def test_expired_record_stops_counting(self) -> None:
        # ADVERSARIAL: competence recorded in 2024 with a 2026-01 expiry does not
        # make anyone competent today.
        record = _attestation("REC-1", "comp-qo-001")
        record["validity"]["expires_at"] = "2026-01-01"
        self._write_attestation(record)
        self.assertIn("attestation_records", self._failures())

    def test_record_valid_today_expires_later(self) -> None:
        # Precision: the same record is fine before its expiry.
        record = _attestation("REC-1", "comp-qo-001")
        record["validity"]["expires_at"] = "2027-01-01"
        self._write_attestation(record)
        self.assertEqual(self._failures(), set())

        checks, _ = tcg.evaluate(self.root, date(2027, 6, 1))
        self.assertIn(
            "attestation_records", {c["check"] for c in checks if c["status"] == "fail"}
        )

    def test_unknown_competence_id_is_refused(self) -> None:
        # ADVERSARIAL: an attestation for a competence nobody defined.
        self._write_attestation(_attestation("REC-1", "comp-invented-999"))
        self.assertIn("attestation_records", self._failures())

    def test_attestation_for_someone_with_no_assigned_role_is_refused(self) -> None:
        record = _attestation("REC-1", "comp-qo-001")
        record["assessee"]["name"] = "a passer-by (nobody@example.com)"
        self._write_attestation(record)
        self.assertIn("attestation_records", self._failures())

    def test_unapproved_record_is_refused(self) -> None:
        record = _attestation("REC-1", "comp-qo-001")
        record["approval"]["approved_by"] = ""
        self._write_attestation(record)
        self.assertIn("attestation_records", self._failures())

    def test_duplicate_record_id_is_refused(self) -> None:
        self._write_attestation(_attestation("REC-1", "comp-qo-001"))
        duplicate = _attestation("REC-1", "comp-qo-002")
        self._dump(self.attestations / "another-file.yaml", duplicate)
        self.assertIn("attestation_records", self._failures())

    # --- independence ------------------------------------------------------

    def test_self_assessment_without_a_waiver_is_refused(self) -> None:
        # ADVERSARIAL: the solo operator assesses themselves and says nothing.
        record = _attestation("REC-1", "comp-qo-001")
        record["assessor"]["name"] = PERSON
        self._write_attestation(record)
        checks, _ = self._evaluate()
        problems = next(c for c in checks if c["check"] == "attestation_records")["problems"]
        self.assertTrue(any("self-assessed" in p for p in problems), problems)

    def test_recorded_waiver_lets_a_self_assessment_count(self) -> None:
        record = _attestation("REC-1", "comp-qo-001")
        record["assessor"]["name"] = PERSON
        self._write_attestation(record)
        self._dump(
            self.training / "independence-waiver.yaml",
            {
                "schema_version": "nomos-independence-waiver-v1",
                "waived_records": [
                    {
                        "record_id": "REC-1",
                        "waived_on": "2026-08-01",
                        "approved_by": PERSON,
                        "compensating_controls": "Adversarial CI gates and public claim boundary.",
                    }
                ],
            },
        )
        self.assertEqual(self._failures(), set())

    def test_hollow_waiver_is_refused(self) -> None:
        # ADVERSARIAL: a waiver with no date, approver or compensating control is
        # a rubber stamp, not a quality decision.
        record = _attestation("REC-1", "comp-qo-001")
        record["assessor"]["name"] = PERSON
        self._write_attestation(record)
        for missing in ("waived_on", "approved_by", "compensating_controls"):
            with self.subTest(missing=missing):
                waiver = {
                    "record_id": "REC-1",
                    "waived_on": "2026-08-01",
                    "approved_by": PERSON,
                    "compensating_controls": "Adversarial CI gates.",
                }
                waiver[missing] = ""
                self._dump(
                    self.training / "independence-waiver.yaml",
                    {
                        "schema_version": "nomos-independence-waiver-v1",
                        "waived_records": [waiver],
                    },
                )
                self.assertIn("attestation_records", self._failures())

    def test_waiver_for_another_record_does_not_cover_this_one(self) -> None:
        record = _attestation("REC-1", "comp-qo-001")
        record["assessor"]["name"] = PERSON
        self._write_attestation(record)
        self._dump(
            self.training / "independence-waiver.yaml",
            {
                "schema_version": "nomos-independence-waiver-v1",
                "waived_records": [
                    {
                        "record_id": "SOME-OTHER-RECORD",
                        "waived_on": "2026-08-01",
                        "approved_by": PERSON,
                        "compensating_controls": "Adversarial CI gates.",
                    }
                ],
            },
        )
        self.assertIn("attestation_records", self._failures())

    # --- role vocabulary ---------------------------------------------------

    def test_role_held_but_absent_from_the_crosswalk_is_refused(self) -> None:
        # ADVERSARIAL: a role handed to a human with nothing deciding what it
        # requires. This is the defect the crosswalk was written to expose.
        assignments = copy.deepcopy(ASSIGNMENTS)
        assignments["assignments"].append({"role": "brand_new_role", "assignee": PERSON})
        self._dump(
            self.root / "docs/regulated/operations/records/2026-06-11-role-assignment-record.yaml",
            assignments,
        )
        checks, _ = self._evaluate()
        problems = next(c for c in checks if c["check"] == "role_crosswalk")["problems"]
        self.assertTrue(any("brand_new_role" in p for p in problems), problems)

    def test_crosswalk_pointing_at_a_missing_matrix_role_is_refused(self) -> None:
        crosswalk = copy.deepcopy(CROSSWALK)
        crosswalk["assigned_roles"][0]["matrix_role_id"] = "role_that_does_not_exist"
        self._dump(self.training / "role-crosswalk.yaml", crosswalk)
        self.assertIn("role_crosswalk", self._failures())

    def test_role_without_defined_training_can_never_be_established(self) -> None:
        # Even with attestations in the tree, a role whose training was never
        # defined stays an open gap rather than becoming a pass.
        crosswalk = copy.deepcopy(CROSSWALK)
        crosswalk["assigned_roles"][0] = {
            "assigned_role": "quality_owner",
            "matrix_role_id": None,
            "sop_role": None,
            "disposition": "requires_definition",
        }
        self._dump(self.training / "role-crosswalk.yaml", crosswalk)
        self._write_attestation(_attestation("REC-1", "comp-qo-001"))
        self._write_attestation(_attestation("REC-2", "comp-qo-002"))
        self.assertEqual(self._status(), "requires_definition")

    def test_vacant_role_has_nobody_to_train(self) -> None:
        assignments = copy.deepcopy(ASSIGNMENTS)
        assignments["assignments"].append({"role": "independent_reviewer", "assignee": "vacant"})
        self._dump(
            self.root / "docs/regulated/operations/records/2026-06-11-role-assignment-record.yaml",
            assignments,
        )
        _, summary = self._evaluate()
        self.assertNotIn("independent_reviewer", summary["roles"])

    # --- published status drift -------------------------------------------

    def test_matrix_claiming_established_without_records_is_refused(self) -> None:
        # ADVERSARIAL: flip the status column and hope nobody recomputes.
        matrix = copy.deepcopy(MATRIX)
        matrix["roles"][0]["status"] = "established"
        self._dump(self.training / "training-matrix.yaml", matrix)
        self.assertIn("published_status", self._failures())

    def test_sop_table_drifting_from_the_records_is_refused(self) -> None:
        (self.root / "docs/regulated/quality-system/training-and-competence-sop.md").write_text(
            SOP.replace("requires_evidence", "established"), encoding="utf-8"
        )
        checks, _ = self._evaluate()
        problems = next(c for c in checks if c["check"] == "published_status")["problems"]
        self.assertTrue(any("training-and-competence-sop" in p for p in problems), problems)

    def test_control_qualified_without_records_is_refused(self) -> None:
        self._dump(
            self.root / "docs/regulated/control-matrix/nomos-control-matrix.yaml",
            {"controls": [{"control_id": "CTL-QS-004", "current_status": "qualified"}]},
        )
        checks, _ = self._evaluate()
        problems = next(c for c in checks if c["check"] == "published_status")["problems"]
        self.assertTrue(any("CTL-QS-004" in p for p in problems), problems)

    # --- process -----------------------------------------------------------

    def test_missing_inputs_are_exit_2_not_a_pass(self) -> None:
        (self.training / "training-matrix.yaml").unlink()
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(self.root)],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn("NOT EVALUATED", result.stderr)

    def test_script_exits_1_on_drift(self) -> None:
        matrix = copy.deepcopy(MATRIX)
        matrix["roles"][0]["status"] = "established"
        self._dump(self.training / "training-matrix.yaml", matrix)
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(self.root)],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)


class ShippedTreeTests(unittest.TestCase):
    def test_real_tree_is_consistent(self) -> None:
        checks, _ = tcg.evaluate(ROOT, AS_OF)
        failures = [c for c in checks if c["status"] == "fail"]
        self.assertEqual(failures, [], failures)

    def test_shipped_state_is_zero_attestations_and_zero_established(self) -> None:
        # Pins the honest state. When the first real attestation lands, this test
        # is the one that must be updated deliberately.
        _, summary = tcg.evaluate(ROOT, AS_OF)
        self.assertEqual(summary["attestations_valid"], 0)
        self.assertEqual(summary["established_roles"], [])
        self.assertGreater(summary["held_roles"], 0)

    def test_attestations_directory_holds_no_generated_record(self) -> None:
        # A guard against the one failure mode that would matter most here: a
        # tool filling in a competence record nobody signed.
        records = tcg.load_attestations(TRAINING_DIR / "attestations")
        self.assertEqual([path.name for path, _ in records], [])

    def test_gate_runs_clean_as_a_subprocess(self) -> None:
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(ROOT), "--as-of", AS_OF.isoformat()],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()

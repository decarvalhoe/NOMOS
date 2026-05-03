from __future__ import annotations

import json
import hashlib
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def write_bytes(path: Path, data: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(data)


def run_script(script: str, *args: str, cwd: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(ROOT / "scripts" / script), *args],
        cwd=cwd,
        text=True,
        capture_output=True,
        check=False,
    )


def make_minimal_regulated_repo(tmp_path: Path) -> Path:
    write(
        tmp_path / "docs/regulated/reference-basis/external-reference-register.yaml",
        """
schema_version: "0.1.0"
nomos_bible_policy:
  all_registered_references_are_canonical: true
references:
  - id: FDA-CSA-2025
    title: Computer Software Assurance
    url: https://www.fda.gov/
    evidence_status: requires_evidence
""".lstrip(),
    )
    write(
        tmp_path / "docs/regulated/evidence-index/evidence-ledger.yaml",
        """
schema_version: "0.1.0"
evidence_categories:
  - id: EV-QMS-001
    current_status: draft_not_effective
""".lstrip(),
    )
    write(
        tmp_path / "docs/regulated/quality-system/quality-manual.md",
        """
document_id: TEST-QMS-001
version: 0.1.0
status: draft
effective_status: not_effective
owner: not_assigned
approver: not_assigned
""".lstrip(),
    )
    write(
        tmp_path / "templates/regulated/validation-protocol.yaml",
        """
schema_version: "0.1.0"
record_type: validation_protocol
""".lstrip(),
    )
    write(
        tmp_path / ".github/ISSUE_TEMPLATE/regulated-gap.yml",
        """
name: Regulated gap
description: Capture a regulated gap.
body:
  - type: input
    id: gap
    attributes:
      label: Gap
    validations:
      required: true
""".lstrip(),
    )
    write(tmp_path / ".github/PULL_REQUEST_TEMPLATE.md", "## Regulated impact\n")
    write(tmp_path / ".github/CODEOWNERS", "# no fake owners\n")
    write(
        tmp_path / ".github/workflows/regulated-documentation-gate.yml",
        """
name: Regulated Documentation Gate
on: workflow_dispatch
jobs:
  gate:
    runs-on: ubuntu-latest
    steps:
      - run: echo gate
""".lstrip(),
    )
    write(
        tmp_path / ".github/workflows/regulated-evidence-pack.yml",
        """
name: Regulated Evidence Pack
on: workflow_dispatch
jobs:
  evidence:
    runs-on: ubuntu-latest
    steps:
      - run: echo evidence
""".lstrip(),
    )
    return tmp_path


class RegulatedAutomationTests(unittest.TestCase):
    def test_evidence_pack_hashes_local_regulated_records(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = make_minimal_regulated_repo(Path(tmp))
            output = repo / "out/evidence-pack.json"

            result = run_script(
                "regulated_evidence_pack.py",
                "--root",
                str(repo),
                "--output",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "generated")
            self.assertEqual(
                report["claim_boundary"],
                "Evidence inventory only; no compliance certification.",
            )
            self.assertGreaterEqual(report["summary"]["records_hashed"], 7)
            self.assertTrue(any(record["path"].endswith("quality-manual.md") for record in report["records"]))
            self.assertTrue(all(record["sha256"] for record in report["records"]))

    def test_github_qms_audit_offline_reports_live_controls_as_unverified(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = make_minimal_regulated_repo(Path(tmp))
            output = repo / "out/github-qms-audit.json"

            result = run_script(
                "regulated_github_qms_audit.py",
                "--root",
                str(repo),
                "--repo",
                "RBOKproject/NOMOS",
                "--offline",
                "--output",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "requires_evidence")
            self.assertTrue(report["offline"])
            self.assertEqual(report["checks"]["issue_forms"]["status"], "present")
            self.assertEqual(report["checks"]["pull_request_template"]["status"], "present")
            self.assertEqual(report["checks"]["codeowners"]["status"], "requires_human_review")
            self.assertEqual(report["checks"]["branch_protection"]["status"], "requires_live_evidence")
            self.assertEqual(report["checks"]["rulesets"]["status"], "requires_live_evidence")

    def test_regulated_docs_gate_creates_report_parent_directory(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = make_minimal_regulated_repo(Path(tmp))
            output = repo / "nested/reports/regulated-doc-gate.json"

            result = run_script(
                "regulated_docs_gate.py",
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "passed")

    def test_reference_canon_marks_gamp5_as_licensed_bible(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            register = repo / "docs/regulated/reference-basis/external-reference-register.yaml"
            write(
                register,
                """
schema_version: "0.1.0"
nomos_bible_policy:
  all_registered_references_are_canonical: true
references:
  - id: FDA-CSA-2025
    title: Computer Software Assurance
    publisher: US FDA
    url: https://www.fda.gov/
    evidence_status: requires_evidence
  - id: ISPE-GAMP5-2E-2022
    title: ISPE GAMP 5 Guide Second Edition
    publisher: ISPE
    url: https://ispe.org/publications/guidance-documents/gamp-5
    content_access_policy: licensed_content_required
    evidence_status: licensed_reference_required_before_clause_mapping
""".lstrip(),
            )
            output = repo / "out/reference-canon.json"

            result = run_script(
                "regulated_reference_canon.py",
                "--root",
                str(repo),
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "requires_evidence")
            self.assertEqual(report["summary"]["canonical_bibles"], 2)
            gamp = next(item for item in report["bibles"] if item["id"] == "ISPE-GAMP5-2E-2022")
            self.assertEqual(gamp["canonical_role"], "nomos_bible")
            self.assertEqual(gamp["content_access_policy"], "licensed_content_required")
            self.assertEqual(gamp["nomos_processing_policy"], "licensed_local_artifact_required")
            self.assertFalse(gamp["full_text_fetch_allowed"])

    def test_reference_canon_verifies_licensed_intake_hash(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp) / "repo"
            licensed_root = Path(tmp) / "licensed"
            payload = b"licensed reference content"
            digest = hashlib.sha256(payload).hexdigest().upper()
            write_bytes(licensed_root / "ISPE-GAMP5-2E-2022/source.pdf", payload)
            write(
                repo / "docs/regulated/reference-basis/external-reference-register.yaml",
                """
schema_version: "0.1.0"
nomos_bible_policy:
  all_registered_references_are_canonical: true
references:
  - id: ISPE-GAMP5-2E-2022
    title: ISPE GAMP 5 Guide Second Edition
    publisher: ISPE
    url: https://ispe.org/publications/guidance-documents/gamp-5
    content_access_policy: licensed_content_required
    evidence_status: local_artifact_registered_license_review_required_before_clause_mapping
""".lstrip(),
            )
            write(
                repo / "docs/regulated/reference-basis/licensed-intakes/ISPE-GAMP5-2E-2022.yaml",
                f"""
schema_version: "0.1.0"
record_type: licensed_reference_intake
reference_id: ISPE-GAMP5-2E-2022
allowed_use:
  internal_processing_by_nomos: approved_for_internal_nomos_processing
  commit_full_text_to_git: false
  customer_redistribution: false
storage:
  licensed_root_env: NOMOS_LICENSED_REFERENCE_ROOT
  local_relative_path: ISPE-GAMP5-2E-2022/source.pdf
source_integrity:
  sha256: {digest}
review:
  reviewer: qms-owner@example.com
  approval_status: approved_for_internal_nomos_processing
""".lstrip(),
            )
            output = repo / "out/reference-canon.json"

            result = run_script(
                "regulated_reference_canon.py",
                "--root",
                str(repo),
                "--licensed-root",
                str(licensed_root),
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "ready_for_processing")
            self.assertEqual(report["summary"]["licensed_reference_gaps"], 0)
            self.assertEqual(report["bibles"][0]["licensed_artifact_status"], "verified")
            self.assertTrue(report["bibles"][0]["license_review_verified"])

    def test_reference_canon_blocks_verified_artifact_without_license_review(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp) / "repo"
            licensed_root = Path(tmp) / "licensed"
            payload = b"licensed reference content"
            digest = hashlib.sha256(payload).hexdigest().upper()
            write_bytes(licensed_root / "ISPE-GAMP5-2E-2022/source.pdf", payload)
            write(
                repo / "docs/regulated/reference-basis/external-reference-register.yaml",
                """
schema_version: "0.1.0"
nomos_bible_policy:
  all_registered_references_are_canonical: true
references:
  - id: ISPE-GAMP5-2E-2022
    title: ISPE GAMP 5 Guide Second Edition
    publisher: ISPE
    url: https://ispe.org/publications/guidance-documents/gamp-5
    content_access_policy: licensed_content_required
    evidence_status: local_artifact_registered_license_review_required_before_clause_mapping
""".lstrip(),
            )
            write(
                repo / "docs/regulated/reference-basis/licensed-intakes/ISPE-GAMP5-2E-2022.yaml",
                f"""
schema_version: "0.1.0"
record_type: licensed_reference_intake
reference_id: ISPE-GAMP5-2E-2022
allowed_use:
  internal_processing_by_nomos: requires_license_review
  commit_full_text_to_git: false
  customer_redistribution: false
storage:
  licensed_root_env: NOMOS_LICENSED_REFERENCE_ROOT
  local_relative_path: ISPE-GAMP5-2E-2022/source.pdf
source_integrity:
  sha256: {digest}
review:
  reviewer: not_assigned
  approval_status: draft
""".lstrip(),
            )
            output = repo / "out/reference-canon.json"

            result = run_script(
                "regulated_reference_canon.py",
                "--root",
                str(repo),
                "--licensed-root",
                str(licensed_root),
                "--strict",
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "requires_evidence")
            self.assertEqual(report["summary"]["licensed_reference_gaps"], 1)
            self.assertEqual(report["bibles"][0]["licensed_artifact_status"], "verified_license_review_required")
            self.assertFalse(report["bibles"][0]["license_review_verified"])
            self.assertEqual(report["gaps"][0]["id"], "GAP-LICENSE-REVIEW-ISPE-GAMP5-2E-2022")

    def test_reference_canon_allows_public_surrogate_for_missing_licensed_reference(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp) / "repo"
            licensed_root = Path(tmp) / "licensed"
            write(
                repo / "docs/regulated/reference-basis/external-reference-register.yaml",
                """
schema_version: "0.1.0"
nomos_bible_policy:
  all_registered_references_are_canonical: true
references:
  - id: ISO-13485-2016
    title: ISO 13485
    publisher: ISO
    url: https://www.iso.org/standard/59752.html
    content_access_policy: licensed_content_required
    evidence_status: summary_reference_only_until_licensed_clause_mapping
""".lstrip(),
            )
            write(
                repo / "docs/regulated/reference-basis/public-surrogate-annexes/ISO-13485-2016.yaml",
                """
schema_version: "0.1.0"
record_type: public_surrogate_annex
reference_id: ISO-13485-2016
surrogate_for: ISO-13485-2016
status: temporary_surrogate_until_official_document_acquired
claim_boundary: "Public surrogate only; no ISO clause-level mapping or certification claim."
sources:
  - id: FDA-QMSR-FAQ
    url: https://www.fda.gov/medical-devices/quality-management-system-regulation-qmsr/quality-management-system-regulation-frequently-asked-questions
    authority: official_public_regulator_source
blocked_claims:
  - iso_clause_level_mapping
  - iso_certification_claim
  - licensed_full_text_processing_claim
""".lstrip(),
            )
            output = repo / "out/reference-canon.json"

            result = run_script(
                "regulated_reference_canon.py",
                "--root",
                str(repo),
                "--licensed-root",
                str(licensed_root),
                "--allow-public-surrogates",
                "--strict",
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "surrogate_ready_for_processing")
            self.assertEqual(report["summary"]["licensed_reference_gaps"], 1)
            self.assertEqual(report["summary"]["surrogate_mitigations"], 1)
            self.assertEqual(report["summary"]["unmitigated_licensed_reference_gaps"], 0)
            iso = report["bibles"][0]
            self.assertEqual(iso["public_surrogate_status"], "available")
            self.assertTrue(iso["surrogate_processing_allowed"])
            self.assertFalse(iso["full_text_fetch_allowed"])
            self.assertEqual(report["gaps"][0]["status"], "temporarily_mitigated")


if __name__ == "__main__":
    unittest.main()

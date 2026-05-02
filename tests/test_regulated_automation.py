from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def write(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


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


if __name__ == "__main__":
    unittest.main()

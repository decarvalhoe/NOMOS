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


REQUIRED_ALCOA_ENVELOPE_PATHS = (
    ("attributable", "actor"),
    ("attributable", "tool"),
    ("attributable", "tool_version"),
    ("attributable", "command"),
    ("contemporaneous", "timestamp_utc"),
    ("original_or_true_copy", "source_commit"),
    ("original_or_true_copy", "source_hash"),
    ("original_or_true_copy", "artifact_hash"),
    ("complete", "derivation"),
    ("complete", "exclusions"),
    ("enduring", "retention_hint"),
)


def nested_value(data: dict[str, object], path: tuple[str, ...]) -> object:
    value: object = data
    for part in path:
        if not isinstance(value, dict):
            return None
        value = value.get(part)
    return value


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
        tmp_path / "docs/regulated/validation-pack/approval-workflow.yaml",
        """
schema_version: "0.1.0"
record_type: validation_approval_workflow
document_id: APPR-NOMOS-001
version: 0.1.0
status: pending_approval
claim_boundary: "Approval workflow only; no validation approval is claimed."
required_roles:
  - id: quality_owner
  - id: product_owner
  - id: technical_owner
evidence_channels:
  protected_pr_review:
    enabled: true
    requires_codeowners: true
    minimum_approvals: 2
  signed_commits_or_tags:
    required_for_effective_release: true
  attestation_artifact:
    path: docs/regulated/validation-pack/validation-approval-record.yaml
immutability_controls:
  codeowners_required: true
  evidence_pack_hashing_required: true
  immutable_release_tag_required_for_effective_status: true
approval_records:
  - document_id: VMP-NOMOS-001
    record_path: docs/regulated/validation-pack/validation-master-plan.md
    approval_status: pending_approval
    required_roles: [quality_owner, product_owner, technical_owner]
    evidence_refs: []
    overclaim_guard: true
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
            for record in report["records"]:
                envelope = record.get("alcoa_envelope")
                self.assertIsInstance(envelope, dict, record)
                for path in REQUIRED_ALCOA_ENVELOPE_PATHS:
                    self.assertNotIn(nested_value(envelope, path), (None, "", []), (record["path"], path))
                self.assertEqual(nested_value(envelope, ("original_or_true_copy", "source_hash")), record["sha256"])
                self.assertEqual(nested_value(envelope, ("original_or_true_copy", "artifact_hash")), record["sha256"])

    def test_evidence_pack_blocks_domain_evidence_without_alcoa_envelope(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = make_minimal_regulated_repo(Path(tmp))
            write(
                repo / "docs/regulated/domain-evidence/gxp-csv-pack.yaml",
                """
schema_version: "0.1.0"
record_type: domain_evidence_artifact
domain_profile: gxp-csv
claim_boundary: "Evidence planning artifact only; no GxP compliance claim."
artifact:
  id: EV-DOMAIN-GXP-CSV-001
  title: GxP CSV sample evidence pack
""".lstrip(),
            )
            output = repo / "out/evidence-pack.json"

            result = run_script(
                "regulated_evidence_pack.py",
                "--root",
                str(repo),
                "--output",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "failed")
            self.assertTrue(
                any(
                    finding["code"] == "MISSING_ALCOA_ENVELOPE_FIELD"
                    and "docs/regulated/domain-evidence/gxp-csv-pack.yaml" in finding["path"]
                    for finding in report["findings"]
                ),
                report.get("findings", []),
            )
    def test_validation_planner_ranks_controls_by_csa_risk(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            domain_profile = repo / "specs/examples/nomos-domain-profile.gxp.valid.yaml"
            control_matrix = repo / "docs/regulated/control-matrix/nomos-control-matrix.yaml"
            crosswalk = repo / "docs/regulated/control-matrix/gxp-csv-control-crosswalk.yaml"
            write(
                domain_profile,
                """
schema_version: "0.1.0"
domain_profile: gxp-csv
name: "GxP/CSV validation readiness"
intended_use:
  statement: "Plan CSA-style validation evidence for customer-owned GxP/CSV assessment."
risk_class:
  level: high
  rationale: "Unsupported regulated claims are high impact."
references:
  - id: FDA-CSA-2025
    status: required
claim_ladder:
  current_level: registered
""".lstrip(),
            )
            write(
                control_matrix,
                """
schema_version: "0.1.0"
controls:
  - control_id: CTL-VAL-HIGH
    title: High Risk Function
    control_family: validation
    intended_use: "A quality-impacting function."
    risk_level: high
    evidence_type: manual_review
    verification_ref: manual review alone is insufficient
    evidence_artifact: review-note
  - control_id: CTL-VAL-LOW
    title: Low Risk Function
    control_family: validation
    intended_use: "A low-impact support function."
    risk_level: low
    evidence_type: manual_review
    verification_ref: documented review
    evidence_artifact: review-note
""".lstrip(),
            )
            write(
                crosswalk,
                """
schema_version: "0.1.0"
record_type: gxp_csv_control_crosswalk
domain_profile: gxp-csv
references:
  - reference_id: FDA-CSA-2025
    disposition: mapped
    controls:
      - CTL-VAL-HIGH
      - CTL-VAL-LOW
    rationale: "CSA planning controls."
""".lstrip(),
            )
            output = repo / "out/risk-based-validation-plan.json"

            result = run_script(
                "regulated_validation_planner.py",
                "--root",
                str(repo),
                "--domain-profile",
                str(domain_profile),
                "--control-matrix",
                str(control_matrix),
                "--crosswalk",
                str(crosswalk),
                "--output",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            plan = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(plan["status"], "generated")
            self.assertEqual(
                plan["claim_boundary"],
                "CSA-style validation planning only; no validation, compliance, or certification claim.",
            )
            controls = {control["control_id"]: control for control in plan["controls"]}
            self.assertEqual(controls["CTL-VAL-HIGH"]["criticality"], "high")
            self.assertEqual(
                controls["CTL-VAL-HIGH"]["required_verification_type"],
                "scripted_or_challenge_evidence",
            )
            self.assertIn("scripted_test", controls["CTL-VAL-HIGH"]["required_evidence"])
            self.assertIn("challenge_case", controls["CTL-VAL-HIGH"]["required_evidence"])
            self.assertFalse(controls["CTL-VAL-HIGH"]["lighter_evidence_allowed"])
            self.assertEqual(controls["CTL-VAL-LOW"]["criticality"], "low")
            self.assertEqual(controls["CTL-VAL-LOW"]["required_verification_type"], "documented_rationale")
            self.assertIn("documented_rationale", controls["CTL-VAL-LOW"]["required_evidence"])
            self.assertTrue(controls["CTL-VAL-LOW"]["lighter_evidence_allowed"])
            self.assertTrue(plan["summary"]["high_risk_controls_require_scripted_or_challenge_evidence"])
            self.assertTrue(plan["summary"]["low_risk_controls_allow_lighter_evidence_with_rationale"])

    def test_iq_oq_pq_generator_distinguishes_deployments_without_rbok_coupling(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            domain_profile = repo / "specs/examples/nomos-domain-profile.gxp.valid.yaml"
            write(
                domain_profile,
                """
schema_version: "0.1.0"
domain_profile: gxp-csv
name: "GxP/CSV validation readiness"
intended_use:
  statement: "Prepare customer-owned validation package templates."
  allowed_uses:
    - "Prepare IQ/OQ/PQ evidence prompts for human review."
  not_authorized:
    - "Claim regulated validation."
risk_class:
  level: high
  rationale: "Unsupported regulated claims are high impact."
claim_ladder:
  current_level: registered
""".lstrip(),
            )
            output = repo / "out/iq-oq-pq-template-pack.json"

            result = run_script(
                "regulated_iq_oq_pq_generator.py",
                "--root",
                str(repo),
                "--domain-profile",
                str(domain_profile),
                "--output",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            pack = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(pack["status"], "generated")
            self.assertEqual(
                pack["claim_boundary"],
                "IQ/OQ/PQ template preparation only; no validation, compliance, or certification claim.",
            )
            deployments = {template["deployment_model"]: template for template in pack["templates"]}
            self.assertEqual(
                set(deployments),
                {"cli-only", "github-workflow", "output-repo", "control-plane", "downstream-rag"},
            )
            for template in deployments.values():
                self.assertIn("iq", template)
                self.assertIn("oq", template)
                self.assertIn("pq", template)
                self.assertIn("intended_use", template)
            self.assertIn("local executable baseline", deployments["cli-only"]["iq"]["focus"])
            self.assertIn("latest run per workflow", " ".join(deployments["github-workflow"]["oq"]["checks"]))
            self.assertIn("published artifact", deployments["output-repo"]["pq"]["focus"])
            self.assertIn("portfolio supervision", deployments["control-plane"]["oq"]["focus"])
            self.assertIn("citation", deployments["downstream-rag"]["pq"]["focus"])
            self.assertNotIn("RBOK", json.dumps(pack, sort_keys=True))

    def test_rag_answer_evidence_blocks_acceptable_answer_without_citation_or_refusal(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            fixtures = repo / "docs/regulated/ai-rag-governance/rag-answer-fixtures.yaml"
            write(
                fixtures,
                """
schema_version: "0.1.0"
answers:
  - answer_id: ANS-MISSING-CITATION
    prompt_id: PROMPT-CITATION-001
    fixture_type: citation
    model:
      provider: example-provider
      name: example-model
      version: "2026-05-14"
    retrieved_chunks:
      - chunk_id: CHUNK-001
        source_id: SRC-001
        source_hash: 0123456789abcdef
        span: "1-3"
    source_spans: []
    citation_status: missing
    refusal_status: not_refused
    confidence: 0.82
    policy_outcome: acceptable
""".lstrip(),
            )
            output = repo / "out/rag-answer-evidence.json"

            result = run_script(
                "regulated_rag_answer_evidence.py",
                "--root",
                str(repo),
                "--fixtures",
                str(fixtures),
                "--output",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "failed")
            self.assertTrue(
                any(
                    finding["code"] == "ACCEPTABLE_WITHOUT_CITATION_OR_REFUSAL"
                    for finding in report["findings"]
                ),
                report.get("findings", []),
            )

    def test_legal_domain_profile_links_citation_custody_and_privilege_fixture(self) -> None:
        import yaml

        profile_path = ROOT / "specs/examples/nomos-domain-profile.legal.valid.yaml"
        profile = yaml.safe_load(profile_path.read_text(encoding="utf-8"))
        self.assertEqual(profile["domain_profile"], "legal-ediscovery")

        artifacts = {artifact["id"]: artifact for artifact in profile["required_artifacts"]}
        required_artifact_ids = {
            "source-authority-register",
            "citation-source-span-ledger",
            "chain-of-custody-ledger",
            "retention-schedule-evidence",
            "privilege-marker-register",
            "policy-contract-trace-matrix",
            "legal-ediscovery-fixture",
        }
        self.assertLessEqual(required_artifact_ids, set(artifacts))

        fixture_path = ROOT / artifacts["legal-ediscovery-fixture"]["path"]
        fixture = yaml.safe_load(fixture_path.read_text(encoding="utf-8"))
        self.assertEqual(fixture["domain_profile"], "legal-ediscovery")
        self.assertIn("no legal advice", fixture["claim_boundary"].lower())
        self.assertIn("no legal sufficiency", fixture["claim_boundary"].lower())

        source = fixture["source_documents"][0]
        self.assertIn("source_hash", source)
        self.assertIn("custody", source)
        self.assertIn("retention", source)
        self.assertIn("privilege", source)
        self.assertEqual(source["privilege"]["marker"], "potentially_privileged")

        span = fixture["source_spans"][0]
        citation = fixture["citations"][0]
        self.assertEqual(span["source_id"], source["source_id"])
        self.assertEqual(citation["span_id"], span["span_id"])
        self.assertEqual(citation["integrity_status"], "source_span_hash_bound")

        trace = fixture["policy_contract_trace"][0]
        self.assertEqual(trace["policy_span_id"], span["span_id"])
        self.assertEqual(trace["trace_status"], "needs_counsel_review")

    def test_six_sigma_capa_profile_groups_findings_with_traceable_context(self) -> None:
        import yaml

        profile_path = ROOT / "specs/examples/nomos-domain-profile.six-sigma-capa.valid.yaml"
        self.assertTrue(profile_path.exists(), f"missing profile: {profile_path}")
        profile = yaml.safe_load(profile_path.read_text(encoding="utf-8"))
        self.assertEqual(profile["domain_profile"], "six-sigma-capa")

        artifacts = {artifact["id"]: artifact for artifact in profile["required_artifacts"]}
        required_artifact_ids = {
            "dmaic-model",
            "ctq-register",
            "defect-taxonomy",
            "deviation-ledger",
            "root-cause-analysis",
            "capa-action-register",
            "control-plan",
            "trend-analysis",
            "management-review-summary",
            "capa-analytics-fixture",
        }
        self.assertLessEqual(required_artifact_ids, set(artifacts))

        fixture_path = ROOT / artifacts["capa-analytics-fixture"]["path"]
        fixture = yaml.safe_load(fixture_path.read_text(encoding="utf-8"))
        self.assertEqual(fixture["domain_profile"], "six-sigma-capa")
        self.assertIn("no certified six sigma", fixture["claim_boundary"].lower())

        required_categories = {
            "ctq",
            "defect",
            "deviation",
            "root_cause",
            "capa_action",
            "control_plan",
            "trend",
            "management_review",
        }
        groups = {group["category"] for group in fixture["finding_groups"]}
        self.assertLessEqual(required_categories, groups)

        for group in fixture["finding_groups"]:
            source_context = group["source_context"]
            evidence_context = group["evidence_context"]
            self.assertTrue(source_context["source_id"])
            self.assertTrue(source_context["source_hash"])
            self.assertTrue(source_context["span"])
            self.assertTrue(evidence_context["artifact_ref"])
            self.assertTrue(evidence_context["evidence_hash"])

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

    def test_regulated_docs_gate_emits_domain_applicability_report(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = make_minimal_regulated_repo(Path(tmp))
            write(repo / "docs/canonical/source-manifest.yaml", "sources: []\n")
            profiles = repo / "specs/examples"
            for domain, status, extra in [
                ("ai-governance", "applicable", ""),
                ("finance-regtech", "not_applicable", ""),
                ("cyber-supplier-assurance", "blocked", ""),
                (
                    "legal-ediscovery",
                    "applicable",
                    """
waiver:
  id: WAIVER-LEGAL-001
  reason: Legal owner deferred this profile.
  approver: legal-owner
  expires_on: "2026-12-31"
""",
                ),
                ("gxp-csv", "applicable", "missing_artifact: true\n"),
            ]:
                artifact_path = "docs/canonical/source-manifest.yaml"
                if "missing_artifact" in extra:
                    artifact_path = "docs/missing/evidence.yaml"
                    extra = ""
                write(
                    profiles / f"nomos-domain-profile.{domain}.valid.yaml",
                    f"""
schema_version: "0.1.0"
domain_profile: {domain}
name: "{domain} profile"
summary: "Test profile"
intended_use:
  statement: "Test intended use"
  allowed_uses:
    - "Test planning"
  not_authorized:
    - "Unsupported claims"
references:
  - id: REF-{domain.upper().replace("-", "")}
    title: "Reference"
    authority_type: internal_policy
    access_policy: internal_only
    status: required
    purpose: "Test reference"
applicability:
  status: {status}
  applies_when:
    - "Profile applies"
  does_not_apply_when:
    - "Profile does not apply"
risk_class:
  level: high
  rationale: "Test risk"
claim_ladder:
  current_level: registered
  authorized_claims:
    - id: CLAIM-{domain.upper().replace("-", "")}
      level: registered
      kind: planning
      statement: "Planning claim"
      evidence:
        - "{artifact_path}"
  blocked_claims:
    - id: BLOCK-{domain.upper().replace("-", "")}
      kind: certification
      statement: "Certification claim"
      reason: "Unsupported"
required_artifacts:
  - id: source-manifest
    type: source_manifest
    path: {artifact_path}
    required: true
    minimum_claim_level: registered
  - id: future-evidence
    type: evidence_pack
    path: docs/missing/future-evidence.yaml
    required: true
    minimum_claim_level: mapped
validation_gates:
  - id: profile-vet
    command: "cue vet"
    required: true
    blocks_claim_levels:
      - mapped
{extra}
""".lstrip(),
                )

            output = repo / ".regulated-doc-gate/regulated-doc-gate-report.json"

            result = run_script(
                "regulated_docs_gate.py",
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            domain_report_path = output.parent / "domain-applicability-report.json"
            self.assertTrue(domain_report_path.exists())
            domain_report = json.loads(domain_report_path.read_text(encoding="utf-8"))
            self.assertEqual(domain_report["status_counts"]["applicable"], 1)
            self.assertEqual(domain_report["status_counts"]["not_applicable"], 1)
            self.assertEqual(domain_report["status_counts"]["blocked"], 1)
            self.assertEqual(domain_report["status_counts"]["waived"], 1)
            self.assertEqual(domain_report["status_counts"]["missing_evidence"], 1)
            self.assertTrue(
                any(finding["code"] == "DOMAIN_REQUIRED_ARTIFACT_MISSING" for finding in domain_report["findings"]),
                domain_report["findings"],
            )
            self.assertFalse(
                any("future-evidence.yaml" in finding["message"] for finding in domain_report["findings"]),
                domain_report["findings"],
            )

    def test_regulated_docs_gate_fails_domain_claim_above_evidence_level(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = make_minimal_regulated_repo(Path(tmp))
            write(repo / "docs/canonical/source-manifest.yaml", "sources: []\n")
            write(
                repo / "specs/examples/nomos-domain-profile.ai.valid.yaml",
                """
schema_version: "0.1.0"
domain_profile: ai-governance
name: "AI governance profile"
summary: "Test profile"
intended_use:
  statement: "Test intended use"
  allowed_uses:
    - "Test planning"
  not_authorized:
    - "Unsupported claims"
references:
  - id: REF-AI
    title: "Reference"
    authority_type: internal_policy
    access_policy: internal_only
    status: required
    purpose: "Test reference"
applicability:
  status: applicable
  applies_when:
    - "Profile applies"
  does_not_apply_when:
    - "Profile does not apply"
risk_class:
  level: high
  rationale: "Test risk"
claim_ladder:
  current_level: registered
  authorized_claims:
    - id: CLAIM-AI-001
      level: evidence_ready
      kind: readiness
      statement: "Evidence-ready claim"
      evidence:
        - "docs/canonical/source-manifest.yaml"
  blocked_claims:
    - id: BLOCK-AI-001
      kind: certification
      statement: "Certification claim"
      reason: "Unsupported"
required_artifacts:
  - id: source-manifest
    type: source_manifest
    path: docs/canonical/source-manifest.yaml
    required: true
    minimum_claim_level: registered
validation_gates:
  - id: profile-vet
    command: "cue vet"
    required: true
    blocks_claim_levels:
      - mapped
""".lstrip(),
            )
            output = repo / ".regulated-doc-gate/regulated-doc-gate-report.json"

            result = run_script(
                "regulated_docs_gate.py",
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            domain_report = json.loads((output.parent / "domain-applicability-report.json").read_text(encoding="utf-8"))
            self.assertTrue(
                any(finding["code"] == "DOMAIN_CLAIM_LEVEL_EXCEEDS_EVIDENCE" for finding in domain_report["findings"]),
                domain_report["findings"],
            )

    def test_regulated_docs_gate_blocks_incomplete_gxp_csv_crosswalk(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = make_minimal_regulated_repo(Path(tmp))
            write(
                repo / "docs/38-domain-opportunity-roadmap.md",
                "# Domain opportunity roadmap\n\nDOR-005 requires a GxP/CSV crosswalk.\n",
            )
            write(
                repo / "docs/regulated/control-matrix/gxp-csv-control-crosswalk.yaml",
                """
schema_version: "0.1.0"
record_type: gxp_csv_control_crosswalk
domain_profile: gxp-csv
claim_boundary: "Planning crosswalk only; no GxP compliance claim."
references:
  - reference_id: FDA-CSA-2025
    disposition: mapped
    controls:
      - CTL-VAL-002
    rationale: "Only one required reference is intentionally present."
""".lstrip(),
            )
            output = repo / "out/regulated-doc-gate.json"

            result = run_script(
                "regulated_docs_gate.py",
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "failed")
            self.assertTrue(
                any(
                    "GxP/CSV crosswalk missing required reference" in finding["message"]
                    for finding in report["findings"]
                )
            )

    def test_regulated_docs_gate_blocks_provider_change_without_impact_assessment(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = make_minimal_regulated_repo(Path(tmp))
            write(
                repo / "docs/regulated/ai-rag-governance/ai-provider-change-ledger.yaml",
                """
schema_version: "0.1.0"
record_type: ai_provider_change_ledger
claim_boundary: "AI provider change-control evidence only; no AI compliance claim."
records:
  - change_id: AI-PROVIDER-001
    provider: example-provider
    model: example-model
    region: eu
    data_use_policy: no_training_on_customer_data
    api_version: "2026-05-14"
    prompt_template_version: prompt-v1
    evaluation_baseline: eval-baseline-v1
    approval_state: approved_preserve_domain_claims
""".lstrip(),
            )
            output = repo / "out/regulated-doc-gate.json"

            result = run_script(
                "regulated_docs_gate.py",
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "failed")
            self.assertTrue(
                any(
                    "AI provider/model changes that preserve domain claims require impact assessment"
                    in finding["message"]
                    for finding in report["findings"]
                ),
                report["findings"],
            )

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
storage:
  licensed_root_env: NOMOS_LICENSED_REFERENCE_ROOT
  local_relative_path: ISPE-GAMP5-2E-2022/source.pdf
source_integrity:
  sha256: {digest}
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

    def test_reference_canon_reports_machine_readable_access_classes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp) / "repo"
            write(
                repo / "docs/regulated/reference-basis/external-reference-register.yaml",
                """
schema_version: "0.1.0"
nomos_bible_policy:
  all_registered_references_are_canonical: true
references:
  - id: PUBLIC-FDA-GUIDANCE
    title: Public FDA Guidance
    publisher: US FDA
    url: https://www.fda.gov/
    content_access_policy: official_public_reference
    evidence_status: requires_evidence
    reference_classification:
      source_class: public
      confidentiality: public
      full_text_redistribution: allowed
      processing_mode: official_snapshot
      retention_obligation: public_snapshot_retained_with_hash
  - id: LICENSED-ISO-STANDARD
    title: Licensed ISO Standard
    publisher: ISO
    url: https://www.iso.org/
    content_access_policy: licensed_content_required
    evidence_status: licensed_reference_required_before_clause_mapping
    reference_classification:
      source_class: licensed
      confidentiality: licensed_restricted
      full_text_redistribution: forbidden
      processing_mode: licensed_read_only_local_artifact
      retention_obligation: license_terms
  - id: PRIVATE-METHOD
    title: Private Internal Method
    publisher: Internal
    url: ""
    content_access_policy: private_reference_only
    evidence_status: private_reference_registered
    reference_classification:
      source_class: private
      confidentiality: private_restricted
      full_text_redistribution: forbidden
      processing_mode: private_read_only_local_artifact
      retention_obligation: owner_policy
  - id: CONFIDENTIAL-SOP
    title: Confidential SOP
    publisher: Partner
    url: ""
    content_access_policy: confidential_reference_only
    evidence_status: confidential_reference_registered
    reference_classification:
      source_class: confidential
      confidentiality: confidential_restricted
      full_text_redistribution: forbidden
      processing_mode: confidential_read_only_local_artifact
      retention_obligation: confidentiality_agreement
  - id: CUSTOMER-BIBLE
    title: Customer Owned Bible
    publisher: Customer
    url: ""
    content_access_policy: customer_owned_confidential
    evidence_status: customer_reference_registered
    reference_classification:
      source_class: customer_owned
      confidentiality: customer_confidential
      full_text_redistribution: forbidden
      processing_mode: customer_read_only_local_artifact
      retention_obligation: customer_contract
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
            self.assertEqual(report["summary"]["source_classes"]["public"], 1)
            self.assertEqual(report["summary"]["source_classes"]["licensed"], 1)
            self.assertEqual(report["summary"]["source_classes"]["private"], 1)
            self.assertEqual(report["summary"]["source_classes"]["confidential"], 1)
            self.assertEqual(report["summary"]["source_classes"]["customer_owned"], 1)
            public = next(item for item in report["bibles"] if item["id"] == "PUBLIC-FDA-GUIDANCE")
            customer = next(item for item in report["bibles"] if item["id"] == "CUSTOMER-BIBLE")
            self.assertTrue(public["full_text_redistribution_allowed"])
            self.assertFalse(customer["full_text_fetch_allowed"])
            self.assertFalse(customer["full_text_redistribution_allowed"])
            self.assertEqual(customer["access_policy"], "customer_owned_confidential")
            self.assertEqual(customer["retention_obligation"], "customer_contract")
            self.assertEqual(customer["nomos_processing_policy"], "read_only_local_artifact_required")

    def test_reference_canon_blocks_forbidden_full_text_redistribution(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp) / "repo"
            licensed_root = Path(tmp) / "licensed"
            payload = b"licensed reference content"
            digest = hashlib.sha256(payload).hexdigest().upper()
            write_bytes(licensed_root / "ISO-9001/source.pdf", payload)
            write(
                repo / "docs/regulated/reference-basis/external-reference-register.yaml",
                """
schema_version: "0.1.0"
nomos_bible_policy:
  all_registered_references_are_canonical: true
references:
  - id: ISO-9001
    title: ISO 9001
    publisher: ISO
    url: https://www.iso.org/
    content_access_policy: licensed_content_required
    evidence_status: local_artifact_registered_license_review_required_before_clause_mapping
    reference_classification:
      source_class: licensed
      confidentiality: licensed_restricted
      full_text_redistribution: forbidden
      processing_mode: licensed_read_only_local_artifact
      retention_obligation: license_terms
""".lstrip(),
            )
            write(
                repo / "docs/regulated/reference-basis/licensed-intakes/ISO-9001.yaml",
                f"""
schema_version: "0.1.0"
record_type: licensed_reference_intake
reference_id: ISO-9001
storage:
  licensed_root_env: NOMOS_LICENSED_REFERENCE_ROOT
  local_relative_path: ISO-9001/source.pdf
source_integrity:
  sha256: {digest}
licensed_use:
  processing_scope: read_only_local_artifact
  full_text_redistribution: allowed
  commit_full_text_allowed: true
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

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "failed")
            self.assertTrue(
                any(
                    finding.get("id") == "REFERENCE_FULL_TEXT_REDISTRIBUTION_FORBIDDEN"
                    for finding in report["findings"]
                )
            )

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

    def test_approval_gate_accepts_pending_controlled_workflow(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = make_minimal_regulated_repo(Path(tmp))
            write(
                repo / "docs/regulated/validation-pack/approval-workflow.yaml",
                """
schema_version: "0.1.0"
record_type: validation_approval_workflow
document_id: APPR-NOMOS-001
version: 0.1.0
status: pending_approval
claim_boundary: "Approval workflow only; no validation approval is claimed."
required_roles:
  - id: quality_owner
    meaning: "Quality owner verifies validation adequacy and regulated claim boundary."
  - id: product_owner
    meaning: "Product owner accepts intended use and product risk."
  - id: technical_owner
    meaning: "Technical owner accepts implementation and reproducibility evidence."
evidence_channels:
  protected_pr_review:
    enabled: true
    requires_codeowners: true
    minimum_approvals: 2
  signed_commits_or_tags:
    required_for_effective_release: true
  attestation_artifact:
    path: docs/regulated/validation-pack/validation-approval-record.yaml
immutability_controls:
  codeowners_required: true
  evidence_pack_hashing_required: true
  immutable_release_tag_required_for_effective_status: true
approval_records:
  - document_id: VMP-NOMOS-001
    record_path: docs/regulated/validation-pack/validation-master-plan.md
    approval_status: pending_approval
    required_roles: [quality_owner, product_owner, technical_owner]
    evidence_refs: []
    overclaim_guard: true
""".lstrip(),
            )
            output = repo / "out/approval-gate.json"

            result = run_script(
                "regulated_approval_gate.py",
                "--root",
                str(repo),
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "pending_approval")
            self.assertEqual(report["summary"]["approval_records"], 1)
            self.assertEqual(report["summary"]["approved_records"], 0)
            self.assertEqual(
                report["required_roles"],
                ["product_owner", "quality_owner", "technical_owner"],
            )

    def test_approval_gate_blocks_approved_status_without_evidence(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = make_minimal_regulated_repo(Path(tmp))
            write(
                repo / "docs/regulated/validation-pack/approval-workflow.yaml",
                """
schema_version: "0.1.0"
record_type: validation_approval_workflow
document_id: APPR-NOMOS-001
version: 0.1.0
status: pending_approval
claim_boundary: "Approval workflow only; no validation approval is claimed."
required_roles:
  - id: quality_owner
  - id: product_owner
  - id: technical_owner
evidence_channels:
  protected_pr_review:
    enabled: true
    requires_codeowners: true
    minimum_approvals: 2
  signed_commits_or_tags:
    required_for_effective_release: true
  attestation_artifact:
    path: docs/regulated/validation-pack/validation-approval-record.yaml
immutability_controls:
  codeowners_required: true
  evidence_pack_hashing_required: true
  immutable_release_tag_required_for_effective_status: true
approval_records:
  - document_id: VMP-NOMOS-001
    record_path: docs/regulated/validation-pack/validation-master-plan.md
    approval_status: approved
    required_roles: [quality_owner, product_owner, technical_owner]
    evidence_refs: []
""".lstrip(),
            )
            output = repo / "out/approval-gate.json"

            result = run_script(
                "regulated_approval_gate.py",
                "--root",
                str(repo),
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "failed")
            self.assertTrue(
                any(finding["code"] == "APPROVED_WITHOUT_EVIDENCE" for finding in report["findings"]),
                report["findings"],
            )

    def test_regulated_docs_gate_runs_approval_gate(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = make_minimal_regulated_repo(Path(tmp))
            write(
                repo / "docs/regulated/validation-pack/approval-workflow.yaml",
                """
schema_version: "0.1.0"
record_type: validation_approval_workflow
document_id: APPR-NOMOS-001
version: 0.1.0
status: pending_approval
required_roles:
  - id: quality_owner
  - id: product_owner
  - id: technical_owner
evidence_channels:
  protected_pr_review:
    enabled: true
    requires_codeowners: true
    minimum_approvals: 2
  signed_commits_or_tags:
    required_for_effective_release: true
  attestation_artifact:
    path: docs/regulated/validation-pack/validation-approval-record.yaml
immutability_controls:
  codeowners_required: true
  evidence_pack_hashing_required: true
  immutable_release_tag_required_for_effective_status: true
approval_records:
  - document_id: VMP-NOMOS-001
    record_path: docs/regulated/validation-pack/validation-master-plan.md
    approval_status: approved
    required_roles: [quality_owner, product_owner, technical_owner]
    evidence_refs: []
""".lstrip(),
            )
            output = repo / "out/regulated-doc-gate.json"

            result = run_script(
                "regulated_docs_gate.py",
                "--report",
                str(output),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertTrue(
                any(finding["message"].find("APPROVED_WITHOUT_EVIDENCE") >= 0 for finding in report["findings"]),
                report["findings"],
            )


if __name__ == "__main__":
    unittest.main()

from __future__ import annotations

import json
import hashlib
import shutil
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


_NOMOS_BUILD: tempfile.TemporaryDirectory | None = None
_NOMOS_BIN: Path | None = None


def nomos_binary() -> Path:
    """Build the Go engine once: the RAG evidence sidecar consumes its verdict
    (#624), so the tests that run the sidecar need a real `nomos`."""
    global _NOMOS_BUILD, _NOMOS_BIN
    if _NOMOS_BIN is None:
        if shutil.which("go") is None:
            raise unittest.SkipTest("go not on PATH — the RAG evidence sidecar consumes the Go gate verdict")
        _NOMOS_BUILD = tempfile.TemporaryDirectory()
        target = Path(_NOMOS_BUILD.name) / "nomos"
        build = subprocess.run(
            ["go", "build", "-o", str(target), "."],
            cwd=ROOT / "cli",
            text=True,
            capture_output=True,
            check=False,
        )
        if build.returncode != 0:
            raise AssertionError(f"engine build failed: {build.stderr}{build.stdout}")
        _NOMOS_BIN = target
    return _NOMOS_BIN


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

    def test_evidence_pack_includes_signed_claim_boundary_predicates(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = make_minimal_regulated_repo(Path(tmp))
            write(
                repo / "docs/regulated/claim-boundary/ckm-refused-claims.json",
                json.dumps(
                    {
                        "_type": "https://in-toto.io/Statement/v1",
                        "subject": [
                            {
                                "name": "rag-answer-evidence.json",
                                "digest": {"sha256": "0123456789abcdef"},
                            }
                        ],
                        "predicateType": "https://nomos.dev/claim-boundary/v1",
                        "predicate": {
                            "projectId": "ckm-test",
                            "generatedAt": "2026-06-09T00:00:00Z",
                            "refusedClaims": [
                                {
                                    "claimId": "claim.no-trace-for-y",
                                    "statement": "No trace exists for Y, so Nomos refuses the claim.",
                                    "reason": "No source-backed atom supports Y.",
                                    "requiredEvidence": ["source_span", "atom_id"],
                                    "decision": "refused",
                                }
                            ],
                            "verifier": "nomos",
                            "signatureMode": "dsse-cosign",
                            "signature": {
                                "keyId": "fixture-key",
                                "signature": "MEUCIQDfixture-signature",
                                "signedAt": "2026-06-09T00:00:00Z",
                                "logUri": "rekor://fixture-entry",
                            },
                            "claimBoundary": "Refusal predicate only; no correctness claim.",
                        },
                    },
                    indent=2,
                )
                + "\n",
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

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertTrue(
                any(
                    record["category"] == "regulated_claim_boundary_attestation"
                    and record["path"] == "docs/regulated/claim-boundary/ckm-refused-claims.json"
                    for record in report["records"]
                ),
                report["records"],
            )
            self.assertEqual(report["summary"]["categories"]["regulated_claim_boundary_attestation"], 1)
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
                "--nomos-bin",
                str(nomos_binary()),
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

    def test_rag_answer_evidence_emits_ckm08_metrics_and_certified_trust_tier(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            fixtures = repo / "docs/regulated/ai-rag-governance/rag-answer-fixtures.yaml"
            source_hash = "0123456789abcdef"
            write(
                fixtures,
                f"""
schema_version: "0.1.0"
answers:
  - answer_id: ANS-CERTIFIED
    prompt_id: PROMPT-CERTIFIED-001
    fixture_type: citation
    answer: "Only cited governed facts are used."
    structured_facts:
      - unit_id: RULE-001
        source: read_model
    citations:
      - source_id: SRC-001
        locator: "section 1"
        chunk_id: CHUNK-001
    uncertainties: []
    requires_human_decision: false
    model:
      provider: example-provider
      name: example-model
      version: "2026-05-14"
    retrieved_chunks:
      - chunk_id: CHUNK-001
        source_id: SRC-001
        source_hash: {source_hash}
        span: "1-3"
        text: "Only cited governed facts are used in a governed answer."
    source_spans:
      - source_id: SRC-001
        source_hash: {source_hash}
        span: "1-3"
        chunk_id: CHUNK-001
        text: "Only cited governed facts are used in a governed answer."
    citation_status: source_backed
    refusal_status: not_refused
    confidence: 0.96
    faithfulness_score: 0.98
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
                "--nomos-bin",
                str(nomos_binary()),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "generated")
            self.assertEqual(report["summary"]["trust_tier"], "certified")
            metrics = report["summary"]["metrics"]
            self.assertEqual(metrics["alce"]["citation_recall"], 1.0)
            self.assertEqual(metrics["alce"]["citation_precision"], 1.0)
            self.assertGreaterEqual(metrics["trust_score"], 0.95)
            self.assertGreaterEqual(metrics["deepeval"]["faithfulness"], 0.95)
            self.assertEqual(report["answers"][0]["trust_tier"], "certified")
            self.assertTrue(report["answers"][0]["response_contract"]["fields"]["structured_facts"])

    def test_rag_answer_evidence_blocks_unfaithful_citation_coverage(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            fixtures = repo / "docs/regulated/ai-rag-governance/rag-answer-fixtures.yaml"
            write(
                fixtures,
                """
schema_version: "0.1.0"
answers:
  - answer_id: ANS-MISMATCHED-CITATION
    prompt_id: PROMPT-CITATION-001
    fixture_type: citation
    answer: "This answer cites the wrong chunk."
    structured_facts: []
    citations:
      - source_id: SRC-001
        locator: "section 1"
        chunk_id: CHUNK-OTHER
    uncertainties: []
    requires_human_decision: false
    model:
      provider: example-provider
      name: example-model
      version: "2026-05-14"
    retrieved_chunks:
      - chunk_id: CHUNK-001
        source_id: SRC-001
        source_hash: 0123456789abcdef
        span: "1-3"
    source_spans:
      - source_id: SRC-001
        source_hash: 0123456789abcdef
        span: "1-3"
        chunk_id: CHUNK-OTHER
    citation_status: source_backed
    refusal_status: not_refused
    confidence: 0.96
    faithfulness_score: 0.98
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
                "--nomos-bin",
                str(nomos_binary()),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "failed")
            self.assertEqual(report["summary"]["trust_tier"], "unverified")
            self.assertTrue(
                any(finding["code"] == "ALCE_CITATION_RECALL_BELOW_GATE" for finding in report["findings"]),
                report.get("findings", []),
            )

    def test_rag_answer_evidence_blocks_high_declared_score_when_spans_have_no_text(self) -> None:
        # CKM-H6-FU (#538) ADVERSARIAL PROOF: a fabricated answer that declares
        # faithfulness_score 0.99 but whose spans carry NO `text` must make the
        # gate EXIT NON-ZERO. Before this fix the same answer passed at 0.99 via a
        # structural-citation fallback (~1.0). The structural citations here are
        # perfectly well-formed (recall/precision 1.0) — the ONLY thing missing is
        # verifiable span text, which is exactly the bypass being closed.
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            fixtures = repo / "docs/regulated/ai-rag-governance/rag-answer-fixtures.yaml"
            source_hash = "0123456789abcdef"
            write(
                fixtures,
                f"""
schema_version: "0.1.0"
answers:
  - answer_id: ANS-NO-SPAN-TEXT-HALLUCINATION
    prompt_id: PROMPT-CITATION-001
    fixture_type: citation
    answer: "Quarterly revenue grew twelve percent on new enterprise contracts in Asia."
    structured_facts:
      - unit_id: RULE-001
        source: read_model
    citations:
      - source_id: SRC-001
        locator: "section 1"
        chunk_id: CHUNK-001
    uncertainties: []
    requires_human_decision: false
    model:
      provider: example-provider
      name: example-model
      version: "2026-05-14"
    retrieved_chunks:
      - chunk_id: CHUNK-001
        source_id: SRC-001
        source_hash: {source_hash}
        span: "1-3"
    source_spans:
      - source_id: SRC-001
        source_hash: {source_hash}
        span: "1-3"
        chunk_id: CHUNK-001
    citation_status: source_backed
    refusal_status: not_refused
    confidence: 0.96
    faithfulness_score: 0.99
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
                "--nomos-bin",
                str(nomos_binary()),
                cwd=repo,
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "failed")
            record = report["answers"][0]
            # Structural citation coverage is perfect — proving the block is due to
            # missing verifiable span text, not a citation-coverage problem.
            self.assertEqual(record["metrics"]["alce"]["citation_recall"], 1.0)
            self.assertEqual(record["metrics"]["alce"]["citation_precision"], 1.0)
            # Faithfulness is the no-span-text floor of 0, NOT the declared 0.99.
            self.assertEqual(record["metrics"]["deepeval"]["faithfulness"], 0.0)
            self.assertEqual(record["metrics"]["groundedness"]["method"], "no_span_text")
            self.assertFalse(record["metrics"]["groundedness"]["self_declared_trusted"])
            self.assertTrue(
                any(
                    finding["code"] == "DEEPEVAL_FAITHFULNESS_BELOW_GATE"
                    for finding in report["findings"]
                ),
                report.get("findings", []),
            )
            # The lexical-proxy limitation is documented in the gate output.
            self.assertIn("negation-blind", report["groundedness_method"]["limitation"].lower())

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

    def test_verifiable_evidence_profile_selects_mechanisms_and_bundle_fields(self) -> None:
        import yaml

        profile_path = ROOT / "specs/examples/nomos-domain-profile.verifiable-evidence.valid.yaml"
        self.assertTrue(profile_path.exists(), f"missing profile: {profile_path}")
        profile = yaml.safe_load(profile_path.read_text(encoding="utf-8"))
        self.assertEqual(profile["domain_profile"], "verifiable-evidence")

        artifacts = {artifact["id"]: artifact for artifact in profile["required_artifacts"]}
        required_artifact_ids = {
            "signed-evidence-bundle-format",
            "mechanism-decision-record",
            "w3c-vc-evaluation",
            "c2pa-evaluation",
            "rfc9162-transparency-log-evaluation",
        }
        self.assertLessEqual(required_artifact_ids, set(artifacts))

        decision_path = ROOT / artifacts["mechanism-decision-record"]["path"]
        decision = yaml.safe_load(decision_path.read_text(encoding="utf-8"))
        self.assertEqual(decision["domain_profile"], "verifiable-evidence")
        self.assertIn("does not prove semantic correctness", decision["claim_boundary"].lower())

        bundle_fields = set(decision["signed_evidence_bundle"]["required_fields"])
        self.assertLessEqual(
            {
                "bundle_id",
                "bundle_version",
                "issuer",
                "artifact_hashes",
                "alcoa_envelope_hash",
                "source_commit",
                "signature_algorithm",
                "signature_value_or_external_ref",
                "verification_instructions",
                "transparency_entry_ref",
            },
            bundle_fields,
        )

        selected = {item["id"] for item in decision["mechanism_decision"]["selected_first"]}
        self.assertLessEqual({"hash-manifest", "detached-signature"}, selected)
        option_statuses = {
            item["id"]: item["status"]
            for item in decision["mechanism_decision"]["evaluated_options"]
        }
        self.assertEqual(option_statuses["w3c-vc-wrapper"], "optional")
        self.assertEqual(option_statuses["c2pa-content-provenance"], "optional")
        self.assertEqual(option_statuses["rfc9162-transparency-log"], "deferred")

    def test_cyber_supplier_profile_distinguishes_supplier_control_statuses(self) -> None:
        import yaml

        profile_path = ROOT / "specs/examples/nomos-domain-profile.cyber-supplier-assurance.valid.yaml"
        self.assertTrue(profile_path.exists(), f"missing profile: {profile_path}")
        profile = yaml.safe_load(profile_path.read_text(encoding="utf-8"))
        self.assertEqual(profile["domain_profile"], "cyber-supplier-assurance")

        references = {reference["id"] for reference in profile["references"]}
        self.assertLessEqual({"NIST-SP-800-218", "NIST-CSF-2"}, references)

        artifacts = {artifact["id"]: artifact for artifact in profile["required_artifacts"]}
        required_artifact_ids = {
            "supplier-assurance-pack",
            "sbom-inventory",
            "vulnerability-register",
            "incident-response-record",
            "branch-protection-evidence",
            "release-provenance-attestation",
        }
        self.assertLessEqual(required_artifact_ids, set(artifacts))

        pack_path = ROOT / artifacts["supplier-assurance-pack"]["path"]
        pack = yaml.safe_load(pack_path.read_text(encoding="utf-8"))
        self.assertEqual(pack["domain_profile"], "cyber-supplier-assurance")
        self.assertIn("no security certification", pack["claim_boundary"].lower())

        statuses = {control["status"] for control in pack["supplier_controls"]}
        self.assertLessEqual({"implemented", "manual", "blocked", "customer_owned"}, statuses)

        mapped_controls = {
            control["control_id"]: control
            for control in pack["supplier_controls"]
        }
        self.assertEqual(mapped_controls["SSDF-PO.3.2"]["evidence_type"], "sbom")
        self.assertEqual(mapped_controls["SSDF-RV.1.2"]["evidence_type"], "vulnerability_register")
        self.assertEqual(mapped_controls["CSF-RS.MA-01"]["status"], "manual")
        self.assertEqual(mapped_controls["CSF-GV.SC-07"]["status"], "customer_owned")

    def test_high_assurance_profile_traces_requirement_verification_waiver(self) -> None:
        import yaml

        profile_path = ROOT / "specs/examples/nomos-domain-profile.high-assurance.valid.yaml"
        self.assertTrue(profile_path.exists(), f"missing profile: {profile_path}")
        profile = yaml.safe_load(profile_path.read_text(encoding="utf-8"))
        self.assertEqual(profile["domain_profile"], "high-assurance-engineering")

        references = {reference["id"] for reference in profile["references"]}
        self.assertLessEqual({"NASA-NPR-7150-2D", "NASA-SWEHB"}, references)

        artifacts = {artifact["id"]: artifact for artifact in profile["required_artifacts"]}
        required_artifact_ids = {
            "lifecycle-evidence-map",
            "requirement-verification-trace",
            "independent-review-record",
            "waiver-register",
            "release-decision-record",
        }
        self.assertLessEqual(required_artifact_ids, set(artifacts))

        fixture_path = ROOT / artifacts["requirement-verification-trace"]["path"]
        fixture = yaml.safe_load(fixture_path.read_text(encoding="utf-8"))
        self.assertEqual(fixture["domain_profile"], "high-assurance-engineering")
        self.assertIn("no aerospace qualification", fixture["claim_boundary"].lower())

        requirements = {
            requirement["requirement_id"]: requirement
            for requirement in fixture["requirements"]
        }
        verifications = {
            verification["verification_id"]: verification
            for verification in fixture["verifications"]
        }
        waivers = {waiver["waiver_id"]: waiver for waiver in fixture["waivers"]}
        reviews = {
            review["review_id"]: review
            for review in fixture["independent_reviews"]
        }
        decisions = {
            decision["decision_id"]: decision
            for decision in fixture["release_decisions"]
        }

        requirement = requirements["HA-REQ-001"]
        self.assertIn("HA-VER-001", requirement["verification_ids"])
        self.assertEqual(requirement["waiver_id"], "HA-WVR-001")
        self.assertEqual(requirement["release_decision_id"], "HA-REL-001")

        verification = verifications["HA-VER-001"]
        self.assertEqual(verification["requirement_id"], "HA-REQ-001")
        self.assertEqual(verification["independent_review_id"], "HA-IR-001")

        waiver = waivers["HA-WVR-001"]
        self.assertEqual(waiver["requirement_id"], "HA-REQ-001")
        self.assertEqual(waiver["verification_id"], "HA-VER-001")
        self.assertEqual(waiver["status"], "approved_with_expiry")

        self.assertEqual(reviews["HA-IR-001"]["reviewed_artifacts"], ["HA-VER-001", "HA-WVR-001"])
        self.assertEqual(decisions["HA-REL-001"]["status"], "conditional_release")
        self.assertIn("HA-WVR-001", decisions["HA-REL-001"]["accepted_waivers"])

    def test_alm_qms_adapter_decision_records_first_adapter_and_risks(self) -> None:
        import yaml

        adr_path = ROOT / "docs/regulated/decisions/ADR-alm-qms-export-adapter-selection.md"
        self.assertTrue(adr_path.exists(), f"missing ADR: {adr_path}")
        adr_text = adr_path.read_text(encoding="utf-8")
        self.assertIn("ADR-ALM-QMS-020", adr_text)
        self.assertIn("First adapter: GitHub issues evidence export", adr_text)
        self.assertIn("Blocked adapters", adr_text)
        self.assertIn("Deferred adapters", adr_text)
        self.assertIn("evidence loss risks", adr_text.lower())
        self.assertIn("not an ALM/QMS replacement", adr_text)

        fixture_path = ROOT / "docs/regulated/domain-packs/alm-qms-export/github-issues-export-fixture.yaml"
        self.assertTrue(fixture_path.exists(), f"missing fixture: {fixture_path}")
        fixture = yaml.safe_load(fixture_path.read_text(encoding="utf-8"))
        self.assertEqual(fixture["adapter_decision_id"], "ADR-ALM-QMS-020")
        self.assertEqual(fixture["chosen_adapter"]["id"], "github-issues-evidence-export")
        self.assertEqual(fixture["chosen_adapter"]["status"], "first_supported")
        self.assertEqual(fixture["chosen_adapter"]["direction"], "nomos_to_external")

        exported_fields = {
            field["name"]
            for field in fixture["chosen_adapter"]["exported_fields"]
        }
        self.assertLessEqual(
            {"canonical_ref", "content_hash", "claim_boundary", "source_span"},
            exported_fields,
        )

        blocked_ids = {adapter["id"] for adapter in fixture["blocked_adapters"]}
        self.assertLessEqual({"bidirectional_reqif_sync", "qms_template_import"}, blocked_ids)

        deferred_ids = {adapter["id"] for adapter in fixture["deferred_adapters"]}
        self.assertLessEqual({"reqif_export", "jira_export", "csv_export", "json_schema_export"}, deferred_ids)

        risks = {risk["risk"] for risk in fixture["evidence_loss_risks"]}
        self.assertLessEqual({"rich_text_normalization", "status_semantics_drift"}, risks)

    def test_control_plane_roadmap_separates_evidence_model_from_optional_ui(self) -> None:
        import yaml

        roadmap_path = ROOT / "docs/regulated/control-plane/multi-corpus-roadmap.yaml"
        self.assertTrue(roadmap_path.exists(), f"missing roadmap: {roadmap_path}")
        roadmap = yaml.safe_load(roadmap_path.read_text(encoding="utf-8"))
        self.assertEqual(roadmap["roadmap_id"], "DOR-022")
        self.assertIn("CLI-first", roadmap["claim_boundary"])

        required_entities = {
            entity["id"]: entity
            for entity in roadmap["required_evidence_model"]["entities"]
        }
        self.assertLessEqual(
            {
                "corpus",
                "domain_profile",
                "source_version",
                "open_findings",
                "claim_level",
                "release_status",
                "periodic_review",
                "evidence_bundle",
            },
            set(required_entities),
        )
        self.assertTrue(all(entity["required_for_mvp"] for entity in required_entities.values()))

        sample = roadmap["portfolio_record_fixture"]
        self.assertEqual(sample["corpus"], "rbok-reference-corpus")
        self.assertEqual(sample["domain_profile"], "gxp-csv")
        self.assertEqual(sample["claim_level"], "mapped")
        self.assertEqual(sample["release_status"], "draft")
        self.assertIn("EV-", sample["evidence_bundle"])
        self.assertGreaterEqual(sample["open_findings"], 0)

        ui = roadmap["optional_ui_dashboard"]
        self.assertEqual(ui["status"], "optional")
        self.assertFalse(ui["required_for_mvp"])
        self.assertIn("control-plane-api", roadmap["deferred_capabilities"])

    def test_commercial_positioning_pack_bounds_pricing_and_public_claims(self) -> None:
        import yaml

        pack_path = ROOT / "docs/regulated/domain-packs/commercial-positioning/commercial-positioning-pack.yaml"
        self.assertTrue(pack_path.exists(), f"missing commercial pack: {pack_path}")
        pack = yaml.safe_load(pack_path.read_text(encoding="utf-8"))
        self.assertEqual(pack["pack_id"], "DOR-023")
        self.assertIn("no certification", pack["claim_boundary"].lower())

        categories = {
            category["id"]: category
            for category in pack["market_category_comparison"]
        }
        self.assertLessEqual(
            {"alm", "validation_lifecycle_management", "qms", "rag_governance", "regtech"},
            set(categories),
        )
        self.assertEqual(categories["alm"]["positioning"], "upstream_and_adjacent")
        self.assertEqual(categories["qms"]["positioning"], "evidence_supplier_not_replacement")

        packaging = {item["id"]: item for item in pack["packaging_assumptions"]}
        self.assertLessEqual({"cli_evidence_core", "domain_pack", "control_plane"}, set(packaging))

        pricing = pack["pricing_assumptions"]
        self.assertEqual(pricing["status"], "strategy_notes_not_financial_claims")
        self.assertFalse(pricing["valuation_claim_allowed"])
        self.assertIn("customer validation", pricing["depends_on"])

        # Commercial positioning is bounded in the neutral valuation-inputs pack,
        # not the public README.
        inputs_text = (ROOT / "docs/external-assessment/valuation-inputs.md").read_text(encoding="utf-8")
        self.assertIn("commercial-positioning-pack.yaml", inputs_text)
        self.assertIn("notes de stratégie", inputs_text)
        self.assertIn("sans revendication de certification", inputs_text)

    def test_rbok_nomos_iq_baseline_records_installation_scope(self) -> None:
        import yaml

        baseline_path = ROOT / "docs/regulated/qualification/rbok-nomos-iq-baseline.yaml"
        self.assertTrue(baseline_path.exists(), f"missing IQ baseline: {baseline_path}")
        baseline = yaml.safe_load(baseline_path.read_text(encoding="utf-8"))
        self.assertEqual(baseline["baseline_id"], "IQ-RBOK-NOMOS-2026-05-06")
        self.assertEqual(baseline["qualification_phase"], "IQ")
        self.assertIn("not OQ or PQ evidence", baseline["claim_boundary"])

        installed = baseline["installed_baseline"]
        self.assertEqual(installed["rbok_develop_sha"], "fd0aee8f")
        self.assertEqual(installed["github_target"]["repo"], "RBOKproject/RBOK")
        self.assertEqual(installed["github_target"]["branch"], "develop")
        self.assertFalse(installed["github_target"]["direct_push_allowed"])

        readiness = baseline["portfolio_readiness"]
        self.assertEqual(readiness["ready_bindings"], 55)
        self.assertEqual(readiness["total_bindings"], 55)
        self.assertEqual(readiness["blockers"], 0)

        artifact_paths = {artifact["path"] for artifact in baseline["ordo_state_artifacts"]}
        self.assertLessEqual(
            {
                "/root/.local/share/orch-state/_portfolio/session_start.json",
                "/root/.local/share/orch-state/_portfolio/clean_plan.json",
                "/root/.local/share/orch-state/_portfolio/unblock_tasks.json",
            },
            artifact_paths,
        )

        self.assertEqual(baseline["proof_dependency"], "#314")
        self.assertTrue(baseline["workflow_run_urls"])
        self.assertEqual(baseline["operator_identity"]["github_user"], "realisonsdotcom")
        self.assertEqual(baseline["tool_identity"]["tool"], "Codex")
        self.assertEqual(baseline["later_phase_boundaries"], ["OQ", "PQ"])

    def test_rbok_nomos_oq_protocol_models_latest_ci_signal(self) -> None:
        import yaml

        protocol_path = ROOT / "docs/regulated/qualification/rbok-nomos-oq-ci-signal-protocol.yaml"
        self.assertTrue(protocol_path.exists(), f"missing OQ protocol: {protocol_path}")
        protocol = yaml.safe_load(protocol_path.read_text(encoding="utf-8"))
        self.assertEqual(protocol["protocol_id"], "OQ-RBOK-NOMOS-CI-2026-05-06")
        self.assertEqual(protocol["qualification_phase"], "OQ")
        self.assertEqual(protocol["current_state_rule"], "latest_run_per_workflow_wins")
        self.assertIn("historical incident telemetry", protocol["claim_boundary"])

        cases = {case["id"]: case for case in protocol["test_cases"]}
        self.assertEqual(cases["stale_failure_healed"]["expected_gate_status"], "warning")
        self.assertEqual(cases["newest_failure"]["expected_gate_status"], "red")
        self.assertEqual(cases["in_progress_deploy"]["expected_gate_status"], "pending_external_wait")

        stale = cases["stale_failure_healed"]
        self.assertEqual(stale["historical_failure"]["run_id"], "25442507323")
        self.assertEqual(stale["newer_signal"]["conclusion"], "success")
        self.assertIn("https://github.com/RBOKproject/RBOK/actions/runs/25442507323", stale["evidence_urls"])

        merge_rule = protocol["merge_rule"]
        self.assertFalse(merge_rule["allow_merge_when_required_current_check_red"])
        self.assertTrue(merge_rule["allow_historical_failure_as_warning_when_superseded"])

        self.assertTrue(protocol["ordo_audit_log_lines"])
        self.assertIn("current gate status", protocol["claim_language_boundary"])
        self.assertIn("historical incident telemetry", protocol["claim_language_boundary"])

    def test_rbok_nomos_pq_pack_lists_required_readonly_journeys_and_blockers(self) -> None:
        import yaml

        pack_path = ROOT / "docs/regulated/qualification/rbok-nomos-pq-readonly-journey-pack.yaml"
        self.assertTrue(pack_path.exists(), f"missing PQ pack: {pack_path}")
        pack = yaml.safe_load(pack_path.read_text(encoding="utf-8"))
        self.assertEqual(pack["pack_id"], "PQ-RBOK-NOMOS-READONLY-2026-05-14")
        self.assertEqual(pack["schema_version"], "0.3.0")
        self.assertEqual(pack["qualification_phase"], "PQ")
        self.assertIn("no production validation", pack["claim_boundary"])
        self.assertEqual(pack["environment"]["url"], "https://app.realisons.com")
        self.assertEqual(pack["historical_environment_replaced"]["url"], "https://dev.realisons.com")
        self.assertEqual(pack["observed_public_checks"]["health"]["result"], "PASS")
        self.assertEqual(pack["observed_public_checks"]["health"]["http_status"], 200)
        self.assertEqual(pack["observed_public_checks"]["root_redirect"]["location"], "/login")
        self.assertEqual(pack["authenticated_profile"]["role"], "client")
        self.assertEqual(pack["authenticated_profile"]["capabilities_endpoint"]["capabilities"], [])
        self.assertEqual(pack["collaborateur_profile"]["role"], "collaborateur")
        self.assertEqual(pack["collaborateur_profile"]["capabilities_endpoint"]["capability_count"], 63)
        self.assertIn(
            "view_rbok_editor",
            pack["collaborateur_profile"]["capabilities_endpoint"]["relevant_capabilities"],
        )

        journeys = {journey["id"]: journey for journey in pack["readonly_journeys"]}
        self.assertLessEqual(
            {
                "documents_list_readonly",
                "document_detail_citation",
                "collaborateur_docs_workspace",
                "permission_denied_missing_capability",
                "empty_endpoint_data",
                "accessibility_readonly_surfaces",
            },
            set(journeys),
        )
        self.assertTrue(all("url" in journey for journey in journeys.values()))
        self.assertTrue(all("evidence_required" in journey for journey in journeys.values()))
        self.assertEqual(journeys["documents_list_readonly"]["result"], "PASS")
        self.assertEqual(
            journeys["documents_list_readonly"]["evidence_observed"]["document"]["document_code"],
            "rbok-core-v0.01",
        )
        self.assertEqual(journeys["document_detail_citation"]["result"], "PASS")
        self.assertTrue(journeys["document_detail_citation"]["evidence_observed"]["source_hash_present"])
        self.assertEqual(
            journeys["document_detail_citation"]["evidence_observed"]["collaborateur_ui"]["citation_sample"],
            "RBOK canonical · 2 · version 1",
        )
        self.assertEqual(journeys["collaborateur_docs_workspace"]["result"], "PASS")
        self.assertTrue(journeys["collaborateur_docs_workspace"]["evidence_observed"]["zero_edition_visible"])
        self.assertEqual(journeys["collaborateur_docs_workspace"]["evidence_observed"]["documents_count"], 6)
        self.assertEqual(journeys["permission_denied_missing_capability"]["result"], "PASS")
        self.assertEqual(journeys["empty_endpoint_data"]["evidence_observed"]["response_count"], 0)
        self.assertEqual(journeys["accessibility_readonly_surfaces"]["result"], "FAIL")
        self.assertEqual(
            journeys["accessibility_readonly_surfaces"]["child_issue"],
            "https://github.com/RBOKproject/RBOK/issues/4016",
        )
        self.assertEqual(
            journeys["accessibility_readonly_surfaces"]["evidence_observed"]["collaborateur_detail_lighthouse_snapshot"]["failed_audits"],
            2,
        )
        self.assertEqual(pack["blockers"], [])
        self.assertEqual(pack["findings"][0]["child_issue"], "https://github.com/RBOKproject/RBOK/issues/4016")
        self.assertEqual(pack["overall_result"], "EXECUTED_WITH_FINDINGS")

    def test_ordo_finding_capa_intake_rule_links_qualification_findings(self) -> None:
        import yaml

        rule_path = ROOT / "docs/regulated/qualification/ordo-finding-capa-intake.yaml"
        self.assertTrue(rule_path.exists(), f"missing ORDO CAPA intake rule: {rule_path}")
        rule = yaml.safe_load(rule_path.read_text(encoding="utf-8"))
        self.assertEqual(rule["rule_id"], "CAPA-ORDO-NOMOS-2026-05-06")
        self.assertEqual(rule["related_issue"], "#411")

        required_fields = set(rule["required_fields"])
        self.assertLessEqual(
            {
                "finding",
                "impact_on_iq_oq_pq_or_release_claim",
                "detection_signal",
                "safe_remediation_candidate",
                "validation_poc_plan",
                "priority",
                "source_links",
            },
            required_fields,
        )

        findings = {finding["id"]: finding for finding in rule["promoted_findings"]}
        self.assertLessEqual(
            {
                "dispatch_submission_reliability",
                "stale_ci_signal_detection",
                "preflight_unblock_plans",
                "dirty_worktree_checkpoints",
                "portfolio_readiness_state",
            },
            set(findings),
        )
        self.assertTrue(all(finding["source_links"] for finding in findings.values()))
        self.assertEqual(findings["stale_ci_signal_detection"]["linked_nomos_issue"], "#409")
        self.assertEqual(rule["future_reconciliation"]["requires_full_transcript_read"], False)

    def test_praxis_activation_gate_keeps_unverified_nomos_atoms_blocked(self) -> None:
        import yaml

        gate_path = ROOT / "docs/regulated/qualification/praxis-activation-gate.yaml"
        self.assertTrue(gate_path.exists(), f"missing Praxis activation gate: {gate_path}")
        gate = yaml.safe_load(gate_path.read_text(encoding="utf-8"))
        self.assertEqual(gate["schema_version"], "0.2.0")
        self.assertEqual(gate["activation_id"], "PRAXIS-NOMOS-ACTIVATION-2026-05-14")
        self.assertEqual(gate["related_issue"], "RBOKproject/PRAXIS#333")
        self.assertEqual(gate["current_status"], "blocked_until_nomos_verified")
        self.assertIn("not an activation approval", gate["claim_boundary"])

        proof = gate["nomos_required_proof"]
        self.assertEqual(proof["required_aq_status"], "accepted")
        required_artifacts = {artifact["path"] for artifact in proof["required_artifacts"]}
        self.assertLessEqual(
            {
                "docs/regulated/qualification/rbok-nomos-iq-baseline.yaml",
                "docs/regulated/qualification/rbok-nomos-oq-ci-signal-protocol.yaml",
                "docs/regulated/qualification/rbok-nomos-pq-readonly-journey-pack.yaml",
            },
            required_artifacts,
        )

        consumer_guard = gate["consumer_guard"]
        self.assertFalse(consumer_guard["praxis_may_consume_unverified_nomos_atoms_as_regulated_evidence"])
        self.assertEqual(consumer_guard["unverified_atom_handling"], "not_qualified_external_input")

        fixture = gate["mapping_contract_fixture"]
        self.assertEqual(fixture["status"], "planned_non_authoritative")
        self.assertEqual(fixture["contract_path"], "docs/regulated/customer-integration/praxis-atom-mapping.md")
        self.assertIn("review_state != approved", fixture["must_reject"])
        self.assertIn("synthetic/not_qualified", fixture["execution_trigger"])

        dossier = gate["dossier_state"]
        self.assertEqual(dossier["praxis_activation"], "blocked")
        self.assertIn("Nomos verification", dossier["blocked_reason"])

        blocked_claims = set(gate["blocked_claims"])
        self.assertIn("Praxis inherits Nomos regulated evidence status", blocked_claims)

        praxis_profile = yaml.safe_load(
            (ROOT / "docs/regulated/product-profiles/praxis.yaml").read_text(encoding="utf-8")
        )
        nomos_profile = yaml.safe_load(
            (ROOT / "docs/regulated/product-profiles/nomos.yaml").read_text(encoding="utf-8")
        )
        template = yaml.safe_load(
            (ROOT / "templates/regulated/regulated-product-profile.yaml").read_text(encoding="utf-8")
        )
        self.assertEqual(
            {praxis_profile["schema_version"], nomos_profile["schema_version"], template["schema_version"]},
            {"0.2.0"},
        )
        self.assertEqual(
            praxis_profile["owned_evidence"][0]["status"], "planned_non_authoritative"
        )
        self.assertIn("#334", " ".join(praxis_profile["critical_path"]["praxis_open_followup"]))
        self.assertIn("open_claim_gates", nomos_profile)
        self.assertNotIn("open_dependencies", nomos_profile)

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

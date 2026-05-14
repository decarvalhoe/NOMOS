#!/usr/bin/env python3
"""Validate regulated documentation guardrails.

This gate is intentionally conservative. It validates structure and blocks
obvious overclaiming, but it does not certify compliance.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for regulated documentation validation.", file=sys.stderr)
    raise SystemExit(2) from exc

from regulated_approval_gate import validate_workflow as validate_approval_workflow
from regulated_ai_provider_ledger import validate_ledger_document as validate_ai_provider_ledger


CONTROLLED_ROOTS = [
    Path("docs/regulated/quality-system"),
    Path("docs/regulated/lifecycle"),
    Path("docs/regulated/data-integrity"),
    Path("docs/regulated/security-privacy"),
    Path("docs/regulated/github-operating-model"),
]

YAML_ROOTS = [
    Path("docs/regulated"),
    Path("templates/regulated"),
    Path(".github"),
]

DOMAIN_PROFILE_ROOT = Path("specs/examples")

DOMAIN_REPORT_STATUSES = [
    "applicable",
    "not_applicable",
    "blocked",
    "waived",
    "missing_evidence",
]

CLAIM_LEVEL_RANK = {
    "registered": 0,
    "mapped": 1,
    "evidence_ready": 2,
    "validated_by_customer": 3,
    "independent_review_ready": 4,
}

REQUIRED_MARKERS = [
    "document_id:",
    "version:",
    "status:",
    "effective_status:",
    "owner:",
    "approver:",
]

PROHIBITED_PATTERNS = [
    (re.compile(r"part_11_claimed:\s*true", re.IGNORECASE), "Part 11 claim is not allowed in draft baseline."),
    (re.compile(r"github_as_validated_eqms:\s*true", re.IGNORECASE), "GitHub eQMS validation is not established."),
    (re.compile(r"github_as_part_11_esignature_system:\s*true", re.IGNORECASE), "GitHub Part 11 e-signature status is not established."),
    (re.compile(r"regulated_grade_claim_allowed:\s*true", re.IGNORECASE), "Regulated-grade claim is not allowed yet."),
    (re.compile(r"external_certification_claim:\s*(allowed|true)", re.IGNORECASE), "External certification claim is not allowed."),
]

DOR_ROADMAP_PATH = Path("docs/38-domain-opportunity-roadmap.md")
GXP_CSV_CROSSWALK_PATH = Path("docs/regulated/control-matrix/gxp-csv-control-crosswalk.yaml")
GXP_CSV_REQUIRED_REFERENCES = {
    "FDA-21CFR11-11.10",
    "EU-EUDRALEX-V4-ANNEX11",
    "FDA-CSA-2025",
    "MHRA-GXP-DATA-INTEGRITY-2018",
    "ISPE-GAMP5-2E-2022",
}
GXP_CSV_REFERENCE_DISPOSITIONS = {"mapped", "blocked", "not_applicable", "waived"}
AI_PROVIDER_LEDGER_PATH = Path("docs/regulated/ai-rag-governance/ai-provider-change-ledger.yaml")


def iter_existing_files(roots: list[Path], suffixes: tuple[str, ...]) -> list[Path]:
    paths: list[Path] = []
    for root in roots:
        if not root.exists():
            continue
        for path in root.rglob("*"):
            if path.is_file() and path.suffix.lower() in suffixes:
                paths.append(path)
    return sorted(set(paths))


def validate_yaml(path: Path, findings: list[dict[str, str]]) -> None:
    try:
        yaml.safe_load(path.read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001 - report parser detail to CI
        findings.append({
            "severity": "error",
            "path": str(path),
            "message": f"YAML parse failed: {exc}",
        })


def validate_controlled_markdown(path: Path, findings: list[dict[str, str]]) -> None:
    if path.name.upper() == "README.MD":
        return
    text = path.read_text(encoding="utf-8")
    for marker in REQUIRED_MARKERS:
        if marker not in text:
            findings.append({
                "severity": "error",
                "path": str(path),
                "message": f"Missing controlled-document marker `{marker}`.",
            })
    for pattern, message in PROHIBITED_PATTERNS:
        if pattern.search(text):
            findings.append({
                "severity": "error",
                "path": str(path),
                "message": message,
            })


def validate_no_overclaim(path: Path, findings: list[dict[str, str]]) -> None:
    text = path.read_text(encoding="utf-8")
    for pattern, message in PROHIBITED_PATTERNS:
        if pattern.search(text):
            findings.append({
                "severity": "error",
                "path": str(path),
                "message": message,
            })


def validate_approval_workflow_file(findings: list[dict[str, str]]) -> None:
    workflow_path = Path("docs/regulated/validation-pack/approval-workflow.yaml")
    if not workflow_path.exists():
        findings.append({
            "severity": "error",
            "path": str(workflow_path),
            "message": "Approval workflow file is missing.",
        })
        return

    try:
        workflow = yaml.safe_load(workflow_path.read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001 - report parser detail to CI
        findings.append({
            "severity": "error",
            "path": str(workflow_path),
            "message": f"Approval workflow YAML parse failed: {exc}",
        })
        return

    if not isinstance(workflow, dict):
        findings.append({
            "severity": "error",
            "path": str(workflow_path),
            "message": "Approval workflow must be a YAML object.",
        })
        return

    approval_findings, _summary = validate_approval_workflow(workflow, workflow_path)
    for finding in approval_findings:
        findings.append({
            "severity": finding["severity"],
            "path": finding["path"],
            "message": f"Approval gate {finding['code']}: {finding['message']}",
        })


def iter_domain_profile_files() -> list[Path]:
    if not DOMAIN_PROFILE_ROOT.exists():
        return []
    return sorted(DOMAIN_PROFILE_ROOT.glob("nomos-domain-profile.*.valid.yaml"))


def domain_claim_status(level: str, current_level: str) -> str:
    level_rank = CLAIM_LEVEL_RANK.get(level, -1)
    current_rank = CLAIM_LEVEL_RANK.get(current_level, -1)
    if level_rank > current_rank:
        return "exceeds_evidence"
    return "allowed"


def profile_report_status(profile: dict, missing_artifacts: list[str]) -> str:
    if profile.get("waiver"):
        return "waived"

    applicability = profile.get("applicability") or {}
    status = str(applicability.get("status", "")).strip()
    if status == "blocked":
        return "blocked"
    if status == "not_applicable":
        return "not_applicable"
    if missing_artifacts:
        return "missing_evidence"
    return "applicable"


def evaluate_domain_profile(path: Path) -> tuple[dict, list[dict[str, str]]]:
    findings: list[dict[str, str]] = []
    try:
        profile = yaml.safe_load(path.read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001 - report parser detail to CI
        finding = {
            "severity": "error",
            "code": "DOMAIN_PROFILE_PARSE_FAILED",
            "path": str(path),
            "message": f"Domain profile YAML parse failed: {exc}",
        }
        return {
            "path": str(path),
            "domain_profile": path.stem,
            "report_status": "blocked",
            "findings": [finding],
        }, [finding]

    if not isinstance(profile, dict):
        finding = {
            "severity": "error",
            "code": "DOMAIN_PROFILE_NOT_OBJECT",
            "path": str(path),
            "message": "Domain profile must be a YAML object.",
        }
        return {
            "path": str(path),
            "domain_profile": path.stem,
            "report_status": "blocked",
            "findings": [finding],
        }, [finding]

    claim_ladder = profile.get("claim_ladder") or {}
    current_level = str(claim_ladder.get("current_level", "")).strip()
    current_rank = CLAIM_LEVEL_RANK.get(current_level, -1)

    missing_artifacts: list[str] = []
    for artifact in profile.get("required_artifacts") or []:
        if not isinstance(artifact, dict):
            continue
        if artifact.get("required", True) is False:
            continue
        minimum_level = str(artifact.get("minimum_claim_level", "registered")).strip()
        minimum_rank = CLAIM_LEVEL_RANK.get(minimum_level, 0)
        if current_rank >= 0 and minimum_rank > current_rank:
            continue
        artifact_path = str(artifact.get("path", "")).strip()
        if artifact_path and not Path(artifact_path).exists():
            missing_artifacts.append(artifact_path)
            findings.append({
                "severity": "error",
                "code": "DOMAIN_REQUIRED_ARTIFACT_MISSING",
                "path": str(path),
                "message": f"Required domain artifact is missing: {artifact_path}",
            })

    public_claim_review: list[dict[str, str]] = []
    for claim in claim_ladder.get("authorized_claims") or []:
        if not isinstance(claim, dict):
            continue
        claim_id = str(claim.get("id", "")).strip()
        claim_level = str(claim.get("level", "")).strip()
        status = domain_claim_status(claim_level, current_level)
        public_claim_review.append({
            "claim_id": claim_id,
            "level": claim_level,
            "statement": str(claim.get("statement", "")).strip(),
            "status": status,
        })
        if status == "exceeds_evidence":
            findings.append({
                "severity": "error",
                "code": "DOMAIN_CLAIM_LEVEL_EXCEEDS_EVIDENCE",
                "path": str(path),
                "message": (
                    f"Domain claim {claim_id} requires {claim_level}, "
                    f"but current evidence level is {current_level}."
                ),
            })

    profile_report = {
        "path": str(path),
        "domain_profile": str(profile.get("domain_profile", "")).strip(),
        "applicability_status": str((profile.get("applicability") or {}).get("status", "")).strip(),
        "report_status": profile_report_status(profile, missing_artifacts),
        "current_level": current_level,
        "missing_artifacts": missing_artifacts,
        "public_claim_review": public_claim_review,
        "blocked_claims": [
            {
                "claim_id": str(claim.get("id", "")).strip(),
                "kind": str(claim.get("kind", "")).strip(),
                "statement": str(claim.get("statement", "")).strip(),
            }
            for claim in claim_ladder.get("blocked_claims") or []
            if isinstance(claim, dict)
        ],
    }
    if profile.get("waiver"):
        profile_report["waiver"] = profile["waiver"]
    return profile_report, findings


def build_domain_applicability_report() -> tuple[dict, list[dict[str, str]]]:
    profiles: list[dict] = []
    findings: list[dict[str, str]] = []
    status_counts = {status: 0 for status in DOMAIN_REPORT_STATUSES}

    for path in iter_domain_profile_files():
        profile_report, profile_findings = evaluate_domain_profile(path)
        profiles.append(profile_report)
        findings.extend(profile_findings)
        status = profile_report.get("report_status", "blocked")
        if status not in status_counts:
            status = "blocked"
        status_counts[status] += 1

    return {
        "schema_version": "0.1.0",
        "status": "failed" if findings else "passed",
        "claim_boundary": "Domain applicability only; no compliance certification.",
        "profile_count": len(profiles),
        "status_counts": status_counts,
        "profiles": profiles,
        "findings": findings,
    }, findings


def validate_gxp_csv_crosswalk(findings: list[dict[str, str]]) -> None:
    if not GXP_CSV_CROSSWALK_PATH.exists():
        if DOR_ROADMAP_PATH.exists():
            findings.append({
                "severity": "error",
                "path": str(GXP_CSV_CROSSWALK_PATH),
                "message": "GxP/CSV crosswalk is required by the domain opportunity roadmap.",
            })
        return

    try:
        crosswalk = yaml.safe_load(GXP_CSV_CROSSWALK_PATH.read_text(encoding="utf-8")) or {}
    except Exception as exc:  # noqa: BLE001 - parser detail belongs in CI output
        findings.append({
            "severity": "error",
            "path": str(GXP_CSV_CROSSWALK_PATH),
            "message": f"GxP/CSV crosswalk YAML parse failed: {exc}",
        })
        return

    references = crosswalk.get("references")
    if not isinstance(references, list):
        findings.append({
            "severity": "error",
            "path": str(GXP_CSV_CROSSWALK_PATH),
            "message": "GxP/CSV crosswalk must declare a references list.",
        })
        return

    by_reference: dict[str, dict] = {}
    for index, reference in enumerate(references):
        if not isinstance(reference, dict):
            findings.append({
                "severity": "error",
                "path": f"{GXP_CSV_CROSSWALK_PATH}:references[{index}]",
                "message": "GxP/CSV crosswalk reference row must be an object.",
            })
            continue
        reference_id = str(reference.get("reference_id", "")).strip()
        disposition = str(reference.get("disposition", "")).strip()
        if not reference_id:
            findings.append({
                "severity": "error",
                "path": f"{GXP_CSV_CROSSWALK_PATH}:references[{index}].reference_id",
                "message": "GxP/CSV crosswalk reference row is missing reference_id.",
            })
            continue
        by_reference[reference_id] = reference
        if disposition not in GXP_CSV_REFERENCE_DISPOSITIONS:
            findings.append({
                "severity": "error",
                "path": f"{GXP_CSV_CROSSWALK_PATH}:{reference_id}.disposition",
                "message": "GxP/CSV crosswalk reference disposition must be mapped, blocked, not_applicable, or waived.",
            })
        controls = reference.get("controls") or []
        if disposition == "mapped" and (not isinstance(controls, list) or not controls):
            findings.append({
                "severity": "error",
                "path": f"{GXP_CSV_CROSSWALK_PATH}:{reference_id}.controls",
                "message": "GxP/CSV crosswalk mapped reference must list at least one NOMOS control.",
            })
        if disposition in {"blocked", "not_applicable", "waived"} and not str(reference.get("rationale", "")).strip():
            findings.append({
                "severity": "error",
                "path": f"{GXP_CSV_CROSSWALK_PATH}:{reference_id}.rationale",
                "message": "GxP/CSV crosswalk non-mapped reference must declare a rationale.",
            })

    for reference_id in sorted(GXP_CSV_REQUIRED_REFERENCES - set(by_reference)):
        findings.append({
            "severity": "error",
            "path": str(GXP_CSV_CROSSWALK_PATH),
            "message": f"GxP/CSV crosswalk missing required reference {reference_id}.",
        })


def validate_ai_provider_ledger_file(findings: list[dict[str, str]]) -> None:
    if not AI_PROVIDER_LEDGER_PATH.exists():
        return

    try:
        ledger = yaml.safe_load(AI_PROVIDER_LEDGER_PATH.read_text(encoding="utf-8")) or {}
    except Exception as exc:  # noqa: BLE001 - parser detail belongs in CI output
        findings.append({
            "severity": "error",
            "path": str(AI_PROVIDER_LEDGER_PATH),
            "message": f"AI provider change ledger YAML parse failed: {exc}",
        })
        return

    if not isinstance(ledger, dict):
        findings.append({
            "severity": "error",
            "path": str(AI_PROVIDER_LEDGER_PATH),
            "message": "AI provider change ledger must be a YAML object.",
        })
        return

    for finding in validate_ai_provider_ledger(ledger, AI_PROVIDER_LEDGER_PATH):
        findings.append({
            "severity": finding["severity"],
            "path": finding["path"],
            "message": f"AI provider ledger {finding['code']}: {finding['message']}",
        })


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--report", default="regulated-doc-gate-report.json")
    args = parser.parse_args()

    findings: list[dict[str, str]] = []
    report_path = Path(args.report)

    yaml_files = iter_existing_files(YAML_ROOTS, (".yaml", ".yml"))
    for path in yaml_files:
        validate_yaml(path, findings)
        validate_no_overclaim(path, findings)

    controlled_md = iter_existing_files(CONTROLLED_ROOTS, (".md",))
    for path in controlled_md:
        validate_controlled_markdown(path, findings)

    reference_register = Path("docs/regulated/reference-basis/external-reference-register.yaml")
    if not reference_register.exists():
        findings.append({
            "severity": "error",
            "path": str(reference_register),
            "message": "External reference register is missing.",
        })

    issue_templates = Path(".github/ISSUE_TEMPLATE")
    if not issue_templates.exists():
        findings.append({
            "severity": "error",
            "path": str(issue_templates),
            "message": "GitHub issue forms are missing.",
        })

    validate_approval_workflow_file(findings)
    validate_gxp_csv_crosswalk(findings)
    validate_ai_provider_ledger_file(findings)

    domain_report, domain_findings = build_domain_applicability_report()
    findings.extend(domain_findings)
    domain_report_path = report_path.parent / "domain-applicability-report.json"

    report = {
        "schema_version": "0.1.0",
        "status": "failed" if findings else "passed",
        "claim_boundary": "Documentation gate only; no compliance certification.",
        "yaml_files_checked": len(yaml_files),
        "controlled_markdown_files_checked": len(controlled_md),
        "domain_applicability_report": str(domain_report_path),
        "domain_applicability": {
            "profile_count": domain_report["profile_count"],
            "status_counts": domain_report["status_counts"],
        },
        "findings": findings,
    }
    report_path.parent.mkdir(parents=True, exist_ok=True)
    domain_report_path.write_text(json.dumps(domain_report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report, indent=2, sort_keys=True))
    return 1 if findings else 0


if __name__ == "__main__":
    raise SystemExit(main())

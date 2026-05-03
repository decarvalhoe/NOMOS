#!/usr/bin/env python3
"""Validate the regulated approval workflow guardrails.

This gate validates whether Nomos has a controlled approval workflow and
whether any approved status is supported by explicit evidence. It does not
approve records and does not claim Part 11 compliance.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for approval workflow validation.", file=sys.stderr)
    raise SystemExit(2) from exc


EXPECTED_ROLES = {"quality_owner", "product_owner", "technical_owner"}
APPROVED_STATES = {"approved", "approved_effective"}
PENDING_STATES = {"draft", "pending_approval", "pending_quality_owner", "pending_product_owner", "pending_technical_owner"}


def add_finding(findings: list[dict[str, str]], code: str, path: str, message: str) -> None:
    findings.append({
        "severity": "error",
        "code": code,
        "path": path,
        "message": message,
    })


def as_bool(value: Any) -> bool:
    return bool(value) is True


def sorted_role_ids(raw_roles: Any) -> list[str]:
    roles: set[str] = set()
    if isinstance(raw_roles, list):
        for role in raw_roles:
            if isinstance(role, dict) and role.get("id"):
                roles.add(str(role["id"]))
            elif isinstance(role, str):
                roles.add(role)
    return sorted(roles)


def list_refs(raw_refs: Any) -> list[str]:
    if raw_refs is None:
        return []
    if isinstance(raw_refs, list):
        return [str(ref) for ref in raw_refs if str(ref).strip()]
    return [str(raw_refs)] if str(raw_refs).strip() else []


def validate_workflow(workflow: dict[str, Any], workflow_path: Path) -> tuple[list[dict[str, str]], dict[str, Any]]:
    findings: list[dict[str, str]] = []
    path = str(workflow_path)

    required_roles = sorted_role_ids(workflow.get("required_roles"))
    missing_roles = sorted(EXPECTED_ROLES - set(required_roles))
    if missing_roles:
        add_finding(findings, "MISSING_REQUIRED_APPROVER_ROLE", path, f"Missing required role(s): {', '.join(missing_roles)}.")

    channels = workflow.get("evidence_channels", {})
    pr_review = channels.get("protected_pr_review", {}) if isinstance(channels, dict) else {}
    signed = channels.get("signed_commits_or_tags", {}) if isinstance(channels, dict) else {}
    attestation = channels.get("attestation_artifact", {}) if isinstance(channels, dict) else {}

    if not as_bool(pr_review.get("enabled")):
        add_finding(findings, "PR_REVIEW_CHANNEL_DISABLED", path, "Protected PR review evidence channel must be enabled.")
    if not as_bool(pr_review.get("requires_codeowners")):
        add_finding(findings, "CODEOWNER_REVIEW_NOT_REQUIRED", path, "Protected PR review must require CODEOWNERS.")
    if int(pr_review.get("minimum_approvals", 0) or 0) < 2:
        add_finding(findings, "INSUFFICIENT_PR_APPROVALS", path, "At least two PR approvals are required for regulated approval evidence.")
    if not as_bool(signed.get("required_for_effective_release")):
        add_finding(findings, "SIGNED_RELEASE_NOT_REQUIRED", path, "Signed commits or signed tags must be required before effective release.")
    if not str(attestation.get("path", "")).strip():
        add_finding(findings, "MISSING_APPROVAL_ATTESTATION_PATH", path, "Approval attestation artifact path is required.")

    immutability = workflow.get("immutability_controls", {})
    if not as_bool(immutability.get("codeowners_required")):
        add_finding(findings, "IMMUTABILITY_CODEOWNERS_MISSING", path, "CODEOWNERS must be part of the immutability controls.")
    if not as_bool(immutability.get("evidence_pack_hashing_required")):
        add_finding(findings, "IMMUTABILITY_HASHING_MISSING", path, "Evidence pack hashing must be required.")
    if not as_bool(immutability.get("immutable_release_tag_required_for_effective_status")):
        add_finding(findings, "IMMUTABLE_RELEASE_TAG_MISSING", path, "Effective approval requires an immutable signed release tag.")

    approval_records = workflow.get("approval_records", [])
    if not isinstance(approval_records, list) or not approval_records:
        add_finding(findings, "NO_APPROVAL_RECORDS", path, "At least one approval record entry is required.")
        approval_records = []

    approved_records = 0
    pending_records = 0
    for idx, record in enumerate(approval_records, start=1):
        record_path = f"{path}#approval_records[{idx}]"
        if not isinstance(record, dict):
            add_finding(findings, "INVALID_APPROVAL_RECORD", record_path, "Approval record must be an object.")
            continue

        status = str(record.get("approval_status", "")).strip()
        if not status:
            add_finding(findings, "MISSING_APPROVAL_STATUS", record_path, "approval_status is required.")
            continue
        if status in APPROVED_STATES:
            approved_records += 1
            refs = list_refs(record.get("evidence_refs"))
            if not refs:
                add_finding(findings, "APPROVED_WITHOUT_EVIDENCE", record_path, "Approved records require immutable evidence_refs.")
            if not record.get("approved_at"):
                add_finding(findings, "APPROVED_WITHOUT_TIMESTAMP", record_path, "Approved records require approved_at.")
            if not list_refs(record.get("approver_refs")):
                add_finding(findings, "APPROVED_WITHOUT_APPROVER_REFS", record_path, "Approved records require approver_refs.")
        elif status in PENDING_STATES:
            pending_records += 1
            if record.get("overclaim_guard") is not True:
                add_finding(findings, "PENDING_WITHOUT_OVERCLAIM_GUARD", record_path, "Pending approval records must set overclaim_guard: true.")
        else:
            add_finding(findings, "UNKNOWN_APPROVAL_STATUS", record_path, f"Unknown approval_status {status!r}.")

        record_roles = set(list_refs(record.get("required_roles")))
        missing_record_roles = sorted(EXPECTED_ROLES - record_roles)
        if missing_record_roles:
            add_finding(findings, "APPROVAL_RECORD_MISSING_ROLE", record_path, f"Approval record missing role(s): {', '.join(missing_record_roles)}.")

    summary = {
        "approval_records": len(approval_records),
        "approved_records": approved_records,
        "pending_records": pending_records,
        "findings": len(findings),
    }
    return findings, summary


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=".")
    parser.add_argument("--workflow", default="docs/regulated/validation-pack/approval-workflow.yaml")
    parser.add_argument("--report", default="approval-gate-report.json")
    args = parser.parse_args()

    root = Path(args.root)
    workflow_path = root / args.workflow
    findings: list[dict[str, str]] = []
    workflow: dict[str, Any] = {}

    if not workflow_path.exists():
        add_finding(findings, "APPROVAL_WORKFLOW_MISSING", str(workflow_path), "Approval workflow file is missing.")
        summary = {"approval_records": 0, "approved_records": 0, "pending_records": 0, "findings": len(findings)}
        required_roles: list[str] = []
    else:
        try:
            loaded = yaml.safe_load(workflow_path.read_text(encoding="utf-8"))
        except Exception as exc:  # noqa: BLE001 - parser detail belongs in CI report
            add_finding(findings, "APPROVAL_WORKFLOW_YAML_INVALID", str(workflow_path), f"YAML parse failed: {exc}")
            loaded = {}
        workflow = loaded if isinstance(loaded, dict) else {}
        role_findings, summary = validate_workflow(workflow, workflow_path)
        findings.extend(role_findings)
        summary["findings"] = len(findings)
        required_roles = sorted_role_ids(workflow.get("required_roles"))

    status = "failed"
    if not findings:
        status = "approved" if summary["approved_records"] > 0 and summary["pending_records"] == 0 else "pending_approval"

    report = {
        "schema_version": "0.1.0",
        "status": status,
        "claim_boundary": "Approval workflow gate only; no validation approval or regulatory compliance certification.",
        "workflow_path": str(workflow_path),
        "required_roles": required_roles,
        "summary": summary,
        "findings": findings,
    }

    report_path = Path(args.report)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report, indent=2, sort_keys=True))
    return 1 if findings else 0


if __name__ == "__main__":
    raise SystemExit(main())

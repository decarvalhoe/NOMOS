#!/usr/bin/env python3
"""Generate a CSA-style risk-based validation plan.

The planner ranks domain controls by risk and assigns the expected evidence
depth. It supports validation planning only and does not claim regulated
validation, compliance, certification, or legal sufficiency.
"""

from __future__ import annotations

import argparse
import json
import sys
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for regulated validation planning.", file=sys.stderr)
    raise SystemExit(2) from exc


CLAIM_BOUNDARY = "CSA-style validation planning only; no validation, compliance, or certification claim."
DEFAULT_DOMAIN_PROFILE = Path("specs/examples/nomos-domain-profile.gxp.valid.yaml")
DEFAULT_CONTROL_MATRIX = Path("docs/regulated/control-matrix/nomos-control-matrix.yaml")
DEFAULT_CROSSWALK = Path("docs/regulated/control-matrix/gxp-csv-control-crosswalk.yaml")
DEFAULT_OUTPUT = Path(".regulated-evidence-pack/risk-based-validation-plan.json")

HIGH_RISK_LEVELS = {"high", "critical"}
LOW_RISK_LEVELS = {"low"}


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def resolve(root: Path, value: str | Path) -> Path:
    path = Path(value)
    return path if path.is_absolute() else root / path


def rel(path: Path, root: Path) -> str:
    return path.resolve().relative_to(root.resolve()).as_posix()


def load_yaml(path: Path) -> dict[str, Any]:
    return yaml.safe_load(path.read_text(encoding="utf-8")) or {}


def as_list(value: Any) -> list[Any]:
    return value if isinstance(value, list) else []


def domain_reference_ids(domain_profile: dict[str, Any]) -> set[str]:
    refs = domain_profile.get("references")
    return {
        str(reference.get("id", "")).strip()
        for reference in as_list(refs)
        if isinstance(reference, dict) and str(reference.get("id", "")).strip()
    }


def crosswalk_control_refs(crosswalk: dict[str, Any]) -> dict[str, list[str]]:
    control_refs: dict[str, list[str]] = {}
    for reference in as_list(crosswalk.get("references")):
        if not isinstance(reference, dict):
            continue
        reference_id = str(reference.get("reference_id", "")).strip()
        for control_id in as_list(reference.get("controls")):
            control_key = str(control_id).strip()
            if not control_key:
                continue
            control_refs.setdefault(control_key, [])
            if reference_id:
                control_refs[control_key].append(reference_id)
    return {control_id: sorted(set(refs)) for control_id, refs in sorted(control_refs.items())}


def control_lookup(control_matrix: dict[str, Any]) -> dict[str, dict[str, Any]]:
    controls: dict[str, dict[str, Any]] = {}
    for control in as_list(control_matrix.get("controls")):
        if not isinstance(control, dict):
            continue
        control_id = str(control.get("control_id", "")).strip()
        if control_id:
            controls[control_id] = control
    return controls


def selected_controls(
    *,
    domain_profile: dict[str, Any],
    control_matrix: dict[str, Any],
    crosswalk: dict[str, Any],
) -> tuple[list[dict[str, Any]], dict[str, list[str]], list[dict[str, str]]]:
    controls_by_id = control_lookup(control_matrix)
    references_by_control = crosswalk_control_refs(crosswalk)
    findings: list[dict[str, str]] = []

    if references_by_control:
        selected: list[dict[str, Any]] = []
        for control_id, references in references_by_control.items():
            control = controls_by_id.get(control_id)
            if control is None:
                findings.append(
                    {
                        "code": "CROSSWALK_CONTROL_NOT_FOUND",
                        "severity": "error",
                        "path": f"crosswalk:{control_id}",
                        "message": f"Crosswalk references unknown control {control_id}.",
                    }
                )
                continue
            selected.append(control)
        return selected, references_by_control, findings

    domain_refs = domain_reference_ids(domain_profile)
    selected = []
    references_by_control = {}
    for control_id, control in controls_by_id.items():
        control_refs = {str(reference).strip() for reference in as_list(control.get("external_refs"))}
        matched_refs = sorted(domain_refs & control_refs)
        if matched_refs:
            selected.append(control)
            references_by_control[control_id] = matched_refs
    return selected, references_by_control, findings


def verification_policy(risk_level: str) -> tuple[str, list[str], bool, str]:
    if risk_level in HIGH_RISK_LEVELS:
        return (
            "scripted_or_challenge_evidence",
            ["scripted_test", "challenge_case", "objective_artifact"],
            False,
            "High and critical controls require scripted or challenge evidence; review-only evidence is not enough.",
        )
    if risk_level in LOW_RISK_LEVELS:
        return (
            "documented_rationale",
            ["documented_rationale", "review_record"],
            True,
            "Low-risk controls may use lighter evidence when the rationale is explicit and retained.",
        )
    return (
        "scripted_or_review_evidence",
        ["scripted_test_or_review_protocol", "documented_rationale", "objective_artifact"],
        True,
        "Medium-risk controls require retained objective evidence and a documented rationale.",
    )


def planned_control(
    control: dict[str, Any],
    *,
    references: list[str],
    domain_risk_level: str,
) -> dict[str, Any]:
    risk_level = str(control.get("risk_level") or domain_risk_level or "medium").strip().lower()
    verification_type, required_evidence, lighter_allowed, rationale = verification_policy(risk_level)
    return {
        "control_id": str(control.get("control_id", "")).strip(),
        "title": str(control.get("title", "")).strip(),
        "function": str(control.get("title", "")).strip(),
        "control_family": str(control.get("control_family", "")).strip(),
        "criticality": risk_level,
        "required_verification_type": verification_type,
        "required_evidence": required_evidence,
        "lighter_evidence_allowed": lighter_allowed,
        "planning_rationale": rationale,
        "source_evidence_type": str(control.get("evidence_type", "")).strip(),
        "source_verification_ref": str(control.get("verification_ref", "")).strip(),
        "evidence_artifact": str(control.get("evidence_artifact", "")).strip(),
        "references": references,
    }


def validate_plan(controls: list[dict[str, Any]]) -> list[dict[str, str]]:
    findings: list[dict[str, str]] = []
    for control in controls:
        control_id = str(control.get("control_id", "unknown"))
        risk_level = str(control.get("criticality", "medium"))
        verification_type = str(control.get("required_verification_type", ""))
        evidence = set(str(item) for item in as_list(control.get("required_evidence")))
        if risk_level in HIGH_RISK_LEVELS and verification_type != "scripted_or_challenge_evidence":
            findings.append(
                {
                    "code": "HIGH_RISK_CONTROL_WITHOUT_SCRIPTED_OR_CHALLENGE_EVIDENCE",
                    "severity": "error",
                    "path": control_id,
                    "message": f"High-risk control {control_id} must require scripted or challenge evidence.",
                }
            )
        if risk_level in HIGH_RISK_LEVELS and not {"scripted_test", "challenge_case"} <= evidence:
            findings.append(
                {
                    "code": "HIGH_RISK_CONTROL_MISSING_EVIDENCE_TYPES",
                    "severity": "error",
                    "path": control_id,
                    "message": f"High-risk control {control_id} must include scripted_test and challenge_case evidence.",
                }
            )
        if risk_level in LOW_RISK_LEVELS and "documented_rationale" not in evidence:
            findings.append(
                {
                    "code": "LOW_RISK_CONTROL_MISSING_RATIONALE",
                    "severity": "error",
                    "path": control_id,
                    "message": f"Low-risk control {control_id} must retain documented rationale for lighter evidence.",
                }
            )
    return findings


def build_report(
    *,
    root: Path,
    domain_profile_path: Path,
    control_matrix_path: Path,
    crosswalk_path: Path,
) -> dict[str, Any]:
    domain_profile = load_yaml(domain_profile_path)
    control_matrix = load_yaml(control_matrix_path)
    crosswalk = load_yaml(crosswalk_path) if crosswalk_path.exists() else {}

    domain_risk = domain_profile.get("risk_class", {})
    domain_risk_level = str(domain_risk.get("level", "medium")).strip().lower() if isinstance(domain_risk, dict) else "medium"
    selected, references_by_control, findings = selected_controls(
        domain_profile=domain_profile,
        control_matrix=control_matrix,
        crosswalk=crosswalk,
    )
    controls = [
        planned_control(
            control,
            references=references_by_control.get(str(control.get("control_id", "")).strip(), []),
            domain_risk_level=domain_risk_level,
        )
        for control in sorted(selected, key=lambda item: str(item.get("control_id", "")))
    ]
    if not controls:
        findings.append(
            {
                "code": "NO_DOMAIN_CONTROLS_SELECTED",
                "severity": "error",
                "path": rel(domain_profile_path, root),
                "message": "No controls could be selected for the domain validation plan.",
            }
        )
    findings.extend(validate_plan(controls))

    counts = Counter(str(control["criticality"]) for control in controls)
    high_controls = [control for control in controls if control["criticality"] in HIGH_RISK_LEVELS]
    low_controls = [control for control in controls if control["criticality"] in LOW_RISK_LEVELS]
    return {
        "schema_version": "0.1.0",
        "status": "failed" if findings else "generated",
        "generated_at_utc": utc_now(),
        "claim_boundary": CLAIM_BOUNDARY,
        "domain": {
            "profile": str(domain_profile.get("domain_profile", "")).strip(),
            "name": str(domain_profile.get("name", "")).strip(),
            "intended_use": domain_profile.get("intended_use", {}),
            "risk_class": domain_profile.get("risk_class", {}),
        },
        "source_documents": {
            "domain_profile": rel(domain_profile_path, root),
            "control_matrix": rel(control_matrix_path, root),
            "crosswalk": rel(crosswalk_path, root) if crosswalk_path.exists() else "not_available",
        },
        "summary": {
            "controls_planned": len(controls),
            "controls_by_criticality": dict(sorted(counts.items())),
            "high_risk_controls_require_scripted_or_challenge_evidence": all(
                control["required_verification_type"] == "scripted_or_challenge_evidence"
                and {"scripted_test", "challenge_case"} <= set(control["required_evidence"])
                for control in high_controls
            ),
            "low_risk_controls_allow_lighter_evidence_with_rationale": all(
                control["lighter_evidence_allowed"] and "documented_rationale" in control["required_evidence"]
                for control in low_controls
            ),
            "non_claim_boundary_enforced": True,
        },
        "controls": controls,
        "findings": findings,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Generate a CSA-style risk-based validation plan.")
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument("--domain-profile", default=str(DEFAULT_DOMAIN_PROFILE), help="Domain profile YAML path.")
    parser.add_argument("--control-matrix", default=str(DEFAULT_CONTROL_MATRIX), help="NOMOS control matrix YAML path.")
    parser.add_argument("--crosswalk", default=str(DEFAULT_CROSSWALK), help="Domain control crosswalk YAML path.")
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT), help="JSON report path.")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    output = resolve(root, args.output)
    output.parent.mkdir(parents=True, exist_ok=True)

    report = build_report(
        root=root,
        domain_profile_path=resolve(root, args.domain_profile),
        control_matrix_path=resolve(root, args.control_matrix),
        crosswalk_path=resolve(root, args.crosswalk),
    )
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report["summary"], indent=2, sort_keys=True))
    return 1 if report["status"] == "failed" else 0


if __name__ == "__main__":
    raise SystemExit(main())

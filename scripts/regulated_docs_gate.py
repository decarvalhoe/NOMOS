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


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--report", default="regulated-doc-gate-report.json")
    args = parser.parse_args()

    findings: list[dict[str, str]] = []

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

    report = {
        "schema_version": "0.1.0",
        "status": "failed" if findings else "passed",
        "claim_boundary": "Documentation gate only; no compliance certification.",
        "yaml_files_checked": len(yaml_files),
        "controlled_markdown_files_checked": len(controlled_md),
        "findings": findings,
    }
    report_path = Path(args.report)
    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report, indent=2, sort_keys=True))
    return 1 if findings else 0


if __name__ == "__main__":
    raise SystemExit(main())

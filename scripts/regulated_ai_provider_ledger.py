#!/usr/bin/env python3
"""Validate and emit the AI provider/model change-control ledger."""

from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for AI provider ledger validation.", file=sys.stderr)
    raise SystemExit(2) from exc


CLAIM_BOUNDARY = "AI provider change-control evidence only; no AI compliance or model-authority claim."
DEFAULT_LEDGER = Path("docs/regulated/ai-rag-governance/ai-provider-change-ledger.yaml")
DEFAULT_OUTPUT = Path(".regulated-evidence-pack/ai-provider-change-ledger.json")
PRESERVE_DOMAIN_CLAIM_STATES = {"approved_preserve_domain_claims", "impact_assessed_preserve_domain_claims"}
REQUIRED_RECORD_FIELDS = (
    "change_id",
    "provider",
    "model",
    "region",
    "data_use_policy",
    "api_version",
    "prompt_template_version",
    "evaluation_baseline",
    "approval_state",
)


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def resolve(root: Path, value: str | Path) -> Path:
    path = Path(value)
    return path if path.is_absolute() else root / path


def rel(path: Path, root: Path) -> str:
    return path.resolve().relative_to(root.resolve()).as_posix()


def load_yaml(path: Path) -> dict[str, Any]:
    return yaml.safe_load(path.read_text(encoding="utf-8")) or {}


def missing(value: Any) -> bool:
    return value is None or value == "" or value == []


def validate_ledger_document(ledger: dict[str, Any], path: Path | str) -> list[dict[str, str]]:
    findings: list[dict[str, str]] = []
    records = ledger.get("records")
    path_label = str(path)
    if not isinstance(records, list) or not records:
        return [
            {
                "code": "AI_PROVIDER_LEDGER_EMPTY",
                "severity": "error",
                "path": path_label,
                "message": "AI provider change ledger must declare at least one record.",
            }
        ]

    for index, record in enumerate(records):
        record_path = f"{path_label}:records[{index}]"
        if not isinstance(record, dict):
            findings.append(
                {
                    "code": "AI_PROVIDER_LEDGER_RECORD_INVALID",
                    "severity": "error",
                    "path": record_path,
                    "message": "AI provider change ledger record must be an object.",
                }
            )
            continue
        change_id = str(record.get("change_id") or f"records[{index}]")
        for field in REQUIRED_RECORD_FIELDS:
            if not missing(record.get(field)):
                continue
            findings.append(
                {
                    "code": "AI_PROVIDER_LEDGER_FIELD_MISSING",
                    "severity": "error",
                    "path": f"{record_path}.{field}",
                    "message": f"AI provider change record {change_id} is missing {field}.",
                }
            )

        approval_state = str(record.get("approval_state", "")).strip()
        impact = record.get("impact_assessment")
        if approval_state in PRESERVE_DOMAIN_CLAIM_STATES:
            if not isinstance(impact, dict) or str(impact.get("status", "")).strip() != "complete":
                findings.append(
                    {
                        "code": "AI_PROVIDER_CHANGE_MISSING_IMPACT_ASSESSMENT",
                        "severity": "error",
                        "path": f"{record_path}.impact_assessment",
                        "message": (
                            "AI provider/model changes that preserve domain claims require "
                            "impact assessment before claims are preserved."
                        ),
                    }
                )
    return findings


def build_report(root: Path, ledger_path: Path) -> dict[str, Any]:
    ledger = load_yaml(ledger_path)
    findings = validate_ledger_document(ledger, ledger_path)
    records = ledger.get("records") if isinstance(ledger.get("records"), list) else []
    return {
        "schema_version": "0.1.0",
        "status": "failed" if findings else "generated",
        "generated_at_utc": utc_now(),
        "claim_boundary": CLAIM_BOUNDARY,
        "source_documents": {
            "ledger": rel(ledger_path, root),
        },
        "summary": {
            "records": len(records),
            "findings": len(findings),
        },
        "records": records,
        "findings": findings,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Emit the AI provider/model change-control ledger.")
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument("--ledger", default=str(DEFAULT_LEDGER), help="Ledger YAML path.")
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT), help="JSON report path.")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    output = resolve(root, args.output)
    output.parent.mkdir(parents=True, exist_ok=True)

    report = build_report(root, resolve(root, args.ledger))
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report["summary"], indent=2, sort_keys=True))
    return 1 if report["status"] == "failed" else 0


if __name__ == "__main__":
    raise SystemExit(main())

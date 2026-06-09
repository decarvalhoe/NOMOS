#!/usr/bin/env python3
"""Generate a regulated evidence inventory from repository-local records.

The generated pack is an ALCOA+-oriented inventory: it hashes records and
records where they came from. It does not certify compliance.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shlex
import subprocess
import sys
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for regulated evidence pack validation.", file=sys.stderr)
    raise SystemExit(2) from exc


CLAIM_BOUNDARY = "Evidence inventory only; no compliance certification."
TOOL_NAME = "scripts/regulated_evidence_pack.py"
TOOL_VERSION = "0.2.0"
RETENTION_HINT = "regulated_evidence_archive; follows docs/regulated/operations/artifact-retention-policy.yaml"
DOMAIN_EVIDENCE_ROOT = Path("docs/regulated/domain-evidence")

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

EVIDENCE_PATTERNS = [
    ("regulated_document", "docs/regulated/**/*.md"),
    ("regulated_claim_boundary_attestation", "docs/regulated/claim-boundary/**/*.json"),
    ("regulated_data", "docs/regulated/**/*.yaml"),
    ("regulated_data", "docs/regulated/**/*.yml"),
    ("regulated_template", "templates/regulated/**/*.md"),
    ("regulated_template", "templates/regulated/**/*.yaml"),
    ("regulated_template", "templates/regulated/**/*.yml"),
    ("github_issue_form", ".github/ISSUE_TEMPLATE/**/*.yaml"),
    ("github_issue_form", ".github/ISSUE_TEMPLATE/**/*.yml"),
    ("github_pull_request_template", ".github/PULL_REQUEST_TEMPLATE.md"),
    ("github_codeowners", ".github/CODEOWNERS"),
    ("github_workflow", ".github/workflows/**/*.yaml"),
    ("github_workflow", ".github/workflows/**/*.yml"),
    ("regulated_tool", "scripts/regulated_*.py"),
]

REQUIRED_LOCAL_CONTROLS = {
    "external_reference_register": "docs/regulated/reference-basis/external-reference-register.yaml",
    "evidence_ledger": "docs/regulated/evidence-index/evidence-ledger.yaml",
    "issue_forms": ".github/ISSUE_TEMPLATE",
    "pull_request_template": ".github/PULL_REQUEST_TEMPLATE.md",
    "codeowners": ".github/CODEOWNERS",
    "regulated_documentation_gate": ".github/workflows/regulated-documentation-gate.yml",
}


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def actor() -> str:
    return os.environ.get("GITHUB_ACTOR") or os.environ.get("USERNAME") or os.environ.get("USER") or "unknown"


def command_string(argv: list[str]) -> str:
    suffix = f" {shlex.join(argv)}" if argv else ""
    return f"python {TOOL_NAME}{suffix}"


def git_value(root: Path, *args: str) -> str:
    try:
        result = subprocess.run(
            ["git", *args],
            cwd=root,
            text=True,
            capture_output=True,
            check=False,
        )
    except OSError:
        return "unavailable"
    if result.returncode != 0:
        return "unavailable"
    return result.stdout.strip() or "unavailable"


def source_commit(root: Path) -> str:
    return os.environ.get("GITHUB_SHA") or git_value(root, "rev-parse", "HEAD")


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def rel(path: Path, root: Path) -> str:
    return path.resolve().relative_to(root.resolve()).as_posix()


def nested_value(data: dict[str, Any], path: tuple[str, ...]) -> Any:
    value: Any = data
    for part in path:
        if not isinstance(value, dict):
            return None
        value = value.get(part)
    return value


def is_missing(value: Any) -> bool:
    return value is None or value == "" or value == []


def alcoa_envelope(
    *,
    path: str,
    digest: str,
    modified_at_utc: str,
    generated_at_utc: str,
    command: str,
    commit: str,
) -> dict[str, Any]:
    return {
        "attributable": {
            "actor": actor(),
            "tool": TOOL_NAME,
            "tool_version": TOOL_VERSION,
            "command": command,
        },
        "legible": {
            "human_readable_path": path,
            "machine_readable_path": path,
        },
        "contemporaneous": {
            "timestamp_utc": generated_at_utc,
            "source_modified_at_utc": modified_at_utc,
        },
        "original_or_true_copy": {
            "source_commit": commit,
            "source_hash": digest,
            "artifact_hash": digest,
            "hash_algorithm": "sha256",
        },
        "accurate": {
            "validation_status": "inventory_hash_recorded",
            "hash_algorithm": "sha256",
        },
        "complete": {
            "derivation": {
                "method": "repository_local_file_hash_inventory",
                "source_path": path,
            },
            "exclusions": [
                "file_content_not_embedded_in_evidence_pack",
            ],
        },
        "consistent": {
            "stable_id_policy": "repository_relative_path",
            "ordering_policy": "path_and_hash_sort_order",
        },
        "enduring": {
            "retention_hint": RETENTION_HINT,
        },
        "available": {
            "retrieval_procedure": "retrieve repository path at source_commit and verify sha256",
            "reconstruction_command": command,
        },
    }


def legacy_alcoa_attributes(envelope: dict[str, Any]) -> dict[str, Any]:
    return {
        "attributable": nested_value(envelope, ("attributable", "actor")),
        "legible": True,
        "contemporaneous": True,
        "original": nested_value(envelope, ("complete", "derivation", "source_path")),
        "accurate": "sha256",
        "complete": "inventory_record_only",
        "consistent": "path_and_hash_sort_order",
        "enduring": nested_value(envelope, ("enduring", "retention_hint")),
        "available": "repository_or_uploaded_artifact",
    }


def collect_records(
    root: Path,
    *,
    generated_at_utc: str,
    command: str,
    commit: str,
) -> list[dict[str, object]]:
    seen: dict[Path, str] = {}
    for category, pattern in EVIDENCE_PATTERNS:
        for path in root.glob(pattern):
            if not path.is_file():
                continue
            seen.setdefault(path.resolve(), category)

    records: list[dict[str, object]] = []
    for path, category in sorted(seen.items(), key=lambda item: rel(item[0], root)):
        stat = path.stat()
        relative_path = rel(path, root)
        modified_at_utc = datetime.fromtimestamp(stat.st_mtime, timezone.utc).isoformat().replace("+00:00", "Z")
        digest = sha256_file(path)
        envelope = alcoa_envelope(
            path=relative_path,
            digest=digest,
            modified_at_utc=modified_at_utc,
            generated_at_utc=generated_at_utc,
            command=command,
            commit=commit,
        )
        records.append(
            {
                "category": category,
                "path": relative_path,
                "sha256": digest,
                "size_bytes": stat.st_size,
                "modified_at_utc": modified_at_utc,
                "alcoa_envelope": envelope,
                "alcoa_attributes": legacy_alcoa_attributes(envelope),
            }
        )
    return records


def find_gaps(root: Path) -> list[dict[str, str]]:
    gaps: list[dict[str, str]] = []
    for control, location in REQUIRED_LOCAL_CONTROLS.items():
        path = root / location
        if path.exists():
            continue
        gaps.append(
            {
                "id": f"GAP-{control.upper().replace('_', '-')}",
                "severity": "major",
                "status": "open",
                "control": control,
                "message": f"Required local regulated control is missing: {location}",
            }
        )

    issue_template_dir = root / ".github/ISSUE_TEMPLATE"
    if issue_template_dir.exists():
        issue_forms = [
            path
            for path in issue_template_dir.glob("*.y*ml")
            if path.name.lower() != "config.yml"
        ]
        if not issue_forms:
            gaps.append(
                {
                    "id": "GAP-GITHUB-ISSUE-FORMS",
                    "severity": "major",
                    "status": "open",
                    "control": "issue_forms",
                    "message": "Issue template directory exists but contains no regulated YAML issue forms.",
                }
            )
    return gaps


def validate_alcoa_envelope(envelope: Any, path: str, findings: list[dict[str, str]]) -> None:
    if not isinstance(envelope, dict):
        findings.append(
            {
                "code": "MISSING_ALCOA_ENVELOPE",
                "severity": "error",
                "path": path,
                "message": "Domain evidence artifact is missing an ALCOA+ envelope.",
            }
        )
        envelope = {}

    for field_path in REQUIRED_ALCOA_ENVELOPE_PATHS:
        if not is_missing(nested_value(envelope, field_path)):
            continue
        findings.append(
            {
                "code": "MISSING_ALCOA_ENVELOPE_FIELD",
                "severity": "error",
                "path": f"{path}:{'.'.join(field_path)}",
                "message": f"Domain evidence artifact is missing required ALCOA+ field {'.'.join(field_path)}.",
            }
        )


def validate_domain_evidence(root: Path) -> list[dict[str, str]]:
    findings: list[dict[str, str]] = []
    evidence_root = root / DOMAIN_EVIDENCE_ROOT
    if not evidence_root.exists():
        return findings

    for path in sorted(evidence_root.glob("*.y*ml")):
        relative_path = rel(path, root)
        try:
            document = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
        except Exception as exc:  # noqa: BLE001 - parser detail belongs in CI output
            findings.append(
                {
                    "code": "DOMAIN_EVIDENCE_YAML_PARSE_FAILED",
                    "severity": "error",
                    "path": relative_path,
                    "message": f"Domain evidence artifact YAML parse failed: {exc}",
                }
            )
            continue
        if not isinstance(document, dict):
            findings.append(
                {
                    "code": "INVALID_DOMAIN_EVIDENCE_ARTIFACT",
                    "severity": "error",
                    "path": relative_path,
                    "message": "Domain evidence artifact must be a YAML object.",
                }
            )
            continue
        validate_alcoa_envelope(document.get("alcoa_envelope") or document.get("alcoa_plus"), relative_path, findings)
    return findings


def build_report(root: Path, command: str | None = None) -> dict[str, object]:
    generated_at_utc = utc_now()
    command = command or command_string([])
    commit = source_commit(root)
    records = collect_records(root, generated_at_utc=generated_at_utc, command=command, commit=commit)
    category_counts = Counter(str(record["category"]) for record in records)
    gaps = find_gaps(root)
    findings = validate_domain_evidence(root)
    return {
        "schema_version": "0.1.0",
        "status": "failed" if findings else "generated",
        "generated_at_utc": generated_at_utc,
        "generated_by": actor(),
        "generated_with": {
            "tool": TOOL_NAME,
            "tool_version": TOOL_VERSION,
            "command": command,
            "source_commit": commit,
        },
        "root": str(root.resolve()),
        "claim_boundary": CLAIM_BOUNDARY,
        "summary": {
            "records_hashed": len(records),
            "categories": dict(sorted(category_counts.items())),
            "blocking_gaps": len(gaps) + len(findings),
            "alcoa_envelope_findings": len(findings),
        },
        "records": records,
        "gaps": gaps,
        "findings": findings,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Generate a local regulated evidence pack.")
    parser.add_argument("--root", default=".", help="Repository root to scan.")
    parser.add_argument(
        "--output",
        default=".regulated-evidence-pack/evidence-pack.json",
        help="JSON report path.",
    )
    args = parser.parse_args()

    root = Path(args.root).resolve()
    output = Path(args.output)
    if not output.is_absolute():
        output = root / output
    output.parent.mkdir(parents=True, exist_ok=True)

    report = build_report(root, command=command_string(sys.argv[1:]))
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report["summary"], indent=2, sort_keys=True))
    return 1 if report["status"] == "failed" else 0


if __name__ == "__main__":
    raise SystemExit(main())

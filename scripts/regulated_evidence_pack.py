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
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path


CLAIM_BOUNDARY = "Evidence inventory only; no compliance certification."

EVIDENCE_PATTERNS = [
    ("regulated_document", "docs/regulated/**/*.md"),
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


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def rel(path: Path, root: Path) -> str:
    return path.resolve().relative_to(root.resolve()).as_posix()


def collect_records(root: Path) -> list[dict[str, object]]:
    seen: dict[Path, str] = {}
    for category, pattern in EVIDENCE_PATTERNS:
        for path in root.glob(pattern):
            if not path.is_file():
                continue
            seen.setdefault(path.resolve(), category)

    records: list[dict[str, object]] = []
    for path, category in sorted(seen.items(), key=lambda item: rel(item[0], root)):
        stat = path.stat()
        records.append(
            {
                "category": category,
                "path": rel(path, root),
                "sha256": sha256_file(path),
                "size_bytes": stat.st_size,
                "modified_at_utc": datetime.fromtimestamp(stat.st_mtime, timezone.utc)
                .isoformat()
                .replace("+00:00", "Z"),
                "alcoa_attributes": {
                    "attributable": actor(),
                    "legible": True,
                    "contemporaneous": True,
                    "original": rel(path, root),
                    "accurate": "sha256",
                    "complete": "inventory_record_only",
                    "consistent": "path_and_hash_sort_order",
                    "enduring": "requires_retention_policy",
                    "available": "repository_or_uploaded_artifact",
                },
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


def build_report(root: Path) -> dict[str, object]:
    records = collect_records(root)
    category_counts = Counter(str(record["category"]) for record in records)
    gaps = find_gaps(root)
    return {
        "schema_version": "0.1.0",
        "status": "generated",
        "generated_at_utc": utc_now(),
        "generated_by": actor(),
        "root": str(root.resolve()),
        "claim_boundary": CLAIM_BOUNDARY,
        "summary": {
            "records_hashed": len(records),
            "categories": dict(sorted(category_counts.items())),
            "blocking_gaps": len(gaps),
        },
        "records": records,
        "gaps": gaps,
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

    report = build_report(root)
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report["summary"], indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

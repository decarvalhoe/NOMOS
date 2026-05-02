#!/usr/bin/env python3
"""Build the Nomos canonical reference bible report.

Every entry in the external reference register is a Nomos bible prerequisite.
This tool classifies how each source may be processed without inventing
evidence or violating licensed-reference boundaries.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for reference canon validation.", file=sys.stderr)
    raise SystemExit(2) from exc


REGISTER_PATH = Path("docs/regulated/reference-basis/external-reference-register.yaml")
LICENSED_PUBLISHERS = ("ISO", "ISPE")
LICENSED_STATUS_MARKERS = (
    "licensed",
    "summary_reference_only",
)


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def load_yaml(path: Path) -> dict[str, Any]:
    return yaml.safe_load(path.read_text(encoding="utf-8")) or {}


def is_licensed_reference(reference: dict[str, Any]) -> bool:
    explicit = str(reference.get("content_access_policy", "")).lower()
    if explicit == "licensed_content_required":
        return True
    publisher = str(reference.get("publisher", "")).upper()
    status = str(reference.get("evidence_status", "")).lower()
    return any(marker in status for marker in LICENSED_STATUS_MARKERS) or any(
        marker in publisher for marker in LICENSED_PUBLISHERS
    )


def classify_reference(reference: dict[str, Any]) -> dict[str, Any]:
    licensed = is_licensed_reference(reference)
    access_policy = str(reference.get("content_access_policy") or "").strip()
    if not access_policy:
        access_policy = "licensed_content_required" if licensed else "official_public_reference"

    processing_policy = (
        "licensed_local_artifact_required"
        if access_policy == "licensed_content_required"
        else "official_snapshot_allowed_with_hash"
    )

    return {
        "id": reference.get("id", "unknown"),
        "title": reference.get("title", "unknown"),
        "publisher": reference.get("publisher", "unknown"),
        "url": reference.get("url", ""),
        "canonical_role": "nomos_bible",
        "content_access_policy": access_policy,
        "nomos_processing_policy": processing_policy,
        "full_text_fetch_allowed": access_policy != "licensed_content_required",
        "metadata_fetch_allowed": True,
        "evidence_status": reference.get("evidence_status", "requires_evidence"),
    }


def gap_for(reference: dict[str, Any], bible: dict[str, Any], licensed_root: Path | None) -> dict[str, str] | None:
    if bible["content_access_policy"] != "licensed_content_required":
        return None
    ref_id = str(reference.get("id", "unknown"))
    if licensed_root and (licensed_root / ref_id).exists():
        return None
    return {
        "id": f"GAP-LICENSED-REFERENCE-{ref_id}",
        "severity": "major",
        "status": "open",
        "reference_id": ref_id,
        "message": "Licensed canonical bible content is not present in the configured licensed reference root.",
    }


def build_report(root: Path, licensed_root: Path | None) -> dict[str, Any]:
    register_path = root / REGISTER_PATH
    if not register_path.exists():
        return {
            "schema_version": "0.1.0",
            "status": "failed",
            "generated_at_utc": utc_now(),
            "claim_boundary": "Reference canon only; no compliance certification.",
            "findings": [
                {
                    "severity": "error",
                    "path": REGISTER_PATH.as_posix(),
                    "message": "External reference register is missing.",
                }
            ],
        }

    register = load_yaml(register_path)
    references = register.get("references") or []
    bible_policy = register.get("nomos_bible_policy") or {}
    findings: list[dict[str, str]] = []

    if bible_policy.get("all_registered_references_are_canonical") is not True:
        findings.append(
            {
                "severity": "error",
                "path": REGISTER_PATH.as_posix(),
                "message": "nomos_bible_policy.all_registered_references_are_canonical must be true.",
            }
        )

    bibles = [classify_reference(reference) for reference in references]
    gaps = [
        gap
        for reference, bible in zip(references, bibles, strict=False)
        if (gap := gap_for(reference, bible, licensed_root)) is not None
    ]
    policy_counts = Counter(str(bible["content_access_policy"]) for bible in bibles)

    status = "failed" if findings else "requires_evidence" if gaps else "ready_for_processing"
    return {
        "schema_version": "0.1.0",
        "status": status,
        "generated_at_utc": utc_now(),
        "claim_boundary": "Reference canon only; no compliance certification.",
        "register": REGISTER_PATH.as_posix(),
        "licensed_reference_root": str(licensed_root) if licensed_root else "not_configured",
        "summary": {
            "canonical_bibles": len(bibles),
            "content_access_policies": dict(sorted(policy_counts.items())),
            "licensed_reference_gaps": len(gaps),
        },
        "bibles": bibles,
        "gaps": gaps,
        "findings": findings,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Build Nomos canonical reference bible report.")
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument(
        "--licensed-root",
        default=os.environ.get("NOMOS_LICENSED_REFERENCE_ROOT"),
        help="Optional local root containing licensed references by reference id.",
    )
    parser.add_argument(
        "--report",
        default=".regulated-doc-gate/reference-canon-report.json",
        help="JSON report path.",
    )
    parser.add_argument("--strict", action="store_true", help="Return non-zero unless ready for processing.")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    licensed_root = Path(args.licensed_root).resolve() if args.licensed_root else None
    report_path = Path(args.report)
    if not report_path.is_absolute():
        report_path = root / report_path
    report_path.parent.mkdir(parents=True, exist_ok=True)

    report = build_report(root, licensed_root)
    report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({"status": report["status"], "summary": report.get("summary", {})}, indent=2, sort_keys=True))
    if report["status"] == "failed":
        return 1
    if args.strict and report["status"] != "ready_for_processing":
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

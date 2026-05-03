#!/usr/bin/env python3
"""Build the Nomos canonical reference bible report.

Every entry in the external reference register is a Nomos bible prerequisite.
This tool classifies how each source may be processed without inventing
evidence or violating licensed-reference boundaries.
"""

from __future__ import annotations

import argparse
import hashlib
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
PUBLIC_SURROGATE_PATH = Path("docs/regulated/reference-basis/public-surrogate-annexes")
LICENSED_PUBLISHERS = ("ISO", "ISPE")
LICENSED_STATUS_MARKERS = (
    "licensed",
    "summary_reference_only",
)
VALID_PUBLIC_SURROGATE_STATUSES = (
    "temporary_surrogate_until_official_document_acquired",
    "temporary_surrogate",
)
APPROVED_LICENSE_REVIEW_STATUSES = (
    "approved",
    "approved_for_internal_nomos_processing",
)
APPROVED_INTERNAL_PROCESSING_VALUES = (
    "approved",
    "approved_internal_only",
    "approved_for_internal_nomos_processing",
    "allowed_internal_processing",
    "allowed_with_restrictions",
)


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def load_yaml(path: Path) -> dict[str, Any]:
    return yaml.safe_load(path.read_text(encoding="utf-8")) or {}


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest().upper()


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


def intake_path(root: Path, ref_id: str) -> Path:
    return root / "docs/regulated/reference-basis/licensed-intakes" / f"{ref_id}.yaml"


def public_surrogate_path(root: Path, ref_id: str) -> Path:
    return root / PUBLIC_SURROGATE_PATH / f"{ref_id}.yaml"


def load_public_surrogate(root: Path, ref_id: str) -> tuple[dict[str, Any] | None, dict[str, str] | None]:
    sidecar = public_surrogate_path(root, ref_id)
    if not sidecar.exists():
        return None, None

    annex = load_yaml(sidecar)
    if str(annex.get("surrogate_for", "")).strip() != ref_id:
        return None, {
            "severity": "error",
            "path": sidecar.as_posix(),
            "message": "Public surrogate annex surrogate_for must match the licensed reference id.",
        }
    if str(annex.get("status", "")).strip() not in VALID_PUBLIC_SURROGATE_STATUSES:
        return None, {
            "severity": "error",
            "path": sidecar.as_posix(),
            "message": "Public surrogate annex has an unsupported status.",
        }
    if not str(annex.get("claim_boundary", "")).strip():
        return None, {
            "severity": "error",
            "path": sidecar.as_posix(),
            "message": "Public surrogate annex must declare a claim_boundary.",
        }
    sources = annex.get("sources") or []
    if not isinstance(sources, list) or not sources:
        return None, {
            "severity": "error",
            "path": sidecar.as_posix(),
            "message": "Public surrogate annex must list at least one public source.",
        }

    return (
        {
            "public_surrogate_status": "available",
            "surrogate_sidecar": sidecar.as_posix(),
            "surrogate_processing_allowed": True,
            "surrogate_source_count": len(sources),
            "surrogate_claim_boundary": str(annex.get("claim_boundary", "")).strip(),
            "blocked_claims_until_licensed_intake": annex.get("blocked_claims") or [],
        },
        None,
    )


def verify_licensed_artifact(
    root: Path,
    ref_id: str,
    licensed_root: Path | None,
) -> tuple[dict[str, Any], dict[str, str] | None]:
    if licensed_root is None:
        return (
            {"licensed_artifact_status": "requires_licensed_root"},
            {
                "id": f"GAP-LICENSED-REFERENCE-{ref_id}",
                "severity": "major",
                "status": "open",
                "reference_id": ref_id,
                "message": "Licensed canonical bible content is not present in the configured licensed reference root.",
            },
        )

    sidecar = intake_path(root, ref_id)
    if not sidecar.exists():
        return (
            {"licensed_artifact_status": "missing_intake_sidecar"},
            {
                "id": f"GAP-LICENSED-INTAKE-{ref_id}",
                "severity": "major",
                "status": "open",
                "reference_id": ref_id,
                "message": "Licensed canonical bible intake sidecar is missing.",
            },
        )

    intake = load_yaml(sidecar)
    local_relative_path = str((intake.get("storage") or {}).get("local_relative_path") or "").strip()
    expected_hash = str((intake.get("source_integrity") or {}).get("sha256") or "").upper().strip()
    artifact = licensed_root / local_relative_path

    if not local_relative_path or not expected_hash:
        return (
            {"licensed_artifact_status": "invalid_intake_sidecar", "intake_sidecar": sidecar.as_posix()},
            {
                "id": f"GAP-LICENSED-INTAKE-INCOMPLETE-{ref_id}",
                "severity": "major",
                "status": "open",
                "reference_id": ref_id,
                "message": "Licensed intake sidecar must include storage.local_relative_path and source_integrity.sha256.",
            },
        )

    if not artifact.exists():
        return (
            {
                "licensed_artifact_status": "missing_artifact",
                "intake_sidecar": sidecar.as_posix(),
                "expected_artifact": str(artifact),
            },
            {
                "id": f"GAP-LICENSED-ARTIFACT-{ref_id}",
                "severity": "major",
                "status": "open",
                "reference_id": ref_id,
                "message": "Licensed canonical bible artifact path from intake sidecar is missing.",
            },
        )

    actual_hash = sha256_file(artifact)
    if actual_hash != expected_hash:
        return (
            {
                "licensed_artifact_status": "hash_mismatch",
                "intake_sidecar": sidecar.as_posix(),
                "expected_sha256": expected_hash,
                "actual_sha256": actual_hash,
            },
            {
                "id": f"GAP-LICENSED-HASH-MISMATCH-{ref_id}",
                "severity": "critical",
                "status": "open",
                "reference_id": ref_id,
                "message": "Licensed canonical bible artifact hash does not match the intake sidecar.",
            },
        )

    verification = {
        "licensed_artifact_status": "verified",
        "intake_sidecar": sidecar.as_posix(),
        "artifact_relative_path": local_relative_path,
        "sha256": actual_hash,
    }
    review_status, review_gap = verify_license_review(intake, ref_id, sidecar)
    verification.update(review_status)
    if review_gap is not None:
        verification["licensed_artifact_status"] = "verified_license_review_required"
        return verification, review_gap

    return (
        verification,
        None,
    )


def verify_license_review(
    intake: dict[str, Any],
    ref_id: str,
    sidecar: Path,
) -> tuple[dict[str, Any], dict[str, str] | None]:
    review = intake.get("review") or {}
    allowed_use = intake.get("allowed_use") or {}
    reviewer = str(review.get("reviewer") or "").strip()
    approval_status = str(review.get("approval_status") or "").strip()
    internal_processing = str(allowed_use.get("internal_processing_by_nomos") or "").strip()
    commit_full_text = allowed_use.get("commit_full_text_to_git")
    customer_redistribution = allowed_use.get("customer_redistribution")

    status = {
        "license_review_status": approval_status or "missing",
        "license_reviewer": reviewer or "missing",
        "licensed_internal_processing": internal_processing or "missing",
        "commit_full_text_to_git": commit_full_text,
        "customer_redistribution": customer_redistribution,
        "license_review_verified": False,
    }

    missing = reviewer == "" or reviewer == "not_assigned"
    if missing or approval_status not in APPROVED_LICENSE_REVIEW_STATUSES:
        return (
            status,
            {
                "id": f"GAP-LICENSE-REVIEW-{ref_id}",
                "severity": "major",
                "status": "open",
                "reference_id": ref_id,
                "message": (
                    "Licensed artifact hash is verified, but the license review is not approved "
                    "by an assigned reviewer."
                ),
                "sidecar": sidecar.as_posix(),
            },
        )

    if internal_processing not in APPROVED_INTERNAL_PROCESSING_VALUES:
        return (
            status,
            {
                "id": f"GAP-LICENSE-USE-{ref_id}",
                "severity": "major",
                "status": "open",
                "reference_id": ref_id,
                "message": (
                    "Licensed artifact hash is verified, but allowed_use.internal_processing_by_nomos "
                    "does not authorize internal Nomos processing."
                ),
                "sidecar": sidecar.as_posix(),
            },
        )

    if commit_full_text is not False or customer_redistribution is not False:
        return (
            status,
            {
                "id": f"GAP-LICENSE-SAFETY-{ref_id}",
                "severity": "critical",
                "status": "open",
                "reference_id": ref_id,
                "message": (
                    "Licensed sidecar must explicitly forbid committing full text to Git and "
                    "customer redistribution before Nomos processing is allowed."
                ),
                "sidecar": sidecar.as_posix(),
            },
        )

    return (
        {
            **status,
            "license_review_verified": True,
        },
        None,
    )


def gap_for(
    root: Path,
    reference: dict[str, Any],
    bible: dict[str, Any],
    licensed_root: Path | None,
    allow_public_surrogates: bool,
) -> tuple[dict[str, str] | None, dict[str, str] | None]:
    if bible["content_access_policy"] != "licensed_content_required":
        return None, None
    ref_id = str(reference.get("id", "unknown"))
    verification, gap = verify_licensed_artifact(root, ref_id, licensed_root)
    bible.update(verification)
    if gap is None:
        return None, None

    surrogate, finding = load_public_surrogate(root, ref_id)
    if finding is not None:
        return gap, finding
    if surrogate is not None:
        bible.update(surrogate)
        if allow_public_surrogates:
            bible["nomos_processing_policy"] = "public_surrogate_annex_allowed_until_licensed_intake"
            gap["status"] = "temporarily_mitigated"
            gap["mitigation"] = "public_surrogate_annex"
            gap["surrogate_sidecar"] = str(surrogate["surrogate_sidecar"])
            gap["message"] = (
                "Licensed canonical bible intake remains missing, but a public surrogate annex "
                "is explicitly enabled for temporary non-clause-level processing."
            )
        else:
            bible["public_surrogate_status"] = "available_but_not_enabled"
            bible["surrogate_processing_allowed"] = False
    return gap, None


def build_report(
    root: Path,
    licensed_root: Path | None,
    allow_public_surrogates: bool = False,
) -> dict[str, Any]:
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
    gaps = []
    for reference, bible in zip(references, bibles, strict=False):
        gap, finding = gap_for(root, reference, bible, licensed_root, allow_public_surrogates)
        if gap is not None:
            gaps.append(gap)
        if finding is not None:
            findings.append(finding)
    policy_counts = Counter(str(bible["content_access_policy"]) for bible in bibles)
    surrogate_mitigations = sum(1 for gap in gaps if gap.get("status") == "temporarily_mitigated")
    unmitigated_gaps = len(gaps) - surrogate_mitigations

    status = (
        "failed"
        if findings
        else "requires_evidence"
        if unmitigated_gaps
        else "surrogate_ready_for_processing"
        if surrogate_mitigations
        else "ready_for_processing"
    )
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
            "surrogate_mitigations": surrogate_mitigations,
            "unmitigated_licensed_reference_gaps": unmitigated_gaps,
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
    parser.add_argument(
        "--allow-public-surrogates",
        action="store_true",
        help="Allow valid public surrogate annexes to temporarily unblock non-clause-level processing.",
    )
    parser.add_argument("--strict", action="store_true", help="Return non-zero unless ready for processing.")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    licensed_root = Path(args.licensed_root).resolve() if args.licensed_root else None
    report_path = Path(args.report)
    if not report_path.is_absolute():
        report_path = root / report_path
    report_path.parent.mkdir(parents=True, exist_ok=True)

    report = build_report(root, licensed_root, allow_public_surrogates=args.allow_public_surrogates)
    report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps({"status": report["status"], "summary": report.get("summary", {})}, indent=2, sort_keys=True))
    if report["status"] == "failed":
        return 1
    strict_ready_statuses = {"ready_for_processing"}
    if args.allow_public_surrogates:
        strict_ready_statuses.add("surrogate_ready_for_processing")
    if args.strict and report["status"] not in strict_ready_statuses:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

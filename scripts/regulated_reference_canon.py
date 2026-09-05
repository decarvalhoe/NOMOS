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
import re
import os
import subprocess
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
SOURCE_CLASSES = ("public", "licensed", "private", "confidential", "customer_owned")
RESTRICTED_SOURCE_CLASSES = ("licensed", "private", "confidential", "customer_owned")
ACCESS_POLICY_SOURCE_CLASSES = {
    "official_public_reference": "public",
    "public_reference": "public",
    "licensed_content_required": "licensed",
    "private_reference_only": "private",
    "private_content_required": "private",
    "confidential_reference_only": "confidential",
    "confidential_content_required": "confidential",
    "customer_owned_confidential": "customer_owned",
    "customer_owned_reference": "customer_owned",
}
SOURCE_CLASS_ACCESS_POLICIES = {
    "public": "official_public_reference",
    "licensed": "licensed_content_required",
    "private": "private_reference_only",
    "confidential": "confidential_reference_only",
    "customer_owned": "customer_owned_confidential",
}
SOURCE_CLASS_PROCESSING_POLICIES = {
    "public": "official_snapshot_allowed_with_hash",
    "licensed": "licensed_local_artifact_required",
    "private": "read_only_local_artifact_required",
    "confidential": "read_only_local_artifact_required",
    "customer_owned": "read_only_local_artifact_required",
}
SOURCE_CLASS_CONFIDENTIALITY = {
    "public": "public",
    "licensed": "licensed_restricted",
    "private": "private_restricted",
    "confidential": "confidential_restricted",
    "customer_owned": "customer_confidential",
}
SOURCE_CLASS_RETENTION = {
    "public": "public_snapshot_retained_with_hash",
    "licensed": "license_terms",
    "private": "owner_policy",
    "confidential": "confidentiality_agreement",
    "customer_owned": "customer_contract",
}
REDISTRIBUTION_KEYS = {
    "customer_redistribution",
    "full_text_redistribution",
    "full_text_redistribution_allowed",
    "redistribution",
    "redistribution_allowed",
}
COMMIT_FULL_TEXT_KEYS = {
    "commit_full_text",
    "commit_full_text_allowed",
    "commit_full_text_to_git",
    "full_text_commit",
    "full_text_commit_allowed",
}
ALLOWED_VALUES = {"allow", "allowed", "permitted", "true", "yes"}

# #641 — licence review. A verified artifact hash proves WHAT is on disk; it
# says nothing about whether NOMOS may process it. These values are the ones an
# assigned reviewer may record to say that it may. Restored from 570ed58, which
# never reached main.
APPROVED_LICENSE_REVIEW_STATUSES = ("approved", "approved_for_internal_nomos_processing")
APPROVED_INTERNAL_PROCESSING_VALUES = (
    "approved",
    "approved_internal_only",
    "approved_for_internal_nomos_processing",
    "allowed_internal_processing",
    "allowed_with_restrictions",
)
NO_FULL_TEXT_POLICY_PATH = Path("docs/regulated/reference-basis/no-full-text-policy.yaml")
NO_FULL_TEXT_POLICY_SCHEMA = "nomos-no-full-text-policy-v1"
_SENTENCE_SPLIT = re.compile(r"(?<=[.!?])\s+")
_NON_WORD = re.compile(r"[^a-z0-9\s]")
_WS = re.compile(r"\s+")


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


def as_mapping(value: Any) -> dict[str, Any]:
    return value if isinstance(value, dict) else {}


def normalized_text(value: Any) -> str:
    return str(value or "").strip().lower()


def value_allows_full_text(value: Any) -> bool:
    if isinstance(value, bool):
        return value
    return normalized_text(value) in ALLOWED_VALUES


def infer_source_class(reference: dict[str, Any], access_policy: str, licensed: bool) -> str:
    classification = as_mapping(reference.get("reference_classification"))
    requested_source_class = normalized_text(classification.get("source_class"))
    if requested_source_class in SOURCE_CLASSES:
        return requested_source_class
    if access_policy in ACCESS_POLICY_SOURCE_CLASSES:
        return ACCESS_POLICY_SOURCE_CLASSES[access_policy]
    return "licensed" if licensed else "public"


def normalized_full_text_redistribution(classification: dict[str, Any], source_class: str) -> str:
    raw_value = normalized_text(classification.get("full_text_redistribution"))
    if raw_value:
        return raw_value
    if "full_text_redistribution_allowed" in classification:
        return "allowed" if value_allows_full_text(classification.get("full_text_redistribution_allowed")) else "forbidden"
    return "source_terms_only" if source_class == "public" else "forbidden"


def classify_reference(reference: dict[str, Any]) -> dict[str, Any]:
    licensed = is_licensed_reference(reference)
    access_policy = str(reference.get("content_access_policy") or "").strip()
    classification = as_mapping(reference.get("reference_classification"))
    if not access_policy:
        access_policy = "licensed_content_required" if licensed else "official_public_reference"

    source_class = infer_source_class(reference, access_policy, licensed)
    if access_policy not in ACCESS_POLICY_SOURCE_CLASSES:
        access_policy = SOURCE_CLASS_ACCESS_POLICIES[source_class]

    processing_policy = SOURCE_CLASS_PROCESSING_POLICIES[source_class]
    confidentiality = str(classification.get("confidentiality") or SOURCE_CLASS_CONFIDENTIALITY[source_class]).strip()
    full_text_redistribution = normalized_full_text_redistribution(classification, source_class)
    retention_obligation = str(classification.get("retention_obligation") or SOURCE_CLASS_RETENTION[source_class]).strip()
    classification_processing_mode = str(classification.get("processing_mode") or processing_policy).strip()
    full_text_redistribution_allowed = value_allows_full_text(full_text_redistribution)

    return {
        "id": reference.get("id", "unknown"),
        "title": reference.get("title", "unknown"),
        "publisher": reference.get("publisher", "unknown"),
        "url": reference.get("url", ""),
        "canonical_role": "nomos_bible",
        "content_access_policy": access_policy,
        "access_policy": access_policy,
        "source_class": source_class,
        "confidentiality": confidentiality,
        "nomos_processing_policy": processing_policy,
        "classification_processing_mode": classification_processing_mode,
        "full_text_fetch_allowed": source_class == "public",
        "full_text_redistribution": full_text_redistribution,
        "full_text_redistribution_allowed": full_text_redistribution_allowed,
        "metadata_fetch_allowed": True,
        "retention_obligation": retention_obligation,
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


def forbidden_full_text_permissions(value: Any, prefix: str = "") -> list[str]:
    violations: list[str] = []
    if isinstance(value, dict):
        for key, nested_value in value.items():
            nested_prefix = f"{prefix}.{key}" if prefix else str(key)
            normalized_key = normalized_text(key)
            if normalized_key in REDISTRIBUTION_KEYS | COMMIT_FULL_TEXT_KEYS and value_allows_full_text(nested_value):
                violations.append(nested_prefix)
            violations.extend(forbidden_full_text_permissions(nested_value, nested_prefix))
    elif isinstance(value, list):
        for index, nested_value in enumerate(value):
            violations.extend(forbidden_full_text_permissions(nested_value, f"{prefix}[{index}]"))
    return violations


def validate_reference_policy(reference: dict[str, Any], bible: dict[str, Any]) -> list[dict[str, str]]:
    findings: list[dict[str, str]] = []
    ref_id = str(reference.get("id", "unknown"))
    source_class = str(bible.get("source_class", "public"))
    classification = as_mapping(reference.get("reference_classification"))
    requested_source_class = normalized_text(classification.get("source_class"))
    if requested_source_class and requested_source_class not in SOURCE_CLASSES:
        findings.append(
            {
                "id": "REFERENCE_CLASSIFICATION_UNSUPPORTED_SOURCE_CLASS",
                "severity": "error",
                "reference_id": ref_id,
                "path": f"{REGISTER_PATH.as_posix()}:{ref_id}:reference_classification.source_class",
                "message": "Reference classification source_class must be public, licensed, private, confidential, or customer_owned.",
            }
        )
    if source_class in RESTRICTED_SOURCE_CLASSES and bible.get("full_text_redistribution_allowed"):
        findings.append(
            {
                "id": "REFERENCE_FULL_TEXT_REDISTRIBUTION_FORBIDDEN",
                "severity": "error",
                "reference_id": ref_id,
                "path": f"{REGISTER_PATH.as_posix()}:{ref_id}:reference_classification.full_text_redistribution",
                "message": "Licensed, private, confidential, and customer-owned references cannot authorize full-text redistribution.",
            }
        )
    if source_class in ("private", "confidential", "customer_owned") and not str(
        bible.get("retention_obligation", "")
    ).strip():
        findings.append(
            {
                "id": "REFERENCE_RETENTION_OBLIGATION_REQUIRED",
                "severity": "error",
                "reference_id": ref_id,
                "path": f"{REGISTER_PATH.as_posix()}:{ref_id}:reference_classification.retention_obligation",
                "message": "Private, confidential, and customer-owned references must declare retention obligations.",
            }
        )
    return findings


def validate_intake_policy(root: Path, ref_id: str, source_class: str) -> list[dict[str, str]]:
    if source_class not in RESTRICTED_SOURCE_CLASSES:
        return []

    sidecar = intake_path(root, ref_id)
    if not sidecar.exists():
        return []

    violations = forbidden_full_text_permissions(load_yaml(sidecar))
    if not violations:
        return []

    return [
        {
            "id": "REFERENCE_FULL_TEXT_REDISTRIBUTION_FORBIDDEN",
            "severity": "error",
            "reference_id": ref_id,
            "path": f"{sidecar.as_posix()}:{violation}",
            "message": "Restricted reference intake must not authorize full-text redistribution or committed full text.",
        }
        for violation in violations
    ]


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

    verification: dict[str, Any] = {
        "licensed_artifact_status": "verified",
        "intake_sidecar": sidecar.as_posix(),
        "artifact_relative_path": local_relative_path,
        "sha256": actual_hash,
        # #641 — the two facts are kept apart on purpose. A matching hash is
        # integrity; permission to process is a human decision recorded below.
        "artifact_hash_verified": True,
        "license_use_approved": False,
    }
    review_status, review_gap = verify_license_review(intake, ref_id, sidecar)
    verification.update(review_status)
    if review_gap is not None:
        verification["licensed_artifact_status"] = "verified_license_review_required"
        return verification, review_gap
    verification["license_use_approved"] = True
    return verification, None


def verify_license_review(
    intake: dict[str, Any],
    ref_id: str,
    sidecar: Path,
) -> tuple[dict[str, Any], dict[str, str] | None]:
    """The review invariants that a verified hash alone never satisfies.

    Order matters and is the order a reviewer works in: someone must be
    assigned and approve; the approval must authorise internal processing; and
    the sidecar must explicitly forbid the two things that would make
    processing unsafe regardless of approval. The gate records the decision; it
    does not make it (#194 is a human act).
    """
    review = intake.get("review") or {}
    allowed_use = intake.get("allowed_use") or {}
    reviewer = str(review.get("reviewer") or "").strip()
    approval_status = str(review.get("approval_status") or "").strip()
    internal_processing = str(allowed_use.get("internal_processing_by_nomos") or "").strip()
    commit_full_text = allowed_use.get("commit_full_text_to_git")
    customer_redistribution = allowed_use.get("customer_redistribution")

    status: dict[str, Any] = {
        "license_review_status": approval_status or "missing",
        "license_reviewer": reviewer or "missing",
        "licensed_internal_processing": internal_processing or "missing",
        "commit_full_text_to_git": commit_full_text,
        "customer_redistribution": customer_redistribution,
        "license_review_verified": False,
    }

    if reviewer in ("", "not_assigned") or approval_status not in APPROVED_LICENSE_REVIEW_STATUSES:
        return status, {
            "id": f"GAP-LICENSE-REVIEW-{ref_id}",
            "severity": "major",
            "status": "open",
            "reference_id": ref_id,
            "message": "Licensed artifact hash is verified, but the license review is not approved by an assigned reviewer.",
            "sidecar": sidecar.as_posix(),
        }
    if internal_processing not in APPROVED_INTERNAL_PROCESSING_VALUES:
        return status, {
            "id": f"GAP-LICENSE-USE-{ref_id}",
            "severity": "major",
            "status": "open",
            "reference_id": ref_id,
            "message": "Licensed artifact hash is verified, but allowed_use.internal_processing_by_nomos does not authorize internal Nomos processing.",
            "sidecar": sidecar.as_posix(),
        }
    if commit_full_text is not False or customer_redistribution is not False:
        return status, {
            "id": f"GAP-LICENSE-SAFETY-{ref_id}",
            "severity": "critical",
            "status": "open",
            "reference_id": ref_id,
            "message": "Licensed sidecar must explicitly forbid committing full text to Git and customer redistribution before Nomos processing is allowed.",
            "sidecar": sidecar.as_posix(),
        }
    return {**status, "license_review_verified": True}, None


# --- #641: protected text actually present ---------------------------------

def load_no_full_text_policy(root: Path) -> dict[str, Any] | None:
    path = root / NO_FULL_TEXT_POLICY_PATH
    if not path.exists():
        return None
    policy = load_yaml(path)
    if policy.get("schema_version") != NO_FULL_TEXT_POLICY_SCHEMA:
        return None
    return policy


def normalize_sentence(text: str) -> str:
    return _WS.sub(" ", _NON_WORD.sub(" ", text.lower())).strip()


def sentinel_digest(sentence: str) -> str:
    return "sha256:" + hashlib.sha256(normalize_sentence(sentence).encode("utf-8")).hexdigest()


def staged_paths(root: Path) -> list[Path]:
    """Files staged for commit, anywhere in the repo. Empty when not a git tree."""
    try:
        out = subprocess.run(
            ["git", "diff", "--cached", "--name-only", "--diff-filter=AM"],
            cwd=root, capture_output=True, text=True, check=True, timeout=60,
        ).stdout
    except (OSError, subprocess.SubprocessError):
        return []
    return [root / line.strip() for line in out.splitlines() if line.strip()]


def scan_committed_full_text(
    root: Path,
    policy: dict[str, Any],
    licensed_hashes: dict[str, str],
    include_staged: bool = False,
) -> tuple[list[dict[str, str]], dict[str, Any]]:
    """Look for licensed text that is REALLY in public trees (or staged).

    Returns findings (errors) and a coverage summary. The summary says how many
    sentinels are registered, because zero sentinels means the sentence check
    covered nothing — and that must read as uncovered, never as clean.
    """
    findings: list[dict[str, str]] = []
    sentinels: dict[str, str] = {}
    uncovered: list[str] = []
    for ref in policy.get("references") or []:
        ref_id = str(ref.get("reference_id", "unknown"))
        digests = [str(d).lower() for d in (ref.get("sentinels") or [])]
        if not digests:
            uncovered.append(ref_id)
        for d in digests:
            sentinels[d] = ref_id

    suffixes = tuple(policy.get("text_suffixes") or [])
    min_chars = int(policy.get("min_sentence_chars") or 60)
    policy_file = (root / NO_FULL_TEXT_POLICY_PATH).resolve()

    candidates: list[Path] = []
    for tree in policy.get("public_trees") or []:
        base = root / str(tree)
        if base.is_file():
            candidates.append(base)
        elif base.is_dir():
            candidates.extend(p for p in base.rglob("*") if p.is_file())
    if include_staged:
        candidates.extend(p for p in staged_paths(root) if p.is_file())

    scanned = 0
    seen: set[Path] = set()
    for path in candidates:
        rp = path.resolve()
        if rp in seen or rp == policy_file or ".git" in rp.parts:
            continue
        seen.add(rp)
        scanned += 1
        rel = rp.relative_to(root.resolve()).as_posix() if rp.is_relative_to(root.resolve()) else rp.as_posix()

        # 1. full copy of a registered licensed artifact
        digest = sha256_file(rp).upper()
        if digest in licensed_hashes:
            findings.append({
                "id": "LICENSED_FULL_TEXT_COMMITTED",
                "severity": "error",
                "reference_id": licensed_hashes[digest],
                "path": rel,
                "message": "A registered licensed artifact is present byte-for-byte in a public tree.",
            })
            continue

        # 2. sentinel sentences
        if not sentinels or rp.suffix.lower() not in suffixes:
            continue
        try:
            text = rp.read_text(encoding="utf-8")
        except (OSError, UnicodeDecodeError):
            continue
        for sentence in _SENTENCE_SPLIT.split(text):
            if len(sentence) < min_chars:
                continue
            d = sentinel_digest(sentence)
            if d in sentinels:
                findings.append({
                    "id": "LICENSED_TEXT_PRESENT",
                    "severity": "error",
                    "reference_id": sentinels[d],
                    "path": rel,
                    "message": "A registered sentinel sentence of a licensed reference is present in a public tree.",
                })
                break

    return findings, {
        "files_scanned": scanned,
        "sentinels_registered": len(sentinels),
        "references_without_sentinels": sorted(uncovered),
        "coverage": "sentinels_registered" if sentinels else "uncovered_no_sentinels_registered",
        "staged_included": include_staged,
    }


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
    include_staged: bool = False,
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
    # #641 — hashes of every registered licensed artifact, for full-copy detection.
    licensed_hashes: dict[str, str] = {}
    for reference in references:
        ref_id = str(reference.get("id", "unknown"))
        sidecar = intake_path(root, ref_id)
        if sidecar.exists():
            h = str((load_yaml(sidecar).get("source_integrity") or {}).get("sha256") or "").upper().strip()
            if h:
                licensed_hashes[h] = ref_id
    for reference, bible in zip(references, bibles, strict=False):
        ref_id = str(reference.get("id", "unknown"))
        findings.extend(validate_reference_policy(reference, bible))
        findings.extend(validate_intake_policy(root, ref_id, str(bible.get("source_class", ""))))
        gap, finding = gap_for(root, reference, bible, licensed_root, allow_public_surrogates)
        if gap is not None:
            gaps.append(gap)
        if finding is not None:
            findings.append(finding)
    no_full_text: dict[str, Any] = {"coverage": "policy_missing"}
    policy = load_no_full_text_policy(root)
    if policy is not None:
        text_findings, no_full_text = scan_committed_full_text(root, policy, licensed_hashes, include_staged)
        findings.extend(text_findings)

    policy_counts = Counter(str(bible["content_access_policy"]) for bible in bibles)
    source_class_counts = Counter(str(bible["source_class"]) for bible in bibles)
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
            "source_classes": dict(sorted(source_class_counts.items())),
            "licensed_reference_gaps": len(gaps),
            "surrogate_mitigations": surrogate_mitigations,
            "unmitigated_licensed_reference_gaps": unmitigated_gaps,
            "licensed_use_approved": sum(1 for b in bibles if b.get("license_use_approved") is True),
            "licensed_hash_verified_only": sum(
                1 for b in bibles if b.get("artifact_hash_verified") is True and not b.get("license_use_approved")
            ),
        },
        "no_full_text": no_full_text,
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
    parser.add_argument(
        "--staged",
        action="store_true",
        help="Also scan files staged for commit (anywhere) for licensed text.",
    )
    args = parser.parse_args()

    root = Path(args.root).resolve()
    licensed_root = Path(args.licensed_root).resolve() if args.licensed_root else None
    report_path = Path(args.report)
    if not report_path.is_absolute():
        report_path = root / report_path
    report_path.parent.mkdir(parents=True, exist_ok=True)

    report = build_report(root, licensed_root, allow_public_surrogates=args.allow_public_surrogates, include_staged=args.staged)
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

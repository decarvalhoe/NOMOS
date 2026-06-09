from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Any

import yaml


def load_yaml(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        loaded = yaml.safe_load(handle)
    if not isinstance(loaded, dict):
        raise SystemExit(f"{path} must contain a YAML object")
    return loaded


def facets_for(atom: dict[str, Any]) -> dict[str, Any]:
    metadata = atom.get("metadata") or {}
    facets = metadata.get("facets") or {}
    return facets if isinstance(facets, dict) else {}


def promotion_for(atom: dict[str, Any]) -> dict[str, Any]:
    metadata = atom.get("metadata") or {}
    promotion = metadata.get("canon_promotion") or {}
    return promotion if isinstance(promotion, dict) else {}


def add_finding(findings: list[dict[str, str]], code: str, target: str) -> None:
    findings.append({"code": code, "target": target})


def validate(bundle: dict[str, Any]) -> dict[str, Any]:
    findings: list[dict[str, str]] = []
    source = bundle.get("source") or {}
    source_id = str(source.get("source_id") or "")
    access_policy = str(source.get("access_policy") or "")
    silo_id = str(source.get("silo_id") or "")
    shared_sources = set(bundle.get("shared_catalog", {}).get("exported_source_ids") or [])
    silo_sources = list(bundle.get("silo_catalog", {}).get("source_ids") or [])
    certificates = {item.get("cert_id"): item for item in bundle.get("certificates", [])}

    if access_policy == "customer_confidential" and source_id in shared_sources:
        add_finding(findings, "customer_confidential_source_exposed", source_id)

    promoted_atoms: dict[str, dict[str, str]] = {}
    for atom in bundle.get("atoms", []):
        atom_id = str(atom.get("atom_id") or "")
        facets = facets_for(atom)
        promotion = promotion_for(atom)
        certificate_id = promotion.get("certificate_id")
        certificate = certificates.get(certificate_id)

        if atom.get("review_state") != "approved":
            add_finding(findings, "review_state_must_be_approved", atom_id)
        if facets.get("provenance") != "user_promoted":
            add_finding(findings, "promoted_atom_requires_user_promoted_provenance", atom_id)
        if facets.get("trust_tier") == "certified":
            add_finding(findings, "user_promoted_cannot_be_certified", atom_id)
        if facets.get("confidentiality") == "customer_confidential":
            if promotion.get("surfacing") != "silo_only" or promotion.get("shared_catalog") is not False:
                add_finding(findings, "customer_confidential_atom_must_remain_siloed", atom_id)
        if promotion.get("source_id") != source_id:
            add_finding(findings, "promotion_source_mismatch", atom_id)
        if promotion.get("silo_id") != silo_id:
            add_finding(findings, "promotion_silo_mismatch", atom_id)
        if certificate is None:
            add_finding(findings, "certificate_required", atom_id)
        elif certificate.get("revoked"):
            add_finding(findings, "certificate_revoked", str(certificate_id))

        promoted_atoms[atom_id] = {
            "provenance": str(facets.get("provenance") or ""),
            "trust_tier": str(facets.get("trust_tier") or ""),
            "confidentiality": str(facets.get("confidentiality") or ""),
            "certificate_id": str(certificate_id or ""),
        }

    return {
        "status": "pass" if not findings else "fail",
        "findings": findings,
        "shared_catalog_exposed_source_ids": sorted(source_id for source_id in shared_sources if source_id),
        "siloed_source_ids": silo_sources,
        "promoted_atoms": promoted_atoms,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--bundle", required=True, type=Path)
    args = parser.parse_args()

    report = validate(load_yaml(args.bundle))
    print(json.dumps(report, indent=2, sort_keys=True))
    return 0 if report["status"] == "pass" else 1


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import sys
from collections import defaultdict
from pathlib import Path
from typing import Any

import yaml


def load_yaml(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as handle:
        data = yaml.safe_load(handle)
    if not isinstance(data, dict):
        raise ValueError("ontology document must be a mapping")
    return data


def validate_disjoint_axes(document: dict[str, Any]) -> list[dict[str, Any]]:
    orthogonality = document.get("orthogonality") or {}
    if orthogonality.get("owl_construct") != "owl:disjointUnionOf":
        return [{"code": "missing_disjoint_union", "detail": "orthogonality.owl_construct must be owl:disjointUnionOf"}]

    disjoint_axes = set(orthogonality.get("disjoint_axes") or [])
    memberships: dict[str, set[str]] = defaultdict(set)

    for axis in document.get("facet_axes") or []:
        axis_id = axis.get("id")
        if axis_id not in disjoint_axes:
            continue
        for term in axis.get("terms") or []:
            term_id = term.get("id")
            if term_id:
                memberships[term_id].add(axis_id)

    findings = []
    for term_id, axes in sorted(memberships.items()):
        if len(axes) > 1:
            findings.append(
                {
                    "code": "disjoint_axis_overlap",
                    "detail": f"term {term_id!r} appears in multiple owl:disjointUnionOf axes",
                    "term_id": term_id,
                    "axes": sorted(axes),
                }
            )
    return findings


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate CKM facet ontology orthogonality.")
    parser.add_argument("--ontology", required=True, type=Path)
    args = parser.parse_args()

    try:
        document = load_yaml(args.ontology)
        findings = validate_disjoint_axes(document)
    except Exception as exc:  # pragma: no cover - defensive CLI boundary
        print(json.dumps({"status": "error", "error": str(exc)}, indent=2), file=sys.stderr)
        return 2

    report = {
        "status": "pass" if not findings else "fail",
        "validator": "ckm_facet_ontology_validate.py",
        "ontology": str(args.ontology),
        "findings": findings,
        "claim_boundary": "Validates declared local owl:disjointUnionOf term orthogonality only.",
    }
    output = json.dumps(report, indent=2, sort_keys=True)
    if findings:
        print(output, file=sys.stderr)
        return 1
    print(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

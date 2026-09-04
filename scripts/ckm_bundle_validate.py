#!/usr/bin/env python3
"""Validate CKM bundle invariants that span CUE list entries."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
# Generated from specs/facets.cue (single source of truth) by
# scripts/ckm_gen_facets_vocab.py. The validator reads the artifact rather than
# hardcoding a second copy of the vocabularies that could drift.
FACETS_VOCAB_PATH = ROOT / "specs" / "generated" / "facets-vocab.json"


def finding(code: str, path: str, message: str) -> dict[str, str]:
    return {"code": code, "severity": "error", "path": path, "message": message}


def as_list(value: Any) -> list[Any]:
    return value if isinstance(value, list) else []


def load_facets_vocab(path: Path = FACETS_VOCAB_PATH) -> dict[str, list[str]]:
    """Load the controlled scalar-axis vocabularies generated from facets.cue.

    Returns a mapping axis -> allowed values. discipline_role / activity /
    risk_tier are open #FacetTermRef lists in the contract and are
    intentionally not enumerated.
    """
    data = json.loads(path.read_text(encoding="utf-8"))
    axes = data.get("scalar_axes", {})
    return {axis: [str(v) for v in as_list(values)] for axis, values in axes.items()}


def validate_facets(
    facets: Any,
    base_path: str,
    vocab: dict[str, list[str]],
) -> list[dict[str, str]]:
    """Validate one facets object's scalar axes against the controlled vocabularies.

    Each set scalar axis whose value is not a member of its vocabulary yields a
    BUNDLE_FACET_VALUE_INVALID finding naming the offending axis and value — this
    is what rejects a forged ``trust_tier: "trust-me-bro"`` that the structural
    checks alone let through.
    """
    if not isinstance(facets, dict):
        return []
    findings: list[dict[str, str]] = []
    for axis, allowed in vocab.items():
        if axis not in facets:
            continue
        value = facets[axis]
        # Scalar axes carry a single string; a non-string here is a shape error.
        if not isinstance(value, str):
            findings.append(
                finding(
                    "BUNDLE_FACET_AXIS_NOT_SCALAR",
                    f"{base_path}.{axis}",
                    f"Facet axis {axis} must be a string, got {type(value).__name__}.",
                )
            )
            continue
        if value not in allowed:
            findings.append(
                finding(
                    "BUNDLE_FACET_VALUE_INVALID",
                    f"{base_path}.{axis}",
                    f"Facet axis {axis} value {value!r} is not in the controlled vocabulary "
                    f"(specs/facets.cue). Allowed: {', '.join(allowed)}.",
                )
            )
    return findings


def validate_bundle(bundle: dict[str, Any], vocab: dict[str, list[str]] | None = None) -> dict[str, Any]:
    if vocab is None:
        vocab = load_facets_vocab()
    findings: list[dict[str, str]] = []
    feeds = as_list(bundle.get("feeds"))
    if not feeds:
        findings.append(finding("BUNDLE_FEED_ABSENT", "feeds", "Bundle must contain at least one feed."))

    node_ids: set[str] = set()
    node_count = 0
    for feed_index, feed in enumerate(feeds):
        nodes = as_list(feed.get("nodes")) if isinstance(feed, dict) else []
        if not nodes:
            findings.append(
                finding(
                    "BUNDLE_FEED_NODES_ABSENT",
                    f"feeds[{feed_index}].nodes",
                    "Each bundle feed must contain at least one node.",
                )
            )
        for node_index, node in enumerate(nodes):
            node_count += 1
            node_path = f"feeds[{feed_index}].nodes[{node_index}]"
            node_id = node.get("node_id") if isinstance(node, dict) else None
            if not node_id:
                findings.append(
                    finding(
                        "BUNDLE_NODE_ID_MISSING",
                        f"{node_path}.node_id",
                        "Bundle node is missing node_id.",
                    )
                )
                continue
            if node_id in node_ids:
                findings.append(
                    finding(
                        "BUNDLE_NODE_ID_DUPLICATE",
                        f"{node_path}.node_id",
                        f"Duplicate bundle node_id {node_id}.",
                    )
                )
            node_ids.add(str(node_id))
            # Facet vocabulary gate: every node's facets must use controlled values.
            if isinstance(node, dict):
                findings.extend(validate_facets(node.get("facets"), f"{node_path}.facets", vocab))

    rag_metadata = as_list(bundle.get("rag_metadata"))
    for index, metadata in enumerate(rag_metadata):
        node_id = metadata.get("node_id") if isinstance(metadata, dict) else None
        if not node_id:
            findings.append(
                finding(
                    "BUNDLE_RAG_METADATA_NODE_ID_MISSING",
                    f"rag_metadata[{index}].node_id",
                    "RAG metadata entry is missing node_id.",
                )
            )
            continue
        if str(node_id) not in node_ids:
            findings.append(
                finding(
                    "BUNDLE_RAG_METADATA_ORPHAN",
                    f"rag_metadata[{index}].node_id",
                    f"RAG metadata references unknown node_id {node_id}.",
                )
            )
        # The rag/chunk facets are also vocabulary-gated when present.
        if isinstance(metadata, dict) and "facets" in metadata:
            findings.extend(validate_facets(metadata.get("facets"), f"rag_metadata[{index}].facets", vocab))

    return {
        "schema_version": "ckm-bundle-validator-v1",
        "status": "fail" if findings else "pass",
        "summary": {
            "feeds": len(feeds),
            "nodes": node_count,
            "rag_metadata": len(rag_metadata),
            "findings": len(findings),
        },
        "findings": findings,
    }


def _single_finding_report(code: str, path: str, message: str) -> dict[str, Any]:
    return {
        "schema_version": "ckm-bundle-validator-v1",
        "status": "fail",
        "summary": {"feeds": 0, "nodes": 0, "rag_metadata": 0, "findings": 1},
        "findings": [finding(code, path, message)],
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate CKM canonical knowledge bundle invariants.")
    parser.add_argument("--bundle", required=True, help="Path to bundle JSON.")
    parser.add_argument(
        "--vocab",
        default=str(FACETS_VOCAB_PATH),
        help="Path to the generated facet vocabulary JSON (default: specs/generated/facets-vocab.json).",
    )
    args = parser.parse_args()

    # The facet vocabulary gate is mandatory: a missing/broken artifact must fail
    # loudly rather than silently skip facet validation (the SEAM-2 whole point).
    try:
        vocab = load_facets_vocab(Path(args.vocab))
    except Exception as exc:  # noqa: BLE001
        report = _single_finding_report(
            "BUNDLE_FACET_VOCAB_UNAVAILABLE",
            str(args.vocab),
            f"Could not load facet vocabulary artifact (run scripts/ckm_gen_facets_vocab.py): {exc}",
        )
        print(json.dumps(report, indent=2, sort_keys=True))
        return 1

    path = Path(args.bundle)
    try:
        bundle = json.loads(path.read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001 - parser detail belongs in CI output
        print(json.dumps(_single_finding_report("BUNDLE_PARSE_FAILED", str(path), str(exc)), indent=2, sort_keys=True))
        return 1

    if not isinstance(bundle, dict):
        print(
            json.dumps(
                _single_finding_report("BUNDLE_INVALID_ROOT", str(path), "Bundle root must be a JSON object."),
                indent=2,
                sort_keys=True,
            )
        )
        return 1

    report = validate_bundle(bundle, vocab)
    print(json.dumps(report, indent=2, sort_keys=True))
    return 1 if report["status"] == "fail" else 0


if __name__ == "__main__":
    raise SystemExit(main())

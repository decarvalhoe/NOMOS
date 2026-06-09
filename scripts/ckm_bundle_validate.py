#!/usr/bin/env python3
"""Validate CKM bundle invariants that span CUE list entries."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any


def finding(code: str, path: str, message: str) -> dict[str, str]:
    return {"code": code, "severity": "error", "path": path, "message": message}


def as_list(value: Any) -> list[Any]:
    return value if isinstance(value, list) else []


def validate_bundle(bundle: dict[str, Any]) -> dict[str, Any]:
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
            node_id = node.get("node_id") if isinstance(node, dict) else None
            if not node_id:
                findings.append(
                    finding(
                        "BUNDLE_NODE_ID_MISSING",
                        f"feeds[{feed_index}].nodes[{node_index}].node_id",
                        "Bundle node is missing node_id.",
                    )
                )
                continue
            if node_id in node_ids:
                findings.append(
                    finding(
                        "BUNDLE_NODE_ID_DUPLICATE",
                        f"feeds[{feed_index}].nodes[{node_index}].node_id",
                        f"Duplicate bundle node_id {node_id}.",
                    )
                )
            node_ids.add(str(node_id))

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


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate CKM canonical knowledge bundle invariants.")
    parser.add_argument("--bundle", required=True, help="Path to bundle JSON.")
    args = parser.parse_args()

    path = Path(args.bundle)
    try:
        bundle = json.loads(path.read_text(encoding="utf-8"))
    except Exception as exc:  # noqa: BLE001 - parser detail belongs in CI output
        report = {
            "schema_version": "ckm-bundle-validator-v1",
            "status": "fail",
            "summary": {"feeds": 0, "nodes": 0, "rag_metadata": 0, "findings": 1},
            "findings": [finding("BUNDLE_PARSE_FAILED", str(path), str(exc))],
        }
        print(json.dumps(report, indent=2, sort_keys=True))
        return 1

    if not isinstance(bundle, dict):
        report = {
            "schema_version": "ckm-bundle-validator-v1",
            "status": "fail",
            "summary": {"feeds": 0, "nodes": 0, "rag_metadata": 0, "findings": 1},
            "findings": [finding("BUNDLE_INVALID_ROOT", str(path), "Bundle root must be a JSON object.")],
        }
        print(json.dumps(report, indent=2, sort_keys=True))
        return 1

    report = validate_bundle(bundle)
    print(json.dumps(report, indent=2, sort_keys=True))
    return 1 if report["status"] == "fail" else 0


if __name__ == "__main__":
    raise SystemExit(main())

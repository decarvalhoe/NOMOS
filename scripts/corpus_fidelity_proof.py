#!/usr/bin/env python3
"""Generate a corpus fidelity proof report.

The report is intentionally conservative: it proves what is represented by the
generated artifacts and records blockers for any source construct that is not
typed, traced, or span-backed. It does not certify compliance.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from collections import Counter
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


HEADING_RE = re.compile(r"^(#{1,6})\s+")
TABLE_ROW_RE = re.compile(r"^\|.+\|\s*$")
TABLE_SEP_RE = re.compile(r"^\|[\s:_-]+(\|[\s:_-]+)+\|\s*$")
LINK_RE = re.compile(r"(?<!!)\[[^\]]+\]\([^)]+\)")
IMAGE_RE = re.compile(r"!\[[^\]]*\]\([^)]+\)")

STRUCTURAL_NODE_TYPES = {"document", "chapter", "section", "subsection", "article"}
TABLE_NODE_TYPES = {"table", "table_row", "table_cell"}
CODE_NODE_TYPES = {"code", "code_block", "fenced_code", "code_fence"}
CALLOUT_NODE_TYPES = {"callout", "blockquote", "note", "warning"}


def add_finding(
    findings: list[dict[str, Any]],
    code: str,
    severity: str,
    message: str,
    *,
    blocking: bool = True,
    detail: dict[str, Any] | None = None,
) -> None:
    finding: dict[str, Any] = {
        "code": code,
        "severity": severity,
        "blocking": blocking,
        "message": message,
    }
    if detail:
        finding["detail"] = detail
    findings.append(finding)


def scan_markdown(corpus: Path) -> dict[str, Any]:
    total_files = 0
    markdown_files = 0
    bytes_total = 0
    lines_total = 0
    heading_levels: Counter[int] = Counter()
    table_rows = 0
    table_count = 0
    code_blocks = 0
    blockquote_lines = 0
    images = 0
    links = 0
    markdown_paths: list[str] = []

    for path in sorted(corpus.rglob("*")):
        if not path.is_file() or ".git" in path.parts:
            continue
        total_files += 1
        if path.suffix.lower() not in {".md", ".mdx"}:
            continue
        markdown_files += 1
        rel = path.relative_to(corpus).as_posix()
        markdown_paths.append(rel)
        data = path.read_bytes()
        bytes_total += len(data)
        text = data.decode("utf-8", errors="replace")
        lines = text.splitlines()
        lines_total += len(lines)

        in_code = False
        pending_table = False
        for line in lines:
            stripped = line.strip()
            if stripped.startswith("```"):
                in_code = not in_code
                if in_code:
                    code_blocks += 1
                continue
            if in_code:
                continue

            if match := HEADING_RE.match(line):
                heading_levels[len(match.group(1))] += 1
            if TABLE_SEP_RE.match(stripped) and pending_table:
                table_count += 1
                pending_table = False
            elif TABLE_ROW_RE.match(stripped):
                table_rows += 1
                pending_table = True
            else:
                pending_table = False
            if stripped.startswith(">"):
                blockquote_lines += 1

        images += len(IMAGE_RE.findall(text))
        links += len(LINK_RE.findall(text))

    return {
        "total_files": total_files,
        "markdown_files": markdown_files,
        "markdown_paths": markdown_paths,
        "bytes_total": bytes_total,
        "lines_total": lines_total,
        "heading_levels": {str(k): heading_levels[k] for k in sorted(heading_levels)},
        "table_rows": table_rows,
        "tables": table_count,
        "code_blocks": code_blocks,
        "blockquote_lines": blockquote_lines,
        "images": images,
        "links": links,
    }


def load_feed_nodes(artifacts_dir: Path) -> tuple[list[dict[str, Any]], list[str], list[dict[str, str]]]:
    """Returns (nodes, readable feed files, unreadable feed files). A feed file
    that cannot be parsed is NEVER skipped silently (docs/43 principle 8): it
    is returned as an unreadable entry and the proof records a blocking finding."""
    candidates = [
        artifacts_dir / "rbok-lawbook-feed.json",
        *sorted(artifacts_dir.glob("*-feed.json")),
        *sorted(artifacts_dir.glob("*.feed.json")),
    ]
    seen: set[Path] = set()
    nodes: list[dict[str, Any]] = []
    feed_files: list[str] = []
    unreadable: list[dict[str, str]] = []
    for path in candidates:
        path = path.resolve()
        if path in seen or not path.exists():
            continue
        seen.add(path)
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, ValueError) as exc:
            unreadable.append({"file": path.name, "error": f"{type(exc).__name__}: {exc}"})
            continue
        if not isinstance(data, dict):
            unreadable.append({"file": path.name, "error": "feed document is not a JSON object"})
            continue
        feed_files.append(path.name)
        if isinstance(data.get("nodes"), list):
            nodes.extend(n for n in data["nodes"] if isinstance(n, dict))
        for feed in data.get("feeds", []):
            if isinstance(feed, dict) and isinstance(feed.get("nodes"), list):
                nodes.extend(n for n in feed["nodes"] if isinstance(n, dict))
    return nodes, feed_files, unreadable


def node_has_span(node: dict[str, Any]) -> bool:
    if isinstance(node.get("byte_span"), dict) or isinstance(node.get("source_span"), dict):
        return True
    # Check nested "span" object (LawbookSourceSpan from Go serialization)
    span = node.get("span")
    if isinstance(span, dict) and span.get("start_line", 0) > 0:
        return True
    return all(key in node for key in ("start_byte", "end_byte", "start_line", "end_line"))


def node_mentions_link(node: dict[str, Any]) -> bool:
    metadata = node.get("metadata")
    if isinstance(metadata, dict):
        blob = json.dumps(metadata, sort_keys=True).lower()
        return "link" in blob or "href" in blob or "url" in blob
    return False


def feed_summary(nodes: list[dict[str, Any]], feed_files: list[str]) -> dict[str, Any]:
    node_types = Counter(str(n.get("node_type", n.get("type", "unknown"))) for n in nodes)
    source_paths = sorted({str(n.get("source_path", "")) for n in nodes if n.get("source_path")})
    nodes_with_spans = sum(1 for node in nodes if node_has_span(node))
    structural_nodes = sum(node_types[t] for t in STRUCTURAL_NODE_TYPES)
    table_nodes = sum(node_types[t] for t in TABLE_NODE_TYPES)
    code_nodes = sum(node_types[t] for t in CODE_NODE_TYPES)
    callout_nodes = sum(node_types[t] for t in CALLOUT_NODE_TYPES)
    link_nodes = sum(1 for node in nodes if node_mentions_link(node))
    max_ordinal_depth = 0
    for node in nodes:
        ordinal = str(node.get("ordinal_path", ""))
        if ordinal:
            max_ordinal_depth = max(max_ordinal_depth, len(ordinal.split(".")))

    return {
        "feed_files": feed_files,
        "total_nodes": len(nodes),
        "node_types": dict(sorted(node_types.items())),
        "source_paths": source_paths,
        "source_paths_represented": len(source_paths),
        "nodes_with_spans": nodes_with_spans,
        "structural_nodes": structural_nodes,
        "table_nodes": table_nodes,
        "code_nodes": code_nodes,
        "callout_nodes": callout_nodes,
        "link_nodes": link_nodes,
        "max_ordinal_depth": max_ordinal_depth,
    }


def has_artifact(artifacts_dir: Path, patterns: list[str]) -> bool:
    for pattern in patterns:
        if any(artifacts_dir.glob(pattern)):
            return True
    return False


def evaluate(scan: dict[str, Any], feed: dict[str, Any], artifacts_dir: Path) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []

    if feed["total_nodes"] == 0:
        add_finding(findings, "FEED_NODES_MISSING", "critical", "No generated feed nodes were found.")
        return findings

    if feed["nodes_with_spans"] < feed["total_nodes"]:
        add_finding(
            findings,
            "BYTE_SPANS_MISSING",
            "critical",
            "Generated nodes do not all carry exact byte/line spans.",
            detail={"nodes": feed["total_nodes"], "nodes_with_spans": feed["nodes_with_spans"]},
        )

    if scan["table_rows"] > 0 and feed["table_nodes"] == 0:
        add_finding(
            findings,
            "TABLE_BLOCKS_NOT_TYPED",
            "high",
            "Source Markdown contains table blocks but the feed has no table/table_row/table_cell nodes.",
            detail={"source_table_rows": scan["table_rows"], "source_tables": scan["tables"]},
        )

    if scan["code_blocks"] > 0 and feed["code_nodes"] == 0:
        add_finding(
            findings,
            "CODE_BLOCKS_NOT_TYPED",
            "high",
            "Source Markdown contains fenced code blocks but the feed has no code block nodes.",
            detail={"source_code_blocks": scan["code_blocks"]},
        )

    if scan["blockquote_lines"] > 0 and feed["callout_nodes"] == 0:
        add_finding(
            findings,
            "CALLOUT_BLOCKS_NOT_TYPED",
            "high",
            "Source Markdown contains blockquotes/callouts but the feed has no typed callout or blockquote nodes.",
            detail={"source_blockquote_lines": scan["blockquote_lines"]},
        )

    if scan["links"] > 0 and feed["link_nodes"] == 0:
        add_finding(
            findings,
            "LINKS_NOT_TYPED",
            "medium",
            "Source Markdown contains links but generated nodes do not expose link metadata.",
            detail={"source_links": scan["links"]},
        )

    if scan["images"] > 0:
        add_finding(
            findings,
            "IMAGES_NOT_TYPED",
            "medium",
            "Source Markdown contains images; image nodes/metadata must be explicitly typed for full fidelity.",
            detail={"source_images": scan["images"]},
        )

    h5_h6 = int(scan["heading_levels"].get("5", 0)) + int(scan["heading_levels"].get("6", 0))
    if h5_h6 > 0:
        add_finding(
            findings,
            "H5_H6_LEGAL_LEVELS_UNPROVEN",
            "high",
            "Source Markdown contains H5/H6 levels; dedicated legal-level mapping is required.",
            detail={"h5_h6_count": h5_h6},
        )

    if not has_artifact(artifacts_dir, ["*toc*.json", "*toc*.yaml", "*structure*.json"]):
        add_finding(
            findings,
            "CERTIFIED_TOC_ARTIFACT_MISSING",
            "high",
            "No certified table-of-contents or structure proof artifact was found.",
        )

    if not has_artifact(artifacts_dir, ["*lexicon*.json", "*lexicon*.yaml", "*glossary*.json"]):
        add_finding(
            findings,
            "LEXICON_ARTIFACT_MISSING",
            "high",
            "No governed lexicon artifact was found for the corpus proof pack.",
        )

    return findings


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--corpus", required=True)
    parser.add_argument("--artifacts-dir", required=True)
    parser.add_argument("--profile", default="rbok-lawbook")
    parser.add_argument("--report", required=True)
    parser.add_argument("--strict", action="store_true", help="exit non-zero unless full fidelity is proven")
    args = parser.parse_args()

    corpus = Path(args.corpus)
    artifacts_dir = Path(args.artifacts_dir)
    report_path = Path(args.report)

    findings: list[dict[str, Any]] = []
    if not corpus.is_dir():
        add_finding(findings, "CORPUS_NOT_FOUND", "critical", f"Corpus directory not found: {corpus}")
        scan = {}
        feed = {}
    elif not artifacts_dir.is_dir():
        add_finding(findings, "ARTIFACTS_DIR_NOT_FOUND", "critical", f"Artifacts directory not found: {artifacts_dir}")
        scan = scan_markdown(corpus)
        feed = {}
    else:
        scan = scan_markdown(corpus)
        nodes, feed_files, unreadable = load_feed_nodes(artifacts_dir)
        feed = feed_summary(nodes, feed_files)
        feed["unreadable_feed_files"] = unreadable
        for entry in unreadable:
            add_finding(
                findings,
                "FEED_FILE_UNREADABLE",
                "critical",
                f"Feed file could not be read as a JSON object and is excluded from the proof: {entry['file']}",
                detail=entry,
            )
        findings.extend(evaluate(scan, feed, artifacts_dir))

    blocking = sum(1 for finding in findings if finding["blocking"])
    full_allowed = blocking == 0
    status = "full_fidelity_proven" if full_allowed else "partial"

    report = {
        "schema_version": "0.1.0",
        "report_type": "corpus_fidelity_proof",
        "profile": args.profile,
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "status": status,
        "full_fidelity_claim_allowed": full_allowed,
        "claim_boundary": "Proof pack only; no regulatory compliance certification.",
        "source_scan": scan,
        "artifact_scan": feed,
        "summary": {
            "findings": len(findings),
            "blocking_findings": blocking,
        },
        "findings": findings,
    }

    report_path.parent.mkdir(parents=True, exist_ok=True)
    report_path.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report, indent=2, sort_keys=True))

    if args.strict and not full_allowed:
        return 1
    return 1 if any(f["code"] in {"CORPUS_NOT_FOUND", "ARTIFACTS_DIR_NOT_FOUND"} for f in findings) else 0


if __name__ == "__main__":
    raise SystemExit(main())

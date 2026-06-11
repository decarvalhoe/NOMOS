"""VRC-35 (#572, doc 45 §3 B1) — the reference retrieval consumer kit.

A deliberately small, deterministic, network-free retrieval consumer that
proves the anti-distractor promise of Knowledge Lenses ON A REAL EMITTED
BUNDLE: the lens runs AT THE BASE LEVEL (a WHERE over facets, applied before
any scoring — doc 40 §5), never as a post-hoc re-rank.

This kit lives OUTSIDE the Go core on purpose: retrieval is a consumer
concern (pgvector+RLS in Aedifica, Qdrant payload filters elsewhere); the
core only guarantees the artifact and the lens semantics. The lexical scorer
here is intentionally naive — the point is the MEASURED gap between
lens-filtered and unfiltered retrieval on the pack's own golden corpus, with
thresholds VERSIONED in the pack harness file.

Exit code: 0 only if every versioned threshold holds and every
`never_retrieve` document stays out at every rank. Anything else: 1.
"""
from __future__ import annotations

import argparse
import json
import re
import unicodedata
from pathlib import Path
from typing import Any

import yaml


# --- lens semantics (mirrors cli/internal/atomization ApplyLens) ------------

def _as_list(value: Any) -> list[Any]:
    if value is None:
        return []
    if isinstance(value, list):
        return value
    return [value]


def _selection_matches(facets: dict[str, Any], selection: dict[str, Any]) -> bool:
    for axis, expected in selection.items():
        if not set(_as_list(facets.get(axis))).intersection(set(_as_list(expected))):
            return False
    return True


def _any_matches(facets: dict[str, Any], selections: list[dict[str, Any]]) -> bool:
    return any(_selection_matches(facets, s) for s in selections)


def lens_includes(facets: dict[str, Any], lens: dict[str, Any]) -> bool:
    exclude = lens.get("exclude") or {}
    if _any_matches(facets, exclude.get("any_of") or []):
        return False
    include = lens.get("include") or {}
    all_of = include.get("all_of") or []
    if all_of and not all(_selection_matches(facets, s) for s in all_of):
        return False
    any_of = include.get("any_of") or []
    if any_of and not _any_matches(facets, any_of):
        return False
    none_of = include.get("none_of") or []
    if none_of and _any_matches(facets, none_of):
        return False
    return True


# --- deterministic lexical scorer -------------------------------------------

def _fold(text: str) -> str:
    text = unicodedata.normalize("NFKD", text)
    return "".join(c for c in text if not unicodedata.combining(c)).lower()


def tokens(text: str) -> set[str]:
    return set(re.findall(r"[a-z0-9]+", _fold(text)))


def rank_documents(query: str, nodes: list[dict[str, Any]]) -> list[str]:
    """Rank source documents by their best node's lexical overlap with the
    query. Deterministic: ties break on document name."""
    q = tokens(query)
    best: dict[str, int] = {}
    for node in nodes:
        score = len(q & tokens(node.get("text", "")))
        doc = node["source_path"]
        if score > best.get(doc, -1):
            best[doc] = score
    ranked = sorted(best.items(), key=lambda kv: (-kv[1], kv[0]))
    return [doc for doc, score in ranked if score > 0]


# --- harness -----------------------------------------------------------------

def load_yaml(path: Path) -> dict[str, Any]:
    loaded = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(loaded, dict):
        raise SystemExit(f"{path} must contain a YAML object")
    return loaded


def bundle_nodes(bundle: dict[str, Any]) -> list[dict[str, Any]]:
    nodes: list[dict[str, Any]] = []
    for feed in bundle.get("feeds", []):
        nodes.extend(feed.get("nodes", []))
    if not nodes:
        raise SystemExit("the bundle carries no nodes — nothing to retrieve from")
    return nodes


def enriched_facets(node: dict[str, Any], document_facets: dict[str, Any]) -> dict[str, Any]:
    """Engine-derived facets merged with the pack's per-document enrichment
    (the consumer-level WHERE data: activity, confidentiality, applicability)."""
    facets = dict(node.get("facets") or {})
    facets.update(document_facets.get(node["source_path"], {}))
    return facets


def load_preset(presets_dir: Path, preset_id: str) -> dict[str, Any]:
    for path in sorted(presets_dir.glob("*.lens.y*ml")):
        lens = load_yaml(path)
        if lens.get("id") == preset_id:
            return lens
    raise SystemExit(f"preset {preset_id} not found under {presets_dir}")


def run_harness(bundle: dict[str, Any], harness: dict[str, Any], presets_dir: Path) -> dict[str, Any]:
    nodes = bundle_nodes(bundle)
    document_facets = harness.get("document_facets") or {}
    queries = harness.get("queries") or []
    if not queries:
        raise SystemExit("the harness declares no queries")
    thresholds = harness.get("thresholds") or {}
    for key in ("min_accuracy_with_lens", "max_accuracy_without_lens", "min_margin"):
        if key not in thresholds:
            raise SystemExit(f"threshold {key} is not versioned in the harness")

    results = []
    hits_with = 0
    hits_without = 0
    never_violations: list[str] = []
    for query in queries:
        lens = load_preset(presets_dir, query["preset"])
        scoped = [n for n in nodes if lens_includes(enriched_facets(n, document_facets), lens)]
        ranking_with = rank_documents(query["text"], scoped)
        ranking_without = rank_documents(query["text"], nodes)
        top_with = ranking_with[0] if ranking_with else None
        top_without = ranking_without[0] if ranking_without else None
        hit_with = top_with == query["expected"]
        hit_without = top_without == query["expected"]
        hits_with += int(hit_with)
        hits_without += int(hit_without)
        never = query.get("never_retrieve")
        never_ok = None
        if never:
            never_ok = never not in ranking_with
            if not never_ok:
                never_violations.append(
                    f"{query['id']}: {never} surfaced at rank {ranking_with.index(never) + 1} through {query['preset']}"
                )
        results.append({
            "id": query["id"],
            "expected": query["expected"],
            "preset": query["preset"],
            "with_lens": {"top": top_with, "hit": hit_with, "ranking": ranking_with},
            "without_lens": {"top": top_without, "hit": hit_without, "ranking": ranking_without},
            "never_retrieve_ok": never_ok,
        })

    accuracy_with = hits_with / len(queries)
    accuracy_without = hits_without / len(queries)
    margin = accuracy_with - accuracy_without
    failures: list[str] = list(never_violations)
    if accuracy_with < thresholds["min_accuracy_with_lens"]:
        failures.append(
            f"accuracy_with_lens {accuracy_with:.2f} < versioned minimum {thresholds['min_accuracy_with_lens']}"
        )
    if accuracy_without > thresholds["max_accuracy_without_lens"]:
        failures.append(
            f"accuracy_without_lens {accuracy_without:.2f} > versioned maximum {thresholds['max_accuracy_without_lens']}"
        )
    if margin < thresholds["min_margin"]:
        failures.append(f"margin {margin:.2f} < versioned minimum {thresholds['min_margin']}")

    return {
        "schema_version": "nomos-reference-retrieval-verdict-v1",
        "domain_profile": harness.get("domain_profile"),
        "queries": results,
        "accuracy_with_lens": accuracy_with,
        "accuracy_without_lens": accuracy_without,
        "margin": margin,
        "thresholds": thresholds,
        "failures": failures,
        "pass": not failures,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bundle", required=True, type=Path, help="real emitted bundle (json)")
    parser.add_argument("--harness", required=True, type=Path, help="pack retrieval harness (yaml)")
    parser.add_argument("--presets-dir", required=True, type=Path, help="pack lens presets directory")
    args = parser.parse_args()

    bundle = json.loads(args.bundle.read_text(encoding="utf-8"))
    harness = load_yaml(args.harness)
    verdict = run_harness(bundle, harness, args.presets_dir)
    print(json.dumps(verdict, indent=2, sort_keys=True, ensure_ascii=False))
    return 0 if verdict["pass"] else 1


if __name__ == "__main__":
    raise SystemExit(main())

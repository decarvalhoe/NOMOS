#!/usr/bin/env python3
"""Validate and emit RAG answer evidence records.

The report captures answer, prompt, model/provider, citation, refusal,
confidence, and policy-outcome metadata. It does not make LLM output
authoritative.
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

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for RAG answer evidence validation.", file=sys.stderr)
    raise SystemExit(2) from exc


CLAIM_BOUNDARY = "RAG answer evidence only; no legal, clinical, regulatory, or model-authority claim."
DEFAULT_FIXTURES = Path("docs/regulated/ai-rag-governance/rag-answer-fixtures.yaml")
DEFAULT_OUTPUT = Path(".regulated-evidence-pack/rag-answer-evidence.json")
REFUSAL_OUTCOMES = {"acceptable_refusal", "unsupported", "blocked_prompt_injection"}
RESPONSE_CONTRACT_FIELDS = (
    "answer",
    "structured_facts",
    "citations",
    "uncertainties",
    "requires_human_decision",
)
ALCE_GATE = 0.95
DEEPEVAL_FAITHFULNESS_GATE = 0.95
TRUST_SCORE_CERTIFIED_GATE = 0.95
TRUST_SCORE_INDICATIVE_GATE = 0.80

# CKM-H6: the gate recomputes groundedness from the retrieved span text instead
# of trusting the producer's self-declared faithfulness_score. The recomputation
# is a deterministic lexical-entailment proxy (no model, no network): an answer
# sentence is "supported" when at least GROUNDEDNESS_SENTENCE_THRESHOLD of its
# content tokens appear in the union of retrieved/cited span text. A neural NLI
# backend is a pluggable upgrade; the contract is that a self-declared score may
# only LOWER the gated value, never raise it.
GROUNDEDNESS_SENTENCE_THRESHOLD = 0.6
GROUNDEDNESS_METHOD = "lexical_entailment_v1"
_GROUNDEDNESS_STOPWORDS = {
    "the", "a", "an", "and", "or", "of", "to", "in", "is", "are", "be", "for",
    "that", "this", "it", "as", "by", "on", "with", "must", "not", "no", "from",
    "its", "was", "were", "has", "have", "had", "but", "any", "all", "may",
}


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def resolve(root: Path, value: str | Path) -> Path:
    path = Path(value)
    return path if path.is_absolute() else root / path


def rel(path: Path, root: Path) -> str:
    return path.resolve().relative_to(root.resolve()).as_posix()


def load_yaml(path: Path) -> dict[str, Any]:
    return yaml.safe_load(path.read_text(encoding="utf-8")) or {}


def as_list(value: Any) -> list[Any]:
    return value if isinstance(value, list) else []


def nonempty(value: Any) -> bool:
    return value is not None and value != "" and value != []


def present(value: Any) -> bool:
    return value is not None and value != ""


def has_source_backed_citation(answer: dict[str, Any]) -> bool:
    if str(answer.get("citation_status", "")).strip() != "source_backed":
        return False
    spans = as_list(answer.get("source_spans"))
    if not spans:
        return False
    for span in spans:
        if not isinstance(span, dict):
            return False
        if not all(nonempty(span.get(field)) for field in ("source_id", "source_hash", "span")):
            return False
    return True


def has_explicit_refusal(answer: dict[str, Any]) -> bool:
    refusal_status = str(answer.get("refusal_status", "")).strip()
    policy_outcome = str(answer.get("policy_outcome", "")).strip()
    return refusal_status in {"refused", "unsupported"} and policy_outcome in REFUSAL_OUTCOMES


def response_contract(answer: dict[str, Any]) -> dict[str, Any]:
    fields = {field: field in answer for field in RESPONSE_CONTRACT_FIELDS}
    return {
        "fields": fields,
        "complete": all(fields.values()),
        "requires_human_decision": bool(answer.get("requires_human_decision")),
    }


def chunk_key(chunk: dict[str, Any]) -> tuple[Any, ...]:
    chunk_id = chunk.get("chunk_id")
    if chunk_id:
        return ("chunk_id", chunk_id)
    return (
        "source_span",
        chunk.get("source_id"),
        chunk.get("source_hash"),
        chunk.get("span"),
    )


def cited_key(span: dict[str, Any]) -> tuple[Any, ...]:
    chunk_id = span.get("chunk_id")
    if chunk_id:
        return ("chunk_id", chunk_id)
    return (
        "source_span",
        span.get("source_id"),
        span.get("source_hash"),
        span.get("span"),
    )


def citation_metrics(answer: dict[str, Any]) -> dict[str, float]:
    retrieved = [item for item in as_list(answer.get("retrieved_chunks")) if isinstance(item, dict)]
    cited = [item for item in as_list(answer.get("source_spans")) if isinstance(item, dict)]
    retrieved_keys = {chunk_key(item) for item in retrieved}
    cited_keys = {cited_key(item) for item in cited}

    if not retrieved_keys and has_explicit_refusal(answer):
        return {"citation_recall": 1.0, "citation_precision": 1.0}

    if not retrieved_keys:
        recall = 1.0 if not cited_keys else 0.0
    else:
        recall = len(retrieved_keys & cited_keys) / len(retrieved_keys)

    if not cited_keys:
        precision = 1.0 if not retrieved_keys else 0.0
    else:
        precision = len(retrieved_keys & cited_keys) / len(cited_keys)

    return {
        "citation_recall": round(recall, 4),
        "citation_precision": round(precision, 4),
    }


def _content_tokens(text: str) -> list[str]:
    tokens = re.findall(r"[a-z0-9]+", text.lower())
    return [t for t in tokens if len(t) >= 3 and t not in _GROUNDEDNESS_STOPWORDS]


def _sentences(text: str) -> list[str]:
    return [part.strip() for part in re.split(r"[.!?]+", text) if part.strip()]


def _support_corpus(answer: dict[str, Any]) -> list[str]:
    """Collect the retrieved/cited span text the answer must be grounded in."""
    texts: list[str] = []
    for collection in ("source_spans", "retrieved_chunks"):
        for item in as_list(answer.get(collection)):
            if not isinstance(item, dict):
                continue
            for field in ("text", "chunk_text", "content"):
                value = item.get(field)
                if isinstance(value, str) and value.strip():
                    texts.append(value)
                    break
    return texts


def recompute_groundedness(answer: dict[str, Any]) -> dict[str, Any] | None:
    """Recompute groundedness from the retrieved span text vs the answer text.

    Returns None when there is no span text to recompute from (the gate then
    falls back to structural citation coverage — it still never trusts the
    self-declared score). This is the H6 fix: the gate derives the score from the
    evidence, not from a number the producer asserted about itself.
    """
    support = _support_corpus(answer)
    answer_text = answer.get("answer")
    if not support or not isinstance(answer_text, str) or not answer_text.strip():
        return None
    support_tokens: set[str] = set()
    for text in support:
        support_tokens.update(_content_tokens(text))
    sentences = _sentences(answer_text)
    if not sentences:
        return None
    supported = 0
    for sentence in sentences:
        tokens = _content_tokens(sentence)
        if not tokens:
            supported += 1
            continue
        covered = sum(1 for token in tokens if token in support_tokens) / len(tokens)
        if covered >= GROUNDEDNESS_SENTENCE_THRESHOLD:
            supported += 1
    return {
        "method": GROUNDEDNESS_METHOD,
        "score": round(supported / len(sentences), 4),
        "supported_sentences": supported,
        "total_sentences": len(sentences),
    }


def faithfulness_score(answer: dict[str, Any], alce: dict[str, float]) -> float:
    if has_explicit_refusal(answer):
        return 1.0
    recomputed = recompute_groundedness(answer)
    if recomputed is not None:
        base = recomputed["score"]
    else:
        # No span text to recompute from: fall back to structural citation
        # coverage. The self-declared score is never trusted to RAISE this.
        base = min(alce["citation_recall"], alce["citation_precision"])
    declared = answer.get("faithfulness_score")
    if isinstance(declared, (int, float)) and 0 <= declared <= 1:
        # A producer may declare itself LESS faithful, never more.
        base = min(base, float(declared))
    return round(base, 4)


def groundedness_detail(answer: dict[str, Any], alce: dict[str, float]) -> dict[str, Any]:
    recomputed = recompute_groundedness(answer)
    declared = answer.get("faithfulness_score")
    detail: dict[str, Any] = {
        "recomputed_from_spans": recomputed is not None,
        "self_declared": declared if isinstance(declared, (int, float)) else None,
        "self_declared_trusted": False,
    }
    if recomputed is not None:
        detail.update(recomputed)
    else:
        detail["method"] = "structural_citation_coverage"
        detail["score"] = round(min(alce["citation_recall"], alce["citation_precision"]), 4)
    return detail


def answer_metrics(answer: dict[str, Any]) -> dict[str, Any]:
    alce = citation_metrics(answer)
    faithfulness = faithfulness_score(answer, alce)
    confidence = answer.get("confidence")
    confidence_score = float(confidence) if isinstance(confidence, (int, float)) else 0.0
    trust_score = round(
        (
            alce["citation_recall"]
            + alce["citation_precision"]
            + faithfulness
            + confidence_score
        )
        / 4,
        4,
    )
    return {
        "alce": alce,
        "deepeval": {
            "faithfulness": faithfulness,
        },
        "groundedness": groundedness_detail(answer, alce),
        "trust_score": trust_score,
    }


def trust_tier(metrics: dict[str, Any], findings: list[dict[str, str]]) -> str:
    if findings:
        return "unverified"
    alce = metrics["alce"]
    faithfulness = metrics["deepeval"]["faithfulness"]
    trust_score = metrics["trust_score"]
    if (
        alce["citation_recall"] >= ALCE_GATE
        and alce["citation_precision"] >= ALCE_GATE
        and faithfulness >= DEEPEVAL_FAITHFULNESS_GATE
        and trust_score >= TRUST_SCORE_CERTIFIED_GATE
    ):
        return "certified"
    if trust_score >= TRUST_SCORE_INDICATIVE_GATE:
        return "indicative"
    return "unverified"


def validate_answer(answer: dict[str, Any], index: int) -> list[dict[str, str]]:
    findings: list[dict[str, str]] = []
    answer_id = str(answer.get("answer_id") or f"answers[{index}]")
    required = [
        "answer_id",
        "prompt_id",
        "model",
        "retrieved_chunks",
        "source_spans",
        "citation_status",
        "refusal_status",
        "confidence",
        "policy_outcome",
    ]
    for field in required:
        if field in answer and present(answer.get(field)):
            continue
        findings.append(
            {
                "code": "MISSING_RAG_ANSWER_FIELD",
                "severity": "error",
                "path": f"{answer_id}:{field}",
                "message": f"RAG answer evidence is missing required field {field}.",
            }
        )

    contract = response_contract(answer)
    if str(answer.get("policy_outcome", "")).strip() == "acceptable" and not contract["complete"]:
        missing = [field for field in RESPONSE_CONTRACT_FIELDS if not contract["fields"][field]]
        findings.append(
            {
                "code": "MISSING_RAG_RESPONSE_CONTRACT_FIELD",
                "severity": "error",
                "path": answer_id,
                "message": "RAG answer is missing recommended response contract field(s): "
                + ", ".join(missing)
                + ".",
            }
        )

    confidence = answer.get("confidence")
    if not isinstance(confidence, (int, float)) or confidence < 0 or confidence > 1:
        findings.append(
            {
                "code": "INVALID_CONFIDENCE",
                "severity": "error",
                "path": f"{answer_id}:confidence",
                "message": "Confidence must be a number between 0 and 1.",
            }
        )

    policy_outcome = str(answer.get("policy_outcome", "")).strip()
    if policy_outcome == "acceptable" and not (
        has_source_backed_citation(answer) or has_explicit_refusal(answer)
    ):
        findings.append(
            {
                "code": "ACCEPTABLE_WITHOUT_CITATION_OR_REFUSAL",
                "severity": "error",
                "path": answer_id,
                "message": "Acceptable RAG answers require source-backed citations or explicit refusal/unsupported state.",
            }
        )

    if str(answer.get("citation_status", "")).strip() == "source_backed" and not has_source_backed_citation(answer):
        findings.append(
            {
                "code": "SOURCE_BACKED_CITATION_WITHOUT_SOURCE_SPANS",
                "severity": "error",
                "path": answer_id,
                "message": "source_backed citation status requires source_id, source_hash, and span.",
            }
        )
    metrics = answer_metrics(answer)
    if str(answer.get("policy_outcome", "")).strip() == "acceptable":
        if metrics["alce"]["citation_recall"] < ALCE_GATE:
            findings.append(
                {
                    "code": "ALCE_CITATION_RECALL_BELOW_GATE",
                    "severity": "error",
                    "path": answer_id,
                    "message": "Retrieved chunks are not fully covered by source-backed citations.",
                }
            )
        if metrics["alce"]["citation_precision"] < ALCE_GATE:
            findings.append(
                {
                    "code": "ALCE_CITATION_PRECISION_BELOW_GATE",
                    "severity": "error",
                    "path": answer_id,
                    "message": "Citations include spans that do not bind to retrieved chunks.",
                }
            )
        if metrics["deepeval"]["faithfulness"] < DEEPEVAL_FAITHFULNESS_GATE:
            findings.append(
                {
                    "code": "DEEPEVAL_FAITHFULNESS_BELOW_GATE",
                    "severity": "error",
                    "path": answer_id,
                    "message": "Faithfulness score is below the cite-or-abstain gate.",
                }
            )
    return findings


def evidence_record(answer: dict[str, Any], findings: list[dict[str, str]]) -> dict[str, Any]:
    model = answer.get("model") if isinstance(answer.get("model"), dict) else {}
    metrics = answer_metrics(answer)
    return {
        "answer_id": answer.get("answer_id"),
        "prompt_id": answer.get("prompt_id"),
        "fixture_type": answer.get("fixture_type", "unspecified"),
        "model": {
            "provider": model.get("provider"),
            "name": model.get("name"),
            "version": model.get("version"),
        },
        "retrieved_chunks": as_list(answer.get("retrieved_chunks")),
        "source_spans": as_list(answer.get("source_spans")),
        "citation_status": answer.get("citation_status"),
        "refusal_status": answer.get("refusal_status"),
        "confidence": answer.get("confidence"),
        "policy_outcome": answer.get("policy_outcome"),
        "acceptable": str(answer.get("policy_outcome", "")).strip() == "acceptable"
        and has_source_backed_citation(answer),
        "explicit_refusal_or_unsupported": has_explicit_refusal(answer),
        "response_contract": response_contract(answer),
        "metrics": metrics,
        "trust_tier": trust_tier(metrics, findings),
    }


def build_report(root: Path, fixtures_path: Path) -> dict[str, Any]:
    fixture_doc = load_yaml(fixtures_path)
    answers = [answer for answer in as_list(fixture_doc.get("answers")) if isinstance(answer, dict)]
    findings: list[dict[str, str]] = []
    records = []
    for index, answer in enumerate(answers):
        answer_findings = validate_answer(answer, index)
        findings.extend(answer_findings)
        records.append(evidence_record(answer, answer_findings))

    fixture_counts = Counter(str(record.get("fixture_type", "unspecified")) for record in records)
    trust_counts = Counter(str(record.get("trust_tier", "unverified")) for record in records)
    if records:
        avg_recall = round(
            sum(record["metrics"]["alce"]["citation_recall"] for record in records) / len(records),
            4,
        )
        avg_precision = round(
            sum(record["metrics"]["alce"]["citation_precision"] for record in records) / len(records),
            4,
        )
        avg_faithfulness = round(
            sum(record["metrics"]["deepeval"]["faithfulness"] for record in records) / len(records),
            4,
        )
        avg_trust_score = round(sum(record["metrics"]["trust_score"] for record in records) / len(records), 4)
    else:
        avg_recall = avg_precision = avg_faithfulness = avg_trust_score = 0.0
    summary_metrics = {
        "alce": {
            "citation_recall": avg_recall,
            "citation_precision": avg_precision,
        },
        "deepeval": {
            "faithfulness": avg_faithfulness,
        },
        "trust_score": avg_trust_score,
    }
    return {
        "schema_version": "0.1.0",
        "status": "failed" if findings else "generated",
        "generated_at_utc": utc_now(),
        "claim_boundary": CLAIM_BOUNDARY,
        "source_documents": {
            "fixtures": rel(fixtures_path, root),
        },
        "summary": {
            "answers_checked": len(records),
            "acceptable_answers": sum(1 for record in records if record["acceptable"]),
            "explicit_refusal_or_unsupported": sum(
                1 for record in records if record["explicit_refusal_or_unsupported"]
            ),
            "fixture_types": dict(sorted(fixture_counts.items())),
            "metrics": summary_metrics,
            "trust_tiers": dict(sorted(trust_counts.items())),
            "trust_tier": trust_tier(summary_metrics, findings),
            "findings": len(findings),
        },
        "answers": records,
        "findings": findings,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Emit and validate RAG answer evidence.")
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument("--fixtures", default=str(DEFAULT_FIXTURES), help="RAG answer fixture YAML path.")
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT), help="JSON evidence report path.")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    output = resolve(root, args.output)
    output.parent.mkdir(parents=True, exist_ok=True)

    report = build_report(root, resolve(root, args.fixtures))
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report["summary"], indent=2, sort_keys=True))
    return 1 if report["status"] == "failed" else 0


if __name__ == "__main__":
    raise SystemExit(main())

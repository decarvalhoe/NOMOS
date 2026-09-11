#!/usr/bin/env python3
"""Validate and emit RAG answer evidence records.

The report captures answer, prompt, model/provider, citation, refusal,
confidence, and policy-outcome metadata. It does not make LLM output
authoritative.

#624 (doc 45 A1 / VRC-10): the verdict comes from the Go engine. This sidecar
runs ``nomos answer gate`` on the fixtures and takes its verdicts as the only
source of the citation metrics, the faithfulness, the trust score and tier, the
cite/abstain decision, the gate findings and the thresholds (``gates``). It
keeps in its own hands only what the engine does not know: the evidence-envelope
validation (required record fields, response contract, confidence range, unique
answer ids) and the shape of the evidence pack.

Engine modes (``--engine``):

* ``required`` (default, what CI runs): no engine, a crashing engine, a timeout,
  non-JSON output or a verdict that cannot be aligned with the fixtures exits 2
  and writes NO report (a stale report at the output path is removed). Nothing
  is scored locally.
* ``fallback``: the engine is still preferred; only when it is unavailable does
  the local lexical proxy (CKM-H6, negation-blind) produce the verdicts. The
  report then says so (``engine.verdict_source: python_fallback``), carries a
  warning finding and caps every trust tier at ``indicative``. This is the
  documented PARTIAL path, never the default.

Engine resolution: ``--nomos-bin``, else ``$NOMOS_BIN``, else ``go run .`` in
``<root>/cli`` (or in the checkout this script ships with), else ``nomos`` on
PATH. ``--scorer-cmd`` / ``--scorer-threshold`` / ``--scorer-timeout`` are
forwarded to the engine (#622), so an NLI second judge reaches the evidence
pack without any model in this sidecar.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from collections import Counter
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for RAG answer evidence validation.", file=sys.stderr)
    raise SystemExit(2) from exc


SCRIPT_ROOT = Path(__file__).resolve().parents[1]
REPORT_SCHEMA_VERSION = "0.2.0"
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

# --- engine consumption (#624) ------------------------------------------------
ENGINE_MODES = ("required", "fallback")
VERDICT_SOURCE_ENGINE = "go_engine"
VERDICT_SOURCE_FALLBACK = "python_fallback"
FINDING_SOURCE_ENVELOPE = "evidence_envelope"
ENGINE_UNAVAILABLE_CODE = "NOMOS_ENGINE_UNAVAILABLE"
FALLBACK_WARNING_CODE = "RAG_GATE_VERDICT_FROM_PYTHON_FALLBACK"
DUPLICATE_ANSWER_ID_CODE = "DUPLICATE_RAG_ANSWER_ID"
FALLBACK_TIER_CAP = "indicative"
DEFAULT_ENGINE_TIMEOUT_SECONDS = 900.0
# 0 = every answer cites or abstains legitimately, 1 = at least one finding.
# Anything else is not a verdict (usage error, unreadable fixtures, crash).
ENGINE_GATE_EXIT_CODES = (0, 1)
TIER_ORDER = ("unverified", "indicative", "certified")

# --- fallback proxy (`--engine fallback` only) ---------------------------------
# These mirror the engine defaults (cli/internal/answer.Defaults()). They are
# NOT read in required mode: there, every threshold comes from the engine's
# `gates` block, and every score from its verdicts.
ALCE_GATE = 0.95
DEEPEVAL_FAITHFULNESS_GATE = 0.95
TRUST_SCORE_CERTIFIED_GATE = 0.95
TRUST_SCORE_INDICATIVE_GATE = 0.80

# CKM-H6: the gate recomputes groundedness from the retrieved span text instead
# of trusting the producer's self-declared faithfulness_score. The recomputation
# is a deterministic lexical-entailment proxy (no model, no network): an answer
# sentence is "supported" when at least GROUNDEDNESS_SENTENCE_THRESHOLD of its
# content tokens appear in the union of retrieved/cited span text. The contract
# is that a self-declared score may only LOWER the gated value, never raise it.
#
# CKM-H6-FU (#538): when an answer REQUIRES grounding (acceptable, non-refusal,
# has answer text) but no span text is available to verify against,
# groundedness is 0 (a blocking finding) — the producer cannot disarm the gate
# by withholding span text, and a declared score cannot raise the 0 floor.
#
# Limitation (documented in the report): the lexical proxy is NEGATION-BLIND —
# "X is covered" and "X is not covered" share content tokens. The engine's
# `--scorer-cmd` second judge (#622) is the upgrade; this proxy never gets one.
GROUNDEDNESS_SENTENCE_THRESHOLD = 0.6
GROUNDEDNESS_METHOD = "lexical_entailment_v1"
GROUNDEDNESS_METHOD_NO_TEXT = "no_span_text"
GROUNDEDNESS_METHOD_REFUSAL = "explicit_refusal"
GROUNDEDNESS_METHOD_STRUCTURAL = "structural_citation_coverage"
GROUNDEDNESS_UPGRADE = (
    "neural NLI entailment — pluggable in the Go gate (nomos answer gate --scorer-cmd, "
    "strictest-wins per sentence, fail-closed; reference adapter scripts/nomos_hhem_scorer.py); "
    "this sidecar forwards --scorer-cmd to the engine and scores nothing itself in required mode"
)
GROUNDEDNESS_LIMITATION = (
    "lexical_entailment_v1 is negation-blind: it matches content-token overlap and "
    "cannot distinguish a claim from its negation. NLI is the pluggable upgrade. "
    "Spans that require grounding but carry no text score 0 (cannot be verified)."
)
FALLBACK_GATES = {
    "alce_gate": ALCE_GATE,
    "faithfulness_gate": DEEPEVAL_FAITHFULNESS_GATE,
    "trust_score_certified": TRUST_SCORE_CERTIFIED_GATE,
    "trust_score_indicative": TRUST_SCORE_INDICATIVE_GATE,
    "sentence_threshold": GROUNDEDNESS_SENTENCE_THRESHOLD,
    "scorer_configured": False,
}
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


def requires_grounding(answer: dict[str, Any]) -> bool:
    """True when the answer asserts content that must be grounded in spans.

    An explicit refusal asserts nothing, so it requires no grounding. Any other
    answer that carries non-empty answer text and is not a refusal must be
    verifiable against retrieved/cited span text.
    """
    if has_explicit_refusal(answer):
        return False
    answer_text = answer.get("answer")
    return isinstance(answer_text, str) and bool(answer_text.strip())


def response_contract(answer: dict[str, Any]) -> dict[str, Any]:
    fields = {field: field in answer for field in RESPONSE_CONTRACT_FIELDS}
    return {
        "fields": fields,
        "complete": all(fields.values()),
        "requires_human_decision": bool(answer.get("requires_human_decision")),
    }


# --- engine: resolution, invocation, alignment ----------------------------------


class EngineError(RuntimeError):
    """The engine produced no usable verdict. Required mode aborts on it."""


@dataclass
class Engine:
    command: list[str]
    cwd: Path | None
    origin: str


@dataclass
class EngineOptions:
    mode: str = "required"
    nomos_bin: str | None = None
    timeout: float = DEFAULT_ENGINE_TIMEOUT_SECONDS
    scorer_cmd: str | None = None
    scorer_threshold: float | None = None
    scorer_timeout: str | None = None

    def scorer_argv(self) -> list[str]:
        if not self.scorer_cmd:
            return []
        argv = ["--scorer-cmd", self.scorer_cmd]
        if self.scorer_threshold is not None:
            argv += ["--scorer-threshold", str(self.scorer_threshold)]
        if self.scorer_timeout:
            argv += ["--scorer-timeout", self.scorer_timeout]
        return argv


def _binary_path(root: Path, value: str) -> str:
    path = Path(value).expanduser()
    if path.is_absolute() or os.sep not in value:
        # Absolute, or a bare name the subprocess resolves on PATH.
        return str(path)
    for base in (Path.cwd(), root):
        candidate = base / path
        if candidate.exists():
            return str(candidate.resolve())
    return str(path)


def resolve_engine(root: Path, nomos_bin: str | None) -> Engine | None:
    """Locate the engine: --nomos-bin, $NOMOS_BIN, `go run .`, then PATH."""
    if nomos_bin:
        return Engine([_binary_path(root, nomos_bin)], None, "--nomos-bin")
    env_bin = os.environ.get("NOMOS_BIN", "").strip()
    if env_bin:
        return Engine([_binary_path(root, env_bin)], None, "NOMOS_BIN")
    if shutil.which("go"):
        for cli_dir in (root / "cli", SCRIPT_ROOT / "cli"):
            if (cli_dir / "go.mod").is_file():
                return Engine(["go", "run", "."], cli_dir, f"go run ({cli_dir})")
    on_path = shutil.which("nomos")
    if on_path:
        return Engine([on_path], None, "PATH")
    return None


def _tail(text: str, limit: int = 400) -> str:
    text = text.strip()
    return text if len(text) <= limit else "…" + text[-limit:]


def _run(engine: Engine, args: list[str], timeout: float) -> subprocess.CompletedProcess[str]:
    argv = [*engine.command, *args]
    try:
        return subprocess.run(
            argv,
            cwd=engine.cwd,
            text=True,
            capture_output=True,
            timeout=timeout,
            check=False,
        )
    except FileNotFoundError as exc:
        raise EngineError(f"engine command not found: {argv[0]} ({exc})") from exc
    except PermissionError as exc:
        raise EngineError(f"engine command not executable: {argv[0]} ({exc})") from exc
    except subprocess.TimeoutExpired as exc:
        raise EngineError(f"engine timed out after {timeout:.0f}s: {' '.join(argv)}") from exc


def engine_version(engine: Engine, timeout: float) -> str | None:
    try:
        proc = _run(engine, ["version"], timeout)
    except EngineError:
        return None
    if proc.returncode != 0:
        return None
    lines = [line.strip() for line in proc.stdout.splitlines() if line.strip()]
    return lines[-1] if lines else None


def run_engine_gate(
    engine: Engine, fixtures_path: Path, options: EngineOptions
) -> tuple[dict[str, Any], list[str], int]:
    """Run `nomos answer gate` and return (gate result, argv, exit code)."""
    args = ["answer", "gate", "--fixtures", str(fixtures_path), *options.scorer_argv()]
    argv = [*engine.command, *args]
    proc = _run(engine, args, options.timeout)
    if proc.returncode not in ENGINE_GATE_EXIT_CODES:
        raise EngineError(
            f"engine exited {proc.returncode} instead of a verdict: "
            f"{_tail(proc.stderr) or '<no stderr>'}"
        )
    try:
        result = json.loads(proc.stdout)
    except ValueError as exc:
        detail = _tail(proc.stderr) or _tail(proc.stdout) or "<empty output>"
        raise EngineError(
            f"engine produced no gate verdict JSON (exit {proc.returncode}): {detail}"
        ) from exc
    if (
        not isinstance(result, dict)
        or result.get("status") not in ("pass", "fail")
        or "verdicts" not in result
    ):
        raise EngineError("engine output is not a gate result (status/verdicts missing)")
    if not isinstance(result.get("gates"), dict):
        raise EngineError(
            "engine emitted no `gates` block: this sidecar consumes the thresholds "
            "from the verdict and refuses to guess them (engine too old?)"
        )
    return result, argv, proc.returncode


def align_verdicts(answers: list[dict[str, Any]], result: dict[str, Any]) -> dict[str, dict[str, Any]]:
    """Map every fixture answer to exactly one engine verdict, by answer_id."""
    verdicts: dict[str, dict[str, Any]] = {}
    for verdict in result.get("verdicts") or []:
        if not isinstance(verdict, dict):
            raise EngineError("engine verdict is not an object")
        verdict_id = str(verdict.get("answer_id", ""))
        if verdict_id in verdicts:
            raise EngineError(f"engine returned two verdicts for answer {verdict_id!r}")
        verdicts[verdict_id] = verdict
    ids = [str(answer.get("answer_id")) for answer in answers]
    missing = [answer_id for answer_id in ids if answer_id not in verdicts]
    if missing:
        raise EngineError("engine returned no verdict for answer(s): " + ", ".join(missing))
    extra = sorted(set(verdicts) - set(ids))
    if extra:
        raise EngineError("engine returned verdict(s) for unknown answer(s): " + ", ".join(extra))
    return verdicts


# --- evidence envelope (the sidecar's own responsibility) -------------------------


def _envelope_finding(code: str, path: str, message: str, severity: str = "error") -> dict[str, str]:
    return {
        "code": code,
        "severity": severity,
        "path": path,
        "message": message,
        "source": FINDING_SOURCE_ENVELOPE,
    }


def validate_envelope(answer: dict[str, Any], index: int) -> list[dict[str, str]]:
    """Record-shape checks the engine does not perform."""
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
            _envelope_finding(
                "MISSING_RAG_ANSWER_FIELD",
                f"{answer_id}:{field}",
                f"RAG answer evidence is missing required field {field}.",
            )
        )

    contract = response_contract(answer)
    if str(answer.get("policy_outcome", "")).strip() == "acceptable" and not contract["complete"]:
        missing = [field for field in RESPONSE_CONTRACT_FIELDS if not contract["fields"][field]]
        findings.append(
            _envelope_finding(
                "MISSING_RAG_RESPONSE_CONTRACT_FIELD",
                answer_id,
                "RAG answer is missing recommended response contract field(s): "
                + ", ".join(missing)
                + ".",
            )
        )

    confidence = answer.get("confidence")
    if not isinstance(confidence, (int, float)) or confidence < 0 or confidence > 1:
        findings.append(
            _envelope_finding(
                "INVALID_CONFIDENCE",
                f"{answer_id}:confidence",
                "Confidence must be a number between 0 and 1.",
            )
        )
    return findings


def validate_answer_ids(answers: list[dict[str, Any]]) -> list[dict[str, str]]:
    """Verdicts are aligned by answer_id: ids must be present and unique."""
    counts = Counter(str(answer.get("answer_id")) for answer in answers if present(answer.get("answer_id")))
    return [
        _envelope_finding(
            DUPLICATE_ANSWER_ID_CODE,
            answer_id,
            f"answer_id {answer_id!r} appears {count} times; verdicts cannot be aligned.",
        )
        for answer_id, count in sorted(counts.items())
        if count > 1
    ]


def answer_ids_alignable(answers: list[dict[str, Any]]) -> bool:
    return all(present(answer.get("answer_id")) for answer in answers) and not validate_answer_ids(answers)


# --- fallback proxy (`--engine fallback` only) ---------------------------------


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
    """Fallback proxy: recompute groundedness from the span text vs the answer.

    Returns a dict with a derived score, or None when grounding is genuinely not
    applicable (an explicit refusal, or an answer with no text to ground). An
    answer that requires grounding but has no span text scores 0
    (method=no_span_text) so it is blocked, never structurally inflated (#538).
    """
    answer_text = answer.get("answer")
    has_answer_text = isinstance(answer_text, str) and bool(answer_text.strip())
    support = _support_corpus(answer)

    if not support:
        if requires_grounding(answer):
            sentences = _sentences(answer_text) if has_answer_text else []
            return {
                "method": GROUNDEDNESS_METHOD_NO_TEXT,
                "score": 0.0,
                "supported_sentences": 0,
                "total_sentences": len(sentences),
                "reason": "answer requires grounding but no span text was provided to verify against",
            }
        return None

    if not has_answer_text:
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
    elif requires_grounding(answer):
        base = 0.0
    else:
        base = min(alce["citation_recall"], alce["citation_precision"])
    declared = answer.get("faithfulness_score")
    if isinstance(declared, (int, float)) and 0 <= declared <= 1:
        # A producer may declare itself LESS faithful, never more.
        base = min(base, float(declared))
    return round(base, 4)


def groundedness_detail(answer: dict[str, Any], alce: dict[str, float]) -> dict[str, Any]:
    recomputed = recompute_groundedness(answer)
    declared = answer.get("faithfulness_score")
    recomputed_from_spans = recomputed is not None and recomputed.get("method") == GROUNDEDNESS_METHOD
    detail: dict[str, Any] = {
        "recomputed_from_spans": recomputed_from_spans,
        "self_declared": declared if isinstance(declared, (int, float)) else None,
        "self_declared_trusted": False,
        "limitation": GROUNDEDNESS_LIMITATION,
        "upgrade": GROUNDEDNESS_UPGRADE,
    }
    if recomputed is not None:
        detail.update(recomputed)
    elif has_explicit_refusal(answer):
        # Parity with the engine (#624): a refusal asserts nothing, and says so.
        detail["method"] = GROUNDEDNESS_METHOD_REFUSAL
        detail["score"] = 1.0
    elif requires_grounding(answer):
        detail["method"] = GROUNDEDNESS_METHOD_NO_TEXT
        detail["score"] = 0.0
        detail["reason"] = "answer requires grounding but no span text was provided to verify against"
    else:
        detail["method"] = GROUNDEDNESS_METHOD_STRUCTURAL
        detail["score"] = round(min(alce["citation_recall"], alce["citation_precision"]), 4)
    return detail


def answer_metrics(answer: dict[str, Any]) -> dict[str, Any]:
    alce = citation_metrics(answer)
    faithfulness = faithfulness_score(answer, alce)
    confidence = answer.get("confidence")
    confidence_score = float(confidence) if isinstance(confidence, (int, float)) else 0.0
    confidence_score = min(1.0, max(0.0, confidence_score))
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


def summary_trust_tier(metrics: dict[str, Any], has_error_findings: bool, gates: dict[str, Any]) -> str:
    """Tier of a metrics block against the thresholds the verdicts were judged with."""
    if has_error_findings:
        return "unverified"
    alce = metrics["alce"]
    faithfulness = metrics["deepeval"]["faithfulness"]
    trust_score = metrics["trust_score"]
    if (
        alce["citation_recall"] >= gates["alce_gate"]
        and alce["citation_precision"] >= gates["alce_gate"]
        and faithfulness >= gates["faithfulness_gate"]
        and trust_score >= gates["trust_score_certified"]
    ):
        return "certified"
    if trust_score >= gates["trust_score_indicative"]:
        return "indicative"
    return "unverified"


def trust_tier(metrics: dict[str, Any], findings: list[dict[str, str]]) -> str:
    """Fallback proxy tier (mirror of the engine's trustTier)."""
    return summary_trust_tier(metrics, bool(findings), FALLBACK_GATES)


def cap_tier(tier: str, cap: str) -> str:
    return tier if TIER_ORDER.index(tier) <= TIER_ORDER.index(cap) else cap


def gate_findings_fallback(answer: dict[str, Any]) -> list[dict[str, str]]:
    """Fallback proxy: the gate findings the engine would raise, computed locally."""
    findings: list[dict[str, str]] = []

    def add(code: str, message: str) -> None:
        findings.append({"code": code, "severity": "error", "message": message})

    policy_outcome = str(answer.get("policy_outcome", "")).strip()
    if str(answer.get("citation_status", "")).strip() == "source_backed" and not has_source_backed_citation(answer):
        add(
            "SOURCE_BACKED_CITATION_WITHOUT_SOURCE_SPANS",
            "source_backed citation status requires source_id, source_hash, and span.",
        )
    if policy_outcome == "acceptable":
        if not (has_source_backed_citation(answer) or has_explicit_refusal(answer)):
            add(
                "ACCEPTABLE_WITHOUT_CITATION_OR_REFUSAL",
                "Acceptable RAG answers require source-backed citations or explicit refusal/unsupported state.",
            )
        metrics = answer_metrics(answer)
        if metrics["alce"]["citation_recall"] < ALCE_GATE:
            add(
                "ALCE_CITATION_RECALL_BELOW_GATE",
                "Retrieved chunks are not fully covered by source-backed citations.",
            )
        if metrics["alce"]["citation_precision"] < ALCE_GATE:
            add(
                "ALCE_CITATION_PRECISION_BELOW_GATE",
                "Citations include spans that do not bind to retrieved chunks.",
            )
        if metrics["deepeval"]["faithfulness"] < DEEPEVAL_FAITHFULNESS_GATE:
            add(
                "DEEPEVAL_FAITHFULNESS_BELOW_GATE",
                "Faithfulness score is below the cite-or-abstain gate.",
            )
    return findings


def validate_answer(answer: dict[str, Any], index: int) -> list[dict[str, str]]:
    """Envelope findings plus the fallback proxy's gate findings (with paths)."""
    answer_id = str(answer.get("answer_id") or f"answers[{index}]")
    findings = validate_envelope(answer, index)
    for finding in gate_findings_fallback(answer):
        findings.append({**finding, "path": answer_id, "source": VERDICT_SOURCE_FALLBACK})
    return findings


def fallback_verdict(answer: dict[str, Any]) -> dict[str, Any]:
    """The engine verdict shape, produced by the local proxy (`--engine fallback`)."""
    metrics = answer_metrics(answer)
    findings = gate_findings_fallback(answer)
    if has_explicit_refusal(answer):
        decision = "abstain"
    elif str(answer.get("policy_outcome", "")).strip() == "acceptable" and not findings:
        decision = "cite"
    else:
        decision = "abstain"
    return {
        "answer_id": answer.get("answer_id"),
        "decision": decision,
        "trust_tier": trust_tier(metrics, findings),
        "citation_recall": metrics["alce"]["citation_recall"],
        "citation_precision": metrics["alce"]["citation_precision"],
        "faithfulness": metrics["deepeval"]["faithfulness"],
        "trust_score": metrics["trust_score"],
        "groundedness": metrics["groundedness"],
        "findings": findings,
    }


# --- report ----------------------------------------------------------------------


def _metric(value: Any) -> float:
    return round(float(value), 4) if isinstance(value, (int, float)) else 0.0


def evidence_record(
    answer: dict[str, Any],
    verdict: dict[str, Any] | None,
    envelope_findings: list[dict[str, str]],
    verdict_source: str | None,
) -> dict[str, Any]:
    model = answer.get("model") if isinstance(answer.get("model"), dict) else {}
    record: dict[str, Any] = {
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
        "envelope_findings": [finding["code"] for finding in envelope_findings],
    }
    if verdict is None:
        # No verdict could be aligned (defective ids): nothing is trusted.
        record.update(
            {
                "decision": None,
                "metrics": None,
                "gate_findings": [],
                "trust_tier": "unverified",
            }
        )
        return record

    gate_findings = [f for f in verdict.get("findings") or [] if isinstance(f, dict)]
    groundedness = dict(verdict.get("groundedness") or {})
    groundedness.setdefault("upgrade", GROUNDEDNESS_UPGRADE)
    tier = str(verdict.get("trust_tier") or "unverified")
    if tier not in TIER_ORDER:
        tier = "unverified"
    if envelope_findings:
        tier = "unverified"
    if verdict_source == VERDICT_SOURCE_FALLBACK:
        tier = cap_tier(tier, FALLBACK_TIER_CAP)
    record.update(
        {
            "decision": verdict.get("decision"),
            "metrics": {
                "alce": {
                    "citation_recall": _metric(verdict.get("citation_recall")),
                    "citation_precision": _metric(verdict.get("citation_precision")),
                },
                "deepeval": {
                    "faithfulness": _metric(verdict.get("faithfulness")),
                },
                "groundedness": groundedness,
                "trust_score": _metric(verdict.get("trust_score")),
                "verdict_source": verdict_source,
            },
            "gate_findings": [str(f.get("code")) for f in gate_findings],
            "trust_tier": tier,
        }
    )
    return record


def _summary_metrics(records: list[dict[str, Any]]) -> dict[str, Any]:
    scored = [record["metrics"] for record in records if record.get("metrics")]
    if scored:
        n = len(scored)
        avg_recall = round(sum(m["alce"]["citation_recall"] for m in scored) / n, 4)
        avg_precision = round(sum(m["alce"]["citation_precision"] for m in scored) / n, 4)
        avg_faithfulness = round(sum(m["deepeval"]["faithfulness"] for m in scored) / n, 4)
        avg_trust_score = round(sum(m["trust_score"] for m in scored) / n, 4)
    else:
        avg_recall = avg_precision = avg_faithfulness = avg_trust_score = 0.0
    return {
        "alce": {
            "citation_recall": avg_recall,
            "citation_precision": avg_precision,
        },
        "deepeval": {
            "faithfulness": avg_faithfulness,
        },
        "trust_score": avg_trust_score,
    }


def build_report(root: Path, fixtures_path: Path, options: EngineOptions | None = None) -> dict[str, Any]:
    options = options or EngineOptions()
    if options.mode not in ENGINE_MODES:
        raise ValueError(f"unknown engine mode {options.mode!r} (expected one of {ENGINE_MODES})")
    fixture_doc = load_yaml(fixtures_path)
    answers = [answer for answer in as_list(fixture_doc.get("answers")) if isinstance(answer, dict)]

    findings: list[dict[str, str]] = []
    envelope_by_index: list[list[dict[str, str]]] = []
    for index, answer in enumerate(answers):
        envelope = validate_envelope(answer, index)
        envelope_by_index.append(envelope)
        findings.extend(envelope)
    findings.extend(validate_answer_ids(answers))

    engine_block: dict[str, Any] = {
        "mode": options.mode,
        "status": "not_run",
        "verdict_source": None,
        "origin": None,
        "command": None,
        "cwd": None,
        "nomos_version": None,
        "gate_status": None,
        "gate_exit_code": None,
        "scorer_configured": bool(options.scorer_cmd),
        "scorer_cmd": options.scorer_cmd or None,
        "fallback_reason": None,
        "not_run_reason": None,
    }
    verdicts: dict[str, dict[str, Any]] = {}
    gates: dict[str, Any] | None = None
    verdict_source: str | None = None
    limitation = GROUNDEDNESS_LIMITATION

    if answer_ids_alignable(answers):
        engine = resolve_engine(root, options.nomos_bin)
        error: str | None = None
        if engine is None:
            error = (
                "no engine found: pass --nomos-bin, set NOMOS_BIN, run from a checkout "
                "with `go` on PATH, or put `nomos` on PATH"
            )
        else:
            engine_block["origin"] = engine.origin
            engine_block["cwd"] = str(engine.cwd) if engine.cwd else None
            engine_block["command"] = [*engine.command, "answer", "gate", "--fixtures", str(fixtures_path), *options.scorer_argv()]
            try:
                result, argv, exit_code = run_engine_gate(engine, fixtures_path, options)
                verdicts = align_verdicts(answers, result)
            except EngineError as exc:
                error = str(exc)
            else:
                gates = dict(result["gates"])
                verdict_source = VERDICT_SOURCE_ENGINE
                engine_block.update(
                    {
                        "status": "verdict",
                        "verdict_source": VERDICT_SOURCE_ENGINE,
                        "command": argv,
                        "nomos_version": engine_version(engine, options.timeout),
                        "gate_status": result.get("status"),
                        "gate_exit_code": exit_code,
                    }
                )
        if error is not None:
            if options.mode == "required":
                raise EngineError(error)
            verdict_source = VERDICT_SOURCE_FALLBACK
            verdicts = {str(answer.get("answer_id")): fallback_verdict(answer) for answer in answers}
            gates = dict(FALLBACK_GATES)
            engine_block.update(
                {
                    "status": "fallback",
                    "verdict_source": VERDICT_SOURCE_FALLBACK,
                    "fallback_reason": error,
                }
            )
            findings.append(
                _envelope_finding(
                    FALLBACK_WARNING_CODE,
                    "engine",
                    "The Go engine was unavailable ("
                    + error
                    + "); verdicts come from the sidecar's lexical proxy (negation-blind, PARTIAL) "
                    "and every trust tier is capped at "
                    + FALLBACK_TIER_CAP
                    + ".",
                    severity="warning",
                )
            )
    else:
        engine_block["not_run_reason"] = (
            "answer ids are missing or duplicated: verdicts could not be aligned with the fixtures"
        )

    records: list[dict[str, Any]] = []
    for answer, envelope in zip(answers, envelope_by_index):
        verdict = verdicts.get(str(answer.get("answer_id")))
        record = evidence_record(answer, verdict, envelope, verdict_source)
        records.append(record)
        if verdict is not None:
            answer_id = str(answer.get("answer_id"))
            for finding in verdict.get("findings") or []:
                if not isinstance(finding, dict):
                    continue
                findings.append(
                    {
                        "code": str(finding.get("code")),
                        "severity": str(finding.get("severity") or "error"),
                        "path": answer_id,
                        "message": str(finding.get("message") or ""),
                        "source": verdict_source,
                    }
                )
            method_limitation = (verdict.get("groundedness") or {}).get("limitation")
            if isinstance(method_limitation, str) and method_limitation.strip():
                limitation = method_limitation

    has_error_findings = any(finding.get("severity") == "error" for finding in findings)
    fixture_counts = Counter(str(record.get("fixture_type", "unspecified")) for record in records)
    trust_counts = Counter(str(record.get("trust_tier", "unverified")) for record in records)
    decision_counts = Counter(str(record["decision"]) for record in records if record.get("decision"))
    summary_metrics = _summary_metrics(records)
    summary_tier = summary_trust_tier(summary_metrics, has_error_findings, gates or FALLBACK_GATES)
    if verdict_source == VERDICT_SOURCE_FALLBACK:
        summary_tier = cap_tier(summary_tier, FALLBACK_TIER_CAP)
    methods_observed = sorted(
        {
            str(record["metrics"]["groundedness"].get("method"))
            for record in records
            if record.get("metrics") and isinstance(record["metrics"].get("groundedness"), dict)
        }
    )
    effective_gates = gates or dict(FALLBACK_GATES)

    return {
        "schema_version": REPORT_SCHEMA_VERSION,
        "status": "failed" if has_error_findings else "generated",
        "generated_at_utc": utc_now(),
        "claim_boundary": CLAIM_BOUNDARY,
        "engine": engine_block,
        "gates": gates,
        "groundedness_method": {
            "name": GROUNDEDNESS_METHOD,
            "verdict_source": verdict_source,
            "methods_observed": methods_observed,
            "sentence_threshold": effective_gates.get("sentence_threshold"),
            "recomputed_from_span_text": True,
            "self_declared_score_can_only_lower": True,
            "no_span_text_rule": (
                "an answer that requires grounding but whose spans carry no text scores 0 "
                "(unverifiable) and is blocked; the declared score cannot raise it"
            ),
            "limitation": limitation,
            "upgrade": GROUNDEDNESS_UPGRADE,
        },
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
            "decisions": dict(sorted(decision_counts.items())),
            "metrics": summary_metrics,
            "trust_tiers": dict(sorted(trust_counts.items())),
            "trust_tier": summary_tier,
            "verdict_source": verdict_source,
            "findings": len(findings),
        },
        "answers": records,
        "findings": findings,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Emit and validate RAG answer evidence from the Go gate verdict.")
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument("--fixtures", default=str(DEFAULT_FIXTURES), help="RAG answer fixture YAML path.")
    parser.add_argument("--output", default=str(DEFAULT_OUTPUT), help="JSON evidence report path.")
    parser.add_argument(
        "--engine",
        choices=ENGINE_MODES,
        default="required",
        help="required (default): no verdict without the Go engine, exit 2 and no report; "
        "fallback: use the marked lexical proxy only when the engine is unavailable.",
    )
    parser.add_argument(
        "--nomos-bin",
        default=None,
        help="nomos binary to run `answer gate` with (default: $NOMOS_BIN, then `go run .` in <root>/cli, then PATH).",
    )
    parser.add_argument(
        "--engine-timeout",
        type=float,
        default=DEFAULT_ENGINE_TIMEOUT_SECONDS,
        help="seconds allowed for one engine invocation (`go run .` may compile first).",
    )
    parser.add_argument("--scorer-cmd", default=None, help="forwarded to the engine: external faithfulness scorer command (#622).")
    parser.add_argument("--scorer-threshold", type=float, default=None, help="forwarded to the engine.")
    parser.add_argument("--scorer-timeout", default=None, help="forwarded to the engine (Go duration, e.g. 2m).")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    output = resolve(root, args.output)
    options = EngineOptions(
        mode=args.engine,
        nomos_bin=args.nomos_bin,
        timeout=args.engine_timeout,
        scorer_cmd=args.scorer_cmd,
        scorer_threshold=args.scorer_threshold,
        scorer_timeout=args.scorer_timeout,
    )

    try:
        report = build_report(root, resolve(root, args.fixtures), options)
    except EngineError as exc:
        removed = False
        if output.exists():
            output.unlink()
            removed = True
        print(f"{ENGINE_UNAVAILABLE_CODE}: {exc}", file=sys.stderr)
        print(
            "no report written"
            + (" (stale report removed)" if removed else "")
            + ": --engine required (the default) refuses to score without the Go engine; "
            "--engine fallback opts into the marked lexical proxy.",
            file=sys.stderr,
        )
        return 2

    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report["summary"], indent=2, sort_keys=True))
    return 1 if report["status"] == "failed" else 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""HHEM-2.1-Open faithfulness scorer for `nomos answer gate|eval --scorer-cmd` (#622).

Reference adapter of the Nomos scorer protocol: reads a `nomos-scorer-request-v1`
JSON document on stdin, scores every (premise, hypothesis) pair with Vectara's
open hallucination evaluation model (HHEM-2.1-Open, Apache-2.0, Hugging Face
`vectara/hallucination_evaluation_model`), and writes a `nomos-scorer-response-v1`
document on stdout. Each score is the model's probability that the premise
(the retrieved/cited support text) supports the hypothesis (one answer
sentence). The Go gate combines it strictest-wins with its own lexical proxy,
so this adapter can only tighten a verdict, never loosen it.

Fail-closed by construction:

* the model backend (transformers/torch) is loaded at runtime; when it is not
  importable or the model cannot be loaded, the adapter exits 3 and writes
  NOTHING on stdout — the gate then fails the answer with
  FAITHFULNESS_SCORER_FAILED rather than falling back silently;
* a malformed request (schema, missing/duplicate/empty ids) exits 2;
* a backend score outside [0,1], not a number, or of the wrong count exits 4:
  a judge that answers outside its contract is refused, not clamped.

Claim boundary: this sidecar is exercised in CI only at the protocol level
(injected backend, backend unavailable). No CI run scores with the neural
model, and Nomos makes no claim about HHEM's accuracy on any corpus.

Usage:

    nomos answer gate --fixtures answers.yaml \
      --scorer-cmd "python3 scripts/nomos_hhem_scorer.py" --scorer-threshold 0.5

Requires `pip install transformers torch` (HHEM-2.1-Open loads with
trust_remote_code=True); the first run downloads the model into the Hugging
Face cache (override with HF_HOME).
"""

from __future__ import annotations

import argparse
import json
import math
import sys
from typing import Any, Callable, Sequence, TextIO

REQUEST_SCHEMA = "nomos-scorer-request-v1"
RESPONSE_SCHEMA = "nomos-scorer-response-v1"
DEFAULT_MODEL = "vectara/hallucination_evaluation_model"
METHOD = "hhem-2.1-open"

EXIT_OK = 0
EXIT_BAD_REQUEST = 2
EXIT_BACKEND_UNAVAILABLE = 3
EXIT_BAD_SCORE = 4

Backend = Callable[[Sequence[tuple[str, str]]], Sequence[Any]]


class ScorerError(Exception):
    """A refusal to score; exit_code is what the process returns."""

    exit_code = EXIT_BAD_REQUEST


class BackendUnavailable(ScorerError):
    exit_code = EXIT_BACKEND_UNAVAILABLE


class BadScore(ScorerError):
    exit_code = EXIT_BAD_SCORE


def load_backend(model_id: str) -> Backend:
    """Load HHEM-2.1-Open; raise BackendUnavailable rather than pretend."""
    try:
        from transformers import AutoModelForSequenceClassification  # type: ignore
    except Exception as err:  # ImportError, but also broken installs
        raise BackendUnavailable(f"transformers backend not importable: {err}") from err
    try:
        model = AutoModelForSequenceClassification.from_pretrained(model_id, trust_remote_code=True)
    except Exception as err:
        raise BackendUnavailable(f"cannot load model {model_id!r}: {err}") from err

    def predict(pairs: Sequence[tuple[str, str]]) -> Sequence[Any]:
        # HHEM-2.1-Open exposes predict([(premise, hypothesis), ...]) and
        # returns one consistency probability per pair.
        return list(model.predict([(premise, hypothesis) for premise, hypothesis in pairs]))

    return predict


def parse_request(raw: str) -> list[dict[str, str]]:
    try:
        doc = json.loads(raw)
    except json.JSONDecodeError as err:
        raise ScorerError(f"request is not JSON: {err}") from err
    if not isinstance(doc, dict) or doc.get("schema_version") != REQUEST_SCHEMA:
        raise ScorerError(f"request schema is not {REQUEST_SCHEMA}")
    pairs = doc.get("pairs")
    if not isinstance(pairs, list):
        raise ScorerError("request carries no pairs list")
    seen: set[str] = set()
    out: list[dict[str, str]] = []
    for index, pair in enumerate(pairs):
        if not isinstance(pair, dict):
            raise ScorerError(f"pair {index}: not an object")
        pair_id = pair.get("id")
        premise = pair.get("premise")
        hypothesis = pair.get("hypothesis")
        if not isinstance(pair_id, str) or not pair_id.strip():
            raise ScorerError(f"pair {index}: empty id")
        if pair_id in seen:
            raise ScorerError(f"pair {index}: duplicate id {pair_id!r}")
        if not isinstance(premise, str) or not isinstance(hypothesis, str):
            raise ScorerError(f"pair {pair_id!r}: premise and hypothesis must be strings")
        seen.add(pair_id)
        out.append({"id": pair_id, "premise": premise, "hypothesis": hypothesis})
    return out


def score_pairs(pairs: list[dict[str, str]], backend: Backend, batch_size: int) -> list[dict[str, Any]]:
    """Score in batches; refuse any score outside the contract."""
    out: list[dict[str, Any]] = []
    for start in range(0, len(pairs), batch_size):
        batch = pairs[start : start + batch_size]
        raw = list(backend([(p["premise"], p["hypothesis"]) for p in batch]))
        if len(raw) != len(batch):
            raise BadScore(f"backend returned {len(raw)} score(s) for {len(batch)} pair(s)")
        for pair, value in zip(batch, raw):
            try:
                score = float(value)
            except (TypeError, ValueError) as err:
                raise BadScore(f"pair {pair['id']!r}: score {value!r} is not a number") from err
            if math.isnan(score) or math.isinf(score) or score < 0.0 or score > 1.0:
                raise BadScore(f"pair {pair['id']!r}: score {score!r} is outside [0,1]")
            out.append({"id": pair["id"], "score": score})
    return out


def run(
    argv: Sequence[str],
    stdin: TextIO,
    stdout: TextIO,
    stderr: TextIO,
    backend_loader: Callable[[str], Backend] = load_backend,
) -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--model", default=DEFAULT_MODEL, help="Hugging Face model id (default: HHEM-2.1-Open)")
    parser.add_argument("--batch-size", type=int, default=16, help="pairs per backend call")
    args = parser.parse_args(list(argv))
    try:
        pairs = parse_request(stdin.read())
        scores: list[dict[str, Any]] = []
        if pairs:
            backend = backend_loader(args.model)
            scores = score_pairs(pairs, backend, max(1, args.batch_size))
    except ScorerError as err:
        # Nothing on stdout: the gate must see a failure, not a partial answer.
        print(f"nomos_hhem_scorer: {err}", file=stderr)
        return err.exit_code
    json.dump({"schema_version": RESPONSE_SCHEMA, "method": METHOD, "scores": scores}, stdout)
    stdout.write("\n")
    return EXIT_OK


def main() -> int:
    return run(sys.argv[1:], sys.stdin, sys.stdout, sys.stderr)


if __name__ == "__main__":
    raise SystemExit(main())

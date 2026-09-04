"""#622 — the HHEM scorer adapter of the Nomos scorer protocol.

Exercised at the PROTOCOL level only: an injected backend proves the exchange
and the contract checks; the real model is never loaded in CI, and the whole
chain (`nomos answer gate --scorer-cmd`) is proved to fail closed when no
model backend is available.
"""

from __future__ import annotations

import importlib.util
import io
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CLI = ROOT / "cli"
SCRIPT = ROOT / "scripts/nomos_hhem_scorer.py"

# Never let a developer machine with transformers installed reach the network.
OFFLINE_ENV = {"HF_HUB_OFFLINE": "1", "TRANSFORMERS_OFFLINE": "1", "HF_HUB_DISABLE_TELEMETRY": "1"}
MISSING_MODEL = "nomos-test/model-that-does-not-exist"


def _load():
    spec = importlib.util.spec_from_file_location("nomos_hhem_scorer", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


MOD = _load()


def request(pairs: list[dict[str, str]], schema: str = MOD.REQUEST_SCHEMA) -> str:
    return json.dumps({"schema_version": schema, "pairs": pairs})


def fake_backend(pairs):
    # Recognises the negation the lexical proxy cannot see.
    return [0.05 if " ne " in f" {hyp} " or " pas " in f" {hyp} " else 0.95 for _, hyp in pairs]


def run(argv, stdin_text, loader):
    out, err = io.StringIO(), io.StringIO()
    rc = MOD.run(argv, io.StringIO(stdin_text), out, err, backend_loader=loader)
    return rc, out.getvalue(), err.getvalue()


PAIRS = [
    {"id": "s0", "premise": "Le delai court des la notification.", "hypothesis": "Le delai court des la notification"},
    {"id": "s1", "premise": "Le delai court des la notification.", "hypothesis": "Le delai ne court pas des la notification"},
]


class HHEMScorerAdapterTests(unittest.TestCase):
    def test_protocol_round_trip_with_injected_backend(self) -> None:
        rc, out, err = run([], request(PAIRS), lambda model: fake_backend)
        self.assertEqual(rc, 0, err)
        doc = json.loads(out)
        self.assertEqual(doc["schema_version"], MOD.RESPONSE_SCHEMA)
        self.assertEqual(doc["method"], MOD.METHOD)
        self.assertEqual([s["id"] for s in doc["scores"]], ["s0", "s1"])
        self.assertEqual([s["score"] for s in doc["scores"]], [0.95, 0.05])

    def test_backend_unavailable_exits_nonzero_and_emits_no_scores(self) -> None:
        def unavailable(model):
            raise MOD.BackendUnavailable("transformers backend not importable")

        rc, out, err = run([], request(PAIRS), unavailable)
        self.assertEqual(rc, MOD.EXIT_BACKEND_UNAVAILABLE)
        self.assertEqual(out, "", "nothing may reach stdout when the judge is absent")
        self.assertIn("not importable", err)

    def test_bad_request_schema_is_refused(self) -> None:
        rc, out, _ = run([], request(PAIRS, schema="nomos-scorer-request-v0"), lambda model: fake_backend)
        self.assertEqual(rc, MOD.EXIT_BAD_REQUEST)
        self.assertEqual(out, "")

    def test_duplicate_or_empty_pair_id_is_refused(self) -> None:
        dup = PAIRS + [dict(PAIRS[0])]
        rc, out, _ = run([], request(dup), lambda model: fake_backend)
        self.assertEqual(rc, MOD.EXIT_BAD_REQUEST)
        self.assertEqual(out, "")
        empty = [dict(PAIRS[0], id="  ")]
        rc, out, _ = run([], request(empty), lambda model: fake_backend)
        self.assertEqual(rc, MOD.EXIT_BAD_REQUEST)
        self.assertEqual(out, "")

    def test_backend_score_out_of_range_is_refused(self) -> None:
        for bad in (1.7, -0.1, float("nan"), "high"):
            with self.subTest(score=bad):
                rc, out, err = run([], request(PAIRS), lambda model, bad=bad: (lambda pairs: [bad] * len(pairs)))
                self.assertEqual(rc, MOD.EXIT_BAD_SCORE)
                self.assertEqual(out, "", "an out-of-contract score is refused, never clamped")

    def test_backend_score_count_mismatch_is_refused(self) -> None:
        rc, out, _ = run([], request(PAIRS), lambda model: (lambda pairs: [0.9]))
        self.assertEqual(rc, MOD.EXIT_BAD_SCORE)
        self.assertEqual(out, "")

    def test_empty_request_does_not_load_the_model(self) -> None:
        def must_not_load(model):
            raise AssertionError("the model must not be loaded for an empty batch")

        rc, out, _ = run([], request([]), must_not_load)
        self.assertEqual(rc, 0)
        self.assertEqual(json.loads(out)["scores"], [])

    def test_subprocess_without_model_fails_closed_offline(self) -> None:
        # Whether transformers is installed or not, an offline load of a
        # non-existent model cannot succeed: exit 3, empty stdout.
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--model", MISSING_MODEL],
            input=request(PAIRS),
            text=True,
            capture_output=True,
            env={**os.environ, **OFFLINE_ENV},
            check=False,
            timeout=300,
        )
        self.assertEqual(result.returncode, MOD.EXIT_BACKEND_UNAVAILABLE, result.stderr)
        self.assertEqual(result.stdout, "")

    def test_gate_with_unavailable_scorer_fails_closed(self) -> None:
        """The full chain: `nomos answer gate --scorer-cmd <this adapter>` with
        no model available must NOT fall back to the lexical verdict."""
        if shutil.which("go") is None:
            raise unittest.SkipTest("go not on PATH — the chain runs the real CLI")
        if any(ch.isspace() for ch in f"{sys.executable}{SCRIPT}"):
            raise unittest.SkipTest("--scorer-cmd is whitespace-split; paths with spaces are not testable here")
        fixture = (
            "answers:\n"
            "  - answer_id: A1\n"
            "    prompt_id: P1\n"
            '    answer: "Le gabarit retient une hauteur de neuf metres au faite."\n'
            "    citation_status: source_backed\n"
            "    policy_outcome: acceptable\n"
            "    confidence: 0.99\n"
            "    source_spans:\n"
            "      - source_id: S1\n"
            '        source_hash: "sha256:abc"\n'
            "        span: L1-L2\n"
            "        chunk_id: c1\n"
            '        text: "Le gabarit retient une hauteur de neuf metres au faite pour le volume principal."\n'
            "    retrieved_chunks:\n"
            "      - chunk_id: c1\n"
            '        text: "Le gabarit retient une hauteur de neuf metres au faite."\n'
        )
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "answers.yaml"
            path.write_text(fixture, encoding="utf-8")
            scorer_cmd = f"{sys.executable} {SCRIPT} --model {MISSING_MODEL}"
            result = subprocess.run(
                ["go", "run", ".", "answer", "gate", "--fixtures", str(path), "--scorer-cmd", scorer_cmd],
                cwd=CLI,
                text=True,
                capture_output=True,
                env={**os.environ, **OFFLINE_ENV},
                check=False,
                timeout=600,
            )
        self.assertEqual(result.returncode, 1, result.stderr)
        verdicts = json.loads(result.stdout)["verdicts"]
        codes = [f["code"] for f in verdicts[0]["findings"]]
        self.assertIn("FAITHFULNESS_SCORER_FAILED", codes)
        self.assertEqual(verdicts[0]["decision"], "abstain")
        self.assertEqual(verdicts[0]["groundedness"]["method"], "scorer_failed")


if __name__ == "__main__":
    unittest.main()

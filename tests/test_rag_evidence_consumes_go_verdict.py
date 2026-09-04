"""#624 — the RAG answer evidence sidecar CONSUMES the Go gate verdict.

Doc 45 A1 / VRC-10: « le sidecar Python devient un consommateur du verdict Go,
pas la source ». The proofs are adversarial (doctrine §2.3):

* required mode (the default, what CI runs) refuses to score without the
  engine: no engine, a crashing engine, an engine that prints no verdict — each
  exits 2 and leaves NO report behind, not even a stale one;
* the report follows the engine, not the local proxy: with an NLI second judge
  forwarded to the engine, a negated claim the lexical proxy accepts is blocked
  in the evidence pack;
* fallback mode is explicit, marked, and caps the trust tier;
* on the real fixtures the engine verdict and the fallback proxy agree
  (parity), so the fallback cannot drift from the engine unnoticed.
"""

from __future__ import annotations

import importlib.util
import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
CLI = ROOT / "cli"
SCRIPT = ROOT / "scripts" / "regulated_rag_answer_evidence.py"
REAL_FIXTURES = ROOT / "docs/regulated/ai-rag-governance/rag-answer-fixtures.yaml"
SUPPORT = "A governed answer must cite the retained source spans or abstain entirely."


def _load_sidecar():
    spec = importlib.util.spec_from_file_location("rag_evidence_sidecar", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    # The sidecar declares PEP 563 annotations; dataclasses resolves them
    # through sys.modules, so register the module before executing it.
    sys.modules["rag_evidence_sidecar"] = module
    spec.loader.exec_module(module)
    return module


SIDECAR = _load_sidecar()


def answer_record(
    answer_text: str,
    *,
    answer_id: str = "ANS-1",
    span_text: str | None = SUPPORT,
    declared: float | None = 0.96,
) -> dict:
    chunk = {"chunk_id": "CHUNK-001", "source_id": "SRC-001", "source_hash": "a" * 64, "span": "lines:1-3"}
    span = dict(chunk)
    if span_text is not None:
        chunk["text"] = span_text
        span["text"] = span_text
    record = {
        "answer_id": answer_id,
        "prompt_id": "PROMPT-1",
        "fixture_type": "citation",
        "answer": answer_text,
        "structured_facts": [{"unit_id": "RULE-001", "source": "read_model"}],
        "citations": [{"source_id": "SRC-001", "locator": "lines:1-3", "chunk_id": "CHUNK-001"}],
        "uncertainties": [],
        "requires_human_decision": False,
        "model": {"provider": "fixture-provider", "name": "fixture-model", "version": "2026-05-14"},
        "retrieved_chunks": [chunk],
        "source_spans": [span],
        "citation_status": "source_backed",
        "refusal_status": "not_refused",
        "confidence": 0.95,
        "policy_outcome": "acceptable",
    }
    if declared is not None:
        record["faithfulness_score"] = declared
    return record


def write_fixtures(directory: Path, answers: list[dict]) -> Path:
    path = directory / "docs/regulated/ai-rag-governance/rag-answer-fixtures.yaml"
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        yaml.safe_dump({"schema_version": "0.1.0", "answers": answers}, sort_keys=False, allow_unicode=True),
        encoding="utf-8",
    )
    return path


def write_executable(path: Path, body: str) -> Path:
    path.write_text(body, encoding="utf-8")
    path.chmod(path.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
    return path


def run_sidecar(root: Path, fixtures: Path, output: Path, *extra: str) -> subprocess.CompletedProcess[str]:
    env = dict(os.environ)
    env.pop("NOMOS_BIN", None)  # the tests decide how the engine is found
    return subprocess.run(
        [sys.executable, str(SCRIPT), "--root", str(root), "--fixtures", str(fixtures), "--output", str(output), *extra],
        cwd=root,
        text=True,
        capture_output=True,
        check=False,
        env=env,
    )


def record_metrics(record: dict) -> dict[str, float]:
    metrics = record["metrics"]
    return {
        "citation_recall": metrics["alce"]["citation_recall"],
        "citation_precision": metrics["alce"]["citation_precision"],
        "faithfulness": metrics["deepeval"]["faithfulness"],
        "trust_score": metrics["trust_score"],
    }


CRASHING_ENGINE = "#!/bin/sh\necho 'engine exploded' >&2\nexit 3\n"
MUTE_ENGINE = "#!/bin/sh\necho 'not a verdict'\nexit 0\n"
FAKE_NLI_SCORER = '''#!/usr/bin/env python3
"""A second judge for the test: any negated hypothesis is unsupported."""
import json
import re
import sys

request = json.load(sys.stdin)
if request.get("schema_version") != "nomos-scorer-request-v1":
    sys.exit(2)
scores = []
for pair in request["pairs"]:
    negated = re.search(r"\\b(not|never|no)\\b", pair["hypothesis"], re.IGNORECASE) is not None
    scores.append({"id": pair["id"], "score": 0.05 if negated else 0.95})
print(json.dumps({"schema_version": "nomos-scorer-response-v1", "method": "fake-negation-nli", "scores": scores}))
'''


class AlignVerdictsTests(unittest.TestCase):
    """Scores are attributed BY ID, never by position."""

    def _answers(self, *ids: str) -> list[dict]:
        return [answer_record("A governed answer must cite the retained source spans.", answer_id=i) for i in ids]

    def _verdicts(self, *ids: str) -> dict[str, Any]:
        result = {"status": "pass", "gates": {}, "verdicts": [{"answer_id": i} for i in ids]}
        return result

    def test_alignment_is_by_id_not_by_position(self) -> None:
        answers = self._answers("ANS-A", "ANS-B")
        shuffled = {"status": "pass", "verdicts": [{"answer_id": "ANS-B"}, {"answer_id": "ANS-A"}]}
        aligned = SIDECAR.align_verdicts(answers, shuffled)
        self.assertEqual({key: value["answer_id"] for key, value in aligned.items()}, {"ANS-A": "ANS-A", "ANS-B": "ANS-B"})

    def test_missing_verdict_is_an_engine_failure(self) -> None:
        answers = self._answers("ANS-A", "ANS-B")
        with self.assertRaises(SIDECAR.EngineError) as caught:
            SIDECAR.align_verdicts(answers, self._verdicts("ANS-A"))
        self.assertIn("ANS-B", str(caught.exception))

    def test_verdict_for_an_unknown_answer_is_an_engine_failure(self) -> None:
        answers = self._answers("ANS-A")
        with self.assertRaises(SIDECAR.EngineError) as caught:
            SIDECAR.align_verdicts(answers, self._verdicts("ANS-A", "ANS-GHOST"))
        self.assertIn("ANS-GHOST", str(caught.exception))

    def test_two_verdicts_for_one_answer_is_an_engine_failure(self) -> None:
        answers = self._answers("ANS-A")
        with self.assertRaises(SIDECAR.EngineError):
            SIDECAR.align_verdicts(answers, self._verdicts("ANS-A", "ANS-A"))


class RequiredModeFailsClosedTests(unittest.TestCase):
    """No engine ⇒ no verdict ⇒ no report. These run without Go."""

    def test_required_mode_without_engine_writes_no_report(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fixtures = write_fixtures(root, [answer_record("A governed answer must cite the retained source spans.")])
            output = root / "out" / "rag-answer-evidence.json"
            output.parent.mkdir()
            output.write_text('{"status": "generated", "stale": true}\n', encoding="utf-8")

            result = run_sidecar(root, fixtures, output, "--nomos-bin", str(root / "no-such-nomos"))

            self.assertEqual(result.returncode, 2, result.stderr + result.stdout)
            self.assertFalse(output.exists(), "a stale report must not survive an engine failure")
            self.assertIn("NOMOS_ENGINE_UNAVAILABLE", result.stderr)
            self.assertIn("no report written", result.stderr)
            self.assertNotIn("python_fallback", result.stdout + result.stderr)

    def test_required_mode_engine_failure_never_falls_back(self) -> None:
        for name, body in {"crash": CRASHING_ENGINE, "no_verdict": MUTE_ENGINE}.items():
            with self.subTest(engine=name), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp)
                fixtures = write_fixtures(root, [answer_record("A governed answer must cite the retained source spans.")])
                engine = write_executable(root / "fake-nomos", body)
                output = root / "out" / "rag-answer-evidence.json"

                result = run_sidecar(root, fixtures, output, "--nomos-bin", str(engine))

                self.assertEqual(result.returncode, 2, result.stderr + result.stdout)
                self.assertFalse(output.exists(), "an engine failure must not produce a report")
                self.assertIn("NOMOS_ENGINE_UNAVAILABLE", result.stderr)
                self.assertNotIn("python_fallback", result.stdout + result.stderr)

    def test_required_mode_refuses_an_engine_that_emits_no_thresholds(self) -> None:
        # A verdict without its `gates` block is not consumable: the report
        # would have to invent the thresholds it was judged against.
        legacy = '''#!/bin/sh
cat <<'JSON'
{"status": "pass", "verdicts": [
  {"answer_id": "ANS-1", "decision": "cite", "trust_tier": "certified",
   "citation_recall": 1.0, "citation_precision": 1.0, "faithfulness": 1.0,
   "trust_score": 1.0, "groundedness": {"method": "lexical_entailment_v1", "score": 1.0},
   "findings": []}
]}
JSON
'''
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fixtures = write_fixtures(root, [answer_record("A governed answer must cite the retained source spans.")])
            engine = write_executable(root / "legacy-nomos", legacy)
            output = root / "out" / "rag-answer-evidence.json"

            result = run_sidecar(root, fixtures, output, "--nomos-bin", str(engine))

            self.assertEqual(result.returncode, 2, result.stderr + result.stdout)
            self.assertFalse(output.exists())
            self.assertIn("gates", result.stderr)
            self.assertIn("NOMOS_ENGINE_UNAVAILABLE", result.stderr)

    def test_unaligned_answer_ids_are_reported_without_the_engine(self) -> None:
        # A fixture defect, not an engine failure: the report says why and the
        # engine is never invoked (the binary path here does not exist).
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            twin_a = answer_record("A governed answer must cite the retained source spans.", answer_id="ANS-TWIN")
            twin_b = answer_record("A governed answer must cite the retained source spans.", answer_id="ANS-TWIN")
            fixtures = write_fixtures(root, [twin_a, twin_b])
            output = root / "out" / "rag-answer-evidence.json"

            result = run_sidecar(root, fixtures, output, "--nomos-bin", str(root / "no-such-nomos"))

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "failed")
            self.assertEqual(report["engine"]["status"], "not_run")
            self.assertIn("DUPLICATE_RAG_ANSWER_ID", {f["code"] for f in report["findings"]})
            for record in report["answers"]:
                self.assertIsNone(record["metrics"])
                self.assertEqual(record["trust_tier"], "unverified")


class FallbackModeTests(unittest.TestCase):
    def test_fallback_mode_is_explicit_and_marked(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            fixtures = write_fixtures(root, [answer_record("A governed answer must cite the retained source spans.")])
            output = root / "out" / "rag-answer-evidence.json"

            result = run_sidecar(root, fixtures, output, "--engine", "fallback", "--nomos-bin", str(root / "no-such-nomos"))

            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "generated")
            engine = report["engine"]
            self.assertEqual(engine["status"], "fallback")
            self.assertEqual(engine["verdict_source"], "python_fallback")
            self.assertTrue(engine["fallback_reason"])
            warnings = [f for f in report["findings"] if f["severity"] == "warning"]
            self.assertEqual([f["code"] for f in warnings], ["RAG_GATE_VERDICT_FROM_PYTHON_FALLBACK"])
            record = report["answers"][0]
            self.assertEqual(record["metrics"]["verdict_source"], "python_fallback")
            self.assertGreaterEqual(record["metrics"]["deepeval"]["faithfulness"], 0.95)
            # A grounded answer would be certified by the engine; the proxy is
            # PARTIAL and never awards more than indicative.
            self.assertEqual(record["trust_tier"], "indicative")
            self.assertEqual(report["summary"]["trust_tier"], "indicative")
            self.assertEqual(report["summary"]["verdict_source"], "python_fallback")
            self.assertEqual(report["gates"]["faithfulness_gate"], 0.95)


@unittest.skipIf(shutil.which("go") is None, "go not on PATH — these tests run the real engine")
class EngineVerdictTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls._tmp = tempfile.TemporaryDirectory()
        cls.nomos = Path(cls._tmp.name) / "nomos"
        build = subprocess.run(
            ["go", "build", "-o", str(cls.nomos), "."],
            cwd=CLI,
            text=True,
            capture_output=True,
            check=False,
        )
        if build.returncode != 0:
            raise AssertionError(f"engine build failed: {build.stderr}{build.stdout}")

    @classmethod
    def tearDownClass(cls) -> None:
        cls._tmp.cleanup()

    def test_report_follows_the_engine_verdict_not_the_python_proxy(self) -> None:
        if " " in sys.executable:
            self.skipTest("--scorer-cmd is whitespace-split; interpreter paths with spaces are not testable here")
        negated = answer_record("A governed answer must not cite the retained source spans.", answer_id="ANS-NEGATED")
        # The lexical proxy alone accepts the negation: every content token of
        # the answer is in the support text (negation-blind by construction).
        proxy = SIDECAR.answer_metrics(negated)
        self.assertGreaterEqual(proxy["deepeval"]["faithfulness"], 0.95)
        self.assertEqual(SIDECAR.gate_findings_fallback(negated), [])

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            scorer = write_executable(root / "fake_nli.py", FAKE_NLI_SCORER)
            fixtures = write_fixtures(root, [negated])
            output = root / "out" / "rag-answer-evidence.json"

            result = run_sidecar(
                root, fixtures, output,
                "--nomos-bin", str(self.nomos),
                "--scorer-cmd", f"{sys.executable} {scorer}",
            )

            self.assertEqual(result.returncode, 1, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertEqual(report["status"], "failed")
            self.assertEqual(report["engine"]["verdict_source"], "go_engine")
            self.assertTrue(report["engine"]["scorer_configured"])
            self.assertTrue(report["gates"]["scorer_configured"])
            record = report["answers"][0]
            self.assertEqual(record["metrics"]["verdict_source"], "go_engine")
            self.assertLess(record["metrics"]["deepeval"]["faithfulness"], 0.95)
            self.assertEqual(record["decision"], "abstain")
            self.assertEqual(record["trust_tier"], "unverified")
            self.assertIn("fake-negation-nli", record["metrics"]["groundedness"]["method"])
            codes = {(f["code"], f["source"]) for f in report["findings"]}
            self.assertIn(("DEEPEVAL_FAITHFULNESS_BELOW_GATE", "go_engine"), codes)

    def test_engine_block_records_provenance_and_refusals_carry_an_explicit_method(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / "rag-answer-evidence.json"
            result = run_sidecar(ROOT, REAL_FIXTURES, output, "--nomos-bin", str(self.nomos))
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))

        engine = report["engine"]
        self.assertEqual(engine["status"], "verdict")
        self.assertEqual(engine["verdict_source"], "go_engine")
        self.assertEqual(engine["origin"], "--nomos-bin")
        self.assertEqual(engine["gate_status"], "pass")
        self.assertEqual(engine["gate_exit_code"], 0)
        self.assertFalse(engine["scorer_configured"])
        self.assertEqual(engine["command"][:3], [str(self.nomos), "answer", "gate"])
        version = subprocess.run([str(self.nomos), "version"], text=True, capture_output=True, check=True).stdout.strip()
        self.assertEqual(engine["nomos_version"], version)

        self.assertEqual(report["gates"]["faithfulness_gate"], 0.95)
        self.assertEqual(report["groundedness_method"]["verdict_source"], "go_engine")
        self.assertIn("negation-blind", report["groundedness_method"]["limitation"].lower())
        self.assertIn("nli", report["groundedness_method"]["upgrade"].lower())

        refusals = [record for record in report["answers"] if record["explicit_refusal_or_unsupported"]]
        self.assertTrue(refusals, "the real fixtures carry refusal answers")
        for record in refusals:
            self.assertEqual(record["metrics"]["groundedness"]["method"], "explicit_refusal")
            self.assertEqual(record["decision"], "abstain")
        for record in report["answers"]:
            self.assertEqual(record["metrics"]["verdict_source"], "go_engine")
        self.assertEqual(report["summary"]["verdict_source"], "go_engine")
        self.assertEqual(report["summary"]["decisions"], {"abstain": 3, "cite": 1})

    def test_fallback_proxy_agrees_with_the_engine_on_the_real_fixtures(self) -> None:
        # Parity guard: the PARTIAL fallback must not drift from the engine.
        fixture_doc = yaml.safe_load(REAL_FIXTURES.read_text(encoding="utf-8"))
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / "rag-answer-evidence.json"
            result = run_sidecar(ROOT, REAL_FIXTURES, output, "--nomos-bin", str(self.nomos))
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))

        by_id = {record["answer_id"]: record for record in report["answers"]}
        self.assertEqual(len(by_id), len(fixture_doc["answers"]))
        for answer in fixture_doc["answers"]:
            engine_record = by_id[answer["answer_id"]]
            proxy = SIDECAR.fallback_verdict(answer)
            with self.subTest(answer=answer["answer_id"]):
                self.assertEqual(record_metrics(engine_record), {
                    key: proxy[key] for key in ("citation_recall", "citation_precision", "faithfulness", "trust_score")
                })
                self.assertEqual(engine_record["trust_tier"], proxy["trust_tier"])
                self.assertEqual(engine_record["decision"], proxy["decision"])
                self.assertEqual(engine_record["metrics"]["groundedness"]["method"], proxy["groundedness"]["method"])

    def test_engine_resolution_defaults_to_go_run_in_the_checkout(self) -> None:
        # No --nomos-bin and no NOMOS_BIN: the sidecar builds and runs the
        # engine of the checkout it ships with — the default for a developer.
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / "rag-answer-evidence.json"
            result = run_sidecar(ROOT, REAL_FIXTURES, output)
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
            report = json.loads(output.read_text(encoding="utf-8"))
        self.assertTrue(report["engine"]["origin"].startswith("go run"), report["engine"])
        self.assertEqual(report["engine"]["verdict_source"], "go_engine")


if __name__ == "__main__":
    unittest.main()

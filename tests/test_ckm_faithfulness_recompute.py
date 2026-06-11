"""CKM-H6 — the cite-or-abstain gate recomputes faithfulness from the retrieved
spans and must not trust a self-declared score.

Audit (#518): CKM-08 was REAL but the faithfulness sub-score was auto-declared.
The proof here is adversarial (doctrine §2.3): an answer that declares a high
faithfulness_score while its claims are absent from the retrieved span text must
make the gate FAIL. Trusting the declared score would let it pass — so the
failure is the proof.
"""

from __future__ import annotations

import importlib.util
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
_SPEC = importlib.util.spec_from_file_location(
    "rag_gate", ROOT / "scripts" / "regulated_rag_answer_evidence.py"
)
gate = importlib.util.module_from_spec(_SPEC)
assert _SPEC.loader is not None
_SPEC.loader.exec_module(gate)


GOVERNANCE_SPAN = "Governed answers must cite the retained source spans or abstain entirely."


def make_answer(answer_text: str, declared: float, span_text: str) -> dict:
    return {
        "answer_id": "ANS-TEST",
        "prompt_id": "P-1",
        "model": {"provider": "x", "name": "y", "version": "z"},
        "answer": answer_text,
        "structured_facts": [{"unit_id": "U1", "source": "read_model"}],
        "citations": [{"source_id": "S1", "locator": "lines:1-3", "chunk_id": "C1"}],
        "uncertainties": [],
        "requires_human_decision": False,
        "retrieved_chunks": [
            {
                "chunk_id": "C1",
                "source_id": "S1",
                "source_hash": "a" * 64,
                "span": "lines:1-3",
                "text": span_text,
            }
        ],
        "source_spans": [
            {
                "source_id": "S1",
                "source_hash": "a" * 64,
                "span": "lines:1-3",
                "chunk_id": "C1",
                "text": span_text,
            }
        ],
        "citation_status": "source_backed",
        "refusal_status": "not_refused",
        "confidence": 0.9,
        "faithfulness_score": declared,
        "policy_outcome": "acceptable",
    }


class FaithfulnessRecomputeTests(unittest.TestCase):
    def test_declared_high_but_unsupported_fails_the_gate(self) -> None:
        # High self-declared faithfulness, but the answer's claims are nowhere in
        # the retrieved span text.
        answer = make_answer(
            "Quarterly revenue grew twelve percent driven by new enterprise contracts in Asia.",
            declared=0.99,
            span_text=GOVERNANCE_SPAN,
        )
        findings = gate.validate_answer(answer, 0)
        codes = {f["code"] for f in findings}
        self.assertIn(
            "DEEPEVAL_FAITHFULNESS_BELOW_GATE",
            codes,
            f"gate trusted the self-declared score; findings={findings}",
        )
        metrics = gate.answer_metrics(answer)
        # The gated value reflects the recomputation, not the 0.99 declaration.
        self.assertLess(metrics["deepeval"]["faithfulness"], 0.95)
        self.assertFalse(metrics["groundedness"]["self_declared_trusted"])
        self.assertTrue(metrics["groundedness"]["recomputed_from_spans"])

    def test_supported_answer_passes_the_gate(self) -> None:
        # Same shape, but the answer IS grounded in the span text.
        answer = make_answer(
            "Governed answers must cite the retained source spans or abstain.",
            declared=0.96,
            span_text=GOVERNANCE_SPAN,
        )
        findings = gate.validate_answer(answer, 0)
        codes = {f["code"] for f in findings}
        self.assertNotIn("DEEPEVAL_FAITHFULNESS_BELOW_GATE", codes, findings)
        metrics = gate.answer_metrics(answer)
        self.assertGreaterEqual(metrics["deepeval"]["faithfulness"], 0.95)
        self.assertTrue(metrics["groundedness"]["recomputed_from_spans"])

    def test_declared_score_can_only_lower_never_raise(self) -> None:
        # Recomputed groundedness is ~0; a 0.99 declaration must not raise it.
        answer = make_answer(
            "Unrelated hallucinated claim about maritime tariffs.",
            declared=0.99,
            span_text=GOVERNANCE_SPAN,
        )
        self.assertLess(gate.faithfulness_score(answer, gate.citation_metrics(answer)), 0.1)

    def test_recompute_groundedness_unit(self) -> None:
        grounded = make_answer(
            "Governed answers must cite the retained source spans.",
            declared=0.9,
            span_text=GOVERNANCE_SPAN,
        )
        ungrounded = make_answer(
            "The stock market rallied on semiconductor earnings.",
            declared=0.9,
            span_text=GOVERNANCE_SPAN,
        )
        g = gate.recompute_groundedness(grounded)
        u = gate.recompute_groundedness(ungrounded)
        self.assertIsNotNone(g)
        self.assertIsNotNone(u)
        self.assertEqual(g["method"], "lexical_entailment_v1")
        self.assertGreaterEqual(g["score"], 0.95)
        self.assertLess(u["score"], 0.5)

    def test_no_span_text_on_grounding_required_answer_scores_zero_and_blocks(self) -> None:
        # CKM-H6-FU (#538): the no-span-text bypass is closed. An answer that
        # requires grounding (acceptable, non-refusal, has text) but whose spans
        # carry NO text can no longer fall back to a ~1.0 structural score; it is
        # unverifiable, scores 0, and is blocked.
        answer = make_answer("Anything at all.", declared=0.99, span_text=GOVERNANCE_SPAN)
        for collection in ("retrieved_chunks", "source_spans"):
            for item in answer[collection]:
                item.pop("text", None)
        metrics = gate.answer_metrics(answer)
        self.assertFalse(metrics["groundedness"]["recomputed_from_spans"])
        self.assertEqual(metrics["groundedness"]["method"], "no_span_text")
        self.assertEqual(metrics["groundedness"]["score"], 0.0)
        self.assertFalse(metrics["groundedness"]["self_declared_trusted"])
        self.assertEqual(metrics["deepeval"]["faithfulness"], 0.0)
        findings = gate.validate_answer(answer, 0)
        codes = {f["code"] for f in findings}
        self.assertIn("DEEPEVAL_FAITHFULNESS_BELOW_GATE", codes, findings)

    def test_no_span_text_declared_score_cannot_raise_above_zero(self) -> None:
        # The "declared score may only lower, never raise" property holds even in
        # the no-text case: a 0.99 declaration cannot lift the 0 floor.
        answer = make_answer("Some factual assertion about revenue.", declared=0.99, span_text=GOVERNANCE_SPAN)
        for collection in ("retrieved_chunks", "source_spans"):
            for item in answer[collection]:
                item.pop("text", None)
        self.assertEqual(gate.faithfulness_score(answer, gate.citation_metrics(answer)), 0.0)

    def test_groundedness_limitation_is_documented_in_detail(self) -> None:
        # The lexical-proxy negation-blindness limitation and the NLI upgrade must
        # be surfaced in the gate output.
        answer = make_answer("Governed answers must cite the retained source spans.", declared=0.96, span_text=GOVERNANCE_SPAN)
        detail = gate.groundedness_detail(answer, gate.citation_metrics(answer))
        self.assertIn("negation-blind", detail["limitation"].lower())
        self.assertIn("nli", detail["upgrade"].lower())


if __name__ == "__main__":
    unittest.main()

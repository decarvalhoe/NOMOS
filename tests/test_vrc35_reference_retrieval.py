from __future__ import annotations

import json
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
CLI = ROOT / "cli"
CORPUS = ROOT / "cli/internal/corpus/testdata/aec-golden-corpus/vd-lausanne"
HARNESS = ROOT / "docs/regulated/domain-packs/built-environment/retrieval-harness.yaml"
PRESETS = ROOT / "docs/regulated/domain-packs/built-environment/aec-lens-presets"
SCRIPT = ROOT / "scripts/nomos_reference_retrieval.py"
COMMIT = "0123456789abcdef0123456789abcdef01234567"


def emit_real_bundle(tmp: Path) -> Path:
    """Emit a REAL bundle from the golden corpus through the actual CLI —
    the harness consumes the artifact contract, never the raw files."""
    corpus_copy = tmp / "corpus"
    shutil.copytree(CORPUS, corpus_copy)
    out = tmp / "out" / "bundle.json"
    out.parent.mkdir()
    result = subprocess.run(
        [
            "go", "run", ".", "bundle",
            "--root", str(corpus_copy),
            "--bundle-id", "aec-golden-retrieval",
            "--repo", "example/aec-golden",
            "--commit", COMMIT,
            "--out", str(out),
        ],
        cwd=CLI,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise AssertionError(f"bundle emission failed: {result.stderr}{result.stdout}")
    return out


def run_kit(bundle: Path, harness: Path) -> tuple[int, dict]:
    result = subprocess.run(
        [
            "python", str(SCRIPT),
            "--bundle", str(bundle),
            "--harness", str(harness),
            "--presets-dir", str(PRESETS),
        ],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    try:
        verdict = json.loads(result.stdout)
    except json.JSONDecodeError as exc:  # pragma: no cover - diagnostic aid
        raise AssertionError(f"kit emitted no verdict: {exc}\n{result.stdout}\n{result.stderr}")
    return result.returncode, verdict


class VRC35ReferenceRetrievalTests(unittest.TestCase):
    """VRC-35 (#572, B1) — « Lens enforced avant génération, au niveau base —
    mesuré » : the consumer kit replays the pack harness on a REAL emitted
    bundle; the anti-distractor gap is measured against VERSIONED thresholds,
    and a lens-excluded chunk is never retrieved at any rank."""

    bundle_path: Path

    @classmethod
    def setUpClass(cls) -> None:
        if shutil.which("go") is None:
            raise unittest.SkipTest("go not on PATH — the kit consumes a real emitted bundle")
        cls._tmp = tempfile.TemporaryDirectory()
        cls.bundle_path = emit_real_bundle(Path(cls._tmp.name))

    @classmethod
    def tearDownClass(cls) -> None:
        cls._tmp.cleanup()

    def test_gate_passes_with_versioned_thresholds(self) -> None:
        code, verdict = run_kit(self.bundle_path, HARNESS)
        self.assertEqual(code, 0, f"the reference gate is red: {verdict.get('failures')}")
        self.assertTrue(verdict["pass"])
        thresholds = yaml.safe_load(HARNESS.read_text(encoding="utf-8"))["thresholds"]
        self.assertEqual(verdict["thresholds"], thresholds, "thresholds must be the versioned ones")
        self.assertGreaterEqual(verdict["accuracy_with_lens"], thresholds["min_accuracy_with_lens"])
        self.assertLessEqual(verdict["accuracy_without_lens"], thresholds["max_accuracy_without_lens"])
        self.assertGreaterEqual(verdict["margin"], thresholds["min_margin"])

    def test_distractors_hijack_without_lens_and_lens_restores(self) -> None:
        """The measured anti-parasite effect: at least two queries where the
        raw lexical winner is the WRONG document and the lens restores the
        right one — accuracy avec Lens > sans Lens, not just equal."""
        _, verdict = run_kit(self.bundle_path, HARNESS)
        restored = [
            q for q in verdict["queries"]
            if q["with_lens"]["hit"] and not q["without_lens"]["hit"]
        ]
        self.assertGreaterEqual(
            len(restored), 2,
            f"expected measurable distractor hijacks, got: "
            f"{[(q['id'], q['without_lens']['top']) for q in verdict['queries']]}",
        )
        self.assertGreater(verdict["accuracy_with_lens"], verdict["accuracy_without_lens"])

    def test_excluded_chunk_is_never_retrieved(self) -> None:
        """The confidential journal is the best RAW lexical match for its
        query by construction — through the public-enquête lens it must not
        appear at ANY rank."""
        _, verdict = run_kit(self.bundle_path, HARNESS)
        conf = next(q for q in verdict["queries"] if q["id"] == "q-confidentiel-jamais-public")
        # The trap is real: raw retrieval surfaces the confidential document…
        self.assertEqual(conf["without_lens"]["top"], "journal-interne.md")
        # …and the lens keeps it out at every rank, not merely off the top.
        self.assertTrue(conf["never_retrieve_ok"])
        self.assertNotIn("journal-interne.md", conf["with_lens"]["ranking"])
        self.assertEqual(conf["with_lens"]["top"], "permis.md")

    def test_tampered_thresholds_fail_closed(self) -> None:
        """Adversarial: if reality drifts below the versioned thresholds the
        gate goes red — proven by demanding an impossible margin."""
        harness = yaml.safe_load(HARNESS.read_text(encoding="utf-8"))
        harness["thresholds"]["min_margin"] = 2.0
        with tempfile.NamedTemporaryFile(
            "w", suffix=".yaml", delete=False, encoding="utf-8", dir=self._tmp.name
        ) as handle:
            yaml.safe_dump(harness, handle, allow_unicode=True)
            tampered = Path(handle.name)
        code, verdict = run_kit(self.bundle_path, tampered)
        self.assertEqual(code, 1, "an unreachable threshold must turn the gate red")
        self.assertFalse(verdict["pass"])
        self.assertTrue(any("margin" in f for f in verdict["failures"]))


if __name__ == "__main__":
    unittest.main()

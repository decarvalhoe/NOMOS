from __future__ import annotations

import json
import shutil
import subprocess
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
LENS_SPEC = ROOT / "specs/knowledge-lens.cue"
LENS_FIXTURE = ROOT / "specs/examples/knowledge-lens.valid.yaml"
CANDIDATE_FIXTURE = ROOT / "specs/examples/knowledge-lens-candidates.valid.yaml"
FILTER_SCRIPT = ROOT / "scripts/ckm_knowledge_lens_filter.py"


class CKMKnowledgeLensTests(unittest.TestCase):
    def test_lens_contract_validates_presets_and_epistemology(self) -> None:
        self.assertTrue(LENS_SPEC.exists(), f"Missing lens CUE contract: {LENS_SPEC}")
        self.assertTrue(LENS_FIXTURE.exists(), f"Missing lens fixture: {LENS_FIXTURE}")

        lens = yaml.safe_load(LENS_FIXTURE.read_text(encoding="utf-8"))
        self.assertEqual(lens["record_type"], "ckm_knowledge_lens_bundle")
        self.assertEqual(lens["default_behavior"], "include_all_when_no_lens")
        self.assertEqual(lens["epistemology"]["applicability"], "objective_fact")
        self.assertEqual(lens["epistemology"]["activation"], "subjective_choice")
        self.assertIn("architect-permit-review", {item["id"] for item in lens["presets"]})

        if shutil.which("cue") is None:
            self.skipTest("cue is not installed")

        result = subprocess.run(
            [
                "cue",
                "vet",
                "specs/atomization-spine.cue",
                "specs/facets.cue",
                "specs/knowledge-lens.cue",
                "specs/examples/knowledge-lens.valid.yaml",
                "-d",
                "#KnowledgeLensBundle",
            ],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_no_lens_preserves_base_retrieval_scope(self) -> None:
        result = self._filter()

        self.assertEqual(result["mode"], "no_lens")
        self.assertEqual(result["stage"], "base_filter")
        self.assertEqual(
            result["included_ids"],
            ["CHUNK-LENS-APPLICABLE", "CHUNK-LENS-PLAUSIBLE-NOT-APPLICABLE"],
        )
        self.assertEqual(result["excluded"], [])

    def test_lens_excludes_plausible_non_applicable_before_generation(self) -> None:
        result = self._filter("--lens", str(LENS_FIXTURE), "--preset", "architect-permit-review")

        self.assertEqual(result["mode"], "lens")
        self.assertEqual(result["stage"], "base_filter")
        self.assertEqual(result["included_ids"], ["CHUNK-LENS-APPLICABLE"])
        self.assertEqual(
            result["excluded"],
            [
                {
                    "id": "CHUNK-LENS-PLAUSIBLE-NOT-APPLICABLE",
                    "reason": "excluded_by_facets.applicability",
                }
            ],
        )

    def _filter(self, *extra_args: str) -> dict:
        self.assertTrue(
            CANDIDATE_FIXTURE.exists(),
            f"Missing candidate fixture: {CANDIDATE_FIXTURE}",
        )
        self.assertTrue(FILTER_SCRIPT.exists(), f"Missing filter script: {FILTER_SCRIPT}")
        result = subprocess.run(
            [
                "python",
                str(FILTER_SCRIPT),
                "--candidates",
                str(CANDIDATE_FIXTURE),
                *extra_args,
            ],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        return json.loads(result.stdout)


if __name__ == "__main__":
    unittest.main()

from __future__ import annotations

import json
import subprocess
import tempfile
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts/pack_core_coupling_check.py"
PACK_MANIFEST = ROOT / "docs/regulated/domain-packs/built-environment/pack.yaml"
METRIC = ROOT / "docs/regulated/domain-packs/built-environment/reproducibility-metric.yaml"


def run_check(*args: str) -> tuple[int, dict]:
    result = subprocess.run(
        ["python", str(SCRIPT), *args],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )
    return result.returncode, json.loads(result.stdout)


class VRC38PackCoreCouplingTests(unittest.TestCase):
    """VRC-38 (#575, D6) — « reproductible = métrique, pas promesse » : the
    coupling instrument measures 0 on the real pack, blocks an unjustified
    core touch in a pack diff, and accepts the ADR escape hatch."""

    def test_real_pack_measures_zero_core_changes(self) -> None:
        code, verdict = run_check("--manifest", str(PACK_MANIFEST))
        self.assertEqual(code, 0, verdict)
        self.assertEqual(verdict["core_changes_required"], 0)
        self.assertEqual(verdict["core_paths_required"], [])
        self.assertTrue(verdict["pass"])

    def test_published_metric_matches_the_computed_value(self) -> None:
        """The pack publishes the metric — it must be the COMPUTED value,
        never a declared wish."""
        _, verdict = run_check("--manifest", str(PACK_MANIFEST))
        published = yaml.safe_load(METRIC.read_text(encoding="utf-8"))
        self.assertEqual(published["metric"], "core_changes_required")
        self.assertEqual(published["value"], verdict["core_changes_required"])
        self.assertEqual(published["target"], 0)

    def _changed(self, paths: list[str]) -> Path:
        handle = tempfile.NamedTemporaryFile(
            "w", suffix=".txt", delete=False, encoding="utf-8", dir=self._dir.name
        )
        handle.write("\n".join(paths) + "\n")
        handle.close()
        return Path(handle.name)

    @classmethod
    def setUpClass(cls) -> None:
        cls._dir = tempfile.TemporaryDirectory()

    @classmethod
    def tearDownClass(cls) -> None:
        cls._dir.cleanup()

    def test_pack_only_diff_passes(self) -> None:
        listing = self._changed([
            "docs/regulated/domain-packs/eu-ai-act/pack.yaml",
            "docs/regulated/domain-packs/eu-ai-act/vocab.yaml",
            "cli/internal/corpus/testdata/eu-ai-act/case.md",
        ])
        code, verdict = run_check("--changed-files", str(listing))
        self.assertEqual(code, 0, verdict)
        self.assertEqual(verdict["core_changes"], 0)
        self.assertEqual(verdict["verdict"], "pack-only")

    def test_unjustified_core_touch_blocks_and_names_the_path(self) -> None:
        listing = self._changed([
            "docs/regulated/domain-packs/eu-ai-act/pack.yaml",
            "cli/internal/app/pack_cmd.go",
        ])
        code, verdict = run_check("--changed-files", str(listing))
        self.assertEqual(code, 1, "an unjustified core change in a pack PR must block")
        self.assertIn("cli/internal/app/pack_cmd.go", verdict["core_paths_touched"])
        self.assertIn("WITHOUT ADR", verdict["verdict"])

    def test_adr_justification_is_the_escape_hatch(self) -> None:
        listing = self._changed([
            "docs/regulated/domain-packs/eu-ai-act/pack.yaml",
            "cli/internal/app/pack_cmd.go",
            "docs/adr/0007-eu-ai-act-core-touch.md",
        ])
        code, verdict = run_check("--changed-files", str(listing))
        self.assertEqual(code, 0, "an ADR-justified core change must pass (review reads the ADR)")
        self.assertEqual(verdict["adr_justifications"], ["docs/adr/0007-eu-ai-act-core-touch.md"])
        self.assertIn("justified", verdict["verdict"])
        # The core path is still REPORTED — justification is not invisibility.
        self.assertIn("cli/internal/app/pack_cmd.go", verdict["core_paths_touched"])


if __name__ == "__main__":
    unittest.main()

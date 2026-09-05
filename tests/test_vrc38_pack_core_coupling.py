from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts/pack_core_coupling_check.py"
PACK_MANIFEST = ROOT / "docs/regulated/domain-packs/built-environment/pack.yaml"
METRIC = ROOT / "docs/regulated/domain-packs/built-environment/reproducibility-metric.yaml"
WORKFLOW = ROOT / ".github/workflows/pack-coupling.yml"
CI_WORKFLOW = ROOT / ".github/workflows/ci.yml"


def run_check(*args: str) -> tuple[int, dict]:
    result = subprocess.run(
        [sys.executable, str(SCRIPT), *args],
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
        code, verdict = run_check(
            "--changed-files", str(listing), "--pack-label-present", "true"
        )
        self.assertEqual(code, 0, verdict)
        self.assertEqual(verdict["core_changes"], 0)
        self.assertEqual(verdict["verdict"], "pack-only")

    def test_unjustified_core_touch_blocks_and_names_the_path(self) -> None:
        listing = self._changed([
            "docs/regulated/domain-packs/eu-ai-act/pack.yaml",
            "cli/internal/app/pack_cmd.go",
        ])
        code, verdict = run_check(
            "--changed-files", str(listing), "--pack-label-present", "true"
        )
        self.assertEqual(code, 1, "an unjustified core change in a pack PR must block")
        self.assertIn("cli/internal/app/pack_cmd.go", verdict["core_paths_touched"])
        self.assertIn("WITHOUT ADR", verdict["verdict"])

    def test_adr_justification_is_the_escape_hatch(self) -> None:
        listing = self._changed([
            "docs/regulated/domain-packs/eu-ai-act/pack.yaml",
            "cli/internal/app/pack_cmd.go",
            "docs/adr/0007-eu-ai-act-core-touch.md",
        ])
        code, verdict = run_check(
            "--changed-files", str(listing), "--pack-label-present", "true"
        )
        self.assertEqual(code, 0, "an ADR-justified core change must pass (review reads the ADR)")
        self.assertEqual(verdict["adr_justifications"], ["docs/adr/0007-eu-ai-act-core-touch.md"])
        self.assertIn("justified", verdict["verdict"])
        # The core path is still REPORTED — justification is not invisibility.
        self.assertIn("cli/internal/app/pack_cmd.go", verdict["core_paths_touched"])

    def test_pack_path_without_label_blocks_and_names_the_path(self) -> None:
        listing = self._changed([
            "docs/regulated/domain-packs/eu-ai-act/pack.yaml",
        ])
        code, verdict = run_check(
            "--changed-files", str(listing), "--pack-label-present", "false"
        )
        self.assertEqual(code, 1, "forgetting the pack label must never bypass D6")
        self.assertTrue(verdict["missing_pack_label"])
        self.assertFalse(verdict["coupling_evaluated"])
        self.assertEqual(
            verdict["pack_marker_paths_touched"],
            ["docs/regulated/domain-packs/eu-ai-act/pack.yaml"],
        )
        self.assertIn("WITHOUT required 'pack' label", verdict["verdict"])

    def test_adr_cannot_waive_a_missing_pack_label(self) -> None:
        listing = self._changed([
            "docs/regulated/domain-packs/eu-ai-act/pack.yaml",
            "docs/adr/0004-pack-change.md",
        ])
        code, verdict = run_check(
            "--changed-files", str(listing), "--pack-label-present", "false"
        )
        self.assertEqual(code, 1)
        self.assertTrue(verdict["missing_pack_label"])
        self.assertEqual(verdict["adr_justifications"], ["docs/adr/0004-pack-change.md"])

    def test_unlabelled_non_pack_diff_is_visibly_not_applicable(self) -> None:
        listing = self._changed(["cli/internal/answer/answer.go"])
        code, verdict = run_check(
            "--changed-files", str(listing), "--pack-label-present", "false"
        )
        self.assertEqual(code, 0, verdict)
        self.assertFalse(verdict["coupling_evaluated"])
        self.assertFalse(verdict["missing_pack_label"])
        self.assertIn("not applicable", verdict["verdict"])

    def test_changed_files_requires_explicit_label_state(self) -> None:
        listing = self._changed(["docs/regulated/domain-packs/eu-ai-act/pack.yaml"])
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--changed-files", str(listing)],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 2)
        self.assertIn("--pack-label-present is required", result.stderr)

    def test_manifest_mode_rejects_label_state(self) -> None:
        result = subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                "--manifest",
                str(PACK_MANIFEST),
                "--pack-label-present",
                "true",
            ],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 2)
        self.assertIn("only valid with --changed-files", result.stderr)

    def test_dedicated_workflow_covers_label_lifecycle_without_skipping(self) -> None:
        workflow = yaml.load(WORKFLOW.read_text(encoding="utf-8"), Loader=yaml.BaseLoader)
        pull_request = workflow["on"]["pull_request"]
        self.assertEqual(
            set(pull_request["types"]),
            {"opened", "synchronize", "reopened", "edited", "labeled", "unlabeled"},
        )
        self.assertNotIn("paths", pull_request)
        job = workflow["jobs"]["pack-coupling"]
        self.assertNotIn("if", job, "the guard must report, not skip")
        text = WORKFLOW.read_text(encoding="utf-8")
        for marker in (
            "github.event.pull_request.base.sha",
            "github.event.pull_request.head.sha",
            "github.event.pull_request.labels.*.name, 'pack'",
            "--no-renames",
            "--pack-label-present",
        ):
            self.assertIn(marker, text)
        ci = CI_WORKFLOW.read_text(encoding="utf-8")
        self.assertNotIn("  pack-coupling:\n", ci)
        self.assertNotIn("types: [opened, synchronize, reopened, edited, labeled, unlabeled]", ci)

    def test_published_metric_points_to_the_always_reporting_workflow(self) -> None:
        published = yaml.safe_load(METRIC.read_text(encoding="utf-8"))
        self.assertIn(".github/workflows/pack-coupling.yml", published["enforced_by"])
        self.assertIn("pack-tree changes require label", published["enforced_by"])


if __name__ == "__main__":
    unittest.main()

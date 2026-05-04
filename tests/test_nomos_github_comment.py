"""Tests for scripts/nomos_github_comment.py.

NGW-06 (#391): the source PR commenter. Every test mocks
subprocess.run so no real `gh api` call is made; the only outbound
contracts under test are the format/marker invariants and the
disable/short-circuit semantics of the CLI.
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

REPO_ROOT = Path(__file__).resolve().parent.parent
SCRIPTS_DIR = REPO_ROOT / "scripts"
SCRIPT_PATH = SCRIPTS_DIR / "nomos_github_comment.py"

if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

import nomos_github_comment as ngc  # noqa: E402


class TestCommentDisabled(unittest.TestCase):
    def test_comment_disabled_when_block_missing(self):
        self.assertTrue(ngc.comment_disabled({}))
        self.assertTrue(ngc.comment_disabled(None))
        self.assertTrue(ngc.comment_disabled({"source_pr_comment": None}))

    def test_comment_disabled_when_enabled_false(self):
        cfg = {"source_pr_comment": {"enabled": False}}
        self.assertTrue(ngc.comment_disabled(cfg))

    def test_comment_enabled_when_true(self):
        cfg = {"source_pr_comment": {"enabled": True, "mode": "summary"}}
        self.assertFalse(ngc.comment_disabled(cfg))


class TestStickyMarker(unittest.TestCase):
    def test_marker_format(self):
        self.assertEqual(
            ngc.sticky_marker("rbok-lawbook"),
            "<!-- nomos-source-pr-comment:rbok-lawbook -->",
        )

    def test_marker_rejects_empty_scope(self):
        with self.assertRaises(ValueError):
            ngc.sticky_marker("")

    def test_sticky_marker_first_line(self):
        # Every rendered body starts with marker + blank line.
        body = ngc.format_comment(
            mode="summary",
            include=list(ngc.INCLUDE_KEYS),
            diff_plan={"impacted": [{"id": "s", "changed_paths": ["a.md"]}]},
            trace_manifest_ref="nomos-trace.yaml",
            gate_status="pass",
            output_location="https://example/out",
            scope_id="s",
        )
        first, second, *_ = body.split("\n", 2)
        self.assertEqual(first, ngc.sticky_marker("s"))
        self.assertEqual(second, "")


class TestSummaryMode(unittest.TestCase):
    def test_summary_mode_format(self):
        diff = {
            "impacted": [
                {"id": "rbok-lawbook", "changed_paths": ["a.md", "b.md", "c.md"]}
            ]
        }
        body = ngc.format_comment(
            mode="summary",
            include=list(ngc.INCLUDE_KEYS),
            diff_plan=diff,
            trace_manifest_ref="nomos-trace.yaml",
            gate_status="pass",
            output_location="https://example/out",
            scope_id="rbok-lawbook",
        )
        self.assertIn("rbok-lawbook", body)
        self.assertIn("**pass**", body)
        self.assertIn("changed paths: 3", body)
        # Summary must NOT enumerate individual paths.
        self.assertNotIn("`a.md`", body)


class TestDetailedMode(unittest.TestCase):
    def test_detailed_mode_includes_paths(self):
        paths = [f"docs/file-{i}.md" for i in range(5)]
        diff = {"impacted": [{"id": "s", "changed_paths": paths}]}
        body = ngc.format_comment(
            mode="detailed",
            include=list(ngc.INCLUDE_KEYS),
            diff_plan=diff,
            trace_manifest_ref="nomos-trace.yaml",
            gate_status="pass",
            output_location="https://example/out",
            scope_id="s",
        )
        for p in paths:
            self.assertIn(p, body)

    def test_detailed_mode_truncates_above_20(self):
        paths = [f"docs/file-{i}.md" for i in range(25)]
        diff = {"impacted": [{"id": "s", "changed_paths": paths}]}
        body = ngc.format_comment(
            mode="detailed",
            include=list(ngc.INCLUDE_KEYS),
            diff_plan=diff,
            trace_manifest_ref="nomos-trace.yaml",
            gate_status="pass",
            output_location=None,
            scope_id="s",
        )
        for p in paths[:20]:
            self.assertIn(p, body)
        for p in paths[20:]:
            # Truncated paths must not appear in the rendered output.
            self.assertNotIn(p, body)
        self.assertIn("... +5 more", body)


class TestFailuresOnlyMode(unittest.TestCase):
    def test_failures_only_pass_status_emits_minimal(self):
        body = ngc.format_comment(
            mode="failures_only",
            include=list(ngc.INCLUDE_KEYS),
            diff_plan={"impacted": [{"id": "s", "changed_paths": ["a"]}]},
            trace_manifest_ref="nomos-trace.yaml",
            gate_status="pass",
            output_location="https://example/out",
            scope_id="s",
        )
        # Marker is always the first line; body shows the "all clear"
        # placeholder so the sticky comment stays alive instead of being
        # orphaned at the previous failed state.
        self.assertTrue(body.startswith(ngc.sticky_marker("s")))
        self.assertIn("No findings", body)
        self.assertIn("**pass**", body)
        # Detailed sub-blocks must NOT appear.
        self.assertNotIn("Diff summary", body)
        self.assertNotIn("Output location", body)

    def test_failures_only_fail_status_renders_full(self):
        body = ngc.format_comment(
            mode="failures_only",
            include=list(ngc.INCLUDE_KEYS),
            diff_plan={"impacted": [{"id": "s", "changed_paths": ["a"]}]},
            trace_manifest_ref="nomos-trace.yaml",
            gate_status="fail",
            output_location="https://example/out",
            scope_id="s",
        )
        self.assertIn("**fail**", body)
        self.assertIn("Output location", body)


class TestIncludeFilter(unittest.TestCase):
    def test_include_filter_drops_unselected(self):
        body = ngc.format_comment(
            mode="summary",
            include=["changed_scopes"],
            diff_plan={"impacted": [{"id": "s", "changed_paths": ["a"]}]},
            trace_manifest_ref="nomos-trace.yaml",
            gate_status="pass",
            output_location="https://example/out",
            scope_id="s",
        )
        self.assertIn("Impacted scopes", body)
        # Other sub-blocks must NOT appear.
        for header in (
            "Diff summary",
            "Output location",
            "Trace manifest",
            "Gate status",
        ):
            self.assertNotIn(header, body)

    def test_unknown_include_keys_ignored(self):
        body = ngc.format_comment(
            mode="summary",
            include=["changed_scopes", "made-up-key"],
            diff_plan={"impacted": [{"id": "s", "changed_paths": ["a"]}]},
            trace_manifest_ref="nomos-trace.yaml",
            gate_status="pass",
            output_location=None,
            scope_id="s",
        )
        self.assertIn("Impacted scopes", body)


class TestFindExistingComment(unittest.TestCase):
    def test_match_by_marker(self):
        marker = ngc.sticky_marker("rbok-lawbook")
        api_response = [
            {"id": 1, "body": "unrelated comment"},
            {"id": 2, "body": f"{marker}\n\nbody"},
            {"id": 3, "body": "<!-- nomos-source-pr-comment:other -->\n\n…"},
        ]
        match = ngc.find_existing_comment(api_response, "rbok-lawbook")
        self.assertIsNotNone(match)
        self.assertEqual(match["id"], 2)

    def test_no_match_returns_none(self):
        marker = ngc.sticky_marker("other-scope")
        api_response = [{"id": 1, "body": f"{marker}\n\n…"}]
        self.assertIsNone(ngc.find_existing_comment(api_response, "rbok-lawbook"))

    def test_empty_response_returns_none(self):
        self.assertIsNone(ngc.find_existing_comment([], "rbok-lawbook"))
        self.assertIsNone(ngc.find_existing_comment(None, "rbok-lawbook"))


class TestPostOrUpdateDryRun(unittest.TestCase):
    def test_post_or_update_dry_run_no_subprocess(self):
        with mock.patch("nomos_github_comment.subprocess.run") as run:
            plan = ngc.post_or_update_comment(
                repo="o/r",
                pr_number=12,
                body="hello",
                scope_id="s",
                dry_run=True,
            )
        run.assert_not_called()
        self.assertEqual(plan["action"], "create")
        self.assertTrue(plan["dry_run"])
        self.assertEqual(plan["scope_id"], "s")

    def test_post_or_update_real_creates_when_no_match(self):
        # subprocess.run is called twice: list comments + create.
        list_proc = mock.Mock(stdout="[]", returncode=0)
        post_proc = mock.Mock(stdout="{}", returncode=0)
        with mock.patch(
            "nomos_github_comment.subprocess.run",
            side_effect=[list_proc, post_proc],
        ) as run:
            plan = ngc.post_or_update_comment(
                repo="o/r", pr_number=12, body="hello", scope_id="s"
            )
        self.assertEqual(plan["action"], "create")
        self.assertEqual(run.call_count, 2)
        post_args = run.call_args_list[1].args[0]
        self.assertIn("POST", post_args)

    def test_post_or_update_real_updates_when_match(self):
        marker = ngc.sticky_marker("s")
        existing = json.dumps([{"id": 99, "body": f"{marker}\n\nold"}])
        list_proc = mock.Mock(stdout=existing, returncode=0)
        patch_proc = mock.Mock(stdout="{}", returncode=0)
        with mock.patch(
            "nomos_github_comment.subprocess.run",
            side_effect=[list_proc, patch_proc],
        ) as run:
            plan = ngc.post_or_update_comment(
                repo="o/r", pr_number=12, body="new", scope_id="s"
            )
        self.assertEqual(plan["action"], "update")
        self.assertEqual(plan["comment_id"], 99)
        patch_args = run.call_args_list[1].args[0]
        self.assertIn("PATCH", patch_args)
        self.assertIn("repos/o/r/issues/comments/99", patch_args)


class TestCLI(unittest.TestCase):
    def _write_config(self, tmp: Path, *, enabled: bool, has_block: bool = True) -> Path:
        if has_block:
            notify_block = {
                "source_pr_comment": {
                    "enabled": enabled,
                    "mode": "summary",
                    "include": [
                        "changed_scopes",
                        "diff_summary",
                        "output_location",
                        "trace_manifest",
                        "gate_status",
                    ],
                }
            }
        else:
            notify_block = {}
        config = {
            "schema_version": "0.1.0",
            "workflows": [
                {
                    "id": "rbok-lawbook",
                    "source": {
                        "repo": "RBOKproject/realisons-business",
                        "base_branch": "main",
                        "paths": ["01_rbok/**"],
                    },
                    "output": {
                        "repo": "RBOKproject/nomos-rbok-artifacts",
                        "branch": "main",
                        "path": "rbok-lawbook/",
                    },
                    "nomos": {
                        "corpus_id": "rbok-lawbook",
                        "project_id": "rbok",
                        "commands": ["scan", "feed"],
                    },
                    "publish": {
                        "mode": "pull_request",
                        "target_repo": "output",
                        "target_branch": "main",
                        "target_path": "rbok-lawbook/",
                        "branch_strategy": "fixed",
                        "risk_class": "low",
                    },
                    "notify": notify_block,
                }
            ],
        }
        cfg = tmp / "corpus-workflows.yaml"
        import yaml as _yaml
        cfg.write_text(_yaml.safe_dump(config), encoding="utf-8")
        return cfg

    def test_cli_exits_zero_when_disabled(self):
        with tempfile.TemporaryDirectory() as raw:
            tmp = Path(raw)
            cfg = self._write_config(tmp, enabled=False)
            result = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT_PATH),
                    "--config",
                    str(cfg),
                    "--workflow-id",
                    "rbok-lawbook",
                    "--gate-status",
                    "pass",
                    "--repo",
                    "o/r",
                    "--pr-number",
                    "1",
                ],
                capture_output=True,
                text=True,
                cwd=str(REPO_ROOT),
            )
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn("comment disabled, no action", result.stdout)

    def test_cli_exits_zero_when_block_missing(self):
        with tempfile.TemporaryDirectory() as raw:
            tmp = Path(raw)
            cfg = self._write_config(tmp, enabled=True, has_block=False)
            result = subprocess.run(
                [
                    sys.executable,
                    str(SCRIPT_PATH),
                    "--config",
                    str(cfg),
                    "--workflow-id",
                    "rbok-lawbook",
                    "--gate-status",
                    "pass",
                    "--repo",
                    "o/r",
                    "--pr-number",
                    "1",
                ],
                capture_output=True,
                text=True,
                cwd=str(REPO_ROOT),
            )
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn("comment disabled, no action", result.stdout)

    def test_cli_dry_run_prints_body(self):
        with tempfile.TemporaryDirectory() as raw:
            tmp = Path(raw)
            cfg = self._write_config(tmp, enabled=True)
            diff = tmp / "diff.json"
            diff.write_text(
                json.dumps(
                    {"impacted": [{"id": "rbok-lawbook", "changed_paths": ["a.md"]}]}
                ),
                encoding="utf-8",
            )
            with mock.patch.dict(os.environ, {}, clear=False):
                result = subprocess.run(
                    [
                        sys.executable,
                        str(SCRIPT_PATH),
                        "--config",
                        str(cfg),
                        "--workflow-id",
                        "rbok-lawbook",
                        "--diff-plan",
                        str(diff),
                        "--gate-status",
                        "pass",
                        "--repo",
                        "o/r",
                        "--pr-number",
                        "1",
                        "--output-location",
                        "https://example/out",
                        "--dry-run",
                    ],
                    capture_output=True,
                    text=True,
                    cwd=str(REPO_ROOT),
                )
        self.assertEqual(result.returncode, 0, msg=result.stderr)
        self.assertIn("dry-run", result.stdout)
        self.assertIn("nomos-source-pr-comment:rbok-lawbook", result.stdout)
        # Summary mode reports the count, not the path list (the latter is
        # exclusive to detailed mode).
        self.assertIn("changed paths: 1", result.stdout)


if __name__ == "__main__":
    unittest.main()

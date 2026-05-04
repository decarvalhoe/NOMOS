"""Tests for scripts/nomos_github_publish.py (NGW-005, #390).

The publisher is exercised entirely in dry-run mode; tests must never make
a real ``git push`` or ``gh`` API call. Subprocess shell-outs are intercepted
either through the ``run_command`` parameter on the publisher entry points
or via ``unittest.mock.patch`` over ``nomos_github_publish._run_subprocess``.
"""

from __future__ import annotations

import io
import json
import os
import subprocess
import sys
import tempfile
import unittest
from contextlib import contextmanager
from unittest import mock

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), os.pardir))
SCRIPTS_DIR = os.path.join(REPO_ROOT, "scripts")
SCRIPT_PATH = os.path.join(SCRIPTS_DIR, "nomos_github_publish.py")

if SCRIPTS_DIR not in sys.path:
    sys.path.insert(0, SCRIPTS_DIR)

import nomos_github_publish as publisher  # noqa: E402


@contextmanager
def _chdir(path: str):
    old = os.getcwd()
    os.chdir(path)
    try:
        yield
    finally:
        os.chdir(old)


class PathGuardTests(unittest.TestCase):
    """Five mandated path-guard cases plus a couple of close cousins."""

    def test_path_guard_under_target_accepted(self) -> None:
        candidates = [
            "rbok-lawbook/feed.json",
            "rbok-lawbook/sub/dir/file.md",
            "rbok-lawbook",  # equal to target_path is also accepted
        ]
        self.assertEqual(publisher.validate_path_guard("rbok-lawbook", candidates), [])

    def test_path_guard_outside_target_rejected(self) -> None:
        violations = publisher.validate_path_guard(
            "rbok-lawbook", ["../escape.txt"]
        )
        self.assertEqual(violations, ["../escape.txt"])

    def test_path_guard_traversal_rejected(self) -> None:
        candidates = [
            "rbok-lawbook/../etc/passwd",
            "rbok-lawbook/sub/../../escape.md",
        ]
        violations = publisher.validate_path_guard("rbok-lawbook", candidates)
        self.assertCountEqual(violations, candidates)

    def test_path_guard_absolute_rejected(self) -> None:
        violations = publisher.validate_path_guard(
            "rbok-lawbook", ["/etc/hosts", "/var/tmp/x"]
        )
        self.assertCountEqual(violations, ["/etc/hosts", "/var/tmp/x"])

    def test_path_guard_drive_letter_rejected(self) -> None:
        # Defence in depth: a Windows-style drive letter must be rejected
        # even when the runner is POSIX.
        self.assertEqual(
            publisher.validate_path_guard("rbok-lawbook", [r"C:\windows\evil.md"]),
            [r"C:\windows\evil.md"],
        )

    def test_path_guard_symlink_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp, _chdir(tmp):
            os.makedirs("rbok-lawbook")
            with open("outside.txt", "w", encoding="utf-8") as fh:
                fh.write("outside")
            os.symlink(os.path.join(tmp, "outside.txt"), "rbok-lawbook/evil.md")
            violations = publisher.validate_path_guard(
                "rbok-lawbook",
                ["rbok-lawbook/evil.md", "rbok-lawbook/safe.md"],
            )
            self.assertEqual(violations, ["rbok-lawbook/evil.md"])


class TargetPathConfigCheckTests(unittest.TestCase):
    """Defence-in-depth check on ``target_path`` itself."""

    def test_target_path_must_be_relative(self) -> None:
        with self.assertRaises(ValueError):
            publisher._validate_target_path("/abs/path")

    def test_target_path_rejects_dotdot(self) -> None:
        with self.assertRaises(ValueError):
            publisher._validate_target_path("foo/../bar")

    def test_target_path_rejects_empty(self) -> None:
        with self.assertRaises(ValueError):
            publisher._validate_target_path("")


class AntiLoopMarkerTests(unittest.TestCase):
    def test_anti_loop_marker_format(self) -> None:
        marker = publisher.build_anti_loop_marker(
            "deadbeef0000000000000000000000000000beef",
            "rbok-lawbook",
        )
        expected = (
            "[nomos-generated]\n"
            "source-sha: deadbeef0000000000000000000000000000beef\n"
            "scope: rbok-lawbook"
        )
        self.assertEqual(marker, expected)

    def test_anti_loop_marker_appended_to_commit_body(self) -> None:
        msg = publisher._compose_commit_message(
            "Refresh rbok-lawbook outputs",
            "deadbeef0000000000000000000000000000beef",
            "rbok-lawbook",
        )
        # Subject is NOT the marker — marker lives in the body.
        self.assertTrue(msg.startswith("Refresh rbok-lawbook outputs\n\n[nomos-generated]"))
        self.assertIn("source-sha: deadbeef0000000000000000000000000000beef", msg)
        self.assertIn("scope: rbok-lawbook", msg)

    def test_anti_loop_marker_requires_inputs(self) -> None:
        with self.assertRaises(ValueError):
            publisher.build_anti_loop_marker("", "scope")
        with self.assertRaises(ValueError):
            publisher.build_anti_loop_marker("sha", "")


class BranchStrategyTests(unittest.TestCase):
    def test_publish_pull_request_branch_strategies(self) -> None:
        cases = [
            (
                "fixed",
                {},
                "nomos/rbok-lawbook",
            ),
            (
                "per_pr",
                {"source_pr_number": 42},
                "nomos/rbok-lawbook/pr-42",
            ),
            (
                "per_source_ref",
                {"source_ref": "feature/update guide"},
                "nomos/rbok-lawbook/feature-update-guide",
            ),
            (
                "dated",
                {"utc_date": "20260504"},
                "nomos/rbok-lawbook/20260504",
            ),
        ]
        for strategy, kwargs, expected in cases:
            with self.subTest(strategy=strategy):
                self.assertEqual(
                    publisher.compute_branch_name(strategy, "rbok-lawbook", **kwargs),
                    expected,
                )

    def test_per_pr_requires_pr_number(self) -> None:
        with self.assertRaises(ValueError):
            publisher.compute_branch_name("per_pr", "rbok-lawbook")

    def test_per_source_ref_requires_ref(self) -> None:
        with self.assertRaises(ValueError):
            publisher.compute_branch_name("per_source_ref", "rbok-lawbook")

    def test_unknown_strategy_raises(self) -> None:
        with self.assertRaises(ValueError):
            publisher.compute_branch_name("rolling", "rbok-lawbook")


class ArtifactOnlyTests(unittest.TestCase):
    def test_publish_artifact_only_no_push(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            outputs = os.path.join(tmp, "outputs")
            os.makedirs(outputs)
            with open(os.path.join(outputs, "feed.json"), "w", encoding="utf-8") as fh:
                fh.write('{"format":"nomos.corpus-feed.v1"}')
            trace = os.path.join(tmp, "nomos-trace.yaml")
            with open(trace, "w", encoding="utf-8") as fh:
                fh.write("schema_version: '0.1.0'\n")
            diff = os.path.join(tmp, "nomos-diff.json")
            with open(diff, "w", encoding="utf-8") as fh:
                fh.write('{"impacted":[]}')

            dest = os.path.join(tmp, "publish-stage")
            with mock.patch.object(publisher, "_run_subprocess") as run_mock:
                result = publisher.publish_artifact_only(
                    outputs_dir=outputs,
                    trace_manifest=trace,
                    diff_plan=diff,
                    dest_dir=dest,
                    dry_run=False,
                )
            run_mock.assert_not_called()

            self.assertEqual(result["mode"], "artifact_only")
            self.assertIn("feed.json", result["prepared_files"])
            self.assertTrue(os.path.isfile(os.path.join(dest, "feed.json")))
            self.assertTrue(os.path.isfile(result["trace_manifest_path"]))
            self.assertTrue(os.path.isfile(result["diff_plan_path"]))

    def test_publish_artifact_only_dry_run_writes_nothing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            outputs = os.path.join(tmp, "outputs")
            os.makedirs(outputs)
            with open(os.path.join(outputs, "feed.json"), "w", encoding="utf-8") as fh:
                fh.write("{}")
            dest = os.path.join(tmp, "stage")
            result = publisher.publish_artifact_only(
                outputs_dir=outputs,
                trace_manifest="",
                diff_plan="",
                dest_dir=dest,
                dry_run=True,
            )
            self.assertTrue(result["dry_run"])
            self.assertFalse(os.path.exists(dest))
            self.assertEqual(result["prepared_files"], ["feed.json"])


class DirectPushGuardTests(unittest.TestCase):
    def test_publish_direct_push_blocked_without_explicit_mode(self) -> None:
        with self.assertRaises(ValueError) as ctx:
            publisher.publish_direct_push(
                publish_mode="pull_request",
                workflow_id="rbok-lawbook",
                source_sha="deadbeef" * 5,
                target_repo="owner/output",
                target_branch="main",
                target_path="rbok-lawbook",
                outputs_dir="",
                trace_manifest="",
            )
        self.assertIn("direct_push", str(ctx.exception))

    def test_publish_direct_push_dry_run_returns_plan(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            outputs = os.path.join(tmp, "outputs")
            os.makedirs(outputs)
            with open(os.path.join(outputs, "feed.json"), "w", encoding="utf-8") as fh:
                fh.write("{}")
            with mock.patch.object(publisher, "_run_subprocess") as run_mock:
                plan = publisher.publish_direct_push(
                    publish_mode="direct_push",
                    workflow_id="rbok-lawbook",
                    source_sha="deadbeef" * 5,
                    target_repo="owner/output",
                    target_branch="main",
                    target_path="rbok-lawbook",
                    outputs_dir=outputs,
                    trace_manifest="",
                    commit_subject="Refresh rbok-lawbook",
                    dry_run=True,
                )
            run_mock.assert_not_called()
            self.assertEqual(plan["mode"], "direct_push")
            self.assertEqual(plan["target_branch"], "main")
            self.assertEqual(plan["tree_files"], ["rbok-lawbook/feed.json"])
            self.assertIn("[nomos-generated]", plan["commit_message"])

    def test_publish_direct_push_path_guard_blocks_symlinks(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            outputs = os.path.join(tmp, "outputs")
            os.makedirs(outputs)
            with open(os.path.join(tmp, "outside.txt"), "w", encoding="utf-8") as fh:
                fh.write("x")
            os.symlink(os.path.join(tmp, "outside.txt"), os.path.join(outputs, "evil.md"))
            with self.assertRaises(ValueError) as ctx:
                publisher.publish_direct_push(
                    publish_mode="direct_push",
                    workflow_id="rbok-lawbook",
                    source_sha="deadbeef" * 5,
                    target_repo="owner/output",
                    target_branch="main",
                    target_path="rbok-lawbook",
                    outputs_dir=outputs,
                    trace_manifest="",
                    dry_run=True,
                )
            self.assertIn("symlink", str(ctx.exception))


class PullRequestPlanTests(unittest.TestCase):
    def test_dry_run_returns_plan_without_subprocess(self) -> None:
        with mock.patch.object(publisher, "_run_subprocess") as run_mock:
            plan = publisher.publish_pull_request(
                workflow_id="rbok-lawbook",
                branch_strategy="per_pr",
                source_sha="deadbeef" * 5,
                source_pr_number=99,
                source_ref="feat/x",
                target_repo="owner/output",
                target_branch="main",
                target_path="rbok-lawbook",
                outputs_dir="",
                trace_manifest="",
                commit_subject="Refresh outputs",
                dry_run=True,
            )
        run_mock.assert_not_called()
        self.assertEqual(plan["mode"], "pull_request")
        self.assertEqual(plan["branch"], "nomos/rbok-lawbook/pr-99")
        self.assertIn("[nomos-generated]", plan["commit_message"])
        self.assertNotIn("gh_pr_create", plan)


class CLITests(unittest.TestCase):
    def _make_minimal_inputs(self, tmp: str) -> dict[str, str]:
        config = os.path.join(tmp, "corpus-workflows.yaml")
        with open(config, "w", encoding="utf-8") as fh:
            fh.write("schema_version: '0.1.0'\n")
        diff = os.path.join(tmp, "nomos-diff.json")
        with open(diff, "w", encoding="utf-8") as fh:
            fh.write('{"impacted":[]}')
        trace = os.path.join(tmp, "nomos-trace.yaml")
        with open(trace, "w", encoding="utf-8") as fh:
            fh.write("schema_version: '0.1.0'\n")
        outputs = os.path.join(tmp, "outputs")
        os.makedirs(outputs)
        return {"config": config, "diff": diff, "trace": trace, "outputs": outputs}

    def test_cli_artifact_only_dry_run_succeeds(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            paths = self._make_minimal_inputs(tmp)
            with open(os.path.join(paths["outputs"], "feed.json"), "w", encoding="utf-8") as fh:
                fh.write("{}")
            result = subprocess.run(
                [
                    sys.executable,
                    SCRIPT_PATH,
                    "--config", paths["config"],
                    "--workflow-id", "rbok-lawbook",
                    "--diff-plan", paths["diff"],
                    "--outputs-dir", paths["outputs"],
                    "--trace-manifest", paths["trace"],
                    "--mode", "artifact_only",
                    "--target-repo", "owner/output",
                    "--target-branch", "main",
                    "--target-path", "rbok-lawbook",
                    "--source-sha", "deadbeef" * 5,
                    "--dry-run",
                ],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, msg=result.stderr)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["mode"], "artifact_only")
            self.assertTrue(payload["dry_run"])
            self.assertIn("feed.json", payload["prepared_files"])

    def test_cli_exits_nonzero_on_path_violation(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            paths = self._make_minimal_inputs(tmp)
            # Plant a symlink inside outputs that the CLI must reject before
            # any publishing is attempted.
            with open(os.path.join(tmp, "outside.txt"), "w", encoding="utf-8") as fh:
                fh.write("x")
            os.symlink(
                os.path.join(tmp, "outside.txt"),
                os.path.join(paths["outputs"], "evil.md"),
            )
            result = subprocess.run(
                [
                    sys.executable,
                    SCRIPT_PATH,
                    "--config", paths["config"],
                    "--workflow-id", "rbok-lawbook",
                    "--diff-plan", paths["diff"],
                    "--outputs-dir", paths["outputs"],
                    "--trace-manifest", paths["trace"],
                    "--mode", "artifact_only",
                    "--target-repo", "owner/output",
                    "--target-branch", "main",
                    "--target-path", "rbok-lawbook",
                    "--source-sha", "deadbeef" * 5,
                    "--dry-run",
                ],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("path guard violations", result.stderr)
            self.assertIn("symlink", result.stderr)

    def test_cli_exits_nonzero_on_bad_target_path(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            paths = self._make_minimal_inputs(tmp)
            result = subprocess.run(
                [
                    sys.executable,
                    SCRIPT_PATH,
                    "--config", paths["config"],
                    "--workflow-id", "rbok-lawbook",
                    "--diff-plan", paths["diff"],
                    "--outputs-dir", paths["outputs"],
                    "--trace-manifest", paths["trace"],
                    "--mode", "artifact_only",
                    "--target-repo", "owner/output",
                    "--target-branch", "main",
                    "--target-path", "rbok-lawbook/../escape",
                    "--source-sha", "deadbeef" * 5,
                    "--dry-run",
                ],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("target_path", result.stderr)


if __name__ == "__main__":
    unittest.main()

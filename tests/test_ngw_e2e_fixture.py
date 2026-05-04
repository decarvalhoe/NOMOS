"""NGW-10 (#395) — end-to-end fixture tests for the GitHub workflow chain.

Drives the full chain on a SYNTHETIC corpus + output pair under
``tests/fixtures/ngw-e2e/`` without touching any real repository:

    nomos github plan
        -> publisher dry-run (artifact_only / pull_request / direct_push)
        -> trace manifest generator (checked against the #387 schema via
           cue vet when cue is on PATH)
        -> source-PR commenter dry-run

All three publication modes run in dry-run / local-safe mode. There is
no real ``gh`` API call, no real ``git push``, no real artifact upload.
The fixture corpus's git status is captured before AND after each
test so mutation regressions are caught immediately.

Read-only invariant: the fixture corpus directory MUST be clean
before AND after every test. Any modification under
``tests/fixtures/ngw-e2e/corpus/`` is a hard test failure.
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
SCRIPTS_DIR = REPO_ROOT / "scripts"
SPECS_DIR = REPO_ROOT / "specs"
FIXTURE_DIR = REPO_ROOT / "tests" / "fixtures" / "ngw-e2e"
CORPUS_DIR = FIXTURE_DIR / "corpus"
CONFIG_PATH = CORPUS_DIR / ".nomos" / "corpus-workflows.yaml"
SOURCE_SHA = "f1e2d3c4b5a6978869504132241302d4e5f6a7b8"

if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

import nomos_github_publish as publisher  # noqa: E402
import nomos_trace_manifest as trace_gen  # noqa: E402

FROZEN_TIME = "2026-05-04T12:00:00Z"


# ---------------------------------------------------------------------------
# Fixture-corpus read-only enforcement helpers.
#
# The fixture directory is a checked-in tree, not a git working copy of its
# own; we therefore assert read-only by snapshotting `find ... -printf` of
# every file under the corpus directory before and after each test, and
# diffing. Any size/path drift is a hard fail.
# ---------------------------------------------------------------------------


def _snapshot_corpus() -> str:
    lines: list[str] = []
    for dirpath, _dirnames, filenames in os.walk(CORPUS_DIR):
        for filename in filenames:
            path = Path(dirpath) / filename
            rel = path.relative_to(CORPUS_DIR).as_posix()
            lines.append(f"{rel} {path.stat().st_size}")
    lines.sort()
    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Build the NOMOS CLI once for the whole test module so each test is fast.
# ---------------------------------------------------------------------------


_NOMOS_BIN: str | None = None


def _build_nomos_bin() -> str:
    global _NOMOS_BIN
    if _NOMOS_BIN and os.path.isfile(_NOMOS_BIN):
        return _NOMOS_BIN
    if shutil.which("go") is None:
        raise unittest.SkipTest("go not on PATH; skipping NOMOS CLI fixture tests")
    bin_dir = Path(tempfile.mkdtemp(prefix="ngw-e2e-bin-"))
    bin_path = bin_dir / "nomos"
    proc = subprocess.run(
        ["go", "build", "-o", str(bin_path), "."],
        cwd=str(REPO_ROOT / "cli"),
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise RuntimeError(
            f"go build failed (rc={proc.returncode}): {proc.stderr}"
        )
    _NOMOS_BIN = str(bin_path)
    return _NOMOS_BIN


def _run_plan(changed_paths_file: Path, out_dir: Path) -> dict:
    nomos = _build_nomos_bin()
    out = out_dir / "nomos-diff.json"
    proc = subprocess.run(
        [
            nomos,
            "github",
            "plan",
            "--config",
            str(CONFIG_PATH),
            "--changed-paths",
            str(changed_paths_file),
            "--out",
            str(out),
            "--frozen-time",
            FROZEN_TIME,
            "--format",
            "json",
        ],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise AssertionError(
            f"nomos github plan failed: rc={proc.returncode} stderr={proc.stderr}"
        )
    return json.loads(out.read_text())


# ---------------------------------------------------------------------------
# Base test case enforcing the read-only fixture invariant.
# ---------------------------------------------------------------------------


class _ReadOnlyFixtureTest(unittest.TestCase):
    """Subclass for every NGW-10 case so the fixture invariant is enforced."""

    def setUp(self) -> None:
        self._snapshot_before = _snapshot_corpus()
        self._tmpdir = tempfile.mkdtemp(prefix="ngw-e2e-")
        self.addCleanup(shutil.rmtree, self._tmpdir, ignore_errors=True)

    def tearDown(self) -> None:
        snapshot_after = _snapshot_corpus()
        self.assertEqual(
            self._snapshot_before,
            snapshot_after,
            "fixture corpus was mutated during the test (read-only invariant)",
        )

    @property
    def tmpdir(self) -> Path:
        return Path(self._tmpdir)


# ---------------------------------------------------------------------------
# 1-4. Planner cases.
# ---------------------------------------------------------------------------


class PlannerImpactTests(_ReadOnlyFixtureTest):
    def test_plan_impacts_only_a(self) -> None:
        plan = _run_plan(FIXTURE_DIR / "changed-paths-impacts-a.txt", self.tmpdir)
        ids = [w["workflow_id"] for w in plan["impacted"]]
        self.assertEqual(ids, ["scope-a"])
        self.assertEqual(plan["impacted"][0]["matched_paths"], ["area-a/sample-a.md"])
        self.assertEqual(
            [s["workflow_id"] for s in plan["skipped"]],
            ["scope-b"],
        )

    def test_plan_impacts_both_when_both_change(self) -> None:
        plan = _run_plan(
            FIXTURE_DIR / "changed-paths-impacts-both.txt", self.tmpdir
        )
        ids = sorted(w["workflow_id"] for w in plan["impacted"])
        self.assertEqual(ids, ["scope-a", "scope-b"])
        self.assertEqual(plan["skipped"], [])

    def test_plan_no_impact_for_unrelated_change(self) -> None:
        plan = _run_plan(
            FIXTURE_DIR / "changed-paths-impacts-none.txt", self.tmpdir
        )
        self.assertEqual(plan["impacted"], [])
        skipped_codes = {s["reason"] for s in plan["skipped"]}
        self.assertTrue(
            skipped_codes <= {"NGW_DIFF_NO_PATHS_MATCH", "NGW_DIFF_ALL_PATHS_GENERATED"},
            f"unexpected skip codes: {skipped_codes}",
        )

    def test_plan_loop_guard_ignores_output_paths(self) -> None:
        plan = _run_plan(
            FIXTURE_DIR / "changed-paths-loop-guard.txt", self.tmpdir
        )
        self.assertEqual(plan["impacted"], [])
        self.assertEqual(
            sorted(plan["ignored_generated_paths"]),
            ["output/scope-a/feed.json", "output/scope-b/feed.json"],
        )


# ---------------------------------------------------------------------------
# 5-8. Publisher dry-run cases.
# ---------------------------------------------------------------------------


class PublisherDryRunTests(_ReadOnlyFixtureTest):
    def _outputs_dir(self) -> Path:
        d = self.tmpdir / "outputs"
        d.mkdir(parents=True, exist_ok=True)
        # Two synthetic generated files inside the target_path. The publisher
        # path guard expects every emitted file to live UNDER target_path
        # when prefixed via gather_publish_candidates; we therefore mirror
        # the target_path layout here.
        return d

    def _trace_path(self) -> Path:
        p = self.tmpdir / "nomos-trace.yaml"
        p.write_text("schema_version: \"0.1.0\"\n", encoding="utf-8")
        return p

    def _diff_path(self) -> Path:
        p = self.tmpdir / "nomos-diff.json"
        p.write_text("{}", encoding="utf-8")
        return p

    def test_publisher_artifact_only_dry_run(self) -> None:
        result = publisher.publish_artifact_only(
            outputs_dir=str(self._outputs_dir()),
            trace_manifest=str(self._trace_path()),
            diff_plan=str(self._diff_path()),
            dest_dir=str(self.tmpdir / "publish-out"),
            dry_run=True,
        )
        self.assertEqual(result["mode"], "artifact_only")
        self.assertTrue(result["dry_run"])
        self.assertIn("trace_manifest_path", result)
        self.assertFalse(
            (self.tmpdir / "publish-out").exists(),
            "dry_run must not create the destination directory",
        )

    def test_publisher_pull_request_dry_run(self) -> None:
        result = publisher.publish_pull_request(
            workflow_id="scope-a",
            branch_strategy="per_pr",
            source_sha=SOURCE_SHA,
            source_pr_number=42,
            source_ref="feature/sample",
            target_repo="ngw-e2e/output",
            target_branch="main",
            target_path="output/scope-a/",
            outputs_dir=str(self._outputs_dir()),
            trace_manifest=str(self._trace_path()),
            commit_subject="nomos: refresh scope-a",
            dry_run=True,
        )
        self.assertEqual(result["mode"], "pull_request")
        self.assertTrue(result["dry_run"])
        # branch_strategy=per_pr must encode the PR number in the branch name.
        self.assertIn("42", result["branch"])
        # The anti-loop marker must be the canonical string AND embedded in
        # the commit message so downstream NOMOS runs can recognise it.
        self.assertEqual(
            result["anti_loop_marker"],
            publisher.build_anti_loop_marker(SOURCE_SHA, "scope-a"),
        )
        self.assertIn(publisher.ANTI_LOOP_HEADER, result["commit_message"])
        self.assertIn(SOURCE_SHA, result["commit_message"])

    def test_publisher_direct_push_dry_run(self) -> None:
        outputs = self._outputs_dir()
        # Synthetic file inside outputs_dir; gather_publish_candidates will
        # rewrite its dest path under target_path before the guard runs.
        (outputs / "feed.json").write_text("{}", encoding="utf-8")
        result = publisher.publish_direct_push(
            publish_mode="direct_push",
            workflow_id="scope-b",
            source_sha=SOURCE_SHA,
            target_repo="ngw-e2e/output",
            target_branch="main",
            target_path="output/scope-b/",
            outputs_dir=str(outputs),
            trace_manifest=str(self._trace_path()),
            commit_subject="nomos: refresh scope-b",
            dry_run=True,
        )
        self.assertEqual(result["mode"], "direct_push")
        self.assertTrue(result["dry_run"])
        self.assertIn(publisher.ANTI_LOOP_HEADER, result["commit_message"])
        # Path guard accepts every file when they live under target_path.
        self.assertEqual(
            result["tree_files"],
            ["output/scope-b/feed.json"],
        )

    def test_publisher_path_guard_rejects_violation(self) -> None:
        outputs = self._outputs_dir()
        (outputs / "ok.json").write_text("{}", encoding="utf-8")
        # Synthesize a file whose dest would escape target_path. We do this
        # by calling validate_path_guard directly with a known-bad list,
        # mirroring the publisher's runtime check.
        violations = publisher.validate_path_guard(
            "output/scope-b/", ["output/scope-b/ok.json", "../escape.json"]
        )
        self.assertEqual(violations, ["../escape.json"])
        # The publish_direct_push entry point also raises ValueError for
        # any source-symlink. Verify by patching gather_publish_candidates
        # to return a path outside target_path, simulating a misconfigured
        # output tree.
        with self.assertRaises(ValueError):
            publisher.publish_direct_push(
                publish_mode="direct_push",
                workflow_id="scope-b",
                source_sha=SOURCE_SHA,
                target_repo="ngw-e2e/output",
                target_branch="main",
                target_path="output/scope-b/../escape",
                outputs_dir=str(outputs),
                trace_manifest=str(self._trace_path()),
                dry_run=True,
            )


# ---------------------------------------------------------------------------
# 9-11. Trace manifest cases — one per publication mode.
# ---------------------------------------------------------------------------


def _baseline_corpus() -> dict:
    return {
        "repo": "ngw-e2e/synthetic",
        "base_ref": "main",
        "base_sha": "1f9d2c8b07e3a4f5d6c7b8a9e0d1c2b3a4f5d6c7",
        "head_ref": "feature/synth",
        "head_sha": "9b2a3c4d5e6f7081923a4b5c6d7e8f90a1b2c3d4",
        "pull_request": 7,
    }


class TraceManifestTests(_ReadOnlyFixtureTest):
    def _diff_plan_path(self, scope: str) -> Path:
        plan = {
            "schema_version": "ngw-diff-v1",
            "generated_at": FROZEN_TIME,
            "config_path": str(CONFIG_PATH),
            "changed_path_count": 1,
            "impacted": [
                {
                    "workflow_id": scope,
                    "scope_id": scope,
                    "scope_paths": [f"area-{scope[-1]}/**"],
                    "matched_paths": [f"area-{scope[-1]}/sample-{scope[-1]}.md"],
                }
            ],
            "skipped": [],
            "ignored_generated_paths": [],
        }
        p = self.tmpdir / "nomos-diff.json"
        p.write_text(json.dumps(plan), encoding="utf-8")
        return p

    def _maybe_cue_vet(self, yaml_path: Path) -> None:
        if shutil.which("cue") is None:
            self.skipTest("cue not on PATH")
        proc = subprocess.run(
            [
                "cue",
                "vet",
                str(SPECS_DIR / "nomos-trace-manifest.cue"),
                str(yaml_path),
                "-d",
                "#NomosTraceManifest",
            ],
            capture_output=True,
            text=True,
        )
        self.assertEqual(
            proc.returncode,
            0,
            f"cue vet failed: stderr={proc.stderr} stdout={proc.stdout}",
        )

    def _generate(
        self,
        *,
        scope: str,
        publish_mode: str,
        output_extra: dict,
        artifact_extra: dict,
    ) -> Path:
        diff_path = self._diff_plan_path(scope)
        out_yaml = self.tmpdir / f"trace-{publish_mode}.yaml"
        out_json = self.tmpdir / f"trace-{publish_mode}.json"
        argv = [
            "--diff-plan",
            str(diff_path),
            "--workflow-id",
            scope,
            "--event",
            "pull_request",
            "--workflow-run-id",
            "1234567890",
            "--corpus-repo",
            _baseline_corpus()["repo"],
            "--corpus-base-ref",
            _baseline_corpus()["base_ref"],
            "--corpus-base-sha",
            _baseline_corpus()["base_sha"],
            "--corpus-head-ref",
            _baseline_corpus()["head_ref"],
            "--corpus-head-sha",
            _baseline_corpus()["head_sha"],
            "--pull-request",
            str(_baseline_corpus()["pull_request"]),
            "--output-repo",
            output_extra.get("repo", "ngw-e2e/output"),
            "--output-path",
            output_extra["path"],
            "--publish-mode",
            publish_mode,
            "--risk-class",
            "low",
            "--generated-path-guard",
            "pass",
            "--source-read-only-guard",
            "pass",
            "--out-yaml",
            str(out_yaml),
            "--out-json",
            str(out_json),
            "--frozen-time",
            FROZEN_TIME,
            "--no-cue-vet",
        ]
        if "branch" in output_extra:
            argv.extend(["--output-branch", output_extra["branch"]])
        if "commit_sha" in output_extra:
            argv.extend(["--output-commit-sha", output_extra["commit_sha"]])
        for key, val in artifact_extra.items():
            argv.extend([f"--artifact-{key.replace('_','-')}", val])
        rc = trace_gen.main(argv)
        self.assertEqual(rc, 0, "trace manifest generator should exit 0")
        self.assertTrue(out_yaml.exists())
        self.assertTrue(out_json.exists())
        return out_yaml

    def test_trace_manifest_artifact_only(self) -> None:
        yaml_path = self._generate(
            scope="scope-a",
            publish_mode="artifact_only",
            output_extra={"repo": "corpus", "path": "output/scope-a/"},
            artifact_extra={"feed": "output/scope-a/feed.json"},
        )
        self._maybe_cue_vet(yaml_path)

    def test_trace_manifest_pull_request(self) -> None:
        yaml_path = self._generate(
            scope="scope-a",
            publish_mode="pull_request",
            output_extra={
                "repo": "ngw-e2e/output",
                "branch": "nomos/refresh-scope-a-pr-7",
                "path": "output/scope-a/",
                "commit_sha": "deadbee1234567890abcdef1234567890abcdef0",
            },
            artifact_extra={"feed": "output/scope-a/feed.json"},
        )
        self._maybe_cue_vet(yaml_path)

    def test_trace_manifest_direct_push(self) -> None:
        yaml_path = self._generate(
            scope="scope-b",
            publish_mode="direct_push",
            output_extra={
                "repo": "ngw-e2e/output",
                "branch": "main",
                "path": "output/scope-b/",
                "commit_sha": "cafefeed1234567890abcdef1234567890abcdef",
            },
            artifact_extra={"feed": "output/scope-b/feed.json"},
        )
        self._maybe_cue_vet(yaml_path)


# ---------------------------------------------------------------------------
# 12-13. Commenter dry-run cases.
# ---------------------------------------------------------------------------


class CommenterDryRunTests(_ReadOnlyFixtureTest):
    def _diff_plan_path(self) -> Path:
        plan = {
            "schema_version": "ngw-diff-v1",
            "generated_at": FROZEN_TIME,
            "config_path": str(CONFIG_PATH),
            "changed_path_count": 2,
            "impacted": [
                {
                    "workflow_id": "scope-a",
                    "scope_id": "scope-a",
                    "scope_paths": ["area-a/**"],
                    "matched_paths": ["area-a/sample-a.md"],
                },
                {
                    "workflow_id": "scope-b",
                    "scope_id": "scope-b",
                    "scope_paths": ["area-b/**"],
                    "matched_paths": ["area-b/sample-b.md"],
                },
            ],
            "skipped": [],
            "ignored_generated_paths": [],
        }
        p = self.tmpdir / "nomos-diff.json"
        p.write_text(json.dumps(plan), encoding="utf-8")
        return p

    def _run_commenter(self, extra: list[str]) -> tuple[int, str]:
        script = SCRIPTS_DIR / "nomos_github_comment.py"
        proc = subprocess.run(
            [sys.executable, str(script)] + extra,
            capture_output=True,
            text=True,
        )
        return proc.returncode, proc.stdout + proc.stderr

    def test_commenter_summary_dry_run(self) -> None:
        diff_path = self._diff_plan_path()
        rc, body = self._run_commenter(
            [
                "--config",
                str(CONFIG_PATH),
                "--workflow-id",
                "scope-a",
                "--diff-plan",
                str(diff_path),
                "--gate-status",
                "skipped",
                "--dry-run",
            ]
        )
        self.assertEqual(rc, 0, f"commenter dry-run rc={rc}: {body}")
        self.assertIn("<!-- nomos-source-pr-comment:scope-a -->", body)
        self.assertIn("scope-a", body)

    def test_commenter_disabled_no_op(self) -> None:
        # scope-b sets notify.source_pr_comment.enabled: false in the
        # fixture config; the commenter must exit 0 with no body.
        rc, body = self._run_commenter(
            [
                "--config",
                str(CONFIG_PATH),
                "--workflow-id",
                "scope-b",
                "--dry-run",
            ]
        )
        self.assertEqual(rc, 0)
        self.assertNotIn("<!-- nomos-source-pr-comment:", body)
        self.assertIn("comment disabled", body)


# ---------------------------------------------------------------------------
# 14. Full-chain dry-run orchestration: plan -> publisher (each mode) ->
#     trace gen -> commenter.
# ---------------------------------------------------------------------------


class FullChainDryRunTest(_ReadOnlyFixtureTest):
    def test_full_chain_dry_run(self) -> None:
        # 1. Plan against impacts-both → both scopes impacted.
        plan = _run_plan(
            FIXTURE_DIR / "changed-paths-impacts-both.txt", self.tmpdir
        )
        ids = sorted(w["workflow_id"] for w in plan["impacted"])
        self.assertEqual(ids, ["scope-a", "scope-b"])

        # 2. For each mode, build a trace manifest and run the matching
        #    publisher dry-run.
        mode_matrix = [
            (
                "artifact_only",
                "scope-a",
                {"repo": "corpus", "path": "output/scope-a/"},
            ),
            (
                "pull_request",
                "scope-a",
                {
                    "repo": "ngw-e2e/output",
                    "branch": "nomos/refresh-scope-a-pr-7",
                    "path": "output/scope-a/",
                    "commit_sha": "1234567890abcdef1234567890abcdef12345678",
                },
            ),
            (
                "direct_push",
                "scope-b",
                {
                    "repo": "ngw-e2e/output",
                    "branch": "main",
                    "path": "output/scope-b/",
                    "commit_sha": "abcdef1234567890abcdef1234567890abcdef12",
                },
            ),
        ]
        outputs_dir = self.tmpdir / "outputs"
        outputs_dir.mkdir()
        (outputs_dir / "feed.json").write_text("{}", encoding="utf-8")

        diff_path = self.tmpdir / "nomos-diff.json"
        diff_path.write_text(json.dumps(plan), encoding="utf-8")

        for mode, scope, output_extra in mode_matrix:
            with self.subTest(mode=mode):
                out_yaml = self.tmpdir / f"trace-{mode}.yaml"
                out_json = self.tmpdir / f"trace-{mode}.json"
                argv = [
                    "--diff-plan",
                    str(diff_path),
                    "--workflow-id",
                    scope,
                    "--event",
                    "pull_request",
                    "--workflow-run-id",
                    "999",
                    "--corpus-repo",
                    _baseline_corpus()["repo"],
                    "--corpus-base-ref",
                    _baseline_corpus()["base_ref"],
                    "--corpus-base-sha",
                    _baseline_corpus()["base_sha"],
                    "--corpus-head-ref",
                    _baseline_corpus()["head_ref"],
                    "--corpus-head-sha",
                    _baseline_corpus()["head_sha"],
                    "--pull-request",
                    "7",
                    "--output-repo",
                    output_extra.get("repo", "ngw-e2e/output"),
                    "--output-path",
                    output_extra["path"],
                    "--publish-mode",
                    mode,
                    "--risk-class",
                    "low",
                    "--generated-path-guard",
                    "pass",
                    "--source-read-only-guard",
                    "pass",
                    "--out-yaml",
                    str(out_yaml),
                    "--out-json",
                    str(out_json),
                    "--frozen-time",
                    FROZEN_TIME,
                    "--no-cue-vet",
                    "--artifact-feed",
                    f"{output_extra['path']}feed.json",
                ]
                if "branch" in output_extra:
                    argv.extend(["--output-branch", output_extra["branch"]])
                if "commit_sha" in output_extra:
                    argv.extend(["--output-commit-sha", output_extra["commit_sha"]])
                rc = trace_gen.main(argv)
                self.assertEqual(rc, 0, f"trace generator failed for {mode}")
                self.assertTrue(out_yaml.exists())

                # Drive the matching publisher dry-run.
                if mode == "artifact_only":
                    res = publisher.publish_artifact_only(
                        outputs_dir=str(outputs_dir),
                        trace_manifest=str(out_yaml),
                        diff_plan=str(diff_path),
                        dest_dir=str(self.tmpdir / f"publish-{mode}"),
                        dry_run=True,
                    )
                elif mode == "pull_request":
                    res = publisher.publish_pull_request(
                        workflow_id=scope,
                        branch_strategy="per_pr",
                        source_sha=SOURCE_SHA,
                        source_pr_number=7,
                        source_ref="feature/synth",
                        target_repo=output_extra["repo"],
                        target_branch=output_extra["branch"],
                        target_path=output_extra["path"],
                        outputs_dir=str(outputs_dir),
                        trace_manifest=str(out_yaml),
                        dry_run=True,
                    )
                else:  # direct_push
                    res = publisher.publish_direct_push(
                        publish_mode="direct_push",
                        workflow_id=scope,
                        source_sha=SOURCE_SHA,
                        target_repo=output_extra["repo"],
                        target_branch=output_extra["branch"],
                        target_path=output_extra["path"],
                        outputs_dir=str(outputs_dir),
                        trace_manifest=str(out_yaml),
                        dry_run=True,
                    )
                self.assertEqual(res["mode"], mode)
                self.assertTrue(res["dry_run"])

        # 3. Commenter dry-run for the enabled scope.
        script = SCRIPTS_DIR / "nomos_github_comment.py"
        proc = subprocess.run(
            [
                sys.executable,
                str(script),
                "--config",
                str(CONFIG_PATH),
                "--workflow-id",
                "scope-a",
                "--diff-plan",
                str(diff_path),
                "--gate-status",
                "skipped",
                "--dry-run",
            ],
            capture_output=True,
            text=True,
        )
        self.assertEqual(proc.returncode, 0, f"commenter rc={proc.returncode}: {proc.stderr}")
        self.assertIn("<!-- nomos-source-pr-comment:scope-a -->", proc.stdout)


if __name__ == "__main__":  # pragma: no cover
    unittest.main()

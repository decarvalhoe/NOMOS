"""NGW-07 (#392) — tests for the trace manifest generator.

Unit tests for the importable API plus a small CLI smoke test that
exercises the end-to-end binary against a temp diff plan. The cue-vet
test is skipped when ``cue`` is not on PATH so the suite remains
runnable on any CI worker.
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
from typing import Any

# Make the script importable even though it lives under scripts/.
SCRIPTS_DIR = Path(__file__).resolve().parents[1] / "scripts"
SPECS_DIR = Path(__file__).resolve().parents[1] / "specs"
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

import nomos_trace_manifest as gen  # noqa: E402


FROZEN_TIME = "2026-05-04T12:00:00Z"


def _baseline_corpus() -> dict:
    return {
        "repo": "RBOKproject/realisons-business",
        "base_ref": "main",
        "base_sha": "1f9d2c8b07e3a4f5d6c7b8a9e0d1c2b3a4f5d6c7",
        "head_ref": "feature/update-guide",
        "head_sha": "9b2a3c4d5e6f7081923a4b5c6d7e8f90a1b2c3d4",
        "pull_request": 123,
    }


def _baseline_scope() -> dict:
    return {"id": "rbok-lawbook", "paths": ["01_rbok/**"]}


def _baseline_diff() -> dict:
    return {
        "changed_paths": ["01_rbok/chapter-1.md", "01_rbok/chapter-2.md"],
        "impacted": True,
    }


def _baseline_output_for_mode(mode: str) -> dict:
    if mode == "artifact_only":
        return {"repo": "corpus", "path": "rbok-lawbook/"}
    return {
        "repo": "RBOKproject/nomos-rbok-artifacts",
        "branch": "main",
        "path": "rbok-lawbook/",
        "commit_sha": "a1b2c3d4e5f6708192a3b4c5d6e7f8091a2b3c4d",
    }


def _baseline_policy(mode: str = "artifact_only", risk: str = "low") -> dict:
    return {
        "publish_mode": mode,
        "risk_class": risk,
        "generated_path_guard": "pass",
        "source_read_only_guard": "pass",
    }


def _baseline_artifacts() -> dict:
    return {
        "feed": "feed.json",
        "body_ledger": "corpus-body-ledger.json",
        "rag_metadata": "rag-metadata.json",
        "attestation": "attestation.json",
        "diff_report": "nomos-diff.json",
    }


def _baseline_diff_plan() -> dict:
    return {
        "schema_version": "ngw-diff-v1",
        "generated_at": FROZEN_TIME,
        "config_path": ".nomos/corpus-workflows.yaml",
        "changed_path_count": 2,
        "impacted": [
            {
                "workflow_id": "rbok-lawbook",
                "scope_id": "rbok-lawbook",
                "scope_paths": ["01_rbok/**"],
                "matched_paths": [
                    "01_rbok/chapter-1.md",
                    "01_rbok/chapter-2.md",
                ],
            }
        ],
        "skipped": [],
        "ignored_generated_paths": [],
    }


def _build(**overrides) -> dict:
    """Helper: build a baseline artifact_only manifest, mutating any
    provided sub-dicts at the top level."""
    kwargs: dict[str, Any] = dict(
        schema_version="0.1.0",
        event="pull_request",
        workflow_run_id="16842073921",
        generated_at=FROZEN_TIME,
        corpus=_baseline_corpus(),
        scope=_baseline_scope(),
        diff=_baseline_diff(),
        output=_baseline_output_for_mode("artifact_only"),
        artifacts=_baseline_artifacts(),
        policy=_baseline_policy("artifact_only"),
    )
    kwargs.update(overrides)
    return gen.build_manifest(**kwargs)


class TestBuildManifestArtifactOnly(unittest.TestCase):
    def test_artifact_only_full_manifest(self):
        manifest = _build()
        self.assertEqual(manifest["schema_version"], "0.1.0")
        self.assertEqual(manifest["run"]["event"], "pull_request")
        self.assertEqual(manifest["scope"]["id"], "rbok-lawbook")
        self.assertEqual(manifest["policy"]["publish_mode"], "artifact_only")
        self.assertIn("artifacts", manifest)
        self.assertEqual(manifest["artifacts"]["feed"], "feed.json")

        with tempfile.TemporaryDirectory() as tmp:
            yaml_path = os.path.join(tmp, "trace.yaml")
            json_path = os.path.join(tmp, "trace.json")
            gen.write_manifest(manifest, yaml_path=yaml_path, json_path=json_path)
            with open(yaml_path, encoding="utf-8") as fh:
                import yaml as _y
                yaml_doc = _y.safe_load(fh)
            with open(json_path, encoding="utf-8") as fh:
                json_doc = json.load(fh)
            self.assertEqual(yaml_doc, json_doc)


class TestBuildManifestRequiredFields(unittest.TestCase):
    def test_pull_request_requires_branch_and_commit_sha(self):
        out = _baseline_output_for_mode("pull_request")
        out["commit_sha"] = ""
        with self.assertRaisesRegex(ValueError, r"output\.commit_sha"):
            _build(output=out, policy=_baseline_policy("pull_request"))

    def test_direct_push_requires_branch_and_commit_sha(self):
        out = _baseline_output_for_mode("direct_push")
        out["branch"] = ""
        with self.assertRaisesRegex(ValueError, r"output\.branch"):
            _build(output=out, policy=_baseline_policy("direct_push"))

    def test_artifact_only_requires_artifacts(self):
        with self.assertRaisesRegex(ValueError, r"artifacts must contain at least one entry"):
            _build(artifacts={}, policy=_baseline_policy("artifact_only"))

    def test_missing_corpus_base_sha(self):
        corpus = _baseline_corpus()
        corpus["base_sha"] = ""
        with self.assertRaisesRegex(ValueError, r"corpus\.base_sha"):
            _build(corpus=corpus)

    def test_missing_scope_id(self):
        scope = _baseline_scope()
        scope["id"] = ""
        with self.assertRaisesRegex(ValueError, r"scope\.id"):
            _build(scope=scope)

    def test_missing_output_path(self):
        out = _baseline_output_for_mode("artifact_only")
        out["path"] = ""
        with self.assertRaisesRegex(ValueError, r"output\.path"):
            _build(output=out)

    def test_invalid_publish_mode(self):
        policy = _baseline_policy("artifact_only")
        policy["publish_mode"] = "weird"
        with self.assertRaisesRegex(ValueError, r"policy\.publish_mode"):
            _build(policy=policy)


class TestDeriveFromDiffPlan(unittest.TestCase):
    def test_derive_from_diff_plan(self):
        derived = gen.derive_from_diff_plan(_baseline_diff_plan(), "rbok-lawbook")
        self.assertEqual(derived["scope"]["id"], "rbok-lawbook")
        self.assertEqual(derived["scope"]["paths"], ["01_rbok/**"])
        self.assertTrue(derived["diff"]["impacted"])
        self.assertEqual(
            derived["diff"]["changed_paths"],
            ["01_rbok/chapter-1.md", "01_rbok/chapter-2.md"],
        )

    def test_derive_from_diff_plan_unknown_workflow(self):
        with self.assertRaisesRegex(ValueError, r"nope"):
            gen.derive_from_diff_plan(_baseline_diff_plan(), "nope")


class TestDeterminism(unittest.TestCase):
    def test_determinism_frozen_time(self):
        manifest_a = _build()
        manifest_b = _build()
        with tempfile.TemporaryDirectory() as tmp:
            ya = os.path.join(tmp, "a.yaml")
            ja = os.path.join(tmp, "a.json")
            yb = os.path.join(tmp, "b.yaml")
            jb = os.path.join(tmp, "b.json")
            gen.write_manifest(manifest_a, yaml_path=ya, json_path=ja)
            gen.write_manifest(manifest_b, yaml_path=yb, json_path=jb)
            self.assertEqual(Path(ya).read_bytes(), Path(yb).read_bytes())
            self.assertEqual(Path(ja).read_bytes(), Path(jb).read_bytes())


class TestExtraArtifactCollection(unittest.TestCase):
    def test_extra_artifact_dedup_and_sort(self):
        # Simulate the CLI parsing path.
        argv = [
            "--diff-plan", "ignored",  # not used by this test
            "--workflow-id", "rbok-lawbook",
            "--event", "push",
            "--workflow-run-id", "1",
            "--corpus-repo", "a/b",
            "--corpus-base-ref", "main",
            "--corpus-base-sha", "1234567",
            "--corpus-head-ref", "main",
            "--corpus-head-sha", "abcdef0",
            "--output-repo", "corpus",
            "--output-path", "out/",
            "--publish-mode", "artifact_only",
            "--risk-class", "low",
            "--generated-path-guard", "pass",
            "--source-read-only-guard", "pass",
            "--artifact-feed", "feed.json",
            "--extra-artifact", "custom_one=v1.json",
            "--extra-artifact", "custom_one=v2.json",  # last wins
            "--extra-artifact", "alpha=alpha.txt",
            "--out-yaml", "ignored.yaml",
            "--out-json", "ignored.json",
        ]
        ns = gen._parse_args(argv)
        artifacts = gen._collect_artifacts(ns)
        # Sorted alphabetically.
        self.assertEqual(list(artifacts.keys()), ["alpha", "custom_one", "feed"])
        # Last value wins for the duplicated key.
        self.assertEqual(artifacts["custom_one"], "v2.json")


@unittest.skipUnless(
    shutil.which("cue") is not None,
    "cue not on PATH; skipping cue vet integration test",
)
class TestCueVet(unittest.TestCase):
    def test_yaml_passes_cue_vet(self):
        manifest = _build()
        with tempfile.TemporaryDirectory() as tmp:
            yaml_path = os.path.join(tmp, "trace.yaml")
            json_path = os.path.join(tmp, "trace.json")
            gen.write_manifest(manifest, yaml_path=yaml_path, json_path=json_path)
            schema = SPECS_DIR / "nomos-trace-manifest.cue"
            proc = subprocess.run(
                ["cue", "vet", str(schema), yaml_path, "-d", "#NomosTraceManifest"],
                capture_output=True,
                text=True,
            )
            self.assertEqual(
                proc.returncode,
                0,
                msg=f"cue vet failed:\nSTDOUT:{proc.stdout}\nSTDERR:{proc.stderr}",
            )


class TestCLI(unittest.TestCase):
    def test_cli_writes_both_yaml_and_json(self):
        with tempfile.TemporaryDirectory() as tmp:
            diff_plan_path = os.path.join(tmp, "diff.json")
            with open(diff_plan_path, "w", encoding="utf-8") as fh:
                json.dump(_baseline_diff_plan(), fh)
            yaml_path = os.path.join(tmp, "trace.yaml")
            json_path = os.path.join(tmp, "trace.json")
            argv = [
                sys.executable,
                str(SCRIPTS_DIR / "nomos_trace_manifest.py"),
                "--diff-plan", diff_plan_path,
                "--workflow-id", "rbok-lawbook",
                "--event", "pull_request",
                "--workflow-run-id", "16842073921",
                "--corpus-repo", "RBOKproject/realisons-business",
                "--corpus-base-ref", "main",
                "--corpus-base-sha", "1f9d2c8b07e3a4f5d6c7b8a9e0d1c2b3a4f5d6c7",
                "--corpus-head-ref", "feature/update-guide",
                "--corpus-head-sha", "9b2a3c4d5e6f7081923a4b5c6d7e8f90a1b2c3d4",
                "--pull-request", "123",
                "--output-repo", "RBOKproject/nomos-rbok-artifacts",
                "--output-branch", "main",
                "--output-path", "rbok-lawbook/",
                "--output-commit-sha", "a1b2c3d4e5f6708192a3b4c5d6e7f8091a2b3c4d",
                "--publish-mode", "direct_push",
                "--risk-class", "low",
                "--generated-path-guard", "pass",
                "--source-read-only-guard", "pass",
                "--artifact-feed", "feed.json",
                "--out-yaml", yaml_path,
                "--out-json", json_path,
                "--frozen-time", FROZEN_TIME,
                "--no-cue-vet",
            ]
            proc = subprocess.run(argv, capture_output=True, text=True)
            self.assertEqual(
                proc.returncode,
                0,
                msg=f"cli exited {proc.returncode}\nSTDOUT:{proc.stdout}\nSTDERR:{proc.stderr}",
            )
            self.assertTrue(os.path.isfile(yaml_path))
            self.assertTrue(os.path.isfile(json_path))
            import yaml as _y
            with open(yaml_path, encoding="utf-8") as fh:
                yaml_doc = _y.safe_load(fh)
            with open(json_path, encoding="utf-8") as fh:
                json_doc = json.load(fh)
            self.assertEqual(yaml_doc, json_doc)
            self.assertEqual(yaml_doc["scope"]["id"], "rbok-lawbook")
            self.assertEqual(yaml_doc["policy"]["publish_mode"], "direct_push")


if __name__ == "__main__":
    unittest.main()

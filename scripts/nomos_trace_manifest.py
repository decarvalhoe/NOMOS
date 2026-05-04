#!/usr/bin/env python3
"""NGW-07 (#392) — mandatory NOMOS trace manifest generator.

Every NOMOS GitHub workflow run MUST emit a trace manifest regardless of
publication mode (artifact_only / pull_request / direct_push). This script
is the single source of truth for that artifact: it writes BOTH
``nomos-trace.yaml`` and ``nomos-trace.json``, validating the payload
against the mandatory-field invariants documented on
``specs/nomos-trace-manifest.cue`` (#387) BEFORE it touches the
filesystem.

Other workflow jobs (the publisher #390, the source PR commenter #391)
consume the artifact this script produces; they MUST NOT synthesize their
own trace blob beyond the placeholder paths inherited from earlier NGW
phases. NGW-010 (#395) wires the consumer side end-to-end.

Pure Python 3.10+. Allowed dependency: ``pyyaml`` (already in tree).
"""

from __future__ import annotations

import argparse
import datetime as _dt
import json
import os
import re
import shutil
import subprocess
import sys
from typing import Any, Iterable

import yaml

# ---------------------------------------------------------------------------
# Constants mirroring specs/nomos-trace-manifest.cue (#387).
# ---------------------------------------------------------------------------

SCHEMA_VERSION_DEFAULT = "0.1.0"
SCHEMA_VERSION_RE = re.compile(r"^0\.[0-9]+\.[0-9]+$")
HEX_SHA_RE = re.compile(r"^[0-9a-f]{7,40}$")
REPO_RE = re.compile(r"^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$")
RFC3339_RE = re.compile(
    r"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?Z$"
)

VALID_EVENTS = (
    "pull_request",
    "push",
    "repository_dispatch",
    "workflow_dispatch",
    "schedule",
)
VALID_PUBLISH_MODES = ("artifact_only", "pull_request", "direct_push")
VALID_RISK_CLASSES = ("low", "medium", "high", "regulated")
VALID_GUARD_STATES = ("pass", "fail", "skipped")

# Five canonical artifact keys recognised by the schema; additional keys
# are accepted as free-form string entries (open map in the CUE schema).
CANONICAL_ARTIFACT_KEYS = (
    "attestation",
    "body_ledger",
    "diff_report",
    "feed",
    "rag_metadata",
)

OUTPUT_REPO_SENTINELS = ("corpus", "output")


# ---------------------------------------------------------------------------
# Field-by-field validators. Each raises ValueError with a stable,
# single-line message naming the failing field — operators read these
# straight out of CI logs.
# ---------------------------------------------------------------------------


def _validate_schema_version(value: str) -> None:
    if not isinstance(value, str) or not SCHEMA_VERSION_RE.match(value):
        raise ValueError(f"schema_version invalid: {value!r} (expected pattern 0.X.Y)")


def _validate_event(value: str) -> None:
    if value not in VALID_EVENTS:
        raise ValueError(
            f"run.event invalid: {value!r} (expected one of {list(VALID_EVENTS)})"
        )


def _validate_workflow_run_id(value: str) -> None:
    if not isinstance(value, str) or not value.strip():
        raise ValueError("run.workflow_run_id must be non-empty")


def _validate_generated_at(value: str) -> None:
    if not isinstance(value, str) or not RFC3339_RE.match(value):
        raise ValueError(
            f"run.generated_at invalid: {value!r} (expected UTC RFC3339, e.g. 2026-05-04T12:00:00Z)"
        )


def _validate_corpus(corpus: dict) -> None:
    if not isinstance(corpus, dict):
        raise ValueError("corpus must be an object")
    repo = corpus.get("repo")
    if not isinstance(repo, str) or not REPO_RE.match(repo):
        raise ValueError(f"corpus.repo invalid: {repo!r} (expected owner/name)")
    for field in ("base_ref", "head_ref"):
        v = corpus.get(field)
        if not isinstance(v, str) or not v.strip():
            raise ValueError(f"corpus.{field} must be non-empty")
    for field in ("base_sha", "head_sha"):
        v = corpus.get(field)
        if not isinstance(v, str) or not HEX_SHA_RE.match(v):
            raise ValueError(
                f"corpus.{field} invalid: {v!r} (expected 7-40 hex characters)"
            )
    pr = corpus.get("pull_request")
    if pr is not None and not (isinstance(pr, int) and pr > 0):
        raise ValueError(
            f"corpus.pull_request invalid: {pr!r} (expected positive integer or absent)"
        )


def _validate_scope(scope: dict) -> None:
    if not isinstance(scope, dict):
        raise ValueError("scope must be an object")
    sid = scope.get("id")
    if not isinstance(sid, str) or not sid.strip():
        raise ValueError("scope.id must be non-empty")
    paths = scope.get("paths")
    if not isinstance(paths, list) or len(paths) == 0:
        raise ValueError("scope.paths must be a non-empty list of glob strings")
    for i, p in enumerate(paths):
        if not isinstance(p, str) or not p.strip():
            raise ValueError(f"scope.paths[{i}] must be a non-empty string")


def _validate_diff(diff: dict) -> None:
    if not isinstance(diff, dict):
        raise ValueError("diff must be an object")
    cp = diff.get("changed_paths")
    if not isinstance(cp, list):
        raise ValueError("diff.changed_paths must be a list (possibly empty)")
    for i, p in enumerate(cp):
        if not isinstance(p, str):
            raise ValueError(f"diff.changed_paths[{i}] must be a string")
    if not isinstance(diff.get("impacted"), bool):
        raise ValueError("diff.impacted must be a boolean")


def _validate_policy(policy: dict) -> None:
    if not isinstance(policy, dict):
        raise ValueError("policy must be an object")
    mode = policy.get("publish_mode")
    if mode not in VALID_PUBLISH_MODES:
        raise ValueError(
            f"policy.publish_mode invalid: {mode!r} (expected one of {list(VALID_PUBLISH_MODES)})"
        )
    risk = policy.get("risk_class")
    if risk not in VALID_RISK_CLASSES:
        raise ValueError(
            f"policy.risk_class invalid: {risk!r} (expected one of {list(VALID_RISK_CLASSES)})"
        )
    for field in ("generated_path_guard", "source_read_only_guard"):
        v = policy.get(field)
        if v not in VALID_GUARD_STATES:
            raise ValueError(
                f"policy.{field} invalid: {v!r} (expected one of {list(VALID_GUARD_STATES)})"
            )


def _validate_output(output: dict, policy: dict) -> None:
    if not isinstance(output, dict):
        raise ValueError("output must be an object")
    repo = output.get("repo")
    if not isinstance(repo, str) or (repo not in OUTPUT_REPO_SENTINELS and not REPO_RE.match(repo)):
        raise ValueError(
            f"output.repo invalid: {repo!r} (expected owner/name, 'corpus', or 'output')"
        )
    path = output.get("path")
    if not isinstance(path, str) or not path.strip():
        raise ValueError("output.path must be non-empty")
    mode = policy.get("publish_mode")
    if mode in ("pull_request", "direct_push"):
        branch = output.get("branch")
        if not isinstance(branch, str) or not branch.strip():
            raise ValueError(
                f"output.branch must be non-empty when policy.publish_mode={mode!r}"
            )
        commit = output.get("commit_sha")
        if not isinstance(commit, str) or not HEX_SHA_RE.match(commit):
            raise ValueError(
                f"output.commit_sha invalid: {commit!r} (expected 7-40 hex when policy.publish_mode={mode!r})"
            )


def _validate_artifacts(artifacts: dict | None, policy: dict) -> dict:
    """Returns the normalised (sorted) artifacts dict. Empty dict is dropped
    when the publish mode does not require any artifact entry; a non-empty
    dict is always emitted under the canonical key order."""
    if artifacts is None:
        artifacts = {}
    if not isinstance(artifacts, dict):
        raise ValueError("artifacts must be an object (mapping name -> path)")
    cleaned: dict[str, str] = {}
    for k, v in artifacts.items():
        if not isinstance(k, str) or not k.strip():
            raise ValueError(f"artifacts key invalid: {k!r} (expected non-empty string)")
        if not isinstance(v, str) or not v.strip():
            raise ValueError(f"artifacts[{k!r}] invalid: {v!r} (expected non-empty path string)")
        cleaned[k] = v
    if policy.get("publish_mode") == "artifact_only" and not cleaned:
        raise ValueError(
            "artifacts must contain at least one entry when policy.publish_mode='artifact_only'"
        )
    return dict(sorted(cleaned.items()))


# ---------------------------------------------------------------------------
# Builders — each takes a checked input dict and returns the wire shape.
# ---------------------------------------------------------------------------


def _build_corpus(corpus: dict) -> dict:
    out = {
        "repo": corpus["repo"],
        "base_ref": corpus["base_ref"],
        "base_sha": corpus["base_sha"],
        "head_ref": corpus["head_ref"],
        "head_sha": corpus["head_sha"],
    }
    if corpus.get("pull_request") is not None:
        out["pull_request"] = int(corpus["pull_request"])
    return out


def _build_scope(scope: dict) -> dict:
    return {"id": scope["id"], "paths": list(scope["paths"])}


def _build_diff(diff: dict) -> dict:
    return {
        "changed_paths": list(diff["changed_paths"]),
        "impacted": bool(diff["impacted"]),
    }


def _build_output(output: dict) -> dict:
    out = {"repo": output["repo"], "path": output["path"]}
    if output.get("branch") is not None and str(output["branch"]).strip():
        out["branch"] = output["branch"]
    if output.get("commit_sha") is not None and str(output["commit_sha"]).strip():
        out["commit_sha"] = output["commit_sha"]
    return out


def _build_policy(policy: dict) -> dict:
    return {
        "publish_mode": policy["publish_mode"],
        "risk_class": policy["risk_class"],
        "generated_path_guard": policy["generated_path_guard"],
        "source_read_only_guard": policy["source_read_only_guard"],
    }


# ---------------------------------------------------------------------------
# Public API.
# ---------------------------------------------------------------------------


def build_manifest(
    *,
    schema_version: str,
    event: str,
    workflow_run_id: str,
    generated_at: str,
    corpus: dict,
    scope: dict,
    diff: dict,
    output: dict,
    artifacts: dict | None,
    policy: dict,
) -> dict:
    """Pure function. Validates, then returns the manifest as a dict ready
    for YAML/JSON serialisation. Raises ValueError on any failed mandatory
    invariant; the message names the failing field. No partial result is
    ever returned; callers that catch the error must NOT write a manifest.
    """
    _validate_schema_version(schema_version)
    _validate_event(event)
    _validate_workflow_run_id(workflow_run_id)
    _validate_generated_at(generated_at)
    _validate_corpus(corpus)
    _validate_scope(scope)
    _validate_diff(diff)
    _validate_policy(policy)
    _validate_output(output, policy)
    artifacts_norm = _validate_artifacts(artifacts, policy)

    manifest: dict[str, Any] = {
        "schema_version": schema_version,
        "run": {
            "event": event,
            "workflow_run_id": workflow_run_id,
            "generated_at": generated_at,
        },
        "corpus": _build_corpus(corpus),
        "scope": _build_scope(scope),
        "diff": _build_diff(diff),
        "output": _build_output(output),
        "policy": _build_policy(policy),
    }
    if artifacts_norm:
        manifest["artifacts"] = artifacts_norm
    return manifest


def write_manifest(manifest: dict, *, yaml_path: str, json_path: str) -> None:
    """Writes both forms. JSON uses ``sort_keys=True`` for determinism;
    YAML uses ``default_flow_style=False`` and ``sort_keys=True``."""
    yaml_blob = yaml.safe_dump(manifest, default_flow_style=False, sort_keys=True)
    json_blob = json.dumps(manifest, sort_keys=True, indent=2) + "\n"
    with open(yaml_path, "w", encoding="utf-8") as fh:
        fh.write(yaml_blob)
    with open(json_path, "w", encoding="utf-8") as fh:
        fh.write(json_blob)


def derive_from_diff_plan(diff_plan: dict, workflow_id: str) -> dict:
    """Extract scope.id/paths and diff.changed_paths/impacted from a #388
    DiffPlan JSON. Raises ValueError when ``workflow_id`` is not in the
    impacted list."""
    if not isinstance(diff_plan, dict):
        raise ValueError("diff_plan must be a dict")
    if not isinstance(workflow_id, str) or not workflow_id.strip():
        raise ValueError("workflow_id must be non-empty")
    impacted_list = diff_plan.get("impacted") or []
    for entry in impacted_list:
        if not isinstance(entry, dict):
            continue
        if entry.get("workflow_id") == workflow_id:
            return {
                "scope": {
                    "id": entry.get("scope_id") or workflow_id,
                    "paths": list(entry.get("scope_paths") or []),
                },
                "diff": {
                    "changed_paths": list(entry.get("matched_paths") or []),
                    "impacted": True,
                },
            }
    raise ValueError(
        f"workflow_id {workflow_id!r} not present in diff_plan.impacted[] "
        f"({len(impacted_list)} impacted entr{'y' if len(impacted_list) == 1 else 'ies'})"
    )


# ---------------------------------------------------------------------------
# CLI.
# ---------------------------------------------------------------------------


def _parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        prog="nomos_trace_manifest",
        description="Generate the mandatory NOMOS trace manifest (NGW-07 #392).",
    )
    p.add_argument("--diff-plan", required=True, help="path to nomos-diff.json from NGW-03 (#388)")
    p.add_argument("--workflow-id", required=True, help="scope id; must match diff_plan.impacted[*].workflow_id")
    p.add_argument("--schema-version", default=SCHEMA_VERSION_DEFAULT)
    p.add_argument("--event", required=True, choices=list(VALID_EVENTS))
    p.add_argument("--workflow-run-id", required=True)
    p.add_argument("--corpus-repo", required=True)
    p.add_argument("--corpus-base-ref", required=True)
    p.add_argument("--corpus-base-sha", required=True)
    p.add_argument("--corpus-head-ref", required=True)
    p.add_argument("--corpus-head-sha", required=True)
    p.add_argument("--pull-request", type=int, default=None)
    p.add_argument("--output-repo", required=True)
    p.add_argument("--output-branch", default="")
    p.add_argument("--output-path", required=True)
    p.add_argument("--output-commit-sha", default="")
    p.add_argument("--publish-mode", required=True, choices=list(VALID_PUBLISH_MODES))
    p.add_argument("--risk-class", required=True, choices=list(VALID_RISK_CLASSES))
    p.add_argument("--generated-path-guard", required=True, choices=list(VALID_GUARD_STATES))
    p.add_argument("--source-read-only-guard", required=True, choices=list(VALID_GUARD_STATES))
    p.add_argument("--artifact-feed", default="")
    p.add_argument("--artifact-body-ledger", default="")
    p.add_argument("--artifact-rag-metadata", default="")
    p.add_argument("--artifact-attestation", default="")
    p.add_argument("--artifact-diff-report", default="")
    p.add_argument(
        "--extra-artifact",
        action="append",
        default=[],
        metavar="KEY=PATH",
        help="additional artifact entry; repeatable; last value wins per key",
    )
    p.add_argument("--out-yaml", required=True)
    p.add_argument("--out-json", required=True)
    p.add_argument("--frozen-time", default="", help="override generated_at (RFC3339, ending with Z)")
    p.add_argument("--no-cue-vet", action="store_true", help="skip the post-write `cue vet` self-check")
    return p.parse_args(argv)


def _resolve_generated_at(frozen: str) -> str:
    if frozen.strip():
        if not RFC3339_RE.match(frozen):
            raise ValueError(
                f"--frozen-time invalid: {frozen!r} (expected UTC RFC3339, e.g. 2026-05-04T12:00:00Z)"
            )
        return frozen
    now = _dt.datetime.now(_dt.timezone.utc).replace(microsecond=0)
    return now.strftime("%Y-%m-%dT%H:%M:%SZ")


def _collect_artifacts(args: argparse.Namespace) -> dict[str, str]:
    """Build the artifact map from the canonical --artifact-* flags plus
    --extra-artifact key=value entries. Repeated keys: last value wins.
    The result is alphabetically sorted by write_manifest, but we also
    sort here so test fixtures see the same ordering."""
    out: dict[str, str] = {}
    canonical_pairs = (
        ("feed", args.artifact_feed),
        ("body_ledger", args.artifact_body_ledger),
        ("rag_metadata", args.artifact_rag_metadata),
        ("attestation", args.artifact_attestation),
        ("diff_report", args.artifact_diff_report),
    )
    for key, value in canonical_pairs:
        if value and value.strip():
            out[key] = value.strip()
    for raw in args.extra_artifact or []:
        if "=" not in raw:
            raise ValueError(
                f"--extra-artifact must be KEY=PATH; got {raw!r}"
            )
        k, _, v = raw.partition("=")
        k = k.strip()
        v = v.strip()
        if not k or not v:
            raise ValueError(
                f"--extra-artifact KEY=PATH must have non-empty parts; got {raw!r}"
            )
        out[k] = v  # last wins
    return dict(sorted(out.items()))


def _cue_vet(yaml_path: str) -> tuple[bool, str]:
    """Run `cue vet` against #387's schema if `cue` is on PATH. Returns
    (ran, message). When cue is missing, returns (False, "cue not on PATH")."""
    if shutil.which("cue") is None:
        return False, "cue not on PATH"
    schema = os.path.join(
        os.path.dirname(__file__), "..", "specs", "nomos-trace-manifest.cue"
    )
    schema = os.path.normpath(schema)
    proc = subprocess.run(
        ["cue", "vet", schema, yaml_path, "-d", "#NomosTraceManifest"],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        return True, (proc.stderr or proc.stdout or "cue vet failed").strip()
    return True, "cue vet OK"


def main(argv: list[str] | None = None) -> int:
    try:
        args = _parse_args(argv)
        with open(args.diff_plan, "r", encoding="utf-8") as fh:
            diff_plan = json.load(fh)
        derived = derive_from_diff_plan(diff_plan, args.workflow_id)
        artifacts = _collect_artifacts(args)
        manifest = build_manifest(
            schema_version=args.schema_version,
            event=args.event,
            workflow_run_id=args.workflow_run_id,
            generated_at=_resolve_generated_at(args.frozen_time),
            corpus={
                "repo": args.corpus_repo,
                "base_ref": args.corpus_base_ref,
                "base_sha": args.corpus_base_sha,
                "head_ref": args.corpus_head_ref,
                "head_sha": args.corpus_head_sha,
                "pull_request": args.pull_request,
            },
            scope=derived["scope"],
            diff=derived["diff"],
            output={
                "repo": args.output_repo,
                "branch": args.output_branch,
                "path": args.output_path,
                "commit_sha": args.output_commit_sha,
            },
            artifacts=artifacts,
            policy={
                "publish_mode": args.publish_mode,
                "risk_class": args.risk_class,
                "generated_path_guard": args.generated_path_guard,
                "source_read_only_guard": args.source_read_only_guard,
            },
        )
    except (ValueError, FileNotFoundError, json.JSONDecodeError) as exc:
        print(f"nomos_trace_manifest: {exc}", file=sys.stderr)
        return 1

    write_manifest(manifest, yaml_path=args.out_yaml, json_path=args.out_json)

    if not args.no_cue_vet:
        ran, msg = _cue_vet(args.out_yaml)
        if ran and msg != "cue vet OK":
            print(f"nomos_trace_manifest: cue vet failed: {msg}", file=sys.stderr)
            return 1
        if not ran:
            print(f"nomos_trace_manifest: {msg}; skipping schema validation (best-effort only)")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())

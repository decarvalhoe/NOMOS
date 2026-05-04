#!/usr/bin/env python3
"""NGW-005 (#390) — output publisher and generated path guard.

This script takes the diff plan from NGW-04 (#388/#389), the directory of
NOMOS-generated outputs, and the workflow config (#386), and publishes the
outputs according to ``publish.mode``:

* ``artifact_only`` — copy outputs + trace manifest into a deterministic
  location for upload as a GitHub Actions artifact. Never opens a PR or
  pushes a commit.
* ``pull_request`` — plan (and optionally execute via ``gh``) a PR against
  the output repository. v0.1 exercises the dry-run path only.
* ``direct_push`` — plan (and optionally execute via ``git``) a direct
  commit on ``target_branch``/``target_path``. Allowed only when the
  workflow config explicitly requested this mode; the path guard runs
  before any push could happen.

The path guard rejects any write that escapes ``target_path`` (literal
``..`` segments, absolute paths, drive letters, symlinks). The anti-loop
marker is appended to the body of every NOMOS-generated commit so that
follow-up runs can recognise their own outputs and avoid infinite loops.
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

ANTI_LOOP_HEADER = "[nomos-generated]"

VALID_MODES = ("artifact_only", "pull_request", "direct_push")
VALID_BRANCH_STRATEGIES = ("fixed", "per_pr", "per_source_ref", "dated")


# ---------------------------------------------------------------------------
# Path guard
# ---------------------------------------------------------------------------


def _validate_target_path(target_path: str) -> None:
    """Defensive check on the target_path config value.

    The CUE schema (#386) already rejects absolute paths and ``..`` segments,
    but we re-check at runtime so the publisher cannot be subverted by a
    malformed config that slipped past schema validation.
    """
    if not target_path:
        raise ValueError("target_path must be non-empty")
    if os.path.isabs(target_path):
        raise ValueError(f"target_path must be relative: {target_path!r}")
    if len(target_path) >= 2 and target_path[1] == ":":
        raise ValueError(f"target_path must not contain a drive letter: {target_path!r}")
    parts = target_path.replace("\\", "/").split("/")
    if ".." in parts:
        raise ValueError(f"target_path must not contain '..': {target_path!r}")


def _is_violating(candidate: str, norm_target: str) -> tuple[bool, str]:
    """Return ``(is_violation, reason)`` for one candidate path."""
    if not candidate:
        return True, "empty path"
    if os.path.isabs(candidate):
        return True, "absolute path"
    if len(candidate) >= 2 and candidate[1] == ":":
        return True, "windows drive letter"
    raw_parts = candidate.replace("\\", "/").split("/")
    if ".." in raw_parts:
        return True, "contains '..' segment"
    norm = os.path.normpath(candidate)
    norm_slash = norm.replace("\\", "/")
    norm_parts = norm_slash.split("/")
    if ".." in norm_parts:
        return True, "normalises to escape via '..'"
    target_slash = norm_target.replace("\\", "/")
    if norm_slash != target_slash and not norm_slash.startswith(target_slash + "/"):
        return True, f"outside target_path {norm_target!r}"
    try:
        if os.path.islink(candidate):
            return True, "candidate is a symlink"
    except OSError:
        pass
    return False, ""


def validate_path_guard(target_path: str, candidate_paths: Iterable[str]) -> list[str]:
    """Return the list of candidate paths that VIOLATE the guard.

    A candidate violates when any of the following is true:

    * it is empty;
    * it is absolute (POSIX root or a Windows drive letter);
    * it contains a ``..`` segment, either literally or after
      ``os.path.normpath``;
    * after normalisation it does not equal ``target_path`` and does not
      start with ``target_path + '/'``;
    * it points at a symlink on disk.

    The empty-list return signals the guard passed.
    """
    norm_target = os.path.normpath(target_path) if target_path else ""
    violations: list[str] = []
    for p in candidate_paths:
        bad, _ = _is_violating(p, norm_target)
        if bad:
            violations.append(p)
    return violations


def violation_reasons(target_path: str, candidate_paths: Iterable[str]) -> list[tuple[str, str]]:
    """Like :func:`validate_path_guard` but returns ``(path, reason)`` tuples."""
    norm_target = os.path.normpath(target_path) if target_path else ""
    out: list[tuple[str, str]] = []
    for p in candidate_paths:
        bad, reason = _is_violating(p, norm_target)
        if bad:
            out.append((p, reason))
    return out


# ---------------------------------------------------------------------------
# Anti-loop marker
# ---------------------------------------------------------------------------


def build_anti_loop_marker(source_sha: str, scope_id: str) -> str:
    """Return the marker text appended to every NOMOS-generated commit body.

    The format is fixed so future NOMOS runs can grep for ``[nomos-generated]``
    and skip commits that they themselves produced.
    """
    if not source_sha:
        raise ValueError("source_sha must be non-empty")
    if not scope_id:
        raise ValueError("scope_id must be non-empty")
    return f"{ANTI_LOOP_HEADER}\nsource-sha: {source_sha}\nscope: {scope_id}"


def _compose_commit_message(subject: str, source_sha: str, scope_id: str) -> str:
    """Compose ``<subject>\\n\\n<marker>``. Subject defaults to a generic line."""
    marker = build_anti_loop_marker(source_sha, scope_id)
    subject = subject.strip() or "nomos-generated outputs"
    return f"{subject}\n\n{marker}"


# ---------------------------------------------------------------------------
# Branch name resolution
# ---------------------------------------------------------------------------


_BRANCH_SAFE_RE = re.compile(r"[^A-Za-z0-9._-]+")


def _sanitize_ref(ref: str) -> str:
    sanitized = _BRANCH_SAFE_RE.sub("-", ref).strip("-")
    return sanitized or "ref"


def compute_branch_name(
    strategy: str,
    workflow_id: str,
    *,
    source_pr_number: int | None = None,
    source_ref: str | None = None,
    utc_date: str | None = None,
) -> str:
    """Return the generated branch name for the chosen ``branch_strategy``.

    Strategies (matching #386):

    * ``fixed`` — single durable branch ``nomos/<workflow-id>``.
    * ``per_pr`` — one branch per source PR. Requires ``source_pr_number``.
    * ``per_source_ref`` — one branch per source ref, sanitised to a safe
      branch token. Requires ``source_ref``.
    * ``dated`` — one branch per UTC date.
    """
    if not workflow_id:
        raise ValueError("workflow_id must be non-empty")
    if strategy == "fixed":
        return f"nomos/{workflow_id}"
    if strategy == "per_pr":
        if source_pr_number is None:
            raise ValueError("per_pr branch strategy requires source_pr_number")
        return f"nomos/{workflow_id}/pr-{int(source_pr_number)}"
    if strategy == "per_source_ref":
        if not source_ref:
            raise ValueError("per_source_ref branch strategy requires source_ref")
        return f"nomos/{workflow_id}/{_sanitize_ref(source_ref)}"
    if strategy == "dated":
        date = utc_date or _dt.datetime.now(_dt.timezone.utc).strftime("%Y%m%d")
        return f"nomos/{workflow_id}/{date}"
    raise ValueError(f"unknown branch_strategy: {strategy!r}")


# ---------------------------------------------------------------------------
# File walking + candidate gathering
# ---------------------------------------------------------------------------


def _walk_files(root: str) -> list[str]:
    """Return absolute paths of every regular file under ``root`` (sorted).

    Symlinks are NOT followed and are reported through the path guard's
    symlink check rather than skipped, so an evil symlink under
    ``outputs_dir`` cannot quietly hide.
    """
    if not root or not os.path.isdir(root):
        return []
    out: list[str] = []
    for dirpath, _dirnames, filenames in os.walk(root, followlinks=False):
        for f in filenames:
            out.append(os.path.join(dirpath, f))
    out.sort()
    return out


def gather_publish_candidates(outputs_dir: str, target_path: str) -> list[tuple[str, str]]:
    """Walk ``outputs_dir`` and return ``(source_abs, dest_rel)`` tuples.

    ``dest_rel`` is the path under the output repository root the file would
    land at. The CLI runs the path guard over ``[dest_rel for _, dest_rel
    in candidates]`` plus a separate symlink check on each ``source_abs``.
    """
    candidates: list[tuple[str, str]] = []
    if not outputs_dir or not os.path.isdir(outputs_dir):
        return candidates
    target_norm = target_path.rstrip("/").replace("\\", "/")
    for src in _walk_files(outputs_dir):
        rel = os.path.relpath(src, outputs_dir).replace(os.sep, "/")
        dest = f"{target_norm}/{rel}" if target_norm else rel
        candidates.append((src, dest))
    return candidates


# ---------------------------------------------------------------------------
# Mode: artifact_only
# ---------------------------------------------------------------------------


def publish_artifact_only(
    *,
    outputs_dir: str,
    trace_manifest: str,
    diff_plan: str,
    dest_dir: str,
    dry_run: bool = True,
) -> dict[str, Any]:
    """Copy outputs + trace + diff plan to ``dest_dir`` for artifact upload.

    Never calls git or gh; never reads or writes any remote. Returns a result
    dict listing the files prepared for upload. The dry-run path leaves the
    filesystem untouched and just records the plan.
    """
    prepared: list[str] = []
    if not dry_run:
        os.makedirs(dest_dir, exist_ok=True)

    if outputs_dir and os.path.isdir(outputs_dir):
        for src in _walk_files(outputs_dir):
            rel = os.path.relpath(src, outputs_dir).replace(os.sep, "/")
            target = os.path.join(dest_dir, rel)
            if not dry_run:
                os.makedirs(os.path.dirname(target) or dest_dir, exist_ok=True)
                shutil.copy2(src, target)
            prepared.append(rel)

    trace_target = ""
    if trace_manifest and os.path.isfile(trace_manifest):
        trace_target = os.path.join(dest_dir, os.path.basename(trace_manifest))
        if not dry_run:
            shutil.copy2(trace_manifest, trace_target)

    diff_target = ""
    if diff_plan and os.path.isfile(diff_plan):
        diff_target = os.path.join(dest_dir, os.path.basename(diff_plan))
        if not dry_run:
            shutil.copy2(diff_plan, diff_target)

    return {
        "mode": "artifact_only",
        "dry_run": dry_run,
        "prepared_files": prepared,
        "trace_manifest_path": trace_target,
        "diff_plan_path": diff_target,
        "dest_dir": dest_dir,
    }


# ---------------------------------------------------------------------------
# Mode: pull_request
# ---------------------------------------------------------------------------


def publish_pull_request(
    *,
    workflow_id: str,
    branch_strategy: str,
    source_sha: str,
    source_pr_number: int | None,
    source_ref: str | None,
    target_repo: str,
    target_branch: str,
    target_path: str,
    outputs_dir: str,
    trace_manifest: str,
    commit_subject: str = "",
    dry_run: bool = True,
    run_command=None,
) -> dict[str, Any]:
    """Plan (and optionally execute) a PR against the output repository.

    v0.1 only exercises the dry-run path in tests. The real-run path shells
    out to ``gh pr create``/``gh pr edit`` via ``run_command``; tests
    monkeypatch ``run_command`` so no network call is made.
    """
    branch = compute_branch_name(
        branch_strategy,
        workflow_id,
        source_pr_number=source_pr_number,
        source_ref=source_ref,
    )
    commit_message = _compose_commit_message(commit_subject, source_sha, workflow_id)
    plan: dict[str, Any] = {
        "mode": "pull_request",
        "dry_run": dry_run,
        "branch": branch,
        "branch_strategy": branch_strategy,
        "target_repo": target_repo,
        "target_branch": target_branch,
        "target_path": target_path,
        "commit_message": commit_message,
        "anti_loop_marker": build_anti_loop_marker(source_sha, workflow_id),
        "outputs_dir": outputs_dir,
        "trace_manifest": trace_manifest,
    }
    if dry_run:
        return plan

    runner = run_command or _run_subprocess
    # Real run: shell out to gh. The actual commit/push of the generated
    # tree is left to NGW-007 (#392) and NGW-010 (#395), which close the
    # end-to-end loop. Here we only plan/refresh the PR shell.
    plan["gh_pr_create"] = runner(
        [
            "gh",
            "pr",
            "create",
            "--repo",
            target_repo,
            "--base",
            target_branch,
            "--head",
            branch,
            "--title",
            commit_subject or f"nomos: refresh {workflow_id}",
            "--body",
            commit_message,
        ]
    )
    return plan


# ---------------------------------------------------------------------------
# Mode: direct_push
# ---------------------------------------------------------------------------


def publish_direct_push(
    *,
    publish_mode: str,
    workflow_id: str,
    source_sha: str,
    target_repo: str,
    target_branch: str,
    target_path: str,
    outputs_dir: str,
    trace_manifest: str,
    commit_subject: str = "",
    dry_run: bool = True,
    run_command=None,
) -> dict[str, Any]:
    """Plan (and optionally execute) a direct push to the output repo.

    Refuses to run unless ``publish_mode == "direct_push"``. Runs the path
    guard over every file under ``outputs_dir`` BEFORE a push could ever
    happen; any violation raises ``ValueError`` so the caller exits non-zero.
    """
    if publish_mode != "direct_push":
        raise ValueError(
            "publish_direct_push requires publish.mode == 'direct_push'; got "
            f"{publish_mode!r}"
        )
    _validate_target_path(target_path)

    candidates = gather_publish_candidates(outputs_dir, target_path)
    src_symlinks = [src for src, _ in candidates if os.path.islink(src)]
    dest_paths = [dest for _, dest in candidates]
    dest_violations = validate_path_guard(target_path, dest_paths)
    if src_symlinks or dest_violations:
        msg = []
        if src_symlinks:
            msg.append(f"source symlinks: {src_symlinks}")
        if dest_violations:
            msg.append(f"destination violations: {dest_violations}")
        raise ValueError("path guard rejected direct_push: " + "; ".join(msg))

    commit_message = _compose_commit_message(commit_subject, source_sha, workflow_id)
    plan: dict[str, Any] = {
        "mode": "direct_push",
        "dry_run": dry_run,
        "target_repo": target_repo,
        "target_branch": target_branch,
        "target_path": target_path,
        "commit_message": commit_message,
        "anti_loop_marker": build_anti_loop_marker(source_sha, workflow_id),
        "tree_files": dest_paths,
        "outputs_dir": outputs_dir,
        "trace_manifest": trace_manifest,
    }
    if dry_run:
        return plan

    runner = run_command or _run_subprocess
    # Real-run path: clone target, copy files, commit, push. The full
    # implementation lands with NGW-007 (#392); v0.1 records the plan so
    # callers can surface it in trace artifacts.
    plan["git_push"] = runner(["git", "push", "origin", target_branch])
    return plan


# ---------------------------------------------------------------------------
# Subprocess wrapper (mock target for tests)
# ---------------------------------------------------------------------------


def _run_subprocess(cmd: list[str]) -> dict[str, Any]:
    """Wrapper around ``subprocess.run`` that returns a serialisable result.

    Tests monkeypatch this function (or the ``run_command`` parameter on
    the publisher entry points) so the unit tests never invoke ``git`` or
    ``gh`` for real.
    """
    completed = subprocess.run(cmd, capture_output=True, text=True, check=False)
    return {
        "argv": cmd,
        "returncode": completed.returncode,
        "stdout": completed.stdout,
        "stderr": completed.stderr,
    }


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


def _build_arg_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="nomos_github_publish",
        description=(
            "NGW-005 (#390) NOMOS output publisher with path guard and "
            "anti-loop commit marker."
        ),
    )
    p.add_argument("--config", required=True, help="path to corpus-workflows.yaml")
    p.add_argument("--workflow-id", required=True, help="scope id from the config")
    p.add_argument("--diff-plan", required=True, help="path to nomos-diff.json")
    p.add_argument("--outputs-dir", required=True, help="directory of generated outputs (may be empty in v0.1)")
    p.add_argument("--trace-manifest", required=True, help="path to nomos-trace.yaml")
    p.add_argument("--mode", required=True, choices=VALID_MODES)
    p.add_argument("--target-repo", required=True)
    p.add_argument("--target-branch", required=True)
    p.add_argument("--target-path", required=True)
    p.add_argument("--source-sha", required=True, help="full source SHA for the anti-loop marker")
    p.add_argument(
        "--branch-strategy",
        default="fixed",
        choices=VALID_BRANCH_STRATEGIES,
        help="only consulted for --mode pull_request (default: fixed)",
    )
    p.add_argument("--source-pr-number", type=int, default=None)
    p.add_argument("--source-ref", default="")
    p.add_argument(
        "--dest-dir",
        default="",
        help="artifact_only destination (default: $RUNNER_TEMP/nomos-publish)",
    )
    p.add_argument("--commit-subject", default="")
    p.add_argument("--dry-run", action="store_true", default=False)
    return p


def _resolve_pr_number(arg_value: int | None) -> int | None:
    if arg_value is not None:
        return arg_value
    env = os.environ.get("GITHUB_PR_NUMBER", "")
    if env.isdigit():
        return int(env)
    return None


def main(argv: list[str] | None = None) -> int:
    parser = _build_arg_parser()
    args = parser.parse_args(argv)

    try:
        _validate_target_path(args.target_path)
    except ValueError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2

    candidates = gather_publish_candidates(args.outputs_dir, args.target_path)
    src_symlinks = [src for src, _ in candidates if os.path.islink(src)]
    dest_paths = [dest for _, dest in candidates]
    dest_violations = violation_reasons(args.target_path, dest_paths)
    if src_symlinks or dest_violations:
        print("path guard violations:", file=sys.stderr)
        for s in src_symlinks:
            print(f"  - source symlink: {s}", file=sys.stderr)
        for path, reason in dest_violations:
            print(f"  - destination {path}: {reason}", file=sys.stderr)
        return 2

    if args.mode == "artifact_only":
        dest_dir = args.dest_dir or os.path.join(
            os.environ.get("RUNNER_TEMP", "/tmp"),
            "nomos-publish",
        )
        result = publish_artifact_only(
            outputs_dir=args.outputs_dir,
            trace_manifest=args.trace_manifest,
            diff_plan=args.diff_plan,
            dest_dir=dest_dir,
            dry_run=args.dry_run,
        )
    elif args.mode == "pull_request":
        result = publish_pull_request(
            workflow_id=args.workflow_id,
            branch_strategy=args.branch_strategy,
            source_sha=args.source_sha,
            source_pr_number=_resolve_pr_number(args.source_pr_number),
            source_ref=args.source_ref,
            target_repo=args.target_repo,
            target_branch=args.target_branch,
            target_path=args.target_path,
            outputs_dir=args.outputs_dir,
            trace_manifest=args.trace_manifest,
            commit_subject=args.commit_subject,
            dry_run=args.dry_run,
        )
    else:  # direct_push
        try:
            result = publish_direct_push(
                publish_mode=args.mode,
                workflow_id=args.workflow_id,
                source_sha=args.source_sha,
                target_repo=args.target_repo,
                target_branch=args.target_branch,
                target_path=args.target_path,
                outputs_dir=args.outputs_dir,
                trace_manifest=args.trace_manifest,
                commit_subject=args.commit_subject,
                dry_run=args.dry_run,
            )
        except ValueError as exc:
            print(f"error: {exc}", file=sys.stderr)
            return 2

    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())

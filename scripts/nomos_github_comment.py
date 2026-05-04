#!/usr/bin/env python3
"""nomos_github_comment.py — Post or update a sticky comment on the source PR.

NGW-06 (#391): the source PR commenter. When a workflow's
`notify.source_pr_comment.enabled` is True (per
`specs/nomos-github-workflow.cue`'s `#NotifySpec`), this script renders
a Markdown comment that summarises the scoped diff plan, gate status,
output location, and trace manifest, then posts it (or updates the
existing sticky one) on the source pull request.

The first line of every rendered comment is an HTML marker:

    <!-- nomos-source-pr-comment:<scope-id> -->

The marker is matched by `find_existing_comment` to update the same
comment across re-runs instead of duplicating it. Per-scope markers
keep multi-scope workflows from colliding on the same PR.

Public CLI:

    python scripts/nomos_github_comment.py \
        --config .nomos/corpus-workflows.yaml \
        --workflow-id rbok-lawbook \
        --diff-plan nomos-diff.json \
        --trace-manifest nomos-trace.yaml \
        --gate-status pass \
        --pr-number 123 \
        --repo RBOKproject/realisons-business \
        --output-location https://example/output \
        [--dry-run]

The script never imports requests or the GitHub SDK; it shells out to
`gh api` so the runner's GITHUB_TOKEN is the sole credential boundary.
Tests mock subprocess so no real HTTP traffic happens during CI.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from typing import Any

import yaml


# Allowed enums (mirror NGW-01 #NotifySpec / spec narrative).
COMMENT_MODES = ("summary", "detailed", "failures_only")
INCLUDE_KEYS = (
    "changed_scopes",
    "diff_summary",
    "output_location",
    "trace_manifest",
    "gate_status",
)
GATE_STATUSES = ("pass", "fail", "warn", "skipped")

DETAILED_PATHS_LIMIT = 20


# ---------------------------------------------------------------------------
# Pure helpers (importable, side-effect-free).
# ---------------------------------------------------------------------------


def sticky_marker(scope_id: str) -> str:
    """Return the HTML comment marker that prefixes every rendered body.

    Per-scope so multi-scope workflows on the same PR don't collide.
    """
    if not scope_id:
        raise ValueError("scope_id must be a non-empty string")
    return f"<!-- nomos-source-pr-comment:{scope_id} -->"


def comment_disabled(notify_config: dict | None) -> bool:
    """Return True when the notify block disables source PR commenting.

    Treats both a missing `notify.source_pr_comment` block AND an
    explicit `enabled: false` as disabled. A block with a truthy
    `enabled: true` is the only enabled case.
    """
    if not notify_config or not isinstance(notify_config, dict):
        return True
    block = notify_config.get("source_pr_comment")
    if not block or not isinstance(block, dict):
        return True
    return not bool(block.get("enabled", False))


def find_existing_comment(api_response: list[dict], scope_id: str) -> dict | None:
    """Return the first PR comment whose body starts with the per-scope marker."""
    marker = sticky_marker(scope_id)
    for comment in api_response or []:
        body = (comment or {}).get("body", "")
        if isinstance(body, str) and body.startswith(marker):
            return comment
    return None


def _format_changed_scopes(
    diff_plan: dict, scope_id: str, gate_status: str
) -> list[str]:
    impacted = (diff_plan or {}).get("impacted") or []
    matched = [w for w in impacted if (w or {}).get("id") == scope_id]
    paths: list[str] = []
    for w in matched:
        paths.extend(w.get("changed_paths") or [])
    if not matched and not paths:
        # Fall back to the top-level changed_paths list when the diff plan
        # uses a flat shape rather than a per-scope `impacted[].changed_paths`.
        paths = list((diff_plan or {}).get("changed_paths") or [])
    lines = [
        "### Impacted scopes",
        f"- `{scope_id}` · gate **{gate_status}** · changed paths: {len(paths)}",
    ]
    return lines


def _format_diff_summary(diff_plan: dict, scope_id: str, mode: str) -> list[str]:
    paths: list[str] = []
    impacted = (diff_plan or {}).get("impacted") or []
    for w in impacted:
        if (w or {}).get("id") == scope_id:
            paths.extend(w.get("changed_paths") or [])
    if not paths:
        paths = list((diff_plan or {}).get("changed_paths") or [])
    lines = ["### Diff summary", f"- changed paths: {len(paths)}"]
    if mode == "detailed":
        if not paths:
            lines.append("- _no changed paths_")
        else:
            shown = paths[:DETAILED_PATHS_LIMIT]
            extra = max(0, len(paths) - DETAILED_PATHS_LIMIT)
            for p in shown:
                lines.append(f"  - `{p}`")
            if extra:
                lines.append(f"  - `... +{extra} more`")
    return lines


def _format_output_location(output_location: str | None) -> list[str]:
    return [
        "### Output location",
        f"- {output_location if output_location else 'n/a'}",
    ]


def _format_trace_manifest(trace_manifest_ref: str | None) -> list[str]:
    return [
        "### Trace manifest",
        f"- {trace_manifest_ref if trace_manifest_ref else 'n/a'}",
    ]


def _format_gate_status(gate_status: str) -> list[str]:
    return [
        "### Gate status",
        f"- **{gate_status}**",
    ]


def format_comment(
    mode: str,
    include: list[str],
    diff_plan: dict,
    trace_manifest_ref: str,
    gate_status: str,
    output_location: str | None,
    scope_id: str,
) -> str:
    """Render the Markdown comment body, prefixed with the sticky marker.

    The first line is always the per-scope marker; the second line is
    blank; from the third line onward is the rendered body. Sub-blocks
    are gated by the `include[]` list so callers see only what they
    asked for. For `failures_only + pass`, the function returns a
    minimal "all clear" placeholder so the sticky comment stays alive
    instead of being orphaned.
    """
    if mode not in COMMENT_MODES:
        raise ValueError(
            f"unknown mode {mode!r}; expected one of {list(COMMENT_MODES)}"
        )
    if gate_status not in GATE_STATUSES:
        raise ValueError(
            f"unknown gate_status {gate_status!r}; expected one of {list(GATE_STATUSES)}"
        )

    marker = sticky_marker(scope_id)

    # failures_only + pass: emit marker + minimal placeholder so the
    # sticky comment is updated to reflect "no current findings"
    # rather than being orphaned at the previous (failed) state.
    if mode == "failures_only" and gate_status == "pass":
        return (
            f"{marker}\n\n"
            f"_No findings reported. Last gate status: **pass**._"
        )

    selected = [k for k in include if k in INCLUDE_KEYS]
    sections: list[list[str]] = []

    if "changed_scopes" in selected:
        sections.append(_format_changed_scopes(diff_plan, scope_id, gate_status))
    if "diff_summary" in selected:
        sections.append(_format_diff_summary(diff_plan, scope_id, mode))
    if "output_location" in selected:
        sections.append(_format_output_location(output_location))
    if "trace_manifest" in selected:
        sections.append(_format_trace_manifest(trace_manifest_ref))
    if "gate_status" in selected:
        sections.append(_format_gate_status(gate_status))

    if mode == "detailed":
        # Append a policy block from the trace manifest reference if it
        # looks like a parsed dict (caller may pass either a path string
        # OR a parsed manifest dict; we accept both shapes).
        if isinstance(trace_manifest_ref, dict):
            policy = trace_manifest_ref.get("policy") or {}
            sections.append(
                [
                    "### Policy",
                    f"- publish_mode: `{policy.get('publish_mode', 'n/a')}`",
                    f"- risk_class: `{policy.get('risk_class', 'n/a')}`",
                ]
            )

    body_parts: list[str] = []
    for sec in sections:
        body_parts.append("\n".join(sec))
    body = "\n\n".join(body_parts) if body_parts else "_no content selected_"

    return f"{marker}\n\n{body}"


# ---------------------------------------------------------------------------
# Side-effecting helpers — shell out to `gh api`.
# ---------------------------------------------------------------------------


def _gh_api(args: list[str]) -> str:
    """Run `gh api` with the supplied arguments and return stdout text."""
    cmd = ["gh", "api", *args]
    proc = subprocess.run(cmd, capture_output=True, text=True, check=True)
    return proc.stdout


def fetch_pr_comments(repo: str, pr_number: int) -> list[dict]:
    """Fetch the PR's issue-comments list via `gh api`."""
    out = _gh_api(
        [
            f"repos/{repo}/issues/{pr_number}/comments",
            "--paginate",
        ]
    )
    out = out.strip()
    if not out:
        return []
    parsed = json.loads(out)
    if isinstance(parsed, list):
        return parsed
    # gh --paginate concatenates JSON arrays; if the server returned
    # multiple chunks, fall back to a per-line parse.
    comments: list[dict] = []
    for line in out.splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            chunk = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(chunk, list):
            comments.extend(chunk)
    return comments


def post_or_update_comment(
    repo: str,
    pr_number: int,
    body: str,
    scope_id: str,
    *,
    dry_run: bool = False,
) -> dict:
    """Create or update the sticky comment for `scope_id` on the source PR.

    On dry-run: does NOT call `gh api`; returns a planned-payload dict
    describing the action it would have taken. Real run: lists the PR's
    comments, looks for the per-scope sticky marker, then PATCHes the
    matched comment or POSTs a new one. Returns a small status dict.
    """
    plan: dict[str, Any] = {
        "action": "create",  # or "update"
        "repo": repo,
        "pr_number": pr_number,
        "scope_id": scope_id,
        "body": body,
        "dry_run": bool(dry_run),
    }
    if dry_run:
        return plan

    existing = fetch_pr_comments(repo, pr_number)
    match = find_existing_comment(existing, scope_id)
    if match and isinstance(match.get("id"), int):
        comment_id = int(match["id"])
        plan["action"] = "update"
        plan["comment_id"] = comment_id
        _gh_api(
            [
                "-X",
                "PATCH",
                f"repos/{repo}/issues/comments/{comment_id}",
                "-f",
                f"body={body}",
            ]
        )
    else:
        plan["action"] = "create"
        _gh_api(
            [
                "-X",
                "POST",
                f"repos/{repo}/issues/{pr_number}/comments",
                "-f",
                f"body={body}",
            ]
        )
    return plan


# ---------------------------------------------------------------------------
# CLI plumbing.
# ---------------------------------------------------------------------------


def _read_yaml(path: str) -> Any:
    with open(path, "r", encoding="utf-8") as fh:
        return yaml.safe_load(fh)


def _read_json(path: str) -> Any:
    with open(path, "r", encoding="utf-8") as fh:
        return json.load(fh)


def _resolve_workflow(config: dict, workflow_id: str) -> dict:
    workflows = (config or {}).get("workflows") or []
    for w in workflows:
        if (w or {}).get("id") == workflow_id:
            return w
    raise SystemExit(f"workflow id {workflow_id!r} not found in config")


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Post or update a sticky comment on the source PR for a NOMOS "
            "scoped run."
        )
    )
    parser.add_argument(
        "--config",
        required=True,
        help="Path to .nomos/corpus-workflows.yaml",
    )
    parser.add_argument(
        "--workflow-id",
        required=True,
        help="Scope id (matches workflows[].id in the config)",
    )
    parser.add_argument(
        "--diff-plan",
        required=False,
        default="",
        help="Path to nomos-diff.json (planner output). Optional when disabled.",
    )
    parser.add_argument(
        "--trace-manifest",
        required=False,
        default="",
        help="Path or URL of the trace manifest. Optional.",
    )
    parser.add_argument(
        "--gate-status",
        required=False,
        default="skipped",
        choices=GATE_STATUSES,
    )
    parser.add_argument(
        "--pr-number",
        required=False,
        type=int,
        default=0,
        help="Source PR number. Required when commenting is enabled.",
    )
    parser.add_argument(
        "--repo",
        required=False,
        default="",
        help="owner/name of the source repo. Required when commenting is enabled.",
    )
    parser.add_argument(
        "--output-location",
        required=False,
        default="",
        help="URL or artifact name describing where the durable output landed.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print the planned body and skip all gh api calls.",
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    config = _read_yaml(args.config) or {}
    workflow = _resolve_workflow(config, args.workflow_id)
    notify = workflow.get("notify") or {}

    if comment_disabled(notify):
        print(f"comment disabled, no action (workflow_id={args.workflow_id})")
        return 0

    block = notify["source_pr_comment"]
    mode = block.get("mode") or "summary"
    include = list(block.get("include") or list(INCLUDE_KEYS))

    diff_plan = _read_json(args.diff_plan) if args.diff_plan else {}
    trace_manifest_ref: Any = args.trace_manifest or ""
    if args.trace_manifest and os.path.isfile(args.trace_manifest):
        try:
            trace_manifest_ref = _read_yaml(args.trace_manifest)
        except yaml.YAMLError:
            trace_manifest_ref = args.trace_manifest

    body = format_comment(
        mode=mode,
        include=include,
        diff_plan=diff_plan,
        trace_manifest_ref=trace_manifest_ref,
        gate_status=args.gate_status,
        output_location=args.output_location or None,
        scope_id=args.workflow_id,
    )

    if args.dry_run:
        print("--- dry-run: planned comment body ---")
        print(body)
        return 0

    if not args.repo or not args.pr_number:
        raise SystemExit(
            "--repo and --pr-number are required when commenting is enabled "
            "and --dry-run is not set"
        )

    plan = post_or_update_comment(
        repo=args.repo,
        pr_number=int(args.pr_number),
        body=body,
        scope_id=args.workflow_id,
        dry_run=False,
    )
    print(f"action={plan.get('action')} scope_id={args.workflow_id}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""Guard the independent roadmap lanes declared in docs/roadmap-lanes.yaml.

This is a dispatch guard, not a claim or compliance validator. It prevents the
repository from recreating the false blockers removed by ADR-VRC-0004:

* only autonomous items enter each lane's dispatch queue;
* hard dependencies target autonomous work in the same lane;
* passive, human and external facts are inputs/claim gates, never task blockers;
* tooling intended for regulated use declares intended use, impact, validation
  state and bounded reliance.
"""

from __future__ import annotations

import argparse
import json
import subprocess
from pathlib import Path
from typing import Any

import yaml


DEFAULT_REGISTRY = Path("docs/roadmap-lanes.yaml")
LANES = {"product", "devops", "regulated"}
DISPATCH = {"autonomous", "passive", "human", "external"}
UMBRELLA_ROLES = {"epic", "parent"}
ITEM_STATES = {"open", "closed"}
DELIVERY_STATES = {"planned", "partial", "implemented", "verified", "blocked", "split"}
EVIDENCE_STATES = {"none", "accumulating", "requires_human", "requires_external", "present"}
CLAIM_STATES = {"bounded", "locked", "unlocked", "prohibited"}
REGULATED_TOOL_FIELDS = {"intended_use", "impact", "validation_state", "reliance"}
TOOL_IMPACTS = {"support", "evidence", "decision", "critical_decision"}
TOOL_VALIDATION_STATES = {
    "planned",
    "development",
    "technically_verified",
    "validated_for_intended_use",
}
TOOL_RELIANCE = {
    "manual_review",
    "manual_verification_required",
    "supporting_use_until_validated",
    "sole_reliance_validated",
}


def validate(registry: dict[str, Any]) -> list[str]:
    failures: list[str] = []
    items = registry.get("items")
    if not isinstance(items, list) or not items:
        return ["items: at least one roadmap item is required"]

    by_issue: dict[int, dict[str, Any]] = {}
    for index, item in enumerate(items):
        if not isinstance(item, dict) or not isinstance(item.get("issue"), int):
            failures.append(f"items[{index}]: integer issue is required")
            continue
        issue = item["issue"]
        if issue in by_issue:
            failures.append(f"issue #{issue}: declared twice")
        by_issue[issue] = item
        if item.get("lane") not in LANES:
            failures.append(f"issue #{issue}: unknown lane {item.get('lane')!r}")
        if item.get("dispatch") not in DISPATCH:
            failures.append(f"issue #{issue}: unknown dispatch state {item.get('dispatch')!r}")
        for field, allowed in (
            ("state", ITEM_STATES),
            ("delivery_state", DELIVERY_STATES),
            ("evidence_state", EVIDENCE_STATES),
            ("claim_state", CLAIM_STATES),
        ):
            if item.get(field) not in allowed:
                failures.append(f"issue #{issue}: unknown {field} {item.get(field)!r}")
        if not isinstance(item.get("depends_on"), list):
            failures.append(f"issue #{issue}: depends_on must be an explicit list")
        if "inputs" in item and not isinstance(item.get("inputs"), list):
            failures.append(f"issue #{issue}: inputs must be a list")
        tool = item.get("regulated_tool")
        if tool is not None:
            if not isinstance(tool, dict):
                failures.append(f"issue #{issue}: regulated_tool must be a mapping")
            else:
                missing = sorted(REGULATED_TOOL_FIELDS - set(tool))
                if missing:
                    failures.append(
                        f"issue #{issue}: regulated_tool misses {', '.join(missing)}"
                    )
                for field in REGULATED_TOOL_FIELDS:
                    if field in tool and not str(tool[field]).strip():
                        failures.append(f"issue #{issue}: regulated_tool.{field} is empty")
                if tool.get("impact") not in TOOL_IMPACTS:
                    failures.append(
                        f"issue #{issue}: unknown regulated_tool impact {tool.get('impact')!r}"
                    )
                if tool.get("validation_state") not in TOOL_VALIDATION_STATES:
                    failures.append(
                        f"issue #{issue}: unknown regulated_tool validation_state "
                        f"{tool.get('validation_state')!r}"
                    )
                if tool.get("reliance") not in TOOL_RELIANCE:
                    failures.append(
                        f"issue #{issue}: unknown regulated_tool reliance {tool.get('reliance')!r}"
                    )
                if (
                    tool.get("reliance") == "sole_reliance_validated"
                    and tool.get("validation_state") != "validated_for_intended_use"
                ):
                    failures.append(
                        f"issue #{issue}: sole reliance requires validated_for_intended_use"
                    )
                if (
                    tool.get("impact") == "critical_decision"
                    and tool.get("validation_state") != "validated_for_intended_use"
                ):
                    failures.append(
                        f"issue #{issue}: critical_decision is prohibited before validated_for_intended_use"
                    )
                if (
                    item.get("claim_state") == "unlocked"
                    and tool.get("validation_state") != "validated_for_intended_use"
                ):
                    failures.append(
                        f"issue #{issue}: an unvalidated regulated tool cannot unlock its claim"
                    )
        if item.get("claim_state") == "unlocked" and item.get("delivery_state") in {
            "planned",
            "blocked",
            "partial",
            "split",
        }:
            failures.append(
                f"issue #{issue}: delivery_state {item.get('delivery_state')} cannot carry an unlocked claim"
            )

    for issue, item in by_issue.items():
        for dependency in item.get("depends_on", []):
            target = by_issue.get(dependency)
            if target is None:
                failures.append(f"issue #{issue}: dependency #{dependency} is not declared")
                continue
            if target.get("dispatch") != "autonomous":
                failures.append(
                    f"issue #{issue}: hard dependency #{dependency} is {target.get('dispatch')}; "
                    "passive/human/external work is a nonblocking input or claim gate"
                )
            if target.get("lane") != item.get("lane"):
                failures.append(
                    f"issue #{issue}: hard dependency #{dependency} crosses "
                    f"{item.get('lane')} -> {target.get('lane')}; use inputs instead"
                )

    # Same-lane autonomous dependencies can still deadlock if they form a
    # cycle. Detect that explicitly instead of leaving every queue item waiting
    # forever while the registry says pass.
    visiting: list[int] = []
    visited: set[int] = set()

    def visit(issue: int) -> None:
        if issue in visited:
            return
        if issue in visiting:
            start = visiting.index(issue)
            cycle = visiting[start:] + [issue]
            failures.append(
                "hard dependency cycle: " + " -> ".join(f"#{node}" for node in cycle)
            )
            return
        visiting.append(issue)
        for dependency in by_issue[issue].get("depends_on", []):
            if dependency in by_issue:
                visit(dependency)
        visiting.pop()
        visited.add(issue)

    for issue in sorted(by_issue):
        visit(issue)

    selection = registry.get("selection_policy")
    selection = selection if isinstance(selection, dict) else {}
    if selection.get("eligible_dispatch") != "autonomous":
        failures.append("selection_policy.eligible_dispatch must be autonomous")
    hard = selection.get("hard_dependencies")
    if not isinstance(hard, dict) or hard.get("same_lane_only") is not True or hard.get("autonomous_only") is not True:
        failures.append(
            "selection_policy.hard_dependencies must require same_lane_only and autonomous_only"
        )
    if selection.get("cross_lane_relationship") != "inputs_are_nonblocking":
        failures.append(
            "selection_policy.cross_lane_relationship must be inputs_are_nonblocking"
        )
    queues = selection.get("dispatch_queues")
    if not isinstance(queues, dict):
        failures.append("selection_policy.dispatch_queues must be a mapping by lane")
        queues = {}
    extra_queues = sorted(set(queues) - LANES)
    if extra_queues:
        failures.append(
            "selection_policy.dispatch_queues has unknown lane(s): "
            + ", ".join(extra_queues)
        )
    ordered: list[int] = []
    for lane in sorted(LANES):
        order = queues.get(lane)
        if not isinstance(order, list):
            failures.append(f"selection_policy.dispatch_queues.{lane} must be a list")
            continue
        if len(order) != len(set(order)):
            failures.append(f"dispatch queue {lane} contains duplicates")
        ordered.extend(order)
        for issue in order:
            item = by_issue.get(issue)
            if item is None:
                failures.append(f"dispatch queue {lane}: issue #{issue} is not declared")
            elif item.get("state") != "open" or item.get("dispatch") != "autonomous":
                failures.append(
                    f"dispatch queue {lane}: issue #{issue} is not an open autonomous item"
                )
            elif item.get("lane") != lane:
                failures.append(
                    f"dispatch queue {lane}: issue #{issue} belongs to {item.get('lane')}"
                )
    if len(ordered) != len(set(ordered)):
        failures.append("an issue appears in more than one dispatch queue")
    for issue, item in by_issue.items():
        for input_issue in item.get("inputs", []):
            if input_issue not in by_issue:
                failures.append(f"issue #{issue}: nonblocking input #{input_issue} is not declared")
    # Umbrella issues: visible, declared, never dispatched, never double-declared.
    umbrellas = registry.get("umbrella_issues") or []
    if not isinstance(umbrellas, list):
        failures.append("umbrella_issues must be a list")
        umbrellas = []
    for index, umbrella in enumerate(umbrellas):
        if not isinstance(umbrella, dict) or not isinstance(umbrella.get("issue"), int):
            failures.append(f"umbrella_issues[{index}]: integer issue is required")
            continue
        issue = umbrella["issue"]
        if issue in by_issue:
            failures.append(f"umbrella issue #{issue} is also declared as a roadmap item")
        if umbrella.get("role") not in UMBRELLA_ROLES:
            failures.append(f"umbrella issue #{issue}: unknown role {umbrella.get('role')!r}")
        if not str(umbrella.get("note", "")).strip():
            failures.append(f"umbrella issue #{issue}: a note is required")
        if issue in ordered:
            failures.append(f"umbrella issue #{issue} appears in a dispatch queue")

    open_autonomous = {
        issue
        for issue, item in by_issue.items()
        if item.get("state") == "open" and item.get("dispatch") == "autonomous"
    }
    missing_from_order = sorted(open_autonomous - set(ordered))
    if missing_from_order:
        failures.append(
            "dispatch queues omit open autonomous issue(s): "
            + ", ".join(f"#{issue}" for issue in missing_from_order)
        )
    return failures


# --- generated queue tables -------------------------------------------------
#
# The queue composition used to be retyped by hand in several documents, each
# with its own wording, and nothing compared them to the registry. The first two
# closures after the registry landed left the delivered items at the head of the
# published queues. So the tables are now GENERATED here, between markers, and
# CI regenerates them and fails on any diff — the same pattern the wiring matrix
# already uses.
QUEUE_DOCS = (
    Path("docs/47-roadmap-lanes-and-risk-based-validation.md"),
    Path("docs/29-post-alpha-release-issue-list.md"),
    Path("docs/15-product-backlog.md"),
)
QUEUE_BEGIN = "<!-- roadmap-queues:begin -->"
QUEUE_END = "<!-- roadmap-queues:end -->"


def render_queue_table(registry: dict[str, Any]) -> str:
    """Render the Product/DevOps queues as one Markdown table, from the registry."""
    by_issue = {
        int(item["issue"]): item
        for item in registry.get("items") or []
        if isinstance(item, dict) and isinstance(item.get("issue"), int)
    }
    queues = (registry.get("selection_policy") or {}).get("dispatch_queues") or {}
    product = [int(i) for i in queues.get("product") or []]
    devops = [int(i) for i in queues.get("devops") or []]

    def cell(issue: int | None) -> str:
        if issue is None:
            return "—"
        item = by_issue.get(issue)
        title = str(item.get("title", "")).strip() if item else "(not declared)"
        return f"#{issue} — {title}"

    rows = []
    for index in range(max(len(product), len(devops), 1)):
        left = product[index] if index < len(product) else None
        right = devops[index] if index < len(devops) else None
        rows.append(f"| {cell(left)} | {cell(right)} |")

    lines = [
        QUEUE_BEGIN,
        "<!-- GENERATED from docs/roadmap-lanes.yaml by scripts/roadmap_lane_guard.py --emit-docs;"
        " do not edit by hand, CI fails on drift -->",
        "| Product queue | DevOps queue |",
        "|---|---|",
        *rows,
        QUEUE_END,
    ]
    return "\n".join(lines)


def emit_docs(root: Path, registry: dict[str, Any]) -> list[str]:
    """Rewrite the marked block of every queue doc. Returns problems, if any."""
    problems: list[str] = []
    table = render_queue_table(registry)
    for rel in QUEUE_DOCS:
        path = root / rel
        if not path.is_file():
            problems.append(f"{rel.as_posix()}: missing")
            continue
        text = path.read_text(encoding="utf-8")
        begin = text.find(QUEUE_BEGIN)
        end = text.find(QUEUE_END)
        if begin < 0 or end < 0 or end < begin:
            problems.append(f"{rel.as_posix()}: no {QUEUE_BEGIN} … {QUEUE_END} block")
            continue
        end += len(QUEUE_END)
        updated = text[:begin] + table + text[end:]
        if updated != text:
            path.write_text(updated, encoding="utf-8")
    return problems


def verify_github(registry: dict[str, Any]) -> list[str]:
    """Compare each item's declared state with GitHub. Network; not for CI.

    The guard above validates the registry's internal consistency and nothing
    else, so it stayed green while two closed issues sat at the head of their
    queues. This is the check that would have noticed.
    """
    problems: list[str] = []
    for item in registry.get("items") or []:
        if not isinstance(item, dict) or not isinstance(item.get("issue"), int):
            continue
        issue = item["issue"]
        try:
            out = subprocess.run(
                ["gh", "issue", "view", str(issue), "--json", "state", "--jq", ".state"],
                capture_output=True,
                text=True,
                check=True,
                timeout=60,
            ).stdout.strip().lower()
        except (OSError, subprocess.SubprocessError) as exc:
            problems.append(f"issue #{issue}: GitHub unreachable ({exc})")
            continue
        declared = str(item.get("state", "")).lower()
        if out != declared:
            problems.append(
                f"issue #{issue}: registry says {declared!r}, GitHub says {out!r}"
            )
    return problems


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=".", help="Repository root")
    parser.add_argument("--registry", default=str(DEFAULT_REGISTRY), help="Roadmap lane registry")
    parser.add_argument(
        "--emit-docs",
        action="store_true",
        help="Regenerate the queue tables in the roadmap docs from the registry.",
    )
    parser.add_argument(
        "--verify-github",
        action="store_true",
        help="Compare declared item states with GitHub (network; not for CI).",
    )
    args = parser.parse_args()
    root = Path(args.root).resolve()
    path = Path(args.registry)
    if not path.is_absolute():
        path = root / path
    try:
        registry = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    except (OSError, yaml.YAMLError) as exc:
        print(json.dumps({"status": "error", "registry": str(path), "failures": [str(exc)]}, indent=2))
        return 2
    failures = validate(registry)
    if args.emit_docs:
        failures.extend(emit_docs(root, registry))
    if args.verify_github:
        failures.extend(verify_github(registry))
    try:
        registry_path = path.resolve().relative_to(root).as_posix()
    except ValueError:
        registry_path = path.as_posix()
    verdict = {
        "status": "fail" if failures else "pass",
        "registry": registry_path,
        "items": len(registry.get("items") or []),
        "autonomous_queues": (registry.get("selection_policy") or {}).get("dispatch_queues", {}),
        "failures": failures,
    }
    print(json.dumps(verdict, indent=2, sort_keys=True))
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())

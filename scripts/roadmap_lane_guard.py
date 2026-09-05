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
from pathlib import Path
from typing import Any

import yaml


DEFAULT_REGISTRY = Path("docs/roadmap-lanes.yaml")
LANES = {"product", "devops", "regulated"}
DISPATCH = {"autonomous", "passive", "human", "external"}
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


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=".", help="Repository root")
    parser.add_argument("--registry", default=str(DEFAULT_REGISTRY), help="Roadmap lane registry")
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

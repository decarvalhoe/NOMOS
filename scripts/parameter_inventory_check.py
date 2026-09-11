#!/usr/bin/env python3
"""NRT-029 (#702) — a parameter inventory (docs/49 §2.2) is checked, not admired.

Reads an inventory written at templates/regulated/parameter-inventory.yaml and
refuses what the template's own semantics forbid:

* a placeholder left from the template (`<...>`) anywhere a value is expected;
* a status outside validated | default | config | obsolete, an evidence kind
  outside measurement | incident | decision | none;
* a `validated` parameter without a dated evidence reference — a value is
  validated by a measurement, an incident or a decision, never by its status;
* a silent-failure entry whose component is not declared, whose detection is
  outside none | log | metric | gate, or whose detection is `none` without a
  finding reference — "nothing says so" is a finding, not a note.

The verdict counts the parameters by status and lists the defaults: those are
the calibration candidates, and the inventory's point is that they are named.

Exit 0 = the inventory holds; 1 = a refusal, named; 2 = usage.
Claim boundary: an inventory proves that values are known and dated. It does
not prove they are optimal.
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from datetime import date
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError as exc:  # pragma: no cover - exercised in CI setup failure
    print("PyYAML is required for the parameter inventory check.", file=sys.stderr)
    raise SystemExit(2) from exc

SCHEMA = "nomos-parameter-inventory-template-v1"
RECORD_TYPE = "parameter_inventory"
STATUSES = ("validated", "default", "config", "obsolete")
EVIDENCE_KINDS = ("measurement", "incident", "decision", "none")
DETECTIONS = ("none", "log", "metric", "gate")
PLACEHOLDER = re.compile(r"<[^>]*>")
DATE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
VERDICT_SCHEMA = "nomos-parameter-inventory-verdict-v1"


class UsageError(RuntimeError):
    pass


def text(value: Any) -> str:
    return str(value).strip() if value is not None else ""


def is_placeholder(value: Any) -> bool:
    return bool(PLACEHOLDER.search(text(value)))


def valid_date(value: Any) -> bool:
    s = text(value)
    if not DATE.match(s):
        return False
    try:
        date.fromisoformat(s)
    except ValueError:
        return False
    return True


def check_inventory(doc: Any) -> tuple[list[str], dict[str, Any]]:
    problems: list[str] = []
    if not isinstance(doc, dict):
        return ["inventory is not a mapping"], {}
    if doc.get("schema_version") != SCHEMA:
        problems.append(f"schema_version must be {SCHEMA!r}, got {doc.get('schema_version')!r}")
    if doc.get("record_type") != RECORD_TYPE:
        problems.append(f"record_type must be {RECORD_TYPE!r}, got {doc.get('record_type')!r}")
    for field in ("system_id", "as_of", "claim_boundary"):
        if not text(doc.get(field)):
            problems.append(f"{field} is required")
        elif is_placeholder(doc.get(field)):
            problems.append(f"{field} still carries the template placeholder {text(doc.get(field))!r}")
    if text(doc.get("as_of")) and not is_placeholder(doc.get("as_of")) and not valid_date(doc.get("as_of")):
        problems.append(f"as_of must be YYYY-MM-DD, got {text(doc.get('as_of'))!r}")
    if len(text(doc.get("claim_boundary"))) < 20:
        problems.append("claim_boundary must say what the inventory does not prove (at least 20 characters)")

    components = doc.get("components")
    declared: set[str] = set()
    by_status: dict[str, int] = {s: 0 for s in STATUSES}
    defaults: list[str] = []
    if not isinstance(components, list) or not components:
        problems.append("components must list at least one component")
        components = []
    for index, component in enumerate(components):
        label = f"components[{index}]"
        if not isinstance(component, dict):
            problems.append(f"{label}: not a mapping")
            continue
        component_id = text(component.get("component_id"))
        if not component_id or is_placeholder(component_id):
            problems.append(f"{label}: component_id is required and must not be a placeholder")
        else:
            if component_id in declared:
                problems.append(f"{label}: component_id {component_id!r} declared twice")
            declared.add(component_id)
            label = component_id
        for field in ("role", "module"):
            if not text(component.get(field)) or is_placeholder(component.get(field)):
                problems.append(f"{label}: {field} is required and must not be a placeholder")
        parameters = component.get("parameters")
        if not isinstance(parameters, list) or not parameters:
            problems.append(f"{label}: parameters must list at least one parameter")
            parameters = []
        for pindex, parameter in enumerate(parameters):
            plabel = f"{label}.parameters[{pindex}]"
            if not isinstance(parameter, dict):
                problems.append(f"{plabel}: not a mapping")
                continue
            name = text(parameter.get("name"))
            if not name or is_placeholder(name):
                problems.append(f"{plabel}: name is required and must not be a placeholder")
            else:
                plabel = f"{label}.{name}"
            if not text(parameter.get("value")) or is_placeholder(parameter.get("value")):
                problems.append(f"{plabel}: value must be the real value, not a placeholder")
            status = text(parameter.get("status"))
            if status not in STATUSES:
                problems.append(f"{plabel}: status must be one of {STATUSES}, got {status!r}")
            else:
                by_status[status] += 1
                if status == "default":
                    defaults.append(plabel)
            evidence = parameter.get("evidence") if isinstance(parameter.get("evidence"), dict) else {}
            kind = text(evidence.get("kind")) or "none"
            if kind not in EVIDENCE_KINDS:
                problems.append(f"{plabel}: evidence.kind must be one of {EVIDENCE_KINDS}, got {kind!r}")
            if status == "validated":
                if kind == "none":
                    problems.append(f"{plabel}: a validated parameter needs evidence (measurement, incident or decision) — a status is not evidence")
                if not text(evidence.get("ref")) or is_placeholder(evidence.get("ref")):
                    problems.append(f"{plabel}: validated without an evidence reference")
                if not valid_date(evidence.get("date")):
                    problems.append(f"{plabel}: validated without a dated evidence (YYYY-MM-DD)")
            if kind != "none" and (not text(evidence.get("ref")) or is_placeholder(evidence.get("ref"))):
                problems.append(f"{plabel}: evidence.kind is {kind!r} but evidence.ref is missing")
            for field in ("impact_if_changed", "owner"):
                if not text(parameter.get(field)) or is_placeholder(parameter.get(field)):
                    problems.append(f"{plabel}: {field} is required and must not be a placeholder")
        for gindex, gotcha in enumerate(component.get("gotchas") or []):
            if not text(gotcha) or is_placeholder(gotcha):
                problems.append(f"{label}.gotchas[{gindex}]: empty or placeholder")

    review = doc.get("silent_failure_review")
    if not isinstance(review, list) or not review:
        problems.append("silent_failure_review must ask the question of at least one component")
        review = []
    for index, entry in enumerate(review):
        label = f"silent_failure_review[{index}]"
        if not isinstance(entry, dict):
            problems.append(f"{label}: not a mapping")
            continue
        component_id = text(entry.get("component_id"))
        if component_id and component_id not in declared:
            problems.append(f"{label}: component {component_id!r} is not declared under components")
        elif not component_id or is_placeholder(component_id):
            problems.append(f"{label}: component_id is required")
        if not text(entry.get("disabled_silently_when")) or is_placeholder(entry.get("disabled_silently_when")):
            problems.append(f"{label}: disabled_silently_when is required")
        detection = text(entry.get("detection"))
        if detection not in DETECTIONS:
            problems.append(f"{label}: detection must be one of {DETECTIONS}, got {detection!r}")
        if detection == "none" and (not text(entry.get("finding_ref")) or is_placeholder(entry.get("finding_ref"))):
            problems.append(f"{label}: detection is none and no finding is referenced — \"nothing says so\" is a finding")
    reviewed = {text(e.get("component_id")) for e in review if isinstance(e, dict)}
    for component_id in sorted(declared - reviewed):
        problems.append(f"{component_id}: no silent_failure_review entry — every component is asked \"if this stops acting, what says so?\"")
    summary = {
        "components": len(declared),
        "parameters_by_status": by_status,
        "defaults": defaults,
        "silent_failure_entries": len(review),
    }
    return problems, summary


def main() -> int:
    parser = argparse.ArgumentParser(description="Check a parameter inventory (docs/49 §2.2) against the template's semantics.")
    parser.add_argument("--inventory", required=True, help="inventory YAML written at templates/regulated/parameter-inventory.yaml")
    parser.add_argument("--report", default=None, help="write the JSON verdict here as well as stdout")
    args = parser.parse_args()
    path = Path(args.inventory)
    try:
        doc = yaml.safe_load(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeDecodeError, yaml.YAMLError) as exc:
        print(f"parameter inventory check: {path}: {exc}", file=sys.stderr)
        return 2
    problems, summary = check_inventory(doc)
    verdict = {
        "schema_version": VERDICT_SCHEMA,
        "status": "fail" if problems else "pass",
        "inventory": str(path),
        "summary": summary,
        "problems": problems,
        "claim_boundary": "Values are known, dated and honestly labelled; nothing here says they are optimal.",
    }
    encoded = json.dumps(verdict, indent=2, sort_keys=True, ensure_ascii=False)
    if args.report:
        Path(args.report).write_text(encoded + "\n", encoding="utf-8")
    print(encoded)
    if problems:
        for problem in problems:
            print(f"parameter inventory check: {problem}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
"""VRC-16 (#562) — training and competence status, computed rather than declared.

The gap (E3): a matrix of roles by competences existed, and zero signed
attestations existed behind it. The quality manual's effectiveness condition on
training records therefore could not be lifted, and nothing in CI said so.

This script computes, per assigned role, whether competence is `established` or
still `requires_evidence`, from signed attestation records and from nothing
else. Absence of a record is never a pass: a role with no attestation is
`requires_evidence`, and a role whose required training has never been defined
is `requires_definition` — an open gap, not a silent success.

What it will not do
-------------------
It does not create attestations. A competence attestation states that a named
human was assessed and found competent; only the people involved can make that
statement. This script reads what they signed and refuses everything else. If it
ever reports `established` for a role, that is because signed records exist in
the tree, not because a tool decided it.

The independence rule
---------------------
``competence-assessment-template.yaml`` requires the assessor to differ from the
assessee. NOMOS is operated by one person holding every role, so that rule is
structurally unsatisfiable today. The gate does not quietly ignore it: a
self-assessed record is refused UNLESS a dated waiver in
``independence-waiver.yaml`` names that record and records compensating
controls, mirroring how the role assignment record handles the vacant
independent reviewer. An unrecorded self-assessment is a forged independence
claim and turns the gate red.

Checks
------
* the three role vocabularies reconcile through ``role-crosswalk.yaml``, and
  every assigned role is dispositioned there;
* every attestation is well-formed: known competence id, known assessee, dated,
  ``result: pass``, signed by BOTH parties, approved, and not expired;
* independence holds, or an explicit dated waiver covers the record;
* the computed per-role status matches what the training matrix, the SOP table
  and the control matrix publish — drift in either direction is a failure.

Run:

    python3 scripts/training_competence_gate.py --root .
    python3 scripts/training_competence_gate.py --root . --as-of 2026-12-31

Exit codes: 0 clean, 1 a check failed, 2 nothing could be evaluated.
"""

from __future__ import annotations

import argparse
import json
import sys
from datetime import date, datetime, timezone
from pathlib import Path
from typing import Any

import yaml

TRAINING_DIR = Path("docs/regulated/operations/training-records")
DEFAULT_MATRIX = TRAINING_DIR / "training-matrix.yaml"
DEFAULT_CROSSWALK = TRAINING_DIR / "role-crosswalk.yaml"
DEFAULT_ATTESTATIONS = TRAINING_DIR / "attestations"
DEFAULT_WAIVER = TRAINING_DIR / "independence-waiver.yaml"
DEFAULT_ASSIGNMENTS = Path("docs/regulated/operations/records/2026-06-11-role-assignment-record.yaml")
DEFAULT_SOP = Path("docs/regulated/quality-system/training-and-competence-sop.md")
DEFAULT_CONTROL_MATRIX = Path("docs/regulated/control-matrix/nomos-control-matrix.yaml")

CROSSWALK_SCHEMA_VERSION = "nomos-role-crosswalk-v1"
TRAINING_CONTROL_ID = "CTL-QS-004"

STATUS_ESTABLISHED = "established"
STATUS_REQUIRES_EVIDENCE = "requires_evidence"
STATUS_REQUIRES_DEFINITION = "requires_definition"
STATUS_VACANT = "vacant"

CLAIM_BOUNDARY = (
    "Computed status of training records held in this repository. It says whether "
    "signed attestations exist and cover the required competences; it is not an "
    "opinion on whether any person is in fact competent."
)


class EvaluationError(RuntimeError):
    """Raised when the inputs do not allow any evaluation at all."""


def load_yaml(path: Path) -> Any:
    if not path.is_file():
        raise EvaluationError(f"missing file: {path}")
    return yaml.safe_load(path.read_text(encoding="utf-8"))


def resolve(root: Path, value: str | Path) -> Path:
    candidate = Path(value)
    return candidate if candidate.is_absolute() else root / candidate


def check(name: str, problems: list[str], detail: dict[str, Any] | None = None) -> dict[str, Any]:
    record: dict[str, Any] = {
        "check": name,
        "status": "pass" if not problems else "fail",
        "problems": problems,
    }
    if detail:
        record["detail"] = detail
    return record


def parse_date(value: Any, label: str, problems: list[str]) -> date | None:
    if not value:
        problems.append(f"{label}: missing date")
        return None
    try:
        return datetime.strptime(str(value).strip()[:10], "%Y-%m-%d").date()
    except ValueError:
        problems.append(f"{label}: unparseable date {value!r} (expected YYYY-MM-DD)")
        return None


# ---------------------------------------------------------------------------
# inputs
# ---------------------------------------------------------------------------


def assigned_humans(assignments_doc: dict[str, Any]) -> dict[str, str]:
    """Return ``{role: assignee}`` for roles actually held by someone."""
    held: dict[str, str] = {}
    for entry in assignments_doc.get("assignments", []) or []:
        if not isinstance(entry, dict):
            continue
        assignee = str(entry.get("assignee", "")).strip()
        role = str(entry.get("role", "")).strip()
        if not role:
            continue
        if not assignee or assignee.lower() == "vacant":
            continue
        held[role] = assignee
    return held


def load_attestations(directory: Path) -> list[tuple[Path, dict[str, Any]]]:
    """Every attestation in the directory, sorted by path for determinism.

    An empty directory is a legitimate state — it is the state this repository
    is in — and yields no records rather than an error.
    """
    if not directory.is_dir():
        return []
    records: list[tuple[Path, dict[str, Any]]] = []
    for path in sorted(directory.glob("*.yaml")):
        if path.name.lower().startswith("readme"):
            continue
        data = yaml.safe_load(path.read_text(encoding="utf-8"))
        if isinstance(data, dict):
            records.append((path, data))
    return records


def waived_records(waiver_doc: Any) -> dict[str, dict[str, Any]]:
    """``{record_id: waiver_entry}`` for independence waivers actually recorded."""
    if not isinstance(waiver_doc, dict):
        return {}
    waived: dict[str, dict[str, Any]] = {}
    for entry in waiver_doc.get("waived_records", []) or []:
        if isinstance(entry, dict) and entry.get("record_id"):
            waived[str(entry["record_id"])] = entry
    return waived


# ---------------------------------------------------------------------------
# checks
# ---------------------------------------------------------------------------


def check_crosswalk(
    crosswalk: Any, matrix: dict[str, Any], held: dict[str, str]
) -> tuple[dict[str, Any], dict[str, dict[str, Any]]]:
    """Every held role must be dispositioned, and every mapping must resolve."""
    problems: list[str] = []

    if not isinstance(crosswalk, dict) or crosswalk.get("schema_version") != CROSSWALK_SCHEMA_VERSION:
        return (
            check(
                "role_crosswalk",
                [
                    "role crosswalk missing or wrong schema_version "
                    f"(expected {CROSSWALK_SCHEMA_VERSION!r})"
                ],
            ),
            {},
        )

    matrix_roles = {
        str(role.get("role_id")): role
        for role in matrix.get("roles", []) or []
        if isinstance(role, dict)
    }

    entries: dict[str, dict[str, Any]] = {}
    for entry in crosswalk.get("assigned_roles", []) or []:
        if not isinstance(entry, dict):
            continue
        role = str(entry.get("assigned_role", "")).strip()
        if not role:
            problems.append("crosswalk entry without assigned_role")
            continue
        if role in entries:
            problems.append(f"{role}: duplicated in the crosswalk")
        entries[role] = entry

        disposition = entry.get("disposition")
        if disposition not in {"mapped", "requires_definition", "vacant"}:
            problems.append(f"{role}: unknown disposition {disposition!r}")
            continue
        if disposition == "mapped":
            mapped = entry.get("matrix_role_id")
            if not mapped:
                problems.append(f"{role}: disposition 'mapped' without matrix_role_id")
            elif str(mapped) not in matrix_roles:
                problems.append(
                    f"{role}: maps to matrix role {mapped!r}, which the training matrix "
                    "does not define"
                )

    for role in sorted(held):
        if role not in entries:
            problems.append(
                f"{role}: held by {held[role]} but absent from the role crosswalk — a role "
                "cannot be handed to a human without deciding what it requires"
            )

    return (
        check("role_crosswalk", problems, {"assigned_roles": len(entries), "held_roles": len(held)}),
        entries,
    )


def check_attestations(
    records: list[tuple[Path, dict[str, Any]]],
    matrix: dict[str, Any],
    held: dict[str, str],
    waived: dict[str, dict[str, Any]],
    as_of: date,
) -> tuple[dict[str, Any], list[dict[str, Any]]]:
    """Validate each attestation; return the ones that count as valid evidence."""
    problems: list[str] = []
    valid: list[dict[str, Any]] = []

    known_competences: dict[str, str] = {}
    for role in matrix.get("roles", []) or []:
        if not isinstance(role, dict):
            continue
        for competence in role.get("required_competences", []) or []:
            if isinstance(competence, dict) and competence.get("id"):
                known_competences[str(competence["id"])] = str(role.get("role_id"))

    assignees = {value for value in held.values()}
    seen_ids: set[str] = set()

    for path, doc in records:
        label = path.name
        record_id = str(doc.get("record_id", "")).strip()
        if not record_id:
            problems.append(f"{label}: no record_id")
        elif record_id in seen_ids:
            problems.append(f"{label}: duplicate record_id {record_id!r}")
        else:
            seen_ids.add(record_id)

        record_problems: list[str] = []

        assessee = doc.get("assessee") or {}
        assessor = doc.get("assessor") or {}
        assessee_name = str(assessee.get("name", "")).strip()
        assessor_name = str(assessor.get("name", "")).strip()
        if not assessee_name:
            record_problems.append("assessee has no name")
        elif assessee_name not in assignees:
            record_problems.append(
                f"assessee {assessee_name!r} holds no assigned role in the role assignment record"
            )
        if not assessor_name:
            record_problems.append("assessor has no name")

        competence_id = str((doc.get("competence") or {}).get("id", "")).strip()
        if not competence_id:
            record_problems.append("no competence id")
        elif competence_id not in known_competences:
            record_problems.append(
                f"competence {competence_id!r} is not defined in the training matrix"
            )

        assessment = doc.get("assessment") or {}
        assessment_date = parse_date(assessment.get("date"), f"{label} assessment", record_problems)
        if str(assessment.get("result", "")).strip().lower() != "pass":
            record_problems.append(
                f"result is {assessment.get('result')!r}; only 'pass' is evidence of competence"
            )

        decision = doc.get("decision") or {}
        if decision.get("competent") is not True:
            record_problems.append("decision.competent is not true")
        if not decision.get("signed_by_assessor"):
            record_problems.append("not signed by the assessor")
        if not decision.get("signed_by_assessee"):
            record_problems.append("not signed by the assessee")
        if not str(decision.get("signed_at", "")).strip():
            record_problems.append("no signature timestamp")

        approval = doc.get("approval") or {}
        if not str(approval.get("approved_by", "")).strip():
            record_problems.append("no approver recorded")
        if not str(approval.get("approved_at", "")).strip():
            record_problems.append("no approval timestamp")

        validity = doc.get("validity") or {}
        expires_raw = validity.get("expires_at")
        if expires_raw:
            expires = parse_date(expires_raw, f"{label} validity", record_problems)
            if expires is not None and expires < as_of:
                record_problems.append(
                    f"expired on {expires.isoformat()} (evaluated as of {as_of.isoformat()})"
                )

        # Independence: refused unless an explicit dated waiver names this record.
        if assessee_name and assessor_name and assessee_name == assessor_name:
            waiver = waived.get(record_id)
            if waiver is None:
                record_problems.append(
                    f"self-assessed ({assessee_name}) with no recorded independence waiver; "
                    "the assessment template requires assessor != assessee"
                )
            else:
                waiver_problems: list[str] = []
                parse_date(waiver.get("waived_on"), f"{label} waiver", waiver_problems)
                if not str(waiver.get("approved_by", "")).strip():
                    waiver_problems.append("waiver has no approver")
                if not str(waiver.get("compensating_controls", "")).strip():
                    waiver_problems.append("waiver records no compensating controls")
                record_problems.extend(f"independence waiver: {p}" for p in waiver_problems)

        if record_problems:
            problems.extend(f"{label}: {p}" for p in record_problems)
            continue

        valid.append(
            {
                "record_id": record_id,
                "path": path.name,
                "assessee": assessee_name,
                "competence_id": competence_id,
                "matrix_role_id": known_competences[competence_id],
                "assessed_on": assessment_date.isoformat() if assessment_date else None,
                "self_assessed": assessee_name == assessor_name,
            }
        )

    return (
        check(
            "attestation_records",
            problems,
            {"records": len(records), "valid": len(valid)},
        ),
        valid,
    )


def compute_status(
    crosswalk_entries: dict[str, dict[str, Any]],
    matrix: dict[str, Any],
    held: dict[str, str],
    valid: list[dict[str, Any]],
) -> dict[str, dict[str, Any]]:
    """Per assigned role: `established` only when every competence is covered."""
    matrix_roles = {
        str(role.get("role_id")): role
        for role in matrix.get("roles", []) or []
        if isinstance(role, dict)
    }

    covered: dict[tuple[str, str], list[str]] = {}
    for record in valid:
        covered.setdefault((record["assessee"], record["competence_id"]), []).append(
            record["record_id"]
        )

    computed: dict[str, dict[str, Any]] = {}
    for role, assignee in sorted(held.items()):
        entry = crosswalk_entries.get(role, {})
        disposition = entry.get("disposition")

        if disposition == "vacant":
            computed[role] = {"status": STATUS_VACANT, "assignee": assignee, "missing": []}
            continue
        if disposition == "requires_definition" or not entry:
            computed[role] = {
                "status": STATUS_REQUIRES_DEFINITION,
                "assignee": assignee,
                "missing": [],
                "reason": "no required training is defined for this role",
            }
            continue

        matrix_role = matrix_roles.get(str(entry.get("matrix_role_id")), {})
        required = [
            str(c.get("id"))
            for c in matrix_role.get("required_competences", []) or []
            if isinstance(c, dict) and c.get("id")
        ]
        if not required:
            computed[role] = {
                "status": STATUS_REQUIRES_DEFINITION,
                "assignee": assignee,
                "missing": [],
                "reason": "mapped matrix role defines no required competence",
            }
            continue

        missing = [cid for cid in required if (assignee, cid) not in covered]
        computed[role] = {
            "status": STATUS_ESTABLISHED if not missing else STATUS_REQUIRES_EVIDENCE,
            "assignee": assignee,
            "required": len(required),
            "covered": len(required) - len(missing),
            "missing": missing,
        }

    return computed


def check_published_status(
    root: Path,
    computed: dict[str, dict[str, Any]],
    matrix: dict[str, Any],
    matrix_path: Path,
    crosswalk_entries: dict[str, dict[str, Any]],
) -> dict[str, Any]:
    """The documents must publish the computed status, in either direction."""
    problems: list[str] = []

    matrix_roles = {
        str(role.get("role_id")): role
        for role in matrix.get("roles", []) or []
        if isinstance(role, dict)
    }
    for role, result in sorted(computed.items()):
        entry = crosswalk_entries.get(role, {})
        if entry.get("disposition") != "mapped":
            continue
        matrix_role = matrix_roles.get(str(entry.get("matrix_role_id")), {})
        published = str(matrix_role.get("status", ""))
        if published != result["status"]:
            problems.append(
                f"{matrix_path.name}: role {entry.get('matrix_role_id')!r} publishes status "
                f"{published!r}, computed {result['status']!r}"
            )

    # The SOP publishes its own per-role status column. Same rule: it must say
    # what the records say.
    sop_path = resolve(root, DEFAULT_SOP)
    if sop_path.is_file():
        sop_text = sop_path.read_text(encoding="utf-8")
        sop_rows: dict[str, str] = {}
        for line in sop_text.splitlines():
            if not line.startswith("|"):
                continue
            cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
            if len(cells) == 3 and cells[0] not in {"Role", "---"} and not cells[0].startswith("-"):
                sop_rows[cells[0]] = cells[2]
        for role, result in sorted(computed.items()):
            entry = crosswalk_entries.get(role, {})
            sop_role = entry.get("sop_role")
            if entry.get("disposition") != "mapped" or not sop_role:
                continue
            published = sop_rows.get(str(sop_role))
            if published is None:
                problems.append(
                    f"{sop_path.name}: no status row for {sop_role!r}, mapped from {role!r}"
                )
            elif published != result["status"]:
                problems.append(
                    f"{sop_path.name}: {sop_role!r} publishes {published!r}, "
                    f"computed {result['status']!r}"
                )
    else:
        problems.append(f"training SOP not found: {DEFAULT_SOP}")

    # The training control may only be qualified when every held, mapped role is
    # established. Anything else keeps it not_qualified.
    control_path = resolve(root, DEFAULT_CONTROL_MATRIX)
    if control_path.is_file():
        controls = yaml.safe_load(control_path.read_text(encoding="utf-8")) or {}
        control = next(
            (
                c
                for c in controls.get("controls", []) or []
                if isinstance(c, dict) and c.get("control_id") == TRAINING_CONTROL_ID
            ),
            None,
        )
        if control is None:
            problems.append(f"control matrix has no {TRAINING_CONTROL_ID}")
        else:
            mapped = [
                result
                for role, result in computed.items()
                if crosswalk_entries.get(role, {}).get("disposition") == "mapped"
            ]
            all_established = bool(mapped) and all(
                result["status"] == STATUS_ESTABLISHED for result in mapped
            )
            published = str(control.get("current_status", ""))
            expected = "qualified" if all_established else "not_qualified"
            if published != expected:
                problems.append(
                    f"{TRAINING_CONTROL_ID}: current_status is {published!r}, expected "
                    f"{expected!r} for the computed training status"
                )
    else:
        problems.append(f"control matrix not found: {DEFAULT_CONTROL_MATRIX}")

    return check("published_status", problems, {"roles": len(computed)})


# ---------------------------------------------------------------------------
# entry point
# ---------------------------------------------------------------------------


def evaluate(root: Path, as_of: date) -> tuple[list[dict[str, Any]], dict[str, Any]]:
    matrix = load_yaml(resolve(root, DEFAULT_MATRIX))
    if not isinstance(matrix, dict):
        raise EvaluationError("training matrix is not a YAML mapping")
    assignments = load_yaml(resolve(root, DEFAULT_ASSIGNMENTS))
    if not isinstance(assignments, dict):
        raise EvaluationError("role assignment record is not a YAML mapping")

    crosswalk_path = resolve(root, DEFAULT_CROSSWALK)
    crosswalk = load_yaml(crosswalk_path) if crosswalk_path.is_file() else None

    waiver_path = resolve(root, DEFAULT_WAIVER)
    waived = waived_records(load_yaml(waiver_path)) if waiver_path.is_file() else {}

    held = assigned_humans(assignments)
    records = load_attestations(resolve(root, DEFAULT_ATTESTATIONS))

    checks: list[dict[str, Any]] = []
    crosswalk_check, entries = check_crosswalk(crosswalk, matrix, held)
    checks.append(crosswalk_check)

    attestation_check, valid = check_attestations(records, matrix, held, waived, as_of)
    checks.append(attestation_check)

    computed = compute_status(entries, matrix, held, valid)
    checks.append(check_published_status(root, computed, matrix, resolve(root, DEFAULT_MATRIX), entries))

    summary = {
        "as_of": as_of.isoformat(),
        "claim_boundary": CLAIM_BOUNDARY,
        "held_roles": len(held),
        "attestations_found": len(records),
        "attestations_valid": len(valid),
        "roles": computed,
        "established_roles": sorted(
            role for role, result in computed.items() if result["status"] == STATUS_ESTABLISHED
        ),
    }
    return checks, summary


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Compute the training and competence status from signed records."
    )
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument(
        "--as-of",
        default=None,
        help="Evaluate expiry as of this date (YYYY-MM-DD); default: today (UTC).",
    )
    parser.add_argument("--json", action="store_true", help="Print the summary as JSON.")
    parser.add_argument("--quiet", action="store_true", help="Print the summary line only.")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    if args.as_of:
        try:
            as_of = datetime.strptime(args.as_of, "%Y-%m-%d").date()
        except ValueError:
            print(f"training-competence: NOT EVALUATED — bad --as-of {args.as_of!r}", file=sys.stderr)
            return 2
    else:
        as_of = datetime.now(timezone.utc).date()

    try:
        checks, summary = evaluate(root, as_of)
    except (EvaluationError, yaml.YAMLError) as exc:
        print(f"training-competence: NOT EVALUATED — {exc}", file=sys.stderr)
        return 2

    if args.json:
        print(json.dumps(summary, indent=2, sort_keys=True))
    elif not args.quiet:
        for entry in checks:
            print(f"  {entry['status']:<4} {entry['check']}")
            for problem in entry["problems"]:
                print(f"       - {problem}", file=sys.stderr)
        for role, result in sorted(summary["roles"].items()):
            missing = result.get("missing") or []
            detail = f" ({len(missing)} competence(s) missing)" if missing else ""
            print(f"  {result['status']:<20} {role}{detail}")

    failed = [entry for entry in checks if entry["status"] == "fail"]
    if failed:
        print(
            f"training-competence: FAIL — {len(failed)} check(s) failed",
            file=sys.stderr,
        )
        return 1

    established = len(summary["established_roles"])
    print(
        f"training-competence: OK — {summary['attestations_valid']} valid attestation(s), "
        f"{established}/{summary['held_roles']} held role(s) established "
        f"(as of {summary['as_of']})"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

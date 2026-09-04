#!/usr/bin/env python3
"""VRC-14 (#560) — repeated CI evidence on the private corpus.

The gap this closes (E3/E6): NOMOS had ONE recorded run on the private RBOK
corpus, and "single recorded run" is not "repeated CI evidence". The public
claim boundary names repeated CI evidence on private corpora as part of the
remaining, release-scoped proof chain.

This script does not assert that the chain exists. It MEASURES it, from the
GitHub Actions run history of the scheduled workflow declared in
``policy.yaml``, and publishes a dated index of the runs it counted. The
measurement is what gates the claim: while the target is unmet, the claim stays
locked, and the gate fails if any document in the tree says otherwise.

What counts as one unit of evidence (``policy.yaml`` is the versioned source):

* the run was triggered by the schedule on the default branch, and completed;
* its conclusion is ``success`` — anything else, including ``cancelled`` and a
  missing conclusion, is not green;
* it archived at least one artifact, and that artifact has NOT expired. A green
  run whose pack is gone leaves nothing to re-inspect, so it is not archived
  evidence. Retention is finite, which means the chain decays on its own unless
  runs keep coming. That is deliberate.

Two numbers are published side by side, because they answer different questions
and the weaker one alone would flatter the result:

* ``consecutive_green_runs`` — consecutive scheduled OCCURRENCES. A missed
  occurrence (a gap wider than the cadence tolerance) breaks the chain: runs
  that happened cannot vouch for weeks that produced nothing. This is the
  number the target is measured against.
* ``consecutive_green_runs_ignoring_cadence`` — the same walk with the cadence
  rule switched off. It is always at least as large, and it is published so the
  cost of the cadence rule is visible rather than hidden.

Modes:

    python3 scripts/repeated_ci_evidence.py --root .            # verify (offline, what CI runs)
    python3 scripts/repeated_ci_evidence.py --root . --collect  # re-measure live (needs `gh`)
    python3 scripts/repeated_ci_evidence.py --root . --collect --publish

Verify is offline and deterministic: it recomputes the measurement from the
recorded runs and fails on any divergence from the published one, on a policy
whose bytes no longer match the digest the index was built against, on a
workflow that lost its schedule or lowered its retention, and on any document
that asserts the claim while the measurement says it is locked.

Exit codes: 0 clean, 1 a check failed, 2 nothing could be measured.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import shutil
import subprocess
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any

import yaml

SCRIPT_ROOT = Path(__file__).resolve().parents[1]
EVIDENCE_DIR = Path("docs/regulated/evidence-index/repeated-ci-evidence")
DEFAULT_POLICY = EVIDENCE_DIR / "policy.yaml"
DEFAULT_LEDGER = Path("docs/regulated/evidence-index/evidence-ledger.yaml")
DEFAULT_CLAIM_BOUNDARY = Path("docs/public-claim-boundary.md")
INDEX_GLOB = "index-*.json"
INDEX_SCHEMA_VERSION = "nomos-repeated-ci-evidence-index-v1"
POLICY_SCHEMA_VERSION = "nomos-repeated-ci-evidence-policy-v1"
TIMESTAMP_FORMAT = "%Y-%m-%dT%H:%M:%SZ"
DEFAULT_REPO = "decarvalhoe/NOMOS"
API_PAGE_SIZE = 100
API_MAX_PAGES = 5
GH_TIMEOUT_SECONDS = 60.0

CLAIM_BOUNDARY = (
    "Measurement of the scheduled-run history of one workflow over one private "
    "corpus. It is evidence that the pipeline kept running green on that corpus, "
    "not a statement about coverage of other corpora, other formats, or the "
    "business correctness of any artifact."
)


class MeasurementError(RuntimeError):
    """Raised when nothing can be measured, as opposed to measuring a failure."""


# ---------------------------------------------------------------------------
# small helpers
# ---------------------------------------------------------------------------


def sha256_file(path: Path) -> str:
    return "sha256:" + hashlib.sha256(path.read_bytes()).hexdigest()


def resolve(root: Path, value: str | Path) -> Path:
    candidate = Path(value)
    return candidate if candidate.is_absolute() else root / candidate


def rel(path: Path, root: Path) -> str:
    try:
        return path.resolve().relative_to(root.resolve()).as_posix()
    except ValueError:
        return path.as_posix()


def check(name: str, problems: list[str], detail: dict[str, Any] | None = None) -> dict[str, Any]:
    record: dict[str, Any] = {
        "check": name,
        "status": "pass" if not problems else "fail",
        "problems": problems,
    }
    if detail:
        record["detail"] = detail
    return record


def parse_ts(value: str) -> datetime:
    """Parse a GitHub timestamp into an aware UTC datetime."""
    text = value.strip()
    if text.endswith("Z"):
        text = text[:-1] + "+00:00"
    parsed = datetime.fromisoformat(text)
    if parsed.tzinfo is None:
        parsed = parsed.replace(tzinfo=timezone.utc)
    return parsed.astimezone(timezone.utc)


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


def load_yaml(path: Path) -> dict[str, Any]:
    if not path.is_file():
        raise MeasurementError(f"missing file: {path}")
    data = yaml.safe_load(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict):
        raise MeasurementError(f"not a YAML mapping: {path}")
    return data


def load_policy(path: Path) -> dict[str, Any]:
    policy = load_yaml(path)
    if policy.get("schema_version") != POLICY_SCHEMA_VERSION:
        raise MeasurementError(
            f"policy schema_version is {policy.get('schema_version')!r}, "
            f"expected {POLICY_SCHEMA_VERSION!r}"
        )
    for section in ("workflow", "cadence", "countable_run", "target", "retention", "claim"):
        if not isinstance(policy.get(section), dict):
            raise MeasurementError(f"policy section missing or not a mapping: {section}")
    return policy


# ---------------------------------------------------------------------------
# measurement
# ---------------------------------------------------------------------------


def run_is_countable(run: dict[str, Any], policy: dict[str, Any]) -> tuple[bool, str]:
    """Return ``(countable, reason)`` for one recorded run.

    The reason is recorded for every run, green or not, so a reader can see why
    a run did not count without re-deriving the rule.
    """
    rules = policy["countable_run"]

    if run.get("status") != "completed":
        return False, f"not completed (status={run.get('status')!r})"
    if run.get("conclusion") != rules.get("conclusion", "success"):
        return False, f"conclusion is {run.get('conclusion')!r}"

    artifacts = run.get("artifacts")
    if not isinstance(artifacts, list):
        return False, "artifact list is missing"
    if rules.get("require_artifact", True) and not artifacts:
        return False, "green run archived no artifact"
    if rules.get("require_unexpired_artifact", True):
        live = [a for a in artifacts if not a.get("expired", True)]
        if not live:
            return False, "every archived artifact has expired"

    return True, "countable"


def measure(runs: list[dict[str, Any]], policy: dict[str, Any], now: datetime) -> dict[str, Any]:
    """Compute the repeated-evidence measurement from recorded runs.

    ``runs`` is newest-first. The function is pure: it reads no clock and no
    network beyond the ``now`` it is handed, so verify can replay it exactly.
    """
    cadence = policy["cadence"]
    target = policy["target"]
    period_days = int(cadence["period_days"])
    max_gap_days = int(cadence["max_gap_days"])
    target_runs = int(target["consecutive_green_runs"])

    ordered = sorted(runs, key=lambda r: parse_ts(r["created_at"]), reverse=True)

    annotated: list[dict[str, Any]] = []
    for run in ordered:
        countable, reason = run_is_countable(run, policy)
        annotated.append({"run": run, "countable": countable, "reason": reason})

    # Missed occurrences across the whole recorded window, independent of the
    # streak: a reader wants to know the schedule went dark even when the gap
    # sits outside the current chain.
    missed_total = 0
    missed_windows: list[dict[str, Any]] = []
    for newer, older in zip(annotated, annotated[1:]):
        gap = parse_ts(newer["run"]["created_at"]) - parse_ts(older["run"]["created_at"])
        if gap <= timedelta(days=max_gap_days):
            continue
        skipped = max(int(round(gap.days / period_days)) - 1, 1)
        missed_total += skipped
        missed_windows.append(
            {
                "after_run_number": older["run"].get("run_number"),
                "after": older["run"]["created_at"],
                "before_run_number": newer["run"].get("run_number"),
                "before": newer["run"]["created_at"],
                "gap_days": gap.days,
                "missed_occurrences": skipped,
            }
        )

    def walk(apply_cadence: bool) -> tuple[int, str | None]:
        """Walk backwards from the newest run; return ``(streak, break_reason)``."""
        if not annotated:
            return 0, "no scheduled run recorded"
        if not annotated[0]["countable"]:
            return 0, f"newest scheduled run is not countable: {annotated[0]['reason']}"

        streak = 1
        for newer, older in zip(annotated, annotated[1:]):
            if not older["countable"]:
                return streak, (
                    f"run #{older['run'].get('run_number')} is not countable: {older['reason']}"
                )
            if apply_cadence:
                gap = parse_ts(newer["run"]["created_at"]) - parse_ts(older["run"]["created_at"])
                if gap > timedelta(days=max_gap_days):
                    return streak, (
                        f"{gap.days}-day gap before run "
                        f"#{newer['run'].get('run_number')} exceeds the "
                        f"{max_gap_days}-day cadence tolerance"
                    )
            streak += 1
        return streak, "no earlier scheduled run recorded"

    consecutive, break_reason = walk(apply_cadence=True)
    ignoring_cadence, _ = walk(apply_cadence=False)

    # A streak that ended months ago is history, not repeated evidence.
    if annotated:
        age = now - parse_ts(annotated[0]["run"]["created_at"])
        streak_is_current = age <= timedelta(days=max_gap_days)
        newest_age_days = age.days
    else:
        streak_is_current = False
        newest_age_days = None

    corpus_commits = sorted(
        {
            str(run.get("corpus_commit"))
            for run in ordered
            if run.get("corpus_commit")
        }
    )

    unlocked = consecutive >= target_runs and (
        streak_is_current or not target.get("require_current_streak", True)
    )

    return {
        "scheduled_runs_recorded": len(ordered),
        "countable_runs": sum(1 for entry in annotated if entry["countable"]),
        "green_runs_total": sum(
            1 for run in ordered if run.get("conclusion") == "success"
        ),
        "consecutive_green_runs": consecutive,
        "consecutive_green_runs_ignoring_cadence": ignoring_cadence,
        "streak_break_reason": break_reason,
        "streak_is_current": streak_is_current,
        "newest_run_age_days": newest_age_days,
        "missed_scheduled_occurrences": missed_total,
        "missed_windows": missed_windows,
        "distinct_corpus_commits": len(corpus_commits),
        "corpus_commits": corpus_commits,
        "target_consecutive_green_runs": target_runs,
        "runs_remaining_to_target": max(target_runs - consecutive, 0),
        "claim_unlocked": unlocked,
    }


# ---------------------------------------------------------------------------
# live collection
# ---------------------------------------------------------------------------


def gh_api(endpoint: str) -> Any:
    if shutil.which("gh") is None:
        raise MeasurementError("gh CLI not found; live collection needs it")
    result = subprocess.run(
        ["gh", "api", endpoint],
        text=True,
        capture_output=True,
        check=False,
        timeout=GH_TIMEOUT_SECONDS,
    )
    if result.returncode != 0:
        detail = (result.stderr or result.stdout).strip()
        raise MeasurementError(f"gh api {endpoint} failed: {detail}")
    try:
        return json.loads(result.stdout or "{}")
    except json.JSONDecodeError as exc:
        raise MeasurementError(f"gh api {endpoint} returned invalid JSON: {exc}") from exc


def artifact_records(repo: str, run_id: int) -> list[dict[str, Any]]:
    payload = gh_api(f"/repos/{repo}/actions/runs/{run_id}/artifacts?per_page={API_PAGE_SIZE}")
    records = []
    for artifact in payload.get("artifacts", []) or []:
        records.append(
            {
                "artifact_id": artifact.get("id"),
                "name": artifact.get("name"),
                "size_in_bytes": artifact.get("size_in_bytes"),
                "expired": bool(artifact.get("expired", True)),
                "expires_at": artifact.get("expires_at"),
            }
        )
    return records


def corpus_commit_from_artifacts(artifacts: list[dict[str, Any]]) -> str | None:
    """The pipeline names its artifact after the corpus commit it consumed.

    That name is the only place the private corpus revision surfaces without
    downloading the pack, and it is what makes "eight runs" distinguishable
    from "one run repeated eight times".
    """
    prefix = "rbok-lawbook-artifacts-"
    for artifact in artifacts:
        name = str(artifact.get("name") or "")
        if name.startswith(prefix):
            commit = name[len(prefix) :]
            if commit:
                return commit
    return None


def collect_runs(repo: str, policy: dict[str, Any]) -> list[dict[str, Any]]:
    workflow = policy["workflow"]
    workflow_id = workflow["workflow_id"]
    event = workflow["event"]
    branch = workflow["branch"]

    collected: list[dict[str, Any]] = []
    for page in range(1, API_MAX_PAGES + 1):
        endpoint = (
            f"/repos/{repo}/actions/workflows/{workflow_id}/runs"
            f"?event={event}&branch={branch}&per_page={API_PAGE_SIZE}&page={page}"
        )
        payload = gh_api(endpoint)
        batch = payload.get("workflow_runs", []) or []
        if not batch:
            break
        for run in batch:
            run_id = run.get("id")
            artifacts = artifact_records(repo, run_id) if run_id else []
            collected.append(
                {
                    "run_id": run_id,
                    "run_number": run.get("run_number"),
                    "run_attempt": run.get("run_attempt"),
                    "event": run.get("event"),
                    "branch": run.get("head_branch"),
                    "head_sha": run.get("head_sha"),
                    "status": run.get("status"),
                    "conclusion": run.get("conclusion"),
                    "created_at": run.get("created_at"),
                    "updated_at": run.get("updated_at"),
                    "html_url": run.get("html_url"),
                    "corpus_commit": corpus_commit_from_artifacts(artifacts),
                    "artifacts": artifacts,
                }
            )
        if len(batch) < API_PAGE_SIZE:
            break

    if not collected:
        raise MeasurementError(
            f"no {event} run of workflow {workflow_id} on {branch} was returned by the API"
        )
    return sorted(collected, key=lambda r: parse_ts(r["created_at"]), reverse=True)


# ---------------------------------------------------------------------------
# offline checks
# ---------------------------------------------------------------------------


def check_policy_digest(index: dict[str, Any], policy_path: Path) -> dict[str, Any]:
    problems: list[str] = []
    published = index.get("policy_digest")
    actual = sha256_file(policy_path)
    if published != actual:
        problems.append(
            f"policy bytes changed since publication (index {published}, tree {actual}); "
            "re-publish a dated index"
        )
    return check("policy_digest", problems, {"published": published, "tree": actual})


def check_run_records(index: dict[str, Any], policy: dict[str, Any]) -> dict[str, Any]:
    problems: list[str] = []
    runs = index.get("runs")
    workflow = policy["workflow"]

    if not isinstance(runs, list) or not runs:
        return check("run_records", ["index records no run"])

    seen: set[Any] = set()
    previous: datetime | None = None
    for position, run in enumerate(runs):
        label = f"run #{run.get('run_number')} (id {run.get('run_id')})"
        for field in ("run_id", "run_number", "created_at", "status", "conclusion"):
            if run.get(field) in (None, ""):
                problems.append(f"{label}: missing field {field}")
        if run.get("run_id") in seen:
            problems.append(f"{label}: duplicate run id")
        seen.add(run.get("run_id"))
        if run.get("event") != workflow["event"]:
            problems.append(f"{label}: event is {run.get('event')!r}, not {workflow['event']!r}")
        if run.get("branch") != workflow["branch"]:
            problems.append(f"{label}: branch is {run.get('branch')!r}, not {workflow['branch']!r}")
        try:
            created = parse_ts(str(run.get("created_at")))
        except (TypeError, ValueError) as exc:
            problems.append(f"{label}: unparseable created_at ({exc})")
            continue
        if previous is not None and created > previous:
            problems.append(f"{label}: runs are not ordered newest-first at position {position}")
        previous = created

    return check("run_records", problems, {"runs": len(runs)})


def check_replay(index: dict[str, Any], policy: dict[str, Any]) -> dict[str, Any]:
    """Recompute the measurement from the recorded runs and compare it.

    The clock is taken from the index's own ``measured_at_utc`` so the replay is
    deterministic: the published numbers must be exactly what the recorded runs
    produce under the committed policy at the moment they were measured.
    """
    problems: list[str] = []
    published = index.get("measurement")
    if not isinstance(published, dict):
        return check("replay", ["index carries no measurement"])

    try:
        measured_at = parse_ts(str(index.get("measured_at_utc")))
    except (TypeError, ValueError) as exc:
        return check("replay", [f"unparseable measured_at_utc ({exc})"])

    replayed = measure(index.get("runs", []), policy, measured_at)

    for key in sorted(set(published) | set(replayed)):
        if key not in published:
            problems.append(f"{key}: absent from the published measurement")
        elif key not in replayed:
            problems.append(f"{key}: published but not produced by the replay")
        elif published[key] != replayed[key]:
            problems.append(f"{key}: published {published[key]!r}, replayed {replayed[key]!r}")

    return check("replay", problems, {"keys": len(replayed)})


def check_workflow_wiring(root: Path, policy: dict[str, Any]) -> dict[str, Any]:
    """The chain only keeps growing while the workflow keeps its schedule.

    Removing the cron or shortening retention silently stops (or erases) the
    evidence. Both turn this gate red instead of quietly freezing the number.
    """
    problems: list[str] = []
    workflow = policy["workflow"]
    path = resolve(root, workflow["path"])
    if not path.is_file():
        return check("workflow_wiring", [f"workflow not found: {workflow['path']}"])

    text = path.read_text(encoding="utf-8")
    cron = workflow["cron"]
    if cron not in text:
        problems.append(f"workflow no longer declares the policy cron {cron!r}")
    if "schedule:" not in text:
        problems.append("workflow no longer has a schedule trigger")

    required_days = int(policy["retention"]["artifact_retention_days"])
    declared: int | None = None
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.startswith("retention-days:"):
            try:
                declared = int(stripped.split(":", 1)[1].strip())
            except ValueError:
                declared = None
            break
    if declared is None:
        problems.append("workflow declares no artifact retention-days")
    elif declared < required_days:
        problems.append(
            f"workflow retention is {declared} days, below the {required_days} "
            "days the published evidence chain relies on"
        )

    return check(
        "workflow_wiring",
        problems,
        {"cron": cron, "retention_days": declared, "required_retention_days": required_days},
    )


def check_ledger(root: Path, index: dict[str, Any], policy: dict[str, Any]) -> dict[str, Any]:
    """The ledger entry and its blocking gap must agree with the measurement."""
    problems: list[str] = []
    claim = policy["claim"]
    path = resolve(root, DEFAULT_LEDGER)
    try:
        ledger = load_yaml(path)
    except MeasurementError as exc:
        return check("evidence_ledger", [str(exc)])

    unlocked = bool(index["measurement"]["claim_unlocked"])

    entries = {
        str(entry.get("id")): entry
        for entry in ledger.get("evidence_categories", []) or []
        if isinstance(entry, dict)
    }
    entry = entries.get(str(claim["ledger_entry"]))
    if entry is None:
        problems.append(f"evidence ledger has no entry {claim['ledger_entry']}")
    else:
        expected_status = "present_measured" if unlocked else "requires_evidence"
        if entry.get("current_status") != expected_status:
            problems.append(
                f"{claim['ledger_entry']}: current_status is "
                f"{entry.get('current_status')!r}, expected {expected_status!r} "
                f"for claim_unlocked={unlocked}"
            )
        expected_claim = claim["id"] if unlocked else "none"
        if entry.get("claim_allowed") != expected_claim:
            problems.append(
                f"{claim['ledger_entry']}: claim_allowed is "
                f"{entry.get('claim_allowed')!r}, expected {expected_claim!r}"
            )

    gaps = {
        str(gap.get("id")): gap
        for gap in ledger.get("blocking_gaps", []) or []
        if isinstance(gap, dict)
    }
    gap = gaps.get(str(claim["blocking_gap"]))
    if gap is None:
        problems.append(f"evidence ledger has no blocking gap {claim['blocking_gap']}")
    else:
        expected_gap_status = "closed" if unlocked else "open"
        if gap.get("status") != expected_gap_status:
            problems.append(
                f"{claim['blocking_gap']}: status is {gap.get('status')!r}, "
                f"expected {expected_gap_status!r} for claim_unlocked={unlocked}"
            )

    return check("evidence_ledger", problems, {"claim_unlocked": unlocked})


def check_claim_language(root: Path, index: dict[str, Any], policy: dict[str, Any]) -> dict[str, Any]:
    """While the target is unmet, nothing in the tree may assert the claim."""
    problems: list[str] = []
    claim = policy["claim"]
    marker = str(claim["unlocked_marker"])
    unlocked = bool(index["measurement"]["claim_unlocked"])

    path = resolve(root, DEFAULT_CLAIM_BOUNDARY)
    if not path.is_file():
        return check("claim_language", [f"claim boundary not found: {DEFAULT_CLAIM_BOUNDARY}"])
    text = path.read_text(encoding="utf-8")

    present = marker.lower() in text.lower()
    if unlocked and not present:
        problems.append(
            f"measurement unlocks the claim but the claim boundary does not state it "
            f"({marker!r} absent)"
        )
    if not unlocked and present:
        problems.append(
            f"claim boundary asserts {marker!r} while the measurement says the claim "
            "is locked"
        )

    # The published streak must appear in the claim boundary, so the prose and
    # the measurement cannot drift apart unnoticed.
    streak = index["measurement"]["consecutive_green_runs"]
    target = index["measurement"]["target_consecutive_green_runs"]
    expected = f"{streak} consecutive"
    if expected.lower() not in text.lower():
        problems.append(
            f"claim boundary does not carry the measured streak ({expected!r}); "
            f"published measurement is {streak}/{target}"
        )

    return check("claim_language", problems, {"marker_present": present, "unlocked": unlocked})


# ---------------------------------------------------------------------------
# envelope and publication
# ---------------------------------------------------------------------------


def build_index(
    repo: str,
    runs: list[dict[str, Any]],
    policy: dict[str, Any],
    policy_path: Path,
    measured_at: datetime,
    published_on: str,
) -> dict[str, Any]:
    return {
        "schema_version": INDEX_SCHEMA_VERSION,
        "issue": policy["issue"],
        "published_on": published_on,
        "measured_at_utc": measured_at.strftime(TIMESTAMP_FORMAT),
        "repository": repo,
        "policy": rel(policy_path, SCRIPT_ROOT),
        "policy_digest": sha256_file(policy_path),
        "claim_boundary": CLAIM_BOUNDARY,
        "measurement": measure(runs, policy, measured_at),
        "runs": runs,
    }


def newest_index(directory: Path) -> Path | None:
    candidates = sorted(directory.glob(INDEX_GLOB))
    return candidates[-1] if candidates else None


def write_index(directory: Path, index: dict[str, Any]) -> Path:
    path = directory / f"index-{index['published_on']}.json"
    path.write_text(json.dumps(index, indent=2, sort_keys=False) + "\n", encoding="utf-8")
    return path


# ---------------------------------------------------------------------------
# entry point
# ---------------------------------------------------------------------------


def verify(root: Path, policy_path: Path, index_path: Path) -> list[dict[str, Any]]:
    policy = load_policy(policy_path)
    index = json.loads(index_path.read_text(encoding="utf-8"))

    checks: list[dict[str, Any]] = []
    if index.get("schema_version") != INDEX_SCHEMA_VERSION:
        checks.append(
            check(
                "index_schema",
                [
                    f"index schema_version is {index.get('schema_version')!r}, "
                    f"expected {INDEX_SCHEMA_VERSION!r}"
                ],
            )
        )
        return checks
    checks.append(check("index_schema", []))
    checks.append(check_policy_digest(index, policy_path))
    checks.append(check_run_records(index, policy))
    checks.append(check_replay(index, policy))
    checks.append(check_workflow_wiring(root, policy))
    checks.append(check_ledger(root, index, policy))
    checks.append(check_claim_language(root, index, policy))
    return checks


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Measure (or replay) the repeated CI evidence chain on the private corpus."
    )
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument("--policy", default=str(DEFAULT_POLICY), help="Versioned policy YAML.")
    parser.add_argument("--index", default=None, help="Published index to verify (default: newest).")
    parser.add_argument("--repo", default=DEFAULT_REPO, help="owner/name of the repository to query.")
    parser.add_argument(
        "--collect",
        action="store_true",
        help="Re-measure live from the GitHub Actions API (needs `gh`).",
    )
    parser.add_argument(
        "--publish",
        action="store_true",
        help="With --collect, write a new dated index instead of only reporting.",
    )
    parser.add_argument(
        "--published-on",
        default=None,
        help="Publication date (YYYY-MM-DD) for --publish; default: today (UTC).",
    )
    parser.add_argument("--quiet", action="store_true", help="Print the summary line only.")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    policy_path = resolve(root, args.policy)
    evidence_dir = policy_path.parent

    try:
        policy = load_policy(policy_path)
    except MeasurementError as exc:
        print(f"repeated-ci-evidence: NOT MEASURED — {exc}", file=sys.stderr)
        return 2

    if args.collect:
        try:
            runs = collect_runs(args.repo, policy)
        except (MeasurementError, subprocess.TimeoutExpired) as exc:
            print(f"repeated-ci-evidence: NOT MEASURED — {exc}", file=sys.stderr)
            return 2

        measured_at = utc_now()
        published_on = args.published_on or measured_at.strftime("%Y-%m-%d")
        index = build_index(args.repo, runs, policy, policy_path, measured_at, published_on)
        summary = index["measurement"]

        if args.publish:
            path = write_index(evidence_dir, index)
            print(f"published {rel(path, root)}")
        if not args.quiet:
            print(json.dumps(summary, indent=2))
        print(
            "repeated-ci-evidence: measured — "
            f"{summary['consecutive_green_runs']}/{summary['target_consecutive_green_runs']} "
            f"consecutive green runs, claim_unlocked={summary['claim_unlocked']}"
        )
        return 0

    index_path = resolve(root, args.index) if args.index else newest_index(evidence_dir)
    if index_path is None or not index_path.is_file():
        print(
            "repeated-ci-evidence: NOT MEASURED — no published index in "
            f"{rel(evidence_dir, root)}; run with --collect --publish",
            file=sys.stderr,
        )
        return 2

    try:
        checks = verify(root, policy_path, index_path)
    except (MeasurementError, json.JSONDecodeError) as exc:
        print(f"repeated-ci-evidence: NOT MEASURED — {exc}", file=sys.stderr)
        return 2

    failed = [entry for entry in checks if entry["status"] == "fail"]
    if not args.quiet:
        for entry in checks:
            print(f"  {entry['status']:<4} {entry['check']}")
            for problem in entry["problems"]:
                print(f"       - {problem}", file=sys.stderr)

    index = json.loads(index_path.read_text(encoding="utf-8"))
    summary = index.get("measurement", {})
    if failed:
        print(
            f"repeated-ci-evidence: FAIL — {len(failed)} check(s) failed against "
            f"{rel(index_path, root)}",
            file=sys.stderr,
        )
        return 1

    print(
        f"repeated-ci-evidence: OK — {rel(index_path, root)} replays exactly; "
        f"{summary.get('consecutive_green_runs')}/"
        f"{summary.get('target_consecutive_green_runs')} consecutive green runs, "
        f"claim_unlocked={summary.get('claim_unlocked')}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

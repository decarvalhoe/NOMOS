"""VRC-14 (#560) — the repeated-evidence gate measures the chain, and refuses drift.

Doctrine §2.3: the proof is the failure. Every test here breaks one rule and
proves the gate turns red — a hand-edited streak, a green run whose pack expired,
a missed week smuggled into the chain, a workflow that lost its schedule or
lowered retention, a ledger that disagrees with the measurement, and prose that
asserts the claim while the measurement says it is locked.

The shipped tree is verified too: the committed index must replay exactly.
"""

from __future__ import annotations

import copy
import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "repeated_ci_evidence.py"
EVIDENCE_DIR = ROOT / "docs" / "regulated" / "evidence-index" / "repeated-ci-evidence"
POLICY = EVIDENCE_DIR / "policy.yaml"

_SPEC = importlib.util.spec_from_file_location("repeated_ci_evidence", SCRIPT)
rce = importlib.util.module_from_spec(_SPEC)
assert _SPEC.loader is not None
_SPEC.loader.exec_module(rce)

BASE = datetime(2026, 9, 1, 5, 0, 0, tzinfo=timezone.utc)


def _policy() -> dict:
    return rce.load_policy(POLICY)


def _run(
    number: int,
    *,
    weeks_ago: int,
    conclusion: str = "success",
    expired: bool = False,
    artifacts: bool = True,
    corpus_commit: str = "d3485001762df83414969a16707fe7e59b1597a7",
) -> dict:
    """One synthetic scheduled run, ``weeks_ago`` weeks before BASE."""
    created = BASE - timedelta(days=7 * weeks_ago)
    pack = (
        [
            {
                "artifact_id": 1000 + number,
                "name": f"rbok-lawbook-artifacts-{corpus_commit}",
                "size_in_bytes": 3998564,
                "expired": expired,
                "expires_at": (created + timedelta(days=90)).strftime(rce.TIMESTAMP_FORMAT),
            }
        ]
        if artifacts
        else []
    )
    return {
        "run_id": 30000000 + number,
        "run_number": number,
        "run_attempt": 1,
        "event": "schedule",
        "branch": "main",
        "head_sha": f"{number:040x}",
        "status": "completed",
        "conclusion": conclusion,
        "created_at": created.strftime(rce.TIMESTAMP_FORMAT),
        "updated_at": created.strftime(rce.TIMESTAMP_FORMAT),
        "html_url": f"https://github.com/example/repo/actions/runs/{30000000 + number}",
        "corpus_commit": corpus_commit if artifacts else None,
        "artifacts": pack,
    }


def _weekly(count: int, **kwargs) -> list[dict]:
    """``count`` unbroken weekly green runs, newest first."""
    return [_run(200 - i, weeks_ago=i, **kwargs) for i in range(count)]


class MeasurementTests(unittest.TestCase):
    """The pure measurement: what counts, what breaks the chain."""

    def test_unbroken_weekly_chain_counts_every_run(self) -> None:
        result = rce.measure(_weekly(8), _policy(), BASE)
        self.assertEqual(result["consecutive_green_runs"], 8)
        self.assertEqual(result["missed_scheduled_occurrences"], 0)
        self.assertTrue(result["claim_unlocked"])

    def test_target_is_a_floor_not_a_rounding(self) -> None:
        # Seven green weekly runs is not eight. The claim stays locked.
        result = rce.measure(_weekly(7), _policy(), BASE)
        self.assertEqual(result["consecutive_green_runs"], 7)
        self.assertEqual(result["runs_remaining_to_target"], 1)
        self.assertFalse(result["claim_unlocked"])

    def test_missed_week_breaks_the_chain(self) -> None:
        # ADVERSARIAL: ten green runs, but the schedule went dark for six weeks
        # in the middle (a 49-day gap between two runs). Runs that happened
        # cannot vouch for weeks that produced nothing.
        runs = _weekly(4) + [_run(190 - i, weeks_ago=10 + i) for i in range(6)]
        result = rce.measure(runs, _policy(), BASE)
        self.assertEqual(result["consecutive_green_runs"], 4)
        # With the cadence rule off, all ten count — published so the cost of
        # the rule is visible instead of hidden.
        self.assertEqual(result["consecutive_green_runs_ignoring_cadence"], 10)
        self.assertEqual(result["missed_scheduled_occurrences"], 6)
        self.assertEqual(result["missed_windows"][0]["gap_days"], 49)
        self.assertIn("cadence tolerance", result["streak_break_reason"])
        self.assertFalse(result["claim_unlocked"])

    def test_delayed_run_inside_tolerance_still_counts(self) -> None:
        # Precision: GitHub delaying a scheduled run by two days is still that
        # week's run, not a missed occurrence.
        runs = _weekly(8)
        late = rce.parse_ts(runs[1]["created_at"]) - timedelta(days=2)
        runs[1]["created_at"] = late.strftime(rce.TIMESTAMP_FORMAT)
        result = rce.measure(runs, _policy(), BASE)
        self.assertEqual(result["consecutive_green_runs"], 8)
        self.assertEqual(result["missed_scheduled_occurrences"], 0)

    def test_expired_pack_makes_a_green_run_uncountable(self) -> None:
        # ADVERSARIAL: the run is green, but its pack aged out. Nothing is left
        # to re-inspect, so it is not archived evidence.
        runs = _weekly(8)
        runs[3]["artifacts"][0]["expired"] = True
        result = rce.measure(runs, _policy(), BASE)
        self.assertEqual(result["consecutive_green_runs"], 3)
        self.assertIn("expired", result["streak_break_reason"])
        self.assertFalse(result["claim_unlocked"])

    def test_green_run_without_any_pack_is_not_evidence(self) -> None:
        runs = _weekly(8)
        runs[2]["artifacts"] = []
        result = rce.measure(runs, _policy(), BASE)
        self.assertEqual(result["consecutive_green_runs"], 2)
        self.assertIn("archived no artifact", result["streak_break_reason"])

    def test_non_success_conclusions_are_never_green(self) -> None:
        # Including the ones that are not outright failures.
        for conclusion in ("failure", "cancelled", "timed_out", "skipped", None):
            with self.subTest(conclusion=conclusion):
                runs = _weekly(8)
                runs[1]["conclusion"] = conclusion
                result = rce.measure(runs, _policy(), BASE)
                self.assertEqual(result["consecutive_green_runs"], 1)
                self.assertFalse(result["claim_unlocked"])

    def test_stale_chain_does_not_unlock_the_claim(self) -> None:
        # ADVERSARIAL: eight consecutive green runs that all ended six months
        # ago. That is history, not repeated evidence.
        result = rce.measure(_weekly(8), _policy(), BASE + timedelta(days=180))
        self.assertEqual(result["consecutive_green_runs"], 8)
        self.assertFalse(result["streak_is_current"])
        self.assertFalse(result["claim_unlocked"])

    def test_newest_run_red_means_no_streak_at_all(self) -> None:
        runs = _weekly(8)
        runs[0]["conclusion"] = "failure"
        result = rce.measure(runs, _policy(), BASE)
        self.assertEqual(result["consecutive_green_runs"], 0)
        self.assertIn("newest scheduled run is not countable", result["streak_break_reason"])

    def test_no_run_at_all_measures_zero_not_success(self) -> None:
        result = rce.measure([], _policy(), BASE)
        self.assertEqual(result["consecutive_green_runs"], 0)
        self.assertFalse(result["claim_unlocked"])

    def test_repeating_one_corpus_revision_is_visible(self) -> None:
        # Eight runs over one frozen corpus is not eight corpus states. The
        # index must let a reader see which of the two it is looking at.
        frozen = rce.measure(_weekly(8), _policy(), BASE)
        self.assertEqual(frozen["distinct_corpus_commits"], 1)

        moving = [
            _run(200 - i, weeks_ago=i, corpus_commit=f"{i:040x}") for i in range(8)
        ]
        self.assertEqual(
            rce.measure(moving, _policy(), BASE)["distinct_corpus_commits"], 8
        )

    def test_measurement_is_order_independent(self) -> None:
        runs = _weekly(6)
        forward = rce.measure(runs, _policy(), BASE)
        shuffled = rce.measure(list(reversed(runs)), _policy(), BASE)
        self.assertEqual(forward, shuffled)


class GateTests(unittest.TestCase):
    """The offline gate over a published index and the tree around it."""

    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = Path(self._tmp.name) / "repo"
        (self.root / "docs" / "regulated" / "evidence-index").mkdir(parents=True)
        (self.root / ".github" / "workflows").mkdir(parents=True)

        self.evidence_dir = self.root / "docs" / "regulated" / "evidence-index" / "repeated-ci-evidence"
        self.evidence_dir.mkdir()
        self.policy_path = self.evidence_dir / "policy.yaml"
        self.policy_path.write_text(POLICY.read_text(encoding="utf-8"), encoding="utf-8")
        self.policy = rce.load_policy(self.policy_path)

        (self.root / ".github" / "workflows" / "rbok-lawbook-e2e.yml").write_text(
            "name: RBOK Lawbook E2E\non:\n  schedule:\n    - cron: '0 5 * * 1'\n"
            "jobs:\n  e2e:\n    steps:\n      - uses: actions/upload-artifact@v4\n"
            "        with:\n          retention-days: 90\n",
            encoding="utf-8",
        )

        self.index = rce.build_index(
            "example/repo",
            _weekly(4),
            self.policy,
            self.policy_path,
            BASE,
            "2026-09-01",
        )
        self.index_path = rce.write_index(self.evidence_dir, self.index)
        self._write_ledger(unlocked=False)
        self._write_claim_boundary(unlocked=False, streak=4)

    def _write_ledger(self, *, unlocked: bool) -> None:
        claim = self.policy["claim"]
        (self.root / "docs" / "regulated" / "evidence-index" / "evidence-ledger.yaml").write_text(
            "evidence_categories:\n"
            f"  - id: {claim['ledger_entry']}\n"
            f"    current_status: {'present_measured' if unlocked else 'requires_evidence'}\n"
            f"    claim_allowed: \"{claim['id'] if unlocked else 'none'}\"\n"
            "blocking_gaps:\n"
            f"  - id: {claim['blocking_gap']}\n"
            f"    status: {'closed' if unlocked else 'open'}\n",
            encoding="utf-8",
        )

    def _write_claim_boundary(self, *, unlocked: bool, streak: int) -> None:
        marker = self.policy["claim"]["unlocked_marker"]
        body = f"# Claim Boundary\n\nMeasured: {streak} consecutive green runs.\n"
        if unlocked:
            body += f"\n{marker.capitalize()}.\n"
        (self.root / "docs" / "public-claim-boundary.md").write_text(body, encoding="utf-8")

    def _verify(self) -> list[dict]:
        return rce.verify(self.root, self.policy_path, self.index_path)

    def _failures(self) -> set[str]:
        return {c["check"] for c in self._verify() if c["status"] == "fail"}

    def _rewrite(self, index: dict) -> None:
        self.index_path.write_text(json.dumps(index, indent=2) + "\n", encoding="utf-8")

    def test_freshly_built_index_passes_every_check(self) -> None:
        self.assertEqual(self._failures(), set())

    def test_hand_edited_streak_is_named(self) -> None:
        # ADVERSARIAL: someone types 8 over the measured 4. The replay recomputes
        # from the recorded runs and names the edited key.
        index = copy.deepcopy(self.index)
        index["measurement"]["consecutive_green_runs"] = 8
        index["measurement"]["claim_unlocked"] = True
        self._rewrite(index)
        checks = {c["check"]: c for c in self._verify()}
        self.assertEqual(checks["replay"]["status"], "fail")
        self.assertTrue(
            any("consecutive_green_runs" in p for p in checks["replay"]["problems"]),
            checks["replay"]["problems"],
        )

    def test_deleting_the_run_that_broke_the_chain_is_caught(self) -> None:
        # ADVERSARIAL: drop an inconvenient red run instead of fixing the chain.
        index = copy.deepcopy(self.index)
        index["runs"] = index["runs"][:2]
        self._rewrite(index)
        self.assertIn("replay", self._failures())

    def test_policy_edited_after_publication_is_caught(self) -> None:
        # ADVERSARIAL: lower the target from 8 to 4 to unlock the claim.
        text = self.policy_path.read_text(encoding="utf-8")
        self.policy_path.write_text(
            text.replace("consecutive_green_runs: 8", "consecutive_green_runs: 4"),
            encoding="utf-8",
        )
        self.assertIn("policy_digest", self._failures())

    def test_workflow_losing_its_schedule_turns_the_gate_red(self) -> None:
        workflow = self.root / ".github" / "workflows" / "rbok-lawbook-e2e.yml"
        workflow.write_text(
            workflow.read_text(encoding="utf-8").replace(
                "  schedule:\n    - cron: '0 5 * * 1'\n", ""
            ),
            encoding="utf-8",
        )
        self.assertIn("workflow_wiring", self._failures())

    def test_lowering_artifact_retention_turns_the_gate_red(self) -> None:
        # Shortening retention silently erases archived evidence.
        workflow = self.root / ".github" / "workflows" / "rbok-lawbook-e2e.yml"
        workflow.write_text(
            workflow.read_text(encoding="utf-8").replace("retention-days: 90", "retention-days: 7"),
            encoding="utf-8",
        )
        problems = {c["check"]: c["problems"] for c in self._verify()}
        self.assertIn("workflow_wiring", self._failures())
        self.assertTrue(any("retention" in p for p in problems["workflow_wiring"]))

    def test_ledger_closing_the_gap_early_turns_the_gate_red(self) -> None:
        # ADVERSARIAL: mark the gap closed while the chain is at 4 of 8.
        self._write_ledger(unlocked=True)
        self.assertIn("evidence_ledger", self._failures())

    def test_prose_asserting_a_locked_claim_turns_the_gate_red(self) -> None:
        # ADVERSARIAL: the documentation claims the thing the measurement denies.
        self._write_claim_boundary(unlocked=True, streak=4)
        checks = {c["check"]: c for c in self._verify()}
        self.assertEqual(checks["claim_language"]["status"], "fail")
        self.assertTrue(
            any("while the measurement says" in p for p in checks["claim_language"]["problems"]),
            checks["claim_language"]["problems"],
        )

    def test_prose_drifting_from_the_measured_streak_is_caught(self) -> None:
        # The claim boundary still says 7 after the measurement moved to 4.
        self._write_claim_boundary(unlocked=False, streak=7)
        self.assertIn("claim_language", self._failures())

    def test_reaching_the_target_requires_the_whole_tree_to_move_together(self) -> None:
        # The unlocked direction is gated just as hard: an index that reaches 8
        # fails until the ledger AND the claim boundary follow.
        self.index = rce.build_index(
            "example/repo", _weekly(8), self.policy, self.policy_path, BASE, "2026-09-01"
        )
        self.assertTrue(self.index["measurement"]["claim_unlocked"])
        self.index_path = rce.write_index(self.evidence_dir, self.index)
        self.assertEqual(self._failures(), {"evidence_ledger", "claim_language"})

        self._write_ledger(unlocked=True)
        self._write_claim_boundary(unlocked=True, streak=8)
        self.assertEqual(self._failures(), set())

    def test_duplicate_run_id_is_refused(self) -> None:
        index = copy.deepcopy(self.index)
        index["runs"][1]["run_id"] = index["runs"][0]["run_id"]
        self._rewrite(index)
        self.assertIn("run_records", self._failures())

    def test_run_from_another_branch_or_event_is_refused(self) -> None:
        for field, value in (("branch", "some-feature"), ("event", "workflow_dispatch")):
            with self.subTest(field=field):
                index = copy.deepcopy(self.index)
                index["runs"][0][field] = value
                self._rewrite(index)
                self.assertIn("run_records", self._failures())

    def test_missing_index_is_exit_2_not_a_pass(self) -> None:
        # The absence of a measurement is not evidence of a chain.
        self.index_path.unlink()
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(self.root),
             "--policy", str(self.policy_path)],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn("NOT MEASURED", result.stderr)

    def test_script_exits_1_on_a_failed_check(self) -> None:
        self._write_ledger(unlocked=True)
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(self.root),
             "--policy", str(self.policy_path)],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)


class ShippedTreeTests(unittest.TestCase):
    """The committed index must replay exactly against the committed tree."""

    def test_published_index_replays(self) -> None:
        index_path = rce.newest_index(EVIDENCE_DIR)
        self.assertIsNotNone(index_path, "no published index in tree")
        failures = [c for c in rce.verify(ROOT, POLICY, index_path) if c["status"] == "fail"]
        self.assertEqual(failures, [], failures)

    def test_shipped_measurement_is_below_target_and_says_so(self) -> None:
        # Pins the honest state: the chain is measured, and it is not there yet.
        index = json.loads(rce.newest_index(EVIDENCE_DIR).read_text(encoding="utf-8"))
        measurement = index["measurement"]
        self.assertFalse(measurement["claim_unlocked"])
        self.assertLess(
            measurement["consecutive_green_runs"],
            measurement["target_consecutive_green_runs"],
        )

    def test_gate_runs_clean_as_a_subprocess(self) -> None:
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(ROOT)],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()

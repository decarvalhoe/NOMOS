"""NRT-033 (#716) — the evidence ledger is an index computed from the tree:
drift, missing locations, stale dates and softened declarations are red."""

from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "evidence_ledger_guard.py"
LEDGER = Path("docs/regulated/evidence-index/evidence-ledger.yaml")


def run(root: Path, *args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, str(SCRIPT), "--root", str(root), "--today", "2026-09-06", *args], capture_output=True, text=True)


def mini(tmp: Path) -> Path:
    root = tmp / "repo"
    (root / "docs/regulated/evidence-index").mkdir(parents=True)
    (root / "docs/regulated/qms").mkdir(parents=True)
    (root / "docs/regulated/qms/sop.md").write_text("# SOP\n\nstatus: draft\n")
    (root / "docs/regulated/plan.md").write_text("# plan\n")
    ledger = {
        "schema_version": "0.1.0", "generated_at": "2026-05-02", "status": "draft",
        "claim_boundary": "Evidence ledger baseline only. Missing evidence is not assumed.",
        "evidence_categories": [
            {"id": "EV-A", "category": "quality_system_documents", "expected_location": "docs/regulated/qms/", "current_status": "draft_not_effective", "claim_allowed": "documentation_baseline_only"},
            {"id": "EV-B", "category": "validation_evidence", "expected_location": "docs/regulated/validation-pack/", "current_status": "requires_evidence", "claim_allowed": "none"},
            {"id": "EV-C", "category": "regulated_automation_reports", "expected_location": ".regulated-evidence-pack/", "current_status": "generated_by_workflow_when_run", "claim_allowed": "evidence_inventory_only"},
            {"id": "EV-D", "category": "plan", "expected_location": "docs/regulated/plan.md", "current_status": "present_draft", "claim_allowed": "execution_plan_only"},
        ],
        "blocking_gaps": [{"id": "GAP-X", "description": "x", "severity": "major", "status": "open", "blocks_claims": ["a"]}],
    }
    (root / LEDGER).write_text(yaml.safe_dump(ledger, sort_keys=False))
    return root


def edit(root: Path, fn) -> None:
    p = root / LEDGER
    doc = yaml.safe_load(p.read_text())
    fn(doc)
    p.write_text(yaml.safe_dump(doc, sort_keys=False))


class LedgerGuardTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = mini(Path(self._tmp.name))

    def test_draft_ledger_is_red_until_written_then_green(self) -> None:
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 1)
        self.assertIn("status is 'draft'", r.stderr)
        self.assertIn("observed block drifts", r.stderr)
        r = run(self.root, "--write")
        self.assertEqual(r.returncode, 0, r.stderr)
        doc = yaml.safe_load((self.root / LEDGER).read_text())
        self.assertEqual(doc["status"], "effective")
        self.assertEqual(doc["generated_at"], "2026-09-06")
        self.assertEqual(doc["evidence_categories"][0]["observed"], {"exists": True, "kind": "directory", "generated_path": False, "files": 1, "statuses": {"draft": 1}, "unmarked": 0})
        self.assertEqual(doc["evidence_categories"][0]["current_status"], "draft_not_effective", "observation never rewrites the declaration")
        self.assertIn("Missing evidence is not assumed", doc["claim_boundary"])
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 0, r.stderr)

    def test_tree_change_without_regeneration_is_drift(self) -> None:
        run(self.root, "--write")
        (self.root / "docs/regulated/qms/sop2.md").write_text("status: effective\n")
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 1)
        self.assertIn("EV-A: observed block drifts from the tree", r.stderr)

    def test_declared_presence_over_missing_location_is_red(self) -> None:
        shutil.rmtree(self.root / "docs/regulated/qms")
        r = run(self.root, "--write")
        self.assertEqual(r.returncode, 1)
        self.assertIn("EV-A: declared draft_not_effective but docs/regulated/qms/ does not exist", r.stderr)

    def test_generated_status_on_a_real_path_is_red(self) -> None:
        edit(self.root, lambda d: d["evidence_categories"][3].__setitem__("current_status", "generated_by_workflow_when_run"))
        r = run(self.root, "--write")
        self.assertEqual(r.returncode, 1)
        self.assertIn("EV-D: generated_by_workflow_when_run is only for generated paths", r.stderr)

    def test_present_over_drafts_and_unknown_vocabulary_are_red(self) -> None:
        edit(self.root, lambda d: d["evidence_categories"][0].__setitem__("current_status", "present"))
        r = run(self.root, "--write")
        self.assertEqual(r.returncode, 1)
        self.assertIn("EV-A: declared present but the files carry draft markers {'draft': 1}", r.stderr)
        edit(self.root, lambda d: d["evidence_categories"][0].__setitem__("current_status", "effective_enough"))
        r = run(self.root, "--write")
        self.assertEqual(r.returncode, 1)
        self.assertIn("current_status 'effective_enough' is not in the ledger vocabulary", r.stderr)

    def test_stale_index_is_red(self) -> None:
        run(self.root, "--write")
        edit(self.root, lambda d: d.__setitem__("generated_at", "2026-05-02"))
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 1)
        self.assertIn("older than 90 days — stale index", r.stderr)

    def test_malformed_gap_and_dropped_claim_boundary_are_red(self) -> None:
        edit(self.root, lambda d: d["blocking_gaps"][0].pop("blocks_claims"))
        r = run(self.root, "--write")
        self.assertEqual(r.returncode, 1)
        self.assertIn("blocking_gaps[0] (GAP-X): missing blocks_claims", r.stderr)
        edit(self.root, lambda d: d.__setitem__("claim_boundary", "all good"))
        r = run(self.root, "--write")
        self.assertEqual(r.returncode, 1)
        self.assertIn("claim_boundary must keep the rule", r.stderr)


class RealLedgerTest(unittest.TestCase):
    def test_the_committed_ledger_is_a_fresh_index(self) -> None:
        r = subprocess.run([sys.executable, str(SCRIPT), "--root", str(ROOT), "--check"], capture_output=True, text=True)
        self.assertEqual(r.returncode, 0, r.stderr)
        doc = yaml.safe_load((ROOT / LEDGER).read_text())
        self.assertEqual(doc["status"], "effective")
        for cat in doc["evidence_categories"]:
            self.assertIn("observed", cat, cat["id"])
        # honest recount: the QMS documents are still drafts and the index says so
        qms = next(c for c in doc["evidence_categories"] if c["id"] == "EV-QMS-001")
        self.assertEqual(qms["current_status"], "draft_not_effective")
        self.assertGreaterEqual(qms["observed"]["statuses"].get("draft", 0), 1)


if __name__ == "__main__":
    unittest.main()

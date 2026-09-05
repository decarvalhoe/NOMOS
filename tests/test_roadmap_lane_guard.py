from __future__ import annotations

import copy
import importlib.util
import sys
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
REGISTRY = ROOT / "docs/roadmap-lanes.yaml"
SPEC = importlib.util.spec_from_file_location(
    "roadmap_lane_guard", ROOT / "scripts/roadmap_lane_guard.py"
)
guard = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
sys.modules["roadmap_lane_guard"] = guard
SPEC.loader.exec_module(guard)


def registry() -> dict:
    return yaml.safe_load(REGISTRY.read_text(encoding="utf-8"))


class RoadmapLaneGuardTests(unittest.TestCase):
    def test_real_registry_is_autonomously_dispatchable(self) -> None:
        self.assertEqual(guard.validate(registry()), [])

    def test_autonomous_item_cannot_wait_on_passive_evidence(self) -> None:
        data = registry()
        item = next(item for item in data["items"] if item["issue"] == 639)
        item["depends_on"] = [560]
        failures = guard.validate(data)
        self.assertTrue(any("dependency #560 is passive" in failure for failure in failures), failures)

    def test_hard_dependency_cannot_cross_lanes(self) -> None:
        data = registry()
        item = next(item for item in data["items"] if item["issue"] == 637)
        item["depends_on"] = [640]
        failures = guard.validate(data)
        self.assertTrue(any("crosses product -> devops" in failure for failure in failures), failures)

    def test_non_autonomous_item_cannot_enter_a_dispatch_queue(self) -> None:
        data = registry()
        data["selection_policy"]["dispatch_queues"]["regulated"].append(562)
        failures = guard.validate(data)
        self.assertTrue(any("#562 is not an open autonomous item" in failure for failure in failures), failures)

    def test_every_open_autonomous_item_is_ordered(self) -> None:
        # Drop whichever item currently heads the product queue; the guard must
        # name it. Not a literal issue number — those drift with every closure.
        data = registry()
        queue = data["selection_policy"]["dispatch_queues"]["product"]
        head = queue[0]
        queue.remove(head)
        failures = guard.validate(data)
        self.assertTrue(
            any(f"omit open autonomous issue(s): #{head}" in failure for failure in failures), failures
        )

    def test_regulated_tool_must_declare_intended_use_and_validation(self) -> None:
        data = copy.deepcopy(registry())
        item = next(item for item in data["items"] if item["issue"] == 640)
        del item["regulated_tool"]["intended_use"]
        del item["regulated_tool"]["validation_state"]
        failures = guard.validate(data)
        self.assertTrue(
            any("regulated_tool misses intended_use, validation_state" in failure for failure in failures),
            failures,
        )

    def test_dependency_cycle_is_refused(self) -> None:
        data = registry()
        issue_610 = next(item for item in data["items"] if item["issue"] == 610)
        issue_611 = next(item for item in data["items"] if item["issue"] == 611)
        issue_610["depends_on"] = [611]
        issue_611["depends_on"] = [610]
        failures = guard.validate(data)
        self.assertTrue(any("hard dependency cycle: #610 -> #611 -> #610" in failure for failure in failures), failures)

    def test_regulated_tool_enums_and_reliance_are_enforced(self) -> None:
        data = registry()
        item = next(item for item in data["items"] if item["issue"] == 637)
        item["regulated_tool"]["impact"] = "looks-important"
        item["regulated_tool"]["validation_state"] = "probably-fine"
        item["regulated_tool"]["reliance"] = "sole_reliance_validated"
        failures = guard.validate(data)
        self.assertTrue(any("unknown regulated_tool impact" in failure for failure in failures), failures)
        self.assertTrue(any("unknown regulated_tool validation_state" in failure for failure in failures), failures)
        self.assertTrue(any("sole reliance requires" in failure for failure in failures), failures)

    def test_critical_decision_and_unlocked_claim_require_validation(self) -> None:
        data = registry()
        item = next(item for item in data["items"] if item["issue"] == 637)
        item["regulated_tool"]["impact"] = "critical_decision"
        item["claim_state"] = "unlocked"
        failures = guard.validate(data)
        self.assertTrue(any("critical_decision is prohibited" in failure for failure in failures), failures)
        self.assertTrue(any("unvalidated regulated tool cannot unlock" in failure for failure in failures), failures)
        self.assertTrue(any("cannot carry an unlocked claim" in failure for failure in failures), failures)

    def test_each_lane_has_its_own_queue(self) -> None:
        # Structural, not literal: hardcoding issue numbers here is how the
        # published queues drifted from reality in the first place.
        data = registry()
        queues = data["selection_policy"]["dispatch_queues"]
        by_issue = {item["issue"]: item for item in data["items"]}
        self.assertEqual(set(queues), {"product", "devops", "regulated"})
        for lane in ("product", "devops"):
            self.assertTrue(queues[lane], f"{lane} queue is empty")
            for issue in queues[lane]:
                self.assertEqual(by_issue[issue]["lane"], lane, issue)
                self.assertEqual(by_issue[issue]["state"], "open", issue)
        self.assertFalse(set(queues["product"]) & set(queues["devops"]))

    def test_unknown_dispatch_queue_is_refused(self) -> None:
        data = registry()
        data["selection_policy"]["dispatch_queues"]["human"] = [562]
        failures = guard.validate(data)
        self.assertTrue(any("unknown lane(s): human" in failure for failure in failures), failures)

    def test_selection_policy_cannot_be_weakened(self) -> None:
        data = registry()
        data["selection_policy"]["eligible_dispatch"] = "human"
        data["selection_policy"]["hard_dependencies"]["autonomous_only"] = False
        data["selection_policy"]["cross_lane_relationship"] = "hard_dependency"
        failures = guard.validate(data)
        self.assertTrue(any("eligible_dispatch must be autonomous" in failure for failure in failures), failures)
        self.assertTrue(any("must require same_lane_only and autonomous_only" in failure for failure in failures), failures)
        self.assertTrue(any("must be inputs_are_nonblocking" in failure for failure in failures), failures)

    def test_item_state_vocabulary_is_closed(self) -> None:
        data = registry()
        item = next(item for item in data["items"] if item["issue"] == 640)
        item["claim_state"] = "probably"
        item["delivery_state"] = "almost"
        failures = guard.validate(data)
        self.assertTrue(any("unknown claim_state 'probably'" in failure for failure in failures), failures)
        self.assertTrue(any("unknown delivery_state 'almost'" in failure for failure in failures), failures)


if __name__ == "__main__":
    unittest.main()


class RegistryTruthTests(unittest.TestCase):
    """Step 0 of the roadmap run: the registry must tell the truth, and the docs
    must repeat it mechanically rather than by hand.

    The two first closures after the registry landed (#642, #640) were closed on
    GitHub while still sitting `state: open` at the head of their queues. The
    guard stayed green because it validates internal consistency only. These
    tests pin the three answers: generated tables, an umbrella section for the
    epic, and a network check that compares with GitHub.
    """

    def test_delivered_items_are_closed_and_out_of_the_queues(self) -> None:
        data = registry()
        by_issue = {item["issue"]: item for item in data["items"]}
        queues = data["selection_policy"]["dispatch_queues"]
        for issue in (640, 642):
            self.assertEqual(by_issue[issue]["state"], "closed", issue)
            self.assertNotIn(issue, queues["product"] + queues["devops"], issue)
            self.assertIn(by_issue[issue]["delivery_state"], {"implemented", "verified"})

    def test_a_delivered_tool_is_technically_verified_never_validated_by_delivery(self) -> None:
        # Moving to validated_for_intended_use is a regulated act (#562), not
        # something a merge can grant itself.
        item = next(i for i in registry()["items"] if i["issue"] == 640)
        self.assertEqual(item["regulated_tool"]["validation_state"], "technically_verified")

    def test_queue_table_is_rendered_from_the_registry(self) -> None:
        # Structural: the table shows exactly the registry's open autonomous
        # items, in queue order, and nothing closed. Hardcoding the current head
        # here would make this test drift with every closure — the very defect
        # this table exists to remove.
        data = registry()
        table = guard.render_queue_table(data)
        self.assertIn(guard.QUEUE_BEGIN, table)
        self.assertIn(guard.QUEUE_END, table)
        self.assertIn("| Product queue | DevOps queue |", table)
        queues = data["selection_policy"]["dispatch_queues"]
        for issue in queues["product"] + queues["devops"]:
            self.assertIn(f"#{issue} —", table, f"queued #{issue} missing from the table")
        for item in data["items"]:
            if item["state"] == "closed":
                self.assertNotIn(f"#{item['issue']} —", table, f"closed #{item['issue']} rendered")

    def test_queue_table_pads_the_shorter_lane(self) -> None:
        data = registry()
        data["selection_policy"]["dispatch_queues"]["devops"] = []
        table = guard.render_queue_table(data)
        rows = [line for line in table.splitlines() if line.startswith("| #")]
        self.assertTrue(rows)
        self.assertTrue(all(line.endswith("| — |") for line in rows), rows)

    def test_queue_table_names_an_undeclared_issue_instead_of_crashing(self) -> None:
        data = registry()
        data["selection_policy"]["dispatch_queues"]["product"] = [999999]
        self.assertIn("#999999 — (not declared)", guard.render_queue_table(data))

    def test_emit_docs_regenerates_only_the_marked_block(self) -> None:
        import tempfile

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for rel in guard.QUEUE_DOCS:
                path = root / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(
                    "# Doc\n\nhand-written prose that must survive\n\n"
                    f"{guard.QUEUE_BEGIN}\n| stale | table |\n{guard.QUEUE_END}\n\n"
                    "more prose after\n",
                    encoding="utf-8",
                )
            self.assertEqual(guard.emit_docs(root, registry()), [])
            text = (root / guard.QUEUE_DOCS[0]).read_text(encoding="utf-8")
            self.assertIn("hand-written prose that must survive", text)
            self.assertIn("more prose after", text)
            self.assertNotIn("| stale | table |", text)
            head = registry()["selection_policy"]["dispatch_queues"]["product"][0]
            self.assertIn(f"#{head} —", text)
            # Idempotent: a second run changes nothing.
            before = text
            guard.emit_docs(root, registry())
            self.assertEqual((root / guard.QUEUE_DOCS[0]).read_text(encoding="utf-8"), before)

    def test_emit_docs_refuses_a_doc_without_markers(self) -> None:
        import tempfile

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for rel in guard.QUEUE_DOCS:
                path = root / rel
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("# Doc with no markers\n", encoding="utf-8")
            problems = guard.emit_docs(root, registry())
            self.assertEqual(len(problems), len(guard.QUEUE_DOCS))
            self.assertTrue(all("no " in p and "block" in p for p in problems), problems)

    def test_shipped_docs_carry_the_generated_block_and_no_drift(self) -> None:
        # What CI enforces: regenerating into the committed docs is a no-op.
        import shutil
        import tempfile

        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            for rel in guard.QUEUE_DOCS:
                (root / rel).parent.mkdir(parents=True, exist_ok=True)
                shutil.copy(ROOT / rel, root / rel)
            self.assertEqual(guard.emit_docs(root, registry()), [])
            for rel in guard.QUEUE_DOCS:
                self.assertEqual(
                    (root / rel).read_bytes(),
                    (ROOT / rel).read_bytes(),
                    f"{rel} drifts from the registry",
                )

    def test_umbrella_epic_is_declared_and_never_dispatched(self) -> None:
        data = registry()
        umbrellas = {u["issue"]: u for u in data["umbrella_issues"]}
        self.assertIn(545, umbrellas)
        self.assertEqual(umbrellas[545]["role"], "epic")
        self.assertEqual(guard.validate(data), [])

    def test_umbrella_cannot_also_be_an_item_or_be_queued(self) -> None:
        data = registry()
        data["items"].append(
            {
                "issue": 545, "title": "epic as item", "state": "open", "lane": "product",
                "dispatch": "autonomous", "delivery_state": "planned", "evidence_state": "none",
                "claim_state": "bounded", "depends_on": [],
            }
        )
        data["selection_policy"]["dispatch_queues"]["product"].append(545)
        failures = guard.validate(data)
        self.assertTrue(any("also declared as a roadmap item" in f for f in failures), failures)
        self.assertTrue(any("appears in a dispatch queue" in f for f in failures), failures)

    def test_umbrella_needs_a_known_role_and_a_note(self) -> None:
        data = registry()
        data["umbrella_issues"] = [{"issue": 545, "role": "whatever", "note": ""}]
        failures = guard.validate(data)
        self.assertTrue(any("unknown role" in f for f in failures), failures)
        self.assertTrue(any("a note is required" in f for f in failures), failures)

    def test_verify_github_reports_unreachable_rather_than_passing(self) -> None:
        # Without network the check must FAIL loudly, never silently pass: the
        # absence of an answer is not agreement.
        import os
        from unittest import mock

        data = {"items": [{"issue": 1, "state": "open"}]}
        env = {**os.environ, "PATH": ""}
        with mock.patch.dict(os.environ, env, clear=True):
            problems = guard.verify_github(data)
        self.assertEqual(len(problems), 1)
        self.assertIn("GitHub unreachable", problems[0])

    def test_verify_github_names_a_state_mismatch(self) -> None:
        from unittest import mock

        fake = mock.Mock()
        fake.stdout = "CLOSED\n"
        with mock.patch.object(guard.subprocess, "run", return_value=fake):
            problems = guard.verify_github({"items": [{"issue": 7, "state": "open"}]})
        self.assertEqual(problems, ["issue #7: registry says 'open', GitHub says 'closed'"])

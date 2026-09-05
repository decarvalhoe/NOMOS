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
        data = registry()
        data["selection_policy"]["dispatch_queues"]["product"].remove(610)
        failures = guard.validate(data)
        self.assertTrue(any("omit open autonomous issue(s): #610" in failure for failure in failures), failures)

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
        data = registry()
        queues = data["selection_policy"]["dispatch_queues"]
        self.assertEqual(set(queues), {"product", "devops", "regulated"})
        self.assertIn(640, queues["devops"])
        self.assertIn(642, queues["product"])
        self.assertNotIn(640, queues["product"])

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

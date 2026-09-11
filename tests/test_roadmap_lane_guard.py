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
sys.path.insert(0, str(ROOT / "scripts"))
import forge_provider  # noqa: E402


def registry() -> dict:
    return yaml.safe_load(REGISTRY.read_text(encoding="utf-8"))


def nonempty_lane(data) -> str:
    """The first lane whose autonomous queue is not exhausted. When every
    autonomous item of both lanes is closed (the roadmap's goal state), reopen
    one closed autonomous item IN THE TEST DATA so the queue rules can still be
    exercised — the registry on disk is not touched. Tests must not assume a
    particular lane still has work: queues empty as items close."""
    queues = data["selection_policy"]["dispatch_queues"]
    for lane in ("product", "devops"):
        if queues[lane]:
            return lane
    item = next(
        item for item in data["items"]
        if item["dispatch"] == "autonomous" and item["state"] == "closed" and item["lane"] in ("product", "devops")
    )
    item["state"] = "open"
    item["delivery_state"] = "planned"
    item["evidence_state"] = "none"
    if item.get("regulated_tool"):
        item["regulated_tool"]["validation_state"] = "planned"
    queues[item["lane"]].append(item["issue"])
    return item["lane"]


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
        queue = data["selection_policy"]["dispatch_queues"][nonempty_lane(data)]
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
        # Structural: any still-planned regulated tool, not a hardcoded issue —
        # the item that used to be named here has since been delivered.
        data = registry()
        item = next(item for item in data["items"] if item.get("regulated_tool"))
        # Put the item back into the planned state IN THE TEST DATA: the rule
        # under test is about a planned, unvalidated tool claiming too much.
        item["delivery_state"] = "planned"
        item["regulated_tool"]["validation_state"] = "planned"
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
            open_autonomous = [
                item["issue"] for item in data["items"]
                if item["lane"] == lane and item["dispatch"] == "autonomous" and item["state"] == "open"
            ]
            # An empty queue is only honest when the lane has nothing autonomous left open.
            self.assertEqual(sorted(queues[lane]), sorted(open_autonomous), lane)
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
        kept = nonempty_lane(data)
        other = "devops" if kept == "product" else "product"
        data["selection_policy"]["dispatch_queues"][other] = []
        table = guard.render_queue_table(data)
        rows = [line for line in table.splitlines() if line.startswith("|") and "#" in line]
        self.assertTrue(rows)
        # every row shows the exhausted lane as an em dash, on the side it occupies
        for line in rows:
            cells = [c.strip() for c in line.strip("|").split("|")]
            self.assertEqual(cells[0 if other == "product" else 1], "—", line)

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
            data = registry()
            lane = nonempty_lane(data)
            self.assertEqual(guard.emit_docs(root, data), [])
            text = (root / guard.QUEUE_DOCS[0]).read_text(encoding="utf-8")
            self.assertIn("hand-written prose that must survive", text)
            self.assertIn("more prose after", text)
            self.assertNotIn("| stale | table |", text)
            head = data["selection_policy"]["dispatch_queues"][lane][0]
            self.assertIn(f"#{head} —", text)
            # Idempotent: a second run changes nothing.
            before = text
            guard.emit_docs(root, data)
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

    def test_verify_tracker_reports_unreachable_rather_than_passing(self) -> None:
        # Without an answer the check must FAIL loudly, never silently pass:
        # the absence of an answer is not agreement.
        fake = forge_provider.FakeProvider()  # no issue seeded → 404
        problems = guard.verify_tracker({"items": [{"issue": 1, "state": "open"}]}, {"github": (fake, "o/r")})
        self.assertEqual(len(problems), 1)
        self.assertIn("issue #1 (github): tracker unreachable", problems[0])
        self.assertEqual(fake.calls, [("get_issue", {"repo": "o/r", "number": 1})])

    def test_verify_tracker_names_a_state_mismatch(self) -> None:
        fake = forge_provider.FakeProvider()
        fake.seed_issues("o/r", [{"number": 7, "title": "x", "state": "closed"}])
        problems = guard.verify_tracker({"items": [{"issue": 7, "state": "open"}]}, {"github": (fake, "o/r")})
        self.assertEqual(problems, ["issue #7 (github): registry says 'open', tracker says 'closed'"])

    def test_verify_tracker_agrees_when_states_match(self) -> None:
        fake = forge_provider.FakeProvider()
        fake.seed_issues("o/r", [{"number": 7, "title": "x", "state": "closed"}])
        self.assertEqual(
            guard.verify_tracker({"items": [{"issue": 7, "state": "closed"}]}, {"github": (fake, "o/r")}), []
        )

    def _run_cli(self, env: dict, *args: str):
        import os
        import subprocess

        base = {k: v for k, v in os.environ.items() if not k.startswith(("NOMOS_FORGE_", "GITHUB_", "GH_"))}
        base["PATH"] = "/nonexistent"
        base.update(env)
        return subprocess.run(
            [sys.executable, str(ROOT / "scripts/roadmap_lane_guard.py"), "--root", str(ROOT), *args],
            capture_output=True, text=True, check=False, env=base,
        )

    def test_verify_tracker_without_repo_names_the_variable(self) -> None:
        import json

        result = self._run_cli({"NOMOS_FORGE_PROVIDER": "fake"}, "--verify-tracker")
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        verdict = json.loads(result.stdout)
        self.assertEqual(verdict["status"], "error")
        self.assertIn("NOMOS_FORGE_REPO", verdict["failures"][0])

    def test_verify_tracker_without_provider_names_the_variable(self) -> None:
        import json

        result = self._run_cli({"NOMOS_FORGE_REPO": "o/r"}, "--verify-tracker")
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn("NOMOS_FORGE_PROVIDER", json.loads(result.stdout)["failures"][0])

    def test_verify_github_is_a_deprecated_alias(self) -> None:
        import json

        result = self._run_cli({"NOMOS_FORGE_PROVIDER": "fake", "NOMOS_FORGE_REPO": "o/r"}, "--verify-github")
        self.assertIn("deprecated", result.stderr)
        self.assertIn("--verify-tracker", result.stderr)
        # The fake tracker knows no issue: every item is unreachable, hence a failure.
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        verdict = json.loads(result.stdout)
        self.assertTrue(all("tracker unreachable" in f for f in verdict["failures"]), verdict["failures"][:3])


class TrackerQualificationTests(unittest.TestCase):
    """ADR-0006 FN-4 (#739): every identifier names its tracker.

    `#N` used to mean "GitHub issue N" by construction. With two trackers during
    the transition (GitHub, the sovereign forge) the registry must say which
    forge each number belongs to, and the verification must read each item on
    THAT forge — a missing configuration is a named failure, not a skipped item.
    """

    def test_shipped_registry_announces_1_1_0_and_a_default_tracker(self) -> None:
        data = registry()
        self.assertEqual(data["schema_version"], "1.1.0")
        self.assertEqual(data["default_tracker"], "github")
        # The worked example: the four forge-neutrality items are explicit.
        explicit = sorted(item["issue"] for item in data["items"] if "tracker" in item)
        self.assertEqual(explicit, [736, 737, 738, 739])
        self.assertTrue(all(item.get("tracker", "github") == "github" for item in data["items"]))

    def test_default_tracker_applies_to_items_without_the_field(self) -> None:
        data = registry()
        item = next(item for item in data["items"] if "tracker" not in item)
        self.assertEqual(guard.item_tracker(data, item), "github")
        data["default_tracker"] = "forgejo"
        self.assertEqual(guard.item_tracker(data, item), "forgejo")
        self.assertEqual(guard.validate(data), [])
        # A registry that says nothing at all is a GitHub registry (1.0.0 reading).
        self.assertEqual(guard.item_tracker({}, {"issue": 1}), "github")

    def test_explicit_tracker_is_validated(self) -> None:
        data = registry()
        item = next(item for item in data["items"] if "tracker" not in item)
        item["tracker"] = "forgejo"
        self.assertEqual(guard.validate(data), [])
        self.assertEqual(guard.item_tracker(data, item), "forgejo")
        self.assertEqual(sorted(guard.registry_trackers(data)), ["forgejo", "github"])

    def test_unknown_tracker_is_refused(self) -> None:
        data = registry()
        data["items"][0]["tracker"] = "bitbucket"
        data["umbrella_issues"][0]["tracker"] = "sourcehut"
        data["default_tracker"] = "jira"
        failures = guard.validate(data)
        issue = data["items"][0]["issue"]
        self.assertTrue(any(f"issue #{issue}: unknown tracker 'bitbucket'" in f for f in failures), failures)
        self.assertTrue(any("umbrella issue #545: unknown tracker 'sourcehut'" in f for f in failures), failures)
        self.assertTrue(any("default_tracker 'jira' is not one of" in f for f in failures), failures)

    def test_schema_1_0_0_is_still_read_but_cannot_carry_tracker_fields(self) -> None:
        # Backward compatibility (docs/16 §3): a 1.0.0 registry is a GitHub
        # registry. Using the 1.1.0 fields without announcing 1.1.0 is refused,
        # otherwise the version would stop meaning anything.
        data = registry()
        data["schema_version"] = "1.0.0"
        del data["default_tracker"]
        for item in data["items"]:
            item.pop("tracker", None)
        self.assertEqual(guard.validate(data), [])
        data["default_tracker"] = "github"
        failures = guard.validate(data)
        self.assertTrue(any("schema_version 1.0.0 cannot carry default_tracker" in f for f in failures), failures)
        data["schema_version"] = "2.0.0"
        failures = guard.validate(data)
        self.assertTrue(any("schema_version '2.0.0' is not one of 1.0.0, 1.1.0" in f for f in failures), failures)
        del data["schema_version"]
        failures = guard.validate(data)
        self.assertTrue(any("schema_version None is not one of" in f for f in failures), failures)

    def test_queue_table_qualifies_an_item_hosted_off_the_default_tracker(self) -> None:
        data = registry()
        lane = nonempty_lane(data)
        head = data["selection_policy"]["dispatch_queues"][lane][0]
        item = next(item for item in data["items"] if item["issue"] == head)
        item["tracker"] = "forgejo"
        table = guard.render_queue_table(data)
        self.assertIn(f"#{head} (forgejo) — ", table)
        # Items on the default tracker keep the bare `#N` — no drift in the docs.
        others = [i for i in data["selection_policy"]["dispatch_queues"][lane] if i != head]
        for issue in others:
            self.assertIn(f"#{issue} — ", table)
            self.assertNotIn(f"#{issue} (", table)

    def test_verify_tracker_reads_each_item_on_its_own_tracker(self) -> None:
        github = forge_provider.FakeProvider()
        github.seed_issues("o/nomos", [{"number": 1, "title": "gh", "state": "open"}])
        forgejo = forge_provider.FakeProvider()
        forgejo.seed_issues("rbok/nomos", [{"number": 1, "title": "forge", "state": "closed"}])
        data = {
            "schema_version": "1.1.0",
            "default_tracker": "github",
            "items": [
                {"issue": 1, "state": "open"},
                {"issue": 1, "state": "closed", "tracker": "forgejo"},
            ],
        }
        bindings = {"github": (github, "o/nomos"), "forgejo": (forgejo, "rbok/nomos")}
        self.assertEqual(guard.verify_tracker(data, bindings), [])
        # Each provider was asked exactly for its own item, in its own repository.
        self.assertEqual(github.calls, [("get_issue", {"repo": "o/nomos", "number": 1})])
        self.assertEqual(forgejo.calls, [("get_issue", {"repo": "rbok/nomos", "number": 1})])
        # The same numbers, swapped: the mismatch names the tracker.
        data["items"][0]["state"] = "closed"
        data["items"][1]["state"] = "open"
        self.assertEqual(
            guard.verify_tracker(data, bindings),
            [
                "issue #1 (github): registry says 'closed', tracker says 'open'",
                "issue #1 (forgejo): registry says 'open', tracker says 'closed'",
            ],
        )

    def test_missing_tracker_configuration_fails_by_name_never_silently(self) -> None:
        # Adversarial: the forgejo item must NOT vanish from the report because
        # nobody configured the forge; it must fail, naming the tracker and the
        # missing variable.
        github = forge_provider.FakeProvider()
        github.seed_issues("o/nomos", [{"number": 5, "title": "gh", "state": "open"}])
        data = {
            "schema_version": "1.1.0",
            "default_tracker": "github",
            "items": [
                {"issue": 5, "state": "open"},
                {"issue": 9, "state": "open", "tracker": "forgejo"},
            ],
        }
        problems = guard.verify_tracker(data, {"github": (github, "o/nomos")})
        self.assertEqual(problems, ["issue #9 (forgejo): tracker not configured"])
        error = forge_provider.ForgeConfigError("NOMOS_FORGE_FORGEJO_URL missing")
        problems = guard.verify_tracker(data, {"github": (github, "o/nomos"), "forgejo": error})
        self.assertEqual(problems, ["issue #9 (forgejo): NOMOS_FORGE_FORGEJO_URL missing"])

    def test_bind_trackers_reports_the_missing_variable_per_secondary_tracker(self) -> None:
        data = {
            "schema_version": "1.1.0",
            "default_tracker": "github",
            "items": [
                {"issue": 5, "state": "open"},
                {"issue": 9, "state": "open", "tracker": "forgejo"},
                {"issue": 3, "state": "open", "tracker": "gitlab"},
            ],
        }
        env = {"NOMOS_FORGE_PROVIDER": "github", "GITHUB_TOKEN": "ghs_x", "NOMOS_FORGE_REPO": "o/nomos"}
        bindings = guard.bind_trackers(data, env, which=lambda _: None)
        self.assertEqual(set(bindings), {"github", "forgejo", "gitlab"})
        provider, repo = bindings["github"]
        self.assertEqual((provider.name, repo), ("github", "o/nomos"))
        self.assertIsInstance(bindings["forgejo"], forge_provider.ForgeConfigError)
        self.assertEqual(str(bindings["forgejo"]), "NOMOS_FORGE_FORGEJO_REPO missing")
        self.assertEqual(str(bindings["gitlab"]), "NOMOS_FORGE_GITLAB_REPO missing")
        # Repo given, URL still absent: the next missing variable is named.
        env["NOMOS_FORGE_FORGEJO_REPO"] = "rbok/nomos"
        self.assertEqual(str(guard.bind_trackers(data, env, which=lambda _: None)["forgejo"]), "NOMOS_FORGE_FORGEJO_URL missing")
        env["NOMOS_FORGE_FORGEJO_URL"] = "https://forge.example"
        self.assertEqual(
            str(guard.bind_trackers(data, env, which=lambda _: None)["forgejo"]),
            "NOMOS_FORGE_FORGEJO_TOKEN_FILE or NOMOS_FORGE_FORGEJO_TOKEN missing",
        )
        env["NOMOS_FORGE_FORGEJO_TOKEN"] = "t"
        provider, repo = guard.bind_trackers(data, env, which=lambda _: None)["forgejo"]
        self.assertEqual((provider.name, repo), ("forgejo", "rbok/nomos"))
        # End to end: the failures carry the tracker and the variable.
        problems = guard.verify_tracker(data, guard.bind_trackers(data, env, which=lambda _: None))
        self.assertIn("issue #3 (gitlab): NOMOS_FORGE_GITLAB_REPO missing", problems)

    def test_bind_trackers_secondary_github_keeps_its_fallbacks(self) -> None:
        # Primary is the forge; GitHub items are read with GITHUB_TOKEN, or `gh`.
        data = {"schema_version": "1.1.0", "default_tracker": "forgejo",
                "items": [{"issue": 1, "state": "open"}, {"issue": 2, "state": "open", "tracker": "github"}]}
        env = {"NOMOS_FORGE_PROVIDER": "forgejo", "NOMOS_FORGE_URL": "https://forge.example",
               "NOMOS_FORGE_TOKEN": "t", "NOMOS_FORGE_REPO": "rbok/nomos"}
        bindings = guard.bind_trackers(data, env, which=lambda _: None)
        self.assertEqual(bindings["forgejo"][1], "rbok/nomos")
        self.assertEqual(str(bindings["github"]), "NOMOS_FORGE_GITHUB_REPO missing")
        env["NOMOS_FORGE_GITHUB_REPO"] = "o/nomos"
        self.assertIn("GITHUB_TOKEN, GH_TOKEN or the `gh` binary missing", str(guard.bind_trackers(data, env, which=lambda _: None)["github"]))
        env["GITHUB_TOKEN"] = "ghs_x"
        provider, repo = guard.bind_trackers(data, env, which=lambda _: None)["github"]
        self.assertEqual((provider.name, repo, provider.uses_gh), ("github", "o/nomos", False))
        del env["GITHUB_TOKEN"]
        provider, _ = guard.bind_trackers(data, env, which=lambda _: "/usr/bin/gh")["github"]
        self.assertTrue(provider.uses_gh)

    def test_bind_trackers_primary_configuration_is_still_required(self) -> None:
        data = {"schema_version": "1.1.0", "items": [{"issue": 1, "state": "open"}]}
        with self.assertRaises(forge_provider.ForgeConfigError):
            guard.bind_trackers(data, {"NOMOS_FORGE_PROVIDER": "fake"}, which=lambda _: None)
        # The fake primary serves every tracker in memory (tests only).
        data["items"].append({"issue": 2, "state": "open", "tracker": "forgejo"})
        bindings = guard.bind_trackers(data, {"NOMOS_FORGE_PROVIDER": "fake", "NOMOS_FORGE_REPO": "o/r"}, which=lambda _: None)
        self.assertEqual({k: v[1] for k, v in bindings.items()}, {"github": "o/r", "forgejo": "o/r"})
        self.assertTrue(all(v[0].name == "fake" for v in bindings.values()))

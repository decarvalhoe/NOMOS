"""Tests for regulated_release_env.py config and verification logic.

The live side is a `FakeProvider` from scripts/forge_provider.py: no `gh`
binary, no network. Doctrine §2.8: a missing provider configuration must
make the CLI fail with the variable's name, never verify nothing and pass.
"""

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

SCRIPTS = Path(__file__).resolve().parent.parent / "scripts"
sys.path.insert(0, str(SCRIPTS))

import forge_provider as fp  # noqa: E402
import regulated_release_env as re_env  # noqa: E402

REPO = "O/R"


def fake_with(live: dict | None, name: str = "regulated-release") -> fp.FakeProvider:
    fake = fp.FakeProvider()
    if live is not None:
        fake.seed_environment(REPO, name, live)
    return fake


def _clean_env(**extra: str) -> dict[str, str]:
    env = {k: v for k, v in os.environ.items() if not k.startswith(("NOMOS_FORGE_", "GITHUB_", "GH_"))}
    env["PATH"] = "/nonexistent"
    env.update(extra)
    return env


SAMPLE_CONFIG = {
    "schema_version": "0.1.0",
    "repository": {"owner": "TestOrg", "name": "TestRepo"},
    "environments": [
        {
            "name": "regulated-release",
            "protection": {
                "required_reviewers": {"users": ["alice"], "teams": []},
                "prevent_self_review": True,
                "wait_timer_minutes": 10,
            },
            "deployment_branch_policy": {"protected_branches_only": True},
            "variables": [
                {"name": "RELEASE_EVIDENCE_REQUIRED", "value": "true"},
            ],
        }
    ],
    "governance_controls": [],
}


class TestLoadConfig(unittest.TestCase):
    def test_load_valid(self):
        yaml = __import__("yaml")
        with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
            yaml.dump(SAMPLE_CONFIG, f)
            path = f.name
        config = re_env.load_config(path)
        os.unlink(path)
        self.assertEqual(config["repository"]["owner"], "TestOrg")

    def test_load_missing(self):
        with self.assertRaises(SystemExit):
            re_env.load_config("/nonexistent.yaml")

    def test_load_repo_config(self):
        config_path = Path(__file__).resolve().parent.parent / re_env.CONFIG_PATH
        if not config_path.exists():
            self.skipTest("repo config not present")
        config = re_env.load_config(str(config_path))
        self.assertIn("environments", config)
        env_names = [e["name"] for e in config["environments"]]
        self.assertIn("regulated-release", env_names)


class TestVerifyEnvironment(unittest.TestCase):
    def _env(self):
        return SAMPLE_CONFIG["environments"][0]

    def test_fully_compliant(self):
        live = {
            "protection_rules": [
                {"type": "required_reviewers", "reviewers": [
                    {"reviewer": {"login": "alice"}}
                ]},
                {"type": "wait_timer", "wait_timer": 10},
            ],
            "prevent_self_review": True,
            "deployment_branch_policy": {"protected_branches": True},
        }
        findings = re_env.verify_environment("O", "R", self._env(), fake_with(live))
        blocking = [f for f in findings if f["blocking"]]
        self.assertEqual(len(blocking), 0, f"unexpected blocking: {blocking}")

    def test_env_not_found(self):
        findings = re_env.verify_environment("O", "R", self._env(), fake_with(None))
        self.assertTrue(any(f["control"] == "ENV-EXISTS" for f in findings))
        self.assertTrue(any(f["blocking"] for f in findings))

    def test_self_review_not_prevented(self):
        live = {
            "protection_rules": [{"type": "wait_timer", "wait_timer": 10}],
            "prevent_self_review": False,
            "deployment_branch_policy": {"protected_branches": True},
        }
        findings = re_env.verify_environment("O", "R", self._env(), fake_with(live))
        self.assertTrue(any(f["control"] == "ENV-SELF-REVIEW" for f in findings))

    def test_wait_timer_too_low(self):
        live = {
            "protection_rules": [{"type": "wait_timer", "wait_timer": 0}],
            "prevent_self_review": True,
            "deployment_branch_policy": {"protected_branches": True},
        }
        findings = re_env.verify_environment("O", "R", self._env(), fake_with(live))
        self.assertTrue(any(f["control"] == "ENV-WAIT-TIMER" for f in findings))

    def test_no_reviewers_configured(self):
        live = {
            "protection_rules": [{"type": "wait_timer", "wait_timer": 10}],
            "prevent_self_review": True,
            "deployment_branch_policy": {"protected_branches": True},
        }
        findings = re_env.verify_environment("O", "R", self._env(), fake_with(live))
        self.assertTrue(any(f["control"] == "ENV-REVIEWERS" for f in findings))

    def test_missing_reviewer(self):
        live = {
            "protection_rules": [
                {"type": "required_reviewers", "reviewers": [
                    {"reviewer": {"login": "bob"}}
                ]},
                {"type": "wait_timer", "wait_timer": 10},
            ],
            "prevent_self_review": True,
            "deployment_branch_policy": {"protected_branches": True},
        }
        findings = re_env.verify_environment("O", "R", self._env(), fake_with(live))
        self.assertTrue(any(f["control"] == "ENV-REVIEWER-MISSING" for f in findings))

    def test_branch_policy_not_protected(self):
        live = {
            "protection_rules": [
                {"type": "required_reviewers", "reviewers": [
                    {"reviewer": {"login": "alice"}}
                ]},
                {"type": "wait_timer", "wait_timer": 10},
            ],
            "prevent_self_review": True,
            "deployment_branch_policy": {"protected_branches": False},
        }
        findings = re_env.verify_environment("O", "R", self._env(), fake_with(live))
        self.assertTrue(any(f["control"] == "ENV-BRANCH-POLICY" for f in findings))

    def test_empty_reviewers_advisory(self):
        live = {
            "protection_rules": [],
            "prevent_self_review": True,
            "deployment_branch_policy": {"protected_branches": True},
        }
        env = {
            "name": "staging",
            "protection": {
                "required_reviewers": {"users": [], "teams": []},
                "prevent_self_review": False,
                "wait_timer_minutes": 0,
            },
            "deployment_branch_policy": {"protected_branches_only": False},
        }
        findings = re_env.verify_environment("O", "R", env, fake_with(live, "staging"))
        self.assertTrue(any(f["control"] == "ENV-REVIEWERS-EMPTY" for f in findings))
        self.assertFalse(any(f["blocking"] for f in findings))


class TestVerifyEnvironmentProvider(unittest.TestCase):
    def test_refusal_by_the_forge_is_a_blocking_finding(self):
        fake = fake_with(None)
        fake.fail("get_environment", fp.NotSupported("forgejo: deployment environments (a GitHub concept) is not supported"))
        findings = re_env.verify_environment("O", "R", SAMPLE_CONFIG["environments"][0], fake)
        self.assertEqual(findings[0]["control"], "ENV-EXISTS")
        self.assertTrue(findings[0]["blocking"])
        self.assertIn("not supported", findings[0]["message"])
        self.assertEqual(fake.calls, [("get_environment", {"repo": REPO, "name": "regulated-release"})])


class TestApplyEnvironment(unittest.TestCase):
    def test_apply_puts_the_payload_through_the_provider(self):
        fake = fp.FakeProvider()
        ok, detail = re_env.apply_environment("O", "R", SAMPLE_CONFIG["environments"][0], fake)
        self.assertEqual((ok, detail), (True, ""))
        op, kwargs = fake.calls[0]
        self.assertEqual((op, kwargs["repo"], kwargs["name"]), ("put_environment", REPO, "regulated-release"))
        self.assertTrue(kwargs["payload"]["prevent_self_review"])
        self.assertEqual(kwargs["payload"]["wait_timer"], 10)
        self.assertEqual(kwargs["payload"]["deployment_branch_policy"], {"protected_branches": True, "custom_branch_policies": False})

    def test_refusal_is_reported_not_masked(self):
        fake = fp.FakeProvider()
        fake.fail("put_environment", fp.NotSupported("gitlab: deployment environments are not supported"))
        ok, detail = re_env.apply_environment("O", "R", SAMPLE_CONFIG["environments"][0], fake)
        self.assertFalse(ok)
        self.assertIn("not supported", detail)


class TestCLIProviderConfiguration(unittest.TestCase):
    """Adversarial: without a forge configuration the CLI must not verify nothing and pass."""

    def _run(self, env: dict[str, str], *args: str) -> subprocess.CompletedProcess:
        yaml = __import__("yaml")
        with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
            yaml.dump(SAMPLE_CONFIG, f)
            config = f.name
        try:
            return subprocess.run(
                [sys.executable, str(SCRIPTS / "regulated_release_env.py"), "--config", config, *args],
                capture_output=True, text=True, check=False, env=env,
            )
        finally:
            os.unlink(config)

    def test_missing_configuration_names_the_variable_and_fails(self):
        result = self._run(_clean_env(), "--verify")
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn("NOMOS_FORGE_PROVIDER", result.stderr)
        self.assertNotIn("ALL CHECKS PASSED", result.stdout)

    def test_fake_provider_with_no_environment_is_a_blocking_failure(self):
        result = self._run(_clean_env(NOMOS_FORGE_PROVIDER="fake"), "--verify", "--format", "json")
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        payload = json.loads(result.stdout)
        self.assertEqual(payload["findings"][0]["control"], "ENV-EXISTS")


class TestResolveField(unittest.TestCase):
    def test_nested(self):
        obj = {"protection": {"prevent_self_review": True}}
        self.assertTrue(re_env.resolve_field(obj, "protection.prevent_self_review"))

    def test_missing(self):
        self.assertIsNone(re_env.resolve_field({}, "a.b.c"))

    def test_variables_list(self):
        obj = {"variables": [{"name": "X", "value": "1"}]}
        self.assertEqual(re_env.resolve_field(obj, "variables.X"), "1")


class TestGovernanceControls(unittest.TestCase):
    def test_control_passes(self):
        config = {
            "environments": [{
                "name": "regulated-release",
                "protection": {"prevent_self_review": True, "wait_timer_minutes": 10,
                               "required_reviewers": {"users": ["a"], "teams": []}},
                "deployment_branch_policy": {"protected_branches_only": True},
                "variables": [{"name": "RELEASE_EVIDENCE_REQUIRED", "value": "true"}],
            }],
            "governance_controls": [
                {"id": "ENV-SELF-REVIEW", "field": "protection.prevent_self_review",
                 "expected": True, "severity": "critical", "blocking": True,
                 "description": "Must prevent self-review."},
            ],
        }
        findings = re_env.verify_governance_controls(config)
        self.assertEqual(len(findings), 0)

    def test_control_fails(self):
        config = {
            "environments": [{
                "name": "regulated-release",
                "protection": {"prevent_self_review": False},
            }],
            "governance_controls": [
                {"id": "ENV-SELF-REVIEW", "field": "protection.prevent_self_review",
                 "expected": True, "severity": "critical", "blocking": True,
                 "description": "Must prevent self-review."},
            ],
        }
        findings = re_env.verify_governance_controls(config)
        self.assertTrue(any(f["control"] == "ENV-SELF-REVIEW" for f in findings))


class TestFindingStructure(unittest.TestCase):
    def test_fields(self):
        findings = re_env.verify_environment("O", "R", SAMPLE_CONFIG["environments"][0], fake_with(None))
        for f in findings:
            for key in ("control", "environment", "severity", "blocking", "message", "remediation"):
                self.assertIn(key, f, f"missing {key}")


class TestPrintFindings(unittest.TestCase):
    def test_json(self):
        import io
        old = sys.stdout
        sys.stdout = buf = io.StringIO()
        re_env.print_findings([{"control": "X", "environment": "e", "blocking": True,
                                "message": "m", "remediation": "r", "severity": "high"}], "json")
        sys.stdout = old
        result = json.loads(buf.getvalue())
        self.assertEqual(result["total"], 1)

    def test_text_empty(self):
        import io
        old = sys.stdout
        sys.stdout = buf = io.StringIO()
        re_env.print_findings([], "text")
        sys.stdout = old
        self.assertIn("ALL CHECKS PASSED", buf.getvalue())


if __name__ == "__main__":
    unittest.main()

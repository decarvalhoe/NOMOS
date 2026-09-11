"""Tests for regulated_branch_protection.py config parsing and verification logic.

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

# Add scripts to path
SCRIPTS = Path(__file__).resolve().parent.parent / "scripts"
sys.path.insert(0, str(SCRIPTS))

import forge_provider as fp  # noqa: E402
import regulated_branch_protection as bp  # noqa: E402

REPO = "TestOrg/TestRepo"
COMPLIANT = {
    "required_pull_request_reviews": {
        "required_approving_review_count": 1,
        "dismiss_stale_reviews": True,
    },
    "required_status_checks": {"contexts": ["CI"], "checks": []},
    "enforce_admins": {"enabled": True},
    "allow_force_pushes": {"enabled": False},
    "allow_deletions": {"enabled": False},
    "required_linear_history": {"enabled": True},
}


def fake_with(live: dict | None) -> fp.FakeProvider:
    """A fake forge holding `live` as the protection of TestOrg/TestRepo@main (or nothing)."""
    fake = fp.FakeProvider()
    if live is not None:
        fake.seed_branch_protection(REPO, "main", live)
    return fake


def _clean_env(**extra: str) -> dict[str, str]:
    """No forge configuration at all, and no `gh` reachable on PATH."""
    env = {k: v for k, v in os.environ.items() if not k.startswith(("NOMOS_FORGE_", "GITHUB_", "GH_"))}
    env["PATH"] = "/nonexistent"
    env.update(extra)
    return env


SAMPLE_CONFIG = {
    "schema_version": "0.1.0",
    "repository": {"owner": "TestOrg", "name": "TestRepo"},
    "branch_rules": [
        {
            "branch": "main",
            "protection": {
                "require_pull_request": True,
                "required_approving_review_count": 1,
                "dismiss_stale_reviews": True,
                "require_linear_history": True,
                "allow_force_pushes": False,
                "allow_deletions": False,
                "required_status_checks": {
                    "strict": True,
                    "contexts": ["CI"],
                },
                "enforce_admins": True,
            },
        }
    ],
}


class TestLoadConfig(unittest.TestCase):
    def test_load_valid_config(self):
        yaml = __import__("yaml", fromlist=[""])
        with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
            yaml.dump(SAMPLE_CONFIG, f)
            f.flush()
            config = bp.load_config(f.name)
        os.unlink(f.name)
        self.assertEqual(config["repository"]["owner"], "TestOrg")
        self.assertEqual(len(config["branch_rules"]), 1)

    def test_load_missing_config(self):
        with self.assertRaises(SystemExit):
            bp.load_config("/nonexistent.yaml")


class TestLoadRepoConfig(unittest.TestCase):
    def test_load_actual_repo_config(self):
        config_path = Path(__file__).resolve().parent.parent / bp.CONFIG_PATH
        if not config_path.exists():
            self.skipTest("repo config not present")
        config = bp.load_config(str(config_path))
        self.assertIn("repository", config)
        self.assertIn("branch_rules", config)
        self.assertTrue(len(config["branch_rules"]) >= 1)
        main_rule = config["branch_rules"][0]
        self.assertEqual(main_rule["branch"], "main")
        prot = main_rule["protection"]
        self.assertTrue(prot["require_pull_request"])
        self.assertFalse(prot["allow_force_pushes"])
        self.assertFalse(prot["allow_deletions"])
        self.assertTrue(prot["require_linear_history"])


class TestVerifyBranch(unittest.TestCase):
    """Verify logic against a fake forge seeded with live protection payloads."""

    def _rule(self):
        return SAMPLE_CONFIG["branch_rules"][0]

    def test_fully_compliant(self):
        fake = fake_with(COMPLIANT)
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule(), fake)
        self.assertEqual(len(findings), 0)
        self.assertEqual(fake.calls, [("get_branch_protection", {"repo": REPO, "branch": "main"})])

    def test_no_protection(self):
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule(), fake_with(None))
        self.assertTrue(any(f["control"] == "BRANCH-PROTECTION-EXISTS" for f in findings))
        self.assertTrue(any(f["blocking"] for f in findings))
        # The provider's reason is carried into the finding, not dropped.
        self.assertIn("no branch protection for TestOrg/TestRepo@main", findings[0]["message"])

    def test_refusal_by_the_forge_is_a_blocking_finding(self):
        fake = fake_with(None)
        fake.fail("get_branch_protection", fp.NotSupported("gitlab: branch protection is not supported"))
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule(), fake)
        self.assertEqual(findings[0]["control"], "BRANCH-PROTECTION-EXISTS")
        self.assertIn("not supported", findings[0]["message"])

    def test_no_pr_required(self):
        live = {**COMPLIANT, "required_pull_request_reviews": None, "required_status_checks": None}
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule(), fake_with(live))
        self.assertTrue(any(f["control"] == "REQUIRE-PR" for f in findings))

    def test_force_push_allowed(self):
        live = {**COMPLIANT, "allow_force_pushes": {"enabled": True}}
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule(), fake_with(live))
        self.assertTrue(any(f["control"] == "NO-FORCE-PUSH" for f in findings))
        self.assertTrue(any(f["blocking"] for f in findings))

    def test_deletion_allowed(self):
        live = {**COMPLIANT, "allow_deletions": {"enabled": True}}
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule(), fake_with(live))
        self.assertTrue(any(f["control"] == "NO-DELETION" for f in findings))

    def test_missing_status_check(self):
        live = {**COMPLIANT, "required_status_checks": {"contexts": ["other-check"], "checks": []}}
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule(), fake_with(live))
        self.assertTrue(any(f["control"] == "STATUS-CHECK-CONTEXT" for f in findings))

    def test_insufficient_reviews(self):
        live = {
            **COMPLIANT,
            "required_pull_request_reviews": {"required_approving_review_count": 0, "dismiss_stale_reviews": True},
        }
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule(), fake_with(live))
        self.assertTrue(any(f["control"] == "REVIEW-COUNT" for f in findings))

    def test_forgejo_shaped_protection_reports_unmapped_controls(self):
        # What Forgejo cannot express (linear history) is absent from the
        # normalised payload and must surface as a finding, not a pass.
        live = fp.forgejo_protection_to_common(
            {"enable_push": False, "required_approvals": 1, "dismiss_stale_approvals": True,
             "enable_status_check": True, "status_check_contexts": ["CI"], "apply_to_admins": True}
        )
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule(), fake_with(live))
        self.assertEqual([f["control"] for f in findings], ["LINEAR-HISTORY"])
        self.assertFalse(findings[0]["blocking"])


class TestApplyBranch(unittest.TestCase):
    def test_apply_puts_the_common_payload_through_the_provider(self):
        fake = fp.FakeProvider()
        ok, detail = bp.apply_branch("TestOrg", "TestRepo", SAMPLE_CONFIG["branch_rules"][0], fake)
        self.assertEqual((ok, detail), (True, ""))
        op, kwargs = fake.calls[0]
        self.assertEqual((op, kwargs["repo"], kwargs["branch"]), ("put_branch_protection", REPO, "main"))
        payload = kwargs["payload"]
        self.assertEqual(payload["required_pull_request_reviews"]["required_approving_review_count"], 1)
        self.assertEqual(payload["required_status_checks"], {"strict": True, "contexts": ["CI"]})
        self.assertTrue(payload["enforce_admins"])
        self.assertTrue(payload["required_linear_history"])

    def test_refusal_is_reported_not_masked(self):
        fake = fp.FakeProvider()
        fake.fail("put_branch_protection", fp.NotSupported("forgejo: cannot express required_linear_history"))
        ok, detail = bp.apply_branch("TestOrg", "TestRepo", SAMPLE_CONFIG["branch_rules"][0], fake)
        self.assertFalse(ok)
        self.assertIn("required_linear_history", detail)


class TestFindingStructure(unittest.TestCase):
    def test_finding_fields(self):
        findings = bp.verify_branch("O", "R", SAMPLE_CONFIG["branch_rules"][0], fp.FakeProvider())
        for f in findings:
            self.assertIn("control", f)
            self.assertIn("branch", f)
            self.assertIn("severity", f)
            self.assertIn("blocking", f)
            self.assertIn("message", f)
            self.assertIn("remediation", f)


class TestCLIProviderConfiguration(unittest.TestCase):
    """Adversarial: without a forge configuration the CLI must not verify nothing and pass."""

    def _run(self, env: dict[str, str], *args: str) -> subprocess.CompletedProcess:
        yaml = __import__("yaml", fromlist=[""])
        with tempfile.NamedTemporaryFile(mode="w", suffix=".yaml", delete=False) as f:
            yaml.dump(SAMPLE_CONFIG, f)
            config = f.name
        try:
            return subprocess.run(
                [sys.executable, str(SCRIPTS / "regulated_branch_protection.py"), "--config", config, *args],
                capture_output=True, text=True, check=False, env=env,
            )
        finally:
            os.unlink(config)

    def test_missing_configuration_names_the_variable_and_fails(self):
        result = self._run(_clean_env(), "--verify")
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn("NOMOS_FORGE_PROVIDER", result.stderr)
        self.assertNotIn("ALL CHECKS PASSED", result.stdout)

    def test_forgejo_without_url_names_the_url_variable(self):
        result = self._run(_clean_env(NOMOS_FORGE_PROVIDER="forgejo", NOMOS_FORGE_TOKEN="t"), "--verify")
        self.assertEqual(result.returncode, 2)
        self.assertIn("NOMOS_FORGE_URL", result.stderr)

    def test_fake_provider_with_no_protection_is_a_blocking_failure(self):
        result = self._run(_clean_env(NOMOS_FORGE_PROVIDER="fake"), "--verify", "--format", "json")
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        payload = json.loads(result.stdout)
        self.assertEqual(payload["findings"][0]["control"], "BRANCH-PROTECTION-EXISTS")


class TestPrintFindings(unittest.TestCase):

    def test_json_format(self):
        import io
        old_stdout = sys.stdout
        sys.stdout = buf = io.StringIO()
        bp.print_findings([{"control": "X", "branch": "main", "blocking": True,
                           "message": "test", "remediation": "fix"}], "json")
        sys.stdout = old_stdout
        result = json.loads(buf.getvalue())
        self.assertEqual(result["total"], 1)
        self.assertEqual(result["blocking"], 1)

    def test_text_format_empty(self):
        import io
        old_stdout = sys.stdout
        sys.stdout = buf = io.StringIO()
        bp.print_findings([], "text")
        sys.stdout = old_stdout
        self.assertIn("ALL CHECKS PASSED", buf.getvalue())


class TestConfigSchema(unittest.TestCase):
    def test_main_branch_has_all_controls(self):
        yaml = __import__("yaml", fromlist=[""])
        config_path = Path(__file__).resolve().parent.parent / bp.CONFIG_PATH
        if not config_path.exists():
            self.skipTest("config not present")
        with open(config_path) as f:
            config = yaml.safe_load(f)
        main_rule = config["branch_rules"][0]
        prot = main_rule["protection"]
        self.assertIn("require_pull_request", prot)
        self.assertIn("required_approving_review_count", prot)
        self.assertIn("allow_force_pushes", prot)
        self.assertIn("allow_deletions", prot)
        self.assertIn("require_linear_history", prot)
        self.assertIn("required_status_checks", prot)
        self.assertIn("contexts", prot["required_status_checks"])


if __name__ == "__main__":
    unittest.main()

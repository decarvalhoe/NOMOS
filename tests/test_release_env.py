"""Tests for regulated_release_env.py config and verification logic."""

import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "scripts"))

import regulated_release_env as re_env  # noqa: E402


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

    @patch("regulated_release_env.gh_api")
    def test_fully_compliant(self, mock_api):
        mock_api.return_value = {
            "protection_rules": [
                {"type": "required_reviewers", "reviewers": [
                    {"reviewer": {"login": "alice"}}
                ]},
                {"type": "wait_timer", "wait_timer": 10},
            ],
            "prevent_self_review": True,
            "deployment_branch_policy": {"protected_branches": True},
        }
        findings = re_env.verify_environment("O", "R", self._env())
        blocking = [f for f in findings if f["blocking"]]
        self.assertEqual(len(blocking), 0, f"unexpected blocking: {blocking}")

    @patch("regulated_release_env.gh_api")
    def test_env_not_found(self, mock_api):
        mock_api.return_value = None
        findings = re_env.verify_environment("O", "R", self._env())
        self.assertTrue(any(f["control"] == "ENV-EXISTS" for f in findings))
        self.assertTrue(any(f["blocking"] for f in findings))

    @patch("regulated_release_env.gh_api")
    def test_self_review_not_prevented(self, mock_api):
        mock_api.return_value = {
            "protection_rules": [{"type": "wait_timer", "wait_timer": 10}],
            "prevent_self_review": False,
            "deployment_branch_policy": {"protected_branches": True},
        }
        findings = re_env.verify_environment("O", "R", self._env())
        self.assertTrue(any(f["control"] == "ENV-SELF-REVIEW" for f in findings))

    @patch("regulated_release_env.gh_api")
    def test_wait_timer_too_low(self, mock_api):
        mock_api.return_value = {
            "protection_rules": [{"type": "wait_timer", "wait_timer": 0}],
            "prevent_self_review": True,
            "deployment_branch_policy": {"protected_branches": True},
        }
        findings = re_env.verify_environment("O", "R", self._env())
        self.assertTrue(any(f["control"] == "ENV-WAIT-TIMER" for f in findings))

    @patch("regulated_release_env.gh_api")
    def test_no_reviewers_configured(self, mock_api):
        mock_api.return_value = {
            "protection_rules": [{"type": "wait_timer", "wait_timer": 10}],
            "prevent_self_review": True,
            "deployment_branch_policy": {"protected_branches": True},
        }
        findings = re_env.verify_environment("O", "R", self._env())
        self.assertTrue(any(f["control"] == "ENV-REVIEWERS" for f in findings))

    @patch("regulated_release_env.gh_api")
    def test_missing_reviewer(self, mock_api):
        mock_api.return_value = {
            "protection_rules": [
                {"type": "required_reviewers", "reviewers": [
                    {"reviewer": {"login": "bob"}}
                ]},
                {"type": "wait_timer", "wait_timer": 10},
            ],
            "prevent_self_review": True,
            "deployment_branch_policy": {"protected_branches": True},
        }
        findings = re_env.verify_environment("O", "R", self._env())
        self.assertTrue(any(f["control"] == "ENV-REVIEWER-MISSING" for f in findings))

    @patch("regulated_release_env.gh_api")
    def test_branch_policy_not_protected(self, mock_api):
        mock_api.return_value = {
            "protection_rules": [
                {"type": "required_reviewers", "reviewers": [
                    {"reviewer": {"login": "alice"}}
                ]},
                {"type": "wait_timer", "wait_timer": 10},
            ],
            "prevent_self_review": True,
            "deployment_branch_policy": {"protected_branches": False},
        }
        findings = re_env.verify_environment("O", "R", self._env())
        self.assertTrue(any(f["control"] == "ENV-BRANCH-POLICY" for f in findings))

    @patch("regulated_release_env.gh_api")
    def test_empty_reviewers_advisory(self, mock_api):
        mock_api.return_value = {
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
        findings = re_env.verify_environment("O", "R", env)
        self.assertTrue(any(f["control"] == "ENV-REVIEWERS-EMPTY" for f in findings))
        self.assertFalse(any(f["blocking"] for f in findings))


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
    @patch("regulated_release_env.gh_api")
    def test_fields(self, mock_api):
        mock_api.return_value = None
        findings = re_env.verify_environment("O", "R", SAMPLE_CONFIG["environments"][0])
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

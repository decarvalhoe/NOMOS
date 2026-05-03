"""Tests for regulated_branch_protection.py config parsing and verification logic."""

import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

# Add scripts to path
sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "scripts"))

import regulated_branch_protection as bp  # noqa: E402


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
    """Test verify logic with mocked gh api responses."""

    def _rule(self):
        return SAMPLE_CONFIG["branch_rules"][0]

    @patch("regulated_branch_protection.gh_api")
    def test_fully_compliant(self, mock_api):
        mock_api.return_value = {
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
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule())
        self.assertEqual(len(findings), 0)

    @patch("regulated_branch_protection.gh_api")
    def test_no_protection(self, mock_api):
        mock_api.return_value = None
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule())
        self.assertTrue(any(f["control"] == "BRANCH-PROTECTION-EXISTS" for f in findings))
        self.assertTrue(any(f["blocking"] for f in findings))

    @patch("regulated_branch_protection.gh_api")
    def test_no_pr_required(self, mock_api):
        mock_api.return_value = {
            "required_pull_request_reviews": None,
            "allow_force_pushes": {"enabled": False},
            "allow_deletions": {"enabled": False},
            "required_linear_history": {"enabled": True},
            "enforce_admins": {"enabled": True},
        }
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule())
        self.assertTrue(any(f["control"] == "REQUIRE-PR" for f in findings))

    @patch("regulated_branch_protection.gh_api")
    def test_force_push_allowed(self, mock_api):
        mock_api.return_value = {
            "required_pull_request_reviews": {
                "required_approving_review_count": 1,
                "dismiss_stale_reviews": True,
            },
            "allow_force_pushes": {"enabled": True},
            "allow_deletions": {"enabled": False},
            "required_linear_history": {"enabled": True},
            "enforce_admins": {"enabled": True},
            "required_status_checks": {"contexts": ["CI"]},
        }
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule())
        self.assertTrue(any(f["control"] == "NO-FORCE-PUSH" for f in findings))
        self.assertTrue(any(f["blocking"] for f in findings))

    @patch("regulated_branch_protection.gh_api")
    def test_deletion_allowed(self, mock_api):
        mock_api.return_value = {
            "required_pull_request_reviews": {
                "required_approving_review_count": 1,
                "dismiss_stale_reviews": True,
            },
            "allow_force_pushes": {"enabled": False},
            "allow_deletions": {"enabled": True},
            "required_linear_history": {"enabled": True},
            "enforce_admins": {"enabled": True},
            "required_status_checks": {"contexts": ["CI"]},
        }
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule())
        self.assertTrue(any(f["control"] == "NO-DELETION" for f in findings))

    @patch("regulated_branch_protection.gh_api")
    def test_missing_status_check(self, mock_api):
        mock_api.return_value = {
            "required_pull_request_reviews": {
                "required_approving_review_count": 1,
                "dismiss_stale_reviews": True,
            },
            "allow_force_pushes": {"enabled": False},
            "allow_deletions": {"enabled": False},
            "required_linear_history": {"enabled": True},
            "enforce_admins": {"enabled": True},
            "required_status_checks": {"contexts": ["other-check"], "checks": []},
        }
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule())
        self.assertTrue(any(f["control"] == "STATUS-CHECK-CONTEXT" for f in findings))

    @patch("regulated_branch_protection.gh_api")
    def test_insufficient_reviews(self, mock_api):
        mock_api.return_value = {
            "required_pull_request_reviews": {
                "required_approving_review_count": 0,
                "dismiss_stale_reviews": True,
            },
            "allow_force_pushes": {"enabled": False},
            "allow_deletions": {"enabled": False},
            "required_linear_history": {"enabled": True},
            "enforce_admins": {"enabled": True},
            "required_status_checks": {"contexts": ["CI"]},
        }
        findings = bp.verify_branch("TestOrg", "TestRepo", self._rule())
        self.assertTrue(any(f["control"] == "REVIEW-COUNT" for f in findings))


class TestFindingStructure(unittest.TestCase):
    @patch("regulated_branch_protection.gh_api")
    def test_finding_fields(self, mock_api):
        mock_api.return_value = None
        findings = bp.verify_branch("O", "R", SAMPLE_CONFIG["branch_rules"][0])
        for f in findings:
            self.assertIn("control", f)
            self.assertIn("branch", f)
            self.assertIn("severity", f)
            self.assertIn("blocking", f)
            self.assertIn("message", f)
            self.assertIn("remediation", f)


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

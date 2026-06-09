"""CKM-00 non-regression harness contract tests."""

from pathlib import Path
import stat
import subprocess
import unittest


ROOT = Path(__file__).resolve().parent.parent
SCRIPT = ROOT / "scripts" / "ckm-non-regression.sh"
WORKFLOW = ROOT / ".github" / "workflows" / "ckm-non-regression.yml"
PR_TEMPLATE = ROOT / ".github" / "PULL_REQUEST_TEMPLATE.md"


class TestCKMNonRegressionScript(unittest.TestCase):
    def test_script_exists_and_is_executable(self):
        self.assertTrue(SCRIPT.exists(), f"Missing script: {SCRIPT}")
        mode = SCRIPT.stat().st_mode
        if mode & stat.S_IXUSR:
            return

        result = subprocess.run(
            ["git", "ls-files", "--stage", "scripts/ckm-non-regression.sh"],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)
        self.assertTrue(
            result.stdout.startswith("100755 "),
            "Script must be executable by owner",
        )

    def test_script_runs_required_guardrails(self):
        content = SCRIPT.read_text()
        required_fragments = [
            "go build -o ../nomos .",
            "go test ./...",
            "-m unittest discover -s tests -v",
            "cue vet specs/nomos-domain-profile.cue",
            "specs/examples/nomos-domain-profile.gxp.valid.yaml",
            "specs/examples/nomos-domain-profile.ai.valid.yaml",
            "specs/examples/nomos-domain-profile.legal.valid.yaml",
            "scripts/e2e.sh",
            "scripts/rbok-runtime-e2e.sh",
            "scripts/rbok-lawbook-e2e.sh",
            "metadata remains open for CKM additive fields",
        ]
        for fragment in required_fragments:
            self.assertIn(fragment, content)


class TestCKMNonRegressionWorkflow(unittest.TestCase):
    def test_workflow_exists_and_targets_main_prs(self):
        self.assertTrue(WORKFLOW.exists(), f"Missing workflow: {WORKFLOW}")
        content = WORKFLOW.read_text()
        self.assertIn("CKM Non-Regression", content)
        self.assertIn("pull_request:", content)
        self.assertIn("branches: [main]", content)
        self.assertIn("scripts/ckm-non-regression.sh", content)

    def test_workflow_does_not_need_write_permissions(self):
        content = WORKFLOW.read_text()
        self.assertIn("contents: read", content)


class TestPullRequestTemplate(unittest.TestCase):
    def test_template_names_ckm_guardrail_for_pivot_changes(self):
        content = PR_TEMPLATE.read_text()
        self.assertIn("CKM non-regression harness", content)
        self.assertIn("metadata-first additive change considered", content)
        self.assertIn("schema_version bump + migration considered", content)


if __name__ == "__main__":
    unittest.main()

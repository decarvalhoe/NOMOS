"""FN-2 (#736) — the sidecar scripts that used to call `gh` now go through
scripts/forge_provider.py.

Covered here: the QMS audit's live reads, the issue-list and taxonomy
seeders. Each is exercised against the in-memory `FakeProvider` (no
binary, no network) and, adversarially, without any forge configuration:
the CLI must exit non-zero and name the missing variable (docs/43 §2.8),
never run a no-op and report success.
"""

from __future__ import annotations

import contextlib
import io
import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = ROOT / "scripts"
if str(SCRIPTS) not in sys.path:
    sys.path.insert(0, str(SCRIPTS))

import create_github_issue_list as issue_list  # noqa: E402
import forge_provider as fp  # noqa: E402
import regulated_github_qms_audit as qms  # noqa: E402
import sync_github_taxonomy as taxonomy  # noqa: E402

REPO = "o/r"
BACKLOG = """# Backlog

## EPIC E1 - Foundation

But :
Make the base solid.

### NOM-101 - First thing

Description :
Do the first thing.

BLOCKS :
- NOM-102

ENABLES :
- everything

DoD :
- it works

### NOM-102 - Second thing

DoD :
- also works
"""


def _clean_env(**extra: str) -> dict[str, str]:
    """No forge configuration at all, and no `gh` reachable on PATH."""
    env = {k: v for k, v in os.environ.items() if not k.startswith(("NOMOS_FORGE_", "GITHUB_", "GH_"))}
    env["PATH"] = "/nonexistent"
    env.update(extra)
    return env


def _run(script: str, env: dict[str, str], *args: str) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, str(SCRIPTS / script), *args],
        capture_output=True, text=True, check=False, env=env, cwd=ROOT,
    )


# ---------------------------------------------------------------------------
# regulated_github_qms_audit.py
# ---------------------------------------------------------------------------


class QmsAuditLiveChecksTests(unittest.TestCase):
    def test_live_checks_read_through_the_provider(self) -> None:
        security = {f: {"status": "enabled"} for f in qms.REQUIRED_SECURITY_FEATURES}
        fake = fp.FakeProvider(
            responses={
                f"/repos/{REPO}/rulesets": [{"id": 1}],
                f"/repos/{REPO}/branches/main/protection": {"enforce_admins": {"enabled": True}},
                f"/repos/{REPO}/environments": {"total_count": 1},
                f"/repos/{REPO}": {"security_and_analysis": security},
            }
        )
        checks = qms.live_checks(REPO, fake)
        self.assertEqual(checks["rulesets"], {"status": "verified", "count": 1})
        self.assertEqual(checks["branch_protection"]["status"], "verified")
        self.assertEqual(checks["branch_protection"]["branches"], ["main"])
        self.assertEqual(checks["protected_environments"]["status"], "requires_human_review")
        self.assertEqual(checks["security_features"]["status"], "verified")
        paths = [kwargs["path"] for op, kwargs in fake.calls if op == "get_json"]
        self.assertIn(f"/repos/{REPO}/branches/develop/protection", paths)  # asked, absent, recorded

    def test_a_refusing_forge_leaves_the_gaps_open(self) -> None:
        fake = fp.FakeProvider()
        fake.fail("get_json", fp.NotSupported("gitlab: generic GET is not supported"))
        checks = qms.live_checks(REPO, fake)
        for name in ("rulesets", "branch_protection", "protected_environments", "security_features"):
            self.assertEqual(checks[name]["status"], "requires_live_evidence", name)
        self.assertIn("not supported", checks["rulesets"]["api_detail"]["error"])
        self.assertEqual(checks["rulesets"]["api_detail"]["provider"], "fake")

    def test_build_report_live_without_configuration_is_a_named_error(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            from unittest import mock

            with mock.patch.dict(os.environ, _clean_env(), clear=True):
                with self.assertRaises(fp.ForgeConfigError) as ctx:
                    qms.build_report(Path(tmp), REPO, offline=False)
        self.assertIn("NOMOS_FORGE_PROVIDER", str(ctx.exception))

    def test_cli_live_without_configuration_exits_and_names_the_variable(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / "audit.json"
            result = _run(
                "regulated_github_qms_audit.py", _clean_env(),
                "--root", tmp, "--repo", REPO, "--output", str(output),
            )
            self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
            self.assertIn("NOMOS_FORGE_PROVIDER", result.stderr)
            self.assertFalse(output.exists(), "no report may be written without live evidence")

    def test_cli_live_with_fake_provider_records_missing_evidence(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            output = Path(tmp) / "audit.json"
            result = _run(
                "regulated_github_qms_audit.py", _clean_env(NOMOS_FORGE_PROVIDER="fake"),
                "--root", tmp, "--repo", REPO, "--output", str(output),
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            report = json.loads(output.read_text(encoding="utf-8"))
            self.assertFalse(report["offline"])
            self.assertEqual(report["checks"]["rulesets"]["status"], "requires_live_evidence")
            self.assertIn("error", report["checks"]["rulesets"]["api_detail"])


# ---------------------------------------------------------------------------
# create_github_issue_list.py
# ---------------------------------------------------------------------------


class IssueListTests(unittest.TestCase):
    def _backlog(self, tmp: str) -> Path:
        path = Path(tmp) / "backlog.md"
        path.write_text(BACKLOG, encoding="utf-8")
        return path

    def test_creates_missing_issues_and_skips_existing_ones(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            epics, issues = issue_list.parse_backlog(self._backlog(tmp))
        fake = fp.FakeProvider()
        fake.seed_issues(REPO, [{"number": 1, "title": "NOM-101 - First thing", "state": "open"}])
        with contextlib.redirect_stdout(io.StringIO()) as out:
            summary = issue_list.sync_issues(REPO, epics, issues, fake)
        self.assertEqual(summary, {"created": 2, "skipped": 1})
        created = [kwargs["title"] for op, kwargs in fake.calls if op == "create_issue"]
        self.assertEqual(created, ["EPIC E1 - Foundation", "NOM-102 - Second thing"])
        self.assertEqual(fake.calls[0], ("list_issues", {"repo": REPO, "state": "all"}))
        self.assertIn("SKIP NOM-101 - First thing", out.getvalue())
        # Second run: everything exists, nothing is created again.
        fake.calls.clear()
        with contextlib.redirect_stdout(io.StringIO()):
            issue_list.sync_issues(REPO, epics, issues, fake)
        self.assertEqual([op for op, _ in fake.calls], ["list_issues"])

    def test_dry_run_lists_but_creates_nothing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            backlog = self._backlog(tmp)
            fake = fp.FakeProvider()
            with contextlib.redirect_stdout(io.StringIO()) as out:
                code = issue_list.main(["--repo", REPO, "--backlog", str(backlog), "--dry-run"], provider=fake)
        self.assertEqual(code, 0)
        self.assertEqual([op for op, _ in fake.calls], ["list_issues"])
        self.assertIn("DRYRUN EPIC E1 - Foundation", out.getvalue())

    def test_cli_without_configuration_exits_and_names_the_variable(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            backlog = self._backlog(tmp)
            result = _run(
                "create_github_issue_list.py", _clean_env(),
                "--repo", REPO, "--backlog", str(backlog), "--dry-run",
            )
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn("NOMOS_FORGE_PROVIDER", result.stderr)
        self.assertNotIn("SUMMARY", result.stdout)


# ---------------------------------------------------------------------------
# sync_github_taxonomy.py
# ---------------------------------------------------------------------------


class TaxonomyTests(unittest.TestCase):
    def _seeded_fake(self) -> fp.FakeProvider:
        fake = fp.FakeProvider()
        numbers = [int(n) for n in taxonomy.EPIC_TO_LABEL] + list(taxonomy.ISSUE_CONFIG)
        fake.seed_issues(REPO, [{"number": n, "title": f"#{n}", "state": "open"} for n in numbers])
        return fake

    def test_seeds_labels_milestones_and_assignments(self) -> None:
        fake = self._seeded_fake()
        with contextlib.redirect_stdout(io.StringIO()):
            taxonomy.sync_taxonomy(REPO, fake)
        ops = [op for op, _ in fake.calls]
        self.assertEqual(ops.count("create_label"), len(taxonomy.LABELS))
        self.assertEqual(ops.count("create_milestone"), len(taxonomy.MILESTONES))
        self.assertEqual(ops.count("edit_issue"), len(taxonomy.EPIC_TO_LABEL) + len(taxonomy.ISSUE_CONFIG))
        self.assertEqual(ops.count("update_label"), 0)
        milestones = {m["title"]: m["id"] for m in fake.list_milestones(REPO)}
        epic = fake.get_issue(REPO, 1)["raw"]
        self.assertEqual(epic["labels"], ["type:epic", "status:seeded", "area:foundation"])
        self.assertEqual(epic["milestone"], milestones[taxonomy.EPIC_MILESTONE])
        started = fake.get_issue(REPO, 11)["raw"]
        self.assertIn("status:in-progress", started["labels"])
        self.assertEqual(started["milestone"], milestones["v0.1 Core Spec"])
        # Milestones are assigned by identifier, never by title.
        for op, kwargs in fake.calls:
            if op == "edit_issue":
                self.assertIsInstance(kwargs["milestone"], int)

    def test_second_run_realigns_a_drifted_label_only(self) -> None:
        fake = self._seeded_fake()
        with contextlib.redirect_stdout(io.StringIO()):
            taxonomy.sync_taxonomy(REPO, fake)
        fake.update_label(REPO, "type:epic", color="000000")
        fake.calls.clear()
        with contextlib.redirect_stdout(io.StringIO()):
            taxonomy.sync_taxonomy(REPO, fake)
        ops = [op for op, _ in fake.calls]
        self.assertEqual(ops.count("create_label"), 0)
        self.assertEqual(ops.count("create_milestone"), 0)
        updates = [kwargs for op, kwargs in fake.calls if op == "update_label"]
        self.assertEqual(len(updates), 1)
        self.assertEqual((updates[0]["name"], updates[0]["color"]), ("type:epic", "5319E7"))

    def test_gitlab_refusal_is_reported_not_masked(self) -> None:
        provider = fp.GitLabProvider("https://gitlab.example", "t", opener=lambda *a, **k: self.fail("network"))
        stderr = io.StringIO()
        with contextlib.redirect_stderr(stderr), contextlib.redirect_stdout(io.StringIO()):
            code = taxonomy.main(["--repo", REPO], provider=provider)
        self.assertEqual(code, 2)
        self.assertIn("not supported", stderr.getvalue())

    def test_cli_without_configuration_exits_and_names_the_variable(self) -> None:
        result = _run("sync_github_taxonomy.py", _clean_env(), "--repo", REPO)
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn("NOMOS_FORGE_PROVIDER", result.stderr)
        self.assertNotIn("completed", result.stdout)


if __name__ == "__main__":
    unittest.main()

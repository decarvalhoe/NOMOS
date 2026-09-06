"""NRT-025 (#678) — the security process is executable, and the proofs are the
refusals (doctrine §2.3):

* the real tree passes the static checks (process, expiring allowlist,
  generated Supported Versions section);
* a hand-edited Supported Versions section is drift, red;
* an allowlist entry without expiry, an expired one, or one valid for too long
  is red; a valid one is what lets a named finding through, for a bounded time;
* a process file missing a triage target is red;
* a known-vulnerable Go dependency pinned in a fixture module that CALLS the
  vulnerable symbol turns govulncheck red (GO-2021-0113), an unexpired
  allowlist entry lets it through, an expired one does not;
* a known-vulnerable Python pin turns pip-audit red (PYSEC-2021-142);
* a requested scanner that is missing is a failed check, never a skip;
* (#696) every dependency manifest tracked by git is scanned, watched by
  Dependabot, or excluded by name with a reason: a forgotten manifest is red,
  a stale or unjustified exclusion is red, a manifest inside a hidden cache is
  not ours, and the real tree names how each of its manifests is covered.
"""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "security_process_gate.py"
PROCESS = ROOT / "docs/security/security-process.yaml"
ALLOWLIST = ROOT / "docs/security/vulnerability-allowlist.yaml"
FIXTURE_GO = ROOT / "tests/fixtures/security/vulnerable-go-module"
FIXTURE_PY = ROOT / "tests/fixtures/security/vulnerable-requirements.txt"
TODAY = "2026-09-06"

sys.path.insert(0, str(ROOT / "scripts"))
import security_process_gate as gate  # noqa: E402


def run_gate(root: Path, *extra: str) -> tuple[int, dict, str]:
    proc = subprocess.run(
        [sys.executable, str(SCRIPT), "--root", str(root), "--today", TODAY, "--check", *extra],
        cwd=root,
        text=True,
        capture_output=True,
        check=False,
    )
    verdict = json.loads(proc.stdout) if proc.stdout.strip().startswith("{") else {}
    return proc.returncode, verdict, proc.stderr


def checks_by_name(verdict: dict) -> dict[str, dict]:
    return {c["name"]: c for c in verdict.get("checks", [])}


def copy_security_tree(target: Path) -> None:
    """A minimal checkout: everything the process file references."""
    process = yaml.safe_load(PROCESS.read_text(encoding="utf-8"))
    files = ["SECURITY.md", "CHANGELOG.md", ".github/dependabot.yml", "scripts/security_process_gate.py"]
    files += [process["supported_versions"]["changelog_ref"], process["scanners"]["pip_audit"]["requirements"], process["allowlist"]["path"]]
    files += list(process["sops"].values())
    files.append("docs/security/security-process.yaml")
    model_ref = process["supported_versions"].get("support_model_ref")
    if model_ref and (ROOT / model_ref).is_file():
        files.append(model_ref)
    # #696: the excluded manifests travel with the tree, otherwise every
    # exclusion would be stale (and red) in the copy.
    files += [item["path"] for item in (process.get("manifests") or {}).get("not_scanned") or []]
    for rel in sorted(set(files)):
        dest = target / rel
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / rel, dest)
    for module in process["scanners"]["govulncheck"]["modules"]:
        dest = target / module
        dest.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / module / "go.mod", dest / "go.mod")


def allowlist_with(entries: list[dict]) -> str:
    return yaml.safe_dump({"schema_version": gate.ALLOWLIST_SCHEMA, "claim_boundary": "test", "entries": entries}, sort_keys=False)


def entry(advisory: str, ecosystem: str, **overrides) -> dict:
    base = {
        "id": advisory,
        "ecosystem": ecosystem,
        "package": "fixture",
        "justification": "fixture acceptance for the gate tests, bounded in time",
        "owner": "tests",
        "accepted_on": "2026-09-01",
        "expires_on": "2026-10-01",
    }
    base.update(overrides)
    return base


class StaticChecksTests(unittest.TestCase):
    def test_real_tree_passes_the_static_checks(self) -> None:
        code, verdict, stderr = run_gate(ROOT)
        self.assertEqual(code, 0, stderr)
        self.assertEqual(verdict["status"], "pass")
        checks = checks_by_name(verdict)
        for name in ("process", "allowlist", "supported_versions"):
            self.assertEqual(checks[name]["status"], "pass", checks[name])
        self.assertEqual(verdict["scanners_requested"], [])

    def test_hand_edited_supported_versions_is_drift(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_security_tree(root)
            security = root / "SECURITY.md"
            text = security.read_text(encoding="utf-8")
            self.assertIn("| `v0.2.0-ALPHA` |", text)
            self.assertIn("best-effort alpha triage (current release)", text)
            security.write_text(text.replace("best-effort alpha triage (current release)", "fully supported with an SLA"), encoding="utf-8")
            code, verdict, stderr = run_gate(root)
            self.assertEqual(code, 1, stderr)
            drift = checks_by_name(verdict)["supported_versions"]
            self.assertEqual(drift["status"], "fail")
            self.assertTrue(any("drifted" in p for p in drift["problems"]), drift["problems"])
            # --write repairs it from the source, then the check passes again.
            code, verdict, stderr = run_gate(root, "--write")
            self.assertEqual(code, 0, stderr)
            self.assertIn("best-effort alpha triage (current release)", security.read_text(encoding="utf-8"))

    def test_supported_versions_follow_the_support_model(self) -> None:
        # NRT-026 (#679): the model is the source; a new version declared there
        # invalidates the rendered table until --write regenerates it.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_security_tree(root)
            process = yaml.safe_load((root / "docs/security/security-process.yaml").read_text(encoding="utf-8"))
            self.assertEqual(process["supported_versions"]["source"], "support_model")
            model_path = root / process["supported_versions"]["support_model_ref"]
            model = yaml.safe_load(model_path.read_text(encoding="utf-8"))
            model["supported_versions"][0]["state"] = "superseded"
            model["supported_versions"].insert(0, {"version": "v0.3.0-ALPHA", "released_on": "2026-12-01", "state": "supported", "security_support": "best-effort alpha triage (current release)", "end_of_support": "until the next tagged release"})
            model_path.write_text(yaml.safe_dump(model, sort_keys=False, allow_unicode=True), encoding="utf-8")
            code, verdict, _ = run_gate(root)
            self.assertEqual(code, 1, "a new version in the support model must invalidate the rendered section")
            code, verdict, stderr = run_gate(root, "--write")
            self.assertEqual(code, 0, stderr)
            text = (root / "SECURITY.md").read_text(encoding="utf-8")
            self.assertIn("| `v0.3.0-ALPHA` | 2026-12-01 | supported | best-effort alpha triage (current release) |", text)
            self.assertIn("| `v0.2.0-ALPHA` | 2026-09-06 | superseded |", text)

    def test_changelog_source_renders_when_no_support_model_is_declared(self) -> None:
        # The fallback path (source: changelog) still renders from the release headings.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_security_tree(root)
            process_path = root / "docs/security/security-process.yaml"
            process_path.write_text(process_path.read_text(encoding="utf-8").replace("source: support_model", "source: changelog", 1), encoding="utf-8")
            changelog = root / "CHANGELOG.md"
            changelog.write_text("## v0.3.0-ALPHA - 2026-12-01\n\n- next\n\n" + changelog.read_text(encoding="utf-8"), encoding="utf-8")
            code, verdict, stderr = run_gate(root, "--write")
            self.assertEqual(code, 0, stderr)
            text = (root / "SECURITY.md").read_text(encoding="utf-8")
            self.assertIn("| `v0.3.0-ALPHA` | 2026-12-01 | Supported — best-effort alpha triage (current release) |", text)
            self.assertIn("| `v0.2.0-ALPHA` | 2026-09-06 | Superseded — not supported |", text)

    def test_missing_triage_target_is_refused(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_security_tree(root)
            process_path = root / "docs/security/security-process.yaml"
            process = yaml.safe_load(process_path.read_text(encoding="utf-8"))
            process["triage"]["targets"] = [t for t in process["triage"]["targets"] if t["severity"] != "critical"]
            process_path.write_text(yaml.safe_dump(process, sort_keys=False), encoding="utf-8")
            code, verdict, stderr = run_gate(root)
            self.assertEqual(code, 1, stderr)
            problems = checks_by_name(verdict)["process"]["problems"]
            self.assertTrue(any("missing severity target(s): critical" in p for p in problems), problems)

    def test_unpinned_requirement_is_refused(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_security_tree(root)
            (root / "scripts/requirements-sidecar.txt").write_text("PyYAML>=6\n", encoding="utf-8")
            code, verdict, _ = run_gate(root)
            self.assertEqual(code, 1)
            problems = checks_by_name(verdict)["process"]["problems"]
            self.assertTrue(any("not pinned" in p for p in problems), problems)


class AllowlistTests(unittest.TestCase):
    def _run_with_allowlist(self, entries: list[dict]) -> tuple[int, dict]:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "allowlist.yaml"
            path.write_text(allowlist_with(entries), encoding="utf-8")
            code, verdict, _ = run_gate(ROOT, "--allowlist", str(path))
        return code, verdict

    def test_undated_entry_is_refused(self) -> None:
        item = entry("GO-2021-0113", "go")
        del item["expires_on"]
        code, verdict = self._run_with_allowlist([item])
        self.assertEqual(code, 1)
        problems = checks_by_name(verdict)["allowlist"]["problems"]
        self.assertTrue(any("expires_on" in p and "required" in p for p in problems), problems)

    def test_expired_entry_is_refused(self) -> None:
        code, verdict = self._run_with_allowlist([entry("GO-2021-0113", "go", accepted_on="2026-01-01", expires_on="2026-03-01")])
        self.assertEqual(code, 1)
        problems = checks_by_name(verdict)["allowlist"]["problems"]
        self.assertTrue(any("expired on 2026-03-01" in p for p in problems), problems)

    def test_over_long_validity_is_refused(self) -> None:
        code, verdict = self._run_with_allowlist([entry("GO-2021-0113", "go", accepted_on="2026-09-01", expires_on="2027-09-01")])
        self.assertEqual(code, 1)
        problems = checks_by_name(verdict)["allowlist"]["problems"]
        self.assertTrue(any("exceeds max_validity_days" in p for p in problems), problems)

    def test_valid_entry_is_active(self) -> None:
        code, verdict = self._run_with_allowlist([entry("GO-2021-0113", "go")])
        self.assertEqual(code, 0)
        self.assertEqual(checks_by_name(verdict)["allowlist"]["detail"]["active"], ["GO-2021-0113"])

    def test_duplicate_and_thin_justification_are_refused(self) -> None:
        code, verdict = self._run_with_allowlist([entry("GO-2021-0113", "go"), entry("GO-2021-0113", "go", justification="meh")])
        self.assertEqual(code, 1)
        problems = checks_by_name(verdict)["allowlist"]["problems"]
        self.assertTrue(any("declared twice" in p for p in problems), problems)
        self.assertTrue(any("justification" in p for p in problems), problems)


class ScannerAvailabilityTests(unittest.TestCase):
    def test_a_missing_requested_scanner_is_a_failed_check(self) -> None:
        code, verdict, _ = run_gate(ROOT, "--scan", "govulncheck", "--govulncheck-cmd", str(ROOT / "no-such-govulncheck"), "--go-module", str(FIXTURE_GO.relative_to(ROOT)))
        self.assertEqual(code, 1)
        problems = checks_by_name(verdict)["govulncheck"]["problems"]
        self.assertTrue(problems and "could not run" in problems[0] or "not available" in problems[0], problems)


@unittest.skipIf(gate.resolve_govulncheck(None) is None or shutil.which("go") is None, "govulncheck/go not available — the Go scanner proof needs the real tool")
class GovulncheckTests(unittest.TestCase):
    module = str(FIXTURE_GO.relative_to(ROOT))

    def test_called_vulnerable_dependency_turns_the_gate_red(self) -> None:
        code, verdict, stderr = run_gate(ROOT, "--scan", "govulncheck", "--go-module", self.module)
        self.assertEqual(code, 1, stderr)
        result = checks_by_name(verdict)["govulncheck"]
        self.assertEqual(result["status"], "fail")
        self.assertTrue(any("GO-2021-0113" in p and "called via" in p for p in result["problems"]), result["problems"])
        module_result = result["detail"]["modules"][0]
        self.assertIn("GO-2021-0113", module_result["called"])
        self.assertTrue(module_result["not_called"], "required-but-not-called findings are reported, never hidden")

    def test_unexpired_allowlist_entry_lets_the_named_finding_through(self) -> None:
        code, _, _ = run_gate(ROOT, "--scan", "govulncheck", "--go-module", self.module)
        self.assertEqual(code, 1, "precondition: the fixture is red without an allowlist")
        first = run_gate(ROOT, "--scan", "govulncheck", "--go-module", self.module)[1]
        called = checks_by_name(first)["govulncheck"]["detail"]["modules"][0]["called"]
        entries = [entry(advisory, "go", package="golang.org/x/text") for advisory in called]
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "allowlist.yaml"
            path.write_text(allowlist_with(entries), encoding="utf-8")
            code, verdict, stderr = run_gate(ROOT, "--scan", "govulncheck", "--go-module", self.module, "--allowlist", str(path))
            self.assertEqual(code, 0, stderr)
            findings = checks_by_name(verdict)["govulncheck"]["detail"]["modules"][0]["findings"]
            self.assertTrue(all(f.get("allowlisted_until") for f in findings if f["called"]))
            # Expired: the same entries a year earlier no longer protect anything.
            path.write_text(allowlist_with([dict(e, accepted_on="2025-01-01", expires_on="2025-03-01") for e in entries]), encoding="utf-8")
            code, verdict, _ = run_gate(ROOT, "--scan", "govulncheck", "--go-module", self.module, "--allowlist", str(path))
            self.assertEqual(code, 1)
            self.assertEqual(checks_by_name(verdict)["allowlist"]["status"], "fail")
            self.assertEqual(checks_by_name(verdict)["govulncheck"]["status"], "fail")


@unittest.skipIf(gate.resolve_pip_audit(None) is None, "pip-audit not available — the Python scanner proof needs the real tool")
class PipAuditTests(unittest.TestCase):
    def test_known_vulnerable_pin_turns_the_gate_red(self) -> None:
        code, verdict, stderr = run_gate(ROOT, "--scan", "pip-audit", "--requirements", str(FIXTURE_PY))
        self.assertEqual(code, 1, stderr)
        result = checks_by_name(verdict)["pip-audit"]
        self.assertTrue(any("pyyaml==5.3.1" in p.lower() and ("PYSEC-2021-142" in p or "CVE-2020-14343" in p) for p in result["problems"]), result["problems"])

    def test_declared_pins_are_clean_or_allowlisted(self) -> None:
        code, verdict, stderr = run_gate(ROOT, "--scan", "pip-audit")
        self.assertEqual(code, 0, stderr)
        self.assertEqual(checks_by_name(verdict)["pip-audit"]["status"], "pass")


class ManifestCoverageTests(unittest.TestCase):
    """#696 — a manifest nobody scans is a manifest nobody sees: every tracked
    dependency manifest is scanned, watched by Dependabot, or excluded by name."""

    def test_real_tree_manifests_are_all_covered(self) -> None:
        code, verdict, stderr = run_gate(ROOT)
        self.assertEqual(code, 0, stderr)
        result = checks_by_name(verdict)["manifests"]
        self.assertEqual(result["status"], "pass", result["problems"])
        self.assertEqual(result["detail"]["source"], "git ls-files")
        self.assertEqual(result["detail"]["uncovered"], [])
        covered = {m["path"]: m["covered_by"] for m in result["detail"]["manifests"]}
        self.assertEqual(covered.get("adapters/node-typescript/fixtures/nextjs-api-ui/package.json"), "dependabot", "the fixture GitHub held 8 advisories on is watched, not forgotten")
        self.assertEqual(covered.get("cli/go.mod"), "scanner")
        self.assertEqual(covered.get("scripts/requirements-sidecar.txt"), "scanner")
        self.assertEqual(covered.get("tests/fixtures/security/vulnerable-go-module/go.mod"), "not_scanned")

    def test_unscanned_manifest_is_red(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_security_tree(root)
            forgotten = root / "tools" / "forgotten" / "package.json"
            forgotten.parent.mkdir(parents=True)
            forgotten.write_text("{}\n", encoding="utf-8")
            code, verdict, stderr = run_gate(root)
            self.assertEqual(code, 1, stderr)
            result = checks_by_name(verdict)["manifests"]
            self.assertEqual(result["detail"]["source"], "walk")
            self.assertEqual(result["detail"]["uncovered"], ["tools/forgotten/package.json"])
            self.assertTrue(any(p.startswith("tools/forgotten/package.json: neither scanned") for p in result["problems"]), result["problems"])

    def test_manifest_inside_a_hidden_cache_is_not_ours(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_security_tree(root)
            cached = root / ".tools" / "cache" / "gomod" / "x@v1" / "go.mod"
            cached.parent.mkdir(parents=True)
            cached.write_text("module x\n", encoding="utf-8")
            code, verdict, stderr = run_gate(root)
            self.assertEqual(code, 0, stderr)
            paths = [m["path"] for m in checks_by_name(verdict)["manifests"]["detail"]["manifests"]]
            self.assertNotIn(".tools/cache/gomod/x@v1/go.mod", paths)

    def test_stale_or_unjustified_exclusion_is_red(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_security_tree(root)
            process_path = root / "docs/security/security-process.yaml"
            process = yaml.safe_load(process_path.read_text(encoding="utf-8"))
            entries = process["manifests"]["not_scanned"]
            orphaned = entries[0]["path"]
            entries[0]["path"] = "adapters/gone/pyproject.toml"
            entries[1]["reason"] = "n/a"
            process_path.write_text(yaml.safe_dump(process, sort_keys=False, allow_unicode=True), encoding="utf-8")
            code, verdict, stderr = run_gate(root)
            self.assertEqual(code, 1, stderr)
            problems = checks_by_name(verdict)["manifests"]["problems"]
            self.assertTrue(any("adapters/gone/pyproject.toml does not exist" in p for p in problems), problems)
            self.assertTrue(any("reason must say why" in p for p in problems), problems)
            self.assertTrue(any(p.startswith(orphaned + ": neither scanned") for p in problems), "the manifest whose exclusion went stale is uncovered, and named")


if __name__ == "__main__":
    unittest.main()

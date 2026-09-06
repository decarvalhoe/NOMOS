"""NRT-026 (#679) — the support model is declared AND checked; the proofs are
the refusals (doctrine §2.3):

* the real tree passes: runners equal the CI matrix, Go versions equal
  cli/go.mod, every declared version is a tag or the candidate, every tag is
  declared, dates match the changelog, generated sections carry no hand edit;
* editing the CI matrix, go.mod or the tag set without updating the model is
  red; a version that is neither a tag nor the candidate is red; a candidate
  that differs from the CLI constant is red;
* a hand-edited Support section is drift, red; --write repairs it;
* a model that forgets to declare the hosted service, the control plane or the
  GitHub App unsupported is red;
* the security gate renders SECURITY.md's Supported Versions from this model.
"""

from __future__ import annotations

import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "support_model_guard.py"
SECURITY_GATE = ROOT / "scripts" / "security_process_gate.py"
MODEL = ROOT / "docs/support-model.yaml"
CI = ROOT / ".github/workflows/ci.yml"
GO_MOD = ROOT / "cli/go.mod"
REAL_TAGS = "v0.1.0-ALPHA,v0.2.0-ALPHA,v1.0.0-BETA.1"


sys.path.insert(0, str(ROOT / "scripts"))
import support_model_guard as guard  # noqa: E402

gate_tags = guard.git_tags


def current_branch() -> str:
    proc = subprocess.run(["git", "-C", str(ROOT), "rev-parse", "--abbrev-ref", "HEAD"], text=True, capture_output=True, check=False)
    return proc.stdout.strip() or "HEAD"


def run_guard(root: Path, *extra: str) -> tuple[int, dict, str]:
    proc = subprocess.run([sys.executable, str(SCRIPT), "--root", str(root), "--check", *extra], cwd=root, text=True, capture_output=True, check=False)
    verdict = json.loads(proc.stdout) if proc.stdout.strip().startswith("{") else {}
    return proc.returncode, verdict, proc.stderr


def checks_by_name(verdict: dict) -> dict[str, dict]:
    return {c["name"]: c for c in verdict.get("checks", [])}


def load_model() -> dict:
    return yaml.safe_load(MODEL.read_text(encoding="utf-8"))


def copy_tree(target: Path) -> None:
    model = load_model()
    files = ["docs/support-model.yaml", ".github/workflows/ci.yml", "cli/go.mod", "CHANGELOG.md", "cli/internal/app/app.go"] + list(model["rendered_in"])
    # Files the model points at (a response target defined elsewhere) must exist in the copy too.
    files += [t["defined_in"] for t in model["response_targets"]["targets"] if t.get("defined_in")]
    # NRT-035: the support surface is checked against the contract registry and the guides.
    files += ["specs/contract-registry.yaml"] + list((model.get("support_surface") or {}).get("guides") or [])
    for rel in files:
        dest = target / rel
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / rel, dest)


def write_model(root: Path, model: dict) -> None:
    (root / "docs/support-model.yaml").write_text(yaml.safe_dump(model, sort_keys=False, allow_unicode=True), encoding="utf-8")


class RealTreeTests(unittest.TestCase):
    def test_real_tree_is_consistent(self) -> None:
        code, verdict, stderr = run_guard(ROOT)
        self.assertEqual(code, 0, stderr)
        checks = checks_by_name(verdict)
        for name in ("model", "platforms", "toolchain", "versions", "rendered"):
            self.assertEqual(checks[name]["status"], "pass", checks[name])
        self.assertIn("v0.2.0-ALPHA", checks["versions"]["detail"]["tags"])
        self.assertEqual(checks["versions"]["detail"]["candidate"], "v1.0.0-BETA.1")

    def test_security_gate_renders_supported_versions_from_the_model(self) -> None:
        process = yaml.safe_load((ROOT / "docs/security/security-process.yaml").read_text(encoding="utf-8"))
        self.assertEqual(process["supported_versions"]["source"], "support_model")
        proc = subprocess.run([sys.executable, str(SECURITY_GATE), "--root", str(ROOT), "--check"], cwd=ROOT, text=True, capture_output=True, check=False)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertIn("GENERATED from docs/support-model.yaml", (ROOT / "SECURITY.md").read_text(encoding="utf-8"))


class DriftTests(unittest.TestCase):
    def test_ci_matrix_edited_without_the_model_is_red(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_tree(root)
            ci = root / ".github/workflows/ci.yml"
            text = ci.read_text(encoding="utf-8")
            self.assertIn("os: [ubuntu-latest, macos-latest, windows-latest]", text)
            ci.write_text(text.replace("os: [ubuntu-latest, macos-latest, windows-latest]", "os: [ubuntu-latest, macos-latest]"), encoding="utf-8")
            code, verdict, stderr = run_guard(root, "--tags", REAL_TAGS)
            self.assertEqual(code, 1, stderr)
            platforms = checks_by_name(verdict)["platforms"]
            self.assertEqual(platforms["status"], "fail")
            self.assertTrue(any("differ from the CI matrix" in p for p in platforms["problems"]), platforms["problems"])

    def test_go_mod_edited_without_the_model_is_red(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_tree(root)
            go_mod = root / "cli/go.mod"
            go_mod.write_text(go_mod.read_text(encoding="utf-8").replace("toolchain go1.26.6", "toolchain go1.27.0"), encoding="utf-8")
            code, verdict, _ = run_guard(root, "--tags", REAL_TAGS)
            self.assertEqual(code, 1)
            problems = checks_by_name(verdict)["toolchain"]["problems"]
            self.assertTrue(any("toolchain_version" in p for p in problems), problems)
            go_mod.write_text(go_mod.read_text(encoding="utf-8").replace("go 1.24.1", "go 1.25.0"), encoding="utf-8")
            code, verdict, _ = run_guard(root, "--tags", REAL_TAGS)
            problems = checks_by_name(verdict)["toolchain"]["problems"]
            self.assertTrue(any("language_version" in p for p in problems), problems)

    def test_new_tag_not_declared_is_red(self) -> None:
        code, verdict, _ = run_guard(ROOT, "--tags", REAL_TAGS + ",v0.3.0-ALPHA")
        self.assertEqual(code, 1)
        problems = checks_by_name(verdict)["versions"]["problems"]
        self.assertTrue(any("tag v0.3.0-ALPHA exists but is not declared" in p for p in problems), problems)

    def test_declared_version_that_is_neither_tag_nor_candidate_is_red(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_tree(root)
            model = load_model()
            model["supported_versions"].append({"version": "v9.9.9", "released_on": "2027-01-01", "state": "supported", "security_support": "none", "end_of_support": "never"})
            write_model(root, model)
            code, verdict, _ = run_guard(root, "--tags", REAL_TAGS, "--write")
            self.assertEqual(code, 1)
            problems = checks_by_name(verdict)["versions"]["problems"]
            self.assertTrue(any("v9.9.9: neither a git tag nor the current candidate" in p for p in problems), problems)

    def test_candidate_must_equal_the_cli_constant(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_tree(root)
            model = load_model()
            model["current_candidate"] = "v0.3.0-ALPHA"
            write_model(root, model)
            code, verdict, _ = run_guard(root, "--tags", REAL_TAGS, "--write")
            self.assertEqual(code, 1)
            problems = checks_by_name(verdict)["versions"]["problems"]
            self.assertTrue(any("current_candidate 'v0.3.0-ALPHA' differs from the CLI Version constant" in p for p in problems), problems)

    def test_released_on_must_match_the_changelog(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_tree(root)
            model = load_model()
            alpha = next(v for v in model["supported_versions"] if v["version"] == "v0.2.0-ALPHA")
            alpha["released_on"] = "2026-12-24"
            write_model(root, model)
            code, verdict, _ = run_guard(root, "--tags", REAL_TAGS, "--write")
            self.assertEqual(code, 1)
            problems = checks_by_name(verdict)["versions"]["problems"]
            self.assertTrue(any("differs from the CHANGELOG.md heading date 2026-09-06" in p for p in problems), problems)

    def test_hand_edited_support_section_is_drift(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_tree(root)
            readme = root / "README.md"
            text = readme.read_text(encoding="utf-8")
            self.assertIn("Tested platforms (CI matrix):", text)
            readme.write_text(text.replace("Tested platforms (CI matrix):", "Tested platforms (all of them):"), encoding="utf-8")
            code, verdict, _ = run_guard(root, "--tags", REAL_TAGS)
            self.assertEqual(code, 1)
            rendered = checks_by_name(verdict)["rendered"]
            self.assertTrue(any("README.md" in p and "drifted" in p for p in rendered["problems"]), rendered["problems"])
            code, verdict, stderr = run_guard(root, "--tags", REAL_TAGS, "--write")
            self.assertEqual(code, 0, stderr)
            self.assertIn("Tested platforms (CI matrix):", readme.read_text(encoding="utf-8"))

    def test_missing_mandatory_unsupported_surface_is_red(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_tree(root)
            model = load_model()
            model["unsupported_surfaces"] = [s for s in model["unsupported_surfaces"] if "control plane" not in s["surface"].lower()]
            write_model(root, model)
            code, verdict, _ = run_guard(root, "--tags", REAL_TAGS, "--write")
            self.assertEqual(code, 1)
            problems = checks_by_name(verdict)["model"]["problems"]
            self.assertTrue(any("'control plane' unsupported" in p for p in problems), problems)

    def test_shallow_tagless_checkout_reads_the_tags_from_origin(self) -> None:
        # CI checkouts are shallow and carry no tags: the guard must still see
        # the real tag set (from origin) instead of declaring every version a ghost.
        source_tags = subprocess.run(["git", "-C", str(ROOT), "tag", "-l", "v*"], text=True, capture_output=True, check=False).stdout.split()
        if not source_tags:
            # The fallback reads `origin`; a source checkout that carries no tag
            # itself (a shallow CI checkout) cannot serve as that origin. The
            # security job fetches the tags before running this module for real.
            self.skipTest("source checkout carries no tag — the origin fallback cannot be proven from it")
        with tempfile.TemporaryDirectory() as tmp:
            clone = Path(tmp) / "shallow"
            proc = subprocess.run(["git", "clone", "-q", "--depth", "1", "--no-tags", "--branch", current_branch(), f"file://{ROOT}", str(clone)], text=True, capture_output=True, check=False)
            if proc.returncode != 0:
                self.skipTest(f"shallow clone unavailable: {proc.stderr.strip()[:200]}")
            local_tags = subprocess.run(["git", "-C", str(clone), "tag", "-l"], text=True, capture_output=True, check=False).stdout.split()
            self.assertEqual(local_tags, [], "precondition: the shallow clone carries no tag")
            # The expected set is the SOURCE checkout's tags, not a literal: every release adds one.
            self.assertEqual(sorted(gate_tags(clone)), sorted(source_tags))

    def test_measured_targets_without_record_are_refused(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_tree(root)
            model = load_model()
            model["response_targets"]["status"] = "measured"
            write_model(root, model)
            code, verdict, _ = run_guard(root, "--tags", REAL_TAGS, "--write")
            self.assertEqual(code, 1)
            problems = checks_by_name(verdict)["model"]["problems"]
            self.assertTrue(any("declared_not_measured" in p for p in problems), problems)


if __name__ == "__main__":
    unittest.main()


class SurfaceTests(unittest.TestCase):
    """NRT-035 (#718): the beta says what it supports, and the guard refuses what contradicts it."""

    def _surface_problems(self, root: Path) -> list[str]:
        code, verdict, _ = run_guard(root, "--tags", REAL_TAGS, "--write")
        self.assertEqual(code, 1)
        return checks_by_name(verdict)["surface"]["problems"]

    def test_forgotten_stable_contract_is_red(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_tree(root)
            model = load_model()
            model["support_surface"]["contracts"]["stable"].remove("nomos-project")
            write_model(root, model)
            problems = self._surface_problems(root)
            self.assertTrue(any("stable contract nomos-project is missing from support_surface" in p for p in problems), problems)

    def test_surface_cannot_claim_more_than_the_registry(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_tree(root)
            model = load_model()
            model["support_surface"]["contracts"]["stable"].append("adapter-manifest")
            write_model(root, model)
            problems = self._surface_problems(root)
            self.assertTrue(any("lists adapter-manifest as supported but the registry says experimental" in p for p in problems), problems)

    def test_guide_relying_on_an_out_of_surface_contract_without_saying_so_is_red(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_tree(root)
            guide = root / "docs/48-customer-integration-guide.md"
            text = guide.read_text(encoding="utf-8").replace("| `adapter-manifest` | experimental |", "| `adapter-manifest` | stable |", 1)
            guide.write_text(text, encoding="utf-8")
            problems = self._surface_problems(root)
            self.assertTrue(any("adapter-manifest is neither in support_surface.contracts.stable nor marked experimental" in p for p in problems), problems)

    def test_guide_replaying_an_uncovered_command_is_red(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            copy_tree(root)
            model = load_model()
            model["support_surface"]["commands"] = [c for c in model["support_surface"]["commands"] if not c.startswith("github")]
            write_model(root, model)
            problems = self._surface_problems(root)
            self.assertTrue(any("replays `nomos github plan` which support_surface.commands does not cover" in p for p in problems), problems)

    def test_real_surface_is_green_and_rendered(self) -> None:
        code, verdict, _ = run_guard(ROOT, "--tags", REAL_TAGS)
        self.assertEqual(code, 0, verdict)
        self.assertEqual(checks_by_name(verdict)["surface"]["problems"], [])
        self.assertIn("Supported contracts (15 stable", (ROOT / "README.md").read_text(encoding="utf-8"))

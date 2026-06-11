"""VRC-00 — the wiring matrix turns red when reality and registry diverge.

Adversarial proofs (doctrine §2.3, docs/45 §8 G3, docs/46 VRC-00):

* a forged ``real`` expectation whose engine anchors do not exist turns red —
  the registry cannot overclaim;
* removing a production caller downgrades a ``real`` capability to ``partial``
  and turns red — the #540 class (``ApplyLens`` had zero production callers);
* engine markers appearing under a ``sidecar`` entry turn red until the registry
  entry is flipped — promotions cannot land silently;
* a ``*Command`` function implemented but neither registered in the app.go
  command map nor called (with a doc-comment decoy that defeats naive
  occurrence counting) turns red — the #543 ``atomize`` class;
* a stale allowlist entry (the command became wired) turns red — the allowlist
  must track the truth in both directions;
* the shipped tree must match the shipped registry (lockstep on the real repo).
"""

from __future__ import annotations

import importlib.util
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "vrc_wiring_matrix.py"

_SPEC = importlib.util.spec_from_file_location("vrc_wiring_matrix", SCRIPT)
wm = importlib.util.module_from_spec(_SPEC)
assert _SPEC.loader is not None
_SPEC.loader.exec_module(wm)


def _write(root: Path, rel: str, text: str) -> None:
    path = root / rel
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def _run(root: Path, registry: dict) -> int:
    registry_path = root / "registry.json"
    registry_path.write_text(json.dumps(registry), encoding="utf-8")
    return wm.main(
        ["--root", str(root), "--registry", str(registry_path), "--out-dir", "", "--quiet"]
    )


def _widget_capability(expected: str) -> dict:
    return {
        "id": "widget",
        "title": "test widget capability",
        "expected": expected,
        "engine": [{"path": "engine/widget.go", "contains": ["func BuildWidget("]}],
        "production_callers": [{"path": "app/run.go", "contains": ["BuildWidget("]}],
        "adversarial_tests": [
            {"path": "engine/widget_test.go", "contains": ["TestBuildWidget_FailsOnTamper"]}
        ],
    }


def _full_widget_tree(root: Path) -> None:
    _write(root, "engine/widget.go", "package widget\n\nfunc BuildWidget() int { return 1 }\n")
    _write(root, "app/run.go", "package app\n\nfunc run() { BuildWidget() }\n")
    _write(
        root,
        "engine/widget_test.go",
        "package widget\n\nfunc TestBuildWidget_FailsOnTamper(t *testing.T) {}\n",
    )


class WiringMatrixCapabilityTests(unittest.TestCase):
    def test_fully_wired_capability_is_real_and_green(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _full_widget_tree(root)
            registry = {"schema_version": "t", "capabilities": [_widget_capability("real")]}
            self.assertEqual(_run(root, registry), 0)

    def test_forged_real_expectation_without_engine_turns_red(self) -> None:
        # The registry cannot overclaim: expected=real with no engine in the tree.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = {"schema_version": "t", "capabilities": [_widget_capability("real")]}
            self.assertEqual(_run(root, registry), 1)
            result = wm.compute_capability(root, _widget_capability("real"))
            self.assertEqual(result["computed"], "absent")
            self.assertTrue(result["mismatch"])

    def test_unwiring_production_caller_downgrades_to_partial(self) -> None:
        # The #540 class: engine + adversarial tests present, zero production callers.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _full_widget_tree(root)
            registry = {"schema_version": "t", "capabilities": [_widget_capability("real")]}
            self.assertEqual(_run(root, registry), 0, "wired tree must be green first")
            (root / "app" / "run.go").unlink()
            self.assertEqual(_run(root, registry), 1)
            result = wm.compute_capability(root, _widget_capability("real"))
            self.assertEqual(result["computed"], "partial")
            self.assertTrue(any("production caller" in r for r in result["reasons"]), result)

    def test_engine_markers_under_sidecar_entry_turn_red(self) -> None:
        # Promotions cannot land silently: the must_be_absent probe fires until
        # the registry entry is flipped with its new anchors.
        capability = {
            "id": "side",
            "expected": "sidecar",
            "sidecar": [{"path": "scripts/side.py", "contains": []}],
            "must_be_absent": [
                {"dir": "cli/internal", "regex": "BuildWidget", "suffixes": [".go"]}
            ],
        }
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _write(root, "scripts/side.py", "# sidecar\n")
            registry = {"schema_version": "t", "capabilities": [capability]}
            self.assertEqual(_run(root, registry), 0)
            _write(root, "cli/internal/widget.go", "package x\n\nfunc BuildWidget() {}\n")
            self.assertEqual(_run(root, registry), 1)
            result = wm.compute_capability(root, capability)
            self.assertTrue(result["mismatch"])
            self.assertTrue(any("flip this registry entry" in d for d in result["drift"]), result)

    def test_anchor_rot_on_sidecar_is_loud(self) -> None:
        # A sidecar whose declared script disappears must not silently stay green.
        capability = {
            "id": "side",
            "expected": "sidecar",
            "sidecar": [{"path": "scripts/side.py", "contains": []}],
        }
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            registry = {"schema_version": "t", "capabilities": [capability]}
            self.assertEqual(_run(root, registry), 1)
            result = wm.compute_capability(root, capability)
            self.assertEqual(result["computed"], "absent")


_APP_GO_UNREGISTERED = """package app

import "io"

type commandFunc func(args []string, stdout io.Writer, stderr io.Writer) int

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	commands := map[string]commandFunc{
		"help": helpCommand,
	}
	_ = commands
	return 0
}

func helpCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	_ = "  help       Print this help"
	return 0
}
"""

# Doc-comment decoy: a naive "name appears twice" heuristic would count the
# comment mention and miss the missing registration. The guard must not.
_FOO_CMD_GO = """package app

import "io"

// FooCommand implements "nomos foo" and is documented but never registered.
func FooCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	_ = io.Discard
	return 0
}
"""

_APP_GO_REGISTERED = """package app

import "io"

type commandFunc func(args []string, stdout io.Writer, stderr io.Writer) int

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	commands := map[string]commandFunc{
		"help": helpCommand,
		"foo":  FooCommand,
	}
	_ = commands
	return 0
}

func helpCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	_ = "  help       Print this help"
	_ = "  foo        Run the foo command"
	return 0
}
"""


class WiringMatrixGenericCheckTests(unittest.TestCase):
    def test_unregistered_command_with_doc_comment_decoy_turns_red(self) -> None:
        # The #543 'atomize' class, generalized.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _write(root, "cli/internal/app/app.go", _APP_GO_UNREGISTERED)
            _write(root, "cli/internal/app/foo_cmd.go", _FOO_CMD_GO)
            registry = {"schema_version": "t", "capabilities": []}
            self.assertEqual(_run(root, registry), 1)
            check = wm.generic_command_checks(root)
            self.assertEqual(check["status"], "fail")
            self.assertTrue(any("FooCommand" in f for f in check["failures"]), check)

    def test_registering_and_advertising_makes_it_green(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _write(root, "cli/internal/app/app.go", _APP_GO_REGISTERED)
            _write(root, "cli/internal/app/foo_cmd.go", _FOO_CMD_GO)
            registry = {"schema_version": "t", "capabilities": []}
            self.assertEqual(_run(root, registry), 0)

    def test_registered_but_not_advertised_in_help_turns_red(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            app_go = _APP_GO_REGISTERED.replace('\t_ = "  foo        Run the foo command"\n', "")
            _write(root, "cli/internal/app/app.go", app_go)
            _write(root, "cli/internal/app/foo_cmd.go", _FOO_CMD_GO)
            registry = {"schema_version": "t", "capabilities": []}
            self.assertEqual(_run(root, registry), 1)

    def test_allowlisted_known_unwired_is_reported_but_not_failing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _write(root, "cli/internal/app/app.go", _APP_GO_UNREGISTERED)
            _write(root, "cli/internal/app/foo_cmd.go", _FOO_CMD_GO)
            registry = {
                "schema_version": "t",
                "capabilities": [],
                "generic_allowlist": {
                    "command_registration": {
                        "known_unwired": [
                            {"name": "FooCommand", "issue": "VRC-09", "note": "test"}
                        ]
                    }
                },
            }
            self.assertEqual(_run(root, registry), 0)
            check = wm.generic_command_checks(
                root, [{"name": "FooCommand", "issue": "VRC-09", "note": "test"}]
            )
            self.assertEqual(check["status"], "pass")
            self.assertTrue(any("FooCommand" in k for k in check["known_unwired"]), check)

    def test_stale_allowlist_entry_turns_red_when_command_becomes_wired(self) -> None:
        # The allowlist must track the truth in both directions.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _write(root, "cli/internal/app/app.go", _APP_GO_REGISTERED)
            _write(root, "cli/internal/app/foo_cmd.go", _FOO_CMD_GO)
            registry = {
                "schema_version": "t",
                "capabilities": [],
                "generic_allowlist": {
                    "command_registration": {
                        "known_unwired": [
                            {"name": "FooCommand", "issue": "VRC-09", "note": "stale"}
                        ]
                    }
                },
            }
            self.assertEqual(_run(root, registry), 1)
            check = wm.generic_command_checks(
                root, [{"name": "FooCommand", "issue": "VRC-09", "note": "stale"}]
            )
            self.assertTrue(any("wired now" in f for f in check["failures"]), check)


class WiringMatrixOutputsAndRealTreeTests(unittest.TestCase):
    def test_outputs_are_generated_and_marked_generated(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            _full_widget_tree(root)
            registry_path = root / "registry.json"
            registry_path.write_text(
                json.dumps({"schema_version": "t", "capabilities": [_widget_capability("real")]}),
                encoding="utf-8",
            )
            out_dir = root / "out"
            code = wm.main(
                [
                    "--root",
                    str(root),
                    "--registry",
                    str(registry_path),
                    "--out-dir",
                    str(out_dir),
                    "--quiet",
                ]
            )
            self.assertEqual(code, 0)
            md = (out_dir / "wiring-matrix.md").read_text(encoding="utf-8")
            self.assertIn("GENERATED FILE", md)
            data = json.loads((out_dir / "wiring-matrix.json").read_text(encoding="utf-8"))
            self.assertEqual(data["schema_version"], wm.MATRIX_SCHEMA_VERSION)
            self.assertEqual(data["capabilities"][0]["computed"], "real")

    def test_real_tree_matches_shipped_registry(self) -> None:
        # Lockstep on the shipped repo: computed statuses must match the
        # committed registry, and generic checks must pass (known-unwired
        # commands are tracked in the allowlist, not hidden).
        self.assertEqual(wm.main(["--root", str(ROOT), "--out-dir", "", "--quiet"]), 0)

    def test_script_runs_as_subprocess(self) -> None:
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(ROOT), "--out-dir", ""],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)


if __name__ == "__main__":
    unittest.main()

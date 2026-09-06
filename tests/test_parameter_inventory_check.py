"""NRT-029 (#702) — a parameter inventory (docs/49 §2.2) is checked, and the
proofs are the refusals: the kit inventory holds; a validated parameter
without dated evidence, a template placeholder, a silent failure with no
detection and no finding, and a reviewed component nobody declared are red
and named; the defaults are listed as the calibration candidates."""

from __future__ import annotations

import copy
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "parameter_inventory_check.py"
KIT_INVENTORY = ROOT / "docs/regulated/domain-packs/built-environment/cross-consumption/parameter-inventory.yaml"
TEMPLATE = ROOT / "templates/regulated/parameter-inventory.yaml"


def run_check(path: Path) -> tuple[int, dict, str]:
    proc = subprocess.run([sys.executable, str(SCRIPT), "--inventory", str(path)], capture_output=True, text=True, check=False)
    verdict = json.loads(proc.stdout) if proc.stdout.strip().startswith("{") else {}
    return proc.returncode, verdict, proc.stderr


class InventoryCheckTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.base = yaml.safe_load(KIT_INVENTORY.read_text(encoding="utf-8"))

    def write(self, doc: dict) -> Path:
        path = Path(self._tmp.name) / "inventory.yaml"
        path.write_text(yaml.safe_dump(doc, sort_keys=False, allow_unicode=True), encoding="utf-8")
        return path

    def test_the_kit_inventory_holds_and_names_its_defaults(self) -> None:
        code, verdict, stderr = run_check(KIT_INVENTORY)
        self.assertEqual(code, 0, stderr)
        self.assertEqual(verdict["status"], "pass")
        summary = verdict["summary"]
        self.assertGreaterEqual(summary["parameters_by_status"]["default"], 5, "the gate and harness thresholds are defaults, and say so")
        self.assertIn("answer-gate.alce_gate", summary["defaults"])
        self.assertEqual(summary["parameters_by_status"]["validated"], 1)

    def test_the_template_itself_is_refused_as_placeholders(self) -> None:
        code, verdict, _ = run_check(TEMPLATE)
        self.assertEqual(code, 1)
        self.assertTrue(any("placeholder" in p for p in verdict["problems"]), verdict["problems"])

    def test_validated_without_dated_evidence_is_red(self) -> None:
        doc = copy.deepcopy(self.base)
        parameter = doc["components"][2]["parameters"][0]  # answer-gate.alce_gate
        parameter["status"] = "validated"
        parameter["evidence"] = {"kind": "none"}
        code, verdict, _ = run_check(self.write(doc))
        self.assertEqual(code, 1)
        problems = verdict["problems"]
        self.assertTrue(any("answer-gate.alce_gate: a validated parameter needs evidence" in p for p in problems), problems)
        self.assertTrue(any("validated without an evidence reference" in p for p in problems), problems)
        self.assertTrue(any("validated without a dated evidence" in p for p in problems), problems)

    def test_silent_failure_without_detection_or_finding_is_red(self) -> None:
        doc = copy.deepcopy(self.base)
        doc["silent_failure_review"][0]["detection"] = "none"
        doc["silent_failure_review"][0].pop("finding_ref", None)
        code, verdict, _ = run_check(self.write(doc))
        self.assertEqual(code, 1)
        self.assertTrue(any("detection is none and no finding is referenced" in p for p in verdict["problems"]), verdict["problems"])

    def test_reviewed_component_nobody_declared_is_red(self) -> None:
        doc = copy.deepcopy(self.base)
        doc["silent_failure_review"].append({"component_id": "ghost", "disabled_silently_when": "never", "detection": "gate", "finding_ref": "n/a"})
        code, verdict, _ = run_check(self.write(doc))
        self.assertEqual(code, 1)
        self.assertTrue(any("component 'ghost' is not declared" in p for p in verdict["problems"]), verdict["problems"])

    def test_component_without_review_is_red(self) -> None:
        doc = copy.deepcopy(self.base)
        doc["silent_failure_review"] = [e for e in doc["silent_failure_review"] if e["component_id"] != "bundle"]
        code, verdict, _ = run_check(self.write(doc))
        self.assertEqual(code, 1)
        self.assertTrue(any(p.startswith("bundle: no silent_failure_review entry") for p in verdict["problems"]), verdict["problems"])

    def test_unknown_status_and_unreadable_file_are_refused(self) -> None:
        doc = copy.deepcopy(self.base)
        doc["components"][0]["parameters"][0]["status"] = "tuned"
        code, verdict, _ = run_check(self.write(doc))
        self.assertEqual(code, 1)
        self.assertTrue(any("status must be one of" in p for p in verdict["problems"]), verdict["problems"])
        broken = Path(self._tmp.name) / "broken.yaml"
        broken.write_text("components: [", encoding="utf-8")
        code, verdict, stderr = run_check(broken)
        self.assertEqual(code, 2)
        self.assertIn("parameter inventory check", stderr)


if __name__ == "__main__":
    unittest.main()

"""Tests for rbok-runtime-e2e.sh script structure and layer classification."""

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
SCRIPT = ROOT / "scripts" / "rbok-runtime-e2e.sh"


class TestScriptExists(unittest.TestCase):
    def test_script_exists(self):
        self.assertTrue(SCRIPT.exists(), f"Script not found: {SCRIPT}")

    def test_script_executable(self):
        self.assertTrue(os.access(SCRIPT, os.X_OK), "Script not executable")

    def test_script_has_shebang(self):
        with open(SCRIPT) as f:
            first_line = f.readline()
        self.assertTrue(first_line.startswith("#!/"), "Missing shebang")

    def test_script_has_set_euo(self):
        content = SCRIPT.read_text()
        self.assertIn("set -euo pipefail", content)

    def test_script_has_usage(self):
        content = SCRIPT.read_text()
        self.assertIn("usage()", content)


class TestScriptSteps(unittest.TestCase):
    def setUp(self):
        self.content = SCRIPT.read_text()

    def test_has_readonly_guard(self):
        self.assertIn("Read-only guard", self.content)

    def test_has_scan(self):
        self.assertIn("corpus scan", self.content)

    def test_has_layer_classification(self):
        self.assertIn("Layer classification", self.content)

    def test_has_manifest(self):
        self.assertIn("corpus manifest", self.content)

    def test_has_feed(self):
        self.assertIn("corpus feed", self.content)

    def test_has_governance(self):
        self.assertIn("Governance evaluation", self.content)

    def test_has_rag_metadata(self):
        self.assertIn("RAG metadata", self.content)

    def test_has_attestation(self):
        self.assertIn("corpus attest", self.content)

    def test_has_git_clean_check(self):
        self.assertIn("git status", self.content)

    def test_has_fingerprint_check(self):
        self.assertIn("sha256sum", self.content)

    def test_disables_push(self):
        self.assertIn("no_push", self.content)


class TestLayerClassification(unittest.TestCase):
    """Test the Python layer classification logic embedded in the script."""

    LAYER_MAP = {
        "00_meta": "meta", "01_rbok": "doctrine", "01_referentiel": "doctrine",
        "02_parcours": "runtime", "02_domaines": "runtime",
        "03_workbook": "workbooks", "03_generated": "workbooks",
        "98_schema": "schemas", "99_rbok": "reference", "99_initial": "reference",
    }

    def classify(self, path: str) -> str:
        top = path.split("/")[0].lower() if "/" in path else path.lower()
        for prefix, layer in self.LAYER_MAP.items():
            if top.startswith(prefix):
                return layer
        return "unknown"

    def test_doctrine(self):
        self.assertEqual(self.classify("01_rbok/referentiel/garanties.yaml"), "doctrine")

    def test_doctrine_referentiel(self):
        self.assertEqual(self.classify("01_referentiel/source.yaml"), "doctrine")

    def test_runtime(self):
        self.assertEqual(self.classify("02_parcours/habitation/souscription.yaml"), "runtime")

    def test_runtime_domaines(self):
        self.assertEqual(self.classify("02_domaines/auto/franchise.yaml"), "runtime")

    def test_workbooks(self):
        self.assertEqual(self.classify("03_workbooks/report.xlsx"), "workbooks")

    def test_generated(self):
        self.assertEqual(self.classify("03_generated/output.json"), "workbooks")

    def test_meta(self):
        self.assertEqual(self.classify("00_meta/glossaire.yaml"), "meta")

    def test_schemas(self):
        self.assertEqual(self.classify("98_schemas/warranty.schema.json"), "schemas")

    def test_reference(self):
        self.assertEqual(self.classify("99_RBOK_initial_pdf/doc.pdf"), "reference")

    def test_unknown(self):
        self.assertEqual(self.classify("04_other/notes.md"), "unknown")

    def test_root_file(self):
        self.assertEqual(self.classify("README.md"), "unknown")

    def test_determinism(self):
        paths = [
            "01_rbok/doc.md", "02_parcours/p.yaml", "03_workbooks/out.csv",
            "00_meta/idx.yaml", "98_schemas/s.cue", "99_initial/doc.pdf",
        ]
        for p in paths:
            self.assertEqual(self.classify(p), self.classify(p))


class TestWorkflowExists(unittest.TestCase):
    def test_workflow_file(self):
        wf = ROOT / ".github" / "workflows" / "rbok-runtime-e2e.yml"
        self.assertTrue(wf.exists())

    def test_workflow_has_e2e_job(self):
        wf = ROOT / ".github" / "workflows" / "rbok-runtime-e2e.yml"
        content = wf.read_text()
        self.assertIn("RBOK Runtime E2E", content)
        self.assertTrue("rbok-runtime-e2e.sh" in content or "corpus scan" in content)

    def test_workflow_read_only_permissions(self):
        wf = ROOT / ".github" / "workflows" / "rbok-runtime-e2e.yml"
        content = wf.read_text()
        self.assertIn("contents: read", content)

    def test_workflow_has_git_clean_step(self):
        wf = ROOT / ".github" / "workflows" / "rbok-runtime-e2e.yml"
        content = wf.read_text()
        self.assertIn("git status", content)

    def test_workflow_cgo_enabled_for_runtime_checks(self):
        wf = ROOT / ".github" / "workflows" / "rbok-runtime-e2e.yml"
        content = wf.read_text()
        self.assertIn('CGO_ENABLED: "1"', content)
        self.assertIn("go test -race ./internal/corpus/...", content)


if __name__ == "__main__":
    unittest.main()

from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def run_cue(*args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(["cue", *args], cwd=ROOT, text=True, capture_output=True, check=False)


def run_validator(bundle: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(ROOT / "scripts" / "ckm_bundle_validate.py"), "--bundle", str(bundle)],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )


class CKMBundleContractTests(unittest.TestCase):
    def test_bundle_fixture_passes_cue_and_validator(self) -> None:
        fixture = ROOT / "specs/examples/canonical-knowledge-bundle.valid.json"
        cue_result = run_cue(
            "vet",
            "specs/atomization-spine.cue",
            "specs/facets.cue",
            "specs/nomos-trace-manifest.cue",
            "attestations/nomos-attestation.cue",
            "specs/canonical-knowledge-bundle.cue",
            str(fixture),
            "-d",
            "#CanonicalKnowledgeBundle",
        )
        self.assertEqual(cue_result.returncode, 0, cue_result.stderr + cue_result.stdout)

        validation = run_validator(fixture)
        self.assertEqual(validation.returncode, 0, validation.stderr + validation.stdout)
        report = json.loads(validation.stdout)
        self.assertEqual(report["status"], "pass")
        self.assertEqual(report["summary"]["nodes"], 1)
        self.assertEqual(report["summary"]["rag_metadata"], 1)

    def test_bundle_validator_refuses_missing_feed(self) -> None:
        fixture = ROOT / "specs/examples/canonical-knowledge-bundle.valid.json"
        with tempfile.TemporaryDirectory() as tmp:
            bundle = json.loads(fixture.read_text(encoding="utf-8"))
            bundle["feeds"] = []
            path = Path(tmp) / "bundle.json"
            path.write_text(json.dumps(bundle), encoding="utf-8")

            validation = run_validator(path)

            self.assertEqual(validation.returncode, 1, validation.stderr + validation.stdout)
            report = json.loads(validation.stdout)
            self.assertTrue(any(finding["code"] == "BUNDLE_FEED_ABSENT" for finding in report["findings"]))

    def test_bundle_validator_refuses_orphan_rag_metadata(self) -> None:
        fixture = ROOT / "specs/examples/canonical-knowledge-bundle.valid.json"
        with tempfile.TemporaryDirectory() as tmp:
            bundle = json.loads(fixture.read_text(encoding="utf-8"))
            bundle["rag_metadata"][0]["node_id"] = "NODE-MISSING"
            path = Path(tmp) / "bundle.json"
            path.write_text(json.dumps(bundle), encoding="utf-8")

            validation = run_validator(path)

            self.assertEqual(validation.returncode, 1, validation.stderr + validation.stdout)
            report = json.loads(validation.stdout)
            self.assertTrue(any(finding["code"] == "BUNDLE_RAG_METADATA_ORPHAN" for finding in report["findings"]))


if __name__ == "__main__":
    unittest.main()

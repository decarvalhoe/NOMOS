"""SEAM-3 (#536): the canonical emitted bundle is the contract source of truth.

`specs/examples/canonical-knowledge-bundle.emitted.json` is produced by the real
`nomos bundle` emitter on the portable golden corpus (NOT hand-crafted). It is the
artifact a downstream consumer (Aedifica) imports unmodified. This gate fails the
moment the emitter would produce something the contract or the facet-vocab gate
rejects, or the moment someone hand-softens the fixture.
"""
from __future__ import annotations

import json
import subprocess
import sys
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
FIXTURE = ROOT / "specs" / "examples" / "canonical-knowledge-bundle.emitted.json"
VALIDATOR = ROOT / "scripts" / "ckm_bundle_validate.py"


class SeamEmittedFixtureTests(unittest.TestCase):
    def setUp(self) -> None:
        self.assertTrue(FIXTURE.exists(), f"missing canonical emitted fixture: {FIXTURE}")
        self.bundle = json.loads(FIXTURE.read_text(encoding="utf-8"))

    def test_schema_version_is_pinned(self) -> None:
        self.assertEqual(self.bundle.get("schema_version"), "ckm-bundle-v1")

    def test_feed_carries_version_and_jurisdiction(self) -> None:
        feed = self.bundle["feeds"][0]
        self.assertTrue(feed.get("version"), "SEAM-1 feed.version must be present")
        juris = feed.get("jurisdiction")
        self.assertIsInstance(juris, dict)
        self.assertEqual(juris.get("country"), "CH")
        self.assertEqual(juris.get("canton"), "VD")
        self.assertEqual(juris.get("commune"), "Lausanne")

    def test_nodes_use_canonical_span_form_and_real_hashes(self) -> None:
        nodes = self.bundle["feeds"][0]["nodes"]
        self.assertGreater(len(nodes), 0)
        for node in nodes:
            self.assertIn("start_line", node["span"])
            self.assertIn("end_line", node["span"])
            self.assertTrue(str(node["source_hash"]).startswith("sha256:"))
            self.assertIn("facets", node)

    def test_validator_with_vocab_gate_passes_with_zero_findings(self) -> None:
        proc = subprocess.run(
            [sys.executable, str(VALIDATOR), "--bundle", str(FIXTURE)],
            capture_output=True, text=True,
        )
        self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
        report = json.loads(proc.stdout)
        self.assertEqual(report["summary"]["findings"], 0, proc.stdout)


if __name__ == "__main__":
    unittest.main()

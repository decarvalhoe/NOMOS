from __future__ import annotations

import subprocess
import shutil
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
PACK_ROOT = ROOT / "docs/regulated/domain-packs/built-environment"
PROFILE = ROOT / "specs/examples/nomos-domain-profile.built-environment.valid.yaml"
CONNECTORS = PACK_ROOT / "ch-source-connectors.yaml"
SIA_SIDECAR = PACK_ROOT / "sia-reference-sidecar.yaml"
PGA_SEGMENTS = PACK_ROOT / "pga-paz-source-segments.valid.yaml"
PGA_BODY_LEDGER = PACK_ROOT / "pga-paz-body-ledger.valid.yaml"


def load_yaml(path: Path):
    return yaml.safe_load(path.read_text(encoding="utf-8"))


class CKMCHSourceConnectorTests(unittest.TestCase):
    def test_machine_connectors_are_hashable_and_datable(self) -> None:
        if shutil.which("cue") is not None:
            result = subprocess.run(
                [
                    "cue",
                    "vet",
                    "specs/built-environment-source-connectors.cue",
                    "docs/regulated/domain-packs/built-environment/ch-source-connectors.yaml",
                    "-d",
                    "#BuiltEnvironmentSourceConnectors",
                ],
                cwd=ROOT,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

        config = load_yaml(CONNECTORS)
        self.assertEqual(config["domain_profile"], "built-environment")
        connectors = {item["source_family"]: item for item in config["connectors"]}
        self.assertEqual(
            set(connectors),
            {"fedlex_eli", "swisstopo_stac", "rdppf_oereb", "ofs"},
        )

        profile = load_yaml(PROFILE)
        artifacts = {artifact["id"]: artifact for artifact in profile["required_artifacts"]}
        for artifact_id in (
            "ch-source-connectors",
            "pga-paz-source-segment-ledger",
            "pga-paz-body-ledger",
            "sia-reference-sidecar",
        ):
            with self.subTest(artifact_id=artifact_id):
                self.assertIn(artifact_id, artifacts)

        for family, connector in connectors.items():
            with self.subTest(family=family):
                self.assertTrue(connector["machine_source"])
                self.assertEqual(connector["retrieval"]["write_policy"], "read_only")
                self.assertEqual(connector["dating"]["as_of_date_policy"], "required")
                self.assertRegex(connector["hashing"]["content_hash"], r"^sha256:[a-f0-9]{64}$")
                self.assertTrue(connector["retrieval"]["retrieved_at_utc"].endswith("Z"))

    def test_pga_paz_pdf_pipeline_has_spans_and_zero_uncovered_bytes(self) -> None:
        if shutil.which("cue") is not None:
            segment_result = subprocess.run(
                [
                    "cue",
                    "vet",
                    "specs/source-segment-ledger.cue",
                    "docs/regulated/domain-packs/built-environment/pga-paz-source-segments.valid.yaml",
                    "-d",
                    "#SourceSegmentLedger",
                ],
                cwd=ROOT,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(segment_result.returncode, 0, segment_result.stderr + segment_result.stdout)

            body_result = subprocess.run(
                [
                    "cue",
                    "vet",
                    "specs/corpus-body-ledger.cue",
                    "docs/regulated/domain-packs/built-environment/pga-paz-body-ledger.valid.yaml",
                    "-d",
                    "#CorpusBodyLedger",
                ],
                cwd=ROOT,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(body_result.returncode, 0, body_result.stderr + body_result.stdout)

        ledger = load_yaml(PGA_SEGMENTS)
        self.assertEqual(ledger["source_id"], "VD-LAUSANNE-PGA-PAZ-PDF")
        self.assertRegex(ledger["source_hash"], r"^sha256:[a-f0-9]{64}$")
        self.assertGreaterEqual(len(ledger["segments"]), 3)
        for segment in ledger["segments"]:
            with self.subTest(segment=segment["segment_id"]):
                self.assertLess(segment["start_byte"], segment["end_byte"])
                self.assertGreaterEqual(segment["start_line"], 1)
                self.assertGreaterEqual(segment["end_line"], segment["start_line"])
                if segment["disposition"] == "canonical_atom":
                    self.assertIn("raw_text_hash", segment)
                    self.assertIn("normalized_text_hash", segment)

        body = load_yaml(PGA_BODY_LEDGER)
        self.assertEqual(body["coverage_summary"]["uncovered_bytes"], 0)
        source = body["sources"][0]
        coverage = source["byte_coverage"]
        covered = sum(
            coverage[key]
            for key in (
                "semantic_bytes",
                "structure_bytes",
                "coverage_only_bytes",
                "metadata_bytes",
                "unsupported_bytes",
                "binary_bytes",
            )
        )
        self.assertEqual(covered, coverage["total_bytes"])
        self.assertEqual(coverage["uncovered_bytes"], 0)

    def test_sia_sidecar_is_hash_only_and_never_commits_full_text(self) -> None:
        self.assertTrue(SIA_SIDECAR.exists(), f"Missing SIA sidecar: {SIA_SIDECAR}")
        sidecar = load_yaml(SIA_SIDECAR)
        self.assertEqual(sidecar["reference_id"], "SIA-REFERENCE")
        self.assertFalse(sidecar["full_text_committed"])
        self.assertEqual(sidecar["storage_policy"], "hash_and_crosswalk_only")
        self.assertRegex(sidecar["reference_hash"], r"^sha256:[a-f0-9]{64}$")

        forbidden_keys = {"full_text", "verbatim_text", "content", "body"}
        self.assertTrue(forbidden_keys.isdisjoint(sidecar.keys()))


if __name__ == "__main__":
    unittest.main()

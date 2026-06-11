"""SEAM-2 (#535): facet vocabulary validator + generated artifact tests.

These tests prove three things:

1. The generated artifact (specs/generated/facets-vocab.json) is in lockstep with
   specs/facets.cue — the single source of truth cannot silently drift.
2. The bundle validator now rejects an out-of-vocabulary facet value (the
   forged ``trust_tier: "trust-me-bro"`` that previously slipped through),
   naming the offending axis and value.
3. The vocabulary the validator enforces is exactly what ``cue vet`` enforces:
   every listed value is accepted by the CUE contract and a forged value is
   rejected.
"""

from __future__ import annotations

import importlib.util
import json
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
VOCAB_PATH = ROOT / "specs" / "generated" / "facets-vocab.json"


def _load_module(name: str, relpath: str):
    spec = importlib.util.spec_from_file_location(name, ROOT / relpath)
    assert spec and spec.loader
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


validator = _load_module("ckm_bundle_validate", "scripts/ckm_bundle_validate.py")
generator = _load_module("ckm_gen_facets_vocab", "scripts/ckm_gen_facets_vocab.py")


def base_bundle() -> dict:
    """A minimal structurally-valid bundle with one clean-faceted node."""
    return {
        "schema_version": "ckm-bundle-v1",
        "bundle_id": "vocab-test",
        "feeds": [
            {
                "feed_id": "f",
                "format": "nomos.canonical-knowledge-feed.v1",
                "nodes": [
                    {
                        "node_id": "NODE-1",
                        "text": "x",
                        "facets": {
                            "nature": "rule",
                            "trust_tier": "indicative",
                            "provenance": "source_backed",
                        },
                    }
                ],
            }
        ],
        "rag_metadata": [],
    }


class FacetsVocabArtifactTests(unittest.TestCase):
    def test_generated_artifact_is_in_sync_with_cue(self) -> None:
        """`--check` must report the committed artifact matches specs/facets.cue."""
        result = subprocess.run(
            [sys.executable, str(ROOT / "scripts" / "ckm_gen_facets_vocab.py"), "--check"],
            cwd=ROOT,
            text=True,
            capture_output=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_artifact_axes_match_cue_disjunctions(self) -> None:
        cue_text = (ROOT / "specs" / "facets.cue").read_text(encoding="utf-8")
        vocab = validator.load_facets_vocab(VOCAB_PATH)
        for axis, definition in generator.SCALAR_AXES.items():
            expected = generator.parse_disjunction(cue_text, definition)
            self.assertEqual(vocab[axis], expected, f"axis {axis} drifted from {definition}")

    def test_trust_tier_certified_present_but_not_auto_derivable(self) -> None:
        # `certified` is a legal vocabulary value (promotion can set it) but the
        # emitter never derives it — see the trust-tier policy doc. The vocab
        # artifact lists it; the no-auto-certify guarantee is enforced in the Go
        # engine (TestBuild_NodesCarryDerivedFacets) and documented separately.
        vocab = validator.load_facets_vocab(VOCAB_PATH)
        self.assertIn("certified", vocab["trust_tier"])
        self.assertIn("indicative", vocab["trust_tier"])


class FacetsVocabValidatorTests(unittest.TestCase):
    def test_clean_facets_pass(self) -> None:
        report = validator.validate_bundle(base_bundle())
        self.assertEqual(report["status"], "pass", report["findings"])

    def test_forged_trust_tier_is_rejected(self) -> None:
        bundle = base_bundle()
        bundle["feeds"][0]["nodes"][0]["facets"]["trust_tier"] = "trust-me-bro"
        report = validator.validate_bundle(bundle)
        self.assertEqual(report["status"], "fail")
        offending = [f for f in report["findings"] if f["code"] == "BUNDLE_FACET_VALUE_INVALID"]
        self.assertEqual(len(offending), 1, report["findings"])
        self.assertIn("trust_tier", offending[0]["message"])
        self.assertIn("trust-me-bro", offending[0]["message"])
        self.assertEqual(offending[0]["path"], "feeds[0].nodes[0].facets.trust_tier")

    def test_forged_nature_is_rejected(self) -> None:
        bundle = base_bundle()
        bundle["feeds"][0]["nodes"][0]["facets"]["nature"] = "vibes"
        report = validator.validate_bundle(bundle)
        self.assertEqual(report["status"], "fail")
        self.assertTrue(
            any(f["code"] == "BUNDLE_FACET_VALUE_INVALID" and "nature" in f["message"] for f in report["findings"])
        )

    def test_rag_metadata_facets_are_also_gated(self) -> None:
        bundle = base_bundle()
        bundle["rag_metadata"].append(
            {"node_id": "NODE-1", "chunk_id": "c", "facets": {"trust_tier": "make-believe"}}
        )
        report = validator.validate_bundle(bundle)
        self.assertEqual(report["status"], "fail")
        self.assertTrue(
            any(
                f["code"] == "BUNDLE_FACET_VALUE_INVALID" and f["path"].startswith("rag_metadata[0]")
                for f in report["findings"]
            )
        )

    def test_aedifica_local_values_are_refused_by_nomos_vocab(self) -> None:
        # Aedifica uses provenance:"official" / nature:"regulatory" for its own
        # OFS-direct rows. Those are NOT in NOMOS's facets.cue. NOMOS's vocab is
        # authoritative, so the validator MUST refuse them (recorded decision in
        # docs/44-facet-trust-tier-policy.md).
        for axis, value in (("provenance", "official"), ("nature", "regulatory")):
            bundle = base_bundle()
            bundle["feeds"][0]["nodes"][0]["facets"][axis] = value
            report = validator.validate_bundle(bundle)
            self.assertEqual(report["status"], "fail", f"{axis}={value} should be refused")
            self.assertTrue(
                any(f["code"] == "BUNDLE_FACET_VALUE_INVALID" and axis in f["message"] for f in report["findings"])
            )

    def test_missing_vocab_artifact_fails_loudly(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            bundle_path = Path(tmp) / "bundle.json"
            bundle_path.write_text(json.dumps(base_bundle()), encoding="utf-8")
            missing_vocab = Path(tmp) / "nope.json"
            result = subprocess.run(
                [
                    sys.executable,
                    str(ROOT / "scripts" / "ckm_bundle_validate.py"),
                    "--bundle",
                    str(bundle_path),
                    "--vocab",
                    str(missing_vocab),
                ],
                cwd=ROOT,
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(result.returncode, 1, result.stdout)
            report = json.loads(result.stdout)
            self.assertTrue(
                any(f["code"] == "BUNDLE_FACET_VOCAB_UNAVAILABLE" for f in report["findings"]),
                report,
            )


class FacetsVocabMatchesCueTests(unittest.TestCase):
    """The validator's vocabulary must match what `cue vet` enforces, end to end."""

    def _cue(self, *args: str) -> subprocess.CompletedProcess[str]:
        if shutil.which("cue") is None:
            raise unittest.SkipTest("cue is not installed")
        return subprocess.run(["cue", "vet", *args], cwd=ROOT, text=True, capture_output=True, check=False)

    def _vet_facets(self, facets: dict, tmp: str) -> subprocess.CompletedProcess[str]:
        """Vet a bare #Facets value against the real CUE contract."""
        path = Path(tmp) / "facets.json"
        path.write_text(json.dumps(facets), encoding="utf-8")
        return self._cue(
            "specs/atomization-spine.cue",
            "specs/facets.cue",
            str(path),
            "-d",
            "#Facets",
        )

    def test_every_listed_value_is_accepted_by_cue(self) -> None:
        vocab = validator.load_facets_vocab(VOCAB_PATH)
        with tempfile.TemporaryDirectory() as tmp:
            for axis, values in vocab.items():
                for value in values:
                    result = self._vet_facets({axis: value}, tmp)
                    self.assertEqual(
                        result.returncode,
                        0,
                        f"cue rejected vocabulary value {axis}={value}:\n{result.stderr}{result.stdout}",
                    )

    def test_forged_value_is_rejected_by_cue(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            result = self._vet_facets({"trust_tier": "trust-me-bro"}, tmp)
            self.assertNotEqual(result.returncode, 0, "cue accepted a forged trust_tier")


if __name__ == "__main__":
    unittest.main()

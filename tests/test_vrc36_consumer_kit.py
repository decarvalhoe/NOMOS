from __future__ import annotations

import copy
import hashlib
import importlib.util
import json
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
CLI = ROOT / "cli"
CORPUS = ROOT / "cli/internal/corpus/testdata/aec-golden-corpus/vd-lausanne"
SCRIPT = ROOT / "scripts/nomos_consumer_kit.py"
COMMIT = "0123456789abcdef0123456789abcdef01234567"


def _load_kit():
    spec = importlib.util.spec_from_file_location("nomos_consumer_kit", SCRIPT)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


KIT = _load_kit()


def emit_real_bundle(tmp: Path) -> dict:
    corpus_copy = tmp / "corpus"
    shutil.copytree(CORPUS, corpus_copy)
    out = tmp / "out" / "bundle.json"
    out.parent.mkdir()
    result = subprocess.run(
        [
            "go", "run", ".", "bundle",
            "--root", str(corpus_copy),
            "--bundle-id", "aec-golden-kit",
            "--repo", "example/aec-golden",
            "--commit", COMMIT,
            "--out", str(out),
        ],
        cwd=CLI,
        text=True,
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        raise AssertionError(f"bundle emission failed: {result.stderr}{result.stdout}")
    return json.loads(out.read_text(encoding="utf-8"))


def reforge_attestation(bundle: dict) -> None:
    """Recompute the attestation digest the way an attacker who controls the
    file would — used to ISOLATE the non-digest detectors in tamper tests."""
    digest = hashlib.sha256(KIT.go_canonical_json(bundle["feeds"])).hexdigest()
    bundle["attestation"]["subject"][0]["digest"]["sha256"] = digest


class VRC36ConsumerKitTests(unittest.TestCase):
    """VRC-36 (#573, E-1) — « consommer NOMOS = passer un kit » : the
    reference importer accepts the REAL emitted bundle and rejects every
    altered form the issue names (hash, unknown facet, schema_version),
    each case isolated and tested."""

    bundle: dict

    @classmethod
    def setUpClass(cls) -> None:
        if shutil.which("go") is None:
            raise unittest.SkipTest("go not on PATH — the kit consumes a real emitted bundle")
        cls._tmp = tempfile.TemporaryDirectory()
        cls.bundle = emit_real_bundle(Path(cls._tmp.name))

    @classmethod
    def tearDownClass(cls) -> None:
        cls._tmp.cleanup()

    def test_real_bundle_passes_the_kit(self) -> None:
        """The kit's digest recomputation must byte-match Go's emission —
        this is the canonicalization contract, proven on real output."""
        verdict = KIT.run_kit(copy.deepcopy(self.bundle))
        self.assertEqual(verdict["status"], "pass", verdict["findings"])

    def test_one_mutated_byte_is_rejected(self) -> None:
        tampered = copy.deepcopy(self.bundle)
        node = tampered["feeds"][0]["nodes"][0]
        node["text"] = node["text"].replace("e", "3", 1)
        verdict = KIT.run_kit(tampered)
        self.assertEqual(verdict["status"], "fail")
        self.assertTrue(any(f["code"] == "KIT_ATTESTATION_DIGEST_MISMATCH" for f in verdict["findings"]),
                        verdict["findings"])

    def test_unknown_facet_value_is_rejected_even_with_reforged_digest(self) -> None:
        tampered = copy.deepcopy(self.bundle)
        tampered["feeds"][0]["nodes"][0]["facets"]["trust_tier"] = "trust-me-bro"
        reforge_attestation(tampered)  # the attacker fixed the digest…
        verdict = KIT.run_kit(tampered)
        self.assertEqual(verdict["status"], "fail")
        self.assertTrue(any(f["code"] == "BUNDLE_FACET_VALUE_INVALID" for f in verdict["findings"]),
                        verdict["findings"])

    def test_wrong_schema_version_is_refused_not_best_effort(self) -> None:
        tampered = copy.deepcopy(self.bundle)
        tampered["schema_version"] = "ckm-bundle-v2"
        reforge_attestation(tampered)
        verdict = KIT.run_kit(tampered)
        self.assertEqual(verdict["status"], "fail")
        self.assertTrue(any(f["code"] == "KIT_SCHEMA_VERSION" for f in verdict["findings"]),
                        verdict["findings"])

    def test_forged_hash_form_is_rejected(self) -> None:
        tampered = copy.deepcopy(self.bundle)
        tampered["feeds"][0]["nodes"][0]["source_hash"] = "sha256:zzzz"
        reforge_attestation(tampered)
        verdict = KIT.run_kit(tampered)
        self.assertEqual(verdict["status"], "fail")
        self.assertTrue(any(f["code"] == "KIT_SOURCE_HASH_FORM" for f in verdict["findings"]),
                        verdict["findings"])

    def test_inconsistent_source_hash_is_rejected(self) -> None:
        """Two nodes of the same source disagreeing on source_hash = a
        partial tamper the FORM check alone cannot see."""
        tampered = copy.deepcopy(self.bundle)
        nodes = tampered["feeds"][0]["nodes"]
        same_source = [n for n in nodes if n["source_path"] == nodes[0]["source_path"]]
        if len(same_source) < 2:
            self.skipTest("corpus produced a single node for the first source")
        same_source[1]["source_hash"] = "sha256:" + "a" * 64
        reforge_attestation(tampered)
        verdict = KIT.run_kit(tampered)
        self.assertEqual(verdict["status"], "fail")
        self.assertTrue(any(f["code"] == "KIT_SOURCE_HASH_INCONSISTENT" for f in verdict["findings"]),
                        verdict["findings"])


if __name__ == "__main__":
    unittest.main()

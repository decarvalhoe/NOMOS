"""NRT-016 (#660) — the Nomos/Praxis exchange contract, three ways.

1. JSON Schema mirror: valid fixture passes, each invalid fixture fails on the
   rule it names (jsonschema, when installed).
2. CUE contract: same verdicts through `cue vet` (when cue is installed).
3. Go engine through the CLI: same verdicts, hashes recomputed against the
   tree, and tamper of a referenced artifact turns it red (when go is installed).
A fixture accepted by one checker and refused by another is a contract bug.
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

import yaml

ROOT = Path(__file__).resolve().parents[1]
EXAMPLES = ROOT / "specs" / "examples"
SCHEMA = ROOT / "specs" / "nomos-praxis-evidence.schema.json"
CUE = ROOT / "specs" / "nomos-praxis-evidence.cue"
VALID = EXAMPLES / "nomos-praxis-evidence.valid.yaml"
INVALID = sorted(EXAMPLES.glob("nomos-praxis-evidence.invalid-*.yaml"))
HAVE_JSONSCHEMA = importlib.util.find_spec("jsonschema") is not None
HAVE_CUE = shutil.which("cue") is not None
HAVE_GO = shutil.which("go") is not None


class _StringTimestamps(yaml.SafeLoader):
    """YAML timestamps stay strings: the contract types them as RFC3339 text."""


_StringTimestamps.add_constructor("tag:yaml.org,2002:timestamp", lambda loader, node: loader.construct_scalar(node))


def load(path: Path) -> dict:
    return yaml.load(path.read_text(encoding="utf-8"), Loader=_StringTimestamps)


class FixturesExist(unittest.TestCase):
    def test_negative_fixtures_exist_and_name_their_rule(self) -> None:
        self.assertTrue(VALID.exists())
        names = {p.name for p in INVALID}
        self.assertIn("nomos-praxis-evidence.invalid-regulated-unverified.yaml", names)
        self.assertIn("nomos-praxis-evidence.invalid-praxis-producer.yaml", names)
        for p in INVALID:
            self.assertIn("NEGATIVE", p.read_text(encoding="utf-8")[:400], p.name)


@unittest.skipUnless(HAVE_JSONSCHEMA, "jsonschema not installed")
class JsonSchemaMirror(unittest.TestCase):
    def setUp(self) -> None:
        import jsonschema
        self.validator = jsonschema.Draft202012Validator(json.loads(SCHEMA.read_text(encoding="utf-8")))

    def test_valid_fixture_passes(self) -> None:
        self.assertEqual(list(self.validator.iter_errors(load(VALID))), [])

    def test_each_invalid_fixture_fails(self) -> None:
        for p in INVALID:
            errors = list(self.validator.iter_errors(load(p)))
            self.assertTrue(errors, f"{p.name} passed the JSON Schema mirror")

    def test_regulated_reliance_needs_verified_artifacts_and_verdict(self) -> None:
        doc = load(VALID)
        doc["reliance"] = "regulated_evidence"
        self.assertTrue(list(self.validator.iter_errors(doc)), "unverified artifacts under regulated reliance must fail")
        for a in doc["nomos_artifacts"]:
            a["verification"] = {"state": "verified", "record_path": "x", "record_sha256": "sha256:" + "0" * 64}
        self.assertTrue(list(self.validator.iter_errors(doc)), "regulated reliance without a bound verdict must fail")
        doc["activation_verdict_path"] = "docs/x.json"
        doc["activation_verdict_sha256"] = "sha256:" + "0" * 64
        self.assertEqual(list(self.validator.iter_errors(doc)), [])

    def test_unknown_field_is_refused(self) -> None:
        doc = load(VALID)
        doc["nomos_artifacts"][0]["block_id"] = "B-1"
        self.assertTrue(list(self.validator.iter_errors(doc)))


@unittest.skipUnless(HAVE_CUE, "cue not installed")
class CueContract(unittest.TestCase):
    def vet(self, path: Path) -> int:
        return subprocess.run(["cue", "vet", str(CUE), str(path), "-d", "#PraxisEvidenceExchange"], capture_output=True, text=True).returncode

    def test_valid_passes_and_invalid_fail(self) -> None:
        self.assertEqual(self.vet(VALID), 0)
        for p in INVALID:
            self.assertNotEqual(self.vet(p), 0, p.name)


@unittest.skipUnless(HAVE_GO, "go not installed")
class GoEngine(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.tmp = tempfile.mkdtemp()
        cls.bin = str(Path(cls.tmp) / "nomos")
        r = subprocess.run(["go", "build", "-o", cls.bin, "."], cwd=ROOT / "cli", capture_output=True, text=True)
        if r.returncode != 0:
            raise unittest.SkipTest("nomos build failed: " + r.stderr[-400:])

    def run_verify(self, exchange: Path, root: Path | None = ROOT) -> subprocess.CompletedProcess[str]:
        args = [self.bin, "evidence", "praxis-verify", "--exchange", str(exchange)]
        if root is not None:
            args += ["--repo-root", str(root)]
        return subprocess.run(args, capture_output=True, text=True)

    def test_valid_fixture_verifies_with_tree_hashes(self) -> None:
        r = self.run_verify(VALID)
        self.assertEqual(r.returncode, 0, r.stderr)
        rep = json.loads(r.stdout)
        self.assertTrue(rep["hashes_checked_against_tree"])
        self.assertEqual(rep["reliance"], "not_qualified_external_input")
        self.assertIn("artifact_hashes", rep["checks"])

    def test_invalid_fixtures_are_refused_with_named_codes(self) -> None:
        want = {
            "nomos-praxis-evidence.invalid-regulated-unverified.yaml": "PRAXIS_RELIANCE_UNSUPPORTED",
            "nomos-praxis-evidence.invalid-praxis-producer.yaml": "PRAXIS_AUTHORITY_INVERTED",
        }
        for p in INVALID:
            r = self.run_verify(p)
            self.assertEqual(r.returncode, 1, p.name)
            self.assertIn(want[p.name], r.stderr, p.name)

    def test_tampered_referenced_artifact_is_red(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            doc = load(VALID)
            for a in doc["nomos_artifacts"]:
                dst = root / a["path"]
                dst.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(ROOT / a["path"], dst)
            ex = root / "exchange.yaml"
            ex.write_text(yaml.safe_dump(doc, sort_keys=False), encoding="utf-8")
            first = self.run_verify(ex, root)
            self.assertEqual(first.returncode, 0, first.stderr)
            target = root / doc["nomos_artifacts"][0]["path"]
            target.write_text(target.read_text(encoding="utf-8") + "\n# tampered\n", encoding="utf-8")
            r = self.run_verify(ex, root)
            self.assertEqual(r.returncode, 1)
            self.assertIn("PRAXIS_ARTIFACT_HASH_MISMATCH", r.stderr)


if __name__ == "__main__":
    unittest.main()

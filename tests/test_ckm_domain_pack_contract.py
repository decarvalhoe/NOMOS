from __future__ import annotations

import shutil
import subprocess
import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
SPEC = ROOT / "specs/domain-pack.cue"
PACK_MANIFEST = ROOT / "docs/regulated/domain-packs/built-environment/pack.yaml"
PACK_DIR = ROOT / "docs/regulated/domain-packs/built-environment"
INVALID_CODE = ROOT / "specs/examples/domain-pack.code-artifact.invalid.yaml"
INVALID_FIELD = ROOT / "specs/examples/domain-pack.mechanics-field.invalid.yaml"

# The whole no-mechanics rule, in one positive allowlist: everything a pack
# tree may contain is declarative data. There is no negative list to bypass.
DECLARATIVE_SUFFIXES = {".yaml", ".yml", ".md", ".json"}


def cue_vet(instance: Path, definition: str = "#DomainPack") -> subprocess.CompletedProcess:
    return subprocess.run(
        ["cue", "vet", str(SPEC.relative_to(ROOT)), str(instance.relative_to(ROOT)), "-d", definition],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )


def load_yaml(path: Path):
    return yaml.safe_load(path.read_text(encoding="utf-8"))


class DomainPackContractTests(unittest.TestCase):
    """VRC-20 (D1, #563) — « un pack est 100 % déclaratif » must be a gate,
    not a sentence in a doc. The REAL built-environment manifest passes the
    contract; a pack that ships code or smuggles a field fails closed."""

    def test_real_pack_manifest_passes_the_contract(self) -> None:
        if shutil.which("cue") is None:
            self.skipTest("cue not on PATH")
        result = cue_vet(PACK_MANIFEST)
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_vocabulary_file_passes_the_vocabulary_shape(self) -> None:
        if shutil.which("cue") is None:
            self.skipTest("cue not on PATH")
        manifest = load_yaml(PACK_MANIFEST)
        vocab = ROOT / manifest["vocabularies"]["file"]
        result = cue_vet(vocab, definition="#PackVocabulary")
        self.assertEqual(result.returncode, 0, result.stderr + result.stdout)

    def test_pack_shipping_code_fails_closed(self) -> None:
        """Adversarial: the vocabulary file is a .py script. The positive
        allowlist cannot match it — cue vet MUST reject the manifest."""
        if shutil.which("cue") is None:
            self.skipTest("cue not on PATH")
        result = cue_vet(INVALID_CODE)
        self.assertNotEqual(result.returncode, 0, "a pack shipping code passed the contract")

    def test_pack_smuggling_a_field_fails_closed(self) -> None:
        """Adversarial: an `entrypoint:` field outside the closed definition
        (mechanics around the path rule). MUST be rejected."""
        if shutil.which("cue") is None:
            self.skipTest("cue not on PATH")
        result = cue_vet(INVALID_FIELD)
        self.assertNotEqual(result.returncode, 0, "an out-of-contract field passed the contract")

    def test_every_declared_artifact_exists(self) -> None:
        """The manifest is a register, not a wish list: every path it names
        must exist in the repo (profile, vocabulary, register, presets,
        golden corpus root + documents)."""
        manifest = load_yaml(PACK_MANIFEST)
        paths = [
            manifest["profile_ref"],
            manifest["vocabularies"]["file"],
            manifest["source_register"]["file"],
        ]
        paths += [preset["file"] for preset in manifest["lens_presets"]]
        for rel in paths:
            with self.subTest(path=rel):
                self.assertTrue((ROOT / rel).exists(), f"declared artifact missing: {rel}")
        corpus_root = ROOT / manifest["golden_corpus"]["root"]
        self.assertTrue(corpus_root.is_dir(), f"golden corpus root missing: {corpus_root}")
        for doc in manifest["golden_corpus"]["documents"]:
            with self.subTest(doc=doc):
                self.assertTrue((corpus_root / doc).is_file(), f"golden corpus doc missing: {doc}")

    def test_declared_axes_carry_terms_and_preset_ids_match(self) -> None:
        """Coherence between the manifest and the artifacts it names: each
        declared vocabulary axis has at least one term; each preset file's id
        is exactly the id the manifest declares for it."""
        manifest = load_yaml(PACK_MANIFEST)
        vocab = load_yaml(ROOT / manifest["vocabularies"]["file"])
        for axis in manifest["vocabularies"]["axes"]:
            with self.subTest(axis=axis):
                terms = vocab.get(axis) or []
                self.assertGreaterEqual(len(terms), 1, f"axis {axis} declared but empty")
                for term in terms:
                    self.assertIn(".", term["id"], f"term {term['id']} is not pack-namespaced")
        for preset in manifest["lens_presets"]:
            with self.subTest(preset=preset["id"]):
                lens = load_yaml(ROOT / preset["file"])
                self.assertEqual(lens["id"], preset["id"], "manifest/preset id drift")
                self.assertTrue(lens.get("include") or lens.get("exclude"),
                                f"{preset['file']} is not a usable lens")
        register = load_yaml(ROOT / manifest["source_register"]["file"])
        self.assertEqual(register["domain_profile"], manifest["domain_profile"])

    def test_pack_tree_contains_no_mechanics(self) -> None:
        """The strongest reading of « rien d'autre », enforced without cue:
        EVERY file under the pack tree is declarative data. A .py/.go/.sh
        dropped anywhere in the pack directory fails this test."""
        offenders = [
            str(p.relative_to(ROOT))
            for p in PACK_DIR.rglob("*")
            if p.is_file() and p.suffix.lower() not in DECLARATIVE_SUFFIXES
        ]
        self.assertEqual(offenders, [], f"non-declarative files in the pack tree: {offenders}")


if __name__ == "__main__":
    unittest.main()

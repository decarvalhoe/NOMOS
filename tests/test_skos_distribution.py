"""#643 — static SKOS distribution: the proofs are the failures.

The committed distribution must be a fresh, byte-identical build of the
versioned authoring sources, must round-trip back to the validated graph, and
every loss — a label, an IRI, a mapping, an index hash, an authoring/CUE drift —
must turn the tool red. Skipped without pyshacl/rdflib (as the SHACL gate is).
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
SCRIPT = ROOT / "scripts" / "skos_distribution.py"
_HAVE_RDF = importlib.util.find_spec("pyshacl") is not None and importlib.util.find_spec("rdflib") is not None

COPIED = (
    "scripts/facet_shacl_gate.py",
    "scripts/skos_distribution.py",
    "specs/facet-vocabularies",
    "specs/generated/facets-vocab.json",
    "specs/generated/skos",
    "specs/shacl",
    "docs/regulated/domain-packs",
)


def run(root: Path, *args: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run([sys.executable, str(root / "scripts" / "skos_distribution.py"), "--root", str(root), *args],
                          capture_output=True, text=True)


def copy_tree(tmp: Path) -> Path:
    root = tmp / "repo"
    for rel in COPIED:
        src, dst = ROOT / rel, root / rel
        dst.parent.mkdir(parents=True, exist_ok=True)
        if src.is_dir():
            shutil.copytree(src, dst)
        else:
            shutil.copy2(src, dst)
    return root


@unittest.skipUnless(_HAVE_RDF, "pyshacl/rdflib are not installed")
class SkosDistributionTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = copy_tree(Path(self._tmp.name))
        self.dist = self.root / "specs" / "generated" / "skos"

    # ---- the committed distribution is honest ----------------------------
    def test_committed_distribution_is_fresh_and_round_trips(self) -> None:
        r = run(ROOT, "--check", "--verify")
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)
        self.assertIn("byte-identical", r.stdout)
        self.assertIn("round-trip", r.stdout)

    def test_build_is_deterministic(self) -> None:
        a, b = Path(self._tmp.name) / "a", Path(self._tmp.name) / "b"
        self.assertEqual(run(self.root, "--out", str(a)).returncode, 0)
        self.assertEqual(run(self.root, "--out", str(b)).returncode, 0)
        for f in sorted(a.iterdir()):
            self.assertEqual(f.read_bytes(), (b / f.name).read_bytes(), f.name)
        index = json.loads((a / "index.json").read_text(encoding="utf-8"))
        names = {g["name"] for g in index["graphs"]}
        self.assertIn("core-facets", names)
        self.assertTrue({"built-environment", "eu-ai-act"} <= names, names)
        core = next(g for g in index["graphs"] if g["name"] == "core-facets")
        self.assertEqual(core["collections"], 6)
        self.assertEqual(core["concepts"], 38)

    # ---- losses in the distributed files -----------------------------------
    def _edit(self, name: str, old: str, new: str) -> None:
        p = self.dist / name
        text = p.read_text(encoding="utf-8")
        self.assertIn(old, text, f"{name} lacks {old!r}")
        p.write_text(text.replace(old, new, 1), encoding="utf-8")

    def test_lost_label_in_turtle_is_red(self) -> None:
        self._edit("core-facets.skos.ttl", '  skos:prefLabel "certified"@en ;\n', "")
        r = run(self.root, "--verify")
        self.assertEqual(r.returncode, 1)
        self.assertIn("core-facets", r.stderr)
        self.assertIn("triple(s) lost", r.stderr)

    def test_changed_iri_in_turtle_is_red(self) -> None:
        self._edit("core-facets.skos.ttl", "<https://nomos.dev/ns/facets/core/trust_tier/certified>\n  a skos:Concept",
                   "<https://nomos.dev/ns/facets/core/trust_tier/certifiedx>\n  a skos:Concept")
        r = run(self.root, "--verify")
        self.assertEqual(r.returncode, 1)
        self.assertIn("not the built graph", r.stderr)

    def test_lost_node_in_jsonld_is_red(self) -> None:
        p = self.dist / "eu-ai-act.skos.jsonld"
        nodes = json.loads(p.read_text(encoding="utf-8"))
        concept = next(n for n in nodes if "http://www.w3.org/2004/02/skos/core#Concept" in n.get("@type", []))
        nodes.remove(concept)
        p.write_text(json.dumps(nodes, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        r = run(self.root, "--verify")
        self.assertEqual(r.returncode, 1)
        self.assertIn("eu-ai-act.skos.jsonld", r.stderr)

    def test_index_hash_tamper_is_red(self) -> None:
        p = self.dist / "index.json"
        index = json.loads(p.read_text(encoding="utf-8"))
        index["graphs"][0]["files"]["turtle"]["sha256"] = "sha256:" + "0" * 64
        p.write_text(json.dumps(index, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        r = run(self.root, "--verify")
        self.assertEqual(r.returncode, 1)
        self.assertIn("sha256 does not match index", r.stderr)

    def test_missing_mapping_is_red(self) -> None:
        p = self.dist / "mappings.json"
        m = json.loads(p.read_text(encoding="utf-8"))
        victim = next(iri for iri in m["iris"] if "/trust_tier/" in iri)
        del m["iris"][victim]
        p.write_text(json.dumps(m, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        r = run(self.root, "--verify")
        self.assertEqual(r.returncode, 1)
        self.assertIn("no static mapping", r.stderr)

    def test_orphan_file_is_drift(self) -> None:
        (self.dist / "stale.skos.ttl").write_text("# left behind\n", encoding="utf-8")
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 4)
        self.assertIn("stale.skos.ttl (orphan)", r.stderr)

    # ---- the sources moved but the distribution did not ---------------------
    def test_pack_label_change_makes_distribution_stale(self) -> None:
        vocab = self.root / "docs/regulated/domain-packs/eu-ai-act/ai-act-vocabulary.yaml"
        text = vocab.read_text(encoding="utf-8")
        line = next(l for l in text.splitlines() if "label_fr:" in l)
        vocab.write_text(text.replace(line, line.rstrip('"') + ' (modifié)"' if line.rstrip().endswith('"') else line + " (modifié)", 1), encoding="utf-8")
        r = run(self.root, "--check", "--verify")
        self.assertEqual(r.returncode, 4, r.stdout + r.stderr)
        self.assertIn("eu-ai-act.skos.ttl", r.stderr)
        self.assertIn("VERIFY FAILED", r.stderr)

    # ---- authoring and CUE cannot drift -------------------------------------
    def test_authoring_missing_a_cue_value_is_refused(self) -> None:
        p = self.root / "specs/facet-vocabularies/core-facets.authoring.yaml"
        text = p.read_text(encoding="utf-8")
        block = ("      - id: certified\n        pref_label_en: certified\n"
                 "        definition_en: Evidence satisfies the configured deterministic gate for the declared scope.\n")
        self.assertIn(block, text)
        p.write_text(text.replace(block, "", 1), encoding="utf-8")
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 1)
        self.assertIn("REFUSED", r.stderr)
        self.assertIn("missing in authoring ['certified']", r.stderr)

    def test_authoring_value_unknown_to_cue_is_refused(self) -> None:
        p = self.root / "specs/facet-vocabularies/core-facets.authoring.yaml"
        text = p.read_text(encoding="utf-8")
        p.write_text(text.replace("      - id: certified\n", "      - id: gold_plated\n        pref_label_en: gold plated\n      - id: certified\n", 1), encoding="utf-8")
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 1)
        self.assertIn("not in CUE ['gold_plated']", r.stderr)

    def test_concept_without_label_is_refused(self) -> None:
        p = self.root / "specs/facet-vocabularies/core-facets.authoring.yaml"
        text = p.read_text(encoding="utf-8")
        p.write_text(text.replace("      - id: certified\n        pref_label_en: certified\n", "      - id: certified\n", 1), encoding="utf-8")
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 1)
        self.assertIn("has no pref_label_en", r.stderr)

    def test_unparsable_authoring_is_a_refusal_not_a_traceback(self) -> None:
        p = self.root / "specs/facet-vocabularies/core-facets.authoring.yaml"
        p.write_text(p.read_text(encoding="utf-8") + "  - id: [unclosed\n", encoding="utf-8")
        r = run(self.root, "--check")
        self.assertEqual(r.returncode, 1)
        self.assertIn("REFUSED", r.stderr)
        self.assertNotIn("Traceback", r.stderr)

    def test_hand_written_stray_vocabulary_is_gone(self) -> None:
        # #643 asked to integrate or remove specs/facet-vocabularies/core-facets.skos.ttl;
        # it was integrated into the authoring source. Its comeback would be a second truth.
        self.assertFalse((ROOT / "specs/facet-vocabularies/core-facets.skos.ttl").exists())
        self.assertTrue((ROOT / "specs/facet-vocabularies/core-facets.authoring.yaml").exists())


if __name__ == "__main__":
    unittest.main()

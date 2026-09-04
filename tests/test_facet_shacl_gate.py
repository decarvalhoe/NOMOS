"""VRC-44 (#580) — the SKOS/SHACL facet gate turns red on a broken vocabulary.

Doctrine §2.3: the proof is the failure. The load-bearing test is the one the
issue names — a NON-ORTHOGONAL vocabulary, a term sitting on two axes declared
disjoint, must make the gate red and say which term and which axes.

These shapes are standard SHACL over a standard SKOS graph, so the same verdict
is reachable with any SHACL engine. That portability is the point: it is a check
a third party can re-run without NOMOS.
"""

from __future__ import annotations

import importlib.util
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCRIPT = ROOT / "scripts" / "facet_shacl_gate.py"
SHAPES = ROOT / "specs" / "shacl" / "facet-ontology.shapes.ttl"

_HAVE_PYSHACL = importlib.util.find_spec("pyshacl") is not None

if _HAVE_PYSHACL:
    _SPEC = importlib.util.spec_from_file_location("facet_shacl_gate", SCRIPT)
    gate = importlib.util.module_from_spec(_SPEC)
    assert _SPEC.loader is not None
    _SPEC.loader.exec_module(gate)


BFO_PROCESS = "http://purl.obolibrary.org/obo/BFO_0000015"
IOF_PROCESS = "https://spec.industrialontologies.org/ontology/core/Core/Process"
BFO_ROLE = "http://purl.obolibrary.org/obo/BFO_0000023"
IOF_ROLE = "https://spec.industrialontologies.org/ontology/core/Core/AgentRole"


@unittest.skipUnless(_HAVE_PYSHACL, "pyshacl is not installed")
class FacetShaclGateTests(unittest.TestCase):
    def setUp(self) -> None:
        self._tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self._tmp.cleanup)
        self.root = Path(self._tmp.name) / "repo"
        self.pack = self.root / "docs" / "regulated" / "domain-packs" / "fixture"
        self.pack.mkdir(parents=True)
        shapes_dir = self.root / "specs" / "shacl"
        shapes_dir.mkdir(parents=True)
        (shapes_dir / "facet-ontology.shapes.ttl").write_text(
            SHAPES.read_text(encoding="utf-8"), encoding="utf-8"
        )

        (self.pack / "pack.yaml").write_text(
            textwrap.dedent(
                """\
                pack_id: fixture
                vocabularies:
                  file: docs/regulated/domain-packs/fixture/vocab.yaml
                ontology:
                  file: docs/regulated/domain-packs/fixture/onto.yaml
                """
            ),
            encoding="utf-8",
        )
        self._write_vocab()
        self._write_onto()

    def _write_vocab(self, shared_term: bool = False) -> None:
        role_term = "t.phase" if shared_term else "t.role"
        (self.pack / "vocab.yaml").write_text(
            textwrap.dedent(
                f"""\
                activity:
                  - id: t.phase
                    label_fr: "Phase"
                discipline_role:
                  - id: {role_term}
                    label_fr: "Rôle"
                """
            ),
            encoding="utf-8",
        )

    def _write_onto(
        self,
        *,
        shared_term: bool = False,
        drop_bfo_mapping: bool = False,
        drop_axis_root: bool = False,
        empty_axis: bool = False,
        bad_term_id: bool = False,
        no_disjoint_axes: bool = False,
        schema_version: str = "ckm-facet-ontology-v1",
    ) -> None:
        role_term = "t.phase" if shared_term else "t.role"
        if bad_term_id:
            role_term = "Role-Sans-Namespace"
        role_maps = (
            "" if drop_bfo_mapping else f"          bfo: {BFO_ROLE}\n"
        ) + f"          iof_core: {IOF_ROLE}\n"
        role_terms = (
            ""
            if empty_axis
            else textwrap.indent(
                f"terms:\n  - id: {role_term}\n    maps_to:\n", "    "
            )
            + textwrap.indent(role_maps, "  ")
        )
        root_line = "" if drop_axis_root else f"    root: {BFO_ROLE}\n"
        disjoint = "" if no_disjoint_axes else "  disjoint_axes: [activity, discipline_role]\n"

        (self.pack / "onto.yaml").write_text(
            f"""schema_version: {schema_version}
facet_axes:
  - id: activity
    root: {BFO_PROCESS}
    iof_class: {IOF_PROCESS}
    terms:
      - id: t.phase
        maps_to:
          bfo: {BFO_PROCESS}
          iof_core: {IOF_PROCESS}
  - id: discipline_role
{root_line}    iof_class: {IOF_ROLE}
{role_terms}orthogonality:
  owl_construct: owl:disjointUnionOf
{disjoint}""",
            encoding="utf-8",
        )

    # --- helpers -----------------------------------------------------------

    def _run(self) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(self.root)],
            capture_output=True,
            text=True,
            check=False,
        )

    def _report(self) -> tuple[bool, str]:
        packs = gate.discover_packs(self.root, None)
        graph = gate.build_graph(self.root, packs[0][0], packs[0][1])
        return gate.validate_pack(graph, self.root / "specs/shacl/facet-ontology.shapes.ttl")

    # --- the honest baseline -----------------------------------------------

    def test_wellformed_vocabulary_conforms(self) -> None:
        conforms, report = self._report()
        self.assertTrue(conforms, report)
        self.assertEqual(self._run().returncode, 0)

    # --- the proof the issue asks for --------------------------------------

    def test_non_orthogonal_vocabulary_turns_the_gate_red(self) -> None:
        # ADVERSARIAL: one term on two axes declared disjoint. owl:disjointUnionOf
        # says that cannot happen; the gate must say which term and which axes.
        self._write_vocab(shared_term=True)
        self._write_onto(shared_term=True)

        conforms, report = self._report()
        self.assertFalse(conforms, "a non-orthogonal vocabulary conformed")
        self.assertIn("OrthogonalityShape", report)
        self.assertIn("t.phase", report)
        self.assertIn("disjointUnionOf is violated", report)

        result = self._run()
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)

    def test_orthogonality_is_unfalsifiable_without_declared_disjoint_axes(self) -> None:
        # ADVERSARIAL: pass the orthogonality check by declaring nothing disjoint.
        # The scheme shape refuses that: an undeclared disjointness cannot be
        # violated, which would make the check theatre.
        self._write_onto(no_disjoint_axes=True)
        conforms, report = self._report()
        self.assertFalse(conforms, "a scheme with no disjoint axes conformed")
        self.assertIn("orthogonality unfalsifiable", report)

    # --- anchoring ---------------------------------------------------------

    def test_term_without_a_bfo_mapping_is_refused(self) -> None:
        self._write_onto(drop_bfo_mapping=True)
        conforms, report = self._report()
        self.assertFalse(conforms)
        self.assertIn("maps to exactly one BFO class", report)

    def test_axis_without_a_root_is_refused(self) -> None:
        self._write_onto(drop_axis_root=True)
        conforms, report = self._report()
        self.assertFalse(conforms)
        self.assertIn("exactly one BFO root", report)

    def test_axis_with_no_term_is_refused(self) -> None:
        # A declaration with nothing behind it.
        self._write_onto(empty_axis=True)
        conforms, report = self._report()
        self.assertFalse(conforms)
        self.assertIn("axis with no term", report)

    def test_unnamespaced_term_id_is_refused(self) -> None:
        self._write_onto(bad_term_id=True)
        conforms, report = self._report()
        self.assertFalse(conforms)
        self.assertIn("pack-namespaced", report)

    def test_term_in_the_ontology_but_not_the_vocabulary_loses_its_label(self) -> None:
        # A term mapped in the ontology but absent from the vocabulary produces a
        # concept with no preferred label, and the shape catches it rather than
        # emitting a silently thinner graph.
        (self.pack / "vocab.yaml").write_text(
            'activity:\n  - id: t.phase\n    label_fr: "Phase"\n', encoding="utf-8"
        )
        conforms, report = self._report()
        self.assertFalse(conforms)
        self.assertIn("exactly one preferred label", report)

    # --- nothing-measured is never a pass ----------------------------------

    def test_wrong_ontology_schema_version_is_exit_2(self) -> None:
        self._write_onto(schema_version="something-else-v9")
        result = self._run()
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertIn("NOT VALIDATED", result.stderr)

    def test_missing_shapes_is_exit_2(self) -> None:
        (self.root / "specs/shacl/facet-ontology.shapes.ttl").unlink()
        result = self._run()
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)

    def test_unknown_pack_is_exit_2(self) -> None:
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(self.root), "--pack", "nope"],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)


@unittest.skipUnless(_HAVE_PYSHACL, "pyshacl is not installed")
class ShippedPacksTests(unittest.TestCase):
    def test_every_shipped_pack_conforms(self) -> None:
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(ROOT)],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_both_verticals_are_covered(self) -> None:
        # The gate must actually reach both packs; validating one and reporting
        # success would be the flattering failure mode.
        result = subprocess.run(
            [sys.executable, str(SCRIPT), "--root", str(ROOT)],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertIn("built-environment", result.stdout)
        self.assertIn("eu-ai-act", result.stdout)

    def test_shapes_are_valid_shacl(self) -> None:
        # A shapes graph that does not parse would silently conform everything.
        from rdflib import Graph

        graph = Graph()
        graph.parse(str(SHAPES), format="turtle")
        self.assertGreater(len(graph), 0)


if __name__ == "__main__":
    unittest.main()

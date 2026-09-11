#!/usr/bin/env python3
"""VRC-44 (#580, doc 45 §3 B4) — SKOS/SHACL validation of pack facet vocabularies.

The gap: facet vocabularies lived in CUE and YAML, with URI-shaped strings that
nothing parsed as IRIs and a SKOS structure nothing checked. `cue vet` validates
the FILE; the Go pack gate validates the pack CONTRACT. Neither produces a
vocabulary a third party can audit with a standard tool.

This script emits the vocabulary as a SKOS/RDF graph plus NOMOS predicates and
validates it with SHACL. The point is not to re-check what the Go gate checks —
the shapes in ``specs/shacl/facet-ontology.shapes.ttl`` are standard SHACL over
a standard graph and can be replayed outside NOMOS. The recorded execution is
pySHACL only; no OWL ``disjointUnionOf`` emission or Jena/TopBraid equivalence
is claimed without a separate run.

What the shapes constrain:

* the scheme carries a label and declares which axes are disjoint — declaring
  none would make orthogonality unfalsifiable;
* every axis is anchored to exactly one BFO root and one IOF class, both real
  IRIs, and carries at least one term;
* every concept has exactly one preferred label, one scheme, one axis, one BFO
  and one IOF mapping, and a pack-namespaced id;
* **orthogonality**: no term sits on two axes declared disjoint.

Run:

    python3 scripts/facet_shacl_gate.py --root .                 # every pack
    python3 scripts/facet_shacl_gate.py --root . --pack eu-ai-act
    python3 scripts/facet_shacl_gate.py --root . --emit-ttl out/ # inspect the graph

Exit codes: 0 conforming, 1 a SHACL violation, 2 nothing could be validated.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import Any

import yaml

try:
    from pyshacl import validate as shacl_validate
    from rdflib import Graph, Literal, Namespace, URIRef
    from rdflib.namespace import RDF, SKOS, XSD
except ImportError as exc:  # pragma: no cover - exercised by the exit-2 path
    print(
        f"facet-shacl: NOT VALIDATED — pyshacl/rdflib unavailable ({exc}); "
        "install with `pip install pyshacl`",
        file=sys.stderr,
    )
    raise SystemExit(2) from exc

PACKS_DIR = Path("docs/regulated/domain-packs")
DEFAULT_SHAPES = Path("specs/shacl/facet-ontology.shapes.ttl")
ONTOLOGY_SCHEMA_VERSION = "ckm-facet-ontology-v1"

NOMOS = Namespace("https://nomos.dev/ns/facets#")
BASE = "https://nomos.dev/ns/pack/"


class GateError(RuntimeError):
    """Raised when the inputs do not allow any validation at all."""


def load_yaml(path: Path) -> Any:
    if not path.is_file():
        raise GateError(f"missing file: {path}")
    return yaml.safe_load(path.read_text(encoding="utf-8"))


def discover_packs(root: Path, only: str | None) -> list[tuple[str, dict[str, Any]]]:
    """Return ``(pack_id, manifest)`` for every pack that declares an ontology."""
    base = root / PACKS_DIR
    if not base.is_dir():
        raise GateError(f"no domain-packs directory at {PACKS_DIR}")
    packs: list[tuple[str, dict[str, Any]]] = []
    for manifest_path in sorted(base.glob("*/pack.yaml")):
        manifest = load_yaml(manifest_path)
        if not isinstance(manifest, dict):
            continue
        pack_id = str(manifest.get("pack_id", manifest_path.parent.name))
        if only and pack_id != only:
            continue
        packs.append((pack_id, manifest))
    if only and not packs:
        raise GateError(f"no pack with pack_id {only!r}")
    if not packs:
        raise GateError("no pack manifest found")
    return packs


def _iri(value: str) -> URIRef:
    return URIRef(value)


def build_graph(root: Path, pack_id: str, manifest: dict[str, Any]) -> Graph:
    """Emit the pack's facet vocabulary as SKOS/RDF + NOMOS predicates.

    Labels come from the vocabulary file and mappings from the ontology file, so
    a term present in one and absent from the other produces a concept that
    fails its shape rather than a silently thinner graph.
    """
    ontology_ref = (manifest.get("ontology") or {}).get("file")
    vocabulary_ref = (manifest.get("vocabularies") or {}).get("file")
    if not ontology_ref or not vocabulary_ref:
        raise GateError(f"{pack_id}: manifest declares no ontology or vocabulary file")

    ontology = load_yaml(root / ontology_ref)
    vocabulary = load_yaml(root / vocabulary_ref)
    if not isinstance(ontology, dict) or not isinstance(vocabulary, dict):
        raise GateError(f"{pack_id}: ontology or vocabulary is not a YAML mapping")
    if ontology.get("schema_version") != ONTOLOGY_SCHEMA_VERSION:
        raise GateError(
            f"{pack_id}: ontology schema_version is "
            f"{ontology.get('schema_version')!r}, expected {ONTOLOGY_SCHEMA_VERSION!r}"
        )

    graph = Graph()
    graph.bind("skos", SKOS)
    graph.bind("nomos", NOMOS)

    scheme = _iri(f"{BASE}{pack_id}/scheme")
    graph.add((scheme, RDF.type, SKOS.ConceptScheme))
    graph.add((scheme, SKOS.prefLabel, Literal(pack_id, lang="en")))

    for axis in ontology.get("orthogonality", {}).get("disjoint_axes", []) or []:
        graph.add((scheme, NOMOS.disjointAxis, _iri(f"{BASE}{pack_id}/axis/{axis}")))

    # Labels are keyed by term id across every axis block of the vocabulary file.
    labels: dict[str, str] = {}
    for key, entries in vocabulary.items():
        if not isinstance(entries, list):
            continue
        for entry in entries:
            if isinstance(entry, dict) and entry.get("id"):
                label = entry.get("label_fr") or entry.get("label") or ""
                labels[str(entry["id"])] = str(label)

    for axis in ontology.get("facet_axes", []) or []:
        if not isinstance(axis, dict) or not axis.get("id"):
            raise GateError(f"{pack_id}: an ontology axis has no id")
        axis_id = str(axis["id"])
        axis_node = _iri(f"{BASE}{pack_id}/axis/{axis_id}")
        graph.add((axis_node, RDF.type, NOMOS.FacetAxis))
        graph.add((axis_node, SKOS.prefLabel, Literal(axis_id, lang="en")))
        if axis.get("root"):
            graph.add((axis_node, NOMOS.bfoRoot, _iri(str(axis["root"]))))
        if axis.get("iof_class"):
            graph.add((axis_node, NOMOS.iofClass, _iri(str(axis["iof_class"]))))

        for term in axis.get("terms", []) or []:
            if not isinstance(term, dict) or not term.get("id"):
                raise GateError(f"{pack_id}: a term on axis {axis_id!r} has no id")
            term_id = str(term["id"])
            concept = _iri(f"{BASE}{pack_id}/concept/{axis_id}/{term_id}")
            graph.add((concept, RDF.type, SKOS.Concept))
            graph.add((concept, SKOS.inScheme, scheme))
            graph.add((concept, NOMOS.onAxis, axis_node))
            graph.add((concept, NOMOS.termId, Literal(term_id, datatype=XSD.string)))
            if term_id in labels:
                graph.add((concept, SKOS.prefLabel, Literal(labels[term_id], lang="fr")))
            maps_to = term.get("maps_to") or {}
            if maps_to.get("bfo"):
                graph.add((concept, NOMOS.bfoMapping, _iri(str(maps_to["bfo"]))))
            if maps_to.get("iof_core"):
                graph.add((concept, NOMOS.iofMapping, _iri(str(maps_to["iof_core"]))))

    return graph


def validate_pack(graph: Graph, shapes_path: Path) -> tuple[bool, str]:
    conforms, _, text = shacl_validate(
        graph,
        shacl_graph=str(shapes_path),
        shacl_graph_format="turtle",
        # pySHACL 0.40 evaluates sh:sparql either way (a mutation turning this
        # off leaves the orthogonality test green). Keep the explicit mode as
        # part of this engine invocation; no cross-engine equivalence is claimed.
        advanced=True,
        inference="none",  # no reasoning: the verdict is on the bytes as written
        abort_on_first=False,
        meta_shacl=False,
    )
    return conforms, text


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=".", help="Repository root.")
    parser.add_argument("--pack", default=None, help="Validate only this pack_id.")
    parser.add_argument("--shapes", default=str(DEFAULT_SHAPES), help="SHACL shapes graph.")
    parser.add_argument(
        "--emit-ttl",
        default=None,
        help="Directory to write each pack's graph as Turtle, for inspection.",
    )
    parser.add_argument("--quiet", action="store_true", help="Summary line only.")
    args = parser.parse_args()

    root = Path(args.root).resolve()
    shapes_path = Path(args.shapes)
    if not shapes_path.is_absolute():
        shapes_path = root / shapes_path
    if not shapes_path.is_file():
        print(f"facet-shacl: NOT VALIDATED — shapes not found at {args.shapes}", file=sys.stderr)
        return 2

    try:
        packs = discover_packs(root, args.pack)
    except GateError as exc:
        print(f"facet-shacl: NOT VALIDATED — {exc}", file=sys.stderr)
        return 2

    failures: list[str] = []
    for pack_id, manifest in packs:
        try:
            graph = build_graph(root, pack_id, manifest)
        except GateError as exc:
            print(f"facet-shacl: NOT VALIDATED — {exc}", file=sys.stderr)
            return 2

        if args.emit_ttl:
            out_dir = Path(args.emit_ttl)
            if not out_dir.is_absolute():
                out_dir = root / out_dir
            out_dir.mkdir(parents=True, exist_ok=True)
            (out_dir / f"{pack_id}.ttl").write_bytes(graph.serialize(format="turtle").encode())

        conforms, report = validate_pack(graph, shapes_path)
        concepts = len(set(graph.subjects(RDF.type, SKOS.Concept)))
        axes = len(set(graph.subjects(RDF.type, NOMOS.FacetAxis)))
        if conforms:
            if not args.quiet:
                print(f"  pass  {pack_id}: {axes} axis(es), {concepts} concept(s) conform")
        else:
            failures.append(pack_id)
            print(f"  FAIL  {pack_id}", file=sys.stderr)
            print(report, file=sys.stderr)

    if failures:
        print(
            f"facet-shacl: FAIL — {len(failures)} pack(s) violate the shapes: "
            + ", ".join(failures),
            file=sys.stderr,
        )
        return 1

    print(f"facet-shacl: OK — {len(packs)} pack(s) conform to {args.shapes}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

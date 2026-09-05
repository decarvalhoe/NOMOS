#!/usr/bin/env python3
"""#643 — static SKOS authoring → deterministic build → verifiable distribution.

Authoring sources (versioned, explicit):
  * specs/facet-vocabularies/core-facets.authoring.yaml — the CORE closed axes:
    IRIs, labels, definitions. Its value sets must equal specs/generated/
    facets-vocab.json (owned by specs/facets.cue); any drift refuses the build.
  * docs/regulated/domain-packs/<pack>/{facet-ontology,*-vocabulary}.yaml — the
    packs' OPEN axes, exactly as the SHACL gate (scripts/facet_shacl_gate.py)
    already reads them. The distributed graph IS the validated graph.

Distribution (specs/generated/skos/, committed, byte-deterministic):
  <name>.skos.ttl   Turtle, emitted by a canonical serializer (sorted triples,
                    fixed prefixes) — rdflib's serializer order is not a contract.
  <name>.skos.jsonld  expanded JSON-LD, sorted keys.
  index.json        every graph: IRIs, files, media types, sha256, counts.
  mappings.json     IRI → file + Content-Type for a static host (no server logic).

Modes:
  (default)   build into specs/generated/skos/
  --check     rebuild into a temp dir; any byte differing from the committed
              distribution → exit 4 (names the files).
  --verify    round-trip: parse each committed .ttl and .jsonld back with rdflib,
              require isomorphism with the freshly built graph, SHACL-validate the
              pack graphs with the shipped shapes, check index hashes and that
              every concept IRI has a mapping → exit 1 on any loss.
  --out DIR   build somewhere else.

What this does NOT claim: no Skosmos/VocBench deployment (they are possible
clients of these files), no ISO 25964 conformance, no ontological correctness of
the BFO/IOF anchors — only that the distributed bytes are exactly the validated
graph and that regeneration reproduces them.
"""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
import sys
import tempfile
from pathlib import Path
from typing import Any

import yaml

try:
    from pyshacl import validate as shacl_validate
    from rdflib import Graph, Literal, Namespace, URIRef
    from rdflib.compare import to_isomorphic
    from rdflib.namespace import RDF, SKOS, XSD
except ImportError as exc:  # pragma: no cover
    print(f"skos-distribution: NOT BUILT — pyshacl/rdflib unavailable ({exc})", file=sys.stderr)
    raise SystemExit(2) from exc

ROOT_DEFAULT = Path(__file__).resolve().parents[1]
AUTHORING = Path("specs/facet-vocabularies/core-facets.authoring.yaml")
FACETS_VOCAB = Path("specs/generated/facets-vocab.json")
OUT_DIR = Path("specs/generated/skos")
SHAPES = Path("specs/shacl/facet-ontology.shapes.ttl")
PACKS_DIR = Path("docs/regulated/domain-packs")
AUTHORING_SCHEMA = "nomos-skos-authoring-v1"
INDEX_SCHEMA = "nomos-skos-distribution-index-v1"
NOMOS = Namespace("https://nomos.dev/ns/facets#")
PREFIXES = {
    "skos": str(SKOS),
    "rdf": str(RDF),
    "xsd": str(XSD),
    "nomos": str(NOMOS),
}
CLAIM_BOUNDARY = (
    "Deterministic distribution of the validated facet vocabularies as static files. "
    "Says nothing about any live SKOS service, ISO 25964 conformance, or the ontological "
    "correctness of the BFO/IOF anchors."
)


class BuildError(RuntimeError):
    pass


def load_gate(root: Path):
    spec = importlib.util.spec_from_file_location("facet_shacl_gate", root / "scripts" / "facet_shacl_gate.py")
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


# ---------------------------------------------------------------- core graph

def load_authoring(root: Path) -> dict[str, Any]:
    try:
        doc = yaml.safe_load((root / AUTHORING).read_text(encoding="utf-8"))
    except (OSError, yaml.YAMLError) as exc:
        raise BuildError(f"{AUTHORING}: unreadable authoring source: {exc}") from exc
    if not isinstance(doc, dict) or doc.get("schema_version") != AUTHORING_SCHEMA:
        raise BuildError(f"{AUTHORING}: schema_version must be {AUTHORING_SCHEMA!r}")
    scheme = doc.get("scheme") or {}
    for key in ("iri", "base", "pref_label_en", "source_of_truth"):
        if not scheme.get(key):
            raise BuildError(f"{AUTHORING}: scheme.{key} is required")
    if not isinstance(doc.get("axes"), list) or not doc["axes"]:
        raise BuildError(f"{AUTHORING}: axes must be a non-empty list")
    return doc


def check_core_drift(root: Path, authoring: dict[str, Any]) -> None:
    """The authoring file and the CUE-derived vocabulary must agree exactly."""
    vocab = json.loads((root / FACETS_VOCAB).read_text(encoding="utf-8"))
    cue_axes = {axis: list(values) for axis, values in vocab["scalar_axes"].items()}
    authored = {}
    for axis in authoring["axes"]:
        if not axis.get("id") or not isinstance(axis.get("concepts"), list):
            raise BuildError(f"{AUTHORING}: an axis lacks id or concepts")
        ids = [c.get("id") for c in axis["concepts"]]
        if any(not i for i in ids):
            raise BuildError(f"{AUTHORING}: axis {axis['id']} has a concept without id")
        if len(set(ids)) != len(ids):
            raise BuildError(f"{AUTHORING}: axis {axis['id']} has duplicate concept ids")
        for c in axis["concepts"]:
            if not c.get("pref_label_en"):
                raise BuildError(f"{AUTHORING}: concept {axis['id']}/{c['id']} has no pref_label_en")
        authored[axis["id"]] = ids
    if set(authored) != set(cue_axes):
        raise BuildError(
            f"core axes drift: authoring has {sorted(authored)}, CUE has {sorted(cue_axes)}"
        )
    for axis, values in cue_axes.items():
        if set(authored[axis]) != set(values):
            missing = sorted(set(values) - set(authored[axis]))
            extra = sorted(set(authored[axis]) - set(values))
            raise BuildError(
                f"core axis {axis!r} drift: missing in authoring {missing}, not in CUE {extra}"
            )


def build_core_graph(authoring: dict[str, Any]) -> Graph:
    g = Graph()
    scheme_doc = authoring["scheme"]
    base = str(scheme_doc["base"])
    scheme = URIRef(str(scheme_doc["iri"]))
    g.add((scheme, RDF.type, SKOS.ConceptScheme))
    g.add((scheme, SKOS.prefLabel, Literal(str(scheme_doc["pref_label_en"]), lang="en")))
    if scheme_doc.get("definition_en"):
        g.add((scheme, SKOS.definition, Literal(str(scheme_doc["definition_en"]), lang="en")))
    for axis in authoring["axes"]:
        axis_id = str(axis["id"])
        coll = URIRef(f"{base}{axis_id}")
        g.add((coll, RDF.type, SKOS.Collection))
        g.add((coll, SKOS.inScheme, scheme))
        g.add((coll, SKOS.prefLabel, Literal(str(axis.get("pref_label_en") or axis_id), lang="en")))
        g.add((coll, NOMOS.axisId, Literal(axis_id, datatype=XSD.string)))
        for concept_doc in axis["concepts"]:
            cid = str(concept_doc["id"])
            concept = URIRef(f"{base}{axis_id}/{cid}")
            g.add((concept, RDF.type, SKOS.Concept))
            g.add((concept, SKOS.inScheme, scheme))
            g.add((concept, SKOS.topConceptOf, scheme))
            g.add((coll, SKOS.member, concept))
            g.add((concept, NOMOS.onAxis, coll))
            g.add((concept, NOMOS.termId, Literal(cid, datatype=XSD.string)))
            g.add((concept, SKOS.notation, Literal(cid, datatype=XSD.string)))
            g.add((concept, SKOS.prefLabel, Literal(str(concept_doc["pref_label_en"]), lang="en")))
            if concept_doc.get("pref_label_fr"):
                g.add((concept, SKOS.prefLabel, Literal(str(concept_doc["pref_label_fr"]), lang="fr")))
            if concept_doc.get("definition_en"):
                g.add((concept, SKOS.definition, Literal(str(concept_doc["definition_en"]), lang="en")))
    return g


# ---------------------------------------------------------------- pack graphs

def pack_manifests(root: Path) -> list[tuple[str, dict[str, Any]]]:
    packs = []
    for manifest_path in sorted((root / PACKS_DIR).glob("*/pack.yaml")):
        manifest = yaml.safe_load(manifest_path.read_text(encoding="utf-8"))
        if isinstance(manifest, dict) and manifest.get("pack_id"):
            packs.append((str(manifest["pack_id"]), manifest))
    return packs


def build_all(root: Path) -> dict[str, Graph]:
    gate = load_gate(root)
    authoring = load_authoring(root)
    check_core_drift(root, authoring)
    graphs = {"core-facets": build_core_graph(authoring)}
    for pack_id, manifest in pack_manifests(root):
        try:
            graphs[pack_id] = gate.build_graph(root, pack_id, manifest)
        except gate.GateError as exc:
            raise BuildError(str(exc)) from exc
    return graphs


# ---------------------------------------------------------------- serializers

def _turtle_term(term, prefixes: dict[str, str]) -> str:
    if isinstance(term, URIRef):
        s = str(term)
        for pfx, ns in prefixes.items():
            if s.startswith(ns) and _is_pname_local(s[len(ns):]):
                return f"{pfx}:{s[len(ns):]}"
        return f"<{s}>"
    if isinstance(term, Literal):
        text = json.dumps(str(term), ensure_ascii=False)  # JSON escaping is valid Turtle escaping
        if term.language:
            return f"{text}@{term.language}"
        if term.datatype and term.datatype != XSD.string:
            return f"{text}^^{_turtle_term(term.datatype, prefixes)}"
        if term.datatype == XSD.string:
            return f"{text}^^xsd:string"
        return text
    raise BuildError(f"blank nodes are not distributed: {term!r}")


def _is_pname_local(local: str) -> bool:
    return bool(local) and all(ch.isalnum() or ch in "_-." for ch in local) and not local.startswith(".") and not local.endswith(".")


def to_canonical_turtle(g: Graph, prefixes: dict[str, str]) -> str:
    used = {pfx: ns for pfx, ns in prefixes.items()}
    lines = [f"@prefix {pfx}: <{ns}> ." for pfx, ns in sorted(used.items())]
    lines.append("")
    triples = sorted((str(s), str(p), _sort_key(o)) for s, p, o in g)
    by_subject: dict[str, list] = {}
    for s, p, o in g:
        by_subject.setdefault(str(s), []).append((str(p), o))
    for subject in sorted(by_subject):
        lines.append(_turtle_term(URIRef(subject), used))
        preds = sorted(by_subject[subject], key=lambda po: (po[0], _sort_key(po[1])))
        for i, (p, o) in enumerate(preds):
            end = " ." if i == len(preds) - 1 else " ;"
            pred = "a" if p == str(RDF.type) else _turtle_term(URIRef(p), used)
            lines.append(f"  {pred} {_turtle_term(o, used)}{end}")
        lines.append("")
    del triples
    return "\n".join(lines).rstrip("\n") + "\n"


def _sort_key(o) -> tuple:
    if isinstance(o, URIRef):
        return (0, str(o), "", "")
    return (1, str(o), o.language or "", str(o.datatype or ""))


def to_expanded_jsonld(g: Graph) -> str:
    nodes: dict[str, dict[str, Any]] = {}
    for s, p, o in g:
        node = nodes.setdefault(str(s), {"@id": str(s)})
        if p == RDF.type:
            node.setdefault("@type", []).append(str(o))
            continue
        if isinstance(o, URIRef):
            value: dict[str, Any] = {"@id": str(o)}
        else:
            value = {"@value": str(o)}
            if o.language:
                value["@language"] = o.language
            elif o.datatype:
                value["@type"] = str(o.datatype)
        node.setdefault(str(p), []).append(value)
    out = []
    for sid in sorted(nodes):
        node = nodes[sid]
        ordered: dict[str, Any] = {"@id": node["@id"]}
        if "@type" in node:
            ordered["@type"] = sorted(node["@type"])
        for key in sorted(k for k in node if k not in ("@id", "@type")):
            ordered[key] = sorted(node[key], key=lambda v: json.dumps(v, sort_keys=True, ensure_ascii=False))
        out.append(ordered)
    return json.dumps(out, indent=2, sort_keys=False, ensure_ascii=False) + "\n"


def sha256_text(text: str) -> str:
    return "sha256:" + hashlib.sha256(text.encode("utf-8")).hexdigest()


def scheme_iri(g: Graph) -> str:
    schemes = sorted(str(s) for s in g.subjects(RDF.type, SKOS.ConceptScheme))
    if len(schemes) != 1:
        raise BuildError(f"graph must have exactly one skos:ConceptScheme, has {len(schemes)}")
    return schemes[0]


def render_distribution(graphs: dict[str, Graph]) -> dict[str, str]:
    """All distributed files as {relative name: text}, deterministic."""
    files: dict[str, str] = {}
    index_entries = []
    mappings: dict[str, dict[str, str]] = {}
    for name in sorted(graphs):
        g = graphs[name]
        ttl = to_canonical_turtle(g, PREFIXES)
        jsonld = to_expanded_jsonld(g)
        files[f"{name}.skos.ttl"] = ttl
        files[f"{name}.skos.jsonld"] = jsonld
        concepts = sorted(str(s) for s in g.subjects(RDF.type, SKOS.Concept))
        collections = sorted(str(s) for s in g.subjects(RDF.type, SKOS.Collection))
        axes = sorted(str(s) for s in g.subjects(RDF.type, NOMOS.FacetAxis))
        scheme = scheme_iri(g)
        entry = {
            "name": name,
            "kind": "core" if name == "core-facets" else "domain-pack",
            "scheme": scheme,
            "files": {
                "turtle": {"path": f"{name}.skos.ttl", "media_type": "text/turtle", "sha256": sha256_text(ttl)},
                "jsonld": {"path": f"{name}.skos.jsonld", "media_type": "application/ld+json", "sha256": sha256_text(jsonld)},
            },
            "triples": len(g),
            "concepts": len(concepts),
            "collections": len(collections),
            "axes": len(axes),
            "concept_iris": concepts,
        }
        index_entries.append(entry)
        for iri in [scheme, *concepts, *collections, *axes]:
            mappings[iri] = {"turtle": f"{name}.skos.ttl", "jsonld": f"{name}.skos.jsonld", "graph": name}
    index = {
        "schema_version": INDEX_SCHEMA,
        "generated_by": "scripts/skos_distribution.py",
        "authoring_sources": [AUTHORING.as_posix(), f"{PACKS_DIR.as_posix()}/<pack>/pack.yaml (ontology + vocabularies files)"],
        "shapes": SHAPES.as_posix(),
        "graphs": index_entries,
        "claim_boundary": CLAIM_BOUNDARY,
    }
    files["index.json"] = json.dumps(index, indent=2, ensure_ascii=False) + "\n"
    files["mappings.json"] = json.dumps(
        {
            "schema_version": "nomos-skos-static-mappings-v1",
            "note": "IRI → file for a static host. Serve the Turtle file as text/turtle and the JSON-LD file as application/ld+json; no server logic is required or claimed.",
            "content_types": {"turtle": "text/turtle", "jsonld": "application/ld+json"},
            "iris": {iri: mappings[iri] for iri in sorted(mappings)},
        },
        indent=2, ensure_ascii=False,
    ) + "\n"
    return files


# ---------------------------------------------------------------- modes

def write_distribution(files: dict[str, str], out_dir: Path) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)
    for name, text in files.items():
        (out_dir / name).write_text(text, encoding="utf-8")


def check(root: Path, files: dict[str, str]) -> list[str]:
    committed = root / OUT_DIR
    drift = []
    for name, text in files.items():
        p = committed / name
        if not p.exists():
            drift.append(f"{name} (missing)")
        elif p.read_text(encoding="utf-8") != text:
            drift.append(name)
    for p in sorted(committed.glob("*")):
        if p.is_file() and p.name not in files and p.name != "README.md":
            drift.append(f"{p.name} (orphan)")
    return drift


def verify(root: Path, graphs: dict[str, Graph]) -> list[str]:
    """Round-trip the COMMITTED files: parse back, compare, validate."""
    committed = root / OUT_DIR
    problems: list[str] = []
    index_path = committed / "index.json"
    if not index_path.exists():
        return [f"{OUT_DIR}/index.json missing — nothing distributed"]
    index = json.loads(index_path.read_text(encoding="utf-8"))
    mappings = json.loads((committed / "mappings.json").read_text(encoding="utf-8"))["iris"]
    listed = {e["name"] for e in index.get("graphs", [])}
    if listed != set(graphs):
        problems.append(f"index lists {sorted(listed)}, sources build {sorted(graphs)}")
    for entry in index.get("graphs", []):
        name = entry["name"]
        expected = graphs.get(name)
        if expected is None:
            continue
        iso_expected = to_isomorphic(expected)
        for fmt, rdf_format in (("turtle", "turtle"), ("jsonld", "json-ld")):
            meta = entry["files"][fmt]
            path = committed / meta["path"]
            if not path.exists():
                problems.append(f"{name}: {meta['path']} missing")
                continue
            text = path.read_text(encoding="utf-8")
            if sha256_text(text) != meta["sha256"]:
                problems.append(f"{name}: {meta['path']} sha256 does not match index")
            parsed = Graph()
            try:
                parsed.parse(data=text, format=rdf_format)
            except Exception as exc:  # noqa: BLE001 — any parse failure is a loss
                problems.append(f"{name}: {meta['path']} does not parse as {fmt}: {exc}")
                continue
            if to_isomorphic(parsed) != iso_expected:
                lost = set(iso_expected) - set(to_isomorphic(parsed))
                gained = set(to_isomorphic(parsed)) - set(iso_expected)
                problems.append(
                    f"{name}: {meta['path']} is not the built graph — {len(lost)} triple(s) lost, {len(gained)} foreign"
                )
        # SHACL on the distributed pack graph (the core vocabulary has no pack shapes).
        if entry["kind"] == "domain-pack":
            g = Graph()
            g.parse(data=(committed / entry["files"]["turtle"]["path"]).read_text(encoding="utf-8"), format="turtle")
            conforms, _, report = shacl_validate(
                g, shacl_graph=str(root / SHAPES), shacl_graph_format="turtle",
                advanced=True, inference="none", abort_on_first=False, meta_shacl=False,
            )
            if not conforms:
                problems.append(f"{name}: distributed graph does not conform to the shapes: {report.strip()[:300]}")
        for iri in entry["concept_iris"]:
            m = mappings.get(iri)
            if not m or m.get("graph") != name:
                problems.append(f"{name}: concept {iri} has no static mapping")
        # every concept keeps a prefLabel and a termId
        for c in expected.subjects(RDF.type, SKOS.Concept):
            if (c, SKOS.prefLabel, None) not in expected:
                problems.append(f"{name}: concept {c} has no skos:prefLabel")
            if (c, NOMOS.termId, None) not in expected:
                problems.append(f"{name}: concept {c} has no nomos:termId")
    return problems


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--root", default=str(ROOT_DEFAULT))
    parser.add_argument("--out", default="", help="write the distribution here instead of specs/generated/skos")
    parser.add_argument("--check", action="store_true", help="compare a fresh build with the committed distribution (exit 4 on drift)")
    parser.add_argument("--verify", action="store_true", help="round-trip and validate the committed distribution (exit 1 on loss)")
    args = parser.parse_args()
    root = Path(args.root).resolve()
    try:
        graphs = build_all(root)
        files = render_distribution(graphs)
    except BuildError as exc:
        print(f"skos-distribution: REFUSED — {exc}", file=sys.stderr)
        return 1
    rc = 0
    if args.check:
        drift = check(root, files)
        if drift:
            print("skos-distribution: DRIFT — committed distribution is not a fresh build:", file=sys.stderr)
            for d in drift:
                print("  -", d, file=sys.stderr)
            rc = 4
        else:
            print(f"skos-distribution: check OK — {len(files)} file(s) byte-identical to a fresh build")
    if args.verify:
        problems = verify(root, graphs)
        if problems:
            print("skos-distribution: VERIFY FAILED:", file=sys.stderr)
            for p in problems:
                print("  -", p, file=sys.stderr)
            rc = rc or 1
        else:
            total = sum(len(g) for g in graphs.values())
            print(f"skos-distribution: verify OK — {len(graphs)} graph(s), {total} triple(s) round-trip, pack graphs conform")
    if not args.check and not args.verify:
        out = Path(args.out) if args.out else root / OUT_DIR
        write_distribution(files, out)
        print(f"skos-distribution: wrote {len(files)} file(s) to {out}")
    return rc


if __name__ == "__main__":
    raise SystemExit(main())

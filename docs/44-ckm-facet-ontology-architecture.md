# CKM Facet Ontology Architecture

NOMOS facet axes are grounded as an ontology alignment pattern, not a horizontal tag layer. CKM-12 keeps the runtime contract additive: atoms and chunks continue to accept facets only under open `metadata.facets`, while `specs/facet-ontology.cue` records how a pack maps those facets to BFO, IOF Core, and domain terms.

## Alignment Pattern

- BFO anchors the upper-level distinction between continuants and occurrents.
- IOF Core anchors industrial information content entities and processes.
- Domain packs add SKOS/OWL terms without changing the base atomization spine.
- OWL `disjointUnionOf` is used as the declared orthogonality model for facet axes that must not share terms.

## ODPs

The core ODPs are:

- `obligation`: a normative information-content pattern, typically surfaced through `nature=obligation`, with provenance and trust tier required for certified claims.
- `process`: an occurrent activity pattern, typically surfaced through `activity`, with scope and provenance required.
- `evidence`: an evidential information-content pattern, surfaced through `nature=evidence`, with provenance and trust tier required.

## Validation Boundary

`scripts/ckm_facet_ontology_validate.py` checks local `owl:disjointUnionOf` term orthogonality for declared axes. It does not perform OWL reasoning, import external ontologies, certify BFO/IOF legal adequacy, or prove that a domain pack is complete.

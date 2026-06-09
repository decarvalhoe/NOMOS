# Specs

`specs/` contains the machine-readable contracts that keep Nomos evidence reproducible.

## Current Contract Families

- project manifest;
- source manifest;
- canonical matrix;
- adapter manifest;
- corpus evidence;
- RBOK lawbook feed;
- RBOK runtime import contract;
- verdict taxonomy;
- fidelity AST;
- TOC artifact;
- atomization spine;
- optional CKM facets;
- canonical knowledge bundle;
- provenance gate;
- ALCOA evidence;
- AI/RAG controls;
- validation inventory;
- evidence contract.

## Release Rule

Schema changes are evidence-affecting changes. They require tests, documentation updates, and a migration note when the change can affect generated artifacts or customer validation records.

## CKM Facets

`facets.cue` defines optional multidimensional facet shapes for `#Atom.metadata`
and `#Chunk.metadata`. Existing atoms and chunks without facets remain valid
against `atomization-spine.cue`.

Facet values use core controlled axes for nature, scope level, trust tier,
provenance, confidentiality, and applicability. Domain packs may add SKOS-backed
terms for `discipline_role`, `activity`, and extensions. Promoting facets from
metadata into a required top-level `#Facets` contract requires a future
`schema_version` bump, migration note, and compatibility fixture.

## Alpha Boundary

The schemas are usable for alpha pilots and internal validation. They may still change before `v1.0`; consumers should pin Nomos versions and retain generated artifacts with their schema version.

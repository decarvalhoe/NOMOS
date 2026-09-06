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
- optional CKM facet ontology alignment;
- optional point-in-time regulatory atom metadata;
- optional CKM facets;
- optional CKM knowledge lens retrieval scoping;
- optional CKM canon promotion guardrails;
- optional CKM business-bible domain fixture;
- canonical knowledge bundle;
- provenance gate;
- ALCOA evidence;
- AI/RAG controls;
- validation inventory;
- evidence contract.

## Stability Registry

`contract-registry.yaml` declares every contract file here with its stability (`stable`, `experimental`, `deprecated`), its version, its sha256, its fixtures, its Go readers and its compatibility fixtures. `nomos contracts status --repo-root .` verifies it in CI: a stable contract whose bytes change without an accepted bump is red (`nomos contracts status --accept <id> --new-version <v>` records a deliberate bump once the file declares the new version). Stability is declared and verified, not inferred; it says nothing about semantic correctness.

## Release Rule

Schema changes are evidence-affecting changes. They require tests, documentation updates, and a migration note when the change can affect generated artifacts or customer validation records.

## CKM Facets

`facets.cue` defines optional multidimensional facet shapes for `#Atom.metadata`
and `#Chunk.metadata`. Existing atoms and chunks without facets remain valid
against `atomization-spine.cue`.

`facet-ontology.cue` defines the optional CKM-12 BFO -> IOF Core -> domain-pack
alignment pattern for facet axes, including obligation/process/evidence ODPs
and declared OWL `disjointUnionOf` orthogonality. This is a pack/design contract
only; it does not tighten `#Atom` or `#Chunk`.

`knowledge-lens.cue` defines the optional CKM-02 retrieval-scope lens contract
for deterministic pre-generation filtering over facet applicability metadata.

`canon-promotion.cue` defines the optional CKM-03 guardrail contract for
customer-confidential user-promoted canon and its certificate evidence.

`domain-cartography.cue` defines the optional consumer-facing domain cartography
contract (docs/48 §2.1): what a domain actually holds, sub-corpus by sub-corpus,
on four layers verified independently; a layer nobody verified says so and may
carry no number, a phantom domain owns no collection, a transversal base is
shared and never duplicated. It measures no retrieval or answer quality.

The business-operations example profile demonstrates the same CKM mechanics for
non-AEC business bibles through `business_bible` sources and `nature: metier`
facet metadata.

Facet values use core controlled axes for nature, scope level, trust tier,
provenance, confidentiality, and applicability. Domain packs may add SKOS-backed
terms for `discipline_role`, `activity`, and extensions. Promoting facets from
metadata into a required top-level `#Facets` contract requires a future
`schema_version` bump, migration note, and compatibility fixture.

## Alpha Boundary

The schemas are usable for alpha pilots and internal validation. They may still change before `v1.0`; consumers should pin Nomos versions and retain generated artifacts with their schema version.

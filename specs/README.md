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
- optional point-in-time regulatory atom metadata;
- provenance gate;
- ALCOA evidence;
- AI/RAG controls;
- validation inventory;
- evidence contract.

## Release Rule

Schema changes are evidence-affecting changes. They require tests, documentation updates, and a migration note when the change can affect generated artifacts or customer validation records.

## Alpha Boundary

The schemas are usable for alpha pilots and internal validation. They may still change before `v1.0`; consumers should pin Nomos versions and retain generated artifacts with their schema version.

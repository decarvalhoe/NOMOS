# Praxis Atom Mapping — Nomos-to-Praxis Evidence Contract

Status: planned
Owner: Nomos core team
Effective: after Nomos atom pipeline reaches `verified` status

## Purpose

This document defines how certified Nomos atoms map to Praxis evidence contracts. It specifies the interface between Nomos (producer) and Praxis (consumer) without prescribing Praxis internals.

Nomos does not own Praxis. This mapping is a boundary contract. Praxis cannot certify or compensate for atoms that Nomos cannot yet produce and validate (see docs/25-regulated-by-design-structure.md).

## Claim Boundary

This mapping is a design document only. It does not claim that the integration is implemented, validated, or production-ready. The current status is `planned`.

## Prerequisite: Nomos Atom Stabilization

The mapping activates only after the following Nomos conditions are met:

| Condition | Gate | Current Status |
|-----------|------|----------------|
| Atom extraction produces stable IDs | ci | implemented |
| Atoms carry content hashes (SHA-256) | ci | implemented |
| Review state lifecycle enforced | ci | implemented |
| Only `approved` atoms enter feeds | rbok-lawbook-e2e | implemented |
| Lawbook feed passes release gate | rbok-lawbook-release-gate | implemented |
| Traceability matrix projects from atoms | ci | implemented |
| Atom-to-engine import contract versioned | ci | implemented |

Until all conditions reach `verified`, Praxis must treat Nomos atoms as `not_qualified`.

## Mapping Interface

### Atom Fields Exposed to Praxis

| Nomos Atom Field | Praxis Contract Field | Description |
|------------------|----------------------|-------------|
| `id` | `nomos_atom_id` | Stable atom identifier |
| `canonical_ref` | `canonical_ref` | Slug-based reference path |
| `type` | `atom_type` | rule, clause, definition, list_item, table, code_block, meta |
| `text` | `content` | Atom text content |
| `content_hash` | `content_hash` | SHA-256 of atom content |
| `review_state` | `certification_state` | Only `approved` atoms cross the boundary |
| `domain` | `domain` | Business domain |
| `source_span.file` | `source_file` | Origin file path |
| `source_span.start_line` | `source_line` | Origin line number |

### Fields NOT Exposed to Praxis

| Nomos Field | Reason |
|-------------|--------|
| `block_id` | Internal AST reference, not stable across versions |
| `parent_id` | Internal hierarchy, Praxis builds its own tree |
| `depth` | Structural metadata, Praxis derives from its own model |

### Evidence Contract Format

Praxis consumes atoms via the lawbook feed JSON format:

```json
{
  "schema_version": "0.1.0",
  "feed_id": "feed-<hash>",
  "domain": "<domain>",
  "generated_at": "<RFC3339>",
  "source_path": "<path>",
  "source_hash": "<sha256>",
  "total_atoms": 42,
  "certified_atoms": 38,
  "rejected_atoms": 4,
  "entries": [
    {
      "node_id": "N-<HASH>",
      "atom_id": "<nomos-atom-id>",
      "node_type": "article",
      "canonical_ref": "<ref>",
      "text": "<content>",
      "review_state": "approved",
      "source_hash": "<sha256>",
      "domain": "<domain>",
      "status": "active",
      "priority": "high"
    }
  ]
}
```

### Engine Import Contract

For direct database import, Praxis can consume the engine import format:

```json
{
  "contract_version": "import-contract-v1",
  "generated_at": "<RFC3339>",
  "domain": "<domain>",
  "total_atoms": 42,
  "certified_atoms": 38,
  "nodes": [
    {
      "external_id": "<nomos-atom-id>",
      "document_external_id": "feed-doc-<hash>",
      "node_type": "article",
      "canonical_ref": "<ref>",
      "content": "<text>",
      "source_path": "<path>",
      "source_hash": "<sha256>",
      "status": "active"
    }
  ]
}
```

## Praxis Responsibilities

Praxis owns the following and Nomos does not interfere:

1. **Runtime scenarios** — Praxis builds its own test scenarios from received atoms.
2. **Invariant execution** — Praxis decides which atoms become runtime invariants.
3. **Evidence retention** — Praxis stores execution evidence per its own ALCOA+ policy.
4. **CAPA records** — Praxis creates nonconformity records when invariants fail.
5. **Feedback loop** — Praxis reports runtime failures back to Nomos for atom amendment.

## Nomos Responsibilities

1. **Atom stability** — Nomos guarantees that atom IDs do not change for the same content.
2. **Content hashing** — Every atom carries a SHA-256 hash for tamper detection.
3. **Certification gate** — Only `approved` atoms enter the feed. Draft, pending, and rejected atoms are filtered.
4. **Schema versioning** — Feed and engine import formats are versioned. Breaking changes increment the major version.
5. **Read-only source** — Nomos guarantees that feed generation does not modify the source corpus.

## Status Mapping

| Nomos Review State | Praxis Receives | Praxis Action |
|--------------------|----------------|---------------|
| `approved` | Yes | Import as active evidence |
| `amended` | No (until re-approved) | Ignore until new approval |
| `pending` | No | Await review completion |
| `draft` | No | Not ready for consumption |
| `rejected` | No | Never enters feed |

## Versioning and Breaking Changes

The contract version (`schema_version` for feeds, `contract_version` for engine import) follows these rules:

- Patch: new optional fields, documentation updates.
- Minor: new atom types, new node types, new optional entry fields.
- Major: removed fields, renamed fields, changed semantics of existing fields.

Praxis must pin to a major version and validate the contract version before import.

## Authoritative Regulated Activation Checklist

Technical schema/import/reject fixtures may run first on synthetic or
`not_qualified_external_input` artifacts. They carry no regulated weight.
Before **authoritative regulated activation**:

- [ ] All Nomos prerequisite conditions reach `verified` status
- [ ] Praxis has a pinned contract version in its import configuration
- [ ] A test feed has been successfully imported into Praxis staging
- [ ] Praxis import validates atom content hashes
- [ ] Praxis import rejects atoms with `review_state != approved`
- [ ] Feedback channel from Praxis to Nomos is operational
- [ ] Shared responsibility matrix is signed by both product owners

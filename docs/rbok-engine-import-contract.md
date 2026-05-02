# RBOK Engine Import Handoff Contract

> Related: RBOK #2168

This contract defines the mapping between the Nomos corpus extraction output
(RBOKLawDocument, RBOKLawNode) and the RBOK Engine persistence layer. It
governs how parsed legal/regulatory content is projected into the engine's
import tables and how governance, citations, and RAG metadata are handled
at the boundary.

## Entity Mapping

| Corpus Model        | Engine Table       | Relationship             |
|---------------------|--------------------|--------------------------|
| RBOKLawDocument     | rbok_documents     | 1:1 per source document  |
| RBOKLawNode         | rbok_nodes         | 1:1 per structural node  |
| Revisions           | rbok_revisions     | 1:N per document         |
| Citations           | Runtime IA         | Resolved at query time   |
| Governance metadata | Blocking publish   | Gate before availability |
| RAG metadata        | Retrieval index    | Indexed for similarity   |

## Import Projection Schema

The import projection is the canonical flat structure that the engine expects
for each node. All fields are required unless marked optional.

### rbok_nodes Import Projection

| Field                    | Type     | Required | Description                                                      |
|--------------------------|----------|----------|------------------------------------------------------------------|
| external_id              | string   | yes      | Globally unique node identifier (corpus-side stable ID)          |
| document_external_id     | string   | yes      | FK to parent document's external_id in rbok_documents            |
| parent_external_id       | string   | no       | FK to parent node's external_id (null for root nodes)            |
| node_type                | string   | yes      | Structural type: `article`, `section`, `chapter`, `title`, `annex`, `definition` |
| display_ref              | string   | yes      | Human-readable reference (e.g., "Art. L.113-2", "Section 3.1")  |
| canonical_ref            | string   | yes      | Machine-readable canonical reference for deduplication           |
| depth                    | integer  | yes      | Nesting depth in document tree (0 = root)                        |
| heading_level            | integer  | no       | Markdown heading level if extracted from MD (1-6)                |
| structure_only           | boolean  | yes      | True if node is structural container with no own content         |
| priority                 | integer  | yes      | Import priority for conflict resolution (higher wins)            |
| status                   | string   | yes      | Lifecycle: `active`, `deprecated`, `abrogated`, `pending`        |
| source_path              | string   | yes      | Relative path in corpus to source file                           |
| source_hash              | string   | yes      | SHA-256 hash of source content at extraction time                |
| content                  | text     | no       | Full text content of the node (null for structure_only=true)     |
| aliases                  | string[] | no       | Alternative references or historical identifiers                 |

### rbok_documents Import Projection

| Field              | Type     | Required | Description                                          |
|--------------------|----------|----------|------------------------------------------------------|
| external_id        | string   | yes      | Globally unique document identifier                  |
| title              | string   | yes      | Document title                                       |
| document_type      | string   | yes      | `law`, `regulation`, `circular`, `guide`, `internal` |
| jurisdiction       | string   | yes      | Jurisdiction code (e.g., `FR`, `EU`)                 |
| publication_date   | date     | no       | Official publication date                            |
| effective_date     | date     | no       | Date when content becomes applicable                 |
| source_path        | string   | yes      | Corpus path to root source file                      |
| source_hash        | string   | yes      | SHA-256 of source file                               |
| version            | string   | yes      | Document version from governance metadata            |
| owner              | string   | yes      | Responsible team/person                              |
| domain             | string   | yes      | Business domain tag                                  |
| status             | string   | yes      | `active`, `deprecated`, `draft`, `archived`          |

### rbok_revisions Import Projection

| Field              | Type     | Required | Description                                          |
|--------------------|----------|----------|------------------------------------------------------|
| external_id        | string   | yes      | Unique revision identifier                           |
| document_external_id | string | yes      | FK to rbok_documents                                 |
| revision_number    | integer  | yes      | Monotonically increasing revision counter            |
| created_at         | datetime | yes      | Timestamp of revision creation                       |
| created_by         | string   | yes      | Author or system that produced the revision          |
| change_summary     | string   | no       | Human-readable description of changes                |
| source_hash        | string   | yes      | Hash of content at this revision                     |
| node_count         | integer  | yes      | Number of nodes in this revision                     |

## Governance Gate (Blocking Publish)

Before any imported content becomes available to end users or the IA runtime:

1. **Lockfile check**: All source files must appear as `approved` in the corpus
   lockfile (`corpus.lock`). Unapproved or rejected content blocks publication.

2. **Governance metadata completeness**: Documents missing `version`, `owner`,
   `status`, or `domain` produce `corpus_partial` findings and block publish
   until resolved.

3. **Reference resolution**: All `canonical_ref` values must resolve to known
   nodes. Dangling references produce `corpus_unresolved_ref` findings and
   block strict-mode publish.

4. **Hash stability**: The `source_hash` at import time must match the lockfile
   hash. Any mismatch indicates an unapproved content change and blocks publish.

### Governance Status Matrix

| Condition                        | Finding Code            | Blocks Publish |
|----------------------------------|-------------------------|----------------|
| Missing version/owner/status     | corpus_partial          | yes            |
| Unapproved source hash           | corpus_unapproved       | yes            |
| Unresolved canonical_ref         | corpus_unresolved_ref   | yes (strict)   |
| Deprecated document still active | corpus_lifecycle_drift  | warning only   |
| Hash mismatch vs lockfile        | corpus_hash_mismatch    | yes            |

## Citations (Runtime IA)

Citations are **not** persisted in the import tables. Instead:

- Each `rbok_node` carries its `canonical_ref` and `display_ref`.
- At query time, the IA runtime resolves citations by matching `canonical_ref`
  values across the node graph.
- Cross-document citations use the format `{document_external_id}#{canonical_ref}`.
- The import contract guarantees that `canonical_ref` is unique within a document
  and stable across revisions (content changes do not alter the ref).

## RAG Metadata (Retrieval)

The following fields are projected into the vector retrieval index:

| Field              | Usage                                              |
|--------------------|----------------------------------------------------|
| external_id        | Chunk identifier for attribution                   |
| canonical_ref      | Faceted filtering by legal reference               |
| display_ref        | Shown in retrieval results for human context       |
| node_type          | Filter by structural type                          |
| domain             | Domain-scoped retrieval                            |
| status             | Exclude deprecated/abrogated from default retrieval|
| content            | Embedded as vector for semantic search             |
| depth              | Boost shallow nodes (more authoritative)           |
| priority           | Secondary ranking signal                           |
| document_external_id | Group chunks by source document                  |

### Embedding Contract

- Only nodes with `structure_only = false` and `content != null` are embedded.
- Embedding granularity: one vector per node (no sub-node chunking at import).
- The retrieval index stores `external_id` as the primary key for attribution
  back to the structured node graph.

## Import Idempotency

- Imports are idempotent: re-importing the same `source_hash` for a given
  `external_id` is a no-op.
- A new `source_hash` for an existing `external_id` creates a new revision
  and updates the active node content.
- Deleted nodes (present in previous revision but absent in new import) are
  marked `status = abrogated`, not physically removed.

## Sequence Diagram

```
Corpus Extraction          Engine Import             Publish Gate
      |                          |                        |
      |-- ExtractionResult ----->|                        |
      |                          |-- upsert documents --->|
      |                          |-- upsert nodes ------->|
      |                          |-- create revision ---->|
      |                          |                        |
      |                          |-- check lockfile ----->|
      |                          |-- check governance --->|
      |                          |-- check refs --------->|
      |                          |                        |
      |                          |<-- publish_ok/block ---|
      |                          |                        |
      |                          |-- index RAG ---------->| (if publish_ok)
```

## Versioning

This contract is versioned as `import-contract-v1`. Breaking changes to field
names, required fields, or governance gate semantics require a major version
bump and migration path documentation.

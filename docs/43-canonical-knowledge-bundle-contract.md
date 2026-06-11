# 43 - Canonical Knowledge Bundle Contract

The CKM bundle is the versioned handoff artifact for downstream consumers such
as Aedifica. It is a read-only product of NOMOS and must be treated as immutable
once published.

## Consumer Mapping

| Bundle path | Consumer table | Notes |
|---|---|---|
| `bundle_id`, `schema_version`, `generated_at` | `knowledge_bundle_versions` | One row per imported bundle version. |
| `feeds[].feed_id` | `knowledge_feeds` | Feed identity and source format. |
| `feeds[].version` (optional) | `knowledge_feeds.version` | Feed content version to pin imports to. Emitter default is deterministic (`<bundle_id>@<generated_at>`); absent on feeds that do not version their payload. |
| `feeds[].jurisdiction` (optional) | `knowledge_feeds.jurisdiction` | Legal scope (`country`/`canton`/`commune`) the feed's nodes apply to. Absent for domain corpora with no jurisdiction. NOMOS only populates it when told (`--country/--canton/--commune`). |
| `feeds[].nodes[].node_id` | `knowledge_nodes` | Primary stable node key. |
| `feeds[].nodes[].text` | `knowledge_nodes.text` | Canonical text consumed by product/RAG surfaces. |
| `feeds[].nodes[].source_path`, `source_hash`, `span` | `knowledge_node_sources` | Citation and audit reconstruction fields. The canonical span form is `start_line`/`end_line` (see Span Form). |
| `feeds[].nodes[].parent_chain` | `knowledge_node_hierarchy` | Store ordered ancestors for navigation and retrieval context. |
| `feeds[].nodes[].facets` | `knowledge_node_facets` | Store axis/value pairs; preserve unknown extension axes as JSON. |
| `rag_metadata[].node_id` | `knowledge_rag_chunks.node_id` | Must resolve to an imported node; orphan metadata blocks import. |
| `trace_manifest` | `knowledge_bundle_trace` | Immutable provenance and publication policy record. |
| `attestation` | `knowledge_bundle_attestations` | Signature/provenance statement; not semantic correctness proof. |

## Import Rules

- Refuse a bundle with no `feeds`.
- Refuse a feed with no `nodes`.
- Refuse duplicate `node_id` values.
- Refuse any `rag_metadata[].node_id` that does not resolve to
  `feeds[].nodes[].node_id`.
- Treat `facets.trust_tier` as a claim boundary, not as a permission to skip
  domain review.

## Span Form

`feeds[].nodes[].span` uses `start_line`/`end_line` (1-based, inclusive) as the
**canonical** form NOMOS emits. `start_byte`/`end_byte`/`locator` are optional
refinements. A consumer that models spans as `start`/`end` must read those
values from `start_line`/`end_line` — they are the source of truth. (Aedifica's
importer accepts both `start`/`end` and `start_line`/`end_line`; NOMOS commits to
the line-span names.)

## Optional Feed Metadata

`feeds[].version` and `feeds[].jurisdiction` are **both optional and additive** —
a feed (and an entire bundle) without them remains contract-valid:

- `version` — the feed content version a consumer pins imports to. The `nomos
  bundle` emitter sets a deterministic default (`<bundle_id>@<generated_at>`) so
  a re-run over the same corpus is byte-identical; override with `--feed-version`.
- `jurisdiction` — `{country?, canton?, commune?}`, the legal scope the feed's
  nodes apply to. NOMOS has no native jurisdiction concept and only populates
  this when explicitly told (`--country/--canton/--commune`). Domain corpora with
  no jurisdiction (AI governance, GxP, …) omit it entirely.

A consumer that derives jurisdiction from its own source-of-record may ignore the
bundle field; a consumer that wants NOMOS to assert it must pass the flags.

## Claim Boundary

The bundle proves a structured handoff shape, source linkage, trace manifest,
and attestation metadata. It does not prove downstream import success,
regulatory correctness, legal correctness, or live RAG behavior.

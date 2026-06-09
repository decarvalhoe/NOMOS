# 43 - Canonical Knowledge Bundle Contract

The CKM bundle is the versioned handoff artifact for downstream consumers such
as Aedifica. It is a read-only product of NOMOS and must be treated as immutable
once published.

## Consumer Mapping

| Bundle path | Consumer table | Notes |
|---|---|---|
| `bundle_id`, `schema_version`, `generated_at` | `knowledge_bundle_versions` | One row per imported bundle version. |
| `feeds[].feed_id` | `knowledge_feeds` | Feed identity and source format. |
| `feeds[].nodes[].node_id` | `knowledge_nodes` | Primary stable node key. |
| `feeds[].nodes[].text` | `knowledge_nodes.text` | Canonical text consumed by product/RAG surfaces. |
| `feeds[].nodes[].source_path`, `source_hash`, `span` | `knowledge_node_sources` | Citation and audit reconstruction fields. |
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

## Claim Boundary

The bundle proves a structured handoff shape, source linkage, trace manifest,
and attestation metadata. It does not prove downstream import success,
regulatory correctness, legal correctness, or live RAG behavior.

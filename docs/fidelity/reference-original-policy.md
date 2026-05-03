# Reference-Original Policy

## Purpose

This policy governs the distinction between original authoritative documents
and derived copies within the Nomos fidelity pipeline. Every document
processed by Nomos must be classified as original, surrogate, or derived,
and the classification determines what claims the pipeline may make.

## Claim Boundary

This policy defines classification rules only. It does not certify that
any specific document is authentic or that its content has been validated.

## Document Classes

| Class | Definition | Allowed Claims |
|---|---|---|
| **original** | The authoritative version obtained directly from the publisher or issuer. Hash-verified against a known digest. | Full: structured_contract, citation_external, golden_case, vector_index |
| **surrogate** | A faithful reproduction (e.g., public eCFR mirror, official PDF download) with known provenance but not the signed original. | Partial: structured_contract, citation_internal, vector_index |
| **derived** | A transformation, translation, summary, or extract of an original. | Limited: citation_internal only |
| **unverified** | Source provenance unknown or hash does not match any known digest. | None: blocked from pipeline |

## Classification Rules

1. A document is **original** if:
   - Its SHA-256 hash matches a digest in the licensed document register
   - It was obtained from a known authoritative source (publisher URL)
   - The register entry has status `verified`

2. A document is **surrogate** if:
   - It comes from a recognized public mirror or official reproduction
   - The register entry has status `surrogate`
   - Its content hash is recorded but may differ from the canonical digest

3. A document is **derived** if:
   - It is a translation, summary, extract, or reformatted version
   - It references an original via `original_ref`
   - The register entry has status `derived`

4. A document is **unverified** if:
   - No register entry exists
   - The hash does not match any known entry
   - The entry has status `unverified` or `blocked`

## Licensed Document Register

The register at `docs/fidelity/licensed-doc-register.yaml` records:

```yaml
documents:
  - id: REF-001
    title: "Document Title"
    publisher: "Publisher Name"
    source_url: "https://..."
    class: original          # original, surrogate, derived, unverified
    status: verified         # verified, surrogate, derived, unverified, blocked
    hash: "sha256:..."       # content digest
    obtained_at: "2026-01-15"
    license: "public"        # public, licensed, restricted
    original_ref: ""         # for derived: ID of the original
    retention: "permanent"
    notes: ""
```

## Pipeline Rules

1. **Originals** flow through the full pipeline: atomization, RAG projection,
   citation graph, evidence generation.

2. **Surrogates** flow through atomization and RAG but citations must be
   marked as `citation_internal` (not `citation_external`).

3. **Derived** documents produce atoms tagged `derived` and are excluded
   from golden case generation. RAG chunks carry `authority: derived`.

4. **Unverified** documents are blocked at the pipeline entry. No atoms,
   chunks, or evidence are generated.

## Hash Verification

Every document entering the pipeline must have its content hash computed
and checked against the register. A mismatch triggers:

- `REF_HASH_MISMATCH` finding (blocking)
- The document is reclassified as `unverified`
- Processing is halted until the register is updated

## Retention

- Original and surrogate entries are retained permanently in the register
- Derived entries follow the retention policy of their original
- Blocked entries are retained for audit trail

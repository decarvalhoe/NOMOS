# Canonical Corpus Mode

## Overview

Not every repository that Nomos manages is a code product. Some repositories
are **authoritative source collections** — corpora of contracts, regulations,
decisions, and reference data that feed downstream products.

Nomos supports these through `mode: canonical_corpus` in the project manifest.

## Product vs Corpus

| Dimension | Product (`mode: product`) | Corpus (`mode: canonical_corpus`) |
|---|---|---|
| Surfaces | Code surfaces (api, ui, worker, etc.) | Docs/data only |
| Toolchain | Build + test required | No build/test |
| Execution | Read-write (code runs) | Read-only (sources consumed) |
| Adapters | Stack-specific detection | None — source inventory only |
| Admission gate | Surface detection + checks | Source inventory completeness |
| Key checks | Sources, contracts, matrix, product-check, strict | Source hash, owner, confidentiality |

## When to Use Corpus Mode

- A repository that stores authoritative contracts (PDF, legal text)
- A regulatory reference collection
- Actuarial tables, tariff schedules, catalogs
- Decision records and governance archives
- Any source repository consumed by multiple downstream products

## Schema Requirements

A corpus project manifest must include:

```yaml
mode: canonical_corpus

source_inventory:
  manifest_path: source-manifest.yaml  # path to the source manifest
  hash_required: true                   # every source must have a hash
  owner_required: true                  # every source must have an owner
  confidentiality_required: true        # every source must declare confidentiality

corpus_policy:
  execution: read_only                  # no code execution
  allowed_consumers:                    # optional: which products may consume
    - product-a
    - product-b
  retention: "7 years"                  # optional: retention policy
```

## What Corpus Mode Does NOT Have

- No `surfaces` (use `corpus_surfaces` for docs/data organization only)
- No `toolchain` (no build, no test commands)
- No adapter detection
- No product-check or strict gate (those require code surfaces)

## Admission in Corpus Mode

A corpus repository is admitted when:

1. `source_inventory.manifest_path` points to a valid source manifest
2. All sources have hashes (if `hash_required: true`)
3. All sources have owners (if `owner_required: true`)
4. All sources declare confidentiality (if `confidentiality_required: true`)
5. `corpus_policy.execution` is `read_only`

## Evidence Chain

```
nomos.project.yaml (mode: canonical_corpus)
  └─ source-manifest.yaml (source inventory)
       ├─ Each source: hash ✓, owner ✓, confidentiality ✓
       └─ nomos-report.json (inventory completeness evidence)
            └─ attestation (signed if regulated)
```

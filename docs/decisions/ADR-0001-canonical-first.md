# ADR-0001 - Canonical-First Methodology

## Status

Accepted

## Date

2026-04-30

## Context

Nomos needs a foundational architectural decision on how domain knowledge flows through the product. The options are: ad-hoc, schema-first, or canonical-first.

## Decision

We adopt the canonical-first methodology: all domain data must originate from authoritative sources, flow through versioned contracts, and be traceable end-to-end from source to product surface.

## Consequences

- Every domain rule must have a source reference
- No sample/mock/fixture data in production paths
- LLMs cite and explain but never decide
- Exceptions require explicit expiry and approval
- Release is blocked if strict checks fail

# ADR-0001: Canonical-First Method

## Status

Accepted

## Context

Product applications risk consuming sample data, hardcoded catalogues, or
untraced business rules. The Nomos project needs a foundational decision
on how to prevent this class of defect.

## Decision

Adopt the Canonical-First method: every business rule, catalogue entry, and
domain entity in the product must trace to an authoritative source with a
hash, owner, and lifecycle status.

## Consequences

- All product surfaces consume read-models derived from canonical contracts.
- Fixtures, samples, and mocks are forbidden in production paths.
- Gates enforce traceability before release promotion.

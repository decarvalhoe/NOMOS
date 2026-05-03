# Public Claim Boundary

This document defines what Nomos can honestly claim at `v0.1.0-ALPHA`.

## Allowed Claims

Nomos may claim that it:

- implements a canonical-first method for source-to-product traceability;
- provides a Go CLI for project diagnosis and corpus processing;
- can scan, manifest, diff, validate sidecars, feed, and attest source corpora;
- has a working `rbok-lawbook` profile for structured Markdown reference corpora;
- can produce certified TOC artifacts, governed lexicon output, RAG metadata, runtime import artifacts, release gate results, fidelity proof reports, and attestations;
- has been validated on a real RBOK lawbook POC with 7191 feed nodes and a green strict fidelity gate;
- includes regulated-readiness documentation, templates, evidence-pack automation, and a GitHub operating model.

## Conditional Claims

Nomos may claim the following only with context:

- **"Regulated-ready"** means the project has a regulated-readiness structure and evidence workflow; it does not mean certified or validated for a customer's regulated use.
- **"Full fidelity proven"** applies to a specific POC output and gate configuration, not every possible corpus and document format.
- **"Read-only corpus processing"** applies when the guard runs against a clean source repository and the source mutation check passes.
- **"RAG-ready"** means traceable metadata exists; it does not mean production vector-store retrieval and LLM behavior have been validated.

## Prohibited Claims At This Release

Do not claim that Nomos is:

- FDA validated;
- 21 CFR Part 11 compliant as a platform;
- EU Annex 11 compliant as a platform;
- GxP validated;
- ISO certified;
- NASA qualified;
- a complete eQMS;
- a substitute for customer validation;
- universally faithful across all PDFs, DOCX, images, scanned documents, legal codes, regulations, or game-rule books without corpus-specific evidence;
- legally authorized to redistribute licensed standards.

## Evidence Rule

Every public claim must map to one of:

- a passing automated gate;
- a generated artifact;
- a reviewed document;
- a controlled decision;
- a known gap with owner and next action.

If a claim cannot be mapped, it must be removed or rewritten as a roadmap item.

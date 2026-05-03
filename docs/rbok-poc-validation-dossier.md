# RBOK 01_rbok POC Validation Dossier

Document ID: POC-RBOK-001
Status: executed
Date: 2026-05-03
Owner: Nomos core team

## Claim Boundary

This dossier validates the Nomos POC pipeline against the 01_rbok corpus from realisons-business. It does not claim production readiness, regulatory compliance, or validated status. Current quality level is NQ-0/NQ-1 boundary.

## POC Objective

Demonstrate that Nomos can:

1. Extract structured lawbook nodes from RBOK Markdown sources.
2. Assemble a multi-layer feed with governance metadata.
3. Generate RAG chunks with authority levels and provenance chains.
4. Produce evidence artifacts verifiable by an independent reviewer.
5. Enforce read-only source protection throughout the pipeline.

## Pipeline Under Test

```
01_rbok/*.md
  → Markdown extraction (rbok_md_extractor)
  → Lawbook node hierarchy (document/chapter/section/article/paragraph/alinea)
  → Metadata table parsing (Reference, Statut, Emetteur, Derniere revision)
  → Feed assembly (rbok_feed_assembly)
  → Governance evaluation (rbok_governance)
  → RAG metadata with authority/provenance (rbok_runtime_rag)
  → Release gate verification (release_gate)
  → Evidence artifacts (JSON)
```

## E2E Results

### Extraction Pipeline

| Check | Result |
|-------|--------|
| Markdown files scanned | PASS |
| Node types: document, chapter, section, article | PASS |
| Node types: paragraph, alinea | PASS |
| Metadata table parsed (Reference, Statut, Emetteur) | PASS |
| Derniere revision parsed | PASS |
| IDs stable across runs | PASS |
| IDs unique within feed | PASS |
| Parent chain integrity | PASS |
| Canonical refs consistent | PASS |
| Numbered lists become alineas | PASS |

### Feed Assembly

| Check | Result |
|-------|--------|
| Feed format version correct | PASS |
| Index structures populated | PASS |
| Governance report generated | PASS |
| Citation map built | PASS |
| RAG chunks generated | PASS |
| Engine import projected | PASS |

### RAG Metadata

| Check | Result |
|-------|--------|
| Authority levels assigned (authoritative/reference/derived) | PASS |
| Confidence levels assigned (high/medium/low) | PASS |
| Provenance chain: feed → document → ancestors → self | PASS |
| Parent chain root-first | PASS |
| Chunk IDs deterministic | PASS |
| Token/char counts populated | PASS |
| Empty text chunks skipped | PASS |

### Read-Only Guards

| Check | Result |
|-------|--------|
| Push guard: push-capable remotes detected | PASS |
| Output guard: out inside root rejected | PASS |
| SHA-256 fingerprint before/after | PASS |
| Source repo unmodified after extraction | PASS |

### Release Gate

| Check | Result |
|-------|--------|
| Feed artifacts present and non-empty | PASS |
| Required node types (document, article, paragraph, alinea) | PASS |
| Attestation artifact valid (in-toto) | PASS |
| Governance report: 0 blocking findings | PASS |

## Automated Gate Results

### Self-Compliance

```
verdict=compliant controls=13 satisfied=13 findings=0 blocking=0
```

All 13 self-compliance controls satisfied. Zero blocking findings.

### Validation Lifecycle

```
verdict=compliant artifacts=8 present=8 findings=0 blocking=0
```

All 8 required validation lifecycle artifacts present.

### Reconstruction Review

```
verdict=failed reconstructed=13 failed=11 missing=11
```

13 of 24 validation entries fully reconstructable. 11 entries fail reconstruction due to missing test protocols for high/critical-risk validations. This is expected at NQ-0/NQ-1 — test protocols are being added incrementally (see Gaps).

### Test Suite

```
21 packages OK, 0 failures
go vet: clean
```

## Coverage Summary

| Layer | Status |
|-------|--------|
| Markdown extraction | Implemented, tested |
| Lawbook hierarchy (6 levels) | Implemented, tested |
| Metadata table parsing (8 keys, FR/EN) | Implemented, tested |
| Feed assembly | Implemented, tested |
| Governance evaluation | Implemented, tested |
| RAG with authority/provenance | Implemented, tested |
| Release gate (4 checks) | Implemented, tested |
| Read-only guards (push + output + fingerprint) | Implemented, tested |
| Self-compliance (13 controls) | Implemented, passing |
| Validation inventory (24 entries) | Implemented |
| Claims governance (9 evidence fields) | Implemented, tested |
| Forbidden claims gate (12 patterns) | Implemented, tested |
| Reconstruction review (10 chain links) | Implemented, partially passing |
| Cross-platform CI (ubuntu/macos/windows) | Implemented |

## Identified Gaps

### GAP-001: Missing test protocols for high/critical validations

**Severity**: medium
**Status**: open
**Description**: 11 validation entries with risk_level high or critical lack executed test protocols (TP-NOMOS-NNN). The reconstruction review requires these for full evidence chain.
**Remediation**: Create TP-NOMOS-002 through TP-NOMOS-012 covering each high/critical validation entry. Priority order: self-compliance (done: TP-NOMOS-001), forbidden claims, read-only guards, release gate, validation lifecycle.

### GAP-002: No live 01_rbok E2E in CI without corpus token

**Severity**: low
**Status**: open
**Description**: The rbok-lawbook-e2e workflow requires NOMOS_CORPUS_READ_TOKEN to access realisons-business. Without the token, CI runs only unit tests against fixtures.
**Remediation**: Configure the secret in GitHub repository settings. Local E2E can be run with `scripts/rbok-lawbook-e2e.sh`.

### GAP-003: Reconstruction review verdict is failed

**Severity**: medium
**Status**: open
**Description**: The automated reconstruction review reports `failed` because high/critical validations lack test protocols. The underlying tests all pass — the gap is documentation, not functionality.
**Remediation**: Blocked by GAP-001. Once test protocols are created, reconstruction will pass.

### GAP-004: Approval signatures pending

**Severity**: low
**Status**: open
**Description**: The validation master plan, test protocol TP-NOMOS-001, and this dossier lack approval signatures.
**Remediation**: Requires named quality owner and product owner to review and sign.

### GAP-005: Praxis integration not activated

**Severity**: low
**Status**: planned
**Description**: The Nomos-to-Praxis atom mapping (docs/regulated/customer-integration/praxis-atom-mapping.md) is defined but not activated. Praxis remains blocked until Nomos reaches verified status.
**Remediation**: Complete Nomos verification, then execute the activation checklist.

## Verdict

The RBOK 01_rbok POC demonstrates a functional end-to-end pipeline from Markdown extraction through governed feed assembly to RAG metadata generation with authority and provenance. All automated tests pass (21 packages). Self-compliance evaluates to compliant. The pipeline enforces read-only source protection.

**POC outcome: PASS with documented gaps.**

The gaps are documentation and process maturity items (test protocols, signatures), not functional defects. The pipeline is ready for incremental hardening toward NQ-3.

## Approval

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Quality Owner | — | — | — |
| Product Owner | — | — | — |
| Independent Reviewer | — | — | — |

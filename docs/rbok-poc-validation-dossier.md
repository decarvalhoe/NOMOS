# RBOK 01_rbok POC Validation Dossier

Document ID: POC-RBOK-001
Status: executed - RBOK lawbook POC gate pass, AQ/regulatory validation still bounded
Date: 2026-05-03
Owner: Nomos core team

## Claim Boundary

This dossier records the Nomos POC pipeline against the `01_rbok` corpus from `realisons-business`. It does not claim production readiness, regulatory compliance, validated system status, or full semantic fidelity for every Markdown/PDF/DOCX/YAML/JSON construct. The current claim is narrower: the full `rbok-lawbook` profile admission is intentional, gate-compatible lawbook artifacts are produced outside the source corpus, and the RBOK lawbook release gate passes for the current artifact contract.

## POC Objective

Demonstrate that Nomos can:

1. Extract structured lawbook nodes from RBOK Markdown sources.
2. Assemble a multi-layer feed with governance metadata.
3. Generate RAG chunks with authority levels and provenance chains.
4. Produce evidence artifacts verifiable by an independent reviewer.
5. Enforce read-only source protection throughout the pipeline.

## Current Execution Update

Execution timestamp: 2026-05-03
Nomos branch: `codex/rbok-poc-update-20260503`
RBOK corpus commit: `ea003e8fe3c35993731c3708a3787df6a3a690df`
Local evidence pack: `C:\Dev\nomos-rbok-poc-run-20260503-dynamic-depth`

### Commands Executed

```powershell
.\nomos-cli.exe corpus diagnose `
  --profile rbok-lawbook `
  --root C:\Dev\realisons-business\01_rbok `
  --format json

.\nomos-cli.exe corpus feed `
  --profile rbok-lawbook `
  --root C:\Dev\realisons-business\01_rbok `
  --artifacts-dir C:\Dev\nomos-rbok-poc-run-20260503-dynamic-depth `
  --corpus-id rbok-lawbook `
  --project-id airbook `
  --format json

.\release-gate.exe `
  --artifacts C:\Dev\nomos-rbok-poc-run-20260503-dynamic-depth `
  --profile rbok-lawbook
```

### Current Results

| Check | Current Result |
|-------|----------------|
| Read-only fingerprint before/after | PASS |
| Full profile diagnosis | PASS: `in_scope`, confidence `high` |
| Full corpus inventory | 240 files under `01_rbok`: 66 primary, 7 reference, 157 derived, 10 out-of-scope warnings |
| Lawbook artifact pack | PASS: 43 documents, 7,022 lawbook nodes |
| Gate-compatible artifacts | PASS: `rbok-lawbook-feed.json`, index, RAG metadata, engine import, governance, attestation |
| Attestation boundary | PASS: predicate scope `full_profile`, diagnosis embedded |
| Release gate | PASS: 0 blocking checks |

### Release Gate Result

The RBOK lawbook release gate passes against the current evidence pack:

```json
{
  "profile": "rbok-lawbook",
  "verdict": "pass",
  "blocking": 0,
  "warnings": 0
}
```

The gate now validates nested multi-feed lawbook nodes and computes the corpus' actual structural depth from parent chains. Current RBOK depth is `max:2` with `chapter` and `section` levels under `document`; the gate no longer hard-codes `article`, but it still fails if heading nodes are orphaned, cyclic, or cannot be traced back to a document.

### Corpus Admission Result

The `rbok-lawbook` profile diagnosis currently returns:

```json
{
  "verdict": "in_scope",
  "confidence": "high",
  "summary": "Corpus has 66 primary, 7 reference, 157 derived sources. Ready for canonical processing."
}
```

The binary blockers were resolved by declaring generated, archived, original/reference, and OS artifact files according to source policy instead of treating every binary as a canonical source.

### POC Interpretation

The updated POC validates the current full `rbok-lawbook` gate lane:

```text
all 01_rbok sources
  -> declared corpus policy
  -> lawbook nodes with dynamic document/heading/paragraph/alinea structure
  -> governance report
  -> RAG/import artifacts
  -> scoped in-toto attestation
  -> RBOK lawbook release gate pass
```

The updated POC still does not claim complete regulatory-grade document fidelity. Tables outside metadata, callouts, code blocks, images, links, exact line/column/byte spans, H5/H6 legal levels, and complete lexicon/TOC certification remain governed by the structure fidelity roadmap.

### Implemented Corrections

1. Full `01_rbok` source policy admission now distinguishes canonical, reference, generated, derived, schema, archived, ignored, and blocked files.
2. `corpus feed --profile rbok-lawbook --artifacts-dir <dir>` produces gate-compatible RBOK lawbook artifacts outside the source corpus.
3. `corpus attest` and generated feed attestations carry explicit scope and embedded diagnosis.
4. Attestation generation rejects `corpus_admissible` when the bound profile diagnosis is blocked or partial.
5. The release gate validates nested multi-feed nodes and dynamic structural depth from `parent_id` chains, which is the basis for a certified index/table of contents.

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

## Historical Baseline Results

The following checks document the earlier fixture-backed and restricted-lane baseline. They are retained for traceability, but they are superseded for current release readiness by the 2026-05-03 execution update above.

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
| Required node types and dynamic structural depth | PASS |
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
**Status**: resolved in issue #315 follow-up branch, pending PR/merge
**Issue**: https://github.com/RBOKproject/NOMOS/issues/315
**Description**: 11 validation entries with risk_level high or critical lacked TP-NOMOS protocol coverage after TP-NOMOS-001. The reconstruction review requires these for a complete evidence chain.
**Resolution**: TP-NOMOS-002 through TP-NOMOS-012 now cover the remaining high/critical validations. The protocols record local execution evidence, deviations where applicable, and pending quality-owner/product-owner approval.

### GAP-002: No live 01_rbok E2E in CI without corpus token

**Severity**: low
**Status**: open
**Description**: The rbok-lawbook-e2e workflow requires NOMOS_CORPUS_READ_TOKEN to access realisons-business. Without the token, CI runs only unit tests against fixtures.
**Remediation**: Configure the secret in GitHub repository settings. Local E2E can be run with `scripts/rbok-lawbook-e2e.sh`.

### GAP-003: Reconstruction review verdict is failed

**Severity**: medium
**Status**: unblocked by issue #315 follow-up branch; hardening remains tracked by issue #317
**Issue**: https://github.com/RBOKproject/NOMOS/issues/317
**Description**: The automated reconstruction review previously reported `failed` because high/critical validations lacked test protocols.
**Resolution**: With TP-NOMOS-002 through TP-NOMOS-012 present, `go test -v ./internal/compliance/... -run Reconstruction` reports `verdict=passed reconstructed=24 failed=0 missing=0`.
**Remaining hardening**: Issue #317 keeps the stricter requirement open: reconstruction should eventually parse protocol execution status and evidence artifacts directly, not only verify that protocol files reference each high/critical validation.

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

### GAP-006: Full `01_rbok` profile admission blocked

**Severity**: high
**Status**: resolved in PR #313 update
**Issue**: https://github.com/RBOKproject/NOMOS/issues/310
**Description**: `nomos corpus diagnose --profile rbok-lawbook --root 01_rbok` previously returned `blocked` because all binary files were treated as blockers, including generated workbooks, OS artifacts, archived files, and reference originals.
**Resolution**: The profile policy now classifies canonical, reference, generated, derived, schema, archived, out-of-scope, and blocked files. Current diagnosis returns `in_scope/high`.

### GAP-007: RBOK lawbook release gate artifact mismatch

**Severity**: high
**Status**: resolved in PR #313 update
**Issue**: https://github.com/RBOKproject/NOMOS/issues/311
**Description**: The prior CLI POC lane emitted `nomos.corpus-feed.v1` with `units`, while `release-gate --profile rbok-lawbook` expected lawbook node artifacts and a separate governance report.
**Resolution**: `corpus feed --profile rbok-lawbook --artifacts-dir <dir>` now emits gate-compatible lawbook feed, index, RAG metadata, engine import, governance, and attestation artifacts. The gate reads nested multi-feed nodes and validates dynamic structural depth from parent chains.

### GAP-008: Attestation verdict can overclaim the full corpus state

**Severity**: high
**Status**: resolved in PR #313 update
**Issue**: https://github.com/RBOKproject/NOMOS/issues/312
**Description**: The local POC command could generate `corpus_admissible` attestation for a restricted Markdown snapshot even when the full `rbok-lawbook` profile diagnosis was blocked.
**Resolution**: Corpus attestations now include explicit `scope` and embedded diagnosis. Generation rejects `corpus_admissible` if the bound diagnosis is blocked or partial.

## Verdict

The RBOK `01_rbok` POC now demonstrates a full `rbok-lawbook` gate lane from profile admission through gate-compatible lawbook artifacts, scoped attestation, dynamic structural-depth validation, and read-only source protection.

**Current POC outcome: RBOK LAWBOOK POC PASS / FULL REGULATORY-GRADE FIDELITY NOT CLAIMED.**

The remaining limitation is not the RBOK POC gate: it is the broader AQ claim for complete document fidelity, including tables beyond metadata, callouts, code blocks, links, images, exact spans, H5/H6 legal levels, certified table of contents, and governed lexicon coverage.

## Approval

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Quality Owner | — | — | — |
| Product Owner | — | — | — |
| Independent Reviewer | — | — | — |

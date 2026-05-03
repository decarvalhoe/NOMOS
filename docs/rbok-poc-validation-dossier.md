# RBOK 01_rbok POC Validation Dossier

Document ID: POC-RBOK-001
Status: executed - current POC partial, full AQ gate blocked
Date: 2026-05-03
Owner: Nomos core team

## Claim Boundary

This dossier records the Nomos POC pipeline against the 01_rbok corpus from realisons-business. It does not claim production readiness, regulatory compliance, or validated status. Current quality level is NQ-0/NQ-1 boundary for the restricted Markdown feed lane, and below AQ release level for the full corpus because admission and release-gate blockers remain.

## POC Objective

Demonstrate that Nomos can:

1. Extract structured lawbook nodes from RBOK Markdown sources.
2. Assemble a multi-layer feed with governance metadata.
3. Generate RAG chunks with authority levels and provenance chains.
4. Produce evidence artifacts verifiable by an independent reviewer.
5. Enforce read-only source protection throughout the pipeline.

## Current Execution Update

Execution timestamp: 2026-05-03 09:34 UTC
Nomos commit: `19559a21935045e99414c9baeff4f8e64811c055`
RBOK corpus commit: `ea003e8fe3c35993731c3708a3787df6a3a690df`
Local evidence pack: `C:\Dev\nomos-rbok-poc-run-20260503-113408`

### Commands Executed

```powershell
.\nomos-cli.exe corpus scan `
  --root C:\Dev\realisons-business\01_rbok `
  --out C:\Dev\nomos-rbok-poc-run-20260503-113408\snapshot.json `
  --ext .md `
  --allow '**/*.md'

.\nomos-cli.exe corpus manifest `
  --snapshot C:\Dev\nomos-rbok-poc-run-20260503-113408\snapshot.json `
  --out C:\Dev\nomos-rbok-poc-run-20260503-113408\source-manifest.yaml `
  --domain lawbook `
  --owner 'RBOK Corpus Team' `
  --id-prefix RBOK

.\nomos-cli.exe corpus validate-sidecar `
  --root C:\Dev\realisons-business\01_rbok `
  --manifest C:\Dev\nomos-rbok-poc-run-20260503-113408\source-manifest.yaml `
  --ext .md `
  --allow '**/*.md'

.\nomos-cli.exe corpus feed `
  --root C:\Dev\realisons-business\01_rbok `
  --snapshot C:\Dev\nomos-rbok-poc-run-20260503-113408\snapshot.json `
  --manifest C:\Dev\nomos-rbok-poc-run-20260503-113408\source-manifest.yaml `
  --corpus-id rbok-lawbook `
  --project-id rbok `
  --out C:\Dev\nomos-rbok-poc-run-20260503-113408\feed.json

.\nomos-cli.exe corpus attest `
  --snapshot C:\Dev\nomos-rbok-poc-run-20260503-113408\snapshot.json `
  --corpus-id rbok-lawbook `
  --project-id rbok `
  --verdict corpus_admissible `
  --confidence high `
  --out C:\Dev\nomos-rbok-poc-run-20260503-113408\attestation.json

.\nomos-cli.exe corpus diagnose `
  --profile rbok-lawbook `
  --root C:\Dev\realisons-business\01_rbok `
  --format json

.\release-gate.exe `
  --artifacts C:\Dev\nomos-rbok-poc-run-20260503-113408 `
  --profile rbok-lawbook
```

### Current Results

| Check | Current Result |
|-------|----------------|
| Read-only fingerprint before/after | PASS |
| Markdown scan lane | PASS: 69 `.md` files, 1,129,585 bytes |
| Markdown sidecar validation | PASS: 69 declared sources, 0 errors |
| Generic feed generation | PASS: 1,765 `rule` units from 69 sources |
| Feed unit status | PARTIAL: 1,765 / 1,765 units are `partial` pending human canonical review |
| Attestation file format | PASS: valid in-toto Statement/v1 |
| Full profile diagnosis | BLOCKED: 22 blocked binary files, 1 warning |
| Full corpus inventory | 240 files under `01_rbok`; only 69 are in the current Markdown lane |
| Release gate | FAIL: 3 blocking checks |

### Release Gate Blocking Findings

The AQ release gate failed against the current evidence pack:

1. `feed_present`: no gate-compatible lawbook feed artifact found.
2. `node_types`: required node types `document`, `article`, `paragraph`, and `alinea` were not found in the gate-visible artifact set.
3. `governance`: no gate-compatible governance report artifact found.

This is not a source corpus mutation issue. The read-only fingerprint check passed. The blocker is that the current CLI POC lane emits `nomos.corpus-feed.v1` units, while the RBOK lawbook release gate expects lawbook node artifacts and governance evidence.

### Corpus Admission Blocking Findings

The `rbok-lawbook` profile diagnosis currently returns:

```json
{
  "verdict": "blocked",
  "confidence": "low",
  "summary": "Corpus has 22 blocked binary file(s). Remove or declare them before admission."
}
```

The blockers are generated `.docx` files, `.DS_Store` artifacts, and reference `.docx` assets inside `01_rbok`. The corpus also contains YAML, JSON, PDF, HTML, script, generated workbook, and reference assets that are not part of the restricted Markdown manifest.

### POC Interpretation

The updated POC validates a restricted Markdown feed lane:

```text
01_rbok Markdown files
  -> snapshot
  -> source manifest
  -> Markdown-only sidecar validation
  -> generic feed units
  -> in-toto attestation
  -> read-only proof
```

The updated POC does not yet validate the full RBOK lawbook promise:

```text
all 01_rbok sources
  -> declared corpus policy
  -> lawbook nodes with document/article/paragraph/alinea fidelity
  -> governance report
  -> RAG/import artifacts
  -> AQ release gate pass
```

### Required Corrections

1. Define the full `01_rbok` source policy: canonical, reference, generated, derived, ignored, and blocked classes for Markdown, YAML, JSON, PDF, DOCX, HTML, scripts, and OS artifacts.
2. Extend the manifest/sidecar lane so non-Markdown sources are either declared, explicitly ignored, or processed by the correct adapter.
3. Align `corpus attest` with diagnosis: the attestation verdict must not claim `corpus_admissible` for the full profile when `diagnose --profile rbok-lawbook` is blocked.
4. Produce gate-compatible RBOK lawbook artifacts: `rbok-lawbook-feed.json`, `rbok-governance.json`, RAG metadata, engine import, citation/index artifacts.
5. Align the release gate with the actual artifact schemas, or make the CLI emit the schema already required by the gate.

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

### GAP-006: Full `01_rbok` profile admission blocked

**Severity**: high
**Status**: open
**Issue**: https://github.com/RBOKproject/NOMOS/issues/310
**Description**: `nomos corpus diagnose --profile rbok-lawbook --root 01_rbok` returns `blocked` because the full corpus contains 22 blocked binary files and additional non-Markdown assets outside the restricted Markdown lane.
**Remediation**: Define and enforce a full source policy for canonical, reference, generated, derived, ignored, and blocked files, then update scan/manifest/sidecar/profile logic accordingly.

### GAP-007: RBOK lawbook release gate artifact mismatch

**Severity**: high
**Status**: open
**Issue**: https://github.com/RBOKproject/NOMOS/issues/311
**Description**: The current CLI POC lane emits `nomos.corpus-feed.v1` with `units`, while `release-gate --profile rbok-lawbook` expects lawbook node artifacts and a separate governance report.
**Remediation**: Make the CLI produce gate-compatible RBOK lawbook artifacts or update the gate to accept the canonical artifact schema intentionally. The gate must not be bypassed by renaming incompatible artifacts.

### GAP-008: Attestation verdict can overclaim the full corpus state

**Severity**: high
**Status**: open
**Issue**: https://github.com/RBOKproject/NOMOS/issues/312
**Description**: The local POC command can generate `corpus_admissible` attestation for the restricted Markdown snapshot even though the full `rbok-lawbook` profile diagnosis is blocked.
**Remediation**: Bind attestation verdicts to the actual diagnosis boundary. If only the Markdown lane is attested, the attestation must state that scope explicitly and must not be presented as full `01_rbok` corpus admissibility.

## Verdict

The RBOK 01_rbok POC demonstrates a functional restricted Markdown feed lane from scan through feed and attestation, with read-only source protection. The updated run also proves that the full RBOK lawbook POC is not yet admissible at AQ release level.

**Current POC outcome: PARTIAL PASS / FULL POC BLOCKED.**

The restricted Markdown lane is operational. The full POC remains blocked by corpus admission policy gaps and by an artifact contract mismatch between the CLI feed output and the RBOK lawbook release gate. These are functional product gaps, not only documentation maturity items.

## Approval

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Quality Owner | — | — | — |
| Product Owner | — | — | — |
| Independent Reviewer | — | — | — |

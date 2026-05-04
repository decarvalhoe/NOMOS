# 29 - Post-Alpha Release Issue List

Date: 2026-05-04
Baseline: `v0.1.0-ALPHA`
Current main: `d7d03e2`

## Purpose

This document translates the post-alpha release train into an actionable
GitHub issue list. It is the planning source for the next implementation
waves and must be kept aligned with `docs/14-product-roadmap.md` and
`docs/15-product-backlog.md`.

The validated priority is **fidelity first**:

```text
v0.2 Fidelity Closure
  -> v0.3 Portable Corpus Fidelity
  -> v0.4 Reference Bible Governance
  -> v0.5 Regulated Evidence Pack
  -> v0.6 Nomos/Praxis Contract
```

No issue below may be used to claim certification, formal validation,
or regulated compliance by itself. Each issue creates product evidence,
operating controls, or claim-boundary clarity only.

## Current Open GitHub Issues

| Issue | Release lane | Role | Status |
|---|---|---|---|
| `#314` | `v0.2` / `v0.3` | AQ epic for RBOK POC proof level and stronger fidelity claims. | Open |
| `#382` | `v0.2` | Short critical atom reconciliation. | Open |
| `#192` | `v0.4` | Acquire and intake ISO 13485:2016. | Open |
| `#193` | `v0.4` | Acquire and intake ISO/IEC/IEEE 12207:2026. | Open |
| `#194` | `v0.4` | Complete GAMP 5 and ISO/IEC 25010 license review. | Open |
| `#196` | `v0.4` | Process public and licensed reference bibles with Nomos. | Open |
| `#320` | `v0.6` | Activate Nomos-to-Praxis atom mapping after Nomos verification. | Open / blocked |

## Dependency Tree

```text
NRT-001 #382 short critical inventory schema/report
  -> NRT-002 #382 short critical classifier + dispositions
  -> NRT-003 #382 semantic quality + strict-gate blocking
  -> NRT-004 #382 RBOK POC rerun with short-critical evidence
  -> NRT-005 #314 AQ claim requalification
  -> v0.2 release decision

NRT-006 portable golden corpus fixtures
  -> NRT-007 AST-to-Nomos comparison report
  -> NRT-008 unsupported-block evidence contract
  -> NRT-009 portable strict fidelity gate
  -> NRT-010 multi-domain POC evidence pack
  -> v0.3 release decision

#192 + #193 + #194
  -> NRT-011 licensed sidecar and no-full-text policy gate
  -> #196 reference bible processing
  -> NRT-012 reference-to-control matrix closure
  -> v0.4 release decision

NRT-013 release evidence bundle by tag
  -> NRT-014 attestation claim_coverage CLI wiring
  -> NRT-015 owner/training/approval records
  -> v0.5 release decision

NRT-016 Nomos/Praxis evidence schema
  -> NRT-017 atom mapping fixture
  -> NRT-018 Praxis activation gate
  -> #320 closure decision
  -> v0.6 release decision
```

## v0.2.0 - Fidelity Closure

Goal: close the technical gap that prevents stronger documentary
fidelity claims.

### NRT-001 - Short Critical Inventory Report

GitHub mapping: child of `#382`.

Deliverables:

- Emit `short-critical-atoms.json` for each processed corpus.
- Include source id, source path, source span, parent chain, block kind,
  table path / YAML path / JSON path when applicable, surrounding
  context, raw text, normalized text, and initial disposition.
- Add CUE/JSON schema and valid/invalid fixtures for the report.

Definition of done:

- Markdown, table, YAML, and JSON fixtures produce report entries for
  short meaningful fragments.
- Noise-only separators remain absent from curated feed/RAG but present
  in the body ledger where applicable.
- `cue vet` passes for the new report fixture.

Verification:

```bash
cd cli
go test ./internal/corpus -run ShortCritical -v
cd ..
cue vet specs/short-critical-atoms.cue specs/examples/short-critical-atoms.valid.yaml
```

Claim impact: enables review of excluded short fragments; does not yet
prove that every fragment is correctly classified.

### NRT-002 - Short Critical Disposition Classifier

GitHub mapping: child of `#382`.

Deliverables:

- Classify entries as `non_semantic`, `contextualized_in_parent`,
  `lexicon_atom`, `identifier_atom`, `normative_value_atom`, or
  `requires_review`.
- Promote standalone critical terms into governed lexicon/value/identifier
  artifacts without creating orphan RAG chunks.
- Cover examples: `GxP`, `ALCOA+`, `21 CFR`, `SOP-01`, `P0`, `Yes`,
  `No`, thresholds, status labels, table cells, YAML/JSON scalars.

Definition of done:

- Every critical short fixture has a deterministic disposition.
- Short but meaningful terms are traceable to parent context or a
  governed artifact.
- Unclear fragments become `requires_review`.

Verification:

```bash
cd cli
go test ./internal/corpus -run ShortCritical -v
```

Claim impact: supports a stronger semantic coverage claim for short
critical fragments inside scoped corpora.

### NRT-003 - Short Critical Strict Gate

GitHub mapping: child of `#382`.

Deliverables:

- Extend semantic quality and strict gate output with short-critical
  findings.
- Fail closed when an entry is `requires_review` or has no disposition.
- Preserve the invariant that curated RAG has no orphan <=10-character
  chunks.

Definition of done:

- Strict gate exits non-zero on unresolved short-critical findings.
- Strict gate records a pass section when the report is clean.
- Existing SFI/FSQ gates remain backward-compatible when no report is
  supplied.

Verification:

```bash
cd cli
go test ./internal/app -run Strict -v
go test ./internal/corpus -run SemanticQuality -v
```

Claim impact: makes short-critical evidence gateable instead of a manual
review note.

### NRT-004 - RBOK POC Rerun With Short-Critical Evidence

GitHub mapping: child of `#382` and `#314`.

Deliverables:

- Extend `scripts/rbok-poc-integrity.sh` to emit and gate
  `short-critical-atoms.json`.
- Update the RBOK POC evidence dossier with actual counts from the run.
- Keep source mutation checks before/after the corpus run.

Definition of done:

- RBOK POC evidence pack includes feed, RAG metadata, body ledger,
  strict gate, attestation, and short-critical report.
- `0` unresolved short-critical findings.
- The dossier states only the bounded claim supported by the run.

Verification:

```bash
bash scripts/rbok-poc-integrity.sh
```

Claim impact: promotes the RBOK POC from alpha source-to-feed evidence
to scoped short-critical semantic fidelity evidence.

### NRT-005 - AQ Claim Requalification

GitHub mapping: `#314`.

Deliverables:

- Split already-proven alpha evidence from remaining stronger-fidelity
  requirements.
- Update `docs/public-claim-boundary.md`,
  `docs/rbok-poc-validation-dossier.md`, and release notes.
- Define exactly what claim level v0.2 earns.

Definition of done:

- `#314` no longer mixes closed AQ sprint items with unresolved future
  claims.
- Public wording is consistent across README, roadmap, backlog, release
  notes, and POC dossier.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Claim impact: prevents accidental overclaim after the v0.2 evidence
improves.

## v0.3.0 - Portable Corpus Fidelity

Goal: prove that the fidelity engine is portable beyond RBOK Markdown.

### NRT-006 - Portable Golden Corpus Fixtures

GitHub mapping: new issue.

Deliverables:

- Add fixtures for legal text, regulatory text, technical standard,
  business corpus, game rules, Markdown, YAML, and JSON.
- Each fixture must declare expected source structures and unsupported
  structures.

Definition of done:

- Fixtures are small, license-safe, and committed to the repo.
- Each fixture has a manifest and expected artifact assertions.

Verification:

```bash
cd cli
go test ./internal/corpus -run Portable -v
```

Claim impact: starts removing RBOK-specific confidence bias.

### NRT-007 - AST-To-Nomos Comparison Report

GitHub mapping: new issue.

Deliverables:

- Emit `portable-fidelity-report.json`.
- Compare parsed source structure against Nomos nodes for H1-H6,
  tables, lists, callouts, code, links, images, annexes, xrefs, and
  front matter.

Definition of done:

- Missing active source structures are findings.
- Structure-only nodes are represented without duplicating semantic
  body bytes.

Verification:

```bash
cd cli
go test ./internal/corpus -run PortableFidelity -v
```

Claim impact: makes structure fidelity auditable across corpus families.

### NRT-008 - Unsupported Block Evidence Contract

GitHub mapping: new issue.

Deliverables:

- Ensure unsupported active blocks become explicit evidence records.
- Classify unsupported blocks as blocking or non-blocking by policy.
- Document the policy in `docs/21-source-feed-integrity-engine.md`.

Definition of done:

- No active source material can be skipped silently.
- Unsupported HTML/PDF/DOCX/OCR cases are explicit and bounded.

Verification:

```bash
cd cli
go test ./internal/corpus -run Unsupported -v
```

Claim impact: supports fail-closed corpus admission.

### NRT-009 - Portable Strict Fidelity Gate

GitHub mapping: new issue.

Deliverables:

- Wire `portable-fidelity-report.json` into the strict gate.
- Fail when active structures are missing, duplicated, or unsupported
  without accepted policy.

Definition of done:

- Strict gate can run against RBOK and non-RBOK fixtures.
- Backward-compatible behavior remains for alpha evidence inputs.

Verification:

```bash
cd cli
go test ./internal/app -run Strict -v
```

Claim impact: turns portability from documentation into a release gate.

### NRT-010 - Multi-Domain Evidence Pack

GitHub mapping: new issue.

Deliverables:

- Produce a recorded evidence pack across all portable fixtures.
- Update roadmap and backlog with actual result boundaries.

Definition of done:

- Every fixture emits feed, body ledger, RAG metadata, short-critical
  report, portable fidelity report, and strict-gate output.

Verification:

```bash
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\e2e.ps1
```

Claim impact: permits scoped wording that Nomos fidelity checks are
portable across the tested corpus families.

## v0.4.0 - Reference Bible Governance

Goal: process regulated reference bibles without license misuse or
overclaiming.

### NRT-011 - Licensed Sidecar And No-Full-Text Gate

GitHub mapping: supports `#192`, `#193`, `#194`, `#196`.

Deliverables:

- Sidecar fields: reference id, title, edition, source owner, local path
  outside Git, SHA256, license status, allowed derivative fields,
  prohibited redistribution fields, reviewer, review date.
- Gate that fails if licensed full text is staged or committed.

Definition of done:

- ISO/GAMP references can be tracked without committing protected text.
- Missing acquisition remains an explicit blocked state, not a fake pass.

Verification:

```bash
python scripts/regulated_reference_canon.py --licensed-root C:\Dev\nomos-licensed-references
```

Claim impact: supports licensed-reference readiness without asserting
regulatory approval.

### NRT-012 - Reference-To-Control Matrix Closure

GitHub mapping: supports `#196`.

Deliverables:

- Emit `reference-to-control-matrix.json`.
- Every cited regulation, standard, or framework is `mapped`,
  `blocked`, `not_applicable`, or `waived`.

Definition of done:

- No active reference remains decorative authority.
- Public claims link to controls and evidence or are blocked.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
```

Claim impact: moves Nomos closer to regulated-readiness evidence, not
certification.

## v0.5.0 - Regulated Evidence Pack

Goal: make release decisions reconstructible by a reviewer.

### NRT-013 - Release Evidence Bundle By Tag

GitHub mapping: new issue.

Deliverables:

- Emit `regulated-release-evidence-pack.json` per tag.
- Include CI URLs, source hashes, corpus hashes, attestations,
  deviations, waivers, approvals, and release decision.

Definition of done:

- A reviewer can reconstruct the release decision from retained files.
- The bundle records missing evidence as blocked, not omitted.

Verification:

```bash
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
```

Claim impact: supports NQ-3/NQ-4 readiness discussions.

### NRT-014 - Attestation Claim Coverage CLI Wiring

GitHub mapping: new issue.

Deliverables:

- Wire body ledger input into `nomos corpus attest`.
- Emit `claim_coverage` in the attestation predicate when a body ledger
  is provided.
- Reject overclaims when body coverage is incomplete.

Definition of done:

- CLI behavior matches the existing Go attestation model.
- POC runner no longer records missing claim coverage as a warning.

Verification:

```bash
cd cli
go test ./internal/app -run Attest -v
go test ./internal/corpus -run Attestation -v
```

Claim impact: closes a known alpha evidence gap.

### NRT-015 - Owner, Training, Approval Records

GitHub mapping: new issue.

Deliverables:

- Record named quality owner, technical owner, reviewer, approval role,
  and required training evidence.
- Keep placeholder or missing records as blocking release findings.

Definition of done:

- Release gate can distinguish documented approval from missing owner
  evidence.

Verification:

```bash
python scripts/regulated_approval_gate.py
```

Claim impact: supports regulated-release governance without replacing
customer validation.

## v0.6.0 - Nomos/Praxis Contract

Goal: activate Praxis only after Nomos evidence is verified enough not
to weaken the claim boundary.

### NRT-016 - Nomos/Praxis Evidence Schema

GitHub mapping: prerequisite for `#320`.

Deliverables:

- Define `nomos-praxis-evidence.schema.json`.
- Include Nomos artifact references, Praxis scenario evidence, runtime
  findings, CAPA status, and claim boundary.

Definition of done:

- Valid and invalid fixtures prove schema behavior.

Verification:

```bash
npx ajv validate -s specs/nomos-praxis-evidence.schema.json -d specs/examples/nomos-praxis-evidence.valid.json
```

Claim impact: creates the shared contract; does not activate Praxis yet.

### NRT-017 - Atom Mapping Fixture

GitHub mapping: prerequisite for `#320`.

Deliverables:

- Emit `nomos-praxis-mapping.json`.
- Map Nomos atoms to Praxis checks and downstream runtime evidence.

Definition of done:

- Fixture demonstrates Nomos authority remains canonical and Praxis
  remains downstream evidence.

Verification:

```bash
npx ajv validate -s specs/nomos-praxis-evidence.schema.json -d specs/examples/nomos-praxis-mapping.valid.json
```

Claim impact: shows the intended evidence flow without overclaiming
runtime assurance.

### NRT-018 - Praxis Activation Gate

GitHub mapping: closes `#320` only if passed.

Deliverables:

- Gate that blocks Praxis activation unless Nomos proof level is
  sufficient.
- Documentation that joint claims declare each product's own quality
  level.

Definition of done:

- `#320` remains blocked unless the gate passes on verified Nomos
  artifacts.

Verification:

```bash
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
```

Claim impact: allows scoped Nomos/Praxis compatibility claims without
turning Praxis into unverified regulated evidence.

## Release-Level Verification

Every release PR must run:

```bash
cd cli
go test ./...
cd ..
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\e2e.ps1
python -m unittest discover -s tests -v
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
```

If Go is not available on a workstation, the PR must cite the green
GitHub Actions run before merge.

## Issue Creation Policy

1. Create GitHub child issues from this document before coding a release
   wave.
2. Each child issue must copy its deliverables, definition of done,
   verification commands, dependencies, and claim impact from this file.
3. Close a child issue only after its evidence artifact is committed or
   linked in the PR.
4. Do not close `#314` or `#320` from documentation alone.
5. Do not close licensed-reference issues from surrogate or public
   references; they require the explicit acquisition / license evidence
   described in the issue.

# 29 - Post-Alpha Release Issue List

Date: 2026-09-05
Baseline: `v0.1.0-ALPHA`

## Purpose

This document translates the post-alpha release train into an actionable
GitHub issue list. It is the planning source for the next implementation
waves and must be kept aligned with `docs/14-product-roadmap.md` and
`docs/15-product-backlog.md`.

This document plans **product and DevOps delivery only**. The independent
regulated-assurance roadmap is [28](28-regulated-compliance-closure-plan.md).
Its calendar evidence, human records, licence acquisitions, approvals and
irreversible writes are nonblocking inputs/claim gates, never engineering
dependencies. Routing is executable in [roadmap-lanes.yaml](roadmap-lanes.yaml)
under [ADR-VRC-0004](adr/0004-independent-roadmaps-risk-based-validation.md).

The validated priority is **fidelity first**:

```text
v0.2 Fidelity Closure
  -> v0.3 Portable Corpus Fidelity
  -> v0.4 Reference tooling and public provenance
  -> v0.5 Evidence and release-support tooling
  -> v0.6 Nomos/Praxis Contract
```

No issue below may be used to claim certification, formal validation,
or regulated compliance by itself. Each issue creates product evidence,
operating controls, or claim-boundary clarity only.

## Current Autonomous Issues

<!-- roadmap-queues:begin -->
<!-- GENERATED from docs/roadmap-lanes.yaml by scripts/roadmap_lane_guard.py --emit-docs; do not edit by hand, CI fails on drift -->
| Product queue | DevOps queue |
|---|---|
| #610 — Define the Recursio web-source contract | #641 — Restore licence-review and no-full-text gates |
| #611 — Verify immutable external corpus snapshots | #644 — Process real public references and retain evidence |
| #612 — Recursio to Nomos offline E2E fixture | #639 — Prepare and verify a v0.2.0-ALPHA candidate bundle |
| #637 — Offline Sigstore bundle verification | #645 — Keyless issuance against injected non-production services |
| #643 — Static SKOS authoring and distribution | — |
<!-- roadmap-queues:end -->

Regulated items #560/#561/#562/#192/#193/#194/#196/#638 are tracked by plan
28 as passive, human or external. They block only their named evidence/use or
claim, and never either queue above. Product and DevOps select independently.

## Dependency Tree

```text
Historical closed foundation (#382 / #314):
NRT-001 short critical inventory schema/report
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

#641 licence/no-full-text gate (synthetic fixtures, autonomous)
  -> #644 actual public-source processing (autonomous)
  -> v0.4 product-tooling decision

#192 + #193 + #194 + #196 (independent regulated inputs)
  -> only their named licensed-source uses and claims

NRT-013 candidate release evidence bundle (no tag/publication)
  -> NRT-014 attestation claim_coverage CLI wiring
  -> v0.5 product-tooling decision

NRT-016 Nomos/Praxis evidence schema
  -> NRT-017 atom mapping fixture
  -> NRT-018 Praxis activation gate
  -> #320 closed technical boundary; regulated reliance remains plan 28
  -> v0.6 release decision
```

Recursio is an independent, fixture-first product sequence:

```text
#610 nomos.web-source contract
  -> #611 immutable external JSONL snapshot verifier/importer
  -> #612 local Recursio -> Nomos E2E
```

Robots/licence decisions for a real website and production Recursio evidence
are later source-specific inputs; the offline contract and fixtures do not wait
for them.

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

## v0.4.0 - Reference Tooling And Public Provenance

Goal: provide reference-policy gates and process actual public sources without
licence misuse or overclaiming. Licensed acquisition/use remains on roadmap 28.

### NRT-011 - Licensed Sidecar And No-Full-Text Gate

GitHub mapping: `#641`; supports regulated issues `#192`, `#193`, `#194`,
`#196` without depending on their decisions.

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

GitHub mapping: public evidence `#644`; regulated licensed use `#196` consumes
the same mapping later without blocking it.

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

## v0.5.0 - Evidence And Release-Support Tooling

Goal: make candidate evidence bundles reconstructible by a reviewer. The
regulated roadmap owns the authentic release decision, approval, tag and
publication.

### NRT-013 - Candidate Release Evidence Bundle

GitHub mapping: `#639`.

Deliverables:

- Emit `regulated-release-evidence-pack.json` per candidate commit; a tag is
  optional input, never invented.
- Include CI URLs, source hashes, corpus hashes, attestations, deviations,
  waivers, approval status and release-decision status. Pending stays pending;
  tooling never invents the record.

Definition of done:

- A reviewer can reconstruct what the candidate contains and which decisions
  remain pending from retained files.
- The bundle records missing evidence as blocked, not omitted.

Verification:

```bash
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
```

Claim impact: proves candidate-bundle preparation only; NQ claims remain on
roadmap 28.

### NRT-014 - Attestation Claim Coverage CLI Wiring (Delivered)

GitHub mapping: VRC-07 `#553` (closed; capability real in the wiring matrix).

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

### Regulated Interface (Nonblocking)

Owner, training, competence, approval and release records are not product
deliverables. They live on the independent regulated roadmap (plan 28,
#561/#562). DevOps may provide technically verified templates with a declared
validation state and bounded reliance, plus status tooling (#639, #640), but
the records remain authentic human acts and never enter this engineering
dependency tree.

## v0.6.0 - Nomos/Praxis Contract

Goal: activate Praxis only after Nomos evidence is verified enough not
to weaken the claim boundary.

### NRT-016 - Nomos/Praxis Evidence Schema

GitHub mapping: follow-up to the technical boundary delivered by closed issue `#320`.

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

GitHub mapping: autonomous fixture work after closed issue `#320`.

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

GitHub mapping: gates regulated Praxis reliance; it does not reopen or close `#320`.

Deliverables:

- Gate that blocks Praxis activation unless Nomos proof level is
  sufficient.
- Documentation that joint claims declare each product's own quality
  level.

Definition of done:

- Regulated Praxis reliance remains blocked unless the gate passes on verified
  Nomos artifacts; technical fixtures may run earlier on `not_qualified`
  inputs.

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
4. Historical issues `#314` and `#320` are closed; do not reuse their closure
   as evidence for a later regulated claim.
5. Do not close licensed-reference issues from surrogate or public
   references; they require the explicit acquisition / license evidence
   described in the issue.

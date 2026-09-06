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
  -> v0.9 Portfolio Governance (machine-readable status, no narrative)
  -> v1.0 Stable Product Release Candidate (stability made measurable)
```

No issue below may be used to claim certification, formal validation,
or regulated compliance by itself. Each issue creates product evidence,
operating controls, or claim-boundary clarity only.

## Current Autonomous Issues

<!-- roadmap-queues:begin -->
<!-- GENERATED from docs/roadmap-lanes.yaml by scripts/roadmap_lane_guard.py --emit-docs; do not edit by hand, CI fails on drift -->
| Product queue | DevOps queue |
|---|---|
| — | — |
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

NRT-016 #660 Nomos/Praxis evidence schema
  -> NRT-017 #661 atom mapping fixture
  -> NRT-018 #662 Praxis activation gate
  -> #320 closed technical boundary; regulated reliance remains plan 28
  -> v0.6 release decision

NRT-019 #667 portfolio status contract + engine (`nomos portfolio status`)
  -> NRT-020 #668 findings and periodic-review index, queryable
  -> NRT-022 #670 control-plane decision under ADR-0006 (wire or remove)
  -> v0.9 product decision

NRT-021 #669 periodic-review record index + guard (DevOps, independent sidecar)
  -> nonblocking input to NRT-020

NRT-023 #676 contract stability registry + compatibility guard
  -> NRT-024 #677 compatibility matrix, version announcement, deprecation enforcement
  -> NRT-027 #680 customer integration guide, commands replayed against fixtures
  -> NRT-028 #681 v1.0 readiness verdict, computed (never "released")
  -> v1.0 product decision (release itself stays #561 / plan 28)

NRT-025 #678 security process, executable (DevOps, independent)
NRT-026 #679 support model, declared and checked (DevOps, independent)
  -> nonblocking inputs to NRT-028
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

GitHub mapping: `#660` (product, autonomous); follow-up to the technical boundary delivered by closed issue `#320`.

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

GitHub mapping: `#661` (product, autonomous); fixture work after closed issue `#320`.

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

GitHub mapping: `#662` (product, autonomous); gates regulated Praxis reliance; it does not reopen or close `#320`.

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

## v0.9.0 - Portfolio Governance

Goal: make the portfolio state — capabilities, roadmap lanes, open findings,
claim levels, evidence bundles and periodic review records — **queryable from
machine sources only**, so that a management review, an audit or a customer
question is answered from computed status, never from narrative memory. This
wave is also the moment ADR-0006 named for deciding the archived control-plane
code: it now has a candidate production caller.

Boundary: portfolio status is a computed *view* of committed evidence. It does
not create evidence, approve anything, or lift a claim; a green view of an
unvalidated tool is still an unvalidated tool.

### NRT-019 - Portfolio Status Contract And Engine

GitHub mapping: `#667` (product, autonomous). Capability-claim (VRC-03):
production caller `nomos portfolio status`, registered in `app.go`.

Deliverables:

- `specs/portfolio-status.cue` (`#PortfolioStatus` v1, closed structs): every
  section derives from a named machine source with `path`, `sha256` and
  `as_of`; sections: capabilities (wiring registry + generated matrix),
  roadmap lanes and queues, evidence-ledger gaps, CAPA records, periodic
  review records, latest release-candidate manifest, Praxis activation
  verdict, repeated-CI measure, competence status, domain packs,
  public-source snapshots.
- Go engine `cli/internal/portfolio` + `nomos portfolio status --repo-root
  [--out] [--format json|md]`: numbers are computed from files, never read
  from prose; an unavailable source yields a section `unavailable` with its
  reason, never a silent omission; a dated source older than the freshness
  policy is flagged `stale`, not hidden.
- CI step publishing `portfolio-status.json` as a workflow artifact and
  asserting that the capabilities section equals the committed wiring matrix.

Definition of done:

- Editing any source moves its `sha256` and the derived numbers; removing a
  source turns its section `unavailable`; a free-text field in the status is
  refused by the contract (closed structs, `cue vet` negative fixture).
- Adversarial tests and mutation-test on every rule; registry entry
  `portfolio_status_engine` (`real`).

Verification:

```bash
cd cli && go test ./internal/portfolio/... -run Portfolio -v
cue vet specs/portfolio-status.cue specs/examples/portfolio-status.valid.json -d '#PortfolioStatus'
```

Claim impact: allows "portfolio status is computed from committed evidence".
It lifts no regulated claim.

### NRT-020 - Findings And Periodic-Review Index, Queryable

GitHub mapping: `#668` (product, autonomous; depends on NRT-019).
Production caller `nomos portfolio findings` / `nomos portfolio reviews`.

Deliverables:

- Normalise every open finding across sources into `findings[]` with a stable
  id (`<source>:<id>`), severity, status, opened date, owner when recorded,
  blocked claims and lane: evidence-ledger gaps, CAPA, unmet activation-gate
  requirements, blocked public-source captures, wiring-matrix mismatches,
  registry/GitHub state divergence.
- Index periodic review records (management review, internal audit, role
  assignment) into `reviews[]`: record id, type, date, decisions, actions with
  status, and the artifacts each input cites, verified to exist.
- Query surface: `--severity`, `--status`, `--source`, `--lane`, `--format`.
- Consistency findings: a CAPA closed on an effectiveness artifact that no
  longer exists, a review decision citing a missing artifact, a gap closed in
  the ledger but still open in a review action → each becomes a finding.

Definition of done:

- Every finding is traceable to its source file and hash; a fabricated
  finding (no source) is refused; adversarial tests for each consistency
  rule; registry entry `portfolio_findings_index` (`real`).

Verification:

```bash
cd cli && go test ./internal/portfolio/... -run Findings -v
```

Claim impact: "open findings are queryable" — it does not close, waive or
prioritise any of them.

### NRT-021 - Periodic Review Record Index And Guard

GitHub mapping: `#669` (DevOps, autonomous, independent sidecar).
Production caller: `scripts/portfolio_review_index.py` in CI.

Deliverables:

- Python sidecar generating `docs/regulated/operations/records/index.json`
  (`nomos-review-record-index-v1`) from the committed review, audit, role and
  CAPA records: ids, types, dates, decision and action counts, cited
  artifacts.
- Guard: every cited artifact path exists; every decision has an id; every
  action has an owner and a status; records are dated; the committed index
  equals a fresh build (`git diff --exit-code`, CI).

Definition of done:

- A cited artifact removed, an action without owner, or a stale index turn
  CI red; adversarial tests; registry entry `review_record_index` (`sidecar`).

Verification:

```bash
python scripts/portfolio_review_index.py --root . --check
```

Claim impact: none on regulated claims; it makes the QMS records countable.

### NRT-022 - Control-Plane Decision Under ADR-0006

GitHub mapping: `#670` (product, autonomous; depends on NRT-019).

ADR-0006 archived `control-plane/` (registry, dashboard, storage) until a
capability-claim issue declared a production caller at the v0.9.x milestone.
NRT-019 provides that caller for portfolio status; the archived code's own
purpose is the **multi-project** view (project manifests + exceptions).

Deliverables:

- ADR-0007 recording the decision, one of:
  - **wire**: `nomos portfolio projects --project <nomos.project.yaml>...
    [--exceptions ...]` calls `dashboard.BuildPortfolio` for the multi-project
    view; the packages move under the CLI module (or are imported), CI gating
    is restored in the same PR, and every rule gains adversarial tests
    (expired exception → visible, unknown verdict → counted, filter → exact);
  - **remove**: `control-plane/` is deleted, the root repository map and
    docs/14 architecture row are updated, and a `must_be_absent` probe keeps
    it out.
- Either way, no archived-but-untested code remains in the tree.

Definition of done:

- Registry: `portfolio_multi_project_view` (`real`) or `control_plane`
  (`absent` with probe); ADR-0006 marked superseded/closed by ADR-0007.

Verification:

```bash
cd cli && go test ./... && uv run --with pyyaml python scripts/vrc_wiring_matrix.py --root .
```

Claim impact: "the repository map has no grey zone" (docs/45 E7) becomes
true again at v0.9; no regulated claim.

## v1.0.0 - Stable Product Release Candidate

Goal: turn the eight sentences of `docs/14` "Definition Of v1.0" into checks
that run. `v1.0` is a *stability* statement about contracts, compatibility,
security process, support model and integration guidance — each of which must
be declared in a machine-readable form, verified in CI, and honest about what
it does not promise. It is not a regulated claim: validated use, QMS
effectiveness and release approval stay on plan 28.

Boundary: a readiness verdict is computed, never a release. The release
decision, tag, notes and approvals remain #561 (regulated lane).

### NRT-023 - Contract Stability Registry And Compatibility Guard

GitHub mapping: `#676` (product, autonomous). Production caller
`nomos contracts status`, registered in `app.go`.

Deliverables:

- `specs/contract-registry.yaml` (`nomos-contract-registry-v1`): every contract
  family under `specs/*.cue` with its current `schema_version`, stability
  (`stable` | `experimental` | `deprecated`), the sha256 of the CUE file at that
  version, its valid/negative fixtures, the CLI readers and writers, and for
  `deprecated` ones `deprecated_since` and `removal_not_before`.
- Go engine + `nomos contracts status --repo-root [--out]`: a CUE file absent
  from the registry, a registry entry without file, a stable contract whose
  file hash changed without a `schema_version` bump, a stable contract without
  a valid fixture, a negative fixture that passes, a deprecated contract without
  dates → each a named refusal (docs/16 "schema changes are evidence-affecting").
- Compatibility fixtures: for a `stable` contract that bumped, the previous
  version's fixture stays and must still be read by the Go reader (where one
  exists) — the read is the proof, not the note.

Definition of done:

- Editing a stable CUE file without bumping turns CI red; adding a CUE file
  without registering it turns CI red; registry and matrix of contracts
  regenerate deterministically. Registry entry `contract_stability_registry`
  (`real`).

Verification:

```bash
cd cli && go test ./internal/contracts/... -v && go run . contracts status --repo-root ..
```

Claim impact: "contracts are versioned and their changes are caught" — not
"contracts are final".

### NRT-024 - Compatibility Matrix, Version Announcement, Deprecation Enforcement

GitHub mapping: `#677` (product, autonomous; depends on NRT-023).

Deliverables:

- `nomos version --json` announces the core version, the schema versions it
  reads and writes (from the contract registry), the adapter-manifest contract
  version and the attestation/bundle format versions.
- The compatibility matrix of `docs/16` becomes a generated section (registry →
  Markdown, `--emit-docs`, `git diff --exit-code` in CI) instead of a
  conceptual example.
- Adapter manifests (`adapters/*/adapter.nomos.yaml`) are checked against the
  core version: `compatibility.nomos_core.min_version`/`max_version` must
  include the current core or the adapter is reported incompatible (fail-closed
  in `nomos contracts status`).
- Deprecation enforcement: reading a `deprecated` contract prints a warning
  naming `removal_not_before`; a deprecated contract past that date without a
  `MAJOR` bump is a refusal.

Definition of done:

- Version output, matrix and adapter compatibility are computed from the same
  registry; a hand-edited matrix or an adapter outside the range turns CI red.
  Registry entry `compatibility_matrix` (`real`).

Verification:

```bash
cd cli && go run . version --json && go test ./internal/contracts/... -run Compat -v
```

Claim impact: "the core announces what it reads and writes; adapters declare
and are checked against it".

### NRT-025 - Security Process, Executable

GitHub mapping: `#678` (DevOps, autonomous, independent).

Deliverables:

- `govulncheck` on `cli/` and `tools/sigstore-verifier/`, `pip-audit` (or
  equivalent) on the Python sidecar dependencies, as CI gates.
- `docs/security/vulnerability-allowlist.yaml`: every accepted finding carries
  id, justification, owner, `expires_on`; an expired or undated entry turns
  the gate red; the gate reads it, never the other way round.
- Dependabot configuration for Go modules, GitHub Actions and Python.
- `docs/security/security-process.yaml` (`nomos-security-process-v1`): intake
  channel, triage targets (declared as targets), disclosure rule, supported
  versions reference, links to the vulnerability SOP; `SECURITY.md` gains a
  generated "Supported Versions" section from the support model (NRT-026 input,
  nonblocking: until then the section is generated from `CHANGELOG.md`).

Definition of done:

- A known-vulnerable dependency pinned in a fixture module turns the gate red;
  an allowlist entry without expiry turns it red; the process file validates.
  Registry entry `security_process_gates` (`sidecar`).

Verification:

```bash
python scripts/security_process_gate.py --root . --check
```

Claim impact: "dependencies are scanned in CI and exceptions expire" — never
"secure" or "certified".

### NRT-026 - Support Model, Declared And Checked

GitHub mapping: `#679` (DevOps, autonomous, independent).

Deliverables:

- `docs/support-model.yaml` (`nomos-support-model-v1`): supported versions and
  their lifecycle state, channels, response targets (declared), tested
  platforms, toolchain versions, explicitly unsupported surfaces (hosted
  service, control plane, App), end-of-support rule.
- Guard: tested platforms equal the CI matrix; the Go version equals
  `cli/go.mod`; every supported version exists as a tag or as the current
  candidate; a generated "Support" section in `README.md` and `SECURITY.md`
  with drift check.

Definition of done:

- Editing the CI matrix, `go.mod` or the tag set without updating the model
  turns CI red; hand-editing the generated sections turns CI red. Registry
  entry `support_model` (`sidecar`).

Verification:

```bash
python scripts/support_model_guard.py --root . --check
```

Claim impact: "support is declared and consistent with what CI tests" — not
"support is contractually guaranteed".

### NRT-027 - Customer Integration Guide, Commands Replayed Against Fixtures

GitHub mapping: `#680` (product, autonomous; depends on NRT-023).

Deliverables:

- `docs/48-customer-integration-guide.md`: one entry point consolidating the
  user manual (34), the integration manual (35), the GitHub workflow setup
  (31) and the App boundary (32) from the integrator's point of view; every
  contract it relies on is named with its stability from the registry; the
  regulated checklist (`templates/regulated/customer-integration-checklist.md`)
  is linked, never duplicated.
- Every command block marked `<!-- replay -->` is executed by
  `scripts/integration_guide_replay.py` against repository fixtures in CI;
  the expected artifacts named in the guide must exist after the run.

Definition of done:

- A command that stops working, or a contract named with a stability the
  registry does not confirm, turns CI red. Registry entry
  `customer_integration_guide` (`sidecar`).

Verification:

```bash
python scripts/integration_guide_replay.py --root . --guide docs/48-customer-integration-guide.md
```

Claim impact: "the integration guide runs" — not "customers have validated it".

### NRT-028 - v1.0 Readiness Verdict, Computed

GitHub mapping: `#681` (product, autonomous; depends on NRT-023, NRT-024,
NRT-027; NRT-025 and NRT-026 are nonblocking inputs).

Deliverables:

- `nomos portfolio release-readiness --repo-root [--out]`: maps each of the
  eight `docs/14` v1.0 criteria to a machine check (contract registry with no
  refusal and every stable contract with compat fixtures; unsupported-block
  policy and strict fidelity gate `real` in the wiring matrix; adapter
  manifests compatible; release candidate and bundle verification available;
  every `regulated_tool` in `roadmap-lanes.yaml` with intended use, validation
  state and reliance; evidence ledger citing versioned contracts; claim guard
  green; security gates and support model present when delivered) and answers
  `ready` or `not_ready` with every unmet criterion named — never `released`.
- CI publishes the verdict with the portfolio artifacts and asserts
  `not_ready` today, as a tripwire, until the criteria are met on purpose.

Definition of done:

- Each criterion has a test that breaks it and sees it named; a forged
  `ready` file is refused on re-read; the verdict binds the status digest of
  NRT-019. Registry entry `release_readiness_verdict` (`real`).

Verification:

```bash
cd cli && go test ./internal/portfolio/... -run Readiness -v && go run . portfolio release-readiness --repo-root ..
```

Claim impact: "v1.0 readiness is computed from the tree" — the release itself
is #561.

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

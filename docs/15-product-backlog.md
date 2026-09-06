# 15 - Nomos Product Backlog

Date: 2026-09-05
Current release: `v0.2.0-ALPHA` (2026-09-06); next stage: `v1.0` stable release candidate (EPIC H)

## Backlog Rule

This file reflects the active product backlog state after the alpha release. Historical issue waves are not repeated here as open work once they have been merged.

The implementation issue list for the next releases lives in
[`docs/29-post-alpha-release-issue-list.md`](29-post-alpha-release-issue-list.md).
Create the GitHub child issues from that document before coding a
release wave.

Roadmap routing is defined by
[`docs/47-roadmap-lanes-and-risk-based-validation.md`](47-roadmap-lanes-and-risk-based-validation.md)
and enforced from [`docs/roadmap-lanes.yaml`](roadmap-lanes.yaml). Product,
DevOps and regulated assurance are independent. Only `dispatch:autonomous`
items enter the engineering queue; calendar evidence, human records,
acquisitions and public writes block their named claim, never task selection.

The GitHub workflow integration issue list lives in
[`docs/30-github-workflow-integration-issue-list.md`](30-github-workflow-integration-issue-list.md).
It covers source-PR triggered NOMOS runs, output-owned workflows,
risk-based publication, mandatory trace manifests, optional source PR
comments, and GitHub App readiness.

The regulated domain opportunity issue list lives in
[`docs/38-domain-opportunity-roadmap.md`](38-domain-opportunity-roadmap.md).
It covers the post-alpha domain expansion lane for GxP/CSV, medical/SaMD,
AI governance, finance/RegTech, legal/eDiscovery, Six Sigma/CAPA,
verifiable evidence, cyber supplier assurance, high-assurance engineering,
ALM/QMS interoperability, domain-pack packaging, and control-plane
opportunities. Its GitHub issues are `#412` through `#435`.

Each active backlog item must have:

- an owner or GitHub issue;
- a dependency relationship;
- an evidence artifact or testable exit gate;
- a clear claim impact.
- one lane (`product`, `devops`, `regulated`) and one dispatch state
  (`autonomous`, `passive`, `human`, `external`).

## Current Autonomous Queues

`docs/roadmap-lanes.yaml` orders Product and DevOps independently; the table
below is generated from it and checked for drift in CI:

<!-- roadmap-queues:begin -->
<!-- GENERATED from docs/roadmap-lanes.yaml by scripts/roadmap_lane_guard.py --emit-docs; do not edit by hand, CI fails on drift -->
| Product queue | DevOps queue |
|---|---|
| #677 — Compatibility matrix, version announcement, deprecation enforcement (NRT-024) | — |
| #680 — Customer integration guide with commands replayed against fixtures (NRT-027) | — |
| #681 — v1.0 readiness verdict, computed (NRT-028) | — |
<!-- roadmap-queues:end -->

Non-dispatchable regulated work is visible in the same registry: #560 is
passive evidence accumulation; #561/#562/#194 require authentic human acts;
#192/#193/#196/#638 require external acquisition or irreversible activation.
None is a product/DevOps dependency, and neither autonomous queue waits for the
other.

## Delivered Alpha Capabilities

The following capabilities are treated as delivered for `v0.1.0-ALPHA`, subject to CI validation and release tagging:

- CLI version set to `0.1.0-ALPHA`.
- Project diagnosis and admission commands.
- Corpus scan, manifest, diff, sidecar validation, feed, and attestation commands.
- RBOK lawbook profile.
- Source spans on generated feed nodes.
- Typed semantic nodes for tables, links, callouts, code blocks, and images.
- Certified table of contents.
- Governed lexicon artifact.
- RAG metadata output.
- Runtime import projection.
- Strict fidelity gate and release gate wiring.
- Regulated-by-design documentation structure.
- Public claim boundary and release notes.

## Alpha Dependency Tree

```text
CLI and corpus commands
  -> RBOK lawbook profile
  -> feed artifacts
  -> source spans + typed blocks + certified TOC + lexicon
  -> strict fidelity gate
  -> RBOK POC dossier
  -> v0.1.0-ALPHA release claim

Public claim boundary
  -> README
  -> release notes
  -> regulated docs
  -> GitHub pre-release

Public reference tooling (#641, #644)
  -> public provenance evidence (autonomous)

Licensed acquisition and decisions (#192, #193, #194, #196)
  -> named clause-level claims only (independent regulated roadmap)

Praxis technical boundary (#320 closed)
  -> autonomous schema/import/reject fixtures on not_qualified inputs

Accepted regulated activation (independent roadmap)
  -> joint claim / NQ-4 candidate
```

## Next Implementation Epics

### EPIC A - Portable Structure Fidelity

Goal: remove RBOK-specific confidence bias by proving the atomization model across multiple document families.

Work:

- AST-to-Nomos comparison for Markdown fixtures with H1-H6, tables, lists, callouts, code, links, images, annexes, xrefs, and front matter.
- Exact line/column/byte spans for every active source block.
- Golden fixtures for legal text, regulatory text, technical standard, business corpus, game rules, YAML, and JSON.
- Explicit unsupported-block records where support is not implemented.
- Short critical atom reconciliation: prove that short but meaningful fragments such as `GxP`, `ALCOA+`, `21 CFR`, `SOP-01`, `P0`, `Yes/No`, status values, thresholds, and identifiers are either represented with their parent context, promoted into governed lexicon/value/identifier atoms, or explicitly classified as non-semantic. They must not become orphan RAG chunks, but they also must not disappear without a disposition.

Exit gate:

```text
No active source block is silently dropped, and every generated node has a source span or explicit unsupported status.
```

#### Delivered Foundation - Short Critical Atom Reconciliation (#382)

Historical issue: `#382` (closed; not a live dispatch target).

Problem statement:

`v0.1.0-ALPHA` records `0` feed units <= 10 characters. That is a useful noise-control result, because punctuation-only and separator-only fragments should not enter the curated feed or RAG. It is not, by itself, a proof that every short critical term was semantically represented. Regulated and operational corpora often carry high-value meaning in short strings: acronyms, clause references, severity labels, status values, yes/no answers, IDs, thresholds, units, and option codes.

Dependency relationship:

```text
Source segment ledger
  -> short-fragment inventory
  -> criticality classifier
  -> disposition report
  -> lexicon/value/identifier promotion where required
  -> semantic quality gate
  -> RAG/context proof
```

Required work:

- Emit a `short-critical-atoms.json` report for every processed corpus, covering excluded short fragments with source id, source span, parent chain, table/YAML/JSON path where applicable, surrounding context, and current disposition.
- Classify each short fragment as `non_semantic`, `contextualized_in_parent`, `lexicon_atom`, `identifier_atom`, `normative_value_atom`, or `requires_review`.
- Promote critical short strings into governed lexicon/value/identifier artifacts when they carry standalone meaning, without creating orphan <=10-character RAG chunks.
- Add fixtures for Markdown paragraphs, lists, tables, callouts, YAML, JSON, legal/regulatory clauses, technical standards, business corpora, and game rules.
- Make the semantic quality gate fail closed when a short critical fragment remains `requires_review` or has no disposition.

Definition of done:

- Body ledger still reports `0` uncovered bytes for admitted text sources.
- Curated feed/RAG still reports `0` junk or orphan <=10-character units.
- Every critical short fragment has a machine-readable disposition and a human-reviewable source span.
- RAG chunks containing critical short terms include enough parent context to be useful and citable.
- Regression fixtures cover at least `GxP`, `ALCOA+`, `21 CFR`, `SOP-01`, `P0`, `Yes`, `No`, threshold values, status labels, table cells, and structured YAML/JSON scalars.
- The strict gate exposes unresolved short-critical findings as blocking evidence.

### EPIC B - Reference Tooling And Public Provenance

Goal: provide gates and retained provenance without waiting for licensed-source
acquisition.

Work:

- Deliver #641 (licence/no-full-text gates) and #644 (actual public-source processing).
- Maintain licensed sidecars without redistributing restricted content.
- Create public surrogate annexes only where license permits.
- Map references to controls, tests, evidence, waivers, and public claims.
- Leave #192/#193/#194/#196 on the independent regulated roadmap; each blocks
  only the named clause-level use.

Exit gate:

```text
Every cited regulation, standard, or framework is mapped or explicitly marked not applicable / blocked.
```

### EPIC C - Release-Support Tooling

Goal: make a release candidate reconstructible without inventing the authentic
release decision.

Work:

- Deliver #639: generate/verify a candidate bundle per commit (optional real tag).
- Retain CI run URLs, hashes, reports, source/corpus attestation, deviations,
  waivers and explicit pending approval/decision states.
- Activate GitHub-native QMS evidence exports.
- Never generate owner, training, signature or approval records.

Exit gate:

```text
A reviewer can reconstruct the candidate and see exactly which regulated
decisions remain pending. Plan 28 owns actual approval/publication.
```

### EPIC D - RAG And Conversational Governance

Goal: make downstream LLM/RAG use precise, concise, cited, and bounded.

Work:

- Retrieval metadata evaluation.
- Citation and refusal tests.
- Prompt-injection and excessive-agency tests.
- Behavior contract for single-question conversational steps.
- Infomaniak model catalog integration where the downstream product requires Swiss-only AI infrastructure.

Exit gate:

```text
RAG output is source-backed, concise by contract, and never replaces canonical authority.
```

### EPIC E - Nomos/Praxis Compatibility

Goal: connect Nomos canonical evidence to Praxis runtime assurance without overclaiming either product.

Work:

- Use the technical boundary delivered by closed issue `#320`.
- Publish atom mapping and evidence ledger contract.
- Run schema/import/reject fixtures on synthetic or `not_qualified` inputs
  before regulated activation.
- Let Praxis rely on Nomos artifacts as regulated evidence only after its own
  activation gate is accepted.
- Feed Praxis runtime evidence and CAPA status back into Nomos release decisions.

Exit gate:

```text
Joint Nomos/Praxis claims are backed by a shared contract and both products declare their own quality level.
```

### EPIC F - Regulated Domain Expansion

Goal: turn NOMOS from a single alpha proof into a portable domain-profile
platform without weakening the claim boundary.

Work:

- Close `DOR-001` through `DOR-004` first: domain profile schema, claim
  ladder, reference intake policy, and multi-domain golden corpus pack.
- Implement domain packs only after the common profile contract exists.
- Prioritize `gxp-csv`, `ai-governance`, and
  `cyber-supplier-assurance` because they align most directly with the
  current regulated documentation and RAG evidence needs.
- Keep finance, legal, medical, Six Sigma, provenance, and high-assurance
  profiles blocked or exploratory until the common evidence gates can
  express `mapped`, `blocked`, `not_applicable`, and `waived` states.

Exit gate:

```text
Every domain profile declares intended use, references, risk class,
claim ladder, required artifacts, blocked claims, verification commands,
and current evidence status.
```

Claim impact:

Domain packs may be advertised only as scoped evidence-support packages.
They do not create compliance, certification, legal advice, medical
validation, financial regulatory approval, or high-assurance
qualification claims.

### EPIC G - Portfolio Governance

Goal: answer "where does the portfolio stand" from computed status, never from
narrative (roadmap v0.9.x). Planned as NRT-019 to NRT-022 in
[29](29-post-alpha-release-issue-list.md#v090---portfolio-governance).

Work:

- Portfolio status contract and engine over machine sources only (registry,
  matrix, lanes, ledger gaps, CAPA, reviews, release candidate, Praxis gate,
  repeated CI, competence, packs, public sources).
- Findings and periodic-review index with a query surface and consistency
  findings.
- Review-record index and guard (DevOps sidecar).
- Control-plane decision under ADR-0006: wire the multi-project view behind a
  real caller or remove the archived code.

Exit gate:

```text
Every number in a management review input is reproducible from committed files
by one command, and stale or unavailable sources are visible, not hidden.
```

Claim impact:

A computed view lifts no claim. Regulated validation, approvals and records
remain on roadmap 28.

### EPIC H - Stable Product Release Candidate (v1.0)

Goal: make the eight `docs/14` v1.0 criteria checkable. Planned as NRT-023 to
NRT-028 in [29](29-post-alpha-release-issue-list.md#v100---stable-product-release-candidate).

Work:

- Contract stability registry with a compatibility guard (stable contract
  changed without bump → red; compat fixtures read by the engine).
- Compatibility matrix and version announcement generated from the registry;
  adapter ranges checked; deprecations enforced.
- Security process executable: vulnerability scans as gates, expiring
  allowlist, Dependabot, declared process file.
- Support model declared and checked against CI matrix, toolchain and tags.
- Customer integration guide whose commands are replayed against fixtures.
- v1.0 readiness verdict computed, never "released".

Exit gate:

```text
Every v1.0 criterion of docs/14 is a check that runs in CI and names what it
finds; the readiness verdict is not_ready until each is met on purpose.
```

Claim impact:

Stability of contracts, process and support is declared and checked. It is
not a regulated claim: validated use, QMS effectiveness and release approval
stay on roadmap 28.

## SFI Wave Status

- SFI-11 (#349) shipped: dossier + command sequence. The alpha release records a bounded RBOK `01_rbok` evidence pack; this does not promote Nomos to universal-fidelity or regulated-validation status.

## FSQ Wave Status (epic #363)

- FSQ-01 (#364) shipped: feed audit (`cli/internal/corpus/cmd/feed-audit/`).
- FSQ-02 (#365) shipped: explicit source admission and non-atomization policy.
- FSQ-03 (#366) shipped: table-row units replace bare table-cell leaks.
- FSQ-04 (#367) shipped: YAML raw/decoded key-path scalar provenance.
- FSQ-05 (#368) shipped: corpus body ledger separate from curated feed.
- FSQ-06 (#369) shipped: semantic quality gate (`CheckSemanticQuality`).
- FSQ-07 (#370) shipped: context-rich RAG chunk composer (`ComposeRAGChunks`).
- FSQ-08 (#371 / #379 / #380) shipped: `scripts/rbok-poc-integrity.sh` was extended through the integrity stages, `docs/rbok-poc-validation-dossier.md` records the AQ-3 bounded POC dossier, and the alpha release notes record the passing evidence pack. Remaining product work is broader adapter/portable-fidelity coverage. Repeated private-CI evidence is the passive regulated issue #560; attestation `claim_coverage` wiring has shipped.

## NGW Wave Status (GitHub workflow integration)

- NGW-01 (#386) shipped: workflow config schema (`specs/nomos-github-workflow.cue`).
- NGW-02 (#387) shipped: trace manifest schema (`specs/nomos-trace-manifest.cue`).
- NGW-03 (#388) shipped: scoped diff planner + `nomos github plan` command.
- NGW-04 (#389) shipped: reusable GitHub Actions workflow (`.github/workflows/nomos-corpus-workflow.yml`) plus two caller templates (`templates/github-workflows/nomos-source-pr.yml`, `nomos-output-dispatch.yml`). Read-only corpus checkout (`persist-credentials: false` AND push remote DISABLED). NGW-04 reads + plans + uploads only; publication is NGW-005 / #390 territory.
- NGW-08 (#393) shipped: source-owned and output-owned setup docs (`docs/31-github-workflow-setup.md`) — config-owner choice, secrets matrix, permissions, branch-protection expectations, publication-mode tradeoffs, step-by-step install, verification checklist, troubleshooting. Forward-references `docs/32-github-app-readiness-boundary.md` (NGW-09 / #394, parallel).
- NGW-10 (#395): E2E fixture workflow shipped — synthetic ≤2KB corpus + output pair under `tests/fixtures/ngw-e2e/`, 14 Python unittest cases (`tests/test_ngw_e2e_fixture.py`) and a bash driver (`scripts/ngw-e2e-fixture.sh`) exercising the planner + all three publication modes (`artifact_only`, `pull_request`, `direct_push`) in dry-run, with trace-manifest cue-validation per mode and a fixture-corpus read-only invariant (pre/post snapshot diff).

## Non-Goals For The Alpha

The alpha backlog does not include:

- formal FDA, EU GMP, ISO, or NASA certification;
- customer-specific GxP validation;
- universal PDF/DOCX/OCR fidelity;
- production vector-store tuning;
- open-source licensing;
- e-signature approval semantics as a validated Part 11 platform.

Those remain future scoped work and must not appear as delivered claims.

# Changelog

All notable changes to Nomos are tracked here. The project uses explicit alpha/beta labels until the public API, evidence contracts, and support model are stable enough for a `v1.0` release (planned as NRT-023..028 in `docs/29`).

## Unreleased

### Added

- Concepts learned from a neighbouring sovereign legal RAG (docs/49): domain cartography
  contract `specs/domain-cartography.cue` with fixtures vetted in CI (an unverified layer
  carries no number, a phantom domain owns no collection), parameter inventory template,
  inference-boundary control in the AI/RAG governance baseline, doctrine principle 8
  « ce qui se tait ment », and a silence guard over the sidecar scripts.
- Support model, declared and checked (NRT-026 #679): `docs/support-model.yaml` and
  `scripts/support_model_guard.py` — tested platforms equal the CI matrix, Go versions equal
  `cli/go.mod`, every declared version is a tag or the current candidate, every tag is
  declared, dates match the changelog; Support sections generated into the READMEs and
  `SECURITY.md`, whose Supported Versions table now renders from the same model.
- Security process, executable (NRT-025 #678): `docs/security/security-process.yaml`
  and an expiring `vulnerability-allowlist.yaml` read by `scripts/security_process_gate.py`;
  `govulncheck` (called vulnerabilities, standard library included) and `pip-audit` on the
  pinned sidecar requirements as a CI job; Dependabot for Go modules, GitHub Actions and
  Python; the Supported Versions section of `SECURITY.md` generated from the changelog.
  The `toolchain go1.26.6` directive remediates the called standard-library findings.

## v0.2.0-ALPHA - 2026-09-06

Second public alpha. Everything below is a registered capability whose status
is computed in CI from the tree (`.vrc-wiring-matrix/`): 57 capabilities,
44 `real`, 11 `sidecar`, 2 `absent` by design, 0 mismatch. The release
decision is recorded in
`docs/regulated/lifecycle/release-records/v0.2.0-ALPHA-release-decision.yaml`;
the release notes are `docs/release-v0.2.0-alpha.md`.

### Added

- Vision-Reality Closure (epic #545): generated wiring matrix and claim-boundary
  guard as CI gates; production callers for every engine capability; cite-or-abstain,
  canon promotion and point-in-time moved into the Go engine; RAG eval harness with
  context metrics; public cite-or-abstain bench with dated, replayed results;
  domain-pack contract and gate with a second vertical (EU AI Act); PDF/DOCX/HTML
  adapters behind capability kits; Swiss live connector; evidence packs as
  CycloneDX/SPDX BOM; deterministic cross-reference graph; facet SHACL validation.
- Rule execution behind a versioned external process boundary with request,
  results and response digests (#642).
- Web-source contract (#610), immutable external snapshot verifier/importer
  (#611) and the offline Recursio → NOMOS end-to-end fixture (#612) with the
  attestation binding the web source type and snapshot coverage.
- Release candidate bundle `nomos release candidate|verify`: content and status
  validated, approvals never invented, VRC-14 recorded as a risk, CI rehearsal
  archiving `v0.2.0-ALPHA-candidate` without publishing (#639).
- Offline Sigstore bundle verification `nomos attest verify-sigstore` and keyless
  issuance against injected non-production services `nomos attest sign-sigstore`,
  both behind the process boundary of ADR-0005 (`tools/sigstore-verifier`);
  production issuance remains forbidden (#637, #645).
- Static SKOS authoring and deterministic distribution of the facet
  vocabularies (#643).
- Nomos/Praxis evidence exchange contract, atom mapping fixture and computed
  activation gate — `blocked` today, never `activated` (#660, #661, #662).
- Portfolio governance: `nomos portfolio status|findings|reviews|projects`
  computed from committed machine sources; review-record index and guard;
  control-plane decision executed (ADR-0007) (#667–#670).
- Licence review and real no-full-text gate (#641); real public references
  captured hash-only with retained artifacts (#644); competence gate aligned to
  the template without fabricating records (#640).
- Independent roadmaps by lane with an executable registry
  (`docs/roadmap-lanes.yaml`, ADR-VRC-0004) and generated queue tables.

### Changed

- `nomos version` reports `0.2.0-ALPHA`.
- `control-plane/` removed: the multi-project view lives in `nomos portfolio
  projects`; registry and storage had no consumer (ADR-0007 supersedes ADR-0006).
- The candidate bundle spec, the Praxis activation record and the portfolio
  status make every pending human decision visible instead of narrated.

### Known open items (recorded, not hidden)

- Regulated lane: #560 (4/8 consecutive green runs), #561 (release SOP
  execution — this release records the owner's decision), #562 (competence
  records), #192/#193/#194/#196 (licensed references), #638 (production
  Sigstore), #576 (umbrella).
- Portfolio findings on this commit: 23, including 3 management-review actions
  overdue since July 2026 and the 11 unmet Praxis activation requirements;
  the evidence ledger is dated 2026-05-02 and reported stale.

## v0.1.0-ALPHA - 2026-05-03

### Added

- Canonical-first CLI with `init`, `validate`, `diagnose`, `corpus`, `version`, and `help`.
- Corpus commands for scan, manifest, validation, diff, feed generation, profile diagnosis, and attestation.
- `rbok-lawbook` corpus profile for lawbook-style Markdown reference corpora.
- Read-only corpus processing workflow with source fingerprint verification.
- Certified table-of-contents artifact and dynamic structural-depth checks.
- Source spans for lawbook feed nodes.
- Typed semantic nodes for tables, links, callouts, code blocks, and images.
- Governed lexicon artifact generation.
- RAG metadata and runtime import projection for downstream application integration.
- Strict fidelity gate wired into the release gate.
- In-toto style corpus attestation output.
- Regulated-by-design documentation structure, templates, evidence pack scripts, and GitHub operating model.
- GitHub Actions for CI, corpus tests, RBOK lawbook E2E, RBOK runtime E2E, fidelity proof reports, regulated documentation gate, and regulated evidence pack.

### Changed

- Release gate now fails if `rbok-strict-fidelity-gate.json` is missing or contains blocking findings.
- Certified TOC generation now uses the canonical TOC generator instead of an ad hoc hash.
- Tables now carry `col_count`, `row_count`, and header metadata.
- Unlabeled fenced code blocks are typed as `plain_text` with `language_declared=false`.
- Windows PowerShell E2E output uses ASCII separators for compatibility with PowerShell 5.1.
- Public documentation now states the alpha claim boundary and regulated-readiness status.

### Validated

- RBOK lawbook POC against a read-only clone of `realisons-business/01_rbok`.
- 240 source files scanned.
- 7191 feed nodes generated.
- 1090 certified TOC entries.
- 7191 nodes with spans.
- Strict fidelity gate passed with 0 blocking findings and 0 findings.
- Fidelity proof reported `full_fidelity_proven`.
- Source mutation check passed.

### Known Limitations

- This release is an alpha and not a regulated certification.
- Customer-specific validation, supplier qualification, security review, and legal review are still required for regulated use.
- Portable fidelity beyond the current Markdown/RBOK lawbook path needs additional corpus-specific validation.
- PDF, DOCX, complex table, image, annex, and mixed-format corpus coverage remains an expansion track.

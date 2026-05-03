# Changelog

All notable changes to Nomos are tracked here. The project uses explicit alpha/beta labels until the public API, evidence contracts, and support model are stable enough for a `v1.0` release.

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

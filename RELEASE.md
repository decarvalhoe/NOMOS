# Release v0.2.0-ALPHA

Nomos `v0.2.0-ALPHA` is the second public alpha of the canonical-first product intelligence platform. `v0.1.0-ALPHA` (2026-05-03) proved the source-to-feed chain on one private corpus; `v0.2.0-ALPHA` (2026-09-06) closes the vision/reality gap audited in `docs/45`: every capability is registered, its status is computed in CI, and every pending human decision is visible as data. Per-version detail lives in `CHANGELOG.md` and `docs/release-v0.2.0-alpha.md`.

## Release Intent

This release is designed to support:

- corpus assessment and canonical-first project bootstrapping;
- read-only processing of authority source repositories;
- lawbook-style Markdown corpus feeds;
- traceable RBOK proof-of-concept validation;
- regulated-readiness documentation and evidence-pack preparation;
- internal customer demos and commercial qualification discussions.

It is not intended to be sold or represented as a certified regulated platform. It is an alpha that proves a substantial pipeline and defines the honest evidence boundary for regulated customers.

## Delivered Capabilities

| Area | v0.2.0-ALPHA status (v0.1.0-ALPHA baseline unchanged unless noted) |
|---|---|
| CLI project diagnosis | Implemented |
| Corpus scan, manifest, diff, sidecar validation | Implemented |
| Corpus feed and attestation | Implemented |
| RBOK lawbook profile | Implemented and POC-tested |
| Source spans | Implemented for generated lawbook feed nodes |
| Typed semantic blocks | Implemented for tables, links, images, callouts, and code blocks |
| Certified table of contents | Implemented and release-gated |
| Governed lexicon artifact | Implemented initial extraction |
| RAG metadata | Implemented as traceable metadata output |
| Runtime import projection | Implemented for RBOK engine integration |
| Strict fidelity gate | Implemented and release-gated |
| Regulated documentation baseline | Installed and gated for structure |
| Evidence pack automation | Implemented initial scripts |
| Formal customer validation package | Partial, not complete |
| Formal regulatory certification | Not claimed |

## Alpha POC Evidence

The latest RBOK lawbook POC was executed against a read-only clone of `realisons-business/01_rbok`.

| Metric | Result |
|---|---|
| Source files scanned | 240 |
| Generated feed nodes | 7191 |
| Certified TOC entries | 1090 |
| Nodes with spans | 7191 |
| Strict fidelity gate | pass |
| Blocking findings | 0 |
| Non-blocking findings | 0 |
| Fidelity proof status | `full_fidelity_proven` |
| Source mutation verification | pass |

## Release Gates

Local gates used for this release:

```bash
go test ./...
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/e2e.ps1
python -m unittest discover -s tests -v
```

GitHub Actions gates:

- Go vet and test for the CLI (the multi-project view lives in `cli/internal/portfolio`, ADR-0007);
- CUE vet;
- YAML lint;
- corpus tests on Linux, macOS, and Windows;
- RBOK lawbook E2E;
- RBOK runtime E2E;
- fidelity proof report generation;
- regulated documentation gate;
- regulated evidence pack.

## Claim Boundary

`v0.2.0-ALPHA` produces corpus feeds, fidelity proof reports, attestations, release candidate bundles, portfolio status and findings. Artifact generation is not, by itself, a proof of source-to-feed fidelity.

Full source-to-feed fidelity is **not yet proven** at the platform level. The blocking proof chain is being implemented under epic `#337` (Source-to-feed integrity and semantic feed hygiene) and depends in particular on:

- `#342` (SFI-04) source integrity gate — coverage, duplicates, junk, unsupported blocks;
- `#345` (SFI-07) feed quality gate — semantic feed and RAG linkage;
- `#346` (SFI-08) strict gate and CI wiring of the corpus integrity report.

The FSQ epic (`#363`) extends the chain with semantic feed-quality gates (FSQ-06 `#369`), context-rich RAG composition (FSQ-07 `#370`), the corpus body ledger (FSQ-05 `#368`), and the FSQ-08 (`#371`) AQ-3 evidence pack. The AQ-3 evidence pack is **ready** when `scripts/rbok-poc-integrity.sh` runs green against `realisons-business/01_rbok`; until a real run is recorded the AQ-3 bounded claim in `docs/rbok-poc-validation-dossier.md` is templated and **not** advertised. AQ-3 does **not** establish AQ-4 (regulated validation) or AQ-5 (certification).

Until those gates are wired and passing, the literal phrase `full_fidelity_proven` is reserved. It must only be emitted by the platform for a build whose corpus integrity report is present and passing. Today this phrase appears on the RBOK POC line above as the result of the existing strict fidelity gate; that claim is scoped to that specific POC corpus and configuration, not to a general source-to-feed proof.

Distinguish:

- **Artifact generation (today)** — feeds, attestations, fidelity proof report, certified TOC, lexicon, RAG metadata, runtime import artifacts.
- **Fidelity proof (gated, future)** — full source-to-feed fidelity claim conditioned on a passing corpus integrity report (`#338`, `#342`, `#345`, `#346`).

No public document, attestation, or release artifact may upgrade an artifact-generation result into a fidelity proof until the corpus integrity gate is in place.

## Known Limitations

- The robust POC target is RBOK lawbook-style Markdown; portability to every legal, regulatory, PDF, DOCX, HTML, table-heavy, image-heavy, or nested annex corpus still requires more validation.
- The regulated documentation set is a baseline, not an approved QMS.
- Licensed reference standards may be tracked by intake record, but their full text must be handled according to license terms and may not be redistributed.
- RAG metadata is generated, but production vector-store ingestion, retrieval evaluation, and LLM behavior control remain integration work.
- Customer validation remains customer-specific and must be performed against intended use, deployment context, risk, and SOPs.
- Open regulated items remain: repeated CI evidence (`#560`, 4/8), release SOP execution (`#561` — this release records the owner's decision, see the release record), competence records (`#562`), licensed reference acquisition/review (`#192`, `#193`, `#194`, `#196`), production Sigstore issuance (`#638`). The Praxis activation gate is `blocked` (11 unmet requirements) and the portfolio reports 23 open findings on the release commit.

## Release Decision

Release as `v0.2.0-ALPHA` if:

- `main` contains this release documentation;
- all GitHub required checks are green;
- the GitHub release is marked as pre-release;
- release notes include the alpha limitations and claim boundary;
- no document claims formal regulated certification.

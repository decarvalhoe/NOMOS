# Release v0.1.0-ALPHA

Nomos `v0.1.0-ALPHA` is the first public alpha release candidate for the canonical-first product intelligence platform.

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

| Area | v0.1.0-ALPHA status |
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

- Go vet and test for CLI and control-plane packages;
- CUE vet;
- YAML lint;
- corpus tests on Linux, macOS, and Windows;
- RBOK lawbook E2E;
- RBOK runtime E2E;
- fidelity proof report generation;
- regulated documentation gate;
- regulated evidence pack.

## Known Limitations

- The robust POC target is RBOK lawbook-style Markdown; portability to every legal, regulatory, PDF, DOCX, HTML, table-heavy, image-heavy, or nested annex corpus still requires more validation.
- The regulated documentation set is a baseline, not an approved QMS.
- Licensed reference standards may be tracked by intake record, but their full text must be handled according to license terms and may not be redistributed.
- RAG metadata is generated, but production vector-store ingestion, retrieval evaluation, and LLM behavior control remain integration work.
- Customer validation remains customer-specific and must be performed against intended use, deployment context, risk, and SOPs.
- Open follow-up issues remain for stronger RBOK proof (`#314`), Nomos/Praxis atom mapping (`#320`), licensed reference acquisition/review (`#192`, `#193`, `#194`), and public/licensed bible processing (`#196`).

## Release Decision

Release as `v0.1.0-ALPHA` if:

- `main` contains this release documentation;
- all GitHub required checks are green;
- the GitHub release is marked as pre-release;
- release notes include the alpha limitations and claim boundary;
- no document claims formal regulated certification.

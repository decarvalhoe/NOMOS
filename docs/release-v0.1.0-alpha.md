# Nomos v0.1.0-ALPHA Release Notes

Date target: 2026-05-04

## Release Position

`v0.1.0-ALPHA` is the first public alpha baseline for Nomos as an
authority-to-product engine. It is suitable for demonstrations, internal
capitalisation evidence, controlled POCs, and regulated-readiness review.
It is not a production validation, regulatory certification, eQMS, or
universal-fidelity claim.

## What Is Included

- Go CLI for project and corpus workflows: `diagnose`, `strict`,
  `corpus scan`, `corpus manifest`, `corpus validate-sidecar`,
  `corpus feed`, `corpus body-ledger`, and `corpus attest`.
- Read-only corpus processing guards and mutation checks.
- Markdown lawbook scanning with typed structure and source spans.
- Generic YAML/JSON structured scalar scanning with structured paths,
  exact source spans, and source-backed feed/RAG provenance.
- Certified table-of-contents, governed lexicon, feed, RAG metadata,
  body ledger, strict gate, and attestation artifacts.
- GitHub-native CI gates for Go, CUE, corpus tests, RBOK lawbook E2E,
  RBOK runtime E2E, regulated documentation, and evidence packs.
- Regulated-by-design documentation baseline (structural; not a certification or validation) and public claim boundary.

## Recorded POC Evidence

Bounded RBOK `01_rbok` source-to-feed evidence pack:
`C:\Dev\nomos-rbok-poc-run-20260504-structured-universal-9`

| Evidence | Result |
|---|---:|
| Corpus commit | `ea003e8fe3c35993731c3708a3787df6a3a690df` |
| Sources declared | 240 |
| Feed units | 3024 |
| RAG chunks | 3024 |
| Source-backed feed units | 3024 / 3024 |
| Source-backed RAG chunks | 3024 / 3024 |
| `table_cell` feed units | 0 |
| Units <= 10 characters | 0 |
| Blocking duplicate groups | 0 |
| Semantic quality | `warn`, 0 blocking findings, 6 reviewable warnings |
| Body ledger | 0 uncovered bytes |
| Strict gate | `pass`, exit code 0 |
| Source mutation check | no mutation detected |

## Claim Boundary

Allowed bounded claim: Nomos can demonstrate source-to-feed integrity on
the recorded RBOK `01_rbok` POC run, with source-backed feed units and
RAG chunks, a complete body ledger for admitted text sources, and a
passing strict gate.

Not claimed:

- platform-wide universal fidelity;
- customer regulated validation;
- FDA, EU Annex 11, ISO, GxP, NASA, or Part 11 compliance;
- complete support for every PDF, DOCX, scanned document, image, legal
  code, regulation, or game-rule corpus;
- a completed short-critical atom reconciliation proving that every
  short but meaningful token was promoted, contextualized, or explicitly
  classified;
- attestation `claim_coverage` fully wired to the body ledger.

## Release Gate

The release may be cut only after the release PR is merged and the
following checks are green on the release commit:

- `go test ./...` from `cli/`;
- `scripts/e2e.ps1`;
- GitHub Actions required checks;
- RBOK `01_rbok` POC runner or the documented equivalent evidence pack.

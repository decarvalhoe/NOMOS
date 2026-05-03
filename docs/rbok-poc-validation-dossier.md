# RBOK 01_rbok POC Validation Dossier

Document ID: `POC-RBOK-001`
Status: executed - alpha POC pass
Release context: `v0.1.0-ALPHA`
Date: 2026-05-03
Owner: Nomos core team

## Claim Boundary

This dossier records the Nomos POC pipeline against the `01_rbok` corpus from `realisons-business`. It supports an alpha claim that Nomos can process a real RBOK lawbook corpus into traceable artifacts with a passing strict fidelity gate and read-only source verification.

It does not claim production readiness, customer validation, regulatory certification, or universal fidelity across every document format and corpus shape.

## POC Objective

Demonstrate that Nomos can:

1. Process a real business reference corpus without mutating source files.
2. Extract structured lawbook nodes from Markdown sources.
3. Preserve source spans for generated feed nodes.
4. Emit typed semantic nodes for tables, links, callouts, and code blocks.
5. Generate a certified table of contents.
6. Generate governed lexicon, RAG metadata, runtime import, governance, and attestation artifacts.
7. Fail the release gate if the strict fidelity gate is red.

## Current Execution Summary

Execution date: 2026-05-03
Corpus: read-only clone of `realisons-business/01_rbok`
Nomos release target: `v0.1.0-ALPHA`

| Check | Result |
|---|---|
| Source files scanned | 240 |
| Markdown files detected | 69 |
| Feed nodes generated | 7191 |
| Nodes with spans | 7191 |
| Certified TOC entries | 1090 |
| Structural nodes | 1090 |
| Table nodes | 65 |
| Code block nodes | 25 |
| Callout nodes | 26 |
| Link nodes | 137 |
| Release gate | PASS |
| Strict fidelity gate | PASS |
| Strict blocking findings | 0 |
| Strict total findings | 0 |
| Fidelity proof status | `full_fidelity_proven` |
| Full fidelity claim allowed for this POC output | `true` |
| Read-only source mutation check | PASS |

## Pipeline Under Test

```text
01_rbok source clone
  -> read-only fingerprint
  -> rbok-lawbook profile diagnosis
  -> lawbook artifact pack
  -> certified TOC
  -> governed lexicon
  -> strict fidelity gate
  -> release gate
  -> corpus fidelity proof
  -> artifact validation
  -> read-only fingerprint verification
```

## Artifacts Produced

The POC emits:

- `diagnosis.json`;
- `artifact-pack.json`;
- `rbok-lawbook-feed.json`;
- `rbok-lawbook-index.json`;
- `rbok-rag-metadata.json`;
- `rbok-engine-import.json`;
- `rbok-governance.json`;
- `rbok-attestation.json`;
- `rbok-governed-lexicon.yaml`;
- `rbok-certified-toc.json`;
- `rbok-strict-fidelity-gate.json`;
- `rbok-fidelity-proof.json`.

## Release Gate Evidence

The release gate checks:

- feed presence and node count;
- required node types;
- dynamic structural depth from parent chains;
- valid attestation;
- governance result;
- certified TOC presence and hash;
- strict fidelity gate pass status.

The gate now fails if `rbok-strict-fidelity-gate.json` is missing or contains blocking findings.

## Interpretation

The RBOK POC validates the current `rbok-lawbook` alpha lane:

```text
RBOK authority corpus
  -> declared corpus policy
  -> lawbook nodes with spans
  -> certified document tree
  -> typed semantic structures
  -> governance and lexicon artifacts
  -> RAG and runtime import metadata
  -> scoped attestation
  -> strict release gate
```

This is a credible POC for the Nomos promise on one real structured business corpus. It remains bounded: additional corpora, legal books, regulations, game rules, PDFs, DOCX files, scanned documents, complex annexes, and customer-controlled regulated workflows require their own evidence.

## Known Residual Work

- Broaden portable atomization beyond the RBOK Markdown profile.
- Validate legal/regulatory and game-rules profiles against independent corpora.
- Expand exact locator evidence beyond current span coverage where byte/column evidence is required.
- Mature customer validation packs and approval records.
- Integrate production RAG/vector-store and LLM behavior controls under customer intended use.

## Verdict

**RBOK `01_rbok` POC: PASS for `v0.1.0-ALPHA`.**

The POC supports a bounded alpha claim: Nomos can transform the current RBOK lawbook corpus into traceable, release-gated artifacts without mutating the source corpus. It does not support a blanket claim of universal regulated-grade fidelity.

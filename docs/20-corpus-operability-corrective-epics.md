# Corpus Operability Corrective Epics

## Purpose

The E15-E22 corpus work added useful internal packages, but it did not deliver
the expected operator-facing workflow for RBOK corpus use. This corrective set
defines the missing acceptance layer: real CLI commands, real corpus admission,
read-only proof, CI without dummy fallbacks, and an end-to-end run against the
`RBOKproject/realisons-business` corpus.

## EPIC E23 - Operable Corpus CLI

Nomos must expose public corpus commands instead of leaving corpus behavior as
internal Go libraries.

Acceptance:

- `nomos corpus scan` emits a snapshot outside the source root.
- `nomos corpus manifest` generates a sidecar source manifest from a snapshot.
- `nomos corpus validate-sidecar` validates the sidecar against the source root.
- `nomos corpus diff`, `feed`, and `attest` are callable from the CLI.
- Unknown commands return a non-zero exit code.

## EPIC E24 - Corpus-Aware Admission

Authoritative documentation repositories must not be classified as
`out_of_scope` when explicitly evaluated as corpus inputs.

Acceptance:

- `nomos diagnose --mode canonical_corpus` emits corpus verdicts.
- Repositories with corpus manifests can become `corpus_admissible`.
- Corpus repositories with missing sidecar evidence become `corpus_partial`.

## EPIC E25 - Read-Only Corpus Proof

Nomos must prove that it does not mutate the source corpus while generating
consumer-side artifacts.

Acceptance:

- Output paths inside the source root are rejected before writing.
- Git status is checked before and after scan.
- E2E output is generated outside `realisons-business`.
- `realisons-business` remains clean before and after the run.

## EPIC E26 - Corpus CI Without Dummy Fallbacks

The corpus workflow must run the actual CLI and fail closed.

Acceptance:

- The workflow no longer calls a non-existent test helper.
- The workflow no longer writes a fake `total_files: 0` snapshot.
- Scan, manifest generation, and sidecar validation use public CLI commands.
- A zero-file snapshot fails the workflow.

## EPIC E27 - RBOK Corpus E2E

The real RBOK business corpus must be a regression target for the corpus flow.

Acceptance:

- The scan runs against `C:\Dev\realisons-business`.
- The current allowlisted corpus produces a valid snapshot and manifest.
- The feed and attestation are generated outside the source corpus.
- The diagnosis reports a corpus verdict rather than `out_of_scope`.

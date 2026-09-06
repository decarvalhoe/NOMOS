# Support

Nomos v0.2.0-ALPHA is available for pilot and qualification work; v0.1.0-ALPHA is superseded.

Support is declared in `docs/support-model.yaml` and checked in CI (NRT-026, #679): tested platforms equal the CI matrix, the Go versions equal `cli/go.mod`, every listed version is a tag or the current candidate, and the generated Support sections of the READMEs and `SECURITY.md` carry no hand edit. Declared, not contractually guaranteed.

## What We Can Support In Alpha

- understanding the canonical-first method;
- assessing whether a corpus is a good Nomos candidate;
- running the CLI locally;
- generating RBOK lawbook profile artifacts;
- interpreting fidelity, release gate, and evidence pack output;
- preparing regulated-readiness documentation;
- defining customer-specific validation and shared responsibility boundaries.

## What Requires Project-Specific Work

- production deployment;
- customer security architecture;
- regulated validation package approval;
- GxP, Part 11, Annex 11, or equivalent certification claims;
- licensed reference acquisition and license review;
- integration with customer ALM, eQMS, document management, vector database, or LLM provider.

## Before Asking For Help

Capture:

- Nomos version: `nomos version`;
- command used;
- corpus profile;
- operating system;
- full error output;
- whether the source repository was clean and read-only;
- generated `diagnosis.json`, `rbok-strict-fidelity-gate.json`, or evidence report when relevant.

# Governance

Nomos governance exists to keep the project credible: evidence must constrain claims.

## Decision Levels

| Level | Examples | Required handling |
|---|---|---|
| Routine implementation | Internal refactor, test cleanup, small docs fix | PR, tests where relevant |
| Evidence-affecting change | Release gate, corpus feed, attestation, TOC, spans, RAG metadata | Issue or PR rationale, tests, updated docs |
| Claim-affecting change | Public README, release notes, regulated posture, customer promises | Explicit claim boundary review |
| Regulated-readiness change | QMS docs, validation pack, ALCOA+, Part 11/Annex 11 mapping | Evidence link, gap or approval record |
| Release change | Version tag, release notes, release gate criteria | Green CI, release evidence, approval |

## Claim Boundary Rule

Nomos may only claim what its current evidence supports. Missing evidence must be documented as a gap, not converted into marketing language.

## Source Authority Rule

No business, legal, regulatory, or product behavior should be treated as canonical unless it is connected to a governed source, accepted exception, or documented decision.

## Release Rule

Release candidates must have:

- green required checks;
- explicit release notes;
- known limitations;
- no unresolved claim-boundary contradictions;
- evidence for any public POC metrics;
- a rollback or follow-up plan when relevant.

## Regulated-Readiness Rule

The repository may describe a regulated-readiness track, templates, and controls. It must not claim that Nomos is a validated regulated system until the relevant validation, approval, audit, security, and supplier evidence exists for that intended use.

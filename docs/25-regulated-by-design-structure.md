# 25 - Regulated By Design Structure

Date: 2026-05-02

## Purpose

This document installs the operating structure required to make Nomos and Praxis regulated by design.

It does not claim that either product is compliant today. It defines the minimum repository structure, evidence objects, ownership rules, and release gates that must exist before either product can defend regulated-grade claims.

The execution model is Nomos first, Praxis downstream:

```text
Nomos builds the governed source-to-evidence baseline.
Praxis consumes that baseline for runtime validation, invariants, evidence retention, and CAPA.
Praxis cannot certify or compensate for Nomos artifacts that Nomos cannot yet produce and validate.
```

## Operating Rule

Every regulated claim must have:

- an intended use;
- an owner;
- a risk classification;
- a mapped external reference or explicit non-applicability decision;
- a control requirement;
- an implementation reference;
- a verification reference;
- an evidence artifact;
- a release gate;
- a current status that prevents overclaiming.

Any claim without those fields stays `not_qualified`.

## Repository Structure

The regulated structure is split into two tracked areas.

`docs/regulated/` carries the governed operating records:

```text
docs/regulated/
  README.md
  product-profiles/
    nomos.yaml
    praxis.yaml
  control-matrix/
    README.md
  validation-pack/
    README.md
  supplier-pack/
    README.md
  release-bundle/
    README.md
  ai-rag-governance/
    README.md
  operations/
    README.md
  customer-integration/
    README.md
  decisions/
    README.md
```

`templates/regulated/` carries reusable controlled templates:

```text
templates/regulated/
  README.md
  regulated-product-profile.yaml
  intended-use.yaml
  control-matrix.yaml
  validation-plan.md
  supplier-assurance-pack.md
  release-evidence-bundle.yaml
  alcoa-evidence-envelope.yaml
  ai-rag-governance.md
  customer-integration-checklist.md
  periodic-review.md
```

## Product Responsibilities

Nomos owns the regulated baseline:

- external reference registry;
- canonical corpus read-only policy;
- control matrix;
- intended-use inventory;
- ALCOA+ evidence envelope;
- validation pack;
- supplier assurance pack;
- release evidence bundle;
- public claim governance;
- Nomos-to-Praxis evidence contract.

Praxis owns downstream runtime assurance:

- Praxis project pack;
- runtime scenarios;
- invariant execution;
- runtime evidence retention;
- nonconformity and CAPA records;
- periodic operational trend reports;
- feedback from runtime failures into Nomos controls.

## Status Model

Use these statuses in product profiles, matrices, and release bundles:

| Status | Meaning |
|---|---|
| `not_qualified` | The claim or control is not defensible. |
| `planned` | Scope is accepted, but no implemented evidence exists. |
| `implemented` | Implementation exists but is not independently verified by a gate. |
| `verified` | A repeatable gate proves the artifact or behavior. |
| `approved` | A named reviewer approved the evidence for a release. |
| `waived` | A time-bound, risk-accepted exception exists. |
| `blocked` | A dependency prevents progress or use. |

`verified` is the minimum status for product evidence. `approved` is the minimum status for release evidence in regulated-client contexts.

## Gates

These gates are mandatory before regulated-grade wording can advance:

1. **Baseline gate**: Nomos CLI and CUE validations are green.
2. **Reference gate**: every external reference in docs/specs maps to the control matrix or an explicit non-applicability record.
3. **Read-only gate**: corpus scans and feeds prove they did not mutate protected source repositories.
4. **Evidence gate**: generated reports include ALCOA+ metadata, source hashes, artifact hashes, command, actor/tool, timestamp, repo, commit, and validation status.
5. **Self-compliance gate**: Nomos can evaluate Nomos with its own CLI and produce deterministic evidence.
6. **Release gate**: the release bundle lists every open critical deviation, waiver, Praxis evidence status, and public claim boundary.
7. **Praxis downstream gate**: Praxis remains `blocked` or `not_qualified` for regulated claims until Nomos producer artifacts are verified.

## Nomos First Implementation

The initial implementation wave is documentation and structure only:

- install the regulated folders and templates;
- create Nomos and Praxis product profiles;
- make the structure visible from the README;
- keep Nomos compliance status honest while CI and self-compliance are not green.

The next implementation wave must wire those templates into schemas and CLI gates:

- `nomos compliance references`;
- `nomos compliance self-check`;
- `nomos release bundle`;
- `nomos corpus feed --profile rbok-lawbook`;
- deterministic ALCOA+ report generation;
- Praxis-compatible evidence export.

## Non-Overclaim Boundary

The presence of this structure means:

```text
regulated_by_design_structure: installed
regulated_grade_claim: not yet allowed
current_nomos_level: NQ-0/NQ-1 boundary
current_praxis_relationship: downstream target, not yet qualified through Nomos
```

Public or customer-facing wording must continue to say "regulated-grade candidate" or "method/prototype under validation" until the gates above pass.

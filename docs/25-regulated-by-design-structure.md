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
  reference-basis/
    README.md
    external-reference-register.yaml
  product-profiles/
    nomos.yaml
    praxis.yaml
  quality-system/
    README.md
  lifecycle/
    README.md
  data-integrity/
    README.md
  security-privacy/
    README.md
  github-operating-model/
    README.md
  control-matrix/
    README.md
  evidence-index/
    README.md
    evidence-ledger.yaml
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
  controlled-document.yaml
  control-matrix.yaml
  deviation-capa-record.yaml
  intended-use.yaml
  regulated-product-profile.yaml
  traceability-matrix.yaml
  training-record.yaml
  validation-plan.md
  validation-protocol.yaml
  validation-summary-report.yaml
  release-evidence-bundle.yaml
  alcoa-evidence-envelope.yaml
  ai-rag-governance.md
  atomization-certification-report.yaml
  supplier-assurance-pack.md
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
| `blocked` | A named input prevents the specific regulated use or claim. Unrelated product/DevOps progress continues (ADR-VRC-0004). |

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

The initial documentation-and-structure wave is installed:

- regulated folders and templates exist;
- Nomos and Praxis product profiles exist;
- the structure is linked from the public docs;
- public claims are bounded for `v0.1.0-ALPHA`.

The current alpha wave has moved beyond structure into operational evidence:

- CLI and corpus commands are implemented;
- RBOK lawbook feed and strict fidelity proof are available;
- regulated documentation and evidence-pack automation exist;
- release docs distinguish implemented evidence from blocked regulated claims.

The next closure wave must harden:

- named quality/security/technical ownership;
- CODEOWNERS and approval evidence;
- GitHub live QMS evidence export;
- licensed-reference review and sidecars;
- reference-to-control closure;
- release evidence bundle reconstruction.

## Non-Overclaim Boundary

The presence of this structure means:

```text
regulated_by_design_structure: installed
regulated_grade_claim: not allowed
current_nomos_level: NQ-2 alpha
current_praxis_relationship: downstream target, not yet qualified through Nomos
```

Public or customer-facing wording must continue to say "alpha regulated-readiness baseline", "regulated-by-design architecture under validation", or equivalent bounded language until the higher-level gates pass.

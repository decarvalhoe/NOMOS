# Training And Competence SOP

document_id: NOMOS-QMS-SOP-002
version: 0.1.0
status: draft
effective_status: not_effective
owner: "decarvalhoe (dev@realisons.com) — quality_owner, per NOMOS-REC-ROLE-2026-001"
approver: not_assigned

## Purpose

Ensure people who create, review, approve, maintain or use regulated Nomos evidence are trained and competent for their assigned tasks.

## Required Training Matrix

| Role | Required training | Evidence status |
|---|---|---|
| Quality owner | QMS, document control, CAPA, management review. | requires_evidence |
| Validation owner | CSA/validation, traceability, protocols, deviations, VSR. | requires_evidence |
| Security owner | SSDF, access, audit, vulnerability, incident, supply-chain controls. | requires_evidence |
| Data integrity owner | ALCOA+, audit trail, records, retention, true-copy exports. | requires_evidence |
| Developer | SDLC, secure coding, tests, PR controls, evidence envelope. | requires_evidence |
| AI/RAG reviewer | source citation, hallucination risks, refusal rules, prompt-injection controls. | requires_evidence |

## Conduct

Training records must include:

- trainee identity;
- role;
- document or module;
- version trained on;
- completion date;
- trainer or self-study method;
- effectiveness check when required;
- record hash or controlled storage reference.

No person may approve regulated release evidence without role training evidence or an approved waiver.

## Computed Status (VRC-16, #562)

The `Evidence status` column above is **computed, not typed**.
`scripts/training_competence_gate.py` reads the signed attestations in
`docs/regulated/operations/training-records/attestations/` and fails CI if this
table, `training-matrix.yaml`, or `CTL-QS-004` in the control matrix publishes
anything the records do not support — in either direction.

Three things follow from that:

- **Absence is never a pass.** A role with no attestation is
  `requires_evidence`. A role whose required training was never defined is
  `requires_definition`. Neither can become `established` by editing a table.
- **One uncovered competence keeps the whole role short.** Partial coverage is
  not competence.
- **A self-assessment is refused** unless a dated waiver in
  `independence-waiver.yaml` names that exact record and states its
  compensating controls. The assessment template requires assessor ≠ assessee,
  and a single operator cannot satisfy it; the waiver makes the exception
  visible instead of implicit.

The role vocabularies of this SOP, of the training matrix, and of the role
assignment record are reconciled by
`docs/regulated/operations/training-records/role-crosswalk.yaml`. Before VRC-16
they shared exactly one role, and four roles held by a named human had no matrix
entry at all.

**Computed 2026-09-04:** 0 valid attestations, 0 of 6 held roles `established`,
`product_owner` still `requires_definition`. The effectiveness condition on
training records is therefore not lifted, and this SOP stays `not_effective`.

## What The Gate Does Not Do (#640)

The gate validates a signature **declaration** and the evidence it references.
It does **not** authenticate a human identity, and no tool in this repository
does.

`signed_by_assessor: true` is an assertion by whoever wrote the file. The gate
checks that the assertion is present, dated, and carries `signature_evidence`
naming where the signature actually lives — a countersigned document, a commit,
an archived record. A declared signature pointing at nothing is refused, because
that is a claim rather than a record. Believing the assertion, and verifying the
person behind it, remains a human act.

Template and gate share one versioned schema
(`nomos-competence-assessment-v1`). An attestation in an unknown version is
refused rather than reinterpreted: silently reading a pre-migration record as if
it were the current shape would turn a migration slip into a wrong competence
status. The migration steps are in the template's own header.

A synthetic worked example lives in `examples/`, deliberately outside
`attestations/`, which is the only directory the gate reads. Copying it into
`attestations/` and changing the names would forge a quality record.

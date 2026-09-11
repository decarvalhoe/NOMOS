# Competence Attestations

One file per person per competence, named `<record_id>.yaml`, shaped like
`../competence-assessment-template.yaml`.

**This directory is empty, and that is the honest state.** Zero competence
attestations exist for the six roles currently held. `scripts/training_competence_gate.py`
computes `requires_evidence` for every one of them and the CI gate keeps every
published status saying so.

## These records cannot be generated

A competence attestation states that a named human was assessed and found
competent. Only the people involved can make that statement. No tool, and no
assistant, may author one on their behalf — a generated attestation is a forged
quality record, and it would be indistinguishable from a real one to every
downstream reader.

The gate reads what people signed. It never fills a gap.

## What the gate requires of each record

| Field | Rule |
|---|---|
| `record_id` | Present and unique across the directory. |
| `assessee.name` | Must hold an assigned role in the role assignment record. |
| `assessor.name` | Present. |
| `competence.id` | Must resolve to a competence in `../training-matrix.yaml`. |
| `assessment.date` | `YYYY-MM-DD`. |
| `assessment.result` | `pass`. Only a pass is evidence of competence. |
| `decision.competent` | `true`. |
| `decision.signed_by_assessor` / `signed_by_assessee` | Both `true`. |
| `decision.signed_at` | Present. |
| `approval.approved_by` / `approved_at` | Present. |
| `validity.expires_at` | If set, must not be in the past. |

A role reaches `established` only when **every** required competence of its
mapped matrix role carries a valid, unexpired record for the assignee. One
missing competence keeps the whole role at `requires_evidence`.

## Independence

`../competence-assessment-template.yaml` requires the assessor to differ from
the assessee. NOMOS is operated by one person holding every role, so that rule
cannot be satisfied as written today.

The gate does not ignore it. A self-assessed record is **refused** unless
`../independence-waiver.yaml` names that exact `record_id` and carries a
waiver date, an approver, and compensating controls — the same shape the role
assignment record used for the vacant independent reviewer. An unrecorded
self-assessment is an implicit independence claim that is not true, and it
turns the gate red.

Deciding whether to grant such a waiver, and what compensating controls justify
it, is a quality decision for the quality owner. It is not a tooling question.

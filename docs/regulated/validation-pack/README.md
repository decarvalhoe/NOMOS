# Validation Pack

This folder is reserved for validation lifecycle documents.

The pack must be risk based. It should prove confidence for the intended use actually claimed, not generate generic documentation volume.

## Minimum Contents

- intended use and out-of-scope statement;
- regulated impact assessment;
- risk assessment;
- URS/SRS or equivalent requirements baseline;
- architecture and data-flow description;
- validation plan;
- test strategy and protocol;
- challenge cases;
- deviation log;
- validation summary;
- rollback and recovery procedure;
- approval record or explicit non-approval status.

Use `templates/regulated/intended-use.yaml` and `templates/regulated/validation-plan.md` as initial templates.

## Current Protocol Coverage

- `TP-NOMOS-001-self-compliance.yaml`: executed self-compliance protocol for VAL-013.
- `TP-NOMOS-002-012-high-critical-validation-protocols.yaml`: local execution protocol pack for the remaining high/critical validation entries VAL-003, VAL-004, VAL-007, VAL-008, VAL-012, VAL-014, VAL-015, VAL-016, VAL-019, VAL-020, and VAL-023.
- `approval-workflow.md` and `approval-workflow.yaml`: GitHub-native approval and signature workflow definition for validation package approval.
- `validation-approval-record.yaml`: pending approval record. It is intentionally not an approval certificate.

Protocol execution evidence is currently local and pending independent review/approval. This pack supports reconstruction of the evidence chain but does not by itself establish regulated-grade compliance.

## Approval Gate

`scripts/regulated_approval_gate.py` validates that the approval workflow includes quality owner, product owner, and technical owner roles, uses CODEOWNERS/protected PR review, requires immutable release evidence for effective approval, and blocks any `approved` status that lacks explicit evidence references.

The regulated documentation gate runs the approval gate. This keeps the dossier in `pending_approval` until named human owners produce review and signature evidence.

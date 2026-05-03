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

Protocol execution evidence is currently local and pending independent review/approval. This pack supports reconstruction of the evidence chain but does not by itself establish regulated-grade compliance.

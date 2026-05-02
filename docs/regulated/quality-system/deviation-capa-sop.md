# Deviation And CAPA SOP

document_id: NOMOS-QMS-SOP-004
version: 0.1.0
status: draft
effective_status: not_effective
owner: not_assigned
approver: not_assigned

## Purpose

Control deviations, nonconformities, corrective actions and preventive actions affecting Nomos regulated evidence.

## Trigger Events

- failed CI or validation gate;
- corpus mutation during read-only operation;
- missing source hash or artifact hash;
- orphan atom, matrix row or chunk;
- unsupported block dropped without finding;
- unapproved public compliance claim;
- audit-trail gap;
- security incident or vulnerability breach;
- customer finding;
- Praxis runtime evidence contradicts Nomos expected behavior.

## Procedure

1. Open a deviation record.
2. Classify severity and affected intended use.
3. Contain the issue.
4. Perform root-cause analysis.
5. Define corrective action.
6. Define preventive action when systemic.
7. Link implementation PRs, tests, evidence and release gates.
8. Verify effectiveness.
9. Close only with owner approval.

## Severity

| Severity | Meaning | Release impact |
|---|---|---|
| minor | No regulated claim or critical evidence impact. | release may proceed with rationale. |
| major | Evidence, validation, security or customer claim affected. | waiver or fix required. |
| critical | False compliance, source mutation, record corruption, security compromise. | release blocked. |

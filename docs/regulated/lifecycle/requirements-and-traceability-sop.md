# Requirements And Traceability SOP

document_id: NOMOS-SDLC-SOP-001
version: 0.1.0
status: draft
effective_status: not_effective
owner: not_assigned
approver: not_assigned

## Purpose

Ensure Nomos requirements are source-backed, testable, risk-ranked and linked to evidence.

## Traceability Chain

```text
external_reference
  -> control_id
  -> product_requirement
  -> design_or_config
  -> implementation
  -> verification
  -> evidence_artifact
  -> release_gate
```

## Requirement Quality Rules

Each requirement must have:

- stable ID;
- intended use;
- risk level;
- source or rationale;
- acceptance criteria;
- verification method;
- owner;
- status;
- evidence link or `requires_evidence`.

## Forbidden

- requirements without testability;
- matrix rows with fake evidence;
- claims linked only to narrative text;
- unresolved ambiguity hidden in implementation.

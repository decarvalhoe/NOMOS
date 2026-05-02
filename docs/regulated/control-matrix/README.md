# Regulated Control Matrix

This folder is reserved for the machine-readable and human-readable control matrix.

The matrix must map:

```text
external_reference
  -> control_id
  -> requirement
  -> intended_use
  -> risk
  -> implementation
  -> verification
  -> evidence
  -> release_gate
  -> claim_boundary
```

## Required Properties

- every cited regulation, framework, standard, or product claim is mapped;
- non-applicable items have a named rationale and reviewer;
- waivers have owner, expiry, risk, mitigation, and release impact;
- critical controls cannot close without generated evidence;
- public wording cannot exceed the weakest mapped evidence status.

Use `templates/regulated/control-matrix.yaml` as the starting structure.

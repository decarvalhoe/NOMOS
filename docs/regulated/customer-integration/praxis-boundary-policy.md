# Praxis Downstream Boundary Policy

> Document ID: RCP-016-PRAXIS-BOUNDARY
> Status: active
> Owner: not_assigned
> Effective: 2026-05-03

## Purpose

This policy defines the controlled boundary between Nomos (evidence
producer) and Praxis (evidence consumer). It prevents premature joint
claims and preserves independent verification until Praxis establishes
its own qualified gates.

## Boundary Definition

```
┌─────────────────────────┐         ┌─────────────────────────┐
│         NOMOS           │         │         PRAXIS          │
│                         │         │                         │
│  Produces:              │         │  Consumes:              │
│  - nomos-report.json    │────────►│  - nomos-report.json    │
│  - evidence artifacts   │         │  - evidence artifacts   │
│  - approval records     │         │  - approval records     │
│  - attestations         │         │  - attestations         │
│                         │         │                         │
│  Gates:                 │         │  Gates:                 │
│  - admission            │         │  - (not qualified)      │
│  - strict               │         │  - (not qualified)      │
│  - approval workflow    │         │  - (not qualified)      │
│                         │         │                         │
│  Claims:                │         │  Claims:                │
│  - Nomos-scoped only    │         │  - NONE until qualified │
└─────────────────────────┘         └─────────────────────────┘
```

## Rules

### Rule 1: No Joint Claims

Nomos and Praxis shall not make joint compliance claims. Each system
must independently qualify its own gates before asserting regulated
compliance.

- Nomos may claim: "Evidence produced and gates passed within Nomos scope."
- Praxis may NOT claim: anything regulated until its own gates are qualified.
- No document shall state or imply that Praxis usage inherits Nomos
  qualification status.

### Rule 2: Nomos Produces, Praxis Consumes

The data flow is unidirectional at the boundary:

| Direction | Artifact | Format | Contract |
|-----------|----------|--------|----------|
| Nomos → Praxis | nomos-report.json | JSON (nomos-report schema v0.1.0) | Read-only consumption |
| Nomos → Praxis | Evidence artifacts | Files + manifest | Immutable after production |
| Nomos → Praxis | Approval records | YAML/JSON | Verified chain before acceptance |
| Nomos → Praxis | Attestations | In-toto / SLSA | Signature verified on receipt |

Praxis shall NOT write back to Nomos artifacts, modify Nomos reports,
or inject data into the Nomos approval chain.

### Rule 3: Praxis Gates Are Not Qualified

Until Praxis independently completes:

1. Its own control matrix with named owners
2. Its own validation master plan
3. Its own test evidence collection
4. Its own approval workflow (separate from Nomos)
5. Independent review by quality_unit

...all Praxis processing of Nomos artifacts is classified as
**informational only** and carries no regulated weight.

### Rule 4: Evidence Handoff Protocol

For **authoritative regulated** downstream consumption:

1. Evidence is finalized (approval record signed, chain valid)
2. Evidence is exported as immutable artifact (hash-locked)
3. Praxis receives via defined interface (CI artifact, API, or Git)
4. Praxis verifies artifact hash matches Nomos attestation
5. Praxis stores as external input (not owned, not modifiable)

Technical integration does not wait for that handoff. Schema checks,
import/reject fixtures and non-regulated dry runs may consume synthetic or
pending Nomos artifacts when they are explicitly marked
`not_qualified_external_input`; they carry no regulated weight and cannot be
used for release, CAPA closure, audit response or customer validation.

### Rule 5: No Praxis Code in Nomos Repository

The Nomos repository shall not contain:

- Praxis application code
- Praxis runtime configuration
- Praxis-specific test fixtures (unless testing the boundary interface)
- Praxis credentials or environment references

Integration tests that verify the boundary contract are permitted in
Nomos under `cli/internal/compliance/` or dedicated integration test
directories.

## Violation Handling

| Violation | Detection | Consequence |
|-----------|-----------|-------------|
| Joint claim in documentation | Documentation gate review | Block merge until corrected |
| Praxis writing to Nomos artifacts | Git diff / lockfile guard | Reject commit |
| Praxis code in Nomos repo | CI path check | Block PR |
| Unqualified Praxis gate referenced as evidence | Strict gate | Finding: boundary_violation |
| Nomos report modified after production | Hash verification | Reject artifact |

## Exceptions

Exceptions to this policy require:

1. Written justification with risk assessment
2. Approval by quality_unit
3. Time-bounded scope (maximum 90 days)
4. Documented in the exceptions register

No exception may override Rules 1 or 3 (no joint claims, no
qualification by inheritance).

## Revision History

| Version | Date | Change |
|---------|------|--------|
| 0.1.0 | 2026-05-03 | Initial boundary policy |

# Validation Master Plan — Nomos

## Document Control

| Field | Value |
|---|---|
| Document ID | VMP-NOMOS-001 |
| Version | 0.1.0-draft |
| Status | Draft — not effective |
| Owner | Quality Owner |
| Approved by | Pending via APPR-NOMOS-001 |
| Effective date | — (pending) |
| Review date | — (pending) |

## Claim Boundary

This validation master plan covers the Nomos CLI and its canonical-first
verification pipeline. It does NOT claim regulated-grade compliance.
Current quality level is NQ-2 alpha.

## Intended Use

Nomos is intended for:
- Verifying that domain knowledge flows from authoritative sources through
  versioned contracts to product surfaces with machine-checkable evidence.
- Producing structured reports, attestations, and SBOMs for audit trails.
- Blocking releases when canonical strict checks fail.

Nomos is NOT intended for:
- Making clinical, financial, or safety-critical decisions.
- Replacing human expert judgment in regulated domains.
- Runtime enforcement of business rules.

## Scope

### In Scope
- CLI commands: validate, strict, sources check, contracts check, product-check
- Manifest schema validation (CUE)
- Cross-manifest consistency checks
- Expiring exceptions enforcement
- Report and attestation generation
- Corpus scan and feed (read-only)

### Out of Scope
- LLM decision-making
- Runtime data processing
- User-facing product surfaces
- Third-party adapter implementations

## Risk-Based Approach

Validation effort is proportional to risk:

| Risk Level | Validation Depth |
|---|---|
| Critical | Full protocol with challenge cases, edge cases, regression |
| High | Protocol with representative cases and boundary conditions |
| Medium | Automated test coverage with spot checks |
| Low | Automated tests only |

## Validation Strategy

1. **Unit tests**: Each Go package has `_test.go` with coverage of happy path,
   error cases, and edge cases.
2. **Integration tests**: Self-compliance test verifies Nomos repo passes its
   own admission and regulated controls.
3. **Schema validation**: CUE schemas enforce structural correctness.
4. **Regression fixtures**: Testdata directories provide stable inputs.
5. **Gate verification**: Strict gate aggregates all blocking checks.

## Acceptance Criteria

- All Go packages pass `go test ./...`
- `go vet ./...` reports no issues
- Self-compliance evaluates to `compliant`
- No blocking findings in strict gate
- All regulated control artifacts present

## Deviation Management

Deviations from this plan require:
- A decision record in `docs/regulated/decisions/`
- Owner and expiry date
- Risk assessment
- Remediation plan

## Approval

| Role | Name | Date | Signature |
|---|---|---|---|
| Quality Owner | — | — | — |
| Product Owner | — | — | — |
| Technical Owner | — | — | — |

Approval is governed by `approval-workflow.yaml` and remains pending until the
required human owner evidence is recorded. The presence of this table is not an
approval claim.

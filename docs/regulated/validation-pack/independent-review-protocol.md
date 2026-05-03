# Independent Reconstruction Review Protocol

Document ID: IRP-NOMOS-001
Status: effective
Owner: quality-owner

## Purpose

This protocol enables an independent reviewer — someone who did not author the evidence — to reconstruct every release claim from the retained evidence chain. If any claim cannot be independently traced from its source through implementation to verification evidence, it fails the review.

This protocol implements Gate 6 (Release gate) from docs/25-regulated-by-design-structure.md.

## Claim Boundary

This protocol covers Nomos v0.1 alpha claims only. It does not extend to Praxis or downstream consumers. Current quality level is NQ-2 alpha; the protocol must pass with retained evidence before any NQ-6 or independent-audit-ready claim.

## Reviewer Qualifications

The independent reviewer must:

- not have authored the evidence under review;
- understand the intended-use model (IU-NOMOS-001);
- have access to the repository at the exact commit under review;
- be able to execute `go test` and `cue vet` commands;
- document findings with severity and remediation.

## Evidence Chain Structure

Each claim must have a complete chain:

```
Claim
  └── Validation Entry (VAL-NNN in validation-inventory.yaml)
        ├── Intended Use Reference (IU-FUNC-NNN if applicable)
        ├── Acceptance Gate (CI workflow)
        ├── Verification Command (reproducible)
        ├── Evidence Artifact (test output, report, attestation)
        └── Test Protocol (TP-NOMOS-NNN if risk >= high)
              ├── Test Cases (TC-NNN)
              ├── Execution Record (timestamp, actor, output)
              ├── Deviations (DEV-NNN if any)
              └── Approval Signatures
```

## Review Steps

### Step 1: Verify Evidence Inventory Completeness

Reconstruct the validation inventory from the repository:

```bash
cd cli && go test -v ./internal/compliance/... -run TestCheckCompleteness_RealInventoryFile
```

Verify:
- All intended-use functions (IU-FUNC-001..006) have validation entries.
- No validation entry has missing required fields.
- No duplicate validation IDs.

### Step 2: Reproduce Evidence Artifacts

For each validation entry in `validation-inventory.yaml`, execute the verification command and confirm it passes:

```bash
# Example for VAL-001:
cd cli && go test -v ./internal/validate/...

# Example for VAL-013 (self-compliance):
cd cli && go test -v ./internal/compliance/... -run SelfCompliance
```

Record the commit hash, timestamp, and exit code for each execution.

### Step 3: Verify Evidence Ledger

Check that the evidence ledger is consistent with actual artifacts:

```bash
cd cli && go test -v ./internal/compliance/... -run Ledger
```

Verify:
- Every `present_draft` or `effective` entry points to an existing file.
- Blocking gaps have documented claims they block.
- No evidence entry has an invalid status.

### Step 4: Verify Self-Compliance

Run the self-compliance gate and confirm Nomos passes its own evaluation:

```bash
cd cli && go test -v ./internal/compliance/... -run TestSelfComplianceOnNomosRepo
```

Expected output: `verdict=compliant controls=N satisfied=N findings=0 blocking=0`

### Step 5: Verify Release Bundle

Check that the release bundle contains all required artifacts for the target quality level:

```bash
cd cli && go test -v ./internal/compliance/... -run ReleaseBundle
```

### Step 6: Verify Read-Only Guards

Confirm that evidence generation did not modify any source repository:

```bash
cd cli && go test -v ./internal/guard/...
```

### Step 7: Verify Claims Governance

Confirm that no forbidden claims exist in public documentation:

```bash
cd cli && go test -v ./internal/compliance/... -run ForbiddenClaim
```

### Step 8: Run Automated Reconstruction Check

Execute the automated reconstruction review:

```bash
cd cli && go test -v ./internal/compliance/... -run Reconstruction
```

This verifies programmatically that every claim in the validation inventory has a complete evidence chain.

## Findings Template

For each finding, record:

| Field | Value |
|-------|-------|
| Finding ID | IRF-NNN |
| Validation Ref | VAL-NNN |
| Severity | low / medium / high / critical |
| Description | What is missing or broken |
| Evidence gap | Which chain link is missing |
| Remediation | What must be done to close |
| Blocking | yes / no |

## Verdict Criteria

| Verdict | Condition |
|---------|-----------|
| **Passed** | All 8 steps pass, zero blocking findings |
| **Passed with observations** | All steps pass, only non-blocking findings |
| **Failed** | Any step fails, or any blocking finding exists |

## Approval

| Role | Name | Date | Signature |
|------|------|------|-----------|
| Independent Reviewer | — | — | — |
| Quality Owner | — | — | — |
| Product Owner | — | — | — |

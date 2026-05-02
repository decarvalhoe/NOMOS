# Nomos Self-Compliance Report

> Generated: 2026-05-02
> Scope: Dogfood Nomos pipeline on the Nomos repository itself
> Related: #137, #138 (regulated scope)

## Pipeline Steps Executed

| Step | Tool | Result |
|------|------|--------|
| Validate manifests | nomos.project.yaml schema check | PASS |
| Check source manifest | docs/canonical/source-manifest.yaml exists | PASS |
| Check canonical matrix | docs/canonical/canonical-matrix.yaml exists | PASS |
| Check control matrix | docs/regulated/control-matrix/nomos-control-matrix.yaml | WARN |
| Check reference registry | docs/regulated/reference-basis/external-reference-register.yaml | WARN |
| Check CI workflows | .github/workflows/ci.yml + 5 regulated workflows | PASS |
| Run strict gate | All controls not_qualified | BLOCKED |

## Overall Verdict

**BLOCKED** — The repository has the structural prerequisites but all 19
controls are `not_qualified` and all external references remain
`requires_evidence`. No regulated claim can advance until findings are
resolved.

## Findings

### F-001: All control matrix owners are not_assigned

- **Severity:** high
- **Code:** governance_owner_missing
- **Location:** docs/regulated/control-matrix/nomos-control-matrix.yaml
- **Detail:** 19/19 controls have `owner: not_assigned`. Regulated controls
  require a named owner before verification can proceed.
- **Remediation:** Assign each control to a named team or individual with
  authority to verify evidence and approve the control.

### F-002: All control matrix statuses are not_qualified

- **Severity:** high
- **Code:** control_not_qualified
- **Location:** docs/regulated/control-matrix/nomos-control-matrix.yaml
- **Detail:** 19/19 controls remain at `current_status: not_qualified`.
  No control has been verified, tested, or approved.
- **Remediation:** For each control, execute the `verification_ref` command,
  collect evidence artifact, and transition status to `qualified` or
  `verified` upon review.

### F-003: Matrix-level owner is not_assigned

- **Severity:** medium
- **Code:** governance_owner_missing
- **Location:** docs/regulated/control-matrix/nomos-control-matrix.yaml (matrix.owner)
- **Detail:** The control matrix itself has `owner: not_assigned`.
- **Remediation:** Assign a quality system owner for the control matrix.

### F-004: Matrix status is not_qualified

- **Severity:** medium
- **Code:** matrix_status_draft
- **Location:** docs/regulated/control-matrix/nomos-control-matrix.yaml (matrix.status)
- **Detail:** Matrix status is `not_qualified` — no overall qualification claim.
- **Remediation:** Progress to `qualified` only after all critical controls
  reach verified status.

### F-005: External references lack collected evidence

- **Severity:** high
- **Code:** ref_requires_evidence
- **Location:** docs/regulated/reference-basis/external-reference-register.yaml
- **Detail:** 12 references have `evidence_status: requires_evidence`.
  Evidence has not been collected, processed, or attested for these references.
- **Remediation:** For each reference, snapshot the official source, hash it,
  atomize with Nomos, and retain generated evidence per nomos-bible-corpus-policy.

### F-006: Licensed references not yet clause-mapped

- **Severity:** medium
- **Code:** ref_licensed_pending
- **Location:** docs/regulated/reference-basis/external-reference-register.yaml
- **Detail:** 2 references (ISO-13485-2016, ISO-IEC-IEEE-12207-2026) are at
  `summary_reference_only_until_licensed_clause_mapping`. 2 references
  (ISO-IEC-25010-2023, ISPE-GAMP5-2E-2022) require license review before
  clause mapping.
- **Remediation:** Acquire licensed copies, create sidecar intake YAML with
  SHA-256, process read-only under nomos-bible-corpus-policy.

### F-007: GitHub live configuration evidence not collected

- **Severity:** medium
- **Code:** ref_requires_live_evidence
- **Location:** docs/regulated/reference-basis/external-reference-register.yaml
- **Detail:** 5 references require `requires_live_github_evidence` (branch
  protection, rulesets, code scanning, secret scanning, environments).
  No evidence snapshots have been collected.
- **Remediation:** Export GitHub API configuration state, hash the export,
  and store as evidence artifacts in the evidence index.

### F-008: CODEOWNERS has no active owners

- **Severity:** medium
- **Code:** governance_owner_missing
- **Location:** .github/CODEOWNERS
- **Detail:** CODEOWNERS file is a placeholder — all rules are commented out.
  No real team or user is assigned.
- **Remediation:** Assign real GitHub teams/users once organizational accounts
  are established.

### F-009: Project manifest declares regulated: false

- **Severity:** low
- **Code:** scope_regulated_mismatch
- **Location:** nomos.project.yaml (compliance.regulated)
- **Detail:** The project manifest declares `regulated: false` but the
  repository carries a full regulated document set (21 regulated SOPs,
  control matrix, reference register, validation master plan).
- **Remediation:** Update `compliance.regulated: true` and add
  `data_sensitivity: internal` once the team decides to activate
  regulated claims formally.

### F-010: No CHANGELOG.md

- **Severity:** low
- **Code:** documentation_missing
- **Location:** (root)
- **Detail:** No CHANGELOG.md exists for tracking release history.
- **Remediation:** Add CHANGELOG.md following Keep a Changelog format
  to support release documentation requirements.

### F-011: Coverage report not generated

- **Severity:** low
- **Code:** evidence_missing
- **Location:** nomos.project.yaml (evidence.required_reports)
- **Detail:** `coverage-report.md` is listed as a required report but
  does not exist in the repository.
- **Remediation:** Add coverage report generation to CI (`go test -coverprofile`)
  and commit or upload as artifact.

## Summary

| Severity | Count |
|----------|-------|
| High     | 3     |
| Medium   | 5     |
| Low      | 3     |
| **Total** | **11** |

## Pass Criteria for Strict Gate

The strict gate will pass when:
1. At least one control reaches `verified` status with a named owner
2. The matrix owner is assigned
3. External reference evidence begins collection (at least 1 reference at `evidence_collected`)
4. `compliance.regulated` is set to `true` in nomos.project.yaml

## Next Actions

1. Assign control matrix owner and per-control owners
2. Run `nomos strict` on first control (CTL-DI-001) to produce verification evidence
3. Snapshot at least one public FDA reference and process through Nomos atomization
4. Update nomos.project.yaml `regulated: true` once team formally commits
5. Activate CODEOWNERS with real GitHub team assignments
6. Generate and publish coverage-report.md in CI

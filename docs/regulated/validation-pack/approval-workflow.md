# Nomos Validation Approval Workflow

## Document Control

| Field | Value |
|---|---|
| Document ID | APPR-NOMOS-001 |
| Version | 0.1.0 |
| Status | Draft - pending approval |
| Effective status | Not effective |
| Owner | Quality Owner |
| Approver | Pending: quality owner, product owner, technical owner |

## Claim Boundary

This workflow defines how Nomos validation evidence will be reviewed and
approved. It does not claim that GitHub is a validated eQMS, that GitHub is a
Part 11 electronic signature system, or that the Nomos validation package is
approved.

## Required Roles

| Role | Approval meaning |
|---|---|
| Quality owner | Confirms validation adequacy, independence, deviation handling, and regulated claim boundary. |
| Product owner | Confirms intended use, business risk acceptance, release readiness, and product claim boundary. |
| Technical owner | Confirms implementation evidence, reproducibility, CI evidence, provenance, and operational feasibility. |

## GitHub-Native Evidence Path

1. Every validation package change is made by pull request.
2. CODEOWNERS must request review for regulated files.
3. Protected branch rules must require pull request review before merge.
4. The validation approval record remains `pending_approval` until named human
   owners add explicit approval evidence.
5. Effective approval requires immutable release evidence: merged PR URL,
   commit SHA, evidence pack hash, approval artifact hash, and signed release
   tag or equivalent externally signed attestation.

## Overclaim Guard

Pending records may exist without signatures only when they explicitly state
`overclaim_guard: true`. A record marked `approved` must include immutable
evidence references, approver references, and an approval timestamp. The
regulated documentation gate blocks `approved` records that do not meet that
standard.


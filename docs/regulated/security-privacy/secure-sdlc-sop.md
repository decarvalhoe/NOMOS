# Secure SDLC SOP

document_id: NOMOS-SEC-SOP-001
version: 0.1.0
status: draft
effective_status: not_effective
owner: not_assigned
approver: not_assigned

## Purpose

Align Nomos development with secure software development and regulated release evidence expectations.

## Minimum Controls

- PR-only changes for protected branches;
- code review for critical changes;
- dependency inventory;
- vulnerability scanning;
- secrets scanning;
- least-privilege CI tokens;
- artifact hashing;
- build provenance;
- security-impact assessment for regulated features;
- vulnerability remediation SLA;
- release security sign-off or documented waiver.

## Evidence

Evidence must be linked in release bundles. Missing scans or provenance are `requires_evidence`, not assumed pass.

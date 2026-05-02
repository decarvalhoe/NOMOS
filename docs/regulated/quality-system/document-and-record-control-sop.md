# Document And Record Control SOP

document_id: NOMOS-QMS-SOP-001
version: 0.1.0
status: draft
effective_status: not_effective
owner: not_assigned
approver: not_assigned

## Purpose

Control documents and records so that Nomos can prove what was approved, when, by whom, for which intended use, and with which source evidence.

## Procedure

1. Create controlled documents from approved templates when available.
2. Assign `document_id`, version, status, owner, reviewer, approver and source references.
3. Keep new documents in `draft` until reviewed.
4. Record review comments in PRs, issues, or controlled review records.
5. Promote to `effective` only when an approval record exists.
6. Preserve previous versions through git history and release bundles.
7. Mark obsolete documents as `superseded`, never delete evidence needed for reconstruction.
8. For records, preserve source hash, artifact hash, actor/tool, timestamp, command, repository, commit and retention category.

## Required Status Values

- `draft`
- `under_review`
- `approved`
- `effective`
- `superseded`
- `retired`
- `blocked`

## Missing Evidence Handling

Use:

- `not_assigned` for absent owners or approvers;
- `requires_evidence` for absent records;
- `not_effective` for unapproved procedures;
- `not_applicable` only with rationale, owner and review date.

## Release Gate

A release cannot claim regulated-grade readiness if critical controlled documents remain `draft` without an approved waiver.

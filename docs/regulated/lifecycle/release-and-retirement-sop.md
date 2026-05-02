# Release And Retirement SOP

document_id: NOMOS-SDLC-SOP-003
version: 0.1.0
status: draft
effective_status: not_effective
owner: not_assigned
approver: not_assigned

## Purpose

Control release, rollback, maintenance and retirement of Nomos versions used for regulated evidence.

## Release Gate

A release bundle must include:

- version and commit;
- intended use;
- open deviations;
- waivers;
- validation status;
- security status;
- data integrity status;
- AI/RAG status;
- corpus read-only evidence;
- release claim boundary;
- approval status.

## Retirement

Retirement must define:

- last supported version;
- affected customers;
- migration path;
- evidence retention;
- rollback availability;
- archived records and hashes.

No regulated evidence artifact may be deleted solely because a version is retired.

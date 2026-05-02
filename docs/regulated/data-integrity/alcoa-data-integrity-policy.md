# ALCOA+ Data Integrity Policy

document_id: NOMOS-DI-POL-001
version: 0.1.0
status: draft
effective_status: not_effective
owner: not_assigned
approver: not_assigned

## Purpose

Ensure Nomos evidence is attributable, legible, contemporaneous, original or true copy, accurate, complete, consistent, enduring and available.

## Required Evidence Envelope

Every generated regulated artifact must include or link to:

- actor;
- tool name and version;
- command;
- timestamp;
- repository URL;
- commit SHA;
- source IDs and hashes;
- artifact hash;
- schema version;
- validation status;
- exclusions and findings;
- retention category.

## Missing Metadata

If required metadata cannot be captured, the artifact status is `not_qualified` for regulated claims.

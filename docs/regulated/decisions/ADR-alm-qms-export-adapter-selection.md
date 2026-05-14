# ADR-ALM-QMS-020 - ALM/QMS Export Adapter Selection

## Status

Accepted

## Date

2026-05-14

## Decision ID

ADR-ALM-QMS-020

## Owner

RBOK Team (engineering-owner)

## Affected Products

- Nomos

## Affected Controls

- CTL-VAL-001 (Risk-Based Validation Strategy)
- CTL-DOC-001 (Quality Risk Management)
- CTL-CC-001 (Configuration And Change Control)

## Context

DOR-020 asks Nomos to choose the first ALM/QMS export target from GitHub
issues, ReqIF, JSON schema, CSV, Jira, or QMS import templates. The goal is
interoperability evidence, not a claim that Nomos replaces an ALM or QMS.

The existing ReqIF boundary decision remains valid: ReqIF is export-only and
must never mutate the canonical chain. DOR-020 narrows the first practical
adapter so the repository has a fixture-backed target before broader vendor
formats are implemented.

## Decision

First adapter: GitHub issues evidence export.

Nomos will first support a GitHub Issues export fixture for evidence-backed
work items. This adapter is a downstream export path from Nomos evidence into a
reviewable issue payload. It is not an ALM/QMS replacement and does not make
GitHub a validated eQMS.

The selected adapter exports:

- stable Nomos canonical references;
- content hashes and source spans;
- evidence status and claim boundaries;
- blocked and deferred claim notes;
- reviewer-facing acceptance criteria.

The fixture is recorded at
`docs/regulated/domain-packs/alm-qms-export/github-issues-export-fixture.yaml`.

## Options Considered

### GitHub issues

Selected first. It matches the repository operating model and can be reviewed
without adding vendor XML or external SaaS configuration. It also keeps export
loss visible because issue fields are explicit and hash-bound.

### ReqIF

Deferred. ReqIF remains the intended regulated ALM interchange direction, but
the adapter needs vendor golden-file testing before it can be first supported.

### JSON schema

Deferred. JSON schema is useful as an intermediate contract, but by itself it
is not an ALM/QMS target. It should describe adapter payloads after the first
adapter fixture stabilizes.

### CSV

Deferred. CSV is useful for inspection, but it loses nested traceability unless
paired with sidecar hashes and field-level loss notes.

### Jira

Deferred. Jira requires tenant-specific field mappings and cannot be claimed
without a customer configuration fixture.

### QMS import templates

Blocked for first implementation. QMS import templates are too vendor-specific
and can imply direct QMS replacement if shipped without customer validation.

## Blocked adapters

- Bidirectional ReqIF sync: blocked because external tool edits must not mutate
  the Nomos canonical chain.
- QMS template import: blocked for first implementation because a template
  import can be mistaken for validated QMS operation.

## Deferred adapters

- ReqIF export: deferred until XML golden files and vendor import checks exist.
- Jira export: deferred until tenant field mapping and workflow status mapping
  are captured.
- CSV export: deferred until sidecar loss annotations are defined.
- JSON schema export: deferred until the GitHub issue payload contract is
  stable enough to derive schema from it.

## Evidence loss risks

- Rich text normalization can alter headings, tables, and lists when converted
  to issue markdown.
- Status semantics drift can occur when Nomos evidence states are mapped to
  tracker labels or workflow columns.
- Issue comments can add review context that is not part of the original Nomos
  artifact unless linked back through a controlled evidence record.
- Attachments can be separated from issue metadata unless hashes are retained.

## Non-Goals

- No ALM/QMS replacement claim.
- No bidirectional synchronization.
- No Jira, ReqIF, CSV, JSON schema, or QMS template implementation in this
  decision.
- No validated eQMS, regulated validation, certification, or legal compliance
  claim.

## Evidence Impact

The first adapter fixture gives reviewers a concrete target payload and makes
loss risks explicit. It does not create customer validation evidence or a
production connector.

## Risk Impact

Risk is medium. GitHub issue payloads are understandable and reviewable, but
they are less structured than ReqIF and cannot preserve all nested traceability
without sidecar metadata.

## Follow-Up Issues

- Add JSON schema for the GitHub issue export payload after the fixture is
  accepted.
- Add ReqIF export golden files for DOORS, Polarion, and Jama imports.
- Add CSV sidecar loss annotations before any CSV release.
- Add customer-specific Jira mapping fixtures before Jira export.

## Revision Triggers

Review this decision if a customer contract requires ReqIF first, if the GitHub
QMS operating model is replaced, or if a validated downstream QMS import path is
approved by the customer.

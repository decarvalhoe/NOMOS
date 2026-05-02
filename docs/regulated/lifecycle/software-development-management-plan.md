# Software Development Management Plan

document_id: NOMOS-SDLC-PLAN-001
version: 0.1.0
status: draft
effective_status: not_effective
owner: not_assigned
approver: not_assigned

## Purpose

Define the software lifecycle controls for Nomos.

## Lifecycle Scope

Nomos lifecycle phases:

1. intended use and risk classification;
2. source/reference registration;
3. requirements and traceability;
4. architecture/design;
5. implementation;
6. verification and validation;
7. release evidence;
8. operations and monitoring;
9. maintenance and change control;
10. retirement.

## Required Baselines

| Baseline | Required contents | Evidence status |
|---|---|---|
| Requirements baseline | URS/SRS/control requirements and acceptance criteria. | requires_evidence |
| Design baseline | architecture, data flows, trust boundaries, source/corpus model. | requires_evidence |
| Configuration baseline | tools, versions, CI workflows, protected branches, environments. | requires_evidence |
| Test baseline | unit, integration, E2E, validation protocols, challenge cases. | partially_established |
| Release baseline | release bundle, validation summary, deviations, approvals. | requires_evidence |

## Reviews

Critical changes require:

- PR review;
- test evidence;
- risk impact assessment;
- traceability update;
- release-bundle impact statement.

## Tailoring

Any lifecycle requirement marked non-applicable must have:

- rationale;
- owner;
- approval or waiver;
- expiry/review date;
- public claim impact.

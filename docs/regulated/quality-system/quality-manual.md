# Quality Manual

document_id: NOMOS-QMS-QM-001
version: 0.1.1
status: draft
effective_status: not_effective
owner: "decarvalhoe (dev@realisons.com) — record NOMOS-REC-ROLE-2026-001"
approver: not_assigned
public_claim_boundary: "Quality-system baseline draft only; no compliance certification claim."

## Scope

This manual defines the draft quality-management system for Nomos and the downstream relationship to Praxis.

Nomos scope:

- canonical source registration;
- corpus read-only controls;
- structure-aware atomization;
- canonical references and traceability matrix;
- corpus feed and evidence generation;
- ALCOA+ evidence envelopes;
- validation and release evidence bundles;
- AI/RAG governance for extraction and citation support.

Praxis scope is downstream runtime evidence and CAPA consumption. Praxis cannot compensate for missing Nomos source-to-evidence controls.

## Quality Policy

Nomos will not claim more compliance than it can prove.

Quality objectives:

- every regulated claim has an intended use and evidence level;
- every external reference maps to a control or explicit non-applicability record;
- every generated artifact has source hashes, artifact hashes, actor/tool, command, timestamp and reproducibility metadata;
- every critical corpus operation proves read-only behavior;
- every release bundle exposes open deviations and waivers.

## Process Map

```text
reference register
  -> control matrix
  -> intended use and risk assessment
  -> requirements and design baseline
  -> implementation and configuration control
  -> verification and validation
  -> ALCOA+ evidence envelope
  -> release bundle
  -> operations, audit, CAPA, periodic review
```

## Roles

| Role | Responsibility | Current status |
|---|---|---|
| Quality owner | Owns QMS procedures, approvals and management review. | assigned — record NOMOS-REC-ROLE-2026-001 (2026-06-11) |
| Product owner | Owns intended use, public claim boundary and prioritization. | assigned — record NOMOS-REC-ROLE-2026-001 (2026-06-11) |
| Validation owner | Owns validation plan, protocols, deviations and summary report. | assigned — record NOMOS-REC-ROLE-2026-001 (2026-06-11) |
| Security owner | Owns secure SDLC, vulnerabilities, access, audit and supply-chain controls. | assigned — record NOMOS-REC-ROLE-2026-001 (2026-06-11) |
| Data integrity owner | Owns ALCOA+, records, retention and audit-trail review. | assigned — record NOMOS-REC-ROLE-2026-001 (2026-06-11) |
| Independent reviewer | Reviews critical controls and release evidence. | waived at alpha — recorded waiver in NOMOS-REC-ROLE-2026-001; re-evaluated at every management review |

All non-vacant roles are held by a single operator; the conflict-of-interest
note and compensating controls are recorded in
[`../operations/records/2026-06-11-role-assignment-record.yaml`](../operations/records/2026-06-11-role-assignment-record.yaml).

## Required Evidence Before Effectiveness

- approved document-control procedure;
- training matrix and training records — **open** (VRC-16 #562);
- control matrix populated with evidence links;
- validation master plan approved;
- release bundle generated from CI — **open** (VRC-15 #561);
- management review record — **recorded** (`NOMOS-REC-MR-2026-001`, 2026-06-11);
- independent review or documented waiver — **waiver recorded** (`NOMOS-REC-ROLE-2026-001`).

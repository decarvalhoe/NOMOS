# Nomos-Praxis Shared Evidence Contract

## Purpose

This contract defines the shared schema for evidence records exchanged
between Nomos (canonical verification) and Praxis (product execution).
Both systems produce and consume evidence. This contract ensures
interoperability and ALCOA+ data integrity across the boundary.

## Claim Boundary

This document defines the evidence contract specification only. It does
not certify that any specific evidence record is compliant or that either
system has been validated. Evidence validity requires execution of the
validation master plan.

## Schema Location

```
specs/evidence-contract.cue
```

## Types

### EvidenceRecord

A single piece of verifiable evidence with full ALCOA+ traceability.

| Field | Type | Required | ALCOA+ |
|---|---|---|---|
| `record_id` | `EV-*` pattern | yes | — |
| `category` | EvidenceCategory enum | yes | — |
| `title` | string | yes | legible |
| `description` | string | yes | legible |
| `producer` | EvidenceProducer enum | yes | attributable |
| `owner` | string | yes | attributable |
| `created_at` | ISO date | yes | contemporaneous |
| `source_hash` | `algo:hex` | yes | original, enduring |
| `content_type` | string | yes | legible |
| `location.type` | file/url/inline/external | yes | available |
| `location.path` | string | yes | available |
| `status` | EvidenceStatus enum | yes | complete |
| `claim_allowed` | string | yes | consistent |
| `review.status` | pending/reviewed/approved/rejected | yes | — |
| `alcoa_assessment` | ALCOAAttributeAssessment[] | no | all |
| `references` | EvidenceReference[] | no | — |

### EvidenceLedger

Master index of all evidence for a product.

| Field | Type | Required |
|---|---|---|
| `product_id` | lowercase slug | yes |
| `categories` | EvidenceLedgerCategory[] | yes |
| `blocking_gaps` | EvidenceGap[] | no |
| `claim_boundary` | string | yes |

### EvidenceCategory (enum)

| Value | Description |
|---|---|
| `external_reference_register` | External reference citations |
| `control_matrix` | Regulatory control mapping |
| `quality_system_document` | QMS policies and procedures |
| `lifecycle_document` | SDLC and change management |
| `validation_evidence` | Validation plans and protocols |
| `test_result` | Automated or manual test results |
| `data_integrity_record` | Data integrity policies |
| `security_evidence` | Security and privacy controls |
| `ai_governance_evidence` | AI/RAG governance records |
| `release_evidence` | Release bundle attestations |
| `corpus_attestation` | Corpus scan attestations |
| `source_manifest` | Source registry manifests |
| `canonical_matrix` | Canonical traceability matrix |
| `decision_record` | ADRs and decision records |
| `training_record` | Training and competency records |
| `audit_record` | Internal or external audit records |
| `deviation_record` | CAPA and deviation records |
| `supplier_record` | Supplier assurance records |

## ALCOA+ Mapping

Each evidence record is designed for ALCOA+ data integrity:

| Attribute | How Enforced |
|---|---|
| **A**ttributable | `producer`, `owner`, `review.reviewer` |
| **L**egible | `title`, `description`, `content_type` |
| **C**ontemporaneous | `created_at`, `updated_at` |
| **O**riginal | `source_hash` (integrity seal) |
| **A**ccurate | `claim_allowed`, `claim_boundary` |
| **C**omplete | `status` lifecycle tracking |
| **C**onsistent | `claim_allowed` must match `status` |
| **E**nduring | `source_hash`, `location.type: file` |
| **A**vailable | `location.path`, `review.status` |

## Exchange Protocol

### Nomos → Praxis

Nomos produces evidence records for:
- Source manifests validated
- Canonical matrix checks passed
- Strict gate results
- Corpus attestations
- Report generation runs

### Praxis → Nomos

Praxis produces evidence records for:
- Test execution results
- Deployment attestations
- Runtime compliance checks
- User acceptance records

### Handoff

1. Producer creates `EvidenceRecord` with `status: generated`
2. Consumer receives and sets `review.status: pending`
3. Reviewer assesses ALCOA+ attributes (optional formal assessment)
4. Record advances to `review.status: approved` or `rejected`
5. Both systems reference the same `record_id`

## Validation

```bash
# CUE schema validation
cue vet specs/evidence-contract.cue

# Go evidence contract checker
go test ./internal/compliance/... -run Evidence
```

# 27 - AAA+ Regulated IT Document Set

Date: 2026-05-02

## Purpose

This document defines the complete controlled document set Nomos needs before it can defend a high-regulated IT posture.

`AAA+` is an internal ambition label. It is not an external certification. It means Nomos aims to be inspectable by demanding regulated IT customers in pharma, medical device, aerospace, finance, critical infrastructure, legal/regulatory knowledge systems, and other high-integrity domains.

The rule is strict:

```text
If evidence is missing, record the gap.
Do not invent the evidence.
Do not imply approval.
Do not make a compliance claim.
```

## Source Basis

The baseline is built from current official or authoritative references available on 2026-05-02:

- FDA 21 CFR Part 11 closed-system controls.
- FDA 21 CFR Part 820 Quality Management System Regulation and ISO 13485 incorporation.
- FDA Computer Software Assurance guidance, September 2025.
- FDA General Principles of Software Validation, January 2002.
- FDA Data Integrity and Compliance With Drug CGMP, December 2018.
- EU EudraLex Volume 4, including Chapter 4 Documentation, Annex 11 Computerised Systems, Annex 15 Qualification and Validation, and pharmaceutical quality-system references.
- ISO 13485:2016, confirmed current in 2025.
- ISO/IEC/IEEE 12207:2026, published in April 2026.
- ISO/IEC 25010:2023.
- NASA NPR 7150.2D and NASA software assurance resources.
- NIST SP 800-218 SSDF.
- NIST SP 800-53 Rev. 5.
- NIST AI RMF 1.0 and Generative AI Profile.
- ISPE GAMP 5, Second Edition, as industry guidance.
- ICH Q9(R1) Quality Risk Management and ICH Q10 Pharmaceutical Quality System.
- W3C PROV-O and Web Annotation for provenance and source anchoring.
- SLSA/in-toto for supply-chain attestations.

The machine-readable register is `docs/regulated/reference-basis/external-reference-register.yaml`.

## Controlled Generation Rule

All documents generated from this baseline must carry:

- document ID;
- title;
- product scope;
- intended use;
- owner or `not_assigned`;
- reviewer or `not_assigned`;
- approval status;
- version;
- effective date or `not_effective`;
- source references;
- open evidence gaps;
- non-applicability decisions with rationale;
- next review date or `not_scheduled`;
- public claim boundary.

Forbidden patterns:

- fake approver names;
- fake approvals;
- fake test evidence;
- fake training records;
- fake audit outcomes;
- hidden assumptions;
- "compliant" wording without evidence and approval.

## AAA+ Document Architecture

```text
reference basis
  -> quality system
  -> product intended use
  -> software lifecycle
  -> requirements and traceability
  -> validation and CSA
  -> data integrity and records
  -> security/privacy/supply chain
  -> AI/RAG governance
  -> supplier/customer integration
  -> operations/CAPA/periodic review
  -> release evidence bundle
```

## Required Document Families

| Family | Required documents | Current repo status |
|---|---|---|
| Reference basis | External reference register, applicability matrix, controlled interpretation records. | Added in `docs/regulated/reference-basis/`. |
| Quality system | Quality manual, document control, record control, training, quality risk management, deviation/CAPA, internal audit, management review. | Added in `docs/regulated/quality-system/`. |
| Product scope | Product profiles, intended use, public claim boundary, customer responsibility matrix. | Existing, needs evidence completion. |
| Software lifecycle | SDMP, requirements management, architecture/design control, configuration management, change control, release/retirement. | Added in `docs/regulated/lifecycle/`. |
| Validation/CSA | Validation master plan, risk assessment, URS/SRS, traceability matrix, test protocols, deviation log, validation summary report. | Existing folder extended by lifecycle baseline and templates. |
| Data integrity | ALCOA+ policy, electronic records/signatures assessment, audit trail, retention, backup/restore, true-copy/export controls. | Added in `docs/regulated/data-integrity/`. |
| Security/privacy | Secure SDLC, access control, audit logging, vulnerability management, incident response, privacy controls, supply-chain provenance. | Added in `docs/regulated/security-privacy/`. |
| AI/RAG | AI intended use, model/tool inventory, prompt-injection controls, retrieval evals, citation/refusal tests, human review. | Existing folder remains control owner. |
| Supplier/customer | Supplier qualification, outsourced activities, customer validation support, shared responsibility matrix. | Existing folders remain control owners. |
| Operations | Incident/problem/CAPA, audit-trail review, periodic review, BCDR, backup/restore, service review. | Existing folder extended by conduct SOPs. |
| Evidence | Evidence ledger, ALCOA+ envelope, release bundle, validation report, audit package. | Added in `docs/regulated/evidence-index/`. |
| GitHub QMS automation | Issue forms, PR template, CODEOWNERS, documentation gate, evidence pack, GitHub settings audit. | Added in `.github/`, `scripts/regulated_*.py`, `tests/test_regulated_automation.py`. |

## Conduct Rules

### DR-01 Controlled Documentation

Every controlled document starts as `draft`. It becomes `effective` only after review and approval. If Nomos lacks an approval workflow, status stays `draft` or `not_effective`.

### DR-02 Evidence First

Evidence fields can only reference existing files, CI runs, signed/hashed artifacts, issue/PR records, or explicit human review records. Unknown evidence is `requires_evidence`.

### DR-03 Traceable Interpretation

External references are not decorative. Each reference maps to:

```text
external reference -> Nomos interpretation -> control -> required document -> evidence -> release gate
```

### DR-04 Risk-Based Rigor

Nomos follows a risk-based CSA/validation model:

- critical source-to-law, release, evidence, audit-trail, access-control, and data-integrity features require scripted evidence and review;
- lower-risk documentation or advisory features may use lighter evidence if the rationale is recorded;
- LLM/RAG output cannot become authority without source citation and review.

### DR-05 No Silent Non-Applicability

If a requirement is not applicable, the record must explain:

- why it is not applicable;
- who accepted that rationale;
- when it must be reviewed again;
- which public claims are blocked by that decision.

### DR-06 Automate Before Claiming

Where a repeated regulated activity can be checked mechanically, it must have a gate or report before the claim advances:

- document metadata and overclaim checks: `scripts/regulated_docs_gate.py`;
- evidence hash inventory: `scripts/regulated_evidence_pack.py`;
- GitHub operating control audit: `scripts/regulated_github_qms_audit.py`;
- CI execution and artifact upload: `.github/workflows/regulated-documentation-gate.yml` and `.github/workflows/regulated-evidence-pack.yml`.

Automation failures create gaps or blocking findings. Automation success creates evidence for the checked scope only; it does not create owner assignment, training, independent audit, Part 11 signature validation, or customer acceptance.

## Document Set Gaps

The following gaps are intentionally visible:

| Gap | Required resolution |
|---|---|
| Named quality owner not established. | Assign accountable owner before any regulated-grade release claim. |
| Formal approval workflow not implemented. | Define approval record semantics before `approved` status is used. |
| Training records absent. | Create role matrix and training records before claiming SOP effectiveness. |
| Independent audit absent. | Schedule independent review before `NQ-6`. |
| Part 11/e-signature capability not implemented. | Keep `part_11_claimed: false` until technical and procedural controls exist. |
| Full ISO/GAMP clause mapping incomplete. | Complete licensed-standard clause mapping with qualified reviewer when required by customer. |
| Nomos/Praxis evidence contract incomplete. | Keep Praxis regulated claim deferred until Nomos producer evidence is stable. |

## Release Claim Boundary

Until the evidence ledger is populated and release approvals exist, Nomos may claim only:

```text
regulated_documentation_baseline: draft
regulated_by_design_structure: installed
aaa_plus_target: defined
regulated_grade_claim: not_allowed
external_certification_claim: not_allowed
```

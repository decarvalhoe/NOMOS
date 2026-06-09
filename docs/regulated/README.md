# Regulated-By-Design Operating Structure

This directory is the governed operating area for Nomos regulated-readiness work, and for later Praxis compatibility.

It exists so regulated customers, quality reviewers, validation leads, security reviewers, and supplier-assurance teams can inspect the claim boundary before they inspect generated artifacts.

## Current Status At v0.1.0-ALPHA

Nomos has a regulated-readiness baseline. It does not yet have a completed regulated customer validation package or formal certification.

| Area | Status |
|---|---|
| Quality-system document skeleton | Installed |
| Lifecycle and validation document skeleton | Installed |
| Data integrity policies | Installed baseline |
| Security and privacy SOP baseline | Installed baseline |
| GitHub operating model | Installed baseline |
| Evidence index and contract | Installed baseline |
| Control matrix structure | Installed baseline |
| Validation-pack templates | Installed baseline |
| Supplier-pack structure | Installed baseline |
| AI/RAG governance baseline | Installed baseline |
| Atomization certification structure | Installed baseline |
| Executed customer validation package | Not complete |
| Approved QMS records | Not complete |
| Regulated certification | Not claimed |

## Execution Model

Nomos is the evidence producer. Praxis is a future downstream runtime assurance consumer.

The intended order is:

1. stabilize Nomos CLI, schemas, corpus feed, and self-compliance;
2. make Nomos evidence deterministic, traceable, and ALCOA+ aligned;
3. publish a stable evidence contract;
4. let Praxis consume verified Nomos artifacts for runtime scenarios, invariants, evidence retention, and CAPA support;
5. validate each customer deployment against intended use and risk.

## Directory Responsibilities

| Directory | Responsibility |
|---|---|
| `product-profiles/` | Product role, assurance level, public-claim boundary, critical dependencies, and evidence ownership. |
| `reference-basis/` | External reference register and applicability evidence boundary. |
| `quality-system/` | Quality manual and SOP baseline for document control, training, risk, CAPA, audit, and management review. |
| `lifecycle/` | SDMP, requirements, validation, configuration, change, release, and retirement controls. |
| `data-integrity/` | ALCOA+, electronic record/signature, retention, and auditability policy baseline. |
| `security-privacy/` | Secure SDLC, access control, audit trail, vulnerability, incident, and BCDR controls. |
| `github-operating-model/` | GitHub-native documentary QMS model, required settings, automation, and limitations. |
| `evidence-index/` | Evidence ledger and generated pack index. |
| `control-matrix/` | External reference to control to requirement to evidence mapping. |
| `validation-pack/` | Intended use, risk assessment, URS/SRS, validation plan, test protocol, deviations, and summary. |
| `supplier-pack/` | Supplier qualification evidence and customer-facing assurance pack. |
| `release-bundle/` | Release-level evidence inventory, deviations, waivers, approvals, and claim status. |
| `ai-rag-governance/` | AI-assisted extraction, RAG, citation, prompt-injection, and human-review controls. |
| `atomization-certification/` | Structure-aware atomization reports, coverage evidence, review status, and certification gates. |
| `ip-governance/` | Trademark, FTO, and third-party license guardrails before public claims or risky integrations. |
| `operations/` | Periodic review, incident/CAPA handling, retention, backup/restore, and audit-trail review. |
| `customer-integration/` | Client validation support, shared responsibility, deployment boundaries, and acceptance checklist. |
| `domain-packs/` | Customer install material for domain packs: per-pack install guide, validation checklist, claim boundary, and the reusable templates new packs adopt (DOR-021). |
| `decisions/` | Controlled decisions that alter scope, claims, validation strategy, or regulated posture. |

## Automation

Evidence-oriented automation:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
python scripts/regulated_github_qms_audit.py --repo RBOKproject/NOMOS --output .regulated-evidence-pack/github-qms-audit.json
```

The GitHub audit may report `requires_live_evidence` when repository settings, audit-log exports, protected environments, retention policy, or security features cannot be verified from repository files alone.

## Non-Negotiable Claim Rule

If evidence is missing, record a gap. Do not rewrite the gap as a claim.

For public wording, use [../public-claim-boundary.md](../public-claim-boundary.md).

# Regulated By Design Operating Structure

This directory is the governed operating area for Nomos and Praxis regulated-by-design work.

It exists so quality, validation, security, supplier-assurance, and regulated-domain reviewers can inspect how claims are controlled before they inspect generated artifacts.

## Execution Model

Nomos is the baseline producer. Praxis is the downstream runtime assurance consumer.

The current execution order is:

1. stabilize Nomos CLI, CUE, corpus feed, and self-compliance;
2. make Nomos evidence deterministic, traceable, and ALCOA+ aligned;
3. export a Praxis-compatible evidence contract;
4. let Praxis consume verified Nomos artifacts for runtime scenarios, invariants, evidence retention, and CAPA.

## Directory Responsibilities

| Directory | Responsibility |
|---|---|
| `product-profiles/` | Product role, NQ level, public-claim boundary, critical dependencies, and evidence ownership. |
| `control-matrix/` | External reference to control to requirement to evidence mapping. |
| `validation-pack/` | Intended use, risk assessment, URS/SRS, validation plan, test protocol, deviations, summary. |
| `supplier-pack/` | Supplier qualification evidence and customer-facing assurance pack. |
| `release-bundle/` | Release-level evidence inventory, deviations, waivers, approvals, and claim status. |
| `ai-rag-governance/` | AI-assisted extraction, RAG, citation, prompt-injection, and human-review controls. |
| `atomization-certification/` | Structure-aware atomization reports, coverage evidence, review status, and certification gates. |
| `operations/` | Periodic review, incident/CAPA handling, retention, backup/restore, audit-trail review. |
| `customer-integration/` | Client validation support, shared responsibility, deployment boundaries, acceptance checklist. |
| `decisions/` | Controlled decisions that alter scope, claims, validation strategy, or regulated posture. |

## Current Status

The structure is installed, but the product is not regulated-grade yet.

Nomos remains at `NQ-0/NQ-1 boundary` until the build, schema, corpus feed, and self-compliance gates are green.

Praxis remains downstream and cannot be used as regulated support evidence for Nomos until Nomos publishes verified producer artifacts and a shared evidence contract.

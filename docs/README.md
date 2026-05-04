# Nomos Documentation

This directory contains the method, product architecture, regulated-readiness baseline, validation records, and decision history for Nomos.

## Start Here

| Document | Use |
|---|---|
| [Public claim boundary](public-claim-boundary.md) | What Nomos can and cannot claim at the current release. |
| [Method overview](01-method-overview.md) | Canonical-first concepts and vocabulary. |
| [Operational procedure](12-operational-procedure.md) | Step-by-step canonical-first operating procedure. |
| [Product roadmap](14-product-roadmap.md) | Product architecture and roadmap. |
| [Regulated quality reference](21-regulated-quality-reference.md) | Quality and compliance baseline for regulated-market readiness. |
| [Regulated implementation plan](23-regulated-implementation-plan.md) | How the regulated-readiness track is implemented. |
| [Regulated by design structure](25-regulated-by-design-structure.md) | Shared structure for Nomos and Praxis readiness work. |
| [Atomization process](26-structure-aware-atomization-process.md) | Structure-aware atomization and fidelity certification approach. |
| [AAA+ regulated document set](27-aaa-regulated-it-document-set.md) | Target document set and non-invention rule. |
| [Compliance closure plan](28-regulated-compliance-closure-plan.md) | Current closure plan for regulated-readiness gaps. |

## Current Release Evidence

For `v0.1.0-ALPHA`, the most important evidence themes are:

- release validation runs CLI, E2E, Python, regulated documentation, and evidence-pack gates;
- RBOK lawbook POC produces a full artifact pack from a read-only clone;
- RBOK `01_rbok` source-to-feed POC records 3024 feed units, 3024 RAG chunks, 3024/3024 source-backed units/chunks, zero uncovered body-ledger bytes, and zero semantic blocking findings;
- strict fidelity and source-to-feed gates are release-gated for the recorded POC scope;
- regulated documentation exists as a baseline but is not a certification;
- public claims are limited by [public-claim-boundary.md](public-claim-boundary.md).

## Regulated Documentation

Regulated-readiness records are under [regulated/](regulated/). They are designed to help regulated customers and auditors inspect intended use, controls, evidence, responsibilities, and gaps. They do not certify Nomos by themselves.

## Decisions

Architecture and product decisions are under [decisions/](decisions/) and [adr/](adr/). Any change to public claims, release gates, validation posture, or evidence contracts should have an issue, PR, and decision record when the impact is material.

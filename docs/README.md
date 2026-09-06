# Nomos Documentation

This directory contains the method, product architecture, regulated-readiness baseline, validation records, and decision history for Nomos.

## Start Here

New readers should begin with the **claim boundary**, then the method overview. Documents are grouped below. **Status tags** (`roadmap`, `plan`, `issue list`, `template`, `market analysis`, `baseline`, `future`) mark forward-looking material so it is not read as delivered capability. Unless tagged, a document describes the current implemented method.

| Document | Use |
|---|---|
| [Public claim boundary](public-claim-boundary.md) | What Nomos can and cannot claim at the current release. **Read this first.** |
| [43 - Development doctrine](43-development-doctrine.md) | Formal cross-product development approach & principles (zero-regression, claim-boundary, no *done* without adversarial proof). NOMOS × Aedifica. |
| [47 - Independent roadmaps and risk-based validation](47-roadmap-lanes-and-risk-based-validation.md) | Router for the independent product, DevOps and regulated-assurance roadmaps; autonomous dispatch rules and risk-based validation of regulated tooling. `active policy` |
| [49 - Neighbouring sovereign legal RAG: concepts, anti-patterns, collaboration](49-sovereign-legal-rag-concepts-and-collaboration.md) | Analysis of a neighbouring sovereign legal RAG from two private dated documents, with no figure or finding of the third-party system reproduced: concepts integrated (domain cartography contract, parameter inventory template, inference boundary control), silent-failure anti-patterns encoded as doctrine principle 8 and a silence guard, and the collaboration brief (verification-first checklist, questions, interfaces, red lines). `analysis + plan` |
| [Release notes v0.1.0-ALPHA](release-v0.1.0-alpha.md) | Current release scope and the publication gate. |
| [External assessment pack](external-assessment/) | Impartial inputs for an external assessment: evidence, maturity, and neutral valuation frameworks. |

### Method and process
| Document | Use |
|---|---|
| [01 - Method overview](01-method-overview.md) | Canonical-first concepts and vocabulary. |
| [02 - Source registry](02-source-registry.md) | How authoritative sources are registered. |
| [03 - Atomization and canonical matrix](03-atomization-and-matrix.md) | Canonical node extraction and the traceability matrix. |
| [04 - Contracts, schemas and read-models](04-contracts-schemas-readmodels.md) | Data contracts, CUE schemas, and read-models. |
| [05 - Knowledge base, vector store and RAG](05-knowledge-base-and-rag.md) | RAG metadata model (traceable metadata; production retrieval not validated). |
| [06 - Product integration](06-product-integration.md) | How downstream products consume artifacts. |
| [07 - Tests, gates and release](07-tests-gates-release.md) | Test and gate model for releases. |
| [08 - Governance and change](08-governance-and-change.md) | Change control over claims, gates, and evidence. |
| [09 - Adaptation guide](09-adaptation-guide.md) | Adapting the method across domains and stacks. |
| [12 - Operational procedure](12-operational-procedure.md) | Step-by-step canonical-first operating procedure. |
| [13 - Agent and skills blueprint](13-agent-skills-blueprint.md) | Blueprint for agent/skill automation. |
| [26 - Structure-aware atomization and certification](26-structure-aware-atomization-process.md) | Atomization engine and fidelity certification approach. |
| [Canonical corpus mode](canonical-corpus-mode.md) | Canonical corpus processing mode. |
| [Verdict taxonomy](verdict-taxonomy.md) | Gate verdict vocabulary. |

### Product direction (forward-looking)
| Document | Use |
|---|---|
| [11 - Generic roadmap and issue list](11-roadmap-and-issues.md) | Generic roadmap template. `roadmap` |
| [14 - Product roadmap](14-product-roadmap.md) | Independent product architecture/version roadmap; no QMS or human-record gates. `roadmap` |
| [15 - Product backlog](15-product-backlog.md) | Implementation backlog and open issues. `backlog` |
| [16 - Versioning policy](16-versioning-policy.md) | Versioning and compatibility policy. |
| [29 - Post-alpha release issue list](29-post-alpha-release-issue-list.md) | Issues planned after the alpha. `issue list` |
| [38 - Domain opportunity roadmap](38-domain-opportunity-roadmap.md) | Market/domain opportunity scan (GxP, medical, AI, finance, legal, etc.) and atomic issue list. `market analysis` / `roadmap` — not delivered capability. |
| [39 - Canonical Knowledge Mesh pivot](39-canonical-knowledge-mesh-pivot.md) | Master pivot plan: CKM thesis, epics, and issue mapping. `plan` |
| [40 - Knowledge mesh architecture](40-knowledge-mesh-architecture.md) | Meta architecture: faceting, lens, canon promotion, thin domain packs. `design` |
| [41 - State-of-the-art positioning](41-state-of-the-art-positioning.md) | Honest novelty verdict per pillar, competitive map, dated windows. `market analysis` |
| [42 - Capitalization and improvement plan](42-capitalization-and-improvement-plan.md) | Adopt/integrate/isolate sourcing plan: catch up, fill gaps, amplify strengths. `plan` |
| [43 - Canonical knowledge bundle contract](43-canonical-knowledge-bundle-contract.md) | CKB bundle handoff contract (consumer seam). *(Note: shares the `43-` prefix with the development doctrine above.)* |
| [44 - CKM facet ontology architecture](44-ckm-facet-ontology-architecture.md) | BFO→IOF→pack facet-ontology architecture note. *(Note: shares the `44-` prefix with the trust-tier policy below.)* |
| [44 - Facet trust-tier policy](44-facet-trust-tier-policy.md) | Trust-tier derivation policy: `certified` never auto-derived. |
| [45 - Vision-reality closure plan](45-vision-reality-closure-plan.md) | Master execution plan closing every audited gap between vision/ADR and reality, with adversarial proof per item. `plan` |
| [46 - VRC epic issue list](46-vrc-epic-issue-list.md) | Atomic issue decomposition of plan 45 (VRC-00..46), CKM-style, with the 5 mandatory governance lines per issue. `issue list` |
| [Roadmap lane registry](roadmap-lanes.yaml) | Machine-readable lane, dispatch, dependency, evidence and claim states; guarded in CI. |

### Source-to-feed integrity engine
| Document | Use |
|---|---|
| [20 - Corpus operability corrective epics](20-corpus-operability-corrective-epics.md) | Corrective epics for corpus operability. `epics` |
| [21 - Source-to-feed integrity engine](21-source-feed-integrity-engine.md) | Source-to-feed integrity engine design. *(Note: shares the `21-` prefix with the regulated quality reference below.)* |
| [10 - Tools and projects to study](10-tools-and-projects-to-study.md) | External tools/projects reference. |

### Regulated-readiness (baseline, not certification)
| Document | Use |
|---|---|
| [21 - Regulated quality and compliance reference](21-regulated-quality-reference.md) | Quality/compliance baseline for regulated-market readiness. `baseline` |
| [22 - Nomos/Praxis synergy and market audit](22-nomos-praxis-synergy-market-audit.md) | Synergy, regulated-market, and blind-spot audit. `market analysis` |
| [23 - Regulated implementation plan](23-regulated-implementation-plan.md) | How the regulated-readiness track is implemented. `plan` |
| [24 - Regulated client compliance evidence](24-regulated-client-compliance-evidence.md) | Template and guidance for client-side compliance evidence. `template` |
| [25 - Regulated by design structure](25-regulated-by-design-structure.md) | Shared readiness structure for Nomos and Praxis. |
| [27 - AAA+ regulated IT document set](27-aaa-regulated-it-document-set.md) | Target document set and the non-invention rule. `target set` |
| [28 - Regulated compliance closure plan](28-regulated-compliance-closure-plan.md) | Independent regulated-assurance roadmap; manual-first controls and risk-based validation of supporting tools. `plan` |
| [regulated/](regulated/) | Regulated-readiness baseline records, templates, and domain packs. `baseline` / `roadmap` |

### GitHub workflow and app
| Document | Use |
|---|---|
| [30 - GitHub workflow integration issue list](30-github-workflow-integration-issue-list.md) | Issues for the GitHub workflow integration. `issue list` |
| [31 - GitHub workflow setup](31-github-workflow-setup.md) | Install guide for the reusable NOMOS GitHub workflow (NGW). |
| [32 - GitHub App readiness boundary](32-github-app-readiness-boundary.md) | GitHub App boundary and event-mapping contract. `future` |

### Manuals and integration
| Document | Use |
|---|---|
| [33 - Documentation guide](33-nomos-documentation-guide.md) | Audiences, artifact roles, claim boundary, and downstream consumption. |
| [34 - User manual](34-nomos-user-manual.md) | Operating Nomos locally, reading outputs, and verifying a run. |
| [35 - Integration manual](35-nomos-integration-manual.md) | End-to-end integration: source repo, workflow, output, runtime. |
| [36 - RBOK integration recommendation plan](36-rbok-integration-recommendation-plan.md) | Downstream RBOK plan (no modification of RBOK from Nomos). `plan` |
| [37 - RBOK Nomos recommendations implementation plan](37-rbok-nomos-recommendations-implementation-plan.md) | Task-by-task downstream implementation plan. `plan` |
| [RBOK engine import contract](rbok-engine-import-contract.md) | Handoff contract for the RBOK engine import. |

### Evidence and validation
| Document | Use |
|---|---|
| [RBOK 01_rbok POC validation dossier](rbok-poc-validation-dossier.md) | Scoped POC evidence (single corpus, recorded run). |
| [Self-compliance report](self-compliance-report.md) | Nomos applied to its own repository. |

### Environment
| Document | Use |
|---|---|
| [Windows corpus setup](windows-corpus-setup.md) | Windows environment setup for corpus commands. |

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

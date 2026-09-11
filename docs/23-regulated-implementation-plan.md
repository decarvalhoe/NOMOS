# 23 - Regulated-Grade Implementation Plan

Date: 2026-09-05
Current release: `v0.1.0-ALPHA`

## Purpose

This plan turns the regulated quality reference, Nomos/Praxis synergy audit, and regulated-by-design structure into the current implementation path.

This is an **independent regulated-assurance plan**, not a product or DevOps
release train. Per [ADR-VRC-0004](adr/0004-independent-roadmaps-risk-based-validation.md),
manual controls may operate before supporting tools exist; tools may be
developed and technically verified before they are validated for a regulated
intended use. Until validation proportional to intended use and risk is
complete, tool output is supporting evidence and remains manually reconciled.
Human records, calendar evidence, licensed acquisition and approvals block only
their named regulated claim, never product/DevOps task selection.

The goal is not to make a stronger claim. The goal is to make every future claim defensible through:

- intended-use definition;
- risk-based validation;
- source and evidence integrity;
- controlled documents;
- release evidence;
- customer qualification support;
- independent reconstruction.

## Current Verdict

Nomos is ready for an alpha release with regulated-readiness positioning.

Nomos is not yet a validated regulated platform.

Current quality position:

```text
NQ-2 alpha achieved by operational CLI, real RBOK corpus evidence, strict fidelity gate, and bounded public docs.
NQ-3 candidate remains open until self-compliance evidence, owner/approval records, and GitHub QMS evidence are closed.
```

## Current Regulated Inputs And Claim Gates

| Issue | Regulated input / claim gate | Blocks |
|---|---|---|
| `#192` | Licensed ISO 13485 acquisition/intake | Complete regulated reference baseline. |
| `#193` | Licensed ISO/IEC/IEEE 12207 acquisition/intake | Lifecycle-standard clause closure. |
| `#194` | GAMP 5 and ISO/IEC 25010 license review | Licensed-standard processing decision. |
| `#196` | Licensed bible processing | Named licensed-reference proof at higher assurance levels. Public processing proceeds in #644. |

These issues block no product or DevOps version. They lock only the regulated
use/claim named in the third column.

## Quality-Level Ladder

| Level | Meaning | Current status |
|---|---|---|
| `NQ-0` | Broken build/schema/gate. | Not current. |
| `NQ-1` | Method documented only. | Passed. |
| `NQ-2` | Operational tool with repeatable gates and real evidence. | Current alpha level. |
| `NQ-3` | Nomos-on-Nomos self-compliance evidence is generated, reviewed, and bounded. | Candidate, not approved. |
| `NQ-4` | Nomos/Praxis or equivalent runtime assurance evidence is contract-linked. | Future. |
| `NQ-5` | Scoped validation pack and release bundle are customer-review ready. | Future. |
| `NQ-6` | Independent reviewer can reconstruct every material claim. | Future. |

## Independent Regulated Workstreams (Not Product Phase Gates)

### Historical Baseline - Alpha Release Closure

Goal: publish `v0.1.0-ALPHA` without overclaim.

Required:

- README, release notes, changelog, support, security, governance, contribution, and claim-boundary docs.
- Version set to `0.1.0-ALPHA`.
- Local Go, E2E, and Python gates.
- Regulated documentation gate and evidence pack.
- GitHub pre-release, not stable production release.

Exit gate:

```text
Release exists, checks are green, and no public document claims formal regulated compliance.
```

### Workstream R1 - NQ-3 Self-Compliance Closure

Goal: make Nomos self-compliance executable and reviewer-ready.

Required:

- deterministic self-compliance report;
- coverage report;
- regulated reference canon report;
- GitHub QMS live evidence or explicit waiver;
- named quality, technical, security, and release owners;
- training matrix and records;
- approval semantics for GitHub reviews;
- CODEOWNERS coverage for controlled docs, specs, evidence scripts, and release files.

Exit gate:

```text
An internal quality reviewer can trace Nomos claims to controlled evidence without relying on chat history.
```

### Product Evidence Interface - Portable Fidelity

Owned and sequenced by product roadmap 14. This roadmap consumes its versioned
evidence; it does not block that implementation on R1 records.

Goal: prove Nomos is not RBOK-only.

Required:

- Markdown AST fidelity fixtures beyond RBOK;
- structured YAML/JSON atomization policy;
- legal/regulatory corpus fixture;
- technical standard fixture;
- game-rule corpus fixture;
- table/list/callout/code/image/link/annex/xref coverage;
- exact source spans for active blocks;
- explicit unsupported-block records.

Exit gate:

```text
The fidelity gate blocks missing active source coverage across portable fixture families.
```

### Workstream R2 - Reference Bible Closure

Goal: make regulatory and quality references controlled authorities.

Required:

- retain `#192`, `#193`, `#194`, and `#196` as explicit external/human states
  until each named source is legitimately available and approved;
- execute public processing independently through #644;
- keep licensed full text outside public Git unless license allows redistribution;
- publish sidecars with source, hash, license holder, allowed use, reviewer, and decision;
- process permitted public and licensed material read-only with Nomos;
- map references to controls, tests, evidence, waivers, and blocked claims.

Exit gate:

```text
No cited framework remains decorative.
```

### Product Evidence Interface - RAG And AI Governance

Owned and sequenced by product roadmap 14. Regulated intended-use validation
may later consume the evidence; it is not an implementation prerequisite.

Goal: make downstream LLM use bounded by the canonical source.

Required:

- retrieval evaluation with expected citations;
- refusal tests when source evidence is missing;
- prompt-injection and excessive-agency controls;
- concise-answer contract for module/question flows;
- model/provider policy for Swiss-only deployments where required;
- traceability from RAG chunk to canonical unit to source span.

Exit gate:

```text
LLM output assists the product without becoming the authority.
```

### Workstream R4 - Nomos/Praxis Regulated Reliance

Goal: connect canonical evidence to runtime assurance.

Required:

- consume the technical boundary delivered by closed issue `#320`;
- publish Nomos-to-Praxis atom mapping;
- validate shared evidence ledger fixtures;
- record Praxis evidence quality level in Nomos release bundles;
- feed runtime deviations/CAPA back into Nomos controls.

Schema/import/reject fixtures may be developed beforehand with synthetic or
`not_qualified` inputs. This workstream gates only authoritative regulated
reliance and joint claims.

Exit gate:

```text
Joint claims are permitted only when both products have compatible evidence and quality levels.
```

### Phase 7 - NQ-5/NQ-6 Validation Readiness

Goal: support regulated customer qualification for a scoped intended use.

Required:

- validation inventory;
- intended-use model;
- risk assessment;
- URS/SRS or equivalent requirements baseline;
- traceability matrix;
- test protocols and challenge cases;
- deviation and CAPA log;
- approval records;
- release evidence bundle;
- retention and reconstruction procedure;
- independent reconstruction review.

Exit gate:

```text
A regulated customer or independent reviewer can qualify Nomos for a declared intended use using retained evidence.
```

## Documentation Alignment Rules

Each PR must update documentation when it changes:

| Change type | Required docs |
|---|---|
| CLI behavior or command surface | README, CLI README, relevant specs, tests. |
| Corpus/fidelity behavior | RBOK POC dossier, atomization docs, public claim boundary if claim changes. |
| Regulated evidence or references | Regulated README, control matrix, validation pack, release bundle docs. |
| Public wording | README, RELEASE, public claim boundary, changelog. |
| Nomos/Praxis interface | Synergy audit, customer integration docs, evidence contract docs. |
| Security/support/governance | SECURITY, SUPPORT, GOVERNANCE, contribution docs. |

## Claim Discipline

Public and sales-facing documents must not imply:

- certification;
- customer validation;
- universal source-format fidelity;
- legal authority;
- Part 11/GxP/Annex 11 compliance;
- ISO certification;
- NASA qualification;
- eQMS replacement;
- licensed-standard redistribution rights.

Unless that exact claim has a release gate and retained evidence.

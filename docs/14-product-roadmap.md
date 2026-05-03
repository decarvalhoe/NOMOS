# 14 - Nomos Product Roadmap

Date: 2026-05-03
Current release: `v0.1.0-ALPHA`

## Positioning

Nomos is a canonical-first product intelligence platform. Its purpose is to turn authoritative source material into governed, traceable, testable product evidence.

The product target is not "works on every repository by magic". The target is:

```text
Nomos admits what it can prove, refuses what it cannot prove, and records the exact evidence boundary.
```

## Current Alpha State

`v0.1.0-ALPHA` establishes a working alpha baseline:

- CLI diagnosis and corpus commands are implemented.
- The RBOK lawbook profile produces real feed, TOC, lexicon, RAG metadata, runtime import, proof report, and attestation artifacts.
- The strict fidelity gate is wired into release validation.
- Source spans and typed blocks are populated for the current RBOK lawbook POC.
- The regulated-by-design documentation structure is installed.
- Public claims are bounded by `docs/public-claim-boundary.md`.

The alpha does not claim formal regulated validation, universal corpus fidelity, complete licensed-standard clause mapping, or production RAG behavior control.

## Quality-Level Position

Nomos currently sits at:

```text
NQ-2 alpha: operational tool with real corpus evidence.
NQ-3 candidate: self-compliance structure exists, but approval and independent evidence closure are not complete.
```

Allowed wording:

```text
Nomos v0.1.0-ALPHA provides a working canonical-first CLI, RBOK lawbook POC evidence, strict fidelity gates, and a regulated-readiness documentation baseline.
```

Blocked wording:

```text
Nomos is a validated regulated platform.
Nomos is Part 11/GxP/Annex 11 compliant as a product.
Nomos can convert any source into legally defensible product law without customer validation.
```

## Product Principles

1. Fail closed when evidence is missing.
2. Read source corpora as protected authorities by default.
3. Preserve source identity, source hash, source span, hierarchy, and governance metadata.
4. Treat adapters and corpus profiles as versioned capabilities with explicit limits.
5. Keep generated RAG chunks downstream from canonical units, never as the authority.
6. Make every public claim traceable to a gate, artifact, document, decision, or open gap.

## Roadmap

| Stage | Target | Exit gate |
|---|---|---|
| `v0.1.0-ALPHA` | Prove the canonical corpus pipeline, RBOK lawbook POC, strict fidelity gate, and public claim boundary. | Local and CI gates green; GitHub pre-release published; no regulated overclaim. |
| `v0.2.x` | Make structure fidelity more portable beyond RBOK Markdown. | AST-to-Nomos comparison covers tables, lists, callouts, code, links, images, annexes, xrefs, H1-H6, and exact source spans across fixtures. |
| `v0.3.x` | Harden adapters and structured-data atomization. | Markdown, YAML, JSON, and adapter fixtures produce governed nodes or explicit unsupported records; no silent skip of active source material. |
| `v0.4.x` | Mature RAG and runtime import contracts. | Retrieval metadata, citation behavior, refusal cases, and downstream engine import are evaluated with traceable tests. |
| `v0.5.x` | Close regulated-readiness evidence. | Reference-to-control matrix, validation inventory, GitHub QMS evidence, training records, approval records, and release bundle are reconstructible. |
| `v0.6.x` | Publish the Nomos-to-Praxis evidence contract. | Praxis can consume verified Nomos artifacts without weakening the Nomos claim boundary. |
| `v1.0` | Production-grade release candidate for scoped intended uses. | Support model, compatibility policy, validation evidence, security process, customer integration guide, and independent reconstruction evidence are complete. |

## Architecture Direction

Nomos remains split into clear responsibility layers:

| Layer | Responsibility |
|---|---|
| Specs | CUE/JSON/YAML contracts, examples, evidence schemas, and compatibility rules. |
| CLI | Deterministic project diagnosis, corpus processing, fidelity gates, reports, and attestations. |
| Corpus profiles | Domain-specific interpretation of source corpora without hard-coding the core engine to one customer. |
| Adapters | Stack-specific detection and extraction capabilities with declared limits. |
| Evidence | Reports, attestations, proof files, release bundles, and ALCOA+ metadata. |
| Regulated docs | Quality, validation, supplier, control, approval, retention, and claim-governance records. |
| Control plane | Optional portfolio-level registry and dashboard, not required for the alpha proof. |

## Active Product Risks

| Risk | Current handling |
|---|---|
| Overclaiming regulated readiness | Public claim boundary and release docs explicitly block certification language. |
| RBOK-specific implementation bias | Roadmap requires portable AST/fidelity fixtures beyond RBOK. |
| Licensed reference misuse | Licensed standards are tracked by intake sidecars; redistribution remains blocked unless license allows it. |
| RAG authority drift | RAG remains downstream from canonical units and requires future retrieval/behavior evals. |
| Praxis dependency overclaim | Praxis is downstream and not used as release-grade evidence until the shared contract is verified. |

## Release Gates

Before public release notes or customer-facing wording change:

```bash
cd cli
go test ./...

powershell -NoProfile -ExecutionPolicy Bypass -File scripts\e2e.ps1
python -m unittest discover -s tests -v
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
```

## Definition Of v1.0

Nomos can be considered a production-grade release candidate only when:

- admitted corpus scopes are explicit and reproducible;
- unsupported source structures become explicit evidence records, not silent gaps;
- source spans and document hierarchy are independently checkable;
- adapters publish compatibility contracts and fixtures;
- generated chunks, matrices, reports, and attestations are reconstructible;
- regulated documentation maps to real gates, owners, approvals, and retained evidence;
- customer validation remains scoped to intended use and is supported by a supplier pack;
- public claims never exceed the current evidence level.

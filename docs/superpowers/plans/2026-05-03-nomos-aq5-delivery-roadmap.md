# NOMOS AQ-5 Delivery Roadmap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. This roadmap is the delivery plan from the current AQ-3 partial proof toward AQ-5 compliance-grade defensibility.

**Goal:** Deliver NOMOS to the target level where it can defensibly claim source-faithful, compliance-grade conversion of governed corpora into lawbook structures, atoms, traceability, RAG outputs, and audit evidence.

**Architecture:** Keep NOMOS universal. Parser adapters produce a common AST; profiles map corpus-specific semantics; gates prove fidelity and prevent overclaiming. RBOK remains the first proof corpus, not the engine boundary.

**Tech Stack:** Go CLI, JSON/YAML/CUE artifacts, GitHub Issues/PRs/Actions, existing NOMOS corpus commands, `realisons-business/01_rbok` as proof corpus, future adapters for DOCX/PDF/HTML.

---

## Why We Do Not Stop At PR #243

PR #243 proves a first controlled slice:

- Markdown active sources now carry richer typed block coverage.
- YAML/JSON structured sources now carry source spans.
- Structure fidelity gate checks 64 active RBOK text sources.
- RBOK read-only POC passes with 5,540/5,540 covered source blocks.

That is useful but not the target. It supports an AQ-3 partial claim for supported text adapters only.

We do not stop there because AQ-5 requires:

- certified table of contents;
- governed lexicon and definition conflict controls;
- semantic fidelity gate;
- cross-reference and annex graph;
- context-safe atom boundaries;
- RAG chunks derived only from certified atoms;
- complete evidence pack;
- validation and quality procedures;
- explicit handling of PDF/DOCX/reference originals;
- controlled AQ claim evaluator.

## Target Claim

NOMOS can claim AQ-5 only when the release evidence proves:

- every active supported source is parsed through a validated adapter;
- every active source block is represented, intentionally excluded, or explicitly unsupported with finding;
- every node and atom has source hash, exact span, quote selector, parent chain, TOC path, semantic role, lifecycle, and allowed-use metadata;
- TOC, lexicon, cross-reference graph, semantic graph, traceability matrix, RAG metadata, attestation, and lockfile are generated reproducibly;
- no protected corpus mutation occurs;
- the AQ claim evaluator blocks claims above the evidence level;
- unresolved gaps are visible and blocking at the right AQ level.

## Delivery Phases

### Phase 0: Baseline And Claim Control

Purpose: stop overclaiming and make every future PR measurable.

Deliverables:

- AQ claim evaluator.
- Evidence pack manifest.
- Updated release gate with AQ target.
- CI artifact upload for proof reports.

Exit criteria:

- AQ-3 partial is explicitly stated for PR #243.
- AQ-4/AQ-5 fail until their required artifacts exist.

### Phase 1: Universal Parser Contract

Purpose: ensure every adapter speaks the same verifiable language.

Deliverables:

- adapter registry;
- parser contract tests;
- original-byte span conformance;
- unsupported-format findings.

Exit criteria:

- Markdown, YAML, JSON, and plain text pass the same adapter conformance suite.
- CRLF and UTF-8 fixtures prove line/column/byte offsets against original bytes.

### Phase 2: Full Text Adapter Fidelity

Purpose: remove silent flattening from supported text formats.

Deliverables:

- Markdown/GFM full block model;
- YAML AST based on `yaml.Node`;
- JSON AST with JSON pointer paths;
- table/list/callout/code/image/link/reference typing;
- source coverage reports per adapter.

Exit criteria:

- deliberately missing or flattened blocks fail structure fidelity.
- structured YAML/JSON paths produce exact field/value spans.

### Phase 3: Certified Hierarchy And TOC

Purpose: prove NOMOS understands and reproduces document structure.

Deliverables:

- `toc.json`;
- NOMOS tree artifact;
- source TOC vs NOMOS tree comparator;
- profile hierarchy mapping rules.

Exit criteria:

- source headings/sections/articles/clauses/modules/questions map to the generated tree;
- missing, reordered, or mis-parented hierarchy fails the gate.

### Phase 4: Semantic Fidelity

Purpose: prove NOMOS understands meaning-bearing structures.

Deliverables:

- semantic role classifier;
- obligation/permission/prohibition/exception/rule/definition/example/case typing;
- internal link and cross-reference graph;
- annex/reference/bibliography graph;
- semantic fidelity report.

Exit criteria:

- semantic nodes and atoms carry roles and references;
- unresolved cross-references are reported;
- semantic fidelity gate blocks silent loss.

### Phase 5: Lexicon And Concept Governance

Purpose: make terms, definitions, acronyms, concepts, domains, and subjects governable.

Deliverables:

- `lexicon.json`;
- concept taxonomy;
- definition conflict report;
- undefined critical term report;
- node/atom concept assignment.

Exit criteria:

- definition conflicts are visible and severity-scored;
- critical terms without definitions can block AQ-4/AQ-5;
- atoms include related term IDs.

### Phase 6: Context-Safe Atomization

Purpose: ensure fine-grained atoms do not lose context.

Deliverables:

- atom schema;
- boundary policies by semantic role;
- deterministic atom IDs;
- over-fragmentation and orphan-atom metrics;
- atom-to-node-to-source traceability.

Exit criteria:

- no atom exists without parent chain, TOC path, source span, semantic role, and review state;
- law/regulation/game-rule/RBOK fixtures produce coherent atom sets.

### Phase 7: RAG And Conversation Provenance

Purpose: make RAG outputs safe, concise, governed, and traceable.

Deliverables:

- source-backed RAG chunks;
- chunk provenance contract;
- response posture controls;
- RBOK/Airbook conversation fixtures;
- provider adapter contract, including Infomaniak-ready usage.

Exit criteria:

- no RAG chunk exists without certified atom provenance;
- conversation tests reject verbose multi-question output;
- prompt/behavior configuration is itself governed corpus data.

### Phase 8: Reference Originals And Non-Text Adapters

Purpose: handle PDF/DOCX/HTML without pretending unsupported formats are atomized.

Deliverables:

- DOCX adapter plan and first extraction support;
- PDF text/layout adapter plan and first extraction support;
- HTML adapter;
- reference-original policy;
- licensed document acquisition register.

Exit criteria:

- unsupported binary/reference sources are admitted or blocked with explicit policy;
- no AQ-5 claim depends on unprocessed originals unless accepted as controlled references.

### Phase 9: Regulated Quality System

Purpose: make NOMOS itself credible in regulated IT contexts.

Deliverables:

- validation master plan;
- intended use statement;
- requirements traceability matrix;
- risk assessment;
- test evidence protocol;
- release checklist;
- audit trail and change-control procedure.

Exit criteria:

- product claims, requirements, tests, risks, and evidence are traceable;
- GitHub workflow can produce the validation evidence pack.

### Phase 10: RBOK AQ-5 Proof Run

Purpose: prove NOMOS on the RBOK corpus without making NOMOS RBOK-specific.

Deliverables:

- RBOK AQ-4 semantic proof;
- RBOK AQ-5 evidence pack;
- remaining gap register;
- final claim decision.

Exit criteria:

- read-only proof remains clean;
- evidence pack is reproducible;
- unresolved gaps are either fixed or explicitly block AQ-5.

## Dependency Tree

```text
NUF-0 Claim Control
  -> NUF-1 Universal Parser Contract
      -> NUF-2 Markdown Full Fidelity
      -> NUF-3 YAML/JSON Structured Adapters
      -> NUF-12 Non-Text Adapters
          -> NUF-4 Certified TOC And Hierarchy
              -> NUF-5 Semantic Typing And Cross-References
                  -> NUF-6 Lexicon And Concept Governance
                      -> NUF-7 Context-Safe Atomization
                          -> NUF-9 RAG And Conversation Provenance
                              -> NUF-10 RBOK AQ-5 Proof Corpus
  -> NUF-8 Blocking Fidelity Gates watches every phase
  -> NUF-11 Regulated Quality System watches every phase
```

## GitHub Issue Structure

Existing macro epics:

- NUF-1: Universal parser contract
- NUF-2: Markdown full fidelity adapter
- NUF-3: YAML/JSON structured adapters
- NUF-4: Certified TOC and hierarchy
- NUF-5: Semantic typing and cross-references
- NUF-6: Lexicon and concept governance
- NUF-7: Context-safe atomization engine
- NUF-8: Blocking fidelity gates
- NUF-9: RAG and conversation provenance
- NUF-10: RBOK POC proof corpus

Additional macro epics required by AQ-5:

- NUF-0: AQ claim control and evidence manifest
- NUF-11: Regulated quality system and validation evidence
- NUF-12: Non-text adapters and reference-original governance

Operational issues to create:

- NUF-0.1 AQ claim evaluator
- NUF-0.2 Evidence pack manifest
- NUF-0.3 CI artifact publishing for proof reports
- NUF-1.1 Adapter interface and registry
- NUF-1.2 Original-byte span conformance
- NUF-1.3 Shared adapter conformance harness
- NUF-1.4 Unsupported format findings
- NUF-2.1 Markdown/GFM full block model
- NUF-2.2 Markdown annex/reference/cross-reference candidates
- NUF-2.3 Markdown losslessness fixture suite
- NUF-3.1 YAML AST with node-level spans
- NUF-3.2 JSON AST with pointer and field/value spans
- NUF-3.3 Structured path semantic mapping
- NUF-4.1 TOC artifact schema and generator
- NUF-4.2 Source TOC vs NOMOS tree comparator
- NUF-4.3 Profile hierarchy mapping rules
- NUF-5.1 Semantic role classifier
- NUF-5.2 Link/citation/cross-reference graph
- NUF-5.3 Annex and bibliography graph
- NUF-6.1 Lexicon artifact and extractor
- NUF-6.2 Definition conflict and undefined term gates
- NUF-6.3 Domain/subject/concept taxonomy
- NUF-7.1 Atom boundary policy
- NUF-7.2 Context-safe atom schema and deterministic IDs
- NUF-7.3 Atom quality metrics
- NUF-8.1 Structure and semantic fidelity gate integration
- NUF-8.2 AQ evidence-level release gate
- NUF-8.3 CI read-only and artifact gates
- NUF-9.1 Source-backed RAG chunk generator
- NUF-9.2 Conversation posture contract and tests
- NUF-9.3 Provider adapter contract for governed LLM/RAG
- NUF-10.1 RBOK AQ-3 proof report
- NUF-10.2 RBOK AQ-4 semantic proof run
- NUF-10.3 RBOK AQ-5 evidence pack and release decision
- NUF-11.1 Intended use and validation master plan
- NUF-11.2 Requirements traceability matrix
- NUF-11.3 Risk assessment and control mapping
- NUF-11.4 Test evidence protocol and release checklist
- NUF-12.1 Reference-original policy and licensed document register
- NUF-12.2 DOCX adapter feasibility and first implementation
- NUF-12.3 PDF text/layout adapter feasibility and first implementation
- NUF-12.4 HTML adapter and external reference handling

## Definition Of Done For AQ-5

- All NUF macro epics are closed or explicitly accepted as not required for the target claim.
- AQ claim evaluator reports AQ-5 with evidence, not manual assertion.
- RBOK proof corpus produces all required artifacts outside the corpus.
- Release gate passes AQ-5 target.
- Read-only proof is clean.
- Evidence pack includes source inventory, policy, parser reports, TOC, lexicon, semantic graph, atom set, RAG metadata, traceability matrix, attestation, risk controls, test evidence, and known gap register.
- Known gap register has no blocker for the AQ-5 claim.

## Immediate Next Work

The next implementation work should start with NUF-0 and NUF-1, not with semantic heuristics:

1. Add AQ target and claim evaluator to release gate.
2. Add evidence pack manifest.
3. Build adapter interface and conformance harness.
4. Migrate current Markdown/YAML/JSON paths to the adapter registry.
5. Only then implement TOC, semantics, lexicon, and atom boundaries.

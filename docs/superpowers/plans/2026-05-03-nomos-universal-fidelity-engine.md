# NOMOS Universal Fidelity Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Do not implement RBOK-only behavior in the engine. RBOK is a proof corpus, not the architecture boundary.

**Goal:** Build NOMOS as a universal, compliance-grade engine that converts any governed business, legal, regulatory, technical, or game-rule corpus into a source-faithful lawbook graph, atom set, traceability matrix, and RAG feed with blocking proof gates.

**Architecture:** Split NOMOS into portable engine layers: source registry, parser adapters, canonical document AST, semantic document model, atomization engine, traceability/RAG outputs, and release gates. Profiles such as `rbok-lawbook`, `legal-regulation`, `quality-standard`, and `game-rules` configure interpretation; they must not hard-code the engine.

**Tech Stack:** Go CLI, JSON/YAML/CUE artifacts, GitHub Actions, existing NOMOS corpus commands, portable parser contracts under `cli/internal/corpus/parse`, current atomization and corpus packages.

---

## Scope Boundary

This plan replaces the narrower "Markdown fidelity gate" trajectory.

NOMOS must not be RBOK-oriented. The RBOK `01_rbok` corpus remains the first hard validation corpus because it contains a realistic mix of canonical Markdown, YAML parcours, JSON AI behavior configuration, reference documents, governance metadata, and skipped/admitted sources. The engine must be reusable for:

- a business book of knowledge;
- a law, regulation, standard, or internal policy;
- a quality/compliance corpus;
- a game rulebook;
- a technical operations corpus;
- future DOCX/PDF/HTML source corpora.

Do not claim "lawbook/compliance-grade fidelity" until the gate proves source-to-NOMOS coverage, exact locators, typed semantics, glossary/TOC integrity, and reproducible output.

## Current Non-OK Items

- Full semantic document reconstruction is not complete.
- Tables, callouts, code blocks, complex links, images, annexes, and cross-references are not all typed finely enough.
- H5/H6 dedicated legal levels are being introduced but are not yet validated as a universal hierarchy model.
- Top-level feed locators can still be logical/ordinal instead of exact line/column/byte locators.
- The structure fidelity gate exists only for the first Markdown proof path and must become universal.
- Complete lexicon, certified table of contents, governed domains/subjects/concepts, and definition conflict checks are not implemented.
- Atomization does not cover every scanned source. Some sources are correctly skipped or admitted as references by policy, but the policy and evidence must be explicit.
- YAML/JSON structured sources need source spans, adapter contracts, and gate coverage.
- PDF/DOCX/reference originals are not semantically atomized until dedicated adapters exist; they must remain admitted references with explicit limitations.

## Quality Claim Ladder

NOMOS product claims must map to evidence levels:

- **AQ-0 Inventory:** source scan, manifest, hashes, policy classification.
- **AQ-1 Read-only proof:** scan/feed/attest with no source mutation and no remote write.
- **AQ-2 Structural feed:** active sources produce nodes, hashes, locators, RAG metadata, traceability rows.
- **AQ-3 Source fidelity:** supported parser adapters prove active source block coverage with exact spans.
- **AQ-4 Semantic fidelity:** TOC, definitions, references, tables, annexes, domains, subjects, and atom context are certified.
- **AQ-5 Compliance-grade defensibility:** repeatable evidence pack, release gates, audit trail, validation docs, unresolved gaps explicit and blocking.

The current RBOK POC should be treated as AQ-2/AQ-3 partial, not AQ-5.

## Universal Architecture

### Layer 1: Source Registry And Policy

Responsibilities:

- classify every source by role, authority, lifecycle, license, source class, and allowed uses;
- decide whether a source is atomized, admitted as reference, derived, archive, blocked, or out of scope;
- attach format adapter expectations;
- preserve read-only guarantees.

Artifacts:

- `source-manifest.yaml`
- `source-reference-register.yaml`
- `corpus-policy-report.json`
- `license-and-rights-report.json`

### Layer 2: Parser Adapter Registry

Each adapter must output the same parser-neutral `DocumentAST`.

Required first adapters:

- `markdown-commonmark-gfm`
- `yaml-structured`
- `json-structured`
- `plain-text`

Planned adapters:

- `csv-table`
- `html`
- `docx`
- `pdf-text-layout`
- `image-reference`

Adapter contract:

- original source hash;
- parser name and version;
- root node;
- typed child nodes;
- exact source span when technically available;
- line, column, byte offsets against original bytes, not normalized-only bytes;
- text quote selector;
- unsupported block findings;
- deterministic ordering.

### Layer 3: Canonical Document Model

NOMOS must support arbitrary source depth and format-specific structures without losing meaning.

Core node families:

- root/document/front_matter/body;
- title/heading/part/chapter/section/subsection/article/clause/subclause/paragraph/alinea;
- list/list_item/table/table_row/table_cell;
- definition/glossary_term/acronym;
- requirement/rule/constraint/exception/permission/prohibition/obligation;
- workflow/module/question/decision/formula;
- callout/note/warning/code_block/example/case_study/testimony;
- annex/appendix/bibliography/citation/cross_reference/link/image/figure;
- unknown_block/unsupported_block with explicit finding.

Profiles map these generic families to corpus-specific names. Example: RBOK maps modules/questions to runtime bindings; legal profiles map articles/clauses; game profiles map rules/effects/exceptions.

### Layer 4: Semantic Enrichment

NOMOS must derive and validate:

- certified table of contents;
- parent chain for every node;
- domain, subject, concept, and source layer;
- controlled lexicon;
- definitions and acronym registry;
- synonyms and aliases;
- unresolved terms;
- conflicting definitions;
- cross-reference graph;
- annex/reference graph;
- semantic role per atom.

### Layer 5: Atomization Engine

Atomization must be granular but context-safe.

Every atom must carry:

- atom ID;
- source ID/path/hash;
- exact source span and text quote;
- canonical reference;
- parent chain;
- TOC path;
- semantic role;
- domain/subject/concept;
- related terms and definitions;
- cross-references;
- lifecycle status;
- confidence and review state;
- traceability links to RAG chunks and tests.

Atom boundaries must avoid isolated fragments that lose legal/business meaning. A fine-grained atom can exist only if its context chain is complete.

### Layer 6: Outputs

Required outputs:

- lawbook feed;
- corpus index;
- table of contents;
- lexicon;
- semantic graph;
- atom set;
- traceability matrix;
- RAG metadata;
- attestation;
- lockfile;
- structure fidelity report;
- semantic fidelity report;
- compliance evidence pack.

### Layer 7: Gates

Release gates must fail when:

- an active supported source block is uncovered;
- an atom lacks source span/hash/text quote;
- a top-level locator falls back to ordinal where exact span exists;
- a table/list/code/callout/image/link/reference is silently flattened;
- an unsupported block lacks an explicit finding;
- a cross-reference target is unresolved without review state;
- a definition conflict is unreported;
- TOC source and NOMOS tree diverge;
- RAG chunk lacks atom/source/traceability provenance;
- source Git status changes before/after feed;
- output is written inside a protected corpus;
- CI has write permissions against the source corpus.

## Implementation Epics And Issues

### Epic NUF-1: Universal Parser Contract

**Outcome:** Every adapter emits a validated `DocumentAST`.

Issues:

- NUF-1.1 Define canonical AST contract with parser metadata, source spans, findings, and child ordering.
- NUF-1.2 Make source span offsets original-byte accurate for LF and CRLF input.
- NUF-1.3 Add adapter registry with format detection and explicit unsupported-format result.
- NUF-1.4 Add adapter conformance tests shared by Markdown, YAML, JSON, and future adapters.

Acceptance:

- contract rejects nodes without source identity, span selector, hash, or unsupported findings;
- CRLF fixtures prove byte offsets match original file bytes;
- unsupported formats are reported, not silently skipped.

### Epic NUF-2: Markdown Full Fidelity Adapter

**Outcome:** Markdown/GFM source structure is represented without silent flattening.

Issues:

- NUF-2.1 Cover H1-H6 and arbitrary heading depth mapping.
- NUF-2.2 Type tables, rows, cells, lists, nested list items, callouts, blockquotes, code blocks, raw HTML, images, and links.
- NUF-2.3 Detect annexes, references, bibliography-like sections, and cross-reference candidates.
- NUF-2.4 Produce a Markdown adapter coverage report.

Acceptance:

- Markdown fixture with all block families passes full source coverage;
- every typed block has a corresponding NOMOS node or explicit unsupported finding;
- source spans include exact line, column, byte, and quote selector.

### Epic NUF-3: YAML/JSON Structured Adapters

**Outcome:** Structured files are first-class corpora, not profile hacks.

Issues:

- NUF-3.1 Parse YAML with node-level line/column metadata from `yaml.Node`.
- NUF-3.2 Parse JSON with object path, field spans, and value spans.
- NUF-3.3 Map structured paths to semantic nodes through profile rules.
- NUF-3.4 Add gate coverage for structured adapters.

Acceptance:

- parcours YAML and AI behavior JSON nodes carry exact spans;
- structured adapters report object paths such as `/parcours/modules/0/objectives/0/questions/0`;
- fidelity gate checks structured sources, not only Markdown.

### Epic NUF-4: Certified TOC And Hierarchy

**Outcome:** NOMOS can prove it understood and reproduced the document hierarchy.

Issues:

- NUF-4.1 Generate `toc.json` from parser AST.
- NUF-4.2 Generate NOMOS tree from lawbook nodes.
- NUF-4.3 Compare source TOC to NOMOS tree and fail on divergence.
- NUF-4.4 Support profile-specific hierarchy labels without engine hard-coding.

Acceptance:

- every heading/section/article/clause appears in TOC and node tree;
- H5/H6 and deeper structured paths are preserved or explicitly mapped;
- a deliberately missing heading fails the gate.

### Epic NUF-5: Semantic Typing And Cross-References

**Outcome:** NOMOS identifies meaning-bearing structures, not only blocks.

Issues:

- NUF-5.1 Detect definitions, obligations, permissions, prohibitions, constraints, exceptions, examples, and cases.
- NUF-5.2 Extract links and cross-reference candidates.
- NUF-5.3 Resolve internal references and report unresolved references.
- NUF-5.4 Preserve annex/reference/bibliography relationships.

Acceptance:

- semantic roles appear in atoms and traceability rows;
- unresolved cross-references are findings with severity;
- no link/citation/reference is silently dropped.

### Epic NUF-6: Lexicon And Concept Governance

**Outcome:** Terms, definitions, acronyms, domains, subjects, and concepts are governed artifacts.

Issues:

- NUF-6.1 Build `lexicon.json` from explicit definitions and metadata.
- NUF-6.2 Detect duplicate/conflicting definitions.
- NUF-6.3 Detect critical terms used without definition.
- NUF-6.4 Attach domain/subject/concept metadata to nodes and atoms.

Acceptance:

- every definition has source span and owner context;
- conflicts are reported and can block AQ-4/AQ-5;
- atoms include related term IDs when terms appear in text.

### Epic NUF-7: Context-Safe Atomization Engine

**Outcome:** Atomization is granular, portable, and not context-destructive.

Issues:

- NUF-7.1 Define atom boundary rules by semantic role and profile.
- NUF-7.2 Attach parent chain, TOC path, semantic role, lexicon links, source span, and review state.
- NUF-7.3 Generate atom IDs deterministically.
- NUF-7.4 Add atom quality metrics: over-fragmentation, under-fragmentation, orphan atoms, missing context.

Acceptance:

- every atom has full provenance and context;
- no atom exists without parent chain and source span;
- law/regulation/game-rule fixtures produce coherent atom sets.

### Epic NUF-8: Blocking Fidelity Gates

**Outcome:** Release gate proves source-to-NOMOS fidelity and blocks unsupported claims.

Issues:

- NUF-8.1 Implement structure fidelity report for all supported adapters.
- NUF-8.2 Implement semantic fidelity report.
- NUF-8.3 Make release gate evaluate AQ claim level.
- NUF-8.4 Add evidence pack output for audits and PR review.

Acceptance:

- active supported source loss fails release;
- unsupported/admitted reference sources are listed with policy reason;
- claim level cannot exceed evidence level.

### Epic NUF-9: RAG And Conversation Provenance

**Outcome:** RAG chunks are derived from certified atoms and cannot break traceability.

Issues:

- NUF-9.1 Generate RAG chunks only from source-backed atoms.
- NUF-9.2 Include source span, atom ID, semantic role, TOC path, lexicon terms, and allowed use in every chunk.
- NUF-9.3 Add profile controls for response posture and allowed answer latitude.
- NUF-9.4 Add RBOK/Airbook conversation fixtures: one concise question per current step.

Acceptance:

- no RAG chunk without source-backed atom;
- AI behavior constraints are represented as governed structured atoms;
- conversation tests reject verbose multi-question output.

### Epic NUF-10: RBOK POC As Proof Corpus

**Outcome:** RBOK validates NOMOS without turning NOMOS into an RBOK-specific engine.

Issues:

- NUF-10.1 Re-run `realisons-business/01_rbok` through read-only NOMOS.
- NUF-10.2 Report all atomized, skipped, admitted, and blocked sources.
- NUF-10.3 Produce TOC, lexicon, structure fidelity, semantic fidelity, traceability, RAG, and attestation artifacts.
- NUF-10.4 Record remaining gaps as issues before claiming AQ-4/AQ-5.

Acceptance:

- source Git HEAD and status unchanged before/after;
- output written outside corpus;
- all artifacts are reproducible;
- report states exact claim level and remaining gaps.

## Work Order

1. Freeze implementation except tests already written for the current gap.
2. Review and validate this plan.
3. Convert epics NUF-1 to NUF-10 into GitHub issues.
4. Implement in order: NUF-1, NUF-2, NUF-3, NUF-4, NUF-8.
5. Re-run RBOK proof corpus.
6. Continue semantic/lexicon/atomization work: NUF-5, NUF-6, NUF-7, NUF-9.
7. Re-run RBOK proof corpus again and publish claim level.

## Immediate Next Slice After Validation

The next code slice should be small and generic:

- prefer exact `source_span.locator` over ordinal `locator` when span exists;
- add structured YAML/JSON source spans through adapter-level helpers;
- extend fidelity checked sources beyond Markdown;
- keep RBOK tests as fixtures only.

This slice must not claim full semantic fidelity. It only closes the locator/span gap for supported text adapters.

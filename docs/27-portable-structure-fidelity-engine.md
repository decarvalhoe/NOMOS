# 27 - Portable Structure Fidelity And Semantic Atomization Engine

Date: 2026-05-03

## Decision

NOMOS must not solve RBOK by hard-coding the shape of `01_rbok`.

The product target is a portable engine that can process a business reference, law book, regulation, quality standard, game rules corpus, or licensed compliance bible through the same evidence chain:

```text
source registry
  -> parser AST
  -> canonical document structure
  -> certified table of contents
  -> controlled lexicon
  -> semantic graph
  -> governed atom set
  -> traceability matrix
  -> RAG projections
  -> provenance envelope
  -> fidelity certificate and release gate
```

RBOK remains the first proof corpus. It must validate the generic capability, not become the generic capability.

## Method

The retained method is evidence-first:

1. Keep a versioned external source register in `docs/research/portable-structure-fidelity-sources.yaml`.
2. Validate that register with `specs/source-reference-register.cue`.
3. Prefer official standards, official specifications, official documentation, and peer-reviewed papers.
4. Treat paid standards as acquisition requirements, not as content to copy into NOMOS.
5. Map every source to a concrete NOMOS design use.
6. Convert every unimplemented reference requirement into an issue or an explicit non-goal.
7. Use RBOK, a legal/regulatory fixture, and a game-rules fixture as portability gates.

The source register is itself a controlled artifact. A source without URL, authority, license status, and NOMOS use is not acceptable evidence.

## Source Baseline

The baseline deliberately combines legal document standards, Markdown/document AST practices, terminology standards, semantic validation, provenance, and RAG/document AI research.

| Area | Baseline sources | NOMOS use |
|---|---|---|
| Legal/document structure | Akoma Ntoso, LegalRuleML, TEI, USLM | Hierarchy, metadata, legal/rule semantics, annexes, citations, schema governance. |
| Markdown and document AST | CommonMark, mdast, Pandoc, markdown-it | Loss-aware block/inline parsing, source maps, tables, definition lists, footnotes, links, images, code. |
| Lexicon and terminology | SKOS, OntoLex-Lemon, ISO 704, ISO 25964 | Controlled vocabulary, definitions, synonyms, concept schemes, mapping quality. |
| Provenance and anchors | PROV-O, Web Annotation, in-toto, SLSA provenance | Entity/activity/agent lineage, source selectors, attestation, reproducible evidence. |
| Semantic validation | SHACL, JSON Schema | Graph/data validation, gate reports, artifact contracts. |
| Document AI and RAG | DocLayNet, Docling, GROBID, LayoutLMv3, RAG, RAPTOR, GraphRAG | Future PDF/layout adapters and traceable retrieval projections. |

## Product Requirements

### 1. Parser AST

Every parser adapter must emit a loss-aware AST with:

- node type;
- source span with line, column, and byte offsets when available;
- raw text or normalized text hash;
- block and inline structure;
- parser name/version;
- unsupported-node findings instead of silent drops.

Markdown must support at minimum:

- headings H1-H6;
- paragraphs;
- lists and nested list items;
- tables;
- block quotes and callouts;
- code blocks;
- thematic breaks;
- link definitions;
- inline links and references;
- images;
- raw HTML or unsupported blocks;
- front matter or governance tables.

### 2. Canonical Structure Tree

The structure tree must be profile-neutral. Required node roles:

- `document`;
- `front_matter`;
- `governance_header`;
- `body`;
- `back_matter`;
- `annex`;
- `division`;
- `chapter`;
- `section`;
- `subsection`;
- `article`;
- `clause`;
- `paragraph_container`;
- `alinea`;
- `enumerated_item`;
- `definition`;
- `table`;
- `table_row`;
- `table_cell`;
- `note`;
- `example`;
- `instruction`;
- `cross_reference`;
- `citation`;
- `image`;
- `code_block`;
- `unknown_block`.

Every structure node must have:

- stable `structural_id`;
- parent id;
- ordinal path;
- source span;
- source hash;
- text hash;
- role;
- title path;
- profile classification;
- confidence;
- review state.

### 3. Certified TOC

NOMOS must produce a machine table of contents from the structure tree, not from a duplicate rendering pass.

Each TOC entry must include:

- `toc_id`;
- structural node id;
- title;
- depth;
- ordinal path;
- title path;
- source span;
- child count;
- atom count;
- skipped/unsupported child count.

Release gates must fail if an active atom cannot be traced to a TOC path.

### 4. Controlled Lexicon

NOMOS must build a lexicon projection inspired by SKOS/OntoLex:

- concept id;
- preferred label;
- alternate labels;
- acronyms;
- definitions;
- source spans for definitions;
- broader/narrower/related concepts;
- domain and subject;
- term occurrences;
- unresolved terms;
- conflicting definitions;
- review state.

Terms may be extracted from explicit glossary sections, definition tables, heading patterns, YAML business fields, and reviewed semantic annotations. Inferred terms must be marked `candidate`, not `approved`.

### 5. Semantic Context

Atoms must carry their context, not just text:

- document id;
- source id/hash/span;
- full title path;
- TOC id;
- parent chain;
- domain;
- subject;
- semantic role;
- governing metadata;
- linked lexicon concepts;
- cross references;
- confidence;
- review state;
- split/merge rationale when applicable.

Semantic roles must include:

- `definition`;
- `principle`;
- `requirement`;
- `rule`;
- `obligation`;
- `permission`;
- `prohibition`;
- `condition`;
- `exception`;
- `procedure`;
- `control`;
- `formula`;
- `example`;
- `evidence`;
- `template`;
- `operational_binding`;
- `unknown`.

### 6. Atomization Rule

The unit of truth is the finest relevant atom.

Split when two statements can have separate status, review owner, exception, date, test, product impact, or cross-reference.

Do not split when the fragment loses its subject, condition, modality, exception, or parent context.

Every split/merge that depends on interpretation must produce a `needs_review` finding.

### 7. RAG Projection

Chunks are projections of atoms or structure nodes. A chunk is never the authority.

Each chunk must list:

- atom ids;
- structure ids;
- source spans;
- title path;
- lexicon concept ids;
- retrieval scope;
- chunking strategy/version;
- derivation activity id.

Hierarchical retrieval and graph retrieval are allowed only if every summary or relation can be traced back to atom ids and source spans.

### 8. Fidelity Gate

The gate must compare:

```text
source snapshot -> parser AST -> structure tree -> TOC -> atoms -> matrix -> chunks -> certificate
```

Blocking failures:

- active source block disappeared;
- unsupported block not recorded;
- table/list/callout/code/image/annex dropped silently;
- atom without source span;
- atom without TOC path;
- atom without parent chain;
- duplicate locator;
- broken cross-reference;
- critical term without lexicon status;
- chunk without atom/structure provenance;
- non-deterministic run for the same source hash and profile.

Warnings:

- low-confidence semantic role;
- candidate term requiring review;
- inferred cross-reference;
- licensed source not acquired;
- binary source admitted only as reference.

## Architecture

### Package Boundaries

| Package | Responsibility |
|---|---|
| `cli/internal/corpus/source` | Source snapshot, source ids, hashes, source registry. |
| `cli/internal/corpus/parse` | Parser-neutral AST model and adapter interfaces. |
| `cli/internal/corpus/parse/markdown` | CommonMark/Pandoc/mdast-compatible Markdown adapter. |
| `cli/internal/corpus/structure` | Canonical structure tree and TOC builder. |
| `cli/internal/corpus/lexicon` | SKOS/OntoLex-inspired lexicon model and extraction candidates. |
| `cli/internal/corpus/semantics` | Semantic role classification and profile rules. |
| `cli/internal/corpus/atomize` | Contextual atom builder with split/merge rationale. |
| `cli/internal/corpus/fidelity` | Source-to-output comparison and gate report. |
| `cli/internal/corpus/provenance` | PROV/Web Annotation/in-toto/SLSA-inspired evidence envelope. |

Existing RBOK-specific code should become a profile that consumes these packages.

### CLI Targets

```text
nomos corpus parse --profile <profile> --root <path> --out ast.json
nomos corpus structure --profile <profile> --ast ast.json --out structure.json
nomos corpus toc --structure structure.json --out toc.json
nomos corpus lexicon --profile <profile> --structure structure.json --out lexicon.json
nomos corpus atomize --profile <profile> --structure structure.json --lexicon lexicon.json --out atoms.json
nomos corpus fidelity --profile <profile> --root <path> --artifacts <dir> --out fidelity-report.json
nomos corpus certify --profile <profile> --artifacts <dir> --out certificate.json
```

The current `nomos corpus feed --profile rbok-lawbook` should remain supported and internally use the portable pipeline once the generic engine is ready.

## Dependency Tree And Issue List

GitHub epic: RBOKproject/NOMOS#220.

```text
EPIC NOMOS-FID (#220)
├─ FID-001 (#221) Source register and research method dossier
├─ FID-002 (#222) Canonical AST and source span contract
│  └─ FID-003 (#223) Markdown adapter with CommonMark/Pandoc/mdast coverage
├─ FID-004 (#224) Canonical structure tree and certified TOC
│  ├─ FID-005 (#225) Structured block support: tables, lists, callouts, code, images, annexes
│  └─ FID-006 (#226) Cross-reference and citation graph
├─ FID-007 (#227) Controlled lexicon and terminology governance
│  └─ FID-008 (#228) Semantic role classifier and contextual atom model
├─ FID-009 (#229) Portable atomization engine and matrix projection
│  ├─ FID-010 (#230) RAG projection derived from atoms and structure
│  └─ FID-011 (#231) Provenance and evidence envelope
├─ FID-012 (#232) Fidelity gate and certification report
└─ FID-013 (#233) Portability validation packs: RBOK, legal/regulation, game-rules
```

### FID-001 - Source Register And Research Method Dossier

Outcome:

- `docs/research/portable-structure-fidelity-sources.yaml`;
- `specs/source-reference-register.cue`;
- this plan document;
- invalid source register fixture proving validation fails.

Exit criteria:

- `cue vet specs/source-reference-register.cue docs/research/portable-structure-fidelity-sources.yaml` passes;
- invalid fixture fails validation;
- every retained external source has a concrete NOMOS use.

### FID-002 - Canonical AST And Source Span Contract

Outcome:

- parser-neutral AST schema;
- source span model with line/column/byte/text selectors;
- unsupported-node finding model;
- deterministic IDs.

Depends on: FID-001.

### FID-003 - Markdown Adapter

Outcome:

- CommonMark-compatible Markdown adapter;
- table/list/link/image/code/callout coverage;
- source spans preserved;
- golden fixtures from RBOK plus synthetic legal/game documents.

Depends on: FID-002.

### FID-004 - Structure Tree And Certified TOC

Outcome:

- profile-neutral structure tree;
- title path and ordinal path;
- H1-H6 mapping;
- TOC export and gate checks.

Depends on: FID-002, FID-003.

### FID-005 - Structured Block Support

Outcome:

- tables, lists, callouts, code blocks, images, annexes, and unknown blocks are first-class structure nodes;
- no silent drops.

Depends on: FID-004.

### FID-006 - Cross-Reference And Citation Graph

Outcome:

- links, anchors, citations, intra-document references, and unresolved references are represented explicitly.

Depends on: FID-004.

### FID-007 - Controlled Lexicon

Outcome:

- SKOS/OntoLex-inspired lexicon artifact;
- definitions, synonyms, acronyms, broader/narrower/related concepts;
- unresolved/conflicting terms report.

Depends on: FID-004.

### FID-008 - Semantic Role Classifier

Outcome:

- semantic roles;
- confidence and review states;
- profile-level role hints for RBOK/legal/game corpora.

Depends on: FID-004, FID-007.

### FID-009 - Portable Atomization Engine

Outcome:

- atoms with context, source spans, title path, lexicon concepts, role, rationale;
- matrix projection aligned to the atomization spine.

Depends on: FID-008.

### FID-010 - RAG Projection

Outcome:

- chunk projection derived from atoms/structure only;
- graph/hierarchical retrieval metadata without authority leakage.

Depends on: FID-009.

### FID-011 - Provenance Envelope

Outcome:

- PROV/Web Annotation/in-toto/SLSA-inspired evidence;
- extraction activity records;
- selectors and attestation subjects.

Depends on: FID-009.

### FID-012 - Fidelity Gate

Outcome:

- source AST to output reconciliation;
- blocking/warning report;
- deterministic rerun check;
- CI gate.

Depends on: FID-005, FID-006, FID-010, FID-011.

### FID-013 - Portability Validation Packs

Outcome:

- RBOK real corpus validation;
- legal/regulatory fixture validation;
- game-rules fixture validation;
- documented gaps for binary/PDF/licensed corpus handling.

Depends on: FID-012.

## First Implementation Batch

The first batch is intentionally generic:

1. Commit FID-001 source register, method, and validation schema.
2. Open the full issue tree in GitHub.
3. Start FID-002 with tests for exact source span and unsupported block preservation.
4. Start FID-003 only after the AST contract is stable.

This prevents RBOK-specific logic from leaking into the foundation.

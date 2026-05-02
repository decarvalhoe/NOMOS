# 26 - Structure-Aware Atomization Engine And Certification Process

Date: 2026-05-02

## Purpose

Nomos must not atomize a business corpus by merely splitting Markdown on headings, paragraphs, or bullet lists.

For RBOK, `01_rbok` is intended to act as the living business-law reference for Realisons. The extraction process must therefore understand document structure before it creates atoms. A paragraph/alinea count is not a quality metric by itself. The quality question is whether every meaningful unit has:

- a stable structural parent;
- a precise source span;
- a source hash;
- a functional role;
- a governed metadata context;
- a review state;
- an evidence path proving that nothing important was dropped, invented, or silently reinterpreted.

This document defines the target process and the implementation backlog for a reliable and certifiable atomization engine.

## Nomos Product Frame

Nomos exists to transform sources of authority into product evidence. Its promise is not "make embeddings from documents". Its promise is:

```text
source of authority
  -> source registry
  -> document structure
  -> relevant atomic unit
  -> canonical reference
  -> traceability matrix row
  -> canonical contract or justified non-structurable status
  -> schema/read-model/core/API/UI/tests where applicable
  -> RAG chunk derived from the same authority chain
  -> evidence report and release gate
```

Every downstream artifact must share the same authority chain. If an atom, matrix row, contract, chunk, test, or UI citation cannot be traced back to a registered source span and hash, it is not compliant with the Nomos method.

This creates one non-negotiable rule: chunks are never the canonical source of truth. A chunk is a retrieval projection derived from a source-backed structure node or atomic unit. A chunk can help explain and cite. It cannot silently create, merge, split, override, or reinterpret product law.

## Coherence Contract

The atomization engine must maintain a single semantic spine across all generated artifacts.

| Layer | Identifier | Required link |
|---|---|---|
| Source manifest | `source_id` | Repository/path/hash/version/status/owner. |
| Parser AST | `ast_node_id` | Source span and parser adapter. |
| Structure tree | `structural_id` | Parent chain, role, ordinal path, source span. |
| Atomic unit | `atom_id` | Structural node, source hash, text hash, role, review state. |
| Canonical reference | `canonical_ref` | Human and machine citation target. |
| Matrix row | `matrix_row_id` or `unit_id` | Atom, source refs, product layers, gaps and decisions. |
| Contract/read-model | `contract_id` or `object_id` | Matrix unit and schema evidence. |
| Chunk | `chunk_id` | Atom IDs or structural node IDs, source hash, retrieval scope. |
| Evidence | `evidence_id` | Validation run, actor/tool, timestamp, result, deviations. |

IDs may be optimized for different consumers, but they must reconcile deterministically. Nomos must be able to answer:

- which source span created this atom?
- which atoms are represented by this matrix row?
- which chunks expose this atom to RAG?
- which product surfaces consume this atom?
- which tests prove the expected behavior?
- which evidence run certified or blocked the current state?

## Granularity Rule

Nomos targets the finest relevant granularity, not the smallest possible text fragment.

An atom is too large if it contains multiple independent rules, exceptions, definitions, obligations, conditions, or catalog entries that could change independently.

An atom is too small if it loses the condition, exception, subject, modality, or structural context required to understand and test it.

The default decision rule is:

1. Preserve the document structure first.
2. Identify the smallest source-backed assertion, instruction, definition, exception, formula, catalog entry, or decision.
3. Keep the parent structural context as metadata, not duplicated prose.
4. Allow a multi-atom chunk only when it is marked as a retrieval context chunk and lists every `atom_id` it contains.
5. Require review for low-confidence splits, merged rules, generated content, tables, or inferred semantics.

## External Reference Baseline

The engine must align with established document and legal-markup practices rather than inventing an isolated model.

| Reference | What Nomos should take from it |
|---|---|
| OASIS Akoma Ntoso / LegalDocML | Legal documents need explicit machine-readable structure, metadata, identifiers, and compliance levels. Nomos should use an AKN-inspired structural model, not necessarily AKN XML as the storage format. |
| OASIS LegalRuleML | Normative content needs rule-aware semantics: obligations, permissions, prohibitions, conditions, temporal scope, norm classification, and authorial tracking. Nomos should produce rule candidates and review states, not pretend every text block is already executable law. |
| CommonMark | Markdown parsing should first identify block structure, then inline structure. Nomos should use a real block parser/AST contract instead of regex-only line scanning. |
| TEI Guidelines | Long-form texts need front matter, body, back matter, divisions, headings, paragraphs, lists, notes, and metadata. Nomos should represent these document roles explicitly. |
| USLM | Legislative schemas need versioned schema governance, approved/proposed branches, root-level schema versioning, and controlled changes to the document model. Nomos atomization schemas must be versioned and migration-managed. |
| Pandoc AST | Practical corpus ingestion needs metadata, tables, definition lists, footnotes, enhanced ordered lists, code blocks, and Markdown extensions. Nomos should preserve these structures before semantic classification. |
| W3C PROV-O | Evidence must model source entities, extraction activities, generated artifacts, and agents/tools. Nomos atomization certificates must use PROV-like lineage. |
| W3C Web Annotation | Source spans and annotations should be represented as targetable resources/selectors, not only as copied text. Nomos needs precise anchors into the source document. |

Primary URLs:

- OASIS Akoma Ntoso Version 1.0: https://www.oasis-open.org/standard/akn-v1-0/
- OASIS LegalDocumentML TC: https://www.oasis-open.org/committees/legaldocml/
- OASIS LegalRuleML Core Specification V1.0: https://www.oasis-open.org/standard/legalruleml-core-specification-version-1-0-oasis-standard/
- CommonMark Spec: https://spec.commonmark.org/0.30/
- TEI Guidelines: https://tei-c.org/release/doc/tei-p5-doc/en/html/index.html
- USLM Schema: https://github.com/usgpo/uslm
- Pandoc: https://pandoc.org/
- W3C PROV-O: https://www.w3.org/TR/prov-o/
- W3C Web Annotation Data Model: https://www.w3.org/TR/annotation-model/

## Target Pipeline

```text
source snapshot
  -> parser adapter
  -> source AST
  -> document structure tree
  -> corpus profile classification
  -> semantic block graph
  -> atomic unit set
  -> reference and matrix projection
  -> chunk projection
  -> provenance envelope
  -> verification report
  -> atomization certificate
```

### 1. Source Snapshot

Inputs are read-only. The source repository must not be modified.

Each input file must record:

- repository;
- branch/ref;
- commit;
- path;
- source hash;
- file class;
- parser adapter;
- parser version;
- profile version;
- generated timestamp;
- command and actor/tool.

### 2. Parser Adapter

The parser adapter produces a loss-aware source AST.

Markdown input must be parsed through a real block parser contract. The current regex-only extractor is acceptable only as a temporary compatibility layer.

Required AST node families:

- document;
- heading;
- paragraph;
- list;
- list item;
- table;
- definition list;
- block quote;
- code block;
- thematic break;
- footnote/reference;
- inline link/reference;
- raw HTML or unsupported block;
- blank/separator block.

Unsupported blocks must become explicit `unknown_block` or `unsupported_block` findings. They must not disappear.

### 3. Document Structure Tree

The structure tree maps raw AST nodes into business/legal document roles.

Required structural roles:

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
- `note`;
- `example`;
- `instruction`;
- `cross_reference`;
- `citation`;
- `unknown_block`.

Each node must have:

- `structural_id`;
- `parent_id`;
- `ordinal_path`;
- `source_span`;
- `source_hash`;
- `text_hash`;
- `role`;
- `status`;
- `confidence`;
- `review_state`.

### 4. RBOK Profile Classification

The RBOK lawbook profile must classify sections by function, not only by heading level.

Examples:

- `00_meta/**` usually contains governance, schema, procedure, and generation protocol content.
- `01_referentiel/**` contains core business reference law.
- `02_domaines/**` contains domain-specific reference material.
- `03_parcours/**` contains parcours definitions and generated operational material; not all generated content should be treated as canonical law without review.
- `98_schemas/**` and archive paths need explicit archival or historical status.

Functional roles:

- `governance_metadata`;
- `definition`;
- `principle`;
- `requirement`;
- `procedure`;
- `rule`;
- `control`;
- `example`;
- `template`;
- `generated_operational_content`;
- `archive`;
- `unknown`.

### 4.a RBOK Engine Pilot Profile

RBOK Engine issue RBOKproject/RBOK#2168 is the pilot product contract for this engine.

The pilot corpus is `RBOKproject/realisons-business/01_rbok`. Nomos must produce an importable, read-only, source-preserving projection for the RBOK Engine doctrine model:

```text
01_rbok source snapshot
  -> RBOK markdown/source AST
  -> lawbook structure tree
  -> atoms and references
  -> canonical matrix rows
  -> RBOK Engine import projection
  -> builder references
  -> runtime IA citations and chunks
```

Minimum RBOK Engine alignment:

- support the intended hierarchy from #2168: `chapter -> section -> subsection -> article -> paragraph -> alinea`;
- preserve governance metadata already present in the corpus;
- classify generated or operational documents differently from canonical doctrine;
- emit `display_ref`, `canonical_ref`, `depth`, `ordinal_path`, `source_path`, `source_hash`, `status`, `priority`, and review state for every structural and atomic node;
- make the Builder and runtime IA consume references that resolve back to the same atom/matrix/chunk chain;
- never mutate `realisons-business`; all artifacts are emitted outside the corpus.

### 4.b Generalized Profiles

The same engine must support several families of authoritative corpora.

| Profile | Structural focus | Atomic focus | Product projection |
|---|---|---|---|
| RBOK/business doctrine | Governance headers, reference chapters, procedures, domain notes, generated operational content. | Principles, definitions, requirements, procedures, controls, examples, templates, generated-content findings. | RBOK Engine nodes, Builder refs, runtime IA citations, business tests. |
| Law/regulation | Jurisdiction, legal instrument, book/title/part/chapter/section/article/paragraph/alinea, amendments, effective dates. | Definitions, obligations, permissions, prohibitions, conditions, exceptions, sanctions, transitional provisions. | Legal/reference matrix, compliance controls, applicability tests, citation-grade RAG. |
| Game rules such as Knight & Wizard | Rule domains, catalogs, legacy rulebooks, skills/classes/races/spells/assets, exceptions, resolution procedures. | Mechanics, catalog entries, exceptions, formulas, glossary terms, ambiguity rows. | YAML catalogs, Zod/Pydantic schemas, rules-core, UI surfaces, golden play cases, vector citations. |

Knight & Wizard is a useful stress test because it forces Nomos to distinguish rule text, catalog truth, legacy behavior, playable mechanics, UI display, and RAG explanation. A K&W atom is compliant only when it can trace through the full chain `source -> canonical matrix -> catalog/schema or rules-core -> API/UI/tests -> chunk metadata`.

### 5. Atomic Unit Set

Atomic units are not merely visual paragraphs. They are the smallest governed source-backed assertions or instructions that can be reviewed, cited, and tested.

Minimum atom fields:

```yaml
atom_id: stable id
source_id: source manifest id
source_path: source path
source_hash: sha256
source_span:
  start_line: number
  end_line: number
  start_offset: number
  end_offset: number
structural_path: document/chapter/section/article/paragraph/alinea
role: rule | definition | principle | procedure | governance_metadata | example | unknown
text: original atom text
normalized_text: normalized but source-preserving text
modality: obligation | permission | prohibition | recommendation | definition | fact | unknown
subject: optional extracted actor
action: optional extracted action
object: optional extracted object
conditions: []
exceptions: []
references: []
metadata_context: {}
confidence: low | medium | high
review_state: draft | needs_review | reviewed | approved | rejected
```

Important rule: semantic fields can be generated as candidates, but they do not become product law until reviewed or verified by a gate appropriate to the risk level.

### 5.a Reference And Matrix Projection

Atomization is incomplete until every governed atom has a canonical reference and a matrix projection.

Required reference fields:

```yaml
reference:
  canonical_ref: stable machine reference
  display_ref: human citation label
  source_locator: line/page/section/selector
  parent_ref: parent structural reference
  aliases: []
  supersedes: []
  superseded_by: []
```

Required matrix linkage:

```yaml
matrix:
  unit_id: same authority unit as atom_id or explicit mapping
  status: covered | partial | missing | not_applicable | deprecated
  source_refs: []
  contract_refs: []
  schema_refs: []
  db_refs: []
  vector_refs: []
  core_refs: []
  api_refs: []
  ui_refs: []
  test_refs: []
  decision_refs: []
  gaps: []
```

An atom may map to a `not_structurable` matrix status when it is explanatory context, an example, or a non-product note. That status is valid only when justified. It must still have source refs and chunk metadata if indexed.

### 5.b Chunk Projection

Chunks are generated after structure and atoms exist.

Minimum chunk fields:

```yaml
chunk_id: stable id
chunk_type: atom | structure_context | table | example | governance | retrieval_context
source_id: source manifest id
source_hash: sha256
source_span: {}
structural_ids: []
atom_ids: []
matrix_unit_ids: []
canonical_refs: []
text: source-preserving chunk text
context_before: optional parent heading/path
context_after: optional continuation context
embedding_model: model id or not_generated
chunking_strategy: profile/version
review_state: draft | needs_review | reviewed | approved | rejected
```

Hard rules:

- no chunk without `source_id`, `source_hash`, source span, and at least one structural or atom reference;
- no atom-critical chunk without matrix linkage;
- no hidden merge of unrelated atoms into one authoritative chunk;
- no dropped text without an explicit exclusion finding;
- no RAG answer may claim authority from a chunk if its linked atom or matrix row is blocked, deprecated, or unreviewed beyond the allowed risk level.

### 6. Provenance Envelope

Every atom and report must preserve:

- source entity;
- extraction activity;
- software agent;
- generated artifact;
- derivation relation;
- validation activity;
- reviewer or explicit absence of review.

This is PROV-O aligned in concept. Nomos does not need to emit RDF first, but the JSON model must be mappable to PROV-O.

### 7. Verification And Certification

Atomization certification is an internal evidence gate. It is not a legal certification.

The certification report must include:

- source inventory;
- parser inventory;
- document profile version;
- schema version;
- source text coverage;
- AST coverage;
- structural coverage;
- atom coverage;
- governance metadata coverage;
- unknown block count;
- unsupported block count;
- orphan node count;
- duplicate ID count;
- unstable ID count compared to previous run;
- cross-reference resolution rate;
- review coverage by criticality;
- matrix linkage coverage;
- chunk linkage coverage;
- open deviations;
- waivers;
- public claim boundary.

Hard failure conditions:

- source repository mutated;
- any non-ignored file has no parser result;
- any non-whitespace source block is dropped without an exclusion reason;
- any atom lacks source hash, source span, parent, text hash, or review state;
- any matrix-critical atom lacks a matrix row or explicit non-applicability reason;
- any chunk lacks source hash, source span, and atom/structure linkage;
- any critical atom is `unknown` without a `needs_review` finding;
- atom IDs are unstable for unchanged source spans;
- generated artifacts are empty or dummy;
- public claim exceeds certificate level.

## Atomization Quality Levels

| Level | Name | Meaning | Claim allowed |
|---|---|---|---|
| AQ-0 | Broken | Parser, schema, or source guard fails. | No atomization claim. |
| AQ-1 | Parsed | Source files parse into AST with no dummy artifacts. | "Parsed corpus draft." |
| AQ-2 | Structured | AST is mapped to a document structure tree with source spans and parent chains. | "Structure-aware corpus draft." |
| AQ-3 | Atomized | Atomic units exist with provenance, coverage metrics, and stable IDs. | "Atomized corpus candidate." |
| AQ-4 | Reviewed | Critical units are reviewed; unknowns/deviations are governed. | "Reviewed atomized corpus." |
| AQ-5 | Certified Internal Baseline | Certificate, review record, drift report, and release gate are approved. | "Certified internal baseline", not external certification. |

RBOK must not be used as Realisons product law until at least `AQ-4` for critical units and `AQ-5` for release baselines.

## Required CLI Surface

Target commands:

```text
nomos atomize parse --profile rbok-lawbook --root <corpus> --out ast.json
nomos atomize structure --profile rbok-lawbook --ast ast.json --out structure.json
nomos atomize units --profile rbok-lawbook --structure structure.json --out atoms.json
nomos atomize references --profile rbok-lawbook --atoms atoms.json --out references.json
nomos atomize matrix --profile rbok-lawbook --atoms atoms.json --out canonical-matrix.yaml
nomos atomize chunks --profile rbok-lawbook --atoms atoms.json --out chunks.json
nomos atomize validate --profile rbok-lawbook --atoms atoms.json --source <corpus>
nomos atomize certify --profile rbok-lawbook --atoms atoms.json --out certificate.json
nomos atomize diff --previous certificate.json --current certificate.json
```

Existing `corpus feed` should consume certified atoms once the atomization engine reaches `AQ-3+`.

## Process

1. Define the RBOK atomization profile and schemas.
2. Select a gold corpus subset:
   - 3 governance/meta documents;
   - 3 core reference documents;
   - 3 domain documents;
   - 3 parcours/generated documents;
   - 2 archive/schema documents.
3. Manually annotate expected structure and atoms for the gold subset.
4. Implement parser adapter and structure tree.
5. Implement deterministic atomization.
6. Emit coverage and provenance reports.
7. Add challenge cases for:
   - front matter tables;
   - nested headings;
   - numbered and bullet lists;
   - definition tables;
   - generated parcours;
   - unknown or malformed blocks;
   - cross-references;
   - metadata changes.
8. Add canonical reference, matrix, and chunk projections.
9. Add CI gates.
10. Add review workflow and certification report.
11. Feed certified atoms into the RBOK lawbook feed and Praxis evidence contract.

## Implementation Backlog

Parent epic:

- RBOKproject/NOMOS#160 - Structure-aware evidence spine for Nomos.

Child issues:

1. RBOKproject/NOMOS#161 - Semantic spine schemas for structures, atoms, refs, matrix, chunks and certificates.
2. RBOKproject/NOMOS#162 - Loss-aware Markdown block AST adapter.
3. RBOKproject/NOMOS#163 - RBOK Engine profile from RBOKproject/RBOK#2168.
4. RBOKproject/NOMOS#164 - Gold corpus annotations for RBOK, law/regulation and K&W-style rules.
5. RBOKproject/NOMOS#165 - Deterministic atoms with stable IDs, source spans, hashes and review states.
6. RBOKproject/NOMOS#166 - Canonical references and traceability matrix projection from atoms.
7. RBOKproject/NOMOS#167 - RAG chunk projection from atoms and structure with safety gates.
8. RBOKproject/NOMOS#168 - Governance metadata extraction and canonical/generated/archive classification.
9. RBOKproject/NOMOS#169 - Verification gates and atomization certificate.
10. RBOKproject/NOMOS#170 - Public `nomos atomize` CLI surface.
11. RBOKproject/NOMOS#171 - RBOK lawbook feed and RBOK Engine import projection from certified atoms.
12. RBOKproject/NOMOS#172 - Generalized profiles for law/regulation and K&W-style game rules.
13. RBOKproject/NOMOS#173 - Praxis evidence mapping after the Nomos contract stabilizes.

## Current Interim Fix Boundary

The current extractor can be patched so every prose paragraph emits an atomic alinea. That is useful as a guard against dropping prose, but it is not enough.

The real target is:

```text
block parser
  -> structure-aware document tree
  -> functional role classifier
  -> source-span-preserving atoms
  -> certification evidence
```

Until that exists, Nomos must describe RBOK output as `atomized corpus candidate`, not certified product law.

# Fidelity Engine — Research Method

## Objective

Design a Markdown fidelity engine that parses, represents, and roundtrips
structured documents without semantic loss. The engine must preserve:

- Block structure (headings, paragraphs, lists, tables, code blocks)
- Inline formatting (emphasis, links, code spans)
- Source positions (line/column for every node)
- Content hashes (deterministic per-node)
- Parent-child relationships
- Metadata (YAML frontmatter, attributes)

## Research Questions

1. **RQ1**: What is the minimal AST representation that preserves all
   CommonMark + GFM structure without information loss?
2. **RQ2**: How should source spans be tracked to support diff-level
   impact analysis on Markdown corpus changes?
3. **RQ3**: What roundtrip guarantees are achievable (byte-identical,
   semantically equivalent, or structure-preserving)?
4. **RQ4**: How do existing parsers (goldmark, tree-sitter, pandoc)
   differ in their handling of ambiguous or underspecified Markdown?

## Source Selection Criteria

Sources are registered in `source-register.yaml` based on:

| Criterion | Threshold |
|-----------|-----------|
| Normative authority | Must be the canonical specification or a widely-adopted reference implementation |
| Relevance to fidelity | Must inform parse structure, AST design, or roundtrip behavior |
| Availability | Must be publicly accessible (no licensed PDFs required) |
| Stability | Must have versioned releases or stable URLs |

## Method

### Phase 1: Specification Analysis

1. Read CommonMark spec cover-to-cover; extract block/inline grammar rules.
2. Map each grammar rule to an AST node type (cross-reference with mdast).
3. Identify GFM extensions that require additional node types.
4. Document ambiguities and underspecified behaviors.

### Phase 2: Parser Evaluation

1. Construct a test corpus of edge-case Markdown documents.
2. Parse with goldmark, tree-sitter-markdown, and pandoc.
3. Compare ASTs: identify divergences in structure, positions, and metadata.
4. Score each parser on: correctness, position fidelity, extensibility, Go compatibility.

### Phase 3: AST Design

1. Define the Nomos Markdown AST (extending mdast where needed).
2. Add source positions, content hashes, and parent chain to every node.
3. Design the roundtrip contract: input → AST → output with defined equivalence.
4. Implement golden tests: known inputs with expected AST snapshots.

### Phase 4: Integration

1. Wire the fidelity parser into the atomization pipeline.
2. Validate that atom extraction preserves source spans.
3. Verify that corpus diff → impact correctly identifies changed spans.
4. Gate: all golden tests pass, roundtrip loss is zero for supported constructs.

## Quality Gates

- **G1**: 100% of CommonMark spec examples parse without error.
- **G2**: Source positions are byte-accurate for all block-level nodes.
- **G3**: Roundtrip produces semantically equivalent output (normalized whitespace).
- **G4**: Content hashes are deterministic and stable across parser versions.
- **G5**: Gold annotations (from `testdata/gold/`) match expected atoms.

## Deliverables

| Artifact | Format | Location |
|----------|--------|----------|
| Source register | YAML | `docs/fidelity/source-register.yaml` |
| Research method | Markdown | `docs/fidelity/research-method.md` |
| AST schema | CUE | `specs/fidelity-ast.cue` (future) |
| Parser implementation | Go | `cli/internal/fidelity/` (future) |
| Golden test corpus | JSON + MD | `cli/internal/fidelity/testdata/` (future) |
| Evaluation report | Markdown | `docs/fidelity/parser-evaluation.md` (future) |

## Constraints

- No licensed references required (all sources are public).
- Parser must be pure Go (no CGO dependency for the core path).
- AST must be serializable to JSON for pipeline interop.
- Source hashes use SHA-256 exclusively.

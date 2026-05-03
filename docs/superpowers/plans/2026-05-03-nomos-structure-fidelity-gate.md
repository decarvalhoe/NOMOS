# Nomos Structure Fidelity Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the existing NOMOS portable atomization seed into a release-blocking structure fidelity gate for RBOK and future legal/regulatory/game-rule corpora.

**Architecture:** Reuse the generic `cli/internal/atomization` Markdown AST as the parser spine, project its source spans and block types into `cli/internal/corpus` lawbook feeds, then add a profile-level fidelity report that compares source blocks to generated NOMOS nodes. RBOK remains the proof profile; the parser and gate stay portable.

**Tech Stack:** Go CLI, JSON artifacts, existing `corpus` profile feed, existing `atomization` Markdown AST, GitHub Actions RBOK lawbook workflows.

---

## Current State

- `codex/nomos-rbok-poc-chain` delivers the real RBOK lawbook profile POC.
- `codex/rbok-poc-structured-runtime-feed` adds YAML parcours and AI behavior JSON atomization.
- `codex/nomos-portable-fidelity-engine` adds the portable model and source register, but is not wired into the RBOK feed.
- The current corpus feed still uses `ExtractMarkdown` in `cli/internal/corpus/rbok_md_extractor.go`, which is regex-oriented and only represents H1-H4 plus paragraph/alinea nodes.

## File Structure

- Modify `cli/internal/corpus/rbok_lawbook_types.go`: add source span and richer document node types.
- Modify `cli/internal/corpus/rbok_md_extractor.go`: delegate Markdown extraction to the AST parser and preserve block spans.
- Modify `cli/internal/corpus/rbok_node_normalize.go`: keep span metadata during normalization.
- Modify `cli/internal/corpus/profile.go`: add `structure_fidelity_report` output and compute source-block coverage.
- Modify `cli/internal/corpus/release_gate.go`: fail the RBOK gate when a fidelity report exists and is blocking.
- Modify `cli/internal/atomization/md_ast.go`: add missing portable Markdown block families when needed.
- Add or modify tests under `cli/internal/corpus/*_test.go` and `cli/internal/atomization/*_test.go`.

## Task 1: Markdown AST Coverage Delta

- [ ] **Step 1: Write failing tests for portable block coverage**

Add tests in `cli/internal/atomization/md_ast_test.go` proving the parser recognizes H5/H6, blockquotes/callouts, raw HTML, image-only paragraphs, and links without dropping source bytes.

Run:

```powershell
cd C:\Dev\nomos-viability-audit\cli
go test ./internal/atomization -run "TestParseMarkdownPortable|TestParseMarkdownH5H6" -count=1
```

Expected before implementation: FAIL for missing block types or properties.

- [ ] **Step 2: Implement minimal parser support**

Add block types and detection in `cli/internal/atomization/md_ast.go` without replacing the existing AST contract.

- [ ] **Step 3: Verify parser package**

Run:

```powershell
cd C:\Dev\nomos-viability-audit\cli
go test ./internal/atomization -count=1
```

Expected: PASS.

## Task 2: Corpus Feed Uses Source-Span AST

- [ ] **Step 1: Write failing corpus tests**

Add tests in `cli/internal/corpus/rbok_md_extractor_test.go` proving:

- H5/H6 headings become dedicated node types;
- nodes expose `source_span` with line, column, and byte range;
- table/code/callout/image/link blocks are represented as typed NOMOS nodes;
- locators use source line anchors instead of only ordinal anchors.

Run:

```powershell
cd C:\Dev\nomos-viability-audit\cli
go test ./internal/corpus -run "TestExtractMarkdown_SourceSpans|TestExtractMarkdown_H5H6|TestExtractMarkdown_TypedBlocks" -count=1
```

Expected before implementation: FAIL.

- [ ] **Step 2: Extend `LawbookNode` contract**

Add `SourceSpan` fields and portable node types while keeping existing JSON compatibility.

- [ ] **Step 3: Rebuild `ExtractMarkdown` on AST blocks**

Project AST blocks into lawbook nodes with parent chain, ordinal path, source span, source hash placeholder, title path metadata, and block-kind metadata.

- [ ] **Step 4: Verify corpus package**

Run:

```powershell
cd C:\Dev\nomos-viability-audit\cli
go test ./internal/corpus -count=1
```

Expected: PASS.

## Task 3: Structure Fidelity Report And Gate

- [ ] **Step 1: Write failing report tests**

Add tests proving the report fails when a non-blank source block is not represented by any node span, when any node lacks span/hash, or when unsupported blocks appear without findings.

Run:

```powershell
cd C:\Dev\nomos-viability-audit\cli
go test ./internal/corpus -run "TestStructureFidelity" -count=1
```

Expected before implementation: FAIL.

- [ ] **Step 2: Implement report model**

Add a `StructureFidelityReport` with source count, checked source count, block count, covered block count, unsupported block count, missing span count, findings, and blocking count.

- [ ] **Step 3: Expose report in profile feed**

Add output flag `structure_fidelity_report` and include it in `rbok-lawbook` default outputs.

- [ ] **Step 4: Wire release gate**

Make RBOK release gate fail when the report is present and `blocking > 0`.

- [ ] **Step 5: Verify gate**

Run:

```powershell
cd C:\Dev\nomos-viability-audit\cli
go test ./internal/corpus ./internal/app -count=1
```

Expected: PASS.

## Task 4: RBOK Real POC Proof

- [ ] **Step 1: Build CLI**

Run:

```powershell
cd C:\Dev\nomos-viability-audit\cli
go build -o ..\bin\nomos.exe .
```

- [ ] **Step 2: Run read-only RBOK POC**

Run the existing RBOK lawbook scan/feed/split/attest/release-gate sequence against `C:\Dev\realisons-business\01_rbok`.

Expected: release gate must fail if the fidelity report detects blocking losses; otherwise pass with a non-empty `structure_fidelity_report`.

- [ ] **Step 3: Publish evidence**

Update the PR body/comment and issues `#207` and `#220` with node counts, source counts, warning list, fidelity report summary, and workflow links.

## Task 5: Remaining Gaps After First Gate

- [ ] **Step 1: Create follow-up issue list if any blocking remains**

Any unsupported PDF/DOCX/layout/semantic-lexicon point must become a concrete issue with acceptance criteria. Do not claim AQ-5 while those gaps remain.

- [ ] **Step 2: State product claim boundary**

Update docs to say whether the current output is AQ-2, AQ-3, AQ-4, or AQ-5. The claim must match evidence.

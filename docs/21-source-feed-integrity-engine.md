# 21 - Source-To-Feed Integrity Engine

This document defines the source-to-feed integrity method NOMOS uses to
prove that the artifacts it produces (feed units, RAG metadata,
attestations) are a faithful, non-duplicated, non-noisy derivation of
the original source corpus.

It is the operator and reviewer reference for epic `#337` (SFI) and is
the single entry point referenced by the strict release gate output,
the public claim boundary, and the regulated documentation set.

This file is owned by SFI-10 (`#348`). The pilot verification on the
RBOK corpus is owned separately by SFI-11 (`#349`); claims about that
specific verification live in `docs/rbok-poc-validation-dossier.md`,
not here.

## 1. Purpose And Claim Boundary

The source-to-feed integrity engine answers, by construction and by
machine check, a single question:

> Are the feed units and RAG metadata that NOMOS hands to downstream
> consumers a faithful derivation of the original source corpus?

"Faithful" is defined narrowly:

- every byte of every active source file is accounted for by a typed
  segment in a per-source ledger;
- every canonical semantic atom in the feed is derived from exactly
  one segment in that ledger;
- every RAG chunk traces back to a canonical-atom segment;
- the integrity report is a present, machine-checkable artifact whose
  pass/fail status gates the strict release.

This is an **internal** contract NOMOS imposes on itself. It is not a
certification, accreditation, or attestation against any external
standard. NOMOS does not claim FDA, EU GMP, GxP, ISO, IEC, NIST, or
any other regulatory approval. The reference baseline in section 10
exists so the engine speaks the same vocabulary as those frameworks,
not to assert compliance with them.

The full vocabulary of permitted and forbidden public claims lives in
[`docs/public-claim-boundary.md`](public-claim-boundary.md). Section
11 of this document restates the post-SFI claim levels, but
`public-claim-boundary.md` remains authoritative.

## 2. Source Segment Ledger

The ledger entry is `corpus.SourceSegment` (see
`cli/internal/corpus/source_segment.go`). Per source file the engine
emits a flat slice of segments that together cover every byte of the
file without overlap at the root level.

Each segment carries:

| Field | Purpose |
|---|---|
| `segment_id` | deterministic id encoding source, byte range, and kind. |
| `source_id`, `source_path` | which source the segment belongs to. |
| `kind` | typed-scanner classification (`heading`, `paragraph`, `list_item`, `table_cell`, `callout`, `code_block`, `metadata`, `decorative_separator`, `blank`, …). |
| `disposition` | role downstream (see section 3). |
| `start_byte` / `end_byte` | half-open byte interval, RFC-5147-aligned. |
| `start_line` / `end_line` / columns | 1-indexed line/column span for human review. |
| `raw_text_hash` | sha256 over the exact byte slice; mandatory for `canonical_atom`. |
| `normalized_text_hash` | sha256 over a whitespace-normalized form; mandatory for `canonical_atom`. |
| `parent_segment_id` | non-empty when a segment is a child (e.g. a list item under a list). |
| `canonical_unit_id` | optional back-pointer to a downstream feed unit. |
| `include_in_feed`, `include_in_rag` | derived booleans the gate uses to validate downstream membership. |
| `unsupported_reason` | mandatory when `disposition == "unsupported_blocking"`. |

The CUE projection of this struct lives in
[`specs/source-segment-ledger.cue`](../specs/source-segment-ledger.cue);
the validating fixture is
[`specs/examples/source-segment-ledger.valid.yaml`](../specs/examples/source-segment-ledger.valid.yaml).

The typed Markdown scanner that emits the ledger is
`cli/internal/corpus/markdown_scanner.go`. It is line-oriented, has
zero third-party dependencies, and is deterministic on byte input.

## 3. Dispositions

`disposition` is the contract that decides where a segment is allowed
to appear downstream. The five values come from
`corpus.Disposition`:

| Disposition | Meaning | Hash required? | Enters feed? | Enters RAG? |
|---|---|---|---|---|
| `canonical_atom` | active semantic content (paragraph, list item, callout body, table cell). | yes — both `raw_text_hash` and `normalized_text_hash`. | yes | yes |
| `structure_only` | heading line, table envelope, blockquote envelope, list envelope — describes structure but carries no semantic body of its own. | optional. | no | no |
| `coverage_only` | blank line, decorative separator, layout-only fragment. Exists so the per-source byte coverage can prove "every byte was seen". | optional. | no | no |
| `excluded_by_policy` | front matter, metadata block, or any segment intentionally suppressed by an explicit policy decision. | optional. | no | no |
| `unsupported_blocking` | a recognised block kind for which the engine has no typed handler yet. Carries a non-empty `unsupported_reason`. | optional. | no | no — AND blocks the SFI-04 source-integrity gate (the build cannot pass while it is present). |

The CUE schema enforces the conditional invariants on the
`canonical_atom` and `unsupported_blocking` rows; `cue vet` rejects a
ledger that violates them.

## 4. Why Every Byte Is Tracked But Not Every Fragment Enters Feed/RAG

A common shortcut in document-processing systems is to extract only
"interesting" fragments and silently drop the rest. NOMOS does not
allow that, because silent drops cannot be distinguished from a
parser bug.

The discipline is:

1. The typed scanner emits one segment per byte range, with a typed
   `kind` and a `disposition` that says "I belong in the feed",
   "I'm just structure", "I'm whitespace", "I'm metadata you asked me
   to ignore", or "I'm an active block I do not yet understand".
2. The SFI-04 source-integrity gate
   (`cli/internal/corpus/source_integrity_gate.go`) walks the ledger
   and surfaces any byte range not claimed by a segment as a
   `SOURCE_UNCOVERED_RANGE` finding. There is no "drop and forget"
   path.
3. Only `canonical_atom` segments are eligible to back a feed unit
   or a RAG chunk. The other dispositions are explicitly recorded
   and explicitly do not flow downstream.

The combination — typed dispositions plus an enforced coverage proof —
is what makes "we tracked everything" a checkable claim instead of an
assertion.

## 5. Parent / Child Duplication Prevention

Markdown is a tree. The naive way to extract feed units — "make each
heading a unit and put everything under it inside that unit's body" —
double-counts every paragraph: once as part of the heading's body,
once again if the engine ever emits paragraphs as their own atoms.

SFI-03 (`#341`, in `cli/internal/corpus/extract_md.go`) splits the
extraction tree into two non-overlapping kinds of records:

- **structural heading entries** (`Kind == "heading"`) carry no
  descendant body bytes. Their `Content` is empty. They exist for
  TOC / navigation context only.
- **canonical semantic leaves** (`paragraph`, `list_item`, `callout`)
  own their own bytes exactly once. They carry the full enclosing
  heading path as ancestry metadata so RAG and feed consumers can
  reconstruct context without owning the bytes twice.

The invariant is **the same source byte span cannot create multiple
canonical semantic atoms**. Tests in
`cli/internal/corpus/extract_md_sfi03_test.go` enforce it pairwise
across every emitted record.

H5 and H6 are typed as headings, not folded silently into the parent
paragraph. This is part of the same invariant: a heading line cannot
be a semantic atom and a structural container at once.

## 6. Source Spans And Hashes For Integrity Proof

Every segment carries an exact half-open byte range. Spans are
RFC 5147-style (`text/plain` byte ranges); they are not approximate
and they do not depend on whitespace normalisation.

Each `canonical_atom` segment also carries two sha256 hashes:

- `raw_text_hash` is computed over the exact bytes in
  `[start_byte, end_byte)`. Any change in input bytes — including
  whitespace — flips the hash.
- `normalized_text_hash` is computed over a deterministic
  normalisation: trim trailing whitespace per line, collapse runs of
  whitespace inside a line to one space, strip leading/trailing
  blank lines. Two semantically equivalent fragments that differ
  only in incidental whitespace produce the same normalized hash.

Both hashes are mandatory for `canonical_atom` and the schema rejects
their absence. Together they let an auditor verify, after the fact,
that a feed unit was built from a specific source byte range and that
its semantic content has not been silently rewritten.

`parent_segment_id` lets containers (lists, blockquotes, tables) link
to their leaves without claiming the leaves' bytes for themselves;
combined with the disposition-based eligibility rules in section 3,
it keeps the ledger a tree without the duplication risk of section 5.

## 7. How To Read The Integrity Report

The SFI-04 source-integrity gate produces
`corpus.IntegrityReport`:

```json
{
  "status": "fail",
  "source_count": 1,
  "segment_count": 12,
  "semantic_segment_count": 4,
  "uncovered_ranges": [
    {"source_id": "DOC-A", "start_byte": 84, "end_byte": 96}
  ],
  "duplicate_semantic_ranges": [],
  "junk_semantic_segments": ["seg:DOC-A:120-123:paragraph"],
  "unsupported_blocking_segments": [],
  "findings": [
    {
      "code": "SOURCE_UNCOVERED_RANGE",
      "source_id": "DOC-A",
      "start_byte": 84,
      "end_byte": 96,
      "message": "uncovered source range with non-whitespace bytes"
    },
    {
      "code": "SOURCE_JUNK_SEMANTIC_ATOM",
      "segment_id": "seg:DOC-A:120-123:paragraph",
      "source_id": "DOC-A",
      "start_byte": 120,
      "end_byte": 123,
      "message": "canonical_atom contains only whitespace, punctuation, or layout markers"
    }
  ]
}
```

Status is `pass` iff `findings` is empty; otherwise the report fails
closed.

The stable finding codes (a public, versioned contract) are:

| Code | Source | Means |
|---|---|---|
| `SOURCE_UNCOVERED_RANGE` | SFI-04 | a span of source bytes was not claimed by any segment. |
| `SOURCE_DUPLICATE_SEMANTIC_SPAN` | SFI-04 | two `canonical_atom` segments cover the same span. |
| `SOURCE_JUNK_SEMANTIC_ATOM` | SFI-04 | a `canonical_atom` contains only whitespace, punctuation, or table separators. |
| `SOURCE_UNSUPPORTED_BLOCKING` | SFI-04 | an `unsupported_blocking` segment is present; the build cannot pass until it is classified. |
| `SOURCE_SEGMENT_INVALID_RANGE` | SFI-04 | a segment has start > end on byte / line / column. |
| `SOURCE_SEGMENT_MISSING_HASH` | SFI-04 | a `canonical_atom` is missing `raw_text_hash` or `normalized_text_hash`. |
| `FEED_UNIT_NO_SEGMENT` | SFI-07 | a source-derived feed unit declares no `source_segment_id`. |
| `FEED_UNIT_NO_SOURCE` | SFI-07 | a source-derived feed unit declares no `source_id`. |
| `FEED_UNIT_NO_SPAN` | SFI-07 | a source-derived feed unit has no byte/line span. |
| `FEED_EMPTY_TEXT` | SFI-07 | a feed unit's `business_rule` is empty after trimming. |
| `FEED_JUNK_TEXT` | SFI-07 | a feed unit's text is whitespace, punctuation, or layout fragments only. |
| `FEED_DUPLICATE_SPAN` | SFI-07 | two feed units share the same source span. |
| `RAG_CHUNK_NO_UNIT` | SFI-06 | a RAG chunk has no `unit_ids`. |
| `RAG_CHUNK_NO_SEGMENT` | SFI-06 | a RAG chunk has no `source_segment_id` or it does not resolve. |
| `RAG_CHUNK_NON_SEMANTIC_SOURCE` | SFI-06 | a RAG chunk's resolved segment is not a `canonical_atom`. |
| `RAG_CHUNK_UNSUPPORTED_BLOCKING` | SFI-06 | a RAG chunk's source segment is `unsupported_blocking`. |
| `RAG_CHUNK_EMPTY_TEXT` | SFI-06 | a RAG chunk's content is empty after trimming. |
| `RAG_CHUNK_MISSING_SPAN` | SFI-06 | a RAG chunk has no byte/line span. |

The CUE schema for the report is
[`specs/corpus-integrity-report.cue`](../specs/corpus-integrity-report.cue);
a passing fixture is
[`specs/examples/corpus-integrity-report.valid.yaml`](../specs/examples/corpus-integrity-report.valid.yaml).

## 8. Strict Gate Evidence Section

SFI-08 (`#346`) wires the integrity check into the strict release
gate (`cli/internal/app/strict_gate.go`). When any
`--corpus-integrity-*` flag is supplied, the gate adds a top-level
`corpus_integrity_check` section to its JSON output:

```json
"corpus_integrity_check": {
  "status": "pass",
  "source_integrity": { "status": "pass", "source_count": 1, "segment_count": 12, "...": "..." },
  "feed_quality":     { "status": "pass", "feed_unit_count": 4, "source_derived_unit_count": 4, "...": "..." },
  "summary": "source_integrity=pass (0 findings); feed_quality=pass (0 findings)"
}
```

The flags that drive the section are:

- `--corpus-integrity-report=PATH` — load a precomputed
  `IntegrityReport` (or aggregate `{source_integrity, feed_quality}`)
  JSON file.
- `--corpus-integrity-source=DIR` — recompute the source-integrity
  report on the fly by walking `DIR/*.md` through the typed scanner.
- `--corpus-integrity-feed=PATH` — combine with
  `--corpus-integrity-source` to also compute the SFI-07 feed
  quality report against a `[]FeedUnit` JSON file.
- `--corpus-integrity-rag=PATH` — combine with
  `--corpus-integrity-source` to feed `[]ChunkMetadata` into the
  same feed-quality computation.

The three section-level statuses are:

| `status` | Meaning | Strict gate effect |
|---|---|---|
| `pass` | every supplied sub-report passed. | gate not blocked by this section. |
| `fail` | at least one supplied sub-report failed, or a load/parse error occurred. | the strict gate flips `valid: false` and the CLI exits non-zero. |
| `not_provided` | no `--corpus-integrity-*` flag was supplied. | the section is omitted entirely; existing gate behaviour is unchanged. |

The `not_provided` status appears in the public CUE schema
(`#CorpusIntegrityCheck`) so consumers can distinguish "we did not
ask the engine" from "we asked and it passed".

A typical FAIL excerpt:

```json
"corpus_integrity_check": {
  "status": "fail",
  "source_integrity": {
    "status": "fail",
    "findings": [
      {"code": "SOURCE_UNCOVERED_RANGE", "source_id": "DOC-A", "start_byte": 1024, "end_byte": 1080, "message": "uncovered source range with non-whitespace bytes"}
    ]
  },
  "feed_quality":  null,
  "summary": "source_integrity=fail (1 findings)"
}
```

The shape `corpus_integrity_check` is
[CUE-checkable](../specs/corpus-integrity-report.cue) via
`#CorpusIntegrityCheck` and is intentionally additive: builds that do
not opt in keep the wire format they had before SFI-08.

## 9. Operator Review Procedure

When the strict gate fails on a `corpus_integrity_check` finding, the
operator (engineering or QA) follows this procedure:

1. **Read the finding codes.** Each code in the table from section 7
   maps to a specific rule. `SOURCE_*` codes describe the ledger
   itself; `FEED_*` codes describe feed units; `RAG_CHUNK_*` codes
   describe RAG metadata. The code is the public, stable contract —
   downstream dashboards, CI, and the regulated dossier all key off
   it.
2. **Inspect the offending artifact by id and span.** Each finding
   carries the `segment_id`, `unit_id`, or `chunk_id` plus the
   exact `start_byte`/`end_byte` (and, for SOURCE findings, the
   line/column on the ledger entry). Open the source file at the
   reported byte range; confirm what the gate saw.
3. **Decide the correction.** Three options, in order of preference:
   - **Fix the source corpus.** A `SOURCE_UNCOVERED_RANGE` on a
     non-whitespace span often means an active block (an HTML
     fragment, a custom directive) was not yet typed by the
     scanner. The right fix is usually to either rephrase the source
     into a typed kind, or to file an issue extending the typed
     scanner. A `SOURCE_DUPLICATE_SEMANTIC_SPAN` typically means a
     profile is emitting the same paragraph twice; trace the
     duplicate `segment_id` back to the extractor.
   - **Fix the profile.** `FEED_UNIT_NO_SEGMENT`,
     `FEED_UNIT_NO_SPAN`, and `RAG_CHUNK_NO_SEGMENT` indicate that
     the profile is producing artifacts not backed by ledger
     entries. The profile is the bug, not the corpus.
   - **Accept the gap.** If the gap is a known limitation
     (e.g. a licensed reference whose body cannot be redistributed),
     it must be **documented as an explicit accepted gap** in the
     POC validation dossier. Do not silence the finding by
     downgrading the disposition; do not patch the gate to ignore
     the code.
4. **Document each accepted gap.** Accepted gaps live in the
   POC validation dossier maintained by SFI-11 (`#349`); see
   `docs/rbok-poc-validation-dossier.md` and the SFI-11 dossier
   appendix for the format. Each entry must record the finding
   code, the affected source id and byte range, the reviewer, the
   reason, and the expiry condition. An accepted gap with no expiry
   condition is a hidden rule and is forbidden by section 7 of
   `docs/07-tests-gates-release.md` (Politique De Dérogation).
5. **Re-run the gate.** Either flip the corpus / profile fix, or
   land the documented gap, then re-run `nomos strict
   --corpus-integrity-*`. The expected end state is `status: pass`
   with the dossier capturing the non-zero count of accepted gaps.

The gate does not propose fixes automatically. It surfaces facts;
the operator owns the decision.

## 10. References

The references are grouped per the parent-epic spec. They are cited
as inspiration and shared vocabulary. NOMOS does not certify against
any of them.

### Regulatory And Quality

- FDA Data Integrity And Compliance With Drug CGMP — guidance on
  ALCOA+ data-integrity expectations.
- 21 CFR Part 11 — electronic records and electronic signatures.
- FDA Computer Software Assurance (CSA) — risk-based assurance for
  production and quality-system software.
- EU GMP EudraLex Volume 4 Annex 11 — computerised systems in EU
  GMP.
- ISPE GAMP 5 — risk-based approach to GxP computerised systems.
- ISO 15489-1 — records management.
- ISO/IEC 25010:2023 — product quality model.
- NIST SP 800-218 — secure software development framework.

### Lawbook And Document Modeling

- OASIS Akoma Ntoso — XML vocabulary for legal documents.
- OASIS LegalRuleML — rule modelling for legal text.
- TEI P5 — guidelines for text encoding and interchange.
- LexML / LexML Brasil — applied lawbook modelling using Akoma Ntoso.

### Parser, Source Span, And Provenance

- RFC 5147 — URI fragments for `text/plain` byte/line addressing
  (used as the model for our half-open `[start_byte, end_byte)`
  spans).
- W3C Web Annotation Data Model — provenance-friendly anchoring of
  text fragments.
- W3C PROV — provenance data model and the vocabulary NOMOS' span
  + hash + parent linkage maps onto.
- OpenLineage — open standard for job/dataset lineage events.

### Terminology

- W3C SKOS — simple knowledge organisation system. Cited as
  inspiration only; NOMOS does not impose a SKOS thesaurus on
  customer corpora.

### RAG And Data Quality

- Lewis et al., "Retrieval-Augmented Generation for
  Knowledge-Intensive NLP Tasks" (2020) — the originating RAG paper.
- Ragas — metrics for retrieval-augmented generation
  (faithfulness, context relevance, answer relevance).
- RAPTOR — recursive abstractive processing for retrieval.
- GraphRAG — graph-augmented retrieval.
- Great Expectations / Soda / Deequ — data-quality test
  frameworks. Cited as inspiration for the gate-as-test pattern,
  not adopted as dependencies.
- W3C SHACL — RDF shape constraints. Cited as a comparable
  schema-constraint contract.

The reference list is informational. None of these standards are
embedded as dependencies; none are claimed to be implemented; none
are redistributed beyond the citation above.

## 11. What NOMOS Can Claim After The SFI Epic

After SFI-04 / SFI-07 / SFI-08 are merged and the corpus integrity
gate runs in CI, NOMOS may claim the following — and only the
following — at the platform level:

- **Source-to-feed coverage is machine-checked.** Every byte of a
  source file is either claimed by a typed segment or surfaced as a
  finding in the integrity report.
- **Feed atoms are derived from canonical source segments only.**
  Source-derived feed units carry the originating `segment_id`,
  byte/line span, and `normalized_text_hash`; matrix-derived units
  are skipped by the gate by design.
- **RAG metadata is source-backed and gate-checked.** Every chunk
  resolves to a `canonical_atom` segment; the SFI-06 build-time
  rejection rules and the SFI-07 after-the-fact gate share the same
  finding codes.
- **The strict release gate fails closed when corpus-integrity
  inputs are present and failing.** A build that supplies
  `--corpus-integrity-*` flags and produces a `fail` sub-report
  cannot pass `nomos strict`.

The matching `public-claim-boundary.md` levels are
`source-integrity-proven` (SFI-04 + SFI-07) and
`full-fidelity-proven` (also SFI-08 wired into the blocking strict
release gate for the build under review).

NOMOS still **cannot** claim, after this epic:

- external certification, accreditation, or attestation against any
  standard listed in section 10;
- full lawbook automation — the engine processes Markdown corpora
  with a typed-atom set; PDFs, DOCX, image-only sources, and
  arbitrary HTML are out of scope for the engine itself (separate
  intake pipelines may produce typed Markdown and feed it back in);
- semantic-equivalence proofs — `normalized_text_hash` proves
  byte-level parity modulo whitespace, not that two paraphrases
  carry the same meaning;
- correctness of downstream LLM answers — the engine guarantees
  source-backing and absence of junk, not retrieval quality, model
  faithfulness, or answer relevance.

These boundaries are restated in the reserved-phrases list of
`docs/public-claim-boundary.md`.

## 12. Pilot vs Hardcoded Scope

The engine is generic. No part of `corpus.SourceSegment`,
`corpus.IntegrityReport`, or
`cli/internal/corpus/markdown_scanner.go` is RBOK-specific. The
typed kinds, dispositions, finding codes, and CUE schemas were
chosen so the same gate runs unmodified against any source corpus —
a lawbook, a regulation, a technical standard, a business doctrine,
or a structured game-rule book.

RBOK (`realisons-business/01_rbok`) is the **first pilot proof
corpus**, not the product scope. It is the corpus on which the
engine is being verified end-to-end as part of SFI-11 (`#349`); the
verification dossier for that POC lives in
`docs/rbok-poc-validation-dossier.md`. Conclusions specific to RBOK
must stay in that dossier and must not be lifted into platform-wide
claims.

If a future corpus exposes a typed kind the scanner does not yet
emit, the right answer is to extend the scanner and the typed-kind
list, not to special-case the corpus inside the gate.

Cross-references:

- claim language and reserved phrases:
  [`docs/public-claim-boundary.md`](public-claim-boundary.md);
- gate vocabulary and exit-code semantics:
  [`docs/07-tests-gates-release.md`](07-tests-gates-release.md);
- backlog status of every SFI child ticket:
  [`docs/15-product-backlog.md`](15-product-backlog.md);
- POC verification: `docs/rbok-poc-validation-dossier.md` (owned by
  SFI-11 `#349`);
- parent epic: `#337`.

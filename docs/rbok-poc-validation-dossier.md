# RBOK 01_rbok POC — Source-to-Feed Integrity Validation Dossier (AQ-3)

This dossier records the full source-to-feed integrity process *and*
the FSQ-01..FSQ-07 semantic feed-quality gates for the RBOK `01_rbok`
proof corpus. It captures the exact command sequence, the run record,
the before/after metrics, the residual gaps, and the **AQ-3 bounded
claim** that may be advertised on the basis of the recorded result.

History:

- **SFI-11 (#349)** opened this dossier with the source-to-feed
  integrity command sequence and a stub run record.
- **FSQ-08 (#371)** — this revision — extends the runner with the
  FSQ-01 feed audit, the FSQ-02 admission-aware manifest, the FSQ-05
  corpus body ledger, the FSQ-06 semantic quality gate, and the FSQ-07
  RAG composer. The strict gate now consumes the body ledger and the
  default SFI-06 semantic profile. The dossier converts the SFI-11
  bounded claim into the **AQ-3** bounded claim.
- **Structured universal follow-up** records the successful Windows
  run against `C:\Dev\realisons-business\01_rbok`, adds structured
  YAML/JSON atomization support, and tightens generic feed admission
  so config/reference material and low-value lead-ins do not pollute
  the curated feed.

The engine method itself is documented separately in
`docs/21-source-feed-integrity-engine.md` (SFI-10 / #348) — see that
file for the algorithmic and governance description. This dossier is
the *run* record; that file is the *engine* record. Operator review
notes specific to this run are in section 11.

A historical record of the prior `v0.1.0-ALPHA` legacy POC
(rbok-lawbook profile + strict fidelity gate, 7191 feed nodes) is
preserved at the end of this dossier under "Historical: legacy
v0.1.0-ALPHA POC record".

## 1. Purpose

What this dossier proves, and only what it proves:

> **AQ-3** evidence-backed semantic feed quality for the RBOK 01_rbok
> POC, with bounded claims and explicit limitations. Does **NOT**
> claim AQ-4/AQ-5 regulated validation.

Concretely, on the recorded run the dossier proves that:

- the source-to-feed integrity pipeline (SFI-04..SFI-08) completed
  with passing reports;
- the FSQ-01..FSQ-07 semantic feed-quality gates produced no
  blocking findings; FSQ-06 may still report reviewable warnings;
- the corpus body ledger has zero uncovered bytes for every admitted
  text source (FSQ-05);
- the corpus working tree was clean before AND after the run.

This is a POC-scoped artefact. RBOK is the first proof corpus for the
generic source-to-feed integrity + semantic-quality engine; it is not
the product scope. The AQ-3 claim is bounded by the recorded run on
the recorded corpus revision and is not lifted to a platform-wide or
regulatory claim. Section 10 lists the explicit AQ-4/AQ-5 non-claims.

## 2. Corpus reference

| Field | Value |
|---|---|
| Logical name | RBOK lawbook reference corpus (`01_rbok`) |
| Expected path | `/root/repos/realisons-business/01_rbok` |
| Repository | `RBOKproject/realisons-business` (private) |
| Last-known commit sha | recorded by the run script in `<RUN_DIR>/corpus-commit.txt` (`git -C $CORPUS_GIT_ROOT rev-parse HEAD`) |
| License / privacy | private; corpus contents must NOT be inlined into any committed artefact in this repository. Allowed metadata only: source ID, relative path, content hash, finding codes, span offsets. |

The corpus is read-only for this run. Any mutation of
`realisons-business` during the run is a hard failure (see step 0 and
step 13 in the command sequence below).

## 3. Run environment

| Field | Value at dossier time | Recorded at run time |
|---|---|---|
| Go toolchain | `go1.26.2 windows/amd64` (recorded run) | `run-environment.txt` |
| CUE toolchain | validated by `scripts/e2e.ps1` | `scripts/e2e.ps1` output |
| OS | Windows/MSYS on local Codex workstation | `run-environment.txt` |
| Nomos commit sha | recorded by the runner | `run-environment.txt :: nomos_commit` |
| Branch | `codex/fsq-semantic-rag-poc` | `run-environment.txt :: nomos_branch` |

The run script (`scripts/rbok-poc-integrity.sh`) records all four
runtime values into `<RUN_DIR>/run-environment.txt` so the dossier
and the run record can be cross-checked.

## 4. Command sequence

Every step below is intended to be executed in order by
`scripts/rbok-poc-integrity.sh`. Each step states: input artefact,
command, output artefact, expected exit code. Every artefact path is
**outside** `realisons-business`. `<RUN_DIR>` defaults to
`./reports/poc/<utc-timestamp>/`.

### Step 0 — Pre-run git status capture

- Input: `realisons-business` working tree.
- Command:
  ```bash
  git -C "$CORPUS_GIT_ROOT" status --short > "$RUN_DIR/corpus-status-before.txt"
  git -C "$CORPUS_GIT_ROOT" rev-parse HEAD  > "$RUN_DIR/corpus-commit.txt"
  ```
- Output: `corpus-status-before.txt` (must be empty), `corpus-commit.txt`.
- Expected exit code: 0. Script aborts (exit 3) on dirty tree.

### Step 1 — Source scan

- Command:
  ```bash
  nomos corpus scan --root "$CORPUS" --out "$RUN_DIR/snapshot.json" --format json
  ```
- Output: `snapshot.json`. Expected exit code: 0.

### Step 2 — Source manifest (FSQ-02 #365 admission-aware)

- Command:
  ```bash
  nomos corpus manifest \
    --snapshot "$RUN_DIR/snapshot.json" \
    --out      "$RUN_DIR/source-manifest.yaml"
  ```
- Output: `source-manifest.yaml`. FSQ-02 admission/atomization fields
  are backfilled from the extension heuristic; operator declarations
  win on subsequent edits.
- Expected exit code: 0.

### Step 3 — Source segment ledger emission

- Status: **TODO(SFI-02 / #340, SFI-08 / #346)** — no dedicated CLI
  emitter for `[]SourceSegment` JSON today. The integrity gate
  computes the ledger in-process via `corpus.ScanMarkdown` when
  `--corpus-integrity-source` is supplied; the body-ledger generator
  (step 8) embeds the per-source ledger into
  `corpus-body-ledger.json`. Step 3 is therefore implicit.

### Step 4 — Source integrity gate (SFI-04, #342)

- Command (today, via the strict-gate flag plumbing):
  ```bash
  ( cd "$NOMOS_REPO/cli" && \
    go run . strict \
      --corpus-integrity-source "$CORPUS" \
      --format json ) > "$RUN_DIR/integrity-source.json"
  ```
- Output: `integrity-source.json` carrying the SFI-04 IntegrityReport.
- Expected exit code: 0 if `Status == "pass"`, 1 otherwise.
- Reference: `cli/internal/corpus/source_integrity_gate.go::CheckSourceIntegrity`.
- The runner invokes the top-level CLI entry point (`go run . strict`).

### Step 5 — Feed generation (SFI-05, #343)

- Command:
  ```bash
  nomos corpus feed \
    --root     "$CORPUS" \
    --snapshot "$RUN_DIR/snapshot.json" \
    --manifest "$RUN_DIR/source-manifest.yaml" \
    --out      "$RUN_DIR/feed.json"
  ```
- Output: `feed.json`. SFI-05 fields populated on source-derived
  units; FSQ-05 (#368) `body_ledger_segment_ids` populated for both
  single-segment and composed table_row units.
- Expected exit code: 0.

### Step 6 — RAG metadata

- Status: the canonical FSQ-07 (#370) composer is
  `corpus.ComposeRAGChunks`; it is reachable today only as a Go
  library entry point. Until a CLI wrapper exists this run uses the
  inline `rag_metadata` array embedded in `feed.json` (SFI-06 path
  via `nomos corpus feed`).
- Command (extraction via jq):
  ```bash
  jq '.rag_metadata // []' "$RUN_DIR/feed.json" > "$RUN_DIR/rag-metadata.json"
  jq '.units        // []' "$RUN_DIR/feed.json" > "$RUN_DIR/feed-units.json"
  ```
- TODO(FSQ-08-followup): expose `ComposeRAGChunks` via a dedicated
  subcommand so the run uses the FSQ-07 context-rich chunks instead
  of the SFI-06 default.

### Step 7 — Feed quality gate (SFI-07, #345)

- Command:
  ```bash
  ( cd "$NOMOS_REPO/cli" && \
    go run . strict \
      --corpus-integrity-source "$CORPUS" \
      --corpus-integrity-feed   "$RUN_DIR/feed.json" \
      --corpus-integrity-rag    "$RUN_DIR/rag-metadata.json" \
      --format json ) > "$RUN_DIR/integrity-feed.json"
  ```
- Output: `integrity-feed.json` (source-integrity + feed-quality
  sub-reports).
- Expected exit code: 0 if both pass.
- Reference: `cli/internal/corpus/feed_quality_gate.go::CheckFeedQuality`.

### Step 8 — Corpus body ledger (FSQ-05, #368)

- Command:
  ```bash
  ( cd "$NOMOS_REPO/cli" && \
    go run ./internal/corpus/cmd/body-ledger \
      --manifest    "$RUN_DIR/source-manifest.yaml" \
      --corpus-root "$CORPUS" \
      --out         "$RUN_DIR/corpus-body-ledger.json" \
      --frozen-time "$RUN_TIMESTAMP" )
  ```
- Output: `corpus-body-ledger.json`. Markdown sources scanned via
  `ScanMarkdown`; binary/reference sources record their bytes under
  `binary_bytes` / `unsupported_bytes`.
- Expected exit code: 0.
- Reference: `cli/internal/corpus/body_ledger.go::BuildCorpusBodyLedger`,
  `cli/internal/corpus/cmd/body-ledger/main.go` (≤150 LOC,
  self-contained).

### Step 9 — Feed audit (FSQ-01, #364)

- Command:
  ```bash
  ( cd "$NOMOS_REPO/cli" && \
    go run ./internal/corpus/cmd/feed-audit \
      --feed        "$RUN_DIR/feed.json" \
      --rag         "$RUN_DIR/rag-metadata.json" \
      --corpus      "$CORPUS" \
      --out         "$RUN_DIR/feed-audit.json" \
      --frozen-time "$RUN_TIMESTAMP" )
  ```
- Output: `feed-audit.json` (length distribution, duplicate normalized
  text groups, table-cell ratio, source coverage, top offenders).
- Expected exit code: 0 (audit is measurement, not a gate).

### Step 10 — Semantic quality gate (FSQ-06, #369)

- Computed by step 11 as part of the strict gate, using the default
  `corpus.DefaultRBOKProfile()`. The semantic_quality sub-report ends
  up under `strict-gate.json :: corpus_integrity_check.semantic_quality`.
- Reference: `cli/internal/corpus/feed_quality_gate.go::CheckSemanticQuality`.

### Step 11 — Strict gate consuming the aggregate

- Command:
  ```bash
  ( cd "$NOMOS_REPO/cli" && \
    go run . strict \
      --corpus-integrity-source "$CORPUS" \
      --corpus-integrity-feed   "$RUN_DIR/feed.json" \
      --corpus-integrity-rag    "$RUN_DIR/rag-metadata.json" \
      --corpus-body-ledger      "$RUN_DIR/corpus-body-ledger.json" \
      --format json ) > "$RUN_DIR/strict-gate.json"
  ```
- Output: `strict-gate.json` containing source_integrity, feed_quality,
  semantic_quality, body_ledger_findings, aggregate_findings.
- Expected exit code: 0 if all gates pass; non-zero otherwise.
- Reference: `cli/internal/app/strict_gate.go::StrictGateCommand`.

### Step 12 — Attestation (bounded AQ-3 claim)

- Command:
  ```bash
  nomos corpus attest \
    --snapshot   "$RUN_DIR/snapshot.json" \
    --corpus-id  rbok-lawbook \
    --project-id rbok \
    > "$RUN_DIR/attestation.json"
  ```
- Output: `attestation.json` plus `attestation-claim.txt` (templated
  AQ-3 bounded-claim text). The bounded claim text is the exact
  language permitted in any release artefact citing this dossier.
- TODO(FSQ-08-followup): wire `--corpus-body-ledger` into the
  `nomos corpus attest` CLI so the predicate's `claim_coverage` block
  is populated automatically. Until then, `claim_coverage` is set in
  the predicate only when callers go through
  `corpus.GenerateCorpusAttestation` directly with `BodyLedger`
  attached. This gap is acknowledged as a known limitation in section
  7.

### Step 13 — Post-run git status capture

- Command:
  ```bash
  git -C "$CORPUS_GIT_ROOT" status --short > "$RUN_DIR/corpus-status-after.txt"
  diff -q "$RUN_DIR/corpus-status-before.txt" "$RUN_DIR/corpus-status-after.txt"
  ```
- Output: `corpus-status-after.txt` (must equal the pre-run capture).
- Expected exit code: 0. Any diff fails the POC (script exit 4) and
  writes `corpus-mutation.diff`.

## 5. Success criteria (AQ-3)

Verbatim from #371. The run is treated as passing AQ-3 only if every
criterion below holds.

- `feed-audit.json` produced; `tokens.le_2 == 0` (no canonical units
  collapse to ≤2 tokens) on the canonical-unit slice.
- Source integrity report (SFI-04): `status == "pass"`.
- Feed quality (SFI-07): `status == "pass"`.
- Semantic quality (FSQ-06): `blocking_finding_count == 0`; a
  `status=warn` report is acceptable when only reviewable warnings
  remain.
- Body ledger (FSQ-05): every source whose
  `admission_status == "admitted"` AND
  `atomization_status in {atomized, coverage_only}` has
  `byte_coverage.uncovered_bytes == 0`. Equivalently:
  `body_ledger_findings == []` in `strict-gate.json`.
- Strict gate exits 0.
- Attestation `claim_coverage.summary_status == "feed_and_body"`.
- Pre/post corpus `git status --short` are both empty.

The runner's `assert_zero` block (in `scripts/rbok-poc-integrity.sh`)
encodes these checks via `jq` against `strict-gate.json`,
`feed-audit.json`, and `attestation.json`. The runner exits non-zero
unless ALL checks pass.

## 6. Run record

| Stage | Name | Exit code | Artefact path | Status |
|---|---|---|---|---|
| 0  | Pre-run git status capture       | 0 | `corpus-status-before.txt`, `corpus-commit.txt` | **PASS** |
| 1  | Source scan                      | 0 | `snapshot.json` | **PASS** |
| 2  | Source manifest (FSQ-02)         | 0 | `source-manifest.yaml` | **PASS** |
| 3  | Source segment ledger            | n/a | embedded in `corpus-body-ledger.json` | **PASS via body ledger** |
| 4  | Source integrity gate (SFI-04)   | 0 | `integrity-source.json` | **PASS** |
| 5  | Feed generation (SFI-05)         | 0 | `feed.json` | **PASS** |
| 6  | RAG metadata                     | 0 | `rag-metadata.json`, `feed-units.json` | **PASS** |
| 7  | Feed quality gate (SFI-07)       | 0 | `integrity-feed.json` | **PASS** |
| 8  | Corpus body ledger (FSQ-05)      | 0 | `corpus-body-ledger.json` | **PASS** |
| 9  | Feed audit (FSQ-01)              | 0 | `feed-audit.json` | **PASS** |
| 10 | Semantic quality (FSQ-06)        | 0 | embedded in `strict-gate.json` | **PASS: 0 blocking findings** |
| 11 | Strict gate (SFI-08)             | 0 | `strict-gate.json` | **PASS** |
| 12 | Attestation (AQ-3 claim)         | 0 | `attestation.json`, `attestation-claim.txt` | **PASS with WARN: `claim_coverage` not yet wired by CLI** |
| 13 | Post-run git status capture      | 0 | `corpus-status-after.txt` | **PASS** |

Recorded local evidence pack:
`C:\Dev\nomos-rbok-poc-run-20260504-structured-universal-9`.
The pack is not committed because it contains private corpus-derived
artefacts. The committed dossier records only metadata, counts,
paths, finding codes, and claim boundaries.

Recorded corpus commit:
`ea003e8fe3c35993731c3708a3787df6a3a690df`.

Recorded run timestamp:
`2026-05-04T10:02:16Z`.

The corpus working tree was clean before and after the run. The AQ-3
success criteria encoded in `scripts/rbok-poc-integrity.sh` all passed:
`feed_audit.tokens.le_2=0`, source integrity `pass`, feed quality
`pass`, semantic quality `blocking_finding_count=0`, body ledger
findings `0`, and strict gate exit code `0`.

## 7. Before / After / Actual metrics

The "Before" column is the SFI-9 audit-evidence baseline used to
motivate the FSQ wave. The "After (target post-FSQ)" column is what
the FSQ-01..FSQ-07 gates are designed to enforce. The "Actual (this
run)" column is filled only from the recorded evidence pack above.

| Metric | Before (SFI-9 audit) | After (target post-FSQ) | Actual (this run) |
|---|---|---|---|
| Feed units                                | 9500   | depends on corpus           | **3024** |
| RAG chunks                                | n/a    | one source-backed chunk per curated unit where possible | **3024** |
| Sources scanned / declared                | 240    | every source classified by FSQ-02 | **240/240** |
| Source-backed feed units                  | n/a    | every curated unit source-backed | **3024/3024** |
| Source-backed RAG chunks                  | n/a    | every curated chunk source-backed | **3024/3024** |
| `table_cell` units in feed                | 3230   | **0** (FSQ-03 collapses cells into table_row units) | **0** |
| Units with ≤2 tokens                      | 3344   | **0** (caught by FSQ-06 blocking) | **0** |
| Units with ≤10 chars                      | 2195   | reduced; ≤2 tokens variant gates this | **0** |
| Duplicate-normalized-text groups (≥3)     | 3704   | **0** (FSQ-06 blocking) | **0 blocking groups** |
| Duplicate-normalized-text groups (2×)     | n/a    | warnings only | **6 warning groups / 12 units** |
| Semantic quality blocking findings        | n/a    | **0** | **0** |
| Semantic quality warnings                 | n/a    | reviewable | **6** |
| Semantic quality informational findings   | n/a    | reviewable | **349 raw/decoded YAML infos** |
| Source integrity findings                 | n/a    | **0** | **0** |
| Feed quality findings                     | n/a    | **0 blocking** | **0 blocking** |
| Body ledger uncovered bytes               | n/a    | **0** for every admitted text source (FSQ-05) | **0** |
| Body ledger total bytes                   | n/a    | measured | **10,615,465** |
| Body ledger semantic bytes                | n/a    | measured | **1,066,951** |
| Body ledger structure bytes               | n/a    | measured | **387,357** |
| Strict-gate exit code                     | n/a    | **0** | **0** |

Do **not** infer future "Actual" columns from the "After (target)"
column. The targets are design intent; actual numbers must come from
a real run. Editing the "Actual" column without a corresponding
evidence pack is forbidden.

## 8. Residual gaps

Each gap below is a known limitation of the SFI / FSQ engine as of
this dispatch. Listing them here keeps the AQ-3 claim honest.

- **Setext-style headings** (underline-style `===` / `---` for
  H1/H2) are not handled by the typed Markdown scanner. Tracked
  under SFI-02 / #340.
- **GFM `\|` escape in table cells** is not handled by the table-cell
  splitter. Cells separated by escaped pipes will be mis-counted.
  Tracked under SFI-02 / #340.
- **Pre-heading paragraphs** (canonical body text appearing before
  the first heading) are intentionally not surfaced as feed units.
  Tracked under SFI-03 / #341 judgment call.
- **HTML blocks** are surfaced as `unsupported_blocking` rather than
  parsed. Conservative by design; corpora with significant HTML will
  fail the integrity gate until the typed scanner grows HTML
  support. Tracked under SFI-02 / #340.
- **YAML raw-vs-decoded representation** remains informational:
  the structured scanner records source spans and decoded scalar
  values, but YAML escaping/block-scalar normalisation is still
  reported as `FEED_RAW_DECODED_MISMATCH` for review. It is not a
  source-integrity failure.
- **Heading-group composition** — FSQ-07 / #370 may or may not
  enable heading-group composition by default, depending on the
  shipped strategy table. Whichever is the case on `c20c5f4..6c14cbf`
  is what this run uses; section 11 reproduces the on-disk strategy
  for the recorded run.
- **No CLI emitter for `[]SourceSegment` ledger** today. The
  body-ledger generator embeds the ledger per-source. Tracked under
  SFI-02 / #340.
- **No dedicated `nomos corpus rag` subcommand** to drive
  `ComposeRAGChunks` (FSQ-07). The runner extracts RAG metadata via
  `jq` from `feed.json`, which uses the SFI-06 BuildRAGMetadata
  path. Tracked as FSQ-08-followup.
- **`nomos corpus attest` does not yet wire `--corpus-body-ledger`**
  into the predicate. `GenerateCorpusAttestation` accepts a
  `BodyLedger` field today (FSQ-05 #368) but the CLI does not
  expose it; until then the runner records this as a WARNing in the
  success-criteria block, not a hard fail. Tracked as
  FSQ-08-followup.
- **CI wiring of the strict gate** for the RBOK POC is out of scope
  for FSQ-08. Until CI runs the strict gate green on consecutive
  builds against `01_rbok`, the dossier may not advertise
  `full-fidelity-proven`. Tracked under SFI-08 / #346.

If the actual POC run reveals additional gaps, append them here
verbatim from the relevant finding codes — do not summarise them
away.

## 9. AQ-3 bounded claim

Exact language permitted in any attestation file or release note that
cites this dossier:

> *Source-to-feed integrity proven AND feed/RAG semantic quality
> gates passed for the recorded RBOK 01_rbok run on commit
> `<CORPUS-COMMIT-SHA>` against NOMOS commit `<NOMOS-SHA>`. NOT a
> claim of regulatory-grade validation, certification, or
> universal-corpus fidelity.*

The complete bounded-claim block (rendered into
`<RUN_DIR>/attestation-claim.txt` by step 12) reads:

> *AQ-3 bounded claim — RBOK 01_rbok POC, NOMOS commit `<NOMOS-SHA>`,
> corpus commit `<CORPUS-COMMIT-SHA>`, run `<RUN_TIMESTAMP>`.
>
> Source-to-feed integrity proven AND feed/RAG semantic quality
> gates passed for the recorded RBOK 01_rbok run on commit
> `<CORPUS-COMMIT-SHA>`. NOT a claim of regulatory-grade validation,
> certification, or universal-corpus fidelity.
>
> Specifically, on this run:
>   - SFI-04 source integrity gate: `status=pass`, 0 findings.
>   - SFI-07 feed quality gate:     `status=pass`, 0 findings.
>   - FSQ-06 semantic quality gate: 0 blocking findings; reviewable warnings may remain.
>   - FSQ-05 body ledger:           every admitted text source has `uncovered_bytes=0`.
>   - SFI-08 strict release gate:   exit 0.
>   - Corpus working tree clean before AND after the run.
>
> This run earns claim level `source-integrity-proven` from
> `docs/public-claim-boundary.md`. Promotion to
> `full-fidelity-proven` still requires the strict gate to be wired
> into CI for this corpus and to remain green across consecutive
> runs (gating issue: #346).*

The AQ-3 claim above is now supported for the recorded evidence pack
listed in section 6. It must not be reused for another corpus,
another corpus commit, another NOMOS build, or a broader regulated
claim without a fresh run and a fresh dossier update.

## 10. AQ-4 / AQ-5 non-claim

This dossier explicitly does **not** establish, and the AQ-3 bounded
claim explicitly does **not** authorise, any of the following:

- **AQ-4 regulated validation** — not asserted here. Customer-specific
  GxP / FDA / EU GMP / ISO validation against an intended use is the
  customer's responsibility; NOMOS is "regulated-ready", not
  "validated".
- **AQ-5 certification** — no external certification body decision is
  asserted; reserved phrases (`certified`, `compliant`, `validated`)
  remain governed by `docs/public-claim-boundary.md`.
- **Platform-wide source-to-feed fidelity across arbitrary corpora.**
  This dossier proves only the recorded RBOK POC run.
- **Universal PDF/DOCX/OCR fidelity.** Only the markdown +
  parcours-YAML sources of `01_rbok` were exercised; binary and
  reference sources (PDF) are recorded under `binary_bytes` /
  `unsupported_bytes` without semantic atomisation.
- **Production vector-store retrieval and LLM behaviour.**
  `RAG-ready` here means traceable metadata exists, not that
  retrieval ranking has been validated.
- **Redistribution rights for any licensed standard processed
  alongside the corpus.** Reference handling stays under the
  reference-bibles backlog (#192–#196).

If a future release wants to escalate any of the above, that
escalation needs its own dossier and its own evidence chain. This
dossier does not transfer.

## 11. Operator review notes

The engine method (algorithms, governance, operator review checklist)
lives in `docs/21-source-feed-integrity-engine.md` (SFI-10 / #348).
For a given run of `scripts/rbok-poc-integrity.sh`:

1. Confirm `<RUN_DIR>/corpus-status-before.txt` and
   `corpus-status-after.txt` are both empty, byte-for-byte equal.
2. Cross-check `<RUN_DIR>/run-environment.txt` against the dossier's
   section 3 (`go_version`, `nomos_commit`, `corpus_commit`).
3. Open `<RUN_DIR>/strict-gate.json` and verify
   `corpus_integrity_check.status == "pass"`,
   `semantic_quality.blocking_finding_count == 0`,
   `body_ledger_findings == []`,
   `aggregate_findings == []`.
4. Open `<RUN_DIR>/feed-audit.json` and verify
   `length_distribution.tokens.le_2 == 0`,
   `duplicate_normalized_text.group_count == 0` (or document any
   accepted residue in this section).
5. Open `<RUN_DIR>/corpus-body-ledger.json` and verify
   `coverage_summary.uncovered_bytes == 0`.
6. Read `<RUN_DIR>/attestation-claim.txt` and confirm the substituted
   `<NOMOS-SHA>` / `<CORPUS-COMMIT-SHA>` / `<RUN_TIMESTAMP>` match
   the `run-environment.txt` capture.
7. If all of the above are clean, append the run's summary table
   into section 7 ("Actual (this run)") and check off the AQ-3
   claim row in `docs/15-product-backlog.md`.

If any of these checks fail, the AQ-3 claim is **not** earned; the
runner's exit code already reflects this and the operator's job is
to record the gap in section 8 with a follow-up ticket reference.

## 12. PR #362 supersession

PR #362 (`codex/rbok-poc-restart-20260504`) was the earlier attempt
at re-running the RBOK POC. It is now superseded by the FSQ epic
(#363) and, specifically, by this PR (FSQ-08 / #371). The
orchestrator will close PR #362 as part of integration cleanup once
this dossier and runner land. This PR does **not** itself close
#362.

## 13. Engine reference

The algorithmic and governance description of the source-to-feed
integrity engine (and the FSQ semantic-quality extensions) is
documented in `docs/21-source-feed-integrity-engine.md` (SFI-10 /
#348). This dossier does not duplicate the engine specification; it
records only the *run* of the engine against the RBOK POC corpus.

## Historical: legacy v0.1.0-ALPHA POC record

The following section preserves the previous content of this file
for traceability. It records the legacy `rbok-lawbook` profile POC
(release gate + strict fidelity gate, distinct from the FSQ-04..FSQ-08
source-to-feed integrity + semantic-quality gates that are the
subject of this dossier). The legacy record is retained verbatim; it
does not extend, amend, or substitute for the AQ-3 bounded claim
above.

> Document ID: `POC-RBOK-001`
> Status: executed - alpha POC pass
> Release context: `v0.1.0-ALPHA`
> Date: 2026-05-03
> Owner: Nomos core team
>
> **Claim Boundary.** This dossier records the Nomos POC pipeline
> against the `01_rbok` corpus from `realisons-business`. It supports
> an alpha claim that Nomos can process a real RBOK lawbook corpus
> into traceable artifacts with a passing strict fidelity gate and
> read-only source verification. It does not claim production
> readiness, customer validation, regulatory certification, or
> universal fidelity across every document format and corpus shape.
>
> **POC Objective.** Demonstrate that Nomos can:
> 1. Process a real business reference corpus without mutating
>    source files.
> 2. Extract structured lawbook nodes from Markdown sources.
> 3. Preserve source spans for generated feed nodes.
> 4. Emit typed semantic nodes for tables, links, callouts, and
>    code blocks.
> 5. Generate a certified table of contents.
> 6. Generate governed lexicon, RAG metadata, runtime import,
>    governance, and attestation artifacts.
> 7. Fail the release gate if the strict fidelity gate is red.
>
> **Current Execution Summary** (legacy run, `v0.1.0-ALPHA`).
> Execution date: 2026-05-03. Corpus: read-only clone of
> `realisons-business/01_rbok`. Nomos release target: `v0.1.0-ALPHA`.
>
> | Check | Result |
> |---|---|
> | Source files scanned | 240 |
> | Markdown files detected | 69 |
> | Feed nodes generated | 7191 |
> | Nodes with spans | 7191 |
> | Certified TOC entries | 1090 |
> | Structural nodes | 1090 |
> | Table nodes | 65 |
> | Code block nodes | 25 |
> | Callout nodes | 26 |
> | Link nodes | 137 |
> | Release gate | PASS |
> | Strict fidelity gate | PASS |
> | Strict blocking findings | 0 |
> | Strict total findings | 0 |
> | Fidelity proof status | `full_fidelity_proven` |
> | Full fidelity claim allowed for this POC output | `true` |
> | Read-only source mutation check | PASS |
>
> **Pipeline Under Test (legacy).**
>
> ```text
> 01_rbok source clone
>   -> read-only fingerprint
>   -> rbok-lawbook profile diagnosis
>   -> lawbook artifact pack
>   -> certified TOC
>   -> governed lexicon
>   -> strict fidelity gate
>   -> release gate
>   -> corpus fidelity proof
>   -> artifact validation
>   -> read-only fingerprint verification
> ```
>
> **Artifacts Produced (legacy).** `diagnosis.json`,
> `artifact-pack.json`, `rbok-lawbook-feed.json`,
> `rbok-lawbook-index.json`, `rbok-rag-metadata.json`,
> `rbok-engine-import.json`, `rbok-governance.json`,
> `rbok-attestation.json`, `rbok-governed-lexicon.yaml`,
> `rbok-certified-toc.json`, `rbok-strict-fidelity-gate.json`,
> `rbok-fidelity-proof.json`.
>
> **Verdict (legacy).** *RBOK `01_rbok` POC: PASS for
> `v0.1.0-ALPHA` (legacy strict fidelity gate, not the SFI-04..SFI-08
> + FSQ-01..FSQ-07 chain). The legacy POC supports a bounded alpha
> claim that Nomos can transform the RBOK lawbook corpus into
> traceable, release-gated artefacts without mutating the source. It
> does not support a blanket claim of universal regulated-grade
> fidelity, and it does not establish the AQ-3
> `source-integrity-proven` claim level introduced by FSQ-08 / #371.*

# RBOK 01_rbok POC — Source-to-Feed Integrity Validation Dossier

This dossier records the full source-to-feed integrity process for the
RBOK `01_rbok` proof corpus. It captures the exact command sequence,
the run record, and the bounded claim that may be advertised on the
basis of the result.

This dossier is owned by SFI-11 (#349). The engine method itself is
documented separately in `docs/21-source-feed-integrity-engine.md`
(SFI-10 / #348) — see that file for the algorithmic and governance
description of the integrity engine. This dossier is the *run* record;
that file is the *engine* record.

A historical record of the prior `v0.1.0-ALPHA` legacy POC (rbok-lawbook
profile + strict fidelity gate, 7191 feed nodes) is preserved at the
end of this dossier under "Historical: legacy v0.1.0-ALPHA POC record".

## 1. Purpose

What this dossier proves, and only what it proves:

- For the recorded run on the recorded corpus revision, the
  source-to-feed integrity process completed and the integrity report
  was produced.
- Any claim made from this dossier is bounded by the actual gate
  result and the corpus state at the time of the run. The dossier
  does **not** claim platform-wide source-to-feed fidelity.

This is a POC-scoped artefact. RBOK is the first proof corpus for the
generic source-to-feed integrity engine; it is not the product scope.

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
step 10 in the command sequence below).

## 3. Run environment

| Field | Value at dossier time |
|---|---|
| Go toolchain | `go version` from `cli/` build host (`go1.22.2 linux/amd64` recorded at dossier authoring time) |
| CUE toolchain | `cue version` (≥ `v0.12.0` used at dossier authoring time) |
| OS | Linux x86_64 (orchestrator host) |
| Nomos commit sha | `git rev-parse HEAD` at the time of the run (dossier-authoring base: `399d653`, `feat(SFI-08)`) |
| Branch | `feat/sfi-11-rbok-poc-rerun` (this PR) at dossier time; the actual run on a host with the corpus must record the merged `main` sha. |

The run script (`scripts/rbok-poc-integrity.sh`) records all four of
these into `<RUN_DIR>/run-environment.txt` so the dossier and the run
record can be cross-checked.

## 4. Command sequence

Every step below is intended to be executed exactly as written, in
order, by `scripts/rbok-poc-integrity.sh`. Each step states: input
artefact, command, output artefact, expected exit code, and where the
artefact is written.

`<RUN_DIR>` is `./reports/poc/<utc-timestamp>/` by default (the
`./reports/...` convention used by the existing
`scripts/rbok-lawbook-e2e.sh` family). Every artefact path is
**outside** `realisons-business`.

### Step 0 — Pre-run git status capture

- Input: `realisons-business` working tree.
- Command:
  ```bash
  git -C "$CORPUS_GIT_ROOT" status --short > "$RUN_DIR/corpus-status-before.txt"
  git -C "$CORPUS_GIT_ROOT" rev-parse HEAD  > "$RUN_DIR/corpus-commit.txt"
  ```
- Output: `corpus-status-before.txt` (must be empty), `corpus-commit.txt`.
- Expected exit code: 0. The script aborts (exit 3) if
  `corpus-status-before.txt` is non-empty before the run starts.

### Step 1 — Source scan (manifest precursor)

- Input: `$CORPUS` directory.
- Command:
  ```bash
  nomos corpus scan --root "$CORPUS" --out "$RUN_DIR/snapshot.json" --format json
  ```
- Output: `snapshot.json` listing every source file with its hash.
- Expected exit code: 0.
- Subcommand: present today (see `cli/internal/app/corpus_cmd.go::corpusScanCommand`).

### Step 2 — Source manifest emission

- Input: `snapshot.json`.
- Command:
  ```bash
  nomos corpus manifest \
    --snapshot "$RUN_DIR/snapshot.json" \
    --out      "$RUN_DIR/source-manifest.yaml"
  ```
- Output: `source-manifest.yaml`.
- Expected exit code: 0.
- Subcommand: present today.

### Step 3 — Source segment ledger emission

- Input: `$CORPUS` directory + `source-manifest.yaml`.
- Status: **TODO — no dedicated CLI subcommand emits a JSON
  `[]SourceSegment` ledger today.** The ledger is computed in-process
  by `corpus.ScanMarkdown` (SFI-02 / #340). Until a dedicated CLI is
  added, the integrity gate computes the ledger internally as part of
  step 4 (`--corpus-integrity-source` walks the directory and runs
  `ScanMarkdown` per file). Tracked as a follow-up to SFI-02 / #340
  and to be revisited under SFI-08 / #346 CLI surface.
- Output: *(none — implicit input to step 4).*

### Step 4 — Source integrity gate (SFI-04, #342)

- Input: directory of `*.md` source files inside `$CORPUS`.
- Command (today, via the strict gate flag plumbing in
  `cli/internal/app/strict_gate.go`):
  ```bash
  ( cd "$NOMOS_REPO/cli" && \
    go run ./internal/app strict \
      --corpus-integrity-source "$CORPUS" \
      --format json ) > "$RUN_DIR/integrity-source.json"
  ```
  Status: **TODO — `nomos strict` is not yet wired into the top-level
  command map (`cli/internal/app/app.go::Run`).** `StrictGateCommand`
  (the function that owns the `--corpus-integrity-*` flags) is
  implemented but not yet selectable from the `nomos` binary. Tracked
  under SFI-08 / #346 as a CLI surface follow-up; until then the run
  script invokes the gate via `go run` against the package directly.
- Output: `integrity-source.json` carrying the SFI-04 `IntegrityReport`
  payload (status, source/segment counts, findings).
- Expected exit code: 0 if `Status == "pass"`, 1 otherwise.
- Reference: `cli/internal/corpus/source_integrity_gate.go::CheckSourceIntegrity`.

### Step 5 — Feed generation (SFI-05, #343)

- Input: `$CORPUS`, `snapshot.json`, `source-manifest.yaml`.
- Command:
  ```bash
  nomos corpus feed \
    --root     "$CORPUS" \
    --snapshot "$RUN_DIR/snapshot.json" \
    --manifest "$RUN_DIR/source-manifest.yaml" \
    --out      "$RUN_DIR/feed.json"
  ```
- Output: `feed.json` (a `Feed` JSON document; SFI-05 fields
  `source_segment_id`, `start_byte`/`end_byte`, `start_line`/`end_line`,
  `normalized_text_hash`, `heading_path` are populated on
  source-derived units).
- Expected exit code: 0. Feed generation fails closed if any source
  fails the SFI-04 gate (see `feed.go::feedUnitsFromSegments`).
- Subcommand: present today.

### Step 6 — RAG metadata (SFI-06, #344)

- Input: `feed.json` (RAG metadata is emitted as part of
  `nomos corpus feed`).
- Status: **TODO — there is no dedicated `nomos corpus rag`
  subcommand today.** RAG metadata is bundled inside the feed
  artefact under the `rag_metadata` key (see
  `feed.go::buildRAGMetadata`). The run script extracts it via `jq`:
  ```bash
  jq '.rag_metadata' "$RUN_DIR/feed.json" > "$RUN_DIR/rag.json"
  jq '.units'        "$RUN_DIR/feed.json" > "$RUN_DIR/feed-units.json"
  ```
  Tracked as a follow-up under SFI-06 / #344 CLI surface (or under
  SFI-08 / #346 alongside the strict-gate CLI wiring).

### Step 7 — Feed quality gate (SFI-07, #345)

- Input: `$CORPUS`, `feed-units.json`, `rag.json`.
- Command (today, via the same strict-gate plumbing):
  ```bash
  ( cd "$NOMOS_REPO/cli" && \
    go run ./internal/app strict \
      --corpus-integrity-source "$CORPUS" \
      --corpus-integrity-feed   "$RUN_DIR/feed-units.json" \
      --corpus-integrity-rag    "$RUN_DIR/rag.json" \
      --format json ) > "$RUN_DIR/integrity-feed.json"
  ```
  Status: **TODO — same wiring caveat as step 4.** Tracked under
  SFI-08 / #346.
- Output: `integrity-feed.json` containing both the source integrity
  sub-report and the feed quality sub-report.
- Expected exit code: 0 if both sub-reports pass, 1 otherwise.
- Reference: `cli/internal/corpus/feed_quality_gate.go::CheckFeedQuality`.

### Step 8 — Attestation with bounded claim

- Input: `snapshot.json`.
- Command:
  ```bash
  nomos corpus attest \
    --snapshot   "$RUN_DIR/snapshot.json" \
    --corpus-id  rbok-lawbook \
    --project-id rbok \
    > "$RUN_DIR/attestation.json"
  ```
- Output: `attestation.json`.
- Expected exit code: 0.
- Bounded claim language to be embedded by the run script alongside
  the attestation in `<RUN_DIR>/attestation-claim.txt`:

  > *On the recorded run of NOMOS commit `<NOMOS-SHA>` against
  > `realisons-business/01_rbok` at commit `<CORPUS-COMMIT-SHA>`, the
  > SFI-04 source integrity gate and the SFI-07 feed quality gate
  > each reported `status=pass` with `0` findings, and the SFI-08
  > strict release gate exited 0. The corpus working tree was clean
  > before and after the run. The build advertises claim level
  > `source-integrity-proven` for this run. Promotion to
  > `full-fidelity-proven` requires this strict gate to be wired
  > into CI for the RBOK POC corpus and to remain green across
  > consecutive runs (gating issue: #346).*

  Both claim levels (`source-integrity-proven`,
  `full-fidelity-proven`) are defined in
  `docs/public-claim-boundary.md`. Do not advertise
  `full-fidelity-proven` on the basis of this dossier alone.

### Step 9 — Strict gate consuming all of the above

- Input: integrity report + feed + RAG produced above.
- Command:
  ```bash
  ( cd "$NOMOS_REPO/cli" && \
    go run ./internal/app strict \
      --corpus-integrity-source "$CORPUS" \
      --corpus-integrity-feed   "$RUN_DIR/feed-units.json" \
      --corpus-integrity-rag    "$RUN_DIR/rag.json" \
      --format json ) > "$RUN_DIR/strict-gate.json"
  ```
  (Once SFI-08 / #346 wires `nomos strict`, the same flags via the
  `nomos` binary directly: `nomos strict --corpus-integrity-source ... --format json`.)
- Output: `strict-gate.json`.
- Expected exit code: 0 if the gate passes, 1 if it fails.
- Reference: `cli/internal/app/strict_gate.go::StrictGateCommand`.

### Step 10 — Post-run git status capture

- Input: `realisons-business` working tree.
- Command:
  ```bash
  git -C "$CORPUS_GIT_ROOT" status --short > "$RUN_DIR/corpus-status-after.txt"
  diff -q "$RUN_DIR/corpus-status-before.txt" "$RUN_DIR/corpus-status-after.txt"
  ```
- Output: `corpus-status-after.txt` (must equal
  `corpus-status-before.txt`, both empty on a clean tree).
- Expected exit code: 0. Any difference fails the POC (script exits
  with code 4 and writes the diff to
  `<RUN_DIR>/corpus-mutation.diff`).

## 5. Success criteria

Verbatim from #349. The run is treated as a passing POC only if every
criterion below holds.

- 0 uncovered active semantic ranges (or explicit policy exclusions).
- 0 duplicate semantic source spans.
- 0 junk semantic feed atoms.
- 0 RAG chunks without source segment linkage.
- 0 parent/child semantic body duplications.
- All unsupported blocks explicit as `unsupported_blocking` or
  `excluded_by_policy`.
- Strict gate passes only if the integrity report passes.

Each criterion above maps to a specific finding code or section in
the integrity / feed-quality reports
(`SOURCE_UNCOVERED_RANGE`, `SOURCE_DUPLICATE_SEMANTIC_SPAN`,
`SOURCE_JUNK_SEMANTIC_ATOM`, RAG-linkage findings emitted by
`feed_quality_gate.go::CheckFeedQuality`, etc.). The
`assert-success` block in `scripts/rbok-poc-integrity.sh` performs
the criterion mapping and is the single source of truth for "did
this run pass".

## 6. Run record

| Step | Name | Exit code | Artefact path | Finding count | Status |
|---|---|---|---|---|---|
| 0  | Pre-run git status capture       | n/a | n/a | n/a | **BLOCKED — corpus not accessible at /root/repos/realisons-business/01_rbok** |
| 1  | Source scan                      | n/a | n/a | n/a | **BLOCKED — corpus not accessible** |
| 2  | Source manifest                  | n/a | n/a | n/a | **BLOCKED — corpus not accessible** |
| 3  | Source segment ledger            | n/a | n/a | n/a | **BLOCKED — corpus not accessible (and no CLI ledger emitter today; see step 3 TODO)** |
| 4  | Source integrity gate (SFI-04)   | n/a | n/a | n/a | **BLOCKED — corpus not accessible** |
| 5  | Feed generation (SFI-05)         | n/a | n/a | n/a | **BLOCKED — corpus not accessible** |
| 6  | RAG metadata (SFI-06)            | n/a | n/a | n/a | **BLOCKED — corpus not accessible** |
| 7  | Feed quality gate (SFI-07)       | n/a | n/a | n/a | **BLOCKED — corpus not accessible** |
| 8  | Attestation (bounded claim)      | n/a | n/a | n/a | **BLOCKED — corpus not accessible** |
| 9  | Strict gate (SFI-08)             | n/a | n/a | n/a | **BLOCKED — corpus not accessible** |
| 10 | Post-run git status capture      | n/a | n/a | n/a | **BLOCKED — corpus not accessible** |

This dossier was produced on the orchestrator agent host
(`/root/repos/Nomos-copilot`). The path
`/root/repos/realisons-business/01_rbok` does **not** exist on this
host at dossier time; therefore the actual SFI-04..SFI-08 source-to-feed
integrity POC run could not be executed. Finding counts are
deliberately left as `n/a` rather than fabricated.

The dossier is marked **pending corpus access**.

The next concrete steps to lift the block:

1. Provision read-only corpus access on the orchestrator host (clone
   `realisons-business` to `/root/repos/realisons-business` and pin
   to a recorded sha) **or** run this dossier on a host where the
   corpus already lives.
2. Re-run `scripts/rbok-poc-integrity.sh` end-to-end and replace this
   table with the resulting per-step rows (exit code, artefact path,
   finding count, status read from the JSON reports).
3. Update the bounded claim language in section 8 to cite the actual
   `<NOMOS-SHA>` and `<CORPUS-COMMIT-SHA>` for the run.

Until those steps land, the dossier records readiness of the command
sequence — not a passing run.

## 7. Remaining gaps

Each gap below is a known limitation of the source-to-feed integrity
engine as of this dispatch. Listing them here keeps the bounded
claim honest.

- **Setext-style headings** (underline-style `===` / `---` for H1/H2)
  are not yet handled by the typed Markdown scanner. Tracked under
  SFI-02 / #340.
- **GFM `\|` escape inside table cells** is not handled by the
  table-cell splitter. Cells separated by escaped pipes will be
  mis-counted. Tracked under SFI-02 / #340.
- **Pre-heading paragraphs** (canonical body text appearing before
  the first heading) are intentionally not surfaced as feed units.
  SFI-04 will flag this only as a coverage finding, not as a feed
  unit. Tracked under SFI-03 / #341 judgment call
  (`extract_md.go::extractFromBytes`).
- **No CLI emitter for the source segment ledger** today. The
  integrity gate reads the ledger in-process. Step 3 is a TODO.
  Tracked under SFI-02 / #340 follow-up (or under SFI-08 / #346
  alongside CLI wiring).
- **`nomos strict` is not in the top-level command map** today.
  `StrictGateCommand` (with `--corpus-integrity-*` flags) is
  implemented in `cli/internal/app/strict_gate.go` but must be
  invoked via `go run` until SFI-08 / #346 wiring lands. Steps 4, 7,
  9 carry this caveat.
- **No dedicated `nomos corpus rag` subcommand**. RAG metadata is
  bundled inside `nomos corpus feed` output. The run script
  extracts it via `jq`. Tracked under SFI-06 / #344 follow-up.
- **HTML blocks** are surfaced as `unsupported_blocking` rather than
  parsed. Intentional, but corpora with significant HTML will fail
  the integrity gate until the typed scanner grows HTML support.
  Tracked under SFI-02 / #340.
- **CI wiring of the strict gate** for the RBOK POC is out of scope
  for SFI-11. Until CI runs the strict gate green on consecutive
  builds against `01_rbok`, the dossier may not advertise
  `full-fidelity-proven`. Tracked under SFI-08 / #346.

If the actual POC run reveals additional gaps, append them here
verbatim from the relevant finding codes — do not summarise them
away.

## 8. Bounded claim

Exact language permitted in any attestation file or release note that
cites this dossier:

> *On the recorded run of NOMOS commit `<NOMOS-SHA>` against
> `realisons-business/01_rbok` at commit `<CORPUS-COMMIT-SHA>`, the
> SFI-04 source integrity gate and the SFI-07 feed quality gate each
> reported `status=pass` with `0` findings, and the SFI-08 strict
> release gate exited 0. The corpus working tree was clean before
> and after the run. The build advertises claim level
> `source-integrity-proven` for this run. Promotion to
> `full-fidelity-proven` requires this strict gate to be wired into
> CI for the RBOK POC corpus and to remain green across consecutive
> runs (gating issue: #346).*

Both claim levels (`source-integrity-proven`,
`full-fidelity-proven`) are defined in
`docs/public-claim-boundary.md` (SFI-00). This dossier may not lift
either claim level beyond what was actually proven on the recorded
run.

While the dossier is marked **pending corpus access** (section 6),
the bounded claim above is *templated text* and **must not** be
advertised. No claim level is conferred by an unrun POC.

## 9. Engine reference

The algorithmic and governance description of the source-to-feed
integrity engine is documented in
`docs/21-source-feed-integrity-engine.md` (SFI-10 / #348). This
dossier intentionally does not duplicate the engine specification;
it records only the *run* of the engine against the RBOK POC corpus.

## Historical: legacy v0.1.0-ALPHA POC record

The following section preserves the previous content of this file
for traceability. It records the legacy `rbok-lawbook` profile POC
(release gate + strict fidelity gate, distinct from the SFI-04..SFI-08
source-to-feed integrity gate that is the subject of this dossier).
The legacy record is retained verbatim; it does not extend, amend,
or substitute for the SFI-11 bounded claim above.

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
> source-to-feed integrity gate). The legacy POC supports a bounded
> alpha claim that Nomos can transform the RBOK lawbook corpus into
> traceable, release-gated artefacts without mutating the source. It
> does not support a blanket claim of universal regulated-grade
> fidelity, and it does not establish the
> `source-integrity-proven` claim level introduced by SFI-11 / #349.*

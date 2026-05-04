# Spec Examples

Ce dossier contient des exemples de manifests produit pour tester le meta-modele Nomos.

Exemples cibles :

- `nomos-project.minimal.yaml` : projet minimal greenfield ;
- `nomos-project.brownfield.yaml` : projet brownfield avec bloquants et scope partiel ;
- `nomos-project.regulated.yaml` : projet regulated avec exigences d'evidence plus fortes ;
- `canonical-matrix.valid.yaml` : matrice canonique valide ;
- `canonical-matrix.invalid-*.yaml` : matrices canoniques invalides pour verifier les contraintes CUE ;
- `verdict-cases.yaml` : cas de verdicts et niveaux de confiance NOM-104 ;
- `nomos-report.minimal.json` : report Nomos minimal valide ;
- `nomos-report.complete.json` : report Nomos complet avec findings, codes erreur, severites et evidence ;
- `adapter-manifest.node-typescript.yaml` : manifeste adapter valide.

Les fichiers `nomos-project.*.yaml` doivent passer avec :

```bash
cue vet specs/nomos-project.cue specs/examples/<fixture>.yaml -d '#Project'
```

Les fichiers `canonical-matrix.invalid-*.yaml` sont des fixtures negatives :
ils doivent echouer avec `cue vet specs/canonical-matrix.cue <fixture> -d
'#CanonicalMatrix'`.

Les fichiers `nomos-report.*.json` doivent rester compatibles avec
`specs/nomos-report.schema.json`.

## SFI-09 (#347) — corpus integrity examples

The fixtures below mirror the Go-side artifacts produced by the
source-to-feed integrity pipeline (SFI-01 through SFI-07). They are
versioned alongside the schemas so a regulated reviewer can replay
the contract without running the CLI.

### `source-segment-ledger.valid.yaml`

Mirrors `[]corpus.SourceSegment` (see
`cli/internal/corpus/source_segment.go`) packaged as a
`#SourceSegmentLedger` envelope. It contains a heading
(`structure_only`), two blanks and one decorative separator
(`coverage_only`), and one paragraph (`canonical_atom`, with both
`raw_text_hash` and `normalized_text_hash`). Byte spans are
non-overlapping and contiguous.

Validate with:

```bash
cue vet specs/examples/source-segment-ledger.valid.yaml \
        specs/source-segment-ledger.cue -d '#SourceSegmentLedger'
```

### `corpus-integrity-report.valid.yaml`

Mirrors `corpus.IntegrityReport` (see
`cli/internal/corpus/source_integrity_gate.go`). A passing report
with `status: pass`, non-zero counts, and no findings. Validate with:

```bash
cue vet specs/examples/corpus-integrity-report.valid.yaml \
        specs/corpus-integrity-report.cue -d '#IntegrityReport'
```

### `source-segment-ledger.invalid.yaml` (intentional negative fixture)

Demonstrates schema strictness: a single canonical_atom segment is
emitted **without** `raw_text_hash` or `normalized_text_hash`. The
conditional invariant on `#SourceSegment` rejects it. This file is
documentation only; it MUST fail `cue vet` and MUST NOT be wired into
any green CI step.

Demonstration:

```bash
cue vet specs/examples/source-segment-ledger.invalid.yaml \
        specs/source-segment-ledger.cue -d '#SourceSegmentLedger'
# expected: non-zero exit with messages on missing raw_text_hash
# and normalized_text_hash for the canonical_atom segment.
```

### Whole-tree validation

All Nomos schemas type-check together with:

```bash
cue vet ./specs/...
```

## NGW-001 (#386) — GitHub workflow descriptor

The fixtures below mirror the configuration contract for
`.nomos/corpus-workflows.yaml` defined by `specs/nomos-github-workflow.cue`.
They describe how a NOMOS-driven workflow declares the corpus source,
the artifact output, the NOMOS command sequence, the publication
policy, and the source PR comment policy. There is no Go struct in
this PR — schema only; downstream NGW tickets read the schema and
build the runtime around it.

### `nomos-github-workflow.source-owned.valid.yaml`

Source-owned configuration: the file lives in the corpus repository,
points to the same repo via `source.repo`, and publishes generated
output to a separate output repository via a pull request. Mirrors
the design doc's `Configuration Contract` example (RBOK lawbook
scope, `pull_request` mode, `medium` risk, source PR comment enabled
with `summary` mode).

```bash
cue vet specs/nomos-github-workflow.cue \
        specs/examples/nomos-github-workflow.source-owned.valid.yaml \
        -d '#NomosGitHubWorkflowConfig'
```

### `nomos-github-workflow.output-owned.valid.yaml`

Output-owned configuration: the file lives in the output repository.
`source.repo` points at the corpus repo, `output.repo: corpus`
expresses "the other one" relative to where the workflow is checked
in, and the workflow uses `direct_push` under `risk_class: low` with
`branch_strategy: fixed`. The notify block disables source PR
comments because output-owned workflows often skip them.

```bash
cue vet specs/nomos-github-workflow.cue \
        specs/examples/nomos-github-workflow.output-owned.valid.yaml \
        -d '#NomosGitHubWorkflowConfig'
```

### `nomos-github-workflow.invalid.yaml` (intentional negative fixture)

Demonstrates that the conditional invariant in `#PublishSpec` is
enforced: the fixture declares `mode: direct_push` AND
`risk_class: regulated` AND omits `controlled_decision`. The schema
rejects it. This fixture is documentation only; it MUST NOT enter a
green CI step.

Expected `cue vet` error substring:

```
workflows.0.publish.controlled_decision: incomplete value !="":
```

```bash
cue vet specs/nomos-github-workflow.cue \
        specs/examples/nomos-github-workflow.invalid.yaml \
        -d '#NomosGitHubWorkflowConfig'
# expected: non-zero exit; controlled_decision flagged as required.
```

## NGW-002 (#387) — Trace manifest

Each NOMOS GitHub workflow run emits a trace manifest
(`nomos-trace.yaml` / `nomos-trace.json`) regardless of publication
mode. The contract is `specs/nomos-trace-manifest.cue` and its
`policy.publish_mode` / `policy.risk_class` enums mirror NGW-01's
`#PublishSpec` exactly so the two schemas cannot drift.

### `nomos-trace-manifest.valid.yaml`

Mirrors the design doc's manifest example (RBOK lawbook scope,
`direct_push` mode, `risk_class: low`, both safety guards `pass`).
Every mandatory field is populated and the `artifacts` block lists
the five well-known NOMOS artifact filenames. Validate with:

```bash
cue vet specs/nomos-trace-manifest.cue \
        specs/examples/nomos-trace-manifest.valid.yaml \
        -d '#NomosTraceManifest'
```

### `nomos-trace-manifest.invalid.yaml` (intentional negative fixture)

Demonstrates that the conditional invariant on `#NomosTraceManifest`
is enforced: the fixture declares `policy.publish_mode: pull_request`
but omits `output.commit_sha`. The schema rejects it. This fixture
is documentation only; it MUST NOT enter a green CI step.

Expected `cue vet` error substring:

```
output.commit_sha: incomplete value =~"^[0-9a-f]{7,40}$":
```

```bash
cue vet specs/nomos-trace-manifest.cue \
        specs/examples/nomos-trace-manifest.invalid.yaml \
        -d '#NomosTraceManifest'
# expected: non-zero exit; output.commit_sha flagged as required by
# the publish_mode=pull_request conditional.
```

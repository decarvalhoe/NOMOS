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

# Public bibles processing (RCP-010 / #196, public half)

[RCP-010 (#196)](https://github.com/RBOKproject/NOMOS/issues/196) asks to process
the **public and licensed** Nomos canonical bibles. This directory delivers the
**public half**, which is actionable today. The **licensed half** (ISO 13485,
ISO/IEC/IEEE 12207, ISO/IEC 25010, ISPE GAMP 5) is **blocked on licensed-document
procurement** (#192/#193/#194) and, per the development doctrine §2.6, must never
be committed as full text — so it is explicitly excluded here rather than faked.

## The split

The external reference register classifies every bible by `source_class`.
`scripts/regulated_reference_canon.py` is the source of truth:

| Class | Count | Processed here? |
|---|---|---|
| **public** (FDA, NIST, NASA, ICH, MHRA, EU, GitHub …) | 23 | ✅ yes — read-only Nomos pipeline |
| **licensed** (ISO ×3, ISPE GAMP 5) | 4 | ⛔ blocked on procurement (#192/#193/#194) |

## What is produced (RCP-010 deliverables)

`scripts/process_public_bibles.py` snapshots the in-repo public reference corpus
into a dedicated, **push-free** git checkout (the corpus is never the live repo)
and runs `nomos corpus scan → manifest → feed → attest` over it:

- **manifests** — `source-manifest.yaml`;
- **atomization reports** — the corpus `feed.json`;
- **attestation** — `attestation.json`;
- **no-source-mutation evidence** — the corpus read-only guard plus a before/after
  `git status` check on the snapshot corpus.

[`processing-summary.json`](processing-summary.json) is the committed, point-in-time
receipt: `read_only_guard: pass`, `source_mutation: none`, `licensed_leak: []`.

## Reproduce

```bash
( cd cli && go build -o /tmp/nomos . )
python scripts/process_public_bibles.py --nomos-bin /tmp/nomos \
  --out docs/regulated/reference-basis/public-bibles-processing/processing-summary.json
```

The script returns non-zero if **any** source mutation is detected, a licensed
bible leaks into the processed set, or the manifests/reports are not produced —
so a green run is the acceptance evidence (doctrine §2.3). The test
`tests/test_public_bibles_processing.py` runs the same pipeline in CI.

## Acceptance mapping (#196)

| Acceptance criterion | Evidence |
|---|---|
| Atomization reports and manifests exist | `artifacts_present.{manifest,atomization_feed}` |
| …without source mutation | `read_only_guard: pass`, `source_mutation: none` |
| Read-only guard proves corpus unchanged before/after | before/after `git status` + `git rev-parse HEAD` equal |
| Licensed bibles not processed | `licensed_leak: []`, listed under `bible_split.licensed_blocked` |

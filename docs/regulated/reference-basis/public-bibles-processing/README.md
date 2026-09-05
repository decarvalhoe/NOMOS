# Public Reference Processing Fixture (RCP-010 / #196)

[RCP-010 (#196)](https://github.com/RBOKproject/NOMOS/issues/196) asks to process
public and licensed canonical bibles. This directory currently proves only the
**pipeline mechanics on two public in-repo policy documents**. It does not prove
that the 23 references classified `public` in the register were fetched or
atomized.

Actual registry-driven public-source processing and retained evidence is the
autonomous issue #644. The licensed half (ISO 13485, ISO/IEC/IEEE 12207,
ISO/IEC 25010, ISPE GAMP 5) remains on the independent regulated roadmap
(#192/#193/#194/#196) and blocks only its named clause-level use.

## Measured Split

`scripts/regulated_reference_canon.py` classifies the reference register;
`scripts/process_public_bibles.py` currently selects two repository documents
as its test corpus. These are different counts:

| Class / corpus | Registered | Actually processed by this fixture | Claim |
|---|---:|---:|---|
| Public references (FDA, NIST, NASA, ICH, MHRA, EU, GitHub…) | 23 | 0 external source snapshots | classified only |
| In-repo reference-policy documents | — | 2 | pipeline mechanics exercised |
| Licensed references (ISO ×3, ISPE GAMP 5) | 4 | 0 | explicitly blocked |

The two processed files are `docs/regulated/reference-basis/README.md` and
`docs/regulated/reference-basis/nomos-bible-corpus-policy.md`.

## What The Fixture Produces

`scripts/process_public_bibles.py` copies those two files into a dedicated,
push-free checkout and runs the read-only Nomos pipeline. During the run it
creates a source manifest, feed and attestation in a temporary artifact
directory; only [`processing-summary.json`](processing-summary.json) is retained
today. The receipt records `read_only_guard: pass`, `source_mutation: none` and
`licensed_leak: []`.

Because the full artifacts are not retained and no registered external public
source is snapshotted, the honest claim is:

> Two public in-repo policy documents exercised the read-only pipeline without
> licensed leakage or source mutation.

It is not “23 public bibles processed.” #644 must retain/content-address every
stage and count only a registered source that completes scan, manifest, feed,
body ledger, attestation and strict gate.

## Reproduce

```bash
( cd cli && go build -o /tmp/nomos . )
python scripts/process_public_bibles.py --nomos-bin /tmp/nomos \
  --out docs/regulated/reference-basis/public-bibles-processing/processing-summary.json
```

The current test `tests/test_public_bibles_processing.py` proves the bounded
fixture behavior. It must not be used as evidence for external-source coverage.

## Current Acceptance Mapping

| Bounded criterion | Evidence |
|---|---|
| Two policy documents selected | `processing-summary.json:corpus.documents` |
| Temporary manifest/feed created during the run | `artifacts_present` receipt fields |
| No source mutation | `read_only_guard: pass`, `source_mutation: none` |
| No licensed bible processed | `licensed_leak: []` |
| Actual public sources retained end-to-end | **open: #644** |

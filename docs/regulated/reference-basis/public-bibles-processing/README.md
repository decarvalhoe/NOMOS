# Public Reference Processing (RCP-010 / #196 — public slice #644)

[RCP-010 (#196)](https://github.com/RBOKproject/NOMOS/issues/196) asks to process
public and licensed canonical bibles. This directory says **exactly** what has
been done with the public half, and counts nothing it did not do.

## What "processed" means

A registered public reference counts as **processed** only when all of these
hold — anything short of it is reported as `blocked` with its reason, never as a
missing row:

1. the register classifies it public (`scripts/regulated_reference_canon.py`,
   the same classifier the licence gate uses — never a hardcoded list);
2. [`public-source-snapshots.yaml`](../public-source-snapshots.yaml) carries a
   dated capture entry: official URL, capture date, sha256, size,
   licence/policy note, version identity;
3. a local capture whose sha256 **equals** the recorded one is present in
   `--captures-dir`, outside the repository;
4. scan, manifest, feed, body ledger, attestation **and** the strict gate all
   exit 0 over that capture.

## Measured — 2026-09-05

`scripts/process_public_bibles.py --capture-live` fetched every public URL of
the register and recorded a **hash only** — nothing stored, nothing quoted:

| State | Count | Meaning |
|---|---:|---|
| Registered as public | 23 | register classification, nothing more |
| Captured hash-only (2026-09-05) | 20 | the URL served bytes with this sha256 that day |
| Blocked at fetch | 3 | reason recorded, see below |
| **Processed end to end** | **0** | no local capture exists; nothing traversed the chain |
| Licensed (ISO ×3, ISPE GAMP 5) | 4 | explicitly blocked, independent regulated roadmap |

Blocked at fetch, by name — a network refusal is a fact about that day, not a
reason to invent a hash:

| Reference | Reason |
|---|---|
| `FDA-CSA-2025` | `HTTPError: HTTP Error 404: Not Found` |
| `ICH-Q9R1` | `HTTPError: HTTP Error 403: Forbidden` |
| `NIST-SP-800-218` | `URLError: <urlopen error [Errno -3] Temporary failure in nam` |

`FDA-CSA-2025` returning 404 is a finding about the **register**: its URL is
stale. Recorded here, not silently corrected.

## The fixture corpus, named as such

Two in-repo policy documents (`README.md`, `nomos-bible-corpus-policy.md`) still
exercise the pipeline offline — all **six** stages now, body ledger and strict
gate included. They are reported under `fixture_documents` with
`counted_as_external: false`, and they never enter any external count.

## What is retained

Every stage's artifact is written to `--retain-dir` (default
`.nomos/retained-public-sources/<ref_id>/<sha256[:16]>/`, git-ignored) and
**content-addressed**: a receipt under [`receipts/`](receipts/) records the
sha256 of each artifact, the capture's sha256, the version identity and every
stage's exit code. The receipts are committed; the artifacts are not, because a
feed and a body ledger carry the atomised text of a third-party document and
the licence register forbids committing third-party full text. A receipt lets
anyone re-run the chain over the same capture and compare digests byte for
byte.

Change one byte of a capture and its sha256 changes: it no longer matches the
recorded one (`blocked: capture_hash_mismatch`) and any receipt keyed by the
old digest is reported **stale**, never reused.

## Reproduce

```bash
( cd cli && go build -o /tmp/nomos . )
# honest state of the tree (no local captures): fixture chain + blocked counts
python scripts/process_public_bibles.py --nomos-bin /tmp/nomos --out docs/regulated/reference-basis/public-bibles-processing/processing-summary.json
# refresh dated hash-only captures (network)
python scripts/process_public_bibles.py --nomos-bin /tmp/nomos --capture-live
# process a real capture kept OUTSIDE the repository
python scripts/process_public_bibles.py --nomos-bin /tmp/nomos --captures-dir ~/nomos-captures --retain-dir ~/nomos-retained
```

## Claim

> 20 public references have a dated hash-only capture; 0 have been processed
> end to end; 2 in-repo policy documents exercise all six stages. Nothing else.

It is not "23 public bibles processed". `tests/test_public_bibles_processing.py`
drives a synthetic capture through the full chain offline and proves that one
changed byte becomes a named mismatch with a stale receipt.

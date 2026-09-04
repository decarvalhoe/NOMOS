# Repeated CI Evidence — Private Corpus (VRC-14, #560)

## The gap this closes

Before this, NOMOS had **one** recorded run on the private RBOK corpus. A single
recorded run is not repeated evidence, and the public claim boundary names
repeated CI evidence on private corpora as part of the remaining, release-scoped
proof chain.

This directory does not assert that the chain exists. It **measures** it and
publishes the measurement, including the parts that are unfavourable.

## Files

| File | Role |
|---|---|
| `policy.yaml` | Versioned definition of what counts as evidence and of the target. Hashed into every index. |
| `index-<date>.json` | Published, dated index of the runs counted at that date, with the measurement. |
| `../evidence-ledger.yaml` | Ledger entry `EV-CI-REPEAT-001` and blocking gap `GAP-REPEATED-CI-EVIDENCE`. |

## What counts as one unit of evidence

A scheduled run counts only if **all three** hold:

1. it was triggered by the schedule on the default branch and completed;
2. its conclusion is `success` — anything else, `cancelled` and a missing
   conclusion included, is not green;
3. it archived at least one artifact and that artifact has **not expired**.

Rule 3 is the one that costs something. A green run whose pack has aged out of
GitHub's retention leaves nothing to re-inspect, so it is no longer archived
evidence. The consequence is deliberate: the chain **decays on its own** unless
runs keep coming. Evidence that cannot be looked at is not evidence.

## Why a missed week breaks the chain

The target is eight *consecutive* green runs on a weekly schedule. A workflow
that ran three times in June, went dark for six weeks, then ran four times in
August has not produced seven consecutive weekly evidences — it produced two
short chains with a hole between them. Runs that happened cannot vouch for weeks
that produced nothing.

So the streak is counted over scheduled **occurrences**, and a gap wider than
`cadence.max_gap_days` (10 days, tolerating a delayed run) is a missed
occurrence that breaks it. The chain must also be **current**: a streak that
ended months ago is history, not repeated evidence.

Both numbers are published side by side, because the weaker rule alone would
flatter the result:

- `consecutive_green_runs` — with the cadence rule. This is what the target is
  measured against.
- `consecutive_green_runs_ignoring_cadence` — the same walk with the rule off.
  Always at least as large. Published so the cost of the cadence rule is visible
  rather than hidden.

## Measured result — 2026-09-04

| Metric | Value |
|---|---|
| Scheduled runs recorded | 7 |
| Green runs | 7 / 7 |
| `consecutive_green_runs` | **4** |
| `consecutive_green_runs_ignoring_cadence` | 7 |
| Target | 8 |
| Missed weekly occurrences | 5 (2026-06-29 → 2026-08-10, 41-day gap) |
| Distinct corpus revisions | 2 |
| Streak is current | yes (newest run 4 days old) |
| **Claim unlocked** | **no** |

Three things this result says plainly:

- The chain is **half** the target, not at it.
- It broke once, in July, for six weeks. The break is named in the index with
  its exact window, not smoothed away.
- Across the whole recorded window the private corpus moved **twice**. So what
  is proven so far is that the pipeline keeps running green over a largely
  unchanging corpus snapshot — reproducibility, not coverage over evolving
  inputs. Eight runs over one frozen corpus would still not be eight runs over
  eight corpus states, and this index makes that visible via
  `distinct_corpus_commits`.

The packs themselves are not stored here: they are the workflow's own artifacts,
retained 90 days, and the index records each one's id, size and expiry so a
reader can fetch it while it lives, or see that it no longer does.

## Running it

```bash
# Verify the published index (offline, deterministic — what CI runs)
python3 scripts/repeated_ci_evidence.py --root .

# Re-measure live from the GitHub Actions API (needs `gh`)
python3 scripts/repeated_ci_evidence.py --root . --collect

# Re-publish a dated index
python3 scripts/repeated_ci_evidence.py --root . --collect --publish
```

Exit codes: `0` clean, `1` a check failed, `2` nothing could be measured (no
policy, no published index, or no API access in `--collect`). Exit 2 never
counts as a pass: the absence of a measurement is not evidence of a chain.

## What the gate refuses

The offline verification is wired into CI and fails on:

- **a policy whose bytes changed** since the index was built — any rule change
  forces a dated re-publication;
- **a published number that no longer matches a replay** of the recorded runs
  under the committed policy, replayed at the index's own recorded clock;
- **malformed run records** — a missing field, a duplicate run id, a run from
  the wrong event or branch, records out of order;
- **a workflow that lost its schedule** or **lowered its artifact retention**
  below what the published chain relies on, because either silently stops or
  erases the evidence;
- **a ledger that disagrees with the measurement** — `EV-CI-REPEAT-001` and
  `GAP-REPEATED-CI-EVIDENCE` must both match `claim_unlocked`, in either
  direction;
- **prose that asserts the claim while the measurement says it is locked**, and
  equally a claim boundary that does not carry the measured streak, so the prose
  and the number cannot drift apart unnoticed.

## Claim boundary

This is a measurement of the scheduled-run history of **one** workflow over
**one** private corpus. It is evidence that the pipeline kept running green on
that corpus. It is not a statement about coverage of other corpora, other
document formats, or the business correctness of any artifact the pipeline
produced.

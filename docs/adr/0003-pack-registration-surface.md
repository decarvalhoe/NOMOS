# ADR-0003: Registration Surface Is Not Engine Coupling

## Status

Accepted — 2026-09-04. Issue: VRC-22 (#565). Complements
[ADR-0002](0002-risk-tier-open-facet-axis.md).

## Context

D6 measures what a new domain vertical costs the core, target **0**. The
instrument (`scripts/pack_core_coupling_check.py`, VRC-38 #575) has two modes:

- `--manifest` — what the pack, as it stands, requires from core trees;
- `--changed-files` — what the PR that lands it touched outside pack trees,
  blocking unless the same PR carries an ADR.

Landing the EU AI Act pack produced a result worth recording. The manifest mode
reports **0**: every artifact the pack declares lives in
`docs/regulated/domain-packs/` or `cli/internal/corpus/testdata/`. The
changed-files mode reports **8 core paths**:

| Path | What it is |
|---|---|
| `.github/workflows/ci.yml` | the step that runs `pack validate` on the pack |
| `scripts/vrc_wiring_matrix_registry.json` | the capability declaration |
| `.vrc-wiring-matrix/wiring-matrix.{json,md}` | the regenerated matrix |
| `cli/internal/app/pack_cmd_test.go` | the test that runs the pack through the gate |
| `specs/examples/nomos-domain-profile.eu-ai-act.valid.yaml` | the profile the manifest references |
| `docs/45-vision-reality-closure-plan.md` | plan status |
| `docs/public-claim-boundary.md` | the claim the pack unlocks, and its bounds |

**None of these is engine logic.** The engine files a pack could couple to —
`cli/internal/app/pack_cmd.go`, `cli/internal/atomization/`,
`cli/internal/ragexport/`, `specs/facets.cue` — are untouched by this PR. The
one engine change this vertical required, the `risk_tier` axis, was taken
separately and is recorded in ADR-0002.

So the two modes measure different things, and the changed-files number
conflates them:

- **registration surface** — declaring that a pack exists, gating it, and
  stating what it claims. Bounded, enumerable, and required of *every* pack.
- **engine coupling** — changing how the machinery works to accommodate one
  domain. This is what D6's target of 0 is actually about.

A consequence follows that had not been written down: **no vertical can ever
land with a changed-files metric of 0.** A pack that nothing runs, nothing
declares and nothing bounds is not landed. The metric as instrumented can
therefore never be satisfied by a real pack PR, and reading a non-zero
changed-files number as "the core was not general enough" would be wrong.

## Decision

Record that the eight paths above are **registration surface**, not engine
coupling, and that touching them is the expected cost of landing any vertical.

D6 continues to be published as two numbers, never one:

- `--manifest`: **0** for the EU AI Act pack. This is the number that answers
  "is the pack a set of declarations?"
- `--changed-files`: non-zero for any landing PR. This number is only
  meaningful once split into registration surface versus engine files.

The engine-file count is the one that must stay at zero without an ADR. For
this PR it is **0**; for the PR that added `risk_tier` it was **1**, with
ADR-0002.

## Consequences

- The guard keeps blocking pack PRs that touch core without an ADR. That is
  correct and stays: silence is never the escape.
- This ADR is the justification for *this* PR's registration surface. It is not
  a standing waiver — a future pack PR that touches
  `cli/internal/atomization/`, `cli/internal/ragexport/`, `pack_cmd.go` or
  `specs/facets.cue` still owes its own written decision, and citing this ADR
  would not cover it.
- **Possible refinement, deliberately not taken here:** the guard could be
  taught the registration allowlist, so pack PRs are measured on engine
  coupling directly and the ADR escape is reserved for real engine changes.
  That is a change to the instrument, and changing a measuring instrument in
  the same PR that it is measuring is bad practice. It belongs in its own
  change, against VRC-38.

## What this ADR does not do

It does not widen what a pack may touch. The pack-allowed trees are unchanged,
and `#PackLocalPath` in `specs/domain-pack.cue` still governs what a manifest
may declare.

## Amendment — label timing cannot bypass D6 (#632, 2026-09-05)

The initial guard lived in the full CI workflow behind a job-level condition
that inspected the `pack` label. GitHub's default pull-request events did not
include `labeled`: the normal sequence "open the PR, then add the label" left
the job permanently skipped until another commit or a reopen. Removing the
label after a successful run had the same invisible failure mode.

Decision:

- D6 moves to `.github/workflows/pack-coupling.yml`, isolated from full CI so a
  label event does not rerun every platform and every gate;
- the workflow reacts to `opened`, `synchronize`, `reopened`, `edited`,
  `labeled` and `unlabeled`, with one concurrency group per PR;
- it always reports: a PR outside the pack tree returns an explicit
  not-applicable pass instead of a skipped job;
- touching `docs/regulated/domain-packs/` without the exact `pack` label is a
  blocking error. An ADR cannot waive a missing label;
- with `pack` present, the core/ADR policy above remains unchanged;
- base and head SHAs come from the event payload and renames are expanded, so
  the measured diff is not altered by a moving base or a rename.

This amendment changes when and how the instrument reports, not what counts as
engine coupling or what a pack is allowed to declare.

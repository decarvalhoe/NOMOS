# Nomos v1.0.0-BETA.1 Release Notes — CANDIDATE

Status: **candidate prepared, approval pending (#720)**. Not tagged, not
published. These notes describe what the candidate contains so that the human
release decision can be taken on measured facts.

## Release Position

`v1.0.0-BETA.1` is the first pre-release of the 1.0 line. It follows
`v0.2.0-ALPHA` (2026-09-06) and closes the beta plan of `docs/51`: the eight
`docs/14` v1.0 criteria are computed `ready` by `nomos portfolio
release-readiness`, that verdict is a required gate of the candidate, the
support surface is declared and checked, and the candidate is prepared with a
pending decision record. A beta is a product statement; it is not a
validated-use, QMS-effectiveness or regulated-readiness claim (`docs/28`).

## What `docs/16` Requires Of A Release

- Core version `1.0.0-BETA.1` — announced by `nomos version --json`.
- Supported schema versions — the 15 `stable` contracts of
  `specs/contract-registry.yaml`, each read at its version by the engine's
  own loader (19 compatibility reads); experimental contracts may change
  without a MAJOR notice. Matrix: `docs/16` "Compatibility Matrix".
- Verified adapters — `jvm`, `node-typescript`, `python`, manifest contract
  0.1.0, compatible with the core (`nomos contracts status`).
- Reference policies — `docs/security/security-process.yaml`,
  `docs/support-model.yaml`, `docs/regulated/evidence-index/evidence-ledger.yaml`,
  `specs/contract-registry.yaml`, `docs/public-claim-boundary.md`.
- Incompatible changes — body-ledger `segments[]` now carry the contract's
  field names instead of Go field names (consumers of the Go names were
  outside the contract); the hand-written body-ledger example became an
  engine-emitted one.
- Migrations — regenerate body ledgers with this core; regenerate the
  evidence-ledger index after changes under indexed locations.

## Delivered Since v0.2.0-ALPHA

See `CHANGELOG.md` "v1.0.0-BETA.1": contract stability registry (#676),
compatibility matrix and version announcement (#677), security process (#678),
support model (#679), integration guide replayed in CI (#680), readiness
verdict (#681), cross-consumption proof kit (#702), regulated-tool blocks
(#715), evidence ledger as generated index (#716), compatibility fixtures per
stable contract (#714), readiness gate on the candidate (#717), support
surface (#718), this candidate (#719).

## Release Gate

The candidate assembles only when every required gate passed on the same
commit, `release-readiness` included; the rehearsal proves the refusal on a
forged gate. `approval_status` stays `pending` in the candidate; the decision
record `docs/regulated/lifecycle/release-records/v1.0.0-BETA.1-release-decision.yaml`
is a draft with `decision: pending`.

## Known Open Items

Regulated lane, never closed by tooling: #560 (repeated CI evidence, 4/8),
#561/#562 (SOP execution, competence records), #192/#193/#194/#196 (licensed
references), #638/#576 (production keyless), #701 (partner). Open evidence
gaps are carried by the candidate manifest.

## Claim Boundary

"A beta candidate exists, pending a human decision." No tag, no publication,
no approval by the PR that prepared it.

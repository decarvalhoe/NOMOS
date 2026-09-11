# Release candidates (#639)

A **release candidate bundle** binds one commit to the facts a human release
decision needs, and proves them offline. It is not a release: `approval_status`
is always `pending`, `release_executed` is always `false`, and no tag, GitHub
Release, notes or approval is produced here. Those are acts under the
[release SOP](../lifecycle/release-and-retirement-sop.md), tracked in #561.

| File | Role |
|---|---|
| `v0.2.0-ALPHA-candidate.yaml` | the declared candidate: level, ledger, acknowledged gaps, risks, required gates, waivers, deviations |
| `scripts/release_candidate_gates.py` | measures the gate set on a commit and records exit codes (`nomos-release-candidate-gates-v1`) |
| `nomos release candidate` | assembles the bundle, or **refuses and writes nothing** |
| `nomos release verify` | re-verifies a bundle offline (and against a tree) |
| `.github/workflows/release-candidate-rehearsal.yml` | the rehearsal: assemble → verify → tamper must fail → archive as artifact |

## What is validated — content and status, not presence

| Rule | Refusal code |
|---|---|
| approval_status is anything but `pending` | `CANDIDATE_APPROVAL_NOT_PENDING` |
| any approval entry is declared | `CANDIDATE_APPROVAL_INVENTED` |
| `release_executed: true` | `CANDIDATE_RELEASE_CLAIMED` |
| a required artifact for the target level is absent | `CANDIDATE_ARTIFACT_MISSING` |
| a present artifact is empty or does not parse as its type | `CANDIDATE_ARTIFACT_UNREADABLE` |
| an open blocking gap of the evidence ledger is not acknowledged | `CANDIDATE_GAP_UNACKNOWLEDGED` |
| an acknowledged gap is closed or unknown in the ledger | `CANDIDATE_GAP_STATUS_MISMATCH` |
| a required gate has no evidence / failed / ran on another commit | `CANDIDATE_GATE_MISSING` / `CANDIDATE_GATE_FAILED` / `CANDIDATE_GATE_COMMIT_MISMATCH` |
| a risk source cannot be read as its declared kind | `CANDIDATE_RISK_UNREADABLE` |
| a waiver has no status, or a waived record lacks date or approver | `CANDIDATE_WAIVER_INCOMPLETE` |
| a deviation has no id, an unknown status or no record | `CANDIDATE_DEVIATION_INVALID` |
| any evidence byte, gates file, hash, gate status or approval changed after assembly | `CANDIDATE_TAMPERED` and the invariant codes above |

## VRC-14 is a risk, not a gate

The repeated-CI measure (`docs/regulated/evidence-index/repeated-ci-evidence/`)
is **read** from its published index and recorded in the bundle with its
measured values (consecutive green runs, target, `claim_unlocked`). The
candidate never waits for it and refuses a manifest that marks any risk as
blocking. Whether an open risk is acceptable is the release decision's, not the
tool's.

## Claim boundary

A verified candidate proves that these artifacts, gates, gaps, risks, waivers
and deviations were bound to this commit and are unchanged. It proves nothing
about the release decision, the quality level, or the acceptability of any gap.

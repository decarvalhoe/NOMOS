# Nomos v0.2.0-ALPHA Release Notes

Date: 2026-09-06 · Kind: pre-release (alpha) · Decision record:
`docs/regulated/lifecycle/release-records/v0.2.0-ALPHA-release-decision.yaml`

## Release Position

`v0.2.0-ALPHA` is the second public alpha. `v0.1.0-ALPHA` proved the
source-to-feed chain on one private corpus. `v0.2.0-ALPHA` closes the
vision/reality gap audited in `docs/45`: every capability is registered in
`scripts/vrc_wiring_matrix_registry.json`, its status is **computed** in CI
from the tree, and every pending human decision is visible as data rather
than narrated. It is suitable for demonstrations, controlled pilots and
regulated-readiness review. It is not a production validation, a certification,
an eQMS, or a universal-fidelity claim.

## What Is Included

Measured on the release commit (`nomos portfolio status`, `.vrc-wiring-matrix/`):

| Measure | Value |
|---|---:|
| Registered capabilities | 57 |
| Computed `real` (Go engine + production caller + adversarial tests + CI gate) | 44 |
| Computed `sidecar` (deliberately out of core, Python) | 11 |
| Computed `absent` by design (production Sigstore issuance; removed control-plane) | 2 |
| Registry/matrix mismatches | 0 |
| Python tests | 476 passed, 26 skipped |
| Go `go test ./...` | green |

Delivered since `v0.1.0-ALPHA` (details in `CHANGELOG.md`):

- **Vision-Reality Closure** (epic #545): wiring matrix and claim-boundary
  guard as gates; cite-or-abstain, canon promotion and point-in-time in the
  Go engine; RAG eval harness and public bench with dated, replayed results;
  domain-pack contract and gate with two verticals; PDF/DOCX/HTML adapters
  behind capability kits; evidence BOMs; deterministic cross-reference graph;
  facet SHACL validation.
- **External sources**: web-source contract, immutable external snapshots,
  offline Recursio → NOMOS end-to-end fixture; real public references
  captured hash-only with retained artifacts; licence review and no-full-text
  gate.
- **Release and Sigstore**: candidate bundle validated on content and status
  with approvals never invented; offline verification of supplied Sigstore
  bundles and keyless issuance against injected non-production services
  behind a process boundary (ADR-0005).
- **Nomos/Praxis**: evidence exchange contract, atom mapping fixture, and a
  computed activation gate — `blocked` today, never `activated`.
- **Portfolio governance**: `nomos portfolio status|findings|reviews|projects`
  over committed machine sources; review-record index; control-plane decided
  (ADR-0007).
- **Independent roadmaps** by lane with an executable registry.

## Recorded Evidence On The Release Commit

| Evidence | Result |
|---|---|
| Portfolio status | 1 section unavailable (release candidate is a run output), 1 stale (evidence ledger dated 2026-05-02) |
| Portfolio findings | 23 open: 6 ledger gaps, 11 Praxis activation requirements, 3 blocked public captures, 3 management-review actions overdue since July 2026 |
| Repeated CI evidence (VRC-14) | 4/8 consecutive green scheduled runs — recorded as a risk, not a precondition |
| Praxis activation gate | `blocked`, 11/11 unmet |
| Competence | 0 valid attestations, 0/6 roles established |
| Candidate bundle | `v0.2.0-ALPHA-candidate` rehearsed in CI on every commit; manifest attached to the GitHub release, `approval_status: pending` inside the candidate by construction |

## Claim Boundary

Allowed bounded claims: the registered capabilities exist, are wired to a
production caller and are measured in CI; the pending regulated decisions are
enumerated and computed, not narrated; the release decision of the repository
owner is recorded with the gaps it knowingly accepts.

Not claimed:

- QMS effectiveness, validated intended use, regulated readiness, or any
  certification (the release SOP itself is not yet effective);
- production keyless signing or any public transparency-log write;
- Praxis reliance as regulated evidence;
- customer validation of any integration;
- security scanning of dependencies (planned as #678);
- a support model beyond best-effort alpha triage (planned as #679).

## Release Gate

Cut only after the release PR is merged and the following are green on the
release commit: GitHub Actions CI (Go vet/test, CUE vet, corpus tests on
three OSes, Python tests, guards, portfolio status), the release-candidate
rehearsal, the regulated documentation gate and the regulated evidence pack.
The GitHub release is a pre-release and carries: these notes, the candidate
manifest and bundle computed on the release commit, the portfolio status and
findings JSON, and the release decision record.

## Known Open Items

Regulated lane, unchanged by this release: #560, #561 (this release records
the owner's decision; the SOP execution items that remain human stay open),
#562, #192, #193, #194, #196, #638, umbrella #576. Product/DevOps: the v1.0
wave (#676–#681) is specified and open.

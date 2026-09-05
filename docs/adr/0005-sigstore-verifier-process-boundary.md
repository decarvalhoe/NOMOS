# ADR-0005 — Sigstore verification behind a versioned process boundary

**Status:** accepted (2026-09-05) · **Issues:** #637 (offline verification), #645 (keyless issuance on injected services), #576 (umbrella), #638 (production issuance, regulated lane)

## Decision

The Sigstore library (`sigstore-go`) is adopted, but it lives in a **separate Go
module**, `tools/sigstore-verifier/` (its own `go.mod`), invoked by the NOMOS
engine over a **versioned JSON protocol on stdin/stdout**
(`nomos.sigstore-verify.request.v1` / `nomos.sigstore-verify.response.v1`).
The engine (`cli/`) does not import it: its direct dependency count stays at
three and the `must_be_absent` probe forbidding `github.com/sigstore` under
`cli/**/*.go` stays green.

## Why not import it into the engine

- #637 mandates an external process boundary "so as not to multiply engine
  dependencies without necessity". `sigstore-go` pulls a large transitive
  graph (protobuf, TUF, OpenTelemetry HTTP instrumentation, cloud metadata
  clients) that the engine would carry for every user, whether or not they
  verify a bundle.
- The engine's argument for trust is its small, auditable surface. Keeping the
  Sigstore graph out of it keeps that argument true.
- The same boundary already exists twice: the rule substrate
  (`cli/internal/ruleexec/substrate.go`) and the fidelity scorer
  (`cli/internal/answer/scorer.go`). A third instance of a proven pattern
  beats a first instance of a new one.

## Why sigstore-go rather than a home-grown verifier

Signature, certificate chain, SCT, inclusion promise, inclusion proof and
checkpoint verification are exactly the code one must not write oneself. The
verifier module takes the upstream implementation and the upstream fixes; the
engine's job is to **not trust its word**: it recomputes the artifact digest,
re-matches the required identity against the response, refuses a `verified:
true` that arrives with a non-zero exit, refuses any mode other than offline,
and binds the exact response bytes by digest into the record.

## Consequences

- `nomos attest verify-sigstore` fails closed: no verifier on the machine →
  non-zero exit, no record, "no verdict". The verifier is resolved from
  `--verifier`, then `$NOMOS_SIGSTORE_VERIFIER`, then `nomos-sigstore-verifier`
  on `PATH`.
- The verifier module pins a newer Go than the engine; CI builds it with its
  own toolchain (job "Offline Sigstore verification gate").
- The protocol is versioned. A response with another schema is refused, never
  interpreted.
- Issuance (#645, #638) will reuse the same module and boundary; production
  Fulcio/Rekor writes remain forbidden by default and are a regulated-lane
  decision (#638).

## Claim boundary

NOMOS **verifies supplied bundles offline** against supplied trust material.
It does not sign with Sigstore, does not obtain identities, does not write to
any transparency log, and does not vouch for the freshness of the trust
material it is handed. The registry keeps `sigstore_keyless` **absent** and adds
`sigstore_offline_verification` as a distinct, bounded capability.

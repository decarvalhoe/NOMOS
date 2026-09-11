# ADR-VRC-0004: Product, DevOps, And Regulated Assurance Have Independent Roadmaps

## Status

Accepted — 2026-09-05.

## Context

The roadmap used one issue state to represent four different facts:

1. product or tooling implementation is delivered;
2. evidence has accumulated for long enough;
3. an accountable human has approved or signed a record;
4. a public regulated claim is unlocked.

That made work appear blocked when the product had no technical dependency at
all. Four more weekly runs, a competence assessment, acquisition of a licensed
standard, an OIDC permission, or an append-only Rekor entry cannot be produced
by writing more code. Conversely, a regulated process does not need to wait for
automation: a controlled manual review can be effective before a supporting
tool exists.

The conflation also encouraged the wrong validation sequence. A script was
sometimes treated as necessary before the process it supports could operate,
and unit tests were at risk of being read as validation of a regulated intended
use. Both directions are wrong.

## Decision

Nomos has three independent roadmaps:

| Lane | Owns | Does not own |
|---|---|---|
| `product` | CLI/runtime behavior, contracts, adapters, corpus mechanics and integrations | QMS effectiveness, human approvals, procurement |
| `devops` | CI/CD, evidence collectors, release tooling, guards and tools that may support regulated work | the regulated decision or record the tool assists |
| `regulated` | intended uses, risk assessments, manual or automated controls, QMS records, training, approvals, reference acquisition, validation and regulated claims | product implementation sequencing |

There is no phase gate across lanes. A lane may consume a versioned artifact
from another lane through an explicit interface, but it may not make the other
lane's calendar, signature, acquisition, public write, or broad claim a hard
dependency. Those facts block only the named claim or regulated use.

### Dispatch

Every live roadmap item has one dispatch state:

- `autonomous`: an agent may select and complete it;
- `passive`: evidence accumulates automatically; never wait here;
- `human`: requires an authentic accountable-human act or record;
- `external`: requires acquisition, external authority, or an irreversible
  public operation.

Only `autonomous` items enter the engineering dispatcher, and each lane selects
independently. A hard dependency may target only another autonomous item in the
same lane. Cross-lane artifacts and non-autonomous facts are recorded as
nonblocking inputs or claim gates.

The executable registry is `docs/roadmap-lanes.yaml`; CI enforces these rules
with `scripts/roadmap_lane_guard.py`.

### Risk-Based Validation Of Regulated Tools

A tool may be developed and technically verified before it is validated for a
regulated intended use. A regulated process may also start manually before the
tool exists. Tool status and process status are independent.

| Impact | Typical use | Minimum reliance before validation | Validation before authoritative use |
|---|---|---|---|
| `support` | authoring, formatting, indexing, reminders | output reviewed manually | intended use + representative functional checks |
| `evidence` | assemble or transform evidence | source/output reconciliation retained | traceability, positive/negative tests, version control, sampled independent reconstruction |
| `decision` | blocking gate, disposition or release recommendation | human verifies the decision and source evidence | approved intended use, risk assessment, requirements, adversarial tests, change control, validation summary |
| `critical_decision` | autonomous legal, clinical, safety or regulatory decision | prohibited | customer/use-specific validation and accountable approval; outside the generic product claim |

Until that validation state is reached, the tool is `supporting_use`; its output
may assist, but it is not the sole authority for a regulated decision. Tests
prove software behavior, not human competence, licence permission, regulatory
applicability, or QMS effectiveness.

### Default Decisions At External Boundaries

- Sigstore starts with offline verification of supplied bundles, then issuance
  against injected/non-production services. OIDC identity and production Rekor
  publication are separate external work.
- Licensed standards start with policy, synthetic fixtures, public surrogates,
  hashes and blocked states. Acquisition blocks only clause-level use and claims.
- Releases may be prepared and rehearsed with open risks and pending approvals.
  Tagging and publication remain authentic regulated acts.
- Competence tooling may be implemented and tested with synthetic records;
  actual assessments and independence waivers remain human records.

## Consequences

- Product and DevOps work continue whenever an autonomous item exists.
- The regulated roadmap remains honest: gaps stay open and claims stay locked,
  but they do not freeze unrelated development.
- A delivered tool does not imply an effective control; an effective manual
  control does not require a delivered tool.
- Roadmap issue closure can describe a delivered slice while a separate claim,
  evidence, human, or activation issue remains open.
- No historical approval, signature, assessment, licence decision, date, or
  external run is backfilled.

## What This ADR Does Not Do

It does not weaken a claim gate, authorize an external write, approve a
licence, sign a competence record, or declare a regulated process effective.
It changes task sequencing and evidence semantics so each of those facts is
tracked in the roadmap that actually owns it.

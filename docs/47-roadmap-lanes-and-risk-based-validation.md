# 47 — Independent Roadmaps And Risk-Based Validation

> Status: active routing policy. Date: 2026-09-05. Decision:
> [ADR-VRC-0004](adr/0004-independent-roadmaps-risk-based-validation.md).

## One Repository, Three Roadmaps

Nomos does not have one linear roadmap. It has three independent queues:

| Roadmap | Canonical plan | Dispatch rule |
|---|---|---|
| Product and integrations | [14](14-product-roadmap.md), [29](29-post-alpha-release-issue-list.md), [45](45-vision-reality-closure-plan.md) | select the first eligible `lane:product`, `dispatch:autonomous` item |
| DevOps and supporting tooling | [29](29-post-alpha-release-issue-list.md) | select the first eligible `lane:devops`, `dispatch:autonomous` item |
| Regulated assurance | [28](28-regulated-compliance-closure-plan.md) | follows its own risks, intended uses, records and claims; manual controls are allowed before tooling |

The machine-readable truth is [roadmap-lanes.yaml](roadmap-lanes.yaml). The
roadmap guard fails CI if an autonomous item depends on a passive, human, or
external item, or if a hard dependency crosses lanes.

## What Blocks What

| Fact | Blocks | Never blocks |
|---|---|---|
| Four more scheduled green runs (#560) | the claim “repeated CI evidence” | product, DevOps, release-bundle preparation |
| Competence attestations / independence waiver (#562) | training-record effectiveness claims | tool development, tests, corpus and integration work |
| Licensed standards (#192/#193/#194/#196) | their clause-level processing and claims | policy gates, public sources, synthetic fixtures, unrelated product work |
| Production OIDC/Rekor approval (#638) | production keyless/public-log claim | offline verify #637, non-production issuance #645, local ECDSA signing |
| Actual release approval/tag (#561) | “release executed via SOP” | candidate bundle preparation and rehearsal (#639) |

An open regulated gap is therefore a valid, informative state, not a dispatcher
dead end.

## Current Autonomous Queues

Each lane has its own order in `dispatch_queues`; DevOps never sits in front of
Product (or the reverse).

<!-- roadmap-queues:begin -->
<!-- GENERATED from docs/roadmap-lanes.yaml by scripts/roadmap_lane_guard.py --emit-docs; do not edit by hand, CI fails on drift -->
| Product queue | DevOps queue |
|---|---|
| — | #737 — Forge neutrality FN-1 — forge_provider boundary, sticky PR comment and CI evidence collector migrated |
| — | #738 — Forge neutrality FN-3 — opposable gates under .forgejo/workflows without marketplace steps |
| — | #736 — Forge neutrality FN-2 — regulated scripts, publisher and lane guard migrated to the provider |
| — | #739 — Forge neutrality FN-4 — provider-qualified tracker identifiers in the roadmap registry |
<!-- roadmap-queues:end -->

The dispatcher selects the first eligible item **in each lane**, skips an item
whose same-lane autonomous dependencies are not complete, and continues to the
next eligible item. It rejects dependency cycles and never waits on a
`passive`, `human`, or `external` item.

## Independent Regulated Roadmap

The regulated roadmap can proceed with manual controls, then validate tooling
when its use and impact justify it:

| Wave | Activity | Tooling relationship | Exit affects |
|---|---|---|---|
| R0 | intended uses, risk classification, owners, claim boundaries | documents/manual review sufficient | scope of regulated assurance only |
| R1 | QMS records, training, review, CAPA, release decisions | tools optional; authentic records required | process-effectiveness claims |
| R2 | reference acquisition/licence decisions and evidence accumulation | collectors may assist; missing evidence remains explicit | named reference/claim only |
| R3 | validate supporting tools according to impact (`support`, `evidence`, `decision`) | development may already be complete; reliance remains bounded until validation | authoritative use of that tool |
| R4 | customer/intended-use validation and independent reconstruction | consumes versioned product artifacts | scoped customer claim, never generic product truth |

Risk-based validation means validation effort follows intended use and impact.
It does not mean missing evidence is accepted silently. A low-risk authoring
helper may be manually checked; a blocking gate needs traceability,
adversarial tests and an approved validation record before a regulated process
relies on it alone.

## Tracker Qualification

Since registry `schema_version` 1.1.0 (ADR-0006 FN-4, #739) an issue number
is qualified by the forge that hosts it. `default_tracker` names the tracker
of every item without a `tracker` field; an item hosted elsewhere carries
`tracker: github | forgejo | gitlab`. A 1.0.0 registry is still read, with
every item on GitHub; it cannot carry the 1.1.0 fields without announcing
1.1.0.

```yaml
schema_version: "1.1.0"
default_tracker: github
items:
  - issue: 739            # #739 on the default tracker
    title: Forge neutrality FN-4 — provider-qualified tracker identifiers
    tracker: github       # optional; explicit here as the worked example
    state: open
    lane: devops
    dispatch: autonomous
    delivery_state: planned
    evidence_state: none
    claim_state: bounded
    depends_on: [737]
```

The generated queue tables print a bare `#N` for the default tracker and
`#N (forgejo)` for an item hosted on another forge, so a number is never
looked up on the wrong forge. `scripts/roadmap_lane_guard.py --verify-tracker`
reads each item on its own tracker through `scripts/forge_provider.py`: the
primary provider (`NOMOS_FORGE_PROVIDER`, `NOMOS_FORGE_REPO`) serves the
tracker of its name, any other tracker is configured by
`NOMOS_FORGE_<TRACKER>_URL`, `_TOKEN_FILE` (or `_TOKEN`) and `_REPO`, and an
item whose tracker is not configured fails by name
(`issue #N (forgejo): NOMOS_FORGE_FORGEJO_URL missing`) — never a silent skip.

## Issue Hygiene

Use tracker labels mirroring the registry, on whichever forge hosts the item:

- lane: `lane:product`, `lane:devops`, `lane:regulated`;
- dispatch: `dispatch:autonomous`, `dispatch:passive`, `dispatch:human`,
  `dispatch:external`.

Parents and nonblocking inputs do not belong in `depends_on`. A broad umbrella
must be split when one part is autonomous and another requires an external or
human act. Closing a delivered technical slice never unlocks a claim owned by
the remaining regulated issue.

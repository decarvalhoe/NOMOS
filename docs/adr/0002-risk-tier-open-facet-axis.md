# ADR-0002: `risk_tier` as a Ninth, Open-Term Facet Axis

## Status

Accepted — 2026-09-04. Issue: VRC-22 (#565). Supersedes nothing.

## Context

`specs/facets.cue` defines eight core-owned facet axes. Exactly two of them —
`discipline_role` and `activity` — are **open-term**: packs supply namespaced
terms, the core supplies the axis. The other six are closed enumerations. The
gate states the doctrine in its own failure message:

> packs own TERMS; core owns AXES

VRC-22 asks for a second real vertical — an EU AI Act evidence pack — and D6
measures how much core had to change to admit it. The honest starting position
was that it should be zero: a pack that needs core surgery proves the core is
not general.

The AI Act's central organising concept is a **risk classification**:
unacceptable, high, limited, minimal. Every obligation in the regulation hangs
off which tier a system falls into. Expressing the vertical without it is not a
simplification, it is a different pack.

None of the eight axes carries that meaning:

- `trust_tier` grades **how far an artifact may be trusted** — a property of the
  evidence, not of the regulated thing;
- `applicability` grades **whether a rule applies** — a relation, not a severity;
- `nature`, `scope_level`, `provenance`, `confidentiality` are unrelated;
- `activity` and `discipline_role` are open, but a risk tier is neither an
  activity nor a role. Forcing it onto them would put `ai_act.risque_eleve`
  next to `ai_act.evaluation_conformite` on an axis meaning "what is being
  done", which is false and would corrupt every lens built on that axis.

The remaining escape was `extensions` / `vocabulary_refs` — an untyped bag. That
would keep D6 at zero by keeping the AI Act's principal concept outside the
facet contract entirely: no lens could scope on it, no ontology mapping would
cover it, and the gate would never validate its terms. The measurement would
read 0 while the vertical was, in the part that matters most, unmodelled.

## Decision

Add `risk_tier` as a **ninth core axis, open-term**, alongside `discipline_role`
and `activity`.

The core keeps ownership of the axis list. Packs still cannot invent axes; this
is a deliberate core change, taken once, with this record.

Touched:

- `specs/facets.cue` — `#FacetAxisVocabulary.axis`, `#Facets.risk_tier`;
- `specs/knowledge-lens.cue`, `specs/domain-pack.cue`, `specs/facet-ontology.cue`;
- `cli/internal/atomization/facets.go` — struct, `IsZero`, `Validate`,
  `axisValues`, lens selection;
- `cli/internal/ragexport/{contract,lens,export}.go`;
- `cli/internal/app/pack_cmd.go` — `packOpenAxes`, pack vocabulary, lens terms.

`scripts/ckm_bundle_validate.py` and `scripts/ckm_gen_facets_vocab.py` needed no
logic change: they enumerate **scalar** axes only, and an open axis is by
construction not enumerated. The regenerated `specs/generated/facets-vocab.json`
differs only in its note — still 38 values across 6 scalar axes.

## Consequences

**D6 is published as 1 named core change, not 0.** That is the honest number for
this vertical, and it is the number VRC-22 records. A reader is entitled to know
that the second vertical cost the core one axis.

- The change is **additive and optional**. Every axis is optional in `#Facets`;
  the built-environment pack declares no `risk_tier` and is unaffected —
  `TestPackValidate_RealBuiltEnvironmentPackIsGreen` runs the shipped pack
  through the same gate on the real tree.
- The new axis gets **no softer treatment** than the two older ones: declared
  but empty fails closed, an unnamespaced term fails closed, and an ontology
  that never registers the axis fails closed.
- **The door did not open.** A pack that declares an axis of its own —
  `sector`, say — still fails with `packs own TERMS; core owns AXES`. Pinned by
  `TestPackValidate_CoreStillOwnsTheAxisList`.
- A third vertical needing a tenth axis needs another ADR. That friction is the
  point: it makes each widening of the core a written decision.

## What this ADR does not decide

It does not decide the AI Act pack's vocabulary, its golden corpus, or its claim
boundary. Those come with the pack itself, and the pack claims **pilot-grade
evidence**, never certified conformity.

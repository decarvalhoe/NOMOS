# EU AI Act Pack — pilot-grade (VRC-22, #565)

The **second** vertical. Its whole purpose is to make one claim checkable:
that a domain pack is a set of declarations, not a fork of the engine.

## What this pack is

| Piece | File |
|---|---|
| Manifest | `pack.yaml` |
| Vocabularies (3 axes, 14 terms) | `ai-act-vocabulary.yaml` |
| BFO → IOF anchoring | `facet-ontology.yaml` |
| Authority register | `eurlex-source-connectors.yaml` |
| Lens presets (3) | `ai-act-lens-presets/` |
| Golden corpus (4 docs) | `cli/internal/corpus/testdata/eu-ai-act-golden-corpus/` |

`nomos pack validate` runs it in CI, on the real chain:

```
pack validate: OK — eu-ai-act: 3 axe(s)/14 terme(s) alignés BFO→IOF,
3 preset(s) résolus, corpus doré 4 doc(s) → 32 node(s), scorecard 5 ligne(s)
```

## The two D6 numbers

D6 asks what a new vertical costs the core. Publishing one number here would
flatter the result, so both are published:

| Question | Measurement | Value |
|---|---|---|
| What does the pack, as it stands, require from core trees? | `pack_core_coupling_check.py --manifest` | **0** |
| What did it take to land it? | the PR diff, `--changed-files` | **1 core change** |

The one change is the `risk_tier` facet axis, justified in
[ADR-0002](../../../adr/0002-risk-tier-open-facet-axis.md). The AI Act
classifies by risk, none of the eight existing axes carries that meaning, and
burying it in the untyped `extensions` bag would have kept the metric at 0 by
putting the vertical's principal concept outside the facet contract entirely —
a zero bought by moving the problem out of the measurement's field of view.

So: 0 as it stands, 1 to get here. A third vertical needing a tenth axis needs
another ADR. That friction is deliberate.

## Why the golden corpus is synthetic

**Not licensing.** EUR-Lex content is reusable for commercial and
non-commercial purposes under Commission Decision 2011/833/EU, CC-BY 4.0 with
acknowledgement — verified 2026-09-04 and recorded in the source register. This
is unlike the SIA standards of the built-environment pack, which are genuinely
licence-blocked and stay hash-only.

The reason is **fidelity**. Reproducing a normative text demands word-for-word
accuracy that no unverified extraction guarantees. A near-miss quotation of a
regulation is worse than no quotation: it reads as authoritative and is wrong.
So the pack registers **where the text is authoritative** — CELEX `32024R1689`,
ELI `http://data.europa.eu/eli/reg/2024/1689/oj`, OJ L 2024/1689 of 12.7.2024,
each with the date it was resolved — and never reproduces what it says.

Every corpus document says so in its own first paragraph, and every entity in
them is marked fictitious.

## What this pack does not do

- It does not claim any AI system is compliant with Regulation (EU) 2024/1689.
- It does not classify a real system into a risk tier. `risk_tier` is a
  vocabulary the pack offers; applying it to a real system is a legal act.
- It does not assess an obligation, stand in for a conformity assessment, a
  notified body, or legal advice.
- It does not resolve point-in-time versions. The regulation applies in stages
  and a consolidated version was in force at the verification date; the pack
  dates its anchor but any dated answer would have to carry its own version.

The claim is **pilot-grade evidence pack**, and the phrase "certified
conformity" appears nowhere by design.

## The axes, and why each term sits where it does

- `activity` — what is being done: risk management, conformity assessment,
  technical documentation, post-market monitoring, registration. Processes.
- `discipline_role` — who bears the obligation: provider, deployer, importer,
  distributor, notifying authority. Roles an operator holds, not properties of
  the operator.
- `risk_tier` — how dangerous the system is: unacceptable, high, limited,
  minimal. A quality inhering in the system, which is why it is neither an
  activity nor a role, and why it needed its own axis.

The three are declared `owl:disjointUnionOf`: a term sits on exactly one.

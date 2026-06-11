# 44 - Facet Trust-Tier Policy and Vocabulary Authority

SEAM-2 (#535). This note records two decisions that govern how NOMOS facets are
derived and validated, so they are explicit policy rather than accidents of the
code.

## 1. `certified` is never auto-derived

The `trust_tier` axis (`specs/facets.cue` `#FacetTrustTier`) admits
`certified | indicative | unverified`. The NOMOS emitter's `DeriveFacets`
(`cli/internal/atomization/facets.go`) **caps derived content at `indicative`**:

- an atom in `ReviewApproved` state → `indicative`
- everything else → `unverified`
- **`certified` is never produced by atomization.**

`certified` is reachable only through an out-of-band promotion / certification
flow (CKM-03 canon promotion), where a human decision is recorded. Tagging a
freshly-atomized atom `certified` would be the exact claim-boundary violation the
audit (#519) flagged — asserting a trust level the engine cannot prove. A Go test
(`TestBuild_NodesCarryDerivedFacets`) fails if any derived node claims
`certified`.

### Downstream implication (explicit decision)

A consumer that gates on `trust_tier == "certified"` will **always** require a
human promotion decision on NOMOS-derived content — no NOMOS-emitted bundle will
satisfy that gate on its own. This is intended: certification is a governance act
the consumer (or a promotion flow) owns, not a side effect of import. Consumers
that want to admit NOMOS-derived content should gate on `indicative` (or lower)
and treat certification as a separate workflow.

## 2. NOMOS vocabulary is authoritative; consumer-local values are refused

Aedifica uses `provenance: "official"` and `nature: "regulatory"` for its **own**
OFS-direct rows. Those values are **not** in `specs/facets.cue`.

**Decision: NOMOS keeps its vocabulary authoritative. `official` / `regulatory`
are consumer-local and are NOT added to `facets.cue`.** Consequently:

- The NOMOS bundle validator (`scripts/ckm_bundle_validate.py`) **refuses** a
  bundle node carrying `provenance: "official"` or `nature: "regulatory"` —
  these are not canonical NOMOS facet values. (Proven in
  `tests/test_ckm_facets_vocab.py::test_aedifica_local_values_are_refused_by_nomos_vocab`.)
- Aedifica may carry those values on its own rows internally, but it must not
  expect to round-trip them through a NOMOS-validated bundle. If a value is
  genuinely canonical it must be added to `facets.cue` via an explicit
  schema_version bump + migration (doctrine §2.1) — not silently widened.

This keeps a single source of truth for the canonical vocabulary and prevents the
contract from drifting to accommodate one consumer's local taxonomy.

## 3. The vocabulary artifact and how to regenerate it

`specs/facets.cue` is the single source of truth. The validator and external
consumers read a generated JSON artifact rather than a hand-maintained second
copy that could drift:

- Artifact: `specs/generated/facets-vocab.json`
- Generator: `scripts/ckm_gen_facets_vocab.py` (parses the scalar-axis
  disjunctions out of `specs/facets.cue`)

Regenerate after any change to the controlled vocabularies in `facets.cue`:

```bash
python scripts/ckm_gen_facets_vocab.py
```

CI (and `scripts/ckm-non-regression.sh`) runs `--check` to fail if the committed
artifact is stale, and a test cross-checks every listed value against `cue vet`
so the artifact provably matches what the CUE contract enforces. The open
term-list axes (`discipline_role`, `activity` — free `#FacetTermRef` SKOS refs)
are intentionally not enumerated.

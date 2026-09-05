# Public Claim Boundary

This document defines what Nomos can honestly claim at `v0.1.0-ALPHA`.

Product delivery, DevOps tooling and regulated assurance have independent
statuses (ADR-VRC-0004, [roadmap router](47-roadmap-lanes-and-risk-based-validation.md)).
A green product capability or a tested supporting tool does not establish QMS
effectiveness or validation for a regulated intended use. A missing human
record, calendar run, licence decision or external activation locks its named
regulated claim; it does not make unrelated product work incomplete.

## Allowed Claims

Nomos may claim that it:

- implements a canonical-first method for source-to-product traceability;
- provides a Go CLI for project diagnosis and corpus processing;
- can scan, manifest, diff, validate sidecars, feed, produce a body ledger, run strict gates, and attest source corpora;
- has a working `rbok-lawbook` profile for structured Markdown reference corpora;
- can produce certified TOC artifacts, governed lexicon output, source-backed RAG metadata, runtime import artifacts, body-ledger evidence, release gate results, fidelity proof reports, and attestations;
- has been exercised on a real RBOK lawbook POC with 7191 feed nodes and a green strict fidelity gate;
- has been exercised on a bounded RBOK `01_rbok` source-to-feed POC with 3024 feed units, 3024 RAG chunks, 3024/3024 source-backed units/chunks, zero uncovered body-ledger bytes, and zero semantic blocking findings;
- includes regulated-readiness documentation, templates, evidence-pack automation, and a GitHub operating model.

## Conditional Claims

Nomos may claim the following only with context:

- **"Regulated-ready"** means the project has a regulated-readiness structure and evidence workflow; it does not mean certified or validated for a customer's regulated use.
- **"Full fidelity proven"** applies to a specific POC output and gate configuration, not every possible corpus and document format.
- **"Source-integrity-proven"** applies only to a recorded run whose source integrity, feed quality, semantic blocking count, body ledger, and strict gate all pass.
- **"Read-only corpus processing"** applies when the guard runs against a clean source repository and the source mutation check passes.
- **"RAG-ready"** means traceable metadata exists; it does not mean production vector-store retrieval and LLM behavior have been validated.

## Prohibited Claims At This Release

Do not claim that Nomos is:

- FDA validated;
- 21 CFR Part 11 compliant as a platform;
- EU Annex 11 compliant as a platform;
- GxP validated;
- ISO certified;
- NASA qualified;
- a complete eQMS;
- a substitute for customer validation;
- universally faithful across all PDFs, DOCX, images, scanned documents, legal codes, regulations, or game-rule books without corpus-specific evidence;
- legally authorized to redistribute licensed standards;
- cleared for trademark use or registration without a recorded trademark clearance decision;
- backed by a freedom-to-operate opinion for RAG governance, patent-search automation, or source-grounded retrieval;
- cleared for unrestricted third-party license use without a recorded third-party license clearance review.

## Evidence Rule

Every public claim must map to one of:

- a passing automated gate;
- a generated artifact;
- a reviewed document;
- a controlled decision;
- a known gap with owner and next action.

If a claim cannot be mapped, it must be removed or rewritten as a roadmap item.

## What NOMOS Proves Today

At `v0.1.0-ALPHA`, NOMOS proves only:

- that it can read a source corpus, build a manifest, and detect drift between source and recorded hashes;
- that it can generate corpus feed artifacts, certified TOC, governed lexicon output, source-backed RAG metadata, runtime import artifacts, body-ledger evidence, fidelity proof reports, and attestations from the configured profile;
- that, on the specific RBOK lawbook POC corpus and configuration, the existing strict fidelity gate has reported `full_fidelity_proven` for the recorded run;
- that, on the recorded RBOK `01_rbok` source-to-feed POC (`C:\Dev\nomos-rbok-poc-run-20260504-structured-universal-9`), source integrity, feed quality, body ledger, and strict gate passed with zero semantic blocking findings.

These are all **artifact-generation** facts and **POC-scoped** facts. They are not, on their own, a platform-wide source-to-feed fidelity proof.

## What NOMOS Does Not Yet Prove

NOMOS does **not** yet prove, at platform scope:

- complete source-to-feed coverage across arbitrary corpora;
- absence of all semantic warnings across arbitrary corpora;
- support for every PDF, DOCX, scanned document, image, legal code, regulation, rule book, or customer-specific format;
- customer-specific regulated validation, intended-use approval, supplier qualification, or certification;
- trademark clearance for the NOMOS name or related wordmarks;
- a freedom-to-operate opinion for implementation or public claims;
- third-party license clearance for dependencies that require OSS or counsel review.

The remaining proof chain is release-scoped: repeated CI evidence on private corpora, broader document-format adapters, and customer validation packs.

Attestation `claim_coverage` is wired (VRC-07 #553): `nomos corpus attest --corpus-body-ledger` verifies the ledger's Merkle inclusion proofs (recomputing every leaf from the ledger rows) and computes `claim_coverage` from the verified ledger; `nomos corpus body-ledger --verify` exposes the same verification standalone. A tampered ledger fails the attestation (adversarial tests in tree). This is a tree capability claim; recorded POC dossiers keep their original run-scoped statements.

RAG interop export is wired (#614): `nomos rag export` emits chunk records any RAG stack can index and later cite (`embedding_text` carries a deterministic structural context prefix; `body_text` is the citable source text and never contains that prefix), refusing any chunk without `chunk_id`, `source_id`, `source_hash`, or body; `nomos rag manifest` fingerprints the export per source and per chunk so index staleness is provable against the source hash; `nomos rag delta` turns two manifests into the exact per-chunk plan (embed / update_metadata / delete) and `nomos rag verify` gates an index manifest against the current feed (exit 1 when stale; a hand-edited manifest whose digest does not match its own chunk list is refused as a baseline). Exports are byte-deterministic (no wall clock is read). With `--bundle`, records carry the node facets; `--lens` enforces a Knowledge Lens on the corpus handed to the index (excluded chunks are named and counted; a facet-less chunk under a lens is excluded), the manifest binds the index to the lens digest, a lens change is a full reindex, and the manifest carries a computed retrieval contract that declares temporal scoping unsupported (no record carries effective dates). Adversarial tests are in tree, and `scripts/rag-export-gate.sh` replays determinism, prefix-leak absence, 1-byte staleness detection, the fresh/stale/tampered verify verdicts on the public reference corpus, and the lens verdicts on the real AEC golden bundle against an independent re-implementation of the consumer-kit semantics, in CI. This is a tree capability claim about the export and scope contracts only; Nomos does not rank, and it claims nothing about retrieval quality, embedding behaviour, or LLM answers, which remain governed by the cite-or-abstain gate and the eval harness.

The cite-or-abstain gate's faithfulness proxy is lexical and negation-blind (stated in every verdict). `nomos answer gate|eval --scorer-cmd` (#622) accepts an external NLI scorer as a second judge through a versioned JSON protocol; the verdict is strictest-wins per sentence (a scorer can only tighten the gate, never loosen it) and any scorer failure, timeout or off-contract response fails the answer closed with `FAITHFULNESS_SCORER_FAILED`. `scripts/nomos_hhem_scorer.py` is the reference adapter for HHEM-2.1-Open. No model ships in the Go engine (a wiring-matrix probe keeps it that way) and no CI run scores with a neural model: the claim covers the protocol, the composition direction and the fail-closed behaviour, not NLI accuracy on any corpus.

The RAG answer evidence record consumes that verdict instead of computing its own (#624): `scripts/regulated_rag_answer_evidence.py` runs `nomos answer gate` on the fixtures and takes every score, decision, finding and threshold from the engine, keeping only the evidence envelope (record fields, response contract, confidence range, unique answer ids). In `required` mode (the default, what CI runs) an unavailable or non-verdict engine exits 2 and writes no report at all — a stale one is removed — so no evidence record claims a gate result the engine did not produce. The `fallback` mode is explicit, marked `python_fallback`, warned, capped at the `indicative` tier, and used by no CI gate (a wiring-matrix probe keeps it out of the shell gates). The claim covers verdict provenance and fail-closed behaviour; it is not a claim that any particular answer was correctly judged.

The cite-or-abstain gate is measured by a public bench (#582, VRC-46): `nomos answer bench` runs the gate over a labelled corpus built on the in-repo public reference-basis documents (nine items, one per failure mode: grounded, forged citation, no span text, negation, over-verbosity, prompt injection, unsupported question) and reports the two error directions separately — `false_cite_rate` (an ungrounded answer published as source-backed, the dangerous error) and `must_cite_recall` (over-abstention). The published result of 2026-09-04, lexical proxy only: `must_cite_recall` 1.0, `must_abstain_recall` 0.8333, `false_cite_rate` 0.1667 — the single miss is the negation item, the documented blind spot of the lexical proxy, published as a miss. `scripts/cite_or_abstain_bench.py` replays the measurement in CI and fails on any drift: a moved source document, a quote that is not verbatim in its source, a cited reference without a dated verification record, a non-deterministic run, a broken bound, or a published number that no longer matches. This is a measurement of the gate over nine public items; it claims nothing about retrieval quality, LLM accuracy, NLI-model accuracy (no CI run scores with a model), or the business correctness of any answer.

Repeated CI evidence on the private corpus is measured, not asserted (#560, VRC-14). `scripts/repeated_ci_evidence.py` reads the scheduled-run history of the RBOK lawbook E2E workflow from the GitHub Actions API and publishes a dated index of every run it counted, under `docs/regulated/evidence-index/repeated-ci-evidence/`. A run counts as one unit of evidence only if it completed green AND still holds an unexpired archived pack: a green run whose pack has aged out leaves nothing to re-inspect, so the chain decays on its own unless runs keep coming. A missed weekly occurrence breaks the chain, because runs that happened cannot vouch for weeks that produced nothing. Measured on 2026-09-04: 4 consecutive green runs against a target of 8, seven scheduled runs recorded and all seven green, five missed weekly occurrences between 2026-06-29 and 2026-08-10, and only two distinct corpus revisions across the whole recorded window — so what is proven so far is that the pipeline kept running green over a largely unchanging corpus snapshot, not that it holds across evolving inputs. **The claim is therefore locked**: the evidence ledger carries `EV-CI-REPEAT-001` as `requires_evidence` and `GAP-REPEATED-CI-EVIDENCE` as open, and the gate fails if any document in the tree states the claim while the measurement says otherwise, if the workflow loses its schedule or lowers its artifact retention, or if the published numbers stop matching a replay of the recorded runs.

A second domain vertical is shipped and gated (#565, VRC-22): `docs/regulated/domain-packs/eu-ai-act/` registers Regulation (EU) 2024/1689 as an authority anchor (CELEX `32024R1689`, ELI, OJ reference, each dated to the day it was resolved) and provides vocabularies for the obligation-bearing roles, the compliance activities, and the four risk tiers — 14 terms across 3 axes, each anchored BFO→IOF, the three axes declared disjoint. `nomos pack validate` runs the pack in CI on the real chain (3 presets resolved, a 4-document golden corpus producing 32 nodes), the same unchanged gate that runs the built-environment pack. The generality this evidences is that a vertical is a set of declarations rather than a fork of the engine; the D6 coupling metric is published as **two** numbers, because one would flatter it: 0 core paths required by the pack as it stands, and 1 core change to get there — the `risk_tier` facet axis, justified in ADR-0002. The pack is **pilot-grade**: it claims no conformity with the AI Act, classifies no real system into a risk tier, assesses no obligation, and substitutes for no conformity assessment, notified body or legal advice. Its golden corpus is **synthetic and says so in every document**; the regulation's text is reusable under Commission Decision 2011/833/EU, so the reason is fidelity rather than licensing — a near-miss quotation of a normative text reads as authoritative and is wrong, so the pack records where the text is authoritative and never what it says.

Cross-references between atoms are traced by rules, never generated (#579, VRC-43): `nomos atomize crossref` parses seven legally distinct cues — subject to, without prejudice to, notwithstanding, by derogation from, in accordance with, within the meaning of, and see-also — and emits each edge with the verbatim text it was built from and the half-open offsets it occupies in the atom. `VerifyCrossRefGraph` re-slices the atom and refuses any edge whose bytes do not reconstruct, any resolved edge whose destination is not in the atom set, and any unsupported entry with no reason; the engine verifies its own output before emitting it. A cue with no parsable target is recorded as `unsupported` rather than dropped, because silence would let a reader believe the rule stands alone, and a locator matching no atom stays `unresolved_target` with the locator kept verbatim rather than attached to a plausible neighbour. The three outcomes — resolved, unresolved, unsupported — are published separately. This claims that the text says what the edge records; it claims nothing about whether a cross-reference is legally correct, currently in force, or exhaustive, and no model participates at any point.

Facet vocabularies are validated as SKOS/RDF plus NOMOS predicates with standard SHACL (#580, VRC-44): `scripts/facet_shacl_gate.py` emits each pack's graph and validates it against `specs/shacl/facet-ontology.shapes.ttl` in CI using pySHACL. The shapes are portable SHACL, but the recorded execution is pySHACL only; no Apache Jena/TopBraid equivalence is claimed until a cross-engine record exists, and the graph does not claim to emit OWL `disjointUnionOf`. The shapes require a scheme that declares which axes are disjoint (declaring none would make orthogonality unfalsifiable, and that case is itself refused), an axis anchored to exactly one BFO root and one IOF class as real IRIs and carrying at least one term, and a concept with exactly one preferred label, scheme, axis, BFO mapping, IOF mapping and a pack-namespaced id. Orthogonality is a SPARQL constraint because it relates three nodes: a term sitting on two axes declared disjoint fails, naming the term and both axes. Both shipped verticals conform (built-environment 2 axes/10 concepts, eu-ai-act 3 axes/14 concepts). This claims conformance of the emitted graph to the shapes; it claims nothing about whether the BFO and IOF anchors chosen for a term are ontologically apt. The validation slice #580 is closed; static authoring/distribution is tracked separately in #643, while VocBench/Skosmos remain optional and ISO 25964 remains outside the current claim.

Computable atoms run on a borrowed substrate, and NOMOS builds no rule engine (#578, VRC-42): `nomos rule exec` hands each ```formula block, verbatim and with its source trace, to an external process over a versioned JSON protocol, and records what comes back. `cli/internal/ruleexec` contains no evaluator, no arithmetic and no expression parser; with no substrate configured nothing is produced at all — never a default, never a zero — and a substrate that is absent, crashed, timed out or off-contract fails closed with no partial record. A result for an atom nobody asked about, a duplicate answer, a missing answer, a `computed` with no value, an `unsupported` with no reason or carrying a value: each is refused. Every value travels with the expression it came from and its canonical ref, file and line range. The process boundary is what the licence register's `process_api_boundary` policy means in practice: NOMOS spawns a process and imports, links and vendors nothing; no engine is named in the Go code, and the reference adapter imports no third-party module. What is shipped is the boundary and a protocol-conformance fixture that evaluates one deliberately trivial arithmetic shape and answers `unsupported` for everything else — **no real rule engine is integrated**, no compatibility with L4/OpenFisca/Catala is claimed, and nothing here opines whether a boundary satisfies the AGPL. Persisted-result integrity remains bounded by #642: until request digests and returned values are fully bound and tamper-tested, do not treat a stored record as independently authenticated merely because the live protocol run passed.

## Claim Levels

NOMOS public claims are organised in increasing assurance order. Each level may only be advertised when the level below it is satisfied.

| Level | Meaning | Gating |
|---|---|---|
| `artifact-generated` | NOMOS produced the artifact (feed, attestation, fidelity proof report, TOC, lexicon, RAG metadata) without crashing and with the documented schema. | Existing `validate` and `canonical:check` gates. Active today. |
| `source-traced` | Generated nodes carry source spans and resolve to a recorded source manifest entry. | Source span emission and manifest hash check. Active today on `rbok-lawbook` profile feeds. |
| `source-integrity-proven` | The corpus integrity report for this build is present and passes (coverage, duplicate spans, junk content, feed linkage, RAG linkage). | Active for recorded POC runs that carry the passing evidence pack; not a platform-wide claim. |
| `full-fidelity-proven` | Source-integrity-proven AND the corpus integrity check is wired into the blocking strict release gate for this build. | Active only for recorded POC runs where the strict gate evidence explicitly supports it; not a platform-wide claim. |

A build may not advertise a level until the gating evidence for that level is recorded and passing. POC-scoped results may carry the corresponding level only with explicit corpus and configuration scoping.

## Reserved Phrases

The following phrases are **reserved** and may only be written by NOMOS or about NOMOS where the cited evidence exists:

- `full_fidelity_proven` — requires a present and passing corpus integrity report wired into the strict release gate (`#346`). Today this phrase is permitted only as the recorded result of the legacy RBOK POC strict fidelity gate, scoped to that POC; it must not be lifted to platform-wide marketing claims.
- `full fidelity proven` / `complete fidelity` — same gating as above; not permitted as a platform-wide claim.
- `validated` (in a regulated sense) — reserved for customer-specific validation against intended use; NOMOS itself is not validated as a platform.
- `certified` — reserved for evidence of an external certification body decision; NOMOS is not certified.
- `compliant` (as a platform) — reserved for an explicit compliance decision against a named framework with a recorded record of evidence; not used for NOMOS at this release.
- `regulated-ready` — permitted with the documented meaning ("regulated-readiness structure and evidence workflow"), never as a synonym for "regulated".

Every emitter of these phrases (release notes, README, attestation text, marketing copy, CLI output) must check this list and the corresponding claim level before publishing.

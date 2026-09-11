# Public Cite-Or-Abstain Bench

document_id: NOMOS-BENCH-COA-001
version: 0.1.0
status: published (measured, reproducible)
issue: VRC-46 (#582), exit of workstream A (docs/45 §2)
claim_boundary: "Measurement of the cite-or-abstain gate over a labelled public corpus; no retrieval-quality, LLM-accuracy, NLI-accuracy, legal, clinical or regulatory claim."

## What Is Measured

The bench measures the **gate** (`nomos answer gate`, VRC-10): given an answer
record that already carries its retrieved spans and its citations, does the
gate reach the decision the record was built to require — `cite` when the
answer is fully grounded and citable, `abstain` when it must not be published
as-is?

Nothing else is in the loop. No retrieval is run, no embedding is computed, no
LLM produces text, no network is used. The items are answer records
constructed by hand, one per failure mode, and labelled by construction. The
bench therefore says how the gate behaves on those failure modes; it says
nothing about how often a given RAG system produces them.

## Corpus

`corpus.yaml` — nine items over two public sources that live in this
repository (`docs/regulated/reference-basis/README.md` and
`nomos-bible-corpus-policy.md`). Every quoted span is verbatim text of its
source; every source is declared with its sha256; no licensed content, no
invented data.

| Category | Items | Expected | What the item probes |
|---|---|---|---|
| `grounded` | 3 | cite | A sentence quoted from the source, cited on the retrieved chunk |
| `forged_citation` | 1 | abstain | The citation binds to a hash that was never retrieved |
| `no_span_text` | 1 | abstain | Identifiers only: no span text the claim can be verified against |
| `negation` | 1 | abstain | The answer contradicts its source while sharing its content tokens |
| `over_verbosity` | 1 | abstain | One grounded sentence plus one the source does not support |
| `prompt_injection` | 1 | abstain | A refusal of an injected instruction (asserts nothing) |
| `unsupported_question` | 1 | abstain | A legitimate "I cannot answer" refusal |

Label discipline: an item without a usable `expected_decision` is a bench
defect, never a measurement. The engine excludes it from every average and
fails the run.

## Metrics

The measurement is asymmetric on purpose: the two error directions do not cost
the same.

| Metric | Definition | Direction |
|---|---|---|
| `must_abstain_recall` | must-abstain items blocked / must-abstain items | safety |
| `false_cite_rate` | must-abstain items cited / must-abstain items | **the dangerous error**: an ungrounded answer published as source-backed |
| `must_cite_recall` | must-cite items cited / must-cite items | usability: over-abstention is a defect too |
| `missed_cites` | must-cite items blocked | usability |
| `agreement` | items whose decision equals the label / items | overall, never used alone |

`false_cite_rate` is reported on its own and per category; it is never folded
into a single accuracy number. The citation metrics inside every verdict
(`citation_recall`, `citation_precision`) follow the ALCE definitions
(`references.yaml`, REF-ALCE-2023).

## Protocol

```bash
# 1. Measure (the engine; byte-deterministic, no wall clock read)
nomos answer bench --corpus docs/regulated/ai-rag-governance/cite-or-abstain-bench/corpus.yaml \
  --thresholds docs/regulated/ai-rag-governance/cite-or-abstain-bench/bench-thresholds.yaml

# 2. Replay the published result and refuse any drift (what CI runs)
python3 scripts/cite_or_abstain_bench.py --root .

# 3. Re-publish a dated result after a change that legitimately moves the numbers
python3 scripts/cite_or_abstain_bench.py --root . --publish

# 4. Resolve the cited references live again before any external use (network)
python3 scripts/cite_or_abstain_bench.py --root . --verify-references
```

The replay gate checks, in order: every source document is unmoved (declared
sha256 = actual) and every quote is verbatim in it; every cited reference
carries a dated verification record; the engine emits identical bytes on two
runs; the versioned bounds of `bench-thresholds.yaml` hold; the measurement
equals the published `results-<date>.json` (measurement block, corpus digest,
sources, threshold values). Any failure is named. A stale or non-reproducible
tree is never published.

## Published Results

| Date | Engine | Configuration | Items | must_cite_recall | must_abstain_recall | false_cite_rate | Missed cites | File |
|---|---|---|---|---|---|---|---|---|
| 2026-09-05 | nomos 0.1.0-ALPHA | lexical proxy, no scorer | 9 | 1.0 (3/3) | 0.8333 (5/6) | 0.1667 (1/6) | 0 | `results-2026-09-04.json` |
| 2026-09-04 | nomos 0.1.0-ALPHA | lexical proxy, no scorer | 9 | 1.0 (3/3) | 0.8333 (5/6) | 0.1667 (1/6) | 0 | `results-2026-09-04.json` |

Per category (2026-09-05, identical to 2026-09-04): `grounded` 3/3 cited; `forged_citation`,
`no_span_text`, `over_verbosity`, `prompt_injection`, `unsupported_question`
1/1 blocked each; `negation` 0/1 blocked — the single false cite.

## The Known Miss, And The Second Judge

The gate's faithfulness proxy is lexical and negation-blind by construction:
"Process public bibles from official snapshots … is not required" shares its
content tokens with the source that says the opposite. The bench publishes
this as a false cite instead of hiding it. The gate accepts a second judge
(`--scorer-cmd`, #622, strictest-wins per sentence, fail-closed), and the Go
test of the bench (`TestBench_TheSecondJudgeMovesTheMeasurement`) proves with
an injected judge that the negation item flips to blocked. No neural model
runs in CI, so no scorer result is published here; to reproduce one with the
open reference model (REF-HHEM-2.1-OPEN):

```bash
nomos answer bench --corpus docs/regulated/ai-rag-governance/cite-or-abstain-bench/corpus.yaml \
  --scorer-cmd "python3 scripts/nomos_hhem_scorer.py" --scorer-threshold 0.5
```

Such a run is a measurement of that model's judgement on these items, not a
NOMOS claim about the model's accuracy.

## Claim Boundary

- The bench measures the gate over nine public items, one per failure mode.
  It is a probe of the method, not an evaluation of a product, and its size
  is stated rather than inflated.
- It measures no retrieval, no embedding, no LLM: LegalBench-RAG
  (REF-LEGALBENCH-RAG-2024) benchmarks the retrieval step of legal RAG; this
  bench is not comparable to it and does not claim to be.
- It claims nothing about the business, legal, clinical or regulatory
  correctness of any answer.
- It claims nothing about any NLI model's accuracy; the published result is
  lexical only.
- Every number above is replayed in CI by `scripts/cite_or_abstain_bench.py`;
  a number that stops replaying turns the build red.

## References

See `references.yaml`: every reference carries its official URL, DOI where it
exists, and the dated record of the verification made before publication
(doc 41: sources are re-verified before any external use). The name "bench"
follows the practice of publishing a reproducible protocol with dated results
(docs/42 §C2); no external benchmark is cited as a source of the numbers.

## Change Log

- 0.1.1 (2026-09-05): re-publication after `nomos-bible-corpus-policy.md` gained two policy steps (#641); the quoted spans are intact, only the source digest moved; every number identical to 0.1.0.
- 0.1.0 (2026-09-04): first publication — corpus of 9 items, lexical result,
  replay gate in CI (#582).

# AI And RAG Governance

This folder is reserved for AI-assisted extraction, RAG, and agent governance records.

Nomos must treat AI output as assistance, not authority. Product law requires source-backed canonical units, deterministic validation, and human or gate-controlled review where risk requires it.

## Required Controls

- deterministic extraction has precedence over LLM extraction;
- generated claims cite source IDs and source hashes;
- prompt-injection and unsafe-output cases are tested;
- low-confidence output becomes `needs_review`, not product law;
- model/provider/version and prompt templates are recorded when relevant;
- RAG answers preserve citations and refusal behavior;
- human review status is retained for critical or ambiguous units;
- the inference boundary is declared per data class (docs/49 §2.3): public
  corpora may be screened through external model APIs for model selection,
  customer, privileged or witness-related data never leave the deployment, and
  the two flows are never mixed in one pipeline run.

`scripts/regulated_rag_answer_evidence.py` emits
`.regulated-evidence-pack/rag-answer-evidence.json` from
`rag-answer-fixtures.yaml`. The gate blocks any answer marked acceptable unless
it has source-backed citations or an explicit refusal/unsupported state.

The sidecar CONSUMES the verdict of the Go engine, it does not produce one
(#624, VRC-10 A1): it runs `nomos answer gate --fixtures …` and takes from the
verdict the citation metrics, the faithfulness, the trust score and tier, the
cite/abstain decision, the gate findings and the thresholds (`gates`) the batch
was judged against. It keeps only what the engine cannot know: the evidence
envelope (required record fields, response contract, confidence range, unique
`answer_id`s). `--engine required` (the default, what CI runs) refuses to score:
no engine, a crash, a timeout, non-JSON output or a verdict that cannot be
aligned with the fixtures exits 2 and leaves no report — a stale report at the
output path is removed. `--engine fallback` is the explicit, marked PARTIAL path
(`engine.verdict_source: python_fallback`, a `RAG_GATE_VERDICT_FROM_PYTHON_FALLBACK`
warning, every trust tier capped at `indicative`); no CI gate uses it. The
engine is located with `--nomos-bin`, else `$NOMOS_BIN`, else `go run .` in
`cli/`, else `nomos` on PATH. `--scorer-cmd` is forwarded to the engine (#622),
so an external NLI judge reaches the evidence record without any model in the
sidecar.

`nomos answer eval --corpus rag-eval-corpus.yaml --thresholds rag-eval-thresholds.yaml`
is the CI harness (VRC-13). Each golden answer declares `expected_chunk_ids`,
the ground-truth chunks for its prompt, so the harness also computes
`context_recall`, rank-weighted `context_precision` and `noise_sensitivity`
(answer sentences supported by distractor chunks only). The thresholds file
carries the versioned bounds; bump them deliberately, never to paper over a
regression, and never set a context bound on a corpus that declares no
expectations (the harness fails closed).

Both `answer gate` and `answer eval` accept an external faithfulness scorer
(`--scorer-cmd`, #622): a second judge over (support text, answer sentence)
pairs through the versioned JSON protocol `nomos-scorer-request-v1` /
`nomos-scorer-response-v1`. The verdict is strictest-wins per sentence (a
scorer can only tighten the gate) and any scorer failure fails the answer
closed with `FAITHFULNESS_SCORER_FAILED`. `scripts/nomos_hhem_scorer.py` is
the reference adapter for HHEM-2.1-Open; the CI harness itself stays lexical
and no CI run scores with a neural model.

`cite-or-abstain-bench/` is the public bench of the gate (VRC-46, #582): a
labelled corpus quoting the in-repo public reference-basis documents
(`corpus.yaml`), versioned bounds (`bench-thresholds.yaml`), the references
the methodology cites with their verification date (`references.yaml`), the
methodology (`README.md`) and dated results (`results-<date>.json`).
`nomos answer bench` measures; `scripts/cite_or_abstain_bench.py` replays the
published result in CI and turns red on any drift (moved source, non-verbatim
quote, unverified reference, non-determinism, broken bound, edited number).
`--publish` writes a new dated result: do it in the same change that
legitimately moves the numbers, never to hide a regression.

`scripts/regulated_ai_provider_ledger.py` emits
`.regulated-evidence-pack/ai-provider-change-ledger.json` from
`ai-provider-change-ledger.yaml`. The regulated documentation gate blocks
provider/model changes that preserve domain claims unless impact assessment is
complete.

Use `templates/regulated/ai-rag-governance.md` as the first controlled record.

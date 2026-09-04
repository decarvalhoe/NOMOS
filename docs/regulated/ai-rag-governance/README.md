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
- human review status is retained for critical or ambiguous units.

`scripts/regulated_rag_answer_evidence.py` emits
`.regulated-evidence-pack/rag-answer-evidence.json` from
`rag-answer-fixtures.yaml`. The gate blocks any answer marked acceptable unless
it has source-backed citations or an explicit refusal/unsupported state.

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

`scripts/regulated_ai_provider_ledger.py` emits
`.regulated-evidence-pack/ai-provider-change-ledger.json` from
`ai-provider-change-ledger.yaml`. The regulated documentation gate blocks
provider/model changes that preserve domain claims unless impact assessment is
complete.

Use `templates/regulated/ai-rag-governance.md` as the first controlled record.

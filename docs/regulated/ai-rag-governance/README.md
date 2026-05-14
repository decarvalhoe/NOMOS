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

`scripts/regulated_ai_provider_ledger.py` emits
`.regulated-evidence-pack/ai-provider-change-ledger.json` from
`ai-provider-change-ledger.yaml`. The regulated documentation gate blocks
provider/model changes that preserve domain claims unless impact assessment is
complete.

Use `templates/regulated/ai-rag-governance.md` as the first controlled record.

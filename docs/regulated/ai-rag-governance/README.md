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

Use `templates/regulated/ai-rag-governance.md` as the first controlled record.

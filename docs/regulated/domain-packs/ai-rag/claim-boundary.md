# Claim Boundary — `ai-rag` Domain Pack

This page bounds public claims about the **`ai-rag`** domain pack
when it is installed on top of Nomos. It does not replace the
project-wide [public claim boundary](../../../public-claim-boundary.md);
it adds pack-specific allowed and prohibited claims on top of it.

## Scope

The `ai-rag` pack is an alpha customer-install surface for Nomos's
AI/RAG governance evidence. It targets customers who run AI-assisted
extraction or RAG over a corpus and need traceable, source-cited,
review-able output rather than free-form LLM responses.

## Pack Allowed Claims

The pack supports the following claims when (and only when) the
customer's signed validation checklist is on file for the environment
being claimed about:

- "The customer's AI/RAG governance record is structured to the
  Nomos AI/RAG control list and is reachable from the customer's
  release-evidence bundle."
- "The customer's RAG output retains source IDs and source hashes for
  every generated unit covered by the customer's adopted record."
- "Low-confidence output is routed to `needs_review` rather than
  treated as product law."
- "Prompt-injection and unsafe-output test cases from the customer's
  named corpus are exercised against the customer's deployment."
- "Model, provider, and version metadata are recorded for any unit
  produced with LLM assistance."

Every allowed claim is conditional. The conditional clause must
appear next to the claim. Recommended pattern:

> "in the customer's environment described by signed validation
> checklist `<checklist-id>`, with references at versions
> `<reference-versions>`, on Nomos `v0.1.0-alpha`."

## Pack Conditional Claims

- **"`ai-rag`-ready"** means the pack's structure is installed in the
  customer repository and the validation checklist passes for the
  environment claimed about. It does not mean certified, validated,
  or regulator-approved.
- **"AI/RAG governance evidence assembled"** means the pack's gates
  pass and the AI/RAG control coverage row in the validation
  checklist is complete. It does not mean the customer's AI system
  has been independently audited.

## Pack Prohibited Claims

In addition to every prohibited claim in the project-wide public
claim boundary, the `ai-rag` pack must never be used to support the
following:

- "Part 11 e-signature platform" or "Part 11 compliant AI platform".
- "FDA validated AI" or "FDA approved AI".
- "EU AI Act conformity certified" or any equivalent regulator
  conformity claim.
- "Bias-free AI", "fair AI", "non-discriminatory AI" without a
  customer-owned bias evaluation evidence record outside the pack.
- "Guaranteed safe AI" or "hallucination-free RAG".
- "Validated LLM" or "qualified model".
- "Customer's AI system certified by Nomos" or similar.
- Any claim that closing DOR-021, installing the `ai-rag` pack, or
  passing the regulated documentation gate authorises a regulated
  decision in the customer's context.

## Evidence Rule

Every public statement about the `ai-rag` pack must map to one of:

- a passing automated gate from the install guide
  ([`install-guide.md`](install-guide.md));
- a generated artefact recorded in the validation checklist
  ([`validation-checklist.md`](validation-checklist.md));
- a reviewed document under `docs/regulated/`;
- a controlled decision under `docs/regulated/decisions/`;
- a known gap with owner and next action.

If a public statement cannot be mapped to one of the above, it must
be removed or rewritten as a roadmap item.

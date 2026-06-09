# NOMOS Preliminary FTO Note

Status: `claim_chart_required_before_public_claim`

This preliminary FTO screen is not a freedom-to-operate opinion. It is not legal advice and does not replace patent counsel review.

## Scope

The current risk scope is RAG governance, source-grounded retrieval, citation or abstention controls, trust scoring, provenance attestation, corpus fidelity gates, and audit evidence for AI-assisted extraction.

This note exists because RAG governance and patent-search automation are active patent landscapes. The project must not claim "patent clear", "free to operate", or equivalent language until patent counsel completes a formal review.

## Controls

Required before public freedom-to-operate claims:

- claim chart required for any patent family identified as plausibly relevant;
- patent counsel review of product claims, workflows, and implementation boundaries;
- recorded search strategy across at least WIPO PATENTSCOPE, Google Patents, and jurisdiction-specific patent registers selected by counsel;
- engineering review showing that implementation choices come from public standards, original design, permissive open source, or independent product requirements.

Engineering hygiene:

- do not implement from patent text;
- do not copy claim language into product requirements as an implementation recipe;
- prefer open standards and original interface contracts;
- isolate high-risk third-party engines behind process or API boundaries;
- record every public assertion in the public claim boundary before release.

## Blocked Claims

The following claims are blocked until this note is replaced by counsel-approved clearance:

- "NOMOS has freedom to operate";
- "RAG governance patent clearance is complete";
- "no relevant patents exist";
- "patent-safe implementation";
- "patent search complete".

Permitted wording is limited to: "A preliminary FTO screen identified patent-review tasks; no freedom-to-operate opinion has been completed."

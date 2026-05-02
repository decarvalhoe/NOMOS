# Quality Risk Management SOP

document_id: NOMOS-QMS-SOP-003
version: 0.1.0
status: draft
effective_status: not_effective
owner: not_assigned
approver: not_assigned

## Purpose

Define risk-based controls for Nomos documentation, software lifecycle, corpus conversion, AI/RAG use, evidence generation and release claims.

## Risk Dimensions

Assess every feature or control by:

- patient/user/business/regulatory impact;
- data integrity impact;
- authority-source impact;
- security/privacy impact;
- detectability of failure;
- reversibility;
- automation level;
- use of LLM/RAG or generated content;
- customer validation dependency.

## Risk Levels

| Level | Meaning | Required evidence |
|---|---|---|
| low | Advisory or non-authoritative feature. | review or automated check with rationale. |
| medium | Can affect engineering decisions or customer confidence. | automated tests and review. |
| high | Can affect source-to-product law, release evidence, security or records. | scripted tests, traceability, review, deviation handling. |
| critical | Can create false compliance, mutate authority sources, or corrupt evidence. | independent review, strict CI gate, release approval, no open critical deviations. |

## Risk Acceptance

Risk acceptance must include:

- risk ID;
- description;
- level;
- affected intended use;
- mitigation;
- residual risk;
- owner;
- expiry/review date;
- public claim impact.

Unowned risk is not accepted.

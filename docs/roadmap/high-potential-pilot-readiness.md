# High-Potential Pilot Readiness Matrix

This matrix closes the HPF roadmap packaging gap by ranking the candidate
pilot lanes, naming the gates that make each lane defensible, and preserving
the public claim boundary. It is a planning and evidence-readiness artifact
only. It makes no certification, no customer validation, no legal sufficiency,
no regulatory approval, no medical-device claim, no financial-regulatory claim,
no security-audit claim, and no compliance claim.

Canonical machine-readable record:
[`docs/roadmap/high-potential-pilot-readiness.yaml`](high-potential-pilot-readiness.yaml).

## Decision

The first two pilot lanes are:

1. `ai_rag_governance`
2. `gxp_csv`

They are first because they already have the strongest combination of bounded
claim language, repository evidence, and automated gates. Finance/RegTech,
Medical/SaMD, legal/eDiscovery, and the portfolio control plane remain
important, but they need customer-owned evidence, licensed references, legal
review, or product packaging before external pilot commitment.

## Status Semantics

| Status | Meaning |
| --- | --- |
| `go` | Repository evidence, claim boundary, and at least one automated gate are present for a bounded pilot conversation. |
| `wait` | Repository evidence exists, but customer-owned execution, licensed references, external reviewers, or product packaging must land before external pilot commitment. |
| `blocked` | A non-repository dependency is missing and cannot be resolved by a code change alone. |

## Ranked Lanes

| Rank | Lane ID | Lane | Issue | Status | Evidence gate or dependency | Claim impact |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | `ai_rag_governance` | AI/RAG governance pilot pack | #473 | `go` | Domain profile, install guide, claim boundary, answer fixtures, provider ledger, regulated docs gate. | Source-backed AI/RAG governance evidence only; no AI certification or regulatory sufficiency claim. |
| 2 | `gxp_csv` | GxP/CSV pilot evidence pack | #472 | `go` | Domain profile, GxP/CSV crosswalk, ALCOA evidence contract, IQ/OQ/PQ skeletons, evidence pack. | Evidence planning for customer review only; no GxP, Part 11, or validated-system claim. |
| 3 | `verifiable_evidence` | Verifiable evidence product layer | #476 | `go` | Domain profile, evidence contract, mechanism decision, generated evidence pack. | Hash-based reconstruction and future signing hooks only; no legal signature or audit acceptance claim. |
| 4 | `cyber_supplier_assurance` | Cyber supplier assurance profile | #475 | `go` | Cyber supplier profile, assurance pack, branch-protection and GitHub operating-model evidence. | Supplier security review evidence only; no security certification or customer supplier qualification claim. |
| 5 | `github_app_workflow` | GitHub App and workflow productization | #477 | `go` | Workflow setup, readiness boundary, CUE contract, publish/comment scripts and tests. | Bounded workflow publication only; no QMS approval replacement or validated workflow claim. |
| 6 | `control_plane_portfolio` | Portfolio control plane MVP | #478 | `wait` | Export-first portfolio evidence model exists; dashboard UI, tenant authz, and workers are deferred. | Portfolio supervision records only; no production dashboard or ALM/QMS replacement claim. |
| 7 | `finance_regtech` | Finance, DORA, and RegTech profile | #475 | `wait` | Finance profile and references exist; customer ICT risk, third-party, disclosure, and legal-review evidence are missing. | Finance evidence-gap mapping only; no DORA, SEC, FINRA, MiCA, disclosure approval, or legal advice claim. |
| 8 | `medical_samd` | Medical/SaMD readiness boundary | #474 | `wait` | Medical/SaMD profile exists; licensed ISO 13485 and ISO/IEC/IEEE 12207 intakes remain open via #192 and #193. | Medical/SaMD planning boundary only; no medical-device, clinical validation, FDA clearance, or ISO certification claim. |
| 9 | `legal_ediscovery` | Legal/eDiscovery readiness boundary | #470 | `blocked` | Requires customer counsel, matter boundary, privilege policy, legal hold rules, retention policy, and acceptance criteria. | Planning fixture only; no legal advice, privilege determination, discovery compliance, or court acceptance claim. |

## Verification

The readiness matrix is guarded by a contract test that checks the
machine-readable record, validates required lanes, verifies referenced
repository artifacts exist, and enforces the no-overclaim boundary:

```bash
python -m unittest tests.test_hpf_readiness -v
```

The matrix also names the regulated documentation and evidence-pack gates used
by downstream pilot conversations:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
```

# 38 - NOMOS Domain Opportunity Roadmap

Date: 2026-09-05
Status: actionable strategic backlog
Scope: NOMOS only; Praxis is mentioned only as downstream evidence dependency

## Purpose

This document turns the current NOMOS alpha state, regulated-readiness
documentation, and external market scan into an actionable development
roadmap.

The objective is not to claim that NOMOS is already compliant,
certified, or validated. The objective is to identify the strongest
regulated and high-integrity markets where the NOMOS authority-to-product
model can become valuable, then convert those opportunities into
implementation issues with clear evidence gates.

Per [ADR-VRC-0004](adr/0004-independent-roadmaps-risk-based-validation.md), this is
a product/domain-planning roadmap. Public and synthetic fixtures, profiles,
contracts and blocked-state tooling proceed without licensed acquisitions or
human approvals. Those external facts live on roadmap 28 and block only a
claim that uses the named source at clause level. A planning profile that
honestly emits `blocked` is not waiting for the blocked evidence.

## Method

The scan used only public or official sources for active planning. Licensed
standards remain prerequisites when clause-level processing is required,
but they must be handled through sidecars and no-full-text redistribution
controls.

Primary source families reviewed:

- FDA 21 CFR Part 11 electronic records and electronic signatures:
  https://www.ecfr.gov/current/title-21/chapter-I/subchapter-A/part-11
- EU GMP EudraLex Volume 4 Annex 11 computerized systems:
  https://health.ec.europa.eu/system/files/2016-11/annex11_01-2011_en_0.pdf
- FDA Computer Software Assurance for production and quality-system
  software:
  https://www.fda.gov/regulatory-information/search-fda-guidance-documents/computer-software-assurance-production-and-quality-system-software-0
- FDA Quality Management System Regulation, effective 2026-02-02, aligned
  with ISO 13485 by incorporation by reference:
  https://www.fda.gov/medical-devices/postmarket-requirements-devices/quality-management-system-regulation-qmsr
- IMDRF Software as a Medical Device clinical evaluation:
  https://www.imdrf.org/documents/software-medical-device-samd-clinical-evaluation
- NIST AI Risk Management Framework:
  https://www.nist.gov/itl/ai-risk-management-framework
- NIST Generative AI Profile:
  https://www.nist.gov/publications/artificial-intelligence-risk-management-framework-generative-artificial-intelligence
- EU AI Act:
  https://digital-strategy.ec.europa.eu/en/policies/regulatory-framework-ai
- ISO/IEC 42001 AI management systems:
  https://www.iso.org/standard/42001
- NIST SP 800-218 Secure Software Development Framework:
  https://csrc.nist.gov/pubs/sp/800/218/final
- NIST Cybersecurity Framework 2.0:
  https://www.nist.gov/cyberframework
- EU DORA Regulation 2022/2554:
  https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:32022R2554
- EU MiCA Regulation 2023/1114:
  https://eur-lex.europa.eu/legal-content/EN/TXT/?uri=CELEX:32023R1114
- SEC cybersecurity disclosure rules:
  https://www.sec.gov/newsroom/press-releases/2023-139
- FINRA Regulatory Notice 24-09 on AI and member obligations:
  https://www.finra.org/rules-guidance/notices/24-09
- EDRM model:
  https://edrm.net/edrm-model/current/
- ASQ DMAIC process:
  https://asq.org/quality-resources/dmaic
- ISPE GAMP 5, Second Edition overview:
  https://ispe.org/publications/guidance-documents/gamp-5-guide-2nd-edition
- MHRA GxP data integrity guidance:
  https://www.gov.uk/government/publications/guidance-on-gxp-data-integrity
- NASA software engineering requirements resources:
  https://www.nasa.gov/intelligent-systems-division/software-management-office/nasa-software-engineering-procedural-requirements-standards-and-related-resources/
- W3C Verifiable Credentials Data Model 2.0:
  https://www.w3.org/TR/vc-data-model-2.0/
- C2PA Content Credentials technical specification:
  https://spec.c2pa.org/specifications/specifications/2.2/specs/C2PA_Specification.html
- RFC 9162 Certificate Transparency Version 2.0:
  https://www.rfc-editor.org/rfc/rfc9162

## Current Project State

NOMOS is currently an alpha product with real proof, not a regulated
product.

Verified project position:

- `v0.1.0-ALPHA` established a working CLI and corpus pipeline.
- The RBOK lawbook POC produced feed, TOC, lexicon, RAG metadata, runtime
  import, body ledger, strict gate output, fidelity proof, and attestation.
- GitHub workflow integration exists for source-owned and output-owned
  corpus processing, including artifact, pull-request, and direct-push
  publication modes with trace manifest requirements.
- Regulated-readiness documentation exists under `docs/regulated/`.
- Public claim boundaries are explicit in `docs/public-claim-boundary.md`.
- The control matrix is still `not_qualified`: owners, approvals, live
  GitHub evidence, training records, licensed-reference closure, and
  independent reconstruction evidence remain incomplete.

Roadmap state as of 2026-09-05:

| Issue | Role / state |
|---|---|
| `#382`, `#314` | Fidelity/AQ foundations — closed, historical inputs. |
| `#408`, `#409`, `#410`, `#411` | Qualification/CAPA foundations — closed; their records remain regulated evidence, not product dependencies. |
| `#320` | Nomos/Praxis compatibility foundation — closed; regulated reliance remains separately bounded. |
| `#192`, `#193`, `#194`, `#196` | Open regulated acquisition/licence/processing claim gates; no domain-development dependency. |
| product queue of `docs/roadmap-lanes.yaml` | Current autonomous product work — see the generated table in `docs/47`. |

Strategic conclusion:

NOMOS must now add domain profiles and evidence packs without weakening
the core principle:

```text
NOMOS admits what it can prove, refuses what it cannot prove, and records the exact evidence boundary.
```

## Domain Opportunity Scan

### 1. GxP, CSV, CSA, And Life Sciences Quality

Market problem:

- Regulated teams must qualify software, prove electronic-record
  trustworthiness, maintain audit trails, manage validation evidence, and
  defend data integrity.
- GAMP 5 and FDA CSA push the market toward risk-based validation rather
  than document volume for its own sake.
- Annex 11 and Part 11 create strong expectations for lifecycle,
  validation, security, audit trails, supplier assessment, record
  protection, and retrievability.

NOMOS opportunity:

- Convert regulations, SOPs, quality manuals, validation plans, and
  corporate policies into canonical controls and evidence requirements.
- Generate supplier packs, validation packs, IQ/OQ/PQ templates, ALCOA+
  evidence envelopes, and release evidence bundles.
- Support customer validation rather than claiming to replace it.

Product components to build:

- `domain_profile: gxp-csv`
- Part 11 / Annex 11 / CSA / GAMP 5 control crosswalk.
- ALCOA+ artifact envelope enforcement.
- Risk-based validation planner.
- IQ/OQ/PQ and supplier-assurance pack generator.

Claim boundary:

NOMOS may support GxP/CSV evidence preparation. It must not claim GxP
validation or Part 11 compliance as a platform without customer-specific
intended use, validation, approvals, and operational controls.

### 2. Medical Device, SaMD, And Health Software

Market problem:

- FDA QMSR now aligns the U.S. medical-device quality system regulation
  with ISO 13485 by incorporation by reference.
- SaMD requires intended-use, risk, safety/effectiveness/performance,
  clinical evaluation, lifecycle traceability, and post-market controls.
- AI-enabled medical software creates additional model, data, and change
  control issues.

NOMOS opportunity:

- Map medical-device source authorities to requirements, risk controls,
  test evidence, clinical-evaluation rationale, and release decisions.
- Produce a device-software evidence pack from source documents and
  product artifacts.

Product components to build:

- `domain_profile: medical-samd`
- Intended-use and risk trace model.
- SaMD clinical-evaluation evidence mapping.
- QMSR/ISO 13485/IEC 62304-style lifecycle evidence bundle.
- AI-SaMD provider/model change-control support.

Claim boundary:

NOMOS may help organize medical software evidence. It must not claim to
make a device compliant or clinically validated.

### 3. AI Governance, RAG, And Model Risk

Market problem:

- EU AI Act, NIST AI RMF, NIST Generative AI Profile, and ISO/IEC 42001
  push organizations toward risk management, governance, monitoring,
  transparency, human oversight, documentation, and evaluation.
- RAG systems need source integrity, citation discipline, prompt-injection
  controls, and provider/model change control.

NOMOS opportunity:

- Become the evidence layer for AI knowledge governance: what sources a
  system can use, how they were transformed, how answers cite them, and
  what confidence/refusal policy applies.
- Support AI technical-file preparation for scoped systems.

Product components to build:

- `domain_profile: ai-governance`
- AI reference crosswalk: EU AI Act, NIST AI RMF, NIST GenAI Profile,
  ISO/IEC 42001.
- RAG answer evidence contract.
- Prompt-injection and citation/refusal evaluation pack.
- Model/provider inventory and change-control ledger.

Claim boundary:

NOMOS may govern source-backed AI/RAG evidence. It must not claim that an
LLM answer is authoritative just because NOMOS generated metadata.

### 4. Finance, RegTech, DORA, Cyber Disclosure, And Crypto

Market problem:

- DORA requires ICT risk management, third-party risk management,
  incident reporting, resilience testing, and governance for EU financial
  entities.
- SEC cybersecurity disclosure rules increase board/governance and
  incident materiality documentation pressure.
- FINRA has warned firms that AI use must still comply with existing
  securities obligations.
- MiCA introduces structured obligations around crypto-asset services and
  disclosures.

NOMOS opportunity:

- Convert finance policies, operational resilience rules, AI policies,
  vendor obligations, crypto disclosure obligations, and internal control
  libraries into traceable controls.
- Produce evidence packs for operational resilience and model governance.

Product components to build:

- `domain_profile: finance-regtech`
- DORA ICT risk and third-party control map.
- SEC/FINRA AI and cybersecurity disclosure evidence pack.
- MiCA disclosure/reference corpus profile.
- Regulated change-impact report for finance corpora.

Claim boundary:

NOMOS may support RegTech evidence organization. It must not claim legal
or regulatory compliance outcomes for a financial institution.

### 5. Legal, eDiscovery, Policy, And Contract Intelligence

Market problem:

- Legal teams manage authorities, policies, contracts, opinions,
  evidence, retention, privilege, custody, and citation integrity.
- eDiscovery workflows need chain-of-custody and defensible processing.
- AI legal assistants face high hallucination and citation-risk pressure.

NOMOS opportunity:

- Build authority-backed legal corpora where every summary, argument,
  policy obligation, or contract clause links back to a source span.
- Support legal memo evidence, policy-to-contract mapping, and document
  retention/custody records.

Product components to build:

- `domain_profile: legal-ediscovery`
- Citation integrity and authority hierarchy.
- Chain-of-custody and retention metadata.
- Policy-to-contract trace matrix.
- AI legal-assistant source boundary.

Claim boundary:

NOMOS may support source-backed legal evidence and review workflows. It
must not provide legal advice or certify legal sufficiency.

### 6. Six Sigma, Operational Excellence, CAPA, And Quality Improvement

Market problem:

- Organizations need to connect customer needs, CTQs, defects, process
  metrics, deviations, root cause, CAPA, and control plans.
- Six Sigma/DMAIC is a recognizable process-improvement frame, but many
  organizations lose traceability between problem statements, controls,
  metrics, and evidence.

NOMOS opportunity:

- Turn process documentation, SOPs, defects, CAPAs, and metrics into
  controlled improvement evidence.
- Connect NOMOS findings to CAPA trend analytics and continuous
  improvement roadmaps.

Product components to build:

- `domain_profile: six-sigma-capa`
- DMAIC/CTQ/control-plan model.
- Finding-to-CAPA taxonomy.
- Deviation trend analytics.
- Management review evidence summary.

Claim boundary:

NOMOS may support Six Sigma-style evidence and improvement governance. It
must not claim certified Six Sigma results.

### 7. Blockchain, Provenance, Transparency Logs, And Verifiable Evidence

Market problem:

- High-integrity digital evidence needs tamper-evidence, signing,
  verifiable provenance, and independent reconstruction.
- W3C Verifiable Credentials, C2PA content credentials, and
  certificate-transparency style append-only logs are useful design
  references for verifiable evidence.

NOMOS opportunity:

- Make NOMOS evidence packs independently verifiable using hashes,
  signatures, credentials, and append-only transparency records.
- Avoid hype: blockchain is optional infrastructure, not the authority.

Product components to build:

- `domain_profile: verifiable-evidence`
- Signed evidence bundle format.
- Optional W3C VC wrapper for release/evidence credentials.
- Optional C2PA-compatible content provenance for human documents.
- Append-only transparency log or compatibility layer.

Claim boundary:

NOMOS may provide tamper-evident and verifiable evidence. It must not
claim that blockchain proves semantic correctness.

### 8. Cybersecurity, Secure SDLC, And Supplier Assurance

Market problem:

- Secure software development, vulnerability management, SBOMs, branch
  protection, release provenance, incident response, and supplier
  evidence are expected by regulated and enterprise buyers.
- NIST SSDF and NIST CSF provide a strong vocabulary for supplier and
  customer assurance.

NOMOS opportunity:

- Produce a software supplier assurance pack that ties source code,
  dependencies, controls, tests, release notes, and security findings to
  evidence.

Product components to build:

- `domain_profile: cyber-supplier-assurance`
- SSDF and CSF control map.
- SBOM/provenance integration.
- Vulnerability and incident evidence ledger.
- Customer security questionnaire evidence export.

Claim boundary:

NOMOS may support security assurance evidence. It is not a replacement
for SOC 2, ISO/IEC 27001 certification, penetration testing, or customer
security review.

### 9. Aerospace, Defense, And High-Assurance Engineering

Market problem:

- High-assurance software requires requirements traceability,
  verification evidence, configuration control, independent review, risk
  handling, and long retention of evidence.
- NASA software engineering requirements are a public and useful high-bar
  reference.

NOMOS opportunity:

- Use aerospace-style traceability as a strict product-quality benchmark,
  even before entering regulated aerospace markets.
- Support evidence-pack generation for high-criticality projects.

Product components to build:

- `domain_profile: high-assurance-engineering`
- Requirements, hazard/risk, verification, waiver, and release-decision
  chain.
- Independent review reconstruction pack.
- Tailoring matrix for high-rigor intended uses.

Claim boundary:

NOMOS may align to high-assurance practices. It must not claim NASA,
DO-178C, defense, or mission qualification without formal program-specific
evidence.

## Cross-Domain Capabilities Required

The domain opportunities above share the same technical substrate.

| Capability | Why it matters | Existing base | Gap |
|---|---|---|---|
| Domain profile framework | Keeps NOMOS universal while allowing domain-specific rules. | Corpus profiles exist. | Need profile schema, domain applicability, claim ladder, reference packs. |
| Portable fidelity engine | Prevents RBOK-only proof. | `#382`, `#314`, FSQ/AQ work. | Need multi-domain golden corpus and portable gate. |
| Reference bible governance | Allows public/licensed/private sources safely. | `#192`, `#193`, `#194`, `#196`. | Need no-full-text gates and reference-to-control closure per domain. |
| Evidence envelope | Makes outputs ALCOA+/audit-friendly. | Regulated docs and attestations exist. | Need enforced envelope on all outputs and release bundles. |
| AI/RAG evidence contract | Makes AI consumption bounded and source-backed. | RAG metadata exists. | Need answer trace, prompt/model/provider evaluation and refusal/citation tests. |
| GitHub workflow productization | Makes NOMOS deployable on customer corpora. | NGW shipped. | Need domain profiles inside workflow config and risk-based publish policy. |
| ALM/QMS interop | Lets NOMOS integrate instead of replacing enterprise systems. | ADR exists for ReqIF/ALM boundaries. | Need export adapters and evidence mapping. |
| Verifiable evidence | Lets reviewers reconstruct and verify without trust in narrative. | Attestations and hashes exist. | Need signing, transparency log, and verification CLI. |
| Control plane | Needed for multi-corpus, multi-client, and periodic review. | `nomos portfolio` views exists. | Need portfolio registry and dashboard scope. |

## Release Train Recommendation

### v0.2 - Fidelity Closure

Historical inputs: `#382` and `#314` are closed; they are not live dispatch
dependencies.

Nonblocking regulated evidence inputs: `#408`, `#409`, `#410`, `#411`.
They affect qualification claims, not the product release.

Outcome:

- Short critical atom reconciliation is implemented.
- RBOK POC evidence is rerun and bounded.
- Claim boundary is updated without overclaiming.

### v0.3 - Domain Profile Foundation

Outcome:

- Domain profile schema exists.
- Domain claim ladder and applicability scorecard exist.
- Multi-domain golden corpus exists for GxP, medical, AI, finance,
  legal, Six Sigma, provenance, cyber, and high-assurance fixtures.
- Each domain can produce mapped, blocked, not-applicable, or waived
  controls.

### v0.4 - Reference Tooling And Public Provenance

No licensed-acquisition dependency. Issues `#192`, `#193`, `#194`, `#196`
are regulated claim gates for their named clause-level use.

Outcome:

- Public/licensed/private reference handling is governed.
- No full licensed text is committed.
- Reference-to-control matrix produces domain evidence boundaries.

### v0.5 - AI/RAG Governance Pack

Outcome:

- RAG answers are evaluated for citation, refusal, concision,
  source-linking, and prompt-injection resistance.
- Provider/model changes are controlled.
- AI-generated extraction never becomes authority without review.

### v0.6 - GitHub App And Workflow Productization

Outcome:

- Reusable workflow supports domain profiles and risk-based publication
  modes.
- GitHub App readiness becomes an implementation lane.
- Trace manifests remain mandatory in artifact, PR, and direct-push
  modes.

### v0.7 - Domain Packs

Outcome:

- First domain packs ship as scoped, non-certified packages:
  `gxp-csv`, `ai-governance`, `cyber-supplier-assurance`,
  `legal-ediscovery`, and `finance-regtech`.

### v0.8 - Verifiable Evidence

Outcome:

- Release/evidence bundles can be hashed, signed, verified, and optionally
  recorded in an append-only transparency log.

### v0.9 - Control Plane

Outcome:

- Multi-corpus portfolio registry tracks domain, reference version,
  evidence status, open findings, periodic review, and release claims.

### v1.0 - Scoped Production Candidate

Outcome:

- NOMOS supports scoped intended uses with supplier pack, validation pack,
  security pack, customer integration guide, evidence bundle, and claim
  boundary reviewed against actual gates.

## Nuclear Issue List

These issue candidates are intentionally small enough to assign, verify,
and close without relying on narrative. GitHub issues created from this
table should copy the dependency, deliverables, definition of done,
verification, and claim impact.

GitHub materialization on 2026-05-08:

| ID | GitHub issue |
|---|---|
| DOR-EPIC | `#412` |
| DOR-001 | `#413` |
| DOR-002 | `#414` |
| DOR-003 | `#415` |
| DOR-004 | `#416` |
| DOR-005 | `#417` |
| DOR-006 | `#418` |
| DOR-007 | `#419` |
| DOR-008 | `#420` |
| DOR-009 | `#421` |
| DOR-010 | `#422` |
| DOR-011 | `#423` |
| DOR-012 | `#424` |
| DOR-013 | `#425` |
| DOR-014 | `#426` |
| DOR-015 | `#427` |
| DOR-016 | `#428` |
| DOR-017 | `#429` |
| DOR-018 | `#430` |
| DOR-019 | `#431` |
| DOR-020 | `#432` |
| DOR-021 | `#433` |
| DOR-022 | `#434` |
| DOR-023 | `#435` |

### DOR-EPIC - Regulated Domain Expansion And Opportunity Roadmap

Dependency: existing alpha baseline, `docs/14-product-roadmap.md`,
`docs/15-product-backlog.md`, `docs/public-claim-boundary.md`.

Deliverables:

- Maintain this roadmap.
- Track all DOR child issues.
- Keep public README and claim boundaries aligned with delivered proof.

Done when:

- Every DOR issue is either delivered, explicitly deferred, or superseded
  by a more precise issue.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Claim impact:

- Planning only. No new compliance claim.

### DOR-001 - Domain Profile Schema And Contract

Dependencies: `#382`, portable fidelity backlog.

Deliverables:

- Define `specs/nomos-domain-profile.cue`.
- Support domain id, intended use, references, applicability, risk class,
  claim ladder, required artifacts, blocked claims, and validation gates.
- Add valid examples for `gxp-csv`, `ai-governance`, and `legal-ediscovery`.

Done when:

- Valid fixtures pass CUE validation.
- Invalid fixture proves that unsupported compliance claims are rejected.

Verification:

```bash
cue vet specs/nomos-domain-profile.cue specs/examples/nomos-domain-profile.gxp.valid.yaml -d '#DomainProfile'
cue vet specs/nomos-domain-profile.cue specs/examples/nomos-domain-profile.ai.valid.yaml -d '#DomainProfile'
```

Claim impact:

- Enables domain-scoped planning; does not prove domain compliance.

### DOR-002 - Domain Claim Ladder And Applicability Scorecard

Dependencies: DOR-001.

Deliverables:

- Define claim levels per domain: `registered`, `mapped`, `evidence_ready`,
  `validated_by_customer`, `independent_review_ready`.
- Emit `domain-applicability-report.json`.
- Fail when a public/domain claim exceeds evidence level.

Done when:

- Report distinguishes applicable, not applicable, blocked, waived, and
  missing evidence.
- README wording can be checked against report status.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Claim impact:

- Prevents overclaim in vertical-market materials.

### DOR-003 - Reference Intake Policy For Public, Licensed, And Private Bibles

Dependencies: none. Issues `#192`, `#193`, `#194`, `#196` are regulated inputs
for actual licensed use, not prerequisites for the policy or its gates.

Deliverables:

- Extend `docs/regulated/reference-basis/nomos-bible-corpus-policy.md`.
- Add a machine-readable classification for public, licensed, private,
  confidential, and customer-owned references.
- Add a gate that blocks full-text redistribution when policy forbids it.

Done when:

- A licensed reference can be hashed and processed read-only without full
  text being committed.
- A private/customer reference can be recorded with access policy and
  retention obligations.

Verification:

```bash
python scripts/regulated_reference_canon.py --report .regulated-doc-gate/reference-canon-report.json
```

Claim impact:

- Supports regulated reference handling without license misuse.

### DOR-004 - Multi-Domain Golden Corpus Pack

Dependencies: DOR-001, `#382`.

Deliverables:

- Add small, license-safe fixtures for GxP, medical/SaMD, AI governance,
  finance, legal/eDiscovery, Six Sigma/CAPA, provenance, cyber, and
  high-assurance engineering.
- Each fixture includes expected source structures and expected
  unsupported records.

Done when:

- Every fixture produces feed, TOC, lexicon or explicit no-lexicon status,
  body ledger, fidelity report, and strict gate output.

Verification:

```bash
cd cli
go test ./internal/corpus -run Portable -v
```

Claim impact:

- Moves NOMOS beyond RBOK-only confidence.

### DOR-005 - GxP/CSV Control Crosswalk

Dependencies: DOR-001, DOR-003. Issues `#194` and `#196` gate only a GAMP 5
clause-level claim; `blocked` is the correct planning output before then.

Deliverables:

- Create `domain_profile: gxp-csv`.
- Map 21 CFR Part 11, Annex 11, FDA CSA, MHRA data integrity, and GAMP 5
  references to NOMOS controls.
- Mark licensed GAMP 5 clause-level processing blocked until license
  review permits it.

Done when:

- Each reference is `mapped`, `blocked`, `not_applicable`, or `waived`.
- No decorative authority remains.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Claim impact:

- Enables GxP/CSV evidence planning; no GxP compliance claim.

### DOR-006 - ALCOA+ Evidence Envelope Enforcement

Dependencies: DOR-005.

Deliverables:

- Add required ALCOA+ fields to generated domain evidence artifacts:
  actor/tool/version, command, timestamp, source commit, source hash,
  artifact hash, derivation, exclusions, and retention hint.
- Gate missing envelopes in regulated/domain outputs.

Done when:

- Domain evidence artifacts cannot pass without required ALCOA+ metadata.

Verification:

```bash
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
```

Claim impact:

- Strengthens data-integrity evidence for regulated buyers.

### DOR-007 - CSA Risk-Based Validation Planner

Dependencies: DOR-005, DOR-006.

Deliverables:

- Generate a risk-ranked validation plan from domain profile controls.
- Classify functions by criticality and required verification type.
- Emit `risk-based-validation-plan.json`.

Done when:

- High-risk controls require scripted or challenge evidence.
- Low-risk controls can justify lighter evidence.

Verification:

```bash
python -m unittest discover -s tests -v
```

Claim impact:

- Supports FDA CSA-style validation planning without claiming validation.

### DOR-008 - IQ/OQ/PQ Template Generator By Intended Use

Dependencies: DOR-007. Issues `#408`, `#409`, `#410` are nonblocking
regulated evidence inputs for actual qualification.

Deliverables:

- Generate intended-use-specific IQ/OQ/PQ templates from domain profile.
- Reuse RBOK-NOMOS qualification lessons without making them RBOK-only.

Done when:

- Template output distinguishes CLI-only, GitHub workflow, output-repo,
  control-plane, and downstream-RAG deployments.

Verification:

```bash
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
```

Claim impact:

- Supports customer validation-package preparation.

### DOR-009 - Medical/SaMD Evidence Profile

Dependencies: DOR-001, DOR-003. Issues `#192` and `#196` gate only ISO 13485
clause-level use; the profile and its explicit blocked placeholders proceed.

Deliverables:

- Create `domain_profile: medical-samd`.
- Map QMSR/ISO 13485, SaMD clinical evaluation, intended use, risk, and
  software lifecycle evidence.
- Keep ISO 13485 full text out of Git unless license permits.

Done when:

- SaMD profile emits intended-use, risk, clinical-evaluation, and
  requirement/test trace placeholders with blocked states where evidence
  is missing.

Verification:

```bash
cue vet specs/nomos-domain-profile.cue specs/examples/nomos-domain-profile.medical-samd.valid.yaml
```

Claim impact:

- Enables medical-device evidence planning; no medical compliance claim.

### DOR-010 - AI Governance Profile

Dependencies: DOR-001, DOR-004.

Deliverables:

- Create `domain_profile: ai-governance`.
- Crosswalk EU AI Act, NIST AI RMF, NIST GenAI Profile, and ISO/IEC 42001.
- Add controls for model inventory, provider inventory, prompt boundary,
  evaluation datasets, human review, and monitoring.

Done when:

- Profile can declare whether a system is advisory, RAG-only, AI-assisted
  extraction, autonomous agent, or high-risk candidate.

Verification:

```bash
cue vet specs/nomos-domain-profile.cue specs/examples/nomos-domain-profile.ai.valid.yaml
```

Claim impact:

- Supports AI governance readiness planning.

### DOR-011 - RAG Answer Evidence Pack

Dependencies: DOR-010.

Deliverables:

- Emit `rag-answer-evidence.json`.
- Include answer id, prompt id, model/provider/version, retrieved chunks,
  source spans, citation status, refusal status, confidence, and policy
  outcome.
- Add prompt-injection, citation, refusal, and over-verbosity fixtures.

Done when:

- A RAG answer cannot be marked acceptable without source-backed
  citations or an explicit refusal/unsupported state.

Verification:

```bash
python -m unittest discover -s tests -v
```

Claim impact:

- Makes downstream AI behavior auditable without making LLM output
  authoritative.

### DOR-012 - Model And Provider Change-Control Ledger

Dependencies: DOR-010, DOR-011.

Deliverables:

- Emit `ai-provider-change-ledger.json`.
- Track provider, model, region, data-use policy, API version, prompt
  template version, evaluation baseline, and approval state.

Done when:

- Provider/model changes require impact assessment before domain claims
  are preserved.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Claim impact:

- Supports AI supplier and model risk governance.

### DOR-013 - Finance/RegTech Domain Profile

Dependencies: DOR-001, DOR-004.

Deliverables:

- Create `domain_profile: finance-regtech`.
- Map DORA ICT risk, SEC cybersecurity disclosure, FINRA AI notice, and
  MiCA reference families to NOMOS evidence controls.

Done when:

- Profile separates ICT risk, third-party risk, cyber disclosure,
  AI-supervision, and crypto-disclosure evidence.

Verification:

```bash
cue vet specs/nomos-domain-profile.cue specs/examples/nomos-domain-profile.finance.valid.yaml
```

Claim impact:

- Enables finance evidence planning; no regulatory compliance claim.

### DOR-014 - Legal/eDiscovery Domain Profile

Dependencies: DOR-001, DOR-004.

Deliverables:

- Create `domain_profile: legal-ediscovery`.
- Model source authority, citation integrity, chain-of-custody, retention,
  privilege marker, policy-to-contract trace, and unsupported legal-advice
  claim boundary.

Done when:

- Legal fixture proves citation/source-span integrity and custody metadata
  without creating legal-advice claims.

Verification:

```bash
cue vet specs/nomos-domain-profile.cue specs/examples/nomos-domain-profile.legal.valid.yaml
```

Claim impact:

- Enables legal corpus governance and eDiscovery-adjacent evidence.

### DOR-015 - Six Sigma And CAPA Analytics Profile

Dependencies: DOR-001. Issue `#411` is a nonblocking regulated CAPA record,
not a prerequisite for the analytics profile.

Deliverables:

- Create `domain_profile: six-sigma-capa`.
- Model DMAIC, CTQ, defect taxonomy, deviation, root cause, CAPA action,
  control plan, trend, and management-review summary.

Done when:

- NOMOS findings can be grouped into CAPA/improvement categories with
  traceable source and evidence context.

Verification:

```bash
python -m unittest discover -s tests -v
```

Claim impact:

- Supports operational excellence and CAPA analytics.

### DOR-016 - Verifiable Evidence Profile

Dependencies: DOR-006, DOR-012.

Deliverables:

- Create `domain_profile: verifiable-evidence`.
- Define signed evidence bundle fields.
- Evaluate W3C Verifiable Credentials, C2PA, and RFC 9162-style
  transparency logs as implementation options.

Done when:

- A decision record selects which verifiability mechanisms NOMOS will
  implement first and which remain optional.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Claim impact:

- Improves evidence trustworthiness; does not prove semantic correctness.

### DOR-017 - Evidence Signing And Verification CLI

Dependencies: DOR-016.

Deliverables:

- Add CLI command to hash, sign or prepare for signing, and verify NOMOS
  evidence bundles.
- Keep unsigned mode available but marked weaker.

Done when:

- Verification fails when an artifact hash changes after signing.

Verification:

```bash
cd cli
go test ./internal/app -run Evidence -v
```

Claim impact:

- Supports independent reconstruction and tamper-evidence.

### DOR-018 - Cyber Supplier Assurance Profile

Dependencies: DOR-001, DOR-006.

Deliverables:

- Create `domain_profile: cyber-supplier-assurance`.
- Map NIST SSDF and NIST CSF controls to NOMOS supplier evidence.
- Include SBOM, vulnerability, incident, branch protection, release
  provenance, and customer questionnaire evidence.

Done when:

- Supplier pack output distinguishes implemented, manual, blocked, and
  customer-owned controls.

Verification:

```bash
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
```

Claim impact:

- Supports enterprise security review and supplier qualification.

### DOR-019 - High-Assurance Engineering Profile

Dependencies: DOR-001, DOR-004.

Deliverables:

- Create `domain_profile: high-assurance-engineering`.
- Map NASA-style lifecycle, requirements, verification, independent
  review, waiver, and release-decision evidence.

Done when:

- Fixture demonstrates requirement-to-verification-to-waiver traceability.

Verification:

```bash
cue vet specs/nomos-domain-profile.cue specs/examples/nomos-domain-profile.high-assurance.valid.yaml
```

Claim impact:

- Raises engineering rigor without claiming aerospace qualification.

### DOR-020 - ALM/QMS Export Adapter Decision

Dependencies: DOR-001, DOR-002.

Deliverables:

- Decide supported export targets for first implementation: GitHub issues,
  ReqIF, JSON schema, CSV, Jira, or QMS import templates.
- Record non-goals and evidence loss risks.

Done when:

- ADR defines first adapter and blocked/deferred adapters.
- Chosen adapter has a fixture.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Claim impact:

- Positions NOMOS as an integration layer rather than pretending to
  replace ALM/QMS platforms immediately.

### DOR-021 - Domain Pack Packaging And Customer Install Guide

Dependencies: DOR-001, DOR-005, DOR-010, DOR-018.

Deliverables:

- Define how a customer installs a domain pack.
- Include required references, licensed-reference policy, workflow config,
  expected outputs, gates, and claim boundary.

Done when:

- At least one domain pack has an install guide and validation checklist.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Claim impact:

- Makes domain packs marketable without overclaiming.

### DOR-022 - Multi-Corpus Control Plane Opportunity

Dependencies: DOR-002, DOR-006, DOR-020.

Deliverables:

- Define control-plane MVP for portfolio view: corpus, domain profile,
  source version, open findings, claim level, release status, periodic
  review, and evidence bundle.
- Keep CLI-first mode as supported baseline.

Done when:

- Control-plane roadmap separates required evidence model from optional
  UI/dashboard.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Claim impact:

- Creates enterprise product direction while preserving CLI credibility.

### DOR-023 - Commercial Positioning And Pricing Evidence Pack

Dependencies: DOR-021.

Deliverables:

- Compare NOMOS positioning against ALM, validation lifecycle management,
  QMS, RAG governance, and RegTech categories.
- Define non-misleading packaging: CLI, workflow, domain pack, supplier
  pack, validation-support pack, enterprise control plane.
- Create pricing assumptions as strategy notes, not financial claims.

Done when:

- README/website/public docs can explain value without claiming
  certification, compliance, or guaranteed legal sufficiency.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Claim impact:

- Improves business credibility without unsupported valuation claims.

## Dependency Map

```text
#382 + #314
  -> DOR-001 domain profile schema
  -> DOR-002 claim ladder
  -> DOR-004 golden corpora

#192 + #193 + #194 + #196 (independent regulated roadmap)
  -> named licensed-source uses and claims only

DOR-003 policy + public/synthetic evidence
  -> DOR-005 GxP/CSV planning profile
  -> DOR-009 medical/SaMD planning profile

DOR-005 + DOR-006
  -> DOR-007 CSA planner
  -> DOR-008 IQ/OQ/PQ templates

DOR-010 AI profile
  -> DOR-011 RAG answer evidence
  -> DOR-012 model/provider change ledger

DOR-016 verifiable evidence profile
  -> DOR-017 signing and verification CLI

DOR-001 + DOR-002 + domain profiles
  -> DOR-020 ALM/QMS adapter decision
  -> DOR-021 domain pack install guide
  -> DOR-022 control plane opportunity
  -> DOR-023 commercial positioning
```

## Immediate Execution Order

1. Do not start with market copy. Start with `DOR-001`, `DOR-002`, and
   `DOR-003`.
2. Follow the autonomous queue in `docs/roadmap-lanes.yaml`; closed historical
   issues `#382` and `#314` are evidence inputs, not live dispatch targets.
3. Build `DOR-004` before any domain pack is called portable.
4. Implement GxP/CSV and AI governance first, because they are closest to
   the existing regulated and RAG documentation.
5. Add finance/legal/medical only after the domain profile framework can
   prove blocked and not-applicable states cleanly.
6. Add verifiable evidence after ALCOA+ envelope enforcement, because
   signing weak evidence only makes weak evidence tamper-evident.
7. Move to commercial packaging only after at least one domain pack has a
   runnable install guide and a bounded evidence pack.

## Non-Negotiables

- No domain pack may claim compliance.
- No licensed reference may be committed in full text unless the license
  explicitly permits it.
- No generated RAG chunk is authority by itself.
- No blockchain/provenance feature may be used as a substitute for source
  fidelity.
- No downstream product or Praxis claim may weaken the NOMOS claim
  boundary.
- Every domain issue must close with a committed artifact, a test, a gate,
  or an explicit blocked/waived record.

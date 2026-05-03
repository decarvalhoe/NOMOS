# 21 - Regulated Quality And Compliance Reference

## Position

Nomos claims that a product can turn an authoritative business corpus into executable, traceable product law. For a regulated IT market, this claim is credible only if Nomos applies the same discipline to itself and can produce objective evidence for every claim.

This document is the hard reference. It does not certify Nomos against any regulation. It defines the control baseline Nomos must implement before it can defend a regulated-grade posture in front of a quality, validation, security, or regulated-domain reviewer.

## Current Hard Audit Verdict

Status: **alpha regulated-readiness baseline; not regulated-grade**.

Reasons:

- The `v0.1.0-ALPHA` line has an operational CLI, real RBOK lawbook POC evidence, source spans, typed blocks, certified TOC, strict fidelity gate, and regulated-readiness documentation baseline.
- The repository can support commercial regulated-readiness discussions and internal pilots.
- Nomos still lacks approved QMS owner records, training records, live GitHub QMS evidence exports, completed licensed-reference review, full reference-to-control closure, and independent reconstruction evidence.
- The public claim boundary must remain alpha-level until those records are complete.
- Praxis remains downstream: it can strengthen Nomos only after the Nomos-to-Praxis evidence contract is verified.

## Non-Negotiable Rule

Every claimed reference must be aligned, not downgraded.

For each external framework, standard, pattern, or tool that Nomos cites, Nomos must maintain:

- a source entry with URL, owner, version/date checked, and applicability;
- mapped controls in a regulated control matrix;
- at least one machine-checkable gate or documented human review;
- evidence artifacts produced in CI;
- a waiver mechanism with owner, expiry, risk, and mitigation when the control is not yet implemented;
- public wording that matches the current evidence level.

No reference may remain as decorative authority.

## Regulated Source Baseline

Nomos must align to the following public sources as a minimum baseline for regulated-grade claims.

| Reference | Why it matters for Nomos | Required Nomos alignment |
|---|---|---|
| FDA 21 CFR Part 11, closed-system controls | Electronic records, audit trails, access control, authority checks, record protection, validation, documentation change control. | Signed/hashed records, immutable audit trails, authority checks, validation evidence, exportable human/electronic records. |
| FDA Data Integrity and CGMP guidance | ALCOA: attributable, legible, contemporaneous, original/true copy, accurate; metadata and audit-trail expectations. | Every generated unit, feed, report, attestation, and CAPA item must preserve source, actor/tool, timestamp, hash, version, metadata, and reconstruction path. |
| EU GMP EudraLex Volume 4 Annex 11 | Risk-based validation, supplier assessment, lifecycle documentation, data migration checks, system inventory, traceable user requirements. | Intended-use risk model, URS/SRS/test traceability, lifecycle validation pack, migration meaning-preservation checks. |
| FDA Computer Software Assurance, 2025 | Risk-based confidence in automation for production and quality systems. | Risk-ranked validation strategy: scripted tests, exploratory tests, unscripted challenge tests, and objective evidence by criticality. |
| NASA NPR 7150.2 and NASA-STD-8739.8 | Software acquisition/development/maintenance/retirement requirements, software assurance, safety, and IV&V discipline. | Requirements baseline, bidirectional traceability, independent review gates, defect prevention, formal inspection for high-criticality controls. |
| NIST SP 800-218 SSDF | Secure software development practices and supplier communication vocabulary. | Secure build provenance, dependency review, vulnerability handling, threat modeling, release evidence, supplier-facing attestations. |
| NIST SP 800-53 Rev. 5 | Security and privacy control catalog with assurance dimension. | Control mapping for access control, audit/accountability, configuration management, system integrity, supply chain, privacy, incident response. |
| NIST AI RMF | Govern, map, measure, manage AI risk. | AI-assisted extraction and RAG must have risk classification, evaluation datasets, refusal/citation tests, monitoring, and human review. |
| OWASP Top 10 for LLM Applications | LLM threat patterns: prompt injection, data leakage, excessive agency, supply-chain risk, output handling. | RAG/agent threat tests, prompt-injection corpus, unsafe-output handling, tool-call containment, provenance-preserving answers. |
| W3C PROV-O | Interoperable provenance vocabulary. | Evidence graph terms for source, activity, agent, generated artifact, derivation, and invalidation. |
| SLSA provenance and in-toto statements | Supply-chain provenance and signed statements. | Build/test/feed/attestation statements that can be verified independently. |

Primary references:

- FDA Part 11 scope guidance: https://www.fda.gov/regulatory-information/search-fda-guidance-documents/part-11-electronic-records-electronic-signatures-scope-and-application
- eCFR 21 CFR 11.10: https://www.ecfr.gov/current/title-21/chapter-I/subchapter-A/part-11/subpart-B/section-11.10
- FDA Data Integrity and Compliance With Drug CGMP: https://www.fda.gov/media/119267/download
- EudraLex Volume 4 Annex 11: https://health.ec.europa.eu/system/files/2016-11/annex11_01-2011_en_0.pdf
- FDA Computer Software Assurance: https://www.fda.gov/regulatory-information/search-fda-guidance-documents/computer-software-assurance-production-and-quality-system-software
- NASA software requirements and assurance resources: https://www.nasa.gov/intelligent-systems-division/software-management-office/nasa-software-engineering-procedural-requirements-standards-and-related-resources/
- NIST SP 800-218 SSDF: https://csrc.nist.gov/pubs/sp/800/218/final
- NIST SP 800-53 Rev. 5: https://csrc.nist.gov/Pubs/sp/800/53/r5/upd1/Final

## Nomos Quality Levels

Nomos must stop using a vague "production ready" concept. The quality level must be explicit.

| Level | Name | Meaning | Release claim allowed |
|---|---|---|---|
| NQ-0 | Broken | Build, schema, or CLI gate is red. | No product claim. |
| NQ-1 | Method documented | Canonical-first method exists, but is not self-proved. | "Method draft" only. |
| NQ-2 | Tool operational | CLI commands work on fixtures and examples. | "Operational prototype". |
| NQ-3 | Self-compliant | Nomos passes Nomos-on-Nomos compliance gates. | "Self-audited canonical tool". |
| NQ-4 | Evidence integrated | Nomos and Praxis share evidence contracts; runtime, corpus, and CAPA evidence are linked. | "Evidence-backed regulated-grade candidate". |
| NQ-5 | Regulated validation pack | Intended use, risk model, URS/SRS, traceability, validation protocol, audit trail, records, signatures, and retention are complete. | "Validation-pack ready", not "certified". |
| NQ-6 | Independent audit ready | Independent reviewer can reconstruct every claim without private tribal knowledge. | "Independent audit ready". |

Current audit: **NQ-2 alpha**, because the CLI and RBOK corpus proof are operational with real evidence. `NQ-3` remains a candidate level until self-compliance evidence, owner/approval records, GitHub QMS evidence, and independent review preparation are closed.

## Control Families

### RQ-01 Intended Use And Predicate Scope

Nomos must define intended use per deployment:

- method documentation only;
- CLI used for engineering quality gates;
- corpus conversion for non-regulated product;
- corpus conversion for regulated product;
- electronic record/signature system;
- AI-assisted extraction system;
- validation evidence generator.

Each intended use must state whether Part 11-like controls, GMP Annex 11-like controls, and data-integrity controls are applicable.

### RQ-02 Source Authority And Corpus Read-Only

Every source must have:

- stable source ID;
- path/URI;
- owner;
- confidentiality/license;
- authority priority;
- source hash;
- status;
- allowed uses;
- change policy;
- retention policy.

Corpus ingestion must be read-only by default. Any write into a source repository is a critical failure.

### RQ-03 ALCOA+ Evidence Model

Every Nomos artifact must be:

- attributable: actor, tool, command, commit, workflow run;
- legible: human-readable export plus machine JSON/CUE/schema;
- contemporaneous: generated timestamp and source version captured at execution;
- original or true copy: source hash and artifact hash preserved;
- accurate: validated against schema and reproduced by command;
- complete: includes metadata and exclusions;
- consistent: stable IDs and deterministic output ordering;
- enduring: retained in a durable artifact store;
- available: retrievable for inspection without rebuilding private state.

### RQ-04 Audit Trail And Record Reconstruction

Nomos must reconstruct:

```text
claim
  -> external reference
  -> Nomos control
  -> source files
  -> CLI command
  -> CI run
  -> generated artifact
  -> hash/signature
  -> review decision
  -> release version
```

Audit-trail records must not obscure previous values. Corrections must append new records.

### RQ-05 Requirements And Traceability

Nomos must maintain a regulated control matrix:

```text
external_ref -> control_id -> requirement -> implementation -> test -> evidence -> release gate
```

NASA-style and Annex 11-style traceability is not optional for regulated-grade claims.

### RQ-06 Validation Lifecycle

Nomos needs a validation pack:

- intended-use statement;
- risk assessment;
- user requirements specification;
- functional/software requirements;
- architecture description;
- traceability matrix;
- validation protocol;
- test scripts and challenge cases;
- deviation log;
- validation summary report;
- release approval;
- rollback and recovery procedure.

### RQ-07 Secure SDLC And Supply Chain

Nomos must align to SSDF/SLSA/in-toto:

- dependency inventory and vulnerability review;
- reproducible release inputs;
- build provenance;
- CI identity and permissions review;
- signed or hash-verifiable attestations;
- branch protection and PR-only release changes;
- generated artifact integrity checks.

### RQ-08 AI And RAG Governance

LLM/RAG usage must be constrained:

- deterministic extraction has precedence over LLM extraction;
- LLM output cannot create authority without citation and review;
- prompt-injection tests are part of CI;
- retrieval answers must cite source IDs and hashes;
- hallucination/refusal/citation evals must be versioned;
- unsafe or low-confidence output must become `needs_review`, not product law.

### RQ-09 Praxis Compatibility

Praxis is the runtime evidence and CAPA counterpart for Nomos.

Nomos must produce canonical artifacts. Praxis must consume those artifacts and prove whether product behavior actually follows them.

Required shared contract:

```text
Nomos SourceManifest
Nomos CanonicalUnit
Nomos CorpusFeed
Nomos Claim
Nomos Attestation
  -> Praxis ProjectPack
  -> Praxis FeatureSpec
  -> Praxis UAT scenario
  -> Praxis RuntimeEvidence
  -> Praxis InvariantResult
  -> Praxis CAPA NonConformity
  -> Nomos control/evidence update
```

Nomos proves "what the law is". Praxis proves "what the product does". A regulated story needs both.

### RQ-09A Nomos/Praxis Synergy And Boundary

Nomos and Praxis are complementary products, not interchangeable products.

Nomos is the authority conversion system:

- inventories authoritative sources;
- atomizes business law into canonical units;
- validates source policies, manifests, sidecars, schemas, feeds, provenance, and attestations;
- protects source corpora from mutation;
- governs claims and release evidence.

Praxis is the execution evidence system:

- reconstructs product surface from code and runtime;
- compiles and executes UAT/API/UI scenarios;
- captures runtime evidence;
- checks invariants;
- computes evidence-backed coverage;
- produces CAPA findings and regression history.

The synergy is the closed loop:

```text
Nomos source law
  -> Nomos canonical units and claims
  -> product implementation
  -> Praxis runtime evidence and invariants
  -> Praxis CAPA findings
  -> Nomos control matrix and release decision
```

This means:

- Nomos without Praxis can prove source-to-artifact traceability, but not that the running product obeys the law.
- Praxis without Nomos can prove product behavior, but not that the expected behavior is the authoritative business law.
- Together, they can defend both halves of the regulated claim: source authority and runtime conformance.

### RQ-09B Praxis Regulatory Parity Note

Praxis regulatory parity is not the direct implementation scope of this Nomos reference PR, but it is a mandatory future control boundary.

If Praxis is used only as an internal exploratory testing tool, Praxis may remain outside the regulated release boundary for a given deployment.

If Praxis output is used as any of the following, Praxis must meet the same regulated-grade baseline as Nomos:

- release go/no-go evidence;
- CAPA source record;
- validation evidence;
- audit response evidence;
- product-law conformance evidence;
- electronic record retained for regulated decision making.

In that case, Praxis must have its own equivalent of:

- intended-use statement;
- risk assessment;
- requirements and traceability matrix;
- ALCOA+ data integrity metadata;
- audit trail and immutable evidence records;
- source/build/test provenance;
- validated project packs and invariants;
- waiver and deviation control;
- claims governance;
- self-compliance gate on `RBOKproject/praxis`.

The Nomos/Praxis compatibility work must therefore avoid a one-way dependency where Nomos becomes regulated while Praxis remains an unqualified evidence generator. A connected regulated toolchain is only as defensible as its weakest evidence-producing tool.

### RQ-10 Claims Governance

Every README, roadmap, website, issue body, and release note claim must be controlled.

Forbidden claims unless evidence exists:

- "regulated-grade";
- "compliant";
- "certified";
- "Part 11 ready";
- "GxP ready";
- "NASA-grade";
- "converts any business reference into law";
- "read-only guaranteed";
- "complete traceability".

Allowed wording must include evidence level and limitations.

## External Reference Alignment Audit

Checked on 2026-05-02 from the current Nomos documentation.

| Reference URL | Status | Alignment required |
|---|---|---|
| `https://www.nasa.gov/wp-content/uploads/2018/09/nasa_systems_engineering_handbook_0.pdf` | reachable | Add requirements traceability controls, lifecycle verification, and change-impact gates. |
| `https://www.w3.org/TR/prov-o/` | reachable | Map Nomos evidence graph terms to PROV-O terms. |
| `https://www.cognitect.com/blog/2011/11/15/documenting-architecture-decisions` | reachable | Enforce ADR shape, status, supersession, and decision-to-release traceability. |
| `https://c4model.com/` | reachable | Add architecture model inventory and drift checks. |
| `https://martinfowler.com/bliki/StranglerFigApplication.html` | reachable | Keep as migration pattern with explicit non-regulatory status. |
| `https://martinfowler.com/bliki/TestPyramid.html` | reachable | Map to validation strategy and risk-based test levels. |
| `https://json-schema.org/` and draft 2020-12 schema | reachable | Validate report/feed/manifest schemas in CI. |
| `https://spec.openapis.org/oas/` | reachable | Use for API evidence where Nomos exposes API surfaces. |
| `https://www.12factor.net/` | reachable | Map to deployment/config portability controls, not regulated validation. |
| `https://www.nist.gov/publications/artificial-intelligence-risk-management-framework-ai-rmf-10` | reachable | Add AI risk register and evaluation controls. |
| `https://owasp.org/www-project-top-10-for-large-language-model-applications/` | reachable | Add LLM threat tests. |
| `https://openlineage.io/` | reachable | Decide whether to adopt or map to Nomos provenance model. |
| `https://www.openpolicyagent.org/docs/latest` | reachable | Use or replace with CUE/native policy gates. Decision required. |
| `https://docs.greatexpectations.io/` | reachable | If cited, add data quality expectation mapping or remove from active controls after replacement. |
| `https://opentelemetry.io/` | reachable | Add observability evidence controls for runs and agents. |
| `https://github.com/openai/evals`, `https://docs.ragas.io/`, `https://langfuse.com/docs`, `https://docs.llamaindex.ai/...` | reachable | Add RAG/LLM evaluation strategy or mark as tool candidates governed by adoption decision. |
| `https://dvc.org/`, `https://backstage.io/docs/features/techdocs/` | reachable | Add adoption decision or compatibility gate if Nomos claims dataset/docs lifecycle support. |
| `https://slsa.dev/provenance/v1`, `https://in-toto.io/Statement/v1` | reachable | Implement verifiable provenance statements and validation tests. |
| `https://example.com/*` | failing placeholders | Replace with real example repos or local fixture identifiers. No placeholder URL in regulated docs. |
| `https://get.nomos.dev/install.sh`, `https://nomos.dev/attestation/v1`, `https://schemas.nomos.dev/*` | unresolved/failing | Either publish and monitor these endpoints or replace with repository-local canonical URLs until DNS exists. |
| `https://github.com/RBOKproject/Nomos` | returned 404 to unauthenticated link check | Treat as private repo link; use GitHub API/CLI evidence in CI, not public URL reachability. |

## Required Evidence Artifacts

Nomos regulated runs must produce at least:

- `nomos-report.json`;
- `nomos-control-matrix.json`;
- `nomos-source-manifest.json`;
- `nomos-corpus-feed.json` when a corpus is in scope;
- `nomos-provenance.intoto.jsonl`;
- `nomos-alcoa-report.json`;
- `nomos-validation-summary.md`;
- `nomos-deviation-log.md`;
- `nomos-waivers.json`;
- `praxis-runtime-evidence.json` when a product target is tested;
- `praxis-capa-report.json`;
- `release-go-no-go.md`.

## Compatibility Contract With Praxis

Minimum shared IDs:

- `control_id`;
- `source_id`;
- `source_hash`;
- `unit_id`;
- `claim_id`;
- `artifact_hash`;
- `test_id`;
- `scenario_id`;
- `finding_id`;
- `capa_id`;
- `release_id`.

Minimum shared severity:

- `critical`: product law, data integrity, security, or read-only break;
- `major`: traceability, validation, or evidence break;
- `minor`: incomplete metadata or non-blocking documentation gap;
- `observation`: improvement without compliance impact.

Minimum shared verdicts:

- `pass`;
- `warn`;
- `fail`;
- `blocked`;
- `not_applicable`;
- `waived_until`.

## Release Gate

A regulated-grade Nomos release cannot ship unless:

- `main` is green;
- Nomos self-compliance is green;
- Praxis can audit Nomos documentation and produce a CAPA report;
- any Praxis artifact used as release, CAPA, validation, or audit evidence has declared its Praxis quality level and parity status;
- external references registry has no unresolved placeholders;
- every active reference has at least one control and one evidence artifact;
- RBOK lawbook E2E is green when RBOK is the declared reference corpus;
- read-only checks prove no mutation of source corpora;
- all critical/major findings are closed or have approved expiring waivers;
- release claims match evidence level.

## Bottom Line

Nomos can reach the promised market position, but only after it becomes a regulated quality system around its own method. The credible path is not softer language. The credible path is stricter evidence.

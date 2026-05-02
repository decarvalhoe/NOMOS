# 24 - Regulated Client Compliance Evidence

Date: 2026-05-02

## Purpose

This document translates regulated-client expectations into concrete Nomos deliverables.

The target audience is a client quality, validation, security, supplier-assurance, or regulated-domain reviewer who needs objective evidence before using Nomos in a regulated software environment.

The key lesson from the references is simple:

```text
Regulated clients do not buy a vendor claim.
They qualify the vendor, define intended use, validate their implemented system,
and retain enough evidence to defend the use of the tool inside their own QMS.
```

Nomos can support that process only if it produces controlled, inspectable, reproducible evidence. Nomos cannot certify the client's compliance by itself.

## Primary References Used

Regulated software and computerized-system expectations were mapped from:

- FDA 21 CFR Part 11 closed-system controls: https://www.ecfr.gov/current/title-21/chapter-I/subchapter-A/part-11/subpart-B/section-11.10
- FDA Part 11 Scope and Application guidance: https://www.fda.gov/regulatory-information/search-fda-guidance-documents/part-11-electronic-records-electronic-signatures-scope-and-application
- FDA Computer Software Assurance guidance, September 2025: https://www.fda.gov/regulatory-information/search-fda-guidance-documents/computer-software-assurance-production-and-quality-system-software
- FDA General Principles of Software Validation: https://www.fda.gov/regulatory-information/search-fda-guidance-documents/general-principles-software-validation
- FDA computerized systems inspection guide for drug establishments: https://www.fda.gov/inspections-compliance-enforcement-and-criminal-investigations/inspection-guides/computerized-systems-drug-establishments-283
- EudraLex Volume 4 Annex 11, current 2011 version: https://health.ec.europa.eu/system/files/2016-11/annex11_01-2011_en_0.pdf
- EudraLex Annex 11 revision draft/concept paper: https://health.ec.europa.eu/document/download/40231f18-e564-4043-94de-c031f813d38b_en
- FDA QMSR / ISO 13485 alignment FAQ: https://www.fda.gov/medical-devices/quality-management-system-regulation-qmsr/quality-management-system-regulation-frequently-asked-questions
- ISPE GAMP 5, second edition overview: https://ispe.org/publications/guidance-documents/gamp-5-guide-2nd-edition
- NIST SP 800-218 SSDF: https://csrc.nist.gov/pubs/sp/800/218/final
- NIST SP 800-53 Rev. 5: https://csrc.nist.gov/Pubs/sp/800/53/r5/upd1/Final
- NASA NPR 7150.2 software verification and validation requirements: https://nodis3.gsfc.nasa.gov/displayCA.cfm?Internal_ID=N_PR_7150_0002_&page_name=Chapter2
- NASA-STD-8739.8 software assurance and software safety standard: https://standards.nasa.gov/standard/nasa/nasa-std-87398
- ISO/IEC 27001 official overview: https://www.iso.org/standard/27001
- AICPA SOC 2 overview: https://www.aicpa-cima.com/topic/audit-assurance/audit-and-assurance-greater-than-soc-2
- NIST AI Risk Management Framework: https://www.nist.gov/itl/ai-risk-management-framework
- OWASP Top 10 for Large Language Model Applications: https://owasp.org/www-project-top-10-for-large-language-model-applications/
- EU AI Act Regulation 2024/1689: https://eur-lex.europa.eu/eli/reg/2024/1689/

## What A Regulated Client Will Expect

### 1. Supplier Qualification Package

The client will assess whether Nomos as a supplier can be relied upon. This is not optional in GxP-style environments: the regulated user remains responsible even when leveraging vendor documentation and service-provider qualification.

Expected evidence:

- company and product overview;
- quality policy and QMS scope;
- SOP index;
- SDLC procedure;
- validation/CSA procedure;
- change control procedure;
- configuration management procedure;
- release management procedure;
- incident/problem/CAPA procedure;
- supplier/subprocessor management procedure;
- training procedure and role training matrix;
- document control and record retention procedure;
- support model, SLA, escalation path, and service reporting;
- security overview, penetration-test summary, vulnerability management process;
- cloud/subprocessor list and data-processing boundaries;
- business continuity and disaster recovery summary;
- audit-rights statement and quality/security questionnaire responses.

Nomos target:

- produce a `Nomos Supplier Assurance Pack`;
- keep it versioned and tied to release evidence;
- define which parts are public, NDA, or client-specific.

### 2. Intended Use And Regulated Impact Assessment

The client will ask: what exactly is Nomos used for?

Possible intended uses:

- method documentation only;
- engineering quality gate;
- canonical corpus converter;
- regulated validation evidence generator;
- electronic record system;
- electronic signature/approval system;
- AI-assisted extraction/RAG system.

Expected evidence:

- intended-use statement;
- out-of-scope statement;
- GxP/regulated impact assessment;
- risk classification;
- data classification;
- record/e-signature applicability assessment;
- AI applicability assessment;
- customer responsibility matrix.

Nomos target:

- every Nomos deployment/profile must have an intended-use inventory item;
- every public claim must map to an intended use and an NQ level;
- no "regulated-grade" wording without a declared intended use.

### 3. Requirements, Configuration, And Design Baseline

Regulated clients expect requirements to be approved, traceable, and maintained through the lifecycle. Annex 11 explicitly expects user requirements to describe required system functions, be risk-based, and remain traceable.

Expected evidence:

- URS: user requirements specification;
- FRS/SRS: functional/software requirements;
- configuration specification;
- architecture description;
- data-flow diagrams;
- interface/control boundary description;
- source/corpus authority model;
- RAG/AI boundary model;
- cybersecurity/threat model;
- requirements-to-tests traceability matrix.

Nomos target:

- `docs/validation/urs.md`;
- `docs/validation/srs.md`;
- `docs/validation/configuration-specification.md`;
- `docs/validation/architecture-and-data-flows.md`;
- `specs/regulated-control-matrix.cue`;
- machine-readable traceability:

```text
reference -> control -> requirement -> implementation -> test -> evidence -> release claim
```

### 4. Risk-Based Validation Or Computer Software Assurance

FDA CSA and GAMP-style practice do not require blind documentation volume. They require justified, risk-based confidence. Critical functions need more rigorous evidence; low-risk support functions can use lighter evidence if justified.

Expected evidence:

- validation master plan or product validation plan;
- GxP impact assessment;
- risk assessment;
- criticality classification by feature/command/profile;
- test strategy;
- test protocols;
- scripted tests for critical paths;
- unscripted/exploratory challenge testing where appropriate;
- automated test tool qualification/adequacy assessment;
- test results;
- deviation log;
- defect and CAPA links;
- validation summary report;
- approval/review records.

Nomos target:

- `docs/validation/validation-plan.md`;
- `docs/validation/risk-assessment.md`;
- `docs/validation/test-strategy.md`;
- `docs/validation/deviation-log.md`;
- `docs/validation/validation-summary.md`;
- `nomos compliance self-check`;
- `nomos release bundle`;
- generated reports with deterministic IDs, hashes, and commands.

### 5. Electronic Records, Audit Trails, And Signatures

If Nomos records are used for regulated decisions, Part 11-like expectations become relevant. This includes validation, accurate copies, retention, access restriction, secure audit trails, operational checks, authority checks, training, accountability policies, and documentation change controls.

Expected evidence:

- electronic-record scope assessment;
- record inventory;
- record retention matrix;
- audit-trail design;
- audit-trail review procedure;
- role/access matrix;
- authority checks;
- authentication/session controls;
- electronic signature meaning and manifestation, if implemented;
- signature-to-record binding, if implemented;
- documentation revision/change-control trail;
- human-readable and electronic export capability.

Nomos target:

- do not claim Part 11 compliance yet;
- first define approval records and controlled review semantics;
- release bundles must distinguish:
  - `advisory_evidence`;
  - `reviewed_evidence`;
  - `approved_release_evidence`;
  - `electronic_signature_ready`;
  - `part_11_claimed`.

### 6. Data Integrity And ALCOA+

Clients will expect records and generated evidence to be attributable, legible, contemporaneous, original/true-copy, accurate, complete, consistent, enduring, and available.

Expected evidence:

- actor identity;
- tool identity and version;
- command line;
- timestamp;
- repository URL;
- commit SHA;
- source hashes;
- generated artifact hashes;
- source-to-artifact derivation path;
- data migration/format conversion checks;
- backup and restore proof;
- retention and retrieval proof;
- previous-value preservation for controlled records.

Nomos target:

Every generated artifact must include an ALCOA+ evidence envelope:

```json
{
  "actor": "...",
  "tool": "nomos",
  "tool_version": "...",
  "command": "...",
  "timestamp": "...",
  "repo": "...",
  "commit": "...",
  "source_hashes": [],
  "artifact_hash": "...",
  "derivation": []
}
```

### 7. Secure SDLC And Supply Chain Assurance

Regulated clients increasingly combine validation review with supplier security review. NIST SSDF gives a common vocabulary for supplier/acquirer communication; NIST 800-53 gives broad control families.

Expected evidence:

- secure SDLC procedure;
- threat model;
- code review policy;
- branch protection and PR-only release policy;
- CI/CD control description;
- SBOM;
- dependency and vulnerability management process;
- vulnerability disclosure and remediation SLA;
- SAST/DAST/container/dependency scan evidence;
- secrets management;
- artifact signing or hash verification;
- SLSA/in-toto provenance;
- incident response procedure;
- access review evidence.

Nomos target:

- ship release provenance with every release;
- define minimum SSDF control mapping;
- create SBOM and dependency review artifacts;
- fail release if generated artifacts cannot be tied to source commit and CI run.

### 8. AI/RAG Governance

If Nomos uses AI to extract, classify, summarize, or recommend canonical units, clients will ask whether the AI output can create regulated authority. The answer must be controlled: deterministic extraction and reviewed source evidence must dominate; LLM output cannot silently become product law.

Expected evidence:

- AI intended-use statement;
- model/provider inventory;
- prompt and tool-call boundary;
- data usage and privacy assessment;
- deterministic-vs-AI responsibility split;
- human review workflow;
- source citation and confidence policy;
- hallucination/refusal/citation evals;
- prompt-injection and data-leakage tests;
- output handling controls;
- model/version change control;
- monitoring and incident handling.

Nomos target:

- AI output creates `candidate` or `needs_review`, never authoritative law by default;
- RAG answers must cite source ID and source hash;
- prompt injection and excessive-agency tests must be part of compliance CI;
- release claims must state whether AI was used, where, and with what review evidence.

### 9. Operational Readiness

Regulated acceptance does not end at validation. Clients expect operation, periodic review, change control, incident response, and retirement controls.

Expected evidence:

- operational SOP;
- backup/restore procedure and test;
- disaster recovery procedure and test;
- monitoring/alerting overview;
- support and escalation procedure;
- access review procedure;
- periodic review procedure;
- audit trail review procedure;
- change/revalidation procedure;
- incident/problem/CAPA procedure;
- data retention and deletion procedure;
- retirement/decommissioning procedure.

Nomos target:

- define operational controls for CLI-only, CI, and control-plane deployment modes separately;
- define which controls are customer-owned versus Nomos-owned;
- expose release artifacts that customers can import into their own QMS.

## Client Integration Procedure

This is the practical path a regulated client will expect.

### Step 1 - Intake And Scope

Outputs:

- intended-use statement;
- regulated-impact assessment;
- data classification;
- deployment model;
- responsibility matrix.

Gate:

- no validation work starts until intended use and owner are approved.

### Step 2 - Supplier Qualification

Outputs:

- supplier assurance pack;
- quality/security questionnaire;
- audit or supplier assessment report;
- quality agreement or contractual controls;
- SLA/support agreement.

Gate:

- vendor qualification accepted or risk waiver approved.

### Step 3 - Validation Planning

Outputs:

- validation plan;
- risk assessment;
- system inventory item;
- validation strategy;
- test strategy;
- required evidence list.

Gate:

- quality/validation owner approves risk-based strategy.

### Step 4 - Requirements And Configuration

Outputs:

- URS;
- SRS/FRS;
- configuration specification;
- architecture/data-flow/interface documentation;
- traceability matrix draft.

Gate:

- requirements baseline approved.

### Step 5 - Build, Configure, Or Deploy Under Change Control

Outputs:

- change record;
- release notes;
- installation/deployment record;
- environment/configuration record;
- access setup evidence.

Gate:

- deployed configuration matches approved baseline.

### Step 6 - Verification And Validation Execution

Outputs:

- test protocols;
- automated test evidence;
- challenge/worst-case tests;
- UAT/PQ evidence;
- deviation log;
- CAPA links where needed.

Gate:

- critical tests pass or have approved deviation with impact assessment.

### Step 7 - Evidence Review And Approval

Outputs:

- traceability matrix final;
- validation summary report;
- open deviation assessment;
- release/go-live approval;
- training completion evidence.

Gate:

- no go-live until approval record is complete.

### Step 8 - Operational Control

Outputs:

- periodic review record;
- access review record;
- audit-trail review record;
- backup/restore evidence;
- incident/CAPA records;
- change/revalidation records.

Gate:

- system remains qualified only while operational controls are maintained.

### Step 9 - Change And Revalidation

Outputs:

- change request;
- impact analysis;
- regression scope;
- revalidation evidence;
- updated validation summary.

Gate:

- significant changes cannot enter regulated use without approved impact assessment and required revalidation.

### Step 10 - Retirement

Outputs:

- retirement plan;
- data export/retention proof;
- access shutdown record;
- final audit-trail/archive package.

Gate:

- records remain retrievable for the required retention period.

## Minimum Document Set For Nomos

Nomos should produce the following before seeking regulated-client pilots.

| Artifact | Why clients expect it | Nomos status target |
|---|---|---|
| Supplier assurance pack | Vendor qualification and audit readiness | Create before regulated pilot |
| QMS/SOP index | Shows controlled process exists | Create before NQ-5 |
| Intended-use statement | Defines validation scope | Required for every profile |
| Regulated impact assessment | Determines GxP/Part 11/AI applicability | Required for every pilot |
| System inventory item | Annex 11-style system listing | Required for NQ-5 |
| URS | Client/user requirements baseline | Required for NQ-5 |
| SRS/FRS | Software/function requirements | Required for NQ-5 |
| Architecture/data-flow/interface doc | Shows boundaries, records, integrations | Required for NQ-5 |
| Configuration specification | Defines controlled implementation | Required for NQ-5 |
| Risk assessment | Drives CSA/validation rigor | Required for NQ-5 |
| Traceability matrix | Links requirements to tests/evidence | Required for NQ-3 and higher |
| Validation plan | Defines method, scope, roles, evidence | Required for NQ-5 |
| Test protocols/results | Objective validation evidence | Required for NQ-2 and higher |
| Deviation/CAPA log | Handles failures without hiding them | Required for NQ-3 and higher |
| Validation summary report | Final validation conclusion | Required for NQ-5 |
| Release evidence bundle | One inspectable release package | Required for NQ-5 |
| Audit-trail design | Controlled records and review | Required before any Part 11-like claim |
| Record retention matrix | Inspection/retrieval support | Required for NQ-5 |
| Security/supply-chain pack | Client security and supplier review | Required for regulated pilot |
| AI/RAG governance pack | Controls AI-generated evidence risk | Required before AI-assisted regulated use |
| Operational SOP pack | Keeps system controlled after go-live | Required for NQ-5 |

## What Is Not Enough

These are common failure modes with regulated clients:

- a vendor "certificate of suitability" without validation protocols and results;
- passing unit tests without approved requirements and traceability;
- screenshots instead of controlled evidence artifacts;
- audit trails that overwrite prior values;
- generated records without actor, timestamp, source hash, and artifact hash;
- automated testing tools with no adequacy assessment;
- AI extraction without human review and source hash citations;
- release notes with no impact assessment;
- data migration without meaning-preservation checks;
- SOC 2 or ISO/IEC 27001 alone, because security assurance does not replace validation;
- "Part 11 compliant" wording without record scope, audit trail, e-signature, access, policy, and validation evidence.

## Nomos Product Implications

Nomos must implement these capabilities before making strong regulated-client claims:

1. A regulated control matrix from external reference to evidence.
2. Nomos-on-Nomos self-compliance.
3. Read-only corpus ingestion with before/after repository proof.
4. ALCOA+ evidence envelope on every generated artifact.
5. Release evidence bundle.
6. Intended-use inventory and validation pack.
7. Public claim governance tied to NQ levels.
8. Security and supply-chain evidence.
9. AI/RAG governance and evaluation evidence.
10. Export format that clients can retain inside their own QMS.

## Customer-Ready Claim Boundary

Allowed before NQ-3:

```text
Nomos is a documented Canonical-First method and implementation prototype.
```

Allowed at NQ-3:

```text
Nomos is self-audited against its own regulated control baseline for a declared intended use.
```

Allowed at NQ-5:

```text
Nomos can provide a validation-pack-ready evidence bundle for a declared intended use.
```

Blocked without external/customer validation:

```text
Nomos is certified.
Nomos makes a customer compliant.
Nomos is Part 11 compliant in all deployments.
Nomos validates regulated software automatically.
Nomos AI can convert authority into law without review.
```

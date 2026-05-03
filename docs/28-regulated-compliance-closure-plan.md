# 28 - Regulated Compliance Closure Plan

Date: 2026-05-03

## Purpose

This document is the current execution plan for closing the remaining Nomos compliance gaps after the final implementation wave merged into `main`.

It supersedes the older recovery framing in `docs/23-regulated-implementation-plan.md` for day-to-day execution. The older plan remains useful as historical context and dependency rationale; this document is the active closure plan.

The objective is not to claim certification. The objective is to make the next public and customer-facing quality claim defensible from repository state, GitHub configuration evidence, licensed-reference sidecars, CI artifacts, validation records, and independent review evidence.

## Verified Current State

Verified on 2026-05-03:

- local branch `main` is aligned with `origin/main`;
- latest merged commit is `709d011 feat: Final wave`;
- GitHub open PR count is `0`;
- GitHub open issue count is `0`;
- `main` CI, regulated documentation gate, regulated evidence pack, and corpus tests are green;
- reference canon status is `requires_evidence`;
- GitHub QMS audit status is `requires_evidence`.

This means Nomos is technically operational and evidence-generating, but not yet regulated-grade.

## Current Claim Boundary

Allowed wording:

```text
Nomos has a regulated-by-design architecture, executable gates, an RBOK lawbook E2E, and a draft QMS/CSV/CSA evidence baseline under active validation.
```

Blocked wording:

```text
Nomos is validated.
Nomos is Part 11 compliant.
Nomos is GAMP 5 compliant.
Nomos is regulated-grade.
Nomos can produce legally defensible software law from any source without customer validation.
```

Those claims remain blocked until the closure gates in this document pass.

## Closure Targets

| Target | Meaning | Required status |
|---|---|---|
| `NQ-3` | Nomos self-compliance is executable and evidence-producing. | All repo-local gates green, reference canon actionable, self-compliance report generated, no critical gap hidden. |
| `NQ-4` | Nomos evidence can be consumed by Praxis or another assurance layer. | Shared evidence contract, fixture validation, RBOK lawbook proof, Praxis status explicitly recorded. |
| `NQ-5` | Scoped validation pack ready for regulated-client review. | Validation inventory, release bundle, approval records, training records, retention policy, reconstruction procedure. |
| `NQ-6` | Independent reconstruction/audit readiness. | Independent reviewer can rebuild release evidence from source, artifacts, hashes, logs, and approvals. |

The immediate target is `NQ-3`, then `NQ-4` only after the Nomos producer evidence is stable.

## Active Gap Register

| Gap | Severity | Blocks | Closure owner |
|---|---|---|---|
| `GAP-QMS-OWNER` | Major | regulated-grade claim, effective QMS | Quality owner must be assigned in controlled records. |
| `GAP-APPROVAL-WORKFLOW` | Major | approved release evidence, Part 11-like records | Define approval semantics and enforce them in GitHub/release bundle. |
| `GAP-TRAINING` | Major | SOP effectiveness | Create role matrix and training records for regulated process users. |
| `GAP-INDEPENDENT-AUDIT` | Major | independent audit readiness | Schedule and record independent reconstruction review. |
| `GAP-GITHUB-LIVE-CONFIG` | Major | GitHub operating model, approved release evidence | Configure and export evidence for branch/ruleset/environment/security/audit controls. |
| `GAP-LICENSED-BIBLES` | Major | GAMP/ISO clause mapping, CSV/CSA claims | Complete licensed artifacts, license review, read-only Nomos processing. |

## Workstream A - Governance And QMS Ownership

Goal: turn the QMS from draft documents into governed, reviewable operating records.

### A1 - Assign quality roles

Files:

- Modify `docs/regulated/quality-system/quality-manual.md`.
- Modify `docs/regulated/product-profiles/nomos.yaml`.
- Modify `.github/CODEOWNERS`.
- Create training records under `docs/regulated/quality-system/training-records/`.

Steps:

1. Assign a named quality owner for Nomos controlled documentation.
2. Assign a named technical approver for CLI/evidence changes.
3. Assign a security owner for GitHub/security controls.
4. Replace CODEOWNERS placeholder comments with real GitHub users or teams.
5. Record each owner in a controlled training/competence record.

Evidence:

- CODEOWNERS active rule exists.
- Training record exists for every active owner.
- PR requiring CODEOWNER review demonstrates routing.

Gate:

```bash
python scripts/regulated_github_qms_audit.py --repo RBOKproject/NOMOS --output .regulated-evidence-pack/github-qms-audit.json
```

Expected result:

- `codeowners` no longer reports `requires_human_review`.
- `GAP-QMS-OWNER` can be closed only after review.

### A2 - Define approval semantics

Files:

- Modify `docs/regulated/data-integrity/electronic-records-and-signatures-policy.md`.
- Modify `docs/regulated/github-operating-model/README.md`.
- Modify `docs/regulated/validation-pack/validation-inventory.yaml`.
- Add release approval record template or reuse `templates/regulated/release-evidence-bundle.yaml`.

Steps:

1. Define what a GitHub approval means for Nomos releases.
2. Define what it does not mean: no Part 11 e-signature claim.
3. Define required approver roles for controlled docs, release bundle, waivers, and validation summary.
4. Add release bundle fields for approval identity, timestamp, evidence URL, meaning, and limitation.

Evidence:

- Approval semantics document updated.
- Release bundle example includes approval fields.
- Documentation gate accepts the record.

Expected result:

- `GAP-APPROVAL-WORKFLOW` moves from open to implemented, not approved.

## Workstream B - GitHub Regulated Operating Model

Goal: make GitHub configuration match the documentary QMS model.

### B1 - Protect `main`

Required GitHub settings:

- branch protection or repository ruleset on `main`;
- pull request required before merge;
- required status checks:
  - `CI`;
  - `Regulated Documentation Gate`;
  - `Regulated Evidence Pack`;
  - `RBOK Lawbook E2E` where corpus-impact paths change;
- required review count at least `1`;
- stale review dismissal;
- force-push disabled;
- branch deletion disabled.

Evidence:

- screenshot or API export of branch protection/ruleset;
- `regulated_github_qms_audit.py` output;
- retained GitHub Actions artifact.

Gate:

```bash
python scripts/regulated_github_qms_audit.py --repo RBOKproject/NOMOS --output .regulated-evidence-pack/github-qms-audit.json
```

Expected result:

- `branch_protection`, `rulesets`, `required_reviews`, and `required_status_checks` move out of `requires_live_evidence`.

### B2 - Configure protected release environment

Required GitHub settings:

- environment name: `regulated-release`;
- required reviewer enabled;
- deployment branch restriction to `main` and release tags;
- self-review prevention when supported.

Evidence:

- environment settings export or screenshot;
- GitHub audit report;
- release dry-run job requiring environment approval.

Expected result:

- `protected_environments` no longer reports `requires_live_evidence`.

### B3 - Define retention and audit-log export

Files:

- Modify `docs/regulated/security-privacy/access-control-and-audit-trail-sop.md`.
- Modify `docs/regulated/evidence-index/README.md`.
- Modify `docs/regulated/operations/README.md`.

Steps:

1. Define evidence retention period for CI artifacts, release bundles, validation packs, audit logs, and licensed-reference sidecars.
2. Define where exported artifacts are stored.
3. Define periodic audit-log export cadence.
4. Add a management review task that verifies export completeness.

Evidence:

- retained Actions artifact;
- audit-log export record;
- retention policy record;
- management review issue.

Expected result:

- `artifact_retention_export` and `audit_log_export` move from open gap to controlled process.

### B4 - Enable and review GitHub security features

Required controls:

- Dependabot enabled where applicable;
- secret scanning enabled where plan permits;
- code scanning enabled or justified as not applicable;
- vulnerability triage procedure linked to CAPA.

Evidence:

- security settings export;
- first review record;
- CAPA path for critical finding.

Expected result:

- `security_features` moves from `requires_human_review` to verified or risk-accepted.

## Workstream C - Licensed Bible Closure

Goal: complete the canonical reference corpus without committing restricted full text.

### C1 - Complete missing licensed bibles

Currently verified:

- `ISPE-GAMP5-2E-2022` with SHA256 `F190D1671A20F9FF4C88387BA339D3BD41F3A4C4CC8CA117672006B85649BCFC`.
- `ISO-IEC-25010-2023` with SHA256 `C1A03CDCF53541C97006D8919007E979FC0C526BAFEEAA4CC2E413DBB8599974`.

Still missing:

- `ISO-13485-2016`;
- `ISO-IEC-IEEE-12207-2026`.

Steps:

1. Acquire enterprise/library copies.
2. Store them under `NOMOS_LICENSED_REFERENCE_ROOT/<reference-id>/`.
3. Create sidecars under `docs/regulated/reference-basis/licensed-intakes/`.
4. Hash with SHA256.
5. Run the reference canon with `--licensed-root`.

Gate:

```bash
python scripts/regulated_reference_canon.py --licensed-root C:\Dev\nomos-licensed-references --report .regulated-doc-gate/reference-canon-local-licensed-report.json
```

Expected result:

- `licensed_reference_gaps` becomes `0`.
- This does not authorize redistribution or extracted chunk commits.

### C2 - License review for acquired bibles

Files:

- Modify sidecars in `docs/regulated/reference-basis/licensed-intakes/`.
- Modify `docs/regulated/reference-basis/nomos-bible-corpus-policy.md`.

Steps:

1. Record license holder.
2. Record internal processing permission.
3. Record whether extracted chunks may be committed.
4. Record whether customer redistribution is prohibited.
5. Assign reviewer and approval status.

Expected result:

- `license_review_required` is replaced by an approved or blocked use decision.

## Workstream D - Bible Self-Processing

Goal: make Nomos process the references it uses as bibles.

### D1 - Public bible snapshots

References:

- FDA CSA 2025;
- 21 CFR Part 11;
- 21 CFR Part 820;
- FDA General Principles of Software Validation;
- FDA Data Integrity CGMP;
- EudraLex Volume 4 / Annex 11;
- NASA NPR 7150.2D;
- NIST SP 800-218;
- NIST SP 800-53;
- NIST AI RMF;
- ICH Q9(R1);
- ICH Q10;
- GitHub Docs references.

Steps:

1. Snapshot each official public reference.
2. Store snapshot outside generated release evidence if full content should not be committed.
3. Commit manifests, source hashes, provenance and coverage.
4. Atomize with legal/regulatory profile.

Expected outputs:

- source manifest;
- sidecar manifest;
- atomization certificate;
- traceability seed matrix;
- RAG metadata with source provenance.

### D2 - Licensed bible processing

Steps:

1. Run `nomos corpus feed` in read-only mode against each licensed reference root.
2. Emit only permitted outputs.
3. Fail if output writes inside licensed source folders.
4. Validate sidecars and source hashes.

Expected result:

- GAMP 5 and ISO/IEC 25010 can be referenced by hash and controlled sidecar.
- Clause-level content remains blocked unless license review permits extracted text storage.

## Workstream E - Reference-To-Control Matrix Closure

Goal: connect every bible to controls, requirements, evidence and release gates.

Files:

- Modify `docs/regulated/control-matrix/nomos-control-matrix.yaml`.
- Modify `docs/regulated/reference-basis/reference-registry.yaml`.
- Modify `specs/provenance-gate.cue` or add schema if required.

Steps:

1. For each bible, create a control mapping row.
2. Link each control to:
   - intended use;
   - risk classification;
   - implementation reference;
   - test reference;
   - evidence artifact;
   - release gate;
   - claim boundary.
3. Mark unmapped items as `requires_evidence` or `not_applicable_with_rationale`.
4. Add a gate that fails on decorative references.

Expected result:

- public references and licensed bibles are not decorative.
- README/product claims can be traced to evidence or blocked status.

## Workstream F - Validation And Release Bundle

Goal: create a scoped, reconstructible validation package for Nomos.

### F1 - Validation inventory

Scope:

- Nomos CLI;
- RBOK lawbook profile;
- atomization profiles;
- regulated documentation/evidence gates;
- reference canon and licensed intake process.

Steps:

1. Update `docs/regulated/validation-pack/validation-inventory.yaml`.
2. Classify risk by intended use and command criticality.
3. Link URS/SRS/test/evidence.
4. Define challenge cases.

### F2 - Release evidence bundle

Required contents:

- CI run URL;
- regulated documentation gate report;
- regulated evidence pack;
- reference canon report;
- GitHub QMS audit report;
- RBOK lawbook E2E artifact;
- source/corpus read-only attestation;
- open deviations and waivers;
- approval record;
- public claim boundary.

Expected result:

- An independent reviewer can reconstruct why a release claim is allowed or blocked.

## Workstream G - Training, CAPA And Operations

Goal: move from documents to operating process.

Steps:

1. Create role matrix.
2. Create training records for QMS users, release approvers, security reviewers and corpus operators.
3. Create CAPA workflow using existing GitHub issue forms.
4. Create periodic review issue template usage instructions.
5. Run one internal management review.

Evidence:

- training records;
- CAPA example;
- management review record;
- periodic review schedule.

Expected result:

- SOP effectiveness can be argued from records, not intent.

## Workstream H - Praxis Compatibility

Goal: keep Praxis in scope without letting it block Nomos NQ-3.

Execution rule:

Nomos must first publish stable producer artifacts. Praxis becomes release-relevant only after it can consume those artifacts and return runtime evidence.

Steps:

1. Keep Praxis status recorded as downstream/not-yet-qualified in Nomos release bundle.
2. Export Nomos evidence contract fixtures.
3. Add cross-repo smoke test only after Nomos evidence contract is stable.
4. Record Praxis evidence quality level before using it in any release claim.

Expected result:

- Nomos does not overclaim Praxis-based assurance.

## Dependency Tree

```text
A1 quality roles
  -> A2 approval semantics
  -> B1 branch protection/rulesets
  -> B2 protected release environment
  -> F2 release evidence bundle

B1 + B2 + B3 + B4
  -> GAP-GITHUB-LIVE-CONFIG closed

C1 missing licensed bibles
  -> C2 license review
  -> D2 licensed bible processing
  -> E reference-control matrix closure

D1 public bible snapshots
  -> E reference-control matrix closure

E reference-control matrix closure
  -> F1 validation inventory
  -> F2 release evidence bundle

G training/CAPA/operations
  -> F2 release evidence bundle
  -> H Praxis compatibility can become release-relevant

F2 release evidence bundle
  -> independent reconstruction audit
  -> NQ-5/NQ-6 claim review
```

## Issue-Ready Backlog

| ID | Title | Blocks | Done when |
|---|---|---|---|
| `RCP-001` | Configure GitHub branch protection and rulesets | GitHub QMS evidence | Audit report verifies branch/ruleset/required checks. |
| `RCP-002` | Activate CODEOWNERS with real roles | QMS owner evidence | CODEOWNERS routes controlled files to real users/teams. |
| `RCP-003` | Configure protected `regulated-release` environment | release approval evidence | Environment requires reviewer and branch restriction. |
| `RCP-004` | Define artifact retention and audit-log export | ALCOA+ enduring/available | Export records exist and retention policy is approved. |
| `RCP-005` | Verify GitHub security features | security evidence | Dependabot/secret/code scanning are enabled or risk-accepted. |
| `RCP-006` | Acquire and intake ISO 13485:2016 | licensed bible closure | SHA256 sidecar verifies local artifact. |
| `RCP-007` | Acquire and intake ISO/IEC/IEEE 12207:2026 | licensed bible closure | SHA256 sidecar verifies local artifact. |
| `RCP-008` | Complete license review for GAMP 5 and ISO/IEC 25010 | clause-level mapping | Sidecars record allowed use and reviewer decision. |
| `RCP-009` | Snapshot public bibles | reference atomization | Official snapshots have hashes and provenance. |
| `RCP-010` | Process public and licensed bibles with Nomos | reference evidence | Atomization reports and manifests exist without source mutation. |
| `RCP-011` | Close reference-to-control matrix | public claim governance | Every reference maps to control/evidence/gate or explicit blocked status. |
| `RCP-012` | Build validation inventory and protocols | NQ-5 validation pack | URS/SRS/tests/evidence are linked. |
| `RCP-013` | Generate release evidence bundle | release readiness | Bundle reconstructs current release claim boundary. |
| `RCP-014` | Create training and competence records | effective SOP use | Role matrix and training records exist. |
| `RCP-015` | Run independent reconstruction review | NQ-6 readiness | Independent review record lists findings and residual risk. |
| `RCP-016` | Preserve Praxis downstream boundary | joint product claims | Release bundle records Praxis evidence status accurately. |

## Operating Gates

Run before any release-claim change:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
python scripts/regulated_reference_canon.py --licensed-root C:\Dev\nomos-licensed-references --report .regulated-doc-gate/reference-canon-local-licensed-report.json
python scripts/regulated_github_qms_audit.py --repo RBOKproject/NOMOS --output .regulated-evidence-pack/github-qms-audit.json
```

Run product gates:

```bash
cd cli
go test ./...
go vet ./...
```

Run schema gates:

```bash
cue vet specs/*.cue
```

## Decision Rules

1. A green CI run is necessary but not sufficient for regulated claims.
2. A licensed PDF hash proves artifact identity, not processing rights.
3. GitHub approval is not a Part 11 signature unless a validated signature process exists.
4. Missing evidence remains a gap; it cannot be closed by wording.
5. Praxis evidence is advisory until Praxis parity and cross-repo evidence contracts are verified.
6. Public claims must match the lowest unresolved blocking gap.

## Completion Definition

The closure plan is complete when:

- `regulated_reference_canon.py` reports no licensed-reference gaps for required bibles;
- `regulated_github_qms_audit.py` has no major `requires_live_evidence` findings;
- QMS owner, approver, training and approval records exist;
- release evidence bundle can be generated and independently reconstructed;
- reference-to-control matrix covers every registered bible;
- validation inventory links intended use, risk, requirements, tests, evidence and release decision;
- remaining gaps are explicitly waived with expiry and risk acceptance;
- README and public-facing docs do not exceed the achieved quality level.

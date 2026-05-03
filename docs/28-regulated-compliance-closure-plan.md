# 28 - Regulated Compliance Closure Plan

Date: 2026-05-03
Release context: `v0.1.0-ALPHA`

## Purpose

This is the active plan for moving Nomos from alpha regulated-readiness to a defensible regulated-market posture.

The objective is not to claim certification prematurely. The objective is to make every future public and customer-facing quality claim reconstructible from repository state, GitHub configuration evidence, licensed-reference sidecars, CI artifacts, validation records, and independent review evidence.

## Verified Current State

At `v0.1.0-ALPHA`:

- Nomos CLI is functional for project diagnosis and corpus workflows.
- RBOK lawbook POC passes with a green strict fidelity gate.
- Public release documentation exists.
- Regulated documentation baseline exists.
- Evidence templates and automation scripts exist.
- The public claim boundary is explicit.
- Formal regulated certification is not claimed.

## Current Claim Boundary

Allowed wording:

```text
Nomos v0.1.0-ALPHA has a canonical-first CLI, a passing RBOK lawbook POC, strict fidelity gates, and a regulated-readiness documentation/evidence baseline.
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
| `NQ-3` | Nomos self-compliance is executable and evidence-producing. | Repo-local gates green, reference canon actionable, no critical gap hidden. |
| `NQ-4` | Nomos evidence can be consumed by Praxis or another assurance layer. | Shared evidence contract, fixture validation, RBOK proof, Praxis boundary recorded. |
| `NQ-5` | Scoped validation pack ready for regulated-client review. | Validation inventory, release bundle, approval records, training records, retention policy. |
| `NQ-6` | Independent reconstruction/audit readiness. | Independent reviewer can rebuild release evidence from source, artifacts, hashes, logs, and approvals. |

The immediate target after `v0.1.0-ALPHA` is `NQ-3`.

## Active Gap Register

| Gap | Severity | Blocks | Closure condition |
|---|---|---|---|
| `GAP-QMS-OWNER` | Major | regulated-grade claim | Named quality, technical, and security owners recorded. |
| `GAP-CODEOWNERS` | Major | controlled review | CODEOWNERS active with real users or teams. |
| `GAP-TRAINING` | Major | SOP effectiveness | Role matrix and training records exist. |
| `GAP-GITHUB-LIVE-CONFIG` | Major | GitHub operating model | Branch/ruleset/environment/security evidence exported and hashed. |
| `GAP-LICENSED-BIBLES` | Major | GAMP/ISO clause mapping | Licensed artifacts acquired, hashed, reviewed, and sidecar-governed. |
| `GAP-REFERENCE-MATRIX` | Major | public claim governance | Every registered reference maps to controls, evidence, and claim status. |
| `GAP-VALIDATION-PACK` | Major | customer validation support | Intended use, risk, URS/SRS, tests, deviations, and summary are linked. |
| `GAP-INDEPENDENT-AUDIT` | Major | NQ-6 readiness | Reconstruction review executed and findings retained. |
| `GAP-PRAXIS-BOUNDARY` | Medium | joint product claims | Praxis evidence status remains explicit and non-overclaimed. |

## Workstream A - Governance And Ownership

Goal: turn draft process documents into governed records.

Steps:

1. Assign named quality, technical, and security owners.
2. Activate CODEOWNERS for controlled docs, release gates, schemas, and evidence scripts.
3. Create training records for owners and release approvers.
4. Define approval meaning and limitation for GitHub reviews.

Done when:

- CODEOWNERS routes controlled files;
- training records exist;
- approval semantics are documented;
- release PRs show appropriate review routing.

## Workstream B - GitHub Evidence

Goal: make GitHub configuration match the documentary QMS model.

Required evidence:

- branch protection or ruleset on `main`;
- required PR review;
- required status checks;
- stale review dismissal where applicable;
- force-push disabled;
- protected release environment;
- security feature status;
- audit-log export or equivalent evidence record;
- artifact retention policy.

Done when:

```bash
python scripts/regulated_github_qms_audit.py --repo RBOKproject/NOMOS --output .regulated-evidence-pack/github-qms-audit.json
```

reports no major unaccepted GitHub evidence gaps, or every remaining gap has a documented waiver with expiry.

## Workstream C - Reference Bibles

Goal: make regulatory and quality references controlled evidence, not decorative citations.

Steps:

1. Acquire required licensed references through legitimate channels.
2. Store full licensed text outside the public repository unless license permits otherwise.
3. Create licensed intake sidecars with SHA-256, source, license holder, permitted use, reviewer, and decision.
4. Snapshot public references from official sources.
5. Process public and licensed references read-only with Nomos.
6. Emit only license-permitted artifacts.

Done when:

- required bibles have hash evidence;
- license review is complete;
- public references have provenance;
- clause-level or summary-level use is explicitly allowed or blocked.

## Workstream D - Reference-To-Control Closure

Goal: ensure every reference supports an actual control, requirement, test, evidence artifact, or blocked claim.

Steps:

1. Update the control matrix.
2. Link references to intended use and risks.
3. Link controls to implementation, tests, evidence, and release gates.
4. Mark non-applicable items with rationale.
5. Fail gates on decorative references.

Done when:

- public claims map to evidence;
- unmapped references are blocked or removed;
- the public README does not exceed the matrix.

## Workstream E - Validation And Release Bundle

Goal: create a scoped validation package a regulated customer can review.

Required bundle content:

- release tag and commit;
- CI run URLs;
- regulated documentation gate report;
- regulated evidence pack;
- reference canon report;
- GitHub QMS audit report;
- RBOK lawbook E2E artifact summary;
- source/corpus read-only attestation;
- open deviations and waivers;
- approval record;
- public claim boundary;
- rollback or follow-up plan.

Done when an independent reviewer can reconstruct the release decision from retained evidence.

## Workstream F - Praxis Compatibility

Goal: keep Praxis aligned without using it as unsupported proof for Nomos.

Rules:

- Nomos must publish stable producer artifacts first.
- Praxis may consume Nomos evidence only through a documented contract.
- Praxis evidence remains advisory until independently validated.
- Joint claims require both product claim boundaries to be updated.

## Dependency Tree

```text
Governance owners
  -> CODEOWNERS and approval semantics
  -> GitHub evidence
  -> release bundle

Licensed and public references
  -> reference-to-control matrix
  -> validation inventory
  -> release bundle

RBOK POC and strict gates
  -> alpha claim boundary
  -> NQ-3 readiness evidence

Release bundle
  -> independent reconstruction review
  -> NQ-5/NQ-6 claim review

Nomos evidence contract
  -> Praxis compatibility
```

## Operating Gates

Run before changing public claims:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
python scripts/regulated_reference_canon.py --report .regulated-doc-gate/reference-canon-report.json
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

## Completion Definition

This closure plan is complete when:

- QMS owners and training records are active;
- GitHub live evidence is exported and retained;
- licensed references are acquired, hashed, reviewed, and governed;
- public references are snapshotted and mapped;
- every claim maps to evidence or a blocked status;
- validation inventory links intended use, risk, requirements, tests, evidence, and release decision;
- release evidence bundle can be independently reconstructed;
- remaining gaps are explicitly waived with expiry and risk acceptance;
- public-facing docs do not exceed the achieved quality level.

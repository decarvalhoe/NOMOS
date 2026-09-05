# 28 - Regulated Compliance Closure Plan

Date: 2026-09-05
Release context: `v0.1.0-ALPHA`

## Purpose

This is the active, **independent regulated-assurance roadmap** for moving Nomos
from alpha regulated-readiness to a defensible regulated-market posture. It is
not a phase of the product or DevOps roadmap.

The objective is not to claim certification prematurely. The objective is to make every future public and customer-facing quality claim reconstructible from repository state, GitHub configuration evidence, licensed-reference sidecars, CI artifacts, validation records, and independent review evidence.

## Independence And Operating Model

[ADR-VRC-0004](adr/0004-independent-roadmaps-risk-based-validation.md) separates
product, DevOps and regulated assurance:

- product versions and integrations advance without waiting for calendar
  evidence, signatures, approvals, acquisitions or public-log writes;
- this roadmap may operate a control manually before automation exists;
- a supporting tool may be developed and technically verified before this
  roadmap validates it for a regulated intended use;
- until that validation is complete, the tool is `supporting_use`: its output
  is reconciled to source evidence and is not the sole authority for a
  regulated decision;
- missing evidence remains a real gap and locks only the named claim or use. It
  is never turned into a fake pass and never freezes unrelated development.

The machine-readable lane and dispatch state is
[`docs/roadmap-lanes.yaml`](roadmap-lanes.yaml). `passive`, `human` and
`external` items remain visible but never enter the autonomous engineering
dispatcher.

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

## Independent Regulated Roadmap

The waves below are sequenced by assurance risk, not by product version. Work
may continue in another wave when one item awaits a human or external input;
the dependent claim simply stays locked.

| Wave | Scope | Manual-first rule | Exit affects |
|---|---|---|---|
| `R0 — scope` | intended uses, risk classification, owners, claim boundaries | documents and accountable review are sufficient | what regulated assurance is being attempted |
| `R1 — operate` | reviews, CAPA, training, release decisions, retention | authentic records matter; tooling is optional | QMS/process-effectiveness claims |
| `R2 — evidence` | scheduled evidence, reference acquisition/licence decisions, source provenance | collectors assist; unavailable evidence remains blocked | only the named evidence/reference claim |
| `R3 — validate tools` | validate tools used by R0-R2 according to intended use and impact | manual verification remains until validation | whether a tool may become authoritative for that use |
| `R4 — intended use` | customer/use-specific validation and independent reconstruction | consumes frozen product artifacts | scoped customer claim, never generic product truth |

### Risk-Based Validation Of Supporting Tools

| Impact | Examples | Before validation | Validation expectation before sole reliance |
|---|---|---|---|
| `support` | authoring, formatting, reminders | manual review of every output | intended use + representative functional checks |
| `evidence` | evidence assembly, transformation, indexing | retained source/output reconciliation | traceability, positive/negative tests, version control, sampled reconstruction |
| `decision` | blocking gate, disposition, release recommendation | accountable human verifies decision and source | approved intended use, risk assessment, requirements, adversarial tests, change control, validation summary |
| `critical_decision` | autonomous legal/clinical/safety conclusion | prohibited | use-specific validation and accountable approval; outside generic Nomos claims |

Development and validation are therefore decoupled, not confused: later
validation is acceptable when reliance remains proportionately bounded in the
meantime.

## Active Gap Register

| Gap | Severity | Blocks | Closure condition |
|---|---|---|---|
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

1. Enforce the intake, review and no-full-text policy with synthetic fixtures
   and explicit blocked states; tooling does not wait for acquisition.
2. Snapshot and process public references from official sources, independently
   of licensed references.
3. Acquire required licensed references through legitimate channels.
4. Store full licensed text outside the public repository unless license permits otherwise.
5. Create licensed intake sidecars with SHA-256, source, license holder, permitted use, reviewer, and decision.
6. Process licensed references read-only only after that decision.
7. Emit only license-permitted artifacts.

Done when:

- every available required bible has hash evidence and every unavailable one is explicitly blocked;
- licence review is complete for each reference actually used at clause level;
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
The bundle may first be prepared and rehearsed with pending approvals and open
risks; that DevOps result is not an executed release. Authentic approvals, tag
and publication remain regulated acts in this roadmap.

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

## Regulated Claim Relationships (Not Engineering Dependencies)

```text
Owners / authentic approvals / training records
  -> process-effectiveness claims only

Eight consecutive retained scheduled runs
  -> repeated-CI-evidence claim only

Each licensed acquisition and licence decision
  -> that reference's clause-level use and claim only

Prepared candidate bundle (DevOps)
  + authentic release decision (regulated)
  -> release-executed-through-SOP claim

Validated intended use + frozen product evidence
  -> scoped customer reliance
```

No arrow above blocks product or DevOps task selection. When an input is absent,
the regulated claim remains `blocked` or `requires_evidence`, and this roadmap
continues on its next eligible activity.

## Operating Gates

Run before changing public claims:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
python scripts/regulated_reference_canon.py --report .regulated-doc-gate/reference-canon-report.json
python scripts/regulated_github_qms_audit.py --repo RBOKproject/NOMOS --output .regulated-evidence-pack/github-qms-audit.json
```

Consume product gates (the product lane owns their implementation):

```bash
cd cli
go test ./...
go vet ./...
```

Run schema gates:

```bash
cue vet specs/*.cue
```

Passing these commands proves their documented software behavior. Before this
roadmap relies on a supporting tool as the sole basis of an `evidence` or
`decision` control, record its intended use, impact, validation state and
reliance boundary in `docs/roadmap-lanes.yaml`, then execute the proportionate
validation described above. A manual control can remain the authority until
then.

## Completion Definition

This regulated-assurance roadmap is complete for a named intended use when:

- QMS owners and training records are active;
- GitHub live evidence is exported and retained;
- licensed references are acquired, hashed, reviewed, and governed;
- public references are snapshotted and mapped;
- every claim maps to evidence or a blocked status;
- validation inventory links intended use, risk, requirements, tests, evidence, and release decision;
- release evidence bundle can be independently reconstructed;
- remaining gaps are explicitly waived with expiry and risk acceptance;
- public-facing docs do not exceed the achieved quality level.

Its completion or incompletion has no effect on whether the product and DevOps
roadmaps continue. It affects only the regulated-use and claim boundary named
by the retained evidence.

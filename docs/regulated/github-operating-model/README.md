# GitHub Regulated Operating Model

document_id: NOMOS-GH-QMS-001
version: 0.1.0
status: draft
effective_status: not_effective
owner: not_assigned
approver: not_assigned
public_claim_boundary: "GitHub operating model draft only; no validated eQMS or Part 11 claim."

## Purpose

Define how Nomos can use GitHub as the primary documentary and evidence operating system for regulated-by-design work.

This model is intentionally conservative:

- GitHub can host controlled documents, issues, PR reviews, workflow evidence, artifacts, releases and audit-log exports.
- GitHub alone does not make Nomos compliant.
- GitHub approvals are not automatically regulated electronic signatures.
- Missing owners, approvals, training, audit exports and retention evidence remain visible gaps.

## Official GitHub Capabilities Used

| GitHub capability | Regulated use | Limitation |
|---|---|---|
| Issue forms | Structured deviations, CAPA, validation records, audit findings and regulated gaps. | Issue forms are converted to markdown and are not immutable by themselves. |
| Pull request templates | Change impact assessment and evidence checklist. | Review discipline depends on branch/ruleset enforcement. |
| Branch protection / rulesets | PR-only change control, required reviews, required checks, blocked force push/deletion. | Must be configured in repository settings; repo files cannot enforce all settings alone. |
| CODEOWNERS | Automatic review requests for controlled areas. | Requires real GitHub users/teams; no fake owners are created. |
| Actions | Automated validation, test execution and evidence generation. | Workflow logs/artifacts need retention and export policy. |
| Workflow artifacts | Retained validation output, release evidence and corpus artifacts. | GitHub artifact retention may be shorter than regulated retention requirements. |
| Artifact attestations | Build provenance and subject digest evidence. | Private/internal repositories may require GitHub Enterprise Cloud for attestations. |
| Environments | Manual approvals before protected jobs/releases. | Availability of required reviewers depends on repository visibility and plan. |
| Releases | Release bundle publication and immutable-ish tagged evidence boundary. | Tag/release protection must be configured; release notes are not a substitute for validation approval. |
| Organization audit log | Review who did what and when at org level. | GitHub docs state the audit log lists events within the last 180 days; longer retention needs export. |

## GitHub-Only QMS Pattern

```text
controlled markdown/yaml docs
  -> PR template impact assessment
  -> required checks
  -> codeowner/quality/security review
  -> issue-linked approval or deviation
  -> Actions artifacts and attestations
  -> release evidence bundle
  -> periodic audit-log export
  -> management review issue
```

## Required Repository Configuration

These settings must be configured in GitHub, not only documented in the repo.

| Control | Required configuration | Evidence status |
|---|---|---|
| PR-only changes | Protect `main` and release branches; disallow direct pushes except approved automation. | requires_evidence |
| Required reviews | Require at least one review; require code owner review for controlled areas when real teams exist. | requires_evidence |
| Required checks | Require CI, regulated documentation gate, RBOK E2E where applicable, CUE/YAML checks. | requires_evidence |
| Stale review dismissal | Dismiss stale approvals after new commits. | requires_evidence |
| Force push/delete | Block force pushes and branch deletion on protected branches. | requires_evidence |
| Rulesets | Add repository/org rulesets for branch/tag protection and restricted bypass. | requires_evidence |
| Environments | Add protected `regulated-release` environment with required reviewer and self-review prevention where supported. | requires_evidence |
| Artifacts | Define retention and export policy for validation and release evidence. | requires_evidence |
| Audit log | Export organization audit log on a schedule if regulated retention exceeds GitHub online window. | requires_evidence |
| Security | Enable Dependabot, secret scanning/code scanning as available under the plan. | requires_evidence |

## Active Repo Tooling

The repo now carries:

- `.github/ISSUE_TEMPLATE/regulated-gap.yml`
- `.github/ISSUE_TEMPLATE/deviation-capa.yml`
- `.github/ISSUE_TEMPLATE/validation-record.yml`
- `.github/ISSUE_TEMPLATE/audit-finding.yml`
- `.github/ISSUE_TEMPLATE/release-readiness.yml`
- `.github/ISSUE_TEMPLATE/controlled-document-change.yml`
- `.github/PULL_REQUEST_TEMPLATE.md`
- `.github/CODEOWNERS` as a no-fake-owner placeholder
- `.github/workflows/regulated-documentation-gate.yml`
- `scripts/regulated_docs_gate.py`
- `.github/workflows/regulated-evidence-pack.yml`
- `scripts/regulated_evidence_pack.py`
- `scripts/regulated_github_qms_audit.py`
- `tests/test_regulated_automation.py`

## Automation Boundary

The current tools can automate:

- YAML and controlled-document metadata checks;
- prohibited overclaim detection;
- repository-local evidence hashing;
- ALCOA+ oriented evidence inventory;
- issue-form, PR-template, CODEOWNERS and workflow presence checks;
- live GitHub settings evidence collection when `gh api` has sufficient access;
- scheduled evidence-pack artifact upload.

The current tools cannot yet automate:

- assigning real quality owners;
- proving personnel training;
- validating GitHub itself as a regulated eQMS;
- turning GitHub reviews into Part 11 signatures;
- guaranteeing retention beyond GitHub plan or export limits;
- independent audit conclusions.

Those items must remain gaps until a governed process and objective evidence exist.

## Evidence Rules

GitHub issue or PR records can be used as evidence only when they include:

- stable URL;
- creation timestamp;
- actor;
- affected intended use;
- risk level;
- source/control references;
- evidence links;
- decision or status;
- reviewer/approver identity when applicable.

If any field is missing, the record remains `requires_evidence`.

## Part 11 Boundary

GitHub reviews, comments and approvals are not claimed as Part 11 electronic signatures.

Before any e-signature claim, Nomos needs:

- intended-use assessment;
- identity and access controls;
- signature meaning;
- signature manifestation;
- signature-to-record binding;
- audit trail;
- record retention;
- system validation;
- written accountability policy;
- training records.

Current status:

```yaml
github_as_documentary_qms: draft_candidate
github_as_validated_eqms: false
github_as_part_11_esignature_system: false
regulated_grade_claim_allowed: false
```

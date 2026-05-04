# NOMOS GitHub Workflow Integration Design

Date: 2026-05-04
Status: design approved in conversation; implementation pending

## Goal

Integrate NOMOS into GitHub workflows so a configured corpus scope can
be processed automatically when a source pull request targets `main`.
NOMOS must detect the scoped differential, regenerate only the impacted
NOMOS resources, publish outputs according to a risk-based policy, and
maintain traceability as a hard product contract.

The user correction is explicit: the trigger is a PR to **`main`** of
the source corpus repository, not "mail".

## Current Repository Context

NOMOS already has these building blocks:

- `nomos corpus scan`, `manifest`, `validate-sidecar`, `feed`,
  `body-ledger`, `attest`, and `strict`.
- Existing GitHub workflows for CI, corpus scan, RBOK lawbook E2E,
  runtime E2E, regulated docs, and evidence packs.
- Read-only corpus checkout patterns that disable corpus push remotes.
- Artifact upload patterns and source mutation guards.

The new work should extend those patterns instead of creating a
parallel engine.

## Product Decisions

### Config Location

NOMOS must support both:

- **source-owned config** in the corpus repository;
- **output-owned config** in the output repository.

There is no implicit fallback between them. The workflow or future App
must declare which config repository is authoritative.

```yaml
nomos_config:
  owner: corpus # corpus | output
  path: .nomos/corpus-workflows.yaml
```

### Workflow Location

NOMOS must support both:

- **source repo workflow**: triggered by `pull_request` to `main` in
  the corpus repository;
- **output repo workflow**: triggered by `repository_dispatch`,
  `workflow_dispatch`, schedule, or another explicit event when a
  consumer/output project drives the update.

The implementation should use reusable GitHub Actions workflows first,
and remain GitHub App-ready. A GitHub App can later orchestrate the same
contract by listening to GitHub events and dispatching the reusable
workflow.

Relevant GitHub references:

- <https://docs.github.com/en/actions/how-tos/reuse-automations/reuse-workflows>
- <https://docs.github.com/en/actions/reference/workflows-and-actions/reusing-workflow-configurations>
- <https://docs.github.com/actions/learn-github-actions/workflow-syntax-for-github-actions>
- <https://docs.github.com/en/rest/checks/runs>

### Publication Modes

Publication is configurable per scope:

```yaml
publish:
  mode: artifact_only # artifact_only | pull_request | direct_push
  target_repo: output # corpus | output | owner/repo
  target_branch: main
  target_path: rbok-lawbook/
  branch_strategy: fixed # fixed | per_pr | per_source_ref | dated
  risk_class: low # low | medium | high | regulated
```

Rules:

- `artifact_only` never commits.
- `pull_request` opens or updates a generated-output PR.
- `direct_push` is allowed only when explicitly configured.
- `target_branch` is configurable; it is not forced to `main`.
- `target_path` is mandatory for any writing mode.
- NOMOS may write only under `target_path`.
- A protected branch rejection is a hard workflow failure, not a silent
  fallback.
- The source corpus remains read-only during analysis in every mode.

Recommended defaults:

| Risk class | Default publication | Direct push policy |
|---|---|---|
| `low` | `direct_push` or `artifact_only` | Allowed to configured branch/path |
| `medium` | `pull_request` | Allowed only to generated branch/path |
| `high` | `pull_request` | Requires explicit waiver |
| `regulated` | `pull_request` + evidence gate | Direct push blocked unless controlled decision exists |

### Traceability Contract

Traceability is mandatory and independent of publication mode.

Every run must produce a trace manifest. The manifest is versioned with
outputs for `pull_request` and `direct_push`, and uploaded as an Actions
artifact for `artifact_only`.

Minimum manifest shape:

```yaml
schema_version: "0.1.0"
run:
  event: pull_request
  workflow_run_id: "..."
  generated_at: "..."
corpus:
  repo: RBOKproject/realisons-business
  base_ref: main
  base_sha: "..."
  head_ref: feature/update-guide
  head_sha: "..."
  pull_request: 123
scope:
  id: rbok-lawbook
  paths:
    - 01_rbok/**
diff:
  changed_paths:
    - 01_rbok/chapter.md
  impacted: true
output:
  repo: RBOKproject/nomos-rbok-artifacts
  branch: main
  path: rbok-lawbook/
  commit_sha: "..."
artifacts:
  feed: feed.json
  body_ledger: corpus-body-ledger.json
  rag_metadata: rag-metadata.json
  attestation: attestation.json
  diff_report: nomos-diff.json
policy:
  publish_mode: direct_push
  risk_class: low
  generated_path_guard: pass
  source_read_only_guard: pass
```

No NOMOS output is valid without a trace manifest. A manifest/output
divergence must fail the workflow.

### Source PR Comment

Commenting on the source PR is configurable:

```yaml
notify:
  source_pr_comment:
    enabled: true
    mode: summary # summary | detailed | failures_only
    include:
      - changed_scopes
      - diff_summary
      - output_location
      - trace_manifest
      - gate_status
```

If enabled, NOMOS comments on the source PR with the scoped diff,
impacted scopes, gate status, and links to outputs, artifacts, PRs, or
direct-push commits.

### PR Preview And Durable Publication

The source PR remains the central event:

```text
PR opened / synchronized / reopened / ready_for_review against main
  -> load explicit NOMOS config
  -> compute scoped diff
  -> run NOMOS incrementally for impacted scopes
  -> emit preview artifacts and trace manifest
  -> comment source PR when configured
  -> publish according to scope policy
```

Default publication policy should be:

```yaml
run_policy:
  pr_source:
    enabled: true
    on_events: [opened, synchronize, reopened, ready_for_review]
    preview_outputs: true
  publish_on:
    pr_source: false
    merge_to_main: true
```

`publish_on.pr_source: true` is allowed for low-risk or explicitly
waived scopes. If a pre-merge output is published, it must be marked as
preview unless the policy explicitly allows durable pre-merge output.

## Configuration Contract

Example `.nomos/corpus-workflows.yaml`:

```yaml
schema_version: "0.1.0"
workflows:
  - id: rbok-lawbook
    description: RBOK lawbook canonical output
    source:
      repo: RBOKproject/realisons-business
      base_branch: main
      paths:
        - 01_rbok/**
      extensions:
        - .md
        - .yaml
        - .json
      profile: rbok-lawbook
    output:
      repo: RBOKproject/nomos-rbok-artifacts
      branch: main
      path: rbok-lawbook/
    nomos:
      corpus_id: rbok-lawbook
      project_id: rbok
      commands:
        - scan
        - manifest
        - feed
        - body-ledger
        - attest
        - strict
    publish:
      mode: pull_request
      target_repo: output
      target_branch: main
      target_path: rbok-lawbook/
      branch_strategy: per_pr
      risk_class: medium
    notify:
      source_pr_comment:
        enabled: true
        mode: summary
        include:
          - changed_scopes
          - diff_summary
          - output_location
          - trace_manifest
          - gate_status
```

## Security And Permissions

The workflow must use least privilege:

- Source corpus checkout: read-only token, `persist-credentials: false`,
  push remote disabled.
- Output publication: separate token with write rights only to the
  output target repository.
- Same-repo output: write token may touch only configured generated
  paths; path guard must fail otherwise.
- Pull request comments: `pull-requests: write` or `issues: write`,
  depending on GitHub event context.
- Direct push: explicit policy plus generated path guard.
- Anti-loop: commits generated by NOMOS must include marker metadata and
  workflow path filters must ignore configured generated output paths.

## GitHub App Readiness

The first implementation should be reusable-workflow based. It must not
preclude a future GitHub App.

The future App should reuse the same:

- config file;
- scope identifiers;
- trace manifest;
- publish policy;
- source PR comment format;
- run result statuses;
- output artifact layout.

The App may later add installation management, check runs, richer PR
annotations, dashboards, and cross-repository orchestration.

## Non-Goals For First Implementation

- No hosted GitHub App backend in the first increment.
- No silent automatic config discovery across source/output repos.
- No direct source corpus mutation during analysis.
- No direct push unless explicitly configured in the scope.
- No regulated compliance claim from workflow integration alone.

## Acceptance Criteria

- A source-owned config can trigger a scoped PR preview against `main`.
- An output-owned config can trigger the same reusable workflow against
  a read-only corpus checkout.
- The workflow computes a scoped differential and skips unaffected
  scopes.
- Every output mode emits a trace manifest.
- `artifact_only`, `pull_request`, and `direct_push` are all represented
  in tests or fixtures.
- Source PR comments are configurable and include the trace link when
  enabled.
- The implementation remains compatible with a future GitHub App.

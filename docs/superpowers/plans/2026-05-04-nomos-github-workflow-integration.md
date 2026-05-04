# NOMOS GitHub Workflow Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build GitHub workflow integration so NOMOS can run on scoped corpus pull requests to `main`, regenerate impacted outputs, publish them according to risk-based policy, and preserve traceability.

**Architecture:** Add a versioned workflow config contract, a scoped diff planner, a reusable GitHub Actions workflow, and a guarded publisher. Source and output repositories can own config; publication can be artifact-only, PR-based, or direct push; every run emits a trace manifest.

**Tech Stack:** Go CLI, CUE schemas, GitHub Actions reusable workflows, Bash/Python helper scripts, GitHub CLI/API, existing NOMOS corpus commands.

---

## Task 1: Config Contract And Fixtures

**Files:**

- Create: `specs/nomos-github-workflow.cue`
- Create: `specs/examples/nomos-github-workflow.source-owned.valid.yaml`
- Create: `specs/examples/nomos-github-workflow.output-owned.valid.yaml`
- Create: `specs/examples/nomos-github-workflow.invalid.yaml`
- Modify: `specs/examples/README.md`

- [ ] **Step 1: Write schema fixtures**

Create fixtures covering:

- source-owned config;
- output-owned config;
- `artifact_only`, `pull_request`, and `direct_push`;
- configurable `target_branch`;
- mandatory `target_path`;
- source PR comment enabled/disabled.

- [ ] **Step 2: Add CUE schema**

Define fields:

- `schema_version`;
- `workflows[].id`;
- `source.repo`, `source.base_branch`, `source.paths`, `source.profile`;
- `output.repo`, `output.branch`, `output.path`;
- `publish.mode`, `publish.target_repo`, `publish.target_branch`,
  `publish.target_path`, `publish.branch_strategy`, `publish.risk_class`;
- `notify.source_pr_comment`.

- [ ] **Step 3: Verify schema**

Run:

```bash
cue vet specs/nomos-github-workflow.cue specs/examples/nomos-github-workflow.source-owned.valid.yaml
cue vet specs/nomos-github-workflow.cue specs/examples/nomos-github-workflow.output-owned.valid.yaml
```

Expected: both commands pass.

- [ ] **Step 4: Verify invalid fixture fails**

Run:

```bash
cue vet specs/nomos-github-workflow.cue specs/examples/nomos-github-workflow.invalid.yaml
```

Expected: command fails because `direct_push` lacks a valid generated
target path or explicit policy.

## Task 2: Trace Manifest Contract

**Files:**

- Create: `specs/nomos-trace-manifest.cue`
- Create: `specs/examples/nomos-trace-manifest.valid.yaml`
- Create: `specs/examples/nomos-trace-manifest.invalid.yaml`
- Modify: `specs/examples/README.md`

- [ ] **Step 1: Write valid trace fixture**

Include:

- run event and workflow run id;
- corpus repo, base/head refs and SHAs;
- pull request number;
- scope id and scoped paths;
- changed paths;
- output repo, branch, path, commit sha;
- artifact filenames;
- policy mode, risk class, generated path guard, source read-only guard.

- [ ] **Step 2: Add CUE schema**

Make `corpus.base_sha`, `corpus.head_sha`, `scope.id`,
`output.path`, and `policy.publish_mode` mandatory.

- [ ] **Step 3: Verify trace schema**

Run:

```bash
cue vet specs/nomos-trace-manifest.cue specs/examples/nomos-trace-manifest.valid.yaml
```

Expected: pass.

- [ ] **Step 4: Verify invalid trace fails**

Run:

```bash
cue vet specs/nomos-trace-manifest.cue specs/examples/nomos-trace-manifest.invalid.yaml
```

Expected: fail due to missing source or output SHA.

## Task 3: Scoped Diff Planner

**Files:**

- Create: `cli/internal/githubworkflow/config.go`
- Create: `cli/internal/githubworkflow/diff.go`
- Create: `cli/internal/githubworkflow/config_test.go`
- Create: `cli/internal/githubworkflow/diff_test.go`
- Modify: `cli/internal/app/app.go`
- Modify: `cli/internal/app/app_test.go`

- [ ] **Step 1: Write config loader tests**

Cover:

- source-owned config load;
- output-owned config load;
- unknown `publish.mode` rejected;
- `direct_push` without `target_path` rejected;
- two workflows with duplicate id rejected.

- [ ] **Step 2: Implement config loader**

Use existing YAML parsing pattern from corpus sidecar parsing. Return a
typed config struct and validation findings.

- [ ] **Step 3: Write scoped diff tests**

Cover:

- PR changes under configured scope mark that scope impacted;
- generated output path changes are ignored to prevent loops;
- changes outside all scopes produce no impacted workflows.

- [ ] **Step 4: Implement diff planner**

Expose a Go function:

```go
func PlanScopedDiff(cfg WorkflowConfig, changedPaths []string) DiffPlan
```

It returns impacted workflows, ignored generated paths, and skipped
workflows with reasons.

- [ ] **Step 5: Add CLI command**

Add:

```bash
nomos github plan \
  --config .nomos/corpus-workflows.yaml \
  --changed-paths changed-paths.txt \
  --out nomos-diff.json
```

Unknown command behavior must remain non-zero.

- [ ] **Step 6: Verify Go tests**

Run:

```bash
cd cli
go test ./internal/githubworkflow ./internal/app -run 'Github|Workflow|Diff|Plan' -v
```

Expected: pass.

## Task 4: Reusable Workflow

**Files:**

- Create: `.github/workflows/nomos-corpus-workflow.yml`
- Create: `templates/github-workflows/nomos-source-pr.yml`
- Create: `templates/github-workflows/nomos-output-dispatch.yml`
- Modify: `docs/windows-corpus-setup.md` or create workflow setup docs if clearer.

- [ ] **Step 1: Add reusable workflow**

Use `workflow_call` inputs:

- `config_owner`;
- `config_path`;
- `corpus_repository`;
- `corpus_ref`;
- `base_ref`;
- `head_ref`;
- `changed_paths_artifact` or newline string;
- `output_repository`;
- `dry_run`.

Secrets:

- `NOMOS_CORPUS_READ_TOKEN`;
- `NOMOS_OUTPUT_WRITE_TOKEN`;

- [ ] **Step 2: Enforce read-only corpus checkout**

Use `persist-credentials: false` and disable push remote after checkout.

- [ ] **Step 3: Build NOMOS CLI and run plan**

The reusable workflow must build the CLI and run `nomos github plan`.
If no workflow scope is impacted, upload a trace manifest with
`impacted=false` and stop without publication.

- [ ] **Step 4: Add source PR caller template**

Template triggers on:

```yaml
pull_request:
  branches: [main]
  types: [opened, synchronize, reopened, ready_for_review]
```

It captures changed files and calls the reusable workflow.

- [ ] **Step 5: Add output-owned caller template**

Template supports `workflow_dispatch` and `repository_dispatch`, then
checks out corpus read-only and calls the reusable workflow.

- [ ] **Step 6: Validate workflow syntax**

Run:

```bash
python - <<'PY'
import yaml, pathlib
for p in pathlib.Path('.github/workflows').glob('*.yml'):
    yaml.safe_load(p.read_text())
for p in pathlib.Path('templates/github-workflows').glob('*.yml'):
    yaml.safe_load(p.read_text())
PY
```

Expected: no YAML parse error.

## Task 5: Output Publisher And Path Guard

**Files:**

- Create: `scripts/nomos_github_publish.py`
- Create: `tests/test_nomos_github_publish.py`
- Modify: `.github/workflows/nomos-corpus-workflow.yml`

- [ ] **Step 1: Write tests for path guard**

Cover:

- writing under `target_path` accepted;
- writing outside `target_path` rejected;
- path traversal `../` rejected;
- direct push without explicit `direct_push` policy rejected.

- [ ] **Step 2: Implement publisher**

Modes:

- `artifact_only`: writes output directory and trace manifest only.
- `pull_request`: creates/updates generated branch and PR.
- `direct_push`: commits directly to configured target branch/path.

- [ ] **Step 3: Add anti-loop marker**

Generated commits must include:

```text
[nomos-generated]
source-sha: <sha>
scope: <scope-id>
```

- [ ] **Step 4: Wire publisher into workflow**

After NOMOS artifacts are produced, call the publisher with config,
diff plan, output directory, and trace manifest path.

- [ ] **Step 5: Verify Python tests**

Run:

```bash
python -m unittest tests/test_nomos_github_publish.py -v
```

Expected: pass.

## Task 6: Source PR Commenter

**Files:**

- Create: `scripts/nomos_github_comment.py`
- Create: `tests/test_nomos_github_comment.py`
- Modify: `.github/workflows/nomos-corpus-workflow.yml`

- [ ] **Step 1: Write formatter tests**

Cover:

- `summary`;
- `detailed`;
- `failures_only`;
- disabled comments produce no API call payload.

- [ ] **Step 2: Implement comment formatter**

Include impacted scopes, diff summary, output location, trace manifest,
and gate status according to config.

- [ ] **Step 3: Use sticky comment marker**

Use an HTML marker:

```html
<!-- nomos-source-pr-comment:<scope-id> -->
```

The script updates the existing comment if present.

- [ ] **Step 4: Verify comment tests**

Run:

```bash
python -m unittest tests/test_nomos_github_comment.py -v
```

Expected: pass.

## Task 7: Trace Manifest Generation

**Files:**

- Create: `scripts/nomos_trace_manifest.py`
- Create: `tests/test_nomos_trace_manifest.py`
- Modify: `.github/workflows/nomos-corpus-workflow.yml`

- [ ] **Step 1: Write trace manifest tests**

Cover all publication modes and require source SHA, output SHA or
artifact reference, scope id, policy, and changed paths.

- [ ] **Step 2: Implement manifest generator**

The generator writes YAML and JSON forms:

- `nomos-trace.yaml`;
- `nomos-trace.json`.

- [ ] **Step 3: Validate manifest against CUE**

Run:

```bash
cue vet specs/nomos-trace-manifest.cue specs/examples/nomos-trace-manifest.valid.yaml
python -m unittest tests/test_nomos_trace_manifest.py -v
```

Expected: pass.

## Task 8: Documentation And Issue List

**Files:**

- Create: `docs/30-github-workflow-integration-issue-list.md`
- Modify: `docs/15-product-backlog.md`
- Modify: `docs/14-product-roadmap.md`
- Modify: `docs/README.md`

- [ ] **Step 1: Document setup**

Describe source-owned and output-owned setup, required secrets, branch
protection expectations, and publication modes.

- [ ] **Step 2: Add issue list**

Create child issue definitions for:

- config schema;
- diff planner;
- reusable workflow;
- publisher;
- PR comment;
- trace manifest;
- GitHub App readiness.

- [ ] **Step 3: Update claim boundary**

State that GitHub workflow integration automates evidence generation
and publication; it does not create regulated validation by itself.

- [ ] **Step 4: Run docs gate**

Run:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Expected: pass.

## Task 9: Full Verification

**Files:**

- No new files; verification only.

- [ ] **Step 1: Run Go tests**

Run:

```bash
cd cli
go test ./...
```

Expected: pass on a machine with Go available.

- [ ] **Step 2: Run Python tests**

Run:

```bash
python -m unittest discover -s tests -v
```

Expected: pass.

- [ ] **Step 3: Run docs/evidence gates**

Run:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
```

Expected: pass.

- [ ] **Step 4: Open PR**

Open a PR to `main` with:

- design link;
- implementation issue list link;
- test output;
- explicit claim boundary.

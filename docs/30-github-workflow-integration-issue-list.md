# 30 - GitHub Workflow Integration Issue List

Date: 2026-05-04
Status: planning backlog; implementation not started

## Purpose

This issue list turns the approved NOMOS GitHub workflow integration
design into implementation-ready work. It must be used before coding the
workflow integration.

Design source:
`docs/superpowers/specs/2026-05-04-nomos-github-workflow-integration-design.md`

Implementation plan:
`docs/superpowers/plans/2026-05-04-nomos-github-workflow-integration.md`

## Dependency Tree

```text
NGW-001 config schema and examples
  -> NGW-002 trace manifest schema
  -> NGW-003 scoped diff planner
  -> NGW-004 reusable GitHub workflow
  -> NGW-005 output publisher and path guard
  -> NGW-006 source PR commenter
  -> NGW-007 trace manifest generation
  -> NGW-008 source-owned and output-owned docs/templates
  -> NGW-009 GitHub App readiness boundary
  -> NGW-010 end-to-end fixture workflow
```

## NGW-001 - Config Schema And Examples

Release lane: GitHub workflow integration v0.1.

Deliverables:

- `specs/nomos-github-workflow.cue`.
- Source-owned and output-owned valid examples.
- Invalid example proving unsafe direct push fails validation.

Definition of done:

- Config supports source repo, output repo, scoped paths, publication
  mode, target branch, target path, risk class, and source PR comments.
- `cue vet` passes valid examples and rejects invalid examples.

Verification:

```bash
cue vet specs/nomos-github-workflow.cue specs/examples/nomos-github-workflow.source-owned.valid.yaml
cue vet specs/nomos-github-workflow.cue specs/examples/nomos-github-workflow.output-owned.valid.yaml
```

Claim impact: creates the workflow contract; does not automate runs yet.

## NGW-002 - Trace Manifest Schema

Release lane: GitHub workflow integration v0.1.

Deliverables:

- `specs/nomos-trace-manifest.cue`.
- Valid and invalid trace examples.

Definition of done:

- Every trace requires source SHA, output reference, scope id, changed
  paths, publication mode, and policy result.

Verification:

```bash
cue vet specs/nomos-trace-manifest.cue specs/examples/nomos-trace-manifest.valid.yaml
```

Claim impact: makes traceability a machine-checkable contract.

## NGW-003 - Scoped Diff Planner

Release lane: GitHub workflow integration v0.1.

Deliverables:

- Go package for loading workflow config and computing impacted scopes.
- CLI command `nomos github plan`.
- `nomos-diff.json` output.

Definition of done:

- PR changes under configured scope mark that scope impacted.
- Generated output paths are ignored to prevent loops.
- Changes outside all scopes produce a no-op plan.

Verification:

```bash
cd cli
go test ./internal/githubworkflow ./internal/app -run 'Github|Workflow|Diff|Plan' -v
```

Claim impact: enables differential runs.

## NGW-004 - Reusable GitHub Workflow

Release lane: GitHub workflow integration v0.1.

Deliverables:

- `.github/workflows/nomos-corpus-workflow.yml`.
- Source PR caller template.
- Output-owned caller template.

Definition of done:

- Source workflow supports `pull_request` to `main`.
- Output workflow supports `workflow_dispatch` and
  `repository_dispatch`.
- Corpus checkout is read-only with disabled push remote.

Verification:

```bash
python - <<'PY'
import yaml, pathlib
for p in pathlib.Path('.github/workflows').glob('*.yml'):
    yaml.safe_load(p.read_text())
for p in pathlib.Path('templates/github-workflows').glob('*.yml'):
    yaml.safe_load(p.read_text())
PY
```

Claim impact: makes NOMOS callable by GitHub Actions.

## NGW-005 - Output Publisher And Path Guard

Release lane: GitHub workflow integration v0.1.

Deliverables:

- Publisher helper supporting `artifact_only`, `pull_request`, and
  `direct_push`.
- Generated path guard.
- Anti-loop commit marker.

Definition of done:

- Direct push is impossible unless explicitly configured.
- Output write attempts outside `target_path` fail.
- Generated commits include source SHA and scope marker.

Verification:

```bash
python -m unittest tests/test_nomos_github_publish.py -v
```

Claim impact: enables risk-based publication while protecting source and
output repos from uncontrolled writes.

## NGW-006 - Source PR Commenter

Release lane: GitHub workflow integration v0.1.

Deliverables:

- Comment formatter and updater.
- Modes: `summary`, `detailed`, `failures_only`.
- Sticky comment marker.

Definition of done:

- Comments are created only when enabled.
- Existing NOMOS comment is updated instead of duplicated.
- Comment includes trace manifest and output location when configured.

Verification:

```bash
python -m unittest tests/test_nomos_github_comment.py -v
```

Claim impact: gives the source PR reviewer direct visibility into NOMOS
diffs and gates.

## NGW-007 - Trace Manifest Generation

Release lane: GitHub workflow integration v0.1.

Deliverables:

- `nomos-trace.yaml`.
- `nomos-trace.json`.
- Workflow upload or commit according to publication mode.

Definition of done:

- No output mode can finish without a trace manifest.
- Manifest/output divergence fails the workflow.

Verification:

```bash
python -m unittest tests/test_nomos_trace_manifest.py -v
cue vet specs/nomos-trace-manifest.cue specs/examples/nomos-trace-manifest.valid.yaml
```

Claim impact: enforces traceability across artifact-only, PR, and direct
push flows.

## NGW-008 - Source-Owned And Output-Owned Setup Docs

Release lane: GitHub workflow integration v0.1.

Deliverables:

- Setup guide for corpus-owned config.
- Setup guide for output-owned config.
- Required secrets and permissions matrix.

Definition of done:

- A maintainer can install the workflow in either source or output repo
  without guessing token names or config paths.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Claim impact: makes onboarding repeatable.

## NGW-009 - GitHub App Readiness Boundary

Release lane: GitHub workflow integration v0.2.

Deliverables:

- Document App event mapping to the same config contract.
- Define future permissions and webhook events.
- Define check-run/status output shape.

Definition of done:

- The workflow implementation can later be orchestrated by a GitHub App
  without changing corpus config or output manifest formats.

Verification:

```bash
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
```

Claim impact: prepares productization without blocking first workflow
delivery.

## NGW-010 - End-To-End Fixture Workflow

Release lane: GitHub workflow integration v0.1.

Deliverables:

- Fixture corpus with two scopes.
- Fixture output directory.
- E2E run proving impacted-scope regeneration and unaffected-scope skip.

Definition of done:

- Source PR-like changed path list impacts only the matching scope.
- All three publication modes are exercised in dry-run or local-safe
  mode.
- Trace manifest is produced for every mode.

Verification:

```bash
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\e2e.ps1
python -m unittest discover -s tests -v
```

Claim impact: proves workflow behavior on controlled fixtures before
connecting real projects.

## Issue Creation Policy

1. Create GitHub issues from `NGW-001` through `NGW-010` before coding.
2. Each issue must copy its deliverables, definition of done,
   verification, dependencies, and claim impact from this file.
3. Do not close an issue until the corresponding PR includes either a
   committed artifact, a fixture, or a linked GitHub Actions run.
4. Do not claim regulated validation from GitHub workflow integration.
5. Do not enable `direct_push` for high or regulated risk classes without
   an explicit controlled decision.

# 31 - GitHub Workflow Setup

This document is the operator setup guide for the NOMOS GitHub
workflow integration (NGW epic, `#385`). It is written for engineers,
QA leads, and regulated-customer maintainers who want to install the
reusable workflow into a corpus repository or an output repository
without guessing token names, config paths, or branch-protection
rules.

It pairs with:

- `docs/32-github-app-readiness-boundary.md` — the future GitHub App
  boundary (parallel ticket `#394`).
- `docs/30-github-workflow-integration-issue-list.md` — the ticket
  inventory and dependency tree.
- `docs/21-source-feed-integrity-engine.md` — the upstream
  source-to-feed integrity method that NGW publishes evidence for.

## 1. Purpose And Audience

You should read this guide when you want to:

- run NOMOS on every pull request that touches a configured corpus
  scope, with the source corpus checked out **read-only**;
- publish the resulting artifacts to a separate output repository
  (or a configured path inside the same repository) under a clearly
  declared publication mode;
- post a sticky comment on the source pull request summarising the
  scoped diff, the gate status, and the location of generated
  outputs;
- record a mandatory trace manifest for every run, regardless of
  publication mode.

The audience is the maintainer who will land the workflow YAML and
secrets. It assumes you understand what NOMOS does (see
`docs/01-method-overview.md`) and that you have read or skimmed the
source-to-feed integrity engine doc. It does not assume any prior
NGW familiarity.

## 2. Claim Boundary

The full vocabulary of permitted and forbidden public claims lives in
[`docs/public-claim-boundary.md`](public-claim-boundary.md) (SFI-00,
`#338`). NGW does not change that boundary. It only automates the
delivery of evidence the upstream gates have already produced.

NGW automates evidence generation and publication. It does not
validate, certify, or assert Part 11 / GxP compliance. The corpus
integrity gate is upstream of NGW; NGW is delivery infrastructure.

Concretely, this guide does **not**:

- say NGW makes a corpus "validated";
- say NGW makes a corpus "certified";
- say NGW makes a corpus "Part 11" or "GxP" compliant;
- say NGW gives a corpus a "regulated-grade" claim.

Those phrases are reserved by SFI-00 and require evidence the workflow
infrastructure cannot, on its own, produce. NGW publishes the trace
manifest, the diff plan, and the artifact bundle so that an upstream
gate result becomes inspectable; it does not lift the underlying
result to a higher claim level.

## 3. Architecture Overview

```text
source corpus repo                         output / artifacts repo
┌──────────────────┐                       ┌──────────────────┐
│ .nomos/          │ ── PR to main ──────▶ │ NOMOS run        │ ── publish ──┐
│  corpus-         │                       │ (reusable        │              │
│  workflows.yaml  │                       │  workflow)       │              │
│ caller workflow  │                       └──────────────────┘              │
│ (templates/...)  │                                                         │
└──────────────────┘                                                         ▼
                                                       artifact / output PR / direct push
                                                       + mandatory trace manifest
```

Three repositories may be involved:

- the **source corpus repository** — where the canonical authority
  text lives. Always checked out read-only.
- the **output repository** — where generated artifacts (feed,
  body ledger, RAG metadata, attestation, trace manifest) are
  published. May be the same repository as the source if the
  operator chooses, but the path guard still applies.
- `RBOKproject/Nomos` — where the reusable workflow lives
  (`.github/workflows/nomos-corpus-workflow.yml`). Caller workflows
  reference it via `uses:`.

The reusable workflow itself is described by tickets `#388` (planner),
`#389` (reusable workflow), `#390` (publisher), `#391` (commenter),
and `#392` (trace generator). This guide does not duplicate those
contracts; it tells the operator how to install and configure the
result.

## 4. Choose Your Config Owner

`.nomos/corpus-workflows.yaml` (the configuration contract from
NGW-01, `specs/nomos-github-workflow.cue`) can live in either the
source corpus repository or the output repository. The choice is
explicit; there is no implicit fallback.

| Config owner | Pros | Cons | Use when |
|---|---|---|---|
| Source-owned (`config_owner: corpus`) | Corpus team controls scopes; PR comments are natural; the source PR is the central event. | Source repo carries operational config and accumulates secrets. | Corpus team owns the workflow lifecycle and reviews scope changes. |
| Output-owned (`config_owner: output`) | Output team controls publication policy independently; source repo stays minimal. | Source repo cannot configure scopes; coordination needed when the corpus changes shape. | The output project drives the cadence (release schedule, downstream consumer); operator wants source-side surface to remain minimal. |

The schema and validating fixtures are pinned in
`specs/nomos-github-workflow.cue` (NGW-01, `#386`):

- source-owned example:
  [`specs/examples/nomos-github-workflow.source-owned.valid.yaml`](../specs/examples/nomos-github-workflow.source-owned.valid.yaml);
- output-owned example:
  [`specs/examples/nomos-github-workflow.output-owned.valid.yaml`](../specs/examples/nomos-github-workflow.output-owned.valid.yaml);
- intentionally invalid (regulated direct_push without
  controlled_decision):
  [`specs/examples/nomos-github-workflow.invalid.yaml`](../specs/examples/nomos-github-workflow.invalid.yaml).

Validate any local config with:

```bash
cue vet specs/nomos-github-workflow.cue \
        path/to/your/.nomos/corpus-workflows.yaml \
        -d '#NomosGitHubWorkflowConfig'
```

## 5. Required Secrets

NGW uses fine-grained, least-privilege tokens. The reusable workflow
in `RBOKproject/Nomos` (the `uses:` target) declares the secrets it
expects under `secrets:`; the caller is responsible for plumbing them
through.

| Secret | Required by | Purpose | Permission scope |
|---|---|---|---|
| `NOMOS_CORPUS_READ_TOKEN` | every workflow run that checks out a corpus other than `${{ github.repository }}` | read-only checkout of the source corpus repository | `contents: read` on the corpus repo only |
| `NOMOS_OUTPUT_WRITE_TOKEN` | `pull_request` and `direct_push` publication modes (NGW-05, `#390`) | publish artifacts (PR or direct push) to the output repository | `contents: write` + `pull-requests: write` on the output repo only |
| `GITHUB_TOKEN` (default) | source-PR sticky comment job (NGW-06, `#391`) | post or update the source-PR sticky comment | `pull-requests: write` declared at the comment job level |

Notes:

- The default `GITHUB_TOKEN` provided by GitHub Actions is sufficient
  for the source-PR comment because the comment is posted on the
  source repository itself.
- For source-owned setups where the corpus is the same repo as the
  workflow caller, `NOMOS_CORPUS_READ_TOKEN` may be set to the
  default `GITHUB_TOKEN` (the source-PR template does this).
- `NOMOS_OUTPUT_WRITE_TOKEN` should never be granted on the source
  corpus repository. The publisher (`scripts/nomos_github_publish.py`)
  treats path guard violations and unauthorised mutations as hard
  failures.
- NGW-04 declares `NOMOS_OUTPUT_WRITE_TOKEN` as `required: false` so
  callers using `mode: artifact_only` do not need to provision it.
  The publisher (NGW-05, `#390`) elevates the requirement when the
  configured mode is `pull_request` or `direct_push`.

## 6. Required Permissions

The caller templates (`templates/github-workflows/nomos-source-pr.yml`
and `templates/github-workflows/nomos-output-dispatch.yml`) declare
the minimum permissions the calling workflow needs. The reusable
workflow declares its own `permissions:` block; per-job overrides are
used inside the reusable workflow for the comment and publish jobs.

Source-owned caller (`nomos-source-pr.yml`):

```yaml
permissions:
  contents: read
  pull-requests: read
  actions: read
```

Output-owned caller (`nomos-output-dispatch.yml`):

```yaml
permissions:
  contents: read
  pull-requests: read
  actions: read
```

Reusable workflow (`.github/workflows/nomos-corpus-workflow.yml`)
top-level permissions:

```yaml
permissions:
  contents: read
  pull-requests: read
  actions: read
```

The reusable workflow elevates permissions per-job:

- the trace-manifest job runs with the same read-only set;
- the source-PR comment job (NGW-06) requests
  `pull-requests: write` on its own job-level `permissions:` block;
- the publisher job (NGW-05) requests `contents: write` against the
  output repository **only** via `NOMOS_OUTPUT_WRITE_TOKEN`,
  never via the default `GITHUB_TOKEN`.

The corpus checkout step combines two protections; both are required
and either alone is insufficient:

```yaml
- uses: actions/checkout@v4
  with:
    persist-credentials: false
- name: Disable corpus push remote (read-only enforcement)
  run: git -C corpus remote set-url --push origin DISABLED
```

## 7. Branch-Protection Expectations

Branch-protection rules belong on the **output** repository's target
branch. The source corpus is read-only by construction, so source-side
protection is the operator's normal review policy and is not an NGW
concern.

| Mode | Output branch protection |
|---|---|
| `artifact_only` | None required; NGW never commits in this mode. The trace manifest is uploaded as an Actions artifact only. |
| `pull_request` | "Require pull request review before merge" recommended on the output target branch. The NGW-generated PR is reviewed normally; merging is the operator's decision. |
| `direct_push` | Path-restricted protection: only the configured `target_path` may be pushed; outside paths must require review. Commits carry the `[nomos-generated]` anti-loop marker (`scripts/nomos_github_publish.py`). The schema reserves direct_push under `risk_class: regulated` for builds that also declare `controlled_decision`; the invalid fixture in `#386` demonstrates the rejection path. |

Refer to `specs/nomos-github-workflow.cue` (`#PublishSpec`) for the
binding contract: the `direct_push` + `regulated` combination
requires a non-empty `controlled_decision` waiver string and `cue vet`
fails closed otherwise.

## 8. Publication-Mode Tradeoffs

| Mode | Latency | Reviewer load | Risk |
|---|---|---|---|
| `artifact_only` | Seconds | None | None — no commits. Ideal for previewing on every PR. |
| `pull_request` | Minutes (review wait) | Medium | Low — the output PR is reviewed; the corpus PR is not blocked by output review. |
| `direct_push` | Seconds | None | Requires explicit policy + path guard + anti-loop marker. The schema rejects direct_push under `risk_class: regulated` unless `controlled_decision` is recorded. |

The reusable workflow honours the configured mode literally. It does
not silently fall back from a more conservative mode to a more
permissive one. A protected-branch rejection is a hard workflow
failure, not a fallback to `artifact_only`.

## 9. Step-By-Step: Source-Owned Setup

You are installing the workflow inside the source corpus repository.
The output may be the same repo (with a path guard) or a separate
output repository.

1. Add the configuration file to the source repo at
   `.nomos/corpus-workflows.yaml`. Start from
   [`specs/examples/nomos-github-workflow.source-owned.valid.yaml`](../specs/examples/nomos-github-workflow.source-owned.valid.yaml)
   and validate locally with:
   ```bash
   cue vet specs/nomos-github-workflow.cue \
           .nomos/corpus-workflows.yaml \
           -d '#NomosGitHubWorkflowConfig'
   ```
2. Copy the source-side caller template
   (`templates/github-workflows/nomos-source-pr.yml`) into the source
   repo at `.github/workflows/nomos.yml`. Adjust the `uses:` ref if
   you want to pin a specific NOMOS tag (defaults to `@main`).
3. If your `publish.mode` is `pull_request` or `direct_push`,
   provision `NOMOS_OUTPUT_WRITE_TOKEN` as a repository secret on
   the source repo. For `mode: artifact_only`, no extra secret is
   needed; the default `GITHUB_TOKEN` is enough.
4. (Optional) Configure the source-PR sticky comment by setting
   `notify.source_pr_comment.enabled: true` in the config and choosing
   `mode` and `include` per the schema. The default
   `GITHUB_TOKEN` covers the `pull-requests: write` permission the
   commenter job needs.
5. Open a test pull request that touches a configured `source.paths`
   glob. Verify the `nomos-diff` and `nomos-trace` Actions artifacts
   appear on the run, the source corpus `git status --short` is
   empty before and after the run (no mutation), and — when notify
   is enabled — the sticky PR comment includes the configured
   sections.

## 10. Step-By-Step: Output-Owned Setup

You are installing the workflow inside the output repository. The
source corpus is read-only and lives in a separate repo.

1. Add the configuration file to the output repo at
   `.nomos/corpus-workflows.yaml`. Start from
   [`specs/examples/nomos-github-workflow.output-owned.valid.yaml`](../specs/examples/nomos-github-workflow.output-owned.valid.yaml).
   `source.repo` points to the corpus repository;
   `output.repo: corpus` resolves to "the source corpus" (the other
   repo) at runtime; `publish.target_repo: output` resolves to the
   repository the workflow file lives in. Validate as above.
2. Copy the output-side caller template
   (`templates/github-workflows/nomos-output-dispatch.yml`) into the
   output repo at `.github/workflows/nomos-refresh.yml`. Adjust the
   `uses:` ref if you want to pin a specific NOMOS tag.
3. Provision two secrets on the output repo:
   - `NOMOS_CORPUS_READ_TOKEN` — fine-grained PAT or App token with
     `contents: read` on the source corpus repository, no write
     scope;
   - `NOMOS_OUTPUT_WRITE_TOKEN` — fine-grained PAT or App token with
     `contents: write` and `pull-requests: write` on the output repo
     itself.
4. Trigger via `workflow_dispatch` (manual operator run with an
   explicit `corpus_ref`) or `repository_dispatch` of type
   `nomos-corpus-update` (carrying `corpus_repository`, `corpus_ref`,
   and `base_ref` in the client payload). The output-side template
   reads from `inputs` first and falls back to
   `github.event.client_payload`.
5. Confirm the workspace contains
   `.nomos/corpus-workflows.yaml` after the implicit
   `actions/checkout@v4` (the template does this in the
   `stage_output_checkout` job and emits a `::warning::` if the file
   is missing).

## 11. Verification Checklist

After installing the workflow, run the following checks. Each one is
deterministic; failures point at a specific configuration error
rather than environmental noise.

- The source corpus's `git status --short` is empty both before and
  after a workflow run. Any non-empty result indicates the read-only
  guarantee was violated and is a hard failure of the integration.
- A trace manifest (`nomos-trace.yaml` and `nomos-trace.json`)
  appears as an Actions artifact for every run, including runs that
  declare `impacted: false` (the no-impact path generates a
  placeholder manifest with `impacted: false`).
- When `notify.source_pr_comment.enabled: true`, the source pull
  request shows a sticky NOMOS comment that lists the configured
  `include` sections (changed_scopes / diff_summary / output_location
  / trace_manifest / gate_status).
- For `mode: direct_push` runs, the resulting commit on the output
  branch carries the `[nomos-generated]` marker on its first line.
  The publisher (`scripts/nomos_github_publish.py`) emits this
  marker so future NOMOS runs can grep for it and skip their own
  outputs (anti-loop guard).
- For `mode: pull_request` runs, the generated output PR carries the
  same anti-loop marker in its title or body so the loop guard
  recognises it.

## 12. Troubleshooting Matrix

| Symptom | Likely cause | Fix |
|---|---|---|
| Workflow logs say "config not found at …" | Wrong `config_owner` or `config_path` in the caller template | Check the `with:` block in `nomos-source-pr.yml` / `nomos-output-dispatch.yml`. For output-owned setups, confirm `actions/checkout@v4` runs before the reusable-workflow call so `$GITHUB_WORKSPACE/.nomos/corpus-workflows.yaml` exists. |
| Path guard rejects every output | `target_path` is absolute or contains a `..` segment | Re-read `#386` schema. `target_path` must be a relative path matching the `#RelativeOutputPath` regex (no leading `/`, no `..` segment). |
| Sticky comment is duplicated on the source PR | The scope `id` changed across runs | Scope ids must be stable across runs of the same workflow. Treat any scope-id change as a config-breaking change and migrate the comment manually if needed. |
| `direct_push` rejected by the output branch | Branch protection on the output target branch blocks the write token | Either grant the output write token a path-restricted bypass for the configured `target_path`, or switch the mode to `pull_request` and let the PR be reviewed normally. |
| Workflow loops on its own outputs | `output.path` and `publish.target_path` disagree | The loop guard ignores generated paths only when both fields agree. Align them; the publisher path guard refuses to write outside `target_path`. |
| `cue vet` rejects the config with `incomplete value !=""` on `controlled_decision` | `mode: direct_push` was paired with `risk_class: regulated` and no `controlled_decision` was declared | This is the `#386` invariant. Either change the risk class, change the mode, or add a controlled-decision waiver string referencing the recorded approval. |
| The output repo has the trace manifest but no feed/body-ledger artifacts | The run took the no-impact path because no configured scope matched the changed paths | This is by design: the no-impact run emits a placeholder trace manifest with `impacted: false` and stops. Inspect the Actions artifact to confirm. |

## 13. Cross-References

- Engine method (SFI epic `#337`):
  [`docs/21-source-feed-integrity-engine.md`](21-source-feed-integrity-engine.md)
- Public claim boundary (SFI-00, `#338`):
  [`docs/public-claim-boundary.md`](public-claim-boundary.md)
- GitHub App readiness boundary (NGW-09, `#394` — parallel ticket;
  the file lands in #394):
  [`docs/32-github-app-readiness-boundary.md`](32-github-app-readiness-boundary.md)
- NGW issue list and dependency tree:
  [`docs/30-github-workflow-integration-issue-list.md`](30-github-workflow-integration-issue-list.md)
- Workflow config schema (NGW-01, `#386`):
  [`specs/nomos-github-workflow.cue`](../specs/nomos-github-workflow.cue)
- Trace manifest schema (NGW-02, `#387`):
  [`specs/nomos-trace-manifest.cue`](../specs/nomos-trace-manifest.cue)
- Reusable workflow (NGW-04, `#389`):
  `.github/workflows/nomos-corpus-workflow.yml`
- Caller templates:
  `templates/github-workflows/nomos-source-pr.yml`,
  `templates/github-workflows/nomos-output-dispatch.yml`

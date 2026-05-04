# 32 - GitHub App Readiness Boundary

This document is owned by NGW-009 (`#394`).

## 1. Purpose

This file is a **forward-compatibility contract** between the v0.1 NOMOS
GitHub workflow integration (the reusable workflow defined in `#389`)
and a future v0.2 NOMOS GitHub App orchestration layer.

It does two things only:

- It declares the surface a future App **must reuse without change**:
  the workflow config (`#386`), the trace manifest (`#387`), the
  reusable workflow (`#389`), the publisher path guard (`#390`), the
  source-PR sticky comment marker (`#391`), and the trace generator
  (`#392`).
- It declares the surface a future App **may add**: installation
  management, GitHub Checks runs, richer PR annotations, dashboards,
  webhook re-runs, and cross-repository scope coordination.

The App is a v0.2 product increment. It is not in scope for the v0.1
release lane delivered by NGW-001 through NGW-008. This document fixes
the boundary so v0.2 can ship without forcing a v0.1 schema change. It
is contractual rather than tutorial; tables and bullet lists are the
intended style.

The v0.1 reusable-workflow installation procedure is documented in
`docs/31-github-workflow-setup.md` (NGW-008, `#393`). This document
covers only the v0.2 App contract.

## 2. Claim boundary

This section reuses the project's claim boundary verbatim. It is
restated here so that no reader can infer extended claims from the
introduction of an App layer.

The claim boundary is owned by `docs/public-claim-boundary.md` (SFI-00).
It is restated here, not redefined.

> Adding a GitHub App does not change NOMOS claims. The App orchestrates
> workflows; it does not validate, certify, or assert Part 11 / GxP
> compliance. NGW is delivery infrastructure.

Concretely:

- The App MUST NOT advertise NOMOS as a regulated, validated, certified,
  or compliant system.
- The App MUST NOT add any claim that is not already permitted by
  `docs/public-claim-boundary.md`.
- The App's user-facing surfaces (check-run titles, PR comments,
  dashboard copy) MUST stay inside the same claim boundary as the
  reusable workflow.

The phrase "what the App does NOT claim" is the only place in this
document where claim language appears. Every other section is mechanical.

## 3. What the App adds

A v0.2 GitHub App MAY add the following capabilities on top of the
reusable workflow. Each item is additive: removing the App must leave
v0.1 fully functional.

- **Installation management on a GitHub organization.** Installations
  cover one or more corpus repositories and one or more output
  repositories without per-repository workflow file authoring.
- **Check runs surfaced on the source PR**, using the GitHub Checks API
  (`https://docs.github.com/en/rest/checks/runs`). Check runs sit next
  to the `Actions` results and provide a separate result lane for
  "diff plan", "publish", and "trace manifest".
- **Richer PR annotations** at the line and file level. The reusable
  workflow can only post a sticky comment (`#391`); the App can post
  per-line annotations for path-guard violations, atomization gaps, and
  body-ledger uncovered ranges.
- **A dashboard for cross-repository orchestration.** A single
  installation view of impacted scopes across multiple corpus repos
  with one row per scope and one column per active gate.
- **Webhook-driven re-runs** triggered by `pull_request` events
  (opened, synchronized, reopened, closed) and by
  `pull_request_review` once a future "publish on approval" mode is
  added (see Section 5).
- **Cross-repository scope coordination** when one source PR impacts
  scopes in multiple corpus repos. The App can reduce N parallel runs
  to one orchestrated run with a shared trace manifest.

## 4. What the App never replaces

These are the contracts the App MUST consume **without** modification.
Each entry names the artifact, the owning ticket, and the reason the
contract is invariant under an App layer.

- **The NGW config schema** (`#386`, `specs/nomos-github-workflow.cue`).
  The App reads the same `.nomos/corpus-workflows.yaml` parsed by the
  Go code in `cli/internal/githubworkflow/config.go`. There is no
  App-specific config file. App-specific knobs (installation id,
  webhook secret, check-run rendering preferences) live in the App's
  own configuration store, not in the corpus repo.
- **The trace manifest schema** (`#387`,
  `specs/nomos-trace-manifest.cue`). The App emits the same
  `nomos-trace.yaml` and `nomos-trace.json` as the reusable workflow.
  Field shapes, value enumerations, and required keys are identical.
- **The reusable workflow** (`#389`,
  `.github/workflows/nomos-corpus-workflow.yml`). The App **dispatches**
  the existing reusable workflow via `workflow_dispatch` or
  `repository_dispatch`. It does not run NOMOS in-process and does not
  ship its own engine. There is exactly one diff planner, one
  publisher, and one trace generator across v0.1 and v0.2.
- **The source-PR sticky comment marker** (`#391`). The App posts
  comments with the same per-scope sticky marker
  (`<!-- nomos-source-pr-comment:<scope-id> -->`). It updates an
  existing comment in place rather than duplicating it; this is the
  same rule as the v0.1 commenter.
- **The run-result statuses.** Status values are exactly
  `pass | fail | warn | skipped`. The App does not introduce additional
  statuses; it maps these onto the GitHub Checks API conclusions per
  Section 7.
- **The output artifact layout** (`#390`). The App writes the same
  files to the same paths under `target_path`. The path guard from
  `scripts/nomos_github_publish.py` is reused unchanged on the App
  side.
- **The anti-loop commit marker** (`#390`). Every NOMOS-generated
  commit body carries the marker
  `[nomos-generated]\nsource-sha: <sha>\nscope: <scope-id>`. The App
  uses the same marker so its outputs cannot trigger a recursive
  workflow run on a corpus that watches output paths.

## 5. Webhook event mapping

The App reacts to GitHub webhooks. Each event maps deterministically to
a reusable-workflow dispatch. The mapping below is the contract; the
App implementation MUST NOT introduce events that bypass the reusable
workflow.

| GitHub webhook event   | Action                                          | NGW invocation                                                       | Notes                                                                  |
|------------------------|-------------------------------------------------|----------------------------------------------------------------------|------------------------------------------------------------------------|
| `pull_request`         | `opened`, `synchronize`, `reopened`, `ready_for_review` | dispatch reusable workflow with `event=pull_request`                | matches the design's "PR Preview" flow; default v0.2 path              |
| `pull_request`         | `closed` with `merged=true`                     | optional dispatch for scopes whose config opted into merge-time publish | reuses the same workflow with the same `event=pull_request` payload    |
| `pull_request_review`  | `submitted`, `edited`                           | no-op for v0.2 unless a future "publish on approval" mode is added   | reserved hook; documented as future                                    |
| `push`                 | branches in scope (typically `base_ref`)        | dispatch reusable workflow with `event=push`                          | covers non-PR direct pushes to the configured base branch              |
| `repository_dispatch`  | type `nomos-corpus-update`                      | dispatch with `event=repository_dispatch`                             | mirrors the output-owned caller template described in `docs/31-*`      |
| `schedule`             | per scope cron                                  | dispatch with `event=schedule`                                        | future scoped scheduling; not in v0.2 minimum surface                  |

Two rules apply across the table:

- The App MUST NOT compute its own scoped diff. Every dispatch
  invokes `nomos github plan` inside the reusable workflow.
- The App MUST forward the source repository SHA (`pull_request.head.sha`
  or `push.after`) so the trace manifest's `corpus.head_sha` is set by
  the reusable workflow exactly as it is in v0.1.

## 6. Permission model

App permissions are at INSTALLATION scope, not at workflow scope. The
App MUST request the **least** permissions for the publication mode the
installation has configured. Installations limited to `artifact_only`
do not need write permissions on any repository.

| Permission             | Repository scope     | Required for                                                       |
|------------------------|----------------------|--------------------------------------------------------------------|
| `Contents: Read`       | corpus repository    | source-corpus checkout (read-only); always required                |
| `Contents: Write`      | output repository    | `pull_request` and `direct_push` publication modes                 |
| `Pull requests: Write` | source repository    | source PR sticky comment (`#391`); required when notify is enabled |
| `Pull requests: Write` | output repository    | open and update generated-output PRs                               |
| `Checks: Write`        | source and output    | check-run statuses surfaced on the source PR                       |
| `Metadata: Read`       | source and output    | required by GitHub for all Apps                                    |

Conditional rules:

- An installation that publishes only as `artifact_only` MUST request
  `Contents: Read` only on the corpus repository, and MUST NOT request
  any `Write` permission on either side.
- An installation in `direct_push` mode MUST surface the same explicit
  policy guard as the publisher (`#390`): the App refuses to push if
  the corpus config does not declare `publish.mode: direct_push`.
- The App MUST NOT escalate permissions implicitly when the operator
  changes `publish.mode` in the corpus config. Permission changes go
  through GitHub's standard "App permission update consent" flow.

## 7. Check-run output shape

The App maps NGW outcomes to GitHub Checks API runs. There is one
check run per logical phase. Names and conclusions are fixed so
downstream tooling (status checks, branch protection rules) can rely
on them.

```
nomos github plan      → check name "NOMOS scoped diff"
  conclusion: "neutral" if no impacted scopes; "neutral" pending else
nomos publish          → check name "NOMOS publish"
  conclusion: "success" | "failure" | "neutral" (artifact_only)
nomos trace            → check name "NOMOS trace manifest"
  conclusion: "success" iff cue vet of the manifest passes against #387
nomos comment          → no check (it is a comment, not a status)
```

Each check's `output.summary` is a one-paragraph excerpt: the diff plan
summary for the scoped-diff check, the manifest's `policy` section for
the publish check, and the manifest's `run` block for the trace check.
Each check's `output.text` MAY include the same markdown content as the
source-PR sticky comment, so a reviewer who clicks the check can read
exactly what the comment contains.

A minimal example payload for the scoped-diff check:

```json
{
  "name": "NOMOS scoped diff",
  "head_sha": "<corpus head sha>",
  "status": "completed",
  "conclusion": "neutral",
  "output": {
    "title": "Scoped diff plan",
    "summary": "0 of 3 scopes impacted by this PR.",
    "text": "| scope | impacted | gates |\n|---|---|---|\n| rbok-lawbook | no | n/a |\n"
  }
}
```

The App MUST NOT introduce check names that are not in the four-row
table above. New phases require a documented schema bump in this file
and a corresponding ticket.

## 8. Forward-compatibility invariants

These invariants are the load-bearing rules of the App contract. Every
one of them MUST hold for v0.2 to ship without breaking v0.1.

1. **Config schema is invariant.** The App MUST read
   `.nomos/corpus-workflows.yaml` parsed by the v0.1 Go code in
   `cli/internal/githubworkflow/config.go` unchanged. New App-specific
   knobs go on the App's own configuration store, not on the corpus
   config.
2. **Trace manifest is byte-identical (modulo timestamps and SHAs).**
   The App's emitted `nomos-trace.yaml` and `nomos-trace.json` MUST be
   byte-identical to what the reusable workflow emits when given the
   same inputs, ignoring fields whose values are timestamps or commit
   SHAs.
3. **No parallel engine.** The App MUST dispatch the reusable workflow
   for every analysis run. It MUST NOT call `nomos` directly from
   App-hosted infrastructure for analysis. (Auxiliary read-only
   reporting calls are out of scope here.)
4. **Sticky comment marker is invariant.** The App MUST use the
   per-scope marker `<!-- nomos-source-pr-comment:<scope-id> -->`
   defined by `#391`. Comment update vs. create logic MUST follow the
   v0.1 commenter rule (one sticky comment per scope per PR).
5. **Path guard applies on the App side.** The App MUST NOT publish
   outputs to paths outside `target_path`. The same path-guard
   semantics from `scripts/nomos_github_publish.py` (no absolute paths,
   no `..` traversal, no symlinks, no Windows drive letters) MUST be
   evaluated by the App before any push or PR write.
6. **Check-run conclusions are deterministic.** Each check's
   `conclusion` value MUST be a deterministic function of the NGW
   manifest's `policy.publish_mode` and per-gate outcomes. There is no
   App-internal scoring layer.

A change to any of these invariants is a contract change and requires a
new ticket plus an `NGW-` schema bump documented in this file.

## 9. App vs Workflow decision matrix

When does an installation use the App, and when does it use the
reusable workflow alone? The matrix below is the operator-facing
guidance.

| Use case                                                | Reusable workflow (v0.1)              | GitHub App (v0.2)                                    |
|---------------------------------------------------------|---------------------------------------|------------------------------------------------------|
| Single corpus repo → single output repo                 | OK                                    | OK; check runs are nicer than just an Actions tab    |
| Many corpuses across one organization, central view     | tedious (one workflow file per repo)  | preferred; one installation, one dashboard           |
| Cross-repo PR coordination (one PR fans into N corpuses)| not supported                         | preferred; one orchestrated run, one trace manifest  |
| No external infra ("zero-host" deployment)              | OK; runs on GitHub-hosted runners      | requires App hosting (operator owns availability)    |
| Read-only audit trace only (no PRs, no pushes)          | OK; `artifact_only` mode               | OK; same `artifact_only` mode, with check-run UX     |
| Per-line PR annotations on path-guard violations        | not supported (sticky comment only)    | preferred                                            |

Two observations:

- Every "OK" in the v0.1 column remains "OK" in the v0.2 column. The
  App never removes a v0.1 capability.
- The "preferred" entries in the v0.2 column are precisely the
  capabilities listed in Section 3. The matrix and Section 3 are
  intentionally redundant so that an operator can read either one.

## 10. What this document does NOT do

This file is a contract, not an implementation. The following are
explicit non-goals:

- It does NOT implement the GitHub App. No code, no webhook handler,
  no Checks API integration ships from this ticket.
- It does NOT host the App. Hosting platform, runtime, secrets
  management, and uptime targets are downstream concerns.
- It does NOT define the App's hosting platform or database. Those
  decisions belong to the v0.2 implementation tickets.
- It does NOT define billing, installation pricing, or any commercial
  surface.
- It does NOT replace `docs/31-github-workflow-setup.md` as the
  supported integration path for v0.1. The reusable workflow is
  production for v0.1; the App is a v0.2 follow-on.
- It does NOT modify the claim boundary. The App layer adds zero
  claims. See Section 2.

## 11. Cross-references

- v0.1 setup guide: [`docs/31-github-workflow-setup.md`](31-github-workflow-setup.md) (NGW-008, `#393`)
- Source-to-feed integrity engine: [`docs/21-source-feed-integrity-engine.md`](21-source-feed-integrity-engine.md)
- Public claim boundary: [`docs/public-claim-boundary.md`](public-claim-boundary.md) (SFI-00)
- NGW issue list: [`docs/30-github-workflow-integration-issue-list.md`](30-github-workflow-integration-issue-list.md)
- NGW design spec: [`docs/superpowers/specs/2026-05-04-nomos-github-workflow-integration-design.md`](superpowers/specs/2026-05-04-nomos-github-workflow-integration-design.md)
- NGW implementation plan: [`docs/superpowers/plans/2026-05-04-nomos-github-workflow-integration.md`](superpowers/plans/2026-05-04-nomos-github-workflow-integration.md)
- NGW config schema (CUE): [`specs/nomos-github-workflow.cue`](../specs/nomos-github-workflow.cue) (`#386`)
- NGW trace manifest schema (CUE): [`specs/nomos-trace-manifest.cue`](../specs/nomos-trace-manifest.cue) (`#387`)
- Reusable workflow: [`.github/workflows/nomos-corpus-workflow.yml`](../.github/workflows/nomos-corpus-workflow.yml) (`#389`-`#392`)
- Output publisher and path guard: [`scripts/nomos_github_publish.py`](../scripts/nomos_github_publish.py) (`#390`)
- Source PR commenter: [`scripts/nomos_github_comment.py`](../scripts/nomos_github_comment.py) (`#391`)
- GitHub App docs (external):
  - <https://docs.github.com/en/apps/creating-github-apps>
  - <https://docs.github.com/en/apps/maintaining-github-apps>
  - <https://docs.github.com/en/rest/checks/runs>
  - <https://docs.github.com/en/webhooks>

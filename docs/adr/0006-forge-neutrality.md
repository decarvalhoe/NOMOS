# ADR-0006 — Forge neutrality: one repository of reference, every other copy is a fork, no provider is privileged in the tooling

**Status:** accepted (2026-09-11), §1 amended the same day (repository of reference) · **Plan:** [docs/52](../52-plan-de-reprise-2026-09.md) · **Related:** ADR-VRC-0004 (independent roadmaps), docs/43 §2.8 ("what stays silent lies")

## Context

On 2026-09-11 the organisation's direction was restated for every RBOK
repository: development has moved to the sovereign Forgejo forge
(`RBOKproject/*`), tooling must also work with GitLab, and GitHub must not be
privileged. NOMOS was measured against that direction the same day:

- the Go engine (`cli/`) has **no** forge dependency — the only GitHub-specific
  code is the `github` subcommand, which plans scoped diffs for a workflow and
  never calls an API;
- the Python sidecars are the coupling: 10 files shell out to the `gh` CLI
  (`nomos_github_comment.py`, `nomos_github_publish.py`,
  `repeated_ci_evidence.py`, `regulated_branch_protection.py`,
  `regulated_release_env.py`, `regulated_github_qms_audit.py`,
  `roadmap_lane_guard.py --verify-github`, `push_and_pr.sh`, two tests) and one
  workflow (`bundle-release.yml`) drives releases with `gh release`;
- 6 of the 13 workflows under `.github/workflows/` depend on `GITHUB_TOKEN` or
  `actions/*` marketplace steps; the sovereign forge's runners have no
  marketplace access (`uses:` is forbidden there);
- the machine-readable roadmap (`docs/roadmap-lanes.yaml`) identifies items by
  GitHub issue number, and the portfolio/readiness tooling reads it;
- the forge mirror `RBOKproject/nomos` had stopped on 2026-08-12 at commit
  `12906aea` plus an agentic-convention kit (`.forgejo/`, `AGENTS.md`), while
  GitHub `main` had moved three months ahead. Both lines share `12906aea`.

## Decision

1. **`github.com/decarvalhoe/NOMOS` is the repository of reference; every
   other copy is a fork.** Decision of the repository owner on 2026-09-11,
   amending the morning's wording (which had made the sovereign forge the
   reference). The GitHub repository was transferred from the `RBOKproject`
   organisation to the owner's account; `github.com/RBOKproject/NOMOS`
   redirects there, so the Go module path and the existing references keep
   resolving. The sovereign forge copy `RBOKproject/nomos` is a downstream
   fork: its `main` follows GitHub `main` by fast-forward only (workflow
   `.forgejo/workflows/synchro-amont.yml`, scheduled and on demand); pull
   requests that change the product open on GitHub; the forge hosts the pull
   requests that only concern the forge itself (`.forgejo/**`) and replays
   the gates on every push and pull request (`portes.sh`). Neutrality is a
   property of the **tooling** (§2–§5), not a statement about where the
   reference lives.
2. **One provider boundary for the sidecars.** Every script that talks to a
   forge API goes through `scripts/forge_provider.py`, configured by
   `NOMOS_FORGE_PROVIDER` (`github` | `forgejo` | `gitlab` | `fake`),
   `NOMOS_FORGE_URL` and `NOMOS_FORGE_TOKEN_FILE`. Tests run with the `fake`
   provider, with no forge binary and no network. New sidecar code never calls
   `gh` directly. The naming mirrors ORDO's adapter
   (`ORDO_PROVIDER_ADAPTER`) so the two projects can share operators and
   documentation.
3. **A missing credential is an error, never a silent no-op.** Per docs/43
   §2.8, a provider that cannot authenticate raises a named error that says
   which variable is missing; it does not degrade to "nothing posted".
4. **Workflows are ported, not duplicated.** The gates that must be opposable
   on the forge (Go tests, Python tests, wiring matrix, claim boundary,
   support model, evidence ledger) are rewritten under `.forgejo/workflows/`
   without marketplace `uses:` steps, following the pattern of the existing
   `controle-*.yml` kit (manual checkout over git). Release publication stays
   on the provider that hosts the release until the forge's release API is
   wired through the same boundary.
5. **Tracker identifiers become provider-qualified.** `docs/roadmap-lanes.yaml`
   items gain a `tracker` field (`github` today); the lane guard's
   `--verify-github` becomes `--verify-tracker` and reads through the provider.
   Until that slice lands, roadmap items are still created on GitHub so the
   existing guard keeps its meaning.

## Consequences

- Positive: NOMOS can be developed and gated from either copy, and released
  from the reference; GitLab consumers of the strict gate (`ci/gitlab/`) get the same
  adapter the maintainers use; the engine's "three direct dependencies"
  argument is untouched because the boundary lives in the sidecars.
- Negative: two copies to keep in step (the sync workflow refuses anything but
  a fast-forward, so a forge-only commit on `main` stops the sync loudly); each slice must keep the wiring matrix, the support
  model and the claim boundary green, which slows the migration.
- Claim boundary: this decision changes where code is hosted and how tools
  authenticate. It creates no regulated claim, no release, no SLA.

## Slices (tracked in docs/52 and the roadmap registry)

| Slice | Content | State on 2026-09-11 |
|---|---|---|
| 0 | Forge copy reconciled with GitHub `main` by merge, then declared a downstream fork with an automatic fast-forward sync | done (forge PR #2, GitHub PR #742, sync workflow 2026-09-11) |
| 1 | `forge_provider.py` + migration of the sticky PR comment and the CI evidence collector | done (GitHub PR #740, #737 closed) |
| 2 | Migration of the regulated scripts, the publisher, the lane guard and the issue/label tooling; `push_and_pr.sh` removed | delivered (#736) |
| 3 | Opposable gates under `.forgejo/workflows/portes.yml` without marketplace steps | forge PR #3 (#738) |
| 4 | Provider-qualified tracker identifiers in the roadmap registry (`schema_version` 1.1.0, `default_tracker`, `tracker` per item, `--verify-tracker` one provider per tracker) | delivered (#739) |

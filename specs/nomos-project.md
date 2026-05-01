# `nomos.project.yaml`

`nomos.project.yaml` is the admission manifest Nomos reads before it can run
repository detection, adapter selection, canonical checks, or release gates. The
machine-readable contract is `specs/nomos-project.cue` with the `#Project`
definition.

## Required Fields

| Field | Why it is required |
|---|---|
| `schema_version` | Pins the manifest contract used by the CLI and adapters. |
| `project.id` | Stable slug used in reports, attestations, and portfolio views. |
| `project.name` | Human-readable project name for reports and review queues. |
| `project.domain` | Declares the business domain before any source mapping. |
| `project.lifecycle` | Selects greenfield or brownfield admission behavior. |
| `project.risk_level` | Sets the baseline strictness for evidence and gates. |
| `project.owners` | Identifies accountable humans for product and domain decisions. |
| `scope.in_scope` | Prevents an admission verdict from applying to undefined product scope. |
| `surfaces` | Gives adapters concrete API, UI, data, worker, infra, docs, event, CLI, or batch surfaces to inspect. |
| `toolchain.build` | Provides the canonical build command Nomos can ask CI or humans to run. |
| `toolchain.test` | Provides the canonical test command for admission and release gates. |

## Optional Fields

| Field | Why it is optional |
|---|---|
| `project.description` | Helpful for reviewers, but not needed for machine admission. |
| `project.repository` | Useful for hosted projects; not required for local or air-gapped repositories. |
| `scope.verdict` | May be absent before `nomos diagnose` or `nomos admit` has run. |
| `scope.confidence` | Meaningful only when a preliminary verdict exists. |
| `scope.out_of_scope` | Required only when explicit exclusions matter. |
| `scope.assumptions` | Used when admission depends on unresolved context. |
| `scope.bounded_contexts` | Useful for larger systems, unnecessary for a small project. |
| `scope.blockers` | Present only when admission is partial, blocked, or risk-gated. |
| `surfaces[].path` | Some surfaces are external or declared before repository layout is final. |
| `surfaces[].stack` | Adapter detection can infer stack later if it is not declared. |
| `surfaces[].critical` | Defaults to `false`; required only for stricter gate targeting. |
| `surfaces[].entrypoints` | Useful for APIs, UIs, jobs, and CLIs; not all surfaces expose stable entrypoints. |
| `toolchain.lint` | Some projects fold lint into build or test. |
| `toolchain.typecheck` | Some stacks have no separate typecheck command. |
| `toolchain.package_managers` | Useful for adapter selection, but detectable in many repositories. |
| `toolchain.ci_systems` | Useful for CI integration, but not required for local admission. |
| `compliance` | Needed only when regulatory or data-sensitivity rules affect gates. |
| `evidence` | Needed only when a project requires explicit report or attestation outputs. |
| `notes` | Human context that should not affect machine validation. |

## Lifecycle Modes

`greenfield` means the project can align its structure to Nomos from the start.
The manifest should normally have a confident `in_scope` verdict and few
blockers.

`brownfield` means Nomos is being introduced to an existing system. The manifest
may use a `partial` or `blocked` verdict, list assumptions, and document blockers
that prevent strict gates.

## Examples

The examples in `specs/examples/` cover the three baseline admission profiles:

- `nomos-project.minimal.yaml`: minimal greenfield product.
- `nomos-project.brownfield.yaml`: legacy brownfield product with blockers and partial scope.
- `nomos-project.regulated.yaml`: regulated high-evidence product with signed attestation needs.

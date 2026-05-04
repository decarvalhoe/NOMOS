# 15 - Nomos Product Backlog

Date: 2026-05-04
Current release target: post-`v0.1.0-ALPHA`

## Backlog Rule

This file reflects the active product backlog state after the alpha release. Historical issue waves are not repeated here as open work once they have been merged.

The implementation issue list for the next releases lives in
[`docs/29-post-alpha-release-issue-list.md`](29-post-alpha-release-issue-list.md).
Create the GitHub child issues from that document before coding a
release wave.

The GitHub workflow integration issue list lives in
[`docs/30-github-workflow-integration-issue-list.md`](30-github-workflow-integration-issue-list.md).
It covers source-PR triggered NOMOS runs, output-owned workflows,
risk-based publication, mandatory trace manifests, optional source PR
comments, and GitHub App readiness.

Each active backlog item must have:

- an owner or GitHub issue;
- a dependency relationship;
- an evidence artifact or testable exit gate;
- a clear claim impact.

## Current GitHub Open Items

As checked on 2026-05-04, open Nomos issues are:

| Issue | Area | Role in dependency tree | Release impact |
|---|---|---|---|
| `#314` | AQ / RBOK POC | Umbrella epic for elevating RBOK POC proof level. | Does not block `v0.1.0-ALPHA` if alpha limitations are explicit; blocks stronger RBOK validation claim. |
| `#320` | Nomos/Praxis | Activate Nomos-to-Praxis atom mapping after Nomos verification. | Deferred; blocks joint Nomos/Praxis regulated claim. |
| `#192` | Reference bibles | Acquire and intake ISO 13485:2016. | Blocks complete licensed-reference baseline. |
| `#193` | Reference bibles | Acquire and intake ISO/IEC/IEEE 12207:2026. | Blocks lifecycle-standard clause closure. |
| `#194` | Reference bibles | Complete license review for GAMP 5 and ISO/IEC 25010. | Blocks licensed-standard processing and redistribution decisions. |
| `#196` | Reference bibles | Process public and licensed bibles with Nomos. | Depends on licensed/public reference readiness; blocks higher assurance reference-to-control proof. |
| `#382` | FSQ future | Short critical atom reconciliation. | Blocks stronger fidelity claims where short strings carry standalone regulatory, operational, legal, or game-rule meaning. |

## Delivered Alpha Capabilities

The following capabilities are treated as delivered for `v0.1.0-ALPHA`, subject to CI validation and release tagging:

- CLI version set to `0.1.0-ALPHA`.
- Project diagnosis and admission commands.
- Corpus scan, manifest, diff, sidecar validation, feed, and attestation commands.
- RBOK lawbook profile.
- Source spans on generated feed nodes.
- Typed semantic nodes for tables, links, callouts, code blocks, and images.
- Certified table of contents.
- Governed lexicon artifact.
- RAG metadata output.
- Runtime import projection.
- Strict fidelity gate and release gate wiring.
- Regulated-by-design documentation structure.
- Public claim boundary and release notes.

## Alpha Dependency Tree

```text
CLI and corpus commands
  -> RBOK lawbook profile
  -> feed artifacts
  -> source spans + typed blocks + certified TOC + lexicon
  -> strict fidelity gate
  -> RBOK POC dossier
  -> v0.1.0-ALPHA release claim

Public claim boundary
  -> README
  -> release notes
  -> regulated docs
  -> GitHub pre-release

Licensed reference acquisition (#192, #193, #194)
  -> public/licensed bible processing (#196)
  -> reference-to-control closure
  -> NQ-5 validation-pack readiness

Nomos verified artifacts
  -> Praxis atom mapping (#320)
  -> joint evidence contract
  -> NQ-4 candidate
```

## Next Implementation Epics

### EPIC A - Portable Structure Fidelity

Goal: remove RBOK-specific confidence bias by proving the atomization model across multiple document families.

Work:

- AST-to-Nomos comparison for Markdown fixtures with H1-H6, tables, lists, callouts, code, links, images, annexes, xrefs, and front matter.
- Exact line/column/byte spans for every active source block.
- Golden fixtures for legal text, regulatory text, technical standard, business corpus, game rules, YAML, and JSON.
- Explicit unsupported-block records where support is not implemented.
- Short critical atom reconciliation: prove that short but meaningful fragments such as `GxP`, `ALCOA+`, `21 CFR`, `SOP-01`, `P0`, `Yes/No`, status values, thresholds, and identifiers are either represented with their parent context, promoted into governed lexicon/value/identifier atoms, or explicitly classified as non-semantic. They must not become orphan RAG chunks, but they also must not disappear without a disposition.

Exit gate:

```text
No active source block is silently dropped, and every generated node has a source span or explicit unsupported status.
```

#### Future item - Short Critical Atom Reconciliation (#382)

Owner / issue: `#382`.

Problem statement:

`v0.1.0-ALPHA` records `0` feed units <= 10 characters. That is a useful noise-control result, because punctuation-only and separator-only fragments should not enter the curated feed or RAG. It is not, by itself, a proof that every short critical term was semantically represented. Regulated and operational corpora often carry high-value meaning in short strings: acronyms, clause references, severity labels, status values, yes/no answers, IDs, thresholds, units, and option codes.

Dependency relationship:

```text
Source segment ledger
  -> short-fragment inventory
  -> criticality classifier
  -> disposition report
  -> lexicon/value/identifier promotion where required
  -> semantic quality gate
  -> RAG/context proof
```

Required work:

- Emit a `short-critical-atoms.json` report for every processed corpus, covering excluded short fragments with source id, source span, parent chain, table/YAML/JSON path where applicable, surrounding context, and current disposition.
- Classify each short fragment as `non_semantic`, `contextualized_in_parent`, `lexicon_atom`, `identifier_atom`, `normative_value_atom`, or `requires_review`.
- Promote critical short strings into governed lexicon/value/identifier artifacts when they carry standalone meaning, without creating orphan <=10-character RAG chunks.
- Add fixtures for Markdown paragraphs, lists, tables, callouts, YAML, JSON, legal/regulatory clauses, technical standards, business corpora, and game rules.
- Make the semantic quality gate fail closed when a short critical fragment remains `requires_review` or has no disposition.

Definition of done:

- Body ledger still reports `0` uncovered bytes for admitted text sources.
- Curated feed/RAG still reports `0` junk or orphan <=10-character units.
- Every critical short fragment has a machine-readable disposition and a human-reviewable source span.
- RAG chunks containing critical short terms include enough parent context to be useful and citable.
- Regression fixtures cover at least `GxP`, `ALCOA+`, `21 CFR`, `SOP-01`, `P0`, `Yes`, `No`, threshold values, status labels, table cells, and structured YAML/JSON scalars.
- The strict gate exposes unresolved short-critical findings as blocking evidence.

### EPIC B - Reference Bible Governance

Goal: turn external references into controlled source authorities.

Work:

- Close `#192`, `#193`, `#194`, and `#196`.
- Maintain licensed sidecars without redistributing restricted content.
- Create public surrogate annexes only where license permits.
- Map references to controls, tests, evidence, waivers, and public claims.

Exit gate:

```text
Every cited regulation, standard, or framework is mapped or explicitly marked not applicable / blocked.
```

### EPIC C - Regulated Release Evidence

Goal: make release decisions reconstructible by a quality reviewer.

Work:

- Generate release evidence bundles per tag.
- Retain CI run URLs, hashes, reports, source/corpus attestation, deviations, waivers, and approvals.
- Activate GitHub-native QMS evidence exports.
- Add named owner and training evidence.

Exit gate:

```text
An independent reviewer can reconstruct why a release was allowed without private tribal knowledge.
```

### EPIC D - RAG And Conversational Governance

Goal: make downstream LLM/RAG use precise, concise, cited, and bounded.

Work:

- Retrieval metadata evaluation.
- Citation and refusal tests.
- Prompt-injection and excessive-agency tests.
- Behavior contract for single-question conversational steps.
- Infomaniak model catalog integration where the downstream product requires Swiss-only AI infrastructure.

Exit gate:

```text
RAG output is source-backed, concise by contract, and never replaces canonical authority.
```

### EPIC E - Nomos/Praxis Compatibility

Goal: connect Nomos canonical evidence to Praxis runtime assurance without overclaiming either product.

Work:

- Close `#320` after Nomos artifacts are verified.
- Publish atom mapping and evidence ledger contract.
- Let Praxis consume Nomos artifacts as downstream evidence.
- Feed Praxis runtime evidence and CAPA status back into Nomos release decisions.

Exit gate:

```text
Joint Nomos/Praxis claims are backed by a shared contract and both products declare their own quality level.
```

## SFI Wave Status

- SFI-11 (#349) shipped: dossier + command sequence. The alpha release records a bounded RBOK `01_rbok` evidence pack; this does not promote Nomos to universal-fidelity or regulated-validation status.

## FSQ Wave Status (epic #363)

- FSQ-01 (#364) shipped: feed audit (`cli/internal/corpus/cmd/feed-audit/`).
- FSQ-02 (#365) shipped: explicit source admission and non-atomization policy.
- FSQ-03 (#366) shipped: table-row units replace bare table-cell leaks.
- FSQ-04 (#367) shipped: YAML raw/decoded key-path scalar provenance.
- FSQ-05 (#368) shipped: corpus body ledger separate from curated feed.
- FSQ-06 (#369) shipped: semantic quality gate (`CheckSemanticQuality`).
- FSQ-07 (#370) shipped: context-rich RAG chunk composer (`ComposeRAGChunks`).
- FSQ-08 (#371 / #379 / #380) shipped: `scripts/rbok-poc-integrity.sh` was extended through the integrity stages, `docs/rbok-poc-validation-dossier.md` records the AQ-3 bounded POC dossier, and the alpha release notes record the passing evidence pack. Remaining future work is not the FSQ epic itself; it is the stronger portable fidelity backlog above, including short critical atom reconciliation, broader adapter fixtures, repeated CI evidence, and attestation `claim_coverage` wiring.

## NGW Wave Status (GitHub workflow integration)

- NGW-01 (#386) shipped: workflow config schema (`specs/nomos-github-workflow.cue`).
- NGW-02 (#387) shipped: trace manifest schema (`specs/nomos-trace-manifest.cue`).
- NGW-03 (#388) shipped: scoped diff planner + `nomos github plan` command.
- NGW-04 (#389) shipped: reusable GitHub Actions workflow (`.github/workflows/nomos-corpus-workflow.yml`) plus two caller templates (`templates/github-workflows/nomos-source-pr.yml`, `nomos-output-dispatch.yml`). Read-only corpus checkout (`persist-credentials: false` AND push remote DISABLED). NGW-04 reads + plans + uploads only; publication is NGW-005 / #390 territory.
- NGW-08 (#393) shipped: source-owned and output-owned setup docs (`docs/31-github-workflow-setup.md`) — config-owner choice, secrets matrix, permissions, branch-protection expectations, publication-mode tradeoffs, step-by-step install, verification checklist, troubleshooting. Forward-references `docs/32-github-app-readiness-boundary.md` (NGW-09 / #394, parallel).

## Non-Goals For The Alpha

The alpha backlog does not include:

- formal FDA, EU GMP, ISO, or NASA certification;
- customer-specific GxP validation;
- universal PDF/DOCX/OCR fidelity;
- production vector-store tuning;
- open-source licensing;
- e-signature approval semantics as a validated Part 11 platform.

Those remain future scoped work and must not appear as delivered claims.

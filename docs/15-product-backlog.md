# 15 - Nomos Product Backlog

Date: 2026-05-03
Current release target: `v0.1.0-ALPHA`

## Backlog Rule

This file reflects the active product backlog state for the alpha release. Historical issue waves are not repeated here as open work once they have been merged.

Each active backlog item must have:

- an owner or GitHub issue;
- a dependency relationship;
- an evidence artifact or testable exit gate;
- a clear claim impact.

## Current GitHub Open Items

As checked during release preparation on 2026-05-03, open Nomos issues are:

| Issue | Area | Role in dependency tree | Release impact |
|---|---|---|---|
| `#314` | AQ / RBOK POC | Umbrella epic for elevating RBOK POC proof level. | Does not block `v0.1.0-ALPHA` if alpha limitations are explicit; blocks stronger RBOK validation claim. |
| `#320` | Nomos/Praxis | Activate Nomos-to-Praxis atom mapping after Nomos verification. | Deferred; blocks joint Nomos/Praxis regulated claim. |
| `#192` | Reference bibles | Acquire and intake ISO 13485:2016. | Blocks complete licensed-reference baseline. |
| `#193` | Reference bibles | Acquire and intake ISO/IEC/IEEE 12207:2026. | Blocks lifecycle-standard clause closure. |
| `#194` | Reference bibles | Complete license review for GAMP 5 and ISO/IEC 25010. | Blocks licensed-standard processing and redistribution decisions. |
| `#196` | Reference bibles | Process public and licensed bibles with Nomos. | Depends on licensed/public reference readiness; blocks higher assurance reference-to-control proof. |
| `#337` | SFI epic — source-to-feed integrity | Umbrella epic for source-to-feed fidelity and semantic feed hygiene (children `#338`–`#349`). | In progress; claim boundary pending all SFI children. Blocks any platform-wide `full_fidelity_proven` claim until the corpus integrity gate is wired and passing. |

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

Exit gate:

```text
No active source block is silently dropped, and every generated node has a source span or explicit unsupported status.
```

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

## Non-Goals For The Alpha

The alpha backlog does not include:

- formal FDA, EU GMP, ISO, or NASA certification;
- customer-specific GxP validation;
- universal PDF/DOCX/OCR fidelity;
- production vector-store tuning;
- open-source licensing;
- e-signature approval semantics as a validated Part 11 platform.

Those remain future scoped work and must not appear as delivered claims.

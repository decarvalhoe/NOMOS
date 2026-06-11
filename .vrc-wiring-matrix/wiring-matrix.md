# VRC Wiring Matrix

> GENERATED FILE — do not edit by hand. Regenerate with: python3 scripts/vrc_wiring_matrix.py --root .
> Statuses are COMPUTED from tree anchors, never declared. The registry
> (`scripts/vrc_wiring_matrix_registry.json`) only says where to look and what status is expected;
> any divergence — in either direction — fails CI.
> Claim boundary: Wiring presence computed from tree anchors; not a proof of functional correctness; no compliance certification.

| Capability | Pillar | Expected | Computed | OK | Promotion | Notes |
|---|---|---|---|---|---|---|
| `attestation_dsse_signing` | P7 | real | real | ✅ | — | — |
| `atom_facets_engine` | P3 | real | real | ✅ | — | — |
| `knowledge_lens` | P4 | real | real | ✅ | — | — |
| `body_ledger_merkle_emission` | P7 | real | real | ✅ | — | — |
| `body_ledger_merkle_verification` | P7 | partial | partial | ✅ | VRC-07 | no production caller declared (the #540 class) |
| `claim_coverage_attestation` | P7 | partial | partial | ✅ | VRC-07 | no production caller declared (the #540 class) |
| `supply_chain_attestation` | P7 | partial | partial | ✅ | VRC-08 | no production caller declared (the #540 class) |
| `swiss_live_connector` | P6 | real | real | ✅ | — | — |
| `canonical_knowledge_bundle` | P6-gen | real | real | ✅ | — | — |
| `cite_or_abstain_gate` | P2 | sidecar | sidecar | ✅ | VRC-10 | implementation lives in sidecar scripts/specs only (doctrine: sidecar = PARTIAL, not done) |
| `canon_promotion` | P5 | sidecar | sidecar | ✅ | VRC-11 | implementation lives in sidecar scripts/specs only (doctrine: sidecar = PARTIAL, not done) |
| `point_in_time_resolver` | P6 | sidecar | sidecar | ✅ | VRC-12 | implementation lives in sidecar scripts/specs only (doctrine: sidecar = PARTIAL, not done) |
| `facet_ontology_alignment` | P3 | sidecar | sidecar | ✅ | VRC-45 | implementation lives in sidecar scripts/specs only (doctrine: sidecar = PARTIAL, not done) |
| `pdf_adapter` | ingestion | stub | stub | ✅ | VRC-30 | — |
| `docx_adapter` | ingestion | partial | partial | ✅ | VRC-41 | no production caller declared (the #540 class) |
| `sigstore_keyless` | P7 | absent | absent | ✅ | VRC-40 | — |
| `strict_fidelity_gate` | P1 | real | real | ✅ | — | — |

## Generic checks

- `command_registration`: **pass**
  - known-unwired: checks.go: SourcesCheckCommand is implemented but neither registered in the app.go command map nor called from production code (the #543 'atomize' class) — known, tracked by VRC-09: doc-comment claims `nomos sources check`; not in the command map
  - known-unwired: checks.go: ContractsCheckCommand is implemented but neither registered in the app.go command map nor called from production code (the #543 'atomize' class) — known, tracked by VRC-09: check-family command never registered
  - known-unwired: checks.go: MatrixCheckCommand is implemented but neither registered in the app.go command map nor called from production code (the #543 'atomize' class) — known, tracked by VRC-09: check-family command never registered
  - known-unwired: checks.go: StrictCommand is implemented but neither registered in the app.go command map nor called from production code (the #543 'atomize' class) — known, tracked by VRC-09: legacy `nomos strict` implementation shadowed by the registered StrictGateCommand
  - known-unwired: checks.go: ExceptionsCheckCommand is implemented but neither registered in the app.go command map nor called from production code (the #543 'atomize' class) — known, tracked by VRC-09: check-family command never registered
  - known-unwired: product_check.go: ProductCheckCommand is implemented but neither registered in the app.go command map nor called from production code (the #543 'atomize' class) — known, tracked by VRC-09: doc-comment claims `product-check`; not in the command map
  - known-unwired: report_cmds.go: ReportCommand is implemented but neither registered in the app.go command map nor called from production code (the #543 'atomize' class) — known, tracked by VRC-09: doc-comment claims `nomos report`; not in the command map
  - known-unwired: report_cmds.go: ExportSPDXCommand is implemented but neither registered in the app.go command map nor called from production code (the #543 'atomize' class) — known, tracked by VRC-09: SPDX BOM exporter exists but is unreachable — feeds VRC-23
  - known-unwired: report_cmds.go: ExportCycloneDXCommand is implemented but neither registered in the app.go command map nor called from production code (the #543 'atomize' class) — known, tracked by VRC-09: CycloneDX BOM exporter exists but is unreachable — feeds VRC-23
  - known-unwired: report_cmds.go: AttestCommand is implemented but neither registered in the app.go command map nor called from production code (the #543 'atomize' class) — known, tracked by VRC-09: exported one-shot signing path (re-routed to the real signer by #542, cited by decision 0005) shadowed by the registered attestCommand

## Summary

- capabilities: 17 (real=7, partial=4, sidecar=4, stub=1, absent=1)
- mismatches: 0
- generic check failures: 0
- known unwired commands (tracked, not hidden): 10

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
| `body_ledger_merkle_verification` | P7 | real | real | ✅ | — | — |
| `claim_coverage_attestation` | P7 | real | real | ✅ | — | — |
| `supply_chain_attestation` | P7 | real | real | ✅ | — | — |
| `swiss_live_connector` | P6 | real | real | ✅ | — | — |
| `canonical_knowledge_bundle` | P6-gen | real | real | ✅ | — | — |
| `cite_or_abstain_gate` | P2 | sidecar | sidecar | ✅ | VRC-10 | implementation lives in sidecar scripts/specs only (doctrine: sidecar = PARTIAL, not done) |
| `canon_promotion` | P5 | sidecar | sidecar | ✅ | VRC-11 | implementation lives in sidecar scripts/specs only (doctrine: sidecar = PARTIAL, not done) |
| `point_in_time_resolver` | P6 | sidecar | sidecar | ✅ | VRC-12 | implementation lives in sidecar scripts/specs only (doctrine: sidecar = PARTIAL, not done) |
| `facet_ontology_alignment` | P3 | sidecar | sidecar | ✅ | VRC-45 | implementation lives in sidecar scripts/specs only (doctrine: sidecar = PARTIAL, not done) |
| `pdf_adapter` | ingestion | real | real | ✅ | — | — |
| `domain_pack_gate` | D2 | real | real | ✅ | — | — |
| `docx_adapter` | ingestion | partial | partial | ✅ | VRC-41 | no production caller declared (the #540 class) |
| `sigstore_keyless` | P7 | absent | absent | ✅ | VRC-40 | — |
| `strict_fidelity_gate` | P1 | real | real | ✅ | — | — |
| `manifest_check_family` | P1 | real | real | ✅ | — | — |
| `report_and_bom_export` | P7 | real | real | ✅ | — | — |

## Generic checks

- `command_registration`: **pass**

## Summary

- capabilities: 20 (real=14, partial=1, sidecar=4, stub=0, absent=1)
- mismatches: 0
- generic check failures: 0
- known unwired commands (tracked, not hidden): 0

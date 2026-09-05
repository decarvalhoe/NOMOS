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
| `cite_or_abstain_gate` | P2 | real | real | ✅ | — | — |
| `canon_promotion` | P5 | real | real | ✅ | — | — |
| `point_in_time_resolver` | P6 | real | real | ✅ | — | — |
| `facet_ontology_alignment` | P3 | real | real | ✅ | — | — |
| `adapter_capability_kits` | C5 | real | real | ✅ | — | — |
| `pdf_adapter` | ingestion | real | real | ✅ | — | — |
| `domain_pack_gate` | D2 | real | real | ✅ | — | — |
| `rag_eval_harness` | B2 | real | real | ✅ | — | — |
| `rag_eval_context_metrics` | B2 | real | real | ✅ | — | — |
| `faithfulness_scorer_interface` | A1 | real | real | ✅ | — | — |
| `hhem_nli_scorer_sidecar` | A1 | sidecar | sidecar | ✅ | — | implementation lives in sidecar scripts/specs only (topology only; delivery and regulated validation are tracked separately) |
| `rag_evidence_sidecar_consumes_go_verdict` | A1 | real | real | ✅ | — | — |
| `rag_interop_export` | interop | real | real | ✅ | — | — |
| `rag_index_staleness_verify` | interop | real | real | ✅ | — | — |
| `rag_lens_scoped_export` | interop | real | real | ✅ | — | — |
| `reference_retrieval_kit` | B1 | sidecar | sidecar | ✅ | — | implementation lives in sidecar scripts/specs only (topology only; delivery and regulated validation are tracked separately) |
| `pack_core_coupling_guard` | D6 | sidecar | sidecar | ✅ | — | implementation lives in sidecar scripts/specs only (topology only; delivery and regulated validation are tracked separately) |
| `consumer_conformance_kit` | E-1 | sidecar | sidecar | ✅ | — | implementation lives in sidecar scripts/specs only (topology only; delivery and regulated validation are tracked separately) |
| `evidence_pack_bom` | A5 | real | real | ✅ | — | — |
| `docx_adapter` | ingestion | real | real | ✅ | — | — |
| `sigstore_keyless` | P7 | absent | absent | ✅ | #645 | — |
| `strict_fidelity_gate` | P1 | real | real | ✅ | — | — |
| `manifest_check_family` | P1 | real | real | ✅ | — | — |
| `report_and_bom_export` | P7 | real | real | ✅ | — | — |
| `eu_ai_act_pack` | D3 | real | real | ✅ | — | — |
| `rule_execution_substrate` | B3 | real | real | ✅ | — | — |
| `cross_reference_graph` | B5 | real | real | ✅ | — | — |
| `vocabulary_skos_shacl` | B4 | sidecar | sidecar | ✅ | — | implementation lives in sidecar scripts/specs only (topology only; delivery and regulated validation are tracked separately) |
| `public_cite_or_abstain_bench` | A-exit | real | real | ✅ | — | — |
| `repeated_ci_evidence_private_corpus` | P1 | sidecar | sidecar | ✅ | — | implementation lives in sidecar scripts/specs only (topology only; delivery and regulated validation are tracked separately) |
| `training_competence_status` | P1 | sidecar | sidecar | ✅ | — | implementation lives in sidecar scripts/specs only (topology only; delivery and regulated validation are tracked separately) |
| `web_source_contract` | P2 | real | real | ✅ | — | — |
| `licensed_reference_review_gate` | P1 | sidecar | sidecar | ✅ | — | implementation lives in sidecar scripts/specs only (topology only; delivery and regulated validation are tracked separately) |
| `external_snapshot_input` | P2 | real | real | ✅ | — | — |
| `public_reference_processing` | P1 | sidecar | sidecar | ✅ | — | implementation lives in sidecar scripts/specs only (topology only; delivery and regulated validation are tracked separately) |
| `recursio_offline_e2e` | P1 | real | real | ✅ | — | — |
| `release_candidate_bundle` | P1 | real | real | ✅ | — | — |
| `sigstore_offline_verification` | P1 | real | real | ✅ | — | — |
| `skos_static_distribution` | B4 | sidecar | sidecar | ✅ | — | implementation lives in sidecar scripts/specs only (topology only; delivery and regulated validation are tracked separately) |

## Generic checks

- `command_registration`: **pass**

## Summary

- capabilities: 48 (real=37, partial=0, sidecar=10, stub=0, absent=1)
- mismatches: 0
- generic check failures: 0
- known unwired commands (tracked, not hidden): 0

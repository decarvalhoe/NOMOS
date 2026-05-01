# Nomos Report: Insurance Example

- Schema: `0.1.0`
- Report type: `nomos-report`
- Generated at: `2026-04-30T00:00:00Z`
- Run: `run-20260430-000000` (`validate`)

## Verdict

| Field | Value |
| --- | --- |
| Status | warn |
| Severity | medium |
| Blocking | false |
| Summary | Nomos can validate the repository, but one product surface needs evidence. |
| Next action | Attach canonical matrix evidence for UI catalogue rendering. |
| Next action | Keep sample data out of product surfaces. |

## Summary

| Metric | Value |
| --- | ---: |
| Checks | 2 |
| Findings | 1 |
| Blocking findings | 0 |
| Evidence | 2 |
| Coverage ratio | 0.80 |
| Units covered | 8/10 |
| Units partial | 1 |
| Units missing | 1 |
| Units not applicable | 0 |

## Checks

| ID | Status | Severity | Category | Name |
| --- | --- | --- | --- | --- |
| `product.ui` | `warning` | `medium` | `product` | Product UI traceability |
| `sources.manifest` | `passed` | `info` | `sources` | Source manifest |

## Findings

### FINDING-002

- Code: `NOMOS_PRODUCT_SAMPLE_LEAK`
- Severity: `medium`
- Status: `open`
- Blocking: `false`
- Target: `ui:web/app/page.tsx`
- Message: UI catalogue still renders sample data.
- Remediation: Replace sample catalogue with read-model data backed by canonical sources.
- Evidence: `EVIDENCE-002`

## Evidence

| ID | Type | Description | Target |
| --- | --- | --- | --- |
| `EVIDENCE-001` | `source_manifest` | Source manifest exists. | `` |
| `EVIDENCE-002` | `code_reference` | UI renders hardcoded sample catalogue. | `ui:web/app/page.tsx` |

# Nomos Self-Compliance Report

Generated: 2026-05-03
Release context: `v0.1.0-ALPHA`
Scope: Nomos repository, release documentation, CLI gates, regulated-readiness baseline

## Executive Verdict

Nomos is self-consistent enough for an alpha release and a regulated-readiness commercial discussion.

Nomos is not yet a validated regulated system. The repository contains the structure, gates, templates, and initial evidence needed to continue toward that posture, but customer validation, approved QMS records, full reference-to-control closure, and live GitHub/QMS evidence remain required.

## Verified For Alpha Release

| Area | Status |
|---|---|
| CLI version set to alpha release | PASS |
| Public README claim boundary | PASS |
| CHANGELOG present | PASS |
| Release readiness document present | PASS |
| Public claim boundary document present | PASS |
| RBOK POC dossier current | PASS |
| Regulated docs baseline present | PASS |
| Strict fidelity gate wired into release gate | PASS |
| RBOK lawbook POC strict gate | PASS |
| Local Go tests | PASS |
| Local Go coverage report | PASS, 87.2% statement coverage |
| Local E2E script | PASS |
| Python automation tests | PASS |

## Product Evidence Summary

- `nomos version` reports `0.1.0-ALPHA`.
- RBOK lawbook POC generated 7191 nodes with spans.
- Certified TOC contains 1090 entries.
- Strict fidelity gate passed with 0 findings for the current POC output.
- Source mutation verification passed on the read-only corpus clone.
- GitHub CI for the strict fidelity gate merge was green.

## Wiring Matrix (Generated)

Capability wiring status — engine code, production caller, adversarial test, CI gate — is computed, never declared, by `scripts/vrc_wiring_matrix.py` against `scripts/vrc_wiring_matrix_registry.json`, and published at [.vrc-wiring-matrix/wiring-matrix.md](../.vrc-wiring-matrix/wiring-matrix.md). CI fails when computed statuses and the registry diverge in either direction, and when any `*Command` function is implemented but neither registered nor called (the #543 class). Hand-editing the generated matrix is forbidden. Known-unwired commands are tracked in the registry allowlist with their promotion issue (VRC-09), not hidden.

## Regulated-Readiness Evidence Summary

Installed evidence structure:

- quality-system baseline;
- lifecycle and validation documents;
- data-integrity and electronic-record policy baseline;
- security and privacy SOP baseline;
- GitHub operating model;
- evidence index and control matrix structure;
- validation pack and supplier pack templates;
- AI/RAG governance baseline;
- atomization certification baseline;
- release bundle structure.

This supports a regulated-readiness claim, not a regulated certification claim.

## Remaining Regulated Gaps

| Gap | Status |
|---|---|
| Named QMS owners and active CODEOWNERS | Owners assigned with COI note + independent-review waiver (record `NOMOS-REC-ROLE-2026-001`, 2026-06-11); CODEOWNERS present |
| First executed QMS cycle (management review, internal audit, CAPA log) | Recorded 2026-06-11: `NOMOS-REC-MR-2026-001`, `NOMOS-REC-AUD-2026-001` (self-audit, not independent), CAPA-2026-001/002/003 closed with effectiveness verified on main |
| Approved training records | Open |
| Live GitHub branch/ruleset/environment/security evidence exports | Open |
| Licensed reference license review and clause mapping | Open |
| Full reference-to-control-to-evidence matrix closure | Open |
| Customer intended-use validation pack | Open |
| Independent reconstruction review | Open |
| Production support and security operating model | Open |

## Claim Boundary

Allowed:

```text
Nomos v0.1.0-ALPHA provides a working canonical-first CLI, RBOK lawbook POC evidence, strict fidelity gates, and a regulated-readiness documentation baseline.
```

Blocked:

```text
Nomos is validated for regulated production use.
Nomos is Part 11 compliant.
Nomos is GxP validated.
Nomos is ISO certified.
Nomos can convert any source into legally defensible software law without customer validation.
```

## Next Actions

1. Assign quality, technical, and security owners.
2. Activate CODEOWNERS and protected release environment evidence.
3. Complete licensed reference intake and license review.
4. Close the reference-to-control matrix with real evidence artifacts.
5. Generate release evidence bundle for every alpha and beta release.
6. Run independent reconstruction review before any higher assurance claim.

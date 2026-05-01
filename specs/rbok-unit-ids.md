# RBOK Unit ID Strategy

## Pattern

```
RBOK-{TYPE}-{slug}-{version}
```

- **RBOK** — fixed namespace prefix, identifies the domain.
- **TYPE** — 3-letter uppercase code for the concept category.
- **slug** — lowercase kebab-case human-readable name.
- **version** — `v` followed by an integer (e.g. `v1`, `v2`).

## Type Codes

| Code | Category | Description | Example |
|------|----------|-------------|---------|
| CNP | Concept | Core RBOK domain concept | `RBOK-CNP-policy-lifecycle-v1` |
| PRC | Principle | Architectural or design principle | `RBOK-PRC-fail-closed-v1` |
| DIM | Dimension | CLAIRE dimension | `RBOK-DIM-contexte-v1` |
| PAR | Parcours | User journey or business process | `RBOK-PAR-souscription-v1` |
| BOJ | Business Object | Canonical entity | `RBOK-BOJ-contrat-assurance-v1` |
| SVC | Service | Business service or capability | `RBOK-SVC-calcul-prime-v1` |
| CLM | Claim | Marketing or product claim | `RBOK-CLM-remboursement-48h-v1` |
| RUL | Rule | Business rule with testable predicate | `RBOK-RUL-age-minimum-v1` |
| EXC | Exception | Documented exception to a rule | `RBOK-EXC-derogation-plafond-v1` |
| FRM | Formula | Calculation or derivation | `RBOK-FRM-prime-annuelle-v1` |
| TRM | Term | Glossary term with canonical definition | `RBOK-TRM-franchise-v1` |
| WFL | Workflow | Multi-step orchestration or approval flow | `RBOK-WFL-validation-sinistre-v1` |
| SCN | Scenario | Test scenario or acceptance case | `RBOK-SCN-sinistre-auto-v1` |
| LGB | Legacy | Legacy behavior preserved for compatibility | `RBOK-LGB-tarif-ancien-v1` |
| AMB | Ambiguity | Open ambiguity requiring human decision | `RBOK-AMB-double-couverture-v1` |
| DEC | Decision | Resolved decision with ADR trace | `RBOK-DEC-migration-api-v2-v1` |

## Rules

1. **Immutability** — once assigned, an ID never changes meaning. If the concept evolves, bump the version.
2. **Uniqueness** — each `RBOK-{TYPE}-{slug}-{version}` is globally unique within the domain.
3. **Slug convention** — lowercase, kebab-case, no underscores, max 64 characters.
4. **Version semantics** — `v1` is the initial version. Increment only when the canonical definition materially changes. Minor clarifications do not require a version bump.
5. **Deprecation** — a deprecated unit keeps its ID. Add status `deprecated` in the canonical matrix and a `decision_refs` entry pointing to the ADR that explains the deprecation.
6. **Cross-reference** — when referencing an RBOK unit from source manifests, contracts, or code, use the full ID string.

## Integration with Nomos

- The `unit_id` field in `#CanonicalMatrix` accepts `#RBOKUnitID` values.
- The `source_id` field in `#SourceManifest` uses the same uppercase pattern but is not prefixed with `RBOK-` (sources are not domain objects).
- CUE validation: `specs/rbok-unit-ids.cue` defines the regex and type prefix enum.

## CLAIRE Dimensions

The CLAIRE framework structures knowledge extraction into dimensions. Each dimension gets a `DIM` unit:

| ID | Dimension |
|----|-----------|
| `RBOK-DIM-contexte-v1` | Contexte — domain boundaries and stakeholders |
| `RBOK-DIM-limites-v1` | Limites — explicit scope boundaries |
| `RBOK-DIM-ambiguites-v1` | Ambiguites — unresolved or conflicting definitions |
| `RBOK-DIM-invariants-v1` | Invariants — rules that must always hold |
| `RBOK-DIM-regles-v1` | Regles — business rules and constraints |
| `RBOK-DIM-exceptions-v1` | Exceptions — documented deviations from rules |

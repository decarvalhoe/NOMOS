# 07 - Tests, Gates Et Release

## Philosophie

La conformité métier n'est pas prouvée par un gros test E2E final. Elle est prouvée par une pyramide qui relie source, contrat, schéma, read-model, core, API et UI.

Les tests doivent répondre à trois questions :

- La donnée vient-elle du canon ?
- Le comportement respecte-t-il la règle ?
- Le produit empêche-t-il les contournements ?

## Familles De Tests

| Famille | But |
|---|---|
| Manifest tests | Sources présentes, hash correct, statut cohérent. |
| Matrix tests | Unités atomisées, couverture, gaps explicites. |
| Contract tests | YAML/JSON valide, références croisées, invariants. |
| Loader tests | Read-model reproductible et idempotent. |
| Core tests | Calculs purs, erreurs métier, cas limites. |
| API tests | Contrat HTTP, provenance, permissions. |
| UI tests | Pas de fixtures, affichage correct, provenance visible. |
| RAG tests | Retrieval, citations, refus, conflits, fraîcheur. |
| Golden cases | Scénarios validés par expert de bout en bout. |

## Golden Cases

Un golden case est un cas métier de référence. Il doit être écrit dans un format lisible, versionné et sourcé.

Exemple :

```yaml
id: GOLDEN-INS-HOME-001
name: "Dégât des eaux couvert avec franchise standard"
source_refs:
  - source_id: CONTRACT-HOME-2026
    locator: "clause 3.1"
inputs:
  event: pipe_leak
  damage_amount_eur: 2400
expected:
  covered: true
  deductible_eur: 250
  payout_eur: 2150
applied_units:
  - INS-HOME-WARRANTY-WATER-DAMAGE
```

Les golden cases sont précieux parce qu'ils transforment l'expertise métier en tests durables.

## Gates

### `validate`

Gate quotidienne :

- format/lint ;
- typecheck ;
- tests unitaires ;
- tests contrats ;
- `canonical:check`.

### `canonical:check`

Gate de cohérence :

- manifest valide ;
- sources actives accessibles ;
- hashes synchronisés ;
- matrice valide ;
- contrats validés ;
- read-model générable ;
- sources actives indexées ou explicitement non indexables.

### `canonical:check:strict`

Gate de conformité :

- zéro sample/mock sur les routes produit ;
- zéro unité critique `missing` ;
- zéro source active sans chunk ;
- zéro contrat sans source refs ;
- zéro ambiguïté critique sans décision ;
- zéro import interdit dans le core ;
- golden cases critiques verts.

### `release:compliance`

Gate release :

- strict vert ;
- couverture métier publiée ;
- changelog source/contrat/code ;
- ADR nouveaux ou modifiés ;
- sauvegarde/rollback vérifiés ;
- approbation propriétaire métier.

## Rapport De Couverture

Chemin recommandé :

```text
docs/canonical/coverage-report.md
```

Il doit contenir :

- nombre de sources par statut ;
- nombre d'unités par type et statut ;
- unités critiques manquantes ;
- sources non indexées ;
- contrats invalides ;
- fuites sample/mock ;
- tests manquants ;
- top risques ;
- décision go/no-go release.

## Politique De Dérogation

Une dérogation peut exister, mais doit être explicite :

- ID ;
- scope ;
- raison ;
- risque ;
- date d'expiration ;
- owner ;
- mitigation ;
- validation humaine.

Une dérogation sans expiration devient une nouvelle règle cachée. Elle doit donc être interdite.


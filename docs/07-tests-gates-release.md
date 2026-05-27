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

### Entrées du strict release gate et section d'intégrité corpus

Le strict release gate agrège les entrées ci-dessus (`validate`, `canonical:check`, `canonical:check:strict`, `release:compliance`). La fidélité source-to-feed est désormais une **section additive opt-in** câblée par SFI-08 (`#346`).

| Entrée du strict-gate | Statut | Notes |
|---|---|---|
| `validate` (lint, typecheck, unit, contract, `canonical:check`) | Actif | Gate quotidien. |
| `canonical:check` (manifest, hashes, matrice, contrats, read-model) | Actif | Gate de cohérence. |
| `canonical:check:strict` (no mocks, aucune unité critique manquante, golden cases) | Actif | Gate de conformité. |
| `release:compliance` (strict vert, couverture, changelog, ADR, rollback, approbation propriétaire) | Actif | Gate de release. |
| `corpus_integrity_check` (couverture source, spans dupliqués, contenu junk, linkage feed, linkage RAG) | Actif, opt-in | Câblé par `#346` (SFI-08) ; entrées issues de `#342` (SFI-04 source-integrity gate) et `#345` (SFI-07 feed-quality gate). |

#### `corpus_integrity_check` — section opt-in

Le strict gate ajoute un champ JSON de premier niveau `corpus_integrity_check`
lorsqu'un flag `--corpus-integrity-*` est fourni à `nomos strict` :

- `--corpus-integrity-report=PATH` — charge un rapport d'intégrité / qualité précalculé ;
- `--corpus-integrity-source=DIR` — recalcule le rapport source-integrity à la volée depuis un répertoire de fichiers `*.md` ;
- `--corpus-integrity-feed=PATH` — combiné à `--corpus-integrity-source` pour calculer aussi le rapport feed-quality contre un fichier JSON `[]FeedUnit` ;
- `--corpus-integrity-rag=PATH` — combiné à `--corpus-integrity-source` pour injecter `[]ChunkMetadata` dans le même calcul feed-quality.

`corpus_integrity_check.status` prend l'une de trois valeurs :

| status | Signification | Effet sur le strict gate |
|---|---|---|
| `pass` | chaque sous-rapport fourni (source-integrity, feed-quality) a réussi. | ne bloque pas le gate. |
| `fail` | au moins un sous-rapport fourni a échoué, ou une erreur de chargement/parsing est survenue. | bascule `valid: false` et la CLI sort en non-zéro. |
| `not_provided` | aucun flag `--corpus-integrity-*` n'a été fourni. | la section est entièrement omise ; le JSON de gate existant est inchangé pour les appelants qui n'optent pas. |

La projection CUE est `#CorpusIntegrityCheck` dans `specs/corpus-integrity-report.cue`. La méthode complète, le catalogue de finding-codes et la procédure de revue opérateur vivent dans [`docs/21-source-feed-integrity-engine.md`](21-source-feed-integrity-engine.md). La règle de phrase réservée pour `full_fidelity_proven` (phrase écrivable uniquement pour les builds dont le rapport d'intégrité corpus est présent et passant) vit dans [`docs/public-claim-boundary.md`](public-claim-boundary.md). Épopée parente : `#337`.

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

## Rapport Machine-Readable

Chemin recommandé :

```text
nomos-report.json
```

Le format public est `specs/nomos-report.schema.json`.

Le report JSON est l'interface stable entre le CLI, les gates CI, les SDK, les
attestations et le control plane. Il doit contenir au minimum :

- le contexte d'execution `run` ;
- le projet et le manifest inspectes ;
- un `summary` numerique ;
- un `verdict` `pass`, `warn`, `fail` ou `blocked` ;
- les resultats de checks ;
- les findings avec severite, code erreur `NOMOS_*`, cible et remediation ;
- l'evidence referencee par les findings ;
- les waivers eventuels avec expiration.

Un `fail` contient au moins un finding bloquant. Un `blocked` signifie que
Nomos n'a pas pu terminer le jugement, par exemple a cause d'une entree
manquante ou d'une erreur d'execution. Les exemples de payload vivent dans
`specs/examples/nomos-report.*.json`.

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

# 04 - Contrats, Schémas Et Read-Models

## Pourquoi

La documentation décrit. Le contrat produit exécute.

Un catalogue YAML/JSON n'est pas un exemple. C'est l'interface stable entre le métier et le logiciel. Les schémas empêchent les approximations. Les read-models rendent les données exploitables par API, UI, CMS et core.

## Contrats Canoniques

Les contrats structurent ce qui peut l'être :

- listes : produits, garanties, compétences, actes, classes, formulaires ;
- règles paramétriques : seuils, coûts, limites, durées, coefficients ;
- conditions : éligibilité, exceptions, prérequis ;
- effets : calcul, attribution, interdiction, avertissement ;
- liens : source, unité, décision, dépendance.

Exemple minimal :

```yaml
id: INS-HOME-WARRANTY-WATER-DAMAGE
type: warranty
name: "Dégât des eaux"
source_refs:
  - source_id: CONTRACT-HOME-2026
    locator: "p. 12, clause 3.1"
    hash: "sha256:..."
status: active
conditions:
  included_events:
    - pipe_leak
    - appliance_overflow
  excluded_events:
    - roof_infiltration_without_maintenance
limits:
  ceiling_eur: 15000
  deductible_eur: 250
decisions: []
```

## Schémas

Le schéma valide :

- structure ;
- types ;
- champs obligatoires ;
- enums ;
- invariants simples ;
- références croisées ;
- version de contrat.

Options par stack :

| Stack | Schéma recommandé |
|---|---|
| TypeScript | Zod, TypeBox, JSON Schema généré. |
| Python | Pydantic, msgspec, JSON Schema. |
| JVM | JSON Schema, Avro, protobuf, Kotlin serialization. |
| Go | JSON Schema + validateurs générés, protobuf si contrat binaire. |
| Data platform | dbt contracts, Great Expectations, OpenAPI/AsyncAPI, Avro. |

Le schéma n'est pas suffisant pour la conformité métier. Il garantit la forme, pas toute la sémantique. Les règles métier doivent être testées.

## Read-Models

Le read-model est dérivé du contrat. Il peut être :

- relationnel : PostgreSQL, MySQL, SQLite ;
- document : MongoDB, DynamoDB ;
- search : Elasticsearch/OpenSearch ;
- graphe : Neo4j, RDF ;
- mémoire : JSON précompilé, SQLite embarqué, WASM bundle.

Canonical-First recommande un read-model relationnel quand :

- les liens entre unités sont importants ;
- les validations de références croisées sont nombreuses ;
- le CMS doit éditer les données ;
- les rapports de couverture doivent être requêtables.

## Règles De Chargement

1. Charger les contrats depuis `data/canonical/` ou équivalent.
2. Valider par schéma.
3. Résoudre les références internes.
4. Calculer un hash de contrat.
5. Écrire le read-model dans une transaction.
6. Enregistrer la provenance : source IDs, source hashes, contract hash, loader version.
7. Refuser les données non sourcées en mode strict.

## Versioning

Chaque contrat doit porter :

- `schema_version` ;
- `effective_from` si le domaine a des dates d'application ;
- `effective_to` si l'entrée est remplacée ;
- `source_refs` ;
- `decision_refs` ;
- `migration_notes` si changement cassant.

Les systèmes réglementaires doivent permettre de poser une question temporelle :

> Quelle règle était applicable le 2025-12-31 ?

## Données Non Structurables

Tout n'a pas besoin de devenir YAML détaillé. Certains contenus restent textuels :

- exemples narratifs ;
- contexte historique ;
- explications ;
- jurisprudence longue ;
- lore ;
- commentaires de doctrine.

Mais même ces contenus doivent avoir :

- une unité ;
- une source ;
- un chunk vectoriel ;
- une metadata ;
- un statut dans la matrice.

## Tests Associés

- Tous les contrats valident contre les schémas.
- Toutes les références sources existent dans le manifest.
- Toutes les références unitaires existent dans la matrice.
- Les read-models sont reproductibles depuis les contrats.
- Les loaders sont idempotents.
- Aucun contrat `active` n'utilise un champ inconnu.
- Aucun produit ne lit un fichier sample/mock en mode strict.


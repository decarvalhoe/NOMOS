# 03 - Atomisation Et Matrice Canonique

## Pourquoi

Une source entière est trop grosse pour être testée. Une règle dans un paragraphe, une entrée de liste, une exception ou une formule est la bonne granularité. L'atomisation transforme le corpus en unités que l'on peut suivre, valider, charger, indexer, tester et afficher.

La matrice est le tableau de bord de vérité. Elle ne décrit pas seulement ce qui existe ; elle montre où chaque unité est présente dans la chaîne produit.

## Unité Atomique

Une unité atomique doit être :

- identifiable par un ID stable ;
- rattachée à une ou plusieurs sources ;
- assez petite pour être testable ;
- assez complète pour ne pas perdre le sens métier ;
- versionnable ;
- capable de porter un statut.

Exemples par domaine :

| Domaine | Unités possibles |
|---|---|
| Assurance | clause, garantie, exclusion, franchise, plafond, formule de prime, règle d'éligibilité. |
| Fiscalité | article, seuil, taux, crédit, exception, formulaire, période d'application. |
| Clinique | recommandation, contre-indication, dosage, protocole, score, critère d'inclusion. |
| Droit | disposition, définition, jurisprudence, exception, délai, sanction. |
| Jeu à règles | règle, race, classe, compétence, spécialisation, sort, atout, handicap, objet, monstre. |
| RH | convention, classification, prime, congé, grille salariale, règle d'ancienneté. |

## IDs

Les IDs doivent être stables et significatifs :

```text
RULE-RESOLUTION-0017
TAX-FR-CREDIT-ENERGY-2026
INS-HOME-WARRANTY-WATER-DAMAGE
CLIN-DIABETES-HBA1C-THRESHOLD
GAME-SKILL-ALCHEMY
```

Ne pas encoder une position de fichier volatile dans l'ID principal. Les positions exactes vont dans les références.

## Matrice

Chemin recommandé :

```text
docs/canonical/<domain>-matrix.yaml
```

Chaque ligne doit couvrir :

| Champ | Rôle |
|---|---|
| `unit_id` | ID atomique. |
| `unit_type` | Type : rule, catalog_entry, exception, formula, term, workflow, etc. |
| `name` | Nom lisible. |
| `domain` | Domaine. |
| `source_refs` | Fichier/URL + lignes/pages/sections + hash. |
| `business_rule` | Résumé contrôlé de la règle métier. |
| `canonical_contract` | Fichier YAML/JSON ou `not_structurable`. |
| `schema_refs` | Schéma ou type qui valide. |
| `db_refs` | Table/vue/read-model concerné. |
| `vector_refs` | Collection/index/chunk IDs attendus. |
| `core_refs` | Fonctions ou modules déterministes. |
| `api_refs` | Routes ou events exposant l'unité. |
| `ui_refs` | Surfaces produit. |
| `test_refs` | Tests unitaires, intégration, E2E, golden cases. |
| `status` | `covered`, `partial`, `missing`, `not_applicable`, `deprecated`. |
| `gaps` | Ce qui manque précisément. |
| `decision_refs` | ADR, decision record, écart, arbitrage. |

## Statuts De Couverture

| Statut | Définition |
|---|---|
| `covered` | Toutes les étapes requises sont prouvées. |
| `partial` | L'unité est partiellement traitée avec gaps documentés. |
| `missing` | L'unité existe dans la source mais pas encore dans le produit. |
| `not_applicable` | L'étape n'a pas de sens pour cette unité, avec justification. |
| `deprecated` | L'unité est remplacée mais conservée pour historique. |

## Règles De Granularité

Trop gros :

- "Règles de combat"
- "Contrat assurance habitation"
- "Protocole diabète"

Correct :

- "Échec critique si plus de résultats 1 que de réussites"
- "Garantie dégât des eaux exclut infiltration par toiture non entretenue"
- "HbA1c >= seuil X déclenche recommandation Y, sauf grossesse"

Trop petit :

- un mot isolé sans impact produit ;
- une phrase de style sans valeur normative ;
- une répétition exacte déjà couverte.

## Conflits Et Ambiguïtés

Un conflit doit devenir une unité de décision :

```yaml
unit_id: DECISION-TAX-ROUNDING-001
unit_type: ambiguity
name: "Arrondi du montant imposable"
source_refs:
  - source_id: TAX-GUIDE-2026
    locator: "section 4.2"
  - source_id: LEGACY-CALC-ROUNDING
    locator: "TaxCalculator.php:188"
conflict: "La source écrite arrondit à l'euro, le legacy tronque les centimes."
decision_ref: ADR-0007
status: partial
```

Ne jamais résoudre ce type d'écart silencieusement dans un moteur.

## Tests Associés

- Toute source active a au moins une unité ou une justification `not_applicable`.
- Toute unité critique a au moins un test.
- Toute unité `covered` a des références valides à chaque couche applicable.
- Aucune unité `missing` n'est autorisée en release stricte si elle est `critical`.
- Toute unité `partial` a un gap explicite et un owner.


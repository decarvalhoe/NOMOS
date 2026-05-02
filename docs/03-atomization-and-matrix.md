# 03 - Atomisation Et Matrice Canonique

## Pourquoi

Une source entière est trop grosse pour être testée. Une règle dans un paragraphe, une entrée de liste, une exception ou une formule est la bonne granularité. L'atomisation transforme le corpus en unités que l'on peut suivre, valider, charger, indexer, tester et afficher.

La matrice est le tableau de bord de vérité. Elle ne décrit pas seulement ce qui existe ; elle montre où chaque unité est présente dans la chaîne produit.

## Cadre NOMOS

L'atomisation ne produit pas une collection de fragments isolés. Elle produit la colonne vertébrale de preuve du projet :

```text
source -> structure documentaire -> unite atomique -> reference canonique -> ligne de matrice -> contrat/read-model/core/tests -> chunk RAG
```

Chaque couche doit pouvoir se réconcilier avec les autres par identifiants stables. Un chunk sans unité, une unité sans source, une ligne de matrice sans référence, ou un contrat sans preuve est une rupture de conformité.

La granularité attendue est la plus fine granularité pertinente. Il ne faut pas découper pour découper. Il faut découper au niveau où une règle, une définition, une exception, une formule, une condition, une entrée de catalogue ou une décision peut changer, être revue, être testée et être citée sans perdre son contexte.

## Unité Atomique

Une unité atomique doit être :

- identifiable par un ID stable ;
- rattachée à une ou plusieurs sources ;
- rattachée à une structure documentaire parent ;
- rattachée à une référence canonique citable ;
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
| `structural_refs` | Parent documentaire : document, chapitre, section, article, paragraphe, alinéa, table, note. |
| `source_refs` | Fichier/URL + lignes/pages/sections + hash. |
| `canonical_refs` | Références stables machine et labels humains citables. |
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

Décision de découpage :

1. Identifier la structure documentaire avant le sens métier.
2. Découper si deux assertions peuvent avoir des statuts, dates, exceptions, tests ou propriétaires différents.
3. Ne pas découper si le fragment perd son sujet, sa condition ou son exception.
4. Transformer les listes en unités quand chaque item porte une règle ou entrée de catalogue.
5. Transformer les tableaux en unités par ligne, cellule ou table complète selon le sens métier, avec justification.
6. Marquer `needs_review` quand la granularité dépend d'une interprétation.

## Chunks Et RAG

Le chunk est une projection de recherche. Il n'est pas l'unité de vérité.

Un chunk doit référencer au minimum :

- `source_id` et `source_hash` ;
- `source_span` ou locator vérifiable ;
- `structural_refs` ;
- `unit_ids` quand le chunk contient une unité ;
- `matrix_refs` quand l'unité est gouvernée par la matrice ;
- `chunking_strategy` et version.

Un chunk peut contenir plusieurs unités seulement s'il est déclaré comme chunk de contexte et liste toutes les unités incluses. Une réponse IA doit citer le chunk mais fonder son autorité sur l'unité et la matrice liées.

## Profils De Corpus

Les mêmes règles s'appliquent à plusieurs familles de corpus :

| Corpus | Structure pertinente | Unité atomique typique | Projection produit |
|---|---|---|---|
| RBOK Engine | chapitre, section, sous-section, article, paragraphe, alinéa, gouvernance. | principe, définition, exigence, procédure, contrôle, exemple. | `rbok_nodes`, Builder refs, runtime IA, tests métier. |
| Livre de loi ou réglementation | instrument, livre, titre, chapitre, section, article, paragraphe, alinéa, amendement. | obligation, permission, interdiction, condition, exception, sanction, date d'effet. | matrice conformité, contrôles, applicabilité, citations. |
| Règles de jeu Knight & Wizard | domaines de règles, catalogues, tables, exceptions, règles legacy. | mécanique, compétence, race, classe, sort, atout, handicap, ambiguïté. | YAML catalogues, schémas, rules-core, UI, golden play tests, RAG cité. |

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

# Verdict Taxonomy

Issue: [NOM-104](https://github.com/RBOKproject/Nomos/issues/16)

Nomos utilise les verdicts pour décider si une surface produit peut entrer dans
la chaîne Canonical-First, quelles preuves doivent exister et qui doit résoudre
les écarts avant qu'une gate stricte puisse passer. Le vocabulaire
machine-readable vit dans `specs/verdicts.cue`.

## Table Des Verdicts

| Verdict | Sens | Niveau de confiance minimal | Effet sur les gates | Escalade par défaut |
| --- | --- | --- | --- | --- |
| `in_scope` | La surface est admise dans le scope Nomos avec autorité active et aucun blocage connu. | `high` | Éligible à `canonical:check` et aux gates de release stricte tant que la preuve reste forte. | `none` |
| `partial` | La surface peut entrer dans Nomos avec écarts, hypothèses et trajectoire de remédiation explicites. | `medium` | Accepté pour bootstrap et adoption brownfield ; insuffisant pour release stricte sur surfaces critiques. | `domain_owner` |
| `blocked` | La surface ne doit pas être promue car autorité, preuve, ownership ou conformité reste non résolu. | `low` | Fait échouer les checks canoniques jusqu'à résolution ou reclassement gouverné. | `product_owner` |
| `out_of_scope` | La surface est volontairement exclue du scope Nomos courant. | `medium` | Ignoré par les gates canoniques sauf surveillance de dérive de frontière. | `none` |

## Niveaux De Confiance

| Niveau | Sens | Preuve minimale |
| --- | --- | --- |
| `low` | La preuve est incomplète, obsolète, contradictoire ou surtout inférée. | Au moins une incertitude ou un écart explicite. |
| `medium` | La preuve soutient le verdict, mais des hypothèses ou une couverture downstream incomplète subsistent. | Une référence de source ou de comportement legacy, plus hypothèses documentées. |
| `high` | La preuve est actuelle, sourcée, owned et cohérente dans la chaîne Nomos requise. | Références de sources actives, owner ou decision record, et aucun blocker ouvert sur le scope admis. |

## Règles D'Escalade

- Escalader `blocked` vers un product owner par défaut ; utiliser un compliance
  owner quand le blocage est légal, réglementaire, safety ou lié à une
  attestation signée.
- Escalader `partial` vers un domain owner quand une source manquante, une
  hypothèse ou une compatibilité legacy affecte la sémantique produit.
- Escalader tout verdict en confiance `low`. Un résultat en confiance faible ne
  s'auto-approuve jamais, même avec un verdict `out_of_scope`.
- Garder `in_scope` sans escalade seulement quand la confiance est `high` et
  qu'aucun blocker ne reste. Si la confiance tombe à `medium`, dégrader vers
  `partial` ou ajouter un decision record explicite.
- `out_of_scope` doit nommer la frontière exclue. Si une surface exclue pilote
  encore un comportement produit actif, la reclasser en `partial` ou `blocked`.

## Cas Limites

| Cas | Verdict correct | Raison |
| --- | --- | --- |
| Un repo brownfield a un comportement legacy utile mais des PDFs source manquants. | `partial` | Le repo peut entrer dans Nomos seulement avec gaps suivis et remédiation avant release stricte. |
| Un calcul réglementé critique n'a pas d'owner de décision accountable. | `blocked` | L'owner manquant empêche la promotion, quelle que soit la couverture de code. |
| Le partner billing existe dans le repo mais n'appartient pas au lancement courant. | `out_of_scope` | La frontière est explicite et peut être surveillée pour dérive. |
| Une API greenfield réglementée a sources actives, owners, contrats et preuve signée. | `in_scope` | La chaîne a assez d'autorité pour entrer dans les gates strictes. |

Valider les exemples de cas limites avec :

```bash
source scripts/nomos-env.sh
cue vet specs/verdicts.cue specs/examples/verdict-cases.yaml -d '#VerdictCaseSet'
```

## Corpus Admission Verdicts

Les verdicts corpus déterminent si un corpus de sources peut alimenter la chaîne
canonique, indépendamment du verdict de surface produit. Un corpus peut être
admissible même si la surface produit est encore `partial` (le corpus est prêt,
le produit pas encore connecté).

### Différence Corpus vs Produit

| Dimension | Verdict Produit | Verdict Corpus |
| --- | --- | --- |
| **Objet évalué** | Surface produit (API, UI, worker…) | Ensemble de documents sources |
| **Question clé** | La surface peut-elle entrer dans les gates Nomos ? | Le corpus peut-il alimenter la chaîne canonique ? |
| **Preuve requise** | Manifests, contrats, tests, coverage | Hashes, provenance, ownership, cohérence |
| **Conséquence blocage** | La surface ne peut pas être promue | Le corpus ne peut pas nourrir les contrats |
| **Héritage** | — | Un corpus `corpus_blocked` bloque les surfaces qui en dépendent |

### Table Des Verdicts Corpus

| Verdict | Sens | Confiance minimale | Effet | Escalade |
| --- | --- | --- | --- | --- |
| `corpus_admissible` | Le corpus est complet, hashé, owné et prêt à alimenter la chaîne sans restriction. | `high` | Alimente contrats, read-models et knowledge base sans review supplémentaire. | `none` |
| `corpus_partial` | Le corpus peut commencer à alimenter la chaîne mais a des gaps connus ou des documents stale nécessitant remédiation. | `medium` | Alimente surfaces non-critiques ; surfaces critiques requièrent complétion. | `domain_owner` |
| `corpus_blocked` | Le corpus ne peut pas alimenter la chaîne : sources critiques manquantes, inaccessibles ou sans owner. | `low` | Exclu de la chaîne canonique ; les surfaces dépendantes héritent du blocage. | `product_owner` |

### Critères D'Admission Corpus

Pour qu'un corpus passe de `corpus_blocked` à `corpus_partial` :

1. Au moins un document source est indexé avec hash et provenance.
2. Les gaps sont listés explicitement dans le source manifest.
3. Un plan de remédiation existe pour chaque source manquante.

Pour qu'un corpus passe de `corpus_partial` à `corpus_admissible` :

1. Tous les documents sont indexés avec metadata de provenance.
2. Chaque document a un hash valide et un owner identifié.
3. Aucun document contradictoire ou stale sans record de supersession.
4. Le scope du corpus est aligné avec le domaine projet déclaré.

### Interaction Corpus × Produit

```text
corpus_admissible + in_scope    → gates strictes éligibles
corpus_admissible + partial     → bootstrap OK, strict après remédiation produit
corpus_partial    + in_scope    → strict bloqué sur surfaces critiques dépendantes
corpus_partial    + partial     → bootstrap uniquement
corpus_blocked    + *           → surfaces dépendantes héritent blocked
```

### Cas Limites Corpus

| Cas | Verdict corpus | Raison |
| --- | --- | --- |
| PDFs réglementaires tous scannés, hashés et ownés. | `corpus_admissible` | Chaîne de provenance complète. |
| Legacy docs partiellement numérisés, 3 PDFs manquants identifiés. | `corpus_partial` | Gaps connus, remédiation planifiée. |
| Contrat partenaire sous NDA, pas de droit d'indexation. | `corpus_blocked` | Restriction légale empêche l'admission. |
| Corpus technique complet mais owner parti sans successeur. | `corpus_blocked` | Pas d'owner accountable pour approuver. |

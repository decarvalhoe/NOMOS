# 51 — Plan autonome vers la release beta (`v1.0.0-BETA.1`)

> Date : 2026-09-06. Statut : plan actif, exécuté par le dispatcher autonome
> (`docs/47`, `docs/roadmap-lanes.yaml`) ; les définitions NRT-031 à NRT-036
> vivent dans `docs/29`, ce doc dit pourquoi, dans quel ordre, et où s'arrête
> l'autonomie. Claim boundary : ce plan ne crée aucune beta. `ready` est un
> verdict calculé sur l'arbre ; une release est un acte humain sous SOP
> (#720) ; ni l'un ni l'autre n'est une claim régulée (position NQ-*, plan
> `docs/28`).

---

## 0. Ce que « beta » veut dire ici — décision

Aucune définition de beta n'existait dans le dépôt. Le `CHANGELOG` dit
seulement que les labels alpha/beta restent tant que l'API publique, les
contrats d'evidence et le modèle de support ne sont pas assez stables pour
une `v1.0` ; `docs/14` définit la `v1.0` par huit critères et `docs/29` les
a transformés en checks (NRT-023 à NRT-028). Décision :

**La beta est `v1.0.0-BETA.1`, première pré-release de la ligne 1.0.** Elle
est atteinte quand l'arbre est un *candidat stable* au sens de `docs/14` —
les huit critères calculés `ready` par `nomos portfolio release-readiness` —
et que ce verdict est un gate requis du candidat de release. La `v1.0.0`
finale est la décision humaine qui suit l'exposition beta ; elle n'est pas
dans ce plan.

Pourquoi pas `v0.3.0-BETA` : `docs/29` a déjà consommé `v0.3` à `v0.9` comme
jalons d'issues livrées ; le jalon suivant est `v1.0.0`, et une beta de la
ligne 1.0 dit ce qu'elle est. SemVer (`docs/16`) admet la pré-release
`1.0.0-BETA.1` ; la casse suit `v0.2.0-ALPHA`.

Conditions, toutes calculées sauf la dernière :

| # | Condition | Où c'est vérifié |
|---|---|---|
| B1 | `release-readiness` = `ready` sur les huit critères de `docs/14` | `nomos portfolio release-readiness`, asserté en CI |
| B2 | le verdict est un gate **requis** du candidat de release : un candidat beta `not_ready` est refusé | `scripts/release_candidate_gates.py`, `nomos release candidate` (NRT-034) |
| B3 | la surface de support de la beta est déclarée en données et recoupée avec ce que les guides citent | `docs/support-model.yaml`, `scripts/support_model_guard.py` (NRT-035) |
| B4 | le candidat est préparé : version, changelog, notes, modèle de support, record de décision `pending` | PR NRT-036, mergeable seulement sur `ready` |
| B5 | tag, publication, approbation | **acte humain** sous SOP, #720 — jamais un outil |

Ce que la beta n'est pas : une claim régulée (validated use, effectivité
QMS, Part 11 — `docs/28`), un SLA (`docs/support-model.yaml` reste une
déclaration), la preuve croisée avec un consommateur externe (#701), une
activation Sigstore keyless (`sigstore_keyless` reste `absent` par
construction ; la beta vérifie hors ligne, elle ne signe pas).

## 1. Où en est l'arbre — mesuré le 2026-09-06

`nomos portfolio release-readiness --repo-root .` sur `main` (`8963821`) :
verdict `not_ready`, trois checks non satisfaits, tout le reste vert.

| Critère | État | Manque nommé par l'outil |
|---|---|---|
| C1 scopes de corpus explicites et reproductibles | non | 14 contrats `stable` sans fixture de compatibilité : canon-promotion, canonical-knowledge-bundle, canonical-matrix, corpus-body-ledger, corpus-integrity-report, domain-pack, facets, knowledge-lens, nomos-praxis-evidence-schema, nomos-project, nomos-report.schema, point-in-time, source-manifest, verdicts |
| C2 structures non supportées = records d'evidence | oui | — |
| C3 spans et hiérarchie vérifiables indépendamment | oui | — |
| C4 adapters : contrats et fixtures de compatibilité | oui | — |
| C5 chunks, matrices, rapports, attestations reconstructibles | oui | — |
| C6 outils d'evidence : usage prévu, validation, reliance | non | items clos sans `regulated_tool` : #642, #610, #611, #612 |
| C7 la doc régulée consomme une evidence versionnée | non | ledger d'evidence en `status: draft`, daté 2026-05-02 (périmé sous la politique de fraîcheur de 90 jours) |
| C8 les claims publiques n'excèdent jamais l'evidence | oui | — |

Portefeuille au même instant : 65 capacités (47 `real`, 16 `sidecar`, 2
`absent` par conception, 0 écart) ; lane produit 17 items autonomes clos, 0
ouvert ; lane devops 9 clos ; lane régulée 1 passif, 3 humains, 5 externes.
Toutes les files de dispatch sont vides : le plan les remplit.

Recalculer avant de croire ce tableau :

```bash
cd cli && go run . portfolio release-readiness --repo-root .. && go run . portfolio status --repo-root .. --format md
```

## 2. Les items

Six items autonomes ferment les trois manques et ajoutent ce qu'une beta
exige au-delà du verdict ; un item humain la publie. Définitions complètes,
definition of done, commandes de vérification et claim impact : `docs/29`,
section « v1.0.0-BETA.1 » ; chaque issue en est la copie.

| NRT | Issue | Lane | Dispatch | Ferme | Dépend de (même lane) | Livrable |
|---|---|---|---|---|---|---|
| NRT-031 | #714 | product | autonomous | C1 | — | `readCompat` relit chaque contrat stable par son lecteur Go réel ; une fixture de compatibilité par contrat, sur les fixtures valides existantes |
| NRT-032 | #715 | product | autonomous | C6 | — | blocs `regulated_tool` de #642, #610, #611, #612 |
| NRT-033 | #716 | devops | autonomous | C7 | — | ledger d'evidence généré depuis l'arbre, `effective` = index en vigueur, dérive et péremption rouges |
| NRT-034 | #717 | product | autonomous | B2 | NRT-031, NRT-032 | gate `release-readiness` requis du candidat beta ; refus asserté en CI, puis succès asserté |
| NRT-035 | #718 | devops | autonomous | B3 | — | `support_surface` dans le modèle de support, guides docs/48 et docs/50 recoupés |
| NRT-036 | #719 | product | autonomous | B4 | NRT-031, NRT-032, NRT-034 | candidat `v1.0.0-BETA.1` préparé ; la PR n'est mergeable que sur `ready` |
| — | #720 | regulated | human | B5 | préalable NRT-036 | tag, publication, record d'approbation signé |

Les entrées inter-lanes (NRT-033 et NRT-035 pour NRT-034 et NRT-036) sont
non bloquantes par construction (`docs/47`) : le câblage se fait sans les
attendre, la bascule verte des tripwires les attend.

## 3. Vagues et files de dispatch

- **Vague A** — en parallèle, aucune dépendance : NRT-032 (une matinée),
  NRT-031 (le gros du plan : quatorze lecteurs), NRT-033, NRT-035.
- **Vague B** — NRT-034, dès que NRT-031 et NRT-032 sont clos ; entre-temps
  le refus du candidat beta est asserté, ce qui est déjà une preuve.
- **Vague C** — NRT-036, dernier pas autonome : sa CI asserte `ready` et un
  candidat beta assemblé ; une PR posée sur un arbre `not_ready` est rouge
  par construction.
- **Sortie** — #720 : vérification du bundle, signature du record, tag,
  notes ; puis une PR de suite (autonome) met le modèle de support et le
  changelog à la date réelle.

Files déclarées dans `docs/roadmap-lanes.yaml` (règle : premier item
autonome ouvert dont les dépendances de même lane sont closes) :
`product: [715, 714, 717, 719]`, `devops: [716, 718]`.

## 4. Hors du chemin beta — et pourquoi

| Item | Dispatch | Pourquoi il ne conditionne pas la beta |
|---|---|---|
| #701 preuve croisée avec le RAG voisin | external | attend le partenaire ; la beta prouve la mécanique côté NOMOS (`docs/50`), pas un consommateur |
| #576 / #638 Sigstore keyless | external | `sigstore_keyless` est `absent` par conception ; la beta vérifie hors ligne (#637) et le dit |
| #192 / #193 / #194 / #196 licences et bibles | external, human | ne bloquent que leurs claims d'usage nommées |
| #562 records de compétence | human | voie régulée ; la beta n'affirme aucune effectivité QMS |
| #560 evidence CI répétée | passive | s'accumule ; ce n'est pas un critère de `docs/14` |
| #561 release alpha via SOP | human | clôture de l'alpha, indépendante ; #720 est l'acte équivalent pour la beta |

## 5. Règles de conduite pendant le plan

- Convention de livraison inchangée : entrée au registre et matrice
  régénérée, lanes closes et `--emit-docs`, CHANGELOG, frontière de claims,
  commit en français co-signé, squash, commentaire de livraison sur l'issue.
- Aucun contrat `stable` ne change sans dépréciation datée (`docs/16`,
  #677) : la beta gèle ; une rupture MAJOR est hors plan.
- Rien ne bascule en silence : le passage de `not_ready` à `ready` et du
  candidat beta refusé à accepté sont des commits qui le nomment et
  retournent l'assertion CI ; un tripwire qui rougit est un finding, jamais
  un bruit à masquer.
- Le verdict est la vérité, le plan sa lecture datée : quand ils divergent,
  c'est le plan qui est mis à jour, jamais le verdict qui est arrangé.
- Chaque lecteur de compatibilité (NRT-031) est le lecteur réel du moteur ;
  un décodage ad hoc pour faire passer un check serait exactement le défaut
  que `docs/49` §3 nomme.

## 6. Risques

| Risque | Effet | Mitigation |
|---|---|---|
| NRT-031 : quatorze lecteurs, tentation du décodeur ad hoc | un check vert qui ne prouve rien | règle §5 ; tests de refus par lecteur ; revue de `readCompat` ligne à ligne |
| « effective » lu comme une effectivité QMS | claim régulée involontaire | `claim_boundary` du ledger et NRT-033 : index en vigueur, statuts recomptés, jamais adoucis |
| saut `0.2.0-ALPHA` → `1.0.0-BETA.1` | matrice de compatibilité et comparaison de versions | NRT-036 teste `1.0.0-BETA.1` dans la comparaison de versions du registre (`compat_test`) avant de bouger la constante |
| état `candidate` du modèle de support | guard rouge si un tag n'existe pas | l'état `candidate` existe déjà dans le guard ; `supported` seulement après le tag (#720) |
| l'acte humain tarde | candidat `pending` longtemps | rien d'autonome ne le raccourcit ; le candidat est re-assemblé sur chaque commit de `main` par le rehearsal, jamais périmé |

## 7. Claim boundary

Ce plan ne livre rien par lui-même. Une fois les six items clos, l'arbre
sera un candidat beta calculé `ready` avec un bundle assemblé et refusable ;
il ne sera une beta qu'après #720. Aucune de ces étapes ne modifie la
position régulée (NQ-2 alpha / NQ-3 candidate, `docs/14`), ne débloque une
claim d'usage validé, ni ne dit quoi que ce soit d'un partenaire.

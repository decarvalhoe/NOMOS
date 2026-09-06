# 49 — Un RAG juridique souverain voisin : concepts intégrés, anti-patterns appris, collaboration préparée

> Sources : deux documents privés datés du 2026-09-05, partagés par le
> propriétaire de NOMOS — un inventaire technique des composants et de leurs
> paramètres réels, et un mémoire d'architecture, de cartographie de domaines et
> de dimensionnement GPU — décrivant un système RAG juridique on-premise sous
> contrainte de souveraineté désigné ici comme « le système voisin ». Ce dépôt est public : le système n'est pas
> nommé, les documents n'y sont pas et n'y seront pas. **Aucun chiffre, aucun paramètre, aucun constat de
> sécurité du système tiers n'est reproduit ici** : ce doc retient les concepts,
> les classes de défauts et ce que NOMOS en fait.
> Régime d'emprunt (`docs/42` §0) : système privé → **s'inspirer du concept,
> jamais code, contenu ni IP**. Les exemples de ce dépôt sont synthétiques.
> Date : 2026-09-06. Statut : analyse + plan ; les artefacts listés en §2 sont
> livrés, la collaboration de §4 attend la décision du propriétaire.
> Claim boundary : ce doc analyse deux documents ; NOMOS n'a rien vérifié du
> système voisin et n'affirme rien sur son état réel.

---

## 0. Pourquoi ce doc

Le système voisin est, presque terme à terme, le **consommateur cible** de
NOMOS : un RAG sous secret professionnel, avec provenance par empreinte,
citations obligatoires, détection de contradiction entre versions normatives,
révision humaine et piste d'audit. Il fait ce que NOMOS ne fait pas et ne
prétend pas faire (retrieval hybride, reranking, NER, graphe, serving), et il
manque de ce que NOMOS fait (verdict cite-or-abstain calculé, fraîcheur d'index
prouvable, résolution point-in-time, ledger et attestation, jeu doré et seuils
versionnés, frontière de claims outillée).

Deux raisons d'écrire ce doc avant toute collaboration :

1. les deux documents pratiquent déjà la discipline de NOMOS — *ne jamais
   affirmer un état sans le vérifier* — et exposent leurs propres écarts. C'est
   rare, et cela rend leurs concepts directement transposables ;
2. les défauts qu'ils décrivent honnêtement forment une **classe** que NOMOS
   doit s'interdire chez lui avant de la reprocher à un partenaire.

## 1. Complémentarité, pas concurrence

| Fonction | Système voisin | NOMOS | Couture possible |
|---|---|---|---|
| Ingestion structurelle, découpage par article/alinéa | oui | atomisation par structure, identité par chemin + hash | NOMOS fournit les atomes canoniques ; le voisin les indexe |
| Dédoublonnage approximatif | oui, par similarité, rendu sensible à l'identité après un incident | jamais de fusion par similarité : identité structurelle, digest par chunk | `nomos rag manifest` : un chunk = une identité, jamais fusionné avec un voisin textuel |
| Retrieval hybride, reranking, expansion | oui | non, et ne le revendique pas | export `jsonl` / `langchain` / `llamaindex` |
| NER, coréférence, graphe de connaissances | oui | non | facettes et lens comme métadonnées d'entrée du graphe |
| Vérification des citations, confiance, contradiction de version | annoncées comme validations de réponse, non inventoriées comme composants | gate cite-or-abstain, tiers par réponse, résolveur point-in-time | leurs enregistrements de réponse → `nomos answer gate` |
| Jeu doré et seuils | un seul domaine couvert | harnais `answer eval`, bench public, seuils versionnés | corpus doré au format du harnais, fourni par le domaine |
| Provenance et audit | empreinte par chunk, chaîne journalière | body ledger Merkle, attestation signée, `claim_coverage` calculée | le ledger NOMOS comme source de leur store de provenance |
| Fraîcheur d'index | par run, en mémoire | `rag manifest` / `delta` / `verify` par source | staleness prouvable après chaque réingestion |
| Souveraineté | frontière formalisée corpus public / données d'affaire | classes de sources, connecteurs hash-only, aucun appel réseau à l'inférence | §2.3 |
| Serving LLM, GPU | oui, dimensionnement MoE raisonné | hors périmètre | rien à intégrer ; savoir utile pour la collaboration |

## 2. Concepts intégrés à NOMOS

### 2.1 Cartographie de domaine à couches indépendantes — contrat livré

Le concept : pour un domaine juridique, dire *ce qui est réellement disponible,
ce qui manque, et pourquoi*, avec trois natures de domaine — **sous-corpus
propre**, **domaine fantôme** (réel juridiquement, sans collection technique,
hébergé dans des voisins, gardé visible pour ne pas disparaître du champ
commercial) et **socle transversal** (partagé par construction, jamais
dupliqué) — et un tableau de complétude à **quatre couches vérifiées
indépendamment** : source, index, enrichissement, graphe. La leçon qui fonde le
contrat : une couche ne se déduit jamais d'une autre ; un texte peut être
indexé sans être enrichi, enrichi sans être relié, présent dans le manifeste et
absent de l'index.

Ce que NOMOS livre : `specs/domain-cartography.cue`
(`nomos-domain-cartography-v1`) avec une fixture valide synthétique et deux
fixtures refusées en CI — une couche déclarée `not_verified` qui porte un
compte (l'inférence est interdite par le schéma), un domaine fantôme qui
possède un sous-corpus. Rôle : record de `customer-integration` et pièce de
toute preuve de consommation (« voici ce que le consommateur tient
réellement »). Capacité `domain_cartography_contract` au registre, comptée
`sidecar` : contrat + fixtures, aucun lecteur Go — le jour où un moteur la lit,
elle se promeut, pas avant.

### 2.2 Inventaire de paramètres « validé / défaut / obsolète » — template livré

Le concept : chaque paramètre qui influence le comportement porte sa valeur
réelle lue dans le code, un statut qui dit *comment* il a obtenu cette valeur,
l'impact d'une modification et, quand il existe, l'incident qui l'a fait
bouger. La conclusion du document source est la plus transposable qui soit :
**la majorité des paramètres sont des défauts jamais recalibrés** ; les valeurs
validées le sont sur des mesures ou des incidents. Un défaut n'est pas un
choix.

Ce que NOMOS livre : `templates/regulated/parameter-inventory.yaml`, orienté
consommateur, avec une section *silent failure review* (« si ceci cesse d'agir,
qu'est-ce qui le dit ? »). Application à NOMOS lui-même, dite sans détour : les
seuils du gate cite-or-abstain (`0.95`, `0.6`) et du harnais sont des **défauts
documentés, hérités du sidecar historique, jamais calibrés sur un corpus** ; ils
sont versionnés et publiés, ce qui est la bonne discipline, mais leur statut est
`default`, pas `validated`. Le bench public mesure le gate avec ces défauts et
le dit.

### 2.3 Frontière d'inférence à deux régimes — contrôle ajouté

Le concept : le corpus de **droit public** peut être criblé via des API cloud
pour *choisir* un modèle, parce que ces données sont déjà publiques ;
l'**inférence sur données d'affaire** reste air-gap sans exception. La frontière
est une décision d'architecture datée, pas une intention.

Ce que NOMOS livre : un contrôle requis dans la baseline de gouvernance IA/RAG
(`docs/regulated/ai-rag-governance/README.md`) — frontière d'inférence
déclarée par classe de données, flux jamais mélangés dans un même run. NOMOS
lui-même n'appelle aucun modèle : le juge NLI vit dans un sidecar, aucun run CI
ne score avec un modèle neuronal, et les connecteurs ne redistribuent que des
empreintes.

### 2.4 Concepts retenus sans artefact

- **Gate de confiance avant le LLM** (regroupement par densité, seuils haut/bas,
  révision humaine en dessous) : NOMOS gate *après* la réponse. Les deux se
  composent : `requires_human_decision` du contrat de réponse est le point
  d'accroche. Aucune mécanique ajoutée tant qu'aucun consommateur ne l'exige.
- **Dimensionnement GPU par mémoire totale** : un modèle MoE réduit le calcul,
  jamais la mémoire résidente ; la VRAM totale dimensionne les cartes ; un gain
  arithmétique n'est pas actionnable si l'exploitation (alimentation,
  garantie, support) ne suit pas. Hors périmètre NOMOS ; retenu pour lire les
  propositions matérielles d'un partenaire.
- **Réduction des appels LLM par filtrage** : mesurable, donc à demander en
  chiffre daté, jamais à recopier.

## 3. Anti-patterns appris — et où NOMOS les encode

Les documents source décrivent leurs propres incidents. Ils dessinent une
classe unique : **un mécanisme qui cesse d'agir sans le dire**. NOMOS l'encode
au principe 8 de `docs/43`. Ligne à ligne :

| Classe de défaut observée | Leçon | Où NOMOS l'encode |
|---|---|---|
| Une fonction se désactive si sa bibliothèque manque ; un `except` large avale un échec d'import et produit zéro résultat sans erreur | Toute dépendance absente ou erreur avalée est un échec nommé | Sidecars : `PyYAML` absent → exit 2 ; scorer sans backend → refus ; gate sécurité : scanner absent = check en échec ; **guard de silence** `tests/test_silence_guard.py` sur `scripts/` (un `except` qui avale sans trace est refusé, les exceptions sont nommées et justifiées) |
| Des vecteurs indexés mais jamais interrogés pendant des semaines | Un index sans consommateur est un caller de production manquant | Matrice de câblage : capacité `real` seulement avec caller de production ; kit de conformité consommateur : l'import est la preuve |
| Un cloisonnement par affaire que chaque requête doit poser | Un filtre par convention n'est pas un cloisonnement | Lens : exclusion calculée à l'export, fail-closed ; kit consommateur : lens imposée au niveau de base, jamais après coup |
| Une chaîne d'audit qui redémarre à chaque période | Une chaîne qui redémarre n'est pas une chaîne | Body ledger Merkle vérifié en attestation ; `claim_coverage` recalculée ; la leçon est portée au SOP piste d'audit avant tout export périodique |
| Des constantes qui décrivent une machine ou un modèle qui n'existent pas | Une constante non vérifiée est une déclaration périmée | Modèle de support vérifié contre la matrice CI, `go.mod` et les tags (#679) ; registre attendu vs calculé |
| Un nom de modèle jamais publié présenté comme choix définitif | Une référence se revérifie avant publication | Références du bench avec `verified_at_utc`, guard de claims « aucune reformulation » |
| Un pourcentage d'avancement transmis par une autre session, contredit par la base | Un avancement reçu se recompte | Statut de portefeuille calculé depuis les sources commitées ; aucun chiffre narré |
| Des seuils de la littérature jamais recalibrés, présentés comme choix | Un défaut n'est pas un choix | Template §2.2 ; statut `default` assumé pour les seuils du gate |
| Deux modules homonymes issus d'une migration inachevée | Une migration se termine ou se déclare | Registre de contrats et guard de compatibilité (#676) ; check générique des commandes fantômes |
| Un domaine « très avancé » en volume, à zéro en enrichissement, sans jeu doré | Un domaine sans jeu doré n'est pas exploitable | Pack de domaine avec corpus doré inclus ; harnais refusant une borne que rien ne mesure |
| Un gain annoncé sans mesure datée | Un chiffre sans mesure n'est pas un résultat | Bench public : résultats datés, rejoués en CI |

Ce que NOMOS a trouvé chez lui en appliquant la grille, et corrigé dans le
même lot : une preuve de fidélité qui **sautait en silence** un fichier de feed
illisible (le fichier devient un finding bloquant) ; le guard de claims qui
**ne scannait pas** un fichier illisible sans le dire (c'est désormais une
violation) ; le guard de chemins de publication qui **supposait sûr** un
candidat qu'il ne pouvait pas inspecter (il le refuse) ; l'export d'audit qui
**réinitialisait** un manifeste corrompu au lieu de s'arrêter (il refuse, la
chaîne d'exports n'est jamais effacée en silence) ; le scan no-full-text qui
**ignorait** un fichier non lisible (il le signale comme non couvert) ; le
parseur de commentaires GitHub qui rendait zéro commentaire quand rien n'était
parsable (il lève). Deux sites avalent encore des erreurs, dans un outil
d'orchestration d'agents et un client de démo, hors chemin de preuve ; ils sont
listés et justifiés dans l'allowlist du guard, pas cachés, et l'allowlist
elle-même est vérifiée : une exemption devenue inutile est refusée.

## 4. Préparer la collaboration avec l'auteur

### 4.1 Vérifier avant de s'appuyer

Le principe des deux documents s'applique à eux-mêmes. Avant toute couture :

1. recompter dans les bases ce que les documents affirment (volumes par
   couche, écarts déjà signalés par l'auteur) — la cartographie §2.1 est le
   format de ce recomptage ;
2. obtenir la revue de sécurité **fraîche** que le mémoire recommande lui-même
   avant toute affirmation de conformité ; aucune evidence NOMOS n'est reliée à
   un système dont la frontière d'autorisation n'a pas été revérifiée ;
3. confirmer par écrit la frontière d'inférence (§2.3) et le fait qu'aucune
   donnée d'affaire, de témoin ou privilégiée ne transite vers NOMOS, même en
   test ;
4. confirmer les conditions de licence des corpus publics utilisés (bulk
   officiel, API de jurisprudence, portails cantonaux) et leur compatibilité
   avec la redistribution hash-only de NOMOS ;
5. vérifier la citation des travaux de recherche invoqués (HyDE, HyPE, CRAG…)
   contre leurs versions datées, comme pour le bench (`references.yaml`).

### 4.2 Questions à poser

1. Comment les trois validations de réponse annoncées — citations, confiance,
   contradiction de version — sont-elles implémentées, mesurées, et sur quel
   jeu de questions ?
2. Existe-t-il un jeu doré par domaine, avec des seuils versionnés ? Sinon,
   qui le construit et qui l'annote ?
3. Quelle est la source canonique d'un texte quand plusieurs versions
   consolidées coexistent, et comment une réponse cite-t-elle la version en
   vigueur à une date ?
4. Le cloisonnement par affaire est-il imposé par le modèle de données ou par
   les requêtes ? Quel test le prouve ?
5. La piste d'audit est-elle continue entre périodes, et qu'est-ce qui détecte
   une suppression ?
6. Quel mécanisme dit qu'une fonction s'est désactivée (dépendance absente,
   index non interrogé, relation non écrite) ? Quels sont les signaux
   d'exploitation existants ?
7. Quels paramètres sont validés, sur quelle mesure datée, et lesquels sont
   des défauts ? L'inventaire §2.2 est le format proposé.
8. Quel est le critère de qualité de la phase 1 et sa dernière mesure datée ?
9. Comment la provenance par chunk est-elle rapprochée du document source
   après une réingestion ? Qu'est-ce qui prouve qu'un index est périmé ?
10. Quelle est la politique de rétention et de suppression des données
    d'affaire, y compris dans les sauvegardes et les snapshots ?
11. Qui opère la plateforme, avec quelle astreinte, et quel est le modèle de
    support déclaré au cabinet ?
12. Quelles attentes de propriété intellectuelle et de licence pour une
    collaboration ? NOMOS n'est pas sous licence open source (`GOVERNANCE.md`,
    README) : tout partage de code exige un accord écrit préalable.
13. La discipline documentaire de l'auteur (mémoire cumulative, décisions
    datées) peut-elle être partagée en lecture ? Elle est compatible avec les
    ADR et le registre de NOMOS.

### 4.3 Interfaces proposées

| Interface | Côté NOMOS | Côté voisin | Preuve |
|---|---|---|---|
| Intake de textes publics | connecteurs suisses hash-only, atomes par structure, résolveur point-in-time | ingestion de bundles au lieu de PDF bruts, provenance conservée | `nomos rag verify` vert après réingestion |
| Export vers l'index | `nomos rag export` (`jsonl`, `langchain`, `llamaindex`), manifeste par source | chargeur vers le magasin vectoriel, `chunk_id` et `source_hash` conservés | kit de conformité consommateur |
| Verdict de réponse | `nomos answer gate` sur enregistrements de réponse | émission d'enregistrements au contrat de réponse (citations, incertitudes, décision humaine) | bench rejoué sur leurs réponses, résultats datés |
| Jeu doré | harnais `answer eval`, seuils versionnés | corpus doré annoté par le domaine | régression rouge en CI |
| Cartographie | contrat §2.1 | tableau de complétude au format du contrat | `cue vet` |
| Inventaire de paramètres | template §2.2 | inventaire rempli | revue de silence |

### 4.4 Lignes rouges

- NOMOS ne reçoit jamais de données d'affaire, de témoin ou privilégiées. La
  preuve croisée se fait sur **corpus publics** uniquement — le droit fédéral
  suisse de la construction est le recouvrement naturel : NOMOS a déjà le
  connecteur Fedlex ELI et le pack built-environment.
- Aucune affirmation « sécurisé », « certifié », « validé » d'un côté ni de
  l'autre ; le guard de claims s'applique à ce qui est écrit dans ce dépôt.
- Aucun code ni contenu du système voisin n'entre dans NOMOS ; les documents
  restent privés ; ce doc reste anonyme sur les chiffres et les constats.
- Un résultat de pipeline du partenaire est traité comme un input à recompter,
  jamais comme une preuve NOMOS.

### 4.5 Première étape conjointe proposée

Une **preuve de consommation croisée** sur le modèle de VRC-37 (Aedifica) :
NOMOS produit un bundle canonique des textes fédéraux suisses de la
construction, le voisin l'ingère en conservant identités et empreintes,
répond à un jeu de questions annoté, et les réponses passent `nomos answer
gate` ; la mesure est publiée datée, avec la cartographie §2.1 du domaine et
l'inventaire §2.2 des paramètres du chemin exercé. Dispatch : `external`
(#701, NRT-030) pour la preuve conjointe, qui attend le partenaire ; la moitié
NOMOS — bundle, cartographie, inventaire, jeu de questions, kit consommateur —
est `autonomous` (#702, NRT-029) et se livre sans lui. Rien n'est promis au
partenaire avant les préalables de §4.1.

### 4.6 Registre des risques

| Risque | Effet | Mitigation |
|---|---|---|
| Revue de sécurité non rejouée depuis plusieurs semaines chez le partenaire | evidence NOMOS reliée à un système à frontière incertaine | §4.1 point 2 avant toute couture |
| Opérateur unique, discipline forte mais non répartie | dépendance à une personne | demander le modèle de support et l'astreinte (§4.2 Q11) |
| Licence NOMOS propriétaire, IP du partenaire privée | blocage juridique tardif | accord écrit avant tout partage (§4.2 Q12) |
| Choix du modèle de production rouvert, matériel non acheté | calendrier du partenaire incertain | la preuve croisée ne dépend d'aucun modèle : gate lexical et bundle suffisent |
| Corpus sous conditions d'usage | redistribution interdite | hash-only côté NOMOS, textes chez le partenaire |
| Chiffres transmis en cours de route | recopie d'un état faux | tout chiffre est recompté (§4.1 point 1) |

## 5. Décisions et suites

Livré dans ce lot : le contrat de cartographie et ses fixtures en CI, le
template d'inventaire, le contrôle de frontière d'inférence, le principe 8 de
la doctrine, le guard de silence et le correctif de la preuve de fidélité, ce
doc. Aucune capacité `real` n'est revendiquée : un contrat sans lecteur est un
`sidecar`.

Décidé le 2026-09-06 (décision déléguée à l'agent par le propriétaire) : la
collaboration est ouverte en deux issues — #702 (NRT-029, `autonomous`) livre
la moitié NOMOS de la preuve croisée de §4.5 sans dépendre du partenaire ;
#701 (NRT-030, `external`) porte la preuve conjointe, ses préalables de §4.1 et
ses lignes rouges de §4.4, et ne bloque que sa propre claim. L'entrée du domaine
juridique dans la matrice de pilotes reste `blocked` tant que #701 n'a pas
livré une mesure datée : un périmètre client ne se déclare pas avant la preuve.

#702 est livré (`docs/50`) : le kit se rejoue en CI contre le corpus doré du
pack — bundle attesté, export et manifeste, lentille fail-closed, `rag verify`
vert puis rouge, preuve d'import du consommateur (digest de l'index recalculé
depuis ses propres enregistrements, chunk altéré refusé), réponses citées ou
abstenues avec chaque citation recoupée au manifeste, jeu doré sous ses seuils,
inventaire de paramètres vérifié (huit défauts nommés) et cartographie de
domaine recomptée. Les textes fédéraux restent des reçus hash-only. Ce que le
kit ne prouve pas est dit dans `docs/50` §4.2 ; la preuve conjointe reste #701.

Ce que NOMOS retient pour lui, au-delà du partenaire : la question de §2.2 —
« si ceci cesse d'agir, qu'est-ce qui le dit ? » — vaut pour chaque mécanique
du dépôt, et la réponse « rien » est un finding.

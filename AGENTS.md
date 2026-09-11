<!--
Modèle à copier à la racine d'un dépôt sous le nom AGENTS.md, puis à adapter.
Ce fichier est lu par les agents qui travaillent sur le dépôt (Claude Code, Codex,
Gemini CLI et les workers ORDO). Il ne remplace pas le README : le README explique
le projet à un humain, AGENTS.md explique comment y travailler sans casser.

Règle de rédaction : n'y mettre que ce qui n'est pas déductible du code. Une
consigne que l'agent aurait devinée seule occupe de la place et dilue le reste.
-->

# Consignes agent — <nom du dépôt>

## Ce qu'est ce dépôt

<!-- Deux phrases. Rôle, et ce dont il est responsable en production s'il l'est. -->

## Commandes

| But | Commande |
|---|---|
| Installer | |
| Tester | |
| Lint / format | |
| Lancer en local | |

<!-- Si une commande est longue ou coûteuse, le dire, et dire ce qu'on lance à la place en boucle courte. -->

## Vérification avant toute PR

<!--
La ou les commandes qui doivent passer. Si le dépôt n'a pas de test, écrivez-le
franchement et indiquez la vérification manuelle attendue — un agent qui ne trouve
pas de test en invente un usage plausible et se trompe.
-->

## Interdits

- Ne jamais modifier : <!-- chemins de secrets, fichiers générés, migrations déjà appliquées -->
- Ne jamais exécuter : <!-- commandes destructrices, déploiements, migrations de base -->
- Ne jamais valider de secret en clair, même dans un fichier d'exemple.

## Règles d'arrêt

Un agent s'arrête et ouvre une demande d'arbitrage (`type:decision` +
`status:blocked`) plutôt que de décider seul, dans ces cas :

- il manque un accès, un jeton ou une permission → `status:blocked`, en disant en
  commentaire ce qui manque exactement ;
- deux consignes se contredisent ;
- l'action serait irréversible ou toucherait la production ;
- le correctif nécessite de sortir du périmètre déclaré dans le ticket.

Le contournement d'un blocage d'accès est lui-même un incident : ne pas le tenter.

## Atomisation — ne jamais rapporter « non actionable »

Un ticket ne porte qu'un seul verdict d'actionnabilité. Quand un agent constate qu'un
ticket n'est pas exécutable en l'état, la réponse attendue n'est pas « non actionable » :
c'est le découpage.

- créer un enfant par action immédiatement exécutable avec résultat attendu,
  périmètre et sections Markdown visibles, uniques et non vides
  `## Critères d’acceptation`, `## Vérification`, `## Parent de suivi`,
  `## Hors périmètre bloqué` et `## Autorisation de prise` ; commentaires HTML
  et blocs de code ne comptent pas ;
- écrire dans `## Parent de suivi` soit `#N`, soit exactement
  `Aucun — ticket autonome.` ; `#N` impose la structure d’enfant et la dépendance
  native, sans phrase magique facultative ;
- créer l'enfant en `status:ready` sans lui ajouter soi-même
  `agent:cli-ready` : seul un humain peut poser cette autorisation ; si la variable
  Actions `COMPTES_AGENTS` manque, le contrôle échoue fermé et révoque le label ;
  chaque compte doit correspondre à `[A-Za-z0-9][A-Za-z0-9._-]*`, sinon la liste
  est mal formée et échoue fermée ; avant de commencer, vérifier que le contrôle
  d'attribution est réussi et que `agent:cli-ready` n'a pas été révoqué ;
- conserver dans le parent les opérations bloquées avec les sections visibles,
  uniques et non vides `## Cause du blocage`, `## Responsable capable de lever`
  et `## Condition de reprise` ;
- lier chaque enfant comme dépendance native du parent ;
- passer le parent en `status:blocked` ou `status:waiting` et lui retirer
  `agent:cli-ready`.

Retirer `agent:cli-ready` ou écrire « non actionable » sans créer les enfants
exécutables n'est pas une atomisation. Avant d'affirmer qu'aucune tranche ne
peut avancer, inventorier séparément préparation, documentation, tests,
implémentation, Ops, décisions humaines, déploiement et UAT.

Un ticket portant `agent:cli-ready` **et** `status:waiting` ou `status:blocked` est une
contradiction : il mélange de l'exécutable et du bloqué. C'est le signal d'atomisation,
et il est refusé automatiquement.

Rapporter « une seule issue sur sept est actionable » décrit un défaut de découpage, pas
un état du monde : les six autres contiennent presque toujours une part exécutable
aujourd'hui.

## Conventions locales

<!--
Style de commit, nommage de branche, découpage attendu des PR, particularités
qui surprendraient (ex. « le dossier vendor/ est commité volontairement »).
-->

## Pièges connus

<!--
Les endroits où le code ment sur ses intentions. C'est la section la plus utile
du fichier ; elle se remplit au fil des erreurs réellement commises.
-->

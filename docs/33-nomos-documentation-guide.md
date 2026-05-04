# 33 - Nomos Documentation Generale

Date: 2026-05-04
Statut: reference d'integration alpha
Scope: NOMOS v0.1.x, GitHub workflow integration, corpus read-only, downstream application integration

## But

Ce document donne la vue generale de NOMOS pour une equipe produit,
technique ou qualite qui veut comprendre ce que NOMOS fait, quels
artefacts il genere, comment les verifier, et quelles limites de claim
doivent rester explicites.

Il complete:

- [`README.md`](../README.md), positionnement public du produit;
- [`01-method-overview.md`](01-method-overview.md), methode canonical-first;
- [`31-github-workflow-setup.md`](31-github-workflow-setup.md), installation GitHub Actions;
- [`public-claim-boundary.md`](public-claim-boundary.md), limites de claims publics;
- [`rbok-poc-validation-dossier.md`](rbok-poc-validation-dossier.md), preuve POC RBOK.

## Definition Courte

NOMOS transforme un corpus d'autorite en artefacts produit controles:

```text
source d'autorite
-> lecture seule
-> scan et admission
-> structure documentaire
-> unites canoniques
-> lexique, TOC, ledgers, metadata RAG
-> gates de fidelite
-> attestations et traces
-> bundle consommable par un logiciel ou une IA
```

NOMOS ne remplace pas l'autorite metier, juridique ou qualite. Il rend
la transformation de cette autorite explicite, tracable, verifiable et
gouvernable.

## Audiences

| Audience | Besoin principal |
|---|---|
| Mainteneur de corpus | Savoir quels dossiers sont lus, ce qui est publie, et comment eviter toute mutation de la source. |
| Developpeur application | Savoir quels artefacts consommer, comment les importer, et quoi ne pas traiter comme source d'autorite. |
| Reviewer qualite | Verifier la preuve, les limitations, les warnings, les hashes et la claim boundary. |
| Equipe produit | Comprendre la valeur: aligner logiciel, IA, RAG et documentation metier sur une reference gouvernee. |
| Operateur GitHub | Installer le workflow, regler les secrets, choisir output repo, PR ou direct push selon le risque. |

## Contrat Fondamental

NOMOS doit respecter cinq invariants:

1. La source d'autorite est lue en lecture seule.
2. Tout artefact derive porte provenance, hash, source path et statut.
3. Les chunks RAG sont downstream des unites canoniques, jamais
   autorite primaire.
4. Les gaps et unsupported records doivent etre explicites.
5. Les claims publics ne depassent jamais la preuve disponible.

## Artefacts Principaux

| Artefact | Role | Consommateur typique |
|---|---|---|
| `rbok-lawbook-feed.json` ou feed equivalent | Corps canonique structure, avec texte et nodes. | Importeur runtime, audit source-to-feed. |
| `rbok-rag-metadata.json` ou metadata RAG equivalent | Metadata de retrieval, provenance, domaines, priorites, parent chains. | RAG retriever, moteur conversationnel. |
| `rbok-engine-import.json` ou import runtime equivalent | Projection structurelle pour application downstream. | Importeur application, tables doctrine. |
| `rbok-certified-toc.json` | Table des matieres reconstruite et certifiee. | UI de navigation, audit structure. |
| `rbok-lawbook-index.json` | Index de nodes par identite, type, profondeur ou racine. | Recherche, debug, import incremental. |
| `rbok-governed-lexicon.yaml` | Lexique gouverne et termes detectes. | Gouvernance metier, QA, prompt guardrails. |
| `corpus-body-ledger.json` | Ledger complet du corps source, y compris contenu non-RAG. | Audit fidelite, source integrity. |
| `semantic-quality-report.json` | Qualite semantique des unites et chunks. | Gate de release, reviewer qualite. |
| `short-critical-atoms.json` | Disposition des fragments courts critiques. | Gate v0.2+, audit de pertes potentielles. |
| `nomos-trace.json` / `nomos-trace.yaml` | Trace GitHub obligatoire d'un run. | Reviewer PR, downstream importer, evidence pack. |
| `attestation.json` | Attestation de build et claims supportes. | Release manager, compliance reviewer. |

Important: certains artefacts ne contiennent pas le texte complet. Un
integrateur ne doit pas supposer que `rag_metadata` ou `engine_import`
suffisent a alimenter un RAG. Le texte source exploitable doit venir du
feed canonique ou d'un artefact explicitement declare comme content
bundle.

## Statut Alpha Actuel

NOMOS v0.1.x a prouve une chaine source-to-feed sur un vrai corpus
prive (`realisons-business/01_rbok`) en lecture seule.

Ce qui est prouve pour le POC enregistre:

- scan de corpus reel;
- generation d'un pack lawbook;
- source spans sur les nodes;
- TOC certifiee;
- lexique gouverne;
- metadata RAG source-backed;
- strict fidelity gate passant;
- absence de mutation du repository source.

Ce qui n'est pas encore un claim universel:

- fidelite parfaite pour tous les formats documentaires;
- validation reglementaire client;
- comportement runtime RAG dans toutes les applications downstream;
- extraction structuree complete de tous les YAML/JSON metier;
- certification Part 11, GxP, Annex 11 ou equivalent.

## Relation A Une Application Downstream

Une application downstream ne doit pas modifier le corpus source. Elle
doit consommer un output NOMOS versionne:

```text
source repo read-only
-> NOMOS workflow
-> output repo or output path
-> application importer
-> doctrine tables / vector store / prompt context
-> runtime trace
```

L'application downstream peut:

- importer un feed comme projection read-only;
- stocker plusieurs versions de feed;
- activer une version apres validation;
- lier des modules, questions ou parcours a des units/chunks NOMOS;
- produire une trace runtime montrant quelle doctrine a informe une
  reponse.

L'application downstream ne doit pas:

- traiter un chunk RAG comme source canonique;
- reecrire le corpus source;
- cacher les warnings d'import;
- activer un feed si les gates obligatoires ont echoue;
- annoncer une compliance que NOMOS ou le client n'ont pas prouvee.

## Modes De Publication

| Mode | Usage recommande |
|---|---|
| `artifact_only` | Preview PR, experimentation, aucun commit genere. |
| `pull_request` | Mode par defaut pour corpus ou outputs critiques. |
| `direct_push` | Uniquement quand le risque est bas ou qu'une decision controlee est documentee. |

Pour un POC applicatif, `pull_request` vers une branche de dev est le
meilleur compromis. Pour une production fluide, `direct_push` peut etre
autorise par scope, path guard, trace manifest, anti-loop marker et
decision risk-based.

## Definition De Pret Pour Integration

Un output NOMOS est pret a etre propose a une application downstream si:

- le corpus source est propre avant et apres run;
- le workflow a produit un trace manifest;
- le strict gate est `pass`;
- le feed contient le texte attendu ou un content bundle equivalent;
- les metadata RAG referencent des units existantes;
- les sources non atomisees ont un statut explicite;
- les warnings restants sont acceptes, justifies ou convertis en issues;
- la version du feed est immuable et auditable.

## Definition De Pret Pour Runtime

Une application downstream est prete a utiliser NOMOS en runtime si:

- elle importe le vrai format NOMOS courant;
- elle stocke feed version, source, units, chunks, locators, metadata et imports;
- elle peut activer/desactiver une version atomiquement;
- elle sait tracer une reponse vers les chunks et sources utilises;
- elle a des tests de non-regression pour prompt, retrieval, citations et refusal;
- elle separe doctrine canonique, commentary, practice et reference;
- elle bloque ou degrade proprement quand aucun feed actif n'existe.

## Recommandation D'organisation

Pour la premiere integration RBOK, le POC peut publier vers
`RBOKproject/RBOK` sur `develop` afin de valider rapidement le flux.

La cible durable devrait etre:

```text
RBOKproject/realisons-business
  source d'autorite, read-only

RBOKproject/nomos-corpus-realisons-business
  outputs NOMOS versionnes

RBOKproject/RBOK
  application downstream, importeur et runtime
```

Cette separation evite que l'application, le corpus et les outputs
generes se polluent mutuellement.

## Limites A Maintenir Dans Les Communications

Formulations autorisees:

- "NOMOS produit des artefacts source-backed et auditables."
- "NOMOS traite le corpus source en lecture seule."
- "Le POC RBOK a produit un pack exploitable avec strict gate passant."
- "Les claims restent limites au scope et au commit prouves."

Formulations interdites sans preuve supplementaire:

- "NOMOS valide n'importe quel corpus."
- "NOMOS rend une application GxP compliant."
- "NOMOS certifie la conformite Part 11."
- "NOMOS garantit qu'une IA repond toujours correctement."

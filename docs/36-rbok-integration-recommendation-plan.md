# 36 - Plan Recommandations RBOK Depuis NOMOS

Date: 2026-05-04
Statut: plan d'execution downstream
Repo downstream cible: `RBOKproject/RBOK`
Branche POC demandee: `develop`

## But

Ce plan transforme le diagnostic NOMOS -> RBOK en travail actionnable.
Il ne modifie pas le code RBOK. L'implementation doit etre faite cote
RBOK, dans le repo downstream `RBOKproject/RBOK`.

## Constat Technique

Le run NOMOS actuel sur `realisons-business/01_rbok` produit un pack
source-to-feed valide pour un POC NOMOS borne. Le record historique a
prouve la chaine lawbook complete; le record structure actuel resserre
la qualite semantique du feed.

Points verifies:

- corpus source read-only;
- 240 sources corpus declarees dans le dossier de preuve;
- record historique: 7191 nodes lawbook, TOC, lexique, feed, metadata
  RAG, engine import et attestation produits;
- record structure actuel: 3024 feed units, 3024 RAG chunks, 3024/3024
  units et chunks source-backed;
- zero body-ledger bytes non couverts dans le record structure;
- zero fragments courts `<= 10` caracteres dans le feed structure;
- strict gate passant sur le scope enregistre.

## Snapshot Audit RBOK Develop

Snapshot d'audit repris pour cadrer le POC downstream:

- repo: `RBOKproject/RBOK`;
- branche: `develop`;
- commit local audite: `9b4f3db4`;
- worktree local audite: propre;
- CI observee: derniere `Deploy DEV` verte;
- PR ouvertes vers `develop` au moment de l'audit: aucune.

Elements RBOK deja presents:

| Surface | Fichier downstream observe |
|---|---|
| Modeles doctrine NOMOS | `backend/app/models/nomos_doctrine.py` |
| Retrieval doctrine/RAG | `backend/app/services/doctrine_rag_retrieval.py` |
| Bindings step -> doctrine | `backend/app/models/step_doctrine_binding.py` |
| Guard anti-edition canonique in-app | `backend/app/api/v1/rbok.py` |

Bloquants observes:

| Bloquant | Fichier downstream observe | Impact |
|---|---|---|
| Importeur NOMOS ancien format plat (`source_hash`, `units`, `chunks`) | `backend/app/services/nomos_importer.py` | Incompatible avec le feed NOMOS courant. |
| Prompt runtime pas encore branche sur `DoctrineRAGRetriever` | `backend/app/services/module_prompt_builder.py` | La conversation utilise encore RAG generique + references legacy. |
| Admin traceability reference des champs non alignes (`unit_count`, `chunk_count`, `import_id`, `unit_key`) | `backend/app/api/v1/admin/doctrine_traceability.py` | Les vues/admin tests ne prouvent pas le format reel. |

Tests downstream observes:

| Test | Etat observe |
|---|---|
| `tests/test_doctrine_retrieval.py --no-cov` | passe |
| `tests/test_doctrine_binding.py --no-cov` | passe |
| `tests/test_nomos_importer_service.py --no-cov` | echoue |
| `tests/test_doctrine_traceability.py --no-cov` | echoue |

Tickets downstream existants:

| Ticket | Etat a requalifier |
|---|---|
| `RBOK#2168` | Epic Doctrine Runtime v2 ouvert. |
| `RBOK#2700` | Epic import NOMOS feed ouvert. |
| `RBOK#2701` | Epic conversation engine NOMOS RAG ouvert. |
| `RBOK#2702`, `#2703`, `#2704`, `#2706`, `#2707`, `#2708`, `#2710`, `#2711` | fermes, mais `#2704`/`#2711` ne doivent pas etre consideres valides tant que les tests importer/traceability echouent. |

Gaps downstream RBOK:

- l'importeur RBOK actuel attend un format ancien et ne sait pas
  importer le feed NOMOS courant;
- `rbok-rag-metadata.json` ne contient pas le texte;
- `rbok-engine-import.json` ne contient pas le corps complet du texte;
- le texte exploitable est dans `rbok-lawbook-feed.json` sous
  `feeds[].nodes[]`;
- les tests doctrine/importer sont exclus de la suite pytest;
- certains tests forces echouent;
- le prompt runtime appelle encore le RAG generique et les references
  RBOK legacy, pas le retriever doctrine NOMOS comme source principale;
- les YAML/JSON parcours/modules/questions ne sont pas encore couverts
  comme objets runtime par le pack lawbook actuel.

## Principe D'integration

RBOK doit consommer NOMOS comme projection read-only:

```text
realisons-business/main
  source de verite metier

NOMOS
  transformation, gates, trace, output bundle

RBOK/develop
  import POC et activation runtime dev

RBOK production
  seulement apres validation downstream
```

## Dependency Tree

```text
A. Output bundle contract
  -> B. RBOK importer real format
    -> C. Import dry-run and activation gate
      -> D. Doctrine runtime retrieval
        -> E. Prompt behavior and step_question_only proof
          -> F. YAML/JSON parcours/module/question extraction
            -> G. Admin traceability and evidence views
              -> H. Dev POC validation dossier
```

## Workstream A - Output Bundle Contract

Objectif: figer ce que RBOK doit recevoir de NOMOS.

Deliverables:

- liste obligatoire des artefacts;
- validation du bundle;
- refus si `feed` absent ou si metadata RAG orphelines;
- documentation de mapping feed -> tables RBOK.

DoD:

- un fichier de test bundle avec feed + rag metadata + trace est valide;
- un bundle sans texte exploitable echoue;
- un bundle sans trace manifest echoue;
- un reviewer peut identifier le commit source et le hash output.

## Workstream B - Importeur RBOK Format NOMOS Courant

Objectif: remplacer l'importeur scaffold par un importeur du format reel.

Changements requis cote RBOK:

- lire `rbok-lawbook-feed.json`;
- parcourir `feeds[].nodes[]`;
- stocker `text`, `source_path`, `source_hash`, `span`,
  `canonical_ref`, `display_ref`, `parent_chain`;
- joindre `rbok-rag-metadata.json` par `node_id`/`chunk_id`;
- importer `engine_import` uniquement comme projection structurelle;
- creer une version de feed immutable;
- activer atomiquement apres validation.

DoD:

- import dry-run sur le pack NOMOS reel;
- import DB sur SQLite/Postgres test;
- idempotence par artifact hash/source commit;
- rollback possible vers feed precedent;
- tests remis dans la suite CI.

## Workstream C - Tests Et Gate Downstream

Objectif: ne plus fermer des tickets sur du scaffold non teste.

Actions:

- retirer l'exclusion des tests doctrine quand ils sont alignes;
- reparer `test_nomos_importer_service.py`;
- reparer `test_doctrine_traceability.py`;
- ajouter test fixture sur le pack NOMOS reel;
- ajouter test "metadata sans texte ne suffit pas";
- ajouter test "YAML parcours non couvert = warning bloquant ou statut explicite".

DoD:

- `pytest tests/test_nomos_importer_service.py --no-cov` passe;
- `pytest tests/test_doctrine_traceability.py --no-cov` passe;
- les tests doctrine ne sont plus ignores silencieusement;
- CI `develop` reste verte.

## Workstream D - Runtime Doctrine RAG

Objectif: brancher la doctrine NOMOS dans le moteur conversationnel.

Actions:

- appeler `DoctrineRAGRetriever` depuis le prompt builder;
- prioriser doctrine NOMOS sur RAG generique quand un feed actif existe;
- passer `module_id`, `question_key`, `query_hint`;
- inclure citations compactes dans le prompt;
- tracer chunk ids, source paths et feed version dans la reponse runtime.

DoD:

- un module avec binding recupere les chunks lies;
- un module sans binding degrade proprement;
- supporting-only ne devient pas doctrine primaire;
- le prompt final respecte `step_question_only`;
- la reponse runtime peut etre auditee.

## Workstream E - Comportement IA Conversationnel

Objectif: eviter la verbosite et les questions supplementaires.

Regle metier:

- une seule question par message;
- la question doit etre celle du step courant;
- pas de sous-questions;
- pas de liste de questions;
- ton concis, precis, bienveillant;
- doctrine NOMOS en support, pas en monologue.

DoD:

- tests de prompt avec une seule question;
- tests de refusal quand le modele tente d'ajouter une question;
- tests Infomaniak provider si active dans l'environnement;
- scenario complet parcours -> module -> question -> trace doctrine.

## Workstream F - YAML/JSON Parcours, Modules Et Questions

Objectif: couvrir le contenu structure metier, pas seulement le lawbook
Markdown.

Actions NOMOS ou downstream a planifier:

- produire un artefact `rbok-parcours-feed.json` ou equivalent;
- atomiser YAML/JSON par path structurel;
- conserver raw value, decoded value, key path, source span;
- differencier definition de parcours, module, objectif, question,
  validation rule, behavior config;
- relier ces objets aux units doctrinales lawbook.

DoD:

- les fichiers YAML/JSON actifs ne disparaissent pas silencieusement;
- chaque parcours/module/question a une identite stable;
- chaque question peut etre liee a une doctrine source-backed;
- le POC declare explicitement ce qui reste hors scope.

## Workstream G - Admin Traceability

Objectif: rendre visible la chaine doctrine dans l'application.

Vues requises:

- feed actif;
- imports et verdicts;
- sources admises/rejetees;
- units et chunks;
- warnings;
- trace step -> chunk -> source;
- rollback feed.

DoD:

- endpoints admin alignes sur les vrais modeles DB;
- pas de champs inexistants dans les schemas;
- UI ou endpoint read-only pour inspecter la version active;
- remediation hints relies aux vrais erreurs d'import.

## Workstream H - POC Develop

Objectif: valider sans toucher production.

Plan:

1. Generer outputs NOMOS depuis `realisons-business/main`.
2. Publier en PR ou path controle vers `RBOK/develop`.
3. Import dry-run.
4. Import DB dev.
5. Activer feed dev.
6. Tester parcours conversationnel.
7. Verifier trace doctrine.
8. Documenter warnings.
9. Decider si POC valide.

DoD:

- aucun commit direct sur `develop`;
- aucune mutation de `realisons-business`;
- CI verte;
- trace complete;
- dossier POC lisible par business et tech;
- decision explicite: go, conditional go, ou blocked.

## Tickets RBOK Crees

Deux tickets downstream sont requis:

1. [`RBOK#2895`](https://github.com/RBOKproject/RBOK/issues/2895) -
   Integration NOMOS -> RBOK sur `develop`.
2. [`RBOK#2896`](https://github.com/RBOKproject/RBOK/issues/2896) -
   Recommandations RBOK pour doctrine runtime, RAG, YAML/JSON, tests et
   traceability.

Ces tickets doivent rester dans `RBOKproject/RBOK`. Le code RBOK ne doit
pas etre modifie depuis le repo NOMOS.

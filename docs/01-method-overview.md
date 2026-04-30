# 01 - Vue D'ensemble De La Méthode

## Objectif

Canonical-First est une méthode d'ingénierie pour les projets où la vérité produit vient d'un corpus métier. Le corpus peut être juridique, médical, technique, ludique, contractuel, scientifique ou organisationnel. L'implémentation ne doit pas seulement "ressembler" à ce corpus ; elle doit prouver comment chaque comportement, donnée et exception dérive de lui.

L'objectif est double :

- empêcher les dérives silencieuses : données inventées, règles simplifiées, exceptions perdues, UI qui contourne le moteur ;
- rendre les agents IA utiles sans leur donner l'autorité de décider le métier.

## Problème Visé

Les projets métier complexes échouent rarement parce qu'un routeur HTTP est mal choisi. Ils échouent parce que la règle appliquée dans le produit n'est plus la règle validée.

Exemples typiques :

- une interface affiche une liste de démonstration alors qu'un catalogue complet existe ;
- un calcul métier est réécrit dans le frontend pour "aller plus vite" ;
- un LLM donne une réponse plausible sans citation exploitable ;
- une exception réglementaire est codée dans un `if` sans justification ;
- un fichier legacy est ignoré car personne ne sait s'il est encore utile ;
- une règle est présente dans la documentation mais absente des tests ;
- la base vectorielle contient un texte, mais la donnée structurée ne l'exploite pas.

Canonical-First force la question suivante : quelle est la chaîne de preuve entre la source et le comportement observé ?

## Vocabulaire

| Terme | Définition |
|---|---|
| Source | Document ou système d'origine : PDF, Markdown, table, HTML scrapé, code legacy, contrat, norme, loi, ticket de décision. |
| Source active | Source qui doit être prise en compte par le produit. |
| Source hors scope | Source connue mais explicitement exclue, avec raison. |
| Unité atomique | Plus petite unité métier traçable : règle, clause, entrée de catalogue, exception, formule, sort, acte, garantie. |
| Contrat canonique | Représentation structurée et versionnée d'une unité ou collection, souvent YAML/JSON. |
| Schéma | Validation machine du contrat : Zod, Pydantic, JSON Schema, protobuf, Avro, etc. |
| Read-model | Modèle relationnel ou document optimisé pour lecture produit, dérivé du contrat. |
| Vector store | Index textuel sémantique du corpus avec metadata de provenance. |
| Core déterministe | Bibliothèque pure qui applique les règles sans dépendre de l'UI, DB, HTTP ou LLM. |
| Golden case | Cas métier de référence, validé par expert, utilisé comme test de bout en bout. |
| Écart | Différence connue entre source, contrat, code ou produit, avec statut et décision. |

## Chaîne D'autorité

La chaîne standard est :

```text
source
  -> unit
  -> canonical contract
  -> schema
  -> read-model
  -> deterministic core
  -> API
  -> UI
  -> tests
```

La base vectorielle est alimentée par les sources et reliée aux unités par metadata :

```text
source
  -> chunks + embeddings + metadata
  -> RAG citation/explanation
```

Le vector store n'est pas une couche d'autorité. Il ne corrige pas le read-model, ne surcharge pas le contrat et ne décide pas une règle. Il rend le contexte retrouvable et citable.

## Niveaux De Maturité

### Niveau 0 - Prototype non canonique

Le produit fonctionne avec des fixtures ou règles codées localement. Acceptable uniquement pour explorer une UX ou un flux technique. Interdit en production métier.

### Niveau 1 - Sources inventoriées

Toutes les sources connues ont une entrée dans le manifest. Les sources manquantes, doublons et hors scope sont visibles.

### Niveau 2 - Unités atomisées

Les règles et objets métier ont des IDs stables. Une première matrice existe. Les ambiguïtés sont documentées.

### Niveau 3 - Contrats canoniques

Les catalogues et règles structurables existent en YAML/JSON validé. Les fixtures produit sont supprimées.

### Niveau 4 - Produit connecté

API, core et UI consomment les read-models issus des contrats. Les tests couvrent les règles critiques et golden cases.

### Niveau 5 - Gouvernance continue

Toute modification de source, contrat, schéma ou comportement déclenche une analyse d'impact. Les releases exigent une gate stricte.

## Critères De Succès

Un projet est réellement Canonical-First si :

- chaque source active est hashée, typée, priorisée et statutaire ;
- chaque unité atomique a un ID stable et des références sources exactes ;
- chaque donnée produit vient d'un contrat validé ;
- chaque contrat a un schéma ;
- chaque unité critique a au moins un test ;
- chaque chunk vectoriel porte `source_id`, `source_hash`, `unit_ids`, `domain`, `version` ;
- aucune surface produit ne dépend de données inventées ;
- les décisions métier non triviales ont un ADR ou un decision record ;
- les experts peuvent auditer le chemin source -> UI sans lire tout le code.


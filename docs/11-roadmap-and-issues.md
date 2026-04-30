# 11 - Roadmap Et Issue List Générique

## Roadmap Type

### v0.1 - Corpus Et Sources

Objectif : rendre le corpus visible.

Livrables :

- `source-manifest.yaml`;
- hash de toutes les sources ;
- premiers statuts ;
- propriétaires ;
- politique licence/confidentialité ;
- glossaire initial.

Gate : toutes les sources connues ont une entrée ou une justification.

### v0.2 - Atomisation

Objectif : transformer le corpus en unités traçables.

Livrables :

- `canonical-matrix.yaml`;
- une ligne par règle/objet/exception critique ;
- IDs stables ;
- ambiguïtés ouvertes ;
- premiers golden cases.

Gate : aucune source primaire active sans unité ou justification.

### v0.3 - Contrats Produit

Objectif : remplacer les données inventées par des contrats.

Livrables :

- contrats YAML/JSON ;
- schémas ;
- loaders ;
- read-model minimal ;
- suppression des samples produit.

Gate : `canonical:check` vert.

### v0.4 - Produit End-To-End

Objectif : connecter core, API, UI et tests.

Livrables :

- core déterministe ;
- routes API ;
- UI consommant les read-models ;
- golden cases E2E ;
- provenance visible.

Gate : aucune route produit ne dépend de fixtures.

### v0.5 - Gouvernance

Objectif : contrôler les changements.

Livrables :

- decision records ;
- workflow d'ambiguïtés ;
- rapport de couverture ;
- approbations métier ;
- dérogations expirantes.

Gate : toute ambiguïté critique a owner et décision ou blocage explicite.

### v0.6 - Assistant IA Conforme

Objectif : utiliser l'IA avec citations et limites.

Livrables :

- ingestion RAG complète ;
- tools typés ;
- evals ;
- observabilité ;
- refus sur manque de preuve.

Gate : le LLM ne calcule pas les décisions critiques et cite ses sources.

### v1.0 - Release Auditée

Objectif : produire une version exploitable.

Livrables :

- `canonical:check:strict` vert ;
- `release:compliance` vert ;
- documentation utilisateur ;
- rollback ;
- backup ;
- rapport d'audit.

## Issue List Générique

### P0 - Socle Canonique

- P0-01 Créer le dépôt et la structure documentaire.
- P0-02 Créer le schéma du source manifest.
- P0-03 Inventorier toutes les sources connues.
- P0-04 Calculer et vérifier les hashes.
- P0-05 Classer priorités, statuts, domaines, propriétaires.
- P0-06 Créer le schéma de matrice canonique.
- P0-07 Atomiser les règles et objets critiques.
- P0-08 Créer les decision records pour ambiguïtés critiques.
- P0-09 Créer `canonical:check`.
- P0-10 Générer `coverage-report.md`.

### P1 - Contrats Et Données

- P1-01 Définir les types d'unités métier.
- P1-02 Créer les contrats YAML/JSON.
- P1-03 Créer les schémas runtime.
- P1-04 Créer les loaders.
- P1-05 Créer le read-model DB.
- P1-06 Ajouter les tests de références croisées.
- P1-07 Supprimer les samples produit.

### P2 - Core Et Produit

- P2-01 Créer le core déterministe.
- P2-02 Porter les règles critiques.
- P2-03 Ajouter les golden cases.
- P2-04 Exposer API avec provenance.
- P2-05 Connecter UI aux read-models.
- P2-06 Bloquer imports interdits.
- P2-07 Ajouter E2E métier.

### P3 - Knowledge Base Et IA

- P3-01 Créer pipeline ingestion depuis manifest.
- P3-02 Définir stratégie chunking.
- P3-03 Indexer corpus avec metadata.
- P3-04 Ajouter evals retrieval/citation.
- P3-05 Ajouter observabilité LLM.
- P3-06 Créer tools typés pour agents.
- P3-07 Ajouter refus si source insuffisante.

### P4 - Release Et Gouvernance

- P4-01 Créer policy dérogations.
- P4-02 Ajouter `canonical:check:strict`.
- P4-03 Ajouter `release:compliance`.
- P4-04 Documenter rollback/backup.
- P4-05 Mettre en place approbation métier.
- P4-06 Publier rapport d'audit.
- P4-07 Formaliser processus de changement.


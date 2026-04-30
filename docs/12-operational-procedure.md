# 12 - Procédure Opérationnelle Détaillée

Cette procédure est le mode opératoire à appliquer sur un vrai projet. Chaque étape indique pourquoi elle existe, comment l'exécuter, les livrables attendus et la définition de fini.

## Phase 0 - Cadrage

### 0.1 Définir Le Domaine Et Le Risque

Pourquoi : la rigueur nécessaire dépend du risque. Un jeu, un simulateur fiscal et un outil clinique n'ont pas le même niveau d'autorité humaine, d'audit et de sécurité.

Comment :

1. Nommer le domaine principal.
2. Décrire les décisions que le produit prendra ou assistera.
3. Classer les décisions : informatives, opérationnelles, financières, légales, médicales, sécurité.
4. Définir qui peut valider ou écraser une décision.
5. Définir les conséquences d'une erreur.

Livrable :

```text
docs/governance/domain-risk-profile.md
```

Definition of Done :

- le niveau de risque est explicite ;
- la hiérarchie d'autorité est écrite ;
- les décisions interdites au LLM sont listées ;
- les critères release sont ajustés au risque.

### 0.2 Définir Le Vocabulaire Projet

Pourquoi : la méthode doit utiliser les mots du métier pour éviter une couche abstraite incomprise.

Comment :

1. Mapper `source`, `unit`, `contract`, `core`, `golden case` aux termes du domaine.
2. Ajouter les types d'unités.
3. Écrire un glossaire minimal.

Livrable :

```text
docs/canonical/glossary.md
```

Definition of Done :

- un expert métier comprend les types d'unités ;
- les agents IA ont les termes exacts à utiliser ;
- aucune catégorie importante n'est nommée "other" sans justification.

## Phase 1 - Registre Des Sources

### 1.1 Inventorier Sans Filtrer

Pourquoi : filtrer trop tôt crée des angles morts.

Comment :

1. Scanner docs, exports, legacy, tickets, dossiers métier.
2. Ajouter chaque élément au manifest en `needs_review`.
3. Inclure aussi les sources suspectes, obsolètes ou doublons.
4. Noter les sources attendues mais absentes.

Livrable :

```text
docs/canonical/source-manifest.yaml
```

Definition of Done :

- tout fichier ou URL connu a une entrée ;
- les sources non disponibles ont une entrée `blocked` ;
- aucune source n'est ignorée oralement.

### 1.2 Hasher Et Normaliser

Pourquoi : sans hash, impossible de savoir si une source a changé.

Comment :

1. Définir la normalisation par type source.
2. Calculer `sha256`.
3. Stocker hash et version.
4. Rejouer le calcul en CI.

Livrable :

```text
tools/canonical hash-sources --write
```

Definition of Done :

- le hash est reproductible ;
- un changement de source fait échouer `canonical:check` ;
- les fichiers binaires et OCR ont une politique claire.

### 1.3 Statuer Et Prioriser

Pourquoi : le produit doit savoir quel poids donner aux sources.

Comment :

1. Assigner `priority`.
2. Assigner `status`.
3. Assigner `owner`.
4. Documenter `license` et `confidentiality`.
5. Faire valider les sources primaires.

Definition of Done :

- aucune source active sans owner ;
- aucune source active sans priorité ;
- aucune source confidentielle sans usage autorisé.

## Phase 2 - Atomisation

### 2.1 Définir Les Types D'unités

Pourquoi : l'atomisation doit être systématique.

Comment :

1. Lister les objets métier.
2. Lister les règles calculatoires.
3. Lister les exceptions.
4. Lister les termes de lexique.
5. Lister les workflows.
6. Lister les comportements legacy à préserver.

Definition of Done :

- chaque type a un exemple ;
- chaque type indique s'il doit devenir contrat structuré, chunk texte ou les deux ;
- les objets legacy importants ont une catégorie dédiée.

### 2.2 Atomiser Source Par Source

Pourquoi : passer directement à l'implémentation entraîne omissions et inventions.

Comment :

1. Lire une source active.
2. Extraire chaque unité.
3. Donner un ID stable.
4. Noter le locator exact.
5. Résumer la règle sans interprétation.
6. Marquer `missing` par défaut.

Livrable :

```text
docs/canonical/<domain>-matrix.yaml
```

Definition of Done :

- chaque source primaire active a des unités ;
- chaque unité a source refs et hash ;
- les doublons sont liés, pas copiés aveuglément.

### 2.3 Traiter Ambiguïtés Et Conflits

Pourquoi : une ambiguïté non gouvernée devient du code caché.

Comment :

1. Créer une unité `ambiguity`.
2. Référencer les sources en conflit.
3. Décrire les options.
4. Créer un ADR ou decision record.
5. Assigner owner et échéance.

Definition of Done :

- aucune ambiguïté critique sans owner ;
- aucune décision critique sans trace ;
- les tests reflètent la décision retenue.

## Phase 3 - Contrats Et Schémas

### 3.1 Concevoir Les Contrats

Pourquoi : les contrats sont la vérité structurée du produit.

Comment :

1. Grouper les unités structurables par catalogue.
2. Écrire un modèle minimal.
3. Inclure `id`, `source_refs`, `status`, `version`.
4. Inclure les conditions et effets sans logique cachée.
5. Séparer données et calculs.

Definition of Done :

- aucun objet produit structurable n'est seulement dans le texte ;
- aucune entrée active sans source refs ;
- les contrats sont lisibles par métier et machine.

### 3.2 Écrire Les Schémas

Pourquoi : un contrat sans schéma dérive vite.

Comment :

1. Choisir Zod, Pydantic, JSON Schema ou équivalent.
2. Valider types, enums, champs obligatoires.
3. Ajouter validations de références croisées.
4. Ajouter tests d'exemples invalides.

Definition of Done :

- les contrats valides passent ;
- les erreurs attendues échouent avec messages utiles ;
- les schémas sont versionnés.

### 3.3 Construire Les Read-Models

Pourquoi : l'application doit consommer une forme optimisée et fiable.

Comment :

1. Définir tables/vues ou documents.
2. Charger depuis les contrats.
3. Enregistrer hash contrat et sources.
4. Vérifier idempotence.
5. Publier une vue de lecture stable.

Definition of Done :

- la DB est reconstructible depuis les contrats ;
- les références cassées font échouer le chargement ;
- l'API ne lit pas les fichiers bruts directement.

## Phase 4 - Knowledge Base

### 4.1 Indexer Depuis Le Manifest

Pourquoi : un index libre ne peut pas prouver sa couverture.

Comment :

1. Lire uniquement les sources du manifest.
2. Respecter `allowed_uses`.
3. Extraire le texte.
4. Chunker par structure métier.
5. Attacher metadata.
6. Embedding et stockage.

Definition of Done :

- toute source active autorisée a des chunks ;
- tout chunk a source hash ;
- les sources interdites ne sont pas indexées.

### 4.2 Évaluer Le RAG

Pourquoi : retrouver "quelque chose" ne suffit pas.

Comment :

1. Créer un golden set de questions.
2. Définir chunks attendus.
3. Mesurer recall, précision, groundedness.
4. Vérifier les refus.
5. Tracer les appels LLM.

Definition of Done :

- les questions critiques retrouvent les sources attendues ;
- les réponses sans preuve sont refusées ;
- les citations sont exploitables.

## Phase 5 - Produit

### 5.1 Implémenter Le Core

Pourquoi : les calculs doivent être déterministes et testables.

Comment :

1. Écrire tests depuis unités/golden cases.
2. Implémenter fonctions pures.
3. Retourner applied units.
4. Retourner erreurs métier typées.
5. Interdire imports infra.

Definition of Done :

- core testable sans DB ;
- tests critiques verts ;
- aucune règle dupliquée dans API/UI.

### 5.2 Connecter API Et UI

Pourquoi : la conformité doit atteindre l'utilisateur final.

Comment :

1. API charge read-models.
2. API appelle core.
3. API expose provenance.
4. UI affiche uniquement API/read-models.
5. UI montre erreurs et sources quand nécessaire.

Definition of Done :

- aucune surface produit sur samples ;
- les golden cases passent via API ;
- l'UI ne masque pas les états métier critiques.

## Phase 6 - Gates Et Release

### 6.1 Installer Les Checks

Pourquoi : la discipline doit être automatisée.

Commandes types :

```text
validate
canonical:check
canonical:check:strict
release:compliance
```

Definition of Done :

- les checks tournent localement et CI ;
- les erreurs indiquent unité/source concernée ;
- le strict est documenté comme objectif release.

### 6.2 Promouvoir

Pourquoi : une release doit être un acte de conformité.

Comment :

1. Rejouer strict.
2. Générer coverage report.
3. Vérifier ADRs.
4. Vérifier backup/rollback.
5. Obtenir approbation métier.
6. Tagger release.

Definition of Done :

- rapport archivé ;
- release liée à manifest/matrix hashes ;
- rollback testé.


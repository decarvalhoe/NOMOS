# 06 - Intégration Produit

## Core Déterministe

Le core métier est une bibliothèque pure. Il reçoit des entrées typées et retourne des sorties typées. Il ne lit pas directement la base, n'appelle pas le LLM, ne dépend pas de React, FastAPI, Next.js, Payload, Django, Rails ou autre surface.

Responsabilités du core :

- appliquer les règles ;
- calculer les résultats ;
- valider les combinaisons ;
- produire des erreurs métier explicites ;
- exposer des fonctions testables.

Interdits :

- importer un client DB ;
- importer un routeur HTTP ;
- importer un composant UI ;
- appeler un modèle LLM ;
- lire un fichier sample ;
- résoudre une ambiguïté sans décision référencée.

## API

L'API expose le core et les read-models. Elle ne doit pas réimplémenter les règles. Elle orchestre :

- authentification/autorisation ;
- chargement des read-models ;
- validation entrée/sortie ;
- appel core ;
- audit log ;
- réponse structurée.

L'API doit exposer la provenance quand c'est utile :

```json
{
  "result": {"eligible": true},
  "applied_units": ["RULE-ELIGIBILITY-001", "EXCEPTION-AGE-002"],
  "source_refs": [
    {"source_id": "POLICY-2026", "locator": "section 7.2"}
  ]
}
```

## UI

L'UI affiche des données produit issues de l'API ou d'un bundle canonique généré. Elle ne porte pas de listes métier inventées.

Règles :

- pas de catalogue métier dans un composant ;
- pas de formule métier dans un handler UI ;
- pas de "fallback demo" visible en mode produit ;
- les états incomplets doivent afficher "donnée non disponible" plutôt qu'une invention ;
- les écrans critiques doivent pouvoir montrer la provenance.

## CMS Et Édition

Un CMS peut éditer les contrats si :

- il écrit dans un format versionné ;
- il conserve les source refs ;
- il produit une PR ou un audit log ;
- il valide les schémas ;
- il déclenche les tests et gates ;
- il ne modifie pas directement le read-model de production sans pipeline.

## Agents IA

Les agents IA sont des collaborateurs, pas des autorités finales.

Ils peuvent :

- résumer une source ;
- proposer une atomisation ;
- détecter des conflits ;
- générer des tests à partir d'une unité ;
- aider à migrer du legacy ;
- expliquer une décision avec citations.

Ils ne peuvent pas :

- inventer une entrée de catalogue ;
- arbitrer une contradiction critique sans humain autorisé ;
- calculer un résultat métier à la place du core ;
- modifier un contrat sans source refs ;
- supprimer une source du manifest sans écart.

## Hiérarchie D'autorité

Chaque domaine doit définir sa hiérarchie :

| Domaine | Exemple |
|---|---|
| Assurance | actuaire > souscripteur senior > juriste > LLM > règle par défaut. |
| Fiscalité | fiscaliste senior > comptable > LLM cité > calcul automatique. |
| Clinique | médecin > pharmacien/infirmier selon acte > LLM protocole > suggestion. |
| Droit | avocat > juriste > LLM citation > règle générale. |
| Jeu | auteur/MJ > table > LLM > automatisme. |

Toute dérogation doit produire une trace.

## Tests Associés

- Le core n'importe aucune dépendance interdite.
- Les routes API appellent le core pour les calculs.
- L'UI ne contient pas de listes métier hardcodées.
- Les agents utilisent des tools typés pour les calculs.
- Les réponses critiques incluent applied units et citations.
- Les permissions empêchent un LLM de promouvoir une décision critique seul.


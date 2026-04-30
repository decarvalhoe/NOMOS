# 09 - Guide D'adaptation Tous Domaines Et Toutes Stacks

## Adapter Le Vocabulaire

La méthode est stable, mais les noms changent.

| Concept générique | Assurance | Fiscalité | Clinique | Droit | Jeu |
|---|---|---|---|---|---|
| Source | contrat | code fiscal | recommandation | texte légal | livre de règles |
| Unité | clause | article | recommandation | disposition | règle/objet |
| Catalogue | garanties | taux/seuils | protocoles | référentiel | races/sorts |
| Core | pricing-core | tax-core | decision-core | compliance-core | rules-core |
| Golden case | sinistre | déclaration | cas patient | cas d'espèce | personnage/combat |
| Owner | actuaire | fiscaliste | médecin | avocat | auteur/MJ |

## Adapter La Stack

### TypeScript

- Contrats : YAML/JSON.
- Schémas : Zod ou TypeBox.
- Core : package TS pur.
- API : Fastify, Hono, NestJS.
- DB : PostgreSQL + Drizzle/Prisma.
- Vector : pgvector, Qdrant, Weaviate.
- Tests : Vitest + Playwright.

### Python

- Contrats : YAML/JSON.
- Schémas : Pydantic ou msgspec.
- Core : package Python pur.
- API : FastAPI.
- DB : PostgreSQL + SQLAlchemy.
- Vector : pgvector, Qdrant, Weaviate.
- Tests : pytest + Playwright.

### JVM

- Contrats : JSON/YAML/Avro/protobuf.
- Schémas : JSON Schema, Kotlin serialization, Bean Validation.
- Core : module Java/Kotlin pur.
- API : Spring Boot, Micronaut, Quarkus.
- DB : PostgreSQL + jOOQ/Flyway.
- Tests : JUnit, Testcontainers, Playwright.

### Go

- Contrats : JSON/YAML/protobuf.
- Schémas : JSON Schema + validateurs, protobuf.
- Core : module Go sans dépendance infra.
- API : chi, Gin, Echo, stdlib.
- DB : PostgreSQL + sqlc.
- Tests : Go test + Playwright.

## Adapter Au Niveau De Risque

| Niveau | Exemple | Rigueur minimale |
|---|---|---|
| Faible | jeu, contenu éditorial interne | manifest + contrats + tests critiques. |
| Moyen | RH, assurance interne, éducation | matrice complète + propriétaires + strict avant release. |
| Fort | fiscalité, droit, finance réglementée | audit trail, dates d'effet, double validation, rollback. |
| Très fort | clinique, dosage, sécurité industrielle | validation humaine qualifiée, logs, évals indépendantes, dérogations strictes. |

## Adapter Au Legacy

Utiliser le modèle Strangler :

1. Identifier les comportements legacy actifs.
2. Les enregistrer comme sources `legacy`.
3. Écrire des characterization tests.
4. Extraire un module pur équivalent.
5. Comparer legacy et nouveau core.
6. Router progressivement.
7. Retirer le legacy seulement quand les golden cases passent.

Ne pas réécrire tout le legacy d'un coup. Le legacy est une source de vérité historique tant qu'il décrit un comportement réel.

## Adapter Aux Petites Équipes

Version légère viable :

- `source-manifest.yaml`;
- `canonical-matrix.yaml`;
- `data/canonical/*.yaml`;
- `schemas/`;
- `tests/golden/`;
- script `canonical:check`;
- ADRs Markdown.

Ne pas installer un gros ALM si l'équipe n'a pas la capacité de le maintenir. La discipline importe plus que l'outil.

## Adapter Aux Grandes Organisations

Version industrialisée :

- ALM exigences : Jama, Polarion, DOORS Next, ReqView ou équivalent ;
- data catalog/lineage : OpenLineage, DataHub, OpenMetadata ;
- CI centralisée ;
- policy-as-code ;
- approbations métier ;
- observabilité LLM ;
- audit trail signé ;
- dashboards de couverture.

## Anti-Patterns

- "On met tout dans le RAG et le LLM répondra."
- "Le sample sera remplacé plus tard."
- "Cette exception est évidente, pas besoin de la documenter."
- "Le frontend peut recalculer pour l'affichage."
- "Le legacy est vieux donc il est faux."
- "Le schéma valide donc la règle est correcte."
- "La source est confidentielle donc on ne la met pas au manifest."
- "La gate stricte est rouge donc on la désactive."


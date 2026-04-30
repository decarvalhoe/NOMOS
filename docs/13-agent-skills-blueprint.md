# 13 - Blueprint Skills Et Agents

## Objectif

Les agents IA doivent être spécialisés pour réduire les inventions et clarifier les responsabilités. Un agent généraliste peut aider, mais une méthode Canonical-First gagne beaucoup à séparer les rôles.

## Set Minimal De Skills

Chaque domaine peut instancier ces skills en remplaçant `<domain>`.

| Skill | Responsabilité | Sortie |
|---|---|---|
| `<domain>-source-registry` | Inventorier, hasher, statuer les sources. | `source-manifest.yaml`, source audit. |
| `<domain>-canonical-matrix` | Atomiser les unités et maintenir la matrice. | `<domain>-matrix.yaml`. |
| `<domain>-catalog-contracts` | Créer/valider contrats et schémas. | YAML/JSON, Zod/Pydantic/JSON Schema, loaders. |
| `<domain>-knowledge-base` | Indexer corpus et auditer vector store. | chunks, metadata, evals retrieval. |
| `<domain>-canonical-compliance` | Vérifier source -> produit. | `coverage-report.md`, gaps. |
| `<domain>-product-compliance` | Vérifier core/API/UI/tests. | fuites produit, tests, correctifs. |

## Skills Optionnels

| Skill | Quand |
|---|---|
| `<domain>-decision-governance` | Domaine à ambiguïtés fréquentes ou risque élevé. |
| `<domain>-golden-cases` | Besoin fort de cas expert de référence. |
| `<domain>-legacy-migration` | Code legacy important à migrer par Strangler. |
| `<domain>-release-compliance` | Releases auditables ou réglementées. |
| `<domain>-security-privacy` | Données sensibles, licences, secret, PII/PHI. |

## Contrat D'un Skill

Un skill doit préciser :

- quand l'utiliser ;
- sources à consulter ;
- fichiers autorisés ;
- sorties attendues ;
- gates à exécuter ;
- actions interdites ;
- format de rapport ;
- exemples d'écarts.

## Exemple De Skill Source Registry

```markdown
# <domain>-source-registry

Use when inventorying, hashing, prioritizing, status-tagging, or auditing canonical sources.

Rules:
- Never ignore a discovered source without manifest entry.
- Never mark a source out_of_scope without reason.
- Preserve hashes and source paths.
- Do not interpret rules; only register sources.

Outputs:
- docs/canonical/source-manifest.yaml
- docs/canonical/source-audit.md
```

## Exemple De Skill Product Compliance

```markdown
# <domain>-product-compliance

Use when modifying core, API, UI, agents, tests, or release gates.

Rules:
- Product surfaces must consume read-models or generated canonical bundles.
- No sample/mock/fixture data in product paths.
- Core must not import DB, HTTP, UI, or LLM.
- LLM must call typed tools for deterministic decisions.
- Critical UI states must not hide rule outcomes.

Outputs:
- changed code
- test evidence
- compliance gaps
```

## Coordination Multi-Agent

Répartition recommandée :

| Agent | Domaine |
|---|---|
| Source agent | manifest, hashes, source coverage. |
| Matrix agent | atomisation, unit IDs, gaps. |
| Contract agent | YAML/JSON, schemas, loaders. |
| Product agent | core/API/UI/tests. |
| Knowledge agent | vector store, chunking, RAG evals. |
| Release agent | gates, coverage report, audit. |

Règles de coordination :

- un seul owner par fichier critique ;
- pas de modification simultanée du même contrat ;
- les agents ne suppriment pas les changements des autres ;
- les agents rapportent les gaps plutôt que les masquer ;
- les décisions métier remontent à un humain autorisé.

## Prompt De Démarrage Pour Un Agent

```text
Tu travailles sur un projet Canonical-First.
Avant toute modification métier, lis le source manifest, la canonical matrix,
les contrats et les decision records.
Ta mission est limitée à : [scope].
Tu dois produire : [outputs].
Tu ne dois pas inventer de données métier.
Tout ajout doit avoir source_refs ou decision_ref.
Si une ambiguïté apparaît, crée/complète une entrée d'écart au lieu de décider silencieusement.
```

## Critères De Qualité D'un Agent

Un bon agent Canonical-First :

- demande moins d'interprétation implicite ;
- remonte les écarts ;
- produit des IDs stables ;
- cite les fichiers et lignes ;
- met à jour les tests ;
- respecte les ownerships ;
- laisse le projet plus auditable qu'avant.

Un mauvais agent :

- "nettoie" des sources en les supprimant ;
- invente des catalogues ;
- remplace une règle par une approximation ;
- fait passer l'UI avant le canon ;
- produit une synthèse non sourcée ;
- désactive une gate.


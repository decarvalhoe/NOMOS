# 10 - Outils Et Projets À Étudier

Cette liste n'est pas une stack imposée. Elle sert de carte d'exploration pour choisir les bons outils selon la taille, le risque et l'équipe.

## Exigences Et Traçabilité

| Outil/projet | À étudier pour | Remarque |
|---|---|---|
| Jama Connect | gestion d'exigences, revue, baseline, traçabilité | adapté aux organisations réglementées. |
| IBM DOORS Next | exigences complexes et industries critiques | puissant mais lourd. |
| Polarion ALM | exigences + tests + workflow | pertinent si l'entreprise vit déjà dans ALM. |
| ReqView | exigences versionnées plus légères | option pragmatique pour équipes petites/moyennes. |
| Markdown + générateur maison | démarrage rapide | idéal avant d'acheter un outil. |

## Documentation As Code

| Outil/projet | À étudier pour |
|---|---|
| MkDocs Material | documentation versionnée, recherche, publication simple. |
| Docusaurus | documentation produit/dev avec versioning. |
| Backstage TechDocs | docs-like-code intégrées à un portail développeur. |
| Mermaid/PlantUML | diagrammes versionnés. |
| C4 Model | architecture lisible par niveaux. |

## Schémas Et Contrats

| Outil/projet | À étudier pour |
|---|---|
| JSON Schema | contrat interopérable, language-agnostic. |
| OpenAPI | contrat API HTTP. |
| AsyncAPI | contrat événements/messages. |
| Zod | schémas TypeScript ergonomiques. |
| Pydantic | schémas Python et validation runtime. |
| Protobuf/Avro | contrats binaires ou data pipelines fortement typés. |

## Données, Qualité Et Lineage

| Outil/projet | À étudier pour |
|---|---|
| OpenLineage | standard ouvert de lineage jobs/datasets. |
| DataHub | catalogue de données, ownership, lineage, gouvernance. |
| OpenMetadata | catalogue, lineage, qualité, gouvernance. |
| Great Expectations | attentes de qualité de données et rapports. |
| dbt tests/contracts | validation de modèles analytiques. |
| DVC | versioning de datasets et artefacts. |
| lakeFS | versioning type Git pour object storage. |

## Policy-As-Code Et Gates

| Outil/projet | À étudier pour |
|---|---|
| Open Policy Agent | règles de conformité et autorisation en policy-as-code. |
| Conftest | tests OPA sur configs, manifests, IaC. |
| Semgrep | détection de patterns interdits dans le code. |
| GitHub Actions/GitLab CI | gates reproductibles. |
| pre-commit | checks locaux avant commit. |

## RAG, Knowledge Base Et Évaluations LLM

| Outil/projet | À étudier pour |
|---|---|
| pgvector | vector store proche du relationnel. |
| Qdrant | vector store spécialisé, self-host clair. |
| Weaviate | vector DB avec fonctionnalités de recherche avancées. |
| LlamaIndex | ingestion, nodes, retrieval, RAG applicatif. |
| Haystack | pipelines RAG et recherche. |
| OpenAI Evals | évaluation LLM et systèmes LLM. |
| Ragas | métriques RAG : faithfulness, context relevance, answer relevance. |
| Langfuse | observabilité, traces, datasets, prompts, évals LLM. |

## Observabilité

| Outil/projet | À étudier pour |
|---|---|
| OpenTelemetry | traces, logs, métriques vendor-neutral. |
| Grafana/Prometheus/Loki/Tempo | stack open source d'observabilité. |
| Sentry | erreurs applicatives et session replay. |
| Langfuse/LangSmith/Helicone | observabilité spécifique LLM. |

## Migration Legacy

| Outil/projet | À étudier pour |
|---|---|
| Strangler Fig pattern | remplacement incrémental d'un legacy. |
| Characterization tests | figer le comportement existant avant migration. |
| Approval tests | comparer sorties textuelles ou structurées complexes. |
| Testcontainers | tests d'intégration reproductibles. |

## Critères De Sélection

Avant d'adopter un outil, répondre :

- Peut-il être versionné ou exporté dans Git ?
- Peut-il être vérifié en CI ?
- Garde-t-il les IDs stables ?
- Expose-t-il des APIs ?
- Supporte-t-il les propriétaires et statuts ?
- Peut-on le quitter sans perdre la chaîne de preuve ?
- Est-il adapté au niveau de risque réel ?

Outil lourd sans discipline = dette. Fichier simple avec discipline = déjà utile.


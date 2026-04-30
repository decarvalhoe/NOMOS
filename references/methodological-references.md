# Références Méthodologiques

Cette méthode est une synthèse pratique. Elle combine des références d'ingénierie système, architecture logicielle, conformité, documentation as-code, RAG et sécurité LLM.

## Traçabilité Et Ingénierie Système

- NASA Systems Engineering Handbook  
  https://www.nasa.gov/wp-content/uploads/2018/09/nasa_systems_engineering_handbook_0.pdf  
  Utilisé pour la notion de traçabilité bidirectionnelle, exigences parent/source, validation, gestion des changements et matrice de requirements.

- W3C PROV-O: The PROV Ontology  
  https://www.w3.org/TR/prov-o/  
  Utilisé pour le vocabulaire de provenance : entités, activités, agents, dérivation et interopérabilité des relations de provenance.

## Architecture Et Décisions

- Michael Nygard, Documenting Architecture Decisions  
  https://www.cognitect.com/blog/2011/11/15/documenting-architecture-decisions  
  Utilisé pour les ADRs légers, versionnés et conservant le contexte, la décision, le statut et les conséquences.

- C4 Model, Simon Brown  
  https://c4model.com/  
  Utilisé pour documenter l'architecture par niveaux : contexte, conteneurs, composants, code, avec notation et outils non imposés.

- Martin Fowler, Strangler Fig Application  
  https://martinfowler.com/bliki/StranglerFigApplication.html  
  Utilisé pour migrer un legacy par remplacement incrémental plutôt que big-bang rewrite.

- Martin Fowler, Test Pyramid  
  https://martinfowler.com/bliki/TestPyramid.html  
  Utilisé pour éviter de concentrer toute la conformité dans quelques tests E2E fragiles.

## Contrats Et Interfaces

- JSON Schema  
  https://json-schema.org/  
  Utilisé comme référence language-agnostic pour valider et documenter les contrats JSON.

- OpenAPI Specification  
  https://spec.openapis.org/oas/  
  Utilisé pour les contrats d'API HTTP, en distinguant schéma machine et texte normatif.

- The Twelve-Factor App  
  https://www.12factor.net/  
  Utilisé pour portabilité, configuration, séparation build/release/run et discipline d'exploitation.

## IA, RAG Et Risque

- Lewis et al., Retrieval-Augmented Generation for Knowledge-Intensive NLP Tasks, NeurIPS 2020  
  https://papers.neurips.cc/paper_files/paper/2020/hash/6b493230205f780e1bc26945df7481e5-Abstract.html  
  Utilisé pour le principe de mémoire non paramétrique récupérée, utile aux tâches intensives en connaissance.

- NIST AI Risk Management Framework 1.0  
  https://www.nist.gov/publications/artificial-intelligence-risk-management-framework-ai-rmf-10  
  Utilisé pour cadrer gouvernance, cartographie, mesure et gestion des risques IA.

- OWASP Top 10 for Large Language Model Applications  
  https://owasp.org/www-project-top-10-for-large-language-model-applications/  
  Utilisé pour les risques LLM : prompt injection, output handling, supply chain, disclosure, agency excessive.

## Outils Et Projets À Étudier

- OpenLineage  
  https://openlineage.io/  
  Projet utile pour comprendre lineage jobs/datasets et propagation de metadata.

- Open Policy Agent  
  https://www.openpolicyagent.org/docs/latest  
  Projet utile pour policy-as-code, gates et décisions de conformité automatisables.

- Great Expectations  
  https://docs.greatexpectations.io/  
  Projet utile pour expectations, suites de qualité de données et rapports de validation.

- OpenTelemetry  
  https://opentelemetry.io/  
  Projet utile pour traces, logs, métriques et instrumentation vendor-neutral.

- OpenAI Evals  
  https://github.com/openai/evals  
  Projet utile pour concevoir des évaluations LLM reproductibles.

- Ragas  
  https://docs.ragas.io/  
  Projet utile pour évaluer retrieval et génération dans des pipelines RAG.

- Langfuse  
  https://langfuse.com/docs  
  Projet utile pour observabilité, traces, prompts, datasets et évals LLM.

- LlamaIndex  
  https://docs.llamaindex.ai/en/stable/understanding/rag/  
  Projet utile pour comprendre documents, nodes, chunking et pipelines RAG applicatifs.

- DVC  
  https://dvc.org/  
  Projet utile pour versionner datasets et artefacts en complément de Git.

- Backstage TechDocs  
  https://backstage.io/docs/features/techdocs/  
  Projet utile pour industrialiser la documentation as-code dans un portail développeur.

## Références À Ajouter Selon Domaine

Chaque projet doit compléter cette liste avec ses sources propres :

- normes ou lois officielles ;
- contrats et avenants ;
- guides métier ;
- legacy code ;
- exports historiques ;
- manuels internes ;
- tickets de décisions ;
- cas réels validés ;
- corpus de tests expert.


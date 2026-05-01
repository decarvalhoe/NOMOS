# ADR-0003: Priorite des adapters — Node > Python > JVM

Status: accepted
Date: 2026-04-30
Owner: Nomos core team

## Contexte

Nomos supporte les stacks techniques via des adapters pluggables. Chaque adapter couvre un ou plusieurs langages, frameworks et surfaces. Les ressources de developpement sont limitees pour la v0.1 ; il faut prioriser les adapters qui couvrent le plus de projets cibles.

## Options considerees

### Option A: Node > Python > JVM

- Node (JavaScript/TypeScript) represente la majorite des projets web full-stack.
- Python couvre le ML/data, les APIs FastAPI/Django, et les scripts d'infrastructure.
- JVM (Java/Kotlin) couvre l'entreprise, les microservices Spring, les systemes Android.
- Cet ordre maximise la couverture avec le minimum d'adapters.

### Option B: Go > Node > Python

- Go est le langage du CLI Nomos lui-meme, mais peu de projets cibles sont en Go.
- Favoriser Go creerait un biais d'auto-reference.

### Option C: Tous en parallele

- Maximise la couverture theorique mais disperse les ressources et retarde la stabilisation.

## Decision

Priorite de developpement des adapters pour la v0.1 :

1. **Node adapter** (JavaScript + TypeScript) — couvre React, Next.js, Express, Fastify, NestJS.
2. **Python adapter** — couvre FastAPI, Django, Flask, Celery, data pipelines.
3. **JVM adapter** (Java + Kotlin) — couvre Spring Boot, Gradle, Maven, Android.

L'adapter Go est un bonus si les ressources le permettent, mais n'est pas prioritaire.

## Consequences

- L'adapter Node est le premier a atteindre le niveau "stable" dans le manifeste adapter.
- Les tests d'integration et les fixtures prioritaires couvrent des projets Node.
- Les projets Go, Rust, C# ne sont pas bloquants pour la release v0.1.
- L'admission accepte tous les langages detectes, mais les adapters ne couvrent que Node/Python/JVM en v0.1.
- Les projets hors couverture adapter recoivent un verdict `partial` avec gap `missing_adapter`.

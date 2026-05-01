# ADR-0001: Go + CUE comme stack technique Nomos

Status: accepted
Date: 2026-04-30
Owner: Nomos core team

## Contexte

Nomos a besoin d'un langage pour le CLI core et d'un langage de schema pour definir les contrats (project manifest, adapter manifest, canonical matrix). Le CLI doit etre distribue comme binaire autonome, sans runtime externe. Les schemas doivent supporter la validation, la generation JSON Schema, et l'export OpenAPI.

## Options considerees

### Option A: Go + CUE

- Go produit un binaire statique, cross-compile facilement, ecosysteme CLI mature.
- CUE est natif Go, supporte la validation, l'export JSON Schema, et la composition de schemas.
- CUE est concu pour la configuration et les contrats, pas pour la logique metier.

### Option B: Rust + JSON Schema

- Performances superieures et securite memoire garantie.
- JSON Schema est plus repandu mais moins expressif pour les contraintes metier.
- Ecosysteme CUE inexistant en Rust, necessite un bridge ou un abandon de CUE.

### Option C: TypeScript + Zod

- Prototypage rapide, ecosysteme riche.
- Necessite un runtime Node.js, complique la distribution binaire.
- Zod ne couvre pas les cas CUE (unification, defaults structurels).

## Decision

Go pour le CLI core et les adapters de reference. CUE pour tous les schemas de contrats et manifestes. Les schemas CUE sont la source de verite ; JSON Schema est derive par export.

## Consequences

- Le CLI est distribue comme un seul binaire sans dependance runtime.
- Les contributeurs doivent connaitre Go et CUE.
- Les adapters tiers peuvent etre ecrits dans n'importe quel langage tant qu'ils respectent les schemas exportes.
- La bibliotheque CUE Go (cuelang.org/go) est une dependance directe du module cli.
- Les schemas CUE vivent dans `specs/` et sont valides par `cue vet` en CI.

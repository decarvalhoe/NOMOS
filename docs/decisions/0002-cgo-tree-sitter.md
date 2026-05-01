# ADR-0002: CGO pour tree-sitter

Status: accepted
Date: 2026-04-30
Owner: Nomos core team

## Contexte

Nomos doit analyser du code source dans plusieurs langages (Go, TypeScript, Python, Java, etc.) pour detecter les surfaces produit, les routes API, les patterns interdits et les modeles de donnees. L'analyse par regex et heuristiques de fichiers (detect package actuel) est suffisante pour la detection de surface mais insuffisante pour l'extraction structurelle (routes, schemas, symboles).

## Options considerees

### Option A: CGO + tree-sitter

- tree-sitter offre un parsing incrementiel, tolerant aux erreurs, multi-langage.
- Les grammaires couvrent tous les langages cibles (Go, TS, Python, Java, Kotlin, Rust, PHP, Ruby, C#).
- Necessite CGO, ce qui complique le cross-compile et augmente la taille du binaire.
- Bibliotheque Go mature : github.com/smacker/go-tree-sitter.

### Option B: Pure Go parsers

- gopls/go/ast pour Go, mais pas de parser Go-natif pour TypeScript, Python, etc.
- Chaque langage necessite un parser separe, maintenance lourde.

### Option C: LSP externe

- Deleguer l'analyse a des language servers externes.
- Ajoute des dependances runtime, complexite de setup, latence.

## Decision

Utiliser tree-sitter via CGO pour l'analyse structurelle multi-langage dans les adapters. Le package `detect` reste en pure Go pour la detection de surface legere (pas de CGO requis pour la detection).

## Consequences

- Le binaire CLI necessite CGO pour les commandes qui font de l'analyse structurelle (diagnose, certains adapters).
- Le build CI doit configurer un compilateur C et les grammaires tree-sitter.
- Les commandes qui n'utilisent pas l'analyse structurelle (init, validate, admit) restent pure Go.
- La taille du binaire augmente d'environ 5-10 MB par grammaire incluse.
- Le cross-compile necessite des toolchains C pour chaque cible (linux/amd64, darwin/arm64, etc.).
- Les adapters tiers peuvent choisir tree-sitter ou une autre strategie d'analyse.

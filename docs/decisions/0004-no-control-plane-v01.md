# ADR-0004: Pas de service control-plane en v0.1

Status: accepted
Date: 2026-04-30
Owner: Nomos core team

## Contexte

L'architecture cible de Nomos inclut un control-plane (service central qui orchestre les checks, stocke les attestations, et expose un dashboard). Le dossier `control-plane/` existe dans l'arborescence mais reste embryonnaire. La question est de savoir si la v0.1 doit inclure ce service.

## Options considerees

### Option A: Pas de control-plane en v0.1

- La v0.1 se concentre sur le CLI et les adapters.
- Les attestations sont generees localement et stockees dans le repo ou en CI.
- Le control-plane est reporte a la v0.2 ou v0.3.

### Option B: Control-plane minimal (API REST)

- Un service HTTP qui recoit les rapports et les stocke.
- Ajoute une dependance infra (base de donnees, deploiement, auth).
- Complexite disproportionnee pour la valeur ajoutee en v0.1.

### Option C: Control-plane serverless (fonctions cloud)

- Moins d'infra a maintenir mais couplage fort avec un cloud provider specifique.

## Decision

Pas de service control-plane en v0.1. Le CLI est l'unique point d'entree. Les attestations et rapports sont des fichiers locaux ou des artefacts CI.

## Consequences

- Le dossier `control-plane/` reste un placeholder avec code exploratoire, non distribue.
- Les attestations sont stockees comme fichiers JSON dans le repo ou comme artefacts CI.
- La verification d'attestation est locale (pas de registre central).
- Le dashboard de conformite n'existe pas en v0.1 ; les rapports sont lus via `jq` ou un viewer JSON.
- La v0.2 pourra introduire un control-plane sans casser la compatibilite CLI.
- Les schemas et formats d'attestation sont concus des la v0.1 pour etre compatibles avec un futur control-plane (format in-toto, enveloppe DSSE).

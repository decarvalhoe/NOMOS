# Canonical Tool Contract

Ce document décrit le contrat minimal d'un outil `canonical`.

## Commandes

### `canonical sources --write`

Met à jour hashes et metadata dérivées du manifest.

### `canonical sources --check`

Échoue si une source active est manquante, hash différent, owner absent ou usage interdit incohérent.

### `canonical matrix --check`

Échoue si la matrice référence une source inconnue, un hash obsolète ou une unité invalide.

### `canonical contracts --check`

Valide les contrats contre schémas et références croisées.

### `canonical kb --check`

Vérifie que toutes les sources actives indexables ont des chunks à jour.

### `canonical product --check`

Cherche les imports interdits, samples produit, contournements core/API/UI.

### `canonical report --write`

Génère `docs/canonical/coverage-report.md`.

### `canonical strict`

Agrège les checks bloquants de release.

## Sorties

Chaque erreur doit inclure :

- code erreur ;
- source ID ou unit ID ;
- fichier ;
- locator ;
- action de correction.


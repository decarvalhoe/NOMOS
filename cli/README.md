# CLI

Ce dossier portera le binaire `nomos` et son coeur d'execution.

Responsabilites cibles :

- parsing des commandes ;
- orchestration des checks ;
- sorties standard JSON / Markdown ;
- chargement des adapters ;
- integration CI locale.

Etat actuel :

- module Go initialise ;
- point d'entree `main.go` ;
- dispatch minimal `help`, `version`, `init`, `validate` ;
- `diagnose` heuristique avec sorties JSON / Markdown ;
- tests unitaires pour le dispatcher, la detection et le diagnostic.

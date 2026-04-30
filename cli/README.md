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
- dispatch minimal `help`, `version`, `init`, `validate`, `diagnose` ;
- `nomos init` initialise un repo vide avec les profils `minimal` et `regulated` ;
- tests unitaires ecrits pour le dispatcher et l'initialisation.

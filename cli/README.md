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
- `validate` lit les manifests YAML Nomos (`nomos-project`, `source-manifest`,
  `canonical-matrix`, `adapter-manifest`) et retourne des erreurs structurees ;
- tests unitaires ecrits pour le dispatcher et les exemples officiels.

Validation de manifests :

```bash
nomos validate specs/examples/nomos-project.minimal.yaml
nomos validate --format json examples/insurance/source-manifest.example.yaml
```

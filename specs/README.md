# Specs

Ce dossier contiendra les schemas source de verite du produit Nomos.

Contenu cible :

- schemas CUE ;
- schemas JSON derives ;
- formats de report ;
- contrats d'adapters ;
- taxonomies de verdicts et d'evidence.

Premiers artefacts poses :

- `nomos-project.cue`
- `source-manifest.cue`
- `canonical-matrix.cue`
- `nomos-report.schema.json`
- `examples/`

## `nomos-report.json`

`nomos-report.schema.json` publie le contrat JSON Schema du report
machine-readable Nomos. Il fixe :

- `schema_version: "0.1.0"` et `report_type: "nomos-report"` ;
- les statuts de verdict : `pass`, `warn`, `fail`, `blocked` ;
- les severites : `info`, `low`, `medium`, `high`, `critical` ;
- les codes d'erreur `NOMOS_*` standard ;
- les types d'evidence acceptes ;
- les sections obligatoires `run`, `project`, `summary`, `verdict`, `checks`,
  `findings` et `evidence`.

Les exemples de payload sont dans `specs/examples/nomos-report.minimal.json`
et `specs/examples/nomos-report.complete.json`.

# Spec Examples

Ce dossier contient des exemples de manifests produit pour tester le meta-modele Nomos.

Exemples cibles :

- `nomos-project.minimal.yaml` : projet minimal greenfield ;
- `nomos-project.brownfield.yaml` : projet brownfield avec bloquants et scope partiel ;
- `nomos-project.regulated.yaml` : projet regulated avec exigences d'evidence plus fortes ;
- `canonical-matrix.valid.yaml` : matrice canonique valide ;
- `canonical-matrix.invalid-*.yaml` : matrices canoniques invalides pour verifier les contraintes CUE ;
- `verdict-cases.yaml` : cas de verdicts et niveaux de confiance NOM-104 ;
- `nomos-report.minimal.json` : report Nomos minimal valide ;
- `nomos-report.complete.json` : report Nomos complet avec findings, codes erreur, severites et evidence ;
- `adapter-manifest.node-typescript.yaml` : manifeste adapter valide.

Les fichiers `nomos-project.*.yaml` doivent passer avec :

```bash
cue vet specs/nomos-project.cue specs/examples/<fixture>.yaml -d '#Project'
```

Les fichiers `canonical-matrix.invalid-*.yaml` sont des fixtures negatives :
ils doivent echouer avec `cue vet specs/canonical-matrix.cue <fixture> -d
'#CanonicalMatrix'`.

Les fichiers `nomos-report.*.json` doivent rester compatibles avec
`specs/nomos-report.schema.json`.

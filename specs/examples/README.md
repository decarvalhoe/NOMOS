# Spec Examples

Ce dossier contient des exemples de manifests produit pour tester le meta-modele Nomos.

Exemples cibles :

- projet minimal greenfield ;
- projet brownfield avec blocants et scope partiel ;
- projet regulated avec exigences d'evidence plus fortes.
- matrice canonique valide ;
- matrices canoniques invalides pour verifier les contraintes CUE ;
- report Nomos minimal valide ;
- report Nomos complet avec findings, codes erreur, severites et evidence.

Les fichiers `canonical-matrix.invalid-*.yaml` sont des fixtures negatives :
ils doivent echouer avec `cue vet specs/canonical-matrix.cue <fixture> -d
'#CanonicalMatrix'`.

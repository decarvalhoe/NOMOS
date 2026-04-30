# Exemple Assurance

## Unités

- Source : contrat, conditions générales, avenant, guide de souscription, code legacy.
- Unité : garantie, exclusion, franchise, plafond, règle d'éligibilité, formule de prime.
- Core : `pricing-core` ou `coverage-core`.
- Golden case : sinistre avec profil assuré, événement, montant, décision attendue.

## Chaîne

```text
conditions générales
  -> garantie/exclusion atomisée
  -> warranties.yaml
  -> schema Warranty
  -> table warranties
  -> coverage-core
  -> API /claims/evaluate
  -> UI sinistre
  -> golden case
```


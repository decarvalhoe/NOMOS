# Exemple Fiscalité

## Unités

- Source : code fiscal, bulletin officiel, formulaire, simulateur legacy.
- Unité : article, seuil, taux, abattement, crédit, exception, période d'effet.
- Core : `tax-core`.
- Golden case : foyer fiscal complet avec résultat attendu et sources.

## Points D'attention

- Les dates d'effet sont critiques.
- Les arrondis doivent être des unités explicites.
- Les simulateurs legacy sont des sources de comportement, pas des sources juridiques primaires.
- Une réponse LLM doit citer mais ne calcule pas l'impôt final.


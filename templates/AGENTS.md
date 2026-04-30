# Agent Instructions - Canonical-First Project

## Règles Absolues

- Ne jamais introduire de donnée métier produit sans source ou décision.
- Ne jamais faire dépendre une surface produit de `sample`, `mock`, `fixture` ou données inventées.
- Ne jamais faire calculer une décision critique par un LLM.
- Ne jamais résoudre une ambiguïté dans le code sans decision record.
- Ne jamais modifier un contrat canonique sans source refs.
- Ne jamais promouvoir une release si `canonical:check:strict` est rouge, sauf dérogation expirante approuvée.

## Avant Toute Modification Métier

1. Lire `docs/canonical/source-manifest.yaml`.
2. Lire `docs/canonical/<domain>-matrix.yaml`.
3. Identifier les sources et unités concernées.
4. Vérifier les contrats et schémas.
5. Ajouter ou modifier les tests avant le code si la règle change.
6. Mettre à jour la matrice et le coverage report.

## Architecture

- Le core métier est pur.
- L'API orchestre mais ne réimplémente pas les règles.
- L'UI consomme API/read-models.
- Le RAG cite et explique, mais ne décide pas.
- Les agents proposent, les humains autorisés valident.


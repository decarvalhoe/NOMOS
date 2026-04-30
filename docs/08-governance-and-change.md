# 08 - Gouvernance Et Changement

## Pourquoi

Un corpus métier vit. Les lois changent, les contrats sont renégociés, les règles de jeu évoluent, les procédures médicales sont révisées, le legacy révèle des comportements oubliés. Sans gouvernance, la matrice devient rapidement décorative.

## Rôles

| Rôle | Responsabilité |
|---|---|
| Source owner | Valide l'existence, licence, statut et priorité d'une source. |
| Domain owner | Valide l'interprétation métier. |
| Tech owner | Garantit schémas, loaders, gates et architecture. |
| Product owner | Arbitre la priorité produit et la surface exposée. |
| Release owner | Autorise la promotion. |
| Agent IA | Propose, analyse, documente, mais ne décide pas seul. |

## Decision Records

Toute décision durable doit être enregistrée.

Cas typiques :

- conflit entre deux sources ;
- interprétation d'une ambiguïté ;
- choix temporaire de compatibilité legacy ;
- exclusion d'une source ;
- changement de priorité d'autorité ;
- migration de schéma ;
- règle non implémentée volontairement ;
- acceptation d'un risque.

Format conseillé : ADR léger avec statut `proposed`, `accepted`, `superseded`, `deprecated`.

## Cycle De Changement

1. Une source change ou une nouvelle source arrive.
2. Le manifest détecte un hash nouveau.
3. Les unités impactées sont listées.
4. Les contrats concernés sont régénérés ou modifiés.
5. Les schémas valident.
6. Les read-models sont reconstruits.
7. Les chunks vectoriels obsolètes sont réindexés.
8. Les tests impactés sont rejoués.
9. La couverture est recalculée.
10. Le domain owner valide les écarts.

## Gestion Des Ambiguïtés

Une ambiguïté doit avoir :

- un ID ;
- les sources concernées ;
- la nature du conflit ;
- les impacts produit ;
- les options possibles ;
- l'option retenue ;
- le décideur ;
- la date ;
- les tests associés.

Une ambiguïté non résolue peut rester `partial`, mais elle ne doit pas être invisible.

## Confidentialité Et Licence

Le manifest doit empêcher deux erreurs :

- indexer en vector store un contenu qui ne peut pas l'être ;
- exposer une source confidentielle dans une citation utilisateur.

Chaque source doit porter :

- `license`;
- `confidentiality`;
- `allowed_uses`;
- `redaction_policy` si nécessaire.

## Audit

Un audit doit pouvoir prendre une unité et reconstruire :

```text
unit_id
  -> source refs
  -> source hash
  -> contrat
  -> schéma
  -> read-model
  -> core/API/UI
  -> tests
  -> décisions
  -> release qui l'a introduite
```

Si cette chaîne n'est pas reconstructible, l'unité n'est pas pleinement conforme.


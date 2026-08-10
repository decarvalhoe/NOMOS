<!--
Modèle de description de PR — convention agentique RBOKproject.
Les sections « Vérifié » et « NON vérifié » sont les deux seules qui comptent
vraiment pour un relecteur : elles disent ce qui a été prouvé et ce qui est cru.
Ne supprimez pas « NON vérifié ». « Rien » est une réponse acceptable ; l'absence
de section ne l'est pas.
-->

Ticket : #

## Ce que fait cette PR

<!-- Une à trois phrases. Le quoi et le pourquoi, pas le comment. -->

## Vérifié

<!--
Commandes réellement exécutées et leur résultat. Coller la sortie utile.
Pas de « les tests passent » sans la commande qui le montre.
-->

```
```

## NON vérifié

<!--
Ce qui n'a pas été testé et pourquoi : environnement indisponible, cas non
reproductible, effet de bord supposé, comportement en production non observé.
C'est ici que se trouve le risque réel de la PR.
-->

## Périmètre touché

- Fichiers / modules :
- Effet sur la production : aucun / différé au prochain déploiement / immédiat
- Réversibilité : révocable par simple `revert` / nécessite une action manuelle (préciser)

## Origine

- [ ] Rédigée par un agent
- [ ] Rédigée par un humain

<!--
Si agent : nommer l'agent et le modèle, et confirmer que le périmètre déclaré
dans le ticket a été respecté. Toute sortie de périmètre doit être signalée ici
explicitement, pas noyée dans le diff.
-->

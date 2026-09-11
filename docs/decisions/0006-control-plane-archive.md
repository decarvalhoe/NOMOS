# ADR-0006: Archivage du code control-plane (gel + retrait du gating CI dédié)

Status: superseded by [ADR-0007](0007-control-plane-decision-portfolio-view.md) (2026-09-06) — condition de réveil remplie par NRT-019 (#667) ; dashboard porté dans `cli/internal/portfolio`, registry et storage retirés.
Date: 2026-06-11
Owner: Nomos core team
Issue: VRC-04 (#550) · Étend: [ADR-0004](0004-no-control-plane-v01.md)

## Contexte

ADR-0004 a décidé qu'aucun service control-plane n'existe en v0.1 ; le dossier
`control-plane/` est resté avec trois packages Go exploratoires (dashboard,
registry, storage) — fonctionnels, testés, **jamais appelés** par la CLI ni par
aucun consommateur. L'audit vision/réalité du 2026-06-11 (docs/45, écart E7) a
pointé l'ambiguïté : du code mort *gaté en CI* (job dédié `go-test-control-plane`
dans `ci.yml` + boucle de l'étape 3 du harnais CKM) consomme du temps de CI et
signale un statut produit que le code n'a pas. La doctrine (docs/43) exige des
états nets : câblé et prouvé, ou explicitement hors périmètre.

## Options considérées

### Option A: Câbler maintenant

Donner un caller de production (reporting portfolio). Prématuré : la roadmap
(docs/14) place le portfolio governance en v0.9.x ; aucun consommateur réel
n'existe ; le câblage ajouterait une surface non demandée avant les chantiers
P1/P2 (le drapeau).

### Option B: Supprimer le code

Perte d'un prototype propre qui informe la conception v0.9.x ; contraire à
« deux produits vivants » (rien ne casse, rien ne disparaît sans raison).

### Option C: Archiver — geler le code, retirer le gating CI dédié

Le code reste dans l'arbre comme exploration documentée ; les jobs CI dédiés
sont retirés ; le README du dossier déclare le gel et la condition de réveil.

## Décision

**Option C.** Le code `control-plane/` est archivé en l'état :

- retrait du job `go-test-control-plane` de `.github/workflows/ci.yml` ;
- retrait de la boucle control-plane de l'étape 3 de
  `scripts/ckm-non-regression.sh` ;
- `control-plane/README.md` marque le statut `archived (ADR-0006)` ;
- la carte du repository (README racine) reflète le statut archivé.

## Condition de réveil

Le code sort de l'archive uniquement via une issue « capability claim »
(template VRC-03) déclarant son **caller de production attendu**, à l'approche
du jalon v0.9.x (portfolio governance, docs/14). Toute réactivation rétablit le
gating CI dans la même PR.

## Conséquences

- CI plus courte et plus honnête : tout ce qui est gaté correspond à une
  surface vivante.
- Le prototype reste lisible pour informer la conception v0.9.x.
- Aucune surface CLI, gate, contrat ou claim public n'est affecté
  (le control-plane n'était référencé par aucun d'eux).

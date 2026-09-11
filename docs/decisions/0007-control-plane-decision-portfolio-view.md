# ADR-0007: Sortie d'archive du control-plane — la vue multi-projets est câblée, le reste est retiré

Status: accepted
Date: 2026-09-06
Owner: Nomos core team
Issue: NRT-022 (#670) · Ferme: [ADR-0006](0006-control-plane-archive.md) (condition de réveil remplie)

## Contexte

ADR-0006 a gelé `control-plane/` (trois packages Go exploratoires : `dashboard`,
`registry`, `storage`) avec une condition de réveil : une issue *capability
claim* déclarant un caller de production, au jalon v0.9.x. NRT-019 (#667) a
livré ce caller — `nomos portfolio` — et la vague v0.9 (docs/29) demande de
trancher : câbler ou retirer, sans laisser de code archivé non testé.

## Examen des trois packages

| Package | Ce qu'il faisait | Consommateur réel possible | Décision |
|---|---|---|---|
| `dashboard` | vue multi-projets à partir de manifestes `nomos.project.yaml` et d'exceptions | oui : les mêmes artefacts que `product-check` et le strict gate | **câblé** |
| `registry` | registre en mémoire de projets et d'exécutions (statuts, historique) | non : les exécutions sont des runs CI et des artefacts commités, déjà tracés par git et les workflows | **retiré** |
| `storage` | dépôt fichier de rapports/attestations avec rétention | non : les artefacts vivent dans le dépôt, les artefacts CI et les bundles candidats (#639) | **retiré** |

## Décision

1. La logique du `dashboard` est **portée** dans `cli/internal/portfolio/projects.go`
   et exposée par `nomos portfolio projects --project <nomos.project.yaml>
   [--exceptions <f>] ...` avec filtres exacts (verdict, stack, risque, owner).
   Le portage lit le **vrai** schéma d'exceptions (`cli/internal/exceptions`),
   marque `undated` une exception sans expiration au lieu de la traiter comme
   valide, refuse une date inanalysable ou une exception sans id, compte les
   exceptions expirées dans le résumé, et nomme le hash de chaque manifeste.
   Le gating CI est rétabli de fait : le package `portfolio` est dans `go test ./...`.
2. `control-plane/` est **supprimé** (dashboard porté, registry et storage sans
   consommateur). Une sonde `must_be_absent` sur le registre de capacités
   maintient l'absence ; la carte du dépôt, `nomos.project.yaml`, RELEASE.md et
   docs/14 sont mis à jour.
3. ADR-0006 est clos par cette ADR.

## Ce que cette décision ne dit pas

Aucun control plane hébergé, API, authentification, rétention ou multi-tenant
n'existe ni n'est promis. `docs/regulated/control-plane/multi-corpus-roadmap.yaml`
(DOR-022) reste la direction entreprise ; la baseline CLI-first en est la
source de vérité, et cette vue en est une réalisation.

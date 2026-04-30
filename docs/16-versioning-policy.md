# 16 - Versioning Policy

## Objectif

Nomos doit versionner differemment ce qui ne casse pas les memes consommateurs. Le coeur, les adapters, les schemas et les policies n'ont pas le meme rythme ni le meme impact.

## Objets Versionnes

Nomos versionne explicitement :

- le coeur produit ;
- les adapters ;
- les schemas ;
- les policies ;
- les formats d'attestation ;
- les SDK.

## Regle Generale

SemVer est la regle par defaut :

- `MAJOR` : rupture de compatibilite ;
- `MINOR` : capacite nouvelle compatible ;
- `PATCH` : correction ou clarifications compatibles.

## 1. Core Product

Le coeur produit correspond au CLI `nomos` et au moteur d'execution.

Exemples de bump `MAJOR` :

- changement incompatible du format `nomos-report.json` ;
- modification du contrat d'un adapter ;
- changement de semantics sur les verdicts (`partial`, `blocked`, etc.).

Exemples de bump `MINOR` :

- nouvelle commande ;
- nouveau mode de sortie compatible ;
- nouveau type d'evidence optionnel.

Exemples de bump `PATCH` :

- bugfix ;
- message d'erreur plus precis ;
- correction d'un faux positif sans impact de contrat.

## 2. Adapters

Chaque adapter a sa propre version.

Format recommande :

`<adapter-name>@<semver>`

Exemples :

- `node-typescript@0.4.0`
- `python@0.3.1`
- `jvm@0.2.0`

Chaque adapter doit declarer :

- sa version ;
- la version minimale du coeur compatible ;
- les features supportees ;
- les limites connues.

Un adapter peut evoluer sans forcer un bump du coeur si son contrat public ne change pas.

### Contrat De Manifeste Adapter

Le contrat public d'un adapter est son manifeste `adapter.nomos.yaml`, valide
contre `specs/adapter-manifest.cue` (`#AdapterManifest`). Le manifeste fait
partie de l'API publique de l'adapter au meme titre que ses commandes.

Champs publics versionnes :

- `adapter.version` ;
- `compatibility.nomos_core.min_version` et `max_version` ;
- `compatibility.manifest_contract.version` ;
- `stack_support` ;
- `capabilities.provides` ;
- `commands` ;
- `limitations` ;
- `test_contract`.

Regles de bump propres au manifeste :

- `MAJOR` : suppression ou renommage d'une capability, commande, surface,
  evidence kind ou output kind ; changement incompatible de protocole de
  commande ;
- `MINOR` : ajout compatible d'une capability, surface, framework, commande
  optionnelle ou evidence kind ;
- `PATCH` : correction de detection, documentation ou metadata sans changement
  de champ public.

Une capability `experimental` peut etre ajoutee en `MINOR`; une capability
`stable` ne peut pas changer de signification sans bump `MAJOR`.

## 3. Schemas

Les schemas Nomos doivent etre versionnes separément du coeur.

Schemas cibles :

- `nomos.project.yaml`
- `source-manifest`
- `canonical-matrix`
- `nomos-report.json`

Regles :

- tout champ supprime ou renomme => `MAJOR` ;
- tout champ optionnel ajoute => `MINOR` ;
- correction d'un exemple ou d'une annotation => `PATCH`.

Le coeur doit annoncer quelles versions de schemas il sait lire et ecrire.

## 4. Policies

Les policies Rego/CUE doivent etre versionnees car elles influencent les gates.

Regles :

- une policy plus stricte pouvant bloquer un repo auparavant vert doit etre consideree au minimum comme un changement `MINOR`, voire `MAJOR` si le comportement attendu change fortement ;
- un bugfix de policy sans changement de semantics attendue peut rester `PATCH`.

Les pipelines doivent pouvoir pinner une version de policy.

## 5. Attestations

Les formats d'attestation doivent etre versionnes explicitement et porter leur version dans le payload.

Regles :

- nouvelle forme de predicate compatible => `MINOR` ;
- modification incompatible du payload => `MAJOR`.

## 6. SDK

Les SDK suivent SemVer et declarent la version du coeur et des schemas avec lesquels ils ont ete testes.

## Compatibility Matrix

Nomos doit publier une matrice de compatibilite entre :

- coeur ;
- adapters ;
- schemas ;
- policies ;
- SDK.

Exemple conceptuel :

| Core | Schema | Adapter | Policy | Status |
|---|---|---|---|---|
| 0.4.x | 0.2.x | node-typescript 0.3.x | 0.2.x | supported |
| 0.4.x | 0.1.x | python 0.1.x | 0.1.x | deprecated |

## Deprecation Policy

Regles minimales :

- une deprecation doit etre annoncee avant suppression ;
- au moins une version `MINOR` doit exister entre deprecation et retrait, sauf faille critique ;
- les messages CLI doivent signaler les formats deprecies ;
- les examples du repo ne doivent pas rester sur des formats deprecies.

## Release Discipline

Chaque release du coeur doit publier :

- version du coeur ;
- versions des schemas supportees ;
- adapters verifies ;
- policies de reference ;
- changements incompatibles ;
- migrations necessaires.

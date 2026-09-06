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

La matrice est calculee par `nomos contracts status --emit-docs` depuis `specs/contract-registry.yaml` et `adapters/*/adapter.nomos.yaml` ; `nomos version --json` annonce les memes donnees. Un adapter hors plage rougit `nomos contracts status` ; une edition manuelle de la section rougit la CI.

<!-- compatibility-matrix:begin -->
<!-- GENERATED from specs/contract-registry.yaml and adapters/*/adapter.nomos.yaml by `nomos contracts status --emit-docs`; do not edit by hand, CI fails on drift -->

Core `0.2.0-ALPHA` — 39 contract(s) registered, 15 stable. `reads`/`writes` = a Go reader/writer is declared in the registry.

| Contract | Version | Stability | Core reads | Core writes |
|---|---|---|---|---|
| `adapter-manifest` | `0.1.0` | experimental | yes | no |
| `alcoa-evidence` | `0.1.0` | experimental | yes | no |
| `canon-promotion` | `0.1.0` | stable | yes | no |
| `canonical-knowledge-bundle` | `ckm-bundle-v1` | stable | yes | no |
| `canonical-matrix` | `0.1.0` | stable | yes | no |
| `corpus-body-ledger` | `0.1.0` | stable | yes | yes |
| `corpus-integrity-report` | `0.1.0` | stable | yes | no |
| `domain-pack` | `nomos-domain-pack-v1` | stable | yes | no |
| `evidence-contract` | `0.1.0` | experimental | yes | no |
| `external-snapshot` | `nomos.external-snapshot.v1` | stable | yes | yes |
| `facet-ontology` | `ckm-facet-ontology-v1` | experimental | yes | no |
| `facets` | `0.1.0` | stable | yes | no |
| `knowledge-lens` | `0.1.0` | stable | yes | no |
| `nomos-github-workflow` | `` | experimental | yes | no |
| `nomos-praxis-evidence` | `nomos-praxis-evidence-exchange-v1` | experimental | yes | no |
| `nomos-praxis-evidence-schema` | `nomos-praxis-evidence-exchange-v1` | stable | no | no |
| `nomos-project` | `0.1.0` | stable | yes | no |
| `nomos-report.schema` | `0.1.0` | stable | yes | yes |
| `nomos-trace-manifest` | `` | experimental | yes | no |
| `point-in-time` | `0.1.0` | stable | yes | no |
| `portfolio-status` | `nomos-portfolio-status-v1` | experimental | yes | yes |
| `rbok-lawbook-feed` | `0.1.0` | experimental | yes | no |
| `source-manifest` | `0.1.0` | stable | yes | yes |
| `validation-inventory` | `0.1.0` | experimental | yes | no |
| `verdicts` | `0.1.0` | stable | yes | no |

| Adapter | Version | Status | Core range | Manifest contract | Schema minimums | Verdict |
|---|---|---|---|---|---|---|
| `jvm` | `0.1.0` | experimental | >= 0.1.0 | `0.1.0` | adapter_manifest 0.1.0, canonical_matrix 0.1.0, nomos_project 0.1.0, source_manifest 0.1.0 | compatible |
| `node-typescript` | `0.1.0` | experimental | >= 0.1.0 | `0.1.0` | adapter_manifest 0.1.0, canonical_matrix 0.1.0, nomos_project 0.1.0, source_manifest 0.1.0 | compatible |
| `python` | `0.1.0` | experimental | >= 0.1.0 | `0.1.0` | adapter_manifest 0.1.0, canonical_matrix 0.1.0, nomos_project 0.1.0, source_manifest 0.1.0 | compatible |

Other formats the core emits: attestation_predicate `https://nomos.dev/attestation/v1`, claim_boundary_predicate `https://nomos.dev/claim-boundary/v1`, contract_status `nomos-contract-status-v1`, portfolio_status `nomos-portfolio-status-v1`, release_candidate_gates `nomos-release-candidate-gates-v1`, release_candidate_spec `nomos-release-candidate-spec-v1`, slsa_provenance_predicate `https://slsa.dev/provenance/v1`, version_announcement `nomos-version-announcement-v1`.

<!-- compatibility-matrix:end -->

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

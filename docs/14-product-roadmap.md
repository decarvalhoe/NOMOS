# 14 - Product Roadmap Nomos v0.1 -> v1.0

## Positionnement

Nomos ne doit pas rester un corpus de doctrine et de templates. Le produit cible est une plateforme de verification Canonical-First capable :

- d'admettre ou refuser un projet selon son scope et sa verifiabilite ;
- de detecter les surfaces produit et les stacks presentes ;
- d'executer des checks reproductibles ;
- de produire des preuves machine-readable ;
- de bloquer une promotion quand l'evidence est insuffisante.

La cible n'est pas "fonctionne sur n'importe quel projet". La cible est :

> fonctionne sur tout projet qui passe l'admission, et prouve quand il fonctionne.

## Principes Produit

1. Fail-closed par defaut.
2. Scope explicite avant toute promesse de couverture.
3. Separation stricte entre methode, adapters, policy, attestations et control plane.
4. Les preuves doivent etre exportables, signables et verifiables hors de Nomos.
5. Les integrations stack sont versionnees comme des adapters, pas codees en dur dans le coeur.

## Architecture Cible

### 1. Spec Layer

Artefacts de verite :

- `nomos.project.yaml`
- `docs/canonical/source-manifest.yaml`
- `docs/canonical/<domain>-matrix.yaml`
- contrats canoniques
- schemas derives

Modele recommande :

- source primaire de contraintes en CUE ;
- export JSON Schema pour l'interoperabilite ;
- export OpenAPI / AsyncAPI quand la surface l'exige.

### 2. Core CLI Layer

CLI `nomos` distribue comme binaire autonome.

Commandes cibles :

- `nomos init`
- `nomos validate`
- `nomos diagnose`
- `nomos admit`
- `nomos sources check`
- `nomos matrix check`
- `nomos contracts check`
- `nomos product-check`
- `nomos strict`
- `nomos report`
- `nomos attest`

### 3. Adapter Layer

Adapters versionnes par stack et surface :

- Node / TypeScript
- Python
- JVM
- Go
- .NET
- infra
- data
- event-driven

Chaque adapter declare :

- heuristiques de detection ;
- surfaces supportees ;
- patterns interdits ;
- points d'extraction de provenance ;
- commandes de verification ;
- limites connues.

### 4. Policy Layer

Policies executables pour gates CI/CD :

- regles de scope ;
- regles de completude ;
- regles de fuite sample/mock ;
- regles de provenance ;
- regles d'exception expirante.

### 5. Evidence Layer

Sorties machine-readable minimales :

- `nomos-report.json`
- `nomos-report.md`
- attestations in-toto
- provenance SLSA
- signatures cosign
- inventaires SPDX / CycloneDX

### 6. Control Plane

Service optionnel pour grande echelle :

- registre projets ;
- historique d'executions ;
- statut des exceptions ;
- vues portfolio ;
- evidence browser.

## Release Train

### v0.1 - Core Spec

Objectif :

figer le meta-modele universel minimal.

Livrables :

- schema `nomos.project.yaml` ;
- schema source manifest en CUE ;
- schema canonical matrix en CUE ;
- taxonomie de scope ;
- taxonomie d'evidence ;
- codes d'erreurs standard.

Gate :

un projet peut etre valide hors ligne sur ses manifests et recevoir un statut d'admission theorique.

### v0.2 - CLI Minimal

Objectif :

rendre le meta-modele executable localement.

Livrables :

- binaire `nomos` ;
- `init` ;
- `validate` ;
- `diagnose` heuristique ;
- format de sortie stable JSON ;
- contrat d'erreurs stable.

Gate :

les exemples Nomos sont validables de bout en bout sans service externe.

### v0.3 - Admission Engine

Objectif :

decider si Nomos peut s'appliquer, et avec quel niveau de confiance.

Livrables :

- detection stack ;
- detection surfaces ;
- scoring de verifiabilite ;
- profils greenfield / brownfield / regulated / low-risk ;
- rapport d'admission motive.

Gate :

au moins 10 repos de reference sont classes correctement avec raisons explicites.

### v0.4 - Adapters v1

Objectif :

supporter reellement plusieurs stacks.

Livrables :

- adapters Node / TypeScript, Python, JVM ;
- parsing Tree-sitter ;
- detection de patterns interdits ;
- conventions d'integration par stack.

Gate :

`nomos product-check` detecte de vraies fuites et ne repose plus sur des regex seules.

### v0.5 - Canonical Checks

Objectif :

couvrir source -> matrix -> contracts -> product.

Livrables :

- `sources check` ;
- `matrix check` ;
- `contracts check` ;
- references croisees ;
- `strict` ;
- rapport couverture/gaps.

Gate :

aucun cas de regression prepare ne sort vert par erreur.

### v0.6 - Brownfield Migration Pack

Objectif :

permettre l'adoption sur legacy sans big-bang rewrite.

Livrables :

- characterization templates ;
- generation initiale de backlog de gaps ;
- mode `partial` ;
- workflow strangler ;
- import semi-assiste des sources.

Gate :

un repo legacy peut entrer dans Nomos avec un verdict `partial` et une trajectoire claire de remediations.

### v0.7 - CI/CD And Policy

Objectif :

rendre les gates opposables dans les pipelines.

Livrables :

- GitHub Action / GitLab job generiques ;
- policies Rego et/ou CUE ;
- exceptions expirantes ;
- annotations PR ;
- severite standardisee.

Gate :

une PR non conforme est bloquee avec messages de correction exploitables.

### v0.8 - Provenance And Attestations

Objectif :

passer de checks locaux a une chaine de preuve transportable.

Livrables :

- modele d'attestation Nomos ;
- export in-toto ;
- provenance SLSA ;
- signature cosign ;
- exports SPDX / CycloneDX.

Gate :

une release Nomos peut etre verifiee hors de la machine qui l'a produite.

### v0.9 - Control Plane

Objectif :

piloter un portefeuille de projets.

Livrables :

- registry projets ;
- stockage rapports et attestations ;
- dashboard multi-projets ;
- historique ;
- API de consultation.

Gate :

la direction technique peut connaitre a tout moment quels projets sont `in_scope`, `partial`, `blocked` ou `out_of_scope`.

### v1.0 - Productized Platform

Objectif :

livrer une plateforme stable, documentee, adoptee sur plusieurs projets.

Livrables :

- SDK stable ;
- adapters versionnes ;
- 3 implementations de reference ;
- matrice de compatibilite ;
- guide operatoire complet ;
- audit de robustesse.

Gate :

Nomos opere sur plusieurs projets greenfield et brownfield avec preuves signees et limites declarees.

## Ordre Exact D'Implementation

1. Figer le meta-modele.
2. Choisir CUE comme source de verite des schemas.
3. Construire le CLI minimal.
4. Construire `diagnose` puis `admit`.
5. Ajouter Tree-sitter et la detection repo/surfaces.
6. Livrer trois adapters prioritaires.
7. Implementer `sources`, `matrix`, `contracts`, `product-check`, `strict`.
8. Integrer les policies dans CI/CD.
9. Ajouter le pack brownfield.
10. Ajouter attestations et signatures.
11. Construire le control plane.
12. Stabiliser SDK, adapters, references et compatibilite v1.

## Ce Qu'il Ne Faut Pas Faire

- construire le control plane avant l'admission ;
- annoncer un support "any stack" avant d'avoir des adapters versionnes ;
- signer des artefacts sans verifier la chaine d'evidence ;
- confondre validation de schema et validite metier ;
- utiliser le LLM comme calculateur de regles critiques.

## Definition De Reussite v1.0

Nomos v1.0 est atteint si :

- le produit sait refuser un projet hors scope avec raisons precises ;
- le produit sait admettre un projet en scope avec niveau de confiance explicite ;
- les checks sont reproductibles localement et en CI ;
- les adapters declarent leurs limites ;
- les preuves sont exportables et verifiables hors de Nomos ;
- le statut d'un portefeuille de projets est observable sans inspection manuelle repo par repo.

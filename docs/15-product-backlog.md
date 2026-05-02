# 15 - Product Backlog Nomos

## Regles De Priorisation

1. Ne jamais commencer un epic aval tant que son predecesseur critique n'est pas stable.
2. Chaque issue doit produire un artefact testable.
3. Toute promesse de generalisation doit etre supportee par un adapter ou une preuve de non-applicabilite.
4. Toute issue qui touche au verdict produit doit sortir une evidence machine-readable.

## Legende

- `BLOCKS` : dependance bloquante.
- `ENABLES` : ce que l'issue debloque ensuite.
- `DoD` : definition of done.

## EPIC E0 - Product Foundations

But :

poser les conventions du produit Nomos lui-meme.

### NOM-001 - Creer l'arborescence produit

Description :

creer les dossiers cibles `cli/`, `adapters/`, `policies/`, `attestations/`, `sdk/`, `control-plane/`, `specs/`.

BLOCKS :

aucune.

ENABLES :

E1, E2, E3, E4.

DoD :

- arborescence creee ;
- README racine mis a jour ;
- conventions de nommage documentees.

### NOM-002 - Definir la politique de versionning

Description :

formaliser versionning du coeur, des adapters, des schemas et des policies.

BLOCKS :

aucune.

ENABLES :

E1, E3, E8.

DoD :

- document de versionning publie ;
- compatibilite ascendante/descendante definie ;
- regles de deprecation explicites.

## EPIC E1 - Universal Spec

But :

figer le meta-modele Nomos.

### NOM-101 - Specifier `nomos.project.yaml`

Description :

decrire projet, scope, surfaces, owners, niveau de risque, commandes de build/test, modes greenfield/brownfield.

BLOCKS :

NOM-001.

ENABLES :

NOM-201, NOM-301, NOM-401.

DoD :

- schema logique documente ;
- exemples minimal, brownfield et regulated fournis ;
- champs obligatoires et optionnels justifies.

### NOM-102 - Convertir `source-manifest` en schema CUE

BLOCKS :

NOM-001.

ENABLES :

NOM-202, NOM-501.

DoD :

- schema CUE compile ;
- exemples valides ;
- erreurs de validation parlantes.

### NOM-103 - Convertir `canonical-matrix` en schema CUE

BLOCKS :

NOM-001.

ENABLES :

NOM-203, NOM-502.

DoD :

- schema CUE compile ;
- contraintes de references exprimees ;
- exemples valides et invalides fournis.

### NOM-104 - Definir la taxonomie des verdicts

Description :

definir `in_scope`, `partial`, `blocked`, `out_of_scope`, ainsi que les niveaux de confiance.

BLOCKS :

NOM-101.

ENABLES :

NOM-302, NOM-701.

DoD :

- table des verdicts publiee ;
- regles d'escalade documentees ;
- exemples de cas limites.

### NOM-105 - Definir le format `nomos-report.json`

BLOCKS :

NOM-101, NOM-104.

ENABLES :

NOM-204, NOM-503, NOM-804.

DoD :

- schema JSON du report publie ;
- severites, codes erreur et evidence types inclus ;
- exemple complet fourni.

## EPIC E2 - CLI Core

But :

rendre le produit executable.

### NOM-201 - Scaffold du CLI Go

BLOCKS :

NOM-001, NOM-002.

ENABLES :

toutes les issues du CLI.

DoD :

- binaire lanceable ;
- structure de commandes creee ;
- tests de base du CLI verts.

### NOM-202 - Implementer `nomos init`

BLOCKS :

NOM-102, NOM-103, NOM-201.

ENABLES :

NOM-204, adoption greenfield.

DoD :

- initialise un repo vide ;
- pose manifests et dossiers attendus ;
- supporte mode minimal et mode regulated.

### NOM-203 - Implementer `nomos validate`

BLOCKS :

NOM-102, NOM-103, NOM-201.

ENABLES :

NOM-501, NOM-502.

DoD :

- valide les manifests ;
- retourne erreurs structurees ;
- zero faux positif sur les exemples officiels.

### NOM-204 - Implementer sorties standard JSON/Markdown

BLOCKS :

NOM-105, NOM-201.

ENABLES :

NOM-302, NOM-503, NOM-804.

DoD :

- sortie JSON stable ;
- sortie Markdown lisible ;
- test de compatibilite snapshot.

## EPIC E3 - Admission Engine

But :

decider si Nomos peut s'appliquer et jusqu'ou.

### NOM-301 - Detecter langages, outils et surfaces

Description :

ajouter detection repo, langages, CI, API, UI, DB, infra, docs.

BLOCKS :

NOM-201.

ENABLES :

NOM-302, E4.

DoD :

- detection sur corpus de repos de test ;
- surfaces au minimum `api`, `ui`, `worker`, `data`, `infra`, `docs` ;
- rapport de detection exporte.

### NOM-302 - Implementer `nomos diagnose`

BLOCKS :

NOM-104, NOM-204, NOM-301.

ENABLES :

NOM-303, NOM-701.

DoD :

- produit verdict preliminaire ;
- liste les blocants ;
- liste les preuves manquantes ;
- classification deterministic sur repos de reference.

### NOM-303 - Implementer `nomos admit`

BLOCKS :

NOM-302.

ENABLES :

E5, E6, industrialisation brownfield.

DoD :

- verdict finalise avec niveau de confiance ;
- refuse explicitement hors scope ;
- supporte `partial` avec plan de remediations.

## EPIC E4 - Polyglot Detection And Adapters

But :

sortir du mode artisanal repo-par-repo.

### NOM-401 - Integrer Tree-sitter

BLOCKS :

NOM-201.

ENABLES :

NOM-402, NOM-403, NOM-404.

DoD :

- integration testee sur plusieurs langages ;
- erreur claire quand grammaire absente ;
- perf acceptable sur repo moyen.

### NOM-402 - Adapter Node / TypeScript v1

BLOCKS :

NOM-301, NOM-401.

ENABLES :

NOM-601.

DoD :

- detection routes, services, mocks, fixtures, hardcoded catalogues ;
- conventions frontend/backend documentees ;
- fixtures de test officielles.

### NOM-403 - Adapter Python v1

BLOCKS :

NOM-301, NOM-401.

ENABLES :

NOM-601.

DoD :

- detection services, routers, serializers, fixtures ;
- conventions FastAPI/Django/Flask minimales couvertes ;
- fixtures de test officielles.

### NOM-404 - Adapter JVM v1

BLOCKS :

NOM-301, NOM-401.

ENABLES :

NOM-601.

DoD :

- detection controllers, services, DTOs, fixtures ;
- conventions Spring prioritaires couvertes ;
- fixtures de test officielles.

### NOM-405 - Declarer le contrat d'un adapter

BLOCKS :

NOM-002, NOM-201.

ENABLES :

extensibilite v1.

DoD :

- format de manifeste adapter publie ;
- versionning et capacites declares ;
- compatibilite CLI/adapters testee.

## EPIC E5 - Canonical Checks

But :

rendre les invariants Nomos executables.

### NOM-501 - Implementer `nomos sources check`

BLOCKS :

NOM-102, NOM-203.

ENABLES :

NOM-504, NOM-701.

DoD :

- verifie presence, hash, owner, statut, allowed uses ;
- codes erreur par source ;
- exemples invalides couverts.

### NOM-502 - Implementer `nomos matrix check`

BLOCKS :

NOM-103, NOM-203.

ENABLES :

NOM-504, NOM-701.

DoD :

- verifie references vers sources, contrats, tests, refs code ;
- remonte unites invalides et gaps ;
- score de couverture calcule.

### NOM-503 - Implementer `nomos report`

BLOCKS :

NOM-204, NOM-501, NOM-502.

ENABLES :

NOM-703, NOM-804.

DoD :

- genere `nomos-report.json` ;
- genere `coverage-report.md` ;
- resume exploitable pour CI et humain.

### NOM-504 - Implementer `nomos contracts check`

BLOCKS :

NOM-501, NOM-502.

ENABLES :

NOM-601, NOM-701.

DoD :

- verifie schemas, refs croisees, source refs obligatoires ;
- supporte CUE et export JSON Schema ;
- cas invalides bloques.

## EPIC E6 - Product Checks

But :

inspecter le code produit et detecter les contournements.

### NOM-601 - Implementer `nomos product-check`

BLOCKS :

NOM-402, NOM-403, NOM-404, NOM-504.

ENABLES :

NOM-602, NOM-701.

DoD :

- detecte imports interdits ;
- detecte listes metier hardcodees ;
- detecte usage produit de mocks/samples/fixtures ;
- evidence liee a des fichiers et symboles.

### NOM-602 - Implementer `nomos strict`

BLOCKS :

NOM-503, NOM-601.

ENABLES :

NOM-703, NOM-801.

DoD :

- agrege tous les checks bloquants ;
- supporte severites et exceptions ;
- retour non-zero sur blocage.

## EPIC E7 - Brownfield Migration

But :

permettre l'adoption sur existant.

### NOM-701 - Mode `partial`

BLOCKS :

NOM-303, NOM-501, NOM-502, NOM-504.

ENABLES :

NOM-702, pilotage brownfield.

DoD :

- partial ne cache jamais les gaps ;
- verdict accompagne d'un plan de fermeture ;
- blocants critiques restent bloquants.

### NOM-702 - Generer un backlog de remediations

BLOCKS :

NOM-701, NOM-602.

ENABLES :

industrialisation de migration.

DoD :

- backlog trie par criticite ;
- chaque gap pointe vers source, code ou contrat ;
- export Markdown et JSON.

### NOM-703 - Templates characterization et strangler

BLOCKS :

NOM-503, NOM-602.

ENABLES :

mise en oeuvre brownfield.

DoD :

- templates de tests fournis ;
- workflow de migration documente ;
- exemple legacy inclus.

## EPIC E8 - CI/CD And Attestations

But :

rendre les verdicts opposables et verifiables hors machine locale.

### NOM-801 - Integration GitHub/GitLab

BLOCKS :

NOM-602.

ENABLES :

NOM-802, NOM-804.

DoD :

- action/job reusable fourni ;
- annotations PR disponibles ;
- mode fail-closed parametre.

### NOM-802 - Exceptions expirantes

BLOCKS :

NOM-801.

ENABLES :

adoption encadree.

DoD :

- format d'exception defini ;
- expiration obligatoire ;
- owner obligatoire ;
- check bloque si exception expiree.

### NOM-803 - Attestations in-toto / SLSA / cosign

BLOCKS :

NOM-602.

ENABLES :

NOM-804, v1 preuve forte.

DoD :

- attestation generee ;
- verification hors machine documentee ;
- signature testee.

### NOM-804 - Exports SPDX / CycloneDX

BLOCKS :

NOM-105, NOM-503, NOM-803.

ENABLES :

interoperabilite ecosysteme.

DoD :

- export reussi ;
- artefacts lies au report Nomos ;
- guide de consommation publie.

## EPIC E9 - Control Plane

But :

superviser plusieurs projets.

### NOM-901 - Registry projets

BLOCKS :

NOM-303, NOM-503.

ENABLES :

NOM-902, NOM-903.

DoD :

- enregistrement d'un projet et de ses executions ;
- statut courant visible ;
- historique minimal conserve.

### NOM-902 - Stockage des reports et attestations

BLOCKS :

NOM-803, NOM-901.

ENABLES :

NOM-903.

DoD :

- consultation d'une execution complete ;
- evidence reliee a une version ;
- retention definie.

### NOM-903 - Dashboard portefeuille

BLOCKS :

NOM-901, NOM-902.

ENABLES :

v1 pilotage organisation.

DoD :

- vue `in_scope/partial/blocked/out_of_scope` ;
- filtres par stack, criticite, owner ;
- exceptions visibles.

## Chemin Critique

Le chemin critique a respecter est :

1. E0 Product Foundations
2. E1 Universal Spec
3. E2 CLI Core
4. E3 Admission Engine
5. E4 Adapters v1
6. E5 Canonical Checks
7. E6 Product Checks
8. E8 CI/CD And Attestations
9. E7 Brownfield Migration
10. E9 Control Plane

Raison :

- sans E1/E2, rien n'est executable ;
- sans E3, aucune promesse de support n'est credible ;
- sans E4, pas de generalisation reelle ;
- sans E5/E6, pas de gate metier solide ;
- sans E8, pas de preuve forte ;
- E7 et E9 doivent s'appuyer sur un coeur deja fiable.

## Definition Of Done v1.0

Nomos v1.0 est considere pret si :

- au moins 3 stacks sont supportees via adapters v1 ;
- `diagnose`, `admit`, `strict` et `attest` sont stables ;
- un projet hors scope est refuse correctement ;
- un projet brownfield peut entrer en mode `partial` avec backlog genere ;
- une execution produit un report, une attestation et une verification reproductible ;
- la chaine CI peut bloquer une promotion de facon explicable ;
- le control plane peut afficher le statut d'un portefeuille de projets.

## Track Regulated-Grade

Ce track etend E8/E9. Il ne remplace pas le backlog produit : il ajoute les exigences necessaires pour que Nomos puisse defendre ses propres promesses sur un marche IT regule.

Reference d'implementation : `docs/23-regulated-implementation-plan.md`.

### RG-0 - Baseline Executable

But :

sortir de l'etat `NQ-0/NQ-1` en rendant le produit verifiable avant tout claim regule.

Issues GitHub :

- Nomos #125 : restore CLI build.
- Nomos #126 : fix CUE schema validation.
- Nomos #127 : wire real RBOK lawbook CLI.
- Nomos #136 : release gate for RBOK lawbook profile.
- Nomos #137 : Nomos-on-Nomos self-compliance.

BLOCKS :

RG-1, RG-2, RG-3, claim governance public.

DoD :

- `go test ./...` et `go vet ./...` verts pour le CLI cible ;
- `cue vet ./...` vert ;
- commandes inconnues en sortie non-zero ;
- pas de fallback dummy produisant une evidence verte ;
- corpus commands exposees et testees.

### RG-1 - Regulated Control Matrix

But :

gouverner les references externes, controles, claims, evidences et waivers.

Issues GitHub :

- Nomos #138 : parent epic regulated-grade.
- Nomos #139 : regulated control matrix/reference registry.
- Nomos #140 : external reference alignment gate.
- Nomos #145 : public claims governance.

BLOCKS :

RG-2, RG-4, NQ-3.

DoD :

- schema `specs/regulated-control-matrix.cue` ;
- exemple Nomos valide ;
- scan des URLs/docs/specs ;
- echec si reference ou claim non gouverne ;
- waivers avec owner, risque, mitigation et expiration ;
- README et roadmap limites au niveau de preuve atteint.

### RG-2 - Nomos Self-Compliance

But :

faire passer Nomos a `NQ-3`.

Issues GitHub :

- Nomos #141 : regulated self-compliance expansion.
- Nomos #142 : ALCOA+ evidence model.
- Nomos #143 : validation lifecycle pack.
- Nomos #146 : AI/RAG risk controls.
- Nomos #147 : SLSA/in-toto provenance gate.

BLOCKS :

RG-3, RG-4, NQ-4.

DoD :

- `mode: nomos_product` admis ;
- `nomos compliance self-check` executable ;
- rapport ALCOA+ avec actor, command, timestamp, repo, commit, hashes ;
- `docs/validation/` initial ;
- self-compliance CI fail-closed ;
- controles critiques non waives ou waives avec expiration et justification.

### RG-3 - RBOK Lawbook Proof

But :

prouver Nomos sur `realisons-business/01_rbok` en lecture seule, comme corpus metier de reference.

Issues GitHub :

- Nomos #124 : RBOK lawbook viability epic.
- Nomos #128 : real RBOK E2E.
- Nomos #129 : contract normalization.
- Nomos #130 : Markdown extraction.
- Nomos #131 : parcours YAML governance.
- Nomos #132 : source policy.
- Nomos #133 : multi-doc feed/RAG/engine.
- Nomos #134 : read-only hardening.
- Nomos #135 : Windows confidence.

BLOCKS :

RG-4 evidence chain, Realisons reference demo.

DoD :

- `mode: canonical_corpus` produit `corpus_admissible`, `corpus_partial` ou `corpus_blocked` ;
- source `01_rbok` jamais modifiee ;
- manifest, sidecar, chunks lawbook, index, RAG metadata, attestation et lockfile generes hors corpus ;
- metadata de gouvernance existante preservee ;
- git status source avant/apres prouve ;
- Windows smoke vert.

### RG-4 - Nomos/Praxis Evidence Chain

But :

faire passer la chaine combinee a `NQ-4`.

Issues GitHub :

- Nomos #144 : shared evidence contract.
- Praxis #247 : compatibility epic.
- Praxis #248 : Nomos project pack.
- Praxis #249 : Nomos compliance invariants.
- Praxis #250 : consume shared evidence contract.
- Praxis #251 : feed Praxis CAPA back into Nomos controls.
- Praxis #252 : cross-repo compatibility smoke test.
- Praxis #253 : regulated parity baseline.

BLOCKS :

RG-5 validation pack, claims "evidence-backed regulated-grade candidate".

DoD :

- schema `nomos-praxis-evidence` publie avec exemples valides/invalides ;
- Nomos produit des claims et controls consommables par Praxis ;
- Praxis produit runtime evidence, invariant results et CAPA consommables par Nomos ;
- smoke cross-repo vert ;
- Nomos release bundle indique le niveau de qualite Praxis utilise.

### RG-5 - Validation Pack And Release Bundle

But :

atteindre `NQ-5` sur un intended use declare.

Issues a convertir depuis `docs/22-nomos-praxis-synergy-market-audit.md` quand RG-1/RG-2 sont acceptes :

- Nomos #149 / SYN-001 : joint evidence ledger contract.
- Nomos #150 / SYN-003 : release evidence bundle format.
- Nomos #151 / SYN-010 : validation inventory/intended-use model.
- Nomos #152 / SYN-008 : e-signature/approval semantics.
- Nomos #153 / SYN-009 : independent review/quality-unit roles.
- Praxis #255 / SYN-011 : validated project pack certification status.
- Praxis #256 / SYN-014 : runtime evidence retention/trend model.

BLOCKS :

NQ-6 independent audit readiness.

DoD :

- validation inventory item ;
- URS/SRS/tests/evidence/release decision traceability ;
- deviation/CAPA log ;
- release bundle reconstructible ;
- SLSA/in-toto provenance ;
- independent review status ;
- retention and reconstruction procedure.

### RG-6 - Market Interoperability And Claims

But :

positionner Nomos/Praxis face aux outils ALM, validation lifecycle, QMS et test management sans surpromesse.

Issues a convertir depuis `docs/22-nomos-praxis-synergy-market-audit.md` :

- Praxis #250 / SYN-002 : Praxis consume shared evidence contract.
- Nomos #154 / SYN-006 : source change impact analysis for required Praxis tests.
- Nomos #155 / SYN-007 : Praxis scenario selection from impacted Nomos claims.
- Nomos #156 / SYN-012 : ReqIF/export/import compatibility decision.
- Nomos #157 / SYN-013 : market positioning/non-claim governance.
- Nomos #158 / SYN-015 : regulated demo reference architecture.
- Praxis #254 / SYN-005 : Praxis-on-Praxis self-compliance gate.

BLOCKS :

commercialisation regulee, integrations entreprise.

DoD :

- decision ReqIF/import/export ;
- claims matrix publique ;
- demo reference architecture ;
- docs de comparaison marche ;
- aucune formulation de certification sans preuve externe.

## Dependency Tree Regulated-Grade

```text
RG-0
  -> RG-1
  -> RG-2
  -> RG-3
  -> RG-4
  -> RG-5
  -> RG-6

NQ-2 = RG-0
NQ-3 = RG-0 + RG-1 + RG-2
NQ-4 = NQ-3 + RG-3 + RG-4 + Nomos #149 + Praxis #247-#252
NQ-5 = NQ-4 + RG-5 + Nomos #150-#153 + Praxis #254-#256
NQ-6 = NQ-5 + independent reconstruction
```

Regle de fermeture :

une issue regulated-grade ne peut pas etre fermee si la documentation associee, les exemples, les schemas et les gates CI ne refletent pas le comportement reel.

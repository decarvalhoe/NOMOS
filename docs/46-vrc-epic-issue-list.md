# 46 — Epic [VRC] Vision-Reality Closure : décomposition en issues atomiques

> Statut : `issue list` (matérialisation du plan [45](45-vision-reality-closure-plan.md)).
> Date : 2026-06-11. Pattern : epic CKM (doc 39) — un epic parent, des issues atomiques,
> garde-fous d'abord. Gouvernance : doc 45 §12 — chaque issue porte 5 lignes
> obligatoires (*ancre vision*, *écart*, *tremplin*, *preuve exigée*, *claim débloqué*)
> plus, quand applicable, la ligne *Registre* (mise à jour attendue de
> `scripts/vrc_wiring_matrix_registry.json`, la matrice G3 faisant foi).
> Écarts E1-E7 : voir la matrice de traçabilité du doc 45 §1.

## Vue d'ensemble epic

| ID | Titre | Type | Phase | Parallèle | Dépend |
|---|---|---|---|---|---|
| VRC-00 | Matrice de câblage générée + guard CI (G3) | garde-fou | 0 | oui (d'abord) | — |
| VRC-01 | Guard claim-boundary étendu aux docs stratégiques + réconciliation doc 40 (G1) | garde-fou | 0 | oui | — |
| VRC-02 | Hygiène numérotation/index docs (G2) | hygiène | 0 | oui | — |
| VRC-03 | Barre d'acceptation : caller de production déclaré (G4) | garde-fou | 0 | oui | — |
| VRC-04 | ADR control-plane : câbler ou archiver (E7) | décision | 0 | oui | — |
| VRC-05 | Owners QMS nommés + records (F1) | QMS | 0 | oui | — |
| VRC-06 | Premier cycle QMS exécuté : review + auto-audit + rétro-CAPA `Sig:""` (F2) | QMS | 0 | non | VRC-05 |
| VRC-07 | Câbler `claim_coverage` + vérification Merkle dans `corpus attest` (F3) | durcissement | 0 | oui | VRC-00 |
| VRC-08 | Caller de production du predicate supply-chain (CKM-05 follow-up) | durcissement | 0 | oui | VRC-00 |
| VRC-09 | Enregistrer ou élaguer les 10 commandes `*Command` orphelines | durcissement | 0 | oui | VRC-00 |
| VRC-10 | Cite-or-abstain dans le moteur Go (A1) | pivot-core | 1 | oui | VRC-00 |
| VRC-11 | Canon promotion dans le moteur Go (A2) | pivot-core | 1 | oui | VRC-00 |
| VRC-12 | Point-in-time dans le moteur Go (A3) | pivot-core | 1 | oui | VRC-00 |
| VRC-13 | Harnais d'évaluation RAG en CI (B2) | mise à niveau | 1 | oui | — |
| VRC-14 | Répétabilité CI d'evidence sur corpus privé (F4) | QMS/evidence | 1 | oui | — |
| VRC-15 | Release `v0.2.0-ALPHA` exécutée via SOP (F5) | QMS/release | 1 | non | VRC-05, VRC-14 |
| VRC-16 | Records formation/compétence (F6) | QMS | 1 | oui | VRC-05 |
| VRC-20 | Contrat de pack domaine (CUE) (D1) | pivot-core | 2 | non | VRC-00 |
| VRC-21 | `nomos pack validate` (gate Go) (D2) | pivot-core | 2 | non | VRC-20 |
| VRC-22 | Pack « EU AI Act evidence » pilot-grade (D3) | pack mince | 2 | non | VRC-21 |
| VRC-23 | Evidence packs en BOM CycloneDX/SPDX (A5) | amplify | 2 | oui | — |
| VRC-30 | Adapter PDF né-numérique, claim ladder (C1) | ingestion | 3 | oui | VRC-00 |
| VRC-31 | Adapter HTML/XML (Fedlex/Akoma Ntoso) (C2) | ingestion | 3 | oui | VRC-00 |
| VRC-32 | Connecteurs CH industrialisés (C4) | ingestion | 3 | oui | — |
| VRC-33 | Kit obligatoire par adapter, enregistrement fail-closed (C5) | garde-fou | 3 | oui | VRC-30 (1ᵉʳ client) |
| VRC-34 | Pack AEC suisse (mince) (D5) | pack mince | 3 | non | VRC-21, VRC-30, VRC-31 |
| VRC-35 | Harnais retrieval de référence + preuve distracteurs (B1) | mise à niveau | 3 | oui | VRC-20 |
| VRC-36 | Kit de conformité consommateur (E-1) | couture | 3 | oui | VRC-20 |
| VRC-37 | Preuve de consommation Aedifica W19 (E-2, cross-repo) | couture | 3 | non | VRC-36 |
| VRC-38 | Métrique de reproductibilité « 0 changement core par pack » (D6) | garde-fou | 3 | oui | VRC-20 |
| VRC-40 | Backend Sigstore keyless (A4) | amplify | 4 | oui | VRC-00 |
| VRC-41 | Adapter DOCX (C3) | ingestion | 4 | oui | VRC-33 |
| VRC-42 | Substrat d'exécution de règles L4/OpenFisca/Catala (B3) | mise à niveau | 4 | oui | — |
| VRC-43 | Graphe de renvois déterministe (B5) | mise à niveau | 4 | oui | — |
| VRC-44 | Outillage vocabulaires SKOS/SHACL/VocBench (B4) | mise à niveau | 4 | oui | VRC-20 |
| VRC-45 | Ontologie facettes portée dans `pack validate` (D4) | pivot-core | 4 | oui | VRC-21 |
| VRC-46 | Bench public cite-or-abstain (sortie chantier A) | amplify | 4 | non | VRC-10, VRC-13 |

Séquencement : garde-fous d'abord (VRC-00 → 03), puis par effet de levier (doc 45 §9).
Zéro régression : CKM-00 reste vert à chaque PR ; tout changement de contrat =
`schema_version` bump + migration.

---

## Phase 0 — Hygiène & déblocage

### VRC-00 — Matrice de câblage générée + guard CI (G3)
- Type : garde-fou · Parallèle : oui (d'abord) · Dépend : —
- Ancre vision : doc 45 §8 G3 ; doctrine §2.3 (« résultat calculé, jamais déclaré ») ; règle RBOK « editing the Actual column without evidence is forbidden ».
- Écart : E1 — classe « implémenté mais non câblé » (#539/#540/#543 : `AtomizeCommand` non enregistré, `ApplyLens` zéro caller, hashes synthétiques) découverte en 2ᵉ passe d'audit au lieu de la PR.
- Tremplin : pattern guard interne (`claim_boundary_guard.py`, #542) — stdlib-only, adversarial-first.
- Preuve exigée : statuts calculés depuis les ancres de l'arbre (jamais déclarés) ; registre déclarant seulement où regarder + statut attendu ; mismatch dans les deux sens → exit 1 ; check générique « toute `*Command` définie est enregistrée ou appelée » ; tests adversariaux : attente `real` forgée sans moteur → rouge ; caller retiré → `partial` → rouge ; moteur Go apparaissant sous une entrée `sidecar` → rouge (« flip the registry ») ; commande non enregistrée (avec doc-comment leurre) → rouge.
- Claim débloqué : « les états PARTIAL sont impossibles à masquer » ; la 2ᵉ passe d'audit devient structurellement inutile.
- Registre : créé (`scripts/vrc_wiring_matrix_registry.json`), artefacts générés sous `.vrc-wiring-matrix/`.
- **Statut : livraison initiale dans la même branche que ce document** (preuve : `tests/test_vrc_wiring_matrix.py` ; câblé dans `ckm-non-regression.sh` étape 6c et `ci.yml` avec `git diff --exit-code`).

### VRC-01 — Guard claim-boundary étendu aux docs stratégiques + réconciliation doc 40 (G1)
- Type : garde-fou · Parallèle : oui · Dépend : —
- Ancre vision : règle d'evidence du [public-claim-boundary](public-claim-boundary.md) ; doc 45 §8 G1.
- Écart : E4 — doc 40 §0/§8 affirme « NOMOS éprouvé et intégré dans plusieurs environnements », non soutenu par l'evidence POC-scoped.
- Tremplin : `claim_boundary_guard.py` existant (étendre les patterns aux claims de maturité/intégration, pas seulement signature).
- Preuve exigée : soit la liste des environnements + records d'intégration est produite, soit la phrase est downgradée ; test adversarial : une phrase forgée « NOMOS est en production chez N clients » dans docs/ → guard rouge.
- Claim débloqué : cohérence interne totale — aucun doc interne ne dépasse le claim boundary.

### VRC-02 — Hygiène numérotation/index docs (G2)
- Type : hygiène · Parallèle : oui · Dépend : —
- Ancre vision : doc 45 §8 G2.
- Écart : collisions `21-` ×2, `43-` ×2, `44-` ×2 ; index racine sans 39-46 (corrigé pour 39-45 le 2026-06-11, à maintenir).
- Tremplin : — (interne).
- Preuve exigée : zéro collision de préfixe ou note d'aliasing explicite par collision ; tous les docs numérotés présents dans `docs/README.md` ; liens internes verts (check de liens en CI souhaitable).
- Claim débloqué : navigabilité documentaire pour l'évaluation externe.

### VRC-03 — Barre d'acceptation : caller de production déclaré (G4)
- Type : garde-fou · Parallèle : oui · Dépend : —
- Ancre vision : doctrine §4 (barre #518) ; doc 45 §8 G4.
- Écart : E1 — les issues CKM revendiquaient des capacités sans déclarer le chemin d'appel de production attendu.
- Tremplin : template d'issue GitHub (`.github/ISSUE_TEMPLATE/`).
- Preuve exigée : template mis à jour avec champ obligatoire « caller de production attendu » ; la revue bloque sans cette ligne ; doc 43 §4 référencé.
- Claim débloqué : le non-câblé est attrapé à la conception, plus seulement à l'audit.

### VRC-04 — ADR control-plane : câbler ou archiver (E7)
- Type : décision · Parallèle : oui · Dépend : —
- Ancre vision : doc 45 §9 P0 ; sobriété (code mort testé contredit la doctrine).
- Écart : E7 — `control-plane/` = 3 packages Go fonctionnels testés en CI, zéro caller, zéro déploiement.
- Tremplin : roadmap v0.9.x (portfolio governance) du doc 14 — si câblage, c'est la cible ; sinon archive propre.
- Preuve exigée : ADR enregistré (docs/decisions/) ; si « archiver » : retrait du job CI + note ; si « câbler » : issue de wiring avec caller déclaré (G4).
- Claim débloqué : carte du repo sans zone grise.

### VRC-05 — Owners QMS nommés + records (F1)
- Type : QMS · Parallèle : oui · Dépend : —
- Ancre vision : quality manual (conditions d'effectivité) ; doc 45 §7 F1.
- Écart : E3 — six rôles `not_assigned`.
- Tremplin : templates existants (`docs/regulated/operations/`).
- Preuve exigée : records d'assignation datés/signés sous `docs/regulated/operations/records/` ; cumul multi-casquettes documenté avec note de conflit d'intérêts ; self-compliance report passe la ligne « Named QMS owners » de Open → established (calculé).
- Claim débloqué : préalable à tout record QMS valide.

### VRC-06 — Premier cycle QMS exécuté (F2)
- Type : QMS · Parallèle : non · Dépend : VRC-05
- Ancre vision : SOPs management review / internal audit / deviation-CAPA ; doc 45 §7 F2.
- Écart : E3 — 0/20 types de records ; chaque SOP avoue son propre vide.
- Tremplin : l'incident `Sig:""` (#542) comme cas d'école : rétro-documenter déviation → CAPA → action corrective (le guard CI) → vérification d'efficacité (test adversarial du guard).
- Preuve exigée : 1 management review enregistrée ; 1 auto-audit avec findings ; 2-3 CAPA réelles dont la rétro-CAPA `Sig:""` fermée avec preuve d'efficacité ; records remplis depuis les templates, datés/signés.
- Claim débloqué : « QMS exécuté » (cycle 1) — précondition de l'offre AI Act (VRC-22).

### VRC-07 — Câbler `claim_coverage` + vérification Merkle dans `corpus attest` (F3)
- Type : durcissement · Parallèle : oui · Dépend : VRC-00
- Ancre vision : README §Roadmap (« claim_coverage dans l'attestation ») ; claim boundary (« not yet wired ») ; dossier RBOK FSQ-08-followup.
- Écart : trouvé par la matrice — `computeClaimCoverage` (cli/internal/corpus/attestation.go) existe et est testé, mais `corpus attest` n'accepte pas le body ledger (caller absent) ; `VerifyMerkleProof` n'a aucun vérificateur de production (tests seulement).
- Tremplin : RFC 9162 (déjà adopté pour les preuves) ; pattern `attest verify` existant.
- Preuve exigée : `nomos corpus attest --corpus-body-ledger <f>` émet `claim_coverage` calculé depuis le ledger ; commande de vérification d'inclusion Merkle exposée ; tests adversariaux : couverture falsifiée → verify rouge ; preuve d'inclusion altérée → rouge ; matrice : `claim_coverage_attestation` et `body_ledger_merkle_verification` passent `partial` → `real`.
- Claim débloqué : lever la ligne « claim_coverage is not yet wired » du claim boundary §What NOMOS Does Not Yet Prove.
- Registre : flip `expected` des deux entrées + ancres callers.

### VRC-08 — Caller de production du predicate supply-chain (CKM-05 follow-up)
- Type : durcissement · Parallèle : oui · Dépend : VRC-00
- Ancre vision : CKM-05 (#500) ; doctrine §2.3 (« mécanique consommée par le moteur »).
- Écart : trouvé par la matrice — `SupplyChainPredicate`/`VerifySupplyChainStatement` (cli/internal/attestation/attestation.go) : définition + tests, **zéro caller de production**.
- Tremplin : in-toto step predicates (déjà adoptés).
- Preuve exigée : émission du predicate supply-chain branchée sur le flux réel (ingestion→canon→embedding) via une commande (`corpus attest --supply-chain` ou `bundle`) ; test adversarial : étape manquante/altérée → verify rouge ; matrice : `supply_chain_attestation` `partial` → `real`.
- Claim débloqué : « attestation supply-chain émise par le pipeline, pas seulement définie ».
- Registre : flip + ancre caller.

### VRC-09 — Enregistrer ou élaguer les 10 commandes `*Command` orphelines
- Type : durcissement · Parallèle : oui · Dépend : VRC-00
- Ancre vision : doctrine §2.3 ; doc 45 §8 G3/G4.
- Écart : trouvé par la matrice (premier run, 2026-06-11) — 10 fonctions `*Command` implémentées dans `cli/internal/app` mais ni enregistrées dans le command map ni appelées, dont les doc-comments revendiquent des surfaces CLI inexistantes : `SourcesCheckCommand` (« nomos sources check »), `ContractsCheckCommand`, `MatrixCheckCommand`, `ExceptionsCheckCommand`, `StrictCommand` (legacy, éclipsé par `StrictGateCommand`), `ProductCheckCommand`, `ReportCommand` (« nomos report »), `ExportSPDXCommand` / `ExportCycloneDXCommand` (**les exporteurs BOM existent déjà mais sont inatteignables — alimente VRC-23**), `AttestCommand` (re-routé sur la vraie signature par #542 et cité par la décision 0005, mais éclipsé par `attestCommand`).
- Tremplin : pattern d'enregistrement #543 (`atomize`).
- Preuve exigée : pour chaque commande, décision tracée — enregistrer (map + help + test `TestRun<X>IsReachable` + caller documenté) ou élaguer (suppression + note de décision) ; l'allowlist `known_unwired` du registre se vide au rythme des décisions (la matrice échoue sur toute entrée périmée, dans les deux sens).
- Claim débloqué : zéro commande fantôme — chaque doc-comment correspond à une surface réelle.
- Registre : retirer les entrées `known_unwired` traitées.

## Phase 1 — Le drapeau réel

### VRC-10 — Cite-or-abstain dans le moteur Go (A1)
- Type : pivot-core · Parallèle : oui · Dépend : VRC-00
- Ancre vision : doc 41 §0/§2 P2 (le wedge — « Harvey admet le gate strict non résolu ») ; doc 45 §2 A1 ; doctrine §2.3 (« sidecar = PARTIAL »).
- Écart : E2 — la faithfulness recalculée vit dans `scripts/regulated_rag_answer_evidence.py` + `tests/test_ckm_faithfulness_recompute.py` ; **rien dans cli/internal**.
- Tremplin : ALCE (citation recall/precision NLI) ; Trust-Align (Trust-Score, refus appris) ; Self-RAG `[IsSUP]` ; FActScore (decompose→verify) ; DeepEval (gate CI) ; HHEM (modèle ouvert) — OSS permissif/ouvert.
- Preuve exigée : package `cli/internal/answer` : faithfulness **recalculée depuis les spans retrouvés**, verdict cite/abstain, seuils configurables ; `nomos answer gate` enregistré + intégré au strict gate ; `trust_tier` exposé par réponse ; adversarial : citation falsifiée (span déplacé/hash altéré) → rouge ; réponse sans span → abstention forcée ; équivalent Go du bypass « no-text » (#542) ; le sidecar devient consommateur du verdict Go.
- Claim débloqué : « cite-or-abstain mesurable et bloquant, calculé depuis les spans, tiers exposés » — P2 frontière.
- Registre : flip `cite_or_abstain_gate` `sidecar` → `real` + ancres engine/caller/adversarial (la matrice force le flip : ses probes d'absence détectent l'arrivée du moteur).

### VRC-11 — Canon promotion dans le moteur Go (A2)
- Type : pivot-core · Parallèle : oui · Dépend : VRC-00
- Ancre vision : doc 40 §6 ; doc 41 §2 P5 (« canon préservant la confidentialité : introuvable ») ; doc 45 §2 A2.
- Écart : E2 — `scripts/ckm_canon_promotion_validate.py` + `specs/canon-promotion.cue`, pas de moteur.
- Tremplin : Wikidata ranks ; modèle Vault ; patterns Guru Verified / Copilot Official Source (concept/UX seulement) ; PKG API.
- Preuve exigée : `nomos canon promote/revoke/verify` : droits → validation → facettage (`provenance=user_promoted`) → `#Certificate` signé DSSE → corpus filtrable par lens ; silo confidentialité ; adversarial : tamper certificat (1 octet → rouge) ; promu jamais émis `certified`/`official` (test) ; certificat révoqué → chunk exclu avec raison ; confidentiel jamais émis hors silo.
- Claim débloqué : « bring-your-own-authority gouverné » — P5 différenciant.
- Registre : flip `canon_promotion` + ancres.

### VRC-12 — Point-in-time dans le moteur Go (A3)
- Type : pivot-core · Parallèle : oui · Dépend : VRC-00
- Ancre vision : doc 42 §B1 (« indispensable pour certifié : citer la bonne version ») ; doc 45 §2 A3.
- Écart : E2 — `scripts/ckm_point_in_time_resolve.py` + `specs/point-in-time.cue`, pas de moteur.
- Tremplin : SAT-Graph RAG / LRMoo ; FRBR + ELI ; legislation.gov.uk (référence d'archi).
- Preuve exigée : modèle temporel d'atome (`valid_from`/`valid_to`, événements ; `supersedes` existant) + résolveur `--as-of <date>` sur `atomize`/`bundle` ; adversarial : version remplacée non retrouvable à date ultérieure sans flag historique (test rouge sans le résolveur).
- Claim débloqué : « citer la bonne version à la bonne date ».
- Registre : flip `point_in_time` + ancres.

### VRC-13 — Harnais d'évaluation RAG en CI (B2)
- Type : mise à niveau · Parallèle : oui · Dépend : —
- Ancre vision : doc 42 §A (« gate CI bloquante sur la fidélité ») ; doc 45 §3 B2.
- Écart : E5 — aucune éval RAG continue.
- Tremplin : RAGAS, TruLens, ARES ; LegalBench-RAG, LRAGE, RAGChecker — OSS.
- Preuve exigée : job CI d'abord non bloquant puis bloquant (pattern fail-open → fail-closed de `ci/README.md`) ; seuils versionnés ; test du gate : régression de citation recall sous seuil → PR bloquée.
- Claim débloqué : « éval RAG non-régressive, orientée régulé ».

### VRC-14 — Répétabilité CI d'evidence sur corpus privé (F4)
- Type : QMS/evidence · Parallèle : oui · Dépend : —
- Ancre vision : README (« le durcissement suivant vise la répétabilité CI ») ; claim boundary §remaining proof chain.
- Écart : E3/E6 — un seul run enregistré ; « single recorded run » ≠ « repeated CI evidence ».
- Tremplin : workflows E2E existants (`rbok-lawbook-e2e.yml`) + scheduled runs.
- Preuve exigée : runs planifiés (hebdo) sur corpus privé ; packs archivés et indexés dans l'evidence ledger ; cible ≥ 8 runs consécutifs verts.
- Claim débloqué : « repeated CI evidence on private corpora » (chaîne de preuve release-scoped du claim boundary).

### VRC-15 — Release `v0.2.0-ALPHA` exécutée via SOP (F5)
- Type : QMS/release · Parallèle : non · Dépend : VRC-05, VRC-14
- Ancre vision : release-and-retirement SOP ; doc 41 §6 (risque « ALPHA-en-apparence ») ; doc 16 (versioning).
- Écart : E3 — un seul tag, aucune release exécutée via la SOP.
- Tremplin : templates release bundle existants.
- Preuve exigée : release bundle conforme au template, approbations enregistrées, tag + notes publiés, evidence pack de release archivé ; le record de release est le premier record SOP exécuté de bout en bout.
- Claim débloqué : « release management opéré » — la maturité réelle devient lisible.

### VRC-16 — Records formation/compétence (F6)
- Type : QMS · Parallèle : oui · Dépend : VRC-05
- Ancre vision : training & competence SOP ; doc 45 §7 F6.
- Écart : E3 — matrice 5 rôles × 4-5 compétences, zéro attestation signée.
- Tremplin : templates existants (`training-matrix.yaml`, `competence-assessment-template.yaml`).
- Preuve exigée : matrice remplie pour les humains nommés, 1-2 attestations par rôle, datées/signées ; statut `requires_evidence` → `established` calculé.
- Claim débloqué : levée de la condition d'effectivité « training records » du quality manual.

## Phase 2 — Fenêtre EU AI Act (livraison ≤ août 2026)

### VRC-20 — Contrat de pack domaine (CUE) (D1)
- Type : pivot-core · Parallèle : non · Dépend : VRC-00
- Ancre vision : doc 40 §7 (« le pack mince ») ; doc 45 §5 D1.
- Écart : la généralité est une promesse, pas un contrat — les profils domaine actuels (`nomos-domain-profile.cue`) ne couvrent ni vocabulaires d'axes ni lens-presets ni golden corpus.
- Tremplin : SKOS (1 scheme/axe) ; pattern NFDIcore ; WK FAB / Palantir OSDK (références d'archi, concept seulement).
- Preuve exigée : `specs/domain-pack.cue` : vocabulaires par axe + registre d'autorité + connecteurs + lens-presets + golden corpus + claim boundary d'instance + scorecard ; un pack contenant du code/mécanique est rejeté (test) ; fixtures valid/invalid.
- Claim débloqué : « un pack est 100 % déclaratif ».

### VRC-21 — `nomos pack validate` (gate Go) (D2)
- Type : pivot-core · Parallèle : non · Dépend : VRC-20
- Ancre vision : doc 45 §5 D2 ; doctrine §2.3.
- Écart : aucune vérification exécutable de conformité d'un pack.
- Tremplin : pipeline corpus existant (scan→feed→ledger→gate→bundle) réutilisé tel quel.
- Preuve exigée : commande enregistrée + caller CI ; exécute le golden corpus du pack sur la chaîne complète ; adversarial : pack mutilé (vocab manquant / golden rouge / preset cassé / claim boundary absent) → **fail closed**, chaque cas testé.
- Claim débloqué : « conformité de pack vérifiable par gate ».
- Registre : nouvelle entrée `domain_pack_gate` (expected `real` à la livraison).

### VRC-22 — Pack « EU AI Act evidence » pilot-grade (D3)
- Type : pack mince · Parallèle : non · Dépend : VRC-21 (+ VRC-06 pour l'offre)
- Ancre vision : doc 41 §5.3 (fenêtre datée août 2026) ; doc 45 §5 D3 — tranche la décision ouverte n°2 du doc 40 §14.
- Écart : E6 — fenêtre à ~2 mois, aucun vertical de généralité prouvé.
- Tremplin : texte officiel AI Act (source d'autorité ingérée, format HTML/XML EUR-Lex) ; ALCOA+/Part 11 comme vocabulaire d'evidence.
- Preuve exigée : le pack passe `pack validate` ; golden corpus AI Act sur chaîne complète ; claim boundary d'instance relu par le guard (« pilot-grade evidence pack », jamais « conformité certifiée ») ; D6 mesuré = 0 changement core.
- Claim débloqué : « la généralité cross-métier est prouvée par un 2ᵉ vertical réel » (exigence CKM-04).

### VRC-23 — Evidence packs en BOM CycloneDX/SPDX (A5)
- Type : amplify · Parallèle : oui · Dépend : VRC-09 (décision sur les exporteurs)
- Ancre vision : doc 42 §B7 (« packaging d'evidence qui rend la maturité lisible ») ; doc 45 §2 A5.
- Écart : evidence packs en format maison uniquement — **mais** la matrice a révélé que `ExportSPDXCommand`/`ExportCycloneDXCommand` existent déjà dans `cli/internal/app/report_cmds.go`, orphelins : le travail commence par câbler/vérifier l'existant, pas par écrire.
- Tremplin : CycloneDX ML-BOM, SPDX 3.0 — standards ouverts ; exporteurs orphelins existants (VRC-09).
- Preuve exigée : émission BOM validée par schéma ; hashes recoupés avec le body-ledger (calculé) ; test : hash divergent → rouge.
- Claim débloqué : « evidence lisible par l'outillage supply-chain standard ».

## Phase 3 — Ingestion & AEC

### VRC-30 — Adapter PDF né-numérique, claim ladder (C1)
- Type : ingestion · Parallèle : oui · Dépend : VRC-00
- Ancre vision : doc 14 v0.2-v0.3 (fidélité portable) ; doc 45 §4 C1 ; le vertical AEC en dépend (PGA/PAZ = PDF).
- Écart : E5 — `PlaceholderAdapter` (« not yet implemented »).
- Tremplin : bibliothèque permissive à instruire au registre licences (pdfium/poppler/pdfcpu…) ; jamais de claim OCR sans preuve.
- Preuve exigée : échelle de claims PDF (text → tagged → OCR hors claim) ; spans = page + offsets/bbox ; non-extractible = unsupported record (pattern body-ledger) ; golden fixture PDF public (PGA/PAZ) → feed source-backed + ledger 0 octet non couvert ; mutation d'un octet → drift détecté ; suppression de l'adapter → gate rouge.
- Claim débloqué : « PDF né-numérique gouverné » — débloque VRC-34.
- Registre : flip `pdf_adapter` `stub` → `real` + ancres.

### VRC-31 — Adapter HTML/XML (C2)
- Type : ingestion · Parallèle : oui · Dépend : VRC-00
- Ancre vision : doc 45 §4 C2 ; sources légales en ligne (Fedlex livre HTML/XML).
- Écart : E5 — aucun adapter HTML.
- Tremplin : Akoma Ntoso, ELI/ECLI ; tree-sitter HTML (toolchain existante).
- Preuve exigée : même barre que VRC-30 sur fixture Fedlex ; spans = chemin DOM + offsets ; identité ELI préservée dans les locators.
- Claim débloqué : « sources légales en ligne ingérées avec identité ».

### VRC-32 — Connecteurs CH industrialisés (C4)
- Type : ingestion · Parallèle : oui · Dépend : —
- Ancre vision : doc 40 §7 (connecteurs du pack) ; doc 45 §4 C4.
- Écart : 2 connecteurs (OFS, Fedlex) sur la cartographie CH complète.
- Tremplin : Juriscraper / Free Law (framework de scrapers, ouvert) ; legislation.gov.uk (archi).
- Preuve exigée : contrat connecteur inchangé (fetch réel + hash réel + evidence, no-full-text) ; swisstopo/STAC, RDPPF/ÖREB, géoportail cantonal, pipeline PDF communal, sidecar hash-only SIA ; chaque connecteur : test live skippable + fixture offline ; hash synthétique → guard rouge (acquis #539).
- Claim débloqué : « registre de sources d'autorité CH opérationnel ».

### VRC-33 — Kit obligatoire par adapter (C5)
- Type : garde-fou · Parallèle : oui · Dépend : VRC-30 (premier client)
- Ancre vision : doc 14 principe 4 (« capability versionnée avec limites déclarées ») ; doc 45 §4 C5.
- Écart : rien n'empêche d'enregistrer un adapter sans claim boundary ni fixtures.
- Tremplin : — (discipline interne, pattern fail-closed existant).
- Preuve exigée : l'enregistrement d'un adapter sans kit complet (fixtures + claim boundary + taxonomie unsupported + fixtures de gate) échoue en CI ; test adversarial avec adapter incomplet.
- Claim débloqué : chaque claim de format mappé à son evidence.

### VRC-34 — Pack AEC suisse (mince) (D5)
- Type : pack mince · Parallèle : non · Dépend : VRC-21, VRC-30, VRC-31
- Ancre vision : doc 40 §7 ; doc 41 §5.5 (AEC suisse inoccupé) ; doc 45 §5 D5.
- Écart : E5 — le 1ᵉʳ vertical n'a ni vocabulaires ni golden corpus.
- Tremplin : AEC3PO/ACCORD (juridiction+phase) ; IFC/buildingSMART (vocab) ; SIA en hash-only.
- Preuve exigée : pack passe `pack validate` ; golden corpus VD/Lausanne vert bout-en-bout ; lens-presets archi (« archi-conception », « DT-chantier », « permis ») ; preuve distracteurs (VRC-35) jouée sur conception vs DT ; D6 = 0 changement core.
- Claim débloqué : « 1ᵉʳ vertical prouvé sur le white space AEC ».

### VRC-35 — Harnais retrieval de référence + preuve distracteurs (B1)
- Type : mise à niveau · Parallèle : oui · Dépend : VRC-20
- Ancre vision : doc 40 §5 (Lens = WHERE au niveau base) ; doc 42 §A/§C3 ; doc 45 §3 B1.
- Écart : E5 — la promesse anti-parasite n'est pas mesurée ; aucun retrieval de prod.
- Tremplin : pgvector + RLS, Qdrant payload filters, reranker cross-encoder ; littérature distracteurs (ACL 2025 : −6 à −11 pts) comme preuve empirique.
- Preuve exigée : kit consommateur de référence (hors core) ; tests distracteurs sur golden corpus : accuracy avec Lens > sans Lens (mesuré, seuils versionnés) ; chunk exclu par Lens jamais retrouvé (test).
- Claim débloqué : « Lens enforced avant génération, au niveau base — mesuré ».

### VRC-36 — Kit de conformité consommateur (E-1)
- Type : couture · Parallèle : oui · Dépend : VRC-20
- Ancre vision : doc 43 §1 (« dépendre de l'artefact, jamais du code ») ; doc 45 §6 E-1.
- Écart : E5 — le bundle contract existe (#534-536) mais aucun kit ne permet à un consommateur de prouver sa conformité.
- Tremplin : `ckm_bundle_validate.py` existant (refus des facettes locales) comme noyau du kit.
- Preuve exigée : importeur de référence + tests publiés avec le contrat ; le kit rejette un bundle altéré (hash, facette inconnue, schema_version) — chaque cas testé.
- Claim débloqué : « consommer NOMOS = passer un kit ».

### VRC-37 — Preuve de consommation Aedifica W19 (E-2, cross-repo)
- Type : couture · Parallèle : non · Dépend : VRC-36 (+ Aedifica W19-01..04)
- Ancre vision : doc 39 (couture CKM-13 ↔ W19-01) ; doc 45 §6 E-2.
- Écart : E5 — aucun consommateur réel n'a consommé un bundle facetté.
- Tremplin : voie OFS-direct Aedifica (défaut sûr, flag-gated).
- Preuve exigée : CI Aedifica verte sur le chemin bundle (import, retrieval scopé par Lens, réponses citées avec tier) ; lien d'evidence enregistré côté NOMOS (référencé, jamais résumé à sa place).
- Claim débloqué : « un consommateur réel consomme un bundle facetté » — aujourd'hui interdit.

### VRC-38 — Métrique de reproductibilité D6
- Type : garde-fou · Parallèle : oui · Dépend : VRC-20
- Ancre vision : doc 45 §5 D6 (« reproductible = métrique, pas promesse »).
- Écart : rien ne mesure le couplage core/pack.
- Tremplin : check CI sur les chemins touchés par une PR de pack.
- Preuve exigée : CI calcule « changements core requis par nouveau pack » ; une PR étiquetée pack touchant `cli/internal/**` → revue bloquante avec justification ADR ; test du check.
- Claim débloqué : « n'importe quel domaine » devient vérifiable.

## Phase 4 — Consolidation

### VRC-40 — Backend Sigstore keyless (A4)
- Type : amplify · Parallèle : oui · Dépend : VRC-00
- Ancre vision : doc 42 §A (in-toto + Sigstore/Rekor) ; doc 45 §2 A4 ; guard #542 (Sigstore = non-présent tant que non livré).
- Écart : signature locale ECDSA seulement ; claims Sigstore interdits par le guard.
- Tremplin : sigstore-go, Fulcio, Rekor — Apache.
- Preuve exigée : backend optionnel à côté de l'ECDSA ; entrée Rekor vérifiée en test d'intégration en ligne (skippable offline, pattern `connector_live_test.go`) ; tamper-fail ; le guard claim-boundary met à jour son marqueur de preuve.
- Claim débloqué : « attestation keyless + transparency log ».
- Registre : flip `sigstore_keyless` `absent` → `real` + ancres (les probes d'absence forcent le flip).

### VRC-41 — Adapter DOCX (C3)
- Type : ingestion · Parallèle : oui · Dépend : VRC-33
- Ancre vision : doc 45 §4 C3.
- Écart : la matrice a révélé un état intermédiaire — `ExtractDocx` (cli/internal/fidelity/docx_adapter.go, « feasibility-level », vrai parsing OOXML zip+XML, testé) existe mais n'a **aucun caller de production** ; `.docx` reste enregistré sur `PlaceholderAdapter`. Statut matrice : `partial`.
- Tremplin : l'extracteur feasibility existant ; spans = XML path locators à ajouter.
- Preuve exigée : même barre que VRC-30 (spans, unsupported records, ledger) ; câblage dans le registry d'adapters ; kit VRC-33 complet.
- Claim débloqué : « DOCX gouverné ». Registre : flip `docx_adapter` `partial` → `real`.

### VRC-42 — Substrat d'exécution de règles (B3)
- Type : mise à niveau · Parallèle : oui · Dépend : —
- Ancre vision : doc 42 §A ; doc 45 §3 B3 ; anti-objectif n°3 (pas de moteur maison).
- Écart : le calculable n'a pas de substrat d'exécution.
- Tremplin : L4 (MCP/REST, vérif formelle) ; OpenFisca (AGPL → frontière API process, registre licences) ; Catala.
- Preuve exigée : démo bornée — un atome `formula` exécuté via le substrat avec trace source ; isolement AGPL documenté au registre.
- Claim débloqué : « le déterministe certifié sans moteur de règles maison ».

### VRC-43 — Graphe de renvois déterministe (B5)
- Type : mise à niveau · Parallèle : oui · Dépend : —
- Ancre vision : doc 42 §B4 ; doc 45 §3 B5.
- Écart : les renvois (« sous réserve de l'art. X ») ne sont pas tracés.
- Tremplin : concept BifrostRAG ; GraphRAG/LightRAG en option.
- Preuve exigée : graphe parsé par règles (jamais LLM) ; renvoi connu du golden corpus présent avec span source ; renvoi non parsable = unsupported explicite.
- Claim débloqué : « cross-références traçables entre atomes ».

### VRC-44 — Outillage vocabulaires SKOS/SHACL (B4)
- Type : mise à niveau · Parallèle : oui · Dépend : VRC-20
- Ancre vision : doc 42 §A ; doc 45 §3 B4.
- Écart : vocabulaires en CUE + SKOS statique, sans authoring/validation outillés.
- Tremplin : SKOS, SHACL, OWL `disjointUnionOf`, VocBench, Skosmos, ISO 25964.
- Preuve exigée : validation SHACL des facettes en CI (complément du `cue vet`) ; vocabulaire non orthogonal → rouge (test).
- Claim débloqué : « facettes authored, servies, validées proprement ».

### VRC-45 — Ontologie facettes dans `pack validate` (D4)
- Type : pivot-core · Parallèle : oui · Dépend : VRC-21
- Ancre vision : doc 42 §B3 (BFO→IOF→pack) ; doc 45 §5 D4.
- Écart : E2 — `ckm_facet_ontology_validate.py` est un sidecar dont le verdict n'est pas rendu par un gate moteur.
- Tremplin : BFO (ISO 21838), IOF Core, NFDIcore, ODP.
- Preuve exigée : l'alignement ontologique est vérifié par `pack validate` (verdict rendu par le gate) ; axe non aligné → pack rejeté (test).
- Claim débloqué : « la généralisation est défendable » (anti-« couche horizontale »).
- Registre : flip `facet_ontology_alignment`.

### VRC-46 — Bench public cite-or-abstain
- Type : amplify · Parallèle : non · Dépend : VRC-10, VRC-13
- Ancre vision : doc 42 §C2 (« publier un bench façon BigLaw Bench ; planter le drapeau ») ; doc 45 §2 sortie A.
- Écart : E6 — le wedge n'est pas prouvé publiquement ; fenêtre 12-24 mois.
- Tremplin : méthodo BigLaw Bench (concept) ; LegalBench-RAG ; HHEM.
- Preuve exigée : bench reproductible publié (méthodo + harnais + résultats datés) ; claims du bench relus par le guard ; re-vérification des sources arXiv avant publication (mise en garde doc 41).
- Claim débloqué : preuve externe du drapeau — « le gate strict que Harvey dit non résolu, mesuré chez NOMOS ».

---

## Couture avec la matrice de câblage (VRC-00)

Chaque issue qui promeut une capacité **doit** flipper l'entrée correspondante de
`scripts/vrc_wiring_matrix_registry.json` dans la même PR (expected `sidecar|stub|absent|partial`
→ `real`) avec les nouvelles ancres. La matrice échoue si la réalité et le registre
divergent — dans les deux sens. C'est le verrou anti-« déclaré fait ».

## Liens issues (matérialisés)

**Epic `RBOKproject/NOMOS`** : **#545** · Livraison VRC-00 : PR **#544** · Matérialisé le 2026-06-11.

| ID | Issue | ID | Issue |
|---|---|---|---|
| VRC-00 | #546 | VRC-22 | #565 |
| VRC-01 | #547 | VRC-23 | #566 |
| VRC-02 | #548 | VRC-30 | #567 |
| VRC-03 | #549 | VRC-31 | #568 |
| VRC-04 | #550 | VRC-32 | #569 |
| VRC-05 | #551 | VRC-33 | #570 |
| VRC-06 | #552 | VRC-34 | #571 |
| VRC-07 | #553 | VRC-35 | #572 |
| VRC-08 | #554 | VRC-36 | #573 |
| VRC-09 | #555 | VRC-37 | #574 |
| VRC-10 | #556 | VRC-38 | #575 |
| VRC-11 | #557 | VRC-40 | #576 |
| VRC-12 | #558 | VRC-41 | #577 |
| VRC-13 | #559 | VRC-42 | #578 |
| VRC-14 | #560 | VRC-43 | #579 |
| VRC-15 | #561 | VRC-44 | #580 |
| VRC-16 | #562 | VRC-45 | #581 |
| VRC-20 | #563 | VRC-46 | #582 |
| VRC-21 | #564 | | |

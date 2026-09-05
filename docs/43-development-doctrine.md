# 43 — Approche & doctrine de développement (NOMOS × Aedifica)

> **Vantage : NOMOS** (moteur). Doc compagnon côté produit :
> `decarvalhoe/aedifica` → `docs/strategy/development-approach.md`.
> Document **formel de référence** — s'applique à toute contribution. Statut : doctrine.
> Aligné sur [public-claim-boundary.md](public-claim-boundary.md) et
> [08-governance-and-change.md](08-governance-and-change.md). Date : 2026-06-10.

## 1. Les deux produits & leur relation

- **NOMOS** — moteur de **savoir canonique certifié** (réglementaire **+ métier**) :
  ingestion *read-only* de sources d'autorité → atomes à spans + hash → TOC certifiée →
  matrice de traçabilité → feed RAG sourcé → *fidelity gate* → evidence. Domain-agnostic,
  spécialisé par **packs minces**.
- **Aedifica** — assistant de **conduite de projet** pour l'architecte suisse, sur tout
  le cycle SIA. **Premier vertical** qui consomme NOMOS via un **bundle contract
  versionné**, **derrière feature flag**, l'**OFS-direct restant le défaut sûr**.
- **Couture** : un consommateur (Aedifica, RBOK…) dépend de l'**artefact** (bundle /
  feed), jamais du code NOMOS. Les produits avancent à leur rythme, sans se bloquer.

## 2. Principes non négociables

1. **Deux produits vivants, zéro régression.** Tout est **additif / rétro-compatible** ;
   la CLI, les profils, la *fidelity gate*, le pipeline d'evidence et l'intégration RBOK
   restent verts ; harnais de non-régression (CKM-00) vert à chaque PR. Tout changement
   de contrat = `schema_version` bump + migration.
2. **Claim-boundary** (doctrine fondatrice de NOMOS). On n'affirme que ce qu'on prouve ;
   toute revendication non prouvée est **downgradée**, jamais maquillée. Cf.
   `aedifica docs/strategy/hardening-discipline.md`.
3. **Pas de *done* sans preuve adversariale.** Un test qui passe ne prouve rien ; un test
   qui **échoue sans le fix** prouve. Le tamis : bug → *revert-and-confirm* ; crypto →
   *tamper-fail* (altérer 1 octet → `verify` rouge) ; mécanique → **consommée par le
   moteur Go + test Go** quand son caller attendu est le moteur (un schéma CUE +
   sidecar ne remplace pas cette mécanique). Un outil volontairement hors core
   peut être livré comme `sidecar` ; intended use, validation et reliance sont
   suivis séparément selon ADR-VRC-0004 ;
   résultat de pipeline → **calculé**, jamais une chaîne `"green"` déclarée ; connecteur →
   **fetch réel + hash réel**.
4. **L'IA propose, le spécialiste décide.** Sourcé ou inconnu ; **l'abstention est une
   réponse légitime** ; le LLM cite/explique sous gouvernance, ne devient jamais l'autorité.
5. **Mécaniques au cœur, spécificités au pack.** Faceting, lens, canon promu, attestation,
   gate = **core** ; un pack domaine (built-environment…) ne fournit que vocabulaires +
   connecteurs. Réglementaire **et** métier dans **une même** architecture.
6. **Capitaliser honnêtement.** Standards ouverts → adopter ; OSS permissif → intégrer ;
   AGPL → **isoler** (frontière process/API, cf. license register) ; commercial →
   s'inspirer du concept, jamais code/contenu/IP ; références payantes (GAMP 5, ISO,
   SIA…) → **jamais de texte intégral** (hash + crosswalk via sidecar).
7. **Roadmaps indépendantes, validation basée sur le risque.** Produit, DevOps et
   assurance régulée avancent séparément (`docs/47`, ADR-VRC-0004). Un contrôle
   régulé peut être manuel avant son outil ; un outil peut être développé avant
   validation de son intended use. Calendrier, signatures, achats et writes
   publics bloquent leurs claims, jamais le dispatcher autonome.

## 3. Modèle de savoir (vocabulaire commun aux deux dev)

- **Facettes** (axes contrôlés) : `nature` {regulatory · metier · project} · `discipline`
  /rôle · `activity` · `scope_level` · `trust_tier` {certified · indicative · unverified}
  · `provenance` {official · user_promoted} · `confidentiality` · `applicability`.
- **Lens** : prédicat inclusion/exclusion sur facettes → **scope le retrieval**
  (anti-parasite) ; défaut = *no lens* = comportement actuel.
- **Canon promu** : élévation d'une source au canon **sous droits + validation** ;
  `user_promoted` ≠ `official` ; confidentiel **en silo**.
- **Trust honnête** : toute citation porte son **tier** ; promu/métier n'usurpe jamais le
  `certified` officiel.

## 4. Process

- **Branche de feature + PR + CI + gates.** **Branch-protection respectée** : revue
  requise, **pas d'override admin** (cf. 08-governance-and-change, GOVERNANCE.md).
- Toute issue revendiquant une **capacité** passe la **barre d'acceptation** de la
  *hardening-discipline* (epic durcissement #518) — incl. la **CI-guard claim-boundary**
  (un check qui échoue si la doc dit « signé/Sigstore/certifié » sans preuve).
- Données régulées : ALCOA+ / Part 11 vocabulaire ; aucune source payante en intégral.

## 5. Références

- **NOMOS `docs/`** : `39-canonical-knowledge-mesh-pivot` · `40-knowledge-mesh-architecture`
  · `41-state-of-the-art-positioning` · `42-capitalization-and-improvement-plan` ;
  `public-claim-boundary.md` · `08-governance-and-change.md` · `38-domain-opportunity-roadmap`.
- **Aedifica `docs/strategy/`** : `nomos-pivot-masterplan` · `nomos-implementation-audit`
  · `hardening-discipline` · `development-approach` (compagnon de ce doc).
- **Epics** : #481 (pivot CKM), #518 (durcissement) ; Aedifica #278 (W19).

<!-- Porté depuis l'analyse stratégique Aedifica — origine du pivot CKM (epic #481).
Mapping de ce dossier : 39 = plan maître · 40 = architecture méta (faceting/lens/promotion)
· 41 = positionnement état de l'art · 42 = capitalisation & amélioration.
Les références internes "nomos-*.md" pointent vers ces mêmes docs (noms Aedifica d'origine). -->

# NOMOS — Plan de pivot consolidé & matérialisation

> Document maître. Consolide les 3 docs stratégiques + les 8 streams de recherche, acte
> le pivot, et le matérialise en **epics + issues** sur deux repos.
> **Principe directeur n°1 : deux produits vivants, zéro régression.** NOMOS *et*
> Aedifica restent fonctionnels et utiles à chaque étape. Tout le travail de pivot est
> **additif et rétro-compatible**.
> Date : 2026-06-08.

## Docs & sources consolidés (référence unique)

| Doc | Rôle |
|---|---|
| `nomos-knowledge-mesh-and-built-environment.md` | L'architecture méta (faceting, lens, promotion, pack mince) |
| `nomos-state-of-the-art-positioning.md` | Le verdict honnête (avant-garde = la synthèse, pas les composants) |
| `nomos-capitalization-and-improvement-plan.md` | Capitaliser : rattraper / combler / amplifier + garde-fous IP |
| 8 rapports d'agents (session) | ~400 sources URLs par pilier (académique + industriel + OSS) |
| `aedifica/jurisdictions/` (W17/W18) | 1ʳᵉ tranche vivante (registre OFS) — reste le défaut sûr |
| `NOMOS/specs/atomization-spine.cue`, `docs/05,09,36,38` | Contrats réels NOMOS (atome, RAG, adaptation, intégration, roadmap) |

## La thèse consolidée (le drapeau)

> **NOMOS = la supply-chain du savoir certifié** — réglementaire **et** métier —
> facetté, scopé par pertinence (lens), enrichissable par l'utilisateur (canon promu),
> packs minces par domaine. On **mène avec la synthèse défendable** (canonical-first +
> cite-or-abstain + evidence signé + claim-boundary, étendu au métier), pas avec
> « couche horizontale » banalisée. built-environment/AEC suisse = 1ʳᵉ preuve verticale ;
> evidence packs EU AI Act (août 2026) = 2ᵉ fenêtre datée.

---

## Principe directeur : deux produits vivants, zéro régression

Symétrique pour les deux repos :

| Produit | Reste utile via… | Le pivot n'y touche que… |
|---|---|---|
| **NOMOS** | CLI + profils existants + fidelity gate + evidence pipeline + intégration RBOK | …de façon **additive** : facettes d'abord dans le `metadata` ouvert de l'`#Atom` ; nouveaux sous-commandes ; nouveaux profils *à côté* des existants ; `schema_version` bump + migration pour tout changement de contrat. Les tests existants (`go test ./...`, e2e, RBOK-E2E) **restent verts**. |
| **Aedifica** | OFS-direct (W17/W18) reste le **défaut** ; surfaces produit inchangées | …derrière **feature flag** : import du bundle NOMOS, retriever doctrine, facettes — tout opt-in. La voie OFS-direct reste verte en CI. |

**Mécanique du découplage : le *bundle contract* versionné.** Aedifica dépend d'un
artefact (bundle NOMOS), jamais du code NOMOS. Les bundles émis portent désormais
les facettes dérivées ; le champ `domain` plat reste une compatibilité du spine,
pas l'état courant du contrat facetté. Les deux avancent à leur rythme sans se bloquer.

---

## Deux chantiers parallèles

### Chantier I — NOMOS core (le pivot) · epic `[CKM]`
Fait évoluer NOMOS vers le Canonical Knowledge Mesh, **sans casser l'existant**.

### Chantier II — Aedifica intégration non-bloquante · epic `[W19]`
Consomme NOMOS « dans l'état » + se prépare aux facettes, **sans casser le produit**.

Les deux chantiers sont **parallélisables** : Aedifica peut commencer à consommer un
bundle built-environment minimal pendant que NOMOS construit faceting/lens/promotion.

---

## Découpage epic / issues

### Epic `[CKM]` — Canonical Knowledge Mesh (NOMOS) — `RBOKproject/NOMOS`

| ID | Titre | Type | Parallèle | Dépend |
|---|---|---|---|---|
| CKM-00 | Non-regression harness : pivot additif, gate/CLI/profils/RBOK-E2E restent verts | garde-fou | oui (d'abord) | — |
| CKM-01 | Knowledge Faceting : `#Facets` contrôlé (via `metadata` → CUE+SHACL), additif | pivot-core | oui | CKM-00 |
| CKM-02 | Knowledge Lens : prédicat inclusion/exclusion sur facettes + scoping retrieval au niveau base | pivot-core | non | CKM-01 |
| CKM-03 | Canon Promotion : workflow droits+validation+provenance+certificat, confidentialité-préservante | pivot-core | non | CKM-01 |
| CKM-04 | `nature=metier` + classe source « bible métier » + golden corpus cross-métier (non-AEC) | pivot-core | oui | CKM-01 |
| CKM-05 | Attestation supply-chain : prédicats in-toto/Sigstore pour ingestion→canon→embedding | amplify (drapeau) | oui | CKM-00 |
| CKM-06 | Body-ledger Merkle : preuves d'inclusion (0 octet non couvert, vérifiable) | amplify (drapeau) | oui | CKM-00 |
| CKM-07 | Claim-boundary signé : prédicat « ce qu'on ne peut PAS prouver » | amplify (drapeau) | non | CKM-05, CKM-06 |
| CKM-08 | Cite-or-abstain mesurable : harnais ALCE + Trust-Score + DeepEval (gate CI) + trust-tier exposé en API | amplify (drapeau) | oui | CKM-00 |
| CKM-09 | `domain_profile: built-environment` (CUE) + source authority register CH + golden corpus VD/Lausanne | pack mince | non | CKM-01 |
| CKM-10 | Connecteurs sources CH (Fedlex/ELI, swisstopo/STAC, RDPPF/ÖREB, OFS) + pipeline PDF PGA/PAZ + sidecar hash SIA | pack mince | oui | CKM-09 |
| CKM-11 | Point-in-time : modèle temporel SAT-Graph/LRMoo + FRBR/ELI | combler | oui | CKM-00 |
| CKM-12 | Architecture ontologique BFO→IOF Core→pack pour les axes de facettes | combler | oui | CKM-01 |
| CKM-13 | Bundle contract versionné (feed facetté + rag-metadata + trace + attestation) — la couture Aedifica | pivot-core | non | CKM-01 (min) |
| CKM-14 | Gouvernance : clearance marque « Nomos » + note FTO + registre licences (isolation AGPL) | garde-fou | oui | — |

### Epic `[W19]` — Intégration NOMOS, non-bloquante (Aedifica) — `decarvalhoe/aedifica`

| ID | Titre | Type | Parallèle | Dépend |
|---|---|---|---|---|
| W19-00 | Keep-shipping guardrail : intégration flag-gated, OFS-direct reste vert en CI | garde-fou | oui (d'abord) | — |
| W19-01 | Adapter d'import bundle NOMOS → CommunePack/Claim/Source/Evidence, derrière flag | intégration | non | CKM-13 (stub OK) |
| W19-02 | Extension data model facet-aware (trust_tier/provenance/facettes sur Claim/Source) — migration additive | intégration | oui | W19-00 |
| W19-03 | Scoping retrieval par Lens (pgvector+RLS) — scaffold, flag-off | préparation | oui | W19-02 |
| W19-04 | Copilote → retriever doctrine (cite-or-abstain), derrière flag, OFS-direct défaut | intégration | non | W19-01 |
| W19-05 | Surfaçage trust-tier UI (certified/indicative/unverified) — réutilise les trust-states existants | intégration | oui | W19-02 |
| W19-06 | Remontée vers NOMOS : `validation_level`/6 trust-states comme apport au core | contribution | oui | — |

---

## Séquencement par effet de levier

1. **D'abord les garde-fous** : CKM-00 + W19-00 (zéro régression posée avant tout).
2. **Rendre le drapeau réel & mesurable** (parallèle) : CKM-05/06/07/08 — la synthèse
   défendable passe de concept à mesuré/signé/shippable.
3. **Poser le socle méta** : CKM-01 (faceting) → débloque 02/03/04/09/12/13.
4. **Couture non-bloquante Aedifica** : CKM-13 (bundle) ↔ W19-01 (adapter, stub d'abord)
   → Aedifica consomme NOMOS « dans l'état » sans rien casser.
5. **Combler** (parallèle) : CKM-11 (point-in-time), CKM-12 (ontologie).
6. **Pack mince + preuve** : CKM-09/10 (built-environment) → W19-04/05 (assistant scopé).
7. **2ᵉ vertical** : evidence packs EU AI Act (fenêtre août 2026) — issue ouverte après
   validation du drapeau.

---

## Registre de sources consolidé (adopter quoi, sous quel régime)

| Brique | Pilier | Régime | Usage |
|---|---|---|---|
| ALCE, FActScore/OpenFActScore, DeepEval, RAGAS/TruLens/ARES | P1/P2 | OSS permissif | Éval + gate CI cite-or-abstain |
| Trust-Align, Self-RAG, CRAG, FRONT, VeriCite | P2 | OSS (vérifier) | Refus appris, support par segment |
| in-toto, Sigstore/Rekor, SLSA | P7 | Apache | Attestation signée par étape |
| W3C PROV-O, VC 2.0, RFC 9162/Merkle, C2PA, CycloneDX/SPDX | P7 | std ouverts | Provenance, body-ledger, BOM evidence |
| ALCOA+, 21 CFR Part 11, Annex 11 | P7 | std/réglementaire | Framing régulé de l'evidence |
| SKOS, OWL (`disjointUnionOf`), SHACL, ISO 25964, VocBench, Skosmos | P3 | std/OSS | Facettes contrôlées + validation |
| BFO (ISO 21838), IOF Core, NFDIcore, ODP | P3/P6-gén | std/OSS | Architecture ontologique core+packs |
| Akoma Ntoso, LegalRuleML, ELI/ECLI, SAT-Graph/LRMoo | P6 | std/académique | Format + identité + point-in-time |
| L4 (MCP/REST/vérif) ; OpenFisca (⚠️ AGPL→API) ; Catala | P6 | OSS (licences mixtes) | Substrat d'exécution de règles |
| pgvector+RLS, Qdrant, reranker, BifrostRAG (concept) | P4 | OSS/concept | Retrieval scopé + graphe de renvois |
| Wikidata ranks, Vault tiers, PKG API (concept) | P5 | concept/OSS | Modèle de promotion/tiers |
| LegalBench-RAG, LRAGE, RAGChecker | éval | OSS | Éval orientée régulé |
| legislation.gov.uk, Juriscraper/Free Law | connecteurs | ouvert | Réf d'archi + framework de scrapers |
| **Produits** (WK FAB, TR, Harvey, Norm AI, Vectara HHEM*, Guru, Copilot, UpCodes, ValGenesis) | transversal | **concept/UX/bench seulement** | S'inspirer, jamais copier code/contenu/IP (*HHEM = modèle ouvert utilisable) |

---

## Garde-fous (rappel)

- **Zéro régression** sur les deux produits (CKM-00, W19-00).
- **IP** : standards ouverts → adopter ; OSS permissif → intégrer ; AGPL → isoler via
  API ; commercial → s'inspirer du concept ; SIA/ISO → jamais de texte intégral.
- **Marque + FTO** : wordmark « Nomos » + clearance + freedom-to-operate (CKM-14).
- **Claim-boundary** : NOMOS admet ce qu'il prouve, refuse le reste ; l'IA propose,
  l'architecte/le spécialiste décide.
- **LPD** : canon confidentiel en silo projet ; canon promu ≠ officiel (provenance
  tracée) ; cloud LLM exclut le confidentiel.

---

## Liens issues (matérialisés)

**Chantier I — NOMOS (`RBOKproject/NOMOS`)** · Epic **#481**
| ID | Issue | ID | Issue |
|---|---|---|---|
| CKM-00 | #482 | CKM-08 | #490 |
| CKM-01 | #483 | CKM-09 | #491 |
| CKM-02 | #484 | CKM-10 | #492 |
| CKM-03 | #485 | CKM-11 | #493 |
| CKM-04 | #486 | CKM-12 | #494 |
| CKM-05 | #487 | CKM-13 | #495 |
| CKM-06 | #488 | CKM-14 | #496 |
| CKM-07 | #489 | | |

**Chantier II — Aedifica (`decarvalhoe/aedifica`)** · Epic **#278**
| ID | Issue | ID | Issue |
|---|---|---|---|
| W19-00 | #279 | W19-04 | #283 |
| W19-01 | #280 | W19-05 | #284 |
| W19-02 | #281 | W19-06 | #285 |
| W19-03 | #282 | | |

> Couture inter-repos : NOMOS **CKM-13** (#495, bundle contract) débloque Aedifica
> **W19-01** (#280, adapter). Aedifica **W19-06** (#285) remonte vers NOMOS
> **CKM-03/08** (#485/#490).
```

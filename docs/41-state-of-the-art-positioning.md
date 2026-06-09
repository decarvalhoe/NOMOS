<!-- Porté depuis l'analyse stratégique Aedifica — origine du pivot CKM (epic #481).
Mapping de ce dossier : 39 = plan maître · 40 = architecture méta (faceting/lens/promotion)
· 41 = positionnement état de l'art · 42 = capitalisation & amélioration.
Les références internes "nomos-*.md" pointent vers ces mêmes docs (noms Aedifica d'origine). -->

# NOMOS — positionnement état de l'art (Step 0 analytique)

> Cartographie exhaustive (académique + industriel + open-source) et évaluation
> **honnête** du caractère avant-gardiste de NOMOS, pilier par pilier.
> Date : 2026-06-08. Méthode : 8 agents de recherche adversariale en parallèle,
> ~400 sources. Consigne : *chercher activement l'antériorité, ne pas flatter.*
> Mise en garde : certaines sources (arXiv très récents, claims produits) viennent
> d'un sweep rapide — **à re-vérifier avant tout usage externe/investisseur**.

---

## 0. Bottom line honnête

**NOMOS n'est pas avant-gardiste au niveau des composants — il l'est au niveau de la
synthèse.** Quasiment chaque brique (citation, abstention, facettes, filtrage
métadonnées, RBAC, rules-as-code, provenance, attestation) **existe déjà**, et
plusieurs se *commoditisent* vite. Le verdict adversarial est d'une constance
frappante sur les 7 piliers : **composant = Commoditisé/Émergent ; intégration =
Frontière/Différenciant.**

Ce qui est **génuinement en avance** (aucun produit ne combine ça en mi-2026) :

1. **Traiter le savoir certifié comme une supply-chain logicielle** — canonical-first
   + atomes à span/hash + *fidelity gate* bloquante + *body-ledger* (0 octet non
   couvert) + attestation signée + **claim-boundary** (signer ce qu'on ne peut PAS
   prouver). (P1 + P7.) **C'est le territoire le plus défendable et le plus neuf.**
2. **Le *cite-or-abstain* strict comme gate runtime + trust tiers** (certified/
   indicative/unverified exposés comme propriété de chaque réponse). Harvey admet
   publiquement que le cite-or-abstain strict est *un objectif de recherche non
   atteint*. (P2.)
3. **Le maillage cross-domaine réglementaire + métier** (réglementation + normes +
   « bibles » métier dans une même architecture). **Aucun concurrent ne combine les
   deux** — tous sont mono-domaine ou réglementaire-pur. (P6-généralisation.)

Ce qui est **on-trend / rattrapage** (NOMOS doit égaler, pas mener) : citation RAG,
gouvernance RAG, détection d'hallucination, packs verticaux, monitoring réglementaire.
Tout ça se commoditise.

**Les moats sont réels mais bornés dans le temps (12–24 mois)** et pressés des deux
côtés : les **incumbents** (Wolters Kluwer FAB, Thomson Reuters CoCounsel) exécutent
déjà l'architecture « core certifié + packs multi-domaines » — mais en *contenu épais,
réglementaire-pur, sans métier, sans preuve crypto* ; les **fast-movers** (Norm AI,
Vectara, Contextual AI) montent vers le « grounding autoritaire » et pourraient ajouter
l'atomisation canonique + la signature en 12–24 mois.

**Deux fenêtres de premier entrant concrètes et datées :** l'**AEC suisse** (inoccupé —
UpCodes est US-only) et les **evidence packs EU AI Act** (les règles high-risk
déclenchent en **août 2026**).

---

## 1. Méthode & couverture

8 axes, chacun balayé en académique + industriel + OSS, avec verdict de nouveauté
*composant* vs *intégration* sur l'échelle **Commoditisé → Émergent → Frontière →
Différenciant** :

| # | Axe | Pilier NOMOS |
|---|---|---|
| 1 | RAG sourcé/certifié, attribution, cite-or-abstain, fidelity gate | P1 + P2 |
| 2 | Organisation des connaissances, classification facettée multidimensionnelle | P3 |
| 3 | Scoping de pertinence/accès, multi-tenant, context engineering | P4 (Lens) |
| 4 | Curation human-in-the-loop, l'expert comme autorité, MDM/data-catalog | P5 (Promotion) |
| 5 | Computational law, rules-as-code, RegTech/GRC | P6 (réglementaire) |
| 6 | Provenance, evidence vérifiable, attestation supply-chain | P7 |
| 7 | Archi « core horizontal + packs verticaux », vertical-AI | P6 (généralisation) |
| 8 | Concurrents directs + marché + benchmark honnête | transversal |

---

## 2. Verdict par pilier

| Pilier | Composant | Intégration | Antériorité la plus proche | Le vrai gap |
|---|---|---|---|---|
| **P1** Canonical-first + fidelity gate + attestation | Frontière | **Différenciant** | RAGShield 2026 (mais = défense anti-poisoning, pas certification de fidélité) | Pipeline source→canon→RAG *signé + gate bloquante + TOC certifiée + « ce qui a été sauté »* en un seul système : introuvable |
| **P2** Cite-or-abstain + trust tiers | Émergent | **Frontière** | Trust-Align (ICLR 2025), Self-RAG, Divide-Then-Align | Trust-tier (certified/indicative/unverified) comme propriété exposée + *structured facts > texte récupéré* : sans équivalent. Harvey admet le gate strict non résolu |
| **P3** Facettes multidimensionnelles | Émergent | **Différenciant** | TopBraid EDG, OG-RAG, AEC3PO (ACCORD) | Standard de facettes *contrôlé, orthogonal, cross-domaine* (rôle/discipline + activité + applicabilité + trust comme contrôles de *retrieval*) : introuvable. ⚠️ pas une percée recherche — un std-body pourrait publier l'équivalent en 12–18 mois |
| **P4** Knowledge Lens (anti-parasite) | Commoditisé | **Frontière/Différenciant** | OwlerLite (scope 1-D), Copilot « official source », medical-device applicability (arXiv 2506.18511) | Épistémologie 2-niveaux (applicabilité *objective* vs activation *subjective*) + prédicats d'**exclusion** multi-axes composés par l'user *pour protéger l'accuracy* : sans équivalent. Validé empiriquement : distracteurs plausibles = −6 à −11 pts (ACL 2025), et les bons retrievers *aggravent* le problème |
| **P5** Canon promu par l'user | Émergent | **Différenciant** | Guru Verified RAG, Collibra/Alation certification, Copilot « Official Source », Wikidata ranks | Canon *préservant la confidentialité* (canon sans exposition) + « le praticien est autorité sur son projet » + cycle certificat/révocation lié à l'autorité, comme mécanique *cross-domaine* : introuvable. La taxo sécurité RAG 2026 pointe explicitement ce gap |
| **P6** Réglementaire / rules-as-code | **Commoditisé** | Différenciant (hors RegTech) | CUBE, Archer/Compliance.ai, AscentAI ($20 Md, ~20 %/an) | L'extraction d'obligations + crosswalk est un **commodity** — NE PAS y aller. Neuf : autorités *au-delà de la réglementation* (normes, SOP, bibles) + promotion gouvernée + mesh déterministe réglementaire+normes+métier |
| **P6-gén** Core horizontal + packs minces | Émergent | **Différenciant** (à l'intersection) | **Wolters Kluwer FAB**, Thomson Reuters CoCounsel, Palantir Ontology | « On est la couche horizontale » est le pattern le plus *sur-revendiqué* du secteur. Neuf *seulement* à l'intersection : source-fidelity cross-domaine **+ savoir métier** + packs réellement minces. Aucun incumbent ne fait le métier |
| **P7** Evidence vérifiable / attestation | Émergent | **Frontière** (2 composants) | in-toto, SLSA, Sigstore, C2PA, ALCOA+ (tous matures *ailleurs*) | Appliquer la provenance supply-chain au savoir = Émergent. **Frontière** : body-ledger *byte-coverage* + claim-boundary signé (signer ce qu'on ne peut pas prouver) |

---

## 3. La lecture transversale

Trois constats honnêtes qui reviennent sur les 8 axes :

1. **La nouveauté est *toujours* à l'intégration, jamais au composant.** Chaque pilier,
   pris isolément, a une antériorité solide. La défense de NOMOS n'est jamais « j'ai
   inventé X » mais « j'ai assemblé X+Y+Z d'une façon que personne n'a assemblée ». Ce
   n'est pas un défaut — beaucoup de produits majeurs sont des intégrations — mais ça
   **interdit le discours « technologie de rupture »** et impose le discours « synthèse
   défendable + exécution + premier entrant ».

2. **Le cœur le plus neuf est cohérent : la « supply-chain du savoir certifié ».** P1
   (canonical-first + gate) + P2 (cite-or-abstain + tiers) + P7 (attestation signée +
   claim-boundary) forment *un seul* thèse, et c'est là que l'antériorité est la plus
   faible. C'est le drapeau à planter. Les autres piliers (facettes, lens, promotion,
   généralisation) sont différenciants mais plus *contestables* (« filtrage métadonnées
   rebrandé », « MDM rebrandé », « couche horizontale de plus »).

3. **Le métier change tout — et personne ne l'occupe.** Le seul angle où *aucun*
   incumbent ni fast-mover n'est présent, c'est **réglementaire + métier dans une même
   architecture certifiée**. Wolters Kluwer/Thomson Reuters = réglementaire-pur, contenu
   épais ; Glean/Palantir = organisationnel, pas autoritaire ; Harvey/UpCodes =
   mono-domaine. Le pivot méta (métier comme standard NOMOS) est donc **validé par le
   marché comme white space** — à condition de ne pas le vendre comme « couche
   horizontale » générique.

---

## 4. Carte concurrentielle (qui occupe quoi)

**Incumbents « core certifié + packs » (au-dessus, contenu épais) :**
- **Wolters Kluwer — FAB platform** : l'analogue le plus structurellement proche.
  Core IA horizontal partagé sur 5 divisions (santé/UpToDate, légal/VitalLaw, fiscal,
  compliance/OneSumX), grounding sur contenu expert, partenariat OpenAI (juin 2026).
  ~6 Md$. **Mais : pas de métier, pas de preuve crypto, packs épais.**
- **Thomson Reuters — CoCounsel** : core de raisonnement partagé légal/fiscal/audit,
  grounding Westlaw. **Mais : 3 domaines, contenu propriétaire épais, pas de crypto.**
- **Palantir — Ontology** : couche sémantique horizontale + objets métier définis par
  le client. **Mais : data opérationnelle, pas de contenu autoritaire/certifié.**

**Fast-movers « grounding autoritaire » (en-dessous, montent) :**
- **Norm AI** : le plus proche en *raisonnement réglementaire* — « Enterprise Data
  Layer », trails de décision avec citations. **Mais : pas domain-agnostic, pas de
  evidence pack signé, pas d'ALCOA+, pas de métier.**
- **Vectara** : le plus governance-forward (HHEM, Guardian Agents, version-aware).
  **Mais : retrieval-first, pas canonical-atom, pas de signature.**
- **Contextual AI / Pinecone Nexus / Squirro / Credal** : citations + gouvernance +
  audit. Tous *retrieval-first*, aucun canonical-first + signé.

**Namesakes « Nomos » (legaltech, encombré) :** nomoslab.in (raisonnement légal
vérifiable, on-prem — overlap réel mais légal-only), nomosai.co.uk (écossais,
early), nomos.infoset.co (dormant). **→ wordmark + vérif marque recommandés.**

**eQMS/CSV (adjacents evidence) :** ValGenesis (« VAL » gouverné, ALCOA+ *mais sur
données de process*, pas sur le savoir), Kneat, Veeva, RegASK (BYOC + audit trail,
workflow pas canonical). **Cible de vente potentielle : NOMOS comme couche canonique
en amont d'eux.**

**AEC code-checking :** UpCodes (11 M de codes US, Plan Review juin 2026, ~27
violations/projet — **US-only**), Verifi3D (BIM rules, encodées à la main),
PermitFlow/Symbium (workflow permis US). **Aucun ne fait la Suisse/EU.**

---

## 5. White space (positions défendables à prendre)

1. **« La couche source canonique des industries régulées »** — se positionner *en
   amont* de tout RAG (Norm AI, RegASK, ValGenesis inclus) comme le certificateur qui
   garantit que le savoir entrant est propre, haché, traçable, ALCOA+. Positionnement
   *infrastructure*, vendable **à** ces acteurs.
2. **Traçabilité cross-standard d'une décision** — un bâtiment doit satisfaire structure
   + énergie + incendie + accessibilité + environnement *simultanément* ; aucun outil ne
   trace une décision de design à travers *tous* les atomes réglementaires applicables
   en un seul evidence pack. Le mesh multi-domaine y répond directement.
3. **Evidence packs EU AI Act** — high-risk déclenche **août 2026** ; doc technique +
   traçabilité deviennent légalement obligatoires ; personne ne génère d'evidence pack
   signé/canonique/ALCOA+ qui satisfait ces exigences. **Fenêtre datée.**
4. **Formalisation de l'expertise (métier + canon promu)** — laisser un praticien
   *canoniser* son savoir tacite avec les mêmes garanties de traçabilité qu'une source
   d'autorité : sans précédent marché, dans *tout* vertical.
5. **AEC suisse** — codes cantonaux + SIA + droit de l'aménagement, AI-native, avec
   traçabilité canonique : inoccupé par tout concurrent bien financé. (= Aedifica.)

---

## 6. Risques honnêtes

- **Moats bornés (12–24 mois).** Norm AI/Vectara/Contextual peuvent ajouter
  atomisation canonique + signature si la demande se confirme. Le standard de facettes
  (P3) pourrait être publié par un std-body / vendeur KM en 12–18 mois.
- **Incumbents au-dessus.** Wolters Kluwer + Thomson Reuters ont le contenu, la
  distribution, les experts. Si NOMOS reste sur le réglementaire, ils gagnent. **Le
  métier + le crypto-evidence + le cross-domaine sont les seuls angles où ils ne sont
  pas.**
- **Le piège « couche horizontale ».** « Core domain-agnostic + packs » est le claim le
  plus banalisé du secteur. Sans la synthèse défendable (supply-chain du savoir +
  métier), c'est indéfendable.
- **RegTech = commodity à 20 Md$.** Ne jamais se présenter comme « plateforme de
  compliance réglementaire » — c'est perdu d'avance contre CUBE/Archer.
- **ALPHA-en-apparence.** NOMOS est éprouvé mais sa doc/release le sous-vend ; pour un
  achat régulé, l'absence de qualification *formelle* (vs la maturité réelle) sera lue
  comme un risque. Le retard de release management est lui-même un risque commercial.

---

## 7. À capitaliser (adopter, ne pas réinventer)

| Pilier | Adopter directement |
|---|---|
| P1/P2 | **ALCE** (test citation), **FActScore/decompose-verify** (vérif d'atomes), **Trust-Align** (cite-or-abstain + Trust-Score), **Self-RAG** ([IsSUP]), **DeepEval+GitHub Actions** (gate CI) |
| P3 | **SKOS** (1 scheme/axe), **OWL `disjointUnionOf`** (orthogonalité), **SHACL** (validation facettes), **ISO 25964** (révision 2026 AI), **VocBench/Skosmos**, **AEC3PO/ACCORD** (juridiction+phase AEC) |
| P4 | Littérature distracteurs (**ACL 2025**) comme *preuve empirique* du problème ; **conformal context engineering** (ECIR 2026) ; filtres `must_not`/`nin` au niveau base (Qdrant/Weaviate + RLS) |
| P5 | **Wikidata ranks** (preferred/normal/deprecated + raison), **modèle Vault** (CANONICAL/WORKING/…), **PKG API** (access_rights+provenance RDF), workflow **Collibra/MDM** comme référence |
| P6 | **OpenFisca** (calcul) + **Catala** (fidélité texte) + **L4** (MCP-natif, REST, vérif formelle, actif 2026) comme *substrat d'exécution* ; **LegalRuleML** + **Akoma Ntoso** + **ELI** (format/identité) ; **SAT-Graph/LRMoo** (point-in-time) ; **SCF** (crosswalk libre) |
| P6-gén | **BFO** (ISO 21838-2) → **IOF Core** (mid-level industriel) → pack domaine ; pattern **NFDIcore** (core + modules) ; **ODP** (obligation/process/evidence) ; étudier **WK FAB** + **Palantir OSDK** comme références d'archi |
| P7 | **in-toto** (enveloppe DSSE par étape), **Sigstore/Rekor** (signature keyless + log append-only), **W3C PROV-O** (graphe provenance), **VC 2.0** (claims signés), **RFC 9162/Merkle** (body-ledger + preuves d'inclusion), **CycloneDX ML-BOM/SPDX 3.0** (schéma BOM), **ALCOA+/Part 11/Annex 11** (framing régulé) |

---

## 8. Ce que ça change pour le pivot méta + la suite

Le Step 0 **valide le pivot méta, avec une correction de cadrage** :

- ✅ **Généraliser au métier est juste** — c'est le seul white space que ni incumbent ni
  fast-mover n'occupe. Le pivot « NOMOS = maillage canonique réglementaire + métier » est
  confirmé par l'absence concurrentielle.
- ⚠️ **Mais ne pas mener avec « couche horizontale »** (claim banalisé). **Mener avec la
  synthèse défendable** : *« la supply-chain du savoir certifié »* (canonical-first +
  cite-or-abstain + evidence signé + claim-boundary), **étendue au métier**, dont
  built-environment/AEC suisse est la première preuve verticale datée.
- 🎯 **Prioriser les piliers par défendabilité**, pas par facilité : P1+P2+P7 (le cœur
  neuf) d'abord ; P3/P4/P5 (différenciants mais contestables) comme mécaniques de
  support ; P6 réglementaire **jamais** comme produit autonome.
- ⏱️ **Jouer les fenêtres datées** : AEC suisse (inoccupé) + evidence packs EU AI Act
  (août 2026). Ce sont les deux avantages de premier entrant concrets.

**Décision inchangée à prendre (inchangée par la recherche, mais mieux informée) :**
acter le pivot méta avec ce cadrage « supply-chain du savoir certifié + métier », et
choisir le 2ᵉ vertical de preuve (au-delà de l'AEC) — sachant que la recherche suggère
**les evidence packs EU AI Act** comme 2ᵉ vertical idéal (white space daté, et c'est
déjà la zone de confort documentaire de NOMOS).

---

## Annexe — sources clés (à re-vérifier avant usage externe)

Académique : ALCE (2305.14627), Self-RAG (2310.11511), Trust-Align (2409.11242),
FActScore (2305.14251), CRAG (2401.15884), distracteurs ACL 2025 (2505.06914),
medical-device applicability (2506.18511), SAT-Graph/LRMoo (2505.00039, 2506.07853),
RAG security taxonomy (2604.08304), faceted classification (Ranganathan, CRG,
Broughton), BFO/DOLCE/IOF/NFDIcore, RAGShield (2604.00387).
Standards : Akoma Ntoso, LegalRuleML, ELI/ECLI, W3C PROV-O, VC 2.0, in-toto, SLSA,
Sigstore, RFC 9162, C2PA, ALCOA+, 21 CFR Part 11 / Annex 11, SKOS/ISO 25964/OWL/SHACL.
RaC : OpenFisca, Catala, Blawx/s(CASP), L4, RegelSpraak.
Industriel : Wolters Kluwer FAB, Thomson Reuters CoCounsel, Palantir Ontology,
Harvey, Norm AI, Vectara, Contextual AI, Pinecone Nexus, Squirro, Credal, Glean, Guru,
ValGenesis, Kneat, Veeva, RegASK, CUBE, Archer, AscentAI, UpCodes, Verifi3D, PermitFlow.
OSS : in-toto, Sigstore/Rekor, OpenLineage, GraphRAG/LightRAG, VocBench/Skosmos,
TopBraid EDG, QuantumPipes Vault, RAGAS/TruLens/DeepEval, ACCORD/AEC3PO.
(URLs complètes dans les rapports d'agents de la session.)
```

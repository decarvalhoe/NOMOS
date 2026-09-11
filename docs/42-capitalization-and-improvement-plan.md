<!-- Porté depuis l'analyse stratégique Aedifica — origine du pivot CKM (epic #481).
Mapping de ce dossier : 39 = plan maître · 40 = architecture méta (faceting/lens/promotion)
· 41 = positionnement état de l'art · 42 = capitalisation & amélioration.
Les références internes "nomos-*.md" pointent vers ces mêmes docs (noms Aedifica d'origine). -->

# NOMOS — plan de capitalisation & d'amélioration (Step 0.b)

> Re-passage des sources déjà compilées (les 8 rapports de recherche), réorienté vers
> l'**action** : (A) mise à niveau sur les composants commoditisés, (B) champs faibles
> à combler, (C) amplification des forces où on se démarque.
> Doctrine : **les concurrents sont un tremplin et un objectif, jamais un frein.** Rien
> n'est à jeter. On pille honnêtement tout ce qui est utilisable — standards ouverts,
> recherche, OSS permissif — sans jamais copier code propriétaire, contenu, ni IP.
> Date : 2026-06-08.
> Statut : baseline d'amélioration historique. Plusieurs slices bornées
> (point-in-time, ontologie dans le gate, RAG eval, renvois, ruleexec) sont
> désormais livrées ; la [matrice générée](../.vrc-wiring-matrix/wiring-matrix.md)
> est la vérité courante. Les lignes ci-dessous restent des sources/trajectoires,
> pas une liste implicite de capacités manquantes.

---

## 0. Garde-fous IP/licence (le cadre du « pillage honnête »)

Trois régimes d'emprunt, à ne jamais confondre :

| Régime | Quoi | Ce qu'on peut faire |
|---|---|---|
| **Standards ouverts** | SKOS, OWL, SHACL, PROV, in-toto, SLSA, Akoma Ntoso, LegalRuleML, ELI/ECLI, ALCOA+, BFO (ISO 21838), RFC 9162, C2PA, CycloneDX/SPDX | **Adopter librement** — c'est fait pour ça |
| **OSS permissif** (MIT/Apache/BSD) | Sigstore, DeepEval, RAGAS, TruLens, ALCE, Self-RAG, Qdrant, LlamaIndex, VocBench, Skosmos, RDFLib, Microsoft GraphRAG | **Intégrer/forker** en respectant l'attribution |
| **OSS copyleft fort (AGPL/GPL)** | OpenFisca-Core (AGPL), LexNLP (AGPL), certains datasets | **Isoler derrière une frontière process/API** (pas de linking dans le produit proprio) ou éviter. ⚠️ vérifier au cas par cas |
| **Produits commerciaux** | WK FAB, Thomson Reuters, Harvey, Norm AI, Vectara, Glean, Guru, ValGenesis | **S'inspirer du concept / interaction / benchmark** uniquement. Jamais : code, contenu propriétaire, corpus, trade-dress |
| **Contenu payant** | SIA, ISO, GAMP 5 | **Jamais de texte intégral**. Hash + crosswalk (NOMOS le fait déjà) |

Plus : **marque** « Nomos » encombrée → wordmark distinctif + clearance ; **FTO patent**
check avant tout claim de nouveauté public (certains brevets RAG-governance peuvent
exister — l'intégration reste ton espace, mais vérifie).

---

## A. Mise à niveau — atteindre l'état de l'art vite (composants commoditisés)

> Principe : sur tout ce qui est commoditisé, **ne rien réinventer** — adopter le
> meilleur existant pour libérer l'énergie vers la synthèse.

| Besoin (pilier) | Adopter | Licence | Effort | Bénéfice |
|---|---|---|---|---|
| Mesurer la qualité de citation (P2) | **ALCE** (citation recall/precision via NLI) | ouvert | S | Métrique objective de « bien cité » dès le départ |
| Vérifier les faits par décomposition (P1/P2) | **FActScore / OpenFActScore** (decompose → verify) | ouvert | M | Vérificateur d'atomes réutilisable dans la gate |
| Gate CI bloquante sur la fidélité (P1) | **DeepEval + GitHub Actions** (seuil faithfulness → bloque la PR) | Apache | S | La « strict gate » devient un gate CI réel, tout de suite |
| Éval RAG continue (P2) | **RAGAS** (faithfulness/relevancy) + **TruLens** (RAG triad) + **ARES** | Apache/MIT | S | Observabilité + non-régression |
| Scoping retrieval au niveau base (P4) | **pgvector + RLS** (puis Qdrant payload filters) + reranker cross-encoder | ouvert | M | Lens enforced *avant* génération (consensus d'ingénierie mûr) |
| Substrat d'exécution de règles (P6) | **L4** (MCP-natif, REST, vérif formelle, actif 2026) ; **OpenFisca** pour le calcul (⚠️ AGPL → via API) ; **Catala** pour la fidélité texte | mixte | L | Le déterministe certifié sans construire un moteur de règles |
| Vocabulaires contrôlés (P3) | **SKOS** + **VocBench**/**Skosmos** ; **SHACL** pour valider les facettes | ouvert | M | Facettes authored + servies + validées proprement |
| Signature/attestation (P7) | **in-toto** (DSSE) + **Sigstore/Rekor** (keyless + log) | Apache | M | Local DSSE est livré ; verify offline #637 et émission non-prod #645 avancent autonomement ; activation production/Rekor #638 reste un claim externe, pas un gate produit |
| Identité/format des sources légales (P6) | **Akoma Ntoso** + **ELI/ECLI** + **LegalRuleML** | ouvert | M | Citations déréférençables + versionnement FRBR gratuit |

**Sortie A :** NOMOS « à niveau » sur tout le substrat commoditisé en réutilisant
~10 briques éprouvées au lieu de les réécrire.

---

## B. Champs faibles à combler (les gaps honnêtes)

> Là où la recherche a montré que NOMOS est mince ou absent — et quoi emprunter pour
> remplir.

1. **Point-in-time / versionnement temporel du droit** (faible). → Adopter le modèle
   **SAT-Graph RAG / LRMoo** (de Martim 2025) : Work→Expression versionnée, événements
   législatifs comme nœuds, requête « le droit au 2024-03-01 ». Combler avec
   **FRBR + ELI** côté identité. *Indispensable pour « certifié » : citer la bonne
   version.*
2. **Anti-hallucination mesurable / faithfulness ≠ correctness** (faible). → Intégrer
   le garde-fou de **2412.18004** (jusqu'à 57 % de citations post-rationalisées) :
   passe de vérification d'*entailment* par atome (NLI) avant d'émettre. Adopter
   **Trust-Align** (refus appris) + **Self-RAG [IsSUP]** (signal de support par segment).
3. **Architecture d'ontologie rigoureuse pour core+packs** (faible — aujourd'hui un
   `domain` plat). → Adopter le pattern académique **BFO (ISO 21838) → IOF Core →
   pack domaine**, template **NFDIcore** (core + modules), **ODP** (patterns
   obligation/process/evidence). Rend la généralisation *défendable* au lieu de
   « couche horizontale » banalisée.
4. **Stack RAG de prod (hybride + rerank + graph de renvois)** (faible/absent). →
   Adopter **hybride vecteur+BM25** + reranker ; pour les renvois entre règlements
   (« sous réserve de l'art. X »), un **graphe de cross-références déterministe**
   (parsé par règle, pas LLM) inspiré de **BifrostRAG** ; GraphRAG/LightRAG en option.
5. **Éval orientée régulé** (faible). → **LegalBench-RAG** + **LRAGE** comme harnais ;
   **RAGChecker** pour isoler retriever vs generator. Gate de release sur citation
   recall + bande d'abstention calibrée.
6. **Connecteurs de sources** (à industrialiser). → S'inspirer de **Juriscraper /
   Free Law** (framework de scrapers open) + **legislation.gov.uk** (archi
   point-in-time, code/données ouverts) comme *références d'architecture*.
7. **Qualification formelle vs maturité réelle** (faible — risque commercial). →
   Combler le retard doc/release : **ALCOA+ / 21 CFR Part 11 / Annex 11** comme
   vocabulaire d'evidence ; **CycloneDX ML-BOM / SPDX 3.0** comme schéma de BOM. Pas de
   nouvelle techno — du *packaging* d'evidence qui rend la maturité réelle *lisible*.

---

## C. Amplifier les forces (aller plus loin là où on se démarque)

> Les 3 territoires génuinement en avance — et les sources qui les poussent encore plus
> loin.

### C1. La « supply-chain du savoir certifié » (P1+P7) — le drapeau
- **Rendre réel** : prédicats **in-toto** custom pour chaque étape (ingestion → canon →
  embedding → feed) ; **body-ledger** ancré façon **RFC 9162/Merkle** (preuves
  d'inclusion = 0 octet non couvert, vérifiable) ; **claim-boundary** comme prédicat
  **signé** (« je ne peux pas prouver Y, donc je refuse »).
- **Pousser plus loin** : s'inspirer de **AIBoMGen** + **Sigstore model-transparency**
  (signer des artefacts ML) ; **W3C VC 2.0** pour des credentials d'evidence
  présentables ; **C2PA** pour la provenance des documents humains promus.
- **Pourquoi ça gagne** : multiple sources 2025-26 confirment ce gap explicitement
  (taxo sécurité RAG ; ALCOA+ appliqué à l'IA). Personne ne signe la *transformation
  source→canon→RAG*. C'est l'angle le plus vide.

### C2. Cite-or-abstain strict + trust tiers (P2)
- **Pousser plus loin** : **Trust-Score** (composite groundedness + refus) comme cible
  d'entraînement (DPO) ; **CRAG** (classifieur de pertinence → action corrective) en
  bolt-on léger ; exposer `certified/indicative/unverified` comme propriété *API* de
  chaque réponse (sans équivalent marché).
- **Tremplin concurrent** : **Harvey admet le gate strict non résolu** → c'est
  exactement le wedge. Publier un **bench façon BigLaw Bench** pour le prouver
  publiquement.

### C3. Maillage réglementaire + métier (P6-gén) + facettes/lens/promotion (P3/P4/P5)
- **Facettes (P3)** : formaliser le standard d'axes contrôlés (le différenciant) avec
  **SKOS+OWL `disjointUnionOf`+SHACL** ; emprunter **AEC3PO/ACCORD** pour
  juridiction+phase côté pack archi.
- **Lens (P4)** : formaliser l'épistémologie 2-niveaux (applicabilité *objective* vs
  activation *subjective*) ; pousser avec **conformal context engineering** (ECIR 2026,
  garanties statistiques sur la rétention de contexte) + **hard-negative mining**
  (2505.18366) pour entraîner le reranker à écarter le plausible-parasite ; **TARG**
  (ne récupérer que si nécessaire). La littérature distracteurs (ACL 2025 : −6 à −11
  pts) est la *preuve empirique* du problème → à mettre dans le pitch.
- **Promotion (P5)** : emprunter le **modèle de rangs Wikidata** (preferred/normal/
  deprecated + raison) et le **Vault** OSS (CANONICAL/WORKING/… + poids/demi-vie) comme
  références ; le différenciant à *concevoir* = **canon préservant la confidentialité**
  (canon sans exposition) + praticien-autorité + cycle certificat/révocation.
- **Métier (le white space)** : c'est le seul angle qu'aucun incumbent n'occupe →
  investir le **vocabulaire d'axes métier** (SKOS) + les **bibles métier** comme classe
  de source de 1er rang. Étudier **WK FAB** et **Palantir OSDK** comme *références
  d'architecture* core+packs (concept, pas copie).

---

## D. Compétiteurs = tremplin (takeaways légalement utilisables)

| Acteur | Ce qu'ils prouvent | Takeaway légal (concept/UX/bench, jamais code/contenu) |
|---|---|---|
| **Wolters Kluwer FAB** | Le modèle core+packs multi-domaines *marche* à l'échelle | Étudier le split core/pack sur 5 divisions comme réf d'archi |
| **Thomson Reuters CoCounsel** | Core de raisonnement partagé multi-vertical | Pattern « core + grounding contenu autoritaire » |
| **Palantir Ontology/OSDK** | Exposer un cœur sémantique en API typées | Pattern d'exposition du core comme SDK pour packs |
| **Norm AI** | « Enterprise Data Layer » d'objets autoritaires | Concept : grounder les agents sur objets canoniques, pas docs bruts |
| **Vectara** | HHEM est un **modèle ouvert** (HF) ; Guardian Agents | Utiliser le HHEM *open* comme éval ; concept détecte+corrige runtime |
| **Harvey** | BigLaw Bench (méthodo) ; admet cite-or-abstain non résolu | Publier ton propre bench ; planter le drapeau sur le gate strict |
| **Guru** | « Verified RAG » : SME valide, expiry, bar l'IA du non-vérifié | Pattern d'interaction de promotion/validation |
| **Copilot « Official Source »** | Marquer une source comme canon | Pattern d'interaction « promote-to-canon » |
| **UpCodes** | Code-checking AEC productisé (US-only) | Forme produit + métrique valeur (~27 violations/projet) ; **Suisse = ton white space** |
| **ValGenesis / RegASK** | Evidence/validation workflow régulé | GTM : vendre NOMOS comme *couche canonique amont* à ces acteurs |
| **legislation.gov.uk** | Législation-as-data point-in-time, code/données ouverts | Réf d'archi à copier (ouvert) |
| **Free Law / Juriscraper** | Corpus + scrapers ouverts | Framework de connecteurs (ouvert) |

---

## E. Séquencement par effet de levier (pas par facilité)

1. **Rendre le drapeau réel & mesurable** (C1+C2) : in-toto/DSSE + ALCE/Trust-Align/
   DeepEval → la « supply-chain du savoir + cite-or-abstain + evidence signé localement »
   passe de concept à *mesuré et shippable*. Sigstore est scindé par
   ADR-VRC-0004 (#637 verify, #645 émission non-prod, #638 activation production) ;
   la publication Rekor ne bloque pas cette sortie produit.
2. **Rattraper le substrat** (A) : pgvector+RLS+reranker (P4), L4/OpenFisca via API (P6),
   harnais d'éval. Vite, en réutilisant l'existant.
3. **Combler les faibles** (B) : point-in-time (SAT-Graph/LRMoo), faithfulness-NLI,
   archi ontologique BFO/IOF, graphe de renvois, packaging ALCOA+/BOM.
4. **Formaliser les mécaniques différenciantes** (C3) : standard de facettes (SKOS/OWL/
   SHACL+BFO), lens (conformal + 2-niveaux), promotion (confidentialité-préservante).
5. **Preuve cross-domaine** : pack built-environment (AEC suisse, white space) + 2ᵉ
   vertical **evidence packs EU AI Act** (fenêtre août 2026).

> Logique : on **rattrape en pillant** (1-2), on **comble en empruntant** (3), on
> **se démarque en s'enrichissant** (4-5). Chaque concurrent plus avancé devient une
> brique de référence ou une cible datée — jamais un frein.

---

## Annexe — où retrouver le détail
Les 8 rapports d'agents de la session portent les URLs complètes (papiers, repos,
produits) par pilier. Doc compagnon : `nomos-state-of-the-art-positioning.md` (le
verdict honnête) et `nomos-knowledge-mesh-and-built-environment.md` (l'architecture
méta). ⚠️ Re-vérifier les arXiv très récents + claims produits avant tout doc externe.
```

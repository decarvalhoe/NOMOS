# 45 — Plan de clôture vision → réalité (master plan d'exécution)

> Statut : `plan` (forward-looking, pas une capacité livrée). Date : 2026-06-11.
> Ancres vision : [ADR-0001 canonical-first](adr/0001-canonical-first-method.md) ·
> [01-method-overview](01-method-overview.md) · [39-pivot CKM](39-canonical-knowledge-mesh-pivot.md) ·
> [40-architecture mesh](40-knowledge-mesh-architecture.md) · [41-état de l'art](41-state-of-the-art-positioning.md) ·
> [42-capitalisation](42-capitalization-and-improvement-plan.md) · [43-doctrine](43-development-doctrine.md) ·
> [public-claim-boundary](public-claim-boundary.md) ·
> [47-roadmap lanes](47-roadmap-lanes-and-risk-based-validation.md).
> Origine : audit écart documentation / intention / réalité du 2026-06-11 (post-#543).
> Objet : faire de NOMOS le logiciel décrit par ses ADR et sa vision — **la supply-chain
> du savoir certifié, reproductible pour n'importe quel domaine, corpus ou métier** —
> en fermant chaque écart constaté, sous le tamis de la doctrine (« done = preuve
> adversariale », « sidecar Python = PARTIAL quand la mécanique attendue est au core »).
> **Périmètre depuis ADR-VRC-0004 (2026-09-05)** : clôture produit et DevOps.
> Le QMS, les records, validations, acquisitions et claims réglementés suivent
> la roadmap régulée indépendante [28](28-regulated-compliance-closure-plan.md).

---

## 0. Thèse du plan — trois mouvements

1. **Performer là où la valeur se crée.** Les trois territoires « génuinement en
   avance » (doc 41 §0) — supply-chain du savoir signée (P1+P7), cite-or-abstain +
   trust tiers (P2), maillage réglementaire+métier — passent intégralement le tamis
   doctrine : moteur Go + caller de production + test adversarial + artefact signé.
   Ce point de départ est désormais fermé : cite-or-abstain vit dans le moteur
   Go, son sidecar Python consomme le verdict, et le bench public le mesure.
2. **Se mettre à niveau par adoption, jamais par réinvention.** Sur tout ce qui est
   commoditisé (retrieval, éval RAG, vocabulaires, exécution de règles), on adopte les
   tremplins du registre doc 42 (standards ouverts → adopter ; OSS permissif →
   intégrer ; AGPL → isoler ; commercial → concept seulement ; contenu payant →
   hash + crosswalk). L'énergie libérée va à la synthèse.
3. **Institutionnaliser la reproductibilité.** « N'importe quel domaine, corpus,
   métier » cesse d'être une promesse et devient un contrat testé : mécaniques 100 %
   core, pack domaine 100 % déclaratif (vocabulaires + connecteurs + golden corpus +
   lens-presets + claim boundary), harnais de conformité `pack validate`, et une
   métrique unique : **zéro changement core requis par nouveau pack**.

Règle transverse : chaque chantier ci-dessous liste sa **preuve exigée** (tamis
doctrine §2.3) et le **claim débloqué** (claim ladder du public-claim-boundary).
Aucun claim n'avance avant sa preuve. CKM-00 (non-régression) reste vert en
continu. **Une preuve calendaire, signature humaine, acquisition, approbation ou
écriture publique ne devient jamais une dépendance d'implémentation** : elle
bloque uniquement son claim dans la roadmap 28. Le dispatcher exécutable est
`docs/roadmap-lanes.yaml`.

---

## 1. Matrice écarts → chantiers (traçabilité de l'audit)

| # | Écart constaté (audit 2026-06-11) | Chantier(s) |
|---|---|---|
| E1 | Code core réel mais **câblage tardif** : capacités livrées non enregistrées dans la CLI, zéro caller de production, fermées seulement en 2ᵉ passe (#539/#540/#543) | G3 (matrice de câblage générée), G4 (caller exigé à l'acceptation) |
| E2 | **4 mécaniques CKM en sidecar Python** = PARTIAL par doctrine : cite-or-abstain/faithfulness, canon promotion, point-in-time, ontologie facettes | A1, A2, A3, D4 |
| E3 | **QMS rédigé, exécuté à zéro** : 0/20 types de records, owners `not_assigned`, aucune release exécutée via SOP | Roadmap régulée indépendante doc 28 (interface F ci-dessous, jamais phase gate produit) |
| E4 | **Contradiction doc-vs-doc** : doc 40 (« éprouvé multi-environnements ») vs claim boundary (« POC-scoped, single run ») | G1, G4 (guard étendu aux docs stratégiques) |
| E5 | **1ᵉʳ vertical (AEC) dépendant de capacités absentes** : PDF = stub, retrieval prod = inexistant, seam Aedifica non prouvé bout-en-bout | C1-C2, B1, E (chantier consommation), D5 |
| E6 | **Fenêtres datées vs roadmap non compressée** : moats 12–24 mois, EU AI Act août 2026, fidélité portable encore en v0.2 | H (séquencement), D3 (pack AI Act borné pilote) |
| E7 | Périphérie ambiguë : control-plane = code mort testé, sdk/policies vides, examples squelettes | H-Phase 0 (décision ADR : câbler ou archiver), G2 |

---

## 2. Chantier A — Le drapeau : rendre la synthèse défendable réelle, mesurée, signée

> Ancre : doc 41 §3 (« P1+P2+P7 forment une seule thèse, c'est le drapeau à planter »),
> doc 42 §C. Écarts E2. Priorité maximale : c'est ce qui se défend, et c'est borné
> dans le temps (Harvey admet le gate strict non résolu ; fast-movers à 12–24 mois).

| # | Action | Tremplin (régime doc 42 §0) | Preuve exigée | Claim débloqué |
|---|---|---|---|---|
| A1 | **Cite-or-abstain dans le moteur Go.** Porter la logique de `scripts/regulated_rag_answer_evidence.py` + `tests/test_ckm_faithfulness_recompute.py` dans `cli/internal/answer/` : faithfulness **recalculée depuis les spans retrouvés** (recouvrement span↔claim, jamais déclarée), verdict cite/abstain, seuils configurables, commande `nomos answer gate`, intégration au strict gate, `trust_tier` exposé comme propriété de chaque réponse dans l'artefact rag-answer-evidence. Le sidecar Python devient un consommateur du verdict Go, pas la source. | ALCE (citation recall/precision, NLI) ; Trust-Align (refus appris, Trust-Score) ; Self-RAG `[IsSUP]` ; FActScore (decompose→verify) ; DeepEval + GitHub Actions (gate CI) ; HHEM (modèle ouvert, éval hallucination) — tous OSS permissif/ouvert | Test Go adversarial : citation falsifiée (span déplacé / hash altéré) → gate rouge ; réponse sans span → abstention forcée ; le bypass « no-text » (fermé côté script en #542) a son équivalent Go ; suppression du filtre → le test échoue | « Cite-or-abstain mesurable et bloquant, calculé depuis les spans, tiers exposés par réponse » — le wedge P2 |
| A2 | **Canon promotion dans le moteur Go (CKM-03).** `nomos canon promote/revoke/verify` : droits → validation → facettage (`provenance=user_promoted`) → `#Certificate` signé DSSE → entrée corpus filtrable par lens ; silo de confidentialité ; révocation propagée au retrieval (chunk exclu avec raison tracée). `scripts/ckm_canon_promotion_validate.py` devient vérification de conformité, pas implémentation. | Wikidata ranks (preferred/normal/deprecated + raison) ; modèle Vault (CANONICAL/WORKING/…) ; patterns Guru Verified RAG et Copilot « Official Source » (concept/UX seulement) ; PKG API (access_rights + provenance) | Tamper-fail certificat (1 octet → verify rouge) ; un atome promu ne sort **jamais** `certified`/`official` (étendre les tests facets au flux promotion) ; certificat révoqué → chunk exclu (test) ; confidentiel jamais émis hors silo (test) | « Bring-your-own-authority gouverné, confidentialité préservée » — P5, sans précédent marché (doc 41 §2) |
| A3 | **Point-in-time dans le moteur Go (CKM-11).** Modèle temporel d'atome (`valid_from`/`valid_to`, événements ; `supersedes` existe déjà) + résolveur `--as-of <date>` sur `atomize`/`bundle` ; identité FRBR/ELI pour les sources légales. `scripts/ckm_point_in_time_resolve.py` → conformité. | SAT-Graph RAG / LRMoo (Work→Expression versionnée, événements législatifs) ; FRBR + ELI ; legislation.gov.uk (référence d'archi point-in-time, code/données ouverts) | Requête as-of retourne la version en vigueur à la date ; une version remplacée n'est pas retrouvable à une date ultérieure sans flag historique explicite (test rouge sans le résolveur) | « Citer la bonne version à la bonne date » — précondition de tout `certified` temporellement honnête |
| A4 | **Sigstore scindé par réversibilité.** #637 vérifie hors ligne des bundles fournis ; #645 implémente l'émission contre des services injectés/non-production ; #638 possède seul l'autorisation OIDC et la publication append-only production. Tant que #638 n'est pas exécutée, le guard traite « NOMOS signe keyless en production » comme non-présent. | sigstore/cosign, Rekor, in-toto — frontières externes et endpoints injectés d'abord | #637 : tamper verify rouge ; #645 : E2E émission→bundle→verify local, production interdite ; #638 : entrée Rekor production + autorisation authentique | #637 « vérifie offline » ; #645 « émet non-production » ; #638 seul « production + transparency log » |
| A5 | **Packaging evidence inter-opérable.** Evidence packs émis aussi en BOM (CycloneDX/SPDX) ; plus tard VC 2.0 / C2PA pour les documents humains promus. | CycloneDX ML-BOM, SPDX 3.0, W3C VC 2.0, C2PA — standards ouverts | BOM validé par schéma ; hashes recoupés avec le body-ledger (calculé, pas déclaré) | « Evidence pack lisible par l'outillage supply-chain standard » |

**Sortie A :** les trois territoires « en avance » passent le tamis complet. Préparer
alors le **bench public** du gate (méthodo façon BigLaw Bench, doc 42 §C2) — c'est la
preuve externe du drapeau, à publier avant que la fenêtre concurrentielle se referme.

---

## 3. Chantier B — Mise à niveau par adoption (le commoditisé, vite)

> Ancre : doc 42 §A (« sur tout ce qui est commoditisé, ne rien réinventer »).
> Écarts E5 (substrat retrieval), risque doc 41 §6 (« rattrapage »).

| # | Action | Tremplin | Preuve exigée | Claim débloqué |
|---|---|---|---|---|
| B1 | **Harnais de retrieval de référence** (kit consommateur, hors core) : pgvector + RLS où la Lens devient le `WHERE` au niveau base ; reranker cross-encoder ; hybride BM25+vecteur. NOMOS reste le certificateur amont ; le harnais sert à prouver la conformité consommateur. | pgvector + RLS, Qdrant payload filters, reranker open ; littérature distracteurs (ACL 2025 : −6 à −11 pts) comme preuve empirique du besoin de Lens | Jeu de tests distracteurs sur golden corpus : accuracy avec Lens > sans Lens (mesuré) ; un chunk exclu par Lens n'est **jamais** retrouvé (test) | « Lens enforced avant génération, au niveau base » — la promesse anti-parasite devient mesurée |
| B2 | **Harnais d'évaluation continue** en CI : d'abord non bloquant, puis bloquant (pattern fail-open → fail-closed de `ci/README.md`). | RAGAS, TruLens, ARES (généraliste) ; LegalBench-RAG, LRAGE, RAGChecker (orienté régulé) — OSS | Seuils versionnés ; une régression de citation recall sous le seuil bloque la PR (test du gate lui-même) | « Éval RAG non-régressive, orientée régulé » |
| B3 | **Substrat d'exécution emprunté, jamais construit — slice protocole livrée (#578).** `nomos rule exec` passe les atomes `formula` à un process externe par protocole JSON versionné, avec trace source ; aucun évaluateur dans NOMOS, aucun résultat sans substrat. La démo et la frontière AGPL exigées sont livrées. #642 durcit le digest de requête et l'intégrité des valeurs persistées. Un adapter vers un moteur nommé sera une issue distincte après revue de sa licence ; son absence ne rouvre pas #578. | L4, OpenFisca, Catala comme adapters futurs ; licences évaluées par adapter | Démo bornée livrée ; #642 : chaque mutation expression/source/digest/statut/valeur/reason → rouge | Claim borné : « frontière externe tracée et fail-closed », aucune compatibilité L4/OpenFisca/Catala ni correction métier |
| B4 | **Validation SKOS/SHACL livrée (#580), authoring/serving séparé (#643).** `facet_shacl_gate.py` émet un graphe SKOS/RDF + prédicats NOMOS et le valide avec pySHACL ; un terme sur deux axes disjoints rougit. #643 ajoute une source d'authoring versionnée et un bundle statique distribuable ; VocBench/Skosmos restent des clients optionnels. ISO 25964 est un claim séparé après accès/licence. | SKOS, SHACL ; VocBench/Skosmos optionnels ; ISO 25964 hors claim courant | Validation négative livrée ; #643 : round-trip et build déterministe | Actuel : « graphe SKOS/RDF conforme aux shapes avec pySHACL » ; futur #643 : « authored et distribuable selon le profil NOMOS » |
| B5 | **Graphe de renvois déterministe** (« sous réserve de l'art. X ») parsé par règles, jamais par LLM. **Livré (VRC-43 #579)** : `nomos atomize crossref` — sept locutions juridiquement distinctes, chaque arête portant son texte verbatim et ses offsets ; `VerifyCrossRefGraph` re-découpe l'atome et refuse toute arête qui ne se reconstruit pas. Locution sans cible parsable = `unsupported` explicite ; cible hors corpus = `unresolved_target` avec le locateur verbatim. | Concept BifrostRAG ; GraphRAG/LightRAG en option | Renvoi connu du golden corpus présent dans le graphe avec span source ; renvoi inventé impossible (parsé, pas généré) | « Cross-références traçables entre atomes » |

---

## 4. Chantier C — Ingestion universelle : « n'importe quel corpus »

> Ancre : ADR-0001 + doc 01 (« chaque source active transformée en structure
> vérifiable ou explicitement exclue ») ; roadmap v0.2/v0.3 (doc 14). Écart E5 :
> le vertical AEC dépend du PDF (PGA/PAZ communaux) ; le légal dépend du HTML/XML.
> Discipline : **un format = une capability versionnée avec limites déclarées**
> (doc 14, principe 4) ; jamais de skip silencieux — unsupported explicite.

| # | Action | Tremplin | Preuve exigée | Claim débloqué |
|---|---|---|---|---|
| C1 | **Adapter PDF structure-aware, par échelle de claims** : (1) PDF né-numérique texte d'abord, spans = page + offsets/bbox locators ; (2) tagged-PDF ; (3) OCR/scanné explicitement hors claim tant que non prouvé. Le non-extractible devient unsupported record dans le body-ledger (pattern existant). | Bibliothèques permissives à instruire au registre licences (pdfium, poppler bindings, pdfcpu Go…) ; jamais de sur-claim OCR | Golden fixture PDF public (un PGA/PAZ communal) → feed source-backed + ledger 0 octet non couvert (unsupported explicites) ; mutation d'un octet du PDF → drift détecté ; suppression de l'adapter → gate rouge | « PDF né-numérique gouverné » — débloque le pack AEC (D5) |
| C2 | **Adapter HTML/XML** (Fedlex livre HTML/XML ; Akoma Ntoso quand disponible). Spans = chemin DOM + offsets. Souvent plus prioritaire que DOCX pour le légal. | Akoma Ntoso, ELI/ECLI ; tree-sitter HTML (déjà dans la toolchain) | Même barre que C1 sur fixture Fedlex ; identité ELI préservée dans les locators | « Sources légales en ligne ingérées avec identité » |
| C3 | **Adapter DOCX** (OOXML = zip XML ; spans = XML path locators). Après C1/C2. | Lecture OOXML standard | Même barre que C1 | « DOCX gouverné » |
| C4 | **Industrialisation des connecteurs.** Étendre le registre au-delà des 2 actuels (OFS, Fedlex) : swisstopo/STAC, RDPPF/ÖREB, géoportails cantonaux, pipeline PDF communal, sidecar hash-only SIA. Contrat connecteur inchangé : fetch réel + hash réel + evidence, no-full-text (doctrine §2.3, #539). | Juriscraper / Free Law (framework de scrapers, ouvert) ; legislation.gov.uk (référence d'archi) | Chaque connecteur a son test live skippable + fixture offline ; un hash synthétique dans un manifest → guard rouge (acquis #539, à maintenir) | « Registre de sources d'autorité CH opérationnel » |
| C5 | **Kit obligatoire par adapter** : fixtures + claim boundary du format + taxonomie unsupported + fixtures de gate fidélité. Aucun format annoncé sans ce kit. | — (discipline interne) | CI : un adapter sans kit complet ne peut pas être enregistré (fail closed) | Chaque claim de format mappé à son evidence (règle du claim boundary) |

---

## 5. Chantier D — Le kit vertical reproductible : « n'importe quel domaine, n'importe quel métier »

> Ancre : doc 40 §7 et §13 (« mécaniques au core, valeurs au pack ; aucune mécanique
> core dupliquée dans le pack, aucune spécificité pack remontée au core »).
> C'est ici que la promesse de généralité devient un **contrat testé**, pas un slogan.

| # | Action | Tremplin | Preuve exigée | Claim débloqué |
|---|---|---|---|---|
| D1 | **Contrat de pack domaine (CUE)** : un pack = vocabulaires SKOS par axe (`discipline`, `activity`) + registre d'autorité des sources + connecteurs + lens-presets + golden corpus + instance de claim boundary + scorecard d'applicabilité. **Rien d'autre.** | SKOS ; pattern NFDIcore (core + modules) ; étude d'archi WK FAB / Palantir OSDK (concept seulement) | `cue vet` du contrat ; un pack contenant de la mécanique (code) est rejeté | « Le pack est déclaratif » |
| D2 | **`nomos pack validate` en Go** : conformité du pack + exécution complète du golden corpus (scan → feed → ledger → gate → bundle) + résolution des lens-presets + présence claim boundary + alignement ontologique (D4). | — | Pack mutilé (vocab manquant / golden corpus rouge / preset cassé / claim boundary absent) → **fail closed**, chaque cas testé | « Conformité de pack vérifiable par gate » |
| D3 | **2ᵉ vertical = pack « EU AI Act evidence »** (fenêtre août 2026 ; zone de confort documentaire de NOMOS, doc 41 §8). Tranche la décision ouverte n°2 du doc 40 §14. Portée pilote bornée : « evidence pack pilot-grade », jamais « conformité AI Act certifiée ». **Livré (VRC-22 #565)** : `docs/regulated/domain-packs/eu-ai-act/` — 3 axes / 14 termes alignés BFO→IOF, ancre EUR-Lex datée (CELEX 32024R1689), 3 presets, corpus doré synthétique 4 docs → 32 nœuds, `pack validate` en CI. Corpus synthétique par **fidélité**, pas par licence (la réutilisation EUR-Lex est autorisée) : le pack enregistre où le texte fait autorité, jamais ce qu'il dit. | Exigences doc technique/traçabilité AI Act (texte officiel = source d'autorité ingérée) ; ALCOA+/Part 11 comme vocabulaire d'evidence | Le pack AI Act passe `pack validate` ; son golden corpus tourne la chaîne complète ; claim boundary du pack relu par le guard | « La généralité cross-métier est prouvée par un 2ᵉ vertical réel » (exigence CKM-04) |
| D4 | **Ontologie portée dans le gate — livrée.** `pack validate` rend le verdict d'alignement BFO → IOF Core → pack ; un axe non aligné rejette le pack. `scripts/ckm_facet_ontology_validate.py` reste un exemple/non-régression hors moteur, pas la source du verdict. | BFO (ISO 21838), IOF Core, NFDIcore, ODP | Axe de facette non aligné → pack rejeté (test) | « La généralisation est défendable, pas une couche horizontale banale » (doc 41 §2 P3) |
| D5 | **Pack AEC suisse (mince)** : vocabulaires SIA/disciplines/activités, connecteurs CH (C4), golden corpus VD/Lausanne, lens-presets archi (« archi-conception », « DT-chantier », « permis »). Dépend de C1/C2. | AEC3PO/ACCORD (juridiction + phase), IFC/buildingSMART (vocab) | Golden corpus AEC vert de bout en bout via `pack validate` ; preuve distracteurs B1 jouée sur les presets archi (conception vs DT) | « 1ᵉʳ vertical prouvé sur le terrain inoccupé » (doc 41 §5) |
| D6 | **Métrique de reproductibilité** : « changements core requis par nouveau pack » — publiée par pack, cible **0**. **Première mesure réelle (VRC-22, pack AI Act) : 1 changement core, pas 0** — l'axe ouvert `risk_tier` ([ADR-0002](adr/0002-risk-tier-open-facet-axis.md)), parce que la classification par risque est le concept central de l'AI Act et qu'aucun des huit axes ne la porte. Le guard `pack_core_coupling_check.py` rend `core changes justified by ADR`. La cible reste 0 pour le pack N+2 : un dixième axe exigera une nouvelle ADR. | — | La PR du pack N+1 (AI Act, puis suivant) ne touche que les répertoires de pack ; tout diff core dans une PR de pack = revue bloquante avec justification ADR | « Reproductible pour n'importe quel domaine » devient une métrique, pas une promesse |

---

## 6. Chantier E — Consommation prouvée : le seam et le harnais consommateur

> Ancre : doc 43 §1 (« un consommateur dépend de l'artefact, jamais du code ») ;
> bundle contract aligné (#534-536). Écart E5 : **aucun consommateur réel n'a encore
> consommé un bundle facetté en production.**

| # | Action | Preuve exigée | Claim débloqué |
|---|---|---|---|
| E-1 | **Kit de conformité consommateur** publié avec le bundle contract : importeur de référence + tests que tout consommateur (Aedifica, RBOK, autres) exécute ; refus des valeurs de facettes locales au consommateur (acquis `ckm_bundle_validate.py`, à maintenir). | Le kit rejette un bundle altéré (hash, facette inconnue, version de schéma) — chaque cas testé | « Consommer NOMOS = passer un kit, pas lire notre code » |
| E-2 | **Preuve Aedifica (W19)** : bundle versionné consommé en CI Aedifica derrière flag, retrieval scopé par Lens, réponses citées avec tier, OFS-direct restant le défaut. NOMOS fournit fixture + contrat ; la preuve vit côté consommateur et est **référencée** dans l'evidence NOMOS (jamais résumée à sa place). | CI Aedifica verte sur le chemin bundle, lien d'evidence enregistré côté NOMOS | « Un consommateur réel consomme un bundle facetté » — aujourd'hui interdit de le dire |
| E-3 | **RBOK runtime E2E reste vert** (harnais CKM-00). | CI existante | Zéro régression (doctrine §2.1) |

---

## 7. Interface F — Roadmap Régulée Indépendante

> La roadmap QMS/evidence a été extraite vers le [plan 28](28-regulated-compliance-closure-plan.md)
> par ADR-VRC-0004. Elle ne constitue plus une phase de livraison produit. Un outil
> régulé peut être développé puis validé plus tard selon intended use et risque ;
> le processus peut fonctionner manuellement avant cet outil.

| Item régulé | État / lane | Effet exact |
|---|---|---|
| F1-F3 — owners, cycle QMS, `claim_coverage` | Records et claims gérés par plan 28 ; la mécanique `claim_coverage` produit reste livrée | Bloque uniquement les claims QMS correspondants |
| F4 / #560 — répétabilité privée | `lane:regulated`, `dispatch:passive`; mesure 4/8, collecteur livré | Bloque « repeated CI evidence », jamais le dispatcher ni #561 |
| F5 / #561 — release réelle | `lane:regulated`, `dispatch:human`; préparation autonome déplacée en #639 | #639 peut produire/rejouer un candidat avec risques ouverts ; seuls approbation/tag/publication authentiques ferment #561 |
| F6 / #562 — compétence | `lane:regulated`, `dispatch:human`; status tooling livré, correctif template en #640 | Les vraies attestations/waivers restent humaines ; leur absence ne bloque aucun développement |
| Références #192-196 | `lane:regulated`, humain/externe ; tooling #641 et public #644 autonomes | L'acquisition bloque seulement l'usage/claim clause-level de la référence concernée |

Les états régulés restent visibles et fail-closed pour leurs claims. Ils ne sont
ni supprimés ni transformés en pass : ils cessent seulement d'être des
dépendances techniques qu'ils n'ont jamais été.

---

## 8. Chantier G — Hygiène des claims et cohérence documentaire

> Ancre : règle d'evidence du claim boundary (« tout claim mappe vers gate / artefact /
> document / décision / gap »). Écarts E1, E4, E7.

| # | Action | Preuve exigée |
|---|---|---|
| G1 | **Réconcilier doc 40 avec le claim boundary** : « éprouvé et intégré dans plusieurs environnements » → soit produire la liste des environnements avec records, soit downgrader la phrase. Marquer les sections forward-looking des docs 39-42. | Le guard claim-boundary (#542) étendu aux docs stratégiques passe vert |
| G2 | **Hygiène d'index** : corriger les collisions de numérotation (`21-` ×2, `43-` ×2, `44-` ×2 — au minimum les noter comme le fait déjà l'index pour 21), référencer 39-45 dans `docs/README.md`. | Index complet, liens valides |
| G3 | **Matrice de câblage générée** (l'institutionnalisation de l'audit) : pour chaque capability — moteur Go ? test adversarial ? **caller de production ?** gate CI ? artefact signé ? claim level ? — générée par script en CI, jamais éditée à la main (règle du dossier RBOK : « editing the Actual column without evidence is forbidden »), publiée dans le self-compliance report. | La matrice attrape la classe « implémenté mais non enregistré » : un symbole exporté sans caller hors tests apparaît PARTIAL automatiquement |
| G4 | **Barre d'acceptation étendue** (#518) : toute issue « capacité » déclare à l'avance son **caller de production attendu**, pas seulement ses tests. | Template d'issue mis à jour ; revue bloque sans cette ligne |

---

## 9. Chantier H — Séquencement par effet de levier et fenêtres datées

> Ancre : doc 42 §E (« séquencement par effet de levier, pas par facilité ») ;
> doc 41 §5-6 (fenêtres : AEC suisse inoccupé ; EU AI Act août 2026 ; moats 12-24 mois).

| Phase | Contenu | Exit gate (adversarial) | Claims débloqués |
|---|---|---|---|
| **P0 — Hygiène & déblocage** (S0-S2) | G1-G4 ; **décision ADR control-plane : câbler au reporting portfolio ou archiver** | Guard étendu vert ; matrice G3 publiée ; aucune commande fantôme | Cohérence interne totale doc/code |
| **P1 — Le drapeau réel** (S2-S8) | A1, A2, A3 livrés dans Go ; B2 éval livrée | Les 3 mécaniques passent le tamis complet ; 0 PARTIAL sur P1/P2/P7 dans la matrice G3 | « Supply-chain du savoir : mesurée et traçable » ; aucun claim QMS induit |
| **P2 — Fenêtre AI Act** (livraison ≤ août 2026) | D1, D2, D3 ; A5 (BOM) | Pack AI Act passe `pack validate` ; claim boundary pilote relu par le guard ; D6 mesuré | « Evidence pack EU AI Act pilot-grade » — 1ᵉʳ vertical de la généralité, jamais conformité |
| **P3 — Ingestion & AEC** (T3-T4 2026) | C1, C2, C4 ; D5 ; B1 ; E-1, E-2 | Golden corpus AEC vert bout-en-bout ; CI Aedifica consomme le bundle flag-gated ; preuve distracteurs mesurée | « 1ᵉʳ vertical AEC prouvé » ; « un consommateur réel » |
| **P4 — Consolidation** (v0.3+) | A4 vérification offline (#637) ; C3 ; B3/B5 ; bench publication-ready/reproductible ; 3ᵉ pack (D6 re-mesuré) | Artefact de bench prêt à publier ; Sigstore fourni vérifié sans write public ; 3ᵉ pack à 0 changement core | Trajectoire v1.0 produit ; writes publics du bench/Rekor restent des activations externes |

Chaque phase : CKM-00 vert en continu ; tout changement de contrat = `schema_version`
bump + migration (doctrine §2.1) ; aucun claim avancé avant son niveau. Les
sorties de la roadmap régulée ne figurent plus dans ces phases produit.

---

## 10. Anti-objectifs (ce que ce plan interdit)

1. Ne jamais vendre « plateforme de compliance réglementaire » — commodity à 20 Md$,
   perdu d'avance contre CUBE/Archer (doc 41 §6).
2. Ne jamais pitcher « couche horizontale » générique — claim le plus banalisé du
   secteur ; mener avec la synthèse défendable.
3. Ne pas construire de moteur de règles — adopter L4/OpenFisca/Catala (B3).
4. Jamais de texte intégral de référence payante (SIA/ISO/GAMP 5) — hash + crosswalk.
5. Le LLM ne devient jamais l'autorité — la chaîne d'autorité (doc 01) est invariante.
6. Pas d'extension de packs plus vite que la fidélité portable (risque doc 14 :
   « market expansion before fidelity closure ») — D5 attend C1/C2.
7. Aucun résultat de pipeline déclaré — uniquement calculé (doctrine §2.3).
8. Aucun nouveau claim « signé / certifié / validé » qui ne passe pas le guard.
9. Aucune spécificité de domaine dans le core ; aucune mécanique dans un pack
   (garde-fou doc 40 §13) — vérifié par D1/D6.

---

## 11. Définition de « vision atteinte » (mesurable, calculée)

La vision **produit** des ADR est considérée atteinte quand toutes ces lignes
sont vertes dans la matrice G3 (générée, jamais déclarée) :

1. **Mécaniques** : 100 % des mécaniques CKM core consommées par le moteur Go avec
   caller de production + test adversarial + gate — zéro état PARTIAL.
2. **Reproductibilité** : les critères de succès du doc 01 satisfaits sur ≥ 2
   verticaux indépendants (AI Act + AEC) via le même core, avec D6 = 0 changement
   core par pack.
3. **Corpus** : Markdown, YAML, JSON, HTML, PDF né-numérique gouvernés avec claim
   ladder par format ; tout le reste = unsupported **explicite**, jamais silencieux.
4. **Runtime** : cite-or-abstain bloquant en CI consommateur, tiers exposés par
   réponse, bench public reproductible publié.
5. **Claims produit** : chaque claim public mappé selon la règle d'evidence, vérifié par le
   guard étendu — y compris dans les documents stratégiques.
6. **Consommation** : au moins un consommateur externe réel (Aedifica) consomme un
   bundle facetté via le kit de conformité, en production flag-gated.

La vision régulée (cycle QMS, formation, release via SOP, répétabilité,
acquisition et validation par intended use) est mesurée séparément par le plan
28. Son état ne modifie pas le statut des mécaniques produit et réciproquement.

---

## 12. Gouvernance du plan

- **Epic** : ouvrir un epic « [VRC] Vision-Reality Closure » regroupant les chantiers
  A-H, parent des issues atomiques (pattern de l'epic #481/#518).
- **Template d'issue** : chaque issue porte 5 lignes obligatoires — *ancre vision*
  (ADR/doc §), *écart* (E1-E7), *tremplin* (source + régime de licence), *preuve
  exigée* (tamis §2.3), *claim débloqué* (niveau du claim ladder).
- **Vérité** : la matrice G3 fait foi de l'avancement — pas les checklists d'issues.
- **Dispatch** : `docs/roadmap-lanes.yaml` fait foi de la prochaine tâche. Une
  dépendance dure ne peut viser qu'un item autonome de la même lane ; les
  autres relations sont des inputs non bloquants ou des claim gates.
- **Rythme** : compatible avec le mode rafale agents constaté, **à condition** que
  chaque rafale soit suivie de sa passe d'audit *avant* tout claim (leçon des
  2 passes de l'epic CKM) ; idéalement, G3/G4 rendent la 2ᵉ passe inutile en
  attrapant le non-câblé dès la PR.

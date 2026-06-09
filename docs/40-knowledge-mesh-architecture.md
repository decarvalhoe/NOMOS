<!-- Porté depuis l'analyse stratégique Aedifica — origine du pivot CKM (epic #481).
Mapping de ce dossier : 39 = plan maître · 40 = architecture méta (faceting/lens/promotion)
· 41 = positionnement état de l'art · 42 = capitalisation & amélioration.
Les références internes "nomos-*.md" pointent vers ces mêmes docs (noms Aedifica d'origine). -->

# NOMOS — du moteur réglementaire au *Canonical Knowledge Mesh* métier

> Analyse fondatrice v2 — élevée au niveau méta (réglementaire **+ métier**, scopée
> par rôle/activité/pertinence, capitalisable par l'utilisateur).
> Statut : analyse / design (aucun code engagé). Date : 2026-06-08.
> Repos : `RBOKproject/NOMOS` (moteur, réel et intégré), `decarvalhoe/aedifica`
> (1ʳᵉ preuve verticale).
> Remplace `nomos-built-environment-and-living-sources.md` (v1, trop ancrée archi).

---

## 0. Le saut conceptuel

La v1 visait trop bas : elle traitait NOMOS comme un moteur **réglementaire** et
built-environment comme le produit. La perspective corrigée :

> **NOMOS doit devenir le maillage canonique du *savoir* — réglementaire ET métier —
> de n'importe quel corps de métier, segmenté de façon multidimensionnelle, activable
> et exclusif par le spécialiste, et alimentable par l'utilisateur lui-même.**
> built-environment n'est qu'un **pack mince** qui fournit les *valeurs* propres à
> l'architecture. Les *mécaniques* sont des standards NOMOS, méta et réutilisables.

Trois mécaniques nouvelles deviennent des **standards NOMOS core** (sections 4-6),
au-dessus de built-environment :

1. **Knowledge Faceting** — chaque atome porte des facettes *contrôlées et
   multidimensionnelles* (nature, discipline/rôle, activité, niveau, tier, provenance,
   confidentialité), pas un simple `domain` plat + `tags` libres.
2. **Knowledge Lens** — le spécialiste compose une *lentille* (inclusion/exclusion sur
   les facettes) qui définit le savoir **actif pour son cas**, protège l'accuracy du
   RAG et bannit le savoir parasite/concurrent/superflu.
3. **Canon Promotion** — n'importe quel utilisateur peut, **avec droits + validation**,
   élever une source (du manuel de référence favori au plan de détail qu'il a mesuré,
   confidentiel ou non) au rang **canonique**. *Bring your own authority*, gouverné.

Et un recadrage factuel : **NOMOS n'est « alpha » que sur le papier** (doc + release
management en retard). En réalité il est éprouvé et intégré dans plusieurs
environnements. Le design ci-dessous vise donc la **performance, la pertinence et la
scalabilité futures**, pas la simple compatibilité avec l'existant.

---

## 1. La démarche du spécialiste — réglementaire **et** métier

L'erreur v1 : ne modéliser que le savoir réglementaire. Le spécialiste mobilise **deux
natures de savoir**, indissociables :

- **Savoir réglementaire** — ce qui *contraint* (zones, gabarits, OPB, RDPPF, normes).
- **Savoir métier** — ce qui *guide la pratique* : bonnes pratiques de conception,
  méthodes, détails constructifs, doctrines d'atelier, manuels de référence, retours
  d'expérience. Les « bibles » de l'archi (et, généralisable, les bibles de tout
  ingénieur / corps de métier).

Et surtout : **le savoir pertinent dépend du rôle et de l'activité, pas seulement de la
phase.** L'exemple polaire de la séance :

| Spécialiste / rôle | Activité en cours | Veut | Ne veut PAS (parasite) |
|---|---|---|---|
| Architecte — conception | Conçoit le projet | Bonnes pratiques de conception, références archi, gabarits | Procédures de **direction de travaux** |
| Directeur de travaux (DT) | Dirige un chantier d'un projet déjà conçu | Méthodes d'exécution, coordination chantier, contrôles | Bonnes pratiques de **conception** archi |

> Cette opposition est *illustrative*. Le vrai besoin est une **granularisation bien
> plus fine** : un même individu change de rôle/activité au fil du projet, et le métier
> de l'archi (comme l'ingénierie en général) se subdivise en multiples sous-domaines.
> Le savoir doit être **adressable et activable** selon (nature × rôle × activité ×
> niveau × pertinence), pas figé sur une grille SIA.

**Conséquence de design :** la phase SIA n'est qu'**une** valeur de l'axe « activité »
du pack built-environment. Le modèle de segmentation doit être **méta** (section 4),
défini au niveau NOMOS, et instancié par chaque domaine.

---

## 2. Les classes de sources — élargies

Le hub n'est pas « sources officielles réglementaires ». Trois classes, toutes
capitalisables en canon :

1. **Sources officielles réglementaires** (canaux d'autorité publics) — Fedlex/ELI,
   swisstopo/geo.admin, RDPPF/ÖREB, OFS, géoportails cantonaux, PGA/PAZ communaux.
   *(cf. cartographie v1 §2, à finir — reste un livrable.)*
2. **Bibles métier** (références professionnelles) — manuels de conception, normes de
   l'art, doctrines, guides techniques, segmentés par **sous-domaine métier**.
   Certaines sont **payantes/licenciées** (SIA, ouvrages) → traitées comme NOMOS le
   fait déjà : `access_policy: no_full_text`, hash + crosswalk, **jamais de texte
   intégral committé**.
3. **Canon promu par l'utilisateur** (bring-your-own-authority) — du plus abstrait
   (le manuel de référence favori du praticien) au plus granulaire (un plan de détail
   qu'il a relevé lui-même sur un bâtiment), **confidentiel ou non**. Élevé au canon
   **sur demande, avec droits, sous validation** (section 6).

> Vérité métier généralisable (point soulevé par Étienne) : **le savoir canonique ne
> vient pas que des canaux officiels.** Le praticien est lui-même une autorité sur son
> projet. Aedifica le pressent déjà (`Document.validation_level: pending → canonical →
> indicative → refused` + `confidential`) — mais c'est une **mécanique généralisable**
> à tout domaine, à remonter dans NOMOS core, pas une spécificité Aedifica.

---

## 3. Pourquoi méta — et pas une 10ᵉ branche réglementaire

La v1 proposait built-environment comme « domaine #10 » à côté de gxp, legal,
finance… C'est juste mais insuffisant : ça **rate la capitalisation holistique**. Le
même besoin — *segmenter le savoir réglementaire + métier par rôle/activité/pertinence,
et laisser l'user enrichir le canon* — vaut pour **tous les corps de métier** :
ingénierie, santé, finance, juridique, industrie. Donc on **généralise NOMOS**, et on
ne laisse à built-environment **que** les spécificités uniques de l'archi (les *valeurs*
des axes, les connecteurs de sources suisses, la taxonomie AEC). Tout le reste — les
*mécaniques* — devient standard NOMOS, réutilisable par le prochain vertical.

---

## 4. Standard NOMOS #1 — Knowledge Faceting (modèle de facettes méta)

### 4.1 Ce que NOMOS porte aujourd'hui (substrat réel)

D'après `specs/atomization-spine.cue`, l'`#Atom` actuel porte : un **`domain` unique
(string plat)**, `kind` (rule/clause/obligation/permission/prohibition/formula/…),
`review_state`, `priority`, `criticality`, des **`tags` libres**, un `metadata` ouvert,
et des relations (`depends_on`, `supersedes`, `ref_ids`). `#Reference` modélise déjà
`contradicts`. `#Certificate` existe.

**Le manque, précis :** un seul `domain` plat + `tags` non contrôlés **ne permettent
pas un scoping de récupération fiable** (multi-rôle, multi-niveau, anti-parasite). Il
faut des **facettes contrôlées et orthogonales**.

### 4.2 La proposition — un bloc `facets` contrôlé sur l'atome

Axes **NOMOS core** (chaque domaine déclare le *vocabulaire contrôlé* des axes qui lui
sont propres) :

| Axe | Rôle | Core / Domaine | Valeurs (built-environment) |
|---|---|---|---|
| `nature` | type de savoir | **core** | `regulatory` · `metier` · `project` · `reference` |
| `discipline` | rôle / pour qui | **vocab par domaine** | `architect.design` · `architect.permit` · `site-supervision` (DT) · `eng.structure` · `eng.mep` · `client` |
| `activity` | activité / cycle de vie | **vocab par domaine** | phases SIA (11→61) **et** activités hors-phase |
| `scope_level` | portée territoriale | **core** | `generalist` · `national` · `regional` · `local` · `site` |
| `trust_tier` | certification | **core** | `certified` · `indicative` · `unverified` |
| `provenance` | origine du canon | **core** | `official` · `metier_bible` · `user_promoted` |
| `confidentiality` | droits d'accès | **core** | `public` · `org` · `project` · `confidential` |
| `applicability` | s'applique-t-il au cas ? | **core** | `applies` · `not_applicable` · `unknown` (fait objectif) |

Principe : les **mécaniques** (la liste d'axes, leur sémantique, leur effet sur le
retrieval) sont **core** ; un **domain pack** ne fait que **peupler les vocabulaires**
des axes `discipline`/`activity` et fournir les connecteurs de sources. C'est
exactement « ne laisser à built-environment que les spécificités uniques à l'archi ».

> Note d'implémentation : c'est additif et non-cassant — `facets` peut d'abord vivre
> dans le `metadata` ouvert de l'`#Atom`, puis être promu en bloc CUE contrôlé
> (`#Facets`) avec validation `cue vet`. Idem côté chunk (le `#Chunk.metadata` porte
> les facettes pour le filtrage au niveau base).

---

## 5. Standard NOMOS #2 — Knowledge Lens (sélection/exclusion, anti-parasite)

### 5.1 Le problème : la pertinence n'est pas binaire ni purement objective

Distinguer **deux notions** que la v1 confondait :

- **Applicabilité objective** — *fait* : ce règlement couvre-t-il cette parcelle ?
  (axe `applicability`).
- **Activation subjective** — *choix du spécialiste* : est-ce que je **veux** ce savoir
  dans mon contexte de travail ? Un DT peut exclure la conception ; un archi peut
  exclure un règlement du quartier d'à côté qui *semble* pertinent mais ne s'applique
  pas — et qui **parasiterait l'accuracy du RAG**.

Les deux doivent coexister. Le spécialiste doit pouvoir **exclure même du savoir
plausiblement pertinent**.

### 5.2 La mécanique : une *lentille* composable

Une **Lens** = un prédicat d'inclusion/exclusion sur les facettes, attaché à
(utilisateur × projet × moment) :

```
Lens "Archi en conception, projet X" =
  include: nature ∈ {metier, regulatory, reference}
           discipline ∈ {architect.design, architect.permit}
           scope_level ⊇ {national, regional, local, site}
           applicability ∈ {applies, unknown}
           trust_tier ⊇ {certified}            # certifié seulement, ou +indicative
  exclude: discipline ∋ site-supervision        # pas de DT
           tags ∋ "quartier:voisin"             # anti-parasite explicite
```

Effets :
- **Filtrage au niveau base, avant génération** (leçon multi-tenant : jamais
  post-LLM). La Lens devient le `WHERE` du retrieval.
- **Accuracy RAG protégée** : le corpus actif est réduit au pertinent → moins de bruit,
  moins de hallucination, citations plus justes.
- **Spécificité gagne** : sur égalité, `site > local > regional > national > generalist`.
- **Réversible et switchable** : changer de rôle = changer de Lens, sans toucher au
  canon.

C'est le « ça m'intéresse dans mon cas ou pas » demandé, **généralisé** et
**capitalisable** (tout domaine définit ses Lens-presets par rôle/activité).

---

## 6. Standard NOMOS #3 — Canon Promotion (l'user comme source d'autorité)

### 6.1 Le substrat existe déjà, dispersé

NOMOS a : `review_state` (draft→pending_review→approved), `authority_type:
customer_source/private_source`, `access_policy: customer_confidential`, `#Certificate`.
Aedifica a : `Document.validation_level` (pending→**canonical**→indicative→refused) +
`validated_by` + `confidential`. → Les pièces sont là, **non assemblées en mécanique
explicite**.

### 6.2 La mécanique : promotion gouvernée

Workflow **core**, réutilisable :

```
soumission user (doc officiel OU perso/confidentiel, abstrait→granulaire)
  → facettage (nature, discipline, activity, scope_level, confidentiality,
               provenance=user_promoted)
  → droits vérifiés (qui peut promouvoir, sur quel périmètre)
  → validation (le spécialiste/owner valide : pending_review → approved)
  → atome canonique + #Certificate (issuer, scope, valid_until)
  → entre dans le corpus, filtrable par Lens, citable par le RAG
  → révocable / versionné (review_state archived, cert revoked)
```

Garanties : **droits + validation obligatoires** (l'user *demande*, l'owner *valide*) ;
**confidentialité respectée** (un plan de détail confidentiel reste en silo projet,
LPD) ; **provenance tracée** (`user_promoted` ≠ `official`, jamais confondus dans une
citation) ; **trust_tier honnête** (un canon promu par l'user n'est pas `certified` au
sens officiel — il est `certified` *au sens de l'atelier* ou `indicative`, explicitement).

C'est la « vérité métier généralisable » : **n'importe quel domaine** peut laisser ses
praticiens capitaliser leur savoir propre, sous gouvernance, dans le même moteur.

---

## 7. built-environment — le pack mince

Ce que le pack archi fournit, **et rien de plus** :

- Les **vocabulaires** des axes `discipline` (architect.design, DT, BET structure,
  CVSE, client…) et `activity` (phases SIA + activités hors-phase).
- Les **connecteurs de sources** suisses (Fedlex/ELI, swisstopo/STAC, RDPPF/ÖREB, OFS ;
  pipeline PDF pour PGA/PAZ ; sidecar hash-only pour SIA).
- Le **source authority register** archi (la cartographie §2 finie).
- Les **Lens-presets** métier (« archi-conception », « DT-chantier », « permis »…).
- Les **golden corpus** archi (VD + Lausanne pour démarrer).

Tout le reste — faceting, lens, promotion, trust tiers, contrat RAG, gates — est
**NOMOS core**, et resservira au vertical suivant (santé, indus, etc.).

---

## 8. NOMOS aujourd'hui — réel, pas alpha ; substrat vs manque

**Recadrage maturité :** NOMOS est éprouvé et intégré (multi-environnements,
plusieurs projets). Son étiquette `v0.1.0-ALPHA` reflète un **retard de doc/release
management**, pas l'état réel du moteur. On conçoit donc pour la **prod, la perf et la
scalabilité**, pas pour un POC.

| Brique méta | Substrat NOMOS existant | Manque à combler (core) |
|---|---|---|
| Faceting | `#Atom.domain` (plat), `tags`, `metadata`, `kind` | bloc `facets` contrôlé multi-axes + filtrage |
| Lens | `applicability` (au niveau *profil* seulement) | Lens per-(user×projet) au niveau *atome/retrieval* |
| Promotion | `review_state`, `customer_source`, `#Certificate` ; (Aedifica `validation_level`) | workflow assemblé droits+validation+provenance |
| Nature métier | `kind` (réglementaire-centré) | `nature ∈ {regulatory, metier, project, reference}` |
| Anti-parasite | `#Reference.contradicts` | exclusion explicite dans la Lens + scoping base |

---

## 9. Aedifica — 1ʳᵉ preuve verticale (perf/scale/futur, pas les lauriers)

Aedifica n'est pas « le consommateur dont les tables existent déjà donc c'est réglé ».
C'est la **première preuve verticale** que la généralisation métier+réglementaire
tient — et il faut la concevoir pour **demain** :

- **Ne pas se reposer sur les tables existantes.** `CommunePack`/`Claim`/`Source`/
  `Evidence` ont prouvé le *modèle*, mais le wiring doit viser **performance**
  (retrieval scopé par Lens au niveau base : pgvector + RLS, reranker, cache),
  **pertinence** (anti-parasite par défaut), **scalabilité** (multi-atelier,
  multi-projet, corpus partagé *pool* + silos projet).
- **Ne pas copier RBOK aveuglément.** Le patron RBOK (projection read-only) reste une
  *référence*, mais on **repense** le contrat de bundle pour porter les **facettes** et
  les **Lens-presets**, pas seulement le feed plat. RBOK a dû écrire un importeur sur un
  format ancien ; on évite cette dette en figeant un **contrat de bundle facetté** dès
  le départ.
- **Aedifica nourrit NOMOS en retour.** Sa mécanique `validation_level` (canon promu) et
  ses 6 trust states sont des **apports** à remonter dans NOMOS core, pas juste une
  réception passive.

---

## 10. Verdict de pertinence (réactions intégrées)

- **Doctrine identique** — confirmé, socle du rapprochement.
- **Tables de réception** — *ne pas dormir sur l'existant* : concevoir pour perf /
  pertinence / scalabilité / futur (cf. §9), l'existant n'est qu'une preuve de modèle.
- **Patron RBOK** — *ne pas s'y limiter* : repenser le contrat de bundle (facetté) avec
  le contexte actuel.
- **Raison d'être** — oui : l'assistant sourcé/certifié, scopé par rôle et pertinence,
  alimenté par l'user, *est* la raison d'être de NOMOS appliquée à un vertical.

**Verdict : GO.** Non plus « conditionnel parce qu'alpha » (NOMOS est réel), mais
**séquencé** : core méta d'abord, pack archi mince ensuite, wiring Aedifica perf/scale
en parallèle.

---

## 11. État de l'art réutilisable (rappel + ajout métier)

Inchangé sur le réglementaire (Akoma Ntoso, ELI/ECLI, LegalRuleML, FRBR, W3C PROV,
OpenFisca/Catala ; références legislation.gov.uk, Free Law, BifrostRAG, Self-RAG/
Trust-Align/ALCE/CRAG ; stack pgvector/Qdrant/LlamaIndex/Ollama ; éval RAGAS/TruLens).
**Ajout métier :** pour le savoir non-réglementaire, les standards de *knowledge
organization* (SKOS pour les vocabulaires contrôlés des facettes `discipline`/
`activity`, taxonomies métier) et les ontologies de domaine (p. ex. IFC/buildingSMART
côté AEC) deviennent les sources des **vocabulaires d'axes**. Alerte nommage NOMOS
(legaltech encombré) : inchangée.

---

## 12. Plan — core méta d'abord, vertical ensuite

> Inversion vs v1 : on **généralise NOMOS d'abord** (la demande), built-environment
> n'arrive qu'après comme pack mince.

**Phase 0 — NOMOS core méta (le capitalisable, tout domaine)**
1. `#Facets` contrôlé sur atome + chunk (additif via `metadata`, puis CUE).
2. Modèle **Lens** (inclusion/exclusion sur facettes) + scoping retrieval au niveau base.
3. Mécanique **Canon Promotion** (droits + validation + provenance + certificat).
4. `nature` métier + Lens-presets génériques. Golden corpus *cross-métier* (un cas
   métier non-archi pour prouver la généralité).

**Phase 1 — pack built-environment (mince)**
Vocabulaires discipline/activity AEC + connecteurs sources CH + source register +
Lens-presets archi + golden corpus VD/Lausanne. *(= ex-Phase B, allégée.)*

**Phase 2 — wiring Aedifica (perf/scale)**
Contrat de bundle **facetté** → adapter d'import → retrieval scopé par Lens (pgvector
+ RLS + reranker) derrière flag ; OFS direct (W17/W18) reste le défaut sûr ; remontée
de `validation_level`/trust-states vers NOMOS core.

**Phase 3 — assistant sourcé scopé**
Copilote sur retriever doctrine + Lens active + rules-as-code pour le calculable +
abstention ; éval RAGAS/ALCE en gate.

---

## 13. Garde-fous (élargis)

Tous ceux de la v1 (source datée+hash ; citation ou abstention ; no-full-text SIA ;
LPD/silo projet ; découplage de code) **plus** :

- **Anti-parasite par défaut** : la Lens exclut le hors-périmètre ; un savoir
  plausiblement pertinent mais non applicable/non activé **n'entre pas** dans le
  contexte RAG.
- **Promotion gouvernée** : pas de canon user sans **droits + validation** ; provenance
  `user_promoted` jamais présentée comme `official` ; confidentiel reste en silo.
- **Honnêteté du tier** : un canon métier ou promu n'usurpe pas le label `certified`
  officiel — le tier est explicite dans toute citation.
- **Méta avant vertical** : aucune spécificité archi ne doit remonter dans le core ;
  inversement, aucune mécanique core ne doit être dupliquée dans le pack.

---

## 14. Décisions ouvertes (pour toi)

1. **Validation du saut méta** : on acte NOMOS = *Canonical Knowledge Mesh* (réglementaire
   + métier + canon user), built-environment = pack mince ? (C'est le pivot de toute la
   suite.)
2. **Golden corpus cross-métier** de la Phase 0 : quel 2ᵉ vertical minimal pour *prouver*
   la généralité (un cas santé ? finance ? un simple « atelier knowledge » interne) ?
3. **Vocabulaires d'axes** : qui possède la taxonomie `discipline`/`activity` archi
   (toi / Étienne) — on la dérive de SIA + pratique, ou d'un référentiel existant ?
4. **Où vit cette analyse** : la garder côté Aedifica, ou la porter dans NOMOS
   (`docs/39-knowledge-mesh-faceting-lens-promotion.md`) comme proposition core soumise
   à leur gouvernance ?
5. **Création des issues** (Phase 0 NOMOS core) : maintenant, ou après ta relecture ?
```

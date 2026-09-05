# 05 - Knowledge Base, Vector Store Et RAG

## Rôle Exact

La base vectorielle contient le corpus pour retrouver et citer le contexte. Elle ne remplace pas les contrats canoniques. Elle ne doit jamais être la seule source d'un calcul déterministe.

Le RAG est utile pour :

- expliquer une règle à un humain ;
- citer les sources exactes ;
- retrouver une ambiguïté ;
- aider un agent à proposer une modification ;
- répondre à des questions de contexte ;
- comparer une implémentation à un passage source.

Le RAG est dangereux s'il :

- calcule directement une décision critique ;
- mélange des sources sans priorité ;
- omet les dates d'application ;
- cite un chunk sans source hash ;
- écrase une donnée structurée.

## Ingestion

L'ingestion doit partir du manifest et des artefacts d'atomisation, jamais d'un scan libre non contrôlé.

Pipeline standard :

```text
source-manifest.yaml
  -> structure tree
  -> atomic units
  -> canonical matrix
  -> chunk projection
  -> attach metadata
  -> embed
  -> store
  -> audit coverage
```

La base vectorielle ne découpe pas le corpus à sa place. Elle consomme une projection issue de la structure documentaire et des unités atomiques. Un chunk qui ne peut pas être relié à une source, une structure, et si applicable une unité/matrice, est un artefact non conforme.

## Metadata Obligatoire

Chaque chunk doit porter au minimum :

| Champ | Rôle |
|---|---|
| `chunk_id` | ID stable ou reproductible. |
| `source_id` | Source manifest. |
| `source_path` | Chemin/URL. |
| `source_hash` | Hash au moment de l'indexation. |
| `domain` | Domaine métier. |
| `unit_ids` | Unités atomiques liées si connues. |
| `matrix_refs` | Lignes de matrice liées si l'unité est gouvernée. |
| `structural_refs` | Chemin documentaire parent : document, section, article, paragraphe, alinéa, table. |
| `locator` | Page, ligne, section, sélecteur, fonction legacy. |
| `chunk_type` | `atom`, `structure_context`, `table`, `example`, `governance`, `retrieval_context`. |
| `chunking_strategy` | Profil et version du découpage. |
| `priority` | Priorité source. |
| `status` | Statut source. |
| `effective_from` | Date d'application si pertinente. |
| `license` | Usage autorisé. |
| `confidentiality` | Niveau de protection. |
| `ingested_at` | Timestamp. |
| `ingestion_version` | Version du pipeline. |

## Chunking

Le chunking doit respecter le sens métier et la structure documentaire.

Mauvaises stratégies :

- découpage tous les 1000 caractères sans tenir compte des sections ;
- mélange de plusieurs règles dans un chunk sans metadata fine ;
- suppression des titres, numéros de clauses ou notes ;
- OCR sans score de confiance.

Bonnes stratégies :

- chunk par section, clause, règle, tableau ou entrée ;
- overlap limité quand le contexte est nécessaire ;
- conservation des titres hiérarchiques ;
- extraction des tableaux en structure lisible ;
- score qualité pour OCR et parsing.

Règles Canonical-First :

- un chunk d'autorité doit correspondre à un atome ou à une structure explicitement référencée ;
- un chunk de contexte peut regrouper plusieurs atomes mais doit lister tous les `unit_ids` et `canonical_refs` ;
- un chunk ne doit pas fusionner des sources de priorité différente sans signaler le conflit ou la raison ;
- un changement de source, structure, atome ou matrice invalide les chunks liés ;
- une réponse RAG doit refuser de conclure quand le chunk retrouvé pointe vers un atome `missing`, `blocked`, `deprecated` ou `needs_review` au-delà du niveau de risque autorisé.

## Export Vers Une Stack RAG

NOMOS n'est pas un moteur RAG : il n'embed pas, ne retrouve pas, ne reclasse
pas. Il remet un corpus prouvable à n'importe quelle stack (LangChain,
LlamaIndex, index maison) et vérifie ce qui en revient via le gate
cite-or-abstain (`nomos answer`). La couture est `nomos rag` :

```bash
nomos rag export   --feed feed.json --format jsonl|langchain|llamaindex --strict
nomos rag export   --bundle bundle.json --lens permis.lens.yaml --document-facets retrieval-harness.yaml
nomos rag manifest --feed feed.json --strict
nomos rag delta    --old manifest-indexe.json --new manifest-courant.json
nomos rag verify   --manifest manifest-indexe.json --feed feed.json
```

Contrat d'export (`nomos-rag-chunk-v1`) :

- `embedding_text` : ce que le consommateur embed **et** indexe en lexical
  (BM25) — préfixe de contexte structurel + corps. Le préfixe est dérivé de la
  structure déjà parsée (source, rôle, domaine, chemin de titres, table et
  ligne, colonnes, chemin YAML), sans modèle dans la boucle : reproductible et
  vérifiable. Sa grammaire est versionnée (`context_prefix_version`) ; tout
  changement de grammaire impose un ré-embedding.
- `body_text` : ce que le consommateur affiche et cite. Le préfixe de contexte
  n'y entre jamais : une citation ne doit contenir que du texte présent dans
  la source.
- `provenance` : `source_id`, `source_hash`, spans (octets, lignes), segments
  source, unité canonique — le minimum pour re-prouver un chunk retrouvé.
- `metadata` : enveloppe filtrable, mêmes noms de champs que le vocabulaire
  corpus.

Règles fail-closed : un chunk sans `chunk_id`, sans `source_id`, sans
`source_hash` ou sans corps est refusé et compté, jamais tu. `--strict` rend
le refus bloquant. L'export est trié par `chunk_id` et ne lit aucune horloge :
deux exports du même feed sont identiques octet pour octet.

`nomos rag manifest` (`nomos-rag-index-manifest-v1`) fixe l'empreinte de ce
qui a été remis à l'index : digest global et digest par source, lié au
`source_hash`. Une source dont le hash bouge invalide exactement ses chunks,
pas l'index entier — la règle « un changement de source invalide les chunks
liés » devient vérifiable côté consommateur. Le manifeste porte aussi
l'empreinte de chaque chunk (`embedding_hash`, `body_hash`, `source_hash`), et
son digest est recalculable depuis cette seule liste.

`nomos rag delta --old A --new B` calcule le plan de réindexation exact entre
deux manifestes (`nomos-rag-index-delta-v1`). Par chunk : `embed` (corps ou
contexte modifié, chunk ajouté), `update_metadata` (texte identique mais
`source_hash` déplacé : rafraîchir la provenance stockée pour que les citations
se re-prouvent) ou `delete` (chunk disparu) ; par source : `unchanged`,
`changed`, `added`, `removed`. Un changement de schéma de record ou de
grammaire de contexte force une réindexation complète. `nomos rag verify
--manifest A --feed feed.json` rejoue ce calcul contre le corpus tel qu'il est
maintenant et sort en code 1 dès que l'index est périmé : c'est le gate qu'un
consommateur exécute avant de faire confiance à son index. Un manifeste
retouché à la main (digest différent de sa propre liste de chunks) ou sans
empreintes par chunk ne peut pas se porter garant d'un index : réindexation
complète, jamais « frais » par défaut.

### Scope De Retrieval : La Lens Au Niveau Du Corpus Remis

Le feed corpus ne porte pas de facettes ; le bundle CKM (`nomos bundle`) si :
ses nœuds portent les axes fermés dérivés par le moteur (`nature`,
`scope_level`, `trust_tier`, `provenance`). Avec `--bundle`, les records
portent `metadata.facets` (aplaties en `facet_<axe>` pour LangChain et
LlamaIndex) ; `--document-facets` attache les axes ouverts du pack par
document source (`activity`, `confidentiality`, `applicability`), avec la
même sémantique que le kit consommateur : l'axe du pack complète ou écrase
celui du nœud.

`--lens <lens.yaml>` applique une Knowledge Lens à l'export lui-même : un
chunk exclu n'est jamais remis à l'index, donc aucun filtre côté consommateur
ne peut le laisser fuir. Les exclusions sont nommées et comptées, jamais
tues ; un chunk sans facettes sous une Lens est exclu (son appartenance ne
peut pas être prouvée) ; `--strict` refuse un export vidé par la Lens. Le
manifeste lie l'index à `lens.id` et au digest de la Lens ; une autre Lens,
ou l'abandon de la Lens, est une réindexation complète (`lens_changed`).

Le manifeste porte un bloc `retrieval_contract` **calculé** : le scope
(`lens` ou `unscoped`), les champs filtrables réellement présents avec leurs
valeurs observées (`priority`, `status`, `source_role`, `facets.*`…), et ce
qu'un consommateur ne doit pas déduire de ces records : `temporal_scoping`
est déclaré non supporté (aucun record ne porte de date d'application ; la
résolution point-in-time reste `nomos pointintime` sur les atomes). NOMOS ne
classe pas : il décide l'appartenance au scope, pas l'ordre.

Le gate CI `scripts/rag-export-gate.sh` rejoue ces propriétés sur le corpus de
référence public du dépôt : export et manifeste bit-identiques, zéro refus,
aucune fuite du préfixe dans le corps citable, une mutation d'un octet d'une
source déplace le digest de cette source seule, et `rag verify` rend frais
(code 0) sur re-scan, périmé (code 1) sur mutation avec un plan limité à la
source touchée, et refuse un manifeste falsifié. Sur le bundle réel du golden
corpus AEC, il exporte sous `LENS-AEC-PERMIS` avec les `document_facets` du
pack et re-dérive les verdicts avec une réimplémentation indépendante de la
sémantique du kit consommateur : ensemble exporté identique, document
confidentiel absent, contrat calculé, et `rag verify` périmé sous une autre
Lens ou sans Lens.

> Périmètre de revendication : le contrat d'export (déterminisme, liaison à la
> provenance, détection de staleness). Aucune qualité de retrieval n'est
> revendiquée ; l'effet du préfixe de contexte sur le retrieval reste à mesurer
> par le harnais d'évaluation.

## Recherche Et Citations

Une réponse assistée doit distinguer :

- données structurées lues depuis le read-model ;
- contexte textuel récupéré depuis le vector store ;
- inférence ou synthèse du LLM ;
- incertitude et ambiguïté.

Format recommandé pour une réponse LLM :

```json
{
  "answer": "...",
  "structured_facts": [
    {"unit_id": "RULE-0017", "source": "read_model"}
  ],
  "citations": [
    {"source_id": "RULEBOOK-2026", "locator": "p. 42", "chunk_id": "chunk-..."}
  ],
  "uncertainties": [],
  "requires_human_decision": false
}
```

## Évaluations

Un RAG Canonical-First doit être évalué sur :

- retrieval coverage : retrouve-t-il les bons chunks ?
- citation faithfulness : les citations supportent-elles la réponse ?
- answer groundedness : la réponse n'ajoute-t-elle pas de règle non sourcée ?
- temporal correctness : utilise-t-il la bonne version ?
- conflict handling : signale-t-il les sources contradictoires ?
- refusal correctness : refuse-t-il de décider sans source suffisante ?

> Ces dimensions sont des cibles d'évaluation de la méthode. À ce stade, NOMOS produit des métadonnées RAG traçables ; l'évaluation RAG en production (retrieval, comportement LLM) n'est pas validée — voir [public-claim-boundary.md](public-claim-boundary.md).

### Gate Cite-Or-Abstain CKM

Le gate CKM est calculé par le moteur Go (`nomos answer gate`) et consigné
comme preuve par `scripts/regulated_rag_answer_evidence.py`. Le sidecar Python
ne note plus rien lui-même : il consomme le verdict du moteur (VRC-10 A1,
#624). Il expose, par réponse et en synthèse :

- `metrics.alce.citation_recall` : part des chunks récupérés couverte par des citations source-backed ;
- `metrics.alce.citation_precision` : part des citations qui se lient aux chunks récupérés ;
- `metrics.deepeval.faithfulness` : score de support par les citations ou score de fixture explicite ;
- `metrics.trust_score` : moyenne déterministe recall/precision/faithfulness/confidence ;
- `trust_tier` : `certified`, `indicative` ou `unverified` ;
- `metrics.verdict_source` : `go_engine` (verdict du moteur) ou `python_fallback`.

Le tiers `certified` exige recall, precision, faithfulness et trust-score au-dessus
du seuil du gate. `indicative` signale une preuve exploitable mais insuffisante
pour une revendication certifiée. `unverified` est utilisé dès qu'un finding
bloquant existe.

Le rapport porte les seuils mêmes qui ont servi au verdict (`gates`, renvoyés
par le moteur) : il ne les duplique donc jamais, et un seuil qui change dans le
moteur apparaît dans le rapport. Ce que le moteur ne sait pas reste au sidecar :
l'enveloppe de preuve (champs obligatoires de l'enregistrement, contrat de
réponse, plage de confiance, unicité des `answer_id`).

Deux modes, un seul défaut :

- `--engine required` (défaut, celui de la CI) : sans moteur, ou si le moteur
  n'émet pas de verdict (crash, timeout, sortie non JSON, verdict non aligné
  sur les fixtures), le sidecar sort en 2 et n'écrit **aucun** rapport — un
  rapport périmé présent à la sortie est supprimé. Rien n'est noté localement ;
- `--engine fallback` : repli EXPLICITE sur le proxy lexical du sidecar
  (CKM-H6, aveugle à la négation) quand le moteur est indisponible. Le rapport
  le dit (`engine.verdict_source: python_fallback`, finding d'avertissement
  `RAG_GATE_VERDICT_FROM_PYTHON_FALLBACK`), et chaque tiers est plafonné à
  `indicative`. Aucun gate CI n'utilise ce mode (une sonde
  `must_be_absent` du registre de wiring l'interdit dans les scripts shell).

`--scorer-cmd` (et son seuil, son timeout) est transmis au moteur : un second
juge NLI atteint donc la preuve sans qu'aucun modèle ne vive dans le sidecar.
Un refus explicite n'affirme rien : son verdict nomme sa méthode de groundedness
(`explicit_refusal`) au lieu de la laisser vide.

Ce gate ne prouve pas la justesse métier finale d'une réponse LLM. Il prouve
seulement que la réponse suit le contrat cite-or-abstain, cite des spans
traçables, conserve les faits structurés et expose les incertitudes ou la décision
humaine requise. La décision réglementaire, juridique, clinique ou métier reste
hors revendication NOMOS.

### Harnais `nomos answer eval` : Métriques De Contexte

Le harnais CI (VRC-13) rejoue le gate cite-or-abstain sur le corpus doré
`docs/regulated/ai-rag-governance/rag-eval-corpus.yaml` contre les seuils
versionnés de `rag-eval-thresholds.yaml` (`citation_recall`,
`citation_precision`, `faithfulness`, réponses en échec). Chaque réponse du
corpus doré déclare en plus `expected_chunk_ids`, la vérité terrain des chunks
pertinents pour le prompt, ce qui permet trois métriques côté retrieval,
calculées et jamais déclarées :

- `context_recall` : part des chunks attendus effectivement retrouvés ;
- `context_precision` : précision pondérée par le rang sur l'ordre de
  `retrieved_chunks` (un distracteur classé avant le chunk pertinent la fait
  baisser, classé après non) ;
- `noise_sensitivity` : part des phrases de la réponse supportées
  **uniquement** par des chunks retrouvés hors attendus. La fidélité lexicale
  compte une telle phrase comme supportée (ses mots sont dans le corpus de
  support) ; cette métrique est celle qui attrape la contamination que la
  fidélité ne voit pas.

Les bornes de contexte (`min_mean_context_recall`,
`min_mean_context_precision`, `max_mean_noise_sensitivity`) sont optionnelles :
absentes, les métriques sont rapportées sans bloquer ; posées alors qu'aucune
réponse ne déclare d'attentes, le harnais échoue (fail-closed : une borne que
personne ne mesure est un faux confort). Limite : le proxy de support reste
lexical et aveugle à la négation.

### Scorer De Fidélité Enfichable (NLI)

Le proxy lexical du gate est aveugle à la négation par construction : « le
délai court » et « le délai ne court pas » partagent leurs mots pleins. Le gate
accepte un second juge, externe au moteur, sur les mêmes paires (texte de
support retrouvé/cité, phrase de la réponse) :

```bash
nomos answer gate --fixtures answers.yaml \
  --scorer-cmd "python3 scripts/nomos_hhem_scorer.py" --scorer-threshold 0.5
nomos answer eval --corpus ... --thresholds ... --scorer-cmd "..."
```

Contrat : `nomos-scorer-request-v1` sur stdin, `nomos-scorer-response-v1` sur
stdout, une probabilité de support dans [0,1] par paire, alignée par `id`.
Règles :

- **le plus strict gagne**, phrase par phrase : une phrase n'est supportée que
  si le proxy lexical ET le scorer la supportent. Brancher un scorer ne peut
  que durcir le verdict, jamais l'assouplir (même sens que la règle « un score
  auto-déclaré ne peut que baisser ») ;
- **fail-closed** : scorer qui échoue, dépasse le délai, répond hors schéma,
  oublie ou duplique une paire, ou renvoie un score hors [0,1] → la réponse
  score 0 et porte le finding `FAITHFULNESS_SCORER_FAILED`, quel que soit son
  `policy_outcome` ; pas de repli silencieux sur le lexical. Les refus
  n'affirment rien et ne sont pas scorés ;
- NOMOS n'embarque aucun modèle : le moteur reste déterministe, le juge
  neuronal vit dans un sidecar. `scripts/nomos_hhem_scorer.py` est
  l'adaptateur de référence pour HHEM-2.1-Open (Vectara, modèle ouvert,
  chargé à l'exécution) ; il refuse d'émettre des scores quand le backend est
  indisponible ou répond hors contrat.

Le verdict expose les deux juges (`lexical_supported_sentences`,
`scorer_supported_sentences`, `supported_sentences` final, `scorer_method`,
`scorer_threshold`). Limite de revendication : la CI n'exerce le sidecar qu'au
niveau du protocole (backend injecté, backend absent) ; aucun run CI ne score
avec le modèle neuronal, et NOMOS ne revendique rien sur la précision de HHEM.

### Bench Public Cite-Or-Abstain

Le bench public (VRC-46, #582) **mesure** le gate au lieu de le déclarer.
`docs/regulated/ai-rag-governance/cite-or-abstain-bench/` contient un corpus
étiqueté construit sur les documents publics du dépôt
(`docs/regulated/reference-basis/`), les seuils versionnés, les références
citées avec leur date de vérification, la méthodologie et les résultats datés.
Chaque item est une réponse déjà produite, avec ses spans et ses citations,
étiquetée `cite` ou `abstain` par construction et rangée dans la catégorie
qu'elle sonde (`grounded`, `forged_citation`, `no_span_text`, `negation`,
`over_verbosity`, `prompt_injection`, `unsupported_question`). Il n'y a ni
modèle, ni retrieval, ni réseau dans la boucle.

```bash
nomos answer bench --corpus docs/regulated/ai-rag-governance/cite-or-abstain-bench/corpus.yaml \
  --thresholds docs/regulated/ai-rag-governance/cite-or-abstain-bench/bench-thresholds.yaml
python3 scripts/cite_or_abstain_bench.py --root .           # rejoue et compare aux résultats publiés
python3 scripts/cite_or_abstain_bench.py --root . --publish # re-publie un résultat daté
```

La mesure est volontairement asymétrique, parce que les deux erreurs ne
coûtent pas la même chose :

- `must_abstain_recall` et `false_cite_rate` : parmi les items qui doivent
  être bloqués, combien le sont, et combien passent. Un faux « cite » est
  l'erreur dangereuse (une réponse non fondée publiée comme sourcée) ; il est
  publié tel quel, par catégorie, jamais noyé dans une exactitude globale ;
- `must_cite_recall` et `missed_cites` : parmi les réponses légitimement
  citables, combien sont citées. La sur-abstention est aussi un défaut (un
  gate qui bloque tout est parfait côté sûreté et inutile) ;
- `agreement` : décision observée = décision attendue, par item.

Un item sans `expected_decision` utilisable est un défaut du bench, jamais une
mesure ; une borne (`max_false_cite_rate`, `min_must_abstain_recall`,
`min_must_cite_recall`) ne peut être posée que sur un côté de la matrice de
confusion effectivement mesuré (fail-closed, même règle que le harnais).

Résultat publié du 2026-09-05 (proxy lexical, aucun scorer ; republication à l'identique après extension du document source cité, première publication 2026-09-04) : 9 items,
`must_cite_recall` 1.0 (3/3), `must_abstain_recall` 0.8333 (5/6),
`false_cite_rate` 0.1667. Le seul faux « cite » est la catégorie `negation`,
l'angle mort documenté du proxy lexical ; le test Go du bench prouve qu'un
second juge injecté la fait basculer, et le même corpus se rejoue avec
`--scorer-cmd` hors CI. La porte de reproduction
(`scripts/cite_or_abstain_bench.py`) vérifie que les sources citées n'ont pas
bougé (sha256 déclaré = réel, chaque span cité verbatim dans sa source), que
chaque référence citée porte une vérification datée, que le moteur est
déterministe (deux runs, mêmes octets), que les seuils tiennent et que la
mesure rejouée est identique à celle publiée : toute dérive est nommée et
rouge en CI.

Ce que le bench ne dit pas : rien sur la qualité d'un retrieval, d'un
embedding ou d'un LLM (aucun n'est dans la boucle), rien sur la justesse
métier des réponses, rien sur la précision d'un modèle NLI (aucun run CI n'en
exécute). Il mesure le gate sur neuf items publics, un par mode de défaillance :
c'est la preuve externe de la méthode, pas une évaluation de produit.

## Tests Associés

- Toutes les sources actives sont indexées.
- Tous les chunks ont metadata complète.
- Tous les chunks référencent un hash de source courant.
- Tous les chunks liés à une règle critique pointent vers une unité et une ligne de matrice.
- Aucun chunk d'autorité ne référence un atome bloqué ou sans review state.
- Un changement de source invalide les chunks correspondants.
- Les questions de golden set retrouvent les chunks attendus.
- Le LLM ne répond pas sans citation pour les domaines marqués critiques.
- Les données structurées priment sur le texte récupéré.

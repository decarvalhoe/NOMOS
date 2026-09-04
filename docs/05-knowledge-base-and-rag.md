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

Le gate CI `scripts/rag-export-gate.sh` rejoue ces propriétés sur le corpus de
référence public du dépôt : export et manifeste bit-identiques, zéro refus,
aucune fuite du préfixe dans le corps citable, une mutation d'un octet d'une
source déplace le digest de cette source seule, et `rag verify` rend frais
(code 0) sur re-scan, périmé (code 1) sur mutation avec un plan limité à la
source touchée, et refuse un manifeste falsifié.

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

Le gate CKM mesure les réponses RAG conservées comme preuves avec
`scripts/regulated_rag_answer_evidence.py`. Il expose, par réponse et en
synthèse :

- `metrics.alce.citation_recall` : part des chunks récupérés couverte par des citations source-backed ;
- `metrics.alce.citation_precision` : part des citations qui se lient aux chunks récupérés ;
- `metrics.deepeval.faithfulness` : score de support par les citations ou score de fixture explicite ;
- `metrics.trust_score` : moyenne déterministe recall/precision/faithfulness/confidence ;
- `trust_tier` : `certified`, `indicative` ou `unverified`.

Le tiers `certified` exige recall, precision, faithfulness et trust-score au-dessus
du seuil du gate. `indicative` signale une preuve exploitable mais insuffisante
pour une revendication certifiée. `unverified` est utilisé dès qu'un finding
bloquant existe.

Ce gate ne prouve pas la justesse métier finale d'une réponse LLM. Il prouve
seulement que la réponse suit le contrat cite-or-abstain, cite des spans
traçables, conserve les faits structurés et expose les incertitudes ou la décision
humaine requise. La décision réglementaire, juridique, clinique ou métier reste
hors revendication NOMOS.

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

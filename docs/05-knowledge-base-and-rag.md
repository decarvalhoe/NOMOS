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

L'ingestion doit partir du manifest, jamais d'un scan libre non contrôlé.

Pipeline standard :

```text
source-manifest.yaml
  -> extract text
  -> normalize
  -> chunk
  -> attach metadata
  -> embed
  -> store
  -> audit coverage
```

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
| `locator` | Page, ligne, section, sélecteur, fonction legacy. |
| `priority` | Priorité source. |
| `status` | Statut source. |
| `effective_from` | Date d'application si pertinente. |
| `license` | Usage autorisé. |
| `confidentiality` | Niveau de protection. |
| `ingested_at` | Timestamp. |
| `ingestion_version` | Version du pipeline. |

## Chunking

Le chunking doit respecter le sens métier.

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

## Tests Associés

- Toutes les sources actives sont indexées.
- Tous les chunks ont metadata complète.
- Tous les chunks référencent un hash de source courant.
- Un changement de source invalide les chunks correspondants.
- Les questions de golden set retrouvent les chunks attendus.
- Le LLM ne répond pas sans citation pour les domaines marqués critiques.
- Les données structurées priment sur le texte récupéré.


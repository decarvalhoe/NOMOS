# 34 - Manuel Utilisateur NOMOS

Date: 2026-05-04
Statut: manuel utilisateur alpha
Audience: operateurs NOMOS, mainteneurs corpus, reviewers techniques

## Objectif

Ce manuel explique comment utiliser NOMOS pour diagnostiquer un projet,
traiter un corpus en lecture seule, produire un pack d'artefacts et lire
les resultats sans surestimer les claims.

Pour installer NOMOS dans GitHub Actions, voir
[`35-nomos-integration-manual.md`](35-nomos-integration-manual.md) et
[`31-github-workflow-setup.md`](31-github-workflow-setup.md).

## Pre-requis

En local:

- Git;
- Go selon `cli/go.mod`;
- Python 3 pour les scripts de gate et evidence;
- CUE pour les schemas;
- un clone du repo NOMOS;
- un clone du corpus source ou un chemin de corpus monte en lecture seule.

Sur Windows, utiliser PowerShell pour les scripts Windows et un shell
Bash pour les scripts `.sh` quand ils l'exigent.

## Regle De Securite Source

Avant tout run, le corpus source doit etre protege:

```bash
git -C <corpus> status --short
git -C <corpus> remote set-url --push origin DISABLED
git -C <corpus> remote -v
```

Apres le run, verifier:

```bash
git -C <corpus> status --short
```

Le resultat attendu est vide. Toute mutation source est un incident.

## Commandes De Base

Depuis le repo NOMOS:

```bash
cd cli
go test ./...
go build -o ../bin/nomos .
../bin/nomos help
```

Commandes corpus attendues selon le build:

```bash
nomos corpus scan
nomos corpus diff
nomos corpus manifest
nomos corpus validate-sidecar
nomos corpus feed
nomos corpus body-ledger
nomos corpus attest
nomos rag export
nomos rag manifest
nomos rag delta
nomos rag verify
nomos answer gate
nomos answer eval
nomos strict
nomos github plan
```

Si une commande inconnue retourne `EXIT=0`, c'est un bug. Une commande
inconnue doit echouer en code non-zero.

## Run POC RBOK Lawbook

Exemple de run read-only sur `realisons-business/01_rbok`:

```bash
bash scripts/rbok-lawbook-e2e.sh \
  --corpus /path/to/realisons-business/01_rbok \
  --out /path/to/out
```

Resultats attendus pour un run sain:

- generation du feed;
- generation des metadata RAG;
- generation de l'import runtime;
- TOC certifiee;
- lexique gouverne;
- strict fidelity gate `pass`;
- attestation;
- aucun changement dans le repo source.

## Lire Le Pack De Sortie

### Feed

Le feed canonique est l'artefact principal.

Pour le profil RBOK lawbook, le texte utile est dans:

```text
rbok-lawbook-feed.json
  feeds[]
    source_path
    source_hash
    nodes[]
      node_id
      node_type
      title
      text
      span
      canonical_ref
      display_ref
      parent_chain
```

Un integrateur doit lire `feeds[].nodes[]`, pas seulement la racine du
JSON.

### Metadata RAG

`rbok-rag-metadata.json` porte les informations de retrieval:

```text
chunk_id
node_id
node_type
canonical_ref
display_ref
domain
priority
governance_status
parent_chain
source_hash
```

Cet artefact ne doit pas etre traite comme le corps complet du texte si
le champ `text` n'est pas present.

### Engine Import

`rbok-engine-import.json` sert de projection structurelle pour une
application downstream:

```text
documents[]
nodes[]
revisions[]
```

Il ne remplace pas le feed si l'application doit construire des chunks
RAG avec contenu.

### TOC

`rbok-certified-toc.json` reconstruit l'arborescence documentaire.
Elle sert a verifier que la profondeur, les titres et l'ordre
documentaire sont coherents.

### Body Ledger

`corpus-body-ledger.json`, quand present, sert a prouver ce qui est
couvert, non semantique, unsupported ou explicitement exclu. Le RAG
n'a pas vocation a contenir tout le body ledger; il doit contenir les
unites utiles et gouvernees.

### Semantic Quality

`semantic-quality-report.json` classe les unites selon leur utilite:

- `non_semantic`;
- `contextualized_in_parent`;
- `lexicon_atom`;
- `identifier_atom`;
- `normative_value_atom`;
- `requires_review`.

Un warning reviewable n'est pas toujours un blocage. Un finding
blocking doit bloquer la publication ou l'activation.

## Verification Minimum

Apres un run:

```bash
cd cli
go test ./...

python -m unittest discover -s tests -v
python scripts/regulated_docs_gate.py --report .regulated-doc-gate/regulated-doc-gate-report.json
python scripts/regulated_evidence_pack.py --output .regulated-evidence-pack/evidence-pack.json
```

Sur Windows:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\e2e.ps1
```

## Diagnostic Des Problemes Courants

| Symptome | Cause probable | Action |
|---|---|---|
| `total_files: 0` | mauvais chemin corpus ou fallback dummy | Corriger `--corpus` et echouer si aucun artefact reel. |
| metadata RAG sans texte | metadata lues seules | Joindre feed + metadata par `node_id` / `chunk_id`. |
| sources YAML absentes | profil courant ne les atomise pas en feed runtime | Produire un artefact YAML/JSON dedie ou etendre le profil. |
| strict gate pass mais runtime inutilisable | gate source-to-feed OK, import downstream non teste | Ajouter test d'import application. |
| PR comment sans output | publication en `artifact_only` ou output token absent | Verifier config publish et secrets. |
| mutation source detectee | step workflow ou script ecrit dans le corpus | Bloquer run, inspecter diff, corriger path output. |

## Bonnes Pratiques D'operation

- Toujours pinner le commit source dans le dossier de preuve.
- Ne jamais committer de document licencie en full text sans droit.
- Ne jamais publier un output sans trace manifest.
- Ne jamais activer un feed downstream sans import dry-run.
- Conserver les warnings dans l'evidence pack.
- Declarer explicitement les sources non atomisees.
- Preferer `pull_request` pour les corpus critiques.
- Reserver `direct_push` aux scopes bas risque ou aux decisions
  controlees.

## Sortie Attendue D'un Run Exploitable

Un run exploitable doit permettre a un reviewer de repondre a ces
questions:

1. Quel commit source a ete lu?
2. Le corpus source a-t-il ete modifie?
3. Quels fichiers ont ete admis, skips ou non atomises?
4. Quels nodes et chunks ont ete generes?
5. Quel texte supporte chaque chunk?
6. Quelle structure documentaire a ete reconstruite?
7. Quels warnings restent ouverts?
8. Quel gate autorise ou refuse la publication?
9. Quel output une application downstream peut importer?
10. Quelle claim publique est supportee?

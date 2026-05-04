# 35 - Manuel D'Integration NOMOS

Date: 2026-05-04
Statut: manuel d'integration alpha
Audience: integrateurs application, DevOps, mainteneurs corpus, reviewers qualite

## Objectif

Ce manuel explique comment connecter un corpus source, NOMOS et une
application downstream de maniere tracable et risk-based.

Il couvre:

- integration GitHub Actions;
- choix source-owned vs output-owned;
- publication artifact, PR ou direct push;
- contrat d'import application;
- activation runtime;
- verification et gates.

## Architecture Cible

```text
source corpus repo
  reference canonique, read-only
        |
        v
NOMOS workflow
  scan, diff, feed, ledger, RAG metadata, gates, trace
        |
        v
output repo or output path
  artefacts versionnes
        |
        v
downstream application
  import read-only projection, active feed, RAG/runtime trace
```

Pour RBOK, le POC peut viser `RBOKproject/RBOK` sur `develop`. La cible durable
recommandee est un output repository separe.

## Choix D'ownership

### Source-owned

La config `.nomos/corpus-workflows.yaml` vit dans le repository source.

Avantages:

- le reviewer de la PR source voit le diff NOMOS;
- le corpus owner controle les scopes;
- adapte aux workflows "quand la source change, verifier l'output".

Inconvenients:

- le repo source porte de la config operationnelle;
- il faut un token output si publication hors repo.

### Output-owned

La config vit dans le repository d'output.

Avantages:

- le downstream owner controle le rythme et la publication;
- la source reste minimale;
- adapte aux applications qui veulent rafraichir leurs artefacts selon
  leur cadence.

Inconvenients:

- il faut coordonner les changements de structure source;
- le trigger source vers output doit etre gere.

## Modes De Publication

| Mode | Effet | Usage |
|---|---|---|
| `artifact_only` | Upload d'artefacts GitHub Actions seulement. | Preview, smoke test, PR source. |
| `pull_request` | Commit des outputs sur une branche et ouverture de PR. | Mode recommande pour POC et contenu critique. |
| `direct_push` | Push direct dans `target_path`. | Flux bas risque ou decision controlee. |

La tracabilite est obligatoire dans tous les modes.

## Exemple Source-Owned

`.nomos/corpus-workflows.yaml` dans le repo source:

```yaml
schema_version: "0.1.0"
workflows:
  - id: rbok-lawbook
    description: RBOK lawbook canonical output
    source:
      repo: RBOKproject/realisons-business
      base_branch: main
      paths:
        - 01_rbok/**
      extensions:
        - .md
        - .yaml
        - .json
      profile: rbok-lawbook
    output:
      repo: RBOKproject/nomos-corpus-realisons-business
      branch: main
      path: rbok-lawbook/
    nomos:
      corpus_id: rbok-lawbook
      project_id: rbok
      commands:
        - scan
        - manifest
        - feed
        - body-ledger
        - attest
        - strict
    publish:
      mode: pull_request
      target_repo: output
      target_branch: main
      target_path: rbok-lawbook/
      branch_strategy: per_pr
      risk_class: medium
    notify:
      source_pr_comment:
        enabled: true
        mode: summary
        include:
          - changed_scopes
          - diff_summary
          - output_location
          - trace_manifest
          - gate_status
```

Caller workflow dans `.github/workflows/nomos.yml`:

```yaml
name: NOMOS scoped check
on:
  pull_request:
    branches: [main]
    types: [opened, synchronize, reopened, ready_for_review]

permissions:
  contents: read
  pull-requests: read
  actions: read

jobs:
  nomos:
    uses: RBOKproject/Nomos/.github/workflows/nomos-corpus-workflow.yml@main
    with:
      config_owner: corpus
      config_path: .nomos/corpus-workflows.yaml
      corpus_repository: ${{ github.repository }}
      corpus_ref: ${{ github.event.pull_request.head.sha }}
      base_ref: ${{ github.event.pull_request.base.ref }}
      head_ref: ${{ github.event.pull_request.head.ref }}
    secrets:
      NOMOS_CORPUS_READ_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Exemple Output-Owned

Le repo output contient `.nomos/corpus-workflows.yaml` et le workflow de
refresh.

```yaml
name: NOMOS scoped corpus refresh
on:
  workflow_dispatch:
    inputs:
      corpus_ref:
        required: true
        type: string
      base_ref:
        required: false
        type: string
        default: main
  repository_dispatch:
    types: [nomos-corpus-update]

permissions:
  contents: read
  pull-requests: read
  actions: read

jobs:
  stage_output_checkout:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          persist-credentials: false

  nomos:
    needs: stage_output_checkout
    uses: RBOKproject/Nomos/.github/workflows/nomos-corpus-workflow.yml@main
    with:
      config_owner: output
      config_path: .nomos/corpus-workflows.yaml
      corpus_repository: ${{ github.event.client_payload.corpus_repository || vars.NOMOS_CORPUS_REPOSITORY }}
      corpus_ref: ${{ inputs.corpus_ref || github.event.client_payload.corpus_ref }}
      base_ref: ${{ inputs.base_ref || github.event.client_payload.base_ref || 'main' }}
      output_repository: ${{ github.repository }}
    secrets:
      NOMOS_CORPUS_READ_TOKEN: ${{ secrets.NOMOS_CORPUS_READ_TOKEN }}
      NOMOS_OUTPUT_WRITE_TOKEN: ${{ secrets.NOMOS_OUTPUT_WRITE_TOKEN }}
```

## Secrets Et Permissions

| Secret | Scope minimal |
|---|---|
| `NOMOS_CORPUS_READ_TOKEN` | `contents: read` sur le repo source. |
| `NOMOS_OUTPUT_WRITE_TOKEN` | `contents: write` et `pull-requests: write` sur le repo output seulement. |
| `GITHUB_TOKEN` | PR comment sur le repo source, si active. |

Le token output ne doit pas donner de droit d'ecriture au repo source.

## Contrat D'output Pour Application

Une application downstream doit importer un bundle, pas un fichier
isole.

Minimum recommande:

```text
rbok-lawbook-feed.json
rbok-rag-metadata.json
rbok-engine-import.json
rbok-certified-toc.json
rbok-lawbook-index.json
rbok-governed-lexicon.yaml
rbok-strict-fidelity-gate.json
rbok-fidelity-proof.json
nomos-trace.json
attestation.json
```

Selon release:

```text
corpus-body-ledger.json
semantic-quality-report.json
short-critical-atoms.json
claim-coverage.json
```

## Regles D'import Application

L'importeur downstream doit:

1. Verifier le trace manifest.
2. Verifier le strict gate.
3. Refuser l'activation si un finding blocking existe.
4. Charger le feed canonique avec le texte.
5. Joindre metadata RAG et feed par `node_id` ou `chunk_id`.
6. Stocker source path, source hash, span et canonical ref.
7. Creer une version de feed immuable.
8. Activer la version atomiquement seulement apres validation.
9. Conserver l'ancienne version tant que la nouvelle n'est pas active.
10. Produire un audit log d'import.

## Tables Downstream Recommandees

Une application peut adapter les noms, mais doit couvrir:

| Projection | Champs critiques |
|---|---|
| feed version | feed name, version tag, source commit, artifact hash, active flag. |
| source | path, hash, admission status, corpus layer, authority. |
| unit | unit id, type, title, body, ordinal, metadata. |
| chunk | chunk id, unit id, content, token count, source locator. |
| locator | canonical ref, display ref, source span, parent chain. |
| rag metadata | keywords, domain, priority, governance status, step bindings. |
| import record | status, verdict, errors, operator, imported at, activated at. |

## Runtime RAG

Le runtime doit traiter NOMOS comme doctrine source-backed:

```text
active feed
-> admitted sources
-> ranked chunks
-> prompt context with citations
-> response trace
```

Le ranking minimum:

1. binding direct module/question/parcours;
2. corpus layer (`canon`, `commentary`, `practice`, `reference`);
3. authority (`primary`, `secondary`, `supporting`);
4. similarity vectorielle ou recherche lexicale;
5. ordre documentaire comme fallback.

Le RAG generique de l'application peut coexister, mais il ne doit pas
ecraser la doctrine NOMOS. La doctrine canonique doit etre priorisee
quand elle existe.

## Integration Des Parcours, Modules Et Questions

Pour une application de parcours conversationnels, NOMOS doit fournir ou
supporter:

- units doctrinales issues du lawbook;
- metadata qui permet de relier un module/question a une unit;
- extraction structuree YAML/JSON des parcours, modules et questions;
- step bindings versionnes;
- prompts qui n'ajoutent pas de questions hors step actif;
- trace runtime: step -> chunk -> unit -> source -> feed version.

Si les YAML/JSON ne sont pas encore dans le feed, l'integration doit les
traiter comme gap POC et creer un artefact dedie avant activation
runtime complete.

## Verification POC Recommandee

Pour un POC RBOK sur `develop`:

1. Generer les outputs NOMOS depuis `realisons-business/main`.
2. Publier les outputs vers un path controle sur `RBOK/develop` ou vers
   un output repo dedie.
3. Lancer un import dry-run dans RBOK.
4. Verifier que le texte vient du feed, pas seulement des metadata.
5. Verifier que les YAML parcours/modules/questions sont soit importes,
   soit explicitement declares non couverts.
6. Activer une version de feed sur environnement dev seulement.
7. Executer un scenario conversationnel avec step question only.
8. Verifier la trace: message -> step -> chunks -> source.
9. Documenter les warnings et limitations.
10. Ne pas promouvoir hors dev avant gates et tests verts.

## Definition Of Done Integration

Une integration NOMOS est done quand:

- la source est lue en read-only;
- chaque run produit un trace manifest;
- le publish mode est documente;
- l'application importe le format NOMOS courant;
- les tests import dry-run passent;
- le runtime utilise la version active seulement;
- la UI/admin permet de voir version, source, warnings et traces;
- le rollback vers feed precedent est possible;
- le comportement LLM est teste sur les flows critiques;
- la documentation downstream indique clairement que NOMOS fournit des
  artefacts derives, pas la source officielle elle-meme.

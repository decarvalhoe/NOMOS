# 50 — Kit de preuve de consommation croisée (côté NOMOS)

> NRT-029 (#702), docs/49 §4.5. La moitié NOMOS de la preuve de consommation
> croisée avec un consommateur externe — celle qui se livre sans lui. NOMOS
> produit un bundle canonique, l'exporte en enregistrements neutres, le
> fingerprinte ; un consommateur l'ingère en conservant `chunk_id` et
> `source_hash`, prouve que son index est l'export, fait passer ses réponses
> par le gate cite-or-abstain et rejoue le jeu doré. La partie conjointe avec
> le partenaire est #701 (NRT-030, `external`) ; rien ici ne la présume.

**Ce kit s'exécute.** Chaque bloc marqué `<!-- replay -->` est rejoué en CI
par `scripts/integration_guide_replay.py` contre les fixtures du dépôt, avec
le CLI construit depuis `cli/` ; les artefacts annoncés doivent exister après
le bloc, et un bloc qui attend un refus échoue si le refus ne vient pas. Ce
que cela prouve : la mécanique tourne sur cette toolchain, contre ce corpus.
Ce que cela ne prouve pas : qu'un consommateur réel l'a rejouée, ni la
qualité d'un retrieval ou d'une réponse.

Conventions des blocs : `$NOMOS` est le binaire construit depuis `cli/`,
`$REPO` la racine du dépôt, `$WORK` un répertoire de travail jetable (le
répertoire courant des blocs). Pré-requis : Go selon `cli/go.mod`, Python 3
avec PyYAML.

## 0. Claim boundary

- Le corpus du kit est le **corpus doré license-safe** du pack
  built-environment (`vd-lausanne`, synthétique). Les textes fédéraux suisses
  de la construction ne sont **jamais redistribués** : NOMOS commet un reçu
  hash-only du connecteur `ch-fedlex-eli`
  (`docs/regulated/ch-connectors/ch-fedlex-eli.evidence.json`) ; le
  consommateur va chercher les textes par ELI et rejoue les mêmes commandes
  chez lui.
- Le kit prouve l'**identité** (chaque chunk est adressable, chaque source est
  hachée), la **fidélité de l'ingestion** (l'index est l'export, octet pour
  octet sur les textes indexés) et la **discipline cite-or-abstain** (une
  réponse cite un chunk que le bundle contient, ou s'abstient). Il ne mesure
  ni un retrieval, ni un modèle, ni un déploiement.
- Tous les seuils exercés sont des **défauts** documentés (`docs/49` §2.2) ;
  l'inventaire de paramètres du kit le dit et la CI le vérifie.
- Aucune donnée d'affaire, de témoin ou privilégiée n'entre dans NOMOS
  (`docs/49` §4.4).

## 1. Contrats sur lesquels ce kit s'appuie

Stabilité et version viennent de `specs/contract-registry.yaml` ; le rejeu
compare cette table au registre. Les versions d'enregistrement du moteur
(`nomos-rag-chunk-v1`, `nomos-rag-index-manifest-v1`,
`nomos-rag-retrieval-contract-v1`) sont portées par le code et par les
artefacts eux-mêmes, non par le registre : un consommateur les lit dans ce
qu'il reçoit.

<!-- contracts -->

| Contrat | Stabilité | Version | Rôle pour le consommateur |
|---|---|---|---|
| `canonical-knowledge-bundle` | stable | ckm-bundle-v1 | bundle canonique produit par `nomos bundle`, vérifié par le kit consommateur avant toute confiance |
| `knowledge-lens` | stable | 0.1.0 | lentille imposée à l'export (`--lens`), exclusion fail-closed au niveau de base |
| `facets` | stable | 0.1.0 | axes de facettes portés par chaque enregistrement exporté (champs de filtre du contrat de retrieval) |
| `domain-pack` | stable | nomos-domain-pack-v1 | pack built-environment : corpus doré, facettes documentaires, presets de lentilles |
| `domain-cartography` | experimental | nomos-domain-cartography-v1 | cartographie du périmètre exercé, comptes recoupés avec le manifeste |
| `point-in-time` | stable | 0.1.0 | résolution à une date, sur les atomes ; le contrat de retrieval déclare le scoping temporel `unsupported` sur les enregistrements |
| `ai-rag-controls` | experimental | 0.1.0 | baseline de gouvernance IA/RAG, dont la frontière d'inférence (`docs/49` §2.3) |

## 2. Côté NOMOS — produire ce que le consommateur reçoit

### 2.1 Le bundle canonique du corpus doré

Le corpus est copié hors du dépôt (un bundle se produit toujours depuis une
copie de travail) ; la trace du bundle nomme le dépôt et le commit.

<!-- replay expects: corpus/permis.md, out/bundle.json -->

```bash
rm -rf corpus && cp -r "$REPO/cli/internal/corpus/testdata/aec-golden-corpus/vd-lausanne" corpus
"$NOMOS" bundle --root corpus --bundle-id ch-construction-kit \
  --domain built-environment --country CH \
  --repo "${GITHUB_REPOSITORY:-decarvalhoe/NOMOS}" --branch "${GITHUB_REF_NAME:-local}" \
  --commit "$(git -C "$REPO" rev-parse HEAD)" --workflow-run-id "${GITHUB_RUN_ID:-local}" \
  --event "${GITHUB_EVENT_NAME:-workflow_dispatch}" --out out/bundle.json
```

### 2.2 Le kit consommateur : ne rien croire avant de vérifier

Avant toute ingestion, le consommateur rejoue le kit de conformité (VRC-36) :
version de schéma exacte, invariants structurels, vocabulaire de facettes,
digest d'attestation **recalculé** sur la charge utile, forme réelle de chaque
`source_hash`. Un octet altéré après émission est un refus nommé.

<!-- replay expects: out/consumer-kit.json -->

```bash
python3 "$REPO/scripts/nomos_consumer_kit.py" --bundle out/bundle.json > out/consumer-kit.json
python3 -c 'import json; v=json.load(open("out/consumer-kit.json")); assert v["status"]=="pass", v["findings"]'
```

### 2.3 L'export neutre et son manifeste

Les facettes documentaires du pack sont attachées par source avant tout
scoping ; `--strict` refuse un export dont un chunk n'est pas citable. Le
manifeste est l'empreinte de ce qui est remis à l'index : digest global,
empreinte par chunk (`source_hash`, `embedding_hash`, `body_hash`), digest par
source, contrat de retrieval calculé (champs filtrables et valeurs qui
existent, scoping non supporté nommé).

<!-- replay expects: out/chunks.jsonl, out/manifest.json -->

```bash
KIT="$REPO/docs/regulated/domain-packs/built-environment"
"$NOMOS" rag export --bundle out/bundle.json --document-facets "$KIT/retrieval-harness.yaml" \
  --format jsonl --output out/chunks.jsonl --strict
"$NOMOS" rag manifest --bundle out/bundle.json --document-facets "$KIT/retrieval-harness.yaml" \
  --output out/manifest.json
python3 -c 'import json; m=json.load(open("out/manifest.json")); assert m["rejected_count"]==0 and m["chunk_count"]>0, m; print(m["chunk_count"], "chunks,", len(m["sources"]), "sources, digest", m["chunk_digest"][:23])'
```

### 2.4 Une lentille est un scope, pas une convention

Avec un preset du pack, l'export exclut au niveau de base tout chunk hors
scope — et tout chunk sans facette, dont l'appartenance ne peut pas être
prouvée. Le manifeste lie l'index à la lentille : une autre lentille est un
autre index.

<!-- replay expects: out/chunks-permis.jsonl, out/manifest-permis.json -->

```bash
KIT="$REPO/docs/regulated/domain-packs/built-environment"
"$NOMOS" rag export --bundle out/bundle.json --document-facets "$KIT/retrieval-harness.yaml" \
  --lens "$KIT/aec-lens-presets/permis.lens.yaml" --format jsonl --output out/chunks-permis.jsonl
"$NOMOS" rag manifest --bundle out/bundle.json --document-facets "$KIT/retrieval-harness.yaml" \
  --lens "$KIT/aec-lens-presets/permis.lens.yaml" --output out/manifest-permis.json
python3 - <<'PY'
import json
u = json.load(open("out/manifest.json")); s = json.load(open("out/manifest-permis.json"))
assert s["excluded_by_lens_count"] > 0, "the lens excluded nothing: it is not a scope"
assert s["chunk_count"] + s["excluded_by_lens_count"] == u["chunk_count"], "scoped + excluded != unscoped"
assert s.get("lens"), "the scoped manifest is not bound to its lens"
print("lens LENS-AEC-PERMIS:", s["chunk_count"], "kept,", s["excluded_by_lens_count"], "excluded")
PY
```

### 2.5 La fraîcheur se prouve, dans les deux sens

`rag verify` compare le corpus tel qu'il est à l'index tel qu'il a été
construit : vert quand rien n'a bougé, rouge (exit 1) avec un plan de
réindexation quand une source a changé. Le kit exige les deux : un gate qui
ne rougit jamais n'est pas un gate.

<!-- replay expects: out/verify.json -->

```bash
KIT="$REPO/docs/regulated/domain-packs/built-environment"
"$NOMOS" rag verify --bundle out/bundle.json --document-facets "$KIT/retrieval-harness.yaml" \
  --manifest out/manifest.json --output out/verify.json
python3 -c 'import json; v=json.load(open("out/verify.json")); assert v["stale"] is False, v'
```

<!-- replay expects: out/verify-stale.json, out/delta.json -->

```bash
KIT="$REPO/docs/regulated/domain-packs/built-environment"
rm -rf corpus-modified && cp -r corpus corpus-modified
printf '\n\nModification fictive posterieure a l export.\n' >> corpus-modified/permis.md
"$NOMOS" bundle --root corpus-modified --bundle-id ch-construction-kit \
  --domain built-environment --country CH \
  --repo "${GITHUB_REPOSITORY:-decarvalhoe/NOMOS}" --branch "${GITHUB_REF_NAME:-local}" \
  --commit "$(git -C "$REPO" rev-parse HEAD)" --workflow-run-id "${GITHUB_RUN_ID:-local}" \
  --event "${GITHUB_EVENT_NAME:-workflow_dispatch}" --out out/bundle-modified.json
if "$NOMOS" rag verify --bundle out/bundle-modified.json --document-facets "$KIT/retrieval-harness.yaml" \
     --manifest out/manifest.json --output out/verify-stale.json; then
  echo "rag verify accepted a stale index"; exit 1
fi
python3 -c 'import json; v=json.load(open("out/verify-stale.json")); assert v["stale"] is True and v["summary"], v'
"$NOMOS" rag manifest --bundle out/bundle-modified.json --document-facets "$KIT/retrieval-harness.yaml" \
  --output out/manifest-modified.json
"$NOMOS" rag delta --old out/manifest.json --new out/manifest-modified.json --output out/delta.json
```

## 3. Côté consommateur — ingérer, puis prouver que l'index est l'export

### 3.1 La preuve d'import

Le consommateur remet un vidage de son index, un enregistrement JSON par
ligne : l'enregistrement neutre tel qu'il l'a stocké, ou la projection
`langchain` / `llamaindex`, ou un vidage plat (`chunk_id`, `source_hash`, puis
les textes ou leurs hachages). `scripts/cross_consumption_import_check.py`
recoupe chaque chunk avec le manifeste — présent une fois, aucun chunk en
trop, `source_hash` identique, textes qui hachent aux empreintes du manifeste,
comptes par source — et **recalcule le digest de l'index** depuis les
enregistrements du consommateur : s'il vaut `chunk_digest`, l'index est
l'export.

<!-- replay expects: out/consumer-index.jsonl, out/import-verdict.json, out/import-verdict-langchain.json -->

```bash
KIT="$REPO/docs/regulated/domain-packs/built-environment"
cp out/chunks.jsonl out/consumer-index.jsonl
"$NOMOS" rag export --bundle out/bundle.json --document-facets "$KIT/retrieval-harness.yaml" \
  --format langchain --output out/consumer-index-langchain.jsonl --strict
python3 "$REPO/scripts/cross_consumption_import_check.py" --manifest out/manifest.json \
  --index out/consumer-index.jsonl --report out/import-verdict.json > /dev/null
python3 "$REPO/scripts/cross_consumption_import_check.py" --manifest out/manifest.json \
  --index out/consumer-index-langchain.jsonl --report out/import-verdict-langchain.json > /dev/null
python3 -c 'import json; v=json.load(open("out/import-verdict.json")); d=v["index_check"]; assert v["status"]=="pass" and d["index_digest"]==d["manifest_chunk_digest"], v["findings"]; print("index digest = manifest digest:", d["index_digest"][:23])'
```

### 3.2 Un chunk altéré après ingestion est un refus nommé

<!-- replay expects: out/import-verdict-tampered.json -->

```bash
python3 - <<'PY'
import json
records = [json.loads(l) for l in open("out/consumer-index.jsonl", encoding="utf-8") if l.strip()]
records[0]["body_text"] += " (altéré après ingestion)"
with open("out/consumer-index-tampered.jsonl", "w", encoding="utf-8") as f:
    for r in records:
        f.write(json.dumps(r, ensure_ascii=False) + "\n")
PY
if python3 "$REPO/scripts/cross_consumption_import_check.py" --manifest out/manifest.json \
     --index out/consumer-index-tampered.jsonl --report out/import-verdict-tampered.json > /dev/null 2>&1; then
  echo "the import check accepted an altered chunk"; exit 1
fi
grep -q 'body_hash' out/import-verdict-tampered.json
```

### 3.3 Les réponses citent ce que le bundle contient, ou s'abstiennent

Les enregistrements de réponse du kit (`docs/cross-consumption-kit/answers.yaml`)
passent `nomos answer gate` : quatre réponses ancrées mot pour mot dans un
chunk du bundle sont citées, la question hors corpus est un refus explicite
— le gate s'abstient, il n'invente pas. Puis chaque citation (`chunk_id`,
`source_hash`, `source_id`) est recoupée avec le manifeste : une réponse ne
peut pas citer un chunk que le bundle ne contient pas, et une dérive du
corpus rend les fixtures rouges au lieu de les laisser mentir.

<!-- replay expects: out/answer-gate.json, out/citations-verdict.json -->

```bash
KIT="$REPO/docs/cross-consumption-kit"
"$NOMOS" answer gate --fixtures "$KIT/answers.yaml" > out/answer-gate.json
python3 -c 'import json; v=json.load(open("out/answer-gate.json")); assert v["status"]=="pass" and v["cited"]==4 and v["abstained"]==1, (v["status"], v["cited"], v["abstained"])'
python3 "$REPO/scripts/cross_consumption_import_check.py" --manifest out/manifest.json --index out/consumer-index.jsonl \
  --citations "$KIT/answers.yaml" --citations "$KIT/eval-corpus.yaml" --report out/citations-verdict.json > /dev/null
```

### 3.4 Le jeu doré et ses seuils versionnés

<!-- replay expects: out/answer-eval.json -->

```bash
KIT="$REPO/docs/cross-consumption-kit"
"$NOMOS" answer eval --corpus "$KIT/eval-corpus.yaml" --thresholds "$KIT/thresholds.yaml" > out/answer-eval.json
python3 -c 'import json; v=json.load(open("out/answer-eval.json")); assert v["status"]=="pass" and v["failed_answers"]==0, v'
```

## 4. Ce que le kit dit de lui-même — et la CI le recompte

### 4.1 Inventaire de paramètres et cartographie de domaine

L'inventaire (`docs/cross-consumption-kit/parameter-inventory.yaml`, template
`docs/49` §2.2) nomme chaque paramètre du chemin exercé avec sa valeur réelle
et son statut : les seuils du gate et du harnais sont `default`, et un
`validated` sans preuve datée est refusé. La cartographie
(`docs/cross-consumption-kit/cartography.yaml`, contrat `docs/49` §2.1, `cue vet` en
CI) déclare ce que le kit tient, couche par couche ; ses comptes sont
**recomptés contre le manifeste** à chaque rejeu, et la couche graphe dit
qu'elle n'est pas vérifiée au lieu de porter un chiffre.

<!-- replay expects: out/inventory-verdict.json, out/cartography-verdict.json -->

```bash
KIT="$REPO/docs/cross-consumption-kit"
python3 "$REPO/scripts/parameter_inventory_check.py" --inventory "$KIT/parameter-inventory.yaml" \
  --report out/inventory-verdict.json > /dev/null
KIT="$KIT" python3 - <<'PY'
import json, os, sys, yaml
c = yaml.safe_load(open(os.environ["KIT"] + "/cartography.yaml", encoding="utf-8"))
m = json.load(open("out/manifest.json", encoding="utf-8"))
dom = next(d for d in c["domains"] if d["id"] == "built-environment-ch")
sub = next(s for s in dom["subcorpora"] if s["id"] == "pack-golden-corpus")
layers = sub["layers"]
facets = next((f["records"] for f in m["retrieval_contract"]["filter_fields"] if f["field"] == "facets.activity"), 0)
checks = {
    "unit_count = manifest sources": (sub["unit_count"], len(m["sources"])),
    "source.count = manifest sources": (layers["source"]["count"], len(m["sources"])),
    "index.count = manifest chunk_count": (layers["index"]["count"], m["chunk_count"]),
    "index rejected = 0": (0, m["rejected_count"]),
    "enrichment.count = records carrying facets.activity": (layers["enrichment"]["count"], facets),
    "graph not verified, no count": ("not_verified", layers["graph"]["verified_by"]),
}
drift = {k: v for k, v in checks.items() if v[0] != v[1]}
verdict = {"status": "fail" if drift else "pass", "as_of": c["as_of"],
           "checks": {k: {"cartography": v[0], "measured": v[1]} for k, v in checks.items()}}
json.dump(verdict, open("out/cartography-verdict.json", "w", encoding="utf-8"), indent=2)
if drift:
    print("cartography drift:", drift)
    sys.exit(1)
print("cartography recounted against the manifest:", len(checks), "checks")
PY
```

### 4.2 Ce que le kit prouve, ce qu'il ne prouve pas

| Prouvé par ce rejeu | Non prouvé, et dit |
|---|---|
| le bundle est attesté et un octet altéré est refusé (§2.2) | qu'un consommateur réel a rejoué le kit (#701) |
| chaque chunk exporté est adressable, haché, citable (§2.3) | la qualité d'un retrieval ou d'un reranking |
| une lentille exclut au niveau de base, fail-closed (§2.4) | qu'une lentille modélise correctement un cloisonnement métier |
| un index périmé rougit, un index frais est vert (§2.5) | la fraîcheur d'un index que NOMOS n'a pas fingerprinté |
| l'index du consommateur est l'export, digest recalculé (§3.1, §3.2) | ce que le consommateur fait de l'index après |
| une réponse cite un chunk réel ou s'abstient (§3.3) | la justesse sémantique d'une réponse (proxy lexical, négation aveugle) |
| le jeu doré tient ses seuils — des défauts, dits tels (§3.4, §4.1) | qu'un seuil est calibré ; quatre questions ne mesurent pas un retriever |

## 5. Pour le partenaire (#701) — les mêmes commandes, sur les vrais textes

1. Récupérer les textes fédéraux par ELI de son côté et commettre le reçu du
   connecteur (`nomos connector fetch --connector-id ch-fedlex-eli --accept
   application/rdf+xml`) : l'identité ELI et le `sha256` du reçu sont ce que
   les deux parties comparent, jamais un texte transmis par NOMOS.
2. Produire le bundle depuis sa copie (§2.1), le vérifier (§2.2), l'exporter et
   le fingerprinter (§2.3) — le manifeste est la seule chose que NOMOS a besoin
   de voir.
3. Ingérer, puis remettre le vidage d'index et le verdict d'import (§3.1) : le
   digest recalculé vaut `chunk_digest`, ou la preuve n'a pas eu lieu.
4. Répondre au jeu de questions annoté (§3.3, §3.4) et remettre les
   enregistrements de réponse ; NOMOS rejoue le gate et le harnais sur ces
   enregistrements, et publie la mesure datée.
5. Remplir la cartographie (§4.1) sur son propre index — chaque couche comptée
   chez lui, `not_verified` là où rien ne l'a été — et l'inventaire des
   paramètres de son chemin.

Lignes rouges (`docs/49` §4.4) : corpus publics uniquement ; aucun code ni
contenu du partenaire n'entre dans NOMOS ; un résultat de pipeline du
partenaire est un input recompté, jamais une preuve NOMOS ; aucune affirmation
« sécurisé », « certifié » ou « validé » d'un côté ni de l'autre.

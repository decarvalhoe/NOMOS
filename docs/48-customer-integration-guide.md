# 48 - Customer Integration Guide

Point d'entree unique pour un integrateur. Il consolide, du point de vue de
celui qui branche NOMOS sur son depot, le manuel utilisateur (`docs/34`), le
manuel d'integration (`docs/35`), la mise en place du workflow GitHub
(`docs/31`) et la frontiere de la GitHub App (`docs/32`). Il ne les remplace
pas : il dit dans quel ordre les lire et quelles commandes executer.

**Ce guide s'execute.** Chaque bloc marque `<!-- replay -->` est rejoue en CI
par `scripts/integration_guide_replay.py` contre les fixtures du depot ; les
artefacts annonces doivent exister apres le bloc. Une commande qui cesse de
fonctionner rougit la CI. Ce que cela prouve : le guide tourne sur cette
toolchain, contre ces fixtures. Ce que cela ne prouve pas : qu'un client l'a
valide dans son contexte (`docs/28`, voie regulee).

Conventions des blocs : `$NOMOS` est le binaire construit depuis `cli/`,
`$REPO` la racine du depot, `$WORK` un repertoire de travail jetable (le
repertoire courant des blocs).

## 0. Claim boundary

- NOMOS calcule et verifie ; il n'approuve rien, ne certifie rien et ne
  publie rien (`docs/public-claim-boundary.md`).
- Une release est un CANDIDAT jusqu'a la decision humaine consignee
  (`RELEASE.md`, `docs/regulated/lifecycle/`).
- L'usage regule (GxP, SaMD, etc.) passe par la checklist
  `templates/regulated/customer-integration-checklist.md` — liee ici, jamais
  dupliquee — et par la voie regulee de `docs/roadmap-lanes.yaml`.

## 1. Contrats sur lesquels ce guide s'appuie

Stabilite et version viennent de `specs/contract-registry.yaml` (NRT-023) ;
`nomos contracts status` les verifie, ce guide les recopie et le rejeu
compare la copie au registre.

<!-- contracts -->

| Contrat | Stabilite | Version | Role pour l'integrateur |
|---|---|---|---|
| `nomos-project` | stable | 0.1.0 | manifeste projet (`nomos init`, `nomos validate`) |
| `source-manifest` | stable | 0.1.0 | manifeste des sources d'un corpus |
| `canonical-matrix` | stable | 0.1.0 | matrice canonique lue par le strict gate |
| `nomos-report.schema` | stable | 0.1.0 | rapport JSON (`nomos report`, `nomos diagnose`) |
| `external-snapshot` | stable | nomos.external-snapshot.v1 | enveloppe immuable d'un export externe (Recursio) |
| `adapter-manifest` | experimental | 0.1.0 | manifeste d'un adapter (`adapters/*/adapter.nomos.yaml`) |
| `nomos-github-workflow` | experimental | - | configuration `.nomos/corpus-workflows.yaml` (`nomos github plan`) |
| `portfolio-status` | experimental | nomos-portfolio-status-v1 | statut de portefeuille calcule (`nomos portfolio status`) |
| `nomos-praxis-evidence` | experimental | nomos-praxis-evidence-exchange-v1 | echange d'evidence Nomos/Praxis |

Un contrat `experimental` peut changer sans preavis de MAJOR ; un contrat
`stable` ne change pas sans bump accepte au registre (`docs/16`).

## 2. Construire le CLI et lire ce qu'il annonce

Pre-requis : Go selon `cli/go.mod`, Python 3 et `jq` ; CUE pour les schemas.
Le coeur annonce ce qu'il lit et ecrit ; la matrice de compatibilite generee
est dans `docs/16`.

<!-- replay expects: bin/nomos, out/version.json, out/contracts-status.json -->

```bash
( cd "$REPO/cli" && go build -o "$WORK/bin/nomos" . )
"$WORK/bin/nomos" version
"$NOMOS" version --json --repo-root "$REPO" > out/version.json
"$NOMOS" contracts status --repo-root "$REPO" --out out/contracts-status.json
jq -e '.adapters | all(.verdict == "compatible")' out/version.json > /dev/null
```

Artefacts : `out/version.json` (version du coeur, contrats lus/ecrits,
formats, verdict des adapters), `out/contracts-status.json`.

## 3. Initialiser un depot applicatif et le valider

`nomos init` ecrit les manifestes dans un repertoire vide ; `nomos validate`
verifie chaque manifeste nomme contre son contrat. Le mode `regulated` ajoute les manifestes de
la voie regulee (`docs/34`).

<!-- replay expects: app/nomos.project.yaml, out/validate.json -->

```bash
"$NOMOS" init --mode minimal app
"$NOMOS" validate --format json app/nomos.project.yaml > out/validate.json
```

## 4. Diagnostiquer un depot avant admission

`nomos diagnose` emet un pre-rapport d'admission au format `nomos-report`
sans rien modifier. Ici sur une fixture du depot ; chez vous, `--root` pointe
sur votre clone.

<!-- replay expects: out/diagnose.json -->

```bash
"$NOMOS" diagnose --root "$REPO/cli/internal/diagnose/testdata/corpus/nomos-ready" --format json > out/diagnose.json
jq -e '.verdict' out/diagnose.json > /dev/null
```

## 5. Le strict gate sur les manifestes

Le strict gate lit le projet, les sources et la matrice canonique et rend
PASS ou un refus nomme ; c'est lui qu'une CI branche (`docs/35`, section
« Contrat D'output Pour Application »).

<!-- replay expects: out/strict-manifests.json -->

```bash
"$NOMOS" strict \
  --project "$REPO/cli/internal/app/testdata/gate-project.yaml" \
  --sources "$REPO/cli/internal/app/testdata/gate-sources.yaml" \
  --matrix  "$REPO/cli/internal/app/testdata/gate-matrix.yaml" \
  --format json > out/strict-manifests.json
```

## 6. Un corpus externe, de l'export a l'attestation

Chaine complete sur la fixture Recursio (`tests/fixtures/recursio-e2e`) :
enveloppe verifiee, manifeste importe, scan, feed, body ledger, attestation,
strict gate avec le snapshot externe. Le corpus lu par NOMOS est une copie
jetable versionnee ; la source n'est jamais modifiee (`docs/34`, « Regle De
Securite Source »).

<!-- replay expects: out/snapshot-verify.json, out/source-manifest.yaml, out/feed.json, out/body-ledger.json, out/attestation.json, out/strict-corpus.json -->

```bash
EXPORT="$REPO/tests/fixtures/recursio-e2e/export"
rm -rf corpus && mkdir corpus && cp -R "$EXPORT/captures" corpus/captures
( cd corpus && git init -q && git add -A && git -c user.email=nomos@local -c user.name=nomos commit -qm "captures" )
"$NOMOS" corpus snapshot verify --envelope "$EXPORT/snapshot.json" --records "$EXPORT/sources.jsonl" --out out/snapshot-verify.json
"$NOMOS" corpus snapshot import --envelope "$EXPORT/snapshot.json" --records "$EXPORT/sources.jsonl" --out out/source-manifest.yaml
"$NOMOS" corpus scan --root corpus --out out/scan.json
"$NOMOS" corpus feed --root corpus --snapshot out/scan.json --manifest out/source-manifest.yaml --out out/feed.json
"$NOMOS" corpus body-ledger --root corpus --manifest out/source-manifest.yaml --out out/body-ledger.json
"$NOMOS" corpus attest --snapshot out/scan.json --corpus-id guide-fixture --project-id nomos \
  --feed out/feed.json --corpus-body-ledger out/body-ledger.json \
  --external-snapshot "$EXPORT/snapshot.json" --external-snapshot-records "$EXPORT/sources.jsonl" --out out/attestation.json
"$NOMOS" strict --external-snapshot "$EXPORT/snapshot.json" --external-snapshot-records "$EXPORT/sources.jsonl" \
  --corpus-integrity-source corpus --corpus-integrity-feed out/feed.json \
  --corpus-body-ledger out/body-ledger.json --format json > out/strict-corpus.json
```

## 7. Le workflow GitHub : planifier un diff scope

`nomos github plan` lit `.nomos/corpus-workflows.yaml` et la liste des chemins
modifies, et ecrit `nomos-diff.json` : c'est l'artefact que le workflow de
`docs/31` consomme. Le choix source-owned / output-owned, les secrets et les
permissions sont dans `docs/31` (sections 4 a 6) ; la GitHub App n'existe pas
et ce que le workflow ne fera jamais est dans `docs/32`.

<!-- replay expects: out/nomos-diff.json -->

```bash
printf 'docs/a.md\ndocs/sub/b.md\n' > changed.txt
"$NOMOS" github plan --config "$REPO/specs/examples/nomos-github-workflow.source-owned.valid.yaml" \
  --changed-paths changed.txt --out out/nomos-diff.json --frozen-time 2026-09-06T12:00:00Z
```

## 8. Lire l'etat du produit lui-meme

Le statut de portefeuille est une vue calculee des sources machine commitees
(registre, lanes, ledger, records) ; il ne leve aucun claim.

<!-- replay expects: out/portfolio-status.json -->

```bash
"$NOMOS" portfolio status --repo-root "$REPO" --out out/portfolio-status.json
```

## 9. Ce que ce guide ne couvre pas

- l'activation d'un usage regule et l'approbation d'une release : voie
  regulee, `docs/28`, `templates/regulated/customer-integration-checklist.md` ;
- le runtime RAG et l'import des parcours : `docs/35` (sections « Runtime
  RAG » et suivantes), non rejoues ici ;
- les scripts E2E complets (`scripts/e2e.sh`, `scripts/recursio-e2e-fixture.sh`,
  `scripts/rbok-lawbook-e2e.sh`) qui restent la reference de bout en bout.

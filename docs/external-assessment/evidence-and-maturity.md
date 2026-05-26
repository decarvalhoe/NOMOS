# Dossier de preuves et de maturité — intrant pour évaluation externe

> Ce document est un **intrant neutre** destiné à une évaluation externe et indépendante de l'état du projet NOMOS. Il n'affirme **aucune valeur** monétaire ou stratégique et ne formule **aucune conclusion** sur la valeur du projet. Il présente des faits vérifiables : ce qui est réellement implémenté et testé, ce qui ne l'est pas, et les écarts connus. L'analyste tire ses propres conclusions.
>
> Le cadrage des affirmations publiques fait foi : voir [public-claim-boundary.md](../public-claim-boundary.md). Les **intrants de valorisation** (cadres comptables et comparables de marché, sans verdict) sont isolés dans [valuation-inputs.md](valuation-inputs.md) pour que l'analyste les applique lui-même.

## Comment lire ce dossier

- Chaque affirmation se rattache à une **preuve** : code, tests, configuration CI, artefact généré, ou un **écart nommé**.
- Les métriques quantitatives ont été mesurées le **2026-05-26** sur le commit `0c9e8fa` (branche `codex/docs-refresh-business-value`, alignée sur `origin/main`).
- Des commandes de reproduction sont fournies (section « Vérifier soi-même ») : rien ici ne demande d'être cru sur parole.
- Périmètre : ce dossier décrit l'état **observé**. Il ne présente pas la feuille de route comme une capacité.

## 1. Ce qu'est NOMOS aujourd'hui

NOMOS est une **CLI Go** (`cli/`, version interne `0.1.0-ALPHA` déclarée dans `cli/internal/app/app.go`) qui transforme un corpus de sources d'autorité en artefacts canoniques tracés (nœuds, TOC, feed/RAG source-backed, body ledger), applique des gates de fidélité, et produit des attestations de style in-toto.

Surface de commandes réellement enregistrées dans le dispatcher (`cli/internal/app/app.go`) :

| Commande | Rôle | Statut |
|---|---|---|
| `init` | Initialise les manifests d'un projet (mode minimal ou regulated). | implémenté |
| `validate` | Valide manifests et schémas (CUE/YAML). | implémenté |
| `diagnose` | Inspecte un dépôt, émet un pré-rapport d'admission (JSON/Markdown). | implémenté |
| `corpus` | Scan → manifest → validate-sidecar → diff → feed → body-ledger → attest. | implémenté |
| `strict` | Gate de release/intégrité strict. | implémenté |
| `github` | Intégration workflow GitHub (planification de diffs scopés). | implémenté |
| `evidence` | Hash, prépare/signe et vérifie des bundles d'evidence. | implémenté |
| `version`, `help` | Triviales. | implémenté |

Le code contient un helper `notImplemented` (`app.go`), mais **aucune commande active ne l'utilise** : il n'y a pas de commande de premier niveau en stub.

## 2. Implémenté et testé

Le pipeline cœur est réel et couvert par des tests (assertions, fixtures golden), pas seulement esquissé.

Métriques mesurées (commit `0c9e8fa`, 2026-05-26) :

| Mesure | Valeur |
|---|---:|
| Lignes Go hors tests (`cli/` + `control-plane/`) | 47 511 |
| Lignes Go de tests | 43 030 |
| Fonctions de test (`Test`/`Benchmark`/`Fuzz`) | 2 158 |
| Fichiers de test | 157 |
| Schémas CUE (`specs/`) | 25 |

Capacités avec implémentation **et** tests :

- scan de corpus en lecture seule, génération de manifest, diff et détection de mutation source ;
- extraction de nœuds canoniques avec spans source, TOC certifiée ;
- génération de feed et de métadonnées RAG source-backed ;
- body ledger (couverture du corps source) ;
- gate strict de fidélité/release ;
- génération d'attestation in-toto + enveloppe d'evidence (DSSE / compatible cosign) ;
- intégration workflow GitHub (diffs scopés).

Gates CI (`.github/workflows/`) : `go vet` et `go test -race` (CLI + control-plane), tests corpus multi-plateforme (Ubuntu / macOS / Windows), E2E RBOK lawbook et runtime, rapports de fidelity proof, gate de documentation régulée, evidence pack. Aucun seuil formel de couverture n'est défini.

## 3. Squelette / non implémenté à ce stade

À distinguer nettement du périmètre livré :

| Élément | État observé | Preuve |
|---|---|---|
| `adapters/` | Specs et fixtures uniquement ; **0 fichier Go**. Pas d'adaptateur exécutable. | `git ls-files 'adapters/*.go'` → vide |
| `policies/` | **1 fichier** (`policies/README.md`). Pas de moteur de policies. | `git ls-files policies/` |
| `control-plane/` | Packages fins ; **aucun serveur HTTP ni persistance** ; non câblé à la CLI. | aucun symbole `ListenAndServe` / `http.Server` |
| Regulated-readiness | **Structurel** : docs, templates et control records squelettes. Pas de QMS opérationnel, pas de certification ni de validation. | `docs/regulated/`, [public-claim-boundary.md](../public-claim-boundary.md) |
| RAG | **Métadonnées traçables** seulement ; retrieval vector-store et comportement LLM non validés en production. | [public-claim-boundary.md](../public-claim-boundary.md) (« RAG-ready ») |
| Domain packs (DOR-xxx) | Issues GitHub / specs ; **pas de code livré**. | [38-domain-opportunity-roadmap.md](../38-domain-opportunity-roadmap.md), `docs/regulated/domain-packs/` |

## 4. Prouvé vs non prouvé

Le projet définit lui-même des **niveaux de claims** et des **phrases réservées** (voir [public-claim-boundary.md](../public-claim-boundary.md)). En résumé factuel :

- **Prouvé (borné)** : sur **un seul** corpus privé (`RBOK 01_rbok`, run et commit enregistrés), le pipeline source→feed a produit 3024 unités feed source-backed, 0 byte non couvert au body ledger, gate strict `pass`, 0 finding sémantique bloquant. Détail et périmètre : [rbok-poc-validation-dossier.md](../rbok-poc-validation-dossier.md).
- **Non prouvé (à l'échelle plateforme)** : fidélité universelle multi-corpus / multi-format ; absence de warnings sémantiques sur corpus arbitraires ; câblage `claim_coverage` dans l'attestation ; validation client régulée.
- La phrase `full_fidelity_proven` est **réservée au run POC enregistré** et ne doit pas être lue comme une preuve à l'échelle plateforme.

## 5. Écarts connus

(Reflète le backlog au moment de la mesure, 2026-05-26 ; source : [15-product-backlog.md](../15-product-backlog.md).)

- `claim_coverage` non encore câblé dans l'attestation par la CLI (limite documentée).
- Acquisition et revue de licences de références régulées encore ouvertes (épopée RCP) → borne la preuve reference-to-control.
- Portabilité au-delà du Markdown RBOK : formats YAML/JSON partiels ; PDF, DOCX, images et documents scannés non supportés.
- Élévation du niveau de preuve POC RBOK encore ouverte (épopée AQ #314).

## 6. Vérifier soi-même

```bash
# Surface de commandes (lecture du dispatcher)
sed -n '19,45p' cli/internal/app/app.go

# Métriques de code
git ls-files 'cli/*.go' 'control-plane/*.go' | grep -v _test.go | xargs wc -l | tail -1
git ls-files 'cli/*_test.go' 'control-plane/*_test.go' | xargs wc -l | tail -1
git grep -hE '^func (Test|Benchmark|Fuzz)' -- '*_test.go' | wc -l
git ls-files '*_test.go' | wc -l
git ls-files 'specs/*.cue' | wc -l

# Couches squelette
git ls-files 'adapters/*.go'   # -> vide
git ls-files policies/         # -> policies/README.md

# Construire et exécuter
cd cli && go build -o ../nomos . && cd ..
./nomos help
go -C cli test ./...
```

---

> Aucune valorisation dans ce document. Les cadres comptables et comparables de marché (sans verdict) sont dans [valuation-inputs.md](valuation-inputs.md), à appliquer par l'analyste.

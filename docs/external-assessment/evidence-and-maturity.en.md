# Evidence and maturity dossier — input for external assessment

> Langues : [FR](evidence-and-maturity.md) · **EN** · [DE](evidence-and-maturity.de.md)

> This document is a **neutral input** for an independent external assessment of the NOMOS project's state. It asserts **no value** (monetary or strategic) and draws **no conclusion** about the project's worth. It presents verifiable facts: what is actually implemented and tested, what is not, and the known gaps. The analyst draws their own conclusions.
>
> The public claim contract is authoritative: see [public-claim-boundary.md](../public-claim-boundary.md). **Valuation inputs** (accounting frameworks and market comparables, with no verdict) are isolated in [valuation-inputs.en.md](valuation-inputs.en.md) for the analyst to apply independently.

## How to read this dossier

- Every statement maps to **evidence**: code, tests, CI configuration, a generated artifact, or a **named gap**.
- Quantitative metrics were measured on **2026-05-26** at commit `0c9e8fa` (branch `codex/docs-refresh-business-value`, aligned with `origin/main`).
- Reproduction commands are provided (section "Verify it yourself"): nothing here asks to be taken on trust.
- Scope: this dossier describes the **observed** state. It does not present the roadmap as capability.

## 1. What NOMOS is today

NOMOS is a **Go CLI** (`cli/`, internal version `0.1.0-ALPHA` declared in `cli/internal/app/app.go`) that turns a corpus of authority sources into traced canonical artifacts (nodes, TOC, source-backed feed/RAG, body ledger), runs fidelity gates, and produces in-toto-style attestations.

Command surface actually registered in the dispatcher (`cli/internal/app/app.go`):

| Command | Role | Status |
|---|---|---|
| `init` | Initialize a project's manifests (minimal or regulated mode). | implemented |
| `validate` | Validate manifests and schemas (CUE/YAML). | implemented |
| `diagnose` | Inspect a repository, emit an admission pre-report (JSON/Markdown). | implemented |
| `corpus` | Scan → manifest → validate-sidecar → diff → feed → body-ledger → attest. | implemented |
| `strict` | Strict release/integrity gate. | implemented |
| `github` | GitHub workflow integration (scoped-diff planning). | implemented |
| `evidence` | Hash, prepare/sign, and verify evidence bundles. | implemented |
| `version`, `help` | Trivial. | implemented |

The code contains a `notImplemented` helper (`app.go`), but **no active command uses it**: there is no top-level command stub.

## 2. Implemented and tested

The core pipeline is real and covered by tests (assertions, golden fixtures), not merely sketched.

Metrics measured (commit `0c9e8fa`, 2026-05-26):

| Measure | Value |
|---|---:|
| Non-test Go lines (`cli/` + `control-plane/`) | 47,511 |
| Go test lines | 43,030 |
| Test functions (`Test`/`Benchmark`/`Fuzz`) | 2,158 |
| Test files | 157 |
| CUE schemas (`specs/`) | 25 |

Capabilities with implementation **and** tests:

- read-only corpus scanning, manifest generation, diff, and source-mutation detection;
- canonical node extraction with source spans, certified TOC;
- feed generation and source-backed RAG metadata;
- body ledger (source-body coverage);
- strict fidelity/release gate;
- in-toto attestation generation + evidence envelope (DSSE / cosign-compatible);
- GitHub workflow integration (scoped diffs).

CI gates (`.github/workflows/`): `go vet` and `go test -race` (CLI + control-plane), cross-platform corpus tests (Ubuntu / macOS / Windows), RBOK lawbook and runtime E2E, fidelity proof reports, regulated documentation gate, evidence pack. No formal coverage threshold is defined.

## 3. Scaffold / not implemented at this stage

To be clearly distinguished from the delivered scope:

| Item | Observed state | Evidence |
|---|---|---|
| `adapters/` | Specs and fixtures only; **0 Go files**. No executable adapter. | `git ls-files 'adapters/*.go'` → empty |
| `policies/` | **1 file** (`policies/README.md`). No policy engine. | `git ls-files policies/` |
| `control-plane/` | Thin packages; **no HTTP server or persistence**; not wired into the CLI. | no `ListenAndServe` / `http.Server` symbols |
| Regulated-readiness | **Structural**: skeleton docs, templates, control records. No operational QMS, no certification or validation. | `docs/regulated/`, [public-claim-boundary.md](../public-claim-boundary.md) |
| RAG | **Traceable metadata** only; production vector-store retrieval and LLM behavior not validated. | [public-claim-boundary.md](../public-claim-boundary.md) ("RAG-ready") |
| Domain packs (DOR-xxx) | GitHub issues / specs; **no shipped code**. | [38-domain-opportunity-roadmap.md](../38-domain-opportunity-roadmap.md), `docs/regulated/domain-packs/` |

## 4. Proven vs. not proven

The project defines its own **claim levels** and **reserved phrases** (see [public-claim-boundary.md](../public-claim-boundary.md)). In factual summary:

- **Proven (bounded)**: on a **single** private corpus (`RBOK 01_rbok`, recorded run and commit), the source→feed pipeline produced 3024 source-backed feed units, 0 uncovered body-ledger bytes, strict gate `pass`, 0 blocking semantic findings. Detail and scope: [rbok-poc-validation-dossier.md](../rbok-poc-validation-dossier.md).
- **Not proven (at platform scope)**: universal multi-corpus / multi-format fidelity; absence of semantic warnings on arbitrary corpora; attestation `claim_coverage` wiring; regulated customer validation.
- The phrase `full_fidelity_proven` is **reserved for the recorded POC run** and must not be read as platform-wide proof.

## 5. Known gaps

(Reflects the backlog at measurement time, 2026-05-26; source: [15-product-backlog.md](../15-product-backlog.md).)

- `claim_coverage` not yet wired into the attestation by the CLI (documented limitation).
- Acquisition and licensing review of regulated references still open (RCP epic) → bounds the reference-to-control proof.
- Portability beyond RBOK Markdown: YAML/JSON formats partial; PDF, DOCX, images, and scanned documents not supported.
- Elevation of the RBOK POC proof level still open (AQ epic #314).

## 6. Verify it yourself

```bash
# Command surface (read the dispatcher)
sed -n '19,45p' cli/internal/app/app.go

# Code metrics
git ls-files 'cli/*.go' 'control-plane/*.go' | grep -v _test.go | xargs wc -l | tail -1
git ls-files 'cli/*_test.go' 'control-plane/*_test.go' | xargs wc -l | tail -1
git grep -hE '^func (Test|Benchmark|Fuzz)' -- '*_test.go' | wc -l
git ls-files '*_test.go' | wc -l
git ls-files 'specs/*.cue' | wc -l

# Scaffold layers
git ls-files 'adapters/*.go'   # -> empty
git ls-files policies/         # -> policies/README.md

# Build and run
cd cli && go build -o ../nomos . && cd ..
./nomos help
go -C cli test ./...
```

---

> No valuation in this document. The accounting frameworks and market comparables (with no verdict) are in [valuation-inputs.en.md](valuation-inputs.en.md), for the analyst to apply.

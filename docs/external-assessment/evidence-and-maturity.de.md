# Evidenz- und Reifedossier — Input fuer externe Bewertung

> Sprachen: [FR](evidence-and-maturity.md) · [EN](evidence-and-maturity.en.md) · **DE**

> Dieses Dokument ist ein **neutraler Input** fuer eine unabhaengige externe Bewertung des Zustands des NOMOS-Projekts. Es behauptet **keinen Wert** (monetaer oder strategisch) und zieht **keine Schlussfolgerung** ueber den Wert des Projekts. Es praesentiert ueberpruefbare Fakten: was tatsaechlich implementiert und getestet ist, was nicht, und die bekannten Luecken. Die Analyse zieht ihre eigenen Schluesse.
>
> Der oeffentliche Claim-Contract ist massgeblich: siehe [public-claim-boundary.md](../public-claim-boundary.md). **Bewertungs-Inputs** (buchhalterische Rahmenwerke und Marktvergleiche, ohne Urteil) sind in [valuation-inputs.de.md](valuation-inputs.de.md) isoliert, zur eigenen Anwendung durch die Analyse.

## Wie dieses Dossier zu lesen ist

- Jede Aussage ist an **Evidenz** gebunden: Code, Tests, CI-Konfiguration, ein generiertes Artefakt oder eine **benannte Luecke**.
- Quantitative Metriken wurden am **2026-05-26** auf Commit `0c9e8fa` gemessen (Branch `codex/docs-refresh-business-value`, ausgerichtet an `origin/main`).
- Reproduktionsbefehle sind angegeben (Abschnitt "Selbst ueberpruefen"): nichts hier verlangt Vertrauen auf Zuruf.
- Scope: Dieses Dossier beschreibt den **beobachteten** Zustand. Es praesentiert die Roadmap nicht als Faehigkeit.

## 1. Was NOMOS heute ist

NOMOS ist eine **Go-CLI** (`cli/`, interne Version `0.1.0-ALPHA`, deklariert in `cli/internal/app/app.go`), die einen Corpus von Autoritaetsquellen in nachverfolgte kanonische Artefakte (Knoten, TOC, source-backed Feed/RAG, Body Ledger) umwandelt, Fidelity Gates ausfuehrt und in-toto-artige Attestierungen erzeugt.

Im Dispatcher tatsaechlich registrierte Befehle (`cli/internal/app/app.go`):

| Befehl | Rolle | Status |
|---|---|---|
| `init` | Initialisiert die Manifeste eines Projekts (Modus minimal oder regulated). | implementiert |
| `validate` | Validiert Manifeste und Schemas (CUE/YAML). | implementiert |
| `diagnose` | Inspiziert ein Repository, gibt einen Admission-Vorbericht aus (JSON/Markdown). | implementiert |
| `corpus` | Scan → manifest → validate-sidecar → diff → feed → body-ledger → attest. | implementiert |
| `strict` | Strikter Release-/Integritaets-Gate. | implementiert |
| `github` | GitHub-Workflow-Integration (Planung von Scoped-Diffs). | implementiert |
| `evidence` | Hash, Vorbereiten/Signieren und Verifizieren von Evidence-Bundles. | implementiert |
| `version`, `help` | Trivial. | implementiert |

Der Code enthaelt einen `notImplemented`-Helper (`app.go`), aber **kein aktiver Befehl nutzt ihn**: es gibt keinen Top-Level-Befehls-Stub.

## 2. Implementiert und getestet

Die Kern-Pipeline ist real und durch Tests abgedeckt (Assertions, Golden Fixtures), nicht nur skizziert.

Gemessene Metriken (Commit `0c9e8fa`, 2026-05-26):

| Messung | Wert |
|---|---:|
| Go-Zeilen ohne Tests (`cli/` + `control-plane/`) | 47.511 |
| Go-Testzeilen | 43.030 |
| Testfunktionen (`Test`/`Benchmark`/`Fuzz`) | 2.158 |
| Testdateien | 157 |
| CUE-Schemas (`specs/`) | 25 |

Faehigkeiten mit Implementierung **und** Tests:

- read-only Corpus-Scanning, Manifest-Generierung, Diff und Source-Mutation-Erkennung;
- kanonische Knotenextraktion mit Source Spans, zertifizierte TOC;
- Feed-Generierung und source-backed RAG-Metadaten;
- Body Ledger (Abdeckung des Quelltextkoerpers);
- strikter Fidelity-/Release-Gate;
- in-toto-Attestierung + Evidence-Envelope (DSSE / cosign-kompatibel);
- GitHub-Workflow-Integration (Scoped Diffs).

CI-Gates (`.github/workflows/`): `go vet` und `go test -race` (CLI + control-plane), plattformuebergreifende Corpus-Tests (Ubuntu / macOS / Windows), RBOK Lawbook- und Runtime-E2E, Fidelity-Proof-Reports, Regulated-Documentation-Gate, Evidence Pack. Es ist kein formaler Coverage-Schwellenwert definiert.

## 3. Skelett / in diesem Stadium nicht implementiert

Klar vom gelieferten Scope zu unterscheiden:

| Element | Beobachteter Zustand | Evidenz |
|---|---|---|
| `adapters/` | Nur Specs und Fixtures; **0 Go-Dateien**. Kein ausfuehrbarer Adapter. | `git ls-files 'adapters/*.go'` → leer |
| `policies/` | **1 Datei** (`policies/README.md`). Keine Policy-Engine. | `git ls-files policies/` |
| `control-plane/` | Duenne Packages; **kein HTTP-Server, keine Persistenz**; nicht an die CLI angebunden. | keine `ListenAndServe` / `http.Server` Symbole |
| Regulated-Readiness | **Strukturell**: Skelett-Dokumente, Templates, Control Records. Kein operatives QMS, keine Zertifizierung oder Validierung. | `docs/regulated/`, [public-claim-boundary.md](../public-claim-boundary.md) |
| RAG | Nur **nachverfolgbare Metadaten**; produktives Vector-Store-Retrieval und LLM-Verhalten nicht validiert. | [public-claim-boundary.md](../public-claim-boundary.md) ("RAG-ready") |
| Domain Packs (DOR-xxx) | GitHub-Issues / Specs; **kein ausgelieferter Code**. | [38-domain-opportunity-roadmap.md](../38-domain-opportunity-roadmap.md), `docs/regulated/domain-packs/` |

## 4. Bewiesen vs. nicht bewiesen

Das Projekt definiert eigene **Claim-Level** und **reservierte Phrasen** (siehe [public-claim-boundary.md](../public-claim-boundary.md)). Faktische Zusammenfassung:

- **Bewiesen (begrenzt)**: auf **einem einzigen** privaten Corpus (`RBOK 01_rbok`, aufgezeichneter Run und Commit) erzeugte die Source→Feed-Pipeline 3024 source-backed Feed-Units, 0 nicht abgedeckte Body-Ledger-Bytes, strict Gate `pass`, 0 blockierende semantische Findings. Detail und Scope: [rbok-poc-validation-dossier.md](../rbok-poc-validation-dossier.md).
- **Nicht bewiesen (auf Plattformebene)**: universelle Multi-Corpus-/Multi-Format-Fidelity; Abwesenheit semantischer Warnings auf beliebigen Corpora; Attestation-`claim_coverage`-Verdrahtung; regulierte Kundenvalidierung.
- Die Phrase `full_fidelity_proven` ist **dem aufgezeichneten POC-Run vorbehalten** und darf nicht als plattformweiter Beweis gelesen werden.

## 5. Bekannte Luecken

(Spiegelt das Backlog zum Messzeitpunkt wider, 2026-05-26; Quelle: [15-product-backlog.md](../15-product-backlog.md).)

- `claim_coverage` noch nicht durch die CLI in die Attestation verdrahtet (dokumentierte Einschraenkung).
- Beschaffung und Lizenzpruefung regulierter Referenzen noch offen (RCP-Epic) → begrenzt den Reference-to-Control-Beweis.
- Portabilitaet ueber RBOK-Markdown hinaus: YAML/JSON-Formate teilweise; PDF, DOCX, Bilder und gescannte Dokumente nicht unterstuetzt.
- Anhebung des RBOK-POC-Beweisniveaus noch offen (AQ-Epic #314).

## 6. Selbst ueberpruefen

```bash
# Befehlsoberflaeche (Dispatcher lesen)
sed -n '19,45p' cli/internal/app/app.go

# Code-Metriken
git ls-files 'cli/*.go' 'control-plane/*.go' | grep -v _test.go | xargs wc -l | tail -1
git ls-files 'cli/*_test.go' 'control-plane/*_test.go' | xargs wc -l | tail -1
git grep -hE '^func (Test|Benchmark|Fuzz)' -- '*_test.go' | wc -l
git ls-files '*_test.go' | wc -l
git ls-files 'specs/*.cue' | wc -l

# Skelett-Schichten
git ls-files 'adapters/*.go'   # -> leer
git ls-files policies/         # -> policies/README.md

# Bauen und ausfuehren
cd cli && go build -o ../nomos . && cd ..
./nomos help
go -C cli test ./...
```

---

> Keine Bewertung in diesem Dokument. Die buchhalterischen Rahmenwerke und Marktvergleiche (ohne Urteil) sind in [valuation-inputs.de.md](valuation-inputs.de.md), zur Anwendung durch die Analyse.

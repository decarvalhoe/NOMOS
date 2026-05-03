# Nomos CLI

The Nomos CLI is the executable core of the alpha product. It diagnoses canonical-first projects, processes source corpora, generates evidence artifacts, and runs gates that prevent unsupported product claims from passing silently.

## Version

`v0.1.0-ALPHA` reports:

```bash
nomos version
```

## Implemented Commands

```bash
nomos help
nomos version
nomos init
nomos validate
nomos diagnose
nomos corpus help
```

The `corpus` surface includes scan, manifest, validate-sidecar, diff, feed, attest, profile diagnosis, and profile listing.

## Build

```bash
go build -o ../nomos .
```

## Test

```bash
go test ./...
go vet ./...
```

For full repository validation from the repo root:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\e2e.ps1
```

## Evidence-Critical Areas

Changes under `cli/internal/corpus`, `cli/internal/fidelity`, `cli/internal/compliance`, `cli/internal/attestation`, `cli/internal/strict`, and `cli/internal/validate` can affect release claims. They require tests and documentation updates when behavior or public evidence changes.

## Alpha Boundary

The CLI can generate evidence packs and POC-grade proof artifacts. It does not certify a customer deployment or replace customer validation in regulated environments.

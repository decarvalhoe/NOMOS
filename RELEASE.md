# Nomos v0.1.0-dev Release Readiness

## Gates

| Gate | Status | Details |
|---|---|---|
| `nomos.project.yaml` exists | PASS | Project manifest with id=nomos, verdict=in_scope |
| `go test ./...` (cli) | PASS | 16 packages, all tests green |
| `go vet ./...` (cli) | PASS | No issues |
| `docs/decisions/` exists | PASS | ADR-0001-canonical-first.md |
| `nomos diagnose` (self) | PASS | verdict=pass, in_scope, confidence=high |
| Source manifest | PASS | docs/canonical/source-manifest.yaml (18 sources) |
| Canonical matrix | PASS | docs/canonical/canonical-matrix.yaml (12 units) |
| CI workflow | PASS | .github/workflows/ci.yml (go-test, cue-vet, yaml-lint) |
| Release artifacts | PASS | reports/nomos-report.json, nomos-spdx.json, nomos-cyclonedx.json, nomos-attestation.json |

## Packages Tested

- admit, app, attestation, checks, checks/contracts, detect, diagnose
- exceptions, export, output, partial, productcheck, remediation
- report, strict, validate

## CLI Commands Implemented

- `nomos init` — scaffold project manifests
- `nomos validate` — schema validation
- `nomos diagnose` — admission pre-report
- `nomos sources check` — source integrity
- `nomos contracts check` — contract validation
- `nomos matrix check` — matrix structure
- `nomos product-check` — project manifest checks
- `nomos strict` — aggregated release gate
- `nomos exceptions check` — expiring exceptions
- `nomos report` — generate nomos-report.json
- `nomos export spdx` — SPDX 2.3 SBOM
- `nomos export cyclonedx` — CycloneDX 1.5 BOM
- `nomos attest` — in-toto attestation

## Evidence

- `reports/nomos-report.json` — full detect report
- `reports/nomos-spdx.json` — SPDX 2.3 SBOM
- `reports/nomos-cyclonedx.json` — CycloneDX 1.5 BOM
- `reports/nomos-attestation.json` — signed attestation envelope

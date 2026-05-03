# CI Integration

Nomos gates are intended to run in CI so source drift, evidence gaps, and unsupported claims fail before release.

## Current Repository Workflows

This repository currently uses GitHub Actions for:

- Go vet and test;
- CUE vet;
- YAML lint;
- corpus tests on Linux, macOS, and Windows;
- RBOK lawbook E2E;
- RBOK runtime E2E;
- fidelity proof report generation;
- regulated documentation gate;
- regulated evidence pack.

## Recommended Adoption Pattern

1. Start with fail-open diagnosis on a branch.
2. Fix manifest, source, schema, and evidence gaps.
3. Move critical checks to fail-closed.
4. Require the gate for merge.
5. Preserve generated reports as release evidence.

## Fail-Closed vs Fail-Open

- **Fail-closed** is required for release gates and regulated-readiness evidence.
- **Fail-open** is acceptable during initial adoption when the objective is discovery and backlog creation.

## Customer Boundary

CI evidence is part of a validation story, not the whole story. Regulated customers still need approved intended use, risk assessment, validation protocols, review records, and operational SOPs.

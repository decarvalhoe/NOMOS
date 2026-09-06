# Security Policy

Nomos is currently in alpha. Security reports are handled as high-priority project issues and should not be disclosed publicly until triaged.

## Supported Versions

<!-- supported-versions:begin -->
<!-- GENERATED from CHANGELOG.md by scripts/security_process_gate.py --emit-docs; do not edit by hand, CI fails on drift -->
| Version | Security support |
|---|---|
| `v0.2.0-ALPHA` (2026-09-06) | Best-effort alpha triage |
| `v0.1.0-ALPHA` (2026-05-03) | Superseded; not supported |
| earlier | Not supported |
<!-- supported-versions:end -->

## Reporting A Vulnerability

Report suspected vulnerabilities through the private project maintainers or the repository security advisory process when available.

Include:

- affected commit, tag, or release;
- reproduction steps;
- expected impact;
- whether source corpus integrity, generated evidence, credentials, CI, or release artifacts are affected;
- any relevant logs with secrets removed.

Dependency vulnerability scanning in CI, an expiring allowlist and Dependabot are planned as NRT-025 (#678); until then there is no automated scan — say so, do not assume it.

## Security Scope

Security-sensitive areas include:

- source corpus read-only guarantees;
- artifact and attestation integrity;
- GitHub Actions permissions;
- token handling;
- generated evidence paths;
- RAG metadata provenance;
- regulated evidence records;
- any future customer deployment or control-plane endpoint.

## Current Alpha Boundary

Nomos v0.2.0-ALPHA is not a hosted security boundary and does not claim production security certification. Customer deployments must perform their own threat modeling, access-control design, secret management, logging, backup, vulnerability management, and validation.

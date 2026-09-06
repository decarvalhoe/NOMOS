# Security Policy

Nomos is currently in alpha. Security reports are handled as high-priority project issues and should not be disclosed publicly until triaged.

## Supported Versions

<!-- supported-versions:begin -->
<!-- GENERATED from CHANGELOG.md by scripts/security_process_gate.py --write (source: changelog until the support model of NRT-026 exists); do not edit by hand, CI fails on drift -->

| Version | Released | Security support |
|---|---|---|
| `v0.2.0-ALPHA` | 2026-09-06 | Supported — best-effort alpha triage (current release) |
| `v0.1.0-ALPHA` | 2026-05-03 | Superseded — not supported |
| older than `v0.1.0-ALPHA` | — | Not supported |

<!-- supported-versions:end -->

## Reporting A Vulnerability

Report suspected vulnerabilities through the private project maintainers or the repository security advisory process when available.

Include:

- affected commit, tag, or release;
- reproduction steps;
- expected impact;
- whether source corpus integrity, generated evidence, credentials, CI, or release artifacts are affected;
- any relevant logs with secrets removed.

The security process is executable (NRT-025, #678): `docs/security/security-process.yaml` declares intake, triage targets (declared, not measured), disclosure and scanners; `scripts/security_process_gate.py` runs `govulncheck` on `cli/` and `tools/sigstore-verifier/` and `pip-audit` on the pinned sidecar requirements in CI, and any accepted finding lives in `docs/security/vulnerability-allowlist.yaml` with an owner and an expiry. Dependabot covers Go modules, GitHub Actions and Python. This proves that dependencies are scanned and that exceptions expire; it is not a security certification. See `docs/security/README.md`.

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

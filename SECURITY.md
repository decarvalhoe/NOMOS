# Security Policy

Nomos is currently in alpha. Security reports are handled as high-priority project issues and should not be disclosed publicly until triaged.

## Supported Versions

<!-- supported-versions:begin -->
<!-- GENERATED from docs/support-model.yaml by scripts/security_process_gate.py --write; do not edit by hand, CI fails on drift -->

| Version | Released | State | Security support |
|---|---|---|---|
| `v0.2.0-ALPHA` | 2026-09-06 | supported | best-effort alpha triage (current release) |
| `v0.1.0-ALPHA` | 2026-05-03 | superseded | none — superseded by v0.2.0-ALPHA |

<!-- supported-versions:end -->

## Support

<!-- support-model:begin -->
<!-- GENERATED from docs/support-model.yaml by scripts/support_model_guard.py --write; do not edit by hand, CI fails on drift -->

| Version | Released | State | Security support | End of support |
|---|---|---|---|---|
| `v0.2.0-ALPHA` | 2026-09-06 | supported | best-effort alpha triage (current release) | until the next tagged release |
| `v0.1.0-ALPHA` | 2026-05-03 | superseded | none — superseded by v0.2.0-ALPHA | 2026-09-06 |

- Current candidate: `v0.2.0-ALPHA` (the CLI `Version` constant).
- Channels: github_issues — https://github.com/decarvalhoe/NOMOS/issues (bugs, questions, integration); github_private_advisory — https://github.com/decarvalhoe/NOMOS/security/advisories/new (vulnerabilities (docs/security/security-process.yaml)); support_guide — SUPPORT.md (what alpha support covers and what requires project-specific work).
- Response targets (declared, not, measured): github_issues — first response within 10 days; github_private_advisory — per docs/security/security-process.yaml.
- Tested platforms (CI matrix): ubuntu-latest, macos-latest, windows-latest.
- Toolchain: Go 1.24.1 (language) / go1.26.6 (toolchain) from cli/go.mod; CUE v0.16.1; Python 3.12.
- Not supported: hosted service (Nomos is a CLI and an evidence toolchain; no hosted endpoint exists or is operated.); control plane (archived by ADR-0006 and decided by ADR-0007 — `nomos portfolio projects` is a view over committed files, not a production control plane.); GitHub App (readiness boundary only (docs/32-github-app-readiness-boundary.md); no app is published or operated.); production deployment (customer-owned (docs/regulated/customer-integration); the alpha proves the method, not a deployment.); regulated validation package approval (regulated lane, human and external acts (docs/28-regulated-compliance-closure-plan.md).).
- End of support: An alpha version is supported until the next tagged release; only the newest tag receives security triage. No version outside this list is supported, and no version becomes supported by being listed here without a tag.
<!-- support-model:end -->

## Reporting A Vulnerability

Report suspected vulnerabilities through the private project maintainers or the repository security advisory process when available.

Include:

- affected commit, tag, or release;
- reproduction steps;
- expected impact;
- whether source corpus integrity, generated evidence, credentials, CI, or release artifacts are affected;
- any relevant logs with secrets removed.

The security process is executable (NRT-025, #678): `docs/security/security-process.yaml` declares intake, triage targets (declared, not measured), disclosure and scanners; `scripts/security_process_gate.py` runs `govulncheck` on `cli/` and `tools/sigstore-verifier/` and `pip-audit` on the pinned sidecar requirements in CI, and any accepted finding lives in `docs/security/vulnerability-allowlist.yaml` with an owner and an expiry. Dependabot covers Go modules, GitHub Actions, Python and the node adapter fixture. The gate also enumerates every dependency manifest tracked by git (#696) and requires each one to be scanned, watched by Dependabot, or excluded by name with a reason: a forgotten manifest is red, not invisible. This proves that dependencies are scanned, that every manifest is accounted for, and that exceptions expire; it is not a security certification. See `docs/security/README.md`.

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

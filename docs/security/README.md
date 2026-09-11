# Security Process (Executable)

NRT-025 (#678). The security process is data the gate reads, not prose:

| File | Role |
|---|---|
| `security-process.yaml` | `nomos-security-process-v1`: intake channels, triage targets (declared, not measured), disclosure rule, supported-versions source, scanners, Dependabot, allowlist policy, SOP references. |
| `vulnerability-allowlist.yaml` | `nomos-vulnerability-allowlist-v1`: accepted findings, each with id, ecosystem, package, justification, owner, `accepted_on`, `expires_on`. Undated or expired entries turn the gate red. |
| `../../SECURITY.md` | Public policy; its "Supported Versions" section is generated between markers from `CHANGELOG.md` (from the support model of NRT-026 once it exists). |
| `../../.github/dependabot.yml` | Version updates for Go modules (`cli/`, `tools/sigstore-verifier/`), GitHub Actions and the pinned Python sidecar requirements. |
| `../../scripts/requirements-sidecar.txt` | The declared, pinned Python dependencies of the sidecars — what CI installs and what `pip-audit` audits. |

```bash
python scripts/security_process_gate.py --root . --check                         # static checks
python scripts/security_process_gate.py --root . --check --scan govulncheck,pip-audit   # what CI runs
python scripts/security_process_gate.py --root . --write                         # regenerate the Supported Versions section
```

`govulncheck` runs at symbol level on every declared Go module: a vulnerability
the code calls (standard library included) is red unless an unexpired allowlist
entry names it; imported-only and required-only findings are reported in the
verdict, never hidden. `pip-audit --no-deps` audits the exact pins. A requested
scanner that is missing is a failed check.

Manifest coverage (#696): the gate enumerates every dependency manifest tracked
by git (`package.json`, `pyproject.toml`, `go.mod`, `Cargo.toml`, `Gemfile`,
`pom.xml`, `requirements*.txt`) and requires each one to be in a scanner scope,
in a Dependabot directory, or listed under `manifests.not_scanned` of
`security-process.yaml` with a reason. A forgotten manifest is red; an exclusion
whose file is gone, or whose reason is empty, is red; the verdict names how each
manifest is covered. Learned on 2026-09-06: three clean scans while GitHub held
8 advisories on a fixture nobody watched — a manifest nobody scans is a
manifest nobody sees.

Claim boundary: "dependencies are scanned in CI and accepted findings expire".
Not "secure", not "certified", not a statement about any deployment. The
regulated SOP that owns vulnerability and incident handling is
`docs/regulated/security-privacy/vulnerability-and-incident-management-sop.md`;
the gate is a supporting tool (`technically_verified`, `supporting_use_until_validated`
in `docs/roadmap-lanes.yaml`), never the process itself.

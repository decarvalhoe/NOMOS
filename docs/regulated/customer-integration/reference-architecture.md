# Regulated Reference Architecture: Nomos + RBOK + Praxis

> Scope: Demonstrates how Nomos canonical intelligence, RBOK lawbook corpus,
> and Praxis evidence production integrate in a regulated (GxP/Part 11) context.

## Component Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        REGULATED ENVIRONMENT                            │
│                                                                         │
│  ┌──────────────┐    ┌──────────────────┐    ┌─────────────────────┐   │
│  │   RBOK       │    │      NOMOS       │    │      PRAXIS         │   │
│  │  Lawbook     │    │   Control Plane  │    │  Evidence Engine    │   │
│  │              │    │                  │    │                     │   │
│  │ ┌──────────┐ │    │ ┌──────────────┐ │    │ ┌───────────────┐  │   │
│  │ │ Corpus   │─┼───►│ │ Admission    │ │    │ │ Test Runner   │  │   │
│  │ │ Parser   │ │    │ │ Gate         │ │    │ │               │  │   │
│  │ └──────────┘ │    │ └──────┬───────┘ │    │ └───────┬───────┘  │   │
│  │ ┌──────────┐ │    │        │         │    │         │          │   │
│  │ │ Feed     │─┼───►│ ┌──────▼───────┐ │    │ ┌───────▼───────┐  │   │
│  │ │ Assembly │ │    │ │ Strict Gate  │─┼───►│ │ Evidence      │  │   │
│  │ └──────────┘ │    │ └──────┬───────┘ │    │ │ Collector     │  │   │
│  │ ┌──────────┐ │    │        │         │    │ └───────┬───────┘  │   │
│  │ │ Lockfile │─┼───►│ ┌──────▼───────┐ │    │         │          │   │
│  │ │ Guard    │ │    │ │ Report Gen   │ │    │ ┌───────▼───────┐  │   │
│  │ └──────────┘ │    │ └──────┬───────┘ │    │ │ Attestation   │  │   │
│  │ ┌──────────┐ │    │        │         │    │ │ Signer        │  │   │
│  │ │ Govern-  │ │    │ ┌──────▼───────┐ │    │ └───────┬───────┘  │   │
│  │ │ ance     │─┼───►│ │ Approval    │◄┼────┤         │          │   │
│  │ │ Checker  │ │    │ │ Workflow    │ │    │ ┌───────▼───────┐  │   │
│  │ └──────────┘ │    │ └─────────────┘ │    │ │ Evidence      │  │   │
│  └──────────────┘    └──────────────────┘    │ │ Pack          │  │   │
│                                              │ └───────────────┘  │   │
│                                              └─────────────────────┘   │
│                                                                         │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                    SHARED INFRASTRUCTURE                          │   │
│  │  ┌────────────┐  ┌──────────────┐  ┌─────────────────────────┐  │   │
│  │  │ Git Repo   │  │ CI/CD        │  │ Artifact Store          │  │   │
│  │  │ (source of │  │ (GitHub      │  │ (evidence packs,        │  │   │
│  │  │  truth)    │  │  Actions)    │  │  attestations, SBOMs)   │  │   │
│  │  └────────────┘  └──────────────┘  └─────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────┘
```

## Evidence Flow

```
 RBOK Lawbook           Nomos                    Praxis
 ────────────           ─────                    ──────

 1. Author commits      2. CI triggers
    regulated corpus       corpus-scan workflow
         │                      │
         ▼                      ▼
 3. Lockfile guard      4. Governance check
    (hash verified)        (version/owner/
         │                  status/domain)
         ▼                      │
 5. Feed assembly       ◄───────┘
    (nodes + hashes)
         │
         ▼
 6. Admission gate ─────────────────────────────► 7. Test execution
    (ref alignment,                                  (golden tests,
     control matrix)                                  characterization)
         │                                                │
         ▼                                                ▼
 8. Strict gate ◄──────────────────────────────── 9. Evidence collection
    (cross-checks,                                   (coverage, logs,
     matrix validation)                               screenshots)
         │                                                │
         ▼                                                ▼
10. Report generation                            11. Attestation signing
    (nomos-report.json)                              (SLSA provenance,
         │                                            e-signatures)
         ▼                                                │
12. Approval workflow ◄───────────────────────── 13. Evidence pack
    (author → reviewer                               assembly
     → approver)                                      │
         │                                            ▼
         ▼                                      14. Artifact upload
15. Release decision                                (immutable store)
    (publish / block)
```

## Integration Points

### RBOK Lawbook → Nomos

| Integration Point | Data | Protocol | Gate |
|-------------------|------|----------|------|
| Corpus feed | `rbok_nodes[]` with hashes | JSON/YAML via Git | Lockfile guard |
| Governance metadata | version, owner, status, domain | YAML fields | corpus_partial check |
| Reference register | external refs with evidence_status | YAML register | ref_alignment gate |
| Node provenance | source_path + source_hash | SHA-256 | Data integrity control |

### Nomos → Praxis

| Integration Point | Data | Protocol | Gate |
|-------------------|------|----------|------|
| Test trigger | strict gate findings | CI workflow dispatch | Blocking findings |
| Evidence request | required evidence artifacts | Report schema | evidence_required flag |
| Approval input | nomos-report.json | JSON file | Verdict pass/blocked |
| Release gate | approval_record with signatures | Approval API | Policy check |

### Praxis → Nomos

| Integration Point | Data | Protocol | Gate |
|-------------------|------|----------|------|
| Test results | coverage + pass/fail per check | CI artifacts | Evidence collection |
| Attestation | SLSA provenance + e-signatures | In-toto format | Attestation validation |
| Evidence pack | bundled artifacts + manifest | Zip + manifest.json | Pack integrity check |
| Human review | approval decision + signer chain | Approval record | Part 11 compliance |

## Part 11 Compliance Points

The reference architecture addresses 21 CFR Part 11 requirements at each layer:

| Part 11 Requirement | RBOK | Nomos | Praxis |
|---------------------|------|-------|--------|
| **11.10(a) Validation** | Corpus lockfile validates source integrity | Strict gate validates all cross-checks | Test runner produces validation evidence |
| **11.10(b) Copies** | True-copy export (hash-verified) | Report generation (deterministic JSON) | Evidence pack (immutable artifact) |
| **11.10(c) Record protection** | Git-signed commits + lockfile | Approval workflow (no bypass) | Artifact store (append-only) |
| **11.10(d) Access control** | CODEOWNERS + branch protection | Role-based approval (author/reviewer/approver) | CI secrets + environment protection |
| **11.10(e) Audit trail** | Git log + governance checker | Approval hash-chain + timestamps | CI run logs + attestation provenance |
| **11.10(g) Authority checks** | Lockfile approval (reviewer required) | Policy enforcement (no self-approve) | Deployment environment (required reviewers) |
| **11.10(k) Documentation** | Regulated doc set (SOPs, VMP) | Control matrix + self-compliance report | Evidence index + retention policy |

## Deployment Topology

```
┌─────────────────────────────────────────┐
│            GitHub Organization           │
│                                          │
│  repo: rbok-lawbook (corpus source)      │
│    └─ .github/workflows/corpus-scan.yml  │
│                                          │
│  repo: nomos (control plane + CLI)       │
│    └─ .github/workflows/ci.yml           │
│    └─ .github/workflows/strict-gate.yml  │
│                                          │
│  repo: praxis (evidence engine)          │
│    └─ .github/workflows/evidence.yml     │
│    └─ .github/workflows/release.yml      │
│                                          │
│  environment: regulated-release          │
│    └─ required reviewers: quality_unit   │
│    └─ deployment branch: main            │
│                                          │
│  artifact store: GitHub Actions artifacts │
│    └─ retention: 90 days (configurable)  │
│    └─ export: required for long-term     │
└─────────────────────────────────────────┘
```

## Sequence: Regulated Release

1. **Author** commits corpus changes to rbok-lawbook
2. **CI** runs `corpus-scan` workflow → lockfile guard + governance check
3. **Nomos** admission gate evaluates ref alignment and control matrix
4. **Nomos** strict gate runs cross-checks (source ↔ matrix ↔ contracts)
5. **Praxis** executes test suite triggered by strict gate pass
6. **Praxis** collects evidence (coverage, logs, attestation)
7. **Praxis** signs attestation (SLSA provenance)
8. **Nomos** generates `nomos-report.json` with verdict
9. **Author** signs approval record (role: author)
10. **Reviewer** reviews and signs (role: reviewer)
11. **Approver** final sign-off (role: approver, self-approve blocked)
12. **CI** verifies approval policy (3 sigs, chain valid, no self-approve)
13. **Release** deploys to `regulated-release` environment

## Failure Modes

| Failure | Detection Point | Effect | Recovery |
|---------|-----------------|--------|----------|
| Unapproved corpus hash | Lockfile guard | Block admission | Re-approve via lockfile add |
| Missing governance field | Corpus governance check | corpus_partial finding | Add version/owner/status/domain |
| Ungoverned external ref | Ref alignment gate | Gate fail | Add to reference register |
| Strict check failure | Strict gate | Block release | Fix source/matrix/contract gap |
| Evidence missing | Praxis collector | Block approval | Run tests, collect evidence |
| Self-approval attempt | Approval policy | Reject signature | Different person approves |
| Chain tampered | ValidateChain | Block release | Re-sign from last valid point |
| Stale reference review | Ref alignment | Warning | Re-review and update checked_on |

## Configuration

Minimal configuration to activate regulated mode:

```yaml
# nomos.project.yaml
compliance:
  regulated: true
  data_sensitivity: internal
  attestation_level: signed
evidence:
  required_reports:
    - nomos-report.json
    - coverage-report.md
  attestation_level: signed
  approval_policy: regulated  # requires author + reviewer + approver
```

# Nomos Regulated Self-Compliance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Nomos defensible as a regulated-grade canonical compliance tool by proving Nomos-on-Nomos compliance and integrating Praxis as the runtime evidence/CAPA counterpart.

**Architecture:** Nomos owns source authority, canonical units, control matrices, provenance, attestations, and read-only corpus conversion. Praxis owns runtime product evidence, invariants, validated coverage, and CAPA reporting. A shared evidence contract links Nomos controls and corpus claims to Praxis scenarios, runtime observations, findings, and corrective/preventive actions.

**Tech Stack:** Go CLI, CUE schemas, JSON artifacts, GitHub Actions, in-toto/SLSA provenance, Praxis Python project packs and CAPA audit pipeline.

---

## File Structure

- Create `docs/21-regulated-quality-reference.md`: regulated quality reference and hard audit baseline.
- Create `specs/regulated-control-matrix.cue`: schema for external references, controls, evidence, waivers, and claims.
- Create `specs/examples/regulated-control-matrix.nomos.yaml`: Nomos self-compliance example matrix.
- Create `specs/nomos-praxis-evidence.schema.json`: shared evidence contract between Nomos and Praxis.
- Create `cli/internal/compliance/`: Go package for regulated control matrix loading and validation.
- Modify `cli/internal/app/`: add `nomos compliance self-check`, `nomos compliance references`, and `nomos compliance export`.
- Create `.github/workflows/nomos-self-compliance.yml`: CI gate for Nomos-on-Nomos compliance.
- Create `docs/validation/`: intended use, risk assessment, validation protocol, deviation log, and validation summary templates.
- Create or update Praxis project pack in `RBOKproject/praxis`: `examples/nomos/` plus adapter/tests so Praxis can audit Nomos.

## Task 1: Repair Baseline Before Any Regulated Claim

**Files:**
- Modify: `cli/internal/corpus/rbok_feed_assembly.go`
- Modify: `specs/rbok-lawbook-feed.cue`
- Test: existing Go/CUE/CI workflows

- [ ] **Step 1: Confirm current failure**

Run:

```powershell
cd C:\Dev\nomos-viability-audit\cli
go test ./...
```

Expected until fixed: FAIL with `buildRAGMetadata` redeclared or wrong signature.

- [ ] **Step 2: Fix the compile break**

Rename the RBOK-specific helper or route it through the existing feed helper with a single signature. The code must compile without build tags or stubs.

- [ ] **Step 3: Fix CUE validation**

Run:

```powershell
cd C:\Dev\nomos-viability-audit
cue vet specs/rbok-lawbook-feed.cue
```

Expected after fix: PASS.

- [ ] **Step 4: Verify**

Run:

```powershell
cd C:\Dev\nomos-viability-audit\cli
go test ./...
go vet ./...
```

Expected: PASS on Linux CI. Windows-specific CGO issues may be tracked separately, but corpus and compliance packages must have Windows smoke coverage.

## Task 2: Add Regulated Control Matrix Schema

**Files:**
- Create: `specs/regulated-control-matrix.cue`
- Create: `specs/examples/regulated-control-matrix.nomos.yaml`
- Test: `cue vet specs/regulated-control-matrix.cue specs/examples/regulated-control-matrix.nomos.yaml`

- [ ] **Step 1: Define required schema concepts**

The CUE schema must require:

```yaml
references:
  - id
  - name
  - url
  - owner
  - applicability
controls:
  - id
  - reference_ids
  - requirement
  - implementation_refs
  - test_refs
  - evidence_refs
  - severity
  - status
waivers:
  - id
  - control_id
  - expires_at
  - owner
  - risk
  - mitigation
claims:
  - id
  - text
  - allowed_when
  - evidence_refs
```

- [ ] **Step 2: Encode allowed enums**

Use:

```text
severity: critical | major | minor | observation
status: planned | implemented | verified | waived | blocked
applicability: active_control | tool_candidate | architecture_pattern | private_link | future_endpoint
```

- [ ] **Step 3: Add Nomos example**

The example must include at least FDA Part 11, FDA ALCOA/data integrity, EudraLex Annex 11, NASA software assurance, NIST SSDF, NIST 800-53, SLSA, in-toto, W3C PROV-O, NIST AI RMF, OWASP LLM Top 10, and all existing Nomos doc references.

- [ ] **Step 4: Verify**

Run:

```powershell
cd C:\Dev\nomos-viability-audit
cue vet specs/regulated-control-matrix.cue specs/examples/regulated-control-matrix.nomos.yaml
```

Expected: PASS.

## Task 3: Implement External Reference Alignment Gate

**Files:**
- Create: `cli/internal/compliance/references.go`
- Create: `cli/internal/compliance/references_test.go`
- Modify: `cli/internal/app/*`
- Test: `go test ./internal/compliance ./internal/app`

- [ ] **Step 1: Write failing tests**

Test cases:

```text
1. placeholder URLs fail unless applicability=future_endpoint and owner is set.
2. unresolved active_control URLs fail.
3. private GitHub URLs are accepted only when applicability=private_link and github evidence exists.
4. every active reference has at least one mapped control.
5. every control has evidence or an expiring waiver.
```

- [ ] **Step 2: Implement URL extraction and validation**

Parse Markdown/YAML/CUE/JSON files for `https?://...`, normalize duplicates, and compare against `regulated-control-matrix`.

- [ ] **Step 3: Wire CLI**

Add:

```text
nomos compliance references --root . --matrix specs/examples/regulated-control-matrix.nomos.yaml --output reports/reference-alignment.json
```

- [ ] **Step 4: Verify**

Run:

```powershell
cd C:\Dev\nomos-viability-audit\cli
go test ./internal/compliance ./internal/app -run Compliance -v
```

Expected: PASS and failures for ungoverned `example.com` placeholders.

## Task 4: Implement Nomos Self-Compliance Profile

**Files:**
- Modify: `nomos.project.yaml`
- Create: `docs/validation/intended-use.md`
- Create: `docs/validation/risk-assessment.md`
- Create: `docs/validation/validation-protocol.md`
- Create: `docs/validation/deviation-log.md`
- Create: `docs/validation/validation-summary.md`
- Modify: `cli/internal/app/*`

- [ ] **Step 1: Define `mode: nomos_product`**

Nomos must be admissible as its own product, with explicit intended use and risk profile.

- [ ] **Step 2: Add self-check command**

Add:

```text
nomos compliance self-check --root . --matrix specs/examples/regulated-control-matrix.nomos.yaml --output reports/nomos-self-compliance.json
```

- [ ] **Step 3: Emit ALCOA metadata**

Every self-check artifact must include:

```json
{
  "actor": "...",
  "tool": "nomos",
  "command": "...",
  "timestamp": "...",
  "repo": "...",
  "commit": "...",
  "source_hashes": [],
  "artifact_hash": "..."
}
```

- [ ] **Step 4: Verify**

Run:

```powershell
cd C:\Dev\nomos-viability-audit
.\cli\nomos.exe compliance self-check --root . --matrix specs/examples/regulated-control-matrix.nomos.yaml --output reports/nomos-self-compliance.json
git status --porcelain
```

Expected: output outside protected source paths and no unexpected source mutation.

## Task 5: Define Nomos-Praxis Evidence Contract

**Files:**
- Create: `specs/nomos-praxis-evidence.schema.json`
- Create: `specs/examples/nomos-praxis-evidence.valid.json`
- Create: `specs/examples/nomos-praxis-evidence.invalid.json`

- [ ] **Step 1: Define shared IDs**

Required:

```text
control_id, source_id, source_hash, unit_id, claim_id, artifact_hash,
test_id, scenario_id, finding_id, capa_id, release_id
```

- [ ] **Step 2: Define shared verdicts**

Required:

```text
pass, warn, fail, blocked, not_applicable, waived_until
```

- [ ] **Step 3: Validate JSON Schema**

Run:

```powershell
cd C:\Dev\nomos-viability-audit
npx ajv validate -s specs/nomos-praxis-evidence.schema.json -d specs/examples/nomos-praxis-evidence.valid.json
```

Expected: valid example passes and invalid example fails.

## Task 6: Add Praxis Nomos Project Pack

**Repository:** `C:\Dev\praxis`

**Files:**
- Create: `examples/nomos/pack.yaml`
- Create: `examples/nomos/praxis.yaml`
- Create: `examples/nomos/features/compliance.yaml`
- Create: `examples/nomos/uat/self-compliance.yaml`
- Create: `examples/nomos/RUNBOOK.md`
- Create: `praxis/projects/nomos.py`
- Test: `tests/test_project_pack_nomos.py`

- [ ] **Step 1: Scaffold pack**

Run:

```powershell
cd C:\Dev\praxis
praxis init nomos --with-adapter
```

- [ ] **Step 2: Add feature catalog**

Features must include:

```text
Self Compliance
External Reference Alignment
Regulated Control Matrix
ALCOA Evidence
Nomos-Praxis Evidence Contract
Release Gate
```

- [ ] **Step 3: Add UAT scenarios**

Scenarios must run the built Nomos CLI against the Nomos repo and assert that compliance artifacts exist and validate.

- [ ] **Step 4: Add custom invariants**

At minimum:

```text
no_source_mutation
no_unmapped_external_reference
no_missing_alcoa_metadata
no_unwaived_critical_control
```

- [ ] **Step 5: Verify**

Run:

```powershell
cd C:\Dev\praxis
pytest tests/test_project_pack_nomos.py -v
```

Expected: PASS.

## Task 6A: Record Praxis Regulatory Parity Boundary

**Files:**
- Modify: `docs/21-regulated-quality-reference.md`
- Modify in Praxis repo: `docs/ARCHITECTURE.md` or a new `docs/REGULATED_PARITY.md`
- Track: `RBOKproject/praxis` issue for deferred Praxis self-compliance parity

- [ ] **Step 1: State the boundary**

Document that Praxis is outside the immediate Nomos implementation scope while it is only an exploratory/support testing tool.

- [ ] **Step 2: State the escalation rule**

Document that Praxis enters the same regulated quality boundary as Nomos when its output is used for release go/no-go, CAPA, validation evidence, audit response evidence, or product-law conformance evidence.

- [ ] **Step 3: Define required parity controls**

Praxis parity must include:

```text
intended use
risk assessment
requirements traceability
ALCOA+ evidence metadata
audit trail
source/build/test provenance
validated project packs and invariants
waiver/deviation control
claims governance
Praxis-on-Praxis self-compliance
```

- [ ] **Step 4: Verify tracking**

Expected: the Nomos regulated epic links to a Praxis deferred parity issue, and the Praxis compatibility epic records that parity is not optional once Praxis evidence becomes regulated evidence.

## Task 7: Add CI Gates

**Files:**
- Create: `.github/workflows/nomos-self-compliance.yml`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Build Nomos CLI in CI**

The workflow must build the public CLI binary first.

- [ ] **Step 2: Run self-compliance**

The workflow must run:

```bash
nomos compliance references --root . --matrix specs/examples/regulated-control-matrix.nomos.yaml --output reports/reference-alignment.json
nomos compliance self-check --root . --matrix specs/examples/regulated-control-matrix.nomos.yaml --output reports/nomos-self-compliance.json
```

- [ ] **Step 3: Upload artifacts**

Upload:

```text
reports/reference-alignment.json
reports/nomos-self-compliance.json
reports/nomos-control-matrix.json
reports/nomos-alcoa-report.json
reports/release-go-no-go.md
```

- [ ] **Step 4: Fail on invalid claims**

The workflow must fail if README or docs contain regulated-grade claims whose evidence level is lower than the claim requires.

## Task 8: Add Release Claim Governance

**Files:**
- Create: `docs/claims/regulated-claims.yaml`
- Create: `cli/internal/compliance/claims.go`
- Create: `cli/internal/compliance/claims_test.go`

- [ ] **Step 1: Create claim registry**

Every public claim must include:

```yaml
id: CLAIM-NOMOS-001
text: "Nomos transforms authoritative business sources into executable product law."
allowed_when:
  - control_id: RQ-02
    min_status: verified
  - control_id: RQ-05
    min_status: verified
evidence_refs: []
```

- [ ] **Step 2: Add scanner**

The scanner must find claim phrases in README/docs/release notes and compare them to the registry.

- [ ] **Step 3: Verify**

Run:

```powershell
cd C:\Dev\nomos-viability-audit\cli
go test ./internal/compliance -run Claim -v
```

Expected: PASS.

## Task 9: Close The Loop With RBOK Lawbook

**Files:**
- Modify existing RBOK lawbook E2E workflow and docs.

- [ ] **Step 1: Ensure RBOK E2E depends on self-compliance**

RBOK `01_rbok` corpus conversion must not run as a credibility claim unless Nomos self-compliance is green.

- [ ] **Step 2: Link Praxis runtime evidence**

When RBOK Engine consumes Nomos lawbook artifacts, Praxis must validate visible product behavior against those artifacts.

- [ ] **Step 3: Verify**

Expected release evidence:

```text
Nomos self-compliance: pass
RBOK lawbook feed: pass
RBOK read-only proof: pass
Praxis RBOK runtime evidence: pass or finding-linked CAPA
Release go/no-go: pass
```

## Task 10: Release Criteria

- [ ] `main` green.
- [ ] Nomos self-compliance workflow green.
- [ ] External reference alignment has zero unmapped active references.
- [ ] No unresolved placeholder URLs in active docs.
- [ ] Praxis Nomos pack passes.
- [ ] Praxis regulatory parity boundary is documented and tracked outside this immediate Nomos scope.
- [ ] RBOK lawbook E2E passes on real `01_rbok`.
- [ ] All critical/major findings closed or waived with expiry.
- [ ] Release notes state the exact quality level: NQ-3, NQ-4, NQ-5, or NQ-6.

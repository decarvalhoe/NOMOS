package compliance

// #639 — the proof is the refusal. Every rule of AssembleCandidate has a test
// that breaks exactly it and asserts the code; then a valid candidate is
// assembled, zipped, and each way of tampering with it turns verification red.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const goodSpec = `schema_version: nomos-release-candidate-spec-v1
product: nomos
version: v9.9.9-TEST
target_level: NQ-1
approval_status: pending
approvals: []
release_executed: false
evidence_ledger: ledger.yaml
gaps_acknowledged: [GAP-A, GAP-B]
risks:
  - id: RISK-CI
    kind: repeated_ci_evidence
    source: ci-index.json
gates:
  required: [g1, g2]
waivers: [waiver.yaml]
deviations:
  - id: DEV-1
    status: open
    record: deviation.md
`

const goodLedger = `schema_version: "0.1.0"
blocking_gaps:
  - id: GAP-A
    severity: major
    status: open
    blocks_claims: [x]
  - id: GAP-B
    severity: major
    status: open
  - id: GAP-CLOSED
    severity: minor
    status: closed
`

const goodWaiver = `schema_version: w1
document_id: W-1
status: draft
waived_records: []
`

func writeCandidateFixture(t *testing.T, mutate func(files map[string]string)) (repo, spec, gates string) {
	t.Helper()
	repo = t.TempDir()
	files := map[string]string{
		"ledger.yaml":       goodLedger,
		"waiver.yaml":       goodWaiver,
		"deviation.md":      "# DEV-1\n\nrecord\n",
		"ci-index.json":     `{"schema_version":"idx","published_on":"2026-09-04","measurement":{"consecutive_green_runs":4,"target_consecutive_green_runs":8,"runs_remaining_to_target":4,"claim_unlocked":false}}`,
		"nomos-report.json": `{"ok":true}`,
		"docs/regulated/evidence-index/evidence-ledger.yaml": goodLedger,
		"docs/canonical/source-manifest.yaml":                "sources: []\n",
		"docs/regulated/product-profiles/nomos.yaml":         "schema_version: \"0.2.0\"\n",
		"spec.yaml":  goodSpec,
		"gates.json": `{"schema_version":"nomos-release-candidate-gates-v1","commit":"abc123","measured_at":"2026-09-05T00:00:00Z","gates":[{"id":"g1","command":"x","exit_code":0,"status":"pass"},{"id":"g2","command":"y","exit_code":0,"status":"pass"}]}`,
	}
	if mutate != nil {
		mutate(files)
	}
	for rel, content := range files {
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo, filepath.Join(repo, "spec.yaml"), filepath.Join(repo, "gates.json")
}

func assemble(t *testing.T, repo, spec, gates string) (CandidateManifest, error) {
	t.Helper()
	return AssembleCandidate(CandidateInput{SpecPath: spec, GatesPath: gates, RepoRoot: repo, Commit: "abc123",
		GeneratedBy: "test", Now: time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)})
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var ce *CandidateError
	if !errors.As(err, &ce) {
		t.Fatalf("want refusal %s, got %v", code, err)
	}
	if ce.Code != code {
		t.Fatalf("want refusal %s, got %s (%s)", code, ce.Code, ce.Message)
	}
}

func wantMessage(t *testing.T, err error, code, fragment string) {
	t.Helper()
	wantCode(t, err, code)
	if !strings.Contains(err.Error(), fragment) {
		t.Fatalf("refusal %s must name %q, got %q", code, fragment, err.Error())
	}
}

// Each content check carries its own diagnostic; a mutation that drops one
// must be caught by the message, not masked by a neighbouring check.
func TestArtifactContentChecksAreDistinct(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(body), 0o644)
		return p
	}
	cases := []struct{ name, body, want string }{
		{"empty.json", "  \n", "file is empty"},
		{"list.yaml", "- a\n- b\n", "not a mapping"},
		{"broken.yaml", "a: [\n", "not valid YAML"},
		{"broken.json", "{", "not valid JSON"},
		{"noheading.md", "just prose\n", "no top-level heading"},
	}
	for _, tc := range cases {
		err := checkArtifactContent(write(tc.name, tc.body))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want error naming %q, got %v", tc.name, tc.want, err)
		}
	}
	for _, ok := range []struct{ name, body string }{{"ok.yaml", "a: 1\n"}, {"ok.json", "{}"}, {"ok.md", "# Title\n"}, {"ok2.md", "intro\n\n# Title\n"}} {
		if err := checkArtifactContent(write(ok.name, ok.body)); err != nil {
			t.Errorf("%s: unexpected %v", ok.name, err)
		}
	}
}

func TestCandidateValidAssembles(t *testing.T) {
	repo, spec, gates := writeCandidateFixture(t, nil)
	m, err := assemble(t, repo, spec, gates)
	if err != nil {
		t.Fatal(err)
	}
	if m.ApprovalStatus != "pending" || m.ReleaseExecuted || m.Verdict != "candidate_verified" {
		t.Fatalf("invariants: %+v", m)
	}
	if len(m.GapsOpen) != 2 || m.GapsOpen[0].ID != "GAP-A" {
		t.Errorf("open gaps carried as-is, got %+v", m.GapsOpen)
	}
	if len(m.Risks) != 1 || m.Risks[0].Blocking || m.Risks[0].Measured["consecutive_green_runs"] != float64(4) || m.Risks[0].Measured["claim_unlocked"] != false {
		t.Errorf("risk must be recorded with its measured 4/8 and never block, got %+v", m.Risks)
	}
	if len(m.Gates) != 2 || len(m.Waivers) != 1 || m.Waivers[0].WaivedRecords != 0 || len(m.Deviations) != 1 {
		t.Errorf("gates/waivers/deviations: %+v %+v %+v", m.Gates, m.Waivers, m.Deviations)
	}
	if !strings.Contains(m.ClaimBoundary, "approval_status is pending") {
		t.Errorf("claim boundary must travel, got %q", m.ClaimBoundary)
	}
}

func TestCandidateRefusals(t *testing.T) {
	cases := []struct {
		name string
		code string
		mut  func(f map[string]string)
	}{
		{"approval approved", CodeCandidateApprovalNotPending, func(f map[string]string) {
			f["spec.yaml"] = strings.Replace(goodSpec, "approval_status: pending", "approval_status: approved", 1)
		}},
		{"approval invented while pending", CodeCandidateApprovalInvented, func(f map[string]string) {
			f["spec.yaml"] = strings.Replace(goodSpec, "approvals: []", "approvals:\n  - by: someone\n    on: 2026-09-05", 1)
		}},
		{"release claimed", CodeCandidateReleaseClaimed, func(f map[string]string) {
			f["spec.yaml"] = strings.Replace(goodSpec, "release_executed: false", "release_executed: true", 1)
		}},
		{"required artifact missing", CodeCandidateArtifactMissing, func(f map[string]string) { delete(f, "nomos-report.json") }},
		{"artifact present but unreadable", CodeCandidateArtifactUnreadable, func(f map[string]string) { f["nomos-report.json"] = "{not json" }},
		{"artifact present but a YAML list", CodeCandidateArtifactUnreadable, func(f map[string]string) { f["docs/canonical/source-manifest.yaml"] = "- a\n" }},
		{"open gap not acknowledged", CodeCandidateGapUnacknowledged, func(f map[string]string) {
			f["spec.yaml"] = strings.Replace(goodSpec, "gaps_acknowledged: [GAP-A, GAP-B]", "gaps_acknowledged: [GAP-A]", 1)
		}},
		{"acknowledged gap is closed in ledger", CodeCandidateGapStatusMismatch, func(f map[string]string) {
			f["spec.yaml"] = strings.Replace(goodSpec, "gaps_acknowledged: [GAP-A, GAP-B]", "gaps_acknowledged: [GAP-A, GAP-B, GAP-CLOSED]", 1)
		}},
		{"acknowledged gap unknown to ledger", CodeCandidateGapStatusMismatch, func(f map[string]string) {
			f["spec.yaml"] = strings.Replace(goodSpec, "gaps_acknowledged: [GAP-A, GAP-B]", "gaps_acknowledged: [GAP-A, GAP-B, GAP-NOPE]", 1)
		}},
		{"required gate absent", CodeCandidateGateMissing, func(f map[string]string) {
			f["gates.json"] = strings.Replace(f["gates.json"], `,{"id":"g2","command":"y","exit_code":0,"status":"pass"}`, "", 1)
		}},
		{"required gate failed", CodeCandidateGateFailed, func(f map[string]string) {
			f["gates.json"] = strings.Replace(f["gates.json"], `"id":"g2","command":"y","exit_code":0,"status":"pass"`, `"id":"g2","command":"y","exit_code":1,"status":"fail"`, 1)
		}},
		{"gate says pass but exit non-zero", CodeCandidateGateFailed, func(f map[string]string) {
			f["gates.json"] = strings.Replace(f["gates.json"], `"id":"g2","command":"y","exit_code":0,"status":"pass"`, `"id":"g2","command":"y","exit_code":3,"status":"pass"`, 1)
		}},
		{"gates measured on another commit", CodeCandidateGateCommitMismatch, func(f map[string]string) {
			f["gates.json"] = strings.Replace(f["gates.json"], `"commit":"abc123"`, `"commit":"def456"`, 1)
		}},
		{"gate evidence wrong schema", CodeCandidateGateMissing, func(f map[string]string) {
			f["gates.json"] = strings.Replace(f["gates.json"], "nomos-release-candidate-gates-v1", "something-else", 1)
		}},
		{"risk source unreadable", CodeCandidateRiskUnreadable, func(f map[string]string) { f["ci-index.json"] = `{"measurement":{}}` }},
		{"risk index lacks claim_unlocked", CodeCandidateRiskUnreadable, func(f map[string]string) {
			f["ci-index.json"] = `{"schema_version":"idx","measurement":{"consecutive_green_runs":4,"target_consecutive_green_runs":8}}`
		}},
		{"risk kind unknown", CodeCandidateRiskUnreadable, func(f map[string]string) {
			f["spec.yaml"] = strings.Replace(goodSpec, "kind: repeated_ci_evidence", "kind: declared_by_hand", 1)
		}},
		{"waiver entry without approver", CodeCandidateWaiverIncomplete, func(f map[string]string) {
			f["waiver.yaml"] = strings.Replace(goodWaiver, "waived_records: []", "waived_records:\n  - record_id: R-1\n    waived_on: 2026-09-05\n", 1)
		}},
		{"waiver without status", CodeCandidateWaiverIncomplete, func(f map[string]string) { f["waiver.yaml"] = "document_id: W\nwaived_records: []\n" }},
		{"deviation with unknown status", CodeCandidateDeviationInvalid, func(f map[string]string) {
			f["spec.yaml"] = strings.Replace(goodSpec, "status: open\n    record: deviation.md", "status: accepted\n    record: deviation.md", 1)
		}},
		{"deviation without record", CodeCandidateDeviationInvalid, func(f map[string]string) { delete(f, "deviation.md") }},
		{"spec wrong schema", CodeCandidateSpecInvalid, func(f map[string]string) {
			f["spec.yaml"] = strings.Replace(goodSpec, "nomos-release-candidate-spec-v1", "v0", 1)
		}},
		{"spec unknown level", CodeCandidateSpecInvalid, func(f map[string]string) {
			f["spec.yaml"] = strings.Replace(goodSpec, "target_level: NQ-1", "target_level: NQ-7", 1)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, spec, gates := writeCandidateFixture(t, tc.mut)
			_, err := assemble(t, repo, spec, gates)
			wantCode(t, err, tc.code)
		})
	}
}

func TestCandidateRiskShapeMessages(t *testing.T) {
	repo, spec, gates := writeCandidateFixture(t, func(f map[string]string) {
		f["ci-index.json"] = `{"schema_version":"idx","measurement":{"consecutive_green_runs":4}}`
	})
	_, err := assemble(t, repo, spec, gates)
	wantMessage(t, err, CodeCandidateRiskUnreadable, "lacks claim_unlocked")
	repo, spec, gates = writeCandidateFixture(t, func(f map[string]string) {
		f["ci-index.json"] = `{"schema_version":"idx","measurement":{"claim_unlocked":false}}`
	})
	_, err = assemble(t, repo, spec, gates)
	wantMessage(t, err, CodeCandidateRiskUnreadable, "lacks consecutive_green_runs")
}

func TestCandidateRequiresCommit(t *testing.T) {
	repo, spec, gates := writeCandidateFixture(t, nil)
	_, err := AssembleCandidate(CandidateInput{SpecPath: spec, GatesPath: gates, RepoRoot: repo})
	wantCode(t, err, CodeCandidateSpecInvalid)
}

func buildZip(t *testing.T) (CandidateManifest, string, string) {
	t.Helper()
	repo, spec, gates := writeCandidateFixture(t, nil)
	m, err := assemble(t, repo, spec, gates)
	if err != nil {
		t.Fatal(err)
	}
	gatesRaw, _ := os.ReadFile(gates)
	zipPath := filepath.Join(t.TempDir(), "c.zip")
	if err := WriteCandidateZip(m, gatesRaw, repo, zipPath); err != nil {
		t.Fatal(err)
	}
	return m, zipPath, repo
}

func TestCandidateZipRoundTrip(t *testing.T) {
	m, zipPath, repo := buildZip(t)
	got, err := VerifyCandidateZip(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	if got.ArtifactsDigest != m.ArtifactsDigest || got.GatesDigest != m.GatesDigest {
		t.Fatal("digests changed through the archive")
	}
	if err := VerifyCandidateManifest(got, repo); err != nil {
		t.Fatalf("verify against tree: %v", err)
	}
}

// rewriteZip copies the archive applying edit to entries by name.
func rewriteZip(t *testing.T, src string, edit func(name string, data []byte) ([]byte, bool)) string {
	t.Helper()
	r, err := zip.OpenReader(src)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range r.File {
		data, err := readZipFile(f)
		if err != nil {
			t.Fatal(err)
		}
		data, keep := edit(f.Name, data)
		if !keep {
			continue
		}
		zw, _ := w.Create(f.Name)
		zw.Write(data)
	}
	w.Close()
	out := filepath.Join(t.TempDir(), "t.zip")
	os.WriteFile(out, buf.Bytes(), 0o644)
	return out
}

func TestCandidateZipTamperIsRefused(t *testing.T) {
	_, zipPath, repo := buildZip(t)
	editManifest := func(mut func(m map[string]any)) func(string, []byte) ([]byte, bool) {
		return func(name string, data []byte) ([]byte, bool) {
			if name != CandidateManifestName {
				return data, true
			}
			var m map[string]any
			json.Unmarshal(data, &m)
			mut(m)
			out, _ := json.Marshal(m)
			return out, true
		}
	}
	cases := []struct {
		name string
		code string
		edit func(name string, data []byte) ([]byte, bool)
	}{
		{"evidence byte flipped", CodeCandidateTampered, func(name string, data []byte) ([]byte, bool) {
			if name == "evidence/nomos-report.json" {
				return []byte(`{"ok":false}`), true
			}
			return data, true
		}},
		{"evidence entry removed", CodeCandidateTampered, func(name string, data []byte) ([]byte, bool) {
			return data, name != "evidence/nomos-report.json"
		}},
		{"gates evidence edited", CodeCandidateTampered, func(name string, data []byte) ([]byte, bool) {
			if name == CandidateGatesName {
				return bytes.Replace(data, []byte(`"exit_code":0`), []byte(`"exit_code":0 `), 1), true
			}
			return data, true
		}},
		{"approval forged in manifest", CodeCandidateApprovalNotPending, editManifest(func(m map[string]any) { m["approval_status"] = "approved" })},
		{"release claimed in manifest", CodeCandidateReleaseClaimed, editManifest(func(m map[string]any) { m["release_executed"] = true })},
		{"artifact hash edited in manifest", CodeCandidateTampered, editManifest(func(m map[string]any) {
			arts := m["artifacts"].([]any)
			arts[0].(map[string]any)["hash"] = "sha256:" + strings.Repeat("0", 64)
		})},
		{"gate flipped to fail in manifest", CodeCandidateGateFailed, editManifest(func(m map[string]any) {
			m["gates"].([]any)[0].(map[string]any)["exit_code"] = float64(1)
		})},
		{"risk marked blocking in manifest", CodeCandidateManifestInvalid, editManifest(func(m map[string]any) {
			m["risks"].([]any)[0].(map[string]any)["blocking"] = true
		})},
		{"manifest removed", CodeCandidateManifestInvalid, func(name string, data []byte) ([]byte, bool) { return data, name != CandidateManifestName }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := VerifyCandidateZip(rewriteZip(t, zipPath, tc.edit))
			wantCode(t, err, tc.code)
		})
	}
	// Tree tamper: the archive is intact but the repository file changed after assembly.
	t.Run("tree file changed after assembly", func(t *testing.T) {
		m, err := VerifyCandidateZip(zipPath)
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(repo, "nomos-report.json"), []byte(`{"ok":"later"}`), 0o644)
		wantCode(t, VerifyCandidateManifest(m, repo), CodeCandidateTampered)
	})
}

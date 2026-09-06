package portfolio

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var readyNow = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func metInputs() readinessInputs {
	rows := map[string]matrixRow{}
	for _, id := range []string{"external_snapshot_input", "recursio_offline_e2e", "strict_fidelity_gate", "body_ledger_merkle_emission", "body_ledger_merkle_verification", "cross_reference_graph", "adapter_capability_kits", "compatibility_matrix", "canonical_knowledge_bundle", "release_candidate_bundle", "claim_coverage_attestation"} {
		rows[id] = matrixRow{Expected: "real", Computed: "real"}
	}
	return readinessInputs{MatrixRows: rows, AdapterCount: 3, UnsupportedEngine: true, LedgerStatus: "effective", LedgerSchemaVersion: "0.1.0", LedgerGeneratedAt: "2026-09-01",
		SecurityProcess: "nomos-security-process-v1", SupportModel: "nomos-support-model-v1", ClaimGuardRan: true}
}

func TestReadinessEvaluateAllMetIsReady(t *testing.T) {
	cs := evaluate(metInputs())
	v, unmet := verdictOf(cs)
	if v != VerdictReady || len(unmet) != 0 || len(cs) != 8 {
		t.Fatalf("%s %v", v, unmet)
	}
}

func TestReadinessEachCriterionBreaksAndIsNamed(t *testing.T) {
	cases := []struct {
		crit, want string
		mutate     func(*readinessInputs)
	}{
		{"C1", "contract-registry: boom", func(in *readinessInputs) { in.RegistryErr = errors.New("boom") }},
		{"C1", "stable contracts without a compatibility fixture: facets", func(in *readinessInputs) { in.StableWithoutCompat = []string{"facets"} }},
		{"C1", "capability recursio_offline_e2e computed \"partial\"", func(in *readinessInputs) {
			in.MatrixRows["recursio_offline_e2e"] = matrixRow{Expected: "real", Computed: "partial", Mismatch: true}
		}},
		{"C2", "unsupported_formats.go is missing", func(in *readinessInputs) { in.UnsupportedEngine = false }},
		{"C2", "capability strict_fidelity_gate is not in the wiring matrix", func(in *readinessInputs) { delete(in.MatrixRows, "strict_fidelity_gate") }},
		{"C3", "capability cross_reference_graph computed \"absent\"", func(in *readinessInputs) {
			in.MatrixRows["cross_reference_graph"] = matrixRow{Expected: "real", Computed: "absent"}
		}},
		{"C4", "adapters-compatible: x requires core", func(in *readinessInputs) { in.AdaptersErr = errors.New("x requires core >= 9") }},
		{"C4", "no adapter manifest found", func(in *readinessInputs) { in.AdapterCount = 0 }},
		{"C4", "adapter manifests not registered as fixtures: adapters/x/adapter.nomos.yaml", func(in *readinessInputs) { in.AdapterFixturesMissing = []string{"adapters/x/adapter.nomos.yaml"} }},
		{"C5", "capability release_candidate_bundle computed \"sidecar\"", func(in *readinessInputs) {
			in.MatrixRows["release_candidate_bundle"] = matrixRow{Expected: "real", Computed: "sidecar"}
		}},
		{"C6", "incomplete regulated_tool on: #7", func(in *readinessInputs) { in.ToolsMissingFields = []string{"#7"} }},
		{"C6", "closed items without regulated_tool: #8", func(in *readinessInputs) { in.ClosedWithoutTool = []string{"#8"} }},
		{"C6", "regulated-tools-declared: no lanes", func(in *readinessInputs) { in.RoadmapErr = errors.New("no lanes") }},
		{"C7", "status is \"draft\"", func(in *readinessInputs) { in.LedgerStatus = "draft" }},
		{"C7", "carries no schema_version", func(in *readinessInputs) { in.LedgerSchemaVersion = "" }},
		{"C7", "is stale under the portfolio freshness policy", func(in *readinessInputs) { in.LedgerStale = true }},
		{"C7", "evidence-ledger: gone", func(in *readinessInputs) { in.LedgerErr = errors.New("gone") }},
		{"C7", "security-process.yaml is absent", func(in *readinessInputs) { in.SecurityProcess = "" }},
		{"C7", "support-model.yaml is absent", func(in *readinessInputs) { in.SupportModel = "" }},
		{"C8", "claim guard red: x", func(in *readinessInputs) { in.ClaimGuardErr = errors.New("claim guard red: x") }},
		{"C8", "could not be executed here", func(in *readinessInputs) { in.ClaimGuardRan = false }},
		{"C1", "wiring matrix unavailable: nope", func(in *readinessInputs) { in.MatrixErr = errors.New("nope") }},
	}
	for _, c := range cases {
		in := metInputs()
		c.mutate(&in)
		cs := evaluate(in)
		v, unmet := verdictOf(cs)
		if v != VerdictNotReady {
			t.Errorf("%s (%s): still ready", c.crit, c.want)
			continue
		}
		joined := strings.Join(unmet, "\n")
		if !strings.Contains(joined, c.crit+"/") || !strings.Contains(joined, c.want) {
			t.Errorf("%s: want %q named, got:\n%s", c.crit, c.want, joined)
		}
		for _, cr := range cs {
			if cr.ID == c.crit && cr.Met {
				t.Errorf("%s reported met while a check failed", c.crit)
			}
		}
	}
}

func realRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	return filepath.Clean(filepath.Join(wd, "..", "..", ".."))
}

func TestReadinessOnTheRealRepositoryIsNotReadyAndVerifies(t *testing.T) {
	root := realRoot(t)
	r, err := ComputeReadiness(ReadinessOptions{RepoRoot: root, Now: readyNow, CoreVersion: "0.2.0-ALPHA", ClaimGuard: func(string) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if r.Verdict != VerdictNotReady || len(r.Unmet) == 0 || len(r.Criteria) != 8 || !strings.HasPrefix(r.StatusDigest, "sha256:") {
		t.Fatalf("today's tree must be not_ready with named reasons: %s %v", r.Verdict, r.Unmet)
	}
	joined := strings.Join(r.Unmet, "\n")
	if !strings.Contains(joined, "C1/stable-contracts-compat-fixtures") {
		t.Fatalf("expected the known gaps named:\n%s", joined)
	}
	path := filepath.Join(t.TempDir(), "readiness.json")
	raw, _ := json.MarshalIndent(r, "", "  ")
	_ = os.WriteFile(path, raw, 0o644)
	if _, err := VerifyReadinessFile(path, root, readyNow); err != nil {
		t.Fatalf("a freshly computed verdict must verify: %v", err)
	}
	if _, err := VerifyReadinessFile(path, root, readyNow.Add(400*24*time.Hour)); err == nil || !strings.Contains(err.Error(), "bound to portfolio status") {
		t.Fatalf("a verdict bound to another status digest must be refused: %v", err)
	}
	md := RenderReadinessMarkdown(r)
	if !strings.Contains(md, "NOT_READY") || !strings.Contains(md, "C1") {
		t.Fatal(md)
	}
}

func TestForgedReadinessIsRefused(t *testing.T) {
	root := realRoot(t)
	r, err := ComputeReadiness(ReadinessOptions{RepoRoot: root, Now: readyNow, CoreVersion: "0.2.0-ALPHA", ClaimGuard: func(string) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	write := func(mod func(m map[string]any)) string {
		raw, _ := json.Marshal(r)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		mod(m)
		out, _ := json.Marshal(m)
		p := filepath.Join(t.TempDir(), "r.json")
		_ = os.WriteFile(p, out, 0o644)
		return p
	}
	// 1. verdict flipped to ready, digest untouched → content edited
	_, err = VerifyReadinessFile(write(func(m map[string]any) { m["verdict"] = "ready" }), "", readyNow)
	if err == nil || !strings.Contains(err.Error(), "readiness_digest does not match") {
		t.Fatalf("%v", err)
	}
	// 2. verdict flipped AND digest recomputed → contradicts criteria
	forged := r
	forged.Verdict = VerdictReady
	forged.ReadinessDigest = digestOf(forged)
	_, err = VerifyReadinessFile(write(func(m map[string]any) { m["verdict"] = "ready"; m["readiness_digest"] = forged.ReadinessDigest }), "", readyNow)
	if err == nil || !strings.Contains(err.Error(), "contradicts its own criteria") {
		t.Fatalf("%v", err)
	}
	// 3. criteria all marked met AND verdict ready AND digest recomputed → unmet list disagrees
	forged2 := r
	forged2.Verdict = VerdictReady
	forged2.Criteria = append([]Criterion(nil), r.Criteria...)
	for i := range forged2.Criteria {
		cs := append([]Check(nil), forged2.Criteria[i].Checks...)
		for j := range cs {
			cs[j].OK = true
		}
		forged2.Criteria[i].Checks, forged2.Criteria[i].Met = cs, true
	}
	forged2.ReadinessDigest = digestOf(forged2)
	raw2, _ := json.Marshal(forged2)
	p2 := filepath.Join(t.TempDir(), "r2.json")
	_ = os.WriteFile(p2, raw2, 0o644)
	_, err = VerifyReadinessFile(p2, "", readyNow)
	if err == nil || !strings.Contains(err.Error(), "unmet list does not match") {
		t.Fatalf("%v", err)
	}
	// 4. there is no `released`
	rel := r
	rel.Verdict = "released"
	rel.ReadinessDigest = digestOf(rel)
	_, err = VerifyReadinessFile(write(func(m map[string]any) { m["verdict"] = "released"; m["readiness_digest"] = rel.ReadinessDigest }), "", readyNow)
	if err == nil || !strings.Contains(err.Error(), "no `released`") {
		t.Fatalf("%v", err)
	}
	// 5. wrong schema
	_, err = VerifyReadinessFile(write(func(m map[string]any) { m["schema_version"] = "v0" }), "", readyNow)
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("%v", err)
	}
	// 6. seven criteria only (digest recomputed)
	seven := r
	seven.Criteria = r.Criteria[:7]
	seven.Verdict, seven.Unmet = verdictOf(seven.Criteria)
	seven.ReadinessDigest = digestOf(seven)
	raw7, _ := json.Marshal(seven)
	p7 := filepath.Join(t.TempDir(), "r7.json")
	_ = os.WriteFile(p7, raw7, 0o644)
	_, err = VerifyReadinessFile(p7, "", readyNow)
	if err == nil || !strings.Contains(err.Error(), "docs/14 defines 8") {
		t.Fatalf("%v", err)
	}
}

func TestReadinessGatherNamesMissingSources(t *testing.T) {
	root := t.TempDir()
	in := gather(ReadinessOptions{RepoRoot: root, Now: readyNow, CoreVersion: "0.2.0-ALPHA", ClaimGuard: func(string) error { return errGuardNotRun }})
	cs := evaluate(in)
	v, unmet := verdictOf(cs)
	joined := strings.Join(unmet, "\n")
	for _, want := range []string{"C1/contract-registry", "C1/matrix:external_snapshot_input: wiring matrix unavailable", "C2/unsupported-formats-engine", "C6/regulated-tools-declared", "C7/evidence-ledger", "C7/security-process", "C7/support-model", "C8/claim-boundary-guard: the public-claim guard could not be executed here"} {
		if !strings.Contains(joined, want) {
			t.Errorf("empty tree must name %q:\n%s", want, joined)
		}
	}
	if v != VerdictNotReady {
		t.Fatal(v)
	}
}


func TestReadinessGatherNamesClosedItemsWithoutTool(t *testing.T) {
	root := t.TempDir()
	_ = os.MkdirAll(filepath.Join(root, "docs"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "docs", "roadmap-lanes.yaml"), []byte(`items:
  - issue: 1
    state: closed
    lane: product
  - issue: 2
    state: open
    lane: product
  - issue: 3
    state: closed
    lane: devops
    regulated_tool: {intended_use: x, impact: support, validation_state: technically_verified, reliance: manual_review}
  - issue: 4
    state: closed
    lane: devops
    regulated_tool: {intended_use: "", impact: support, validation_state: technically_verified, reliance: manual_review}
`), 0o644)
	in := gather(ReadinessOptions{RepoRoot: root, Now: readyNow, CoreVersion: "0.2.0-ALPHA", ClaimGuard: func(string) error { return nil }})
	if strings.Join(in.ClosedWithoutTool, ",") != "#1" || strings.Join(in.ToolsMissingFields, ",") != "#4" {
		t.Fatalf("closed-without-tool=%v missing-fields=%v", in.ClosedWithoutTool, in.ToolsMissingFields)
	}
}

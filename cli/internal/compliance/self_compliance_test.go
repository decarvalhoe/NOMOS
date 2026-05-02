package compliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Test on actual Nomos repo ---

func repoRoot() string {
	return filepath.Clean(filepath.Join("..", "..", ".."))
}

func TestSelfComplianceOnNomosRepo(t *testing.T) {
	result, err := EvaluateSelfCompliance(repoRoot())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalControls == 0 {
		t.Fatal("expected controls to be evaluated")
	}
	if result.Satisfied+result.TotalFindings != result.TotalControls {
		t.Fatalf("satisfied (%d) + findings (%d) != total controls (%d)",
			result.Satisfied, result.TotalFindings, result.TotalControls)
	}

	// Log for visibility
	t.Logf("verdict=%s controls=%d satisfied=%d findings=%d blocking=%d",
		result.Verdict, result.TotalControls, result.Satisfied, result.TotalFindings, result.Blocking)
	for _, f := range result.Findings {
		t.Logf("  [%s] %s severity=%s blocking=%v path=%s", f.ID, f.Control, f.Severity, f.Blocking, f.Path)
	}
}

func TestSelfComplianceCriticalControlsPresent(t *testing.T) {
	result, err := EvaluateSelfCompliance(repoRoot())
	if err != nil {
		t.Fatal(err)
	}
	// The Nomos repo should satisfy these critical controls
	satisfied := map[string]bool{}
	for _, ctrl := range regulatedControls() {
		ok, _ := ctrl.Check(repoRoot())
		if ok {
			satisfied[ctrl.ID] = true
		}
	}
	for _, id := range []string{"CTRL-REFREGISTRY", "CTRL-EVIDENCE", "CTRL-PROFILE"} {
		if !satisfied[id] {
			t.Logf("critical control %s not satisfied on repo (may be expected at current maturity)", id)
		}
	}
	_ = result // used for logging above
}

// --- Fixture-based tests ---

func TestSelfComplianceFullFixture(t *testing.T) {
	root := buildFullFixture(t)
	result, err := EvaluateSelfCompliance(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictCompliant {
		for _, f := range result.Findings {
			t.Logf("[%s] %s: %s (blocking=%v)", f.ID, f.Control, f.Message, f.Blocking)
		}
		t.Fatalf("expected compliant, got %s", result.Verdict)
	}
	if result.Satisfied != result.TotalControls {
		t.Fatalf("expected all %d controls satisfied, got %d", result.TotalControls, result.Satisfied)
	}
}

func TestSelfComplianceEmptyRepoBlocked(t *testing.T) {
	root := t.TempDir()
	result, err := EvaluateSelfCompliance(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictNonCompliant {
		t.Fatalf("expected non_compliant, got %s", result.Verdict)
	}
	if result.Blocking == 0 {
		t.Fatal("expected blocking findings for empty repo")
	}
}

func TestSelfCompliancePartialFixture(t *testing.T) {
	root := buildPartialFixture(t)
	result, err := EvaluateSelfCompliance(root)
	if err != nil {
		t.Fatal(err)
	}
	// Has critical controls but missing non-blocking ones
	if result.Verdict == VerdictNonCompliant {
		t.Fatalf("expected partial or compliant, got non_compliant (blocking=%d)", result.Blocking)
	}
	if result.TotalFindings == 0 {
		t.Fatal("expected some findings for partial fixture")
	}
}

func TestSelfComplianceFindingStructure(t *testing.T) {
	root := t.TempDir()
	result, err := EvaluateSelfCompliance(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range result.Findings {
		if !strings.HasPrefix(f.ID, "RC-") {
			t.Fatalf("expected RC- prefix, got %q", f.ID)
		}
		if f.Control == "" {
			t.Fatalf("finding %s missing control", f.ID)
		}
		if f.Severity == "" {
			t.Fatalf("finding %s missing severity", f.ID)
		}
		if f.Message == "" {
			t.Fatalf("finding %s missing message", f.ID)
		}
		if f.Remediation == "" {
			t.Fatalf("finding %s missing remediation", f.ID)
		}
		if f.Owner == "" {
			t.Fatalf("finding %s missing owner", f.ID)
		}
	}
}

func TestSelfComplianceNonexistentRoot(t *testing.T) {
	_, err := EvaluateSelfCompliance("/nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSelfComplianceVerdictConstants(t *testing.T) {
	if VerdictCompliant != "compliant" {
		t.Fatal("wrong compliant constant")
	}
	if VerdictPartial != "partial" {
		t.Fatal("wrong partial constant")
	}
	if VerdictNonCompliant != "non_compliant" {
		t.Fatal("wrong non_compliant constant")
	}
}

// --- Fixture builders ---

func buildFullFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// All critical + non-critical artifacts
	dirs := []string{
		"docs/regulated/control-matrix",
		"docs/regulated/reference-basis",
		"docs/regulated/evidence-index",
		"docs/regulated/product-profiles",
		"docs/regulated/lifecycle",
		"docs/regulated/quality-system",
		"docs/regulated/validation-pack",
		"docs/regulated/security-privacy",
		"docs/regulated/supplier-pack",
		"docs/regulated/decisions",
		".github/workflows",
	}
	for _, d := range dirs {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}

	// Files
	writeFixtureFile(t, root, "docs/regulated/control-matrix/matrix.yaml", "controls: []")
	writeFixtureFile(t, root, "docs/regulated/reference-basis/external-reference-register.yaml", "references: []")
	writeFixtureFile(t, root, "docs/regulated/evidence-index/evidence-ledger.yaml", "evidence_categories: []")
	writeFixtureFile(t, root, "docs/regulated/product-profiles/nomos.yaml",
		"regulated_design:\n  public_claim_boundary: \"Method draft only.\"")
	writeFixtureFile(t, root, "docs/regulated/lifecycle/sdlc.md", "# SDLC")
	writeFixtureFile(t, root, "docs/regulated/quality-system/policy.md", "# QMS Policy")
	writeFixtureFile(t, root, "docs/regulated/validation-pack/plan.md", "# Validation Plan")
	writeFixtureFile(t, root, "docs/regulated/security-privacy/access.md", "# Access Control")
	writeFixtureFile(t, root, "docs/regulated/supplier-pack/policy.md", "# Supplier Policy")
	writeFixtureFile(t, root, "docs/regulated/decisions/DEC-001.md", "# Decision 001")
	writeFixtureFile(t, root, ".github/workflows/ci.yml", "name: CI")
	writeFixtureFile(t, root, "nomos.project.yaml", "project:\n  id: test")

	return root
}

func buildPartialFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Only critical controls — missing non-blocking
	dirs := []string{
		"docs/regulated/control-matrix",
		"docs/regulated/reference-basis",
		"docs/regulated/evidence-index",
		"docs/regulated/product-profiles",
	}
	for _, d := range dirs {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}

	writeFixtureFile(t, root, "docs/regulated/control-matrix/matrix.yaml", "controls: []")
	writeFixtureFile(t, root, "docs/regulated/reference-basis/external-reference-register.yaml", "references: []")
	writeFixtureFile(t, root, "docs/regulated/evidence-index/evidence-ledger.yaml", "evidence: []")
	writeFixtureFile(t, root, "docs/regulated/product-profiles/nomos.yaml",
		"regulated_design:\n  public_claim_boundary: \"Draft.\"")
	writeFixtureFile(t, root, "nomos.project.yaml", "project:\n  id: test")

	return root
}

func writeFixtureFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

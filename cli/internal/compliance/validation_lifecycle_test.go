package compliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nomosRoot() string {
	return filepath.Clean(filepath.Join("..", "..", ".."))
}

// --- Nomos repo test ---

func TestValidationLifecycleOnNomosRepo(t *testing.T) {
	result, err := EvaluateValidationLifecycle(nomosRoot())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalArtifacts == 0 {
		t.Fatal("expected artifacts to be evaluated")
	}
	if result.Present+result.TotalFindings != result.TotalArtifacts {
		t.Fatalf("present (%d) + findings (%d) != total (%d)",
			result.Present, result.TotalFindings, result.TotalArtifacts)
	}
	t.Logf("verdict=%s artifacts=%d present=%d findings=%d blocking=%d",
		result.Verdict, result.TotalArtifacts, result.Present, result.TotalFindings, result.Blocking)
	for _, f := range result.Findings {
		t.Logf("  [%s] %s severity=%s blocking=%v", f.ID, f.Control, f.Severity, f.Blocking)
	}
}

func TestValidationLifecycleNomosRepoHasCriticalArtifacts(t *testing.T) {
	result, err := EvaluateValidationLifecycle(nomosRoot())
	if err != nil {
		t.Fatal(err)
	}
	// After this ticket, all critical artifacts should be present
	if result.Blocking > 0 {
		for _, f := range result.Findings {
			if f.Blocking {
				t.Errorf("blocking finding: [%s] %s — %s", f.ID, f.Control, f.Message)
			}
		}
		t.Fatalf("expected no blocking findings, got %d", result.Blocking)
	}
}

// --- Full fixture ---

func TestValidationLifecycleFullFixture(t *testing.T) {
	root := buildValidationFixture(t, true)
	result, err := EvaluateValidationLifecycle(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictCompliant {
		for _, f := range result.Findings {
			t.Logf("[%s] %s", f.ID, f.Control)
		}
		t.Fatalf("expected compliant, got %s", result.Verdict)
	}
	if result.Present != result.TotalArtifacts {
		t.Fatalf("expected all %d artifacts present, got %d", result.TotalArtifacts, result.Present)
	}
}

// --- Empty repo ---

func TestValidationLifecycleEmptyRepo(t *testing.T) {
	root := t.TempDir()
	result, err := EvaluateValidationLifecycle(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictNonCompliant {
		t.Fatalf("expected non_compliant, got %s", result.Verdict)
	}
	if result.Blocking == 0 {
		t.Fatal("expected blocking findings")
	}
}

// --- Partial fixture (only master plan, no intended-use) ---

func TestValidationLifecyclePartialFixture(t *testing.T) {
	root := t.TempDir()
	valDir := filepath.Join(root, "docs", "regulated", "validation-pack")
	os.MkdirAll(valDir, 0o755)
	os.WriteFile(filepath.Join(valDir, "validation-master-plan.md"),
		[]byte("# Plan\n\n## Scope\n\n## Acceptance Criteria\n\n## Approval\n"), 0o644)
	// Missing intended-use-model.yaml → blocking

	result, err := EvaluateValidationLifecycle(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictNonCompliant {
		t.Fatalf("expected non_compliant (missing intended-use), got %s", result.Verdict)
	}
	hasIntendedUse := false
	for _, f := range result.Findings {
		if f.Control == "VAL-INTENDED-USE" {
			hasIntendedUse = true
		}
	}
	if !hasIntendedUse {
		t.Fatal("expected VAL-INTENDED-USE finding")
	}
}

// --- Finding structure ---

func TestValidationLifecycleFindingStructure(t *testing.T) {
	root := t.TempDir()
	result, err := EvaluateValidationLifecycle(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range result.Findings {
		if !strings.HasPrefix(f.ID, "VL-") {
			t.Fatalf("expected VL- prefix, got %q", f.ID)
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

// --- Content checks ---

func TestValidationLifecycleContentChecks(t *testing.T) {
	root := t.TempDir()
	valDir := filepath.Join(root, "docs", "regulated", "validation-pack")
	os.MkdirAll(valDir, 0o755)
	// Master plan without required sections
	os.WriteFile(filepath.Join(valDir, "validation-master-plan.md"),
		[]byte("# Plan\n\nSome content."), 0o644)
	// Intended use without claim_boundary
	os.WriteFile(filepath.Join(valDir, "intended-use-model.yaml"),
		[]byte("intended_use:\n  primary_functions:\n    - id: F1\n"), 0o644)
	os.WriteFile(filepath.Join(valDir, "test-protocol-template.yaml"),
		[]byte("protocol: {}"), 0o644)

	result, err := EvaluateValidationLifecycle(root)
	if err != nil {
		t.Fatal(err)
	}
	controls := map[string]bool{}
	for _, f := range result.Findings {
		controls[f.Control] = true
	}
	// Should flag missing sections and claim boundary
	if !controls["VAL-PLAN-SCOPE"] {
		t.Fatal("expected VAL-PLAN-SCOPE finding")
	}
	if !controls["VAL-INTENDED-USE-BOUNDARY"] {
		t.Fatal("expected VAL-INTENDED-USE-BOUNDARY finding")
	}
}

// --- Error handling ---

func TestValidationLifecycleNonexistentRoot(t *testing.T) {
	_, err := EvaluateValidationLifecycle("/nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Fixture builder ---

func buildValidationFixture(t *testing.T, full bool) string {
	t.Helper()
	root := t.TempDir()
	valDir := filepath.Join(root, "docs", "regulated", "validation-pack")
	os.MkdirAll(valDir, 0o755)

	os.WriteFile(filepath.Join(valDir, "validation-master-plan.md"),
		[]byte("# VMP\n\n## Scope\n\nIn scope.\n\n## Acceptance Criteria\n\nAll pass.\n\n## Approval\n\n| Role |\n"), 0o644)

	os.WriteFile(filepath.Join(valDir, "intended-use-model.yaml"),
		[]byte("intended_use:\n  primary_functions:\n    - id: F1\nclaim_boundary: \"Draft only.\"\n"), 0o644)

	if full {
		os.WriteFile(filepath.Join(valDir, "test-protocol-template.yaml"),
			[]byte("protocol:\n  id: TP-001\n"), 0o644)
	}
	return root
}

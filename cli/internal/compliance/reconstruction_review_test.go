package compliance

import (
	"os"
	"path/filepath"
	"testing"
)

func setupReconstructionFixture(t *testing.T) (string, ReconstructionConfig) {
	t.Helper()
	dir := t.TempDir()

	// Create validation pack dir.
	packDir := filepath.Join(dir, "docs", "regulated", "validation-pack")
	os.MkdirAll(packDir, 0o755)

	// Create evidence index dir.
	evidenceDir := filepath.Join(dir, "docs", "regulated", "evidence-index")
	os.MkdirAll(evidenceDir, 0o755)

	return dir, DefaultReconstructionConfig(dir)
}

func writeFixtureInventory(t *testing.T, dir string, content string) {
	t.Helper()
	p := filepath.Join(dir, "docs", "regulated", "validation-pack", "validation-inventory.yaml")
	os.WriteFile(p, []byte(content), 0o644)
}

func writeFixtureIntendedUse(t *testing.T, dir string, content string) {
	t.Helper()
	p := filepath.Join(dir, "docs", "regulated", "validation-pack", "intended-use-model.yaml")
	os.WriteFile(p, []byte(content), 0o644)
}

func writeFixtureProtocol(t *testing.T, dir string, name string, content string) {
	t.Helper()
	p := filepath.Join(dir, "docs", "regulated", "validation-pack", name)
	os.WriteFile(p, []byte(content), 0o644)
}

const completeInventory = `schema_version: "0.1.0"
document_id: VI-TEST-001
status: draft
owner: test-owner
product: nomos

validations:
  - id: VAL-001
    intended_use_ref: IU-FUNC-001
    title: "Schema validation"
    risk_level: medium
    validation_type: automated
    method: "Unit tests"
    evidence_artifact: "go test ./validate/..."
    acceptance_gate: ci
    status: implemented
    owner: team
    verification_command: "cd cli && go test ./validate/..."
`

const highRiskInventory = `schema_version: "0.1.0"
document_id: VI-TEST-002
status: draft
owner: test-owner
product: nomos

validations:
  - id: VAL-013
    title: "Self-compliance"
    risk_level: critical
    validation_type: automated
    method: "Self-evaluation"
    evidence_artifact: "go test ./compliance/..."
    acceptance_gate: ci
    status: implemented
    owner: team
    verification_command: "cd cli && go test ./compliance/..."
`

const incompleteInventory = `schema_version: "0.1.0"
document_id: VI-TEST-003
status: draft
owner: test-owner
product: nomos

validations:
  - id: VAL-001
    title: ""
    risk_level: invalid
    validation_type: automated
    method: ""
    evidence_artifact: ""
    acceptance_gate: ""
    status: planned
    owner: ""
`

const intendedUseContent = `intended_use:
  primary_functions:
    - id: IU-FUNC-001
      function: "Schema validation"
`

func TestReconstructionReview_CompleteChain(t *testing.T) {
	dir, config := setupReconstructionFixture(t)
	writeFixtureInventory(t, dir, completeInventory)
	writeFixtureIntendedUse(t, dir, intendedUseContent)

	result, err := RunReconstructionReview(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != "passed" {
		t.Fatalf("expected passed, got %s", result.Verdict)
	}
	if result.Reconstructed != 1 {
		t.Fatalf("expected 1 reconstructed, got %d", result.Reconstructed)
	}
	if result.Failed != 0 {
		t.Fatalf("expected 0 failed, got %d", result.Failed)
	}
}

func TestReconstructionReview_IncompleteChain(t *testing.T) {
	dir, config := setupReconstructionFixture(t)
	writeFixtureInventory(t, dir, incompleteInventory)

	result, err := RunReconstructionReview(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != "failed" {
		t.Fatalf("expected failed, got %s", result.Verdict)
	}
	if result.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", result.Failed)
	}
	if result.TotalMissing == 0 {
		t.Fatal("expected missing links")
	}

	r := result.Results[0]
	missingNames := map[string]bool{}
	for _, link := range r.Chain {
		if link.Status == "missing" {
			missingNames[link.Name] = true
		}
	}
	for _, expected := range []string{"risk_level", "method", "evidence_artifact", "acceptance_gate", "owner"} {
		if !missingNames[expected] {
			t.Fatalf("expected missing link %q", expected)
		}
	}
}

func TestReconstructionReview_HighRiskRequiresProtocol(t *testing.T) {
	dir, config := setupReconstructionFixture(t)
	writeFixtureInventory(t, dir, highRiskInventory)

	// No protocol file exists.
	result, err := RunReconstructionReview(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != "failed" {
		t.Fatalf("expected failed (no protocol for critical risk), got %s", result.Verdict)
	}

	// Check that test_protocol link is missing.
	found := false
	for _, link := range result.Results[0].Chain {
		if link.Name == "test_protocol" && link.Status == "missing" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected missing test_protocol link")
	}
}

func TestReconstructionReview_HighRiskWithProtocol(t *testing.T) {
	dir, config := setupReconstructionFixture(t)
	writeFixtureInventory(t, dir, highRiskInventory)
	writeFixtureProtocol(t, dir, "TP-NOMOS-001.yaml", "validation_ref: VAL-013\nstatus: executed\n")

	result, err := RunReconstructionReview(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != "passed" {
		for _, r := range result.Results {
			for _, link := range r.Chain {
				if link.Status == "missing" {
					t.Logf("missing: %s — %s", link.Name, link.Detail)
				}
			}
		}
		t.Fatalf("expected passed with protocol present, got %s", result.Verdict)
	}
}

func TestReconstructionReview_IntendedUseTrace(t *testing.T) {
	dir, config := setupReconstructionFixture(t)
	writeFixtureInventory(t, dir, completeInventory)
	writeFixtureIntendedUse(t, dir, intendedUseContent)

	result, err := RunReconstructionReview(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check intended_use_trace link is present.
	found := false
	for _, link := range result.Results[0].Chain {
		if link.Name == "intended_use_trace" && link.Status == "present" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected intended_use_trace link to be present")
	}
}

func TestReconstructionReview_IntendedUseMissing(t *testing.T) {
	dir, config := setupReconstructionFixture(t)
	writeFixtureInventory(t, dir, completeInventory)
	// No intended-use file.

	result, err := RunReconstructionReview(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fail because IU-FUNC-001 can't be found.
	found := false
	for _, link := range result.Results[0].Chain {
		if link.Name == "intended_use_trace" && link.Status == "missing" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected missing intended_use_trace when IU file absent")
	}
}

func TestReconstructionReview_MissingInventory(t *testing.T) {
	config := ReconstructionConfig{
		RepoRoot:      "/nonexistent",
		InventoryPath: "/nonexistent/inventory.yaml",
	}
	_, err := RunReconstructionReview(config)
	if err == nil {
		t.Fatal("expected error for missing inventory")
	}
}

func TestReconstructionReview_VerificationCommand(t *testing.T) {
	dir, config := setupReconstructionFixture(t)

	inv := `schema_version: "0.1.0"
document_id: VI-TEST-004
status: draft
owner: test
product: nomos

validations:
  - id: VAL-001
    title: "Test"
    risk_level: low
    validation_type: automated
    method: "tests"
    evidence_artifact: "test output"
    acceptance_gate: ci
    status: implemented
    owner: team
    verification_command: "not-a-valid-tool foo bar"
`
	writeFixtureInventory(t, dir, inv)

	result, err := RunReconstructionReview(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, link := range result.Results[0].Chain {
		if link.Name == "verification_command" && link.Status == "missing" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected missing verification_command for unrecognized tool")
	}
}

func TestReconstructionReview_RealNomosRepo(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	invPath := filepath.Join(repoRoot, "docs", "regulated", "validation-pack", "validation-inventory.yaml")
	if _, err := os.Stat(invPath); err != nil {
		t.Skipf("real inventory not available: %v", err)
	}

	config := DefaultReconstructionConfig(repoRoot)
	result, err := RunReconstructionReview(config)
	if err != nil {
		t.Fatalf("RunReconstructionReview on real repo: %v", err)
	}

	if result.TotalValidations < 10 {
		t.Fatalf("expected at least 10 validations, got %d", result.TotalValidations)
	}

	// Log summary for visibility.
	t.Logf("verdict=%s reconstructed=%d failed=%d missing=%d",
		result.Verdict, result.Reconstructed, result.Failed, result.TotalMissing)

	// No critical-risk validation should have 0 chain links.
	for _, r := range result.Results {
		if len(r.Chain) == 0 {
			t.Fatalf("validation %s has 0 chain links", r.ValidationID)
		}
	}
}

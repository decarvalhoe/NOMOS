package compliance

import (
	"os"
	"path/filepath"
	"testing"
)

func writeYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

const validInventoryYAML = `schema_version: "0.1.0"
document_id: VI-NOMOS-001
status: draft
owner: quality-owner
product: nomos

validations:
  - id: VAL-001
    intended_use_ref: IU-FUNC-001
    title: "Manifest schema validation"
    risk_level: medium
    validation_type: automated
    method: "Unit tests + CUE vet"
    evidence_artifact: "go test ./internal/validate/..."
    acceptance_gate: ci
    status: implemented
    owner: core-team
  - id: VAL-002
    intended_use_ref: IU-FUNC-002
    title: "Source integrity checks"
    risk_level: medium
    validation_type: automated
    method: "Unit tests with hash verification"
    evidence_artifact: "go test ./internal/checks/..."
    acceptance_gate: ci
    status: verified
    owner: core-team
`

const intendedUseYAML = `intended_use:
  primary_functions:
    - id: IU-FUNC-001
      function: "Manifest schema validation"
      risk_level: medium
      verification: "go test ./internal/validate/..."
    - id: IU-FUNC-002
      function: "Source integrity checks"
      risk_level: medium
      verification: "go test ./internal/checks/..."
`

func TestLoadInventory(t *testing.T) {
	dir := t.TempDir()
	p := writeYAML(t, dir, "inventory.yaml", validInventoryYAML)

	inv, err := LoadInventory(p)
	if err != nil {
		t.Fatalf("LoadInventory: %v", err)
	}
	if inv.DocumentID != "VI-NOMOS-001" {
		t.Fatalf("expected VI-NOMOS-001, got %q", inv.DocumentID)
	}
	if len(inv.Validations) != 2 {
		t.Fatalf("expected 2 validations, got %d", len(inv.Validations))
	}
}

func TestLoadInventory_NotFound(t *testing.T) {
	_, err := LoadInventory("/nonexistent/path.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadIntendedUse(t *testing.T) {
	dir := t.TempDir()
	p := writeYAML(t, dir, "intended-use.yaml", intendedUseYAML)

	model, err := LoadIntendedUse(p)
	if err != nil {
		t.Fatalf("LoadIntendedUse: %v", err)
	}
	if len(model.IntendedUse.PrimaryFunctions) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(model.IntendedUse.PrimaryFunctions))
	}
}

func TestCheckCompleteness_AllComplete(t *testing.T) {
	inv := ValidationInventory{
		Validations: []ValidationEntry{
			{ID: "VAL-001", IntendedUseRef: "IU-FUNC-001", Title: "Test A", RiskLevel: "medium", ValidationType: "automated", Method: "tests", EvidenceArtifact: "go test", AcceptanceGate: "ci", Status: "implemented", Owner: "team"},
			{ID: "VAL-002", IntendedUseRef: "IU-FUNC-002", Title: "Test B", RiskLevel: "high", ValidationType: "automated", Method: "tests", EvidenceArtifact: "go test", AcceptanceGate: "ci", Status: "verified", Owner: "team"},
		},
	}
	model := &IntendedUseModel{}
	model.IntendedUse.PrimaryFunctions = []IntendedUseFunc{
		{ID: "IU-FUNC-001", Function: "Func A"},
		{ID: "IU-FUNC-002", Function: "Func B"},
	}

	result := CheckCompleteness(inv, model)
	if result.Verdict != "pass" {
		t.Fatalf("expected pass, got %s; findings: %v", result.Verdict, result.Findings)
	}
	if result.Complete != 2 {
		t.Fatalf("expected 2 complete, got %d", result.Complete)
	}
	if result.CoveredFunctions != 2 {
		t.Fatalf("expected 2 covered functions, got %d", result.CoveredFunctions)
	}
}

func TestCheckCompleteness_MissingFields(t *testing.T) {
	inv := ValidationInventory{
		Validations: []ValidationEntry{
			{ID: "VAL-001", Title: "", RiskLevel: "invalid", Status: "unknown"},
		},
	}

	result := CheckCompleteness(inv, nil)
	if result.Verdict != "fail" {
		t.Fatalf("expected fail, got %s", result.Verdict)
	}
	if result.Incomplete != 1 {
		t.Fatalf("expected 1 incomplete, got %d", result.Incomplete)
	}
	if len(result.Findings) == 0 {
		t.Fatal("expected findings for missing fields")
	}

	fieldsSeen := map[string]bool{}
	for _, f := range result.Findings {
		fieldsSeen[f.Field] = true
	}
	for _, required := range []string{"title", "risk_level", "status", "method", "evidence_artifact", "acceptance_gate", "owner"} {
		if !fieldsSeen[required] {
			t.Fatalf("expected finding for field %q", required)
		}
	}
}

func TestCheckCompleteness_DuplicateIDs(t *testing.T) {
	inv := ValidationInventory{
		Validations: []ValidationEntry{
			{ID: "VAL-001", Title: "A", RiskLevel: "low", ValidationType: "automated", Method: "m", EvidenceArtifact: "e", AcceptanceGate: "ci", Status: "implemented", Owner: "o"},
			{ID: "VAL-001", Title: "B", RiskLevel: "low", ValidationType: "automated", Method: "m", EvidenceArtifact: "e", AcceptanceGate: "ci", Status: "implemented", Owner: "o"},
		},
	}

	result := CheckCompleteness(inv, nil)
	if result.Verdict != "fail" {
		t.Fatalf("expected fail for duplicate IDs, got %s", result.Verdict)
	}
	found := false
	for _, f := range result.Findings {
		if f.Field == "id" && f.Severity == "critical" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected critical finding for duplicate id")
	}
}

func TestCheckCompleteness_UncoveredIntendedUse(t *testing.T) {
	inv := ValidationInventory{
		Validations: []ValidationEntry{
			{ID: "VAL-001", IntendedUseRef: "IU-FUNC-001", Title: "A", RiskLevel: "low", ValidationType: "automated", Method: "m", EvidenceArtifact: "e", AcceptanceGate: "ci", Status: "implemented", Owner: "o"},
		},
	}
	model := &IntendedUseModel{}
	model.IntendedUse.PrimaryFunctions = []IntendedUseFunc{
		{ID: "IU-FUNC-001", Function: "Covered"},
		{ID: "IU-FUNC-002", Function: "Uncovered"},
		{ID: "IU-FUNC-003", Function: "Also uncovered"},
	}

	result := CheckCompleteness(inv, model)
	if result.Verdict != "fail" {
		t.Fatalf("expected fail for uncovered functions, got %s", result.Verdict)
	}
	if result.CoveredFunctions != 1 {
		t.Fatalf("expected 1 covered, got %d", result.CoveredFunctions)
	}
	if len(result.UncoveredFuncs) != 2 {
		t.Fatalf("expected 2 uncovered, got %d", len(result.UncoveredFuncs))
	}
}

func TestCheckCompleteness_NilIntendedUse(t *testing.T) {
	inv := ValidationInventory{
		Validations: []ValidationEntry{
			{ID: "VAL-001", Title: "A", RiskLevel: "low", ValidationType: "automated", Method: "m", EvidenceArtifact: "e", AcceptanceGate: "ci", Status: "implemented", Owner: "o"},
		},
	}

	result := CheckCompleteness(inv, nil)
	if result.Verdict != "pass" {
		t.Fatalf("expected pass with nil intended-use, got %s", result.Verdict)
	}
}

func TestCheckCompleteness_EmptyInventory(t *testing.T) {
	inv := ValidationInventory{}
	result := CheckCompleteness(inv, nil)
	if result.TotalValidations != 0 {
		t.Fatalf("expected 0 total, got %d", result.TotalValidations)
	}
	if result.Verdict != "pass" {
		t.Fatalf("expected pass for empty inventory, got %s", result.Verdict)
	}
}

func TestCheckCompleteness_RealInventoryFile(t *testing.T) {
	invPath := filepath.Join("..", "..", "..", "docs", "regulated", "validation-pack", "validation-inventory.yaml")
	if _, err := os.Stat(invPath); err != nil {
		t.Skipf("real inventory not available: %v", err)
	}

	inv, err := LoadInventory(invPath)
	if err != nil {
		t.Fatalf("load real inventory: %v", err)
	}
	if len(inv.Validations) < 10 {
		t.Fatalf("expected at least 10 validations, got %d", len(inv.Validations))
	}

	iuPath := filepath.Join("..", "..", "..", "docs", "regulated", "validation-pack", "intended-use-model.yaml")
	var model *IntendedUseModel
	if _, err := os.Stat(iuPath); err == nil {
		m, err := LoadIntendedUse(iuPath)
		if err != nil {
			t.Fatalf("load intended-use: %v", err)
		}
		model = &m
	}

	result := CheckCompleteness(inv, model)
	if result.TotalValidations < 10 {
		t.Fatalf("expected at least 10 total validations, got %d", result.TotalValidations)
	}
	// The real inventory should pass structural checks.
	for _, f := range result.Findings {
		if f.Severity == "critical" {
			t.Fatalf("critical finding in real inventory: %s — %s", f.Field, f.Message)
		}
	}
}

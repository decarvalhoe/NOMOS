package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testdataPath(name string) string {
	return filepath.Join("testdata", name)
}

func TestValidMatrix(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "contracts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "contracts", "rule.yaml"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckContracts(testdataPath("valid-matrix.yaml"), base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Units[0].Errors)
	}
	if len(result.Units) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(result.Units))
	}
	if result.Units[0].UnitID != "UNIT-001" {
		t.Fatalf("expected UNIT-001, got %q", result.Units[0].UnitID)
	}
}

func TestNoContract(t *testing.T) {
	result, err := CheckContracts(testdataPath("no-contract.yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertError(t, result.Units[0], "NO_CONTRACT")
}

func TestInvalidContractStatus(t *testing.T) {
	result, err := CheckContracts(testdataPath("invalid-contract-status.yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertError(t, result.Units[0], "INVALID_CONTRACT_STATUS")
}

func TestMissingSourceRefs(t *testing.T) {
	result, err := CheckContracts(testdataPath("missing-source-refs.yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertError(t, result.Units[0], "NO_SOURCE_REFS")
}

func TestInvalidUnitID(t *testing.T) {
	result, err := CheckContracts(testdataPath("invalid-unit-id.yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertError(t, result.Units[0], "INVALID_UNIT_ID")
}

func TestContractFileNotFound(t *testing.T) {
	base := t.TempDir()
	result, err := CheckContracts(testdataPath("contract-file-missing.yaml"), base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertError(t, result.Units[0], "CONTRACT_FILE_NOT_FOUND")
}

func TestContractFileSkippedWithoutBaseDir(t *testing.T) {
	result, err := CheckContracts(testdataPath("contract-file-missing.yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid when baseDir is empty (skip file check), got errors: %v", result.Units[0].Errors)
	}
}

func TestMultipleErrors(t *testing.T) {
	result, err := CheckContracts(testdataPath("multiple-errors.yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}

	u := result.Units[0]
	assertError(t, u, "MISSING_UNIT_ID")
	assertError(t, u, "INVALID_UNIT_TYPE")
	assertError(t, u, "MISSING_NAME")
	assertError(t, u, "MISSING_DOMAIN")
	assertError(t, u, "INVALID_CRITICALITY")
	assertError(t, u, "MISSING_BUSINESS_RULE")
	assertError(t, u, "NO_SOURCE_REFS")
	assertError(t, u, "NO_CONTRACT")
	assertError(t, u, "INVALID_STATUS")
}

func TestCheckContractsFromBytes(t *testing.T) {
	data := []byte(`
units:
  - unit_id: UNIT-OK
    unit_type: term
    name: "A term"
    domain: legal
    criticality: low
    source_refs:
      - source_id: SRC-001
        locator: "section 2"
    business_rule: "Defined as X."
    canonical_contract:
      path: contracts/term.yaml
      object_id: UNIT-OK
      status: planned
    status: partial
`)
	result, err := CheckContractsFromBytes(data, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Units[0].Errors)
	}
}

func TestInvalidYAML(t *testing.T) {
	_, err := CheckContractsFromBytes([]byte(`{not valid`), "")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestNonexistentManifest(t *testing.T) {
	_, err := CheckContracts("/nonexistent/matrix.yaml", "")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestOfficialExamplePasses(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	examplePath := filepath.Join(repoRoot, "examples", "insurance", "canonical-matrix.example.yaml")
	result, err := CheckContracts(examplePath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Official example has planned contract and partial status — should still pass checks
	for _, u := range result.Units {
		for _, e := range u.Errors {
			t.Errorf("[%s] %s: %s", e.Code, e.UnitID, e.Message)
		}
	}
	if !result.Valid {
		t.Fatal("expected official example to pass checks")
	}
}

func assertError(t *testing.T, uc UnitCheck, code string) {
	t.Helper()
	for _, e := range uc.Errors {
		if e.Code == code {
			return
		}
	}
	var codes []string
	for _, e := range uc.Errors {
		codes = append(codes, e.Code)
	}
	t.Fatalf("expected error code %s in [%s]", code, strings.Join(codes, ", "))
}

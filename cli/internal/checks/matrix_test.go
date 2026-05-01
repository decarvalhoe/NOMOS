package checks

import (
	"path/filepath"
	"testing"
)

func TestCheckMatrixValidFile(t *testing.T) {
	m, err := ParseMatrixFile(filepath.Join("testdata", "matrix-valid.yaml"))
	if err != nil {
		t.Fatalf("parse valid matrix: %v", err)
	}

	result := CheckMatrix(m)

	if result.Format != MatrixCheckFormat {
		t.Fatalf("expected format %q, got %q", MatrixCheckFormat, result.Format)
	}
	if result.TotalUnits != 3 {
		t.Fatalf("expected 3 total units, got %d", result.TotalUnits)
	}
	if result.CoveredUnits != 1 {
		t.Fatalf("expected 1 covered unit, got %d", result.CoveredUnits)
	}
	if result.CoverageScore < 0.33 || result.CoverageScore > 0.34 {
		t.Fatalf("expected coverage score ~0.33, got %f", result.CoverageScore)
	}

	errorFindings := filterByCode(result.Findings, SeverityError)
	if len(errorFindings) != 0 {
		t.Fatalf("expected no errors for valid matrix, got %d: %v", len(errorFindings), errorFindings)
	}
}

func TestCheckMatrixMissingTest(t *testing.T) {
	m, err := ParseMatrixFile(filepath.Join("testdata", "matrix-missing-test.yaml"))
	if err != nil {
		t.Fatalf("parse matrix: %v", err)
	}

	result := CheckMatrix(m)

	assertHasFindingCode(t, result, CodeMissingTest)

	if result.CoveredUnits != 0 {
		t.Fatalf("expected 0 covered units (covered unit has error), got %d", result.CoveredUnits)
	}
}

func TestCheckMatrixBrokenContract(t *testing.T) {
	m, err := ParseMatrixFile(filepath.Join("testdata", "matrix-broken-contract.yaml"))
	if err != nil {
		t.Fatalf("parse matrix: %v", err)
	}

	result := CheckMatrix(m)

	assertHasFindingCode(t, result, CodeBrokenContractRef)

	found := findFinding(result, CodeBrokenContractRef, "INS-HOME-WARRANTY-WATER-DAMAGE")
	if found == nil {
		t.Fatalf("expected BROKEN_CONTRACT_REF for INS-HOME-WARRANTY-WATER-DAMAGE")
	}
	if found.Severity != SeverityError {
		t.Fatalf("expected severity error, got %q", found.Severity)
	}
}

func TestCheckMatrixInvalidSource(t *testing.T) {
	m, err := ParseMatrixFile(filepath.Join("testdata", "matrix-invalid-source.yaml"))
	if err != nil {
		t.Fatalf("parse matrix: %v", err)
	}

	result := CheckMatrix(m)

	assertHasFindingCode(t, result, CodeMissingSourceRef)

	// Should have two findings: invalid source_id and empty locator.
	count := countFindingsWithCode(result, CodeMissingSourceRef)
	if count != 2 {
		t.Fatalf("expected 2 MISSING_SOURCE_REF findings, got %d", count)
	}
}

func TestCheckMatrixDeprecatedNoDecision(t *testing.T) {
	m, err := ParseMatrixFile(filepath.Join("testdata", "matrix-deprecated-no-decision.yaml"))
	if err != nil {
		t.Fatalf("parse matrix: %v", err)
	}

	result := CheckMatrix(m)

	assertHasFindingCode(t, result, CodeDeprecatedNoDecision)

	found := findFinding(result, CodeDeprecatedNoDecision, "DEC-OLD-BEHAVIOR")
	if found == nil {
		t.Fatalf("expected DEPRECATED_NO_DECISION finding")
	}
	if found.Severity != SeverityWarning {
		t.Fatalf("expected severity warning, got %q", found.Severity)
	}
}

func TestCheckMatrixCoverageScore(t *testing.T) {
	m := CanonicalMatrix{
		SchemaVersion: "0.1.0",
		Units: []Unit{
			{
				UnitID: "UNIT-A", UnitType: "rule", Name: "A", Domain: "test",
				Criticality: "low", BusinessRule: "rule A",
				SourceRefs: []SourceRef{{SourceID: "SRC-1", Locator: "p.1"}},
				TestRefs:   []string{"tests/a_test.go"},
				Status:     "covered",
			},
			{
				UnitID: "UNIT-B", UnitType: "rule", Name: "B", Domain: "test",
				Criticality: "low", BusinessRule: "rule B",
				SourceRefs: []SourceRef{{SourceID: "SRC-1", Locator: "p.2"}},
				TestRefs:   []string{"tests/b_test.go"},
				Status:     "covered",
			},
			{
				UnitID: "UNIT-C", UnitType: "rule", Name: "C", Domain: "test",
				Criticality: "low", BusinessRule: "rule C",
				SourceRefs: []SourceRef{{SourceID: "SRC-1", Locator: "p.3"}},
				Status:     "missing",
				Gaps:       []string{"Not implemented yet."},
			},
			{
				UnitID: "UNIT-D", UnitType: "rule", Name: "D", Domain: "test",
				Criticality: "low", BusinessRule: "rule D",
				SourceRefs: []SourceRef{{SourceID: "SRC-1", Locator: "p.4"}},
				Status:     "partial",
				Gaps:       []string{"Partial implementation."},
			},
		},
	}

	result := CheckMatrix(m)

	if result.TotalUnits != 4 {
		t.Fatalf("expected 4 total units, got %d", result.TotalUnits)
	}
	if result.CoveredUnits != 2 {
		t.Fatalf("expected 2 covered units, got %d", result.CoveredUnits)
	}
	if result.CoverageScore != 0.5 {
		t.Fatalf("expected coverage score 0.5, got %f", result.CoverageScore)
	}
}

func TestParseMatrixEmptyUnits(t *testing.T) {
	_, err := ParseMatrix([]byte("schema_version: \"0.1.0\"\nunits: []\n"))
	if err == nil {
		t.Fatalf("expected error for empty units")
	}
}

func TestParseMatrixInvalidYAML(t *testing.T) {
	_, err := ParseMatrix([]byte("not: [valid: yaml: content"))
	if err == nil {
		t.Fatalf("expected error for invalid YAML")
	}
}

// --- helpers ---

func filterByCode(findings []Finding, sev Severity) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Severity == sev {
			out = append(out, f)
		}
	}
	return out
}

func assertHasFindingCode(t *testing.T, result MatrixCheckResult, code string) {
	t.Helper()
	for _, f := range result.Findings {
		if f.Code == code {
			return
		}
	}
	t.Fatalf("expected finding with code %q, got %v", code, result.Findings)
}

func findFinding(result MatrixCheckResult, code string, unitID string) *Finding {
	for _, f := range result.Findings {
		if f.Code == code && f.UnitID == unitID {
			return &f
		}
	}
	return nil
}

func countFindingsWithCode(result MatrixCheckResult, code string) int {
	count := 0
	for _, f := range result.Findings {
		if f.Code == code {
			count++
		}
	}
	return count
}

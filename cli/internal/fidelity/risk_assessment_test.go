package fidelity

import (
	"strings"
	"testing"
)

func TestComputeRiskLevel(t *testing.T) {
	cases := []struct {
		sev  RiskSeverity
		like RiskLikelihood
		want RiskLevel
	}{
		{SeverityNegligible, LikelihoodRare, RiskLow},
		{SeverityMinor, LikelihoodUnlikely, RiskMedium},
		{SeverityMajor, LikelihoodPossible, RiskHigh},
		{SeverityCritical, LikelihoodLikely, RiskCritical},
		{SeverityCritical, LikelihoodAlmostCertain, RiskCritical},
		{SeverityNegligible, LikelihoodAlmostCertain, RiskMedium},
		{SeverityMajor, LikelihoodRare, RiskLow},
	}
	for _, tc := range cases {
		got := ComputeRiskLevel(tc.sev, tc.like)
		if got != tc.want {
			t.Errorf("ComputeRiskLevel(%s, %s) = %s, want %s", tc.sev, tc.like, got, tc.want)
		}
	}
}

func TestComputeRiskScore(t *testing.T) {
	score := ComputeRiskScore(SeverityCritical, LikelihoodAlmostCertain)
	if score != 20 {
		t.Fatalf("expected 20, got %d", score)
	}
	score = ComputeRiskScore(SeverityNegligible, LikelihoodRare)
	if score != 1 {
		t.Fatalf("expected 1, got %d", score)
	}
}

func testFunctions() []FunctionRisk {
	return []FunctionRisk{
		{FunctionID: "F-001", Name: "Schema validation", Severity: SeverityMajor, Likelihood: LikelihoodPossible},
		{FunctionID: "F-002", Name: "Source checks", Severity: SeverityMinor, Likelihood: LikelihoodUnlikely},
		{FunctionID: "F-003", Name: "Release gate", Severity: SeverityCritical, Likelihood: LikelihoodLikely},
	}
}

func testControls() []Control {
	return []Control{
		{ID: "C-001", Name: "Unit tests", Type: "preventive", Functions: []string{"F-001", "F-002"}, Implemented: true, Verified: true, Evidence: "go test"},
		{ID: "C-002", Name: "CI gate", Type: "detective", Functions: []string{"F-001", "F-003"}, Implemented: true, Verified: true, Evidence: "ci.yml"},
		{ID: "C-003", Name: "Review", Type: "corrective", Functions: []string{"F-003"}, Implemented: true, Verified: false},
	}
}

func TestAssessRisks_AllControlled(t *testing.T) {
	ra := AssessRisks(testFunctions(), testControls())

	if ra.TotalFunctions != 3 {
		t.Fatalf("expected 3 functions, got %d", ra.TotalFunctions)
	}
	if ra.TotalControls != 3 {
		t.Fatalf("expected 3 controls, got %d", ra.TotalControls)
	}
	if ra.ControlCoverage != 1.0 {
		t.Fatalf("expected 100%% coverage, got %.0f%%", ra.ControlCoverage*100)
	}
	if ra.UnmitigatedCount != 0 {
		t.Fatalf("expected 0 unmitigated, got %d", ra.UnmitigatedCount)
	}
}

func TestAssessRisks_ControlMapping(t *testing.T) {
	ra := AssessRisks(testFunctions(), testControls())

	for _, f := range ra.Functions {
		if f.FunctionID == "F-001" {
			if len(f.Controls) != 2 {
				t.Fatalf("F-001 should have 2 controls, got %d", len(f.Controls))
			}
		}
		if f.FunctionID == "F-003" {
			if len(f.Controls) != 2 {
				t.Fatalf("F-003 should have 2 controls, got %d", len(f.Controls))
			}
		}
	}
}

func TestAssessRisks_ResidualReduction(t *testing.T) {
	ra := AssessRisks(testFunctions(), testControls())

	for _, f := range ra.Functions {
		if f.FunctionID == "F-001" {
			// Major×Possible = High, all controls verified → Medium
			if f.Residual != RiskMedium {
				t.Fatalf("F-001 residual: expected medium, got %s", f.Residual)
			}
		}
		if f.FunctionID == "F-003" {
			// Critical×Likely = Critical, C-003 not verified → no reduction
			if f.Residual != RiskCritical {
				t.Fatalf("F-003 residual: expected critical (unverified control), got %s", f.Residual)
			}
		}
	}
}

func TestAssessRisks_Unmitigated(t *testing.T) {
	funcs := []FunctionRisk{
		{FunctionID: "F-010", Name: "Unmapped", Severity: SeverityMajor, Likelihood: LikelihoodPossible},
	}
	ra := AssessRisks(funcs, nil)

	if ra.UnmitigatedCount != 1 {
		t.Fatalf("expected 1 unmitigated, got %d", ra.UnmitigatedCount)
	}
	if ra.Verdict != "unacceptable" {
		t.Fatalf("expected unacceptable, got %s", ra.Verdict)
	}
	if ra.ControlCoverage != 0 {
		t.Fatalf("expected 0 coverage, got %.2f", ra.ControlCoverage)
	}
}

func TestAssessRisks_Acceptable(t *testing.T) {
	funcs := []FunctionRisk{
		{FunctionID: "F-001", Name: "Low risk", Severity: SeverityMinor, Likelihood: LikelihoodRare},
	}
	controls := []Control{
		{ID: "C-001", Name: "Test", Type: "preventive", Functions: []string{"F-001"}, Implemented: true, Verified: true},
	}
	ra := AssessRisks(funcs, controls)

	if ra.Verdict != "acceptable" {
		t.Fatalf("expected acceptable, got %s", ra.Verdict)
	}
}

func TestAssessRisks_CriticalResidualForces_Unacceptable(t *testing.T) {
	funcs := []FunctionRisk{
		{FunctionID: "F-001", Name: "Critical", Severity: SeverityCritical, Likelihood: LikelihoodAlmostCertain},
	}
	// Control exists but not verified → no reduction → residual=critical
	controls := []Control{
		{ID: "C-001", Name: "Gate", Type: "detective", Functions: []string{"F-001"}, Implemented: true, Verified: false},
	}
	ra := AssessRisks(funcs, controls)

	if ra.Verdict != "unacceptable" {
		t.Fatalf("expected unacceptable for critical residual, got %s", ra.Verdict)
	}
}

func TestAssessRisks_SortedByScore(t *testing.T) {
	ra := AssessRisks(testFunctions(), testControls())

	for i := 1; i < len(ra.Functions); i++ {
		if ra.Functions[i].RiskScore > ra.Functions[i-1].RiskScore {
			t.Fatal("expected functions sorted by risk score descending")
		}
	}
}

func TestAssessRisks_RiskLevelCounts(t *testing.T) {
	ra := AssessRisks(testFunctions(), testControls())

	total := 0
	for _, count := range ra.ByRiskLevel {
		total += count
	}
	if total != ra.TotalFunctions {
		t.Fatalf("risk level counts sum %d != total %d", total, ra.TotalFunctions)
	}
}

func TestAssessRisks_Empty(t *testing.T) {
	ra := AssessRisks(nil, nil)
	if ra.TotalFunctions != 0 {
		t.Fatalf("expected 0, got %d", ra.TotalFunctions)
	}
	if ra.Verdict != "acceptable" {
		t.Fatalf("expected acceptable for empty, got %s", ra.Verdict)
	}
}

func TestReduceRisk(t *testing.T) {
	cases := map[RiskLevel]RiskLevel{
		RiskCritical: RiskHigh,
		RiskHigh:     RiskMedium,
		RiskMedium:   RiskLow,
		RiskLow:      RiskLow,
	}
	for input, want := range cases {
		got := reduceRisk(input)
		if got != want {
			t.Errorf("reduceRisk(%s) = %s, want %s", input, got, want)
		}
	}
}

func TestFormatRiskSummary(t *testing.T) {
	ra := AssessRisks(testFunctions(), testControls())
	summary := FormatRiskSummary(ra)

	if !strings.Contains(summary, "Risk Assessment") {
		t.Fatal("expected header in summary")
	}
	if !strings.Contains(summary, "F-001") {
		t.Fatal("expected function ID in summary")
	}
	if !strings.Contains(summary, "Coverage") {
		t.Fatal("expected coverage in summary")
	}
}

func TestFormatRiskSummary_Unacceptable(t *testing.T) {
	ra := AssessRisks([]FunctionRisk{
		{FunctionID: "F-X", Severity: SeverityCritical, Likelihood: LikelihoodLikely},
	}, nil)
	summary := FormatRiskSummary(ra)
	if !strings.Contains(summary, "UNACCEPTABLE") {
		t.Fatal("expected UNACCEPTABLE in summary")
	}
}

func TestSeverityRank(t *testing.T) {
	if SeverityNegligible.rank() >= SeverityMinor.rank() {
		t.Fatal("negligible should rank below minor")
	}
	if SeverityMinor.rank() >= SeverityMajor.rank() {
		t.Fatal("minor should rank below major")
	}
	if SeverityMajor.rank() >= SeverityCritical.rank() {
		t.Fatal("major should rank below critical")
	}
}

func TestLikelihoodRank(t *testing.T) {
	if LikelihoodRare.rank() >= LikelihoodUnlikely.rank() {
		t.Fatal("rare should rank below unlikely")
	}
	if LikelihoodPossible.rank() >= LikelihoodLikely.rank() {
		t.Fatal("possible should rank below likely")
	}
}

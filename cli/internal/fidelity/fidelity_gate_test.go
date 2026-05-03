package fidelity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var gateTime = time.Date(2026, 5, 3, 14, 0, 0, 0, time.UTC)

func allPassInputs() []CheckInput {
	return []CheckInput{
		{Category: CatASTCoverage, TotalBytes: 1000, CoveredBytes: 1000, LostBytes: 0, IsLossless: true},
		{Category: CatAtomComplete, TotalAtoms: 10, AtomsWithText: 10, AtomsWithHash: 10, AtomsWithSpan: 10, AtomsWithParent: 8, RootAtoms: 2},
		{Category: CatRefIntegrity, TotalRefs: 5, ResolvedRefs: 5},
		{Category: CatLexiconCompliance, LexiconResult: &GateResult{Pass: true, Checked: 20}},
	}
}

func TestRunGateAllPass(t *testing.T) {
	report := RunGate(GateInput{Checks: allPassInputs(), Now: gateTime})

	if !report.Pass {
		t.Fatalf("expected pass, summary: %s", report.Summary)
	}
	if report.Format != CertReportFormat {
		t.Fatalf("format: %q", report.Format)
	}
	if len(report.Checks) != 4 {
		t.Fatalf("expected 4 checks, got %d", len(report.Checks))
	}
	if report.Score < 0.99 {
		t.Fatalf("expected score ~1.0, got %f", report.Score)
	}
	if !strings.Contains(report.Summary, "passed") {
		t.Fatalf("summary: %q", report.Summary)
	}
}

func TestRunGateASTLoss(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatASTCoverage, TotalBytes: 1000, CoveredBytes: 800, LostBytes: 200, IsLossless: false},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})

	if report.Pass {
		t.Fatal("expected fail for 20% content loss")
	}
	if report.Checks[0].Status != CheckFailed {
		t.Fatalf("status: %q", report.Checks[0].Status)
	}
	if report.Checks[0].Score < 0.79 || report.Checks[0].Score > 0.81 {
		t.Fatalf("score: %f", report.Checks[0].Score)
	}
}

func TestRunGateASTNearLossless(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatASTCoverage, TotalBytes: 1000, CoveredBytes: 960, LostBytes: 40, IsLossless: false},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})

	if !report.Pass {
		t.Fatal("near-lossless should pass (warning, not blocking)")
	}
	if report.Checks[0].Status != CheckWarning {
		t.Fatalf("status: %q", report.Checks[0].Status)
	}
}

func TestRunGateASTEmpty(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatASTCoverage, TotalBytes: 0, CoveredBytes: 0},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if report.Checks[0].Status != CheckSkipped {
		t.Fatalf("empty should be skipped: %q", report.Checks[0].Status)
	}
}

func TestRunGateAtomComplete(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatAtomComplete, TotalAtoms: 10, AtomsWithText: 10, AtomsWithHash: 10, AtomsWithSpan: 10, AtomsWithParent: 8, RootAtoms: 2},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if report.Checks[0].Status != CheckPassed {
		t.Fatalf("status: %q, msg: %s", report.Checks[0].Status, report.Checks[0].Message)
	}
}

func TestRunGateAtomIncomplete(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatAtomComplete, TotalAtoms: 10, AtomsWithText: 5, AtomsWithHash: 10, AtomsWithSpan: 10, AtomsWithParent: 8, RootAtoms: 2},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if report.Checks[0].Status == CheckPassed {
		t.Fatal("incomplete atoms should not pass")
	}
	if len(report.Checks[0].Details) == 0 {
		t.Fatal("expected details listing missing fields")
	}
}

func TestRunGateAtomEmpty(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatAtomComplete, TotalAtoms: 0},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if report.Checks[0].Status != CheckSkipped {
		t.Fatalf("status: %q", report.Checks[0].Status)
	}
}

func TestRunGateRefIntegrityPass(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatRefIntegrity, TotalRefs: 10, ResolvedRefs: 10},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if report.Checks[0].Status != CheckPassed {
		t.Fatalf("status: %q", report.Checks[0].Status)
	}
}

func TestRunGateRefIntegrityDangling(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatRefIntegrity, TotalRefs: 10, ResolvedRefs: 8, DanglingRefs: []string{"ref/a", "ref/b"}},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if report.Pass {
		t.Fatal("dangling refs should fail")
	}
	if report.Checks[0].Status != CheckFailed {
		t.Fatalf("status: %q", report.Checks[0].Status)
	}
	if len(report.Checks[0].Details) != 2 {
		t.Fatalf("expected 2 details, got %d", len(report.Checks[0].Details))
	}
}

func TestRunGateRefEmpty(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatRefIntegrity, TotalRefs: 0},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if report.Checks[0].Status != CheckSkipped {
		t.Fatalf("status: %q", report.Checks[0].Status)
	}
}

func TestRunGateLexiconPass(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatLexiconCompliance, LexiconResult: &GateResult{Pass: true, Checked: 15}},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if report.Checks[0].Status != CheckPassed {
		t.Fatalf("status: %q", report.Checks[0].Status)
	}
}

func TestRunGateLexiconDeprecated(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatLexiconCompliance, LexiconResult: &GateResult{
			Pass: false, Checked: 10,
			Findings: []Finding{
				{Word: "old-term", Code: CodeDeprecatedTerm, Blocking: true, Message: "deprecated"},
			},
		}},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if report.Pass {
		t.Fatal("deprecated lexicon term should fail gate")
	}
	if len(report.Checks[0].Details) == 0 {
		t.Fatal("expected details")
	}
}

func TestRunGateLexiconWarnings(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatLexiconCompliance, LexiconResult: &GateResult{
			Pass: true, Checked: 10,
			Findings: []Finding{
				{Word: "unknown", Code: CodeUngoverned, Blocking: false},
			},
		}},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if !report.Pass {
		t.Fatal("ungoverned warnings should not fail gate")
	}
	if report.Checks[0].Status != CheckWarning {
		t.Fatalf("status: %q", report.Checks[0].Status)
	}
}

func TestRunGateLexiconNil(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatLexiconCompliance},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if report.Checks[0].Status != CheckSkipped {
		t.Fatalf("nil lexicon should skip: %q", report.Checks[0].Status)
	}
}

func TestRunGateUnknownCategory(t *testing.T) {
	inputs := []CheckInput{
		{Category: "unknown_check"},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if report.Checks[0].Status != CheckSkipped {
		t.Fatalf("unknown should skip: %q", report.Checks[0].Status)
	}
}

func TestRunGateEmpty(t *testing.T) {
	report := RunGate(GateInput{Now: gateTime})
	if !report.Pass {
		t.Fatal("empty gate should pass")
	}
	if len(report.Checks) != 0 {
		t.Fatalf("expected 0 checks, got %d", len(report.Checks))
	}
}

func TestRunGateTimestamp(t *testing.T) {
	report := RunGate(GateInput{Now: gateTime})
	if report.GeneratedAt != "2026-05-03T14:00:00Z" {
		t.Fatalf("timestamp: %q", report.GeneratedAt)
	}
}

func TestRunGateScore(t *testing.T) {
	report := RunGate(GateInput{Checks: allPassInputs(), Now: gateTime})
	if report.Score < 0.99 {
		t.Fatalf("all-pass score should be ~1.0, got %f", report.Score)
	}
}

func TestRunGateMixedResults(t *testing.T) {
	inputs := []CheckInput{
		{Category: CatASTCoverage, TotalBytes: 100, CoveredBytes: 100, IsLossless: true},
		{Category: CatRefIntegrity, TotalRefs: 5, ResolvedRefs: 3, DanglingRefs: []string{"x", "y"}},
	}
	report := RunGate(GateInput{Checks: inputs, Now: gateTime})
	if report.Pass {
		t.Fatal("mixed with blocking failure should not pass")
	}
	if !strings.Contains(report.Summary, "ref_integrity") {
		t.Fatalf("summary should mention failed category: %q", report.Summary)
	}
}

func TestWriteCertReport(t *testing.T) {
	report := RunGate(GateInput{Checks: allPassInputs(), Now: gateTime})

	var buf bytes.Buffer
	if err := WriteCertReport(&buf, report); err != nil {
		t.Fatalf("write: %v", err)
	}

	var decoded CertificationReport
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Format != CertReportFormat {
		t.Fatalf("format: %q", decoded.Format)
	}
	if decoded.Pass != report.Pass {
		t.Fatal("round-trip pass mismatch")
	}
}

func TestChecksSorted(t *testing.T) {
	report := RunGate(GateInput{Checks: allPassInputs(), Now: gateTime})
	for i := 1; i < len(report.Checks); i++ {
		if report.Checks[i].Category < report.Checks[i-1].Category {
			t.Fatalf("checks not sorted: %q before %q",
				report.Checks[i-1].Category, report.Checks[i].Category)
		}
	}
}

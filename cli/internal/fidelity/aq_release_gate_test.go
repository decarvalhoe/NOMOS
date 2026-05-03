package fidelity

import (
	"strings"
	"testing"
)

func fullEvidence() AQEvidence {
	return AQEvidence{
		TestsPassing:          true,
		TestCount:             100,
		CoverageRatio:         0.85,
		SchemaValidation:      true,
		LexiconCompliance:     true,
		EvidenceChainComplete: true,
		FidelityGatePassed:    true,
		SelfCompliancePassed:  true,
		IndependentReview:     true,
		ReconstructionPassed:  true,
		RegulatoryPack:        true,
		SBOMPresent:           true,
		ProvenancePresent:     true,
		ApprovalSigned:        true,
	}
}

func TestAQGate_AQ5_FullEvidence(t *testing.T) {
	result := EvaluateAQGate(AQ5, fullEvidence())
	if result.Verdict != "pass" {
		t.Fatalf("expected pass, got %s", result.Verdict)
	}
	if result.AchievedLevel != "AQ-5" {
		t.Fatalf("expected AQ-5, got %s", result.AchievedLevel)
	}
	if result.CriticalFailed != 0 {
		t.Fatalf("expected 0 critical failed, got %d", result.CriticalFailed)
	}
}

func TestAQGate_AQ3_FullEvidence(t *testing.T) {
	result := EvaluateAQGate(AQ3, fullEvidence())
	if result.Verdict != "pass" {
		t.Fatalf("expected pass, got %s", result.Verdict)
	}
	// Only checks up to AQ-3 requirements.
	if result.TotalChecks > 8 {
		t.Fatalf("expected at most 8 checks for AQ-3, got %d", result.TotalChecks)
	}
}

func TestAQGate_AQ1_MinimalEvidence(t *testing.T) {
	ev := AQEvidence{TestsPassing: true, TestCount: 5}
	result := EvaluateAQGate(AQ1, ev)
	if result.Verdict != "pass" {
		t.Fatalf("expected pass for AQ-1 with tests, got %s", result.Verdict)
	}
	if result.AchievedLevel != "AQ-1" {
		t.Fatalf("expected AQ-1, got %s", result.AchievedLevel)
	}
}

func TestAQGate_AQ1_NoTests(t *testing.T) {
	ev := AQEvidence{TestsPassing: false, TestCount: 0}
	result := EvaluateAQGate(AQ1, ev)
	if result.Verdict != "fail" {
		t.Fatalf("expected fail, got %s", result.Verdict)
	}
	if result.CriticalFailed < 1 {
		t.Fatal("expected critical failure")
	}
	if result.AchievedLevel != "AQ-0" {
		t.Fatalf("expected AQ-0, got %s", result.AchievedLevel)
	}
}

func TestAQGate_AQ3_MissingLexicon(t *testing.T) {
	ev := fullEvidence()
	ev.LexiconCompliance = false
	result := EvaluateAQGate(AQ3, ev)
	if result.Verdict != "fail" {
		t.Fatalf("expected fail without lexicon, got %s", result.Verdict)
	}
	if result.AchievedLevel != "AQ-2" {
		t.Fatalf("expected AQ-2 achieved, got %s", result.AchievedLevel)
	}
}

func TestAQGate_AQ3_MissingSelfCompliance(t *testing.T) {
	ev := fullEvidence()
	ev.SelfCompliancePassed = false
	// Self-compliance is non-critical at AQ-3.
	result := EvaluateAQGate(AQ3, ev)
	if result.Verdict != "pass_with_warnings" {
		t.Fatalf("expected pass_with_warnings, got %s", result.Verdict)
	}
	if result.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", result.Failed)
	}
}

func TestAQGate_AQ4_MissingReview(t *testing.T) {
	ev := fullEvidence()
	ev.IndependentReview = false
	result := EvaluateAQGate(AQ4, ev)
	if result.Verdict != "fail" {
		t.Fatalf("expected fail without review, got %s", result.Verdict)
	}
}

func TestAQGate_AQ5_MissingApproval(t *testing.T) {
	ev := fullEvidence()
	ev.ApprovalSigned = false
	result := EvaluateAQGate(AQ5, ev)
	if result.Verdict != "fail" {
		t.Fatalf("expected fail without approval, got %s", result.Verdict)
	}
	if result.AchievedLevel != "AQ-4" {
		t.Fatalf("expected AQ-4 achieved, got %s", result.AchievedLevel)
	}
}

func TestAQGate_AQ5_MissingSBOM_NonCritical(t *testing.T) {
	ev := fullEvidence()
	ev.SBOMPresent = false
	result := EvaluateAQGate(AQ5, ev)
	// SBOM is non-critical.
	if result.Verdict != "pass_with_warnings" {
		t.Fatalf("expected pass_with_warnings, got %s", result.Verdict)
	}
}

func TestAQGate_AQ2_LowCoverage(t *testing.T) {
	ev := AQEvidence{
		TestsPassing:     true,
		TestCount:        10,
		CoverageRatio:    0.40,
		SchemaValidation: true,
	}
	result := EvaluateAQGate(AQ2, ev)
	if result.Verdict != "fail" {
		t.Fatalf("expected fail for low coverage, got %s", result.Verdict)
	}
	if result.AchievedLevel != "AQ-1" {
		t.Fatalf("expected AQ-1, got %s", result.AchievedLevel)
	}
}

func TestAQGate_AQ2_GoodCoverage(t *testing.T) {
	ev := AQEvidence{
		TestsPassing:     true,
		TestCount:        10,
		CoverageRatio:    0.75,
		SchemaValidation: true,
	}
	result := EvaluateAQGate(AQ2, ev)
	if result.Verdict != "pass" {
		t.Fatalf("expected pass, got %s", result.Verdict)
	}
	if result.AchievedLevel != "AQ-2" {
		t.Fatalf("expected AQ-2, got %s", result.AchievedLevel)
	}
}

func TestAQGate_AchievedLevelProgressive(t *testing.T) {
	// Evidence for AQ-3 but target AQ-5 → achieved should be AQ-3.
	ev := AQEvidence{
		TestsPassing:          true,
		TestCount:             50,
		CoverageRatio:         0.80,
		SchemaValidation:      true,
		LexiconCompliance:     true,
		EvidenceChainComplete: true,
		FidelityGatePassed:    true,
		SelfCompliancePassed:  true,
	}
	result := EvaluateAQGate(AQ5, ev)
	if result.AchievedLevel != "AQ-3" {
		t.Fatalf("expected AQ-3, got %s", result.AchievedLevel)
	}
}

func TestAQGate_ZeroTarget(t *testing.T) {
	result := EvaluateAQGate(AQ0, AQEvidence{})
	if result.Verdict != "pass" {
		t.Fatalf("expected pass for AQ-0, got %s", result.Verdict)
	}
	if result.TotalChecks != 0 {
		t.Fatalf("expected 0 checks for AQ-0, got %d", result.TotalChecks)
	}
}

func TestAQLevel_String(t *testing.T) {
	cases := map[AQLevel]string{AQ0: "AQ-0", AQ1: "AQ-1", AQ3: "AQ-3", AQ5: "AQ-5"}
	for level, expected := range cases {
		if level.String() != expected {
			t.Fatalf("expected %s, got %s", expected, level.String())
		}
	}
}

func TestDefaultAQRequirements_Count(t *testing.T) {
	reqs := DefaultAQRequirements()
	if len(reqs) != 14 {
		t.Fatalf("expected 14 requirements, got %d", len(reqs))
	}
}

func TestFormatGateReport(t *testing.T) {
	result := EvaluateAQGate(AQ3, fullEvidence())
	report := FormatGateReport(result)
	if !strings.Contains(report, "PASS") {
		t.Fatal("expected PASS in report")
	}
	if !strings.Contains(report, "AQ-3") {
		t.Fatal("expected AQ-3 in report")
	}
}

func TestFormatGateReport_Failure(t *testing.T) {
	result := EvaluateAQGate(AQ3, AQEvidence{})
	report := FormatGateReport(result)
	if !strings.Contains(report, "FAIL") {
		t.Fatal("expected FAIL in report")
	}
	if !strings.Contains(report, "CRITICAL") {
		t.Fatal("expected CRITICAL in report")
	}
}

func TestEvaluateAQGateWith_CustomRequirements(t *testing.T) {
	custom := []AQRequirement{
		{ID: "CUSTOM-1", Description: "Custom check", MinLevel: AQ1, Critical: true},
	}
	ev := AQEvidence{} // won't satisfy unknown requirement
	result := EvaluateAQGateWith(AQ1, ev, custom)
	if result.TotalChecks != 1 {
		t.Fatalf("expected 1 check, got %d", result.TotalChecks)
	}
}

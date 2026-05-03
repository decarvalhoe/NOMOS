package fidelity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var proofTime = time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)

func fullAQ3Input() AQ3ProofInput {
	return AQ3ProofInput{
		Domain:     "insurance",
		SourceHash: "sha256:abc",
		Now:        proofTime,

		ASTTotalBytes:   10000,
		ASTCoveredBytes: 9500,
		ASTIsLossless:   false,

		TotalAtoms:      100,
		AtomsWithText:   100,
		AtomsWithHash:   100,
		AtomsWithSpan:   100,
		AtomsWithParent: 95,
		RootAtoms:       5,

		FidelityGatePass:  true,
		FidelityGateScore: 0.92,
		FidelityChecks: []CheckResult{
			{Category: CatASTCoverage, Status: CheckPassed, Score: 0.95, Message: "AST coverage OK"},
			{Category: CatAtomComplete, Status: CheckPassed, Score: 0.98, Message: "Atoms complete"},
		},

		LexiconGoverned: 45,
		LexiconTotal:    50,

		SelfCompliancePass:      true,
		SelfComplianceControls:  13,
		SelfComplianceSatisfied: 13,

		EvidenceChainComplete: true,
		ValidationEntries:     24,
		ReconstructedEntries:  24,
	}
}

func TestGenerateAQ3Proof_FullPass(t *testing.T) {
	report := GenerateAQ3Proof(fullAQ3Input())

	if !report.Achieved {
		t.Fatalf("expected AQ-3 achieved, got false: %s", report.Summary)
	}
	if report.TargetLevel != "AQ-3" {
		t.Fatalf("expected target AQ-3, got %s", report.TargetLevel)
	}
	if report.Format != AQ3ProofFormat {
		t.Fatalf("expected format %s, got %s", AQ3ProofFormat, report.Format)
	}
	if report.Domain != "insurance" {
		t.Fatalf("expected domain insurance, got %s", report.Domain)
	}
	if report.OverallScore <= 0 {
		t.Fatalf("expected positive score, got %f", report.OverallScore)
	}
	if len(report.Sections) != 6 {
		t.Fatalf("expected 6 sections, got %d", len(report.Sections))
	}
	for _, s := range report.Sections {
		if !s.Passed {
			t.Fatalf("section %s should pass, got score %.2f", s.Name, s.Score)
		}
	}
}

func TestGenerateAQ3Proof_LowCoverage(t *testing.T) {
	input := fullAQ3Input()
	input.ASTCoveredBytes = 5000 // 50% < 90% threshold
	report := GenerateAQ3Proof(input)

	if report.Achieved {
		t.Fatal("expected not achieved with low coverage")
	}

	var structSection *AQ3Section
	for i := range report.Sections {
		if report.Sections[i].Name == "structure_fidelity" {
			structSection = &report.Sections[i]
			break
		}
	}
	if structSection == nil || structSection.Passed {
		t.Fatal("expected structure_fidelity to fail")
	}
}

func TestGenerateAQ3Proof_IncompleteAtoms(t *testing.T) {
	input := fullAQ3Input()
	input.AtomsWithText = 50 // 50% < 95% threshold
	report := GenerateAQ3Proof(input)

	if report.Achieved {
		t.Fatal("expected not achieved with incomplete atoms")
	}
}

func TestGenerateAQ3Proof_FidelityGateFail(t *testing.T) {
	input := fullAQ3Input()
	input.FidelityGatePass = false
	input.FidelityGateScore = 0.40
	report := GenerateAQ3Proof(input)

	if report.Achieved {
		t.Fatal("expected not achieved with fidelity gate failure")
	}
}

func TestGenerateAQ3Proof_LowLexicon(t *testing.T) {
	input := fullAQ3Input()
	input.LexiconGoverned = 10 // 20% < 80% threshold
	report := GenerateAQ3Proof(input)

	if report.Achieved {
		t.Fatal("expected not achieved with low lexicon")
	}
}

func TestGenerateAQ3Proof_SelfComplianceFail(t *testing.T) {
	input := fullAQ3Input()
	input.SelfCompliancePass = false
	input.SelfComplianceSatisfied = 5
	report := GenerateAQ3Proof(input)

	if report.Achieved {
		t.Fatal("expected not achieved without self-compliance")
	}
}

func TestGenerateAQ3Proof_EvidenceChainIncomplete(t *testing.T) {
	input := fullAQ3Input()
	input.EvidenceChainComplete = false
	input.ReconstructedEntries = 10
	report := GenerateAQ3Proof(input)

	if report.Achieved {
		t.Fatal("expected not achieved with incomplete chain")
	}
}

func TestGenerateAQ3Proof_ZeroAtoms(t *testing.T) {
	input := fullAQ3Input()
	input.TotalAtoms = 0
	report := GenerateAQ3Proof(input)

	if report.Achieved {
		t.Fatal("expected not achieved with zero atoms")
	}
}

func TestGenerateAQ3Proof_ZeroASTBytes(t *testing.T) {
	input := fullAQ3Input()
	input.ASTTotalBytes = 0
	report := GenerateAQ3Proof(input)

	if report.Achieved {
		t.Fatal("expected not achieved with zero AST bytes")
	}
}

func TestGenerateAQ3Proof_SummaryContainsAchieved(t *testing.T) {
	report := GenerateAQ3Proof(fullAQ3Input())
	if !strings.Contains(report.Summary, "ACHIEVED") {
		t.Fatalf("expected ACHIEVED in summary, got %q", report.Summary)
	}
}

func TestGenerateAQ3Proof_SummaryContainsNotAchieved(t *testing.T) {
	input := fullAQ3Input()
	input.FidelityGatePass = false
	report := GenerateAQ3Proof(input)
	if !strings.Contains(report.Summary, "NOT ACHIEVED") {
		t.Fatalf("expected NOT ACHIEVED in summary, got %q", report.Summary)
	}
	if !strings.Contains(report.Summary, "fidelity_gate") {
		t.Fatal("expected failed section name in summary")
	}
}

func TestGenerateAQ3Proof_SectionNames(t *testing.T) {
	report := GenerateAQ3Proof(fullAQ3Input())
	expected := []string{
		"structure_fidelity", "atom_completeness", "fidelity_gate",
		"lexicon_compliance", "self_compliance", "evidence_chain",
	}
	for i, name := range expected {
		if report.Sections[i].Name != name {
			t.Fatalf("section[%d] expected %q, got %q", i, name, report.Sections[i].Name)
		}
	}
}

func TestGenerateAQ3Proof_FidelityCheckDetails(t *testing.T) {
	report := GenerateAQ3Proof(fullAQ3Input())
	var gate *AQ3Section
	for i := range report.Sections {
		if report.Sections[i].Name == "fidelity_gate" {
			gate = &report.Sections[i]
			break
		}
	}
	if gate == nil {
		t.Fatal("fidelity_gate section not found")
	}
	// Should include sub-check details.
	found := false
	for _, d := range gate.Details {
		if strings.Contains(d, "AST coverage OK") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected fidelity check details in section")
	}
}

func TestWriteAQ3Proof(t *testing.T) {
	report := GenerateAQ3Proof(fullAQ3Input())
	var buf bytes.Buffer
	if err := WriteAQ3Proof(&buf, report); err != nil {
		t.Fatalf("WriteAQ3Proof: %v", err)
	}
	var decoded AQ3ProofReport
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Format != AQ3ProofFormat {
		t.Fatalf("format mismatch: %s", decoded.Format)
	}
	if decoded.TargetLevel != "AQ-3" {
		t.Fatalf("target mismatch: %s", decoded.TargetLevel)
	}
}

func TestGenerateAQ3Proof_Timestamp(t *testing.T) {
	report := GenerateAQ3Proof(fullAQ3Input())
	if report.GeneratedAt != "2026-05-03T12:00:00Z" {
		t.Fatalf("expected fixed timestamp, got %s", report.GeneratedAt)
	}
}

func TestGenerateAQ3Proof_Lossless(t *testing.T) {
	input := fullAQ3Input()
	input.ASTIsLossless = true
	input.ASTCoveredBytes = input.ASTTotalBytes
	report := GenerateAQ3Proof(input)

	var structSection *AQ3Section
	for i := range report.Sections {
		if report.Sections[i].Name == "structure_fidelity" {
			structSection = &report.Sections[i]
			break
		}
	}
	found := false
	for _, d := range structSection.Details {
		if strings.Contains(d, "lossless: true") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected lossless detail")
	}
}

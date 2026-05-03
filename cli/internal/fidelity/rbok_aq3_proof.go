package fidelity

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const AQ3ProofFormat = "nomos.rbok-aq3-proof.v1"

// AQ3Section is a named section of the proof report.
type AQ3Section struct {
	Name     string   `json:"name"`
	Passed   bool     `json:"passed"`
	Score    float64  `json:"score"`
	Details  []string `json:"details"`
}

// AQ3ProofReport is the complete AQ-3 proof for an RBOK corpus.
type AQ3ProofReport struct {
	Format       string       `json:"format"`
	GeneratedAt  string       `json:"generated_at"`
	Domain       string       `json:"domain"`
	SourceHash   string       `json:"source_hash,omitempty"`
	TargetLevel  string       `json:"target_level"`
	Achieved     bool         `json:"achieved"`
	OverallScore float64      `json:"overall_score"`
	Sections     []AQ3Section `json:"sections"`
	Summary      string       `json:"summary"`
}

// AQ3ProofInput holds the evidence data required for AQ-3 proof.
type AQ3ProofInput struct {
	Domain     string
	SourceHash string
	Now        time.Time

	// Structure fidelity
	ASTTotalBytes   int
	ASTCoveredBytes int
	ASTIsLossless   bool

	// Atom completeness
	TotalAtoms      int
	AtomsWithText   int
	AtomsWithHash   int
	AtomsWithSpan   int
	AtomsWithParent int
	RootAtoms       int

	// Fidelity gate
	FidelityGatePass  bool
	FidelityGateScore float64
	FidelityChecks    []CheckResult

	// Lexicon compliance
	LexiconGoverned int
	LexiconTotal    int

	// Self-compliance
	SelfCompliancePass bool
	SelfComplianceControls int
	SelfComplianceSatisfied int

	// Evidence chain
	EvidenceChainComplete bool
	ValidationEntries     int
	ReconstructedEntries  int
}

// GenerateAQ3Proof builds the AQ-3 proof report from evidence inputs.
func GenerateAQ3Proof(input AQ3ProofInput) AQ3ProofReport {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	sections := []AQ3Section{
		evalStructureFidelity(input),
		evalAtomComplete(input),
		evalFidelityGateSection(input),
		evalLexiconSection(input),
		evalSelfComplianceSection(input),
		evalEvidenceChainSection(input),
	}

	allPass := true
	totalScore := 0.0
	for _, s := range sections {
		if !s.Passed {
			allPass = false
		}
		totalScore += s.Score
	}
	overallScore := totalScore / float64(len(sections))

	summary := buildAQ3Summary(sections, allPass, overallScore)

	return AQ3ProofReport{
		Format:       AQ3ProofFormat,
		GeneratedAt:  now.Format(time.RFC3339),
		Domain:       input.Domain,
		SourceHash:   input.SourceHash,
		TargetLevel:  "AQ-3",
		Achieved:     allPass,
		OverallScore: overallScore,
		Sections:     sections,
		Summary:      summary,
	}
}

// WriteAQ3Proof writes the report as indented JSON.
func WriteAQ3Proof(w io.Writer, report AQ3ProofReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func evalStructureFidelity(input AQ3ProofInput) AQ3Section {
	s := AQ3Section{Name: "structure_fidelity"}

	if input.ASTTotalBytes == 0 {
		s.Score = 0
		s.Details = append(s.Details, "no AST bytes — source not parsed")
		return s
	}

	coverageRatio := float64(input.ASTCoveredBytes) / float64(input.ASTTotalBytes)
	s.Score = coverageRatio
	s.Details = append(s.Details, fmt.Sprintf("coverage: %.1f%% (%d/%d bytes)",
		coverageRatio*100, input.ASTCoveredBytes, input.ASTTotalBytes))

	if input.ASTIsLossless {
		s.Details = append(s.Details, "lossless: true")
	} else {
		lostBytes := input.ASTTotalBytes - input.ASTCoveredBytes
		s.Details = append(s.Details, fmt.Sprintf("lost bytes: %d", lostBytes))
	}

	s.Passed = coverageRatio >= 0.90
	if !s.Passed {
		s.Details = append(s.Details, "FAIL: coverage below 90% threshold")
	}
	return s
}

func evalAtomComplete(input AQ3ProofInput) AQ3Section {
	s := AQ3Section{Name: "atom_completeness"}

	if input.TotalAtoms == 0 {
		s.Score = 0
		s.Details = append(s.Details, "no atoms extracted")
		return s
	}

	checks := []struct {
		name  string
		count int
	}{
		{"with_text", input.AtomsWithText},
		{"with_hash", input.AtomsWithHash},
		{"with_span", input.AtomsWithSpan},
		{"with_parent", input.AtomsWithParent + input.RootAtoms},
	}

	totalRatio := 0.0
	for _, c := range checks {
		ratio := float64(c.count) / float64(input.TotalAtoms)
		totalRatio += ratio
		s.Details = append(s.Details, fmt.Sprintf("%s: %d/%d (%.0f%%)",
			c.name, c.count, input.TotalAtoms, ratio*100))
	}
	s.Score = totalRatio / float64(len(checks))
	s.Passed = s.Score >= 0.95
	if !s.Passed {
		s.Details = append(s.Details, "FAIL: atom completeness below 95%")
	}
	return s
}

func evalFidelityGateSection(input AQ3ProofInput) AQ3Section {
	s := AQ3Section{Name: "fidelity_gate"}
	s.Passed = input.FidelityGatePass
	s.Score = input.FidelityGateScore

	if s.Passed {
		s.Details = append(s.Details, fmt.Sprintf("gate passed (score: %.2f)", input.FidelityGateScore))
	} else {
		s.Details = append(s.Details, "FAIL: fidelity gate did not pass")
	}

	for _, c := range input.FidelityChecks {
		status := "PASS"
		if c.Status == CheckFailed {
			status = "FAIL"
		} else if c.Status == CheckWarning {
			status = "WARN"
		}
		s.Details = append(s.Details, fmt.Sprintf("  [%s] %s: %s", status, c.Category, c.Message))
	}
	return s
}

func evalLexiconSection(input AQ3ProofInput) AQ3Section {
	s := AQ3Section{Name: "lexicon_compliance"}

	if input.LexiconTotal == 0 {
		s.Score = 0
		s.Details = append(s.Details, "no lexicon terms checked")
		return s
	}

	ratio := float64(input.LexiconGoverned) / float64(input.LexiconTotal)
	s.Score = ratio
	s.Passed = ratio >= 0.80
	s.Details = append(s.Details, fmt.Sprintf("governed: %d/%d (%.0f%%)",
		input.LexiconGoverned, input.LexiconTotal, ratio*100))
	if !s.Passed {
		s.Details = append(s.Details, "FAIL: lexicon governance below 80%")
	}
	return s
}

func evalSelfComplianceSection(input AQ3ProofInput) AQ3Section {
	s := AQ3Section{Name: "self_compliance"}
	s.Passed = input.SelfCompliancePass

	if input.SelfComplianceControls == 0 {
		s.Score = 0
		s.Details = append(s.Details, "no controls evaluated")
		return s
	}

	ratio := float64(input.SelfComplianceSatisfied) / float64(input.SelfComplianceControls)
	s.Score = ratio
	s.Details = append(s.Details, fmt.Sprintf("controls: %d/%d satisfied",
		input.SelfComplianceSatisfied, input.SelfComplianceControls))
	if !s.Passed {
		s.Details = append(s.Details, "FAIL: self-compliance did not pass")
	}
	return s
}

func evalEvidenceChainSection(input AQ3ProofInput) AQ3Section {
	s := AQ3Section{Name: "evidence_chain"}
	s.Passed = input.EvidenceChainComplete

	if input.ValidationEntries == 0 {
		s.Score = 0
		s.Details = append(s.Details, "no validation entries")
		return s
	}

	ratio := float64(input.ReconstructedEntries) / float64(input.ValidationEntries)
	s.Score = ratio
	s.Details = append(s.Details, fmt.Sprintf("reconstructed: %d/%d entries (%.0f%%)",
		input.ReconstructedEntries, input.ValidationEntries, ratio*100))
	if !s.Passed {
		s.Details = append(s.Details, "FAIL: evidence chain incomplete")
	}
	return s
}

func buildAQ3Summary(sections []AQ3Section, allPass bool, score float64) string {
	var b strings.Builder
	if allPass {
		fmt.Fprintf(&b, "AQ-3 ACHIEVED — overall score %.2f\n", score)
	} else {
		fmt.Fprintf(&b, "AQ-3 NOT ACHIEVED — overall score %.2f\n", score)
		fmt.Fprintln(&b, "Failed sections:")
		for _, s := range sections {
			if !s.Passed {
				fmt.Fprintf(&b, "  - %s (score: %.2f)\n", s.Name, s.Score)
			}
		}
	}
	return strings.TrimSpace(b.String())
}

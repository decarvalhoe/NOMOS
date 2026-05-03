package fidelity

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

const CertReportFormat = "nomos.fidelity-certification.v1"

// CheckCategory groups fidelity checks.
type CheckCategory string

const (
	CatASTCoverage     CheckCategory = "ast_coverage"
	CatAtomComplete    CheckCategory = "atom_completeness"
	CatRefIntegrity    CheckCategory = "ref_integrity"
	CatLexiconCompliance CheckCategory = "lexicon_compliance"
)

// CheckStatus is the outcome of a single check.
type CheckStatus string

const (
	CheckPassed  CheckStatus = "passed"
	CheckWarning CheckStatus = "warning"
	CheckFailed  CheckStatus = "failed"
	CheckSkipped CheckStatus = "skipped"
)

// CheckInput provides data for one fidelity check.
type CheckInput struct {
	Category CheckCategory
	// AST coverage
	TotalBytes   int
	CoveredBytes int
	LostBytes    int
	IsLossless   bool
	// Atom completeness
	TotalAtoms      int
	AtomsWithText   int
	AtomsWithHash   int
	AtomsWithSpan   int
	AtomsWithParent int
	RootAtoms       int
	// Ref integrity
	TotalRefs     int
	ResolvedRefs  int
	DanglingRefs  []string
	// Lexicon
	LexiconResult *GateResult
}

// CheckResult is the outcome of one fidelity check.
type CheckResult struct {
	Category CheckCategory `json:"category"`
	Status   CheckStatus   `json:"status"`
	Score    float64       `json:"score"`
	Message  string        `json:"message"`
	Details  []string      `json:"details,omitempty"`
	Blocking bool          `json:"blocking"`
}

// CertificationReport is the full fidelity gate output.
type CertificationReport struct {
	Format      string        `json:"format"`
	GeneratedAt string        `json:"generated_at"`
	Pass        bool          `json:"pass"`
	Score       float64       `json:"score"`
	Checks      []CheckResult `json:"checks"`
	Summary     string        `json:"summary"`
}

// GateInput configures a fidelity gate run.
type GateInput struct {
	Checks []CheckInput
	Now    time.Time
}

// RunGate executes all fidelity checks and produces a certification report.
func RunGate(input GateInput) CertificationReport {
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var results []CheckResult
	for _, check := range input.Checks {
		results = append(results, evaluate(check))
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Category < results[j].Category
	})

	pass := true
	totalScore := 0.0
	for _, r := range results {
		if r.Blocking && r.Status == CheckFailed {
			pass = false
		}
		totalScore += r.Score
	}

	avgScore := 0.0
	if len(results) > 0 {
		avgScore = totalScore / float64(len(results))
	}

	summary := "All fidelity checks passed."
	if !pass {
		var failed []string
		for _, r := range results {
			if r.Status == CheckFailed {
				failed = append(failed, string(r.Category))
			}
		}
		summary = fmt.Sprintf("Fidelity gate failed: %s", strings.Join(failed, ", "))
	}

	return CertificationReport{
		Format:      CertReportFormat,
		GeneratedAt: now.Format(time.RFC3339),
		Pass:        pass,
		Score:       avgScore,
		Checks:      results,
		Summary:     summary,
	}
}

// WriteCertReport writes the report as indented JSON.
func WriteCertReport(w io.Writer, report CertificationReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func evaluate(c CheckInput) CheckResult {
	switch c.Category {
	case CatASTCoverage:
		return evalASTCoverage(c)
	case CatAtomComplete:
		return evalAtomCompleteness(c)
	case CatRefIntegrity:
		return evalRefIntegrity(c)
	case CatLexiconCompliance:
		return evalLexicon(c)
	default:
		return CheckResult{
			Category: c.Category,
			Status:   CheckSkipped,
			Message:  fmt.Sprintf("unknown category %q", c.Category),
		}
	}
}

func evalASTCoverage(c CheckInput) CheckResult {
	r := CheckResult{
		Category: CatASTCoverage,
		Blocking: true,
	}

	if c.TotalBytes == 0 {
		r.Status = CheckSkipped
		r.Score = 1.0
		r.Message = "empty source, nothing to check"
		return r
	}

	ratio := float64(c.CoveredBytes) / float64(c.TotalBytes)
	r.Score = ratio

	if c.IsLossless {
		r.Status = CheckPassed
		r.Message = fmt.Sprintf("lossless: %d/%d bytes covered (100%%)", c.CoveredBytes, c.TotalBytes)
	} else if ratio >= 0.95 {
		r.Status = CheckWarning
		r.Message = fmt.Sprintf("near-lossless: %d/%d bytes covered (%.1f%%), %d bytes lost",
			c.CoveredBytes, c.TotalBytes, ratio*100, c.LostBytes)
		r.Blocking = false
	} else {
		r.Status = CheckFailed
		r.Message = fmt.Sprintf("content loss: %d/%d bytes covered (%.1f%%), %d bytes lost",
			c.CoveredBytes, c.TotalBytes, ratio*100, c.LostBytes)
	}

	return r
}

func evalAtomCompleteness(c CheckInput) CheckResult {
	r := CheckResult{
		Category: CatAtomComplete,
		Blocking: true,
	}

	if c.TotalAtoms == 0 {
		r.Status = CheckSkipped
		r.Score = 1.0
		r.Message = "no atoms to check"
		return r
	}

	checks := []struct {
		name  string
		count int
	}{
		{"text", c.AtomsWithText},
		{"hash", c.AtomsWithHash},
		{"span", c.AtomsWithSpan},
	}

	totalFields := 0
	presentFields := 0
	var missing []string

	for _, ch := range checks {
		totalFields += c.TotalAtoms
		presentFields += ch.count
		if ch.count < c.TotalAtoms {
			missing = append(missing, fmt.Sprintf("%s: %d/%d", ch.name, ch.count, c.TotalAtoms))
		}
	}

	// Parent check: all non-root atoms should have a parent.
	expectedWithParent := c.TotalAtoms - c.RootAtoms
	if expectedWithParent > 0 {
		totalFields += expectedWithParent
		presentFields += c.AtomsWithParent
		if c.AtomsWithParent < expectedWithParent {
			missing = append(missing, fmt.Sprintf("parent: %d/%d", c.AtomsWithParent, expectedWithParent))
		}
	}

	r.Score = float64(presentFields) / float64(totalFields)
	r.Details = missing

	if len(missing) == 0 {
		r.Status = CheckPassed
		r.Message = fmt.Sprintf("all %d atoms complete", c.TotalAtoms)
	} else if r.Score >= 0.90 {
		r.Status = CheckWarning
		r.Message = fmt.Sprintf("atom completeness %.1f%%: %s", r.Score*100, strings.Join(missing, ", "))
		r.Blocking = false
	} else {
		r.Status = CheckFailed
		r.Message = fmt.Sprintf("atom completeness %.1f%%: %s", r.Score*100, strings.Join(missing, ", "))
	}

	return r
}

func evalRefIntegrity(c CheckInput) CheckResult {
	r := CheckResult{
		Category: CatRefIntegrity,
		Blocking: true,
	}

	if c.TotalRefs == 0 {
		r.Status = CheckSkipped
		r.Score = 1.0
		r.Message = "no references to check"
		return r
	}

	r.Score = float64(c.ResolvedRefs) / float64(c.TotalRefs)

	if len(c.DanglingRefs) == 0 {
		r.Status = CheckPassed
		r.Message = fmt.Sprintf("all %d references resolved", c.TotalRefs)
	} else {
		r.Status = CheckFailed
		r.Message = fmt.Sprintf("%d/%d references dangling", len(c.DanglingRefs), c.TotalRefs)
		for _, ref := range c.DanglingRefs {
			r.Details = append(r.Details, "dangling: "+ref)
		}
	}

	return r
}

func evalLexicon(c CheckInput) CheckResult {
	r := CheckResult{
		Category: CatLexiconCompliance,
		Blocking: true,
	}

	if c.LexiconResult == nil {
		r.Status = CheckSkipped
		r.Score = 1.0
		r.Message = "no lexicon result provided"
		return r
	}

	lr := c.LexiconResult

	if lr.Checked == 0 {
		r.Status = CheckSkipped
		r.Score = 1.0
		r.Message = "no terms checked"
		return r
	}

	blockingCount := 0
	warningCount := 0
	for _, f := range lr.Findings {
		if f.Blocking {
			blockingCount++
		} else {
			warningCount++
		}
	}

	r.Score = float64(lr.Checked-blockingCount) / float64(lr.Checked)

	if blockingCount > 0 {
		r.Status = CheckFailed
		r.Message = fmt.Sprintf("%d blocking lexicon finding(s), %d warnings", blockingCount, warningCount)
		for _, f := range lr.Findings {
			if f.Blocking {
				r.Details = append(r.Details, fmt.Sprintf("[%s] %s", f.Code, f.Message))
			}
		}
	} else if warningCount > 0 {
		r.Status = CheckWarning
		r.Message = fmt.Sprintf("lexicon clean but %d ungoverned term(s)", warningCount)
		r.Blocking = false
	} else {
		r.Status = CheckPassed
		r.Message = fmt.Sprintf("all %d terms governed", lr.Checked)
	}

	return r
}

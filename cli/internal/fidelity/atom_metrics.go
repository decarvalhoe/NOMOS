package fidelity

import (
	"fmt"
	"strings"
)

// AtomInput is the minimal atom representation for quality scoring.
type AtomInput struct {
	AtomID       string   `json:"atom_id"`
	Kind         string   `json:"kind"`
	Title        string   `json:"title"`
	Content      string   `json:"content"`
	SourceNodeID string   `json:"source_node_id,omitempty"`
	SourceHash   string   `json:"source_hash,omitempty"`
	ReviewState  string   `json:"review_state,omitempty"`
	RefIDs       []string `json:"ref_ids,omitempty"`
	TestRefs     []string `json:"test_refs,omitempty"`
	ContractRef  string   `json:"contract_ref,omitempty"`
}

// AtomMetrics holds quality scores for a single atom.
type AtomMetrics struct {
	AtomID             string  `json:"atom_id"`
	Completeness       float64 `json:"completeness"`        // 0-1: required fields present
	Coherence          float64 `json:"coherence"`           // 0-1: content quality
	SourceCoverage     float64 `json:"source_coverage"`     // 0-1: source provenance
	SemanticRoleCoverage float64 `json:"semantic_role_coverage"` // 0-1: downstream refs
	OverallScore       float64 `json:"overall_score"`       // weighted average
	Grade              string  `json:"grade"`               // A/B/C/D/F
	Issues             []string `json:"issues,omitempty"`
}

// AtomMetricsReport aggregates metrics across a set of atoms.
type AtomMetricsReport struct {
	SchemaVersion  string        `json:"schema_version"`
	TotalAtoms     int           `json:"total_atoms"`
	AverageScore   float64       `json:"average_score"`
	GradeDistribution map[string]int `json:"grade_distribution"`
	Metrics        []AtomMetrics `json:"metrics"`
}

// MetricsConfig configures scoring weights.
type MetricsConfig struct {
	WeightCompleteness   float64
	WeightCoherence      float64
	WeightSourceCoverage float64
	WeightSemanticRole   float64
	MinContentLength     int
}

// DefaultMetricsConfig returns balanced default weights.
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		WeightCompleteness:   0.30,
		WeightCoherence:      0.25,
		WeightSourceCoverage: 0.25,
		WeightSemanticRole:   0.20,
		MinContentLength:     20,
	}
}

// ScoreAtom computes quality metrics for a single atom.
func ScoreAtom(atom AtomInput, config MetricsConfig) AtomMetrics {
	var issues []string

	completeness := scoreCompleteness(atom, &issues)
	coherence := scoreCoherence(atom, config, &issues)
	sourceCoverage := scoreSourceCoverage(atom, &issues)
	semanticRole := scoreSemanticRoleCoverage(atom, &issues)

	overall := config.WeightCompleteness*completeness +
		config.WeightCoherence*coherence +
		config.WeightSourceCoverage*sourceCoverage +
		config.WeightSemanticRole*semanticRole

	return AtomMetrics{
		AtomID:               atom.AtomID,
		Completeness:         completeness,
		Coherence:            coherence,
		SourceCoverage:       sourceCoverage,
		SemanticRoleCoverage: semanticRole,
		OverallScore:         clamp(overall),
		Grade:                gradeFromScore(overall),
		Issues:               issues,
	}
}

// ScoreAtoms computes metrics for a batch and produces a report.
func ScoreAtoms(atoms []AtomInput, config MetricsConfig) AtomMetricsReport {
	metrics := make([]AtomMetrics, 0, len(atoms))
	grades := map[string]int{}
	totalScore := 0.0

	for _, atom := range atoms {
		m := ScoreAtom(atom, config)
		metrics = append(metrics, m)
		grades[m.Grade]++
		totalScore += m.OverallScore
	}

	avg := 0.0
	if len(atoms) > 0 {
		avg = totalScore / float64(len(atoms))
	}

	return AtomMetricsReport{
		SchemaVersion:     "0.1.0",
		TotalAtoms:        len(atoms),
		AverageScore:      clamp(avg),
		GradeDistribution: grades,
		Metrics:           metrics,
	}
}

func scoreCompleteness(atom AtomInput, issues *[]string) float64 {
	score := 0.0
	checks := 0

	// Required fields.
	checks++
	if atom.AtomID != "" {
		score++
	} else {
		*issues = append(*issues, "missing atom_id")
	}

	checks++
	if atom.Kind != "" {
		score++
	} else {
		*issues = append(*issues, "missing kind")
	}

	checks++
	if atom.Title != "" {
		score++
	} else {
		*issues = append(*issues, "missing title")
	}

	checks++
	if atom.Content != "" {
		score++
	} else {
		*issues = append(*issues, "missing content")
	}

	checks++
	if atom.SourceHash != "" {
		score++
	} else {
		*issues = append(*issues, "missing source_hash")
	}

	checks++
	if atom.ReviewState != "" {
		score++
	} else {
		*issues = append(*issues, "missing review_state")
	}

	if checks == 0 {
		return 0
	}
	return score / float64(checks)
}

func scoreCoherence(atom AtomInput, config MetricsConfig, issues *[]string) float64 {
	if atom.Content == "" {
		return 0
	}

	score := 0.0
	checks := 0

	// Content length.
	checks++
	minLen := config.MinContentLength
	if minLen == 0 {
		minLen = 20
	}
	if len(atom.Content) >= minLen {
		score++
	} else {
		*issues = append(*issues, fmt.Sprintf("content too short (%d < %d)", len(atom.Content), minLen))
	}

	// Title-content alignment: title words should appear in content.
	checks++
	if atom.Title != "" && titleAligns(atom.Title, atom.Content) {
		score++
	} else if atom.Title != "" {
		*issues = append(*issues, "title does not align with content")
	}

	// Not just whitespace/punctuation.
	checks++
	meaningful := strings.TrimSpace(atom.Content)
	if len(meaningful) > 5 && hasAlphaContent(meaningful) {
		score++
	}

	if checks == 0 {
		return 0
	}
	return score / float64(checks)
}

func scoreSourceCoverage(atom AtomInput, issues *[]string) float64 {
	score := 0.0
	checks := 0

	checks++
	if atom.SourceHash != "" {
		score++
	}

	checks++
	if atom.SourceNodeID != "" {
		score++
	} else {
		*issues = append(*issues, "missing source_node_id")
	}

	checks++
	if atom.ReviewState == "approved" || atom.ReviewState == "pending_review" {
		score++
	} else if atom.ReviewState == "draft" {
		score += 0.5
	}

	if checks == 0 {
		return 0
	}
	return score / float64(checks)
}

func scoreSemanticRoleCoverage(atom AtomInput, issues *[]string) float64 {
	score := 0.0
	checks := 0

	// Has references to other atoms.
	checks++
	if len(atom.RefIDs) > 0 {
		score++
	}

	// Has test coverage.
	checks++
	if len(atom.TestRefs) > 0 {
		score++
	} else {
		*issues = append(*issues, "no test references")
	}

	// Has contract mapping.
	checks++
	if atom.ContractRef != "" {
		score++
	} else {
		*issues = append(*issues, "no contract reference")
	}

	if checks == 0 {
		return 0
	}
	return score / float64(checks)
}

func titleAligns(title, content string) bool {
	titleWords := strings.Fields(strings.ToLower(title))
	contentLower := strings.ToLower(content)
	matched := 0
	for _, w := range titleWords {
		if len(w) < 3 {
			continue
		}
		if strings.Contains(contentLower, w) {
			matched++
		}
	}
	significant := 0
	for _, w := range titleWords {
		if len(w) >= 3 {
			significant++
		}
	}
	if significant == 0 {
		return true
	}
	return float64(matched)/float64(significant) >= 0.5
}

func hasAlphaContent(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= 'À' && r <= 'ÿ') {
			return true
		}
	}
	return false
}

func gradeFromScore(score float64) string {
	switch {
	case score >= 0.9:
		return "A"
	case score >= 0.75:
		return "B"
	case score >= 0.6:
		return "C"
	case score >= 0.4:
		return "D"
	default:
		return "F"
	}
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

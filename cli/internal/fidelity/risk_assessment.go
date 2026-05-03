package fidelity

import (
	"fmt"
	"sort"
	"strings"
)

// RiskSeverity classifies the impact of a failure.
type RiskSeverity string

const (
	SeverityNegligible RiskSeverity = "negligible"
	SeverityMinor      RiskSeverity = "minor"
	SeverityMajor      RiskSeverity = "major"
	SeverityCritical   RiskSeverity = "critical"
)

func (s RiskSeverity) rank() int {
	switch s {
	case SeverityNegligible:
		return 1
	case SeverityMinor:
		return 2
	case SeverityMajor:
		return 3
	case SeverityCritical:
		return 4
	default:
		return 0
	}
}

// RiskLikelihood classifies how likely a failure is.
type RiskLikelihood string

const (
	LikelihoodRare       RiskLikelihood = "rare"
	LikelihoodUnlikely   RiskLikelihood = "unlikely"
	LikelihoodPossible   RiskLikelihood = "possible"
	LikelihoodLikely     RiskLikelihood = "likely"
	LikelihoodAlmostCertain RiskLikelihood = "almost_certain"
)

func (l RiskLikelihood) rank() int {
	switch l {
	case LikelihoodRare:
		return 1
	case LikelihoodUnlikely:
		return 2
	case LikelihoodPossible:
		return 3
	case LikelihoodLikely:
		return 4
	case LikelihoodAlmostCertain:
		return 5
	default:
		return 0
	}
}

// RiskLevel is the computed risk from severity × likelihood.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// FunctionRisk describes the risk assessment for a single function.
type FunctionRisk struct {
	FunctionID  string         `json:"function_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Severity    RiskSeverity   `json:"severity"`
	Likelihood  RiskLikelihood `json:"likelihood"`
	RiskLevel   RiskLevel      `json:"risk_level"`
	RiskScore   int            `json:"risk_score"`
	Controls    []string       `json:"controls"`
	Mitigations []string       `json:"mitigations,omitempty"`
	Residual    RiskLevel      `json:"residual_risk"`
}

// Control is a risk mitigation control.
type Control struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"` // preventive, detective, corrective
	Functions   []string `json:"functions"`
	Implemented bool     `json:"implemented"`
	Verified    bool     `json:"verified"`
	Evidence    string   `json:"evidence,omitempty"`
}

// RiskAssessment is the full risk evaluation output.
type RiskAssessment struct {
	TotalFunctions   int            `json:"total_functions"`
	TotalControls    int            `json:"total_controls"`
	ByRiskLevel      map[RiskLevel]int `json:"by_risk_level"`
	UnmitigatedCount int            `json:"unmitigated_count"`
	Functions        []FunctionRisk `json:"functions"`
	Controls         []Control      `json:"controls"`
	ControlCoverage  float64        `json:"control_coverage"`
	Verdict          string         `json:"verdict"`
}

// ComputeRiskLevel returns the risk level from severity × likelihood.
func ComputeRiskLevel(severity RiskSeverity, likelihood RiskLikelihood) RiskLevel {
	score := severity.rank() * likelihood.rank()
	return scoreToLevel(score)
}

// ComputeRiskScore returns the numeric risk score.
func ComputeRiskScore(severity RiskSeverity, likelihood RiskLikelihood) int {
	return severity.rank() * likelihood.rank()
}

func scoreToLevel(score int) RiskLevel {
	switch {
	case score >= 12:
		return RiskCritical
	case score >= 8:
		return RiskHigh
	case score >= 4:
		return RiskMedium
	default:
		return RiskLow
	}
}

// AssessRisks evaluates risks for a set of functions against controls.
func AssessRisks(functions []FunctionRisk, controls []Control) RiskAssessment {
	controlMap := buildControlMap(controls)

	byLevel := map[RiskLevel]int{}
	unmitigated := 0

	for i := range functions {
		f := &functions[i]
		f.RiskScore = ComputeRiskScore(f.Severity, f.Likelihood)
		f.RiskLevel = ComputeRiskLevel(f.Severity, f.Likelihood)

		// Find mapped controls.
		f.Controls = controlMap[f.FunctionID]

		// Compute residual risk.
		if len(f.Controls) > 0 && allControlsVerified(f.Controls, controls) {
			f.Residual = reduceRisk(f.RiskLevel)
		} else if len(f.Controls) > 0 {
			f.Residual = f.RiskLevel
		} else {
			f.Residual = f.RiskLevel
			unmitigated++
		}

		byLevel[f.RiskLevel]++
	}

	coveredFunctions := 0
	for _, f := range functions {
		if len(f.Controls) > 0 {
			coveredFunctions++
		}
	}
	coverage := 0.0
	if len(functions) > 0 {
		coverage = float64(coveredFunctions) / float64(len(functions))
	}

	verdict := "acceptable"
	if unmitigated > 0 {
		verdict = "unacceptable"
	}
	for _, f := range functions {
		if f.Residual == RiskCritical {
			verdict = "unacceptable"
			break
		}
	}

	// Sort functions by risk score descending.
	sort.Slice(functions, func(i, j int) bool {
		return functions[i].RiskScore > functions[j].RiskScore
	})

	return RiskAssessment{
		TotalFunctions:   len(functions),
		TotalControls:    len(controls),
		ByRiskLevel:      byLevel,
		UnmitigatedCount: unmitigated,
		Functions:        functions,
		Controls:         controls,
		ControlCoverage:  coverage,
		Verdict:          verdict,
	}
}

func buildControlMap(controls []Control) map[string][]string {
	m := map[string][]string{}
	for _, c := range controls {
		for _, fid := range c.Functions {
			m[fid] = appendUnique(m[fid], c.ID)
		}
	}
	return m
}

func allControlsVerified(controlIDs []string, controls []Control) bool {
	byID := map[string]Control{}
	for _, c := range controls {
		byID[c.ID] = c
	}
	for _, id := range controlIDs {
		c, ok := byID[id]
		if !ok || !c.Verified {
			return false
		}
	}
	return true
}

func reduceRisk(level RiskLevel) RiskLevel {
	switch level {
	case RiskCritical:
		return RiskHigh
	case RiskHigh:
		return RiskMedium
	case RiskMedium:
		return RiskLow
	default:
		return RiskLow
	}
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

// FormatRiskSummary returns a human-readable risk summary.
func FormatRiskSummary(ra RiskAssessment) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Risk Assessment: %s\n", strings.ToUpper(ra.Verdict))
	fmt.Fprintf(&b, "Functions: %d | Controls: %d | Coverage: %.0f%%\n",
		ra.TotalFunctions, ra.TotalControls, ra.ControlCoverage*100)
	fmt.Fprintf(&b, "Unmitigated: %d\n\n", ra.UnmitigatedCount)

	for _, f := range ra.Functions {
		fmt.Fprintf(&b, "  %s [%s] score=%d residual=%s controls=%s\n",
			f.FunctionID, f.RiskLevel, f.RiskScore, f.Residual,
			strings.Join(f.Controls, ","))
	}
	return b.String()
}

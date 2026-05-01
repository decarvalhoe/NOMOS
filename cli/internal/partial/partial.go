package partial

import (
	"fmt"
	"sort"
)

const PartialFormat = "nomos.partial.v1"

// Verdict constants matching specs/verdicts.cue.
const (
	VerdictInScope    = "in_scope"
	VerdictPartial    = "partial"
	VerdictBlocked    = "blocked"
	VerdictOutOfScope = "out_of_scope"
)

// Confidence constants matching specs/verdicts.cue.
const (
	ConfidenceLow    = "low"
	ConfidenceMedium = "medium"
	ConfidenceHigh   = "high"
)

// Escalation constants matching specs/verdicts.cue.
const (
	EscalationNone            = "none"
	EscalationDomainOwner     = "domain_owner"
	EscalationProductOwner    = "product_owner"
	EscalationComplianceOwner = "compliance_owner"
)

// Gap represents a single coverage or traceability gap.
type Gap struct {
	ID          string `json:"id"`
	UnitID      string `json:"unit_id,omitempty"`
	Surface     string `json:"surface,omitempty"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Remediation string `json:"remediation,omitempty"`
	Source      string `json:"source"`
}

// Blocker represents a critical issue that prevents promotion even in partial mode.
type Blocker struct {
	ID          string `json:"id"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	Remediation string `json:"remediation,omitempty"`
}

// ClosurePlanItem is a single remediation action in the closure plan.
type ClosurePlanItem struct {
	GapID       string `json:"gap_id"`
	Action      string `json:"action"`
	Priority    int    `json:"priority"`
	Criticality string `json:"criticality"`
}

// PartialInput aggregates check results to evaluate for partial mode.
type PartialInput struct {
	ProjectID       string    `json:"project_id"`
	Lifecycle       string    `json:"lifecycle"`
	RiskLevel       string    `json:"risk_level"`
	ScopeVerdict    string    `json:"scope_verdict,omitempty"`
	Gaps            []Gap     `json:"gaps"`
	Blockers        []Blocker `json:"blockers"`
	CoverageScore   float64   `json:"coverage_score"`
	TotalUnits      int       `json:"total_units"`
	CoveredUnits    int       `json:"covered_units"`
	HasSourceRefs   bool      `json:"has_source_refs"`
	HasTestRefs     bool      `json:"has_test_refs"`
	HasContractRefs bool      `json:"has_contract_refs"`
}

// PartialResult is the output of a partial mode evaluation.
type PartialResult struct {
	Format      string            `json:"format"`
	ProjectID   string            `json:"project_id"`
	Verdict     string            `json:"verdict"`
	Confidence  string            `json:"confidence"`
	Escalation  string            `json:"escalation"`
	Gaps        []Gap             `json:"gaps"`
	Blockers    []Blocker         `json:"blockers"`
	ClosurePlan []ClosurePlanItem `json:"closure_plan"`
	Summary     string            `json:"summary"`
}

// Evaluate takes aggregated check inputs and produces a partial mode result.
// Core invariants from NOM-701:
//   - partial never hides gaps
//   - verdict is accompanied by a closure plan
//   - critical blockers remain blocking (verdict becomes "blocked")
func Evaluate(input PartialInput) PartialResult {
	result := PartialResult{
		Format:    PartialFormat,
		ProjectID: input.ProjectID,
		Gaps:      input.Gaps,
		Blockers:  input.Blockers,
	}

	// Determine verdict.
	result.Verdict, result.Confidence, result.Escalation = computeVerdict(input)

	// Build closure plan from gaps.
	result.ClosurePlan = buildClosurePlan(input.Gaps)

	// Generate summary.
	result.Summary = buildSummary(result)

	return result
}

func computeVerdict(input PartialInput) (verdict, confidence, escalation string) {
	// Critical blockers always force "blocked".
	if hasCriticalBlockers(input.Blockers) {
		return VerdictBlocked, ConfidenceLow, EscalationProductOwner
	}

	// Any blockers at all force "blocked".
	if len(input.Blockers) > 0 {
		return VerdictBlocked, ConfidenceLow, EscalationProductOwner
	}

	// No gaps and good coverage → in_scope.
	if len(input.Gaps) == 0 && input.CoverageScore >= 1.0 {
		return VerdictInScope, ConfidenceHigh, EscalationNone
	}

	// No gaps but coverage < 1.0 — still partial if units are missing coverage.
	if len(input.Gaps) == 0 && input.CoverageScore < 1.0 && input.TotalUnits > 0 {
		return VerdictPartial, ConfidenceMedium, EscalationDomainOwner
	}

	// Gaps present with high risk → lower confidence.
	if input.RiskLevel == "critical" || input.RiskLevel == "high" {
		return VerdictPartial, ConfidenceLow, EscalationDomainOwner
	}

	return VerdictPartial, ConfidenceMedium, EscalationDomainOwner
}

func hasCriticalBlockers(blockers []Blocker) bool {
	for _, b := range blockers {
		if b.Severity == "critical" {
			return true
		}
	}
	return false
}

func buildClosurePlan(gaps []Gap) []ClosurePlanItem {
	items := make([]ClosurePlanItem, 0, len(gaps))
	for _, g := range gaps {
		action := g.Remediation
		if action == "" {
			action = "Resolve gap: " + g.Description
		}
		items = append(items, ClosurePlanItem{
			GapID:       g.ID,
			Action:      action,
			Priority:    severityPriority(g.Severity),
			Criticality: g.Severity,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Priority < items[j].Priority
	})

	return items
}

func severityPriority(severity string) int {
	switch severity {
	case "critical":
		return 1
	case "high":
		return 2
	case "medium":
		return 3
	case "low":
		return 4
	default:
		return 5
	}
}

func buildSummary(result PartialResult) string {
	switch result.Verdict {
	case VerdictBlocked:
		return fmt.Sprintf(
			"Project %s is blocked with %d blocker(s). Resolve blockers before partial admission.",
			result.ProjectID, len(result.Blockers),
		)
	case VerdictInScope:
		return fmt.Sprintf(
			"Project %s is fully in scope with no gaps.",
			result.ProjectID,
		)
	case VerdictPartial:
		return fmt.Sprintf(
			"Project %s admitted in partial mode with %d gap(s) and %d remediation(s) in closure plan.",
			result.ProjectID, len(result.Gaps), len(result.ClosurePlan),
		)
	default:
		return fmt.Sprintf("Project %s: verdict %s.", result.ProjectID, result.Verdict)
	}
}

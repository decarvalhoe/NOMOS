package partial

import (
	"strings"
	"testing"
)

func TestEvaluateFullCoverage(t *testing.T) {
	result := Evaluate(PartialInput{
		ProjectID:     "greenfield-api",
		Lifecycle:     "greenfield",
		RiskLevel:     "low",
		CoverageScore: 1.0,
		TotalUnits:    10,
		CoveredUnits:  10,
		HasSourceRefs: true,
		HasTestRefs:   true,
	})

	assertVerdict(t, result, VerdictInScope)
	assertConfidence(t, result, ConfidenceHigh)
	assertEscalation(t, result, EscalationNone)
	if len(result.Gaps) != 0 {
		t.Fatalf("expected 0 gaps, got %d", len(result.Gaps))
	}
	if len(result.ClosurePlan) != 0 {
		t.Fatalf("expected empty closure plan, got %d items", len(result.ClosurePlan))
	}
}

func TestEvaluatePartialWithGaps(t *testing.T) {
	result := Evaluate(PartialInput{
		ProjectID:     "legacy-policy-engine",
		Lifecycle:     "brownfield",
		RiskLevel:     "medium",
		CoverageScore: 0.6,
		TotalUnits:    10,
		CoveredUnits:  6,
		HasSourceRefs: true,
		Gaps: []Gap{
			{ID: "GAP-001", Severity: "high", Description: "Missing source for exclusions", Remediation: "Complete PDF corpus"},
			{ID: "GAP-002", Severity: "medium", Description: "No golden test for deductible", Remediation: "Add golden test"},
		},
	})

	assertVerdict(t, result, VerdictPartial)
	assertConfidence(t, result, ConfidenceMedium)
	assertEscalation(t, result, EscalationDomainOwner)

	if len(result.Gaps) != 2 {
		t.Fatalf("expected 2 gaps, got %d", len(result.Gaps))
	}
	if len(result.ClosurePlan) != 2 {
		t.Fatalf("expected 2 closure plan items, got %d", len(result.ClosurePlan))
	}

	// Closure plan should be sorted by priority (high before medium).
	if result.ClosurePlan[0].Criticality != "high" {
		t.Fatalf("expected first closure item to be high criticality, got %q", result.ClosurePlan[0].Criticality)
	}
	if result.ClosurePlan[1].Criticality != "medium" {
		t.Fatalf("expected second closure item to be medium criticality, got %q", result.ClosurePlan[1].Criticality)
	}
}

func TestEvaluatePartialHighRiskLowersConfidence(t *testing.T) {
	result := Evaluate(PartialInput{
		ProjectID:     "regulated-core",
		Lifecycle:     "brownfield",
		RiskLevel:     "high",
		CoverageScore: 0.8,
		TotalUnits:    20,
		CoveredUnits:  16,
		Gaps: []Gap{
			{ID: "GAP-001", Severity: "medium", Description: "Missing test for edge case"},
		},
	})

	assertVerdict(t, result, VerdictPartial)
	assertConfidence(t, result, ConfidenceLow)
	assertEscalation(t, result, EscalationDomainOwner)
}

func TestEvaluatePartialCriticalRiskLowersConfidence(t *testing.T) {
	result := Evaluate(PartialInput{
		ProjectID:     "critical-system",
		Lifecycle:     "brownfield",
		RiskLevel:     "critical",
		CoverageScore: 0.9,
		TotalUnits:    10,
		CoveredUnits:  9,
		Gaps: []Gap{
			{ID: "GAP-001", Severity: "low", Description: "Minor doc gap"},
		},
	})

	assertVerdict(t, result, VerdictPartial)
	assertConfidence(t, result, ConfidenceLow)
}

func TestEvaluateBlockedWithBlockers(t *testing.T) {
	result := Evaluate(PartialInput{
		ProjectID:     "blocked-project",
		Lifecycle:     "brownfield",
		RiskLevel:     "high",
		CoverageScore: 0.5,
		TotalUnits:    10,
		CoveredUnits:  5,
		Blockers: []Blocker{
			{ID: "BLOCK-001", Severity: "high", Description: "No decision owner for critical calc"},
		},
		Gaps: []Gap{
			{ID: "GAP-001", Severity: "high", Description: "Missing source"},
		},
	})

	assertVerdict(t, result, VerdictBlocked)
	assertConfidence(t, result, ConfidenceLow)
	assertEscalation(t, result, EscalationProductOwner)

	if len(result.Blockers) != 1 {
		t.Fatalf("expected 1 blocker, got %d", len(result.Blockers))
	}
}

func TestEvaluateBlockedWithCriticalBlocker(t *testing.T) {
	result := Evaluate(PartialInput{
		ProjectID:     "critical-blocked",
		Lifecycle:     "brownfield",
		RiskLevel:     "low",
		CoverageScore: 0.95,
		TotalUnits:    20,
		CoveredUnits:  19,
		Blockers: []Blocker{
			{ID: "BLOCK-001", Severity: "critical", Description: "Compliance prerequisite unresolved"},
		},
	})

	assertVerdict(t, result, VerdictBlocked)
	assertConfidence(t, result, ConfidenceLow)
	assertEscalation(t, result, EscalationProductOwner)
}

func TestEvaluatePartialWithIncompleteCoverage(t *testing.T) {
	result := Evaluate(PartialInput{
		ProjectID:     "incomplete-proj",
		Lifecycle:     "brownfield",
		RiskLevel:     "low",
		CoverageScore: 0.5,
		TotalUnits:    4,
		CoveredUnits:  2,
	})

	assertVerdict(t, result, VerdictPartial)
	assertConfidence(t, result, ConfidenceMedium)
}

func TestGapsNeverHidden(t *testing.T) {
	gaps := []Gap{
		{ID: "GAP-001", Severity: "low", Description: "Minor gap"},
		{ID: "GAP-002", Severity: "high", Description: "Major gap"},
		{ID: "GAP-003", Severity: "medium", Description: "Moderate gap"},
	}

	result := Evaluate(PartialInput{
		ProjectID:     "gap-test",
		Lifecycle:     "brownfield",
		RiskLevel:     "medium",
		CoverageScore: 0.7,
		TotalUnits:    10,
		CoveredUnits:  7,
		Gaps:          gaps,
	})

	if len(result.Gaps) != 3 {
		t.Fatalf("partial must never hide gaps: expected 3, got %d", len(result.Gaps))
	}
	if len(result.ClosurePlan) != 3 {
		t.Fatalf("closure plan must cover all gaps: expected 3, got %d", len(result.ClosurePlan))
	}
}

func TestClosurePlanSortedByCriticality(t *testing.T) {
	gaps := []Gap{
		{ID: "GAP-LOW", Severity: "low", Description: "Low gap"},
		{ID: "GAP-CRIT", Severity: "critical", Description: "Critical gap", Remediation: "Fix now"},
		{ID: "GAP-MED", Severity: "medium", Description: "Medium gap"},
		{ID: "GAP-HIGH", Severity: "high", Description: "High gap"},
	}

	result := Evaluate(PartialInput{
		ProjectID:     "sort-test",
		Lifecycle:     "brownfield",
		RiskLevel:     "medium",
		CoverageScore: 0.5,
		TotalUnits:    8,
		CoveredUnits:  4,
		Gaps:          gaps,
	})

	expected := []string{"critical", "high", "medium", "low"}
	for i, item := range result.ClosurePlan {
		if item.Criticality != expected[i] {
			t.Fatalf("closure plan[%d]: expected criticality %q, got %q", i, expected[i], item.Criticality)
		}
	}
}

func TestClosurePlanDefaultAction(t *testing.T) {
	result := Evaluate(PartialInput{
		ProjectID:     "default-action",
		Lifecycle:     "brownfield",
		RiskLevel:     "low",
		CoverageScore: 0.5,
		TotalUnits:    2,
		CoveredUnits:  1,
		Gaps: []Gap{
			{ID: "GAP-001", Severity: "medium", Description: "Missing test coverage"},
		},
	})

	if len(result.ClosurePlan) != 1 {
		t.Fatalf("expected 1 closure plan item, got %d", len(result.ClosurePlan))
	}
	if !strings.HasPrefix(result.ClosurePlan[0].Action, "Resolve gap:") {
		t.Fatalf("expected default action prefix, got %q", result.ClosurePlan[0].Action)
	}
}

func TestSummaryContainsProjectID(t *testing.T) {
	result := Evaluate(PartialInput{
		ProjectID:     "my-project",
		Lifecycle:     "brownfield",
		RiskLevel:     "medium",
		CoverageScore: 0.5,
		TotalUnits:    4,
		CoveredUnits:  2,
		Gaps: []Gap{
			{ID: "GAP-001", Severity: "medium", Description: "Gap"},
		},
	})

	if !strings.Contains(result.Summary, "my-project") {
		t.Fatalf("summary should contain project ID, got %q", result.Summary)
	}
}

func TestResultFormat(t *testing.T) {
	result := Evaluate(PartialInput{
		ProjectID:     "fmt-test",
		Lifecycle:     "greenfield",
		RiskLevel:     "low",
		CoverageScore: 1.0,
		TotalUnits:    1,
		CoveredUnits:  1,
	})

	if result.Format != PartialFormat {
		t.Fatalf("expected format %q, got %q", PartialFormat, result.Format)
	}
}

// --- helpers ---

func assertVerdict(t *testing.T, result PartialResult, expected string) {
	t.Helper()
	if result.Verdict != expected {
		t.Fatalf("expected verdict %q, got %q", expected, result.Verdict)
	}
}

func assertConfidence(t *testing.T, result PartialResult, expected string) {
	t.Helper()
	if result.Confidence != expected {
		t.Fatalf("expected confidence %q, got %q", expected, result.Confidence)
	}
}

func assertEscalation(t *testing.T, result PartialResult, expected string) {
	t.Helper()
	if result.Escalation != expected {
		t.Fatalf("expected escalation %q, got %q", expected, result.Escalation)
	}
}

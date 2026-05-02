package compliance

import (
	"testing"
)

func TestAnalyzeImpactNoChanges(t *testing.T) {
	report := AnalyzeImpact(nil, DefaultTestMappings())
	if report.TotalChanges != 0 {
		t.Fatalf("expected 0 changes, got %d", report.TotalChanges)
	}
	if report.TotalImpacted != 0 {
		t.Fatalf("expected 0 impacted, got %d", report.TotalImpacted)
	}
	if report.RequiresReview {
		t.Fatal("no changes should not require review")
	}
}

func TestAnalyzeImpactSpecChange(t *testing.T) {
	changes := []SourceChange{
		{Path: "specs/nomos-report.schema.json", ChangeType: "modified"},
	}
	report := AnalyzeImpact(changes, DefaultTestMappings())

	if report.TotalImpacted == 0 {
		t.Fatal("expected impacted tests for spec change")
	}
	if report.MaxSeverity != ImpactCritical {
		t.Fatalf("expected critical severity for spec change, got %s", report.MaxSeverity)
	}
	if !report.RequiresReview {
		t.Fatal("critical change should require review")
	}

	hasSchemaTest := false
	for _, test := range report.TestsImpacted {
		if test.TestID == "praxis.schema-validation" {
			hasSchemaTest = true
		}
	}
	if !hasSchemaTest {
		t.Fatal("expected praxis.schema-validation to be impacted")
	}
}

func TestAnalyzeImpactComplianceChange(t *testing.T) {
	changes := []SourceChange{
		{Path: "cli/internal/compliance/approval.go", ChangeType: "modified"},
	}
	report := AnalyzeImpact(changes, DefaultTestMappings())

	if report.MaxSeverity != ImpactHigh {
		t.Fatalf("expected high severity, got %s", report.MaxSeverity)
	}
	if !report.RequiresReview {
		t.Fatal("high severity should require review")
	}

	found := false
	for _, test := range report.TestsImpacted {
		if test.TestID == "praxis.compliance-gate" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected praxis.compliance-gate impacted")
	}
}

func TestAnalyzeImpactDocChange(t *testing.T) {
	changes := []SourceChange{
		{Path: "docs/regulated/lifecycle/validation-master-plan.md", ChangeType: "modified"},
	}
	report := AnalyzeImpact(changes, DefaultTestMappings())

	if report.MaxSeverity != ImpactMedium {
		t.Fatalf("expected medium severity for doc change, got %s", report.MaxSeverity)
	}
	if report.RequiresReview {
		t.Fatal("medium severity should not require review")
	}
}

func TestAnalyzeImpactControlMatrixCritical(t *testing.T) {
	changes := []SourceChange{
		{Path: "docs/regulated/control-matrix/nomos-control-matrix.yaml", ChangeType: "modified"},
	}
	report := AnalyzeImpact(changes, DefaultTestMappings())

	if report.MaxSeverity != ImpactCritical {
		t.Fatalf("expected critical for control matrix, got %s", report.MaxSeverity)
	}

	found := false
	for _, test := range report.TestsImpacted {
		if test.TestID == "praxis.control-matrix-validation" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected control-matrix-validation impacted")
	}
}

func TestAnalyzeImpactMultipleChanges(t *testing.T) {
	changes := []SourceChange{
		{Path: "cli/internal/corpus/lockfile.go", ChangeType: "modified"},
		{Path: "cli/internal/corpus/diff.go", ChangeType: "modified"},
		{Path: "reports/nomos-report.json", ChangeType: "modified"},
	}
	report := AnalyzeImpact(changes, DefaultTestMappings())

	if report.TotalChanges != 3 {
		t.Fatalf("expected 3 changes, got %d", report.TotalChanges)
	}
	if report.TotalImpacted == 0 {
		t.Fatal("expected impacted tests")
	}

	// corpus changes should trigger corpus-integrity.
	found := false
	for _, test := range report.TestsImpacted {
		if test.TestID == "praxis.corpus-integrity" {
			found = true
			if len(test.TriggerPaths) < 2 {
				t.Fatalf("expected multiple trigger paths, got %d", len(test.TriggerPaths))
			}
		}
	}
	if !found {
		t.Fatal("expected corpus-integrity impacted")
	}
}

func TestAnalyzeImpactNoMatchingPattern(t *testing.T) {
	changes := []SourceChange{
		{Path: "random/unrelated/file.txt", ChangeType: "added"},
	}
	report := AnalyzeImpact(changes, DefaultTestMappings())

	if report.TotalImpacted != 0 {
		t.Fatalf("expected 0 impacted for unrelated file, got %d", report.TotalImpacted)
	}
}

func TestAnalyzeImpactProjectManifest(t *testing.T) {
	changes := []SourceChange{
		{Path: "nomos.project.yaml", ChangeType: "modified"},
	}
	report := AnalyzeImpact(changes, DefaultTestMappings())

	if report.MaxSeverity != ImpactHigh {
		t.Fatalf("expected high for manifest, got %s", report.MaxSeverity)
	}

	found := false
	for _, test := range report.TestsImpacted {
		if test.TestID == "praxis.manifest-validation" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected manifest-validation impacted")
	}
}

func TestAnalyzeImpactSortedBySeverity(t *testing.T) {
	changes := []SourceChange{
		{Path: "reports/nomos-report.json", ChangeType: "modified"},     // low
		{Path: "specs/nomos-project.cue", ChangeType: "modified"},       // critical
		{Path: "docs/regulated/lifecycle/sop.md", ChangeType: "added"},  // medium
	}
	report := AnalyzeImpact(changes, DefaultTestMappings())

	if len(report.TestsImpacted) == 0 {
		t.Fatal("expected impacted tests")
	}
	// First should be critical severity.
	if report.TestsImpacted[0].Severity != ImpactCritical {
		t.Fatalf("expected critical first, got %s", report.TestsImpacted[0].Severity)
	}
}

func TestAnalyzeImpactCustomMappings(t *testing.T) {
	mappings := []TestMapping{
		{
			SourcePattern: "custom/*",
			TestTargets:   []string{"custom.test-a", "custom.test-b"},
			Severity:      ImpactHigh,
		},
	}
	changes := []SourceChange{
		{Path: "custom/module.go", ChangeType: "modified"},
	}

	report := AnalyzeImpact(changes, mappings)

	if report.TotalImpacted != 2 {
		t.Fatalf("expected 2 impacted, got %d", report.TotalImpacted)
	}
}

func TestAnalyzeImpactAdapterChange(t *testing.T) {
	changes := []SourceChange{
		{Path: "adapters/jvm/adapter.nomos.yaml", ChangeType: "modified"},
	}
	report := AnalyzeImpact(changes, DefaultTestMappings())

	found := false
	for _, test := range report.TestsImpacted {
		if test.TestID == "praxis.adapter-contract" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected adapter-contract impacted")
	}
}

func TestAnalyzeImpactDeduplicatesTriggerPaths(t *testing.T) {
	// Same file matched by two mappings targeting the same test.
	mappings := []TestMapping{
		{SourcePattern: "src/*", TestTargets: []string{"test-a"}, Severity: ImpactLow},
		{SourcePattern: "src/*.go", TestTargets: []string{"test-a"}, Severity: ImpactHigh},
	}
	changes := []SourceChange{
		{Path: "src/main.go", ChangeType: "modified"},
	}

	report := AnalyzeImpact(changes, mappings)

	if report.TotalImpacted != 1 {
		t.Fatalf("expected 1 deduplicated test, got %d", report.TotalImpacted)
	}
	// Severity should escalate to highest.
	if report.TestsImpacted[0].Severity != ImpactHigh {
		t.Fatalf("expected escalated to high, got %s", report.TestsImpacted[0].Severity)
	}
}

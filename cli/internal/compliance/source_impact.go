package compliance

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ImpactSeverity classifies how critical a source change is.
type ImpactSeverity string

const (
	ImpactCritical ImpactSeverity = "critical"
	ImpactHigh     ImpactSeverity = "high"
	ImpactMedium   ImpactSeverity = "medium"
	ImpactLow      ImpactSeverity = "low"
)

// SourceChange represents a changed file in the repository.
type SourceChange struct {
	Path       string `json:"path"`
	ChangeType string `json:"change_type"` // added, modified, deleted
	Hash       string `json:"hash,omitempty"`
}

// TestMapping defines the relationship between source paths and test targets.
type TestMapping struct {
	SourcePattern string   `json:"source_pattern"` // glob pattern
	TestTargets   []string `json:"test_targets"`   // test identifiers
	Severity      ImpactSeverity `json:"severity"`
}

// ImpactedTest represents a test that must be re-run due to source changes.
type ImpactedTest struct {
	TestID       string         `json:"test_id"`
	Reason       string         `json:"reason"`
	Severity     ImpactSeverity `json:"severity"`
	TriggerPaths []string       `json:"trigger_paths"`
}

// ImpactReport summarizes the analysis of source changes to test impact.
type ImpactReport struct {
	SourcesChanged []SourceChange `json:"sources_changed"`
	TestsImpacted  []ImpactedTest `json:"tests_impacted"`
	TotalChanges   int            `json:"total_changes"`
	TotalImpacted  int            `json:"total_impacted"`
	MaxSeverity    ImpactSeverity `json:"max_severity"`
	RequiresReview bool           `json:"requires_review"`
}

// DefaultTestMappings returns standard source-to-test mappings for a Nomos+Praxis project.
func DefaultTestMappings() []TestMapping {
	return []TestMapping{
		{
			SourcePattern: "specs/*.cue",
			TestTargets:   []string{"praxis.schema-validation", "praxis.contract-tests"},
			Severity:      ImpactCritical,
		},
		{
			SourcePattern: "specs/*.json",
			TestTargets:   []string{"praxis.schema-validation", "praxis.contract-tests"},
			Severity:      ImpactCritical,
		},
		{
			SourcePattern: "cli/internal/compliance/*",
			TestTargets:   []string{"praxis.compliance-gate", "praxis.approval-flow"},
			Severity:      ImpactHigh,
		},
		{
			SourcePattern: "cli/internal/atomization/*",
			TestTargets:   []string{"praxis.rag-pipeline", "praxis.chunk-validation"},
			Severity:      ImpactHigh,
		},
		{
			SourcePattern: "cli/internal/corpus/*",
			TestTargets:   []string{"praxis.corpus-integrity", "praxis.feed-assembly"},
			Severity:      ImpactHigh,
		},
		{
			SourcePattern: "docs/regulated/*",
			TestTargets:   []string{"praxis.documentation-gate", "praxis.sop-coverage"},
			Severity:      ImpactMedium,
		},
		{
			SourcePattern: "docs/regulated/control-matrix/*",
			TestTargets:   []string{"praxis.control-matrix-validation", "praxis.compliance-gate"},
			Severity:      ImpactCritical,
		},
		{
			SourcePattern: "docs/regulated/reference-basis/*",
			TestTargets:   []string{"praxis.ref-alignment", "praxis.bible-integrity"},
			Severity:      ImpactHigh,
		},
		{
			SourcePattern: ".github/workflows/*",
			TestTargets:   []string{"praxis.ci-validation", "praxis.gate-execution"},
			Severity:      ImpactMedium,
		},
		{
			SourcePattern: "nomos.project.yaml",
			TestTargets:   []string{"praxis.manifest-validation", "praxis.scope-check"},
			Severity:      ImpactHigh,
		},
		{
			SourcePattern: "reports/*",
			TestTargets:   []string{"praxis.report-schema", "praxis.evidence-pack"},
			Severity:      ImpactLow,
		},
		{
			SourcePattern: "adapters/*",
			TestTargets:   []string{"praxis.adapter-contract", "praxis.adapter-smoke"},
			Severity:      ImpactMedium,
		},
	}
}

// AnalyzeImpact determines which Praxis tests are impacted by source changes.
func AnalyzeImpact(changes []SourceChange, mappings []TestMapping) ImpactReport {
	report := ImpactReport{
		SourcesChanged: changes,
		TotalChanges:   len(changes),
		MaxSeverity:    ImpactLow,
	}

	// Track impacted tests to avoid duplicates.
	impacted := map[string]*ImpactedTest{}

	for _, change := range changes {
		for _, mapping := range mappings {
			if matchPattern(change.Path, mapping.SourcePattern) {
				for _, testID := range mapping.TestTargets {
					if existing, ok := impacted[testID]; ok {
						existing.TriggerPaths = appendUnique(existing.TriggerPaths, change.Path)
						if severityRank(mapping.Severity) > severityRank(existing.Severity) {
							existing.Severity = mapping.Severity
						}
					} else {
						impacted[testID] = &ImpactedTest{
							TestID:       testID,
							Reason:       fmt.Sprintf("source change in %s", change.Path),
							Severity:     mapping.Severity,
							TriggerPaths: []string{change.Path},
						}
					}
				}
				if severityRank(mapping.Severity) > severityRank(report.MaxSeverity) {
					report.MaxSeverity = mapping.Severity
				}
			}
		}
	}

	// Collect into sorted slice.
	for _, test := range impacted {
		sort.Strings(test.TriggerPaths)
		report.TestsImpacted = append(report.TestsImpacted, *test)
	}
	sort.Slice(report.TestsImpacted, func(i, j int) bool {
		si := severityRank(report.TestsImpacted[i].Severity)
		sj := severityRank(report.TestsImpacted[j].Severity)
		if si != sj {
			return si > sj
		}
		return report.TestsImpacted[i].TestID < report.TestsImpacted[j].TestID
	})

	report.TotalImpacted = len(report.TestsImpacted)
	report.RequiresReview = report.MaxSeverity == ImpactCritical || report.MaxSeverity == ImpactHigh

	return report
}

func matchPattern(path, pattern string) bool {
	// Support directory prefix patterns (e.g., "cli/internal/corpus/*").
	if strings.HasSuffix(pattern, "/*") {
		dir := strings.TrimSuffix(pattern, "/*")
		if strings.HasPrefix(path, dir+"/") {
			return true
		}
	}
	// Glob match.
	matched, _ := filepath.Match(pattern, path)
	if matched {
		return true
	}
	// Try matching just the filename against the pattern base.
	matched, _ = filepath.Match(pattern, filepath.Base(path))
	return matched
}

func severityRank(s ImpactSeverity) int {
	switch s {
	case ImpactCritical:
		return 4
	case ImpactHigh:
		return 3
	case ImpactMedium:
		return 2
	case ImpactLow:
		return 1
	default:
		return 0
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

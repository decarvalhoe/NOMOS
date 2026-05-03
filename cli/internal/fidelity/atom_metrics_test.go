package fidelity

import (
	"testing"
)

func perfectAtom() AtomInput {
	return AtomInput{
		AtomID:       "ATOM-PERFECT",
		Kind:         "rule",
		Title:        "Water damage liability",
		Content:      "The insurer is liable for water damage when the event matches the canonical warranty conditions and no exclusion applies.",
		SourceNodeID: "NODE-123",
		SourceHash:   "sha256:aabb",
		ReviewState:  "approved",
		RefIDs:       []string{"ATOM-DEP-1"},
		TestRefs:     []string{"tests/golden/water-damage.yaml"},
		ContractRef:  "data/canonical/warranties.yaml",
	}
}

func minimalAtom() AtomInput {
	return AtomInput{
		AtomID:  "ATOM-MINIMAL",
		Kind:    "rule",
		Title:   "Minimal",
		Content: "Short.",
	}
}

func TestScoreAtomPerfect(t *testing.T) {
	m := ScoreAtom(perfectAtom(), DefaultMetricsConfig())

	if m.OverallScore < 0.9 {
		t.Fatalf("expected score >= 0.9 for perfect atom, got %f", m.OverallScore)
	}
	if m.Grade != "A" {
		t.Fatalf("expected grade A, got %s", m.Grade)
	}
	if m.Completeness != 1.0 {
		t.Fatalf("expected completeness 1.0, got %f", m.Completeness)
	}
}

func TestScoreAtomMinimal(t *testing.T) {
	m := ScoreAtom(minimalAtom(), DefaultMetricsConfig())

	if m.OverallScore > 0.5 {
		t.Fatalf("expected score < 0.5 for minimal atom, got %f", m.OverallScore)
	}
	if m.Grade == "A" || m.Grade == "B" {
		t.Fatalf("expected poor grade, got %s", m.Grade)
	}
	if len(m.Issues) == 0 {
		t.Fatal("expected issues for minimal atom")
	}
}

func TestScoreAtomEmpty(t *testing.T) {
	m := ScoreAtom(AtomInput{}, DefaultMetricsConfig())

	if m.OverallScore > 0.1 {
		t.Fatalf("expected very low score for empty atom, got %f", m.OverallScore)
	}
	if m.Grade != "F" {
		t.Fatalf("expected grade F, got %s", m.Grade)
	}
}

func TestScoreAtomCompleteness(t *testing.T) {
	atom := perfectAtom()
	m := ScoreAtom(atom, DefaultMetricsConfig())
	if m.Completeness != 1.0 {
		t.Fatalf("expected 1.0 completeness, got %f", m.Completeness)
	}

	// Remove one field.
	atom.SourceHash = ""
	m = ScoreAtom(atom, DefaultMetricsConfig())
	if m.Completeness >= 1.0 {
		t.Fatalf("expected < 1.0 completeness with missing hash, got %f", m.Completeness)
	}
}

func TestScoreAtomCoherence(t *testing.T) {
	atom := perfectAtom()
	m := ScoreAtom(atom, DefaultMetricsConfig())
	if m.Coherence < 0.9 {
		t.Fatalf("expected high coherence, got %f", m.Coherence)
	}

	// Short content.
	atom.Content = "x"
	m = ScoreAtom(atom, DefaultMetricsConfig())
	if m.Coherence > 0.7 {
		t.Fatalf("expected lower coherence for short content, got %f", m.Coherence)
	}
}

func TestScoreAtomSourceCoverage(t *testing.T) {
	atom := perfectAtom()
	m := ScoreAtom(atom, DefaultMetricsConfig())
	if m.SourceCoverage < 0.9 {
		t.Fatalf("expected high source coverage, got %f", m.SourceCoverage)
	}

	// No source node.
	atom.SourceNodeID = ""
	m = ScoreAtom(atom, DefaultMetricsConfig())
	if m.SourceCoverage >= 1.0 {
		t.Fatalf("expected < 1.0 source coverage without node, got %f", m.SourceCoverage)
	}
}

func TestScoreAtomSemanticRole(t *testing.T) {
	atom := perfectAtom()
	m := ScoreAtom(atom, DefaultMetricsConfig())
	if m.SemanticRoleCoverage < 0.9 {
		t.Fatalf("expected high semantic role coverage, got %f", m.SemanticRoleCoverage)
	}

	// No test refs.
	atom.TestRefs = nil
	m = ScoreAtom(atom, DefaultMetricsConfig())
	if m.SemanticRoleCoverage >= 1.0 {
		t.Fatalf("expected < 1.0 without test refs, got %f", m.SemanticRoleCoverage)
	}
}

func TestScoreAtomReviewStateEffect(t *testing.T) {
	atom := perfectAtom()
	atom.ReviewState = "draft"
	m := ScoreAtom(atom, DefaultMetricsConfig())

	atomApproved := perfectAtom()
	mApproved := ScoreAtom(atomApproved, DefaultMetricsConfig())

	if m.SourceCoverage >= mApproved.SourceCoverage {
		t.Fatalf("draft should score lower than approved: %f vs %f", m.SourceCoverage, mApproved.SourceCoverage)
	}
}

func TestScoreAtoms(t *testing.T) {
	atoms := []AtomInput{perfectAtom(), minimalAtom()}
	report := ScoreAtoms(atoms, DefaultMetricsConfig())

	if report.TotalAtoms != 2 {
		t.Fatalf("expected 2, got %d", report.TotalAtoms)
	}
	if report.AverageScore <= 0 || report.AverageScore >= 1 {
		t.Fatalf("expected average between 0 and 1, got %f", report.AverageScore)
	}
	if len(report.GradeDistribution) < 2 {
		t.Fatalf("expected at least 2 grades, got %v", report.GradeDistribution)
	}
	if report.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", report.SchemaVersion)
	}
}

func TestScoreAtomsEmpty(t *testing.T) {
	report := ScoreAtoms(nil, DefaultMetricsConfig())
	if report.TotalAtoms != 0 {
		t.Fatalf("expected 0, got %d", report.TotalAtoms)
	}
	if report.AverageScore != 0 {
		t.Fatalf("expected 0 avg, got %f", report.AverageScore)
	}
}

func TestGradeFromScore(t *testing.T) {
	cases := []struct {
		score float64
		grade string
	}{
		{0.95, "A"}, {0.90, "A"}, {0.80, "B"}, {0.75, "B"},
		{0.65, "C"}, {0.60, "C"}, {0.45, "D"}, {0.40, "D"},
		{0.30, "F"}, {0.0, "F"},
	}
	for _, tc := range cases {
		got := gradeFromScore(tc.score)
		if got != tc.grade {
			t.Errorf("gradeFromScore(%f) = %s, want %s", tc.score, got, tc.grade)
		}
	}
}

func TestDefaultMetricsConfigWeightsSum(t *testing.T) {
	c := DefaultMetricsConfig()
	sum := c.WeightCompleteness + c.WeightCoherence + c.WeightSourceCoverage + c.WeightSemanticRole
	if sum < 0.99 || sum > 1.01 {
		t.Fatalf("weights should sum to 1.0, got %f", sum)
	}
}

func TestTitleAligns(t *testing.T) {
	if !titleAligns("Water damage coverage", "The water damage coverage applies to residential properties") {
		t.Fatal("expected alignment")
	}
	if titleAligns("Completely unrelated topic", "Nothing matches here at all in this text") {
		t.Fatal("expected no alignment")
	}
}

package githubworkflow

import (
	"reflect"
	"testing"
)

func mkWorkflow(id string, sourcePaths []string, outputPath string) Workflow {
	return Workflow{
		ID:     id,
		Source: SourceSpec{Paths: sourcePaths},
		Output: OutputSpec{Path: outputPath},
	}
}

func TestPlanScopedDiff_SinglePathMatch(t *testing.T) {
	t.Parallel()
	cfg := WorkflowConfig{
		Workflows: []Workflow{
			mkWorkflow("w1", []string{"docs/**"}, "out/"),
		},
	}
	plan := PlanScopedDiff(cfg, []string{"docs/intro.md"})

	if len(plan.Impacted) != 1 {
		t.Fatalf("expected 1 impacted, got %d (%+v)", len(plan.Impacted), plan)
	}
	if plan.Impacted[0].WorkflowID != "w1" {
		t.Fatalf("expected w1, got %q", plan.Impacted[0].WorkflowID)
	}
	if !reflect.DeepEqual(plan.Impacted[0].MatchedPaths, []string{"docs/intro.md"}) {
		t.Fatalf("expected matched=[docs/intro.md], got %v", plan.Impacted[0].MatchedPaths)
	}
	if len(plan.Skipped) != 0 {
		t.Fatalf("expected zero skipped, got %v", plan.Skipped)
	}
}

func TestPlanScopedDiff_NoMatch(t *testing.T) {
	t.Parallel()
	cfg := WorkflowConfig{
		Workflows: []Workflow{
			mkWorkflow("w1", []string{"docs/**"}, "out/"),
		},
	}
	plan := PlanScopedDiff(cfg, []string{"unrelated/file.md"})

	if len(plan.Impacted) != 0 {
		t.Fatalf("expected zero impacted, got %v", plan.Impacted)
	}
	if len(plan.Skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %v", plan.Skipped)
	}
	if plan.Skipped[0].Reason != ReasonNoPathsMatch {
		t.Fatalf("expected reason=%s, got %q", ReasonNoPathsMatch, plan.Skipped[0].Reason)
	}
}

func TestPlanScopedDiff_GeneratedPathIgnored(t *testing.T) {
	t.Parallel()
	cfg := WorkflowConfig{
		Workflows: []Workflow{
			mkWorkflow("publisher", []string{"docs/**"}, "rbok-lawbook/"),
			mkWorkflow("downstream", []string{"rbok-lawbook/**"}, "downstream-out/"),
		},
	}
	// A path that lives under publisher's output should be ignored entirely
	// (loop guard) — downstream must NOT light up just because publisher
	// regenerated its own output.
	plan := PlanScopedDiff(cfg, []string{"rbok-lawbook/feed.json"})

	if len(plan.Impacted) != 0 {
		t.Fatalf("expected zero impacted (loop guard), got %v", plan.Impacted)
	}
	if !reflect.DeepEqual(plan.IgnoredGeneratedPaths, []string{"rbok-lawbook/feed.json"}) {
		t.Fatalf("expected ignored=[rbok-lawbook/feed.json], got %v", plan.IgnoredGeneratedPaths)
	}
	// Both workflows must be skipped with NGW_DIFF_NO_PATHS_MATCH.
	if len(plan.Skipped) != 2 {
		t.Fatalf("expected 2 skipped, got %v", plan.Skipped)
	}
	for _, s := range plan.Skipped {
		if s.Reason != ReasonNoPathsMatch {
			t.Fatalf("expected %s for %q, got %q", ReasonNoPathsMatch, s.WorkflowID, s.Reason)
		}
	}
}

func TestPlanScopedDiff_TwoWorkflowsOneImpacted(t *testing.T) {
	t.Parallel()
	cfg := WorkflowConfig{
		Workflows: []Workflow{
			mkWorkflow("docs-pipe", []string{"docs/**"}, "docs-out/"),
			mkWorkflow("schemas-pipe", []string{"schemas/**"}, "schemas-out/"),
		},
	}
	plan := PlanScopedDiff(cfg, []string{
		"docs/section/intro.md",
		"unrelated/notes.txt",
	})

	if len(plan.Impacted) != 1 || plan.Impacted[0].WorkflowID != "docs-pipe" {
		t.Fatalf("expected only docs-pipe impacted, got %+v", plan.Impacted)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].WorkflowID != "schemas-pipe" {
		t.Fatalf("expected only schemas-pipe skipped, got %+v", plan.Skipped)
	}
	if plan.Skipped[0].Reason != ReasonNoPathsMatch {
		t.Fatalf("expected schemas-pipe skip reason %s, got %q", ReasonNoPathsMatch, plan.Skipped[0].Reason)
	}
}

func TestPlanScopedDiff_GlobStarStarMatch(t *testing.T) {
	t.Parallel()
	cfg := WorkflowConfig{
		Workflows: []Workflow{
			mkWorkflow("rbok", []string{"01_rbok/**"}, "rbok-out/"),
		},
	}
	plan := PlanScopedDiff(cfg, []string{
		"01_rbok/sub/file.md",
		"01_rbok/top.md",
		"02_other/file.md",
	})

	if len(plan.Impacted) != 1 {
		t.Fatalf("expected 1 impacted, got %d (%+v)", len(plan.Impacted), plan)
	}
	want := []string{"01_rbok/sub/file.md", "01_rbok/top.md"}
	if !reflect.DeepEqual(plan.Impacted[0].MatchedPaths, want) {
		t.Fatalf("expected matched=%v, got %v", want, plan.Impacted[0].MatchedPaths)
	}
}

// matchGlob is exercised directly to lock down the glob semantics the
// dispatch enumerates ("`*`, `**`, `?`").
func TestMatchGlob_Cases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"01_rbok/**", "01_rbok/sub/file.md", true},
		{"01_rbok/**", "01_rbok", true},
		{"01_rbok/**", "01_rbok_other/file.md", false},
		{"docs/*.md", "docs/intro.md", true},
		{"docs/*.md", "docs/sub/intro.md", false},
		{"docs/**/*.md", "docs/sub/intro.md", true},
		{"docs/**/*.md", "docs/intro.md", true},
		{"**", "anything/here.txt", true},
		{"file?.md", "file1.md", true},
		{"file?.md", "file12.md", false},
		{"prefix/sub/", "prefix/sub/foo.md", false},
	}
	for _, c := range cases {
		got := matchGlob(c.pattern, c.path)
		if got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

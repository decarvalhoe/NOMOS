package dashboard

import (
	"path/filepath"
	"testing"
	"time"
)

func td(name string) string {
	return filepath.Join("testdata", name)
}

var now = time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

func allInputs() []ProjectInput {
	return []ProjectInput{
		{ProjectPath: td("project-alpha.yaml"), ExceptionsPath: td("exceptions-alpha.yaml")},
		{ProjectPath: td("project-beta.yaml")},
		{ProjectPath: td("project-gamma.yaml")},
		{ProjectPath: td("project-no-verdict.yaml")},
	}
}

func TestBuildPortfolioSummary(t *testing.T) {
	view, err := BuildPortfolio(allInputs(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if view.Summary.Total != 4 {
		t.Fatalf("expected 4 projects, got %d", view.Summary.Total)
	}
	if view.Summary.InScope != 1 {
		t.Fatalf("expected 1 in_scope, got %d", view.Summary.InScope)
	}
	if view.Summary.Partial != 1 {
		t.Fatalf("expected 1 partial, got %d", view.Summary.Partial)
	}
	if view.Summary.Blocked != 1 {
		t.Fatalf("expected 1 blocked, got %d", view.Summary.Blocked)
	}
	if view.Summary.Unknown != 1 {
		t.Fatalf("expected 1 unknown, got %d", view.Summary.Unknown)
	}
}

func TestProjectEntryFields(t *testing.T) {
	view, err := BuildPortfolio(allInputs(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	alpha := findProject(t, view, "alpha")
	if alpha.Name != "Alpha Service" {
		t.Fatalf("expected Alpha Service, got %q", alpha.Name)
	}
	if alpha.Domain != "insurance" {
		t.Fatalf("expected insurance, got %q", alpha.Domain)
	}
	if alpha.Verdict != "in_scope" {
		t.Fatalf("expected in_scope, got %q", alpha.Verdict)
	}
	if alpha.RiskLevel != "high" {
		t.Fatalf("expected high, got %q", alpha.RiskLevel)
	}
	if alpha.CriticalSurfaces != 1 {
		t.Fatalf("expected 1 critical surface, got %d", alpha.CriticalSurfaces)
	}
	if len(alpha.Stacks) != 2 {
		t.Fatalf("expected 2 stacks, got %d: %v", len(alpha.Stacks), alpha.Stacks)
	}
	if len(alpha.Owners) != 1 || alpha.Owners[0] != "Alice" {
		t.Fatalf("expected [Alice], got %v", alpha.Owners)
	}
}

func TestExceptionsVisible(t *testing.T) {
	view, err := BuildPortfolio(allInputs(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	alpha := findProject(t, view, "alpha")
	if len(alpha.Exceptions) != 2 {
		t.Fatalf("expected 2 exceptions, got %d", len(alpha.Exceptions))
	}

	// First exception is still valid
	if alpha.Exceptions[0].Expired {
		t.Fatal("expected EXC-ALPHA-001 to not be expired")
	}
	// Second exception is expired
	if !alpha.Exceptions[1].Expired {
		t.Fatal("expected EXC-ALPHA-002 to be expired")
	}
}

func TestNoVerdictDefaultsToUnknown(t *testing.T) {
	view, err := BuildPortfolio(allInputs(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	delta := findProject(t, view, "delta")
	if delta.Verdict != "unknown" {
		t.Fatalf("expected unknown verdict, got %q", delta.Verdict)
	}
}

func TestFilterByVerdict(t *testing.T) {
	view, err := BuildPortfolio(allInputs(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filtered := ApplyFilter(view, Filter{Verdicts: []string{"in_scope"}})
	if filtered.Summary.Total != 1 {
		t.Fatalf("expected 1 project, got %d", filtered.Summary.Total)
	}
	if filtered.Projects[0].ID != "alpha" {
		t.Fatalf("expected alpha, got %q", filtered.Projects[0].ID)
	}
}

func TestFilterByStack(t *testing.T) {
	view, err := BuildPortfolio(allInputs(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filtered := ApplyFilter(view, Filter{Stacks: []string{"java"}})
	if filtered.Summary.Total != 1 {
		t.Fatalf("expected 1 project, got %d", filtered.Summary.Total)
	}
	if filtered.Projects[0].ID != "beta" {
		t.Fatalf("expected beta, got %q", filtered.Projects[0].ID)
	}
}

func TestFilterByRiskLevel(t *testing.T) {
	view, err := BuildPortfolio(allInputs(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filtered := ApplyFilter(view, Filter{RiskLevels: []string{"critical"}})
	if filtered.Summary.Total != 1 {
		t.Fatalf("expected 1 project, got %d", filtered.Summary.Total)
	}
	if filtered.Projects[0].ID != "beta" {
		t.Fatalf("expected beta, got %q", filtered.Projects[0].ID)
	}
}

func TestFilterByOwner(t *testing.T) {
	view, err := BuildPortfolio(allInputs(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filtered := ApplyFilter(view, Filter{Owners: []string{"Bob"}})
	if filtered.Summary.Total != 1 {
		t.Fatalf("expected 1 project, got %d", filtered.Summary.Total)
	}
	if filtered.Projects[0].ID != "beta" {
		t.Fatalf("expected beta, got %q", filtered.Projects[0].ID)
	}
}

func TestFilterByOwnerCaseInsensitive(t *testing.T) {
	view, err := BuildPortfolio(allInputs(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filtered := ApplyFilter(view, Filter{Owners: []string{"bob"}})
	if filtered.Summary.Total != 1 {
		t.Fatalf("expected 1 project, got %d", filtered.Summary.Total)
	}
}

func TestFilterCombined(t *testing.T) {
	view, err := BuildPortfolio(allInputs(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Filter by stack=python AND verdict=in_scope → alpha
	filtered := ApplyFilter(view, Filter{Stacks: []string{"python"}, Verdicts: []string{"in_scope"}})
	if filtered.Summary.Total != 1 {
		t.Fatalf("expected 1, got %d", filtered.Summary.Total)
	}
	if filtered.Projects[0].ID != "alpha" {
		t.Fatalf("expected alpha, got %q", filtered.Projects[0].ID)
	}
}

func TestFilterNoMatch(t *testing.T) {
	view, err := BuildPortfolio(allInputs(), now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filtered := ApplyFilter(view, Filter{Stacks: []string{"rust"}})
	if filtered.Summary.Total != 0 {
		t.Fatalf("expected 0, got %d", filtered.Summary.Total)
	}
}

func TestEmptyPortfolio(t *testing.T) {
	view, err := BuildPortfolio(nil, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.Summary.Total != 0 {
		t.Fatalf("expected 0, got %d", view.Summary.Total)
	}
}

func TestMissingProjectFile(t *testing.T) {
	_, err := BuildPortfolio([]ProjectInput{{ProjectPath: "/nonexistent.yaml"}}, now)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func findProject(t *testing.T, view PortfolioView, id string) ProjectEntry {
	t.Helper()
	for _, p := range view.Projects {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("project %q not found", id)
	return ProjectEntry{}
}

package portfolio

// NRT-022 (#670) — the multi-project view ported from the archived dashboard,
// with the rules that make it honest: an expired exception is visible and
// counted, an undated one is flagged, an unparsable date or a nameless
// exception is an error, a missing verdict is counted unknown, a missing or
// nameless manifest is an error, and filters are exact.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var viewNow = time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)

func td(name string) string { return filepath.Join("testdata", "projects", name) }

func allInputs() []ProjectInput {
	return []ProjectInput{
		{ProjectPath: td("project-alpha.yaml"), ExceptionsPath: td("exceptions-alpha.yaml")},
		{ProjectPath: td("project-beta.yaml")},
		{ProjectPath: td("project-gamma.yaml")},
		{ProjectPath: td("project-no-verdict.yaml")},
	}
}

func find(t *testing.T, v ProjectsView, id string) ProjectEntry {
	t.Helper()
	for _, p := range v.Projects {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("project %s not in view", id)
	return ProjectEntry{}
}

func TestProjectsSummaryAndFields(t *testing.T) {
	v, err := BuildProjects(allInputs(), viewNow)
	if err != nil {
		t.Fatal(err)
	}
	s := v.Summary
	if s.Total != 4 || s.InScope != 1 || s.Partial != 1 || s.Blocked != 1 || s.Unknown != 1 || s.Exceptions != 2 || s.ExpiredExceptions != 1 {
		t.Fatalf("summary: %+v", s)
	}
	alpha := find(t, v, "alpha")
	if alpha.Name != "Alpha Service" || alpha.Domain != "insurance" || alpha.RiskLevel != "high" || alpha.CriticalSurfaces != 1 || strings.Join(alpha.Stacks, ",") != "python,typescript" || alpha.Owners[0] != "Alice" {
		t.Fatalf("alpha: %+v", alpha)
	}
	if !strings.HasPrefix(alpha.Source.Sha256, "sha256:") {
		t.Fatal("each project row names its manifest hash")
	}
	if find(t, v, "delta").Verdict != "unknown" {
		t.Fatal("a manifest without scope.verdict is counted unknown, not dropped")
	}
	if v.Projects[0].ID != "alpha" || v.Projects[3].ID != "gamma" {
		t.Fatal("projects are sorted by id for a deterministic view")
	}
}

func TestExceptionsAreVisibleWithExpiryComputedAtViewTime(t *testing.T) {
	v, _ := BuildProjects(allInputs(), viewNow)
	alpha := find(t, v, "alpha")
	if len(alpha.Exceptions) != 2 {
		t.Fatalf("exceptions: %+v", alpha.Exceptions)
	}
	byID := map[string]ExceptionEntry{}
	for _, x := range alpha.Exceptions {
		byID[x.ID] = x
	}
	if byID["EXC-ALPHA-001"].Expired || !byID["EXC-ALPHA-002"].Expired || byID["EXC-ALPHA-002"].Approver != "quality@example.com" || byID["EXC-ALPHA-001"].DecisionRef != "DEC-2026-001" {
		t.Fatalf("expiry/fields: %+v", byID)
	}
	// Move the clock past the first expiry: both expired now.
	later, _ := BuildProjects(allInputs(), time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	if later.Summary.ExpiredExceptions != 2 {
		t.Fatalf("expiry follows the view clock, got %d", later.Summary.ExpiredExceptions)
	}
}

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	os.WriteFile(p, []byte(body), 0o644)
	return p
}

func TestExceptionRulesFailClosed(t *testing.T) {
	proj := td("project-alpha.yaml")
	undated := writeTemp(t, "e.yaml", "exceptions:\n  - id: X-1\n    summary: s\n    status: active\n")
	v, err := BuildProjects([]ProjectInput{{ProjectPath: proj, ExceptionsPath: undated}}, viewNow)
	if err != nil || !v.Projects[0].Exceptions[0].Undated || v.Projects[0].Exceptions[0].Expired {
		t.Fatalf("an exception without expires_at is flagged undated, never treated as valid forever: %v %+v", err, v.Projects)
	}
	bad := writeTemp(t, "e2.yaml", "exceptions:\n  - id: X-1\n    expires_at: next-year\n")
	if _, err := BuildProjects([]ProjectInput{{ProjectPath: proj, ExceptionsPath: bad}}, viewNow); err == nil || !strings.Contains(err.Error(), "unparsable expires_at") {
		t.Fatalf("unparsable expiry must be an error, got %v", err)
	}
	nameless := writeTemp(t, "e3.yaml", "exceptions:\n  - summary: no id\n    expires_at: \"2026-12-31\"\n")
	if _, err := BuildProjects([]ProjectInput{{ProjectPath: proj, ExceptionsPath: nameless}}, viewNow); err == nil || !strings.Contains(err.Error(), "has no id") {
		t.Fatalf("an exception without id must be an error, got %v", err)
	}
}

func TestManifestRulesFailClosed(t *testing.T) {
	if _, err := BuildProjects([]ProjectInput{{ProjectPath: td("missing.yaml")}}, viewNow); err == nil || !strings.Contains(err.Error(), "read manifest") {
		t.Fatalf("missing manifest: %v", err)
	}
	noID := writeTemp(t, "p.yaml", "project:\n  name: Nameless\nscope:\n  verdict: in_scope\n")
	if _, err := BuildProjects([]ProjectInput{{ProjectPath: noID}}, viewNow); err == nil || !strings.Contains(err.Error(), "no project.id") {
		t.Fatalf("manifest without id: %v", err)
	}
	broken := writeTemp(t, "b.yaml", "project: [\n")
	if _, err := BuildProjects([]ProjectInput{{ProjectPath: broken}}, viewNow); err == nil || !strings.Contains(err.Error(), "parse manifest") {
		t.Fatalf("broken manifest: %v", err)
	}
}

func TestFiltersAreExact(t *testing.T) {
	v, _ := BuildProjects(allInputs(), viewNow)
	cases := []struct {
		name string
		f    ProjectFilter
		want []string
	}{
		{"verdict", ProjectFilter{Verdicts: []string{"blocked"}}, []string{"gamma"}},
		{"stack", ProjectFilter{Stacks: []string{"python"}}, []string{"alpha"}},
		{"risk", ProjectFilter{RiskLevels: []string{"critical"}}, []string{"beta"}},
		{"owner exact", ProjectFilter{Owners: []string{"Charlie"}}, []string{"gamma"}},
		{"owner case-insensitive", ProjectFilter{Owners: []string{"alice"}}, []string{"alpha"}},
		{"combined", ProjectFilter{Verdicts: []string{"in_scope", "partial"}, Stacks: []string{"java"}}, []string{"beta"}},
		{"no match", ProjectFilter{Verdicts: []string{"out_of_scope"}}, []string{}},
		{"verdict typo matches nothing", ProjectFilter{Verdicts: []string{"in-scope"}}, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := FilterProjects(v, tc.f)
			var ids []string
			for _, p := range out.Projects {
				ids = append(ids, p.ID)
			}
			if strings.Join(ids, ",") != strings.Join(tc.want, ",") || out.Summary.Total != len(tc.want) {
				t.Fatalf("filter %s → %v (summary %+v), want %v", tc.name, ids, out.Summary, tc.want)
			}
		})
	}
}

func TestEmptyViewAndMarkdown(t *testing.T) {
	v, err := BuildProjects(nil, viewNow)
	if err != nil || v.Summary.Total != 0 || v.Projects == nil {
		t.Fatalf("empty view: %v %+v", err, v)
	}
	full, _ := BuildProjects(allInputs(), viewNow)
	var sb strings.Builder
	WriteProjectsMarkdown(&sb, full)
	for _, must := range []string{"4 project(s)", "2 exception(s), 1 expired", "| alpha (Alpha Service) | in_scope |", "| delta (Delta New) | unknown |", "neither validates a project nor grants an exception"} {
		if !strings.Contains(sb.String(), must) {
			t.Fatalf("markdown must contain %q:\n%s", must, sb.String())
		}
	}
}

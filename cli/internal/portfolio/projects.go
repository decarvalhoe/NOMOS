package portfolio

// NRT-022 (#670) — the multi-project view, the one purpose the archived
// control-plane dashboard actually served, now behind a real caller
// (`nomos portfolio projects`). It reads REAL NOMOS artifacts: nomos.project.yaml
// manifests (identity, scope verdict, surfaces) and the strict gate's
// exceptions manifest (docs/… exceptions.yaml, the same schema
// cli/internal/exceptions reads). It computes, it never decides: an expired
// exception is shown expired, a missing verdict is counted unknown, and a
// manifest that cannot be read is an error, not an empty row.

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ProjectInput pairs a project manifest with its optional exceptions manifest.
type ProjectInput struct {
	ProjectPath    string
	ExceptionsPath string
}

// ProjectsView is the multi-project output.
type ProjectsView struct {
	SchemaVersion string         `json:"schema_version"`
	GeneratedAt   string         `json:"generated_at"`
	Summary       ProjectSummary `json:"summary"`
	Projects      []ProjectEntry `json:"projects"`
	ClaimBoundary string         `json:"claim_boundary"`
}

// ProjectSummary counts projects by scope verdict and exceptions by state.
type ProjectSummary struct {
	Total             int `json:"total"`
	InScope           int `json:"in_scope"`
	Partial           int `json:"partial"`
	Blocked           int `json:"blocked"`
	OutOfScope        int `json:"out_of_scope"`
	Unknown           int `json:"unknown"`
	Exceptions        int `json:"exceptions"`
	ExpiredExceptions int `json:"expired_exceptions"`
}

// ProjectEntry is one project row.
type ProjectEntry struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Domain           string           `json:"domain"`
	Verdict          string           `json:"verdict"`
	RiskLevel        string           `json:"risk_level"`
	Lifecycle        string           `json:"lifecycle"`
	Owners           []string         `json:"owners"`
	Stacks           []string         `json:"stacks"`
	CriticalSurfaces int              `json:"critical_surfaces"`
	Source           Source           `json:"source"`
	Exceptions       []ExceptionEntry `json:"exceptions"`
}

// ExceptionEntry is a visible exception with its expiry computed at view time.
type ExceptionEntry struct {
	ID          string `json:"id"`
	Summary     string `json:"summary"`
	Owner       string `json:"owner"`
	Approver    string `json:"approver"`
	Severity    string `json:"severity"`
	Status      string `json:"status"`
	ExpiresAt   string `json:"expires_at"`
	Expired     bool   `json:"expired"`
	Undated     bool   `json:"undated"`
	DecisionRef string `json:"decision_ref"`
}

// ProjectFilter selects rows; every criterion is exact (owners case-insensitive).
type ProjectFilter struct {
	Verdicts   []string
	Stacks     []string
	RiskLevels []string
	Owners     []string
}

const ProjectsClaimBoundary = "A computed multi-project view of nomos.project.yaml manifests and exceptions manifests. " +
	"Verdicts, risk levels and exceptions are read as declared; the view neither validates a project nor grants an exception."

type projectManifest struct {
	Project struct {
		ID        string `yaml:"id"`
		Name      string `yaml:"name"`
		Domain    string `yaml:"domain"`
		Lifecycle string `yaml:"lifecycle"`
		RiskLevel string `yaml:"risk_level"`
		Owners    []struct {
			Name  string `yaml:"name"`
			Role  string `yaml:"role"`
			Email string `yaml:"email"`
		} `yaml:"owners"`
	} `yaml:"project"`
	Scope struct {
		Verdict string `yaml:"verdict"`
	} `yaml:"scope"`
	Surfaces []struct {
		Name     string `yaml:"name"`
		Stack    string `yaml:"stack"`
		Critical bool   `yaml:"critical"`
	} `yaml:"surfaces"`
	CorpusSurfaces []struct {
		Name string `yaml:"name"`
	} `yaml:"corpus_surfaces"`
}

type exceptionsManifest struct {
	Exceptions []struct {
		ID          string `yaml:"id"`
		Summary     string `yaml:"summary"`
		Owner       string `yaml:"owner"`
		Approver    string `yaml:"approver"`
		Severity    string `yaml:"severity"`
		Status      string `yaml:"status"`
		ExpiresAt   string `yaml:"expires_at"`
		DecisionRef string `yaml:"decision_ref"`
	} `yaml:"exceptions"`
}

// BuildProjects computes the view. Any unreadable manifest is an error.
func BuildProjects(inputs []ProjectInput, now time.Time) (ProjectsView, error) {
	view := ProjectsView{SchemaVersion: "nomos-portfolio-projects-v1", GeneratedAt: now.UTC().Format("2006-01-02T15:04:05Z"), Projects: []ProjectEntry{}, ClaimBoundary: ProjectsClaimBoundary}
	for _, in := range inputs {
		entry, err := buildProject(in, now)
		if err != nil {
			return ProjectsView{}, fmt.Errorf("project %s: %w", in.ProjectPath, err)
		}
		view.Projects = append(view.Projects, entry)
	}
	sort.Slice(view.Projects, func(i, j int) bool { return view.Projects[i].ID < view.Projects[j].ID })
	view.Summary = summarise(view.Projects)
	return view, nil
}

// FilterProjects returns the rows matching every criterion; summary recomputed.
func FilterProjects(view ProjectsView, f ProjectFilter) ProjectsView {
	out := ProjectsView{SchemaVersion: view.SchemaVersion, GeneratedAt: view.GeneratedAt, Projects: []ProjectEntry{}, ClaimBoundary: view.ClaimBoundary}
	for _, p := range view.Projects {
		if matches(p, f) {
			out.Projects = append(out.Projects, p)
		}
	}
	out.Summary = summarise(out.Projects)
	return out
}

func buildProject(in ProjectInput, now time.Time) (ProjectEntry, error) {
	raw, err := os.ReadFile(in.ProjectPath)
	if err != nil {
		return ProjectEntry{}, fmt.Errorf("read manifest: %w", err)
	}
	var m projectManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return ProjectEntry{}, fmt.Errorf("parse manifest: %w", err)
	}
	if strings.TrimSpace(m.Project.ID) == "" {
		return ProjectEntry{}, fmt.Errorf("manifest has no project.id")
	}
	e := ProjectEntry{ID: m.Project.ID, Name: m.Project.Name, Domain: m.Project.Domain, Verdict: m.Scope.Verdict, RiskLevel: m.Project.RiskLevel, Lifecycle: m.Project.Lifecycle,
		Owners: []string{}, Stacks: []string{}, Exceptions: []ExceptionEntry{}}
	if e.Verdict == "" {
		e.Verdict = "unknown"
	}
	for _, o := range m.Project.Owners {
		if o.Name != "" {
			e.Owners = append(e.Owners, o.Name)
		}
	}
	stacks := map[string]bool{}
	for _, s := range m.Surfaces {
		if s.Stack != "" {
			stacks[s.Stack] = true
		}
		if s.Critical {
			e.CriticalSurfaces++
		}
	}
	for s := range stacks {
		e.Stacks = append(e.Stacks, s)
	}
	sort.Strings(e.Stacks)
	e.Source = sourceOf(in.ProjectPath, raw)
	if in.ExceptionsPath != "" {
		excs, err := loadExceptions(in.ExceptionsPath, now)
		if err != nil {
			return ProjectEntry{}, err
		}
		e.Exceptions = excs
	}
	return e, nil
}

func loadExceptions(path string, now time.Time) ([]ExceptionEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read exceptions: %w", err)
	}
	var m exceptionsManifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse exceptions: %w", err)
	}
	out := []ExceptionEntry{}
	for i, x := range m.Exceptions {
		if strings.TrimSpace(x.ID) == "" {
			return nil, fmt.Errorf("exceptions: entry %d has no id", i+1)
		}
		ee := ExceptionEntry{ID: x.ID, Summary: x.Summary, Owner: x.Owner, Approver: x.Approver, Severity: x.Severity, Status: x.Status, ExpiresAt: x.ExpiresAt, DecisionRef: x.DecisionRef}
		if strings.TrimSpace(x.ExpiresAt) == "" {
			ee.Undated = true // an exception without expiry is shown as such, never as valid forever
		} else {
			t, err := time.Parse("2006-01-02", strings.TrimSpace(x.ExpiresAt))
			if err != nil {
				return nil, fmt.Errorf("exceptions: %s has an unparsable expires_at %q", x.ID, x.ExpiresAt)
			}
			ee.Expired = now.After(t)
		}
		out = append(out, ee)
	}
	return out, nil
}

func summarise(projects []ProjectEntry) ProjectSummary {
	s := ProjectSummary{Total: len(projects)}
	for _, p := range projects {
		switch p.Verdict {
		case "in_scope":
			s.InScope++
		case "partial":
			s.Partial++
		case "blocked":
			s.Blocked++
		case "out_of_scope":
			s.OutOfScope++
		default:
			s.Unknown++
		}
		for _, x := range p.Exceptions {
			s.Exceptions++
			if x.Expired {
				s.ExpiredExceptions++
			}
		}
	}
	return s
}

func matches(p ProjectEntry, f ProjectFilter) bool {
	if len(f.Verdicts) > 0 && !slices.Contains(f.Verdicts, p.Verdict) {
		return false
	}
	if len(f.RiskLevels) > 0 && !slices.Contains(f.RiskLevels, p.RiskLevel) {
		return false
	}
	if len(f.Owners) > 0 {
		ok := false
		for _, want := range f.Owners {
			for _, have := range p.Owners {
				if strings.EqualFold(want, have) {
					ok = true
				}
			}
		}
		if !ok {
			return false
		}
	}
	if len(f.Stacks) > 0 {
		ok := false
		for _, want := range f.Stacks {
			if slices.Contains(p.Stacks, want) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func sourceOf(path string, raw []byte) Source {
	return Source{Path: path, Sha256: sha256Of(raw), Freshness: "undated"}
}

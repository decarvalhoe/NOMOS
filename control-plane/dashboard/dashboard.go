package dashboard

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type projectManifest struct {
	Project struct {
		ID        string  `yaml:"id"`
		Name      string  `yaml:"name"`
		Domain    string  `yaml:"domain"`
		Lifecycle string  `yaml:"lifecycle"`
		RiskLevel string  `yaml:"risk_level"`
		Owners    []owner `yaml:"owners"`
	} `yaml:"project"`
	Scope struct {
		Verdict string   `yaml:"verdict"`
		InScope []string `yaml:"in_scope"`
	} `yaml:"scope"`
	Surfaces []surface `yaml:"surfaces"`
}

type owner struct {
	Name  string `yaml:"name"`
	Role  string `yaml:"role"`
	Email string `yaml:"email"`
}

type surface struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Stack    string `yaml:"stack"`
	Critical bool   `yaml:"critical"`
}

type exceptionsManifest struct {
	Exceptions []exception `yaml:"exceptions"`
}

type exception struct {
	ID        string `yaml:"id"`
	Summary   string `yaml:"summary"`
	Owner     string `yaml:"owner"`
	Severity  string `yaml:"severity"`
	Status    string `yaml:"status"`
	ExpiresAt string `yaml:"expires_at"`
}

// ProjectInput holds paths to manifests for a single project.
type ProjectInput struct {
	ProjectPath    string
	ExceptionsPath string
}

// PortfolioView is the top-level dashboard output.
type PortfolioView struct {
	GeneratedAt string         `json:"generated_at"`
	Summary     Summary        `json:"summary"`
	Projects    []ProjectEntry `json:"projects"`
}

// Summary counts projects by scope verdict.
type Summary struct {
	Total      int `json:"total"`
	InScope    int `json:"in_scope"`
	Partial    int `json:"partial"`
	Blocked    int `json:"blocked"`
	OutOfScope int `json:"out_of_scope"`
	Unknown    int `json:"unknown"`
}

// ProjectEntry represents a single project in the dashboard.
type ProjectEntry struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Domain           string            `json:"domain"`
	Verdict          string            `json:"verdict"`
	RiskLevel        string            `json:"risk_level"`
	Lifecycle        string            `json:"lifecycle"`
	Owners           []string          `json:"owners"`
	Stacks           []string          `json:"stacks"`
	CriticalSurfaces int               `json:"critical_surfaces"`
	Exceptions       []ExceptionEntry  `json:"exceptions,omitempty"`
}

// ExceptionEntry is a visible exception in the dashboard.
type ExceptionEntry struct {
	ID        string `json:"id"`
	Summary   string `json:"summary"`
	Severity  string `json:"severity"`
	Status    string `json:"status"`
	ExpiresAt string `json:"expires_at"`
	Expired   bool   `json:"expired"`
}

// Filter controls which projects appear in the view.
type Filter struct {
	Verdicts    []string
	Stacks      []string
	RiskLevels  []string
	Owners      []string
}

// BuildPortfolio builds a portfolio view from multiple project inputs.
func BuildPortfolio(inputs []ProjectInput, now time.Time) (PortfolioView, error) {
	view := PortfolioView{
		GeneratedAt: now.UTC().Format(time.RFC3339),
	}

	for _, input := range inputs {
		entry, err := buildEntry(input, now)
		if err != nil {
			return PortfolioView{}, fmt.Errorf("project %s: %w", input.ProjectPath, err)
		}
		view.Projects = append(view.Projects, entry)
	}

	view.Summary = computeSummary(view.Projects)
	return view, nil
}

// ApplyFilter returns a new view with only matching projects.
func ApplyFilter(view PortfolioView, filter Filter) PortfolioView {
	filtered := PortfolioView{
		GeneratedAt: view.GeneratedAt,
	}

	for _, p := range view.Projects {
		if matchesFilter(p, filter) {
			filtered.Projects = append(filtered.Projects, p)
		}
	}

	filtered.Summary = computeSummary(filtered.Projects)
	return filtered
}

func buildEntry(input ProjectInput, now time.Time) (ProjectEntry, error) {
	data, err := os.ReadFile(input.ProjectPath)
	if err != nil {
		return ProjectEntry{}, fmt.Errorf("reading project: %w", err)
	}

	var proj projectManifest
	if err := yaml.Unmarshal(data, &proj); err != nil {
		return ProjectEntry{}, fmt.Errorf("parsing project: %w", err)
	}

	entry := ProjectEntry{
		ID:        proj.Project.ID,
		Name:      proj.Project.Name,
		Domain:    proj.Project.Domain,
		Verdict:   proj.Scope.Verdict,
		RiskLevel: proj.Project.RiskLevel,
		Lifecycle: proj.Project.Lifecycle,
	}

	if entry.Verdict == "" {
		entry.Verdict = "unknown"
	}

	for _, o := range proj.Project.Owners {
		if o.Name != "" {
			entry.Owners = append(entry.Owners, o.Name)
		}
	}

	stackSet := make(map[string]bool)
	for _, s := range proj.Surfaces {
		if s.Stack != "" {
			stackSet[s.Stack] = true
		}
		if s.Critical {
			entry.CriticalSurfaces++
		}
	}
	for stack := range stackSet {
		entry.Stacks = append(entry.Stacks, stack)
	}
	sort.Strings(entry.Stacks)

	if input.ExceptionsPath != "" {
		excs, err := loadExceptions(input.ExceptionsPath, now)
		if err != nil {
			return ProjectEntry{}, err
		}
		entry.Exceptions = excs
	}

	return entry, nil
}

func loadExceptions(path string, now time.Time) ([]ExceptionEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading exceptions: %w", err)
	}

	var manifest exceptionsManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parsing exceptions: %w", err)
	}

	var entries []ExceptionEntry
	for _, exc := range manifest.Exceptions {
		ee := ExceptionEntry{
			ID:        exc.ID,
			Summary:   exc.Summary,
			Severity:  exc.Severity,
			Status:    exc.Status,
			ExpiresAt: exc.ExpiresAt,
		}
		if exc.ExpiresAt != "" {
			if expiresAt, err := time.Parse("2006-01-02", exc.ExpiresAt); err == nil {
				ee.Expired = now.After(expiresAt)
			}
		}
		entries = append(entries, ee)
	}
	return entries, nil
}

func computeSummary(projects []ProjectEntry) Summary {
	s := Summary{Total: len(projects)}
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
	}
	return s
}

func matchesFilter(p ProjectEntry, f Filter) bool {
	if len(f.Verdicts) > 0 && !slices.Contains(f.Verdicts, p.Verdict) {
		return false
	}
	if len(f.RiskLevels) > 0 && !slices.Contains(f.RiskLevels, p.RiskLevel) {
		return false
	}
	if len(f.Owners) > 0 {
		found := false
		for _, fo := range f.Owners {
			for _, po := range p.Owners {
				if strings.EqualFold(fo, po) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	if len(f.Stacks) > 0 {
		found := false
		for _, fs := range f.Stacks {
			if slices.Contains(p.Stacks, fs) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

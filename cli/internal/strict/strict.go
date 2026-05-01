package strict

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type sourceManifest struct {
	Sources []source `yaml:"sources"`
}

type source struct {
	ID     string `yaml:"id"`
	Domain string `yaml:"domain"`
	Status string `yaml:"status"`
}

type canonicalMatrix struct {
	Units []unit `yaml:"units"`
}

type unit struct {
	UnitID     string      `yaml:"unit_id"`
	Domain     string      `yaml:"domain"`
	SourceRefs []sourceRef `yaml:"source_refs"`
	Status     string      `yaml:"status"`
}

type sourceRef struct {
	SourceID string `yaml:"source_id"`
}

type projectManifest struct {
	Project struct {
		Domain string `yaml:"domain"`
	} `yaml:"project"`
	Scope struct {
		InScope []string `yaml:"in_scope"`
	} `yaml:"scope"`
	Surfaces []surfaceDecl `yaml:"surfaces"`
}

type surfaceDecl struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
}

type StrictResult struct {
	Valid  bool         `json:"valid"`
	Errors []StrictError `json:"errors,omitempty"`
}

type StrictError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StrictInput struct {
	ProjectPath string
	SourcesPath string
	MatrixPath  string
}

func Check(input StrictInput) (StrictResult, error) {
	var proj *projectManifest
	var src *sourceManifest
	var mat *canonicalMatrix

	if input.ProjectPath != "" {
		p, err := loadYAML[projectManifest](input.ProjectPath)
		if err != nil {
			return StrictResult{}, fmt.Errorf("loading project: %w", err)
		}
		proj = p
	}

	if input.SourcesPath != "" {
		s, err := loadYAML[sourceManifest](input.SourcesPath)
		if err != nil {
			return StrictResult{}, fmt.Errorf("loading sources: %w", err)
		}
		src = s
	}

	if input.MatrixPath != "" {
		m, err := loadYAML[canonicalMatrix](input.MatrixPath)
		if err != nil {
			return StrictResult{}, fmt.Errorf("loading matrix: %w", err)
		}
		mat = m
	}

	return CheckFromManifests(proj, src, mat), nil
}

func CheckFromManifests(proj *projectManifest, src *sourceManifest, mat *canonicalMatrix) StrictResult {
	var errors []StrictError

	if src != nil {
		checkDuplicateSourceIDs(&errors, src)
	}
	if mat != nil {
		checkDuplicateUnitIDs(&errors, mat)
	}
	if src != nil && mat != nil {
		checkSourceRefIntegrity(&errors, src, mat)
		checkOrphanSources(&errors, src, mat)
	}
	if proj != nil && mat != nil {
		checkDomainCoverage(&errors, proj, mat)
	}

	return StrictResult{
		Valid:  len(errors) == 0,
		Errors: errors,
	}
}

func checkDuplicateSourceIDs(errors *[]StrictError, src *sourceManifest) {
	seen := make(map[string]int)
	for _, s := range src.Sources {
		seen[s.ID]++
	}
	for id, count := range seen {
		if count > 1 {
			addError(errors, "DUPLICATE_SOURCE_ID",
				fmt.Sprintf("source id %q appears %d times", id, count))
		}
	}
}

func checkDuplicateUnitIDs(errors *[]StrictError, mat *canonicalMatrix) {
	seen := make(map[string]int)
	for _, u := range mat.Units {
		seen[u.UnitID]++
	}
	for id, count := range seen {
		if count > 1 {
			addError(errors, "DUPLICATE_UNIT_ID",
				fmt.Sprintf("unit_id %q appears %d times", id, count))
		}
	}
}

func checkSourceRefIntegrity(errors *[]StrictError, src *sourceManifest, mat *canonicalMatrix) {
	sourceIDs := make(map[string]bool)
	for _, s := range src.Sources {
		sourceIDs[s.ID] = true
	}

	for _, u := range mat.Units {
		for _, ref := range u.SourceRefs {
			if !sourceIDs[ref.SourceID] {
				addError(errors, "DANGLING_SOURCE_REF",
					fmt.Sprintf("unit %q references source %q which does not exist in source manifest",
						u.UnitID, ref.SourceID))
			}
		}
	}
}

func checkOrphanSources(errors *[]StrictError, src *sourceManifest, mat *canonicalMatrix) {
	referenced := make(map[string]bool)
	for _, u := range mat.Units {
		for _, ref := range u.SourceRefs {
			referenced[ref.SourceID] = true
		}
	}

	for _, s := range src.Sources {
		if s.Status == "out_of_scope" || s.Status == "superseded" || s.Status == "duplicate" {
			continue
		}
		if !referenced[s.ID] {
			addError(errors, "ORPHAN_SOURCE",
				fmt.Sprintf("source %q is active but not referenced by any unit", s.ID))
		}
	}
}

func checkDomainCoverage(errors *[]StrictError, proj *projectManifest, mat *canonicalMatrix) {
	inScope := make(map[string]bool)
	for _, s := range proj.Scope.InScope {
		inScope[s] = true
	}

	for _, u := range mat.Units {
		if u.Domain == "" {
			continue
		}
		if !inScope[u.Domain] {
			addError(errors, "DOMAIN_NOT_IN_SCOPE",
				fmt.Sprintf("unit %q has domain %q which is not in project scope.in_scope",
					u.UnitID, u.Domain))
		}
	}
}

func addError(errors *[]StrictError, code string, message string) {
	*errors = append(*errors, StrictError{
		Code:    code,
		Message: message,
	})
}

func loadYAML[T any](path string) (*T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var v T
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

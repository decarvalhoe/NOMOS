package corpus

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var rbekRefPattern = regexp.MustCompile(`(?i)(?:rbok|RBOK)[_\-:]([A-Za-z0-9._\-/]+)`)

// Finding severity levels for corpus extraction.
const (
	SeverityWarning = "warning"
	SeverityError   = "error"
)

// ParcoursManifest is the top-level structure of an RBOK parcours YAML file.
type ParcoursManifest struct {
	Version  string            `yaml:"version" json:"version"`
	Owner    string            `yaml:"owner" json:"owner"`
	Status   string            `yaml:"status" json:"status"`
	Domain   string            `yaml:"domain" json:"domain"`
	Parcours []ParcoursEntry   `yaml:"parcours" json:"parcours"`
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// ParcoursEntry represents a single parcours (learning path).
type ParcoursEntry struct {
	ID               string        `yaml:"id" json:"id"`
	Name             string        `yaml:"name" json:"name"`
	SourceRBOK       string        `yaml:"source_rbok" json:"source_rbok"`
	ContenuReference string        `yaml:"contenu_reference" json:"contenu_reference"`
	Modules          []ModuleEntry `yaml:"modules" json:"modules"`
}

// ModuleEntry represents a module within a parcours.
type ModuleEntry struct {
	ID               string          `yaml:"id" json:"id"`
	Name             string          `yaml:"name" json:"name"`
	SourceRBOK       string          `yaml:"source_rbok" json:"source_rbok"`
	ContenuReference string          `yaml:"contenu_reference" json:"contenu_reference"`
	Questions        []QuestionEntry `yaml:"questions" json:"questions"`
}

// QuestionEntry represents a question within a module.
type QuestionEntry struct {
	ID               string `yaml:"id" json:"id"`
	Text             string `yaml:"text" json:"text"`
	SourceRBOK       string `yaml:"source_rbok" json:"source_rbok"`
	ContenuReference string `yaml:"contenu_reference" json:"contenu_reference"`
}

// ExtractionFinding reports a governance or reference issue.
type ExtractionFinding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Location string `json:"location,omitempty"`
	Message  string `json:"message"`
}

// ExtractionResult holds the output of a parcours extraction.
type ExtractionResult struct {
	Manifest   ParcoursManifest    `json:"manifest"`
	References []string            `json:"references"`
	Findings   []ExtractionFinding `json:"findings"`
}

// ExtractParcours parses an RBOK parcours YAML file and extracts governance
// metadata, normalized references, and findings for missing/unresolved data.
func ExtractParcours(path string) (ExtractionResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExtractionResult{}, err
	}
	return ExtractParcoursFromBytes(path, data)
}

// ExtractParcoursFromBytes parses RBOK parcours YAML content.
func ExtractParcoursFromBytes(path string, data []byte) (ExtractionResult, error) {
	var manifest ParcoursManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return ExtractionResult{}, fmt.Errorf("parse %s: %w", path, err)
	}

	result := ExtractionResult{Manifest: manifest}

	// Check governance metadata.
	result.Findings = append(result.Findings, checkGovernance(path, manifest)...)

	// Normalize and collect references.
	for i := range manifest.Parcours {
		p := &manifest.Parcours[i]
		p.SourceRBOK = normalizeRef(p.SourceRBOK)
		p.ContenuReference = normalizeRef(p.ContenuReference)
		collectRef(&result.References, p.SourceRBOK)
		collectRef(&result.References, p.ContenuReference)

		for j := range p.Modules {
			m := &p.Modules[j]
			m.SourceRBOK = normalizeRef(m.SourceRBOK)
			m.ContenuReference = normalizeRef(m.ContenuReference)
			collectRef(&result.References, m.SourceRBOK)
			collectRef(&result.References, m.ContenuReference)

			for k := range m.Questions {
				q := &m.Questions[k]
				q.SourceRBOK = normalizeRef(q.SourceRBOK)
				q.ContenuReference = normalizeRef(q.ContenuReference)
				collectRef(&result.References, q.SourceRBOK)
				collectRef(&result.References, q.ContenuReference)
			}
		}
	}

	// Update manifest in result with normalized values.
	result.Manifest = manifest

	// Detect unresolved RBOK references in raw content.
	result.Findings = append(result.Findings, detectUnresolvedRefs(path, data, result.References)...)

	return result, nil
}

// ValidateReferences checks that all collected references resolve against a known set.
func ValidateReferences(result ExtractionResult, knownRefs map[string]bool) []ExtractionFinding {
	var findings []ExtractionFinding
	for _, ref := range result.References {
		if !knownRefs[ref] {
			findings = append(findings, ExtractionFinding{
				Code:     "corpus_unresolved_ref",
				Severity: SeverityError,
				Path:     result.Manifest.Domain,
				Location: ref,
				Message:  fmt.Sprintf("RBOK reference %q does not resolve to a known source", ref),
			})
		}
	}
	return findings
}

func checkGovernance(path string, m ParcoursManifest) []ExtractionFinding {
	var findings []ExtractionFinding

	if m.Version == "" {
		findings = append(findings, ExtractionFinding{
			Code:     "corpus_partial",
			Severity: SeverityWarning,
			Path:     path,
			Message:  "missing governance metadata: version",
		})
	}
	if m.Owner == "" {
		findings = append(findings, ExtractionFinding{
			Code:     "corpus_partial",
			Severity: SeverityWarning,
			Path:     path,
			Message:  "missing governance metadata: owner",
		})
	}
	if m.Status == "" {
		findings = append(findings, ExtractionFinding{
			Code:     "corpus_partial",
			Severity: SeverityWarning,
			Path:     path,
			Message:  "missing governance metadata: status",
		})
	}
	if m.Domain == "" {
		findings = append(findings, ExtractionFinding{
			Code:     "corpus_partial",
			Severity: SeverityWarning,
			Path:     path,
			Message:  "missing governance metadata: domain",
		})
	}

	return findings
}

func detectUnresolvedRefs(path string, data []byte, collected []string) []ExtractionFinding {
	// Build set of already-collected refs.
	known := make(map[string]bool, len(collected))
	for _, ref := range collected {
		known[ref] = true
	}

	var findings []ExtractionFinding
	matches := rbekRefPattern.FindAllStringSubmatch(string(data), -1)
	for _, match := range matches {
		ref := normalizeRef(match[0])
		if ref != "" && !known[ref] {
			findings = append(findings, ExtractionFinding{
				Code:     "corpus_unresolved_ref",
				Severity: SeverityWarning,
				Path:     path,
				Location: ref,
				Message:  fmt.Sprintf("RBOK reference %q found in content but not in structured fields", ref),
			})
			known[ref] = true // avoid duplicates
		}
	}
	return findings
}

func normalizeRef(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	// Normalize separators: replace _ and - with /
	ref = strings.ReplaceAll(ref, "\\", "/")
	// Ensure lowercase prefix
	if strings.HasPrefix(strings.ToLower(ref), "rbok") {
		ref = "rbok" + ref[4:]
	}
	return ref
}

func collectRef(refs *[]string, ref string) {
	if ref == "" {
		return
	}
	for _, existing := range *refs {
		if existing == ref {
			return
		}
	}
	*refs = append(*refs, ref)
}

// ErrInvalidParcours is returned when a YAML file cannot be parsed as parcours.
var ErrInvalidParcours = errors.New("invalid parcours YAML")

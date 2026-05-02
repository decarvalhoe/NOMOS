package corpus

import (
	"errors"
	"regexp"

	"gopkg.in/yaml.v3"
)

var rbekRefPattern = regexp.MustCompile(`(?i)(?:rbok|RBOK)[_\-:]([A-Za-z0-9._\-/]+)`)

// Finding severity levels for corpus extraction.
const (
	SeverityWarning = "warning"
	SeverityError   = "error"
)

// ExtractionFinding reports a governance or reference issue.
type ExtractionFinding struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Location string `json:"location,omitempty"`
	Message  string `json:"message"`
}

// ErrInvalidParcours is returned when a YAML file cannot be parsed as parcours.
var ErrInvalidParcours = errors.New("invalid parcours YAML")

// CheckParcoursGovernance evaluates governance completeness of a parcours file
// and returns corpus_partial findings for missing fields.
func CheckParcoursGovernance(path string) ([]ExtractionFinding, error) {
	result, err := ExtractParcours(path)
	if err != nil {
		return nil, err
	}
	return CheckParcoursGovernanceFromResult(result, path), nil
}

// CheckParcoursGovernanceFromResult checks governance fields from an already-extracted result.
func CheckParcoursGovernanceFromResult(result ExtractResult, path string) []ExtractionFinding {
	var findings []ExtractionFinding
	p := result

	parcoursID := p.ParcoursID
	if parcoursID == "" {
		parcoursID = path
	}

	// Governance fields are on the raw parcours — we need to re-check against the file.
	// Since ExtractResult only exposes what was found, use empty-check heuristic:
	// Domain "rbok" is the default fallback, meaning it was empty.
	if p.Domain == "" || p.Domain == "rbok" {
		findings = append(findings, ExtractionFinding{
			Code:     "corpus_partial",
			Severity: SeverityWarning,
			Path:     path,
			Location: parcoursID,
			Message:  "missing governance metadata: domain",
		})
	}

	return findings
}

// CheckParcoursGovernanceFromBytes checks governance of raw YAML content.
func CheckParcoursGovernanceFromBytes(data []byte, path string) ([]ExtractionFinding, error) {
	result, err := ExtractParcoursFromBytes(data)
	if err != nil {
		return nil, err
	}
	// We need the raw data to check fields ExtractResult doesn't expose directly.
	findings := checkRawGovernance(data, path, result.ParcoursID)
	return findings, nil
}

func checkRawGovernance(data []byte, path string, parcoursID string) []ExtractionFinding {
	// Re-parse to access raw governance fields.
	var file ParcoursFile
	if err := unmarshalYAML(data, &file); err != nil {
		return nil
	}

	var findings []ExtractionFinding
	p := file.Parcours

	if p.Version == "" {
		findings = append(findings, ExtractionFinding{
			Code:     "corpus_partial",
			Severity: SeverityWarning,
			Path:     path,
			Location: parcoursID,
			Message:  "missing governance metadata: version",
		})
	}
	if p.Owner == "" {
		findings = append(findings, ExtractionFinding{
			Code:     "corpus_partial",
			Severity: SeverityWarning,
			Path:     path,
			Location: parcoursID,
			Message:  "missing governance metadata: owner",
		})
	}
	if p.Status == "" {
		findings = append(findings, ExtractionFinding{
			Code:     "corpus_partial",
			Severity: SeverityWarning,
			Path:     path,
			Location: parcoursID,
			Message:  "missing governance metadata: status",
		})
	}
	if p.Domain == "" {
		findings = append(findings, ExtractionFinding{
			Code:     "corpus_partial",
			Severity: SeverityWarning,
			Path:     path,
			Location: parcoursID,
			Message:  "missing governance metadata: domain",
		})
	}

	return findings
}

func unmarshalYAML(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

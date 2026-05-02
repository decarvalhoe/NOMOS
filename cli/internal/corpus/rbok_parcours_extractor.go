package corpus

import (
	"errors"
	"regexp"

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


// ExtractParcours parses an RBOK parcours YAML file and extracts governance
// metadata, normalized references, and findings for missing/unresolved data.


// ValidateReferences checks that all collected references resolve against a known set.





// ErrInvalidParcours is returned when a YAML file cannot be parsed as parcours.
var ErrInvalidParcours = errors.New("invalid parcours YAML")

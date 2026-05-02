package compliance

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationEntry represents a single validation in the inventory.
type ValidationEntry struct {
	ID                  string `yaml:"id"                   json:"id"`
	IntendedUseRef      string `yaml:"intended_use_ref"     json:"intended_use_ref"`
	Title               string `yaml:"title"                json:"title"`
	RiskLevel           string `yaml:"risk_level"           json:"risk_level"`
	ValidationType      string `yaml:"validation_type"      json:"validation_type"`
	Method              string `yaml:"method"               json:"method"`
	EvidenceArtifact    string `yaml:"evidence_artifact"    json:"evidence_artifact"`
	AcceptanceGate      string `yaml:"acceptance_gate"      json:"acceptance_gate"`
	Status              string `yaml:"status"               json:"status"`
	Owner               string `yaml:"owner"                json:"owner"`
	VerificationCommand string `yaml:"verification_command" json:"verification_command"`
	LastVerified        string `yaml:"last_verified"        json:"last_verified"`
}

// ValidationInventory is the top-level structure of the inventory YAML.
type ValidationInventory struct {
	SchemaVersion string            `yaml:"schema_version" json:"schema_version"`
	DocumentID    string            `yaml:"document_id"    json:"document_id"`
	Status        string            `yaml:"status"         json:"status"`
	Owner         string            `yaml:"owner"          json:"owner"`
	Product       string            `yaml:"product"        json:"product"`
	GeneratedFrom string            `yaml:"generated_from" json:"generated_from"`
	Validations   []ValidationEntry `yaml:"validations"    json:"validations"`
}

// IntendedUseFunc is a function from the intended-use model.
type IntendedUseFunc struct {
	ID           string `yaml:"id"            json:"id"`
	Function     string `yaml:"function"      json:"function"`
	RiskLevel    string `yaml:"risk_level"    json:"risk_level"`
	Verification string `yaml:"verification"  json:"verification"`
}

// IntendedUseModel is the simplified intended-use YAML structure.
type IntendedUseModel struct {
	IntendedUse struct {
		PrimaryFunctions []IntendedUseFunc `yaml:"primary_functions"`
	} `yaml:"intended_use"`
}

// InventoryFinding describes a completeness issue.
type InventoryFinding struct {
	ValidationID string `json:"validation_id"`
	Field        string `json:"field"`
	Severity     string `json:"severity"`
	Message      string `json:"message"`
}

// InventoryCheckResult holds the completeness check outcome.
type InventoryCheckResult struct {
	TotalValidations int                `json:"total_validations"`
	Complete         int                `json:"complete"`
	Incomplete       int                `json:"incomplete"`
	CoveredFunctions int                `json:"covered_functions"`
	UncoveredFuncs   []string           `json:"uncovered_functions,omitempty"`
	Findings         []InventoryFinding `json:"findings"`
	Verdict          string             `json:"verdict"`
}

// LoadInventory reads and parses a validation inventory YAML file.
func LoadInventory(path string) (ValidationInventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ValidationInventory{}, fmt.Errorf("read inventory: %w", err)
	}
	var inv ValidationInventory
	if err := yaml.Unmarshal(data, &inv); err != nil {
		return ValidationInventory{}, fmt.Errorf("parse inventory: %w", err)
	}
	return inv, nil
}

// LoadIntendedUse reads and parses an intended-use model YAML file.
func LoadIntendedUse(path string) (IntendedUseModel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return IntendedUseModel{}, fmt.Errorf("read intended-use: %w", err)
	}
	var model IntendedUseModel
	if err := yaml.Unmarshal(data, &model); err != nil {
		return IntendedUseModel{}, fmt.Errorf("parse intended-use: %w", err)
	}
	return model, nil
}

// CheckCompleteness verifies the inventory against structural and
// intended-use requirements. Every validation entry must have required
// fields, and every intended-use function must be covered.
func CheckCompleteness(inv ValidationInventory, intendedUse *IntendedUseModel) InventoryCheckResult {
	var findings []InventoryFinding
	complete := 0
	incomplete := 0

	seenIDs := map[string]bool{}
	coveredRefs := map[string]bool{}

	for _, v := range inv.Validations {
		entryFindings := checkEntry(v, seenIDs)
		findings = append(findings, entryFindings...)
		if len(entryFindings) == 0 {
			complete++
		} else {
			incomplete++
		}
		seenIDs[v.ID] = true
		if v.IntendedUseRef != "" {
			coveredRefs[v.IntendedUseRef] = true
		}
	}

	// Check intended-use coverage.
	var uncovered []string
	coveredCount := 0
	if intendedUse != nil {
		for _, f := range intendedUse.IntendedUse.PrimaryFunctions {
			if coveredRefs[f.ID] {
				coveredCount++
			} else {
				uncovered = append(uncovered, fmt.Sprintf("%s (%s)", f.ID, f.Function))
				findings = append(findings, InventoryFinding{
					Field:    "intended_use_coverage",
					Severity: "high",
					Message:  fmt.Sprintf("intended-use function %s (%s) has no validation entry", f.ID, f.Function),
				})
			}
		}
	}

	verdict := "pass"
	if incomplete > 0 || len(uncovered) > 0 {
		verdict = "fail"
	}

	return InventoryCheckResult{
		TotalValidations: len(inv.Validations),
		Complete:         complete,
		Incomplete:       incomplete,
		CoveredFunctions: coveredCount,
		UncoveredFuncs:   uncovered,
		Findings:         findings,
		Verdict:          verdict,
	}
}

var validRiskLevels = map[string]bool{
	"low": true, "medium": true, "high": true, "critical": true,
}

var validStatuses = map[string]bool{
	"not_qualified": true, "planned": true, "implemented": true,
	"verified": true, "approved": true, "waived": true, "blocked": true,
}

var validTypes = map[string]bool{
	"automated": true, "manual": true, "hybrid": true,
}

func checkEntry(v ValidationEntry, seenIDs map[string]bool) []InventoryFinding {
	var findings []InventoryFinding

	if v.ID == "" {
		findings = append(findings, InventoryFinding{
			ValidationID: v.ID, Field: "id", Severity: "critical",
			Message: "validation id is required",
		})
	} else if seenIDs[v.ID] {
		findings = append(findings, InventoryFinding{
			ValidationID: v.ID, Field: "id", Severity: "critical",
			Message: fmt.Sprintf("duplicate validation id %q", v.ID),
		})
	}

	if strings.TrimSpace(v.Title) == "" {
		findings = append(findings, InventoryFinding{
			ValidationID: v.ID, Field: "title", Severity: "critical",
			Message: "title is required",
		})
	}
	if !validRiskLevels[v.RiskLevel] {
		findings = append(findings, InventoryFinding{
			ValidationID: v.ID, Field: "risk_level", Severity: "high",
			Message: fmt.Sprintf("invalid risk_level %q", v.RiskLevel),
		})
	}
	if !validTypes[v.ValidationType] {
		findings = append(findings, InventoryFinding{
			ValidationID: v.ID, Field: "validation_type", Severity: "medium",
			Message: fmt.Sprintf("invalid validation_type %q", v.ValidationType),
		})
	}
	if strings.TrimSpace(v.Method) == "" {
		findings = append(findings, InventoryFinding{
			ValidationID: v.ID, Field: "method", Severity: "high",
			Message: "method is required",
		})
	}
	if strings.TrimSpace(v.EvidenceArtifact) == "" {
		findings = append(findings, InventoryFinding{
			ValidationID: v.ID, Field: "evidence_artifact", Severity: "high",
			Message: "evidence_artifact is required",
		})
	}
	if strings.TrimSpace(v.AcceptanceGate) == "" {
		findings = append(findings, InventoryFinding{
			ValidationID: v.ID, Field: "acceptance_gate", Severity: "high",
			Message: "acceptance_gate is required",
		})
	}
	if !validStatuses[v.Status] {
		findings = append(findings, InventoryFinding{
			ValidationID: v.ID, Field: "status", Severity: "high",
			Message: fmt.Sprintf("invalid status %q", v.Status),
		})
	}
	if strings.TrimSpace(v.Owner) == "" {
		findings = append(findings, InventoryFinding{
			ValidationID: v.ID, Field: "owner", Severity: "medium",
			Message: "owner is required",
		})
	}

	return findings
}

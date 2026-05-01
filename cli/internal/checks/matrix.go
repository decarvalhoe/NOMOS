package checks

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

const MatrixCheckFormat = "nomos.matrix-check.v1"

// Diagnostic codes emitted by CheckMatrix.
const (
	CodeMissingSourceRef      = "MISSING_SOURCE_REF"
	CodeMissingTest           = "MISSING_TEST"
	CodeBrokenContractRef     = "BROKEN_CONTRACT_REF"
	CodeEmptySourceRefs       = "EMPTY_SOURCE_REFS"
	CodeInvalidUnitID         = "INVALID_UNIT_ID"
	CodeCoveredWithGaps       = "COVERED_WITH_GAPS"
	CodeDeprecatedNoDecision  = "DEPRECATED_NO_DECISION"
)

var unitIDPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]*$`)
var sourceIDPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]*$`)

// CanonicalMatrix is the top-level structure parsed from a canonical-matrix YAML file.
type CanonicalMatrix struct {
	SchemaVersion string `yaml:"schema_version"`
	Units         []Unit `yaml:"units"`
}

// Unit represents a single traceability unit in the matrix.
type Unit struct {
	UnitID           string            `yaml:"unit_id"`
	UnitType         string            `yaml:"unit_type"`
	Name             string            `yaml:"name"`
	Domain           string            `yaml:"domain"`
	Criticality      string            `yaml:"criticality"`
	SourceRefs       []SourceRef       `yaml:"source_refs"`
	BusinessRule     string            `yaml:"business_rule"`
	CanonicalContract *ContractRef     `yaml:"canonical_contract,omitempty"`
	TestRefs         []string          `yaml:"test_refs,omitempty"`
	DecisionRefs     []string          `yaml:"decision_refs,omitempty"`
	CoreRefs         []CodeRef         `yaml:"core_refs,omitempty"`
	SchemaRefs       []string          `yaml:"schema_refs,omitempty"`
	Status           string            `yaml:"status"`
	Gaps             []string          `yaml:"gaps,omitempty"`
}

// SourceRef points to an upstream source document.
type SourceRef struct {
	SourceID string `yaml:"source_id"`
	Locator  string `yaml:"locator"`
	Hash     string `yaml:"hash,omitempty"`
}

// ContractRef links a unit to its canonical contract file.
type ContractRef struct {
	Path     string `yaml:"path"`
	ObjectID string `yaml:"object_id"`
	Status   string `yaml:"status"`
}

// CodeRef points to a code module/symbol.
type CodeRef struct {
	Package string `yaml:"package,omitempty"`
	Module  string `yaml:"module"`
	Symbol  string `yaml:"symbol,omitempty"`
}

// Severity indicates the severity of a diagnostic finding.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Finding describes a single check diagnostic.
type Finding struct {
	UnitID   string   `json:"unit_id"`
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// MatrixCheckResult holds the outcome of a matrix check run.
type MatrixCheckResult struct {
	Format        string    `json:"format"`
	TotalUnits    int       `json:"total_units"`
	CoveredUnits  int       `json:"covered_units"`
	CoverageScore float64   `json:"coverage_score"`
	Findings      []Finding `json:"findings"`
}

// ParseMatrixFile reads and parses a canonical-matrix YAML file.
func ParseMatrixFile(path string) (CanonicalMatrix, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CanonicalMatrix{}, fmt.Errorf("read matrix file: %w", err)
	}
	return ParseMatrix(data)
}

// ParseMatrix parses canonical-matrix YAML content.
func ParseMatrix(data []byte) (CanonicalMatrix, error) {
	var m CanonicalMatrix
	if err := yaml.Unmarshal(data, &m); err != nil {
		return CanonicalMatrix{}, fmt.Errorf("parse matrix yaml: %w", err)
	}
	if len(m.Units) == 0 {
		return CanonicalMatrix{}, fmt.Errorf("matrix contains no units")
	}
	return m, nil
}

// CheckMatrix validates a canonical matrix and returns diagnostics with a coverage score.
func CheckMatrix(m CanonicalMatrix) MatrixCheckResult {
	result := MatrixCheckResult{
		Format:     MatrixCheckFormat,
		TotalUnits: len(m.Units),
	}

	for _, u := range m.Units {
		checkUnit(u, &result)
	}

	if result.TotalUnits > 0 {
		result.CoverageScore = float64(result.CoveredUnits) / float64(result.TotalUnits)
	}

	return result
}

func checkUnit(u Unit, result *MatrixCheckResult) {
	// Validate unit_id format.
	if !unitIDPattern.MatchString(u.UnitID) {
		result.Findings = append(result.Findings, Finding{
			UnitID:   u.UnitID,
			Code:     CodeInvalidUnitID,
			Severity: SeverityError,
			Message:  fmt.Sprintf("unit_id %q does not match pattern ^[A-Z0-9][A-Z0-9-]*$", u.UnitID),
		})
	}

	// Check source_refs are present and valid.
	if len(u.SourceRefs) == 0 {
		result.Findings = append(result.Findings, Finding{
			UnitID:   u.UnitID,
			Code:     CodeEmptySourceRefs,
			Severity: SeverityError,
			Message:  "unit has no source_refs",
		})
	} else {
		for _, ref := range u.SourceRefs {
			if !sourceIDPattern.MatchString(ref.SourceID) {
				result.Findings = append(result.Findings, Finding{
					UnitID:   u.UnitID,
					Code:     CodeMissingSourceRef,
					Severity: SeverityError,
					Message:  fmt.Sprintf("source_ref source_id %q is invalid", ref.SourceID),
				})
			}
			if ref.Locator == "" {
				result.Findings = append(result.Findings, Finding{
					UnitID:   u.UnitID,
					Code:     CodeMissingSourceRef,
					Severity: SeverityError,
					Message:  fmt.Sprintf("source_ref %q has empty locator", ref.SourceID),
				})
			}
		}
	}

	// Check covered units must have test_refs.
	if u.Status == "covered" && len(u.TestRefs) == 0 {
		result.Findings = append(result.Findings, Finding{
			UnitID:   u.UnitID,
			Code:     CodeMissingTest,
			Severity: SeverityError,
			Message:  "unit has status \"covered\" but no test_refs",
		})
	}

	// Check covered units should not have non-empty gaps.
	if u.Status == "covered" && len(u.Gaps) > 0 {
		hasNonEmpty := false
		for _, g := range u.Gaps {
			if g != "" {
				hasNonEmpty = true
				break
			}
		}
		if hasNonEmpty {
			result.Findings = append(result.Findings, Finding{
				UnitID:   u.UnitID,
				Code:     CodeCoveredWithGaps,
				Severity: SeverityWarning,
				Message:  "unit has status \"covered\" but non-empty gaps",
			})
		}
	}

	// Check canonical_contract.object_id matches unit_id.
	if u.CanonicalContract != nil {
		if u.CanonicalContract.ObjectID != u.UnitID {
			result.Findings = append(result.Findings, Finding{
				UnitID:   u.UnitID,
				Code:     CodeBrokenContractRef,
				Severity: SeverityError,
				Message: fmt.Sprintf(
					"canonical_contract.object_id %q does not match unit_id %q",
					u.CanonicalContract.ObjectID, u.UnitID,
				),
			})
		}
		if u.CanonicalContract.Path == "" {
			result.Findings = append(result.Findings, Finding{
				UnitID:   u.UnitID,
				Code:     CodeBrokenContractRef,
				Severity: SeverityError,
				Message:  "canonical_contract.path is empty",
			})
		}
	}

	// Check deprecated units must have decision_refs.
	if u.Status == "deprecated" && len(u.DecisionRefs) == 0 {
		result.Findings = append(result.Findings, Finding{
			UnitID:   u.UnitID,
			Code:     CodeDeprecatedNoDecision,
			Severity: SeverityWarning,
			Message:  "unit has status \"deprecated\" but no decision_refs",
		})
	}

	// Count covered units for score.
	if u.Status == "covered" {
		// Only count as covered if there are no error-level findings for this unit.
		hasError := false
		for _, f := range result.Findings {
			if f.UnitID == u.UnitID && f.Severity == SeverityError {
				hasError = true
				break
			}
		}
		if !hasError {
			result.CoveredUnits++
		}
	}
}

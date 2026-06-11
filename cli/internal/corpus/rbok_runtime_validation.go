package corpus

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// ValidationGateStatus is the outcome of the validation gate.
type ValidationGateStatus string

const (
	ValidationPassed  ValidationGateStatus = "passed"
	ValidationFailed  ValidationGateStatus = "failed"
	ValidationWarning ValidationGateStatus = "warning"
)

// ValidationCheck is a single check within the validation pack.
type ValidationCheck struct {
	ID       string               `json:"id"`
	Name     string               `json:"name"`
	Status   ValidationGateStatus `json:"status"`
	Message  string               `json:"message"`
	Severity string               `json:"severity"` // "info", "medium", "high", "critical"
}

// ValidationPack is the complete output validation result.
type ValidationPack struct {
	SchemaVersion string               `json:"schema_version"`
	GateID        string               `json:"gate_id"`
	Status        ValidationGateStatus `json:"status"`
	GeneratedAt   string               `json:"generated_at"`
	ContentHash   string               `json:"content_hash,omitempty"`
	Checks        []ValidationCheck    `json:"checks"`
	Summary       ValidationSummary    `json:"summary"`
}

// ValidationSummary provides aggregate check results.
type ValidationSummary struct {
	TotalChecks int `json:"total_checks"`
	Passed      int `json:"passed"`
	Failed      int `json:"failed"`
	Warnings    int `json:"warnings"`
}

// ValidationInput holds the artifacts to validate.
type ValidationInput struct {
	Feed          *RuntimeFeed
	NodeCount     int
	HasGovernance bool
	HasRAGMeta    bool
	HasIndex      bool
	HasEngine     bool
	LayerCount    int
	ContentHash   string
}

// ValidationOptions configures the validation gate.
type ValidationOptions struct {
	GateID        string
	MinNodes      int
	RequireRAG    bool
	RequireEngine bool
	RequireIndex  bool
	Now           time.Time
}

// DefaultValidationOptions returns sensible defaults.
func DefaultValidationOptions() ValidationOptions {
	return ValidationOptions{
		GateID:        "rbok.output.validation",
		MinNodes:      1,
		RequireRAG:    true,
		RequireEngine: true,
		RequireIndex:  true,
	}
}

// ValidateRuntimeOutput runs all output validation checks and produces a ValidationPack.
func ValidateRuntimeOutput(input ValidationInput, opts ValidationOptions) ValidationPack {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if opts.GateID == "" {
		opts.GateID = "rbok.output.validation"
	}
	if opts.MinNodes == 0 {
		opts.MinNodes = 1
	}

	checks := runValidationChecks(input, opts)

	failed := 0
	passed := 0
	warnings := 0
	for _, c := range checks {
		switch c.Status {
		case ValidationPassed:
			passed++
		case ValidationFailed:
			failed++
		case ValidationWarning:
			warnings++
		}
	}

	status := ValidationPassed
	if failed > 0 {
		status = ValidationFailed
	} else if warnings > 0 {
		status = ValidationWarning
	}

	return ValidationPack{
		SchemaVersion: "0.1.0",
		GateID:        opts.GateID,
		Status:        status,
		GeneratedAt:   now.Format(time.RFC3339),
		ContentHash:   input.ContentHash,
		Checks:        checks,
		Summary: ValidationSummary{
			TotalChecks: len(checks),
			Passed:      passed,
			Failed:      failed,
			Warnings:    warnings,
		},
	}
}

// WriteValidationPack writes the pack as indented JSON.
func WriteValidationPack(w io.Writer, pack ValidationPack) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(pack)
}

func runValidationChecks(input ValidationInput, opts ValidationOptions) []ValidationCheck {
	var checks []ValidationCheck

	// Check: feed present
	checks = append(checks, checkFeedPresent(input))

	// Check: minimum node count
	checks = append(checks, checkMinNodes(input, opts.MinNodes))

	// Check: layers present
	checks = append(checks, checkLayers(input))

	// Check: governance report present
	checks = append(checks, checkGovernancePresent(input))

	// Check: RAG metadata present
	checks = append(checks, checkRAGMetadata(input, opts.RequireRAG))

	// Check: index present
	checks = append(checks, checkIndex(input, opts.RequireIndex))

	// Check: engine import present
	checks = append(checks, checkEngineImport(input, opts.RequireEngine))

	// Check: content hash present
	checks = append(checks, checkContentHash(input))

	return checks
}

func checkFeedPresent(input ValidationInput) ValidationCheck {
	if input.Feed == nil {
		return ValidationCheck{
			ID: "output.feed_present", Name: "Feed artifact present",
			Status: ValidationFailed, Severity: "critical",
			Message: "Runtime feed artifact is missing.",
		}
	}
	return ValidationCheck{
		ID: "output.feed_present", Name: "Feed artifact present",
		Status: ValidationPassed, Severity: "info",
		Message: "Runtime feed artifact is present.",
	}
}

func checkMinNodes(input ValidationInput, minNodes int) ValidationCheck {
	if input.NodeCount < minNodes {
		return ValidationCheck{
			ID: "output.min_nodes", Name: "Minimum node count",
			Status: ValidationFailed, Severity: "high",
			Message: fmt.Sprintf("Node count %d is below minimum %d.", input.NodeCount, minNodes),
		}
	}
	return ValidationCheck{
		ID: "output.min_nodes", Name: "Minimum node count",
		Status: ValidationPassed, Severity: "info",
		Message: fmt.Sprintf("Node count %d meets minimum %d.", input.NodeCount, minNodes),
	}
}

func checkLayers(input ValidationInput) ValidationCheck {
	if input.LayerCount == 0 {
		return ValidationCheck{
			ID: "output.layers", Name: "Layer provenance",
			Status: ValidationFailed, Severity: "high",
			Message: "No layers present in feed output.",
		}
	}
	return ValidationCheck{
		ID: "output.layers", Name: "Layer provenance",
		Status: ValidationPassed, Severity: "info",
		Message: fmt.Sprintf("%d layer(s) present.", input.LayerCount),
	}
}

func checkGovernancePresent(input ValidationInput) ValidationCheck {
	if !input.HasGovernance {
		return ValidationCheck{
			ID: "output.governance", Name: "Governance report present",
			Status: ValidationFailed, Severity: "high",
			Message: "Governance report is missing from output.",
		}
	}
	return ValidationCheck{
		ID: "output.governance", Name: "Governance report present",
		Status: ValidationPassed, Severity: "info",
		Message: "Governance report is present.",
	}
}

func checkRAGMetadata(input ValidationInput, required bool) ValidationCheck {
	if !input.HasRAGMeta {
		status := ValidationWarning
		severity := "medium"
		if required {
			status = ValidationFailed
			severity = "high"
		}
		return ValidationCheck{
			ID: "output.rag_metadata", Name: "RAG metadata present",
			Status: status, Severity: severity,
			Message: "RAG metadata is missing from output.",
		}
	}
	return ValidationCheck{
		ID: "output.rag_metadata", Name: "RAG metadata present",
		Status: ValidationPassed, Severity: "info",
		Message: "RAG metadata is present.",
	}
}

func checkIndex(input ValidationInput, required bool) ValidationCheck {
	if !input.HasIndex {
		status := ValidationWarning
		severity := "medium"
		if required {
			status = ValidationFailed
			severity = "high"
		}
		return ValidationCheck{
			ID: "output.index", Name: "Feed index present",
			Status: status, Severity: severity,
			Message: "Feed index is missing from output.",
		}
	}
	return ValidationCheck{
		ID: "output.index", Name: "Feed index present",
		Status: ValidationPassed, Severity: "info",
		Message: "Feed index is present.",
	}
}

func checkEngineImport(input ValidationInput, required bool) ValidationCheck {
	if !input.HasEngine {
		status := ValidationWarning
		severity := "medium"
		if required {
			status = ValidationFailed
			severity = "high"
		}
		return ValidationCheck{
			ID: "output.engine_import", Name: "Engine import present",
			Status: status, Severity: severity,
			Message: "Engine import projection is missing from output.",
		}
	}
	return ValidationCheck{
		ID: "output.engine_import", Name: "Engine import present",
		Status: ValidationPassed, Severity: "info",
		Message: "Engine import projection is present.",
	}
}

func checkContentHash(input ValidationInput) ValidationCheck {
	if input.ContentHash == "" {
		return ValidationCheck{
			ID: "output.content_hash", Name: "Content hash",
			Status: ValidationWarning, Severity: "medium",
			Message: "Content hash is not set on feed output.",
		}
	}
	return ValidationCheck{
		ID: "output.content_hash", Name: "Content hash",
		Status: ValidationPassed, Severity: "info",
		Message: "Content hash is present.",
	}
}

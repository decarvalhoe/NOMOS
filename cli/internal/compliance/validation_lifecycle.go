package compliance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidationArtifact describes an expected artifact in the validation pack.
type ValidationArtifact struct {
	ID          string
	Name        string
	RelPath     string
	Severity    string
	Blocking    bool
	Check       func(root string) (bool, string)
	Remediation string
	Owner       string
}

// ValidationLifecycleResult holds the validation pack evaluation.
type ValidationLifecycleResult struct {
	Verdict       string    `json:"verdict"        yaml:"verdict"`
	TotalArtifacts int      `json:"total_artifacts" yaml:"total_artifacts"`
	Present       int       `json:"present"        yaml:"present"`
	TotalFindings int       `json:"total_findings" yaml:"total_findings"`
	Blocking      int       `json:"blocking"       yaml:"blocking"`
	Findings      []Finding `json:"findings"       yaml:"findings"`
}

// EvaluateValidationLifecycle checks the validation-pack for completeness.
func EvaluateValidationLifecycle(root string) (ValidationLifecycleResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ValidationLifecycleResult{}, fmt.Errorf("resolve root: %w", err)
	}
	if info, err := os.Stat(absRoot); err != nil || !info.IsDir() {
		return ValidationLifecycleResult{}, fmt.Errorf("root must be a directory: %s", absRoot)
	}

	artifacts := validationArtifacts()
	var findings []Finding
	present := 0
	idx := 0

	for _, art := range artifacts {
		ok, detail := art.Check(absRoot)
		if ok {
			present++
			continue
		}
		idx++
		findings = append(findings, Finding{
			ID:          fmt.Sprintf("VL-%04d", idx),
			Control:     art.ID,
			Severity:    art.Severity,
			Blocking:    art.Blocking,
			Path:        detail,
			Message:     fmt.Sprintf("validation artifact %s (%s) missing", art.ID, art.Name),
			Remediation: art.Remediation,
			Owner:       art.Owner,
		})
	}

	blocking := 0
	for _, f := range findings {
		if f.Blocking {
			blocking++
		}
	}

	verdict := VerdictCompliant
	if blocking > 0 {
		verdict = VerdictNonCompliant
	} else if len(findings) > 0 {
		verdict = VerdictPartial
	}

	return ValidationLifecycleResult{
		Verdict:        verdict,
		TotalArtifacts: len(artifacts),
		Present:        present,
		TotalFindings:  len(findings),
		Blocking:       blocking,
		Findings:       findings,
	}, nil
}

func validationArtifacts() []ValidationArtifact {
	return []ValidationArtifact{
		{
			ID:          "VAL-MASTER-PLAN",
			Name:        "Validation master plan",
			RelPath:     "docs/regulated/validation-pack/validation-master-plan.md",
			Severity:    "critical",
			Blocking:    true,
			Check:       checkFile("docs/regulated/validation-pack/validation-master-plan.md"),
			Remediation: "Create validation-master-plan.md with scope, strategy, acceptance criteria, and approval section.",
			Owner:       "quality-owner",
		},
		{
			ID:          "VAL-INTENDED-USE",
			Name:        "Intended-use model",
			RelPath:     "docs/regulated/validation-pack/intended-use-model.yaml",
			Severity:    "critical",
			Blocking:    true,
			Check:       checkFile("docs/regulated/validation-pack/intended-use-model.yaml"),
			Remediation: "Create intended-use-model.yaml with primary functions, out-of-scope, user profiles, and claim boundary.",
			Owner:       "product-owner",
		},
		{
			ID:          "VAL-INTENDED-USE-BOUNDARY",
			Name:        "Intended-use claim boundary declared",
			RelPath:     "docs/regulated/validation-pack/intended-use-model.yaml",
			Severity:    "high",
			Blocking:    true,
			Check:       checkIntendedUseClaimBoundary,
			Remediation: "Add claim_boundary field to intended-use-model.yaml.",
			Owner:       "product-owner",
		},
		{
			ID:          "VAL-INTENDED-USE-FUNCTIONS",
			Name:        "Intended-use primary functions listed",
			RelPath:     "docs/regulated/validation-pack/intended-use-model.yaml",
			Severity:    "high",
			Blocking:    true,
			Check:       checkIntendedUseFunctions,
			Remediation: "Add intended_use.primary_functions with at least one function entry.",
			Owner:       "product-owner",
		},
		{
			ID:          "VAL-TEST-PROTOCOL",
			Name:        "Test protocol template",
			RelPath:     "docs/regulated/validation-pack/test-protocol-template.yaml",
			Severity:    "medium",
			Blocking:    false,
			Check:       checkFile("docs/regulated/validation-pack/test-protocol-template.yaml"),
			Remediation: "Create test-protocol-template.yaml with test case structure, deviation log, and approval section.",
			Owner:       "validation-owner",
		},
		{
			ID:          "VAL-PLAN-SCOPE",
			Name:        "Validation plan has scope section",
			RelPath:     "docs/regulated/validation-pack/validation-master-plan.md",
			Severity:    "medium",
			Blocking:    false,
			Check:       checkFileContains("docs/regulated/validation-pack/validation-master-plan.md", "## Scope"),
			Remediation: "Add a ## Scope section to the validation master plan.",
			Owner:       "quality-owner",
		},
		{
			ID:          "VAL-PLAN-ACCEPTANCE",
			Name:        "Validation plan has acceptance criteria",
			RelPath:     "docs/regulated/validation-pack/validation-master-plan.md",
			Severity:    "medium",
			Blocking:    false,
			Check:       checkFileContains("docs/regulated/validation-pack/validation-master-plan.md", "## Acceptance Criteria"),
			Remediation: "Add a ## Acceptance Criteria section to the validation master plan.",
			Owner:       "quality-owner",
		},
		{
			ID:          "VAL-PLAN-APPROVAL",
			Name:        "Validation plan has approval section",
			RelPath:     "docs/regulated/validation-pack/validation-master-plan.md",
			Severity:    "low",
			Blocking:    false,
			Check:       checkFileContains("docs/regulated/validation-pack/validation-master-plan.md", "## Approval"),
			Remediation: "Add a ## Approval section with role/name/date/signature table.",
			Owner:       "quality-owner",
		},
	}
}

func checkFile(rel string) func(string) (bool, string) {
	return func(root string) (bool, string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return false, rel
		}
		return true, ""
	}
}

func checkFileContains(rel string, needle string) func(string) (bool, string) {
	return func(root string) (bool, string) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(path)
		if err != nil {
			return false, rel
		}
		if !strings.Contains(string(data), needle) {
			return false, rel + " (missing: " + needle + ")"
		}
		return true, ""
	}
}

func checkIntendedUseClaimBoundary(root string) (bool, string) {
	rel := "docs/regulated/validation-pack/intended-use-model.yaml"
	path := filepath.Join(root, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		return false, rel
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, rel
	}
	cb, ok := doc["claim_boundary"]
	if !ok || strings.TrimSpace(fmt.Sprint(cb)) == "" {
		return false, rel + " (missing claim_boundary)"
	}
	return true, ""
}

func checkIntendedUseFunctions(root string) (bool, string) {
	rel := "docs/regulated/validation-pack/intended-use-model.yaml"
	path := filepath.Join(root, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		return false, rel
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, rel
	}
	iu, ok := doc["intended_use"]
	if !ok {
		return false, rel + " (missing intended_use)"
	}
	iuMap, ok := iu.(map[string]any)
	if !ok {
		return false, rel + " (intended_use not a map)"
	}
	funcs, ok := iuMap["primary_functions"]
	if !ok {
		return false, rel + " (missing primary_functions)"
	}
	funcList, ok := funcs.([]any)
	if !ok || len(funcList) == 0 {
		return false, rel + " (primary_functions empty)"
	}
	return true, ""
}

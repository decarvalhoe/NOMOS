package productcheck

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	lowerIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

var (
	lifecycleValues   = []string{"greenfield", "brownfield"}
	riskLevels        = []string{"low", "medium", "high", "critical"}
	scopeVerdicts     = []string{"in_scope", "partial", "blocked", "out_of_scope"}
	confidenceLevels  = []string{"low", "medium", "high"}
	severities        = []string{"low", "medium", "high", "critical"}
	surfaceTypes      = []string{"api", "ui", "worker", "data", "infra", "docs", "event", "cli", "batch"}
	dataSensitivity   = []string{"public", "internal", "restricted", "secret"}
	reportTypes       = []string{"nomos-report.json", "coverage-report.md", "attestation", "sbom", "provenance"}
	attestationLevels = []string{"none", "basic", "signed"}
)

type projectManifest struct {
	SchemaVersion string  `yaml:"schema_version"`
	Project       project `yaml:"project"`
	Scope         scope   `yaml:"scope"`
	Surfaces      []surface `yaml:"surfaces"`
	Toolchain     toolchain `yaml:"toolchain"`
	Compliance    compliance `yaml:"compliance"`
	Evidence      evidence  `yaml:"evidence"`
	Notes         string    `yaml:"notes"`
}

type project struct {
	ID          string  `yaml:"id"`
	Name        string  `yaml:"name"`
	Description string  `yaml:"description"`
	Repository  string  `yaml:"repository"`
	Domain      string  `yaml:"domain"`
	Lifecycle   string  `yaml:"lifecycle"`
	RiskLevel   string  `yaml:"risk_level"`
	Owners      []owner `yaml:"owners"`
}

type owner struct {
	Name  string `yaml:"name"`
	Role  string `yaml:"role"`
	Email string `yaml:"email"`
}

type scope struct {
	Verdict         string    `yaml:"verdict"`
	Confidence      string    `yaml:"confidence"`
	InScope         []string  `yaml:"in_scope"`
	OutOfScope      []string  `yaml:"out_of_scope"`
	Assumptions     []string  `yaml:"assumptions"`
	BoundedContexts []string  `yaml:"bounded_contexts"`
	Blockers        []blocker `yaml:"blockers"`
}

type blocker struct {
	ID          string `yaml:"id"`
	Severity    string `yaml:"severity"`
	Description string `yaml:"description"`
	Remediation string `yaml:"remediation"`
}

type surface struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	Path        string   `yaml:"path"`
	Stack       string   `yaml:"stack"`
	Critical    bool     `yaml:"critical"`
	Entrypoints []string `yaml:"entrypoints"`
}

type toolchain struct {
	Build           []string `yaml:"build"`
	Test            []string `yaml:"test"`
	Lint            []string `yaml:"lint"`
	Typecheck       []string `yaml:"typecheck"`
	PackageManagers []string `yaml:"package_managers"`
	CISystems       []string `yaml:"ci_systems"`
}

type compliance struct {
	Regulated         bool     `yaml:"regulated"`
	Standards         []string `yaml:"standards"`
	DataSensitivity   string   `yaml:"data_sensitivity"`
	ExceptionsAllowed bool     `yaml:"exceptions_allowed"`
}

type evidence struct {
	RequiredReports  []string `yaml:"required_reports"`
	AttestationLevel string   `yaml:"attestation_level"`
}

type CheckResult struct {
	Valid  bool         `json:"valid"`
	Errors []CheckError `json:"errors,omitempty"`
}

type CheckError struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

func CheckProduct(manifestPath string) (CheckResult, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return CheckResult{}, fmt.Errorf("reading manifest: %w", err)
	}
	return CheckProductFromBytes(data)
}

func CheckProductFromBytes(data []byte) (CheckResult, error) {
	var manifest projectManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return CheckResult{}, fmt.Errorf("parsing manifest: %w", err)
	}

	var errors []CheckError
	checkProject(&errors, manifest.Project)
	checkScope(&errors, manifest.Scope)
	checkSurfaces(&errors, manifest.Surfaces)
	checkCompliance(&errors, manifest.Compliance)
	checkEvidence(&errors, manifest.Evidence)
	checkBlockers(&errors, manifest.Scope.Blockers)

	return CheckResult{
		Valid:  len(errors) == 0,
		Errors: errors,
	}, nil
}

func checkProject(errors *[]CheckError, p project) {
	addRequired(errors, "project.id", p.ID, "MISSING_PROJECT_ID")
	if p.ID != "" && !lowerIDPattern.MatchString(p.ID) {
		addError(errors, "INVALID_PROJECT_ID", "project.id",
			fmt.Sprintf("id %q must match %s", p.ID, lowerIDPattern.String()))
	}
	addRequired(errors, "project.name", p.Name, "MISSING_PROJECT_NAME")
	addRequired(errors, "project.domain", p.Domain, "MISSING_DOMAIN")
	addRequired(errors, "project.lifecycle", p.Lifecycle, "MISSING_LIFECYCLE")
	addEnum(errors, "project.lifecycle", p.Lifecycle, lifecycleValues, "INVALID_LIFECYCLE")
	addRequired(errors, "project.risk_level", p.RiskLevel, "MISSING_RISK_LEVEL")
	addEnum(errors, "project.risk_level", p.RiskLevel, riskLevels, "INVALID_RISK_LEVEL")

	if len(p.Owners) == 0 {
		addError(errors, "NO_OWNERS", "project.owners", "at least one owner is required")
	}
	for i, o := range p.Owners {
		if strings.TrimSpace(o.Name) == "" {
			addError(errors, "MISSING_OWNER_NAME", fmt.Sprintf("project.owners[%d].name", i), "owner name is required")
		}
	}
}

func checkScope(errors *[]CheckError, s scope) {
	addEnum(errors, "scope.verdict", s.Verdict, scopeVerdicts, "INVALID_SCOPE_VERDICT")
	addEnum(errors, "scope.confidence", s.Confidence, confidenceLevels, "INVALID_CONFIDENCE")

	if len(s.InScope) == 0 {
		addError(errors, "NO_IN_SCOPE", "scope.in_scope", "at least one in_scope item is required")
	}
}

func checkSurfaces(errors *[]CheckError, surfaces []surface) {
	if len(surfaces) == 0 {
		addError(errors, "NO_SURFACES", "surfaces", "at least one surface is required")
		return
	}
	for i, s := range surfaces {
		prefix := fmt.Sprintf("surfaces[%d]", i)
		if strings.TrimSpace(s.Name) == "" {
			addError(errors, "MISSING_SURFACE_NAME", prefix+".name", "surface name is required")
		}
		if strings.TrimSpace(s.Type) == "" {
			addError(errors, "MISSING_SURFACE_TYPE", prefix+".type", "surface type is required")
		} else if !slices.Contains(surfaceTypes, s.Type) {
			addError(errors, "INVALID_SURFACE_TYPE", prefix+".type",
				fmt.Sprintf("type %q must be one of: %s", s.Type, strings.Join(surfaceTypes, ", ")))
		}
	}
}

func checkCompliance(errors *[]CheckError, c compliance) {
	addEnum(errors, "compliance.data_sensitivity", c.DataSensitivity, dataSensitivity, "INVALID_DATA_SENSITIVITY")
}

func checkEvidence(errors *[]CheckError, e evidence) {
	for i, report := range e.RequiredReports {
		if !slices.Contains(reportTypes, report) {
			addError(errors, "INVALID_REPORT_TYPE", fmt.Sprintf("evidence.required_reports[%d]", i),
				fmt.Sprintf("report %q must be one of: %s", report, strings.Join(reportTypes, ", ")))
		}
	}
	addEnum(errors, "evidence.attestation_level", e.AttestationLevel, attestationLevels, "INVALID_ATTESTATION_LEVEL")
}

func checkBlockers(errors *[]CheckError, blockers []blocker) {
	for i, b := range blockers {
		prefix := fmt.Sprintf("scope.blockers[%d]", i)
		if strings.TrimSpace(b.ID) == "" {
			addError(errors, "MISSING_BLOCKER_ID", prefix+".id", "blocker id is required")
		}
		if strings.TrimSpace(b.Severity) == "" {
			addError(errors, "MISSING_BLOCKER_SEVERITY", prefix+".severity", "blocker severity is required")
		} else if !slices.Contains(severities, b.Severity) {
			addError(errors, "INVALID_BLOCKER_SEVERITY", prefix+".severity",
				fmt.Sprintf("severity %q must be one of: %s", b.Severity, strings.Join(severities, ", ")))
		}
		if strings.TrimSpace(b.Description) == "" {
			addError(errors, "MISSING_BLOCKER_DESCRIPTION", prefix+".description", "blocker description is required")
		}
	}
}

func addRequired(errors *[]CheckError, path string, value string, code string) {
	if strings.TrimSpace(value) == "" {
		addError(errors, code, path, "value is required")
	}
}

func addEnum(errors *[]CheckError, path string, value string, allowed []string, code string) {
	if value == "" {
		return
	}
	if !slices.Contains(allowed, value) {
		addError(errors, code, path,
			fmt.Sprintf("%q must be one of: %s", value, strings.Join(allowed, ", ")))
	}
}

func addError(errors *[]CheckError, code string, path string, message string) {
	*errors = append(*errors, CheckError{
		Code:    code,
		Path:    path,
		Message: message,
	})
}

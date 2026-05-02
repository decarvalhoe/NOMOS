package compliance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Verdicts for self-compliance evaluation.
const (
	VerdictCompliant    = "compliant"
	VerdictPartial      = "partial"
	VerdictNonCompliant = "non_compliant"
)

// Finding describes a single compliance gap.
type Finding struct {
	ID          string `json:"id"          yaml:"id"`
	Control     string `json:"control"     yaml:"control"`
	Severity    string `json:"severity"    yaml:"severity"`
	Blocking    bool   `json:"blocking"    yaml:"blocking"`
	Path        string `json:"path"        yaml:"path"`
	Message     string `json:"message"     yaml:"message"`
	Remediation string `json:"remediation" yaml:"remediation"`
	Owner       string `json:"owner"       yaml:"owner"`
}

// SelfComplianceResult holds the full evaluation.
type SelfComplianceResult struct {
	Verdict       string    `json:"verdict"        yaml:"verdict"`
	TotalControls int       `json:"total_controls" yaml:"total_controls"`
	Satisfied     int       `json:"satisfied"      yaml:"satisfied"`
	TotalFindings int       `json:"total_findings" yaml:"total_findings"`
	Blocking      int       `json:"blocking"       yaml:"blocking"`
	Findings      []Finding `json:"findings"       yaml:"findings"`
}

// Control describes a regulated control to verify.
type Control struct {
	ID           string
	Name         string
	Severity     string
	Blocking     bool
	Check        func(root string) (bool, string)
	Remediation  string
	Owner        string
}

// EvaluateSelfCompliance checks a Nomos repo against regulated controls.
func EvaluateSelfCompliance(root string) (SelfComplianceResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return SelfComplianceResult{}, fmt.Errorf("resolve root: %w", err)
	}
	if info, err := os.Stat(absRoot); err != nil || !info.IsDir() {
		return SelfComplianceResult{}, fmt.Errorf("root must be a directory: %s", absRoot)
	}

	controls := regulatedControls()
	var findings []Finding
	satisfied := 0
	idx := 0

	for _, ctrl := range controls {
		ok, detail := ctrl.Check(absRoot)
		if ok {
			satisfied++
			continue
		}
		idx++
		findings = append(findings, Finding{
			ID:          fmt.Sprintf("RC-%04d", idx),
			Control:     ctrl.ID,
			Severity:    ctrl.Severity,
			Blocking:    ctrl.Blocking,
			Path:        detail,
			Message:     fmt.Sprintf("control %s (%s) not satisfied", ctrl.ID, ctrl.Name),
			Remediation: ctrl.Remediation,
			Owner:       ctrl.Owner,
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

	return SelfComplianceResult{
		Verdict:       verdict,
		TotalControls: len(controls),
		Satisfied:     satisfied,
		TotalFindings: len(findings),
		Blocking:      blocking,
		Findings:      findings,
	}, nil
}

func regulatedControls() []Control {
	return []Control{
		{
			ID:          "CTRL-MATRIX",
			Name:        "Control matrix exists",
			Severity:    "critical",
			Blocking:    true,
			Check:       checkControlMatrix,
			Remediation: "Create docs/regulated/control-matrix/ with a YAML control matrix mapping references to controls, evidence, and gates.",
			Owner:       "quality-owner",
		},
		{
			ID:          "CTRL-REFREGISTRY",
			Name:        "Reference registry exists",
			Severity:    "critical",
			Blocking:    true,
			Check:       checkReferenceRegistry,
			Remediation: "Create docs/regulated/reference-basis/external-reference-register.yaml listing all cited external references.",
			Owner:       "regulatory-owner",
		},
		{
			ID:          "CTRL-EVIDENCE",
			Name:        "Evidence ledger exists",
			Severity:    "critical",
			Blocking:    true,
			Check:       checkEvidenceLedger,
			Remediation: "Create docs/regulated/evidence-index/evidence-ledger.yaml cataloging all evidence categories and their status.",
			Owner:       "quality-owner",
		},
		{
			ID:          "CTRL-PROFILE",
			Name:        "Product profile exists",
			Severity:    "high",
			Blocking:    true,
			Check:       checkProductProfile,
			Remediation: "Create docs/regulated/product-profiles/nomos.yaml with product identity, quality level, and owned evidence.",
			Owner:       "product-owner",
		},
		{
			ID:          "CTRL-PROFILE-CLAIM",
			Name:        "Product profile claim boundary declared",
			Severity:    "high",
			Blocking:    true,
			Check:       checkProductProfileClaimBoundary,
			Remediation: "Add public_claim_boundary field to docs/regulated/product-profiles/nomos.yaml.",
			Owner:       "product-owner",
		},
		{
			ID:          "CTRL-LIFECYCLE",
			Name:        "Lifecycle documents exist",
			Severity:    "medium",
			Blocking:    false,
			Check:       checkLifecycleDocs,
			Remediation: "Create docs/regulated/lifecycle/ with SDLC and change management documentation.",
			Owner:       "quality-owner",
		},
		{
			ID:          "CTRL-QMS",
			Name:        "Quality system documents exist",
			Severity:    "medium",
			Blocking:    false,
			Check:       checkQMSDocs,
			Remediation: "Create docs/regulated/quality-system/ with quality policy and procedures.",
			Owner:       "quality-owner",
		},
		{
			ID:          "CTRL-VALIDATION",
			Name:        "Validation pack exists",
			Severity:    "medium",
			Blocking:    false,
			Check:       checkValidationPack,
			Remediation: "Create docs/regulated/validation-pack/ with validation plan and protocols.",
			Owner:       "validation-owner",
		},
		{
			ID:          "CTRL-SECURITY",
			Name:        "Security and privacy documents exist",
			Severity:    "medium",
			Blocking:    false,
			Check:       checkSecurityDocs,
			Remediation: "Create docs/regulated/security-privacy/ with access control and data protection documentation.",
			Owner:       "security-owner",
		},
		{
			ID:          "CTRL-SUPPLIER",
			Name:        "Supplier assurance pack exists",
			Severity:    "low",
			Blocking:    false,
			Check:       checkSupplierPack,
			Remediation: "Create docs/regulated/supplier-pack/ with supplier qualification and AI governance documentation.",
			Owner:       "quality-owner",
		},
		{
			ID:          "CTRL-DECISIONS",
			Name:        "Regulated decisions directory exists",
			Severity:    "medium",
			Blocking:    false,
			Check:       checkDecisionsDocs,
			Remediation: "Create docs/regulated/decisions/ with regulated decision records.",
			Owner:       "quality-owner",
		},
		{
			ID:          "CTRL-PROJECT",
			Name:        "nomos.project.yaml exists",
			Severity:    "high",
			Blocking:    true,
			Check:       checkProjectManifest,
			Remediation: "Create nomos.project.yaml at the repository root.",
			Owner:       "product-owner",
		},
		{
			ID:          "CTRL-GATES-CI",
			Name:        "CI workflow exists",
			Severity:    "high",
			Blocking:    false,
			Check:       checkCIWorkflow,
			Remediation: "Create .github/workflows/ with CI workflow running go test and go vet.",
			Owner:       "devops-owner",
		},
	}
}

func checkControlMatrix(root string) (bool, string) {
	return dirHasContent(root, "docs/regulated/control-matrix")
}

func checkReferenceRegistry(root string) (bool, string) {
	return fileExists(root, "docs/regulated/reference-basis/external-reference-register.yaml")
}

func checkEvidenceLedger(root string) (bool, string) {
	return fileExists(root, "docs/regulated/evidence-index/evidence-ledger.yaml")
}

func checkProductProfile(root string) (bool, string) {
	return fileExists(root, "docs/regulated/product-profiles/nomos.yaml")
}

func checkProductProfileClaimBoundary(root string) (bool, string) {
	path := filepath.Join(root, "docs/regulated/product-profiles/nomos.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return false, "docs/regulated/product-profiles/nomos.yaml"
	}
	var profile map[string]any
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return false, "docs/regulated/product-profiles/nomos.yaml"
	}
	rd, ok := profile["regulated_design"]
	if !ok {
		return false, "docs/regulated/product-profiles/nomos.yaml (missing regulated_design)"
	}
	rdMap, ok := rd.(map[string]any)
	if !ok {
		return false, "docs/regulated/product-profiles/nomos.yaml (regulated_design not a map)"
	}
	claim, ok := rdMap["public_claim_boundary"]
	if !ok || strings.TrimSpace(fmt.Sprint(claim)) == "" {
		return false, "docs/regulated/product-profiles/nomos.yaml (missing public_claim_boundary)"
	}
	return true, ""
}

func checkLifecycleDocs(root string) (bool, string) {
	return dirHasContent(root, "docs/regulated/lifecycle")
}

func checkQMSDocs(root string) (bool, string) {
	return dirHasContent(root, "docs/regulated/quality-system")
}

func checkValidationPack(root string) (bool, string) {
	return dirHasContent(root, "docs/regulated/validation-pack")
}

func checkSecurityDocs(root string) (bool, string) {
	return dirHasContent(root, "docs/regulated/security-privacy")
}

func checkSupplierPack(root string) (bool, string) {
	return dirHasContent(root, "docs/regulated/supplier-pack")
}

func checkDecisionsDocs(root string) (bool, string) {
	return dirHasContent(root, "docs/regulated/decisions")
}

func checkProjectManifest(root string) (bool, string) {
	return fileExists(root, "nomos.project.yaml")
}

func checkCIWorkflow(root string) (bool, string) {
	return dirHasContent(root, ".github/workflows")
}

func fileExists(root string, rel string) (bool, string) {
	path := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false, rel
	}
	return true, ""
}

func dirHasContent(root string, rel string) (bool, string) {
	dirPath := filepath.Join(root, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return false, rel
	}
	for _, e := range entries {
		if !e.IsDir() && e.Name() != ".gitkeep" {
			return true, ""
		}
	}
	return false, rel
}

package compliance

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var evidenceIDPattern = regexp.MustCompile(`^EV-[A-Z0-9][A-Z0-9._-]*$`)
var gapIDPattern = regexp.MustCompile(`^GAP-[A-Z0-9][A-Z0-9._-]*$`)

var validCategories = map[string]bool{
	"external_reference_register": true, "control_matrix": true,
	"quality_system_document": true, "lifecycle_document": true,
	"validation_evidence": true, "test_result": true,
	"data_integrity_record": true, "security_evidence": true,
	"ai_governance_evidence": true, "release_evidence": true,
	"corpus_attestation": true, "source_manifest": true,
	"canonical_matrix": true, "decision_record": true,
	"training_record": true, "audit_record": true,
	"deviation_record": true, "supplier_record": true,
}

var validStatuses = map[string]bool{
	"planned": true, "draft": true, "present": true, "generated": true,
	"requires_evidence": true, "effective": true, "superseded": true, "expired": true,
}

var validGapSeverities = map[string]bool{
	"minor": true, "major": true, "critical": true,
}

// EvidenceContractResult holds the validation result.
type EvidenceContractResult struct {
	Verdict       string    `json:"verdict"        yaml:"verdict"`
	TotalRecords  int       `json:"total_records"  yaml:"total_records"`
	ValidRecords  int       `json:"valid_records"  yaml:"valid_records"`
	TotalFindings int       `json:"total_findings" yaml:"total_findings"`
	Blocking      int       `json:"blocking"       yaml:"blocking"`
	Findings      []Finding `json:"findings"       yaml:"findings"`
}

// ValidateEvidenceLedger checks an evidence-ledger.yaml against the contract.
func ValidateEvidenceLedger(path string) (EvidenceContractResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EvidenceContractResult{}, fmt.Errorf("read ledger: %w", err)
	}
	return ValidateEvidenceLedgerFromBytes(data)
}

// ValidateEvidenceLedgerFromBytes validates evidence ledger YAML bytes.
func ValidateEvidenceLedgerFromBytes(data []byte) (EvidenceContractResult, error) {
	var ledger evidenceLedgerDoc
	if err := yaml.Unmarshal(data, &ledger); err != nil {
		return EvidenceContractResult{}, fmt.Errorf("parse ledger: %w", err)
	}

	var findings []Finding
	idx := 0

	if strings.TrimSpace(ledger.ClaimBoundary) == "" {
		idx++
		findings = append(findings, Finding{
			ID: fmt.Sprintf("EC-%04d", idx), Control: "LEDGER-CLAIM-BOUNDARY",
			Severity: "critical", Blocking: true, Path: "claim_boundary",
			Message: "evidence ledger must declare a claim_boundary",
			Remediation: "Add claim_boundary field stating what evidence proves and what it does not.",
			Owner: "quality-owner",
		})
	}

	totalRecords := len(ledger.Categories)
	validRecords := 0

	for i, cat := range ledger.Categories {
		prefix := fmt.Sprintf("evidence_categories[%d]", i)

		if !evidenceIDPattern.MatchString(cat.ID) {
			idx++
			findings = append(findings, Finding{
				ID: fmt.Sprintf("EC-%04d", idx), Control: "RECORD-ID-FORMAT",
				Severity: "high", Blocking: true, Path: prefix + ".id",
				Message: fmt.Sprintf("id %q must match EV-* pattern", cat.ID),
				Remediation: "Use format EV-{CATEGORY}-{NNN}.", Owner: "quality-owner",
			})
		}

		if !validCategories[cat.Category] && cat.Category != "" {
			idx++
			findings = append(findings, Finding{
				ID: fmt.Sprintf("EC-%04d", idx), Control: "RECORD-CATEGORY",
				Severity: "medium", Blocking: false, Path: prefix + ".category",
				Message: fmt.Sprintf("unknown category %q", cat.Category),
				Remediation: "Use a category from the evidence-contract.cue enum.", Owner: "quality-owner",
			})
		}

		if !validStatuses[cat.CurrentStatus] && cat.CurrentStatus != "" {
			idx++
			findings = append(findings, Finding{
				ID: fmt.Sprintf("EC-%04d", idx), Control: "RECORD-STATUS",
				Severity: "medium", Blocking: false, Path: prefix + ".current_status",
				Message: fmt.Sprintf("unknown status %q", cat.CurrentStatus),
				Remediation: "Use a status from the evidence-contract.cue enum.", Owner: "quality-owner",
			})
		}

		if strings.TrimSpace(cat.ExpectedLocation) == "" {
			idx++
			findings = append(findings, Finding{
				ID: fmt.Sprintf("EC-%04d", idx), Control: "RECORD-LOCATION",
				Severity: "high", Blocking: true, Path: prefix + ".expected_location",
				Message: "expected_location is required",
				Remediation: "Specify the file or directory where this evidence should exist.", Owner: "quality-owner",
			})
		}

		if strings.TrimSpace(cat.ClaimAllowed) == "" {
			idx++
			findings = append(findings, Finding{
				ID: fmt.Sprintf("EC-%04d", idx), Control: "RECORD-CLAIM",
				Severity: "high", Blocking: true, Path: prefix + ".claim_allowed",
				Message: "claim_allowed is required",
				Remediation: "State what claim this evidence supports, or 'none' if not yet claimable.", Owner: "quality-owner",
			})
		}

		if len(findings) == 0 || findings[len(findings)-1].Path != prefix+".id" {
			validRecords++
		}
	}

	for i, gap := range ledger.BlockingGaps {
		prefix := fmt.Sprintf("blocking_gaps[%d]", i)

		if !gapIDPattern.MatchString(gap.ID) {
			idx++
			findings = append(findings, Finding{
				ID: fmt.Sprintf("EC-%04d", idx), Control: "GAP-ID-FORMAT",
				Severity: "medium", Blocking: false, Path: prefix + ".id",
				Message: fmt.Sprintf("gap id %q must match GAP-* pattern", gap.ID),
				Remediation: "Use format GAP-{AREA}-{NNN}.", Owner: "quality-owner",
			})
		}

		if !validGapSeverities[gap.Severity] && gap.Severity != "" {
			idx++
			findings = append(findings, Finding{
				ID: fmt.Sprintf("EC-%04d", idx), Control: "GAP-SEVERITY",
				Severity: "medium", Blocking: false, Path: prefix + ".severity",
				Message: fmt.Sprintf("unknown gap severity %q", gap.Severity),
				Remediation: "Use minor, major, or critical.", Owner: "quality-owner",
			})
		}
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

	return EvidenceContractResult{
		Verdict:       verdict,
		TotalRecords:  totalRecords,
		ValidRecords:  validRecords,
		TotalFindings: len(findings),
		Blocking:      blocking,
		Findings:      findings,
	}, nil
}

// CheckEvidenceContractPresence verifies the evidence contract artifacts exist.
func CheckEvidenceContractPresence(root string) (EvidenceContractResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return EvidenceContractResult{}, err
	}

	var findings []Finding
	idx := 0
	checks := []struct {
		rel      string
		control  string
		severity string
		blocking bool
	}{
		{"specs/evidence-contract.cue", "CONTRACT-SCHEMA", "critical", true},
		{"docs/regulated/evidence-index/evidence-contract.md", "CONTRACT-DOCS", "high", true},
		{"docs/regulated/evidence-index/evidence-ledger.yaml", "CONTRACT-LEDGER", "critical", true},
	}

	present := 0
	for _, c := range checks {
		path := filepath.Join(absRoot, filepath.FromSlash(c.rel))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			present++
			continue
		}
		idx++
		findings = append(findings, Finding{
			ID: fmt.Sprintf("EC-%04d", idx), Control: c.control,
			Severity: c.severity, Blocking: c.blocking, Path: c.rel,
			Message:     fmt.Sprintf("evidence contract artifact missing: %s", c.rel),
			Remediation: fmt.Sprintf("Create %s per the evidence contract specification.", c.rel),
			Owner:       "quality-owner",
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

	return EvidenceContractResult{
		Verdict:       verdict,
		TotalRecords:  len(checks),
		ValidRecords:  present,
		TotalFindings: len(findings),
		Blocking:      blocking,
		Findings:      findings,
	}, nil
}

type evidenceLedgerDoc struct {
	SchemaVersion string                `yaml:"schema_version"`
	Status        string                `yaml:"status"`
	ClaimBoundary string                `yaml:"claim_boundary"`
	Categories    []evidenceCategoryDoc `yaml:"evidence_categories"`
	BlockingGaps  []evidenceGapDoc      `yaml:"blocking_gaps"`
}

type evidenceCategoryDoc struct {
	ID               string `yaml:"id"`
	Category         string `yaml:"category"`
	ExpectedLocation string `yaml:"expected_location"`
	CurrentStatus    string `yaml:"current_status"`
	ClaimAllowed     string `yaml:"claim_allowed"`
}

type evidenceGapDoc struct {
	ID          string   `yaml:"id"`
	Description string   `yaml:"description"`
	Severity    string   `yaml:"severity"`
	Status      string   `yaml:"status"`
	BlocksClaims []string `yaml:"blocks_claims"`
}

package compliance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReviewGateResult holds the outcome of the review gate evaluation.
type ReviewGateResult struct {
	Verdict       string    `json:"verdict"        yaml:"verdict"`
	TotalRecords  int       `json:"total_records"  yaml:"total_records"`
	Compliant     int       `json:"compliant"      yaml:"compliant"`
	TotalFindings int       `json:"total_findings" yaml:"total_findings"`
	Blocking      int       `json:"blocking"       yaml:"blocking"`
	Findings      []Finding `json:"findings"       yaml:"findings"`
}

// ReviewGatePolicy defines what the gate checks for each record type.
type ReviewGatePolicy struct {
	// CriticalRecordTypes require independent reviewer + quality_unit.
	CriticalRecordTypes []string
	// StandardRecordTypes require author + reviewer.
	StandardRecordTypes []string
}

// DefaultReviewGatePolicy returns the default gate policy.
func DefaultReviewGatePolicy() ReviewGatePolicy {
	return ReviewGatePolicy{
		CriticalRecordTypes: []string{
			"control_matrix",
			"validation_evidence",
			"release_evidence",
			"deviation_record",
			"regulated_evidence",
		},
		StandardRecordTypes: []string{
			"quality_system_document",
			"lifecycle_document",
			"security_evidence",
			"standard_evidence",
		},
	}
}

// EvaluateReviewGates checks approval records against review policies.
func EvaluateReviewGates(records []ApprovalRecord, policy ReviewGatePolicy) ReviewGateResult {
	var findings []Finding
	compliant := 0
	idx := 0

	criticalSet := toSet(policy.CriticalRecordTypes)
	standardSet := toSet(policy.StandardRecordTypes)

	for _, rec := range records {
		var recFindings []Finding

		if criticalSet[rec.RecordType] {
			recFindings = checkCriticalRecord(rec, &idx)
		} else if standardSet[rec.RecordType] {
			recFindings = checkStandardRecord(rec, &idx)
		} else {
			recFindings = checkMinimalRecord(rec, &idx)
		}

		if len(recFindings) == 0 {
			compliant++
		}
		findings = append(findings, recFindings...)
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

	return ReviewGateResult{
		Verdict:       verdict,
		TotalRecords:  len(records),
		Compliant:     compliant,
		TotalFindings: len(findings),
		Blocking:      blocking,
		Findings:      findings,
	}
}

// EvaluateReviewGatesFromDir scans a directory for approval YAML files.
func EvaluateReviewGatesFromDir(dir string, policy ReviewGatePolicy) (ReviewGateResult, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return ReviewGateResult{}, fmt.Errorf("resolve dir: %w", err)
	}

	var records []ApprovalRecord
	err = filepath.WalkDir(absDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !d.Type().IsRegular() {
			return nil
		}
		lower := strings.ToLower(d.Name())
		if !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
			return nil
		}
		if !strings.Contains(lower, "approval") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}
		var rec ApprovalRecord
		if err := yaml.Unmarshal(data, &rec); err != nil {
			return nil // skip invalid files
		}
		if rec.RecordID != "" {
			records = append(records, rec)
		}
		return nil
	})
	if err != nil {
		return ReviewGateResult{}, fmt.Errorf("scan dir: %w", err)
	}

	return EvaluateReviewGates(records, policy), nil
}

func checkCriticalRecord(rec ApprovalRecord, idx *int) []Finding {
	var findings []Finding

	// Must have independent reviewer (not the author).
	if !hasIndependentRole(rec, RoleReviewer) {
		*idx++
		findings = append(findings, Finding{
			ID: fmt.Sprintf("RG-%04d", *idx), Control: "INDEPENDENT-REVIEW",
			Severity: "critical", Blocking: true,
			Path:    rec.RecordPath,
			Message: fmt.Sprintf("critical record %s (%s) missing independent reviewer", rec.RecordID, rec.RecordType),
			Remediation: "Add an approved signature from a reviewer who is not the author.",
			Owner: "quality-owner",
		})
	}

	// Must have quality_unit sign-off.
	if !hasApprovedRole(rec, RoleQualityUnit) {
		*idx++
		findings = append(findings, Finding{
			ID: fmt.Sprintf("RG-%04d", *idx), Control: "QUALITY-UNIT-SIGNOFF",
			Severity: "critical", Blocking: true,
			Path:    rec.RecordPath,
			Message: fmt.Sprintf("critical record %s (%s) missing quality unit sign-off", rec.RecordID, rec.RecordType),
			Remediation: "Add an approved signature from the quality_unit role.",
			Owner: "quality-owner",
		})
	}

	// Must have author.
	if !hasApprovedRole(rec, RoleAuthor) {
		*idx++
		findings = append(findings, Finding{
			ID: fmt.Sprintf("RG-%04d", *idx), Control: "AUTHOR-ATTRIBUTION",
			Severity: "high", Blocking: true,
			Path:    rec.RecordPath,
			Message: fmt.Sprintf("record %s missing author attribution", rec.RecordID),
			Remediation: "Add a signature from the author role.",
			Owner: "quality-owner",
		})
	}

	// Must not have self-approval (author == approver/reviewer).
	if hasSelfApproval(rec) {
		*idx++
		findings = append(findings, Finding{
			ID: fmt.Sprintf("RG-%04d", *idx), Control: "NO-SELF-APPROVAL",
			Severity: "critical", Blocking: true,
			Path:    rec.RecordPath,
			Message: fmt.Sprintf("record %s has self-approval: author is also reviewer or approver", rec.RecordID),
			Remediation: "Ensure reviewer and approver are different people from the author.",
			Owner: "quality-owner",
		})
	}

	// Status must be approved for gate to pass.
	if rec.Status != ApprovalApproved {
		*idx++
		findings = append(findings, Finding{
			ID: fmt.Sprintf("RG-%04d", *idx), Control: "APPROVAL-STATUS",
			Severity: "high", Blocking: true,
			Path:    rec.RecordPath,
			Message: fmt.Sprintf("critical record %s has status %q, expected approved", rec.RecordID, rec.Status),
			Remediation: "Complete the approval workflow to move the record to approved status.",
			Owner: "quality-owner",
		})
	}

	return findings
}

func checkStandardRecord(rec ApprovalRecord, idx *int) []Finding {
	var findings []Finding

	// Must have author.
	if !hasApprovedRole(rec, RoleAuthor) {
		*idx++
		findings = append(findings, Finding{
			ID: fmt.Sprintf("RG-%04d", *idx), Control: "AUTHOR-ATTRIBUTION",
			Severity: "medium", Blocking: false,
			Path:    rec.RecordPath,
			Message: fmt.Sprintf("record %s missing author attribution", rec.RecordID),
			Remediation: "Add a signature from the author role.",
			Owner: "quality-owner",
		})
	}

	// Must have reviewer.
	if !hasApprovedRole(rec, RoleReviewer) {
		*idx++
		findings = append(findings, Finding{
			ID: fmt.Sprintf("RG-%04d", *idx), Control: "REVIEWER-PRESENT",
			Severity: "medium", Blocking: false,
			Path:    rec.RecordPath,
			Message: fmt.Sprintf("standard record %s missing reviewer", rec.RecordID),
			Remediation: "Add an approved signature from a reviewer.",
			Owner: "quality-owner",
		})
	}

	return findings
}

func checkMinimalRecord(rec ApprovalRecord, idx *int) []Finding {
	var findings []Finding

	if !hasApprovedRole(rec, RoleAuthor) {
		*idx++
		findings = append(findings, Finding{
			ID: fmt.Sprintf("RG-%04d", *idx), Control: "AUTHOR-ATTRIBUTION",
			Severity: "low", Blocking: false,
			Path:    rec.RecordPath,
			Message: fmt.Sprintf("record %s missing author attribution", rec.RecordID),
			Remediation: "Add a signature from the author role.",
			Owner: "quality-owner",
		})
	}

	return findings
}

func hasApprovedRole(rec ApprovalRecord, role ApprovalRole) bool {
	for _, sig := range rec.Signatures {
		if sig.Role == role && sig.Decision == DecisionApproved {
			return true
		}
	}
	return false
}

func hasIndependentRole(rec ApprovalRecord, role ApprovalRole) bool {
	authors := map[string]bool{}
	for _, sig := range rec.Signatures {
		if sig.Role == RoleAuthor {
			authors[sig.SignerID] = true
		}
	}
	for _, sig := range rec.Signatures {
		if sig.Role == role && sig.Decision == DecisionApproved && !authors[sig.SignerID] {
			return true
		}
	}
	return false
}

func hasSelfApproval(rec ApprovalRecord) bool {
	authors := map[string]bool{}
	for _, sig := range rec.Signatures {
		if sig.Role == RoleAuthor {
			authors[sig.SignerID] = true
		}
	}
	for _, sig := range rec.Signatures {
		if sig.Decision != DecisionApproved {
			continue
		}
		if (sig.Role == RoleReviewer || sig.Role == RoleApprover) && authors[sig.SignerID] {
			return true
		}
	}
	return false
}

func toSet(items []string) map[string]bool {
	m := make(map[string]bool, len(items))
	for _, item := range items {
		m[item] = true
	}
	return m
}

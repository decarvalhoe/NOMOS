package compliance

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ALCOAPlusAttribute enumerates the 8 ALCOA+ data integrity attributes.
type ALCOAPlusAttribute string

const (
	Attributable    ALCOAPlusAttribute = "attributable"
	Legible         ALCOAPlusAttribute = "legible"
	Contemporaneous ALCOAPlusAttribute = "contemporaneous"
	Original        ALCOAPlusAttribute = "original"
	Accurate        ALCOAPlusAttribute = "accurate"
	Complete        ALCOAPlusAttribute = "complete"
	Consistent      ALCOAPlusAttribute = "consistent"
	Enduring        ALCOAPlusAttribute = "enduring"
	Available       ALCOAPlusAttribute = "available"
)

// AllAttributes returns all 8 ALCOA+ attributes in canonical order.
func AllAttributes() []ALCOAPlusAttribute {
	return []ALCOAPlusAttribute{
		Attributable, Legible, Contemporaneous, Original, Accurate,
		Complete, Consistent, Enduring, Available,
	}
}

// IsValid returns true if the attribute is recognized.
func (a ALCOAPlusAttribute) IsValid() bool {
	for _, attr := range AllAttributes() {
		if a == attr {
			return true
		}
	}
	return false
}

// ALCOACompliance indicates how well an attribute is satisfied.
type ALCOACompliance string

const (
	Satisfied     ALCOACompliance = "satisfied"
	Partial       ALCOACompliance = "partial"
	NotSatisfied  ALCOACompliance = "not_satisfied"
	NotApplicable ALCOACompliance = "not_applicable"
)

// IsValid returns true if the compliance level is recognized.
func (c ALCOACompliance) IsValid() bool {
	switch c {
	case Satisfied, Partial, NotSatisfied, NotApplicable:
		return true
	default:
		return false
	}
}

// ReviewStatus indicates the review lifecycle of an evidence record.
type ReviewStatus string

const (
	ReviewPending  ReviewStatus = "pending"
	ReviewReviewed ReviewStatus = "reviewed"
	ReviewApproved ReviewStatus = "approved"
	ReviewRejected ReviewStatus = "rejected"
)

// IsValid returns true if the review status is recognized.
func (r ReviewStatus) IsValid() bool {
	switch r {
	case ReviewPending, ReviewReviewed, ReviewApproved, ReviewRejected:
		return true
	default:
		return false
	}
}

// AttributeAssessment is the assessment of a single ALCOA+ attribute.
type AttributeAssessment struct {
	Attribute  ALCOAPlusAttribute `json:"attribute"`
	Compliance ALCOACompliance    `json:"compliance"`
	Evidence   string             `json:"evidence"`
	Assessor   string             `json:"assessor,omitempty"`
	AssessedAt string             `json:"assessed_at,omitempty"`
	Notes      string             `json:"notes,omitempty"`
}

// EvidenceRecord is an evidence record annotated with ALCOA+ data integrity.
type EvidenceRecord struct {
	RecordID          string                `json:"record_id"`
	EvidenceRef       string                `json:"evidence_ref"`
	Owner             string                `json:"owner"`
	CreatedAt         string                `json:"created_at"`
	SourceHash        string                `json:"source_hash"`
	Domain            string                `json:"domain"`
	Assessments       []AttributeAssessment `json:"assessments"`
	OverallCompliance ALCOACompliance       `json:"overall_compliance"`
	ReviewStatus      ReviewStatus          `json:"review_status"`
	Reviewer          string                `json:"reviewer,omitempty"`
	ReviewedAt        string                `json:"reviewed_at,omitempty"`
	Metadata          map[string]any        `json:"metadata,omitempty"`
}

// ALCOAReport is a batch of assessed evidence records.
type ALCOAReport struct {
	SchemaVersion string           `json:"schema_version"`
	ReportID      string           `json:"report_id"`
	GeneratedAt   string           `json:"generated_at"`
	Domain        string           `json:"domain"`
	RecordCount   int              `json:"record_count"`
	Records       []EvidenceRecord `json:"records"`
	Summary       ALCOASummary     `json:"summary"`
}

// ALCOASummary provides aggregate compliance counts.
type ALCOASummary struct {
	SatisfiedCount     int     `json:"satisfied_count"`
	PartialCount       int     `json:"partial_count"`
	NotSatisfiedCount  int     `json:"not_satisfied_count"`
	NotApplicableCount int     `json:"not_applicable_count"`
	ComplianceRatio    float64 `json:"compliance_ratio"`
}

var (
	recordIDPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._-]*$`)
	reportIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	hashPattern     = regexp.MustCompile(`^(sha256|sha384|sha512):[A-Fa-f0-9]+$`)
)

// NewEvidenceRecord creates an evidence record in pending review state
// with empty assessments for all 8 ALCOA+ attributes.
func NewEvidenceRecord(recordID, evidenceRef, owner, sourceHash, domain string, now time.Time) EvidenceRecord {
	assessments := make([]AttributeAssessment, 0, len(AllAttributes()))
	for _, attr := range AllAttributes() {
		assessments = append(assessments, AttributeAssessment{
			Attribute:  attr,
			Compliance: NotSatisfied,
			Evidence:   "",
		})
	}
	return EvidenceRecord{
		RecordID:          recordID,
		EvidenceRef:       evidenceRef,
		Owner:             owner,
		CreatedAt:         now.Format(time.RFC3339),
		SourceHash:        sourceHash,
		Domain:            domain,
		Assessments:       assessments,
		OverallCompliance: NotSatisfied,
		ReviewStatus:      ReviewPending,
	}
}

// ComputeOverallCompliance derives the overall compliance from individual assessments.
func ComputeOverallCompliance(assessments []AttributeAssessment) ALCOACompliance {
	if len(assessments) == 0 {
		return NotSatisfied
	}

	hasNotSatisfied := false
	hasPartial := false
	allSatisfiedOrNA := true

	for _, a := range assessments {
		switch a.Compliance {
		case NotSatisfied:
			hasNotSatisfied = true
			allSatisfiedOrNA = false
		case Partial:
			hasPartial = true
			allSatisfiedOrNA = false
		case Satisfied, NotApplicable:
			// ok
		}
	}

	switch {
	case allSatisfiedOrNA:
		return Satisfied
	case hasNotSatisfied:
		return NotSatisfied
	case hasPartial:
		return Partial
	default:
		return NotSatisfied
	}
}

// GenerateReport builds an ALCOAReport from a set of evidence records.
func GenerateReport(reportID, domain string, records []EvidenceRecord, now time.Time) ALCOAReport {
	summary := computeSummary(records)
	return ALCOAReport{
		SchemaVersion: "0.1.0",
		ReportID:      reportID,
		GeneratedAt:   now.Format(time.RFC3339),
		Domain:        domain,
		RecordCount:   len(records),
		Records:       records,
		Summary:       summary,
	}
}

func computeSummary(records []EvidenceRecord) ALCOASummary {
	var s ALCOASummary
	for _, r := range records {
		switch r.OverallCompliance {
		case Satisfied:
			s.SatisfiedCount++
		case Partial:
			s.PartialCount++
		case NotSatisfied:
			s.NotSatisfiedCount++
		case NotApplicable:
			s.NotApplicableCount++
		}
	}
	total := len(records)
	if total > 0 {
		s.ComplianceRatio = float64(s.SatisfiedCount) / float64(total)
	}
	return s
}

// ValidateRecord checks an EvidenceRecord for schema conformance.
func ValidateRecord(r EvidenceRecord) []string {
	var errs []string

	if !recordIDPattern.MatchString(r.RecordID) {
		errs = append(errs, fmt.Sprintf("record_id %q must match %s", r.RecordID, recordIDPattern.String()))
	}
	if strings.TrimSpace(r.EvidenceRef) == "" {
		errs = append(errs, "evidence_ref is required")
	}
	if strings.TrimSpace(r.Owner) == "" {
		errs = append(errs, "owner is required")
	}
	if strings.TrimSpace(r.CreatedAt) == "" {
		errs = append(errs, "created_at is required")
	}
	if !hashPattern.MatchString(r.SourceHash) {
		errs = append(errs, fmt.Sprintf("source_hash %q must match %s", r.SourceHash, hashPattern.String()))
	}
	if strings.TrimSpace(r.Domain) == "" {
		errs = append(errs, "domain is required")
	}
	if len(r.Assessments) == 0 {
		errs = append(errs, "at least one assessment is required")
	}
	for i, a := range r.Assessments {
		if !a.Attribute.IsValid() {
			errs = append(errs, fmt.Sprintf("assessments[%d].attribute %q is not valid", i, a.Attribute))
		}
		if !a.Compliance.IsValid() {
			errs = append(errs, fmt.Sprintf("assessments[%d].compliance %q is not valid", i, a.Compliance))
		}
	}
	if !r.OverallCompliance.IsValid() {
		errs = append(errs, fmt.Sprintf("overall_compliance %q is not valid", r.OverallCompliance))
	}
	if !r.ReviewStatus.IsValid() {
		errs = append(errs, fmt.Sprintf("review_status %q is not valid", r.ReviewStatus))
	}

	return errs
}

// ValidateReport checks an ALCOAReport for schema conformance.
func ValidateReport(r ALCOAReport) []string {
	var errs []string

	if !reportIDPattern.MatchString(r.ReportID) {
		errs = append(errs, fmt.Sprintf("report_id %q must match %s", r.ReportID, reportIDPattern.String()))
	}
	if strings.TrimSpace(r.Domain) == "" {
		errs = append(errs, "domain is required")
	}
	if strings.TrimSpace(r.GeneratedAt) == "" {
		errs = append(errs, "generated_at is required")
	}
	if r.RecordCount != len(r.Records) {
		errs = append(errs, fmt.Sprintf("record_count %d != len(records) %d", r.RecordCount, len(r.Records)))
	}
	for i, rec := range r.Records {
		recErrs := ValidateRecord(rec)
		for _, e := range recErrs {
			errs = append(errs, fmt.Sprintf("records[%d]: %s", i, e))
		}
	}

	return errs
}

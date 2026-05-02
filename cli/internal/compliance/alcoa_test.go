package compliance

import (
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)

func TestAllAttributesReturns8(t *testing.T) {
	attrs := AllAttributes()
	if len(attrs) != 9 {
		t.Fatalf("expected 9 ALCOA+ attributes, got %d", len(attrs))
	}
	// First 5 are ALCOA, last 4 are Plus
	if attrs[0] != Attributable {
		t.Fatalf("first attribute should be attributable, got %s", attrs[0])
	}
	if attrs[8] != Available {
		t.Fatalf("last attribute should be available, got %s", attrs[8])
	}
}

func TestAttributeIsValid(t *testing.T) {
	for _, attr := range AllAttributes() {
		if !attr.IsValid() {
			t.Fatalf("expected %s to be valid", attr)
		}
	}
	if ALCOAPlusAttribute("bogus").IsValid() {
		t.Fatal("expected bogus to be invalid")
	}
}

func TestComplianceIsValid(t *testing.T) {
	valid := []ALCOACompliance{Satisfied, Partial, NotSatisfied, NotApplicable}
	for _, c := range valid {
		if !c.IsValid() {
			t.Fatalf("expected %s to be valid", c)
		}
	}
	if ALCOACompliance("unknown").IsValid() {
		t.Fatal("expected unknown to be invalid")
	}
}

func TestNewEvidenceRecordStartsPending(t *testing.T) {
	r := NewEvidenceRecord("REC-001", "evidence/report.json", "Alice", "sha256:aabb", "insurance", testNow)

	if r.RecordID != "REC-001" {
		t.Fatalf("expected REC-001, got %s", r.RecordID)
	}
	if r.ReviewStatus != ReviewPending {
		t.Fatalf("expected pending, got %s", r.ReviewStatus)
	}
	if r.OverallCompliance != NotSatisfied {
		t.Fatalf("expected not_satisfied initially, got %s", r.OverallCompliance)
	}
	if len(r.Assessments) != 9 {
		t.Fatalf("expected 9 assessments (one per attribute), got %d", len(r.Assessments))
	}
	for _, a := range r.Assessments {
		if a.Compliance != NotSatisfied {
			t.Fatalf("expected initial assessment not_satisfied, got %s", a.Compliance)
		}
	}
}

func TestComputeOverallAllSatisfied(t *testing.T) {
	var assessments []AttributeAssessment
	for _, attr := range AllAttributes() {
		assessments = append(assessments, AttributeAssessment{
			Attribute: attr, Compliance: Satisfied, Evidence: "proof",
		})
	}
	result := ComputeOverallCompliance(assessments)
	if result != Satisfied {
		t.Fatalf("expected satisfied, got %s", result)
	}
}

func TestComputeOverallWithNA(t *testing.T) {
	assessments := []AttributeAssessment{
		{Attribute: Attributable, Compliance: Satisfied, Evidence: "ok"},
		{Attribute: Legible, Compliance: NotApplicable, Evidence: "n/a"},
		{Attribute: Contemporaneous, Compliance: Satisfied, Evidence: "ok"},
	}
	result := ComputeOverallCompliance(assessments)
	if result != Satisfied {
		t.Fatalf("expected satisfied (NA counts as ok), got %s", result)
	}
}

func TestComputeOverallPartial(t *testing.T) {
	assessments := []AttributeAssessment{
		{Attribute: Attributable, Compliance: Satisfied, Evidence: "ok"},
		{Attribute: Legible, Compliance: Partial, Evidence: "partial"},
		{Attribute: Contemporaneous, Compliance: Satisfied, Evidence: "ok"},
	}
	result := ComputeOverallCompliance(assessments)
	if result != Partial {
		t.Fatalf("expected partial, got %s", result)
	}
}

func TestComputeOverallNotSatisfied(t *testing.T) {
	assessments := []AttributeAssessment{
		{Attribute: Attributable, Compliance: Satisfied, Evidence: "ok"},
		{Attribute: Legible, Compliance: NotSatisfied, Evidence: "missing"},
	}
	result := ComputeOverallCompliance(assessments)
	if result != NotSatisfied {
		t.Fatalf("expected not_satisfied, got %s", result)
	}
}

func TestComputeOverallEmpty(t *testing.T) {
	result := ComputeOverallCompliance(nil)
	if result != NotSatisfied {
		t.Fatalf("expected not_satisfied for empty, got %s", result)
	}
}

func TestGenerateReport(t *testing.T) {
	r1 := NewEvidenceRecord("REC-001", "ref1", "Alice", "sha256:aa", "insurance", testNow)
	r1.OverallCompliance = Satisfied

	r2 := NewEvidenceRecord("REC-002", "ref2", "Bob", "sha256:bb", "insurance", testNow)
	r2.OverallCompliance = Partial

	report := GenerateReport("alcoa-report-001", "insurance", []EvidenceRecord{r1, r2}, testNow)

	if report.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", report.SchemaVersion)
	}
	if report.RecordCount != 2 {
		t.Fatalf("expected 2 records, got %d", report.RecordCount)
	}
	if report.Summary.SatisfiedCount != 1 {
		t.Fatalf("expected 1 satisfied, got %d", report.Summary.SatisfiedCount)
	}
	if report.Summary.PartialCount != 1 {
		t.Fatalf("expected 1 partial, got %d", report.Summary.PartialCount)
	}
	if report.Summary.ComplianceRatio != 0.5 {
		t.Fatalf("expected ratio 0.5, got %f", report.Summary.ComplianceRatio)
	}
}

func TestValidateRecordValid(t *testing.T) {
	r := NewEvidenceRecord("REC-001", "evidence/file.json", "Alice", "sha256:aabbccdd", "insurance", testNow)
	// Fill evidence text for each assessment
	for i := range r.Assessments {
		r.Assessments[i].Evidence = "proof exists"
	}
	errs := ValidateRecord(r)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateRecordInvalidID(t *testing.T) {
	r := NewEvidenceRecord("bad-id", "ref", "owner", "sha256:aa", "domain", testNow)
	errs := ValidateRecord(r)
	assertContains(t, errs, "record_id")
}

func TestValidateRecordMissingOwner(t *testing.T) {
	r := NewEvidenceRecord("REC-001", "ref", "", "sha256:aa", "domain", testNow)
	errs := ValidateRecord(r)
	assertContains(t, errs, "owner")
}

func TestValidateRecordInvalidHash(t *testing.T) {
	r := NewEvidenceRecord("REC-001", "ref", "owner", "md5:bad", "domain", testNow)
	errs := ValidateRecord(r)
	assertContains(t, errs, "source_hash")
}

func TestValidateRecordInvalidAttribute(t *testing.T) {
	r := NewEvidenceRecord("REC-001", "ref", "owner", "sha256:aa", "domain", testNow)
	r.Assessments[0].Attribute = "invalid"
	errs := ValidateRecord(r)
	assertContains(t, errs, "attribute")
}

func TestValidateReportValid(t *testing.T) {
	r := NewEvidenceRecord("REC-001", "ref", "owner", "sha256:aa", "domain", testNow)
	for i := range r.Assessments {
		r.Assessments[i].Evidence = "proof"
	}
	report := GenerateReport("alcoa-test", "domain", []EvidenceRecord{r}, testNow)
	errs := ValidateReport(report)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateReportCountMismatch(t *testing.T) {
	report := ALCOAReport{
		ReportID:    "alcoa-test",
		Domain:      "domain",
		GeneratedAt: testNow.Format(time.RFC3339),
		RecordCount: 5,
		Records:     nil,
	}
	errs := ValidateReport(report)
	assertContains(t, errs, "record_count")
}

func TestValidateReportPropagatesRecordErrors(t *testing.T) {
	r := NewEvidenceRecord("bad-id", "ref", "owner", "sha256:aa", "domain", testNow)
	report := GenerateReport("alcoa-test", "domain", []EvidenceRecord{r}, testNow)
	errs := ValidateReport(report)
	assertContains(t, errs, "records[0]")
}

func TestReviewStatusValid(t *testing.T) {
	valid := []ReviewStatus{ReviewPending, ReviewReviewed, ReviewApproved, ReviewRejected}
	for _, s := range valid {
		if !s.IsValid() {
			t.Fatalf("expected %s to be valid", s)
		}
	}
	if ReviewStatus("unknown").IsValid() {
		t.Fatal("expected unknown to be invalid")
	}
}

func assertContains(t *testing.T, errs []string, keyword string) {
	t.Helper()
	for _, e := range errs {
		if strings.Contains(e, keyword) {
			return
		}
	}
	t.Fatalf("expected error containing %q in %v", keyword, errs)
}

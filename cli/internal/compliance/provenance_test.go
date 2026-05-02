package compliance

import (
	"strings"
	"testing"
	"time"
)

var provNow = time.Date(2026, 5, 2, 14, 0, 0, 0, time.UTC)

func validRecord() ProvenanceRecord {
	return ProvenanceRecord{
		RecordID:  "PROV-001",
		BuilderID: "https://github.com/actions/runner",
		BuildType: "github-actions",
		SLSALevel: SLSA2,
		Invocation: ProvenanceInvocation{
			ConfigSource: ConfigSource{
				URI:        "https://github.com/org/repo",
				Digest:     "sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				EntryPoint: ".github/workflows/release.yml",
			},
		},
		Subjects: []ProvenanceSubject{
			{Name: "nomos-cli", Digest: map[string]string{"sha256": "deadbeef"}},
		},
		Metadata: ProvenanceMetadata{
			BuildStartedOn:  "2026-05-02T10:00:00Z",
			BuildFinishedOn: "2026-05-02T10:05:00Z",
			Completeness:    Completeness{Parameters: true},
		},
		Verified:   true,
		VerifiedAt: "2026-05-02T10:06:00Z",
		Verifier:   "cosign",
	}
}

func TestSLSALevelRank(t *testing.T) {
	cases := []struct {
		level SLSALevel
		rank  int
	}{
		{SLSA1, 1}, {SLSA2, 2}, {SLSA3, 3}, {SLSA4, 4},
	}
	for _, tc := range cases {
		if tc.level.Rank() != tc.rank {
			t.Errorf("%s.Rank() = %d, want %d", tc.level, tc.level.Rank(), tc.rank)
		}
	}
}

func TestSLSALevelMeetsMinimum(t *testing.T) {
	if !SLSA2.MeetsMinimum(SLSA1) {
		t.Error("SLSA2 should meet SLSA1 minimum")
	}
	if !SLSA2.MeetsMinimum(SLSA2) {
		t.Error("SLSA2 should meet SLSA2 minimum")
	}
	if SLSA1.MeetsMinimum(SLSA2) {
		t.Error("SLSA1 should not meet SLSA2 minimum")
	}
}

func TestCheckProvenancePassed(t *testing.T) {
	result := CheckProvenance([]ProvenanceRecord{validRecord()}, ProvenanceGateOptions{
		MinLevel: SLSA1,
		Now:      provNow,
	})
	if result.Status != GatePassed {
		t.Fatalf("expected passed, got %s", result.Status)
	}
	if !result.Summary.MinLevelMet {
		t.Fatal("expected min level met")
	}
	if result.Summary.VerifiedCount != 1 {
		t.Fatalf("expected 1 verified, got %d", result.Summary.VerifiedCount)
	}
}

func TestCheckProvenanceFailsUnverified(t *testing.T) {
	r := validRecord()
	r.Verified = false
	result := CheckProvenance([]ProvenanceRecord{r}, ProvenanceGateOptions{
		MinLevel: SLSA1,
		Now:      provNow,
	})
	if result.Status != GateFailed {
		t.Fatalf("expected failed for unverified, got %s", result.Status)
	}
	if result.Summary.UnverifiedCount != 1 {
		t.Fatalf("expected 1 unverified, got %d", result.Summary.UnverifiedCount)
	}
	hasFinding := false
	for _, f := range result.Findings {
		if f.Code == "PROVENANCE_UNVERIFIED" {
			hasFinding = true
		}
	}
	if !hasFinding {
		t.Fatal("expected PROVENANCE_UNVERIFIED finding")
	}
}

func TestCheckProvenanceFailsInsufficientLevel(t *testing.T) {
	r := validRecord()
	r.SLSALevel = SLSA1
	result := CheckProvenance([]ProvenanceRecord{r}, ProvenanceGateOptions{
		MinLevel: SLSA3,
		Now:      provNow,
	})
	if result.Status != GateFailed {
		t.Fatalf("expected failed for insufficient level, got %s", result.Status)
	}
	hasFinding := false
	for _, f := range result.Findings {
		if f.Code == "PROVENANCE_LEVEL_INSUFFICIENT" {
			hasFinding = true
		}
	}
	if !hasFinding {
		t.Fatal("expected PROVENANCE_LEVEL_INSUFFICIENT finding")
	}
}

func TestCheckProvenanceNoRecords(t *testing.T) {
	result := CheckProvenance(nil, ProvenanceGateOptions{
		MinLevel: SLSA1,
		Now:      provNow,
	})
	if result.Status != GateFailed {
		t.Fatalf("expected failed for no records, got %s", result.Status)
	}
	if len(result.Findings) != 1 || result.Findings[0].Code != "PROVENANCE_MISSING" {
		t.Fatalf("expected PROVENANCE_MISSING finding, got %v", result.Findings)
	}
}

func TestCheckProvenanceMissingBuilder(t *testing.T) {
	r := validRecord()
	r.BuilderID = ""
	result := CheckProvenance([]ProvenanceRecord{r}, ProvenanceGateOptions{
		MinLevel: SLSA1,
		Now:      provNow,
	})
	hasFinding := false
	for _, f := range result.Findings {
		if f.Code == "PROVENANCE_MISSING_BUILDER" {
			hasFinding = true
		}
	}
	if !hasFinding {
		t.Fatal("expected PROVENANCE_MISSING_BUILDER finding")
	}
}

func TestCheckProvenanceMissingInvocation(t *testing.T) {
	r := validRecord()
	r.Invocation.ConfigSource.URI = ""
	result := CheckProvenance([]ProvenanceRecord{r}, ProvenanceGateOptions{
		MinLevel: SLSA1,
		Now:      provNow,
	})
	hasFinding := false
	for _, f := range result.Findings {
		if f.Code == "PROVENANCE_MISSING_INVOCATION" {
			hasFinding = true
		}
	}
	if !hasFinding {
		t.Fatal("expected PROVENANCE_MISSING_INVOCATION finding")
	}
}

func TestCheckProvenanceNoSubjects(t *testing.T) {
	r := validRecord()
	r.Subjects = nil
	result := CheckProvenance([]ProvenanceRecord{r}, ProvenanceGateOptions{
		MinLevel: SLSA1,
		Now:      provNow,
	})
	hasFinding := false
	for _, f := range result.Findings {
		if f.Code == "PROVENANCE_NO_SUBJECTS" {
			hasFinding = true
		}
	}
	if !hasFinding {
		t.Fatal("expected PROVENANCE_NO_SUBJECTS finding")
	}
}

func TestCheckProvenanceMultipleRecords(t *testing.T) {
	r1 := validRecord()
	r2 := validRecord()
	r2.RecordID = "PROV-002"
	r2.SLSALevel = SLSA3

	result := CheckProvenance([]ProvenanceRecord{r1, r2}, ProvenanceGateOptions{
		MinLevel: SLSA1,
		Now:      provNow,
	})
	if result.Status != GatePassed {
		t.Fatalf("expected passed, got %s", result.Status)
	}
	if result.Summary.TotalRecords != 2 {
		t.Fatalf("expected 2 total records, got %d", result.Summary.TotalRecords)
	}
	// Achieved level should be the minimum across records (SLSA2)
	if result.SLSALevel != SLSA2 {
		t.Fatalf("expected achieved SLSA2 (min of records), got %s", result.SLSALevel)
	}
}

func TestCheckProvenanceDefaultGateID(t *testing.T) {
	result := CheckProvenance([]ProvenanceRecord{validRecord()}, ProvenanceGateOptions{
		MinLevel: SLSA1,
		Now:      provNow,
	})
	if result.GateID != "provenance.slsa" {
		t.Fatalf("expected default gate ID provenance.slsa, got %s", result.GateID)
	}
}

func TestValidateProvenanceRecordValid(t *testing.T) {
	errs := ValidateProvenanceRecord(validRecord())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateProvenanceRecordInvalidID(t *testing.T) {
	r := validRecord()
	r.RecordID = "bad-id"
	errs := ValidateProvenanceRecord(r)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "record_id") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected record_id error, got %v", errs)
	}
}

func TestValidateProvenanceRecordMissingFields(t *testing.T) {
	r := ProvenanceRecord{RecordID: "PROV-001"}
	errs := ValidateProvenanceRecord(r)
	if len(errs) < 4 {
		t.Fatalf("expected multiple errors for empty record, got %d: %v", len(errs), errs)
	}
}

func TestSLSALevelIsValid(t *testing.T) {
	for _, l := range []SLSALevel{SLSA1, SLSA2, SLSA3, SLSA4} {
		if !l.IsValid() {
			t.Fatalf("expected %s to be valid", l)
		}
	}
	if SLSALevel("slsa5").IsValid() {
		t.Fatal("expected slsa5 to be invalid")
	}
}

func TestCheckProvenanceSchemaVersion(t *testing.T) {
	result := CheckProvenance([]ProvenanceRecord{validRecord()}, ProvenanceGateOptions{
		MinLevel: SLSA1,
		Now:      provNow,
	})
	if result.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", result.SchemaVersion)
	}
}

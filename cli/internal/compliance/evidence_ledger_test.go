package compliance

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleLedger() EvidenceLedger {
	return EvidenceLedger{
		SchemaVersion: "0.1.0",
		GeneratedAt:   "2026-05-02",
		Status:        "draft",
		ClaimBoundary: "Evidence ledger baseline only.",
		EvidenceCategories: []EvidenceCategory{
			{ID: "EV-REF-001", Category: "external_reference_register", ExpectedLocation: "docs/regulated/refs.yaml", CurrentStatus: EvidencePresent, ClaimAllowed: "reference_registered"},
			{ID: "EV-QMS-001", Category: "quality_system_documents", ExpectedLocation: "docs/regulated/quality-system/", CurrentStatus: EvidenceDraftNotEffective, ClaimAllowed: "documentation_baseline_only"},
			{ID: "EV-VAL-001", Category: "validation_evidence", ExpectedLocation: "docs/regulated/validation-pack/", CurrentStatus: EvidenceRequiresEvidence, ClaimAllowed: "none"},
		},
		BlockingGaps: []BlockingGap{
			{ID: "GAP-QMS-OWNER", Description: "Quality owner not assigned.", Severity: SeverityMajor, Status: GapOpen, BlocksClaims: []string{"regulated_grade", "effective_qms"}},
			{ID: "GAP-TRAINING", Description: "Training records absent.", Severity: SeverityMajor, Status: GapOpen, BlocksClaims: []string{"effective_sop_use"}},
		},
	}
}

func TestParseLedger(t *testing.T) {
	data := []byte(`
schema_version: "0.1.0"
generated_at: "2026-05-02"
status: draft
claim_boundary: "test"
evidence_categories:
  - id: EV-001
    category: test_cat
    expected_location: "test/path"
    current_status: present_draft
    claim_allowed: "test_claim"
blocking_gaps: []
`)
	ledger, err := ParseLedger(data)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if ledger.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", ledger.SchemaVersion)
	}
	if len(ledger.EvidenceCategories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(ledger.EvidenceCategories))
	}
}

func TestParseLedgerInvalid(t *testing.T) {
	_, err := ParseLedger([]byte(`{invalid`))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadLedgerFromRepo(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "regulated", "evidence-index", "evidence-ledger.yaml")
	ledger, err := LoadLedger(path)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if ledger.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", ledger.SchemaVersion)
	}
	if len(ledger.EvidenceCategories) == 0 {
		t.Fatal("expected at least one evidence category")
	}
	if len(ledger.BlockingGaps) == 0 {
		t.Fatal("expected at least one blocking gap")
	}
}

func TestVerifyLedgerFindsBlockingGaps(t *testing.T) {
	result := VerifyLedger(sampleLedger(), "")

	if result.OpenGaps != 2 {
		t.Fatalf("expected 2 open gaps, got %d", result.OpenGaps)
	}
	if len(result.BlockedClaims) != 3 {
		t.Fatalf("expected 3 blocked claims, got %d: %v", len(result.BlockedClaims), result.BlockedClaims)
	}
}

func TestVerifyLedgerFindsMissingEvidence(t *testing.T) {
	result := VerifyLedger(sampleLedger(), "")

	hasMissing := false
	for _, f := range result.Findings {
		if f.Code == "EVIDENCE_MISSING" {
			hasMissing = true
		}
	}
	if !hasMissing {
		t.Fatal("expected EVIDENCE_MISSING finding for EV-VAL-001")
	}
}

func TestVerifyLedgerCountsActionable(t *testing.T) {
	result := VerifyLedger(sampleLedger(), "")

	if result.TotalEntries != 3 {
		t.Fatalf("expected 3 total entries, got %d", result.TotalEntries)
	}
	if result.ActionableEntries != 1 {
		t.Fatalf("expected 1 actionable, got %d", result.ActionableEntries)
	}
	if result.BlockedEntries != 2 {
		t.Fatalf("expected 2 blocked, got %d", result.BlockedEntries)
	}
}

func TestVerifyLedgerComplianceRatio(t *testing.T) {
	result := VerifyLedger(sampleLedger(), "")
	expected := 1.0 / 3.0
	if result.ComplianceRatio < expected-0.01 || result.ComplianceRatio > expected+0.01 {
		t.Fatalf("expected ratio ~0.333, got %f", result.ComplianceRatio)
	}
}

func TestVerifyLedgerValid(t *testing.T) {
	ledger := EvidenceLedger{
		SchemaVersion: "0.1.0",
		GeneratedAt:   "2026-05-02",
		EvidenceCategories: []EvidenceCategory{
			{ID: "EV-001", Category: "test", ExpectedLocation: "test/", CurrentStatus: EvidenceEffective, ClaimAllowed: "ok"},
		},
	}
	result := VerifyLedger(ledger, "")
	if !result.Valid {
		t.Fatalf("expected valid ledger, got findings: %v", result.Findings)
	}
}

func TestRegisterEvidenceEntry(t *testing.T) {
	ledger := sampleLedger()
	newEntry := EvidenceCategory{ID: "EV-NEW-001", Category: "new_cat", ExpectedLocation: "new/path", CurrentStatus: EvidencePresent, ClaimAllowed: "new_claim"}

	RegisterEvidenceEntry(&ledger, newEntry)
	if len(ledger.EvidenceCategories) != 4 {
		t.Fatalf("expected 4 entries after register, got %d", len(ledger.EvidenceCategories))
	}
}

func TestRegisterEntryUpdates(t *testing.T) {
	ledger := sampleLedger()
	updated := EvidenceCategory{ID: "EV-REF-001", Category: "updated_cat", ExpectedLocation: "new/path", CurrentStatus: EvidenceEffective, ClaimAllowed: "full"}

	RegisterEvidenceEntry(&ledger, updated)
	if len(ledger.EvidenceCategories) != 3 {
		t.Fatalf("expected 3 entries (updated in place), got %d", len(ledger.EvidenceCategories))
	}
	if ledger.EvidenceCategories[0].CurrentStatus != EvidenceEffective {
		t.Fatalf("expected effective, got %s", ledger.EvidenceCategories[0].CurrentStatus)
	}
}

func TestUpdateEntryStatus(t *testing.T) {
	ledger := sampleLedger()
	err := UpdateEntryStatus(&ledger, "EV-VAL-001", EvidenceEffective)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ledger.EvidenceCategories[2].CurrentStatus != EvidenceEffective {
		t.Fatalf("expected effective, got %s", ledger.EvidenceCategories[2].CurrentStatus)
	}
}

func TestUpdateEntryStatusNotFound(t *testing.T) {
	ledger := sampleLedger()
	err := UpdateEntryStatus(&ledger, "NONEXISTENT", EvidenceEffective)
	if err == nil {
		t.Fatal("expected error for missing entry")
	}
}

func TestResolveGap(t *testing.T) {
	ledger := sampleLedger()
	err := ResolveGap(&ledger, "GAP-QMS-OWNER", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ledger.BlockingGaps[0].Status != GapResolved {
		t.Fatalf("expected resolved, got %s", ledger.BlockingGaps[0].Status)
	}

	// Verify claims are unblocked after resolution.
	result := VerifyLedger(ledger, "")
	for _, claim := range result.BlockedClaims {
		if claim == "regulated_grade" || claim == "effective_qms" {
			t.Fatalf("expected claims unblocked after gap resolution, still blocked: %s", claim)
		}
	}
}

func TestResolveGapNotFound(t *testing.T) {
	ledger := sampleLedger()
	err := ResolveGap(&ledger, "NONEXISTENT", time.Now())
	if err == nil {
		t.Fatal("expected error for missing gap")
	}
}

func TestValidateLedger(t *testing.T) {
	errs := ValidateLedger(sampleLedger())
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateLedgerMissingVersion(t *testing.T) {
	ledger := sampleLedger()
	ledger.SchemaVersion = ""
	errs := ValidateLedger(ledger)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "schema_version") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected schema_version error, got %v", errs)
	}
}

func TestValidateLedgerDuplicateID(t *testing.T) {
	ledger := sampleLedger()
	ledger.EvidenceCategories = append(ledger.EvidenceCategories, EvidenceCategory{
		ID: "EV-REF-001", Category: "dup", ExpectedLocation: "dup/", CurrentStatus: EvidencePresent,
	})
	errs := ValidateLedger(ledger)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "duplicated") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected duplicate ID error, got %v", errs)
	}
}

func TestValidateLedgerEmptyCategories(t *testing.T) {
	ledger := EvidenceLedger{SchemaVersion: "0.1.0", GeneratedAt: "2026-05-02"}
	errs := ValidateLedger(ledger)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "evidence_category") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected empty categories error, got %v", errs)
	}
}

func TestEvidenceStatusIsValid(t *testing.T) {
	valid := []EvidenceStatus{EvidencePresent, EvidenceDraftNotEffective, EvidenceRequiresEvidence, EvidenceGeneratedByCI, EvidenceEffective, EvidenceArchived}
	for _, s := range valid {
		if !s.IsValid() {
			t.Fatalf("expected %s to be valid", s)
		}
	}
	if EvidenceStatus("bogus").IsValid() {
		t.Fatal("expected bogus to be invalid")
	}
}

func TestEvidenceStatusIsActionable(t *testing.T) {
	actionable := []EvidenceStatus{EvidencePresent, EvidenceGeneratedByCI, EvidenceEffective}
	for _, s := range actionable {
		if !s.IsActionable() {
			t.Fatalf("expected %s to be actionable", s)
		}
	}
	notActionable := []EvidenceStatus{EvidenceDraftNotEffective, EvidenceRequiresEvidence, EvidenceArchived}
	for _, s := range notActionable {
		if s.IsActionable() {
			t.Fatalf("expected %s to NOT be actionable", s)
		}
	}
}

package compliance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sig(signerID string, role ApprovalRole, decision ApprovalDecision) ApprovalSignature {
	return ApprovalSignature{
		SignerID: signerID, SignerName: signerID,
		Role: role, Decision: decision,
		SignedAt: time.Now().UTC(), Method: MethodManual,
	}
}

func criticalRecord(id string, sigs ...ApprovalSignature) ApprovalRecord {
	return ApprovalRecord{
		RecordID: id, RecordType: "regulated_evidence",
		RecordPath: "evidence/" + id + ".yaml",
		Status: ApprovalApproved, Signatures: sigs,
	}
}

// --- Fully compliant critical record ---

func TestReviewGateFullyCompliant(t *testing.T) {
	rec := criticalRecord("REC-001",
		sig("alice", RoleAuthor, DecisionApproved),
		sig("bob", RoleReviewer, DecisionApproved),
		sig("carol", RoleApprover, DecisionApproved),
		sig("diana", RoleQualityUnit, DecisionApproved),
	)
	result := EvaluateReviewGates([]ApprovalRecord{rec}, DefaultReviewGatePolicy())
	if result.Verdict != VerdictCompliant {
		t.Fatalf("expected compliant, got %s (findings: %v)", result.Verdict, result.Findings)
	}
	if result.Compliant != 1 {
		t.Fatalf("expected 1 compliant, got %d", result.Compliant)
	}
}

// --- Missing independent reviewer ---

func TestReviewGateMissingReviewer(t *testing.T) {
	rec := criticalRecord("REC-002",
		sig("alice", RoleAuthor, DecisionApproved),
		sig("diana", RoleQualityUnit, DecisionApproved),
	)
	result := EvaluateReviewGates([]ApprovalRecord{rec}, DefaultReviewGatePolicy())
	if result.Verdict != VerdictNonCompliant {
		t.Fatalf("expected non_compliant, got %s", result.Verdict)
	}
	assertFindingControl(t, result.Findings, "INDEPENDENT-REVIEW")
}

// --- Missing quality unit ---

func TestReviewGateMissingQualityUnit(t *testing.T) {
	rec := criticalRecord("REC-003",
		sig("alice", RoleAuthor, DecisionApproved),
		sig("bob", RoleReviewer, DecisionApproved),
	)
	result := EvaluateReviewGates([]ApprovalRecord{rec}, DefaultReviewGatePolicy())
	if result.Verdict != VerdictNonCompliant {
		t.Fatalf("expected non_compliant, got %s", result.Verdict)
	}
	assertFindingControl(t, result.Findings, "QUALITY-UNIT-SIGNOFF")
}

// --- Self-approval blocked ---

func TestReviewGateSelfApproval(t *testing.T) {
	rec := criticalRecord("REC-004",
		sig("alice", RoleAuthor, DecisionApproved),
		sig("alice", RoleReviewer, DecisionApproved), // same person!
		sig("diana", RoleQualityUnit, DecisionApproved),
	)
	result := EvaluateReviewGates([]ApprovalRecord{rec}, DefaultReviewGatePolicy())
	if result.Verdict != VerdictNonCompliant {
		t.Fatalf("expected non_compliant, got %s", result.Verdict)
	}
	assertFindingControl(t, result.Findings, "NO-SELF-APPROVAL")
}

// --- Reviewer not independent (is author) ---

func TestReviewGateReviewerIsAuthor(t *testing.T) {
	rec := criticalRecord("REC-005",
		sig("alice", RoleAuthor, DecisionApproved),
		sig("alice", RoleReviewer, DecisionApproved),
		sig("bob", RoleApprover, DecisionApproved),
		sig("diana", RoleQualityUnit, DecisionApproved),
	)
	result := EvaluateReviewGates([]ApprovalRecord{rec}, DefaultReviewGatePolicy())
	assertFindingControl(t, result.Findings, "INDEPENDENT-REVIEW")
	assertFindingControl(t, result.Findings, "NO-SELF-APPROVAL")
}

// --- Not approved status ---

func TestReviewGateNotApprovedStatus(t *testing.T) {
	rec := ApprovalRecord{
		RecordID: "REC-006", RecordType: "regulated_evidence",
		RecordPath: "evidence/REC-006.yaml",
		Status: ApprovalInReview,
		Signatures: []ApprovalSignature{
			sig("alice", RoleAuthor, DecisionApproved),
			sig("bob", RoleReviewer, DecisionApproved),
			sig("diana", RoleQualityUnit, DecisionApproved),
		},
	}
	result := EvaluateReviewGates([]ApprovalRecord{rec}, DefaultReviewGatePolicy())
	assertFindingControl(t, result.Findings, "APPROVAL-STATUS")
}

// --- Standard record (lighter requirements) ---

func TestReviewGateStandardCompliant(t *testing.T) {
	rec := ApprovalRecord{
		RecordID: "REC-007", RecordType: "standard_evidence",
		Signatures: []ApprovalSignature{
			sig("alice", RoleAuthor, DecisionApproved),
			sig("bob", RoleReviewer, DecisionApproved),
		},
	}
	result := EvaluateReviewGates([]ApprovalRecord{rec}, DefaultReviewGatePolicy())
	if result.Verdict != VerdictCompliant {
		t.Fatalf("expected compliant for standard, got %s (findings: %v)", result.Verdict, result.Findings)
	}
}

func TestReviewGateStandardMissingReviewer(t *testing.T) {
	rec := ApprovalRecord{
		RecordID: "REC-008", RecordType: "standard_evidence",
		Signatures: []ApprovalSignature{
			sig("alice", RoleAuthor, DecisionApproved),
		},
	}
	result := EvaluateReviewGates([]ApprovalRecord{rec}, DefaultReviewGatePolicy())
	if result.Verdict != VerdictPartial {
		t.Fatalf("expected partial for standard missing reviewer, got %s", result.Verdict)
	}
	assertFindingControl(t, result.Findings, "REVIEWER-PRESENT")
}

// --- Unknown record type (minimal check) ---

func TestReviewGateUnknownTypeMinimal(t *testing.T) {
	rec := ApprovalRecord{
		RecordID: "REC-009", RecordType: "other",
		Signatures: []ApprovalSignature{
			sig("alice", RoleAuthor, DecisionApproved),
		},
	}
	result := EvaluateReviewGates([]ApprovalRecord{rec}, DefaultReviewGatePolicy())
	if result.Verdict != VerdictCompliant {
		t.Fatalf("expected compliant for unknown type with author, got %s", result.Verdict)
	}
}

// --- Empty records ---

func TestReviewGateEmptyRecords(t *testing.T) {
	result := EvaluateReviewGates(nil, DefaultReviewGatePolicy())
	if result.Verdict != VerdictCompliant {
		t.Fatalf("expected compliant for empty, got %s", result.Verdict)
	}
	if result.TotalRecords != 0 {
		t.Fatalf("expected 0, got %d", result.TotalRecords)
	}
}

// --- Mixed records ---

func TestReviewGateMixedRecords(t *testing.T) {
	records := []ApprovalRecord{
		criticalRecord("GOOD",
			sig("alice", RoleAuthor, DecisionApproved),
			sig("bob", RoleReviewer, DecisionApproved),
			sig("carol", RoleApprover, DecisionApproved),
			sig("diana", RoleQualityUnit, DecisionApproved),
		),
		criticalRecord("BAD",
			sig("alice", RoleAuthor, DecisionApproved),
			// missing reviewer, quality_unit
		),
	}
	result := EvaluateReviewGates(records, DefaultReviewGatePolicy())
	if result.Verdict != VerdictNonCompliant {
		t.Fatalf("expected non_compliant for mixed, got %s", result.Verdict)
	}
	if result.Compliant != 1 {
		t.Fatalf("expected 1 compliant, got %d", result.Compliant)
	}
}

// --- Finding structure ---

func TestReviewGateFindingStructure(t *testing.T) {
	rec := criticalRecord("REC-BAD") // no signatures at all
	result := EvaluateReviewGates([]ApprovalRecord{rec}, DefaultReviewGatePolicy())
	for _, f := range result.Findings {
		if !strings.HasPrefix(f.ID, "RG-") {
			t.Fatalf("expected RG- prefix, got %q", f.ID)
		}
		if f.Control == "" || f.Severity == "" || f.Message == "" || f.Remediation == "" || f.Owner == "" {
			t.Fatalf("finding %s has empty field", f.ID)
		}
	}
}

// --- Dir scanning ---

func TestReviewGatesFromDir(t *testing.T) {
	dir := t.TempDir()
	// Write a valid approval record
	data := `record_id: REC-DIR-001
record_type: regulated_evidence
record_path: evidence/rec.yaml
status: approved
signatures:
  - signer_id: alice
    role: author
    decision: approved
    method: manual_attestation
  - signer_id: bob
    role: reviewer
    decision: approved
    method: manual_attestation
  - signer_id: diana
    role: quality_unit
    decision: approved
    method: manual_attestation
`
	os.WriteFile(filepath.Join(dir, "approval-rec.yaml"), []byte(data), 0o644)

	result, err := EvaluateReviewGatesFromDir(dir, DefaultReviewGatePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalRecords != 1 {
		t.Fatalf("expected 1 record, got %d", result.TotalRecords)
	}
}

func TestReviewGatesFromDirEmpty(t *testing.T) {
	result, err := EvaluateReviewGatesFromDir(t.TempDir(), DefaultReviewGatePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalRecords != 0 {
		t.Fatalf("expected 0 records, got %d", result.TotalRecords)
	}
}

// --- helper ---

func assertFindingControl(t *testing.T, findings []Finding, control string) {
	t.Helper()
	for _, f := range findings {
		if f.Control == control {
			return
		}
	}
	var controls []string
	for _, f := range findings {
		controls = append(controls, f.Control)
	}
	t.Fatalf("expected finding with control %q, got %v", control, controls)
}

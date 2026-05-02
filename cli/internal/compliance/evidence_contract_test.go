package compliance

import (
	"path/filepath"
	"strings"
	"testing"
)

func evidenceRoot() string {
	return filepath.Clean(filepath.Join("..", "..", ".."))
}

// --- Presence checks ---

func TestEvidenceContractPresenceOnRepo(t *testing.T) {
	result, err := CheckEvidenceContractPresence(evidenceRoot())
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictCompliant {
		for _, f := range result.Findings {
			t.Logf("[%s] %s: %s", f.ID, f.Control, f.Message)
		}
		t.Fatalf("expected compliant, got %s", result.Verdict)
	}
}

func TestEvidenceContractPresenceEmptyRepo(t *testing.T) {
	result, err := CheckEvidenceContractPresence(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictNonCompliant {
		t.Fatalf("expected non_compliant, got %s", result.Verdict)
	}
	if result.Blocking == 0 {
		t.Fatal("expected blocking findings")
	}
}

// --- Ledger validation ---

func TestValidateEvidenceLedgerOnRepo(t *testing.T) {
	path := filepath.Join(evidenceRoot(), "docs", "regulated", "evidence-index", "evidence-ledger.yaml")
	result, err := ValidateEvidenceLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("verdict=%s records=%d valid=%d findings=%d blocking=%d",
		result.Verdict, result.TotalRecords, result.ValidRecords, result.TotalFindings, result.Blocking)
	for _, f := range result.Findings {
		t.Logf("  [%s] %s %s", f.ID, f.Control, f.Message)
	}
}

func TestValidateEvidenceLedgerValid(t *testing.T) {
	data := []byte(`
schema_version: "0.1.0"
status: draft
claim_boundary: "Evidence baseline only."
evidence_categories:
  - id: EV-REF-001
    category: external_reference_register
    expected_location: "docs/refs.yaml"
    current_status: present
    claim_allowed: "reference_registered"
  - id: EV-QMS-001
    category: quality_system_document
    expected_location: "docs/qms/"
    current_status: draft
    claim_allowed: "documentation_baseline_only"
blocking_gaps:
  - id: GAP-AUDIT-001
    description: "No audit."
    severity: major
    status: open
    blocks_claims:
      - independent_audit
`)
	result, err := ValidateEvidenceLedgerFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalRecords != 2 {
		t.Fatalf("expected 2 records, got %d", result.TotalRecords)
	}
	if result.Blocking > 0 {
		for _, f := range result.Findings {
			t.Logf("[%s] %s", f.ID, f.Message)
		}
		t.Fatalf("expected no blocking, got %d", result.Blocking)
	}
}

func TestValidateEvidenceLedgerMissingClaimBoundary(t *testing.T) {
	data := []byte(`
schema_version: "0.1.0"
evidence_categories:
  - id: EV-001
    category: test_result
    expected_location: "tests/"
    current_status: generated
    claim_allowed: "test_pass"
`)
	result, err := ValidateEvidenceLedgerFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range result.Findings {
		if f.Control == "LEDGER-CLAIM-BOUNDARY" && f.Blocking {
			found = true
		}
	}
	if !found {
		t.Fatal("expected LEDGER-CLAIM-BOUNDARY blocking finding")
	}
}

func TestValidateEvidenceLedgerBadRecordID(t *testing.T) {
	data := []byte(`
claim_boundary: "test"
evidence_categories:
  - id: BAD-FORMAT
    category: test_result
    expected_location: "x"
    current_status: draft
    claim_allowed: "none"
`)
	result, err := ValidateEvidenceLedgerFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range result.Findings {
		if f.Control == "RECORD-ID-FORMAT" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected RECORD-ID-FORMAT finding")
	}
}

func TestValidateEvidenceLedgerMissingLocation(t *testing.T) {
	data := []byte(`
claim_boundary: "test"
evidence_categories:
  - id: EV-001
    category: test_result
    expected_location: ""
    current_status: draft
    claim_allowed: "none"
`)
	result, err := ValidateEvidenceLedgerFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range result.Findings {
		if f.Control == "RECORD-LOCATION" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected RECORD-LOCATION finding")
	}
}

func TestValidateEvidenceLedgerBadGapSeverity(t *testing.T) {
	data := []byte(`
claim_boundary: "test"
evidence_categories: []
blocking_gaps:
  - id: GAP-X-001
    description: "test"
    severity: extreme
    status: open
    blocks_claims: [x]
`)
	result, err := ValidateEvidenceLedgerFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range result.Findings {
		if f.Control == "GAP-SEVERITY" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected GAP-SEVERITY finding")
	}
}

func TestValidateEvidenceLedgerFindingStructure(t *testing.T) {
	data := []byte(`
evidence_categories:
  - id: bad
    expected_location: ""
    claim_allowed: ""
`)
	result, err := ValidateEvidenceLedgerFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range result.Findings {
		if !strings.HasPrefix(f.ID, "EC-") {
			t.Fatalf("expected EC- prefix, got %q", f.ID)
		}
		if f.Control == "" || f.Severity == "" || f.Message == "" || f.Remediation == "" || f.Owner == "" {
			t.Fatalf("finding %s has empty required field", f.ID)
		}
	}
}

func TestValidateEvidenceLedgerInvalidYAML(t *testing.T) {
	_, err := ValidateEvidenceLedgerFromBytes([]byte(`{broken`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateEvidenceLedgerMissingFile(t *testing.T) {
	_, err := ValidateEvidenceLedger("/nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error")
	}
}

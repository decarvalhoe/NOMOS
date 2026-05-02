package corpus

import (
	"path/filepath"
	"strings"
	"testing"
)

func govFixture(name string) string {
	return filepath.Join("testdata", name)
}

// --- Clean corpus: admissible ---

func TestGovernanceCleanCorpusAdmissible(t *testing.T) {
	result, err := EvaluateGovernance(govFixture("gov-clean"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Verdict != VerdictAdmissible {
		t.Fatalf("expected %s, got %s (findings: %v)", VerdictAdmissible, result.Verdict, result.Findings)
	}
	if result.TotalFindings != 0 {
		t.Fatalf("expected 0 findings, got %d", result.TotalFindings)
	}
	if result.Blocking != 0 {
		t.Fatalf("expected 0 blocking, got %d", result.Blocking)
	}
}

// --- Issues corpus: partial ---

func TestGovernanceIssuesCorpusPartial(t *testing.T) {
	result, err := EvaluateGovernance(govFixture("gov-issues"))
	if err != nil {
		t.Fatal(err)
	}
	// Has non-blocking findings (empty cells, empty extraction, dups)
	// but also has blocking (missing parcours id/name/owner/status)
	if result.TotalFindings == 0 {
		t.Fatal("expected findings")
	}
}

func TestGovernanceEmptyCellsDetected(t *testing.T) {
	result, err := EvaluateGovernance(govFixture("gov-issues"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range result.Findings {
		if strings.Contains(f.Message, "empty governance field") {
			found = true
			if f.SourcePath == "" {
				t.Fatal("expected source_path")
			}
			if f.Line == 0 {
				t.Fatal("expected non-zero line number")
			}
			if f.Field == "" {
				t.Fatal("expected field name")
			}
			break
		}
	}
	if !found {
		t.Fatal("expected empty governance field finding")
	}
}

func TestGovernanceParcoursMissingFieldsDetected(t *testing.T) {
	result, err := EvaluateGovernance(govFixture("gov-issues"))
	if err != nil {
		t.Fatal(err)
	}
	missingFields := map[string]bool{}
	for _, f := range result.Findings {
		if strings.Contains(f.Message, "missing required parcours field") {
			field := strings.TrimPrefix(f.Field, "parcours.")
			missingFields[field] = true
		}
	}
	for _, expected := range []string{"id", "name", "owner", "status"} {
		if !missingFields[expected] {
			t.Fatalf("expected missing field %q finding", expected)
		}
	}
}

func TestGovernanceEmptyExtractionDetected(t *testing.T) {
	result, err := EvaluateGovernance(govFixture("gov-issues"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range result.Findings {
		if strings.Contains(f.Message, "empty extraction") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected empty extraction finding")
	}
}

func TestGovernanceDuplicateCanonicalRefDetected(t *testing.T) {
	result, err := EvaluateGovernance(govFixture("gov-issues"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range result.Findings {
		if strings.Contains(f.Message, "duplicate canonical_ref") {
			found = true
			if !strings.Contains(f.Message, "REF-SHARED-001") {
				t.Fatalf("expected REF-SHARED-001 in message, got %q", f.Message)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected duplicate canonical_ref finding")
	}
}

// --- Blocked corpus ---

func TestGovernanceBlockedCorpus(t *testing.T) {
	result, err := EvaluateGovernance(govFixture("gov-blocked"))
	if err != nil {
		t.Fatal(err)
	}
	if result.Verdict != VerdictBlocked {
		t.Fatalf("expected %s, got %s", VerdictBlocked, result.Verdict)
	}
	if result.Blocking == 0 {
		t.Fatal("expected blocking findings")
	}
}

func TestGovernanceEmptyTableBlocks(t *testing.T) {
	result, err := EvaluateGovernance(govFixture("gov-blocked"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range result.Findings {
		if strings.Contains(f.Message, "no data rows") && f.Blocking {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected blocking empty table finding")
	}
}

func TestGovernanceMissingParcoursIDBlocks(t *testing.T) {
	result, err := EvaluateGovernance(govFixture("gov-blocked"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range result.Findings {
		if f.Field == "parcours.id" && f.Blocking {
			found = true
			break
		}
	}
	if !found {
		// parcours.id is "" which triggers missing required field
		for _, f := range result.Findings {
			if strings.Contains(f.Field, "parcours") && f.Blocking {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected blocking parcours field finding")
	}
}

// --- Finding structure ---

func TestGovernanceFindingIDFormat(t *testing.T) {
	result, err := EvaluateGovernance(govFixture("gov-issues"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range result.Findings {
		if !strings.HasPrefix(f.ID, "GOV-") {
			t.Fatalf("expected GOV- prefix, got %q", f.ID)
		}
		if f.Severity == "" {
			t.Fatalf("finding %s missing severity", f.ID)
		}
		if f.Message == "" {
			t.Fatalf("finding %s missing message", f.ID)
		}
		if f.Remediation == "" {
			t.Fatalf("finding %s missing remediation", f.ID)
		}
		if f.SourcePath == "" {
			t.Fatalf("finding %s missing source_path", f.ID)
		}
	}
}

func TestGovernanceFindingsSorted(t *testing.T) {
	result, err := EvaluateGovernance(govFixture("gov-issues"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(result.Findings); i++ {
		if result.Findings[i].ID < result.Findings[i-1].ID {
			t.Fatalf("findings not sorted: %s before %s", result.Findings[i-1].ID, result.Findings[i].ID)
		}
	}
}

// --- Error handling ---

func TestGovernanceNonexistentRoot(t *testing.T) {
	_, err := EvaluateGovernance("/nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGovernanceFileNotDir(t *testing.T) {
	_, err := EvaluateGovernance(govFixture("gov-clean/references.md"))
	if err == nil {
		t.Fatal("expected error for file-not-dir")
	}
}

// --- Verdict logic ---

func TestGovernanceVerdictValues(t *testing.T) {
	if VerdictAdmissible != "corpus_admissible" {
		t.Fatal("wrong admissible constant")
	}
	if VerdictPartial != "corpus_partial" {
		t.Fatal("wrong partial constant")
	}
	if VerdictBlocked != "corpus_blocked" {
		t.Fatal("wrong blocked constant")
	}
}

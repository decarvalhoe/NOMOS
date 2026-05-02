package compliance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckAlignmentPassesWhenAllGoverned(t *testing.T) {
	root := t.TempDir()
	registerPath := writeRegister(t, root, []RegisterEntry{
		{ID: "ICH-Q9R1", Title: "ICH Q9(R1) Quality Risk Management", EvidenceStatus: "requires_evidence", CheckedOn: "2026-05-01"},
		{ID: "ISO-13485-2016", Title: "ISO 13485:2016", EvidenceStatus: "requires_evidence", CheckedOn: "2026-04-01"},
	})
	docPath := writeDoc(t, root, "design.md", "We follow ICH Q9 and ISO 13485 standards.\n")

	result, err := CheckAlignment([]string{docPath}, registerPath)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !result.Pass {
		t.Fatalf("expected gate to pass, findings: %+v", result.Findings)
	}
}

func TestCheckAlignmentFailsWhenRefNotGoverned(t *testing.T) {
	root := t.TempDir()
	registerPath := writeRegister(t, root, []RegisterEntry{
		{ID: "ICH-Q9R1", Title: "ICH Q9(R1)", EvidenceStatus: "requires_evidence", CheckedOn: "2026-05-01"},
	})
	docPath := writeDoc(t, root, "plan.md", "This system complies with GAMP 5 and SLSA level 3.\n")

	result, err := CheckAlignment([]string{docPath}, registerPath)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if result.Pass {
		t.Fatal("expected gate to fail — GAMP5 and SLSA not in register")
	}

	hasGAMP := false
	hasSLSA := false
	for _, f := range result.Findings {
		if f.Code == "ref_not_governed" {
			if contains(f.RefMatch, "GAMP") {
				hasGAMP = true
			}
			if contains(f.RefMatch, "SLSA") {
				hasSLSA = true
			}
		}
	}
	if !hasGAMP {
		t.Fatal("expected finding for ungoverned GAMP5")
	}
	if !hasSLSA {
		t.Fatal("expected finding for ungoverned SLSA")
	}
}

func TestCheckAlignmentDetectsMissingEvidenceStatus(t *testing.T) {
	root := t.TempDir()
	registerPath := writeRegister(t, root, []RegisterEntry{
		{ID: "ISO-13485-2016", Title: "ISO 13485:2016", EvidenceStatus: "", CheckedOn: "2026-04-01"},
	})
	docPath := writeDoc(t, root, "spec.md", "Per ISO 13485 clause 7.3\n")

	result, err := CheckAlignment([]string{docPath}, registerPath)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if result.Pass {
		t.Fatal("expected gate to fail — missing evidence_status")
	}

	found := false
	for _, f := range result.Findings {
		if f.Code == "ref_missing_evidence_status" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ref_missing_evidence_status finding: %+v", result.Findings)
	}
}

func TestCheckAlignmentDetectsMissingReviewDate(t *testing.T) {
	root := t.TempDir()
	registerPath := writeRegister(t, root, []RegisterEntry{
		{ID: "NIST-SP-800-218", Title: "NIST SP 800-218 SSDF", EvidenceStatus: "requires_evidence", CheckedOn: ""},
	})
	docPath := writeDoc(t, root, "security.md", "Follow NIST SP 800-218 for secure SDLC.\n")

	result, err := CheckAlignment([]string{docPath}, registerPath)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	// Missing review date is warning, not error — gate still passes.
	if !result.Pass {
		t.Fatalf("expected gate to pass (warning only), findings: %+v", result.Findings)
	}

	found := false
	for _, f := range result.Findings {
		if f.Code == "ref_missing_review_date" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected ref_missing_review_date finding")
	}
}

func TestCheckAlignmentDetectsStaleReview(t *testing.T) {
	root := t.TempDir()
	registerPath := writeRegister(t, root, []RegisterEntry{
		{ID: "FDA-21CFR11-11.10", Title: "21 CFR Part 11", EvidenceStatus: "requires_evidence", CheckedOn: "2020-01-01"},
	})
	docPath := writeDoc(t, root, "audit.md", "Systems must comply with 21 CFR Part 11.\n")

	result, err := CheckAlignment([]string{docPath}, registerPath)
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	found := false
	for _, f := range result.Findings {
		if f.Code == "ref_review_stale" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected ref_review_stale finding for 2020 review date")
	}
}

func TestCheckAlignmentMultipleFiles(t *testing.T) {
	root := t.TempDir()
	registerPath := writeRegister(t, root, []RegisterEntry{
		{ID: "ICH-Q10", Title: "ICH Q10", EvidenceStatus: "requires_evidence", CheckedOn: "2026-03-01"},
	})
	doc1 := writeDoc(t, root, "a.md", "Follows ICH Q10 principles.\n")
	doc2 := writeDoc(t, root, "b.md", "Also references GAMP 5 framework.\n")

	result, err := CheckAlignment([]string{doc1, doc2}, registerPath)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if result.Pass {
		t.Fatal("expected gate to fail — GAMP5 not governed")
	}
}

func TestCheckAlignmentEmptyDocs(t *testing.T) {
	root := t.TempDir()
	registerPath := writeRegister(t, root, []RegisterEntry{
		{ID: "ICH-Q9R1", Title: "ICH Q9(R1)", EvidenceStatus: "requires_evidence", CheckedOn: "2026-05-01"},
	})

	result, err := CheckAlignment([]string{}, registerPath)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !result.Pass {
		t.Fatal("expected pass with no docs scanned")
	}
	if len(result.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(result.Findings))
	}
}

func TestCheckAlignmentRegisterNotFound(t *testing.T) {
	_, err := CheckAlignment([]string{}, "/nonexistent-register.yaml")
	if !errors.Is(err, ErrRegisterNotFound) {
		t.Fatalf("expected ErrRegisterNotFound, got: %v", err)
	}
}

func TestGateErrorReturnsNilOnPass(t *testing.T) {
	result := AlignmentResult{Pass: true}
	if err := GateError(result); err != nil {
		t.Fatalf("expected nil error on pass, got: %v", err)
	}
}

func TestGateErrorReturnsErrorOnFail(t *testing.T) {
	result := AlignmentResult{
		Pass: false,
		Findings: []AlignmentFinding{
			{Code: "ref_not_governed", Severity: "error"},
			{Code: "ref_not_governed", Severity: "error"},
		},
	}
	err := GateError(result)
	if !errors.Is(err, ErrGateFailed) {
		t.Fatalf("expected ErrGateFailed, got: %v", err)
	}
}

func TestCheckAlignmentNoCitationsInFile(t *testing.T) {
	root := t.TempDir()
	registerPath := writeRegister(t, root, []RegisterEntry{
		{ID: "ICH-Q9R1", Title: "ICH Q9(R1)", EvidenceStatus: "requires_evidence", CheckedOn: "2026-05-01"},
	})
	docPath := writeDoc(t, root, "readme.md", "This is a simple readme with no external references.\n")

	result, err := CheckAlignment([]string{docPath}, registerPath)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !result.Pass {
		t.Fatal("expected pass with no citations")
	}
}

// Helpers

func writeRegister(t *testing.T, root string, entries []RegisterEntry) string {
	t.Helper()
	reg := Register{
		SchemaVersion: "0.1.0",
		References:    entries,
	}
	data, err := os.CreateTemp(root, "register-*.yaml")
	if err != nil {
		t.Fatalf("create register: %v", err)
	}
	defer data.Close()

	content := "schema_version: \"0.1.0\"\nreferences:\n"
	for _, e := range reg.References {
		content += "  - id: " + e.ID + "\n"
		content += "    title: \"" + e.Title + "\"\n"
		content += "    evidence_status: " + e.EvidenceStatus + "\n"
		content += "    checked_on: \"" + e.CheckedOn + "\"\n"
	}
	if _, err := data.WriteString(content); err != nil {
		t.Fatalf("write register: %v", err)
	}
	return data.Name()
}

func writeDoc(t *testing.T, root, name, content string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write doc %s: %v", name, err)
	}
	return path
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsCI(s, substr))
}

func containsCI(s, substr string) bool {
	return len(s) >= len(substr) && (indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	sl := len(substr)
	for i := 0; i <= len(s)-sl; i++ {
		if eqCI(s[i:i+sl], substr) {
			return i
		}
	}
	return -1
}

func eqCI(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

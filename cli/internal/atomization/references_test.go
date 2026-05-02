package atomization

import (
	"testing"
)

func testAtom(text string) Atom {
	return Atom{
		ID:           "ATOM-001",
		Title:        "Test atom",
		Type:         "rule",
		Domain:       "insurance",
		Criticality:  "high",
		BusinessRule: "The insured must declare within 5 days.",
		SourceBlock: Block{
			ID:   "block-1",
			Text: text,
			Hash: "sha256:abc123",
			Span: Span{StartLine: 10, EndLine: 12},
		},
	}
}

// --- ExtractReferences ---

func TestExtractReferences_ExplicitSource(t *testing.T) {
	atom := testAtom("This rule comes from source: RULEBOOK-2026, art. 42.")
	refs := ExtractReferences(atom)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].SourceID != "RULEBOOK-2026" {
		t.Fatalf("expected RULEBOOK-2026, got %q", refs[0].SourceID)
	}
	if refs[0].Locator != "art. 42" {
		t.Fatalf("expected locator 'art. 42', got %q", refs[0].Locator)
	}
	if refs[0].AtomID != "ATOM-001" {
		t.Fatalf("expected ATOM-001, got %q", refs[0].AtomID)
	}
}

func TestExtractReferences_MultipleSourceRefs(t *testing.T) {
	atom := testAtom("source: REF-001 and source: REF-002")
	refs := ExtractReferences(atom)
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(refs))
	}
}

func TestExtractReferences_DedupSameSource(t *testing.T) {
	atom := testAtom("source: REF-001, again source: REF-001")
	refs := ExtractReferences(atom)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref (deduped), got %d", len(refs))
	}
}

func TestExtractReferences_InferredFromBlock(t *testing.T) {
	atom := testAtom("No explicit source reference here.")
	refs := ExtractReferences(atom)
	if len(refs) != 1 {
		t.Fatalf("expected 1 inferred ref, got %d", len(refs))
	}
	if refs[0].SourceID != "INSURANCE-SRC" {
		t.Fatalf("expected INSURANCE-SRC, got %q", refs[0].SourceID)
	}
	if refs[0].Locator != "line:10" {
		t.Fatalf("expected line:10 locator, got %q", refs[0].Locator)
	}
}

func TestExtractReferences_EmptyBlock(t *testing.T) {
	atom := Atom{
		ID:           "ATOM-002",
		Domain:       "finance",
		BusinessRule: "source: FIN-REF-001 p. 15",
		SourceBlock:  Block{},
	}
	refs := ExtractReferences(atom)
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref from business rule, got %d", len(refs))
	}
	if refs[0].SourceID != "FIN-REF-001" {
		t.Fatalf("expected FIN-REF-001, got %q", refs[0].SourceID)
	}
}

func TestExtractReferences_LocatorPatterns(t *testing.T) {
	cases := []struct {
		text    string
		locator string
	}{
		{"source: REF-001 p. 42", "p. 42"},
		{"source: REF-001 article 7", "article 7"},
		{"source: REF-001 § 12", "§ 12"},
		{"source: REF-001 s. 3", "s. 3"},
		{"source: REF-001 section 5", "section 5"},
	}
	for _, tc := range cases {
		atom := testAtom(tc.text)
		refs := ExtractReferences(atom)
		if len(refs) == 0 {
			t.Fatalf("text %q: expected refs", tc.text)
		}
		if refs[0].Locator != tc.locator {
			t.Fatalf("text %q: expected locator %q, got %q", tc.text, tc.locator, refs[0].Locator)
		}
	}
}

// --- ExtractCodeRefs ---

func TestExtractCodeRefs_Implements(t *testing.T) {
	atom := testAtom("implements: cli/internal/checks/sources.go")
	refs := ExtractCodeRefs(atom)
	if len(refs) != 1 {
		t.Fatalf("expected 1 code ref, got %d", len(refs))
	}
	if refs[0].Module != "cli/internal/checks/sources.go" {
		t.Fatalf("expected path, got %q", refs[0].Module)
	}
}

func TestExtractCodeRefs_See(t *testing.T) {
	atom := testAtom("see: pkg/domain/policy.go")
	refs := ExtractCodeRefs(atom)
	if len(refs) != 1 {
		t.Fatalf("expected 1 code ref, got %d", len(refs))
	}
}

func TestExtractCodeRefs_NoCodePath(t *testing.T) {
	atom := testAtom("This is just text without code paths.")
	refs := ExtractCodeRefs(atom)
	if len(refs) != 0 {
		t.Fatalf("expected 0 code refs, got %d", len(refs))
	}
}

func TestExtractCodeRefs_DedupSamePath(t *testing.T) {
	atom := testAtom("ref: a/b.go and see: a/b.go")
	refs := ExtractCodeRefs(atom)
	if len(refs) != 1 {
		t.Fatalf("expected 1 code ref (deduped), got %d", len(refs))
	}
}

// --- ExtractTestRefs ---

func TestExtractTestRefs_TestedBy(t *testing.T) {
	atom := testAtom("tested by tests/check_test.go")
	refs := ExtractTestRefs(atom)
	if len(refs) != 1 {
		t.Fatalf("expected 1 test ref, got %d", len(refs))
	}
	if refs[0].Path != "tests/check_test.go" {
		t.Fatalf("expected path, got %q", refs[0].Path)
	}
}

func TestExtractTestRefs_TestEquals(t *testing.T) {
	atom := testAtom("test: internal/validate/validate_test.go")
	refs := ExtractTestRefs(atom)
	if len(refs) != 1 {
		t.Fatalf("expected 1 test ref, got %d", len(refs))
	}
}

func TestExtractTestRefs_NoTestRef(t *testing.T) {
	atom := testAtom("No test mentioned here.")
	refs := ExtractTestRefs(atom)
	if len(refs) != 0 {
		t.Fatalf("expected 0 test refs, got %d", len(refs))
	}
}

// --- ProjectTraceMatrix ---

func TestProjectTraceMatrix_FullyCovered(t *testing.T) {
	atoms := []Atom{
		{
			ID: "ATOM-001", Title: "Declare sinistre", Type: "rule",
			Domain: "insurance", Criticality: "high",
			BusinessRule: "source: RULEBOOK-2026 art. 5, implements: cli/checks.go, tested by tests/checks_test.go",
			SourceBlock:  Block{ID: "b1", Text: "source: RULEBOOK-2026 art. 5, implements: cli/checks.go, tested by tests/checks_test.go", Hash: "sha256:abc", Span: Span{StartLine: 1}},
		},
	}
	matrix := ProjectTraceMatrix(atoms, "insurance")

	if matrix.TotalRows != 1 {
		t.Fatalf("expected 1 row, got %d", matrix.TotalRows)
	}
	if matrix.Covered != 1 {
		t.Fatalf("expected 1 covered, got %d", matrix.Covered)
	}
	if matrix.Rows[0].Status != "covered" {
		t.Fatalf("expected covered, got %s", matrix.Rows[0].Status)
	}
	if len(matrix.Rows[0].Gaps) != 0 {
		t.Fatalf("expected 0 gaps, got %v", matrix.Rows[0].Gaps)
	}
}

func TestProjectTraceMatrix_PartialCoverage(t *testing.T) {
	atoms := []Atom{
		{
			ID: "ATOM-002", Title: "Rule 2", Type: "rule",
			Domain: "insurance", Criticality: "medium",
			BusinessRule: "source: REF-001",
			SourceBlock:  Block{ID: "b2", Text: "source: REF-001", Hash: "sha256:def", Span: Span{StartLine: 5}},
		},
	}
	matrix := ProjectTraceMatrix(atoms, "insurance")

	if matrix.Partial != 1 {
		t.Fatalf("expected 1 partial, got %d", matrix.Partial)
	}
	if matrix.Rows[0].Status != "partial" {
		t.Fatalf("expected partial, got %s", matrix.Rows[0].Status)
	}
	if len(matrix.Rows[0].Gaps) == 0 {
		t.Fatal("expected gaps for partial row")
	}
}

func TestProjectTraceMatrix_Missing(t *testing.T) {
	atoms := []Atom{
		{
			ID: "ATOM-003", Title: "Rule 3", Type: "rule",
			BusinessRule: "No references at all",
			SourceBlock:  Block{},
		},
	}
	matrix := ProjectTraceMatrix(atoms, "unknown")

	if matrix.Missing != 1 {
		t.Fatalf("expected 1 missing, got %d", matrix.Missing)
	}
}

func TestProjectTraceMatrix_MultipleAtoms(t *testing.T) {
	atoms := []Atom{
		{
			ID: "A1", Title: "Covered", Type: "rule", Domain: "d",
			BusinessRule: "source: REF-001, implements: a/b.go, tested by tests/t.go",
			SourceBlock:  Block{ID: "b1", Text: "source: REF-001, implements: a/b.go, tested by tests/t.go", Hash: "sha256:x", Span: Span{StartLine: 1}},
		},
		{
			ID: "A2", Title: "Partial", Type: "term", Domain: "d",
			BusinessRule: "source: REF-002",
			SourceBlock:  Block{ID: "b2", Text: "source: REF-002", Hash: "sha256:y", Span: Span{StartLine: 5}},
		},
		{
			ID: "A3", Title: "Missing", Type: "exception",
			BusinessRule: "Nothing here",
			SourceBlock:  Block{},
		},
	}
	matrix := ProjectTraceMatrix(atoms, "test-domain")

	if matrix.TotalRows != 3 {
		t.Fatalf("expected 3 rows, got %d", matrix.TotalRows)
	}
	if matrix.Covered != 1 {
		t.Fatalf("expected 1 covered, got %d", matrix.Covered)
	}
	if matrix.Partial != 1 {
		t.Fatalf("expected 1 partial, got %d", matrix.Partial)
	}
	if matrix.Missing != 1 {
		t.Fatalf("expected 1 missing, got %d", matrix.Missing)
	}
	if matrix.Domain != "test-domain" {
		t.Fatalf("expected domain test-domain, got %s", matrix.Domain)
	}
	if matrix.SchemaVersion != "0.1.0" {
		t.Fatalf("expected schema version 0.1.0, got %s", matrix.SchemaVersion)
	}
}

func TestProjectTraceMatrix_EmptyAtoms(t *testing.T) {
	matrix := ProjectTraceMatrix(nil, "empty")
	if matrix.TotalRows != 0 {
		t.Fatalf("expected 0 rows, got %d", matrix.TotalRows)
	}
}

func TestProjectTraceMatrix_DomainFallback(t *testing.T) {
	atoms := []Atom{
		{
			ID: "A1", Title: "No domain atom", Type: "rule",
			BusinessRule: "source: REF-001",
			SourceBlock:  Block{ID: "b1", Text: "source: REF-001", Hash: "sha256:z", Span: Span{StartLine: 1}},
		},
	}
	matrix := ProjectTraceMatrix(atoms, "fallback-domain")

	if matrix.Rows[0].Domain != "fallback-domain" {
		t.Fatalf("expected fallback domain, got %s", matrix.Rows[0].Domain)
	}
}

func TestProjectTraceMatrix_GapMessages(t *testing.T) {
	atoms := []Atom{
		{
			ID: "A1", Title: "No refs", Type: "rule",
			BusinessRule: "Plain text without any references",
			SourceBlock:  Block{},
		},
	}
	matrix := ProjectTraceMatrix(atoms, "d")

	gaps := matrix.Rows[0].Gaps
	hasSource, hasCode, hasTest := false, false, false
	for _, g := range gaps {
		if g == "no source reference" {
			hasSource = true
		}
		if g == "no code reference" {
			hasCode = true
		}
		if g == "no test reference" {
			hasTest = true
		}
	}
	if !hasSource || !hasCode || !hasTest {
		t.Fatalf("expected all 3 gap types, got %v", gaps)
	}
}

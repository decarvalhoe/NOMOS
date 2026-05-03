package fidelity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRegister() LicensedDocRegister {
	return LicensedDocRegister{
		SchemaVersion: "0.1.0",
		Documents: []RegisterEntry{
			{ID: "REF-001", Title: "Code des assurances", Publisher: "Légifrance",
				Class: ClassOriginal, Status: StatusVerified, Hash: "sha256:orig001",
				License: "public", Retention: "permanent"},
			{ID: "REF-002", Title: "eCFR Mirror", Publisher: "eCFR",
				Class: ClassSurrogate, Status: StatusSurrogate, Hash: "sha256:surr002",
				License: "public"},
			{ID: "REF-003", Title: "Summary of Art. L113-2", Publisher: "Internal",
				Class: ClassDerivedDoc, Status: StatusDerivedDoc, Hash: "sha256:deriv003",
				OriginalRef: "REF-001"},
			{ID: "REF-004", Title: "Blocked Document", Publisher: "Unknown",
				Class: ClassUnverified, Status: StatusBlocked, Hash: "sha256:block004"},
		},
	}
}

// --- Classification by hash ---

func TestClassifyOriginal(t *testing.T) {
	r := ClassifyDocument("sha256:orig001", testRegister())
	if r.Class != ClassOriginal {
		t.Fatalf("expected original, got %q", r.Class)
	}
	if !r.HashMatch {
		t.Fatal("expected hash match")
	}
	if r.Blocked {
		t.Fatal("original should not be blocked")
	}
	assertContainsStr(t, r.AllowedUses, "citation_external")
	assertContainsStr(t, r.AllowedUses, "golden_case")
}

func TestClassifySurrogate(t *testing.T) {
	r := ClassifyDocument("sha256:surr002", testRegister())
	if r.Class != ClassSurrogate {
		t.Fatalf("expected surrogate, got %q", r.Class)
	}
	assertContainsStr(t, r.AllowedUses, "citation_internal")
	assertNotContainsStr(t, r.AllowedUses, "citation_external")
}

func TestClassifyDerived(t *testing.T) {
	r := ClassifyDocument("sha256:deriv003", testRegister())
	if r.Class != ClassDerivedDoc {
		t.Fatalf("expected derived, got %q", r.Class)
	}
	if len(r.AllowedUses) != 1 || r.AllowedUses[0] != "citation_internal" {
		t.Fatalf("expected only citation_internal, got %v", r.AllowedUses)
	}
}

func TestClassifyBlocked(t *testing.T) {
	r := ClassifyDocument("sha256:block004", testRegister())
	if !r.Blocked {
		t.Fatal("expected blocked")
	}
}

func TestClassifyUnknownHash(t *testing.T) {
	r := ClassifyDocument("sha256:unknown", testRegister())
	if r.Class != ClassUnverified {
		t.Fatalf("expected unverified, got %q", r.Class)
	}
	if !r.Blocked {
		t.Fatal("unverified should be blocked")
	}
	if r.HashMatch {
		t.Fatal("expected no hash match")
	}
	if len(r.AllowedUses) != 0 {
		t.Fatalf("expected no allowed uses, got %v", r.AllowedUses)
	}
}

// --- Classification by ID ---

func TestClassifyByIDFound(t *testing.T) {
	r := ClassifyByID("REF-001", testRegister())
	if r.Class != ClassOriginal {
		t.Fatalf("expected original, got %q", r.Class)
	}
	if r.EntryID != "REF-001" {
		t.Fatalf("expected REF-001, got %q", r.EntryID)
	}
}

func TestClassifyByIDNotFound(t *testing.T) {
	r := ClassifyByID("NONEXISTENT", testRegister())
	if r.Class != ClassUnverified {
		t.Fatalf("expected unverified, got %q", r.Class)
	}
	if !r.Blocked {
		t.Fatal("expected blocked")
	}
}

// --- Register evaluation ---

func TestEvaluateRegisterCounts(t *testing.T) {
	result := EvaluateRegister(testRegister())
	if result.TotalDocs != 4 {
		t.Fatalf("expected 4, got %d", result.TotalDocs)
	}
	if result.Original != 1 {
		t.Fatalf("expected 1 original, got %d", result.Original)
	}
	if result.Surrogate != 1 {
		t.Fatalf("expected 1 surrogate, got %d", result.Surrogate)
	}
	if result.Derived != 1 {
		t.Fatalf("expected 1 derived, got %d", result.Derived)
	}
	if result.Blocked != 1 {
		t.Fatalf("expected 1 blocked, got %d", result.Blocked)
	}
}

func TestEvaluateRegisterClean(t *testing.T) {
	reg := LicensedDocRegister{Documents: []RegisterEntry{
		{ID: "R1", Title: "Doc", Class: ClassOriginal, Status: StatusVerified, Hash: "sha256:x"},
	}}
	result := EvaluateRegister(reg)
	if len(result.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %v", len(result.Findings), result.Findings)
	}
}

func TestEvaluateRegisterMissingHash(t *testing.T) {
	reg := LicensedDocRegister{Documents: []RegisterEntry{
		{ID: "R1", Title: "Doc", Class: ClassOriginal, Status: StatusVerified, Hash: ""},
	}}
	result := EvaluateRegister(reg)
	assertFindingCode(t, result.Findings, "REF_NO_HASH")
}

func TestEvaluateRegisterMissingTitle(t *testing.T) {
	reg := LicensedDocRegister{Documents: []RegisterEntry{
		{ID: "R1", Title: "", Class: ClassOriginal, Status: StatusVerified, Hash: "sha256:x"},
	}}
	result := EvaluateRegister(reg)
	assertFindingCode(t, result.Findings, "REF_NO_TITLE")
}

func TestEvaluateRegisterDerivedNoOriginal(t *testing.T) {
	reg := LicensedDocRegister{Documents: []RegisterEntry{
		{ID: "R1", Title: "Derived", Class: ClassDerivedDoc, Status: StatusDerivedDoc, Hash: "sha256:x", OriginalRef: ""},
	}}
	result := EvaluateRegister(reg)
	assertFindingCode(t, result.Findings, "REF_DERIVED_NO_ORIGINAL")
}

func TestEvaluateRegisterUnverifiedStatus(t *testing.T) {
	reg := LicensedDocRegister{Documents: []RegisterEntry{
		{ID: "R1", Title: "Unverified", Class: ClassUnverified, Status: StatusUnverified, Hash: "sha256:x"},
	}}
	result := EvaluateRegister(reg)
	assertFindingCode(t, result.Findings, "REF_UNVERIFIED")
}

// --- YAML parsing ---

func TestParseRegister(t *testing.T) {
	data := []byte(`
schema_version: "0.1.0"
documents:
  - id: R1
    title: Test Doc
    class: original
    status: verified
    hash: "sha256:abc"
    license: public
`)
	reg, err := ParseRegister(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Documents) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(reg.Documents))
	}
	if reg.Documents[0].Class != ClassOriginal {
		t.Fatalf("expected original, got %q", reg.Documents[0].Class)
	}
}

func TestParseRegisterInvalid(t *testing.T) {
	_, err := ParseRegister([]byte(`{broken`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadRegisterFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "register.yaml")
	os.WriteFile(path, []byte("schema_version: '0.1.0'\ndocuments:\n  - id: R1\n    title: T\n    class: original\n    status: verified\n    hash: sha256:x\n"), 0o644)
	reg, err := LoadRegister(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Documents) != 1 {
		t.Fatal("expected 1")
	}
}

func TestLoadRegisterMissing(t *testing.T) {
	_, err := LoadRegister("/nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Hash file ---

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	os.WriteFile(path, []byte("test content"), 0o644)
	h, err := HashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "sha256:") {
		t.Fatalf("expected sha256 prefix, got %q", h)
	}
}

func TestHashFileDeterministic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	os.WriteFile(path, []byte("stable"), 0o644)
	h1, _ := HashFile(path)
	h2, _ := HashFile(path)
	if h1 != h2 {
		t.Fatal("hash not deterministic")
	}
}

// --- JSON ---

func TestRefPolicyJSON(t *testing.T) {
	result := EvaluateRegister(testRegister())
	var buf bytes.Buffer
	if err := WriteRefPolicyJSON(&buf, result); err != nil {
		t.Fatal(err)
	}
	var decoded RefPolicyResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.TotalDocs != result.TotalDocs {
		t.Fatal("roundtrip mismatch")
	}
}

// --- Constants ---

func TestDocClassConstants(t *testing.T) {
	if ClassOriginal != "original" || ClassSurrogate != "surrogate" ||
		ClassDerivedDoc != "derived" || ClassUnverified != "unverified" {
		t.Fatal("class constants wrong")
	}
}

func TestDocStatusConstants(t *testing.T) {
	if StatusVerified != "verified" || StatusBlocked != "blocked" {
		t.Fatal("status constants wrong")
	}
}

// --- helpers ---

func assertContainsStr(t *testing.T, slice []string, want string) {
	t.Helper()
	for _, s := range slice {
		if s == want {
			return
		}
	}
	t.Fatalf("expected %q in %v", want, slice)
}

func assertNotContainsStr(t *testing.T, slice []string, notWant string) {
	t.Helper()
	for _, s := range slice {
		if s == notWant {
			t.Fatalf("did not expect %q in %v", notWant, slice)
		}
	}
}

func assertFindingCode(t *testing.T, findings []RefPolicyFinding, code string) {
	t.Helper()
	for _, f := range findings {
		if f.Code == code {
			return
		}
	}
	var codes []string
	for _, f := range findings {
		codes = append(codes, f.Code)
	}
	t.Fatalf("expected finding %q in %v", code, codes)
}

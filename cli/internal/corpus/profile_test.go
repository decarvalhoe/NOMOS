package corpus

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- Profile registry ---

func TestLookupProfileRBOKLawbook(t *testing.T) {
	p, err := LookupProfile("rbok-lawbook")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	assertEqual(t, ProfileRBOKLawbook, p.Name)
	if len(p.Outputs) != 4 {
		t.Fatalf("expected 4 outputs, got %d", len(p.Outputs))
	}
}

func TestLookupProfileCaseInsensitive(t *testing.T) {
	_, err := LookupProfile("RBOK-Lawbook")
	if err != nil {
		t.Fatalf("expected case-insensitive lookup, got: %v", err)
	}
}

func TestLookupProfileUnknown(t *testing.T) {
	_, err := LookupProfile("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestKnownProfiles(t *testing.T) {
	names := KnownProfiles()
	if len(names) == 0 {
		t.Fatal("expected at least one known profile")
	}
	found := false
	for _, n := range names {
		if n == ProfileRBOKLawbook {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %q in known profiles", ProfileRBOKLawbook)
	}
}

// --- Profile feed ---

func TestRunProfileFeedRBOKLawbook(t *testing.T) {
	dir := makeTestCorpus(t)

	result, err := RunProfileFeed(ProfileFeedInput{
		Profile:    ProfileRBOKLawbook,
		CorpusRoot: dir,
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertEqual(t, ProfileRBOKLawbook, result.Profile)
	if result.SourceCount == 0 {
		t.Fatal("expected sources")
	}
	if len(result.Sections) != 4 {
		t.Fatalf("expected 4 sections, got %d", len(result.Sections))
	}
	for _, flag := range allOutputFlags {
		if _, ok := result.Sections[flag]; !ok {
			t.Fatalf("missing section %q", flag)
		}
	}
}

func TestRunProfileFeedSelectedOutputs(t *testing.T) {
	dir := makeTestCorpus(t)

	result, err := RunProfileFeed(ProfileFeedInput{
		Profile:    ProfileRBOKLawbook,
		CorpusRoot: dir,
		Outputs:    []OutputFlag{OutputIndex, OutputGovernance},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(result.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(result.Sections))
	}
}

func TestRunProfileFeedUnknownProfile(t *testing.T) {
	_, err := RunProfileFeed(ProfileFeedInput{
		Profile:    "bogus",
		CorpusRoot: t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestRunProfileFeedEmptyCorpus(t *testing.T) {
	result, err := RunProfileFeed(ProfileFeedInput{
		Profile:    ProfileRBOKLawbook,
		CorpusRoot: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	assertEqual(t, 0, result.SourceCount)
}

func TestIndexSectionExcludesOutOfScope(t *testing.T) {
	dir := makeTestCorpus(t)

	result, err := RunProfileFeed(ProfileFeedInput{
		Profile:    ProfileRBOKLawbook,
		CorpusRoot: dir,
		Outputs:    []OutputFlag{OutputIndex},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	var entries []IndexEntry
	if err := json.Unmarshal(result.Sections[OutputIndex], &entries); err != nil {
		t.Fatalf("unmarshal index: %v", err)
	}
	for _, e := range entries {
		if e.Role == RoleOutOfScope {
			t.Fatalf("index should not contain out_of_scope entry: %s", e.Path)
		}
	}
}

func TestGovernanceSectionIncludesAll(t *testing.T) {
	dir := makeTestCorpus(t)

	result, err := RunProfileFeed(ProfileFeedInput{
		Profile:    ProfileRBOKLawbook,
		CorpusRoot: dir,
		Outputs:    []OutputFlag{OutputGovernance},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	var entries []GovernanceEntry
	if err := json.Unmarshal(result.Sections[OutputGovernance], &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != result.SourceCount {
		t.Fatalf("governance should include all %d sources, got %d", result.SourceCount, len(entries))
	}
}

func TestWriteProfileFeedJSON(t *testing.T) {
	dir := makeTestCorpus(t)
	result, _ := RunProfileFeed(ProfileFeedInput{
		Profile:    ProfileRBOKLawbook,
		CorpusRoot: dir,
	})

	var buf bytes.Buffer
	if err := WriteProfileFeedJSON(&buf, result); err != nil {
		t.Fatalf("write: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("empty output")
	}

	var decoded ProfileFeedResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertEqual(t, result.Profile, decoded.Profile)
}

func TestImportSectionOnlyImportable(t *testing.T) {
	dir := makeTestCorpus(t)

	result, err := RunProfileFeed(ProfileFeedInput{
		Profile:    ProfileRBOKLawbook,
		CorpusRoot: dir,
		Outputs:    []OutputFlag{OutputImport},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	var entries []IndexEntry
	if err := json.Unmarshal(result.Sections[OutputImport], &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Only primary/secondary should be importable (structured_contract or vector_index).
	for _, e := range entries {
		if e.Priority == "out_of_scope" || e.Priority == "reference" {
			t.Fatalf("import section should not contain %s priority: %s", e.Priority, e.Path)
		}
	}
}

// --- Diagnose ---

func TestDiagnoseProfileInScope(t *testing.T) {
	dir := makeTestCorpus(t)

	v, err := DiagnoseProfile(ProfileRBOKLawbook, dir)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	assertEqual(t, ProfileRBOKLawbook, v.Profile)
	assertEqual(t, "in_scope", v.Verdict)
	assertEqual(t, "high", v.Confidence)
	if len(v.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %v", v.Blockers)
	}
}

func TestDiagnoseProfileBlockedNoPrimary(t *testing.T) {
	dir := t.TempDir()
	writeTestCorpusFile(t, dir, "99_RBOK_initial_pdf/doc.pdf", []byte("%PDF-1.4"))

	v, err := DiagnoseProfile(ProfileRBOKLawbook, dir)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	assertEqual(t, "blocked", v.Verdict)
	if len(v.Blockers) == 0 {
		t.Fatal("expected blockers for missing primary")
	}
}

func TestDiagnoseProfileBlockedBinary(t *testing.T) {
	dir := makeTestCorpus(t)
	bin := make([]byte, 64)
	bin[0] = 0x00
	writeTestCorpusFile(t, dir, "00_meta/firmware.bin", bin)

	v, err := DiagnoseProfile(ProfileRBOKLawbook, dir)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	assertEqual(t, "blocked", v.Verdict)
}

func TestDiagnoseProfileAllowsDeclaredReferenceDerivedAndOutOfScopeBinaries(t *testing.T) {
	dir := makeTestCorpus(t)
	bin := make([]byte, 64)
	bin[0] = 0x00
	writeTestCorpusFile(t, dir, "99_RBOK_initial_pdf/RBOK.docx", bin)
	writeTestCorpusFile(t, dir, "03_parcours/generated/workbooks/parcours.docx", bin)
	writeTestCorpusFile(t, dir, "98_schémas/Archive/realisons-architecture-parcours.docx", bin)
	writeTestCorpusFile(t, dir, ".DS_Store", bin)

	v, err := DiagnoseProfile(ProfileRBOKLawbook, dir)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	if v.Verdict == "blocked" {
		t.Fatalf("declared non-lawbook binaries must not block admission: %+v", v)
	}
	for _, blocker := range v.Blockers {
		if contains(blocker, "blocked binary") {
			t.Fatalf("unexpected binary blocker for declared non-lawbook binary: %v", v.Blockers)
		}
	}
}

func TestDiagnoseProfilePartialNoReference(t *testing.T) {
	dir := t.TempDir()
	writeTestCorpusFile(t, dir, "00_meta/glossary.yaml", []byte("terms: []\n"))
	writeTestCorpusFile(t, dir, "02_domaines/garanties.de.yaml", []byte("translated: true\n"))

	v, err := DiagnoseProfile(ProfileRBOKLawbook, dir)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	assertEqual(t, "partial", v.Verdict)
}

func TestDiagnoseProfilePartialPrimaryOnly(t *testing.T) {
	dir := t.TempDir()
	writeTestCorpusFile(t, dir, "01_referentiel/manifest.yaml", []byte("sources: []\n"))

	v, err := DiagnoseProfile(ProfileRBOKLawbook, dir)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	assertEqual(t, "partial", v.Verdict)
	assertEqual(t, "medium", v.Confidence)
}

func TestDiagnoseProfileEmptyCorpus(t *testing.T) {
	v, err := DiagnoseProfile(ProfileRBOKLawbook, t.TempDir())
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	assertEqual(t, "blocked", v.Verdict)
}

func TestDiagnoseProfileUnknown(t *testing.T) {
	_, err := DiagnoseProfile("fake-profile", t.TempDir())
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestDiagnoseProfileOutOfScopeWarning(t *testing.T) {
	dir := makeTestCorpus(t)
	writeTestCorpusFile(t, dir, "scripts/build.sh", []byte("#!/bin/bash\n"))

	v, err := DiagnoseProfile(ProfileRBOKLawbook, dir)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	found := false
	for _, w := range v.Warnings {
		if contains(w, "out-of-scope") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected out-of-scope warning, got %v", v.Warnings)
	}
}

// --- helpers ---

func makeTestCorpus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestCorpusFile(t, dir, "00_meta/glossary.yaml", []byte("terms:\n  - id: T1\n    label: Term One\n"))
	writeTestCorpusFile(t, dir, "01_referentiel/manifest.yaml", []byte("sources: []\n"))
	writeTestCorpusFile(t, dir, "02_domaines/habitation/garanties.yaml", []byte("garanties:\n  - water_damage\n"))
	writeTestCorpusFile(t, dir, "99_RBOK_initial_pdf/contract.pdf", []byte("%PDF-1.4 original"))
	return dir
}

func writeTestCorpusFile(t *testing.T, root, rel string, content []byte) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsLower(s, substr)
}

func containsLower(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

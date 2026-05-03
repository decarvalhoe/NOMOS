package corpus

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Profile registry ---

func TestLookupProfileRBOKLawbook(t *testing.T) {
	p, err := LookupProfile("rbok-lawbook")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	assertEqual(t, ProfileRBOKLawbook, p.Name)
	if len(p.Outputs) != 8 {
		t.Fatalf("expected 8 outputs, got %d", len(p.Outputs))
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
	if result.UnitCount == 0 {
		t.Fatal("expected atomized units")
	}
	if len(result.Sections) != len(allOutputFlags) {
		t.Fatalf("expected %d sections, got %d", len(allOutputFlags), len(result.Sections))
	}
	for _, flag := range allOutputFlags {
		if _, ok := result.Sections[flag]; !ok {
			t.Fatalf("missing section %q", flag)
		}
	}
}

func TestRunProfileFeedBuildsLawbookArtifacts(t *testing.T) {
	dir := makeRealisonsBusinessFixture(t)

	result, err := RunProfileFeed(ProfileFeedInput{
		Profile:    ProfileRBOKLawbook,
		CorpusRoot: dir,
		Outputs:    []OutputFlag{OutputFeed, OutputRAGMetadata, OutputAtomizationReport},
	})
	if err != nil {
		t.Fatalf("profile feed: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %v", result.Errors)
	}
	if result.UnitCount == 0 {
		t.Fatal("expected atomized units")
	}

	var assembly MultiFeedAssembly
	if err := json.Unmarshal(result.Sections[OutputFeed], &assembly); err != nil {
		t.Fatalf("unmarshal feed assembly: %v", err)
	}
	if assembly.TotalNodes != result.UnitCount {
		t.Fatalf("assembly total_nodes=%d, result unit_count=%d", assembly.TotalNodes, result.UnitCount)
	}
	if assembly.DocumentCount < 2 {
		t.Fatalf("expected markdown and parcours documents, got %d", assembly.DocumentCount)
	}
	if len(assembly.RAGMetadata) != result.UnitCount {
		t.Fatalf("expected one RAG chunk per node, got chunks=%d units=%d", len(assembly.RAGMetadata), result.UnitCount)
	}

	hasCanonical := false
	hasRuntime := false
	for _, feed := range assembly.Feeds {
		for _, node := range feed.Nodes {
			if node.SourceHash == "" || !strings.HasPrefix(node.SourceHash, "sha256:") {
				t.Fatalf("node %s missing source hash", node.NodeID)
			}
			if node.Locator == "" {
				t.Fatalf("node %s missing locator", node.NodeID)
			}
			switch node.SourceClass {
			case "canonical_corpus":
				hasCanonical = true
			case "runtime_binding":
				hasRuntime = true
			}
		}
	}
	if !hasCanonical {
		t.Fatal("expected canonical corpus nodes")
	}
	if !hasRuntime {
		t.Fatal("expected runtime binding nodes")
	}

	var report ProfileAtomizationReport
	if err := json.Unmarshal(result.Sections[OutputAtomizationReport], &report); err != nil {
		t.Fatalf("unmarshal atomization report: %v", err)
	}
	if report.TotalNodes != result.UnitCount {
		t.Fatalf("report total_nodes=%d, result unit_count=%d", report.TotalNodes, result.UnitCount)
	}
	if report.MissingSourceHash != 0 || report.MissingLocator != 0 {
		t.Fatalf("expected complete traceability, got missing_hash=%d missing_locator=%d", report.MissingSourceHash, report.MissingLocator)
	}
}

func TestRunProfileFeedTraceabilityMatrixCoversEveryNode(t *testing.T) {
	dir := makeRealisonsBusinessFixture(t)

	result, err := RunProfileFeed(ProfileFeedInput{
		Profile:    ProfileRBOKLawbook,
		CorpusRoot: dir,
		Outputs:    []OutputFlag{OutputTraceabilityMatrix},
	})
	if err != nil {
		t.Fatalf("profile feed: %v", err)
	}

	var matrix []TraceabilityEntry
	if err := json.Unmarshal(result.Sections[OutputTraceabilityMatrix], &matrix); err != nil {
		t.Fatalf("unmarshal traceability matrix: %v", err)
	}
	if len(matrix) != result.UnitCount {
		t.Fatalf("expected %d traceability rows, got %d", result.UnitCount, len(matrix))
	}
	for _, row := range matrix {
		if row.NodeID == "" || row.CanonicalRef == "" || row.SourcePath == "" {
			t.Fatalf("incomplete traceability row: %+v", row)
		}
		if !strings.HasPrefix(row.SourceHash, "sha256:") {
			t.Fatalf("row %s has invalid source hash %q", row.NodeID, row.SourceHash)
		}
		if row.SourceClass == "" || row.CorpusLayer == "" || row.Authority == "" {
			t.Fatalf("row %s missing governance classification: %+v", row.NodeID, row)
		}
	}
}

func TestRunProfileFeedTraceabilityOrderIsDeterministic(t *testing.T) {
	dir := makeRealisonsBusinessFixture(t)

	first, err := RunProfileFeed(ProfileFeedInput{
		Profile:    ProfileRBOKLawbook,
		CorpusRoot: dir,
		Outputs:    []OutputFlag{OutputTraceabilityMatrix},
	})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := RunProfileFeed(ProfileFeedInput{
		Profile:    ProfileRBOKLawbook,
		CorpusRoot: dir,
		Outputs:    []OutputFlag{OutputTraceabilityMatrix},
	})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if !bytes.Equal(first.Sections[OutputTraceabilityMatrix], second.Sections[OutputTraceabilityMatrix]) {
		t.Fatal("traceability matrix changed between identical runs")
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

func TestRunProfileFeedAdmittedBinariesAreWarnings(t *testing.T) {
	dir := makeTestCorpus(t)
	bin := make([]byte, 64)
	bin[0] = 0x00
	writeTestCorpusFile(t, dir, "01_rbok/99_RBOK_initial_pdf/source.docx", bin)

	result, err := RunProfileFeed(ProfileFeedInput{
		Profile:    ProfileRBOKLawbook,
		CorpusRoot: dir,
	})
	if err != nil {
		t.Fatalf("profile feed: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no blocking errors, got %v", result.Errors)
	}
	found := false
	for _, warning := range result.Warnings {
		if strings.Contains(warning, "non-atomized binary") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected non-atomized binary warning, got %v", result.Warnings)
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
	assertEqual(t, "corpus_admissible", v.Verdict)
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
	assertEqual(t, "corpus_blocked", v.Verdict)
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
	assertEqual(t, "corpus_blocked", v.Verdict)
}

func TestDiagnoseProfileDoesNotBlockAdmittedReferenceBinaries(t *testing.T) {
	dir := makeTestCorpus(t)
	bin := make([]byte, 64)
	bin[0] = 0x00
	writeTestCorpusFile(t, dir, "01_rbok/99_RBOK_initial_pdf/source.docx", bin)
	writeTestCorpusFile(t, dir, "04_marketing/logo/logo.png", bin)
	writeTestCorpusFile(t, dir, "99_archive/old/source.docx", bin)

	v, err := DiagnoseProfile(ProfileRBOKLawbook, dir)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	assertEqual(t, "corpus_admissible", v.Verdict)
	for _, blocker := range v.Blockers {
		if strings.Contains(blocker, "source.docx") || strings.Contains(blocker, "logo.png") {
			t.Fatalf("expected admitted/reference binaries not to block, got blockers %v", v.Blockers)
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
	assertEqual(t, "corpus_partial", v.Verdict)
}

func TestDiagnoseProfilePartialPrimaryOnly(t *testing.T) {
	dir := t.TempDir()
	writeTestCorpusFile(t, dir, "01_referentiel/manifest.yaml", []byte("sources: []\n"))

	v, err := DiagnoseProfile(ProfileRBOKLawbook, dir)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	assertEqual(t, "corpus_partial", v.Verdict)
	assertEqual(t, "medium", v.Confidence)
}

func TestDiagnoseProfileEmptyCorpus(t *testing.T) {
	v, err := DiagnoseProfile(ProfileRBOKLawbook, t.TempDir())
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	assertEqual(t, "corpus_blocked", v.Verdict)
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

func TestDiagnoseProfileRealisonsBusinessRootLayout(t *testing.T) {
	dir := t.TempDir()
	writeTestCorpusFile(t, dir, "01_rbok/00_meta/RBOK_structure_v1.md", []byte("# RBOK\n\nCore doctrine.\n"))
	writeTestCorpusFile(t, dir, "01_rbok/03_parcours/PAR_ACC_ADMIN.yaml", []byte("id: PAR_ACC_ADMIN\nmodules: []\n"))
	writeTestCorpusFile(t, dir, "01_rbok/99_RBOK_initial_pdf/source.pdf", []byte("%PDF-1.4 original"))
	writeTestCorpusFile(t, dir, "02_organisation/equipe.md", []byte("# Equipe\n\nSupport.\n"))
	writeTestCorpusFile(t, dir, "99_archive/old.md", []byte("# Old\n\nArchived.\n"))

	v, err := DiagnoseProfile(ProfileRBOKLawbook, dir)
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	assertEqual(t, "corpus_admissible", v.Verdict)
	assertEqual(t, "high", v.Confidence)
	if len(v.Blockers) != 0 {
		t.Fatalf("expected no blockers, got %v", v.Blockers)
	}
	if !strings.Contains(v.Summary, "runtime") {
		t.Fatalf("expected runtime binding summary, got %q", v.Summary)
	}
}

// --- helpers ---

func makeTestCorpus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestCorpusFile(t, dir, "00_meta/glossary.md", []byte("# Glossary\n\nTerm One governs the corpus.\n"))
	writeTestCorpusFile(t, dir, "01_referentiel/manifest.yaml", []byte("sources: []\n"))
	writeTestCorpusFile(t, dir, "02_domaines/habitation/garanties.yaml", []byte("garanties:\n  - water_damage\n"))
	writeTestCorpusFile(t, dir, "99_RBOK_initial_pdf/contract.pdf", []byte("%PDF-1.4 original"))
	return dir
}

func makeRealisonsBusinessFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTestCorpusFile(t, dir, "01_rbok/00_meta/RBOK_structure_v1.md", []byte(`# RBOK Structure

| Champ | Valeur |
|---|---|
| Reference | RBOK-STRUCT-001 |
| Status | active |
| Version | 1.0 |
| Domaine | rbok |

Core doctrine.

## Principes

- Git est la source canonique.
- Nomos produit le feed vivant.
`))
	writeTestCorpusFile(t, dir, "01_rbok/03_parcours/PAR_ACC_ADMIN.yaml", []byte(`
parcours:
  code: PAR_ACC_ADMIN
  name: Les bases administratives
  domain: rbok
  version: "1.0"
  owner: rbok@example.com
  status: active
  modules:
    - code: MOD_ADMIN_STATUT
      name: Statut et structure
      type: conversational
      ai_instructions: Poser uniquement la question du step courant.
      source_rbok: RBOK-STRUCT-001
      objectives:
        - key: statut-juridique
          titre: Statut et inscriptions
          questions:
            - key: statut-choisi
              label: Quel statut juridique avez-vous choisi ?
              type: select
              help_text: Question concise du step courant.
`))
	writeTestCorpusFile(t, dir, "01_rbok/99_RBOK_initial_pdf/source.pdf", []byte("%PDF-1.4 original"))
	writeTestCorpusFile(t, dir, "02_organisation/equipe.md", []byte("# Equipe\n\nSupport context.\n"))
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

package corpus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupArtifactPackCorpus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Create a minimal RBOK lawbook source with definitions.
	content := `# Reglement Test

| Champ | Valeur |
|-------|--------|
| Reference | REF-TEST-001 |
| Statut | En vigueur |

## Chapitre 1 - Definitions

Franchise : montant restant a la charge de l'assure apres remboursement par la mutuelle.

On entend par sinistre tout evenement dommageable couvert par le contrat d'assurance auto.

## Chapitre 2 - Obligations

L'assure doit declarer tout sinistre dans un delai de 5 jours ouvrables.

Le non-respect de ce delai peut entrainer la decheance de garantie.
`
	// Write under 01_referentiel/ to match RBOK classification.
	refDir := filepath.Join(dir, "01_referentiel")
	os.MkdirAll(refDir, 0o755)
	os.WriteFile(filepath.Join(refDir, "reglement-test.md"), []byte(content), 0o644)
	return dir
}

func TestWriteRBOKLawbookArtifactPack_ProducesGovernedLexicon(t *testing.T) {
	corpusDir := setupArtifactPackCorpus(t)
	outDir := t.TempDir()

	result, err := WriteRBOKLawbookArtifactPack(corpusDir, outDir, RBOKLawbookArtifactPackOptions{})
	if err != nil {
		t.Fatalf("WriteRBOKLawbookArtifactPack: %v", err)
	}

	// Check governed lexicon is in the artifact list.
	found := false
	for _, a := range result.Artifacts {
		if a == "rbok-governed-lexicon.yaml" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected rbok-governed-lexicon.yaml in artifacts, got %v", result.Artifacts)
	}

	// Check file exists and is non-empty.
	lexPath := filepath.Join(outDir, "rbok-governed-lexicon.yaml")
	info, err := os.Stat(lexPath)
	if err != nil {
		t.Fatalf("rbok-governed-lexicon.yaml not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("rbok-governed-lexicon.yaml is empty")
	}

	// Verify content has defined terms.
	data, _ := os.ReadFile(lexPath)
	content := string(data)
	if !strings.Contains(content, "defined") {
		t.Fatal("expected 'defined' status in governed lexicon")
	}
	if !strings.Contains(content, "schema_version") {
		t.Fatal("expected schema_version in governed lexicon")
	}
}

func TestWriteRBOKLawbookArtifactPack_AllExpectedArtifacts(t *testing.T) {
	corpusDir := setupArtifactPackCorpus(t)
	outDir := t.TempDir()

	result, err := WriteRBOKLawbookArtifactPack(corpusDir, outDir, RBOKLawbookArtifactPackOptions{})
	if err != nil {
		t.Fatalf("WriteRBOKLawbookArtifactPack: %v", err)
	}

	expected := []string{
		"rbok-lawbook-feed.json",
		"rbok-lawbook-index.json",
		"rbok-rag-metadata.json",
		"rbok-engine-import.json",
		"rbok-governance.json",
		"rbok-attestation.json",
		"rbok-governed-lexicon.yaml",
		"short-critical-atoms.json",
	}
	for _, name := range expected {
		path := filepath.Join(outDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing artifact: %s", name)
		}
	}
	data, err := os.ReadFile(filepath.Join(outDir, "short-critical-atoms.json"))
	if err != nil {
		t.Fatalf("read short-critical-atoms.json: %v", err)
	}
	var shortReport ShortCriticalAtomsReport
	if err := json.Unmarshal(data, &shortReport); err != nil {
		t.Fatalf("parse short-critical-atoms.json: %v", err)
	}
	if shortReport.Format != ShortCriticalAtomsFormat {
		t.Fatalf("short-critical-atoms format=%q", shortReport.Format)
	}
	if len(result.Artifacts) < len(expected) {
		t.Fatalf("expected at least %d artifacts, got %d", len(expected), len(result.Artifacts))
	}
}

func TestBuildGovernedLexiconFromFeeds_ExtractsDefinitions(t *testing.T) {
	feeds := []LawbookFeed{
		{
			Domain: "insurance",
			Nodes: []LawbookNode{
				{NodeID: "N-001", NodeType: NodeParagraph, Text: "Franchise : montant restant a la charge de l'assure apres le remboursement.", Domain: "insurance"},
				{NodeID: "N-002", NodeType: NodeParagraph, Text: "On entend par sinistre tout evenement dommageable couvert par le contrat.", Domain: "insurance"},
				{NodeID: "N-003", NodeType: NodeParagraph, Text: "L'assure doit declarer le sinistre.", Domain: "insurance"},
			},
		},
	}
	lex := buildGovernedLexiconFromFeeds(feeds)

	if lex.TotalDefined < 1 {
		t.Fatalf("expected at least 1 defined term, got %d", lex.TotalDefined)
	}
	if lex.SchemaVersion != "0.1.0" {
		t.Fatalf("expected schema 0.1.0, got %s", lex.SchemaVersion)
	}

	foundFranchise := false
	for _, term := range lex.Terms {
		if strings.ToLower(term.Term) == "franchise" {
			foundFranchise = true
			if term.Status != "defined" {
				t.Fatalf("franchise status should be defined, got %s", term.Status)
			}
			if term.Definition == "" {
				t.Fatal("franchise should have a definition")
			}
		}
	}
	if !foundFranchise {
		t.Fatal("expected Franchise in governed lexicon")
	}
}

func TestBuildGovernedLexiconFromFeeds_EmptyFeeds(t *testing.T) {
	lex := buildGovernedLexiconFromFeeds(nil)
	if lex.TotalDefined != 0 {
		t.Fatalf("expected 0 defined, got %d", lex.TotalDefined)
	}
}

func TestBuildGovernedLexiconFromFeeds_Dedup(t *testing.T) {
	feeds := []LawbookFeed{
		{
			Nodes: []LawbookNode{
				{NodeID: "N-1", NodeType: NodeParagraph, Text: "Franchise : definition dans le premier document source."},
				{NodeID: "N-2", NodeType: NodeParagraph, Text: "Franchise : definition dans le second document source."},
			},
		},
	}
	lex := buildGovernedLexiconFromFeeds(feeds)
	count := 0
	for _, t := range lex.Terms {
		if strings.ToLower(t.Term) == "franchise" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 franchise (deduped), got %d", count)
	}
}

func TestMarshalGovernedLexiconYAML(t *testing.T) {
	lex := governedLexiconArtifact{
		SchemaVersion: "0.1.0",
		Domain:        "test",
		TotalDefined:  1,
		Terms: []governedLexiconEntry{
			{Term: "Alpha", Definition: "first", Status: "defined"},
		},
	}
	data, err := marshalGovernedLexiconYAML(lex)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), "Alpha") {
		t.Fatal("expected term in YAML")
	}
}

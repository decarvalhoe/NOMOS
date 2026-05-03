package fidelity

import (
	"bytes"
	"strings"
	"testing"
)

func castWithDefinitions() CAST {
	return CAST{
		Root:       "root",
		SourceHash: "abc123",
		Nodes: []CNode{
			{ID: "h1", Kind: "heading", Text: "# Glossaire", RawText: "# Glossaire", Span: Span{StartLine: 1, EndLine: 1}},
			{ID: "p1", Kind: "paragraph", Text: "**Assuré**: Personne physique ou morale qui souscrit un contrat d'assurance.", RawText: "**Assuré**: Personne physique ou morale qui souscrit un contrat d'assurance.", Span: Span{StartLine: 3, EndLine: 3}},
			{ID: "p2", Kind: "paragraph", Text: "**Sinistre** — Événement dommageable donnant lieu à une déclaration.", RawText: "**Sinistre** — Événement dommageable donnant lieu à une déclaration.", Span: Span{StartLine: 5, EndLine: 5}},
			{ID: "p3", Kind: "paragraph", Text: "- **Franchise**: Montant restant à la charge de l'assuré.", RawText: "- **Franchise**: Montant restant à la charge de l'assuré.", Span: Span{StartLine: 7, EndLine: 7}},
		},
	}
}

func castOutsideGlossary() CAST {
	return CAST{
		Root:       "root",
		SourceHash: "def456",
		Nodes: []CNode{
			{ID: "h1", Kind: "heading", Text: "# Introduction", RawText: "# Introduction", Span: Span{StartLine: 1, EndLine: 1}},
			{ID: "p1", Kind: "paragraph", Text: "**Cotisation**: Somme versée par l'assuré en échange de la garantie.", RawText: "**Cotisation**: Somme versée par l'assuré en échange de la garantie.", Span: Span{StartLine: 3, EndLine: 3}},
		},
	}
}

func TestExtractLexiconFromGlossary(t *testing.T) {
	artifact := ExtractLexicon(castWithDefinitions(), ExtractionConfig{
		Domain:     "insurance",
		SourceHash: "abc123",
	})

	if artifact.Domain != "insurance" {
		t.Fatalf("expected insurance, got %s", artifact.Domain)
	}
	if artifact.TermCount < 3 {
		t.Fatalf("expected at least 3 terms, got %d", artifact.TermCount)
	}

	termMap := map[string]ExtractedTerm{}
	for _, term := range artifact.Terms {
		termMap[strings.ToLower(term.Term)] = term
	}

	if _, ok := termMap["assuré"]; !ok {
		t.Fatal("expected term 'Assuré'")
	}
	if _, ok := termMap["sinistre"]; !ok {
		t.Fatal("expected term 'Sinistre'")
	}
	if _, ok := termMap["franchise"]; !ok {
		t.Fatal("expected term 'Franchise'")
	}
}

func TestExtractLexiconHighConfidenceInGlossary(t *testing.T) {
	artifact := ExtractLexicon(castWithDefinitions(), ExtractionConfig{Domain: "test"})

	for _, term := range artifact.Terms {
		if term.Confidence != "high" {
			t.Fatalf("expected high confidence in glossary section, got %s for %s", term.Confidence, term.Term)
		}
	}
}

func TestExtractLexiconMediumConfidenceOutside(t *testing.T) {
	artifact := ExtractLexicon(castOutsideGlossary(), ExtractionConfig{Domain: "test"})

	if artifact.TermCount == 0 {
		t.Fatal("expected at least 1 term extracted")
	}
	for _, term := range artifact.Terms {
		if term.Confidence != "medium" {
			t.Fatalf("expected medium confidence outside glossary, got %s", term.Confidence)
		}
	}
}

func TestExtractLexiconDeduplicates(t *testing.T) {
	cast := CAST{
		Root: "root",
		Nodes: []CNode{
			{ID: "h1", Kind: "heading", Text: "# Glossary", RawText: "# Glossary"},
			{ID: "p1", Kind: "paragraph", Text: "**Term A**: First definition.", RawText: "**Term A**: First definition."},
			{ID: "p2", Kind: "paragraph", Text: "**Term A**: Second definition.", RawText: "**Term A**: Second definition."},
		},
	}

	artifact := ExtractLexicon(cast, ExtractionConfig{Domain: "test"})

	count := 0
	for _, t := range artifact.Terms {
		if strings.ToLower(t.Term) == "term a" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 deduplicated term, got %d", count)
	}
}

func TestExtractLexiconEmpty(t *testing.T) {
	cast := CAST{Root: "root", Nodes: []CNode{
		{ID: "h1", Kind: "heading", Text: "# Title", RawText: "# Title"},
		{ID: "p1", Kind: "paragraph", Text: "No definitions here.", RawText: "No definitions here."},
	}}

	artifact := ExtractLexicon(cast, ExtractionConfig{Domain: "test"})

	if artifact.TermCount != 0 {
		t.Fatalf("expected 0 terms, got %d", artifact.TermCount)
	}
}

func TestExtractLexiconSorted(t *testing.T) {
	cast := CAST{
		Root: "root",
		Nodes: []CNode{
			{ID: "h1", Kind: "heading", Text: "# Definitions", RawText: "# Definitions"},
			{ID: "p1", Kind: "paragraph", Text: "**Zeta**: Last term alphabetically.", RawText: "**Zeta**: Last term alphabetically."},
			{ID: "p2", Kind: "paragraph", Text: "**Alpha**: First term alphabetically.", RawText: "**Alpha**: First term alphabetically."},
		},
	}

	artifact := ExtractLexicon(cast, ExtractionConfig{Domain: "test"})

	if len(artifact.Terms) < 2 {
		t.Fatalf("expected 2 terms, got %d", len(artifact.Terms))
	}
	if strings.ToLower(artifact.Terms[0].Term) > strings.ToLower(artifact.Terms[1].Term) {
		t.Fatalf("expected sorted: %s before %s", artifact.Terms[0].Term, artifact.Terms[1].Term)
	}
}

func TestExtractLexiconColonPattern(t *testing.T) {
	cast := CAST{
		Root: "root",
		Nodes: []CNode{
			{ID: "p1", Kind: "paragraph", Text: "Garantie: Protection contractuelle contre un risque défini.", RawText: "Garantie: Protection contractuelle contre un risque défini.", Span: Span{StartLine: 5}},
		},
	}

	artifact := ExtractLexicon(cast, ExtractionConfig{Domain: "test"})

	if artifact.TermCount != 1 {
		t.Fatalf("expected 1 term from colon pattern, got %d", artifact.TermCount)
	}
	if artifact.Terms[0].Term != "Garantie" {
		t.Fatalf("expected 'Garantie', got %s", artifact.Terms[0].Term)
	}
}

func TestExtractLexiconPatternTracking(t *testing.T) {
	artifact := ExtractLexicon(castWithDefinitions(), ExtractionConfig{Domain: "test"})

	for _, term := range artifact.Terms {
		if len(term.Patterns) == 0 {
			t.Fatalf("expected patterns tracked for %s", term.Term)
		}
	}
}

func TestExtractLexiconLineNumber(t *testing.T) {
	artifact := ExtractLexicon(castWithDefinitions(), ExtractionConfig{Domain: "test"})

	for _, term := range artifact.Terms {
		if term.Line == 0 {
			t.Fatalf("expected line number for %s", term.Term)
		}
	}
}

func TestValidateLexiconArtifactValid(t *testing.T) {
	artifact := ExtractLexicon(castWithDefinitions(), ExtractionConfig{Domain: "insurance", SourceHash: "h"})
	errs := ValidateLexiconArtifact(artifact)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateLexiconArtifactMissingDomain(t *testing.T) {
	artifact := LexiconArtifact{TermCount: 0, Terms: nil}
	errs := ValidateLexiconArtifact(artifact)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "domain") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected domain error, got %v", errs)
	}
}

func TestValidateLexiconArtifactCountMismatch(t *testing.T) {
	artifact := LexiconArtifact{Domain: "x", TermCount: 5, Terms: nil}
	errs := ValidateLexiconArtifact(artifact)
	found := false
	for _, e := range errs {
		if strings.Contains(e, "term_count") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected count mismatch error, got %v", errs)
	}
}

func TestWriteLexiconYAML(t *testing.T) {
	artifact := ExtractLexicon(castWithDefinitions(), ExtractionConfig{Domain: "insurance"})

	var buf bytes.Buffer
	if err := WriteLexiconYAML(&buf, artifact); err != nil {
		t.Fatalf("write error: %v", err)
	}

	content := buf.String()
	if !strings.Contains(content, "domain: insurance") {
		t.Fatalf("expected domain in YAML, got:\n%s", content)
	}
	if !strings.Contains(content, "term:") {
		t.Fatalf("expected term entries in YAML")
	}
}

func TestToLexicon(t *testing.T) {
	artifact := ExtractLexicon(castWithDefinitions(), ExtractionConfig{Domain: "insurance"})
	lex := artifact.ToLexicon()

	if lex.TermCount() != artifact.TermCount {
		t.Fatalf("expected %d terms in lexicon, got %d", artifact.TermCount, lex.TermCount())
	}
	if !lex.IsGoverned("assuré") {
		t.Fatal("expected 'assuré' to be governed")
	}
}

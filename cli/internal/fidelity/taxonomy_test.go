package fidelity

import (
	"testing"
)

func buildTestTaxonomy() *Taxonomy {
	t := NewTaxonomy()
	t.AddDomain("insurance", "Insurance", "Insurance domain")
	t.AddSubject("insurance/health", "Health Insurance", "insurance", "Health coverage")
	t.AddSubject("insurance/auto", "Auto Insurance", "insurance", "Vehicle coverage")
	t.AddConcept("insurance/health/policy", "Policy", "insurance/health", "Insurance contract")
	t.AddConcept("insurance/health/claim", "Claim", "insurance/health", "Claim for reimbursement")
	t.AddConcept("insurance/auto/premium", "Premium", "insurance/auto", "Insurance premium")
	t.LinkTerm("insurance/health/policy", "police")
	t.LinkTerm("insurance/health/policy", "contrat")
	t.LinkTerm("insurance/health/claim", "sinistre")
	t.LinkTerm("insurance/auto/premium", "prime")
	return t
}

func TestNewTaxonomy(t *testing.T) {
	tax := NewTaxonomy()
	if tax.Size() != 0 {
		t.Fatalf("expected 0 taxons, got %d", tax.Size())
	}
}

func TestAddDomain(t *testing.T) {
	tax := NewTaxonomy()
	if err := tax.AddDomain("ins", "Insurance", ""); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if tax.Size() != 1 {
		t.Fatalf("expected 1, got %d", tax.Size())
	}
	roots := tax.Roots()
	if len(roots) != 1 || roots[0] != "ins" {
		t.Fatalf("expected root ins, got %v", roots)
	}
}

func TestAddDomain_Duplicate(t *testing.T) {
	tax := NewTaxonomy()
	tax.AddDomain("ins", "Insurance", "")
	if err := tax.AddDomain("ins", "Insurance2", ""); err == nil {
		t.Fatal("expected error for duplicate domain")
	}
}

func TestAddSubject(t *testing.T) {
	tax := NewTaxonomy()
	tax.AddDomain("ins", "Insurance", "")
	if err := tax.AddSubject("ins/health", "Health", "ins", ""); err != nil {
		t.Fatalf("AddSubject: %v", err)
	}
	parent, _ := tax.Get("ins")
	if len(parent.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(parent.Children))
	}
}

func TestAddSubject_MissingParent(t *testing.T) {
	tax := NewTaxonomy()
	if err := tax.AddSubject("s", "S", "missing", ""); err == nil {
		t.Fatal("expected error for missing parent")
	}
}

func TestAddConcept(t *testing.T) {
	tax := buildTestTaxonomy()
	concept, ok := tax.Get("insurance/health/policy")
	if !ok {
		t.Fatal("concept not found")
	}
	if concept.Level != LevelConcept {
		t.Fatalf("expected concept level, got %s", concept.Level)
	}
	if concept.Label != "Policy" {
		t.Fatalf("expected label Policy, got %q", concept.Label)
	}
}

func TestLinkTerm(t *testing.T) {
	tax := buildTestTaxonomy()
	concept, _ := tax.Get("insurance/health/policy")
	if concept.TermCount != 2 {
		t.Fatalf("expected 2 terms, got %d", concept.TermCount)
	}
}

func TestLinkTerm_Dedup(t *testing.T) {
	tax := buildTestTaxonomy()
	tax.LinkTerm("insurance/health/policy", "police")
	concept, _ := tax.Get("insurance/health/policy")
	if concept.TermCount != 2 {
		t.Fatalf("expected 2 (no dup), got %d", concept.TermCount)
	}
}

func TestLinkTerm_MissingTaxon(t *testing.T) {
	tax := NewTaxonomy()
	if err := tax.LinkTerm("missing", "term"); err == nil {
		t.Fatal("expected error for missing taxon")
	}
}

func TestClassify(t *testing.T) {
	tax := buildTestTaxonomy()
	ids := tax.Classify("sinistre")
	if len(ids) != 1 || ids[0] != "insurance/health/claim" {
		t.Fatalf("expected claim taxon, got %v", ids)
	}
}

func TestClassify_CaseInsensitive(t *testing.T) {
	tax := buildTestTaxonomy()
	ids := tax.Classify("POLICE")
	if len(ids) != 1 {
		t.Fatalf("expected 1 result, got %d", len(ids))
	}
}

func TestClassify_NotFound(t *testing.T) {
	tax := buildTestTaxonomy()
	ids := tax.Classify("unknown-term")
	if len(ids) != 0 {
		t.Fatalf("expected 0 results, got %d", len(ids))
	}
}

func TestAncestors(t *testing.T) {
	tax := buildTestTaxonomy()
	ancestors := tax.Ancestors("insurance/health/policy")
	if len(ancestors) != 2 {
		t.Fatalf("expected 2 ancestors, got %d: %v", len(ancestors), ancestors)
	}
	if ancestors[0] != "insurance" {
		t.Fatalf("expected root ancestor insurance, got %q", ancestors[0])
	}
	if ancestors[1] != "insurance/health" {
		t.Fatalf("expected second ancestor insurance/health, got %q", ancestors[1])
	}
}

func TestAncestors_Root(t *testing.T) {
	tax := buildTestTaxonomy()
	ancestors := tax.Ancestors("insurance")
	if len(ancestors) != 0 {
		t.Fatalf("expected 0 ancestors for root, got %d", len(ancestors))
	}
}

func TestDescendants(t *testing.T) {
	tax := buildTestTaxonomy()
	desc := tax.Descendants("insurance")
	if len(desc) != 5 {
		t.Fatalf("expected 5 descendants, got %d: %v", len(desc), desc)
	}
}

func TestDescendants_Leaf(t *testing.T) {
	tax := buildTestTaxonomy()
	desc := tax.Descendants("insurance/health/policy")
	if len(desc) != 0 {
		t.Fatalf("expected 0 descendants for leaf, got %d", len(desc))
	}
}

func TestAllTerms(t *testing.T) {
	tax := buildTestTaxonomy()
	terms := tax.AllTerms("insurance")
	if len(terms) != 4 {
		t.Fatalf("expected 4 terms across all descendants, got %d: %v", len(terms), terms)
	}
}

func TestAllTerms_Subject(t *testing.T) {
	tax := buildTestTaxonomy()
	terms := tax.AllTerms("insurance/health")
	if len(terms) != 3 {
		t.Fatalf("expected 3 terms (police, contrat, sinistre), got %d: %v", len(terms), terms)
	}
}

func TestAllTerms_Concept(t *testing.T) {
	tax := buildTestTaxonomy()
	terms := tax.AllTerms("insurance/health/policy")
	if len(terms) != 2 {
		t.Fatalf("expected 2 terms, got %d", len(terms))
	}
}

func TestSize(t *testing.T) {
	tax := buildTestTaxonomy()
	if tax.Size() != 6 {
		t.Fatalf("expected 6 taxons, got %d", tax.Size())
	}
}

func TestFlat(t *testing.T) {
	tax := buildTestTaxonomy()
	flat := tax.Flat()
	if len(flat) != 6 {
		t.Fatalf("expected 6 flat taxons, got %d", len(flat))
	}
	// Should be sorted by ID.
	for i := 1; i < len(flat); i++ {
		if flat[i].ID < flat[i-1].ID {
			t.Fatal("flat output should be sorted by ID")
		}
	}
}

func TestBuildFromLexicon(t *testing.T) {
	lex := NewLexicon()
	lex.Add(Term{Canonical: "Police", Domain: "assurance", Synonyms: []string{"contrat"}, Definition: "Insurance contract"})
	lex.Add(Term{Canonical: "Sinistre", Domain: "assurance", Definition: "Claim event"})
	lex.Add(Term{Canonical: "Variable", Domain: "", Definition: "A programming variable"})

	tax := BuildFromLexicon(lex)

	if tax.Size() == 0 {
		t.Fatal("expected taxons from lexicon")
	}

	// Should have 2 domains: assurance, general.
	roots := tax.Roots()
	if len(roots) != 2 {
		t.Fatalf("expected 2 domain roots, got %d: %v", len(roots), roots)
	}

	// Classify by term.
	ids := tax.Classify("police")
	if len(ids) == 0 {
		t.Fatal("expected police to classify")
	}
	ids = tax.Classify("contrat")
	if len(ids) == 0 {
		t.Fatal("expected synonym contrat to classify")
	}
}

func TestBuildFromLexicon_NilLexicon(t *testing.T) {
	tax := BuildFromLexicon(nil)
	if tax.Size() != 0 {
		t.Fatalf("expected 0 taxons for nil lexicon, got %d", tax.Size())
	}
}

func TestAddEmptyID(t *testing.T) {
	tax := NewTaxonomy()
	if err := tax.AddDomain("", "X", ""); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestLinkTermEmpty(t *testing.T) {
	tax := NewTaxonomy()
	tax.AddDomain("d", "D", "")
	if err := tax.LinkTerm("d", ""); err == nil {
		t.Fatal("expected error for empty term")
	}
}

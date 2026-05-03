package fidelity

import "testing"

func insuranceLexicon() *Lexicon {
	l := NewLexicon()
	l.Add(Term{
		Canonical:  "garantie",
		Synonyms:   []string{"warranty", "couverture"},
		Deprecated: []string{"assurance-couv"},
		Domain:     "insurance",
		Definition: "Protection contractuelle contre un risque.",
	})
	l.Add(Term{
		Canonical:  "franchise",
		Synonyms:   []string{"deductible", "excess"},
		Domain:     "insurance",
		Definition: "Part restant a la charge de l'assure.",
	})
	l.Add(Term{
		Canonical:  "sinistre",
		Synonyms:   []string{"claim", "loss-event"},
		Deprecated: []string{"accident-couvert"},
		Domain:     "insurance",
	})
	l.Add(Term{
		Canonical:  "exclusion",
		Domain:     "insurance",
	})
	l.Add(Term{
		Canonical:  "plafond",
		Synonyms:   []string{"cap", "ceiling", "limit"},
		Domain:     "insurance",
	})
	return l
}

func TestResolveCanonical(t *testing.T) {
	l := insuranceLexicon()
	canon, status := l.Resolve("garantie")
	assertEqual(t, "garantie", canon)
	assertEqual(t, TermCanonical, status)
}

func TestResolveSynonym(t *testing.T) {
	l := insuranceLexicon()
	canon, status := l.Resolve("warranty")
	assertEqual(t, "garantie", canon)
	assertEqual(t, TermSynonym, status)
}

func TestResolveDeprecated(t *testing.T) {
	l := insuranceLexicon()
	canon, status := l.Resolve("assurance-couv")
	assertEqual(t, "garantie", canon)
	assertEqual(t, TermDeprecated, status)
}

func TestResolveUnknown(t *testing.T) {
	l := insuranceLexicon()
	canon, status := l.Resolve("inconnu")
	assertEqual(t, "", canon)
	assertEqual(t, TermStatus(""), status)
}

func TestResolveCaseInsensitive(t *testing.T) {
	l := insuranceLexicon()
	canon, _ := l.Resolve("GARANTIE")
	assertEqual(t, "garantie", canon)

	canon, _ = l.Resolve("Warranty")
	assertEqual(t, "garantie", canon)
}

func TestIsGoverned(t *testing.T) {
	l := insuranceLexicon()
	if !l.IsGoverned("garantie") {
		t.Fatal("canonical should be governed")
	}
	if !l.IsGoverned("deductible") {
		t.Fatal("synonym should be governed")
	}
	if !l.IsGoverned("accident-couvert") {
		t.Fatal("deprecated should be governed")
	}
	if l.IsGoverned("inconnu") {
		t.Fatal("unknown should not be governed")
	}
}

func TestTermCount(t *testing.T) {
	l := insuranceLexicon()
	if l.TermCount() != 5 {
		t.Fatalf("expected 5, got %d", l.TermCount())
	}
}

func TestAllTermsSorted(t *testing.T) {
	l := insuranceLexicon()
	terms := l.AllTerms()
	for i := 1; i < len(terms); i++ {
		if terms[i].Canonical < terms[i-1].Canonical {
			t.Fatalf("not sorted: %q before %q", terms[i-1].Canonical, terms[i].Canonical)
		}
	}
}

func TestAddDuplicate(t *testing.T) {
	l := NewLexicon()
	l.Add(Term{Canonical: "test"})
	err := l.Add(Term{Canonical: "test"})
	if err == nil {
		t.Fatal("expected error for duplicate")
	}
}

func TestAddEmpty(t *testing.T) {
	l := NewLexicon()
	err := l.Add(Term{Canonical: ""})
	if err == nil {
		t.Fatal("expected error for empty canonical")
	}
}

// --- Gate: CheckText ---

func TestCheckTextClean(t *testing.T) {
	l := insuranceLexicon()
	r := l.CheckText("La garantie couvre le sinistre.", "test")
	if !r.Pass {
		t.Fatalf("expected pass, findings: %v", r.Findings)
	}
}

func TestCheckTextDeprecated(t *testing.T) {
	l := insuranceLexicon()
	r := l.CheckText("La assurance-couv est active.", "test:line:5")
	if r.Pass {
		t.Fatal("deprecated term should fail gate")
	}
	if len(r.Findings) == 0 {
		t.Fatal("expected findings")
	}
	found := false
	for _, f := range r.Findings {
		if f.Code == CodeDeprecatedTerm && f.Word == "assurance-couv" {
			found = true
			if f.Canonical != "garantie" {
				t.Fatalf("expected canonical garantie, got %q", f.Canonical)
			}
			if !f.Blocking {
				t.Fatal("deprecated should be blocking")
			}
		}
	}
	if !found {
		t.Fatal("expected DEPRECATED_TERM finding for assurance-couv")
	}
}

func TestCheckTextUngoverned(t *testing.T) {
	l := insuranceLexicon()
	r := l.CheckText("Le montant du remboursement est calcule.", "test")
	// "remboursement" and "calcule" are ungoverned but non-blocking in text mode.
	ungoverned := 0
	for _, f := range r.Findings {
		if f.Code == CodeUngoverned {
			ungoverned++
			if f.Blocking {
				t.Fatal("ungoverned in text should not be blocking")
			}
		}
	}
	if ungoverned == 0 {
		t.Fatal("expected ungoverned findings")
	}
	// Pass should still be true (ungoverned in text is warning only).
	if !r.Pass {
		t.Fatal("ungoverned text terms should not fail gate")
	}
}

func TestCheckTextSkipsShortWords(t *testing.T) {
	l := insuranceLexicon()
	r := l.CheckText("Le de la", "test")
	// "Le", "de", "la" are <=2 chars, should be skipped.
	for _, f := range r.Findings {
		if len(f.Word) <= 2 {
			t.Fatalf("should skip short word %q", f.Word)
		}
	}
}

func TestCheckTextNoDuplicateFindings(t *testing.T) {
	l := insuranceLexicon()
	r := l.CheckText("garantie garantie garantie", "test")
	// Same word repeated should produce at most 1 finding.
	count := 0
	for _, f := range r.Findings {
		if f.Word == "garantie" {
			count++
		}
	}
	if count > 1 {
		t.Fatalf("expected deduplicated findings, got %d", count)
	}
}

// --- Gate: CheckTerms ---

func TestCheckTermsAllGoverned(t *testing.T) {
	l := insuranceLexicon()
	r := l.CheckTerms([]string{"garantie", "franchise", "exclusion"}, "atom:A-001")
	if !r.Pass {
		t.Fatalf("expected pass, findings: %v", r.Findings)
	}
}

func TestCheckTermsSynonymPasses(t *testing.T) {
	l := insuranceLexicon()
	r := l.CheckTerms([]string{"warranty", "deductible"}, "atom:A-002")
	if !r.Pass {
		t.Fatalf("synonyms should pass, findings: %v", r.Findings)
	}
}

func TestCheckTermsDeprecatedBlocks(t *testing.T) {
	l := insuranceLexicon()
	r := l.CheckTerms([]string{"garantie", "accident-couvert"}, "atom:A-003")
	if r.Pass {
		t.Fatal("deprecated term should block")
	}
	found := false
	for _, f := range r.Findings {
		if f.Code == CodeDeprecatedTerm {
			found = true
		}
	}
	if !found {
		t.Fatal("expected DEPRECATED_TERM finding")
	}
}

func TestCheckTermsUngovernedBlocks(t *testing.T) {
	l := insuranceLexicon()
	r := l.CheckTerms([]string{"garantie", "mot-inconnu"}, "atom:A-004")
	if r.Pass {
		t.Fatal("ungoverned term should block in CheckTerms")
	}
	found := false
	for _, f := range r.Findings {
		if f.Code == CodeUngoverned && f.Word == "mot-inconnu" {
			found = true
			if !f.Blocking {
				t.Fatal("ungoverned in CheckTerms should be blocking")
			}
		}
	}
	if !found {
		t.Fatal("expected UNGOVERNED_TERM finding")
	}
}

func TestCheckTermsEmpty(t *testing.T) {
	l := insuranceLexicon()
	r := l.CheckTerms(nil, "test")
	if !r.Pass {
		t.Fatal("empty list should pass")
	}
	if r.Checked != 0 {
		t.Fatalf("checked: %d", r.Checked)
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("La garantie couvre l'assuré (hors franchise).")
	if len(tokens) == 0 {
		t.Fatal("expected tokens")
	}
	// Should include "l'assuré" as one token (apostrophe is word char).
	found := false
	for _, tok := range tokens {
		if tok == "l'assuré" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected l'assuré as token, got %v", tokens)
	}
}

func TestGateResultLocation(t *testing.T) {
	l := insuranceLexicon()
	r := l.CheckTerms([]string{"inconnu"}, "atom:A-123:line:45")
	if len(r.Findings) == 0 {
		t.Fatal("expected finding")
	}
	if r.Findings[0].Location != "atom:A-123:line:45" {
		t.Fatalf("location: %q", r.Findings[0].Location)
	}
}

// --- helpers ---

func assertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

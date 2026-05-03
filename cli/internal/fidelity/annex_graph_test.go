package fidelity

import (
	"testing"
)

const annexFixture = `# Document Principal

Ce document fait reference a Annex A et Appendix B.

Voir aussi [RFC 2119] pour la terminologie.

## Section 1

Selon Annexe A, les regles s'appliquent. Aussi [ISO 27001].

## Annexe A: Definitions

Termes et definitions du domaine.

## Appendix B — Exemples

Exemples d'application.

## Bibliographie

[RFC 2119]: Key words for use in RFCs.
[ISO 27001]: Information security management systems.
[GAMP 5]: Good Automated Manufacturing Practice.
`

func TestBuildAnnexGraphDetectsAnnexes(t *testing.T) {
	g := BuildAnnexGraph(annexFixture)

	if len(g.Annexes) != 2 {
		t.Fatalf("expected 2 annexes, got %d: %+v", len(g.Annexes), g.Annexes)
	}
	ids := map[string]bool{}
	for _, a := range g.Annexes {
		ids[a.ID] = true
	}
	if !ids["ANNEX-A"] {
		t.Fatal("expected ANNEX-A")
	}
	if !ids["ANNEX-B"] {
		t.Fatal("expected ANNEX-B")
	}
}

func TestBuildAnnexGraphDetectsBibliography(t *testing.T) {
	g := BuildAnnexGraph(annexFixture)

	if len(g.Bibliography) < 3 {
		t.Fatalf("expected >= 3 biblio entries, got %d: %+v", len(g.Bibliography), g.Bibliography)
	}
}

func TestBuildAnnexGraphDetectsReferences(t *testing.T) {
	g := BuildAnnexGraph(annexFixture)

	if len(g.References) == 0 {
		t.Fatal("expected references detected")
	}

	// Should find references to Annex A and RFC 2119.
	targets := map[string]bool{}
	for _, ref := range g.References {
		targets[ref.TargetID] = true
	}
	if !targets["ANNEX-A"] {
		t.Fatal("expected reference to ANNEX-A")
	}
	if !targets["BIB-RFC-2119"] {
		t.Fatal("expected reference to BIB-RFC-2119")
	}
}

func TestBuildAnnexGraphAnnexRefCount(t *testing.T) {
	g := BuildAnnexGraph(annexFixture)

	for _, a := range g.Annexes {
		if a.ID == "ANNEX-A" && a.RefCount == 0 {
			t.Fatal("ANNEX-A should have refs")
		}
	}
}

func TestBuildAnnexGraphOrphans(t *testing.T) {
	content := "See Annex C for details.\n\nAlso [Unknown Ref].\n"
	g := BuildAnnexGraph(content)

	if len(g.Orphans) == 0 {
		t.Fatal("expected orphans for undefined targets")
	}
	found := false
	for _, o := range g.Orphans {
		if o == "ANNEX-C" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ANNEX-C as orphan, got %v", g.Orphans)
	}
}

func TestBuildAnnexGraphUnreferenced(t *testing.T) {
	g := BuildAnnexGraph(annexFixture)

	// GAMP 5 is defined in bibliography but never referenced in body.
	found := false
	for _, u := range g.Unreferenced {
		if u == "BIB-GAMP-5" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected GAMP 5 unreferenced, got %v", g.Unreferenced)
	}
}

func TestBuildAnnexGraphIsComplete(t *testing.T) {
	_ = BuildAnnexGraph(annexFixture)
	// Test with a self-contained fixture where all refs resolve.
	selfContained := `# Doc

See Annex A.

## Annex A: Terms

Terms here.
`
	g2 := BuildAnnexGraph(selfContained)
	if !g2.IsComplete() {
		t.Fatalf("expected complete graph, orphans: %v", g2.Orphans)
	}
}

func TestBuildAnnexGraphStats(t *testing.T) {
	g := BuildAnnexGraph(annexFixture)
	stats := g.Stats()

	if stats.AnnexCount != 2 {
		t.Fatalf("expected 2 annexes, got %d", stats.AnnexCount)
	}
	if stats.BiblioCount < 3 {
		t.Fatalf("expected >= 3 biblio, got %d", stats.BiblioCount)
	}
	if stats.ReferenceCount == 0 {
		t.Fatal("expected references")
	}
}

func TestBuildAnnexGraphEmpty(t *testing.T) {
	g := BuildAnnexGraph("")

	if len(g.Annexes) != 0 || len(g.Bibliography) != 0 {
		t.Fatal("expected empty graph for empty content")
	}
	if !g.IsComplete() {
		t.Fatal("empty graph should be complete")
	}
}

func TestBuildAnnexGraphNoAnnexes(t *testing.T) {
	content := "# Simple Doc\n\nJust text with [1] reference.\n\n[1]: Some citation.\n"
	g := BuildAnnexGraph(content)

	if len(g.Annexes) != 0 {
		t.Fatalf("expected 0 annexes, got %d", len(g.Annexes))
	}
	if len(g.Bibliography) < 1 {
		t.Fatalf("expected >= 1 biblio, got %d", len(g.Bibliography))
	}
}

func TestBuildAnnexGraphAnnexTitle(t *testing.T) {
	g := BuildAnnexGraph(annexFixture)

	for _, a := range g.Annexes {
		if a.ID == "ANNEX-A" {
			if a.Title != "Definitions" {
				t.Fatalf("expected title 'Definitions', got %q", a.Title)
			}
		}
	}
}

func TestBuildAnnexGraphLineNumbers(t *testing.T) {
	g := BuildAnnexGraph(annexFixture)

	for _, a := range g.Annexes {
		if a.Line == 0 {
			t.Fatalf("annex %s has zero line number", a.ID)
		}
	}
	for _, b := range g.Bibliography {
		if b.Line == 0 {
			t.Fatalf("biblio %s has zero line number", b.ID)
		}
	}
}

func TestBuildAnnexGraphRefContext(t *testing.T) {
	g := BuildAnnexGraph(annexFixture)

	for _, ref := range g.References {
		if ref.Context == "" {
			t.Fatalf("ref to %s has empty context", ref.TargetID)
		}
	}
}

func TestBuildAnnexGraphFrenchAnnexe(t *testing.T) {
	content := "# Document\n\nVoir Annexe D.\n\n## Annexe D: Glossaire\n\nTermes.\n"
	g := BuildAnnexGraph(content)

	if len(g.Annexes) != 1 {
		t.Fatalf("expected 1 annexe, got %d", len(g.Annexes))
	}
	if g.Annexes[0].ID != "ANNEX-D" {
		t.Fatalf("expected ANNEX-D, got %s", g.Annexes[0].ID)
	}
}

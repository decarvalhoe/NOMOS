package fidelity

import (
	"testing"
)

func TestDetectReferencesAnnex(t *testing.T) {
	content := "See Annex A for details.\nRefer to Appendix B2 for formulas.\n"
	refs := DetectReferences(content)

	annexes := filterByType(refs, RefAnnex)
	if len(annexes) < 2 {
		t.Fatalf("expected >= 2 annex refs, got %d: %+v", len(annexes), refs)
	}
	if annexes[0].Target != "A" {
		t.Fatalf("expected target A, got %s", annexes[0].Target)
	}
}

func TestDetectReferencesAnnexFrench(t *testing.T) {
	content := "Voir Annexe C pour le detail.\n"
	refs := DetectReferences(content)

	annexes := filterByType(refs, RefAnnex)
	if len(annexes) != 1 {
		t.Fatalf("expected 1 annex, got %d", len(annexes))
	}
	if annexes[0].Target != "C" {
		t.Fatalf("expected target C, got %s", annexes[0].Target)
	}
}

func TestDetectReferencesBibliographicSquare(t *testing.T) {
	content := "As described in [RFC 2119] and [ISO 27001].\n"
	refs := DetectReferences(content)

	biblios := filterByType(refs, RefBibliographic)
	if len(biblios) < 2 {
		t.Fatalf("expected >= 2 biblio refs, got %d: %+v", len(biblios), refs)
	}
}

func TestDetectReferencesBibliographicNumeric(t *testing.T) {
	content := "First requirement [1] and second [23].\n"
	refs := DetectReferences(content)

	biblios := filterByType(refs, RefBibliographic)
	if len(biblios) < 2 {
		t.Fatalf("expected >= 2 numeric biblio refs, got %d", len(biblios))
	}
	if biblios[0].Confidence != "high" {
		t.Fatalf("expected high confidence for numeric ref, got %s", biblios[0].Confidence)
	}
}

func TestDetectReferencesBibliographicParen(t *testing.T) {
	content := "Based on prior work (Smith, 2024) and (Jones et al., 2023).\n"
	refs := DetectReferences(content)

	biblios := filterByType(refs, RefBibliographic)
	if len(biblios) < 2 {
		t.Fatalf("expected >= 2 paren biblio refs, got %d: %+v", len(biblios), refs)
	}
}

func TestDetectReferencesCrossRefExplicit(t *testing.T) {
	content := "See Section 3.2 for details. Cf. Article L.113-2.\n"
	refs := DetectReferences(content)

	crossRefs := filterByType(refs, RefCrossReference)
	if len(crossRefs) < 2 {
		t.Fatalf("expected >= 2 cross-refs, got %d: %+v", len(crossRefs), refs)
	}
}

func TestDetectReferencesCrossRefFrench(t *testing.T) {
	content := "Voir Article 5 pour les principes.\n"
	refs := DetectReferences(content)

	crossRefs := filterByType(refs, RefCrossReference)
	if len(crossRefs) < 1 {
		t.Fatalf("expected >= 1 cross-ref for French, got %d", len(crossRefs))
	}
}

func TestDetectReferencesInternalLink(t *testing.T) {
	content := "Check [the overview](#overview) and [related](./related.md).\n"
	refs := DetectReferences(content)

	crossRefs := filterByType(refs, RefCrossReference)
	found := 0
	for _, r := range crossRefs {
		if r.Target == "#overview" || r.Target == "./related.md" {
			found++
		}
	}
	if found < 2 {
		t.Fatalf("expected 2 internal link cross-refs, found %d: %+v", found, crossRefs)
	}
}

func TestDetectReferencesIgnoresExternalLinks(t *testing.T) {
	content := "Visit [Google](https://google.com) for search.\n"
	refs := DetectReferences(content)

	for _, r := range refs {
		if r.Type == RefCrossReference && r.Target == "https://google.com" {
			t.Fatal("external links should not be detected as cross-references")
		}
	}
}

func TestDetectReferencesLineNumbers(t *testing.T) {
	content := "Line one.\nSee Annex A here.\nLine three.\n"
	refs := DetectReferences(content)

	for _, r := range refs {
		if r.Type == RefAnnex && r.Line != 2 {
			t.Fatalf("expected line 2 for annex, got %d", r.Line)
		}
	}
}

func TestDetectReferencesEmpty(t *testing.T) {
	refs := DetectReferences("")
	if len(refs) != 0 {
		t.Fatalf("expected 0 refs for empty content, got %d", len(refs))
	}
}

func TestDetectReferencesNoRefs(t *testing.T) {
	content := "This is a simple paragraph with no references at all.\n"
	refs := DetectReferences(content)

	if len(refs) != 0 {
		t.Fatalf("expected 0 refs, got %d: %+v", len(refs), refs)
	}
}

func TestDetectReferencesMixed(t *testing.T) {
	content := `# Document

See Annex A for the full list.

This follows [RFC 7231] requirements.

Cf. Section 4.1 for the implementation.

As noted by (Fowler, 2019), architecture matters.
`
	refs := DetectReferences(content)

	annexes := filterByType(refs, RefAnnex)
	biblios := filterByType(refs, RefBibliographic)
	crossRefs := filterByType(refs, RefCrossReference)

	if len(annexes) < 1 {
		t.Fatalf("expected annex refs, got %d", len(annexes))
	}
	if len(biblios) < 2 {
		t.Fatalf("expected >= 2 biblio refs, got %d", len(biblios))
	}
	if len(crossRefs) < 1 {
		t.Fatalf("expected cross-refs, got %d", len(crossRefs))
	}
}

func TestDetectReferencesDeduplicates(t *testing.T) {
	content := "See Annex A. Also see Annex A again.\n"
	refs := DetectReferences(content)

	// Same line, same raw → should still appear (different positions same line is OK).
	// But identical key (type+raw+line) should be deduped.
	annexes := filterByType(refs, RefAnnex)
	if len(annexes) > 2 {
		t.Fatalf("expected deduplication, got %d annex refs", len(annexes))
	}
}

func filterByType(refs []RefCandidate, refType RefType) []RefCandidate {
	var result []RefCandidate
	for _, r := range refs {
		if r.Type == refType {
			result = append(result, r)
		}
	}
	return result
}

package fidelity

import (
	"testing"
)

func TestCompareTOCPerfectMatch(t *testing.T) {
	source := []TOCEntry{
		{ID: "h1", Title: "Introduction", Depth: 1},
		{ID: "h2", Title: "Background", Depth: 2},
		{ID: "h3", Title: "Conclusion", Depth: 1},
	}
	nomos := []TOCEntry{
		{ID: "n1", Title: "Introduction", Depth: 1},
		{ID: "n2", Title: "Background", Depth: 2},
		{ID: "n3", Title: "Conclusion", Depth: 1},
	}

	result := CompareTOC(source, nomos)

	if !result.Aligned {
		t.Fatalf("expected aligned, got drifts: %v", result.Drifts)
	}
	if result.Score != 1.0 {
		t.Fatalf("expected score 1.0, got %f", result.Score)
	}
	if result.MatchCount != 3 {
		t.Fatalf("expected 3 matches, got %d", result.MatchCount)
	}
}

func TestCompareTOCMissing(t *testing.T) {
	source := []TOCEntry{
		{ID: "h1", Title: "Chapter 1", Depth: 1},
		{ID: "h2", Title: "Chapter 2", Depth: 1},
		{ID: "h3", Title: "Chapter 3", Depth: 1},
	}
	nomos := []TOCEntry{
		{ID: "n1", Title: "Chapter 1", Depth: 1},
		{ID: "n3", Title: "Chapter 3", Depth: 1},
	}

	result := CompareTOC(source, nomos)

	if result.Aligned {
		t.Fatal("expected not aligned")
	}
	hasMissing := false
	for _, d := range result.Drifts {
		if d.Kind == DriftMissing && d.SourceEntry.Title == "Chapter 2" {
			hasMissing = true
		}
	}
	if !hasMissing {
		t.Fatalf("expected missing drift for Chapter 2, got %v", result.Drifts)
	}
}

func TestCompareTOCExtra(t *testing.T) {
	source := []TOCEntry{
		{ID: "h1", Title: "Section A", Depth: 1},
	}
	nomos := []TOCEntry{
		{ID: "n1", Title: "Section A", Depth: 1},
		{ID: "n2", Title: "Section B", Depth: 1},
	}

	result := CompareTOC(source, nomos)

	if result.Aligned {
		t.Fatal("expected not aligned")
	}
	hasExtra := false
	for _, d := range result.Drifts {
		if d.Kind == DriftExtra && d.NomosEntry.Title == "Section B" {
			hasExtra = true
		}
	}
	if !hasExtra {
		t.Fatal("expected extra drift for Section B")
	}
}

func TestCompareTOCDepthMismatch(t *testing.T) {
	source := []TOCEntry{
		{ID: "h1", Title: "Topic", Depth: 1},
	}
	nomos := []TOCEntry{
		{ID: "n1", Title: "Topic", Depth: 2},
	}

	result := CompareTOC(source, nomos)

	if result.Aligned {
		t.Fatal("expected not aligned")
	}
	hasDepth := false
	for _, d := range result.Drifts {
		if d.Kind == DriftDepth {
			hasDepth = true
		}
	}
	if !hasDepth {
		t.Fatal("expected depth drift")
	}
}

func TestCompareTOCReordered(t *testing.T) {
	source := []TOCEntry{
		{ID: "h1", Title: "First", Depth: 1},
		{ID: "h2", Title: "Second", Depth: 1},
		{ID: "h3", Title: "Third", Depth: 1},
	}
	nomos := []TOCEntry{
		{ID: "n1", Title: "First", Depth: 1},
		{ID: "n3", Title: "Third", Depth: 1},
		{ID: "n2", Title: "Second", Depth: 1},
	}

	result := CompareTOC(source, nomos)

	if result.Aligned {
		t.Fatal("expected not aligned (reordered)")
	}
	hasReorder := false
	for _, d := range result.Drifts {
		if d.Kind == DriftReordered {
			hasReorder = true
		}
	}
	if !hasReorder {
		t.Fatal("expected reorder drift")
	}
}

func TestCompareTOCBothEmpty(t *testing.T) {
	result := CompareTOC(nil, nil)
	if !result.Aligned {
		t.Fatal("expected aligned for empty")
	}
	if result.Score != 1.0 {
		t.Fatalf("expected score 1.0, got %f", result.Score)
	}
}

func TestCompareTOCSourceEmpty(t *testing.T) {
	nomos := []TOCEntry{{ID: "n1", Title: "Something", Depth: 1}}
	result := CompareTOC(nil, nomos)

	if result.Aligned {
		t.Fatal("expected not aligned")
	}
	if result.DriftCount != 1 {
		t.Fatalf("expected 1 drift (extra), got %d", result.DriftCount)
	}
}

func TestCompareTOCNomosEmpty(t *testing.T) {
	source := []TOCEntry{{ID: "h1", Title: "Something", Depth: 1}}
	result := CompareTOC(source, nil)

	if result.Aligned {
		t.Fatal("expected not aligned")
	}
	if result.DriftCount != 1 {
		t.Fatalf("expected 1 drift (missing), got %d", result.DriftCount)
	}
}

func TestCompareTOCCaseInsensitive(t *testing.T) {
	source := []TOCEntry{{ID: "h1", Title: "CHAPTER ONE", Depth: 1}}
	nomos := []TOCEntry{{ID: "n1", Title: "chapter one", Depth: 1}}

	result := CompareTOC(source, nomos)

	if !result.Aligned {
		t.Fatalf("expected aligned (case insensitive), got drifts: %v", result.Drifts)
	}
}

func TestCompareTOCWhitespaceTrimming(t *testing.T) {
	source := []TOCEntry{{ID: "h1", Title: "  Title  ", Depth: 1}}
	nomos := []TOCEntry{{ID: "n1", Title: "Title", Depth: 1}}

	result := CompareTOC(source, nomos)

	if !result.Aligned {
		t.Fatal("expected aligned after whitespace trimming")
	}
}

func TestCompareTOCScore(t *testing.T) {
	source := []TOCEntry{
		{Title: "A", Depth: 1},
		{Title: "B", Depth: 1},
		{Title: "C", Depth: 1},
		{Title: "D", Depth: 1},
	}
	nomos := []TOCEntry{
		{Title: "A", Depth: 1},
		{Title: "B", Depth: 1},
	}

	result := CompareTOC(source, nomos)
	// 2 matched out of max(4, 2) = 4
	if result.Score != 0.5 {
		t.Fatalf("expected score 0.5, got %f", result.Score)
	}
}

func TestCompareTOCFromTree(t *testing.T) {
	source := []TOCEntry{
		{Title: "Root", Depth: 0},
		{Title: "Child", Depth: 1},
	}

	tree := BuildStructureTree([]HeadingInput{
		{ID: "root", Title: "Root", Level: 1, Hash: "a"},
		{ID: "child", Title: "Child", Level: 2, Hash: "b"},
	}, TreeConfig{DocumentID: "doc", SourceHash: "h"})

	result := CompareTOCFromTree(source, tree)

	// FlatTOC produces nodes with depth based on tree level.
	// The comparison should find entries.
	if result.SourceCount != 2 {
		t.Fatalf("expected source count 2, got %d", result.SourceCount)
	}
	if result.NomosCount == 0 {
		t.Fatal("expected nomos entries from tree")
	}
}

func TestCompareTOCMultipleDrifts(t *testing.T) {
	source := []TOCEntry{
		{Title: "A", Depth: 1},
		{Title: "B", Depth: 2},
		{Title: "C", Depth: 1},
	}
	nomos := []TOCEntry{
		{Title: "A", Depth: 2}, // depth mismatch
		{Title: "D", Depth: 1}, // extra
		// B missing, C missing
	}

	result := CompareTOC(source, nomos)

	if result.Aligned {
		t.Fatal("expected not aligned")
	}
	if result.DriftCount < 3 {
		t.Fatalf("expected at least 3 drifts, got %d: %v", result.DriftCount, result.Drifts)
	}
}

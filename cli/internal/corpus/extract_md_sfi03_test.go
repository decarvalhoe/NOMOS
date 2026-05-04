package corpus

import (
	"strings"
	"testing"
)

// TestSFI03NoParentChildBodyDuplication is the headline check for #341:
// the same source byte span cannot create multiple canonical semantic
// atoms, and a heading entry must not own its descendants' body text.
func TestSFI03NoParentChildBodyDuplication(t *testing.T) {
	doc := "# H1 title\nParagraph under H1.\n## H2 title\nParagraph under H2.\n"
	units, err := ExtractMarkdownUnitsFromReader("docs/sfi03.md", strings.NewReader(doc))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	bodies := []string{"Paragraph under H1.", "Paragraph under H2."}

	// Heading entries must never carry descendant body text.
	for _, u := range units {
		if u.Kind != HeadingUnitKindHeading {
			continue
		}
		for _, body := range bodies {
			if strings.Contains(u.Content, body) {
				t.Fatalf("heading %q owns descendant body %q (Content=%q)", u.Title, body, u.Content)
			}
		}
	}

	// Each paragraph appears as exactly one semantic atom.
	for _, body := range bodies {
		hits := 0
		for _, u := range units {
			if !u.IsSemanticLeaf() {
				continue
			}
			if strings.Contains(u.Content, body) {
				hits++
			}
		}
		if hits != 1 {
			t.Fatalf("paragraph %q must appear exactly once as a semantic leaf, got %d", body, hits)
		}
	}

	// No two HeadingUnit records overlap on byte ranges (strict
	// non-duplication of canonical atoms over the same span).
	for i := 0; i < len(units); i++ {
		a := units[i]
		if a.EndByte == 0 && a.StartByte == 0 {
			continue
		}
		for j := i + 1; j < len(units); j++ {
			b := units[j]
			if b.EndByte == 0 && b.StartByte == 0 {
				continue
			}
			if rangesOverlap(a.StartByte, a.EndByte, b.StartByte, b.EndByte) {
				t.Fatalf("byte ranges overlap: %s(%d-%d) and %s(%d-%d)",
					a.Kind, a.StartByte, a.EndByte, b.Kind, b.StartByte, b.EndByte)
			}
		}
	}
}

// TestSFI03H5H6Preserved: H5/H6 lines must remain typed as headings,
// not folded into a parent paragraph.
func TestSFI03H5H6Preserved(t *testing.T) {
	doc := "# Top\n\n##### H5 entry\n\nParagraph after H5.\n\n###### H6 entry\n\nParagraph after H6.\n"
	units, err := ExtractMarkdownUnitsFromReader("docs/sfi03_h5h6.md", strings.NewReader(doc))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	wantHeadings := []struct {
		level int
		title string
	}{
		{1, "Top"},
		{5, "H5 entry"},
		{6, "H6 entry"},
	}

	headings := filterHeadingEntries(units)
	if len(headings) != len(wantHeadings) {
		t.Fatalf("expected %d heading entries, got %d", len(wantHeadings), len(headings))
	}
	for i, want := range wantHeadings {
		if headings[i].Level != want.level || headings[i].Title != want.title {
			t.Fatalf("heading %d: want (%d, %q), got (%d, %q)",
				i, want.level, want.title, headings[i].Level, headings[i].Title)
		}
		if headings[i].Kind != HeadingUnitKindHeading {
			t.Fatalf("heading %d: kind must be %q, got %q",
				i, HeadingUnitKindHeading, headings[i].Kind)
		}
	}

	// No semantic leaf should swallow an H5 or H6 line as paragraph
	// text.
	for _, u := range filterSemanticLeaves(units) {
		if strings.Contains(u.Content, "##### ") || strings.Contains(u.Content, "###### ") {
			t.Fatalf("H5/H6 line leaked into semantic leaf %q (Content=%q)", u.Title, u.Content)
		}
	}
}

// TestSFI03HeadingAncestryRetained: a paragraph under H1 > H2 > H3
// carries the full heading path on the unit, in order.
func TestSFI03HeadingAncestryRetained(t *testing.T) {
	doc := "# Alpha\n\n## Beta\n\n### Gamma\n\nThe deepest paragraph.\n"
	units, err := ExtractMarkdownUnitsFromReader("docs/sfi03_path.md", strings.NewReader(doc))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	var leaf *HeadingUnit
	for i := range units {
		if units[i].IsSemanticLeaf() && strings.Contains(units[i].Content, "deepest paragraph") {
			leaf = &units[i]
			break
		}
	}
	if leaf == nil {
		t.Fatal("expected a semantic leaf for the deepest paragraph")
	}

	wantPath := []string{"Alpha", "Beta", "Gamma"}
	if len(leaf.HeadingPath) != len(wantPath) {
		t.Fatalf("HeadingPath length mismatch: want %v, got %v", wantPath, leaf.HeadingPath)
	}
	for i, w := range wantPath {
		if leaf.HeadingPath[i] != w {
			t.Fatalf("HeadingPath[%d]: want %q, got %q", i, w, leaf.HeadingPath[i])
		}
	}
	if leaf.Title != "Gamma" || leaf.Level != 3 {
		t.Fatalf("leaf must inherit nearest heading metadata, got Title=%q Level=%d",
			leaf.Title, leaf.Level)
	}
}

// TestSFI03ListItemsAreSemanticLeaves: a list under a heading produces
// one semantic leaf per list item (not a single concatenated body on
// the heading).
func TestSFI03ListItemsAreSemanticLeaves(t *testing.T) {
	doc := "# Items\n\n- alpha semantic item\n- beta semantic item\n- gamma semantic item\n"
	units, err := ExtractMarkdownUnitsFromReader("docs/sfi03_list.md", strings.NewReader(doc))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	leafContents := map[string]int{}
	for _, u := range filterSemanticLeaves(units) {
		if u.Kind != KindListItem {
			continue
		}
		// The list item body should round-trip into the leaf content.
		for _, want := range []string{"alpha semantic item", "beta semantic item", "gamma semantic item"} {
			if strings.Contains(u.Content, want) {
				leafContents[want]++
			}
		}
	}
	for _, want := range []string{"alpha semantic item", "beta semantic item", "gamma semantic item"} {
		if leafContents[want] != 1 {
			t.Fatalf("list item %q must appear exactly once as a list_item leaf, got %d",
				want, leafContents[want])
		}
	}

	// The heading entry must not own the list bodies.
	for _, u := range filterHeadingEntries(units) {
		for _, body := range []string{"alpha semantic item", "beta semantic item", "gamma semantic item"} {
			if strings.Contains(u.Content, body) {
				t.Fatalf("heading %q swallowed list body %q", u.Title, body)
			}
		}
	}
}

// TestSFI03LeafIDsUniqueWithinHeading: multiple semantic leaves under
// the same heading must have distinct IDs (the same path+title would
// collide if leaf IDs reused the heading scheme).
func TestSFI03LeafIDsUniqueWithinHeading(t *testing.T) {
	doc := "# One\n\nFirst paragraph.\n\nSecond paragraph.\n"
	units, err := ExtractMarkdownUnitsFromReader("docs/sfi03_ids.md", strings.NewReader(doc))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	seen := map[string]int{}
	for _, u := range units {
		if u.ID == "" {
			t.Fatalf("unit %+v has empty ID", u)
		}
		seen[u.ID]++
	}
	for id, count := range seen {
		if count > 1 {
			t.Fatalf("duplicate unit ID %s appears %d times", id, count)
		}
	}
}

// rangesOverlap returns true when [aStart,aEnd) and [bStart,bEnd)
// share at least one byte. Adjacent ranges (b starts where a ends)
// do not overlap.
func rangesOverlap(aStart, aEnd, bStart, bEnd int) bool {
	if aEnd <= bStart {
		return false
	}
	if bEnd <= aStart {
		return false
	}
	return true
}

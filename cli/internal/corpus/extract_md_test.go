package corpus

import (
	"strings"
	"testing"
)

const fixtureMarkdown = `# Project Overview

This is the introduction to the project.
It spans multiple lines.

## Architecture

The system uses a layered architecture.

### Data Layer

PostgreSQL with read replicas.

### API Layer

REST + gRPC dual protocol.

## Deployment

Kubernetes with Helm charts.

# Appendix

Additional reference material.

#### Deep Heading

This is a level 4 heading.
`

func filterHeadingEntries(units []HeadingUnit) []HeadingUnit {
	out := make([]HeadingUnit, 0, len(units))
	for _, u := range units {
		if u.Kind == HeadingUnitKindHeading {
			out = append(out, u)
		}
	}
	return out
}

func filterSemanticLeaves(units []HeadingUnit) []HeadingUnit {
	out := make([]HeadingUnit, 0, len(units))
	for _, u := range units {
		if u.IsSemanticLeaf() {
			out = append(out, u)
		}
	}
	return out
}

func TestExtractMarkdownUnitsBasic(t *testing.T) {
	units, err := ExtractMarkdownUnitsFromReader("docs/overview.md", strings.NewReader(fixtureMarkdown))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	headings := filterHeadingEntries(units)
	if len(headings) != 7 {
		t.Fatalf("expected 7 heading entries, got %d", len(headings))
	}

	assertUnit(t, headings[0], 1, "Project Overview", 1)
	assertUnit(t, headings[1], 2, "Architecture", 6)
	assertUnit(t, headings[2], 3, "Data Layer", 10)
	assertUnit(t, headings[3], 3, "API Layer", 14)
	assertUnit(t, headings[4], 2, "Deployment", 18)
	assertUnit(t, headings[5], 1, "Appendix", 22)
	assertUnit(t, headings[6], 4, "Deep Heading", 26)

	for _, h := range headings {
		if strings.TrimSpace(h.Content) != "" {
			t.Fatalf("heading %q must not own descendant body, got Content=%q", h.Title, h.Content)
		}
	}
}

func TestExtractMarkdownUnitsContent(t *testing.T) {
	units, _ := ExtractMarkdownUnitsFromReader("test.md", strings.NewReader(fixtureMarkdown))

	leaves := filterSemanticLeaves(units)
	if len(leaves) == 0 {
		t.Fatal("expected at least one semantic leaf")
	}

	// Body text appears once on a semantic leaf, never on a heading.
	wantSubstrings := []string{
		"introduction to the project",
		"PostgreSQL",
	}
	for _, want := range wantSubstrings {
		hits := 0
		for _, u := range units {
			if strings.Contains(u.Content, want) {
				hits++
				if u.Kind == HeadingUnitKindHeading {
					t.Fatalf("body text %q must not appear on a heading entry, got %q", want, u.Title)
				}
			}
		}
		if hits != 1 {
			t.Fatalf("expected body text %q to appear exactly once across units, got %d", want, hits)
		}
	}
}

func TestExtractMarkdownUnitsStableIDs(t *testing.T) {
	units1, _ := ExtractMarkdownUnitsFromReader("path.md", strings.NewReader("# Title\n\nBody\n"))
	units2, _ := ExtractMarkdownUnitsFromReader("path.md", strings.NewReader("# Title\n\nDifferent body\n"))

	if units1[0].ID != units2[0].ID {
		t.Fatalf("IDs should be stable for same path+title: %s vs %s", units1[0].ID, units2[0].ID)
	}
}

func TestExtractMarkdownUnitsDifferentPathsDifferentIDs(t *testing.T) {
	units1, _ := ExtractMarkdownUnitsFromReader("a.md", strings.NewReader("# Title\n"))
	units2, _ := ExtractMarkdownUnitsFromReader("b.md", strings.NewReader("# Title\n"))

	if units1[0].ID == units2[0].ID {
		t.Fatal("IDs should differ for different paths")
	}
}

func TestExtractMarkdownUnitsEmptyFile(t *testing.T) {
	units, err := ExtractMarkdownUnitsFromReader("empty.md", strings.NewReader(""))
	if err != nil {
		t.Fatalf("extract empty: %v", err)
	}
	if len(units) != 0 {
		t.Fatalf("expected 0 units for empty file, got %d", len(units))
	}
}

func TestExtractMarkdownUnitsNoHeadings(t *testing.T) {
	content := "Just a paragraph.\n\nAnother paragraph.\n"
	units, err := ExtractMarkdownUnitsFromReader("plain.md", strings.NewReader(content))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	// SFI-03 (#341): pre-heading semantic content is intentionally
	// not emitted as a unit; SFI-04 will surface it as a coverage
	// finding. The contract here is that no headings means no units.
	if len(units) != 0 {
		t.Fatalf("expected 0 units with no headings, got %d", len(units))
	}
}

func TestExtractMarkdownUnitsH5H6PreservedAsHeadings(t *testing.T) {
	content := "# Valid\n\nContent\n\n##### Too Deep\n\nIgnored\n\n###### Even Deeper\n\nAlso ignored\n"
	units, _ := ExtractMarkdownUnitsFromReader("test.md", strings.NewReader(content))

	headings := filterHeadingEntries(units)
	wantLevels := []int{1, 5, 6}
	if len(headings) != len(wantLevels) {
		t.Fatalf("expected %d heading entries, got %d", len(wantLevels), len(headings))
	}
	for i, want := range wantLevels {
		if headings[i].Level != want {
			t.Fatalf("heading %d: expected level %d, got %d (title=%q)",
				i, want, headings[i].Level, headings[i].Title)
		}
	}
	// H5/H6 must not be folded into the H1's body content.
	h1 := headings[0]
	if strings.Contains(h1.Content, "Too Deep") || strings.Contains(h1.Content, "Even Deeper") {
		t.Fatalf("H5/H6 must remain typed as headings, not folded into H1 content: %q", h1.Content)
	}
	for _, u := range filterSemanticLeaves(units) {
		if strings.Contains(u.Content, "Too Deep") || strings.Contains(u.Content, "Even Deeper") {
			t.Fatalf("H5/H6 lines must not appear inside a semantic leaf: %q", u.Content)
		}
	}
}

func TestExtractMarkdownUnitsTrailingHashes(t *testing.T) {
	content := "## Heading With Trailing ##\n\nBody\n"
	units, _ := ExtractMarkdownUnitsFromReader("test.md", strings.NewReader(content))

	headings := filterHeadingEntries(units)
	if len(headings) != 1 {
		t.Fatalf("expected 1 heading entry, got %d", len(headings))
	}
	if headings[0].Title != "Heading With Trailing" {
		t.Fatalf("expected trailing hashes stripped, got: %q", headings[0].Title)
	}
}

func TestExtractMarkdownUnitsRequiresSpaceAfterHash(t *testing.T) {
	content := "#NotAHeading\n\n# Real Heading\n\nBody\n"
	units, _ := ExtractMarkdownUnitsFromReader("test.md", strings.NewReader(content))

	headings := filterHeadingEntries(units)
	if len(headings) != 1 {
		t.Fatalf("expected 1 heading entry (#NotAHeading is not valid), got %d", len(headings))
	}
	if headings[0].Title != "Real Heading" {
		t.Fatalf("unexpected title: %q", headings[0].Title)
	}
}

func TestUnitIDDeterministic(t *testing.T) {
	id1 := UnitID("docs/spec.md", "API Design")
	id2 := UnitID("docs/spec.md", "API Design")
	if id1 != id2 {
		t.Fatalf("UnitID should be deterministic: %s vs %s", id1, id2)
	}
	if len(id1) != 16 {
		t.Fatalf("expected 16 char hex ID, got %d: %s", len(id1), id1)
	}
}

func TestExtractMarkdownUnitsFromFile(t *testing.T) {
	root := t.TempDir()
	content := "# File Test\n\nBody content.\n\n## Section\n\nMore text.\n"
	writeTestFile(t, root, "doc.md", content)

	units, err := ExtractMarkdownUnits(root + "/doc.md")
	if err != nil {
		t.Fatalf("extract from file: %v", err)
	}
	headings := filterHeadingEntries(units)
	if len(headings) != 2 {
		t.Fatalf("expected 2 heading entries, got %d", len(headings))
	}
	leaves := filterSemanticLeaves(units)
	if len(leaves) != 2 {
		t.Fatalf("expected 2 semantic leaves, got %d", len(leaves))
	}
}

func TestExtractMarkdownUnitsFileNotFound(t *testing.T) {
	_, err := ExtractMarkdownUnits("/nonexistent-file-2002.md")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func assertUnit(t *testing.T, unit HeadingUnit, level int, title string, line int) {
	t.Helper()
	if unit.Level != level {
		t.Fatalf("expected level %d for %q, got %d", level, title, unit.Level)
	}
	if unit.Title != title {
		t.Fatalf("expected title %q, got %q", title, unit.Title)
	}
	if unit.Line != line {
		t.Fatalf("expected line %d for %q, got %d", line, title, unit.Line)
	}
	if unit.ID == "" {
		t.Fatalf("expected non-empty ID for %q", title)
	}
}

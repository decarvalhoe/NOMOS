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

func TestExtractMarkdownUnitsBasic(t *testing.T) {
	units, err := ExtractMarkdownUnitsFromReader("docs/overview.md", strings.NewReader(fixtureMarkdown))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if len(units) != 7 {
		t.Fatalf("expected 7 units, got %d", len(units))
	}

	assertUnit(t, units[0], 1, "Project Overview", 1)
	assertUnit(t, units[1], 2, "Architecture", 6)
	assertUnit(t, units[2], 3, "Data Layer", 10)
	assertUnit(t, units[3], 3, "API Layer", 14)
	assertUnit(t, units[4], 2, "Deployment", 18)
	assertUnit(t, units[5], 1, "Appendix", 22)
	assertUnit(t, units[6], 4, "Deep Heading", 26)
}

func TestExtractMarkdownUnitsContent(t *testing.T) {
	units, _ := ExtractMarkdownUnitsFromReader("test.md", strings.NewReader(fixtureMarkdown))

	if !strings.Contains(units[0].Content, "introduction to the project") {
		t.Fatalf("expected intro content, got: %s", units[0].Content)
	}
	if !strings.Contains(units[2].Content, "PostgreSQL") {
		t.Fatalf("expected data layer content, got: %s", units[2].Content)
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
	if len(units) != 0 {
		t.Fatalf("expected 0 units with no headings, got %d", len(units))
	}
}

func TestExtractMarkdownUnitsIgnoresH5H6(t *testing.T) {
	content := "# Valid\n\nContent\n\n##### Too Deep\n\nIgnored\n\n###### Even Deeper\n\nAlso ignored\n"
	units, _ := ExtractMarkdownUnitsFromReader("test.md", strings.NewReader(content))

	if len(units) != 1 {
		t.Fatalf("expected 1 unit (H5/H6 ignored), got %d", len(units))
	}
	if !strings.Contains(units[0].Content, "Too Deep") {
		t.Fatalf("H5 line should be part of H1 content: %s", units[0].Content)
	}
}

func TestExtractMarkdownUnitsTrailingHashes(t *testing.T) {
	content := "## Heading With Trailing ##\n\nBody\n"
	units, _ := ExtractMarkdownUnitsFromReader("test.md", strings.NewReader(content))

	if len(units) != 1 {
		t.Fatalf("expected 1 unit, got %d", len(units))
	}
	if units[0].Title != "Heading With Trailing" {
		t.Fatalf("expected trailing hashes stripped, got: %q", units[0].Title)
	}
}

func TestExtractMarkdownUnitsRequiresSpaceAfterHash(t *testing.T) {
	content := "#NotAHeading\n\n# Real Heading\n\nBody\n"
	units, _ := ExtractMarkdownUnitsFromReader("test.md", strings.NewReader(content))

	if len(units) != 1 {
		t.Fatalf("expected 1 unit (#NotAHeading is not valid), got %d", len(units))
	}
	if units[0].Title != "Real Heading" {
		t.Fatalf("unexpected title: %q", units[0].Title)
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
	if len(units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(units))
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

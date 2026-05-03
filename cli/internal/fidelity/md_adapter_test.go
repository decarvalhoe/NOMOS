package fidelity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	return string(data)
}

func findNodes(cast CAST, kind NodeKind) []CNode {
	var out []CNode
	for _, n := range cast.Nodes {
		if n.Kind == kind {
			out = append(out, n)
		}
	}
	return out
}

func TestParseMarkdown_RootDocument(t *testing.T) {
	cast := ParseMarkdown("# Title\n\nText.")
	if cast.Root == "" {
		t.Fatal("expected root ID")
	}
	docs := findNodes(cast, KindDocument)
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
}

func TestParseMarkdown_Headings(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	headings := findNodes(cast, KindHeading)
	if len(headings) < 4 {
		t.Fatalf("expected at least 4 headings, got %d", len(headings))
	}
	if cast.Coverage.Headings < 4 {
		t.Fatalf("expected coverage.headings >= 4, got %d", cast.Coverage.Headings)
	}
	// Check levels.
	levels := map[int]int{}
	for _, h := range headings {
		levels[h.Level]++
	}
	if levels[1] != 1 {
		t.Fatalf("expected 1 H1, got %d", levels[1])
	}
	if levels[2] != 2 {
		t.Fatalf("expected 2 H2, got %d", levels[2])
	}
}

func TestParseMarkdown_Paragraphs(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	if cast.Coverage.Paragraphs == 0 {
		t.Fatal("expected paragraphs")
	}
	paras := findNodes(cast, KindParagraph)
	if len(paras) < 3 {
		t.Fatalf("expected at least 3 paragraphs, got %d", len(paras))
	}
}

func TestParseMarkdown_Lists(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	if cast.Coverage.Lists < 2 {
		t.Fatalf("expected at least 2 lists, got %d", cast.Coverage.Lists)
	}
	if cast.Coverage.ListItems < 6 {
		t.Fatalf("expected at least 6 list items, got %d", cast.Coverage.ListItems)
	}
	lists := findNodes(cast, KindList)
	hasOrdered := false
	hasUnordered := false
	for _, l := range lists {
		if l.Props != nil && l.Props["list_type"] == "ordered" {
			hasOrdered = true
		}
		if l.Props != nil && l.Props["list_type"] == "unordered" {
			hasUnordered = true
		}
	}
	if !hasOrdered {
		t.Fatal("expected ordered list")
	}
	if !hasUnordered {
		t.Fatal("expected unordered list")
	}
}

func TestParseMarkdown_CodeBlock(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	if cast.Coverage.CodeBlocks != 1 {
		t.Fatalf("expected 1 code block, got %d", cast.Coverage.CodeBlocks)
	}
	codes := findNodes(cast, KindCodeBlock)
	if len(codes) != 1 {
		t.Fatalf("expected 1 code node, got %d", len(codes))
	}
	if codes[0].Props["language"] != "python" {
		t.Fatalf("expected language python, got %q", codes[0].Props["language"])
	}
	if !strings.Contains(codes[0].Text, "def hello") {
		t.Fatalf("expected code content, got %q", codes[0].Text)
	}
}

func TestParseMarkdown_Blockquote(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	if cast.Coverage.Blockquotes != 1 {
		t.Fatalf("expected 1 blockquote, got %d", cast.Coverage.Blockquotes)
	}
	bqs := findNodes(cast, KindBlockquote)
	if len(bqs) != 1 {
		t.Fatalf("expected 1 blockquote node, got %d", len(bqs))
	}
	if !strings.Contains(bqs[0].Text, "blockquote") {
		t.Fatalf("expected blockquote text, got %q", bqs[0].Text)
	}
}

func TestParseMarkdown_Table(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	if cast.Coverage.Tables != 1 {
		t.Fatalf("expected 1 table, got %d", cast.Coverage.Tables)
	}
	tables := findNodes(cast, KindTable)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table node, got %d", len(tables))
	}
	// Header + 2 data rows = 3 children.
	if len(tables[0].Children) != 3 {
		t.Fatalf("expected 3 table rows, got %d", len(tables[0].Children))
	}
	// Check header row.
	rows := findNodes(cast, KindTableRow)
	if len(rows) < 3 {
		t.Fatalf("expected at least 3 table rows, got %d", len(rows))
	}
	headerRow := rows[0]
	if headerRow.Props["role"] != "header" {
		t.Fatal("expected first row to have role=header")
	}
	cells := findNodes(cast, KindTableCell)
	if len(cells) < 9 {
		t.Fatalf("expected at least 9 cells (3x3), got %d", len(cells))
	}
}

func TestParseMarkdown_ThematicBreaks(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	if cast.Coverage.ThematicBreaks < 2 {
		t.Fatalf("expected at least 2 thematic breaks, got %d", cast.Coverage.ThematicBreaks)
	}
}

func TestParseMarkdown_Links(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	if cast.Coverage.Links < 2 {
		t.Fatalf("expected at least 2 links, got %d", cast.Coverage.Links)
	}
}

func TestParseMarkdown_Images(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	if cast.Coverage.Images < 1 {
		t.Fatalf("expected at least 1 image, got %d", cast.Coverage.Images)
	}
}

func TestParseMarkdown_HTMLBlock(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	if cast.Coverage.HTMLBlocks < 1 {
		t.Fatalf("expected at least 1 HTML block, got %d", cast.Coverage.HTMLBlocks)
	}
	htmlNodes := findNodes(cast, KindHTML)
	if len(htmlNodes) < 1 {
		t.Fatalf("expected HTML node, got %d", len(htmlNodes))
	}
	if htmlNodes[0].Props["tag"] != "div" {
		t.Fatalf("expected tag=div, got %q", htmlNodes[0].Props["tag"])
	}
}

func TestParseMarkdown_ParentChain(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)

	for _, n := range cast.Nodes {
		if n.Kind == KindDocument {
			continue
		}
		if n.ParentID == "" {
			t.Fatalf("node %s (%s) has no parent", n.ID, n.Kind)
		}
	}
}

func TestParseMarkdown_StableIDs(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	c1 := ParseMarkdown(src)
	c2 := ParseMarkdown(src)

	if len(c1.Nodes) != len(c2.Nodes) {
		t.Fatalf("node count changed: %d vs %d", len(c1.Nodes), len(c2.Nodes))
	}
	for i := range c1.Nodes {
		if c1.Nodes[i].ID != c2.Nodes[i].ID {
			t.Fatalf("node[%d] ID unstable: %q vs %q", i, c1.Nodes[i].ID, c2.Nodes[i].ID)
		}
	}
}

func TestParseMarkdown_UniqueIDs(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)
	seen := map[string]bool{}
	for _, n := range cast.Nodes {
		if seen[n.ID] {
			t.Fatalf("duplicate ID: %s (%s)", n.ID, n.Kind)
		}
		seen[n.ID] = true
	}
}

func TestParseMarkdown_SourceHash(t *testing.T) {
	cast := ParseMarkdown("# Test\n\nHello.")
	if cast.SourceHash == "" {
		t.Fatal("expected non-empty source hash")
	}
	if len(cast.SourceHash) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(cast.SourceHash))
	}
}

func TestParseMarkdown_Empty(t *testing.T) {
	cast := ParseMarkdown("")
	if len(cast.Nodes) != 1 {
		t.Fatalf("expected 1 node (document root), got %d", len(cast.Nodes))
	}
}

func TestParseMarkdown_FullCoverage(t *testing.T) {
	src := loadFixture(t, "commonmark-sample.md")
	cast := ParseMarkdown(src)

	cov := cast.Coverage
	checks := map[string]int{
		"headings":        cov.Headings,
		"paragraphs":      cov.Paragraphs,
		"lists":           cov.Lists,
		"list_items":      cov.ListItems,
		"code_blocks":     cov.CodeBlocks,
		"blockquotes":     cov.Blockquotes,
		"tables":          cov.Tables,
		"thematic_breaks": cov.ThematicBreaks,
		"links":           cov.Links,
		"images":          cov.Images,
	}
	for name, count := range checks {
		if count == 0 {
			t.Fatalf("coverage.%s is 0 — missing CommonMark element", name)
		}
	}
}

func TestParseMarkdown_InlineLink(t *testing.T) {
	cast := ParseMarkdown("Click [here](https://example.com) for more.")
	paras := findNodes(cast, KindParagraph)
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
	if paras[0].Props == nil || paras[0].Props["link_0_href"] != "https://example.com" {
		t.Fatalf("expected link prop, got %v", paras[0].Props)
	}
}

func TestParseMarkdown_InlineImage(t *testing.T) {
	cast := ParseMarkdown("See ![logo](img/logo.png) here.")
	paras := findNodes(cast, KindParagraph)
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph, got %d", len(paras))
	}
	if paras[0].Props == nil || paras[0].Props["image_0_src"] != "img/logo.png" {
		t.Fatalf("expected image prop, got %v", paras[0].Props)
	}
}

package fidelity

import (
	"strings"
	"testing"
)

// --- Table ---

func TestParseTable(t *testing.T) {
	lines := []string{
		"| Name | Value | Status |",
		"|---|---|---|",
		"| alpha | 1 | active |",
		"| beta | 2 | draft |",
	}
	tb, consumed := ParseTable(lines, 0)
	if consumed != 4 {
		t.Fatalf("expected 4 lines consumed, got %d", consumed)
	}
	if tb.ColCount != 3 {
		t.Fatalf("expected 3 cols, got %d", tb.ColCount)
	}
	if tb.RowCount != 2 {
		t.Fatalf("expected 2 rows, got %d", tb.RowCount)
	}
	if tb.Headers[0] != "Name" || tb.Headers[2] != "Status" {
		t.Fatalf("unexpected headers: %v", tb.Headers)
	}
	if tb.Rows[0][1] != "1" || tb.Rows[1][0] != "beta" {
		t.Fatalf("unexpected row data: %v", tb.Rows)
	}
	if tb.Span.StartLine != 1 || tb.Span.EndLine != 4 {
		t.Fatalf("unexpected span: %v", tb.Span)
	}
	if tb.Hash == "" {
		t.Fatal("expected non-empty hash")
	}
}

func TestParseTableNoSeparator(t *testing.T) {
	lines := []string{"| A | B |", "| 1 | 2 |"}
	_, consumed := ParseTable(lines, 0)
	if consumed != 0 {
		t.Fatalf("expected 0 consumed without separator, got %d", consumed)
	}
}

func TestParseTableEmpty(t *testing.T) {
	_, consumed := ParseTable(nil, 0)
	if consumed != 0 {
		t.Fatalf("expected 0 consumed, got %d", consumed)
	}
}

func TestTableToCNode(t *testing.T) {
	tb := TableBlock{ID: "tbl-1", Headers: []string{"A", "B"}, ColCount: 2, RowCount: 3,
		Span: Span{1, 5}, Hash: "h"}
	cn := TableToCNode(tb, "parent")
	if cn.Kind != KindTable {
		t.Fatalf("expected table kind, got %q", cn.Kind)
	}
	if cn.Props["col_count"] != "2" || cn.Props["row_count"] != "3" {
		t.Fatalf("unexpected props: %v", cn.Props)
	}
	if cn.ParentID != "parent" {
		t.Fatalf("expected parent, got %q", cn.ParentID)
	}
}

// --- List ---

func TestParseListUnordered(t *testing.T) {
	lines := []string{"- item one", "- item two", "- item three"}
	lb, consumed := ParseList(lines, 0)
	if consumed != 3 {
		t.Fatalf("expected 3, got %d", consumed)
	}
	if lb.Ordered {
		t.Fatal("expected unordered")
	}
	if len(lb.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(lb.Items))
	}
	if lb.Items[0].Text != "item one" {
		t.Fatalf("expected 'item one', got %q", lb.Items[0].Text)
	}
}

func TestParseListOrdered(t *testing.T) {
	lines := []string{"1. first", "2. second"}
	lb, consumed := ParseList(lines, 0)
	if consumed != 2 {
		t.Fatalf("expected 2, got %d", consumed)
	}
	if !lb.Ordered {
		t.Fatal("expected ordered")
	}
	if lb.Items[0].Text != "first" {
		t.Fatalf("expected 'first', got %q", lb.Items[0].Text)
	}
}

func TestParseListNested(t *testing.T) {
	lines := []string{"- top", "  - nested", "- back"}
	lb, consumed := ParseList(lines, 0)
	if consumed != 3 {
		t.Fatalf("expected 3, got %d", consumed)
	}
	if lb.Items[1].Depth != 1 {
		t.Fatalf("expected depth 1 for nested, got %d", lb.Items[1].Depth)
	}
}

func TestParseListEmpty(t *testing.T) {
	_, consumed := ParseList(nil, 0)
	if consumed != 0 {
		t.Fatalf("expected 0, got %d", consumed)
	}
}

func TestListToCNode(t *testing.T) {
	lb := ListBlock{ID: "l-1", Ordered: true,
		Items: []ListItem{{ID: "i1"}, {ID: "i2"}},
		Span: Span{1, 2}, Hash: "h"}
	cn := ListToCNode(lb, "parent")
	if cn.Kind != KindList {
		t.Fatalf("expected list, got %q", cn.Kind)
	}
	if cn.Props["ordered"] != "true" {
		t.Fatalf("expected ordered=true, got %q", cn.Props["ordered"])
	}
	if len(cn.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(cn.Children))
	}
}

// --- Callout ---

func TestParseCalloutNote(t *testing.T) {
	lines := []string{"> [!NOTE] Important info", "> This is the body.", "> Another line."}
	cb, consumed := ParseCallout(lines, 0)
	if cb == nil {
		t.Fatal("expected callout")
	}
	if consumed != 3 {
		t.Fatalf("expected 3, got %d", consumed)
	}
	if cb.Kind != "note" {
		t.Fatalf("expected note, got %q", cb.Kind)
	}
	if cb.Title != "Important info" {
		t.Fatalf("expected title 'Important info', got %q", cb.Title)
	}
	if !strings.Contains(cb.Body, "This is the body.") {
		t.Fatalf("expected body content, got %q", cb.Body)
	}
}

func TestParseCalloutWarning(t *testing.T) {
	lines := []string{"> [!WARNING]", "> Danger ahead."}
	cb, consumed := ParseCallout(lines, 0)
	if cb == nil {
		t.Fatal("expected callout")
	}
	if consumed != 2 {
		t.Fatalf("expected 2, got %d", consumed)
	}
	if cb.Kind != "warning" {
		t.Fatalf("expected warning, got %q", cb.Kind)
	}
}

func TestParseCalloutNotBlockquote(t *testing.T) {
	lines := []string{"Not a blockquote"}
	cb, _ := ParseCallout(lines, 0)
	if cb != nil {
		t.Fatal("expected nil for non-blockquote")
	}
}

func TestParseCalloutPlainBlockquote(t *testing.T) {
	lines := []string{"> Just a regular quote."}
	cb, _ := ParseCallout(lines, 0)
	if cb != nil {
		t.Fatal("expected nil for plain blockquote (no callout prefix)")
	}
}

func TestCalloutToCNode(t *testing.T) {
	cb := CalloutBlock{ID: "c-1", Kind: "tip", Title: "Pro tip", Body: "Do this.",
		Span: Span{1, 3}, Hash: "h"}
	cn := CalloutToCNode(cb, "parent")
	if cn.Kind != KindBlockquote {
		t.Fatalf("expected blockquote, got %q", cn.Kind)
	}
	if cn.Props["callout_kind"] != "tip" {
		t.Fatalf("expected tip, got %q", cn.Props["callout_kind"])
	}
}

// --- Code block ---

func TestParseCodeBlock(t *testing.T) {
	lines := []string{"```python", "def hello():", "    print('hi')", "```"}
	cb, consumed := ParseCodeBlock(lines, 0)
	if consumed != 4 {
		t.Fatalf("expected 4, got %d", consumed)
	}
	if cb.Language != "python" {
		t.Fatalf("expected python, got %q", cb.Language)
	}
	if cb.LineCount != 2 {
		t.Fatalf("expected 2 code lines, got %d", cb.LineCount)
	}
	if !strings.Contains(cb.Code, "def hello():") {
		t.Fatalf("expected code content, got %q", cb.Code)
	}
}

func TestParseCodeBlockNoLang(t *testing.T) {
	lines := []string{"```", "some code", "```"}
	cb, consumed := ParseCodeBlock(lines, 0)
	if consumed != 3 {
		t.Fatalf("expected 3, got %d", consumed)
	}
	if cb.Language != "" {
		t.Fatalf("expected empty language, got %q", cb.Language)
	}
}

func TestCodeBlockToCNode(t *testing.T) {
	cb := CodeBlockParsed{ID: "cb-1", Language: "go", Code: "fmt.Println()", LineCount: 1,
		Span: Span{1, 3}, Hash: "h"}
	cn := CodeBlockToCNode(cb, "parent")
	if cn.Kind != KindCodeBlock {
		t.Fatalf("expected code_block, got %q", cn.Kind)
	}
	if cn.Props["language"] != "go" {
		t.Fatalf("expected go, got %q", cn.Props["language"])
	}
}

// --- Image ---

func TestParseImage(t *testing.T) {
	ib := ParseImage("![Alt text](image.png)", 5)
	if ib == nil {
		t.Fatal("expected image")
	}
	if ib.AltText != "Alt text" {
		t.Fatalf("expected 'Alt text', got %q", ib.AltText)
	}
	if ib.URL != "image.png" {
		t.Fatalf("expected 'image.png', got %q", ib.URL)
	}
	if ib.Span.StartLine != 6 {
		t.Fatalf("expected line 6, got %d", ib.Span.StartLine)
	}
}

func TestParseImageWithTitle(t *testing.T) {
	ib := ParseImage(`![Logo](logo.svg "Company Logo")`, 0)
	if ib == nil {
		t.Fatal("expected image")
	}
	if ib.Title != "Company Logo" {
		t.Fatalf("expected title 'Company Logo', got %q", ib.Title)
	}
}

func TestParseImageNotImage(t *testing.T) {
	ib := ParseImage("Just text", 0)
	if ib != nil {
		t.Fatal("expected nil for non-image")
	}
}

func TestImageToCNode(t *testing.T) {
	ib := ImageBlock{ID: "img-1", AltText: "diagram", URL: "d.png",
		Span: Span{1, 1}, Hash: "h"}
	cn := ImageToCNode(ib, "parent")
	if cn.Kind != KindImage {
		t.Fatalf("expected image, got %q", cn.Kind)
	}
	if cn.Props["url"] != "d.png" {
		t.Fatalf("expected d.png, got %q", cn.Props["url"])
	}
}

// --- Annex ---

func TestParseAnnexHeading(t *testing.T) {
	ab := ParseAnnexHeading("## Annex A: Rate Tables", 10)
	if ab == nil {
		t.Fatal("expected annex")
	}
	if ab.Number != "A" {
		t.Fatalf("expected number 'A', got %q", ab.Number)
	}
	if ab.Title != "Rate Tables" {
		t.Fatalf("expected title 'Rate Tables', got %q", ab.Title)
	}
}

func TestParseAnnexAppendix(t *testing.T) {
	ab := ParseAnnexHeading("# Appendix 1 - Glossary", 0)
	if ab == nil {
		t.Fatal("expected annex")
	}
	if ab.Number != "1" {
		t.Fatalf("expected number '1', got %q", ab.Number)
	}
}

func TestParseAnnexAnnexe(t *testing.T) {
	ab := ParseAnnexHeading("### Annexe B: Références", 0)
	if ab == nil {
		t.Fatal("expected annex")
	}
	if ab.Number != "B" {
		t.Fatalf("expected 'B', got %q", ab.Number)
	}
}

func TestParseAnnexNotAnnex(t *testing.T) {
	ab := ParseAnnexHeading("## Regular Heading", 0)
	if ab != nil {
		t.Fatal("expected nil for non-annex")
	}
}

func TestAnnexToCNode(t *testing.T) {
	ab := AnnexBlock{ID: "ax-1", Title: "Rates", Number: "A",
		Span: Span{1, 1}, Hash: "h"}
	cn := AnnexToCNode(ab, "parent")
	if cn.Props["is_annex"] != "true" {
		t.Fatal("expected is_annex=true")
	}
	if cn.Props["annex_number"] != "A" {
		t.Fatalf("expected annex_number 'A', got %q", cn.Props["annex_number"])
	}
}

// --- Hash determinism ---

func TestBlockHashDeterminism(t *testing.T) {
	lines := []string{"| A | B |", "|---|---|", "| 1 | 2 |"}
	tb1, _ := ParseTable(lines, 0)
	tb2, _ := ParseTable(lines, 0)
	if tb1.Hash != tb2.Hash {
		t.Fatal("table hash not deterministic")
	}
}

// --- Span correctness ---

func TestBlockSpanCorrect(t *testing.T) {
	lines := []string{"- a", "- b", "- c"}
	lb, _ := ParseList(lines, 5)
	if lb.Span.StartLine != 6 {
		t.Fatalf("expected start 6, got %d", lb.Span.StartLine)
	}
	if lb.Span.EndLine != 8 {
		t.Fatalf("expected end 8, got %d", lb.Span.EndLine)
	}
}

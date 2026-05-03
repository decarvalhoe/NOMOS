package fidelity

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"testing"
)

// buildTestDocx creates a minimal valid DOCX (ZIP with word/document.xml).
func buildTestDocx(paragraphs []wxParagraph, tables []wxTable) []byte {
	_ = xml.Name{} // ensure xml import used

	var bodyXML bytes.Buffer
	bodyXML.WriteString(xml.Header)
	bodyXML.WriteString(`<document xmlns="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`)
	bodyXML.WriteString(`<body>`)
	for _, p := range paragraphs {
		bodyXML.WriteString(`<p>`)
		bodyXML.WriteString(`<pPr>`)
		if p.Properties.Style.Val != "" {
			bodyXML.WriteString(fmt.Sprintf(`<pStyle val="%s"/>`, p.Properties.Style.Val))
		}
		bodyXML.WriteString(`</pPr>`)
		for _, r := range p.Runs {
			bodyXML.WriteString(`<r>`)
			for _, t := range r.Text {
				bodyXML.WriteString(fmt.Sprintf(`<t>%s</t>`, t.Content))
			}
			bodyXML.WriteString(`</r>`)
		}
		bodyXML.WriteString(`</p>`)
	}
	for _, tbl := range tables {
		bodyXML.WriteString(`<tbl>`)
		for _, row := range tbl.Rows {
			bodyXML.WriteString(`<tr>`)
			for _, cell := range row.Cells {
				bodyXML.WriteString(`<tc>`)
				for _, cp := range cell.Paragraphs {
					bodyXML.WriteString(`<p><r>`)
					for _, t := range cp.Runs[0].Text {
						bodyXML.WriteString(fmt.Sprintf(`<t>%s</t>`, t.Content))
					}
					bodyXML.WriteString(`</r></p>`)
				}
				bodyXML.WriteString(`</tc>`)
			}
			bodyXML.WriteString(`</tr>`)
		}
		bodyXML.WriteString(`</tbl>`)
	}
	bodyXML.WriteString(`</body></document>`)

	// Package into ZIP.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("word/document.xml")
	w.Write(bodyXML.Bytes())
	zw.Close()

	return buf.Bytes()
}

// simpleDocx builds a test DOCX with headings and paragraphs.
func simpleDocx() []byte {
	paras := []wxParagraph{
		{
			Properties: wxParaProps{Style: wxStyle{Val: "Heading1"}},
			Runs:       []wxRun{{Text: []wxText{{Content: "Title"}}}},
		},
		{
			Runs: []wxRun{{Text: []wxText{{Content: "First paragraph content."}}}},
		},
		{
			Properties: wxParaProps{Style: wxStyle{Val: "Heading2"}},
			Runs:       []wxRun{{Text: []wxText{{Content: "Section One"}}}},
		},
		{
			Runs: []wxRun{{Text: []wxText{{Content: "Section content here."}}}},
		},
	}
	return buildTestDocx(paras, nil)
}

func TestExtractDocxBasic(t *testing.T) {
	data := simpleDocx()
	result, err := ExtractDocxFromBytes(data, "test.docx")
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}

	if result.Status != DocxStatus {
		t.Fatalf("expected status %s, got %s", DocxStatus, result.Status)
	}
	if result.FileName != "test.docx" {
		t.Fatalf("expected test.docx, got %s", result.FileName)
	}
	if result.NodeCount == 0 {
		t.Fatal("expected at least one node")
	}
}

func TestExtractDocxHeadings(t *testing.T) {
	data := simpleDocx()
	result, _ := ExtractDocxFromBytes(data, "test.docx")

	if result.Headings < 2 {
		t.Fatalf("expected at least 2 headings, got %d", result.Headings)
	}

	// First node should be heading level 1.
	foundH1 := false
	for _, n := range result.Nodes {
		if n.Type == "heading" && n.Level == 1 && n.Text == "Title" {
			foundH1 = true
		}
	}
	if !foundH1 {
		t.Fatal("expected heading level 1 with text 'Title'")
	}
}

func TestExtractDocxParagraphs(t *testing.T) {
	data := simpleDocx()
	result, _ := ExtractDocxFromBytes(data, "test.docx")

	if result.Paragraphs < 2 {
		t.Fatalf("expected at least 2 paragraphs, got %d", result.Paragraphs)
	}
}

func TestExtractDocxTable(t *testing.T) {
	table := wxTable{
		Rows: []wxTableRow{
			{Cells: []wxTableCell{
				{Paragraphs: []wxParagraph{{Runs: []wxRun{{Text: []wxText{{Content: "Header A"}}}}}}},
				{Paragraphs: []wxParagraph{{Runs: []wxRun{{Text: []wxText{{Content: "Header B"}}}}}}},
			}},
			{Cells: []wxTableCell{
				{Paragraphs: []wxParagraph{{Runs: []wxRun{{Text: []wxText{{Content: "Cell 1"}}}}}}},
				{Paragraphs: []wxParagraph{{Runs: []wxRun{{Text: []wxText{{Content: "Cell 2"}}}}}}},
			}},
		},
	}
	data := buildTestDocx(nil, []wxTable{table})
	result, err := ExtractDocxFromBytes(data, "tables.docx")
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}

	if result.Tables < 1 {
		t.Fatalf("expected at least 1 table, got %d", result.Tables)
	}
	// Table node should contain cell text.
	found := false
	for _, n := range result.Nodes {
		if n.Type == "table" && n.Text != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected table node with text content")
	}
}

func TestExtractDocxInvalidZip(t *testing.T) {
	_, err := ExtractDocxFromBytes([]byte("not a zip"), "bad.docx")
	if err == nil {
		t.Fatal("expected error for invalid zip")
	}
}

func TestExtractDocxMissingDocumentXML(t *testing.T) {
	// Create ZIP without word/document.xml.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("other/file.txt")
	w.Write([]byte("hello"))
	zw.Close()

	_, err := ExtractDocxFromBytes(buf.Bytes(), "empty.docx")
	if err == nil {
		t.Fatal("expected error for missing document.xml")
	}
}

func TestExtractDocxEmptyDocument(t *testing.T) {
	// Document with no paragraphs.
	data := buildTestDocx(nil, nil)
	result, err := ExtractDocxFromBytes(data, "empty-doc.docx")
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if result.NodeCount != 0 {
		t.Fatalf("expected 0 nodes for empty doc, got %d", result.NodeCount)
	}
}

func TestExtractDocxHeadingLevelDetection(t *testing.T) {
	cases := []struct {
		style    string
		expected int
	}{
		{"Heading1", 1},
		{"Heading2", 2},
		{"Heading3", 3},
		{"Titre1", 1},
		{"Titre2", 2},
	}
	for _, tc := range cases {
		got := detectHeadingLevel(tc.style, "")
		if got != tc.expected {
			t.Errorf("detectHeadingLevel(%q) = %d, want %d", tc.style, got, tc.expected)
		}
	}
}

func TestExtractDocxNonHeadingStyle(t *testing.T) {
	if detectHeadingLevel("Normal", "") != 0 {
		t.Fatal("Normal style should not be a heading")
	}
	if detectHeadingLevel("ListParagraph", "") != 0 {
		t.Fatal("ListParagraph should not be a heading")
	}
}

func TestExtractDocxOutlineLevel(t *testing.T) {
	if detectHeadingLevel("", "0") != 1 {
		t.Fatal("outline level 0 should map to heading 1")
	}
	if detectHeadingLevel("", "2") != 3 {
		t.Fatal("outline level 2 should map to heading 3")
	}
}

func TestDocxNodeIDs(t *testing.T) {
	data := simpleDocx()
	result, _ := ExtractDocxFromBytes(data, "test.docx")

	ids := map[string]bool{}
	for _, n := range result.Nodes {
		if n.ID == "" {
			t.Fatal("node has empty ID")
		}
		if ids[n.ID] {
			t.Fatalf("duplicate node ID: %s", n.ID)
		}
		ids[n.ID] = true
	}
}

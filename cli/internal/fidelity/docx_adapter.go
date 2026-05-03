package fidelity

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// DocxStatus indicates the feasibility status of this adapter.
const DocxStatus = "feasibility"

// DocxNode is a structural element extracted from a DOCX file.
type DocxNode struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"` // "heading", "paragraph", "table", "list_item"
	Level    int      `json:"level,omitempty"` // heading level 1-9
	Text     string   `json:"text"`
	Style    string   `json:"style,omitempty"`
	ParaIdx  int      `json:"para_idx"` // sequential paragraph index in document
	Children []string `json:"children,omitempty"`
}

// DocxExtraction is the result of extracting structure from a DOCX file.
type DocxExtraction struct {
	Status     string     `json:"status"` // "feasibility"
	FileName   string     `json:"file_name"`
	NodeCount  int        `json:"node_count"`
	Nodes      []DocxNode `json:"nodes"`
	Headings   int        `json:"headings"`
	Paragraphs int        `json:"paragraphs"`
	Tables     int        `json:"tables"`
	Errors     []string   `json:"errors,omitempty"`
}

// ExtractDocx extracts structure from a DOCX file (ZIP-packaged OOXML).
// This is a feasibility-level implementation — sufficient to prove extraction
// is viable but not production-hardened.
func ExtractDocx(r io.ReaderAt, size int64, fileName string) (DocxExtraction, error) {
	result := DocxExtraction{
		Status:   DocxStatus,
		FileName: fileName,
	}

	zr, err := zip.NewReader(r, size)
	if err != nil {
		return result, fmt.Errorf("open zip: %w", err)
	}

	// Find word/document.xml — the main content part.
	var docFile *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			docFile = f
			break
		}
	}
	if docFile == nil {
		return result, fmt.Errorf("word/document.xml not found in archive")
	}

	rc, err := docFile.Open()
	if err != nil {
		return result, fmt.Errorf("open document.xml: %w", err)
	}
	defer rc.Close()

	nodes, errs := parseDocumentXML(rc)
	result.Nodes = nodes
	result.NodeCount = len(nodes)
	result.Errors = errs

	for _, n := range nodes {
		switch n.Type {
		case "heading":
			result.Headings++
		case "paragraph":
			result.Paragraphs++
		case "table":
			result.Tables++
		}
	}

	return result, nil
}

// ExtractDocxFromBytes is a convenience wrapper for in-memory DOCX content.
func ExtractDocxFromBytes(data []byte, fileName string) (DocxExtraction, error) {
	reader := newBytesReaderAt(data)
	return ExtractDocx(reader, int64(len(data)), fileName)
}

// --- OOXML parsing (feasibility level) ---

// Minimal OOXML structures for document.xml parsing.
type wxDocument struct {
	Body wxBody `xml:"body"`
}

type wxBody struct {
	Paragraphs []wxParagraph `xml:"p"`
	Tables     []wxTable     `xml:"tbl"`
	// Note: in real OOXML, body contains mixed p/tbl in order.
	// For feasibility, we parse them separately.
	Content []wxBodyElement `xml:",any"`
}

type wxBodyElement struct {
	XMLName xml.Name
	Inner   []byte `xml:",innerxml"`
}

type wxParagraph struct {
	Properties wxParaProps `xml:"pPr"`
	Runs       []wxRun     `xml:"r"`
}

type wxParaProps struct {
	Style    wxStyle    `xml:"pStyle"`
	NumPr    wxNumPr    `xml:"numPr"`
	OutlvlPr wxOutlvl   `xml:"outlineLvl"`
}

type wxStyle struct {
	Val string `xml:"val,attr"`
}

type wxNumPr struct {
	ILvl wxILvl `xml:"ilvl"`
}

type wxILvl struct {
	Val string `xml:"val,attr"`
}

type wxOutlvl struct {
	Val string `xml:"val,attr"`
}

type wxRun struct {
	Text []wxText `xml:"t"`
}

type wxText struct {
	Content string `xml:",chardata"`
}

type wxTable struct {
	Rows []wxTableRow `xml:"tr"`
}

type wxTableRow struct {
	Cells []wxTableCell `xml:"tc"`
}

type wxTableCell struct {
	Paragraphs []wxParagraph `xml:"p"`
}

func parseDocumentXML(r io.Reader) ([]DocxNode, []string) {
	var doc wxDocument
	var errs []string

	decoder := xml.NewDecoder(r)
	if err := decoder.Decode(&doc); err != nil {
		errs = append(errs, fmt.Sprintf("xml decode: %v", err))
		return nil, errs
	}

	var nodes []DocxNode
	paraIdx := 0

	for _, p := range doc.Body.Paragraphs {
		text := extractRunText(p.Runs)
		style := p.Properties.Style.Val
		level := detectHeadingLevel(style, p.Properties.OutlvlPr.Val)

		nodeType := "paragraph"
		if level > 0 {
			nodeType = "heading"
		} else if p.Properties.NumPr.ILvl.Val != "" {
			nodeType = "list_item"
		}

		if text == "" && nodeType == "paragraph" {
			paraIdx++
			continue
		}

		node := DocxNode{
			ID:      fmt.Sprintf("docx-node-%d", paraIdx),
			Type:    nodeType,
			Level:   level,
			Text:    text,
			Style:   style,
			ParaIdx: paraIdx,
		}
		nodes = append(nodes, node)
		paraIdx++
	}

	// Extract tables.
	for i, tbl := range doc.Body.Tables {
		text := extractTableText(tbl)
		nodes = append(nodes, DocxNode{
			ID:      fmt.Sprintf("docx-table-%d", i),
			Type:    "table",
			Text:    text,
			ParaIdx: paraIdx,
		})
		paraIdx++
	}

	return nodes, errs
}

func extractRunText(runs []wxRun) string {
	var sb strings.Builder
	for _, run := range runs {
		for _, t := range run.Text {
			sb.WriteString(t.Content)
		}
	}
	return strings.TrimSpace(sb.String())
}

func extractTableText(tbl wxTable) string {
	var cells []string
	for _, row := range tbl.Rows {
		for _, cell := range row.Cells {
			for _, p := range cell.Paragraphs {
				text := extractRunText(p.Runs)
				if text != "" {
					cells = append(cells, text)
				}
			}
		}
	}
	if len(cells) > 5 {
		return strings.Join(cells[:5], " | ") + " ..."
	}
	return strings.Join(cells, " | ")
}

func detectHeadingLevel(style string, outlineLvl string) int {
	// Common DOCX heading styles: Heading1, Heading2, ... or Titre1, Titre2
	lower := strings.ToLower(style)
	if strings.HasPrefix(lower, "heading") || strings.HasPrefix(lower, "titre") {
		for i := 1; i <= 9; i++ {
			if strings.Contains(lower, fmt.Sprintf("%d", i)) {
				return i
			}
		}
		return 1
	}
	// Check outline level.
	if outlineLvl != "" {
		for i := 0; i <= 8; i++ {
			if outlineLvl == fmt.Sprintf("%d", i) {
				return i + 1
			}
		}
	}
	return 0
}

// bytesReaderAt wraps a byte slice to implement io.ReaderAt.
type bytesReaderAt struct {
	data []byte
}

func newBytesReaderAt(data []byte) *bytesReaderAt {
	return &bytesReaderAt{data: data}
}

func (r *bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(r.data)) {
		return 0, io.EOF
	}
	n := copy(p, r.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

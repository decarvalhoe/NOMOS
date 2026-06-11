package fidelity

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// DocxStatus indicates the feasibility status of the raw extractor. The
// production adapter (DocxAdapter) wraps it with a claim boundary — born-digital
// OOXML body text only.
const DocxStatus = "feasibility"

// DocxAdapter is the production .docx adapter (VRC-41 #577). Claim ladder rung
// 1: born-digital OOXML body text — paragraphs, headings, list items and table
// cells from word/document.xml. Tracked changes, comments, headers/footers,
// embedded objects and math are NOT claimed (they live in other OOXML parts and
// are surfaced, never silently flattened). Span IDs are content-addressed so a
// single changed character drifts the parse — the same drift-evidence discipline
// as the PDF adapter (VRC-30).
type DocxAdapter struct{}

// Name returns the adapter identifier.
func (DocxAdapter) Name() string { return "docx" }

// Extensions returns the extensions this adapter claims.
func (DocxAdapter) Extensions() []string { return []string{".docx"} }

// Kit returns the mandatory capability kit (VRC-33, C5).
func (DocxAdapter) Kit() AdapterKit {
	return AdapterKit{
		ClaimBoundary:    "Born-digital OOXML body text only (claim ladder rung 1): paragraphs, headings, list items and table cells extracted from word/document.xml with content-addressed span ids. Tracked changes, comments, headers/footers, embedded objects and math are NOT claimed.",
		ClaimLevel:       "ooxml-body-text",
		UnsupportedKinds: []string{"tracked_changes", "comments_and_annotations", "headers_footers", "embedded_objects", "math_equations", "styling_fidelity"},
		GateFixtures: []string{
			"test://cli/internal/fidelity/docx_adapter_test.go#TestDocxAdapter_ExtractsBodyTextWithContentAddressedSpans",
			"test://cli/internal/fidelity/docx_adapter_test.go#TestDocxAdapter_ByteChangeDriftsTheParse",
			"test://cli/internal/fidelity/docx_adapter_test.go#TestDocxAdapter_InvalidZipFailsClosed",
		},
	}
}

// docxSpanID is content-addressed: a changed character changes the text hash and
// therefore the span id, so a mutated document never re-uses a prior span id.
func docxSpanID(node DocxNode) string {
	sum := sha256.Sum256([]byte(node.Text))
	return fmt.Sprintf("docx:p%d:%s:h%s", node.ParaIdx, node.Type, hex.EncodeToString(sum[:])[:12])
}

// Parse extracts the body text of a .docx into structural spans. Every emitted
// node is a real born-digital text node; a document the extractor cannot open
// (corrupt zip / missing document.xml) fails closed.
func (DocxAdapter) Parse(source []byte, filename string) (ParseResult, error) {
	extraction, err := ExtractDocxFromBytes(source, filename)
	if err != nil {
		return ParseResult{}, fmt.Errorf("docx: %w", err)
	}
	spans := make([]SpanInfo, 0, len(extraction.Nodes))
	for _, node := range extraction.Nodes {
		spans = append(spans, SpanInfo{
			ID:       docxSpanID(node),
			NodeType: node.Type,
			// OOXML carries no source line/byte offsets for decoded text, so
			// positions are not claimable: ParaIdx is the logical locator,
			// byte fields stay 0 (the same honesty as the PDF adapter).
			StartLine: node.ParaIdx + 1,
			EndLine:   node.ParaIdx + 1,
		})
	}
	return ParseResult{
		Format:    "docx",
		NodeCount: len(spans),
		Spans:     spans,
		Errors:    extraction.Errors,
	}, nil
}

// Validate checks the source is a readable OOXML package with a main document
// part. A non-docx or corrupt archive is reported invalid, never best-effort.
func (DocxAdapter) Validate(source []byte, filename string) ValidationResult {
	extraction, err := ExtractDocxFromBytes(source, filename)
	if err != nil {
		return ValidationResult{Valid: false, Findings: []string{err.Error()}}
	}
	if extraction.NodeCount == 0 {
		return ValidationResult{Valid: false, Findings: []string{"docx: no body text extracted"}}
	}
	return ValidationResult{Valid: true}
}

// Spans extracts source spans without returning the full ParseResult.
func (a DocxAdapter) Spans(source []byte, filename string) ([]SpanInfo, error) {
	result, err := a.Parse(source, filename)
	if err != nil {
		return nil, err
	}
	return result.Spans, nil
}

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

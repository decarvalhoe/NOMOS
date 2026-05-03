package fidelity

import (
	"fmt"
	"regexp"
	"strings"
)

// --- Structured block types ---

// TableBlock represents a parsed Markdown table.
type TableBlock struct {
	ID       string     `json:"id"`
	Headers  []string   `json:"headers"`
	Rows     [][]string `json:"rows"`
	ColCount int        `json:"col_count"`
	RowCount int        `json:"row_count"`
	Span     Span       `json:"span"`
	Hash     string     `json:"hash"`
}

// ListBlock represents a parsed Markdown list.
type ListBlock struct {
	ID      string      `json:"id"`
	Ordered bool        `json:"ordered"`
	Items   []ListItem  `json:"items"`
	Span    Span        `json:"span"`
	Hash    string      `json:"hash"`
}

// ListItem is a single item in a list.
type ListItem struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Depth   int    `json:"depth"`
	Index   int    `json:"index"`
}

// CalloutBlock represents an admonition/callout (blockquote with prefix).
type CalloutBlock struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"` // note, warning, tip, important, caution
	Title   string `json:"title"`
	Body    string `json:"body"`
	Span    Span   `json:"span"`
	Hash    string `json:"hash"`
}

// CodeBlockParsed represents a fenced code block with language.
type CodeBlockParsed struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Code     string `json:"code"`
	LineCount int   `json:"line_count"`
	Span     Span   `json:"span"`
	Hash     string `json:"hash"`
}

// ImageBlock represents an image reference.
type ImageBlock struct {
	ID      string `json:"id"`
	AltText string `json:"alt_text"`
	URL     string `json:"url"`
	Title   string `json:"title,omitempty"`
	Span    Span   `json:"span"`
	Hash    string `json:"hash"`
}

// AnnexBlock represents a document annex (heading-delimited section).
type AnnexBlock struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Number  string   `json:"number"`
	Content []string `json:"content"` // child node IDs
	Span    Span     `json:"span"`
	Hash    string   `json:"hash"`
}

// --- Parsers ---

var (
	calloutPrefixRe = regexp.MustCompile(`(?i)^\[!(NOTE|WARNING|TIP|IMPORTANT|CAUTION)\](.*)$`)
	annexHeadingRe  = regexp.MustCompile(`(?i)^(#{1,3})\s+(annex|annexe|appendix|schedule)\s+([A-Za-z0-9]+)[:\s-]*(.*)$`)
	imgFullRe       = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)]+?)(?:\s+"([^"]+)")?\)$`)
)

// ParseTable parses lines starting at startLine as a Markdown table.
// Returns the parsed table and number of lines consumed.
func ParseTable(lines []string, startLine int) (TableBlock, int) {
	if len(lines) < 2 {
		return TableBlock{}, 0
	}

	headers := parseBlockCells(lines[0])
	if len(headers) == 0 {
		return TableBlock{}, 0
	}

	// Line 2 must be separator
	if !tableSepRe.MatchString(strings.TrimSpace(lines[1])) {
		return TableBlock{}, 0
	}

	var rows [][]string
	consumed := 2
	for i := 2; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !tableRowRe.MatchString(trimmed) {
			break
		}
		rows = append(rows, parseBlockCells(lines[i]))
		consumed++
	}

	raw := strings.Join(lines[:consumed], "\n")
	tb := TableBlock{
		ID:       nodeID("table", startLine),
		Headers:  headers,
		Rows:     rows,
		ColCount: len(headers),
		RowCount: len(rows),
		Span:     Span{StartLine: startLine + 1, EndLine: startLine + consumed},
		Hash:     hashStr(raw),
	}
	return tb, consumed
}

// ParseList parses lines as a Markdown list (ordered or unordered).
// Returns the parsed list and number of lines consumed.
func ParseList(lines []string, startLine int) (ListBlock, int) {
	if len(lines) == 0 {
		return ListBlock{}, 0
	}

	firstTrimmed := strings.TrimSpace(lines[0])
	ordered := olItemRe.MatchString(firstTrimmed)
	unordered := ulItemRe.MatchString(firstTrimmed)
	if !ordered && !unordered {
		return ListBlock{}, 0
	}

	var items []ListItem
	consumed := 0
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			break
		}

		var text string
		var depth int
		if m := ulItemRe.FindStringSubmatch(lines[i]); m != nil {
			depth = len(m[1]) / 2
			text = m[2]
		} else if m := olItemRe.FindStringSubmatch(lines[i]); m != nil {
			depth = len(m[1]) / 2
			text = m[2]
		} else {
			break
		}

		items = append(items, ListItem{
			ID:    nodeID("list_item", startLine+i),
			Text:  strings.TrimSpace(text),
			Depth: depth,
			Index: len(items),
		})
		consumed++
	}

	if len(items) == 0 {
		return ListBlock{}, 0
	}

	raw := strings.Join(lines[:consumed], "\n")
	return ListBlock{
		ID:      nodeID("list", startLine),
		Ordered: ordered,
		Items:   items,
		Span:    Span{StartLine: startLine + 1, EndLine: startLine + consumed},
		Hash:    hashStr(raw),
	}, consumed
}

// ParseCallout parses a blockquote as a GitHub-style callout.
// Returns nil if the blockquote is not a callout.
func ParseCallout(lines []string, startLine int) (*CalloutBlock, int) {
	if len(lines) == 0 {
		return nil, 0
	}

	// Strip > prefix from all lines
	var stripped []string
	consumed := 0
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(trimmed, ">") {
			break
		}
		text := strings.TrimPrefix(trimmed, ">")
		text = strings.TrimPrefix(text, " ")
		stripped = append(stripped, text)
		consumed++
	}

	if len(stripped) == 0 {
		return nil, 0
	}

	// Check for callout prefix on first line
	m := calloutPrefixRe.FindStringSubmatch(stripped[0])
	if m == nil {
		return nil, 0
	}

	kind := strings.ToLower(m[1])
	title := strings.TrimSpace(m[2])
	if title == "" {
		title = strings.Title(kind)
	}

	body := ""
	if len(stripped) > 1 {
		body = strings.TrimSpace(strings.Join(stripped[1:], "\n"))
	}

	raw := strings.Join(lines[:consumed], "\n")
	return &CalloutBlock{
		ID:    nodeID("callout", startLine),
		Kind:  kind,
		Title: title,
		Body:  body,
		Span:  Span{StartLine: startLine + 1, EndLine: startLine + consumed},
		Hash:  hashStr(raw),
	}, consumed
}

// ParseCodeBlock parses a fenced code block.
// Returns the parsed block and number of lines consumed.
func ParseCodeBlock(lines []string, startLine int) (CodeBlockParsed, int) {
	if len(lines) == 0 {
		return CodeBlockParsed{}, 0
	}

	m := codeOpenRe.FindStringSubmatch(strings.TrimSpace(lines[0]))
	if m == nil {
		return CodeBlockParsed{}, 0
	}
	lang := m[1]

	var codeLines []string
	consumed := 1
	for i := 1; i < len(lines); i++ {
		consumed++
		if strings.TrimSpace(lines[i]) == "```" {
			break
		}
		codeLines = append(codeLines, lines[i])
	}

	code := strings.Join(codeLines, "\n")
	raw := strings.Join(lines[:consumed], "\n")

	return CodeBlockParsed{
		ID:        nodeID("code_block", startLine),
		Language:  lang,
		Code:      code,
		LineCount: len(codeLines),
		Span:      Span{StartLine: startLine + 1, EndLine: startLine + consumed},
		Hash:      hashStr(raw),
	}, consumed
}

// ParseImage parses an image line.
func ParseImage(line string, lineNum int) *ImageBlock {
	trimmed := strings.TrimSpace(line)
	m := imgFullRe.FindStringSubmatch(trimmed)
	if m == nil {
		return nil
	}

	ib := &ImageBlock{
		ID:      nodeID("image", lineNum),
		AltText: m[1],
		URL:     m[2],
		Span:    Span{StartLine: lineNum + 1, EndLine: lineNum + 1},
		Hash:    hashStr(trimmed),
	}
	if len(m) > 3 {
		ib.Title = m[3]
	}
	return ib
}

// ParseAnnexHeading checks if a heading line declares an annex.
func ParseAnnexHeading(line string, lineNum int) *AnnexBlock {
	m := annexHeadingRe.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return nil
	}
	return &AnnexBlock{
		ID:     nodeID("annex", lineNum),
		Title:  strings.TrimSpace(m[4]),
		Number: m[3],
		Span:   Span{StartLine: lineNum + 1, EndLine: lineNum + 1},
		Hash:   hashStr(line),
	}
}

// --- Block to CNode converters ---

// TableToCNode converts a TableBlock to a CNode for the CAST.
func TableToCNode(tb TableBlock, parentID string) CNode {
	return CNode{
		ID: tb.ID, Kind: KindTable, ParentID: parentID,
		Text:    fmt.Sprintf("%d×%d table", tb.RowCount, tb.ColCount),
		RawText: fmt.Sprintf("| %s |", strings.Join(tb.Headers, " | ")),
		Span:    tb.Span, Hash: tb.Hash,
		Props: map[string]string{
			"col_count": fmt.Sprintf("%d", tb.ColCount),
			"row_count": fmt.Sprintf("%d", tb.RowCount),
			"headers":   strings.Join(tb.Headers, ","),
		},
	}
}

// ListToCNode converts a ListBlock to a CNode.
func ListToCNode(lb ListBlock, parentID string) CNode {
	ordered := "false"
	if lb.Ordered {
		ordered = "true"
	}
	childIDs := make([]string, 0, len(lb.Items))
	for _, item := range lb.Items {
		childIDs = append(childIDs, item.ID)
	}
	return CNode{
		ID: lb.ID, Kind: KindList, ParentID: parentID,
		Text:     fmt.Sprintf("%d items", len(lb.Items)),
		Children: childIDs,
		Span:     lb.Span, Hash: lb.Hash,
		Props:    map[string]string{"ordered": ordered, "item_count": fmt.Sprintf("%d", len(lb.Items))},
	}
}

// CalloutToCNode converts a CalloutBlock to a CNode.
func CalloutToCNode(cb CalloutBlock, parentID string) CNode {
	return CNode{
		ID: cb.ID, Kind: KindBlockquote, ParentID: parentID,
		Text:    cb.Body, RawText: fmt.Sprintf("[!%s] %s", strings.ToUpper(cb.Kind), cb.Title),
		Span:    cb.Span, Hash: cb.Hash,
		Props:   map[string]string{"callout_kind": cb.Kind, "callout_title": cb.Title},
	}
}

// CodeBlockToCNode converts a CodeBlockParsed to a CNode.
func CodeBlockToCNode(cb CodeBlockParsed, parentID string) CNode {
	return CNode{
		ID: cb.ID, Kind: KindCodeBlock, ParentID: parentID,
		Text: cb.Code, Span: cb.Span, Hash: cb.Hash,
		Props: map[string]string{"language": cb.Language, "line_count": fmt.Sprintf("%d", cb.LineCount)},
	}
}

// ImageToCNode converts an ImageBlock to a CNode.
func ImageToCNode(ib ImageBlock, parentID string) CNode {
	return CNode{
		ID: ib.ID, Kind: KindImage, ParentID: parentID,
		Text: ib.AltText, Span: ib.Span, Hash: ib.Hash,
		Props: map[string]string{"url": ib.URL, "alt_text": ib.AltText, "title": ib.Title},
	}
}

// AnnexToCNode converts an AnnexBlock to a CNode.
func AnnexToCNode(ab AnnexBlock, parentID string) CNode {
	return CNode{
		ID: ab.ID, Kind: KindHeading, ParentID: parentID,
		Text: fmt.Sprintf("Annex %s: %s", ab.Number, ab.Title),
		Span: ab.Span, Hash: ab.Hash,
		Props: map[string]string{"annex_number": ab.Number, "annex_title": ab.Title, "is_annex": "true"},
	}
}

// --- helpers ---

func parseBlockCells(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.Trim(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

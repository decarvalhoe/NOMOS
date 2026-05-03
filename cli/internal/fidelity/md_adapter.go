package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// NodeKind enumerates CommonMark block and inline node types.
type NodeKind string

const (
	KindDocument       NodeKind = "document"
	KindHeading        NodeKind = "heading"
	KindParagraph      NodeKind = "paragraph"
	KindBlockquote     NodeKind = "blockquote"
	KindList           NodeKind = "list"
	KindListItem       NodeKind = "list_item"
	KindCodeBlock      NodeKind = "code_block"
	KindThematicBreak  NodeKind = "thematic_break"
	KindTable          NodeKind = "table"
	KindTableRow       NodeKind = "table_row"
	KindTableCell      NodeKind = "table_cell"
	KindLink           NodeKind = "link"
	KindImage          NodeKind = "image"
	KindHTML           NodeKind = "html_block"
)

// CNode is a node in the Canonical AST.
type CNode struct {
	ID       string            `json:"id"`
	Kind     NodeKind          `json:"kind"`
	Text     string            `json:"text"`
	RawText  string            `json:"raw_text"`
	Level    int               `json:"level,omitempty"`
	Props    map[string]string `json:"props,omitempty"`
	Children []string          `json:"children,omitempty"`
	ParentID string            `json:"parent_id,omitempty"`
	Span     Span              `json:"span"`
	Hash     string            `json:"hash"`
}

// Span locates a node in the source.
type Span struct {
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
}

// CAST is the Canonical AST output.
type CAST struct {
	Root       string  `json:"root"`
	Nodes      []CNode `json:"nodes"`
	SourceHash string  `json:"source_hash"`
	Coverage   Coverage `json:"coverage"`
}

// Coverage reports which CommonMark elements were detected.
type Coverage struct {
	Headings       int `json:"headings"`
	Paragraphs     int `json:"paragraphs"`
	Lists          int `json:"lists"`
	ListItems      int `json:"list_items"`
	CodeBlocks     int `json:"code_blocks"`
	Blockquotes    int `json:"blockquotes"`
	Tables         int `json:"tables"`
	ThematicBreaks int `json:"thematic_breaks"`
	Links          int `json:"links"`
	Images         int `json:"images"`
	HTMLBlocks     int `json:"html_blocks"`
}

var (
	headingRe      = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	codeOpenRe     = regexp.MustCompile("^```(\\w*)\\s*$")
	thematicRe     = regexp.MustCompile(`^(\*\*\*|---|___)\s*$`)
	ulItemRe       = regexp.MustCompile(`^(\s*)[-*+]\s+(.+)$`)
	olItemRe       = regexp.MustCompile(`^(\s*)\d+[.)]\s+(.+)$`)
	blockquoteRe   = regexp.MustCompile(`^>\s?(.*)$`)
	tableRowRe     = regexp.MustCompile(`^\|(.+)\|$`)
	tableSepRe     = regexp.MustCompile(`^\|[\s:_-]+(\|[\s:_-]+)+\|$`)
	linkRe         = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	imageRe        = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	htmlBlockRe    = regexp.MustCompile(`^<([a-zA-Z][a-zA-Z0-9]*)\b[^>]*>`)
	htmlCloseRe    = regexp.MustCompile(`^</([a-zA-Z][a-zA-Z0-9]*)>`)
)

// ParseMarkdown parses Markdown source into a Canonical AST.
func ParseMarkdown(source string) CAST {
	lines := strings.Split(source, "\n")
	cast := CAST{SourceHash: hashStr(source)}
	var cov Coverage

	rootID := nodeID("document", 0)
	root := CNode{
		ID:   rootID,
		Kind: KindDocument,
		Span: Span{StartLine: 1, EndLine: len(lines)},
		Hash: hashStr(source),
	}
	cast.Root = rootID
	cast.Nodes = append(cast.Nodes, root)

	var headingStack [7]string
	headingStack[0] = rootID

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Blank line — skip.
		if trimmed == "" {
			i++
			continue
		}

		// Thematic break.
		if thematicRe.MatchString(trimmed) {
			parent := currentParent(headingStack)
			n := CNode{
				ID: nodeID("thematic_break", i), Kind: KindThematicBreak,
				RawText: line, Span: Span{StartLine: i + 1, EndLine: i + 1},
				Hash: hashStr(line), ParentID: parent,
			}
			cast.Nodes = append(cast.Nodes, n)
			addChildRef(&cast, parent, n.ID)
			cov.ThematicBreaks++
			i++
			continue
		}

		// Heading.
		if m := headingRe.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			title := strings.TrimSpace(m[2])
			parent := parentForLevel(headingStack, level)
			n := CNode{
				ID: nodeID("heading", i), Kind: KindHeading, Level: level,
				Text: title, RawText: line,
				Span: Span{StartLine: i + 1, EndLine: i + 1},
				Hash: hashStr(line), ParentID: parent,
			}
			// Extract inline links/images from heading text.
			n.Props = extractInlineRefs(title)
			cast.Nodes = append(cast.Nodes, n)
			addChildRef(&cast, parent, n.ID)
			headingStack[level] = n.ID
			for l := level + 1; l <= 6; l++ {
				headingStack[l] = ""
			}
			cov.Headings++
			i++
			continue
		}

		// Fenced code block.
		if m := codeOpenRe.FindStringSubmatch(line); m != nil {
			start := i
			lang := m[1]
			i++
			var body []string
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				body = append(body, lines[i])
				i++
			}
			end := i
			if i < len(lines) {
				i++
			}
			parent := currentParent(headingStack)
			content := strings.Join(body, "\n")
			raw := strings.Join(lines[start:min(i, len(lines))], "\n")
			props := map[string]string{}
			if lang != "" {
				props["language"] = lang
			}
			n := CNode{
				ID: nodeID("code_block", start), Kind: KindCodeBlock,
				Text: content, RawText: raw,
				Span:  Span{StartLine: start + 1, EndLine: end + 1},
				Hash:  hashStr(content), ParentID: parent, Props: props,
			}
			cast.Nodes = append(cast.Nodes, n)
			addChildRef(&cast, parent, n.ID)
			cov.CodeBlocks++
			continue
		}

		// HTML block.
		if htmlBlockRe.MatchString(trimmed) {
			start := i
			tag := htmlBlockRe.FindStringSubmatch(trimmed)[1]
			closeTag := "</" + tag + ">"
			i++
			for i < len(lines) && !strings.Contains(lines[i], closeTag) {
				i++
			}
			if i < len(lines) {
				i++
			}
			raw := strings.Join(lines[start:min(i, len(lines))], "\n")
			parent := currentParent(headingStack)
			n := CNode{
				ID: nodeID("html_block", start), Kind: KindHTML,
				Text: raw, RawText: raw,
				Span:  Span{StartLine: start + 1, EndLine: i},
				Hash:  hashStr(raw), ParentID: parent,
				Props: map[string]string{"tag": tag},
			}
			cast.Nodes = append(cast.Nodes, n)
			addChildRef(&cast, parent, n.ID)
			cov.HTMLBlocks++
			continue
		}

		// Blockquote.
		if blockquoteRe.MatchString(line) {
			start := i
			var bqLines []string
			for i < len(lines) && blockquoteRe.MatchString(lines[i]) {
				m := blockquoteRe.FindStringSubmatch(lines[i])
				bqLines = append(bqLines, m[1])
				i++
			}
			parent := currentParent(headingStack)
			content := strings.TrimSpace(strings.Join(bqLines, "\n"))
			raw := strings.Join(lines[start:i], "\n")
			n := CNode{
				ID: nodeID("blockquote", start), Kind: KindBlockquote,
				Text: content, RawText: raw,
				Span:  Span{StartLine: start + 1, EndLine: i},
				Hash:  hashStr(content), ParentID: parent,
			}
			cast.Nodes = append(cast.Nodes, n)
			addChildRef(&cast, parent, n.ID)
			cov.Blockquotes++
			continue
		}

		// Table.
		if tableRowRe.MatchString(trimmed) && i+1 < len(lines) && tableSepRe.MatchString(strings.TrimSpace(lines[i+1])) {
			start := i
			tableParent := currentParent(headingStack)
			tableID := nodeID("table", start)
			tableNode := CNode{
				ID: tableID, Kind: KindTable, RawText: "",
				Span: Span{StartLine: start + 1}, ParentID: tableParent,
			}

			// Header row.
			headerCells := parseCells(lines[i])
			headerRow := CNode{
				ID: nodeID("table_row", i), Kind: KindTableRow, ParentID: tableID,
				Span: Span{StartLine: i + 1, EndLine: i + 1}, Hash: hashStr(lines[i]),
				Props: map[string]string{"role": "header"},
			}
			for ci, cell := range headerCells {
				cellNode := CNode{
					ID: nodeID(fmt.Sprintf("table_cell_%d", ci), i), Kind: KindTableCell,
					Text: cell, ParentID: headerRow.ID,
					Span: Span{StartLine: i + 1, EndLine: i + 1}, Hash: hashStr(cell),
				}
				headerRow.Children = append(headerRow.Children, cellNode.ID)
				cast.Nodes = append(cast.Nodes, cellNode)
			}
			tableNode.Children = append(tableNode.Children, headerRow.ID)
			cast.Nodes = append(cast.Nodes, headerRow)

			i += 2 // skip header + separator
			for i < len(lines) && tableRowRe.MatchString(strings.TrimSpace(lines[i])) {
				cells := parseCells(lines[i])
				row := CNode{
					ID: nodeID("table_row", i), Kind: KindTableRow, ParentID: tableID,
					Span: Span{StartLine: i + 1, EndLine: i + 1}, Hash: hashStr(lines[i]),
				}
				for ci, cell := range cells {
					cellNode := CNode{
						ID: nodeID(fmt.Sprintf("table_cell_%d", ci), i), Kind: KindTableCell,
						Text: cell, ParentID: row.ID,
						Span: Span{StartLine: i + 1, EndLine: i + 1}, Hash: hashStr(cell),
					}
					row.Children = append(row.Children, cellNode.ID)
					cast.Nodes = append(cast.Nodes, cellNode)
				}
				tableNode.Children = append(tableNode.Children, row.ID)
				cast.Nodes = append(cast.Nodes, row)
				i++
			}
			tableNode.Span.EndLine = i
			tableNode.Hash = hashStr(strings.Join(lines[start:i], "\n"))
			cast.Nodes = append(cast.Nodes, tableNode)
			addChildRef(&cast, tableParent, tableID)
			cov.Tables++
			continue
		}

		// List (unordered or ordered).
		if ulItemRe.MatchString(line) || olItemRe.MatchString(line) {
			start := i
			parent := currentParent(headingStack)
			ordered := olItemRe.MatchString(line)
			listID := nodeID("list", start)
			listKind := "unordered"
			if ordered {
				listKind = "ordered"
			}
			listNode := CNode{
				ID: listID, Kind: KindList, ParentID: parent,
				Span:  Span{StartLine: start + 1},
				Props: map[string]string{"list_type": listKind},
			}

			for i < len(lines) {
				li := lines[i]
				var itemText string
				if m := ulItemRe.FindStringSubmatch(li); m != nil {
					itemText = m[2]
				} else if m := olItemRe.FindStringSubmatch(li); m != nil {
					itemText = m[2]
				} else {
					break
				}
				item := CNode{
					ID: nodeID("list_item", i), Kind: KindListItem,
					Text: strings.TrimSpace(itemText), RawText: li, ParentID: listID,
					Span: Span{StartLine: i + 1, EndLine: i + 1}, Hash: hashStr(itemText),
				}
				// Extract inline links/images.
				item.Props = extractInlineRefs(itemText)
				listNode.Children = append(listNode.Children, item.ID)
				cast.Nodes = append(cast.Nodes, item)
				cov.ListItems++
				i++
			}
			listNode.Span.EndLine = i
			listNode.Hash = hashStr(strings.Join(lines[start:i], "\n"))
			cast.Nodes = append(cast.Nodes, listNode)
			addChildRef(&cast, parent, listID)
			cov.Lists++
			continue
		}

		// Paragraph (fallback).
		start := i
		var paraLines []string
		for i < len(lines) {
			l := lines[i]
			t := strings.TrimSpace(l)
			if t == "" || headingRe.MatchString(l) || codeOpenRe.MatchString(l) ||
				thematicRe.MatchString(t) || blockquoteRe.MatchString(l) ||
				htmlBlockRe.MatchString(t) || htmlCloseRe.MatchString(t) {
				break
			}
			// Check if next line starts a table.
			if tableRowRe.MatchString(t) && i+1 < len(lines) && tableSepRe.MatchString(strings.TrimSpace(lines[i+1])) {
				break
			}
			// Check if this is a list item.
			if ulItemRe.MatchString(l) || olItemRe.MatchString(l) {
				break
			}
			paraLines = append(paraLines, l)
			i++
		}
		if len(paraLines) > 0 {
			content := strings.TrimSpace(strings.Join(paraLines, "\n"))
			raw := strings.Join(paraLines, "\n")
			parent := currentParent(headingStack)
			n := CNode{
				ID: nodeID("paragraph", start), Kind: KindParagraph,
				Text: content, RawText: raw,
				Span: Span{StartLine: start + 1, EndLine: i},
				Hash: hashStr(content), ParentID: parent,
			}
			// Extract inline links and images.
			n.Props = extractInlineRefs(content)
			cast.Nodes = append(cast.Nodes, n)
			addChildRef(&cast, parent, n.ID)
			cov.Paragraphs++

			// Count inline links and images.
			cov.Links += len(linkRe.FindAllString(content, -1))
			cov.Images += len(imageRe.FindAllString(content, -1))
		}
	}

	cast.Coverage = cov
	return cast
}

func currentParent(stack [7]string) string {
	for l := 6; l >= 0; l-- {
		if stack[l] != "" {
			return stack[l]
		}
	}
	return ""
}

func parentForLevel(stack [7]string, level int) string {
	for l := level - 1; l >= 0; l-- {
		if stack[l] != "" {
			return stack[l]
		}
	}
	return ""
}

func nodeID(prefix string, line int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", prefix, line)))
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(h[:6]))
}

func hashStr(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func addChildRef(cast *CAST, parentID, childID string) {
	for i := range cast.Nodes {
		if cast.Nodes[i].ID == parentID {
			cast.Nodes[i].Children = append(cast.Nodes[i].Children, childID)
			return
		}
	}
}

func parseCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

func extractInlineRefs(text string) map[string]string {
	props := map[string]string{}
	if imgs := imageRe.FindAllStringSubmatch(text, -1); len(imgs) > 0 {
		for i, m := range imgs {
			props[fmt.Sprintf("image_%d_alt", i)] = m[1]
			props[fmt.Sprintf("image_%d_src", i)] = m[2]
		}
	}
	if links := linkRe.FindAllStringSubmatch(text, -1); len(links) > 0 {
		for i, m := range links {
			// Skip image matches (prefixed with !).
			if strings.Contains(text, "!["+m[1]+"]") {
				continue
			}
			props[fmt.Sprintf("link_%d_text", i)] = m[1]
			props[fmt.Sprintf("link_%d_href", i)] = m[2]
		}
	}
	if len(props) == 0 {
		return nil
	}
	return props
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

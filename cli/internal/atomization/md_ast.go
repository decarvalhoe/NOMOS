package atomization

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// BlockType enumerates the Markdown block-level elements.
type BlockType string

const (
	BlockDocument  BlockType = "document"
	BlockHeading   BlockType = "heading"
	BlockParagraph BlockType = "paragraph"
	BlockList      BlockType = "list"
	BlockListItem  BlockType = "list_item"
	BlockCodeBlock BlockType = "code_block"
	BlockTable     BlockType = "table"
	BlockTableRow  BlockType = "table_row"
	BlockMetadata  BlockType = "metadata"
	BlockBlankLine BlockType = "blank_line"
)

// Span records the byte offset range in the original source.
type Span struct {
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
	StartByte int `json:"start_byte"`
	EndByte   int `json:"end_byte"`
}

// Block is a single AST node representing a Markdown block element.
type Block struct {
	ID       string            `json:"id"`
	Type     BlockType         `json:"type"`
	Level    int               `json:"level,omitempty"`
	Text     string            `json:"text"`
	RawText  string            `json:"raw_text"`
	Span     Span              `json:"span"`
	Hash     string            `json:"hash"`
	ParentID string            `json:"parent_id,omitempty"`
	Children []string          `json:"children,omitempty"`
	Props    map[string]string `json:"props,omitempty"`
}

// AST is the parsed block tree for a Markdown document.
type AST struct {
	Blocks      []Block  `json:"blocks"`
	Root        string   `json:"root"`
	SourceHash  string   `json:"source_hash"`
	SourceLen   int      `json:"source_len"`
	LossReport  LossReport `json:"loss_report"`
}

// LossReport tracks content that was not captured by any block.
type LossReport struct {
	TotalSourceBytes   int          `json:"total_source_bytes"`
	CoveredBytes       int          `json:"covered_bytes"`
	LostBytes          int          `json:"lost_bytes"`
	LossRatio          float64      `json:"loss_ratio"`
	LostSpans          []LostSpan   `json:"lost_spans,omitempty"`
	IsLossless         bool         `json:"is_lossless"`
}

// LostSpan identifies a range of source that no block covers.
type LostSpan struct {
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Preview   string `json:"preview"`
}

var (
	headingRe   = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	codeOpenRe  = regexp.MustCompile("^```(\\w*)\\s*$")
	tableRowRe  = regexp.MustCompile(`^\|.+\|$`)
	tableSepRe  = regexp.MustCompile(`^\|[\s:_-]+(\|[\s:_-]+)+\|$`)
	listItemRe  = regexp.MustCompile(`^(\s*)(?:[-*+]|\d+[.)]) (.+)$`)
	metaTableRe = regexp.MustCompile(`^\|\s*(.+?)\s*\|\s*(.+?)\s*\|$`)
)

// ParseMarkdown parses Markdown source into a block AST.
// It is loss-aware: after parsing, it computes which source bytes are
// not covered by any block and reports them.
func ParseMarkdown(source string) AST {
	lines := strings.Split(source, "\n")
	lineOffsets := computeLineOffsets(source)

	ast := AST{
		SourceHash: hashContent(source),
		SourceLen:  len(source),
	}

	// Root document block.
	rootID := blockID("document", 0, 0)
	root := Block{
		ID:      rootID,
		Type:    BlockDocument,
		Text:    "",
		RawText: "",
		Span:    Span{StartLine: 1, EndLine: len(lines), StartByte: 0, EndByte: len(source)},
		Hash:    hashContent(source),
	}
	ast.Root = rootID
	ast.Blocks = append(ast.Blocks, root)

	// Parsing state.
	var headingStack [7]string // headingStack[level] = block ID of current heading at that level
	headingStack[0] = rootID

	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Blank line.
		if trimmed == "" {
			blk := makeBlock(BlockBlankLine, "", line, i, i, lineOffsets, parentFromStack(headingStack, 0))
			ast.Blocks = append(ast.Blocks, blk)
			addChild(&ast, blk.ParentID, blk.ID)
			i++
			continue
		}

		// Code block (fenced).
		if m := codeOpenRe.FindStringSubmatch(line); m != nil {
			startLine := i
			lang := m[1]
			i++
			var bodyLines []string
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				bodyLines = append(bodyLines, lines[i])
				i++
			}
			endLine := i
			if i < len(lines) {
				i++ // skip closing ```
				endLine = i - 1
			}
			raw := strings.Join(lines[startLine:min(i, len(lines))], "\n")
			body := strings.Join(bodyLines, "\n")
			parent := parentFromStack(headingStack, 0)
			blk := makeBlock(BlockCodeBlock, body, raw, startLine, endLine, lineOffsets, parent)
			if lang != "" {
				blk.Props = map[string]string{"language": lang}
			}
			ast.Blocks = append(ast.Blocks, blk)
			addChild(&ast, parent, blk.ID)
			continue
		}

		// Heading.
		if m := headingRe.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			title := strings.TrimSpace(m[2])
			parent := parentFromStack(headingStack, level)

			blk := makeBlock(BlockHeading, title, line, i, i, lineOffsets, parent)
			blk.Level = level
			ast.Blocks = append(ast.Blocks, blk)
			addChild(&ast, parent, blk.ID)

			headingStack[level] = blk.ID
			for l := level + 1; l <= 6; l++ {
				headingStack[l] = ""
			}

			// Check for metadata table immediately after H1.
			if level == 1 && i+1 < len(lines) {
				metaBlocks, consumed := parseMetaTable(lines, i+1, lineOffsets, blk.ID)
				for _, mb := range metaBlocks {
					ast.Blocks = append(ast.Blocks, mb)
					addChild(&ast, blk.ID, mb.ID)
				}
				i += consumed
			}

			i++
			continue
		}

		// Table.
		if tableRowRe.MatchString(line) {
			startLine := i
			var tableLines []string
			for i < len(lines) && tableRowRe.MatchString(strings.TrimSpace(lines[i])) {
				tableLines = append(tableLines, lines[i])
				i++
			}
			raw := strings.Join(tableLines, "\n")
			parent := parentFromStack(headingStack, 0)
			tblBlk := makeBlock(BlockTable, raw, raw, startLine, i-1, lineOffsets, parent)
			ast.Blocks = append(ast.Blocks, tblBlk)
			addChild(&ast, parent, tblBlk.ID)

			// Parse rows (skip separator).
			for ri, tl := range tableLines {
				if tableSepRe.MatchString(strings.TrimSpace(tl)) {
					continue
				}
				rowBlk := makeBlock(BlockTableRow, strings.TrimSpace(tl), tl, startLine+ri, startLine+ri, lineOffsets, tblBlk.ID)
				ast.Blocks = append(ast.Blocks, rowBlk)
				addChild(&ast, tblBlk.ID, rowBlk.ID)
			}
			continue
		}

		// List.
		if listItemRe.MatchString(line) {
			startLine := i
			parent := parentFromStack(headingStack, 0)
			var listLines []string
			for i < len(lines) && (listItemRe.MatchString(lines[i]) || (strings.TrimSpace(lines[i]) != "" && strings.HasPrefix(lines[i], "  "))) {
				listLines = append(listLines, lines[i])
				i++
			}
			raw := strings.Join(listLines, "\n")
			listBlk := makeBlock(BlockList, raw, raw, startLine, i-1, lineOffsets, parent)
			ast.Blocks = append(ast.Blocks, listBlk)
			addChild(&ast, parent, listBlk.ID)

			// Parse individual items.
			for li, ll := range listLines {
				if m := listItemRe.FindStringSubmatch(ll); m != nil {
					itemBlk := makeBlock(BlockListItem, strings.TrimSpace(m[2]), ll, startLine+li, startLine+li, lineOffsets, listBlk.ID)
					ast.Blocks = append(ast.Blocks, itemBlk)
					addChild(&ast, listBlk.ID, itemBlk.ID)
				}
			}
			continue
		}

		// Paragraph (default).
		startLine := i
		var paraLines []string
		for i < len(lines) {
			tl := strings.TrimSpace(lines[i])
			if tl == "" || headingRe.MatchString(lines[i]) || codeOpenRe.MatchString(lines[i]) ||
				tableRowRe.MatchString(lines[i]) || listItemRe.MatchString(lines[i]) {
				break
			}
			paraLines = append(paraLines, lines[i])
			i++
		}
		raw := strings.Join(paraLines, "\n")
		text := strings.TrimSpace(raw)
		parent := parentFromStack(headingStack, 0)
		blk := makeBlock(BlockParagraph, text, raw, startLine, i-1, lineOffsets, parent)
		ast.Blocks = append(ast.Blocks, blk)
		addChild(&ast, parent, blk.ID)
	}

	ast.LossReport = computeLossReport(source, lines, ast.Blocks, lineOffsets)
	return ast
}

func makeBlock(typ BlockType, text, raw string, startLine, endLine int, offsets []int, parentID string) Block {
	startByte := 0
	if startLine < len(offsets) {
		startByte = offsets[startLine]
	}
	endByte := 0
	if endLine < len(offsets) {
		endByte = offsets[endLine] + len(strings.Split(raw, "\n")[0])
	}
	if endLine+1 < len(offsets) {
		endByte = offsets[endLine+1] - 1
	}

	id := blockID(string(typ), startLine, endLine)
	return Block{
		ID:       id,
		Type:     typ,
		Text:     text,
		RawText:  raw,
		Span:     Span{StartLine: startLine + 1, EndLine: endLine + 1, StartByte: startByte, EndByte: endByte},
		Hash:     hashContent(raw),
		ParentID: parentID,
	}
}

func parentFromStack(stack [7]string, level int) string {
	for l := level - 1; l >= 0; l-- {
		if stack[l] != "" {
			return stack[l]
		}
	}
	return stack[0] // root
}

func addChild(ast *AST, parentID, childID string) {
	for i := range ast.Blocks {
		if ast.Blocks[i].ID == parentID {
			ast.Blocks[i].Children = append(ast.Blocks[i].Children, childID)
			return
		}
	}
}

func parseMetaTable(lines []string, start int, offsets []int, parentID string) ([]Block, int) {
	i := start
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || !metaTableRe.MatchString(lines[i]) {
		return nil, 0
	}

	startLine := i
	var metaLines []string
	for i < len(lines) {
		tl := strings.TrimSpace(lines[i])
		if tl == "" {
			break
		}
		if !metaTableRe.MatchString(tl) && !tableSepRe.MatchString(tl) {
			break
		}
		metaLines = append(metaLines, lines[i])
		i++
	}

	if len(metaLines) < 2 {
		return nil, 0
	}

	raw := strings.Join(metaLines, "\n")
	blk := makeBlock(BlockMetadata, raw, raw, startLine, i-1, offsets, parentID)
	blk.Props = make(map[string]string)

	for _, ml := range metaLines {
		if tableSepRe.MatchString(strings.TrimSpace(ml)) {
			continue
		}
		m := metaTableRe.FindStringSubmatch(ml)
		if m != nil {
			key := strings.ToLower(strings.TrimSpace(m[1]))
			val := strings.TrimSpace(m[2])
			blk.Props[key] = val
		}
	}

	return []Block{blk}, i - start
}

func computeLineOffsets(source string) []int {
	offsets := []int{0}
	for i, b := range source {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

func computeLossReport(source string, lines []string, blocks []Block, offsets []int) LossReport {
	totalBytes := len(source)
	if totalBytes == 0 {
		return LossReport{IsLossless: true}
	}

	// Track which lines are covered by non-blank, non-document blocks.
	coveredLines := make([]bool, len(lines))
	for _, blk := range blocks {
		if blk.Type == BlockDocument || blk.Type == BlockBlankLine {
			continue
		}
		for l := blk.Span.StartLine - 1; l < blk.Span.EndLine && l < len(lines); l++ {
			coveredLines[l] = true
		}
	}

	// Find uncovered lines with non-whitespace content.
	var lostSpans []LostSpan
	coveredBytes := 0
	for i, covered := range coveredLines {
		lineLen := len(lines[i])
		if i < len(lines)-1 {
			lineLen++ // account for \n
		}
		if covered {
			coveredBytes += lineLen
		} else if strings.TrimSpace(lines[i]) == "" {
			coveredBytes += lineLen // blank lines don't count as loss
		} else {
			preview := lines[i]
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			lostSpans = append(lostSpans, LostSpan{
				StartLine: i + 1,
				EndLine:   i + 1,
				Preview:   preview,
			})
		}
	}

	lostBytes := totalBytes - coveredBytes
	if lostBytes < 0 {
		lostBytes = 0
	}

	lossRatio := 0.0
	if totalBytes > 0 {
		lossRatio = float64(lostBytes) / float64(totalBytes)
	}

	return LossReport{
		TotalSourceBytes: totalBytes,
		CoveredBytes:     coveredBytes,
		LostBytes:        lostBytes,
		LossRatio:        lossRatio,
		LostSpans:        lostSpans,
		IsLossless:       len(lostSpans) == 0,
	}
}

func blockID(prefix string, startLine, endLine int) string {
	raw := fmt.Sprintf("%s:%d:%d", prefix, startLine, endLine)
	h := sha256.Sum256([]byte(raw))
	return "B-" + strings.ToUpper(hex.EncodeToString(h[:6]))
}

func hashContent(s string) string {
	h := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(h[:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

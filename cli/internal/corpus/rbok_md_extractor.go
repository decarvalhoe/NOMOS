package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/atomization"
	parsemodel "github.com/RBOKproject/Nomos/cli/internal/corpus/parse"
)

// MDExtractResult holds the output of Markdown lawbook extraction.
type MDExtractResult struct {
	Nodes []LawbookNode `json:"nodes"`
}

var (
	headerRe   = regexp.MustCompile(`^(#{1,4})\s+(.+)$`)
	metaRowRe  = regexp.MustCompile(`^\|\s*(.+?)\s*\|\s*(.+?)\s*\|$`)
	metaSepRe  = regexp.MustCompile(`^\|[\s:_-]+\|[\s:_-]+\|$`)
	listItemRe = regexp.MustCompile(`^\s*(?:[-*+]|\d+[.)])\s+(.+\S)\s*$`)
	metaKeyMap = map[string]string{
		"reference":         "reference",
		"référence":         "reference",
		"ref":               "reference",
		"statut":            "statut",
		"status":            "statut",
		"emetteur":          "emetteur",
		"émetteur":          "emetteur",
		"issuer":            "emetteur",
		"auteur":            "author",
		"author":            "author",
		"droits":            "rights",
		"rights":            "rights",
		"confidentialite":   "confidentiality",
		"confidentialité":   "confidentiality",
		"confidentiality":   "confidentiality",
		"derniere revision": "derniere_revision",
		"dernière révision": "derniere_revision",
		"last revision":     "derniere_revision",
		"date de creation":  "date_creation",
		"date de création":  "date_creation",
		"created":           "date_creation",
		"revise par":        "revise_par",
		"révisé par":        "revise_par",
		"revised by":        "revise_par",
		"date":              "date",
		"version":           "version",
		"domaine":           "domaine",
		"domain":            "domaine",
		"portee":            "portee",
		"portée":            "portee",
		"scope":             "portee",
	}
)

// ExtractMarkdown parses Markdown content into a flat list of LawbookNodes
// by first building the portable Markdown AST, then projecting AST blocks into
// NOMOS lawbook nodes. RBOK uses this as its first profile, but the block
// contract is intentionally generic.
func ExtractMarkdown(source string, docSlug string) MDExtractResult {
	source = normalizeLineEndings(source)
	ast := atomization.ParseMarkdown(source)
	if len(ast.Blocks) <= 1 {
		return MDExtractResult{Nodes: []LawbookNode{}}
	}
	if !astHasHeading(ast) {
		return MDExtractResult{Nodes: []LawbookNode{}}
	}

	var nodes []LawbookNode
	blockToNodeID := map[string]string{}
	nodeByID := map[string]LawbookNode{}

	for _, block := range ast.Blocks {
		switch block.Type {
		case atomization.BlockDocument, atomization.BlockBlankLine, atomization.BlockThematicBreak:
			continue
		case atomization.BlockHeading:
			node := headingBlockToNode(ast, block, docSlug, blockToNodeID)
			nodes = append(nodes, node)
			blockToNodeID[block.ID] = node.NodeID
			nodeByID[node.NodeID] = node
		case atomization.BlockMetadata:
			continue
		case atomization.BlockParagraph:
			paragraph := portableBlockToNode(block, docSlug, blockToNodeID, nodeByID, NodeParagraph)
			nodes = append(nodes, paragraph)
			blockToNodeID[block.ID] = paragraph.NodeID
			nodeByID[paragraph.NodeID] = paragraph
			alinea := childTextNode(block, paragraph, docSlug, NodeAlinea, "alinea", 1)
			nodes = append(nodes, alinea)
			nodeByID[alinea.NodeID] = alinea
		case atomization.BlockList:
			paragraph := portableBlockToNode(block, docSlug, blockToNodeID, nodeByID, NodeParagraph)
			paragraph.Metadata["block_kind"] = string(atomization.BlockList)
			nodes = append(nodes, paragraph)
			blockToNodeID[block.ID] = paragraph.NodeID
			nodeByID[paragraph.NodeID] = paragraph
		case atomization.BlockListItem:
			parentID := blockToNodeID[block.ParentID]
			parent := nodeByID[parentID]
			if parent.NodeID == "" {
				parent = portableBlockToNode(block, docSlug, blockToNodeID, nodeByID, NodeParagraph)
				nodes = append(nodes, parent)
				nodeByID[parent.NodeID] = parent
				parentID = parent.NodeID
			}
			itemIndex := countChildren(nodes, parentID, NodeAlinea) + 1
			alinea := childTextNode(block, parent, docSlug, NodeAlinea, "list_item", itemIndex)
			nodes = append(nodes, alinea)
			blockToNodeID[block.ID] = alinea.NodeID
			nodeByID[alinea.NodeID] = alinea
		case atomization.BlockTable:
			node := portableBlockToNode(block, docSlug, blockToNodeID, nodeByID, NodeTable)
			nodes = append(nodes, node)
			blockToNodeID[block.ID] = node.NodeID
			nodeByID[node.NodeID] = node
		case atomization.BlockTableRow:
			node := portableBlockToNode(block, docSlug, blockToNodeID, nodeByID, NodeTableRow)
			nodes = append(nodes, node)
			blockToNodeID[block.ID] = node.NodeID
			nodeByID[node.NodeID] = node
		case atomization.BlockCodeBlock:
			node := portableBlockToNode(block, docSlug, blockToNodeID, nodeByID, NodeCodeBlock)
			nodes = append(nodes, node)
			blockToNodeID[block.ID] = node.NodeID
			nodeByID[node.NodeID] = node
		case atomization.BlockCallout:
			node := portableBlockToNode(block, docSlug, blockToNodeID, nodeByID, NodeCallout)
			nodes = append(nodes, node)
			blockToNodeID[block.ID] = node.NodeID
			nodeByID[node.NodeID] = node
		case atomization.BlockBlockQuote:
			node := portableBlockToNode(block, docSlug, blockToNodeID, nodeByID, NodeBlockQuote)
			nodes = append(nodes, node)
			blockToNodeID[block.ID] = node.NodeID
			nodeByID[node.NodeID] = node
		case atomization.BlockImage:
			node := portableBlockToNode(block, docSlug, blockToNodeID, nodeByID, NodeImage)
			nodes = append(nodes, node)
			blockToNodeID[block.ID] = node.NodeID
			nodeByID[node.NodeID] = node
		case atomization.BlockRawHTML:
			node := portableBlockToNode(block, docSlug, blockToNodeID, nodeByID, NodeRawHTML)
			node.Metadata["review_state"] = "needs_review"
			node.Metadata["unsupported_reason"] = "raw HTML preserved without semantic interpretation"
			nodes = append(nodes, node)
			blockToNodeID[block.ID] = node.NodeID
			nodeByID[node.NodeID] = node
		}
	}

	if len(nodes) == 0 {
		return MDExtractResult{Nodes: []LawbookNode{}}
	}
	return MDExtractResult{Nodes: nodes}
}

func headingBlockToNode(ast atomization.AST, block atomization.Block, docSlug string, blockToNodeID map[string]string) LawbookNode {
	nodeType := headingToNodeType(block.Level)
	canonicalRef := buildBlockCanonicalRef(docSlug, nodeType, block)
	nodeID := computeNodeID(canonicalRef)
	parentID := blockToNodeID[block.ParentID]
	var metaAny map[string]any
	if block.Level == 1 {
		metadata := metadataForHeading(ast, block.ID)
		if len(metadata) > 0 {
			metaAny = map[string]any{}
			for k, v := range metadata {
				metaAny[k] = v
			}
		}
	}
	node := LawbookNode{
		NodeID:       nodeID,
		NodeType:     nodeType,
		Depth:        nodeType.Depth(),
		Title:        block.Text,
		Text:         block.Text,
		ParentID:     parentID,
		CanonicalRef: canonicalRef,
		DisplayRef:   buildDisplayRef(nodeType, block.Text),
		Metadata:     metaAny,
		SourceSpan:   sourceSpanForBlock(block),
	}
	node.Locator = sourceSpanLocator("", node.SourceSpan)
	return node
}

func astHasHeading(ast atomization.AST) bool {
	for _, block := range ast.Blocks {
		if block.Type == atomization.BlockHeading {
			return true
		}
	}
	return false
}

func portableBlockToNode(block atomization.Block, docSlug string, blockToNodeID map[string]string, nodeByID map[string]LawbookNode, nodeType LawbookNodeType) LawbookNode {
	parentID := blockToNodeID[block.ParentID]
	parentDepth := -1
	if parent, ok := nodeByID[parentID]; ok {
		parentDepth = parent.Depth
	}
	depth := parentDepth + 1
	if nodeType.hasFixedDepth() {
		depth = nodeType.Depth()
	}
	canonicalRef := buildBlockCanonicalRef(docSlug, nodeType, block)
	title := block.Text
	if nodeType == NodeCodeBlock {
		title = "code block"
	} else if nodeType == NodeTable {
		title = "table"
	} else if nodeType == NodeTableRow {
		title = "table row"
	}
	metadata := map[string]any{
		"block_id":   block.ID,
		"block_kind": string(block.Type),
		"text_hash":  block.Hash,
	}
	for k, v := range block.Props {
		metadata[k] = v
	}
	node := LawbookNode{
		NodeID:       computeNodeID(canonicalRef),
		NodeType:     nodeType,
		Depth:        depth,
		Title:        title,
		Text:         firstNonEmpty(block.Text, block.RawText),
		ParentID:     parentID,
		CanonicalRef: canonicalRef,
		DisplayRef:   fmt.Sprintf("%s L%d-%d", nodeType, block.Span.StartLine, block.Span.EndLine),
		Metadata:     metadata,
		SourceSpan:   sourceSpanForBlock(block),
	}
	node.Locator = sourceSpanLocator("", node.SourceSpan)
	return node
}

func childTextNode(block atomization.Block, parent LawbookNode, docSlug string, nodeType LawbookNodeType, blockKind string, index int) LawbookNode {
	ref := fmt.Sprintf("%s/%s/%d", parent.CanonicalRef, nodeType, index)
	metadata := map[string]any{
		"block_id":   block.ID,
		"block_kind": blockKind,
		"text_hash":  block.Hash,
	}
	for k, v := range block.Props {
		metadata[k] = v
	}
	node := LawbookNode{
		NodeID:       computeNodeID(ref),
		NodeType:     nodeType,
		Depth:        nodeType.Depth(),
		Text:         firstNonEmpty(block.Text, block.RawText),
		ParentID:     parent.NodeID,
		CanonicalRef: ref,
		DisplayRef:   fmt.Sprintf("%s %d", nodeType, index),
		Metadata:     metadata,
		SourceSpan:   sourceSpanForBlock(block),
	}
	node.Locator = sourceSpanLocator("", node.SourceSpan)
	_ = docSlug
	return node
}

func buildBlockCanonicalRef(docSlug string, nodeType LawbookNodeType, block atomization.Block) string {
	base := lawbookSlugify(firstNonEmpty(block.Text, string(block.Type)))
	if base == "" {
		base = string(nodeType)
	}
	return fmt.Sprintf("%s/%s/%s-L%d", docSlug, nodeType, base, block.Span.StartLine)
}

func metadataForHeading(ast atomization.AST, headingID string) map[string]any {
	out := map[string]any{}
	for _, block := range ast.Blocks {
		if block.ParentID != headingID || block.Type != atomization.BlockMetadata {
			continue
		}
		for k, v := range block.Props {
			normalKey := strings.ToLower(strings.TrimSpace(k))
			if mapped, ok := metaKeyMap[normalKey]; ok {
				out[mapped] = v
			} else {
				out[normalKey] = v
			}
		}
	}
	return out
}

func sourceSpanForBlock(block atomization.Block) *parsemodel.SourceSpan {
	startLine := block.Span.StartLine
	endLine := block.Span.EndLine
	startColumn := 1
	endColumn := len(lastRawLine(block.RawText)) + 1
	startByte := block.Span.StartByte
	endByte := block.Span.EndByte
	return &parsemodel.SourceSpan{
		StartLine:   &startLine,
		EndLine:     &endLine,
		StartColumn: &startColumn,
		EndColumn:   &endColumn,
		StartByte:   &startByte,
		EndByte:     &endByte,
		TextQuote: &parsemodel.TextQuoteSelector{
			Exact: strings.TrimSpace(firstNonEmpty(block.Text, block.RawText)),
		},
	}
}

func sourceSpanLocator(path string, span *parsemodel.SourceSpan) string {
	if span == nil || span.StartLine == nil || span.EndLine == nil {
		return ""
	}
	startCol := 1
	endCol := 1
	if span.StartColumn != nil {
		startCol = *span.StartColumn
	}
	if span.EndColumn != nil {
		endCol = *span.EndColumn
	}
	locator := fmt.Sprintf("#L%d:C%d-L%d:C%d", *span.StartLine, startCol, *span.EndLine, endCol)
	if span.StartByte != nil && span.EndByte != nil {
		locator += fmt.Sprintf("@B%d-B%d", *span.StartByte, *span.EndByte)
	}
	return path + locator
}

func countChildren(nodes []LawbookNode, parentID string, nodeType LawbookNodeType) int {
	count := 0
	for _, node := range nodes {
		if node.ParentID == parentID && node.NodeType == nodeType {
			count++
		}
	}
	return count
}

func lastRawLine(raw string) string {
	if raw == "" {
		return ""
	}
	lines := strings.Split(raw, "\n")
	return lines[len(lines)-1]
}

func normalizeLineEndings(source string) string {
	source = strings.ReplaceAll(source, "\r\n", "\n")
	source = strings.ReplaceAll(source, "\r", "\n")
	return source
}

func headingToNodeType(level int) LawbookNodeType {
	switch level {
	case 1:
		return NodeDocument
	case 2:
		return NodeChapter
	case 3:
		return NodeSection
	case 4:
		return NodeArticle
	case 5:
		return NodeClause
	case 6:
		return NodeSubclause
	default:
		return NodeSubclause
	}
}

func findParent(stack [5]int, level int) int {
	for l := level - 1; l >= 1; l-- {
		if stack[l] >= 0 {
			return stack[l]
		}
	}
	return -1
}

func buildCanonicalRef(docSlug string, nodeType LawbookNodeType, title string) string {
	slug := lawbookSlugify(title)
	return fmt.Sprintf("%s/%s/%s", docSlug, nodeType, slug)
}

func buildDisplayRef(nodeType LawbookNodeType, title string) string {
	return fmt.Sprintf("%s: %s", nodeType, title)
}

func computeNodeID(canonicalRef string) string {
	h := sha256.Sum256([]byte(canonicalRef))
	return "N-" + strings.ToUpper(hex.EncodeToString(h[:8]))
}

func lawbookSlugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// parseMetadataTable attempts to parse a Markdown table immediately following
// an H1 header. Returns the metadata map and number of lines consumed.
func parseMetadataTable(lines []string, start int) (map[string]string, int) {
	i := start
	// Skip blank lines before table.
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) {
		return nil, 0
	}

	// Check for table header row.
	firstRow := metaRowRe.FindStringSubmatch(lines[i])
	if firstRow == nil {
		return nil, 0
	}
	i++

	// Separator row.
	if i >= len(lines) || !metaSepRe.MatchString(lines[i]) {
		return nil, 0
	}
	i++

	meta := map[string]string{}
	// Parse data rows.
	for i < len(lines) {
		row := metaRowRe.FindStringSubmatch(lines[i])
		if row == nil {
			break
		}
		key := strings.TrimSpace(row[1])
		value := strings.TrimSpace(row[2])
		normalKey := strings.ToLower(key)
		if mapped, ok := metaKeyMap[normalKey]; ok {
			meta[mapped] = value
		} else {
			meta[normalKey] = value
		}
		i++
	}

	if len(meta) == 0 {
		return nil, 0
	}
	return meta, i - start
}

// extractSubNodes splits body content into paragraph containers and atomic
// alinea nodes. Every non-empty paragraph block emits at least one alinea.
func extractSubNodes(body string, parentID string, parentRef string, lineStart int, lineEnd int) []LawbookNode {
	var nodes []LawbookNode
	paragraphs := splitParagraphs(body)
	pCount := 0

	for _, para := range paragraphs {
		text := strings.TrimSpace(para)
		if text == "" {
			continue
		}

		pCount++
		paragraphRef := fmt.Sprintf("%s/paragraph/%d", parentRef, pCount)
		paragraph := LawbookNode{
			NodeID:       computeNodeID(paragraphRef),
			NodeType:     NodeParagraph,
			Depth:        NodeParagraph.Depth(),
			Title:        "",
			Text:         text,
			ParentID:     parentID,
			CanonicalRef: paragraphRef,
			DisplayRef:   fmt.Sprintf("paragraph %d", pCount),
		}
		nodes = append(nodes, paragraph)

		alineas := atomizeAlineas(text)
		for ai, item := range alineas {
			ref := fmt.Sprintf("%s/alinea/%d", paragraphRef, ai+1)
			nodes = append(nodes, LawbookNode{
				NodeID:       computeNodeID(ref),
				NodeType:     NodeAlinea,
				Depth:        NodeAlinea.Depth(),
				Title:        "",
				Text:         item,
				ParentID:     paragraph.NodeID,
				CanonicalRef: ref,
				DisplayRef:   fmt.Sprintf("alinea %d", ai+1),
			})
		}
	}
	return nodes
}

func atomizeAlineas(text string) []string {
	lines := strings.Split(text, "\n")
	var items []string
	allListItems := true
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		match := listItemRe.FindStringSubmatch(trimmed)
		if match == nil {
			allListItems = false
			break
		}
		items = append(items, strings.TrimSpace(match[1]))
	}
	if allListItems && len(items) > 0 {
		return items
	}
	return []string{strings.TrimSpace(text)}
}

// splitParagraphs splits text on blank lines.
func splitParagraphs(text string) []string {
	var result []string
	var current strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if isMarkdownStructuralSeparator(line) {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			continue
		}
		if strings.TrimSpace(line) == "" {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			continue
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

func isMarkdownStructuralSeparator(line string) bool {
	trimmed := strings.TrimSpace(line)
	if len(trimmed) < 3 {
		return false
	}
	for _, marker := range []rune{'-', '*', '_'} {
		matches := true
		for _, r := range trimmed {
			if r != marker {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

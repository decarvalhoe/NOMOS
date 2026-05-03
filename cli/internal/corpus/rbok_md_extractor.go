package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
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
		"reference":          "reference",
		"référence":          "reference",
		"ref":                "reference",
		"statut":             "statut",
		"status":             "statut",
		"emetteur":           "emetteur",
		"émetteur":           "emetteur",
		"issuer":             "emetteur",
		"derniere revision":  "derniere_revision",
		"dernière révision":  "derniere_revision",
		"last revision":      "derniere_revision",
		"date":               "date",
		"version":            "version",
		"domaine":            "domaine",
		"domain":             "domaine",
		"portee":             "portee",
		"portée":             "portee",
		"scope":              "portee",
	}
)

// ExtractMarkdown parses Markdown content into a flat list of LawbookNodes
// following the hierarchy: H1→document, H2→chapter, H3→section, H4→article.
// Body text under headings becomes paragraph containers with atomic alineas.
// ExtractMarkdownWithSpans parses Markdown with exact source spans.
// sourcePath is the file path used in span metadata.
func ExtractMarkdownWithSpans(source string, docSlug string, sourcePath string) MDExtractResult {
	lines := strings.Split(source, "\n")

	// Precompute byte offsets for each line (1-indexed line → byte offset).
	lineOffsets := computeLineOffsets(source)

	var nodes []LawbookNode
	stack := [5]int{-1, -1, -1, -1, -1}

	i := 0
	for i < len(lines) {
		line := lines[i]

		m := headerRe.FindStringSubmatch(line)
		if m == nil {
			i++
			continue
		}

		headingLine := i // 0-based
		level := len(m[1])
		title := strings.TrimSpace(m[2])
		nodeType := headingToNodeType(level)
		parentIdx := findParent(stack, level)
		parentID := ""
		if parentIdx >= 0 {
			parentID = nodes[parentIdx].NodeID
		}

		canonicalRef := buildCanonicalRef(docSlug, nodeType, title)
		nodeID := computeNodeID(canonicalRef)

		i++
		bodyStart := i
		var bodyLines []string

		var metadata map[string]string
		if level == 1 {
			meta, consumed := parseMetadataTable(lines, i)
			if consumed > 0 {
				metadata = meta
				i += consumed
				bodyStart = i
			}
		}

		for i < len(lines) {
			if headerRe.MatchString(lines[i]) {
				break
			}
			bodyLines = append(bodyLines, lines[i])
			i++
		}

		bodyContent := strings.TrimSpace(strings.Join(bodyLines, "\n"))
		lineEnd := i // exclusive, 0-based

		// Compute span: heading line to last body line (or heading line if no body).
		spanStartLine := headingLine + 1 // 1-indexed
		spanEndLine := lineEnd           // 1-indexed (exclusive → last content line)
		if spanEndLine < spanStartLine {
			spanEndLine = spanStartLine
		}
		byteOff := lineOffset(lineOffsets, headingLine)
		byteEnd := lineOffset(lineOffsets, lineEnd)
		if lineEnd >= len(lines) {
			byteEnd = len(source)
		}

		span := LawbookSourceSpan{
			File:       sourcePath,
			StartLine:  spanStartLine,
			StartCol:   1,
			EndLine:    spanEndLine,
			EndCol:     len(lines[min(lineEnd-1, len(lines)-1)]) + 1,
			ByteOffset: byteOff,
			ByteLength: byteEnd - byteOff,
		}

		displayRef := buildDisplayRef(nodeType, title)

		var metaAny map[string]any
		if metadata != nil {
			metaAny = make(map[string]any, len(metadata))
			for k, v := range metadata {
				metaAny[k] = v
			}
		}

		node := LawbookNode{
			NodeID:       nodeID,
			NodeType:     nodeType,
			Depth:        nodeType.Depth(),
			Title:        title,
			Text:         bodyContent,
			ParentID:     parentID,
			CanonicalRef: canonicalRef,
			DisplayRef:   displayRef,
			Span:         span,
			Metadata:     metaAny,
		}

		nodeIdx := len(nodes)
		nodes = append(nodes, node)

		stack[level] = nodeIdx
		for l := level + 1; l <= 4; l++ {
			stack[l] = -1
		}

		if parentIdx >= 0 {
			_ = nodeID
		}

		if bodyContent != "" {
			subNodes := extractSubNodesWithSpans(bodyContent, nodeID, canonicalRef, bodyStart, lineEnd, lines, lineOffsets, sourcePath)
			nodes = append(nodes, subNodes...)
		}
	}

	if len(nodes) == 0 {
		return MDExtractResult{Nodes: []LawbookNode{}}
	}
	return MDExtractResult{Nodes: nodes}
}

// ExtractMarkdown is the legacy entry point without source path.
func ExtractMarkdown(source string, docSlug string) MDExtractResult {
	return ExtractMarkdownWithSpans(source, docSlug, "")
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
	default:
		return NodeArticle
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

// extractSubNodes is the legacy version without spans.
func extractSubNodes(body string, parentID string, parentRef string, lineStart int, lineEnd int) []LawbookNode {
	return extractSubNodesWithSpans(body, parentID, parentRef, lineStart-1, lineEnd, nil, nil, "")
}

// extractSubNodesWithSpans splits body content into paragraphs/alineas with exact spans.
func extractSubNodesWithSpans(body string, parentID string, parentRef string, bodyStart0 int, bodyEnd0 int, allLines []string, lineOffsets []int, sourcePath string) []LawbookNode {
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
		// Compute paragraph span.
		paraSpan := LawbookSourceSpan{File: sourcePath}
		if lineOffsets != nil {
			// Approximate: paragraph starts at bodyStart0 + offset into body
			paraLine := bodyStart0 + 1 // 1-indexed, conservative
			paraSpan.StartLine = paraLine
			paraSpan.EndLine = paraLine
			paraSpan.StartCol = 1
			paraSpan.ByteOffset = lineOffset(lineOffsets, bodyStart0)
			paraSpan.ByteLength = len(text)
		}

		paragraph := LawbookNode{
			NodeID:       computeNodeID(paragraphRef),
			NodeType:     NodeParagraph,
			Depth:        NodeParagraph.Depth(),
			Title:        "",
			Text:         text,
			ParentID:     parentID,
			CanonicalRef: paragraphRef,
			DisplayRef:   fmt.Sprintf("paragraph %d", pCount),
			Span:         paraSpan,
		}
		nodes = append(nodes, paragraph)

		alineas := atomizeAlineas(text)
		for ai, item := range alineas {
			ref := fmt.Sprintf("%s/alinea/%d", paragraphRef, ai+1)
			alineaSpan := LawbookSourceSpan{
				File: sourcePath, StartCol: 1,
				ByteLength: len(item),
			}
			if paraSpan.IsValid() {
				alineaSpan.StartLine = paraSpan.StartLine + ai
				alineaSpan.EndLine = alineaSpan.StartLine
				alineaSpan.ByteOffset = paraSpan.ByteOffset
			}
			nodes = append(nodes, LawbookNode{
				NodeID:       computeNodeID(ref),
				NodeType:     NodeAlinea,
				Depth:        NodeAlinea.Depth(),
				Title:        "",
				Text:         item,
				ParentID:     paragraph.NodeID,
				CanonicalRef: ref,
				DisplayRef:   fmt.Sprintf("alinea %d", ai+1),
				Span:         alineaSpan,
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

// computeLineOffsets returns byte offset for each line (0-indexed).
func computeLineOffsets(source string) []int {
	offsets := []int{0}
	for i, b := range []byte(source) {
		if b == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

// lineOffset returns byte offset for a 0-indexed line number.
func lineOffset(offsets []int, line0 int) int {
	if line0 < 0 {
		return 0
	}
	if line0 >= len(offsets) {
		if len(offsets) > 0 {
			return offsets[len(offsets)-1]
		}
		return 0
	}
	return offsets[line0]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// splitParagraphs splits text on blank lines.
func splitParagraphs(text string) []string {
	var result []string
	var current strings.Builder
	for _, line := range strings.Split(text, "\n") {
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

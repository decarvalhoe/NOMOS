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
// following the hierarchy: H1→document, H2→chapter, H3→section, H4→article.
// Body text under headings becomes paragraph containers with atomic alineas.
func ExtractMarkdown(source string, docSlug string) MDExtractResult {
	source = normalizeLineEndings(source)
	lines := strings.Split(source, "\n")
	var nodes []LawbookNode

	// Stack tracks the current parent at each depth (index = heading level 1-4).
	// stack[0] is unused, stack[1] = current H1 node index, etc.
	stack := [5]int{-1, -1, -1, -1, -1}

	i := 0
	for i < len(lines) {
		line := lines[i]

		m := headerRe.FindStringSubmatch(line)
		if m == nil {
			// Non-header line outside any heading — skip.
			i++
			continue
		}

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

		// Collect body content until next header or EOF.
		i++
		bodyStart := i
		var bodyLines []string

		// For H1, try to parse metadata table first.
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
			if !isMarkdownStructuralSeparator(lines[i]) {
				bodyLines = append(bodyLines, lines[i])
			}
			i++
		}

		bodyContent := strings.TrimSpace(strings.Join(bodyLines, "\n"))
		lineEnd := i // exclusive, 0-based

		displayRef := buildDisplayRef(nodeType, title)

		// Convert metadata to map[string]any
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
			Metadata:     metaAny,
		}

		nodeIdx := len(nodes)
		nodes = append(nodes, node)

		// Update stack.
		stack[level] = nodeIdx
		// Clear deeper levels.
		for l := level + 1; l <= 4; l++ {
			stack[l] = -1
		}

		// Register as child of parent.
		if parentIdx >= 0 {
			_ = nodeID // parent tracking
		}

		// Extract paragraph/alinea sub-nodes from body content.
		if bodyContent != "" {
			subNodes := extractSubNodes(bodyContent, nodeID, canonicalRef, bodyStart+1, lineEnd)
			nodes = append(nodes, subNodes...)
		}

		_ = bodyStart // used above
	}

	if len(nodes) == 0 {
		return MDExtractResult{Nodes: []LawbookNode{}}
	}
	return MDExtractResult{Nodes: nodes}
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

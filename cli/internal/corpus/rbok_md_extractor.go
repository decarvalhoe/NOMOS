package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// NodeType identifies the structural role of a lawbook node.
type NodeType string

const (
	NodeDocument  NodeType = "document"
	NodeChapter   NodeType = "chapter"
	NodeSection   NodeType = "section"
	NodeArticle   NodeType = "article"
	NodeParagraph NodeType = "paragraph"
	NodeAlinea    NodeType = "alinea"
)

// LawbookNode represents a single structural element extracted from Markdown.
type LawbookNode struct {
	ID           string            `json:"id"`
	Type         NodeType          `json:"type"`
	Depth        int               `json:"depth"`
	Title        string            `json:"title"`
	Content      string            `json:"content,omitempty"`
	ParentID     string            `json:"parent_id,omitempty"`
	ParentChain  []string          `json:"parent_chain"`
	CanonicalRef string            `json:"canonical_ref"`
	DisplayRef   string            `json:"display_ref"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Children     []string          `json:"children,omitempty"`
	LineStart    int               `json:"line_start"`
	LineEnd      int               `json:"line_end"`
}

// ExtractResult holds the full extraction output.
type ExtractResult struct {
	Nodes []LawbookNode `json:"nodes"`
}

var (
	headerRe   = regexp.MustCompile(`^(#{1,4})\s+(.+)$`)
	metaRowRe  = regexp.MustCompile(`^\|\s*(.+?)\s*\|\s*(.+?)\s*\|$`)
	metaSepRe  = regexp.MustCompile(`^\|[\s:_-]+\|[\s:_-]+\|$`)
	metaKeyMap = map[string]string{
		"reference": "reference",
		"référence": "reference",
		"statut":    "statut",
		"status":    "statut",
		"emetteur":  "emetteur",
		"émetteur":  "emetteur",
		"issuer":    "emetteur",
	}
)

// ExtractMarkdown parses Markdown content into a flat list of LawbookNodes
// following the hierarchy: H1→document, H2→chapter, H3→section, H4→article.
// Body text under headings becomes paragraph nodes; list items become alinea nodes.
func ExtractMarkdown(source string, docSlug string) ExtractResult {
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
		headerLine := i + 1 // 1-based

		nodeType := headingToNodeType(level)
		parentIdx := findParent(stack, level)
		parentID := ""
		var parentChain []string
		if parentIdx >= 0 {
			parentID = nodes[parentIdx].ID
			parentChain = append([]string{}, nodes[parentIdx].ParentChain...)
			parentChain = append(parentChain, parentID)
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
			bodyLines = append(bodyLines, lines[i])
			i++
		}

		bodyContent := strings.TrimSpace(strings.Join(bodyLines, "\n"))
		lineEnd := i // exclusive, 0-based

		displayRef := buildDisplayRef(nodeType, title)

		node := LawbookNode{
			ID:           nodeID,
			Type:         nodeType,
			Depth:        level,
			Title:        title,
			Content:      bodyContent,
			ParentID:     parentID,
			ParentChain:  parentChain,
			CanonicalRef: canonicalRef,
			DisplayRef:   displayRef,
			Metadata:     metadata,
			LineStart:    headerLine,
			LineEnd:      lineEnd,
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
			nodes[parentIdx].Children = append(nodes[parentIdx].Children, nodeID)
		}

		// Extract paragraph/alinea sub-nodes from body content.
		if bodyContent != "" {
			subNodes := extractSubNodes(bodyContent, nodeID, canonicalRef, bodyStart+1, lineEnd)
			for si := range subNodes {
				subNodes[si].ParentChain = append(append([]string{}, parentChain...), nodeID)
			}
			nodes = append(nodes, subNodes...)
		}

		_ = bodyStart // used above
	}

	if len(nodes) == 0 {
		return ExtractResult{Nodes: []LawbookNode{}}
	}
	return ExtractResult{Nodes: nodes}
}

func headingToNodeType(level int) NodeType {
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

func buildCanonicalRef(docSlug string, nodeType NodeType, title string) string {
	slug := slugify(title)
	return fmt.Sprintf("%s/%s/%s", docSlug, nodeType, slug)
}

func buildDisplayRef(nodeType NodeType, title string) string {
	return fmt.Sprintf("%s: %s", nodeType, title)
}

func computeNodeID(canonicalRef string) string {
	h := sha256.Sum256([]byte(canonicalRef))
	return "node-" + hex.EncodeToString(h[:8])
}

func slugify(s string) string {
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

// extractSubNodes splits body content into paragraph and alinea nodes.
func extractSubNodes(body string, parentID string, parentRef string, lineStart int, lineEnd int) []LawbookNode {
	var nodes []LawbookNode
	paragraphs := splitParagraphs(body)
	pCount := 0

	for _, para := range paragraphs {
		text := strings.TrimSpace(para)
		if text == "" {
			continue
		}

		// Check if this is a list block (all lines start with - or *).
		listLines := strings.Split(text, "\n")
		isList := true
		for _, ll := range listLines {
			trimmed := strings.TrimSpace(ll)
			if trimmed != "" && !strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "* ") {
				isList = false
				break
			}
		}

		if isList {
			for ai, ll := range listLines {
				item := strings.TrimSpace(ll)
				if item == "" {
					continue
				}
				item = strings.TrimPrefix(item, "- ")
				item = strings.TrimPrefix(item, "* ")
				item = strings.TrimSpace(item)
				ref := fmt.Sprintf("%s/alinea/%d", parentRef, ai+1)
				nodes = append(nodes, LawbookNode{
					ID:           computeNodeID(ref),
					Type:         NodeAlinea,
					Depth:        6,
					Title:        "",
					Content:      item,
					ParentID:     parentID,
					CanonicalRef: ref,
					DisplayRef:   fmt.Sprintf("alinea %d", ai+1),
					LineStart:    lineStart,
					LineEnd:      lineEnd,
				})
			}
		} else {
			pCount++
			ref := fmt.Sprintf("%s/paragraph/%d", parentRef, pCount)
			nodes = append(nodes, LawbookNode{
				ID:           computeNodeID(ref),
				Type:         NodeParagraph,
				Depth:        5,
				Title:        "",
				Content:      text,
				ParentID:     parentID,
				CanonicalRef: ref,
				DisplayRef:   fmt.Sprintf("paragraph %d", pCount),
				LineStart:    lineStart,
				LineEnd:      lineEnd,
			})
		}
	}
	return nodes
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

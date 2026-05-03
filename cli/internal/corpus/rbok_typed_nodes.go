package corpus

import (
	"fmt"
	"regexp"
	"strings"
)

// Regex patterns for typed block detection in Markdown content.
var (
	mdTableRe      = regexp.MustCompile(`(?m)^\|.+\|$`)
	mdTableSepRe   = regexp.MustCompile(`(?m)^\|[\s:|-]+\|$`)
	mdCodeFenceRe  = regexp.MustCompile("(?m)^```(\\w*)\\s*$")
	mdCalloutRe    = regexp.MustCompile(`(?m)^>\s*\[!(NOTE|WARNING|TIP|IMPORTANT|CAUTION)\]`)
	mdBlockquoteRe = regexp.MustCompile(`(?m)^>\s+\S`)
	mdLinkRe       = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	mdImageRe      = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
)

// EmitTypedNodesFromExtraction post-processes an MDExtractResult and
// appends typed semantic nodes (table, code_block, callout, link, image)
// to the node list. This wires AQ-326 into the real corpus feed.
func EmitTypedNodesFromExtraction(result *MDExtractResult) {
	if result == nil {
		return
	}

	var newNodes []LawbookNode

	for i := range result.Nodes {
		node := &result.Nodes[i]
		// Only process paragraphs — alineas duplicate paragraph text
		// and would produce duplicate typed nodes.
		if node.NodeType != NodeParagraph {
			continue
		}

		text := node.Text

		// Detect tables (paragraph that looks like a table).
		if isMarkdownTable(text) {
			tableMetadata := extractTableMetadata(text)
			newNodes = append(newNodes, LawbookNode{
				NodeID:       computeNodeID(node.CanonicalRef + "/table"),
				NodeType:     NodeTable,
				Depth:        node.Depth,
				Title:        "Table",
				Text:         text,
				ParentID:     node.ParentID,
				CanonicalRef: node.CanonicalRef + "/table",
				DisplayRef:   "table",
				Span:         node.Span,
				Metadata:     tableMetadata,
			})
		}

		// Detect code blocks (fenced).
		if codeBlocks := extractCodeBlocks(text, node); len(codeBlocks) > 0 {
			newNodes = append(newNodes, codeBlocks...)
		}

		// Detect callouts.
		if callout := extractCallout(text, node); callout != nil {
			newNodes = append(newNodes, *callout)
		}

		// Detect links.
		links := extractLinks(text, node)
		newNodes = append(newNodes, links...)

		// Detect images.
		images := extractImages(text, node)
		newNodes = append(newNodes, images...)
	}

	result.Nodes = append(result.Nodes, newNodes...)
}

// CountSemanticNodes returns counts of typed semantic nodes in results.
func CountSemanticNodes(nodes []LawbookNode) (tables, codeBlocks, callouts, links, images int) {
	for _, n := range nodes {
		switch n.NodeType {
		case NodeTable:
			tables++
		case NodeCodeBlock:
			codeBlocks++
		case NodeCallout:
			callouts++
		case NodeLink:
			links++
		case NodeImage:
			images++
		}
	}
	return
}

func isMarkdownTable(text string) bool {
	lines := strings.Split(text, "\n")
	if len(lines) < 2 {
		return false
	}
	hasHeader := mdTableRe.MatchString(strings.TrimSpace(lines[0]))
	hasSep := false
	for _, line := range lines[1:] {
		if mdTableSepRe.MatchString(strings.TrimSpace(line)) {
			hasSep = true
			break
		}
	}
	return hasHeader && hasSep
}

func extractTableMetadata(text string) map[string]any {
	lines := markdownTableLines(text)
	if len(lines) == 0 {
		return map[string]any{
			"col_count": "0",
			"row_count": "0",
		}
	}

	headers := splitMarkdownTableRow(lines[0])
	rowCount := 0
	for _, line := range lines[1:] {
		if mdTableSepRe.MatchString(strings.TrimSpace(line)) {
			continue
		}
		if len(splitMarkdownTableRow(line)) > 0 {
			rowCount++
		}
	}

	return map[string]any{
		"col_count": fmt.Sprintf("%d", len(headers)),
		"row_count": fmt.Sprintf("%d", rowCount),
		"headers":   strings.Join(headers, "|"),
	}
}

func markdownTableLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if mdTableRe.MatchString(trimmed) || mdTableSepRe.MatchString(trimmed) {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func splitMarkdownTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cell := strings.TrimSpace(part)
		if cell != "" {
			cells = append(cells, cell)
		}
	}
	return cells
}

func extractCodeBlocks(text string, parent *LawbookNode) []LawbookNode {
	matches := mdCodeFenceRe.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}

	var nodes []LawbookNode
	fenceCount := 0
	var lang string
	var startIdx int

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if fenceCount%2 == 0 {
				// Opening fence.
				lang = strings.TrimPrefix(trimmed, "```")
				lang = strings.TrimSpace(lang)
				startIdx = i
			} else {
				// Closing fence.
				content := strings.Join(lines[startIdx+1:i], "\n")
				ref := fmt.Sprintf("%s/code_block/%d", parent.CanonicalRef, len(nodes))
				declared := lang != ""
				effectiveLang := lang
				if effectiveLang == "" {
					effectiveLang = "plain_text"
				}
				nodes = append(nodes, LawbookNode{
					NodeID:       computeNodeID(ref),
					NodeType:     NodeCodeBlock,
					Depth:        parent.Depth,
					Title:        fmt.Sprintf("code (%s)", effectiveLang),
					Text:         content,
					ParentID:     parent.ParentID,
					CanonicalRef: ref,
					DisplayRef:   fmt.Sprintf("code_block [%s]", effectiveLang),
					Span:         parent.Span,
					Metadata: map[string]any{
						"language":          effectiveLang,
						"language_declared": declared,
					},
				})
			}
			fenceCount++
		}
	}
	return nodes
}

func extractCallout(text string, parent *LawbookNode) *LawbookNode {
	// Try GitHub-style callout first.
	matches := mdCalloutRe.FindStringSubmatch(text)
	if matches != nil {
		calloutType := matches[1]
		lines := strings.Split(text, "\n")
		var content []string
		foundMarker := false
		for _, line := range lines {
			if !foundMarker && mdCalloutRe.MatchString(line) {
				foundMarker = true
				continue
			}
			if foundMarker {
				clean := strings.TrimPrefix(line, "> ")
				clean = strings.TrimPrefix(clean, ">")
				content = append(content, strings.TrimSpace(clean))
			}
		}
		ref := fmt.Sprintf("%s/callout/%s", parent.CanonicalRef, strings.ToLower(calloutType))
		return &LawbookNode{
			NodeID:       computeNodeID(ref),
			NodeType:     NodeCallout,
			Depth:        parent.Depth,
			Title:        calloutType,
			Text:         strings.TrimSpace(strings.Join(content, "\n")),
			ParentID:     parent.ParentID,
			CanonicalRef: ref,
			DisplayRef:   fmt.Sprintf("callout [%s]", calloutType),
			Span:         parent.Span,
			Metadata:     map[string]any{"callout_type": calloutType},
		}
	}

	// Fallback: plain blockquote (> lines) — common in real RBOK content.
	if !mdBlockquoteRe.MatchString(text) {
		return nil
	}
	lines := strings.Split(text, "\n")
	allQuoted := true
	var content []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, ">") {
			allQuoted = false
			break
		}
		clean := strings.TrimPrefix(trimmed, ">")
		clean = strings.TrimSpace(clean)
		content = append(content, clean)
	}
	if !allQuoted || len(content) == 0 {
		return nil
	}
	ref := fmt.Sprintf("%s/callout/blockquote", parent.CanonicalRef)
	return &LawbookNode{
		NodeID:       computeNodeID(ref),
		NodeType:     NodeCallout,
		Depth:        parent.Depth,
		Title:        "blockquote",
		Text:         strings.TrimSpace(strings.Join(content, "\n")),
		ParentID:     parent.ParentID,
		CanonicalRef: ref,
		DisplayRef:   "callout [blockquote]",
		Span:         parent.Span,
		Metadata:     map[string]any{"callout_type": "blockquote"},
	}
}

func extractLinks(text string, parent *LawbookNode) []LawbookNode {
	matches := mdLinkRe.FindAllStringSubmatch(text, -1)
	var nodes []LawbookNode
	for i, m := range matches {
		// Skip images (preceded by !).
		idx := strings.Index(text, m[0])
		if idx > 0 && text[idx-1] == '!' {
			continue
		}
		ref := fmt.Sprintf("%s/link/%d", parent.CanonicalRef, i)
		nodes = append(nodes, LawbookNode{
			NodeID:       computeNodeID(ref),
			NodeType:     NodeLink,
			Depth:        parent.Depth,
			Title:        m[1],
			Text:         m[1],
			ParentID:     parent.NodeID,
			CanonicalRef: ref,
			DisplayRef:   fmt.Sprintf("link: %s", m[1]),
			Span:         parent.Span,
			Metadata:     map[string]any{"href": m[2], "text": m[1]},
		})
	}
	return nodes
}

func extractImages(text string, parent *LawbookNode) []LawbookNode {
	matches := mdImageRe.FindAllStringSubmatch(text, -1)
	var nodes []LawbookNode
	for i, m := range matches {
		ref := fmt.Sprintf("%s/image/%d", parent.CanonicalRef, i)
		nodes = append(nodes, LawbookNode{
			NodeID:       computeNodeID(ref),
			NodeType:     NodeImage,
			Depth:        parent.Depth,
			Title:        m[1],
			Text:         m[1],
			ParentID:     parent.NodeID,
			CanonicalRef: ref,
			DisplayRef:   fmt.Sprintf("image: %s", m[1]),
			Span:         parent.Span,
			Metadata:     map[string]any{"src": m[2], "alt": m[1]},
		})
	}
	return nodes
}

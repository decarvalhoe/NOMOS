package fidelity

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	KindCallout NodeKind = "callout"
)

var (
	calloutRe = regexp.MustCompile(`(?m)^\s*>\s*\[!(NOTE|WARNING|TIP|IMPORTANT|CAUTION)\]\s*$`)
)

// TypedBlocksResult extends a CAST with separate link, image, and callout nodes.
type TypedBlocksResult struct {
	Links    []TypedLink    `json:"links"`
	Images   []TypedImage   `json:"images"`
	Callouts []TypedCallout `json:"callouts"`
}

// TypedLink is a link emitted as its own node.
type TypedLink struct {
	NodeID   string `json:"node_id"`
	ParentID string `json:"parent_id"`
	Text     string `json:"text"`
	Href     string `json:"href"`
	Line     int    `json:"line"`
}

// TypedImage is an image emitted as its own node.
type TypedImage struct {
	NodeID   string `json:"node_id"`
	ParentID string `json:"parent_id"`
	Alt      string `json:"alt"`
	Src      string `json:"src"`
	Line     int    `json:"line"`
}

// TypedCallout is a GFM-style callout/admonition.
type TypedCallout struct {
	NodeID      string `json:"node_id"`
	ParentID    string `json:"parent_id"`
	CalloutType string `json:"callout_type"` // NOTE, WARNING, TIP, etc.
	Content     string `json:"content"`
	Line        int    `json:"line"`
}

// EmitTypedBlocks processes a CAST and emits separate typed nodes for
// links, images, and callouts. It appends these nodes to the CAST
// and returns a summary.
func EmitTypedBlocks(cast *CAST) TypedBlocksResult {
	var result TypedBlocksResult

	// Collect new nodes to append (avoid modifying slice during iteration).
	var newNodes []CNode

	for _, node := range cast.Nodes {
		// Emit link nodes from paragraphs.
		if node.Kind == KindParagraph || node.Kind == KindListItem {
			links := emitLinks(node)
			for _, l := range links {
				result.Links = append(result.Links, l)
				newNodes = append(newNodes, CNode{
					ID:       l.NodeID,
					Kind:     KindLink,
					Text:     l.Text,
					RawText:  fmt.Sprintf("[%s](%s)", l.Text, l.Href),
					ParentID: l.ParentID,
					Span:     Span{StartLine: l.Line, EndLine: l.Line},
					Hash:     hashStr(l.Href),
					Props:    map[string]string{"href": l.Href, "text": l.Text},
				})
			}

			images := emitImages(node)
			for _, img := range images {
				result.Images = append(result.Images, img)
				newNodes = append(newNodes, CNode{
					ID:       img.NodeID,
					Kind:     KindImage,
					Text:     img.Alt,
					RawText:  fmt.Sprintf("![%s](%s)", img.Alt, img.Src),
					ParentID: img.ParentID,
					Span:     Span{StartLine: img.Line, EndLine: img.Line},
					Hash:     hashStr(img.Src),
					Props:    map[string]string{"src": img.Src, "alt": img.Alt},
				})
			}
		}

		// Emit callout nodes from blockquotes.
		if node.Kind == KindBlockquote {
			if callout := emitCallout(node); callout != nil {
				result.Callouts = append(result.Callouts, *callout)
				newNodes = append(newNodes, CNode{
					ID:       callout.NodeID,
					Kind:     KindCallout,
					Text:     callout.Content,
					RawText:  node.RawText,
					ParentID: callout.ParentID,
					Span:     node.Span,
					Hash:     hashStr(callout.Content),
					Props:    map[string]string{"callout_type": callout.CalloutType},
				})
			}
		}
	}

	cast.Nodes = append(cast.Nodes, newNodes...)

	// Update coverage.
	cast.Coverage.Links = len(result.Links)
	cast.Coverage.Images = len(result.Images)

	return result
}

func emitLinks(node CNode) []TypedLink {
	text := node.RawText
	if text == "" {
		text = node.Text
	}
	matches := linkRe.FindAllStringSubmatch(text, -1)
	var links []TypedLink
	for i, m := range matches {
		// Skip image matches.
		idx := strings.Index(text, m[0])
		if idx > 0 && text[idx-1] == '!' {
			continue
		}
		links = append(links, TypedLink{
			NodeID:   fmt.Sprintf("%s-link-%d", node.ID, i),
			ParentID: node.ID,
			Text:     m[1],
			Href:     m[2],
			Line:     node.Span.StartLine,
		})
	}
	return links
}

func emitImages(node CNode) []TypedImage {
	text := node.RawText
	if text == "" {
		text = node.Text
	}
	matches := imageRe.FindAllStringSubmatch(text, -1)
	var images []TypedImage
	for i, m := range matches {
		images = append(images, TypedImage{
			NodeID:   fmt.Sprintf("%s-image-%d", node.ID, i),
			ParentID: node.ID,
			Alt:      m[1],
			Src:      m[2],
			Line:     node.Span.StartLine,
		})
	}
	return images
}

func emitCallout(node CNode) *TypedCallout {
	text := node.RawText
	if text == "" {
		text = node.Text
	}
	matches := calloutRe.FindStringSubmatch(text)
	if matches == nil {
		return nil
	}

	calloutType := matches[1]
	// Extract content after the callout marker line.
	lines := strings.Split(text, "\n")
	var contentLines []string
	foundMarker := false
	for _, line := range lines {
		if !foundMarker && calloutRe.MatchString(line) {
			foundMarker = true
			continue
		}
		if foundMarker {
			clean := strings.TrimPrefix(line, "> ")
			clean = strings.TrimPrefix(clean, ">")
			contentLines = append(contentLines, strings.TrimSpace(clean))
		}
	}
	content := strings.TrimSpace(strings.Join(contentLines, "\n"))

	return &TypedCallout{
		NodeID:      fmt.Sprintf("%s-callout", node.ID),
		ParentID:    node.ID,
		CalloutType: calloutType,
		Content:     content,
		Line:        node.Span.StartLine,
	}
}

// CountTypedBlocks returns counts of typed block elements in a CAST.
func CountTypedBlocks(cast CAST) (tables, codeBlocks, links, images, callouts int) {
	for _, node := range cast.Nodes {
		switch node.Kind {
		case KindTable:
			tables++
		case KindCodeBlock:
			codeBlocks++
		case KindLink:
			links++
		case KindImage:
			images++
		case KindCallout:
			callouts++
		}
	}
	return
}

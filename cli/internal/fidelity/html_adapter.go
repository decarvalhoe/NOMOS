package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// HTML-specific node kinds.
const (
	KindHTMLElement    NodeKind = "html_element"
	KindHTMLText       NodeKind = "html_text"
	KindHTMLComment    NodeKind = "html_comment"
	KindHTMLDoctype    NodeKind = "html_doctype"
)

// ExternalRefKind classifies the type of external reference.
type ExternalRefKind string

const (
	ExtRefImage      ExternalRefKind = "image"
	ExtRefStylesheet ExternalRefKind = "stylesheet"
	ExtRefScript     ExternalRefKind = "script"
	ExtRefLink       ExternalRefKind = "link"
	ExtRefFavicon    ExternalRefKind = "favicon"
	ExtRefFont       ExternalRefKind = "font"
	ExtRefMedia      ExternalRefKind = "media"
	ExtRefOther      ExternalRefKind = "other"
)

// ExternalRef is a reference to an external resource found in HTML.
type ExternalRef struct {
	Kind     ExternalRefKind `json:"kind"`
	URL      string          `json:"url"`
	Tag      string          `json:"tag"`
	Attr     string          `json:"attr"`
	Line     int             `json:"line"`
	IsRemote bool            `json:"is_remote"`
}

// HTMLNode is a node in the HTML AST.
type HTMLNode struct {
	ID       string            `json:"id"`
	Kind     NodeKind          `json:"kind"`
	Tag      string            `json:"tag,omitempty"`
	Text     string            `json:"text,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
	Children []string          `json:"children,omitempty"`
	ParentID string            `json:"parent_id,omitempty"`
	Depth    int               `json:"depth"`
	Line     int               `json:"line"`
	Hash     string            `json:"hash"`
}

// HAST is the HTML AST output.
type HAST struct {
	Root         string        `json:"root"`
	Nodes        []HTMLNode    `json:"nodes"`
	ExternalRefs []ExternalRef `json:"external_refs"`
	SourceHash   string        `json:"source_hash"`
	TotalNodes   int           `json:"total_nodes"`
	RefSummary   RefSummary    `json:"ref_summary"`
}

// RefSummary counts external references by kind.
type RefSummary struct {
	Images      int `json:"images"`
	Stylesheets int `json:"stylesheets"`
	Scripts     int `json:"scripts"`
	Links       int `json:"links"`
	Fonts       int `json:"fonts"`
	Media       int `json:"media"`
	Other       int `json:"other"`
	TotalRemote int `json:"total_remote"`
	TotalLocal  int `json:"total_local"`
}

var (
	htmlTagRe      = regexp.MustCompile(`<(/?)([a-zA-Z][a-zA-Z0-9]*)\b([^>]*)(/?)>`)
	htmlAttrRe     = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9_-]*)=(?:"([^"]*)"|'([^']*)')`)
	htmlCommentRe  = regexp.MustCompile(`<!--(.*?)-->`)
	htmlDoctypeRe  = regexp.MustCompile(`(?i)<!DOCTYPE\s+[^>]+>`)
	htmlSelfClose  = map[string]bool{
		"br": true, "hr": true, "img": true, "input": true,
		"meta": true, "link": true, "area": true, "base": true,
		"col": true, "embed": true, "source": true, "track": true, "wbr": true,
	}
)

// ParseHTML parses HTML source into an HAST with external reference tracking.
func ParseHTML(source string) HAST {
	h := sha256.Sum256([]byte(source))
	hast := HAST{SourceHash: hex.EncodeToString(h[:])}

	lines := strings.Split(source, "\n")
	rootID := htmlNodeID("document", 0)
	hast.Root = rootID
	hast.Nodes = append(hast.Nodes, HTMLNode{
		ID:   rootID,
		Kind: KindDocument,
		Tag:  "document",
		Hash: hast.SourceHash,
		Line: 1,
	})

	var stack []string
	stack = append(stack, rootID)

	for lineNum, line := range lines {
		lineIdx := lineNum + 1

		// Doctype.
		if htmlDoctypeRe.MatchString(line) {
			parent := stack[len(stack)-1]
			node := HTMLNode{
				ID:       htmlNodeID("doctype", lineNum),
				Kind:     KindHTMLDoctype,
				Text:     strings.TrimSpace(htmlDoctypeRe.FindString(line)),
				ParentID: parent,
				Depth:    len(stack) - 1,
				Line:     lineIdx,
				Hash:     htmlHash(line),
			}
			hast.Nodes = append(hast.Nodes, node)
			addHTMLChild(&hast, parent, node.ID)
			continue
		}

		// Comments.
		for _, m := range htmlCommentRe.FindAllStringSubmatchIndex(line, -1) {
			commentText := line[m[2]:m[3]]
			parent := stack[len(stack)-1]
			node := HTMLNode{
				ID:       htmlNodeID("comment", lineNum+m[0]),
				Kind:     KindHTMLComment,
				Text:     strings.TrimSpace(commentText),
				ParentID: parent,
				Depth:    len(stack) - 1,
				Line:     lineIdx,
				Hash:     htmlHash(commentText),
			}
			hast.Nodes = append(hast.Nodes, node)
			addHTMLChild(&hast, parent, node.ID)
		}

		// Tags.
		for _, match := range htmlTagRe.FindAllStringSubmatch(line, -1) {
			isClose := match[1] == "/"
			tag := strings.ToLower(match[2])
			attrStr := match[3]
			isSelfClose := match[4] == "/" || htmlSelfClose[tag]

			if isClose {
				// Pop stack if matching.
				if len(stack) > 1 {
					stack = stack[:len(stack)-1]
				}
				continue
			}

			parent := stack[len(stack)-1]
			attrs := parseAttrs(attrStr)

			node := HTMLNode{
				ID:       htmlNodeID(fmt.Sprintf("%s-%d", tag, lineNum), len(hast.Nodes)),
				Kind:     KindHTMLElement,
				Tag:      tag,
				Attrs:    attrs,
				ParentID: parent,
				Depth:    len(stack) - 1,
				Line:     lineIdx,
				Hash:     htmlHash(match[0]),
			}
			hast.Nodes = append(hast.Nodes, node)
			addHTMLChild(&hast, parent, node.ID)

			// Extract external refs.
			refs := extractExternalRefs(tag, attrs, lineIdx)
			hast.ExternalRefs = append(hast.ExternalRefs, refs...)

			if !isSelfClose {
				stack = append(stack, node.ID)
			}
		}

		// Text content (lines without tags, or between tags).
		trimmed := strings.TrimSpace(htmlTagRe.ReplaceAllString(line, ""))
		trimmed = strings.TrimSpace(htmlCommentRe.ReplaceAllString(trimmed, ""))
		trimmed = strings.TrimSpace(htmlDoctypeRe.ReplaceAllString(trimmed, ""))
		if trimmed != "" {
			parent := stack[len(stack)-1]
			node := HTMLNode{
				ID:       htmlNodeID(fmt.Sprintf("text-%d", lineNum), len(hast.Nodes)),
				Kind:     KindHTMLText,
				Text:     trimmed,
				ParentID: parent,
				Depth:    len(stack) - 1,
				Line:     lineIdx,
				Hash:     htmlHash(trimmed),
			}
			hast.Nodes = append(hast.Nodes, node)
			addHTMLChild(&hast, parent, node.ID)
		}
	}

	hast.TotalNodes = len(hast.Nodes)
	hast.RefSummary = summarizeRefs(hast.ExternalRefs)
	return hast
}

func extractExternalRefs(tag string, attrs map[string]string, line int) []ExternalRef {
	var refs []ExternalRef

	switch tag {
	case "img":
		if src := attrs["src"]; src != "" {
			refs = append(refs, ExternalRef{Kind: ExtRefImage, URL: src, Tag: tag, Attr: "src", Line: line, IsRemote: isRemoteURL(src)})
		}
	case "link":
		rel := strings.ToLower(attrs["rel"])
		href := attrs["href"]
		if href == "" {
			break
		}
		switch {
		case rel == "stylesheet":
			refs = append(refs, ExternalRef{Kind: ExtRefStylesheet, URL: href, Tag: tag, Attr: "href", Line: line, IsRemote: isRemoteURL(href)})
		case strings.Contains(rel, "icon"):
			refs = append(refs, ExternalRef{Kind: ExtRefFavicon, URL: href, Tag: tag, Attr: "href", Line: line, IsRemote: isRemoteURL(href)})
		case rel == "preload" && strings.Contains(attrs["as"], "font"):
			refs = append(refs, ExternalRef{Kind: ExtRefFont, URL: href, Tag: tag, Attr: "href", Line: line, IsRemote: isRemoteURL(href)})
		default:
			refs = append(refs, ExternalRef{Kind: ExtRefLink, URL: href, Tag: tag, Attr: "href", Line: line, IsRemote: isRemoteURL(href)})
		}
	case "script":
		if src := attrs["src"]; src != "" {
			refs = append(refs, ExternalRef{Kind: ExtRefScript, URL: src, Tag: tag, Attr: "src", Line: line, IsRemote: isRemoteURL(src)})
		}
	case "a":
		if href := attrs["href"]; href != "" && isRemoteURL(href) {
			refs = append(refs, ExternalRef{Kind: ExtRefLink, URL: href, Tag: tag, Attr: "href", Line: line, IsRemote: true})
		}
	case "video", "audio":
		if src := attrs["src"]; src != "" {
			refs = append(refs, ExternalRef{Kind: ExtRefMedia, URL: src, Tag: tag, Attr: "src", Line: line, IsRemote: isRemoteURL(src)})
		}
	case "source":
		if src := attrs["src"]; src != "" {
			refs = append(refs, ExternalRef{Kind: ExtRefMedia, URL: src, Tag: tag, Attr: "src", Line: line, IsRemote: isRemoteURL(src)})
		}
	}

	// Background image in style attribute.
	if style := attrs["style"]; style != "" {
		if strings.Contains(style, "url(") {
			urlStart := strings.Index(style, "url(") + 4
			urlEnd := strings.Index(style[urlStart:], ")")
			if urlEnd > 0 {
				url := strings.Trim(style[urlStart:urlStart+urlEnd], "\"' ")
				refs = append(refs, ExternalRef{Kind: ExtRefImage, URL: url, Tag: tag, Attr: "style", Line: line, IsRemote: isRemoteURL(url)})
			}
		}
	}

	return refs
}

func isRemoteURL(url string) bool {
	lower := strings.ToLower(url)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "//")
}

func parseAttrs(s string) map[string]string {
	attrs := map[string]string{}
	for _, m := range htmlAttrRe.FindAllStringSubmatch(s, -1) {
		key := strings.ToLower(m[1])
		val := m[2]
		if val == "" {
			val = m[3]
		}
		attrs[key] = val
	}
	if len(attrs) == 0 {
		return nil
	}
	return attrs
}

func summarizeRefs(refs []ExternalRef) RefSummary {
	var s RefSummary
	for _, r := range refs {
		switch r.Kind {
		case ExtRefImage:
			s.Images++
		case ExtRefStylesheet:
			s.Stylesheets++
		case ExtRefScript:
			s.Scripts++
		case ExtRefLink:
			s.Links++
		case ExtRefFont:
			s.Fonts++
		case ExtRefMedia:
			s.Media++
		case ExtRefFavicon:
			s.Images++
		default:
			s.Other++
		}
		if r.IsRemote {
			s.TotalRemote++
		} else {
			s.TotalLocal++
		}
	}
	return s
}

func htmlNodeID(prefix string, idx int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("html:%s:%d", prefix, idx)))
	return "hn-" + hex.EncodeToString(h[:6])
}

func htmlHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func addHTMLChild(hast *HAST, parentID, childID string) {
	for i := range hast.Nodes {
		if hast.Nodes[i].ID == parentID {
			hast.Nodes[i].Children = append(hast.Nodes[i].Children, childID)
			return
		}
	}
}

// HASTToCAST converts an HAST to the generic CAST format.
func HASTToCAST(hast HAST) CAST {
	nodes := make([]CNode, 0, len(hast.Nodes))
	for _, hn := range hast.Nodes {
		props := map[string]string{}
		if hn.Tag != "" {
			props["tag"] = hn.Tag
		}
		for k, v := range hn.Attrs {
			props["attr_"+k] = v
		}
		if len(props) == 0 {
			props = nil
		}

		nodes = append(nodes, CNode{
			ID:       hn.ID,
			Kind:     hn.Kind,
			Text:     hn.Text,
			Children: hn.Children,
			ParentID: hn.ParentID,
			Level:    hn.Depth,
			Hash:     hn.Hash,
			Props:    props,
			Span:     Span{StartLine: hn.Line, EndLine: hn.Line},
		})
	}
	return CAST{Root: hast.Root, Nodes: nodes, SourceHash: hast.SourceHash}
}

// ParseHTMLToCAST is a convenience function that parses HTML into CAST.
func ParseHTMLToCAST(source string) CAST {
	return HASTToCAST(ParseHTML(source))
}

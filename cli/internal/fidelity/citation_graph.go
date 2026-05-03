package fidelity

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// RefKind classifies the type of reference.
type RefKind string

const (
	RefInternal      RefKind = "internal"      // node-to-node within same document
	RefCrossDocument RefKind = "cross_document" // node-to-node in different document
	RefExternal      RefKind = "external"      // node-to-URL or external resource
	RefAnchor        RefKind = "anchor"        // heading/id anchor link
)

// Citation represents a single reference from a source node to a target.
type Citation struct {
	SourceNodeID string  `json:"source_node_id"`
	TargetID     string  `json:"target_id"`
	Kind         RefKind `json:"kind"`
	Label        string  `json:"label,omitempty"`
	Line         int     `json:"line,omitempty"`
	Broken       bool    `json:"broken"`
	Reason       string  `json:"reason,omitempty"`
}

// CitationGraph holds all citations and provides query methods.
type CitationGraph struct {
	Citations    []Citation        `json:"citations"`
	NodeIDs      map[string]bool   `json:"-"`
	AnchorIDs    map[string]string `json:"-"` // anchor -> owning node ID
	DocumentIDs  map[string]bool   `json:"-"`
}

// CitationGraphSummary provides aggregate stats.
type CitationGraphSummary struct {
	TotalCitations  int            `json:"total_citations"`
	InternalCount   int            `json:"internal_count"`
	CrossDocCount   int            `json:"cross_document_count"`
	ExternalCount   int            `json:"external_count"`
	AnchorCount     int            `json:"anchor_count"`
	BrokenCount     int            `json:"broken_count"`
	BySource        map[string]int `json:"by_source"`
	BrokenRefs      []Citation     `json:"broken_refs,omitempty"`
}

// LinkInput represents a link found in the AST for citation extraction.
type LinkInput struct {
	SourceNodeID string
	Target       string // URL, #anchor, or node reference
	Label        string
	Line         int
}

// GraphConfig configures citation graph building.
type GraphConfig struct {
	KnownNodeIDs    []string // all valid node IDs in the corpus
	KnownAnchors   map[string]string // anchor ID -> owning node ID
	KnownDocuments []string // valid document IDs for cross-doc detection
}

var (
	urlPattern    = regexp.MustCompile(`^https?://`)
	anchorPattern = regexp.MustCompile(`^#[a-zA-Z0-9_-]+$`)
)

// BuildCitationGraph constructs a citation graph from extracted links.
func BuildCitationGraph(links []LinkInput, config GraphConfig) CitationGraph {
	g := CitationGraph{
		NodeIDs:     make(map[string]bool, len(config.KnownNodeIDs)),
		AnchorIDs:   config.KnownAnchors,
		DocumentIDs: make(map[string]bool, len(config.KnownDocuments)),
	}
	if g.AnchorIDs == nil {
		g.AnchorIDs = map[string]string{}
	}
	for _, id := range config.KnownNodeIDs {
		g.NodeIDs[id] = true
	}
	for _, id := range config.KnownDocuments {
		g.DocumentIDs[id] = true
	}

	for _, link := range links {
		citation := classifyLink(link, g)
		g.Citations = append(g.Citations, citation)
	}

	return g
}

// BrokenRefs returns all citations marked as broken.
func (g CitationGraph) BrokenRefs() []Citation {
	var broken []Citation
	for _, c := range g.Citations {
		if c.Broken {
			broken = append(broken, c)
		}
	}
	return broken
}

// OutgoingFrom returns all citations originating from a given node.
func (g CitationGraph) OutgoingFrom(nodeID string) []Citation {
	var result []Citation
	for _, c := range g.Citations {
		if c.SourceNodeID == nodeID {
			result = append(result, c)
		}
	}
	return result
}

// IncomingTo returns all citations pointing to a given target.
func (g CitationGraph) IncomingTo(targetID string) []Citation {
	var result []Citation
	for _, c := range g.Citations {
		if c.TargetID == targetID {
			result = append(result, c)
		}
	}
	return result
}

// Summarize produces aggregate statistics for the graph.
func (g CitationGraph) Summarize() CitationGraphSummary {
	s := CitationGraphSummary{
		TotalCitations: len(g.Citations),
		BySource:       map[string]int{},
	}

	for _, c := range g.Citations {
		s.BySource[c.SourceNodeID]++
		switch c.Kind {
		case RefInternal:
			s.InternalCount++
		case RefCrossDocument:
			s.CrossDocCount++
		case RefExternal:
			s.ExternalCount++
		case RefAnchor:
			s.AnchorCount++
		}
		if c.Broken {
			s.BrokenCount++
			s.BrokenRefs = append(s.BrokenRefs, c)
		}
	}

	sort.Slice(s.BrokenRefs, func(i, j int) bool {
		return s.BrokenRefs[i].TargetID < s.BrokenRefs[j].TargetID
	})

	return s
}

func classifyLink(link LinkInput, g CitationGraph) Citation {
	target := strings.TrimSpace(link.Target)
	c := Citation{
		SourceNodeID: link.SourceNodeID,
		TargetID:     target,
		Label:        link.Label,
		Line:         link.Line,
	}

	switch {
	case urlPattern.MatchString(target):
		c.Kind = RefExternal
		// External links are not validated for brokenness (requires HTTP).

	case anchorPattern.MatchString(target):
		c.Kind = RefAnchor
		anchor := strings.TrimPrefix(target, "#")
		if _, ok := g.AnchorIDs[anchor]; !ok {
			c.Broken = true
			c.Reason = fmt.Sprintf("anchor %q not found in known anchors", anchor)
		}

	case g.NodeIDs[target]:
		c.Kind = RefInternal

	case g.DocumentIDs[target]:
		c.Kind = RefCrossDocument

	default:
		// Try to determine if it looks like a node ref or is broken.
		if looksLikeNodeRef(target) {
			c.Kind = RefInternal
			c.Broken = true
			c.Reason = fmt.Sprintf("target %q not found in known node IDs", target)
		} else if looksLikeCrossDocRef(target) {
			c.Kind = RefCrossDocument
			c.Broken = true
			c.Reason = fmt.Sprintf("target %q not found in known documents", target)
		} else {
			c.Kind = RefExternal
		}
	}

	return c
}

func looksLikeNodeRef(target string) bool {
	// Node refs typically are uppercase with dashes/dots.
	if len(target) == 0 {
		return false
	}
	upper := strings.ToUpper(target)
	return upper == target && !strings.Contains(target, "/") && !strings.Contains(target, ".")
}

func looksLikeCrossDocRef(target string) bool {
	// Cross-doc refs contain a path separator or doc prefix.
	return strings.Contains(target, "/") && !urlPattern.MatchString(target)
}

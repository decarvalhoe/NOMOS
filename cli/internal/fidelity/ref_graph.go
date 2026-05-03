package fidelity

import (
	"fmt"
	"sort"
	"strings"
)

// EdgeType classifies the kind of reference link.
type EdgeType string

const (
	EdgeInternal  EdgeType = "internal"
	EdgeExternal  EdgeType = "external"
	EdgeCrossDoc  EdgeType = "cross_doc"
)

// GraphNode represents a referenceable unit in the graph.
type GraphNode struct {
	ID       string `json:"id"`
	Document string `json:"document,omitempty"`
	Title    string `json:"title,omitempty"`
}

// GraphEdge represents a directed reference from source to target.
type GraphEdge struct {
	SourceID string   `json:"source_id"`
	TargetID string   `json:"target_id"`
	Type     EdgeType `json:"type"`
	Label    string   `json:"label,omitempty"`
	Line     int      `json:"line,omitempty"`
}

// BrokenRef records an unresolved reference.
type BrokenRef struct {
	SourceID string   `json:"source_id"`
	TargetID string   `json:"target_id"`
	Type     EdgeType `json:"type"`
	Label    string   `json:"label,omitempty"`
	Line     int      `json:"line,omitempty"`
	Reason   string   `json:"reason"`
}

// RefGraph holds the complete reference graph for analysis.
type RefGraph struct {
	Nodes      []GraphNode  `json:"nodes"`
	Edges      []GraphEdge  `json:"edges"`
	BrokenRefs []BrokenRef  `json:"broken_refs"`
	nodeIndex  map[string]bool
}

// NewRefGraph creates an empty reference graph.
func NewRefGraph() *RefGraph {
	return &RefGraph{
		nodeIndex: make(map[string]bool),
	}
}

// AddNode registers a referenceable node in the graph.
func (g *RefGraph) AddNode(id, document, title string) {
	if g.nodeIndex[id] {
		return
	}
	g.nodeIndex[id] = true
	g.Nodes = append(g.Nodes, GraphNode{ID: id, Document: document, Title: title})
}

// AddEdge adds a directed reference edge between two nodes.
func (g *RefGraph) AddEdge(sourceID, targetID string, edgeType EdgeType, label string, line int) {
	g.Edges = append(g.Edges, GraphEdge{
		SourceID: sourceID,
		TargetID: targetID,
		Type:     edgeType,
		Label:    label,
		Line:     line,
	})
}

// Resolve validates all edges and identifies broken references.
func (g *RefGraph) Resolve() {
	g.BrokenRefs = nil
	for _, edge := range g.Edges {
		if !g.nodeIndex[edge.TargetID] {
			g.BrokenRefs = append(g.BrokenRefs, BrokenRef{
				SourceID: edge.SourceID,
				TargetID: edge.TargetID,
				Type:     edge.Type,
				Label:    edge.Label,
				Line:     edge.Line,
				Reason:   fmt.Sprintf("target %q not found in graph", edge.TargetID),
			})
		}
	}
}

// IsResolved returns true if there are no broken references.
func (g *RefGraph) IsResolved() bool {
	return len(g.BrokenRefs) == 0
}

// IncomingEdges returns all edges pointing to a given node.
func (g *RefGraph) IncomingEdges(nodeID string) []GraphEdge {
	var result []GraphEdge
	for _, e := range g.Edges {
		if e.TargetID == nodeID {
			result = append(result, e)
		}
	}
	return result
}

// OutgoingEdges returns all edges originating from a given node.
func (g *RefGraph) OutgoingEdges(nodeID string) []GraphEdge {
	var result []GraphEdge
	for _, e := range g.Edges {
		if e.SourceID == nodeID {
			result = append(result, e)
		}
	}
	return result
}

// OrphanNodes returns nodes with no incoming or outgoing edges.
func (g *RefGraph) OrphanNodes() []GraphNode {
	connected := map[string]bool{}
	for _, e := range g.Edges {
		connected[e.SourceID] = true
		connected[e.TargetID] = true
	}
	var orphans []GraphNode
	for _, n := range g.Nodes {
		if !connected[n.ID] {
			orphans = append(orphans, n)
		}
	}
	return orphans
}

// Stats returns summary statistics for the graph.
func (g *RefGraph) Stats() GraphStats {
	internal, external, crossDoc := 0, 0, 0
	for _, e := range g.Edges {
		switch e.Type {
		case EdgeInternal:
			internal++
		case EdgeExternal:
			external++
		case EdgeCrossDoc:
			crossDoc++
		}
	}
	return GraphStats{
		TotalNodes:    len(g.Nodes),
		TotalEdges:    len(g.Edges),
		InternalEdges: internal,
		ExternalEdges: external,
		CrossDocEdges: crossDoc,
		BrokenCount:   len(g.BrokenRefs),
		OrphanCount:   len(g.OrphanNodes()),
	}
}

// GraphStats summarizes the reference graph.
type GraphStats struct {
	TotalNodes    int `json:"total_nodes"`
	TotalEdges    int `json:"total_edges"`
	InternalEdges int `json:"internal_edges"`
	ExternalEdges int `json:"external_edges"`
	CrossDocEdges int `json:"cross_doc_edges"`
	BrokenCount   int `json:"broken_count"`
	OrphanCount   int `json:"orphan_count"`
}

// RefCandidateInput is the input type for building graphs from detected references.
type RefCandidateInput struct {
	Type   string `json:"type"` // "annex", "bibliographic", "cross_reference"
	Target string `json:"target"`
	Label  string `json:"label,omitempty"`
	Line   int    `json:"line"`
}

// BuildFromCandidates constructs a graph from reference candidates and known node IDs.
func BuildFromCandidates(sourceDoc string, candidates []RefCandidateInput, knownNodes []GraphNode) *RefGraph {
	g := NewRefGraph()

	// Register known nodes.
	for _, n := range knownNodes {
		g.AddNode(n.ID, n.Document, n.Title)
	}

	// Ensure source doc is a node.
	g.AddNode(sourceDoc, sourceDoc, "")

	// Process candidates into edges.
	for _, c := range candidates {
		targetID := normalizeTargetID(c.Target)
		edgeType := classifyEdgeTypeFromCandidate(c, sourceDoc)

		g.AddEdge(sourceDoc, targetID, edgeType, c.Label, c.Line)
	}

	g.Resolve()
	return g
}

// TopologicalOrder returns nodes in dependency order (sources before targets).
// Returns nil if graph has cycles.
func (g *RefGraph) TopologicalOrder() []string {
	inDegree := map[string]int{}
	for _, n := range g.Nodes {
		inDegree[n.ID] = 0
	}
	for _, e := range g.Edges {
		if g.nodeIndex[e.TargetID] {
			inDegree[e.TargetID]++
		}
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, e := range g.Edges {
			if e.SourceID == node && g.nodeIndex[e.TargetID] {
				inDegree[e.TargetID]--
				if inDegree[e.TargetID] == 0 {
					queue = append(queue, e.TargetID)
					sort.Strings(queue)
				}
			}
		}
	}

	if len(order) < len(g.Nodes) {
		return nil // cycle detected
	}
	return order
}

func normalizeTargetID(target string) string {
	target = strings.TrimSpace(target)
	target = strings.TrimPrefix(target, "#")
	target = strings.TrimPrefix(target, "./")
	target = strings.TrimPrefix(target, "../")
	return target
}

func classifyEdgeTypeFromCandidate(c RefCandidateInput, sourceDoc string) EdgeType {
	target := c.Target
	switch {
	case strings.HasPrefix(target, "#"):
		return EdgeInternal
	case strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../"):
		return EdgeCrossDoc
	case strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://"):
		return EdgeExternal
	case c.Type == "annex":
		return EdgeInternal
	case c.Type == "bibliographic":
		return EdgeExternal
	case c.Type == "cross_reference":
		return EdgeInternal
	default:
		return EdgeInternal
	}
}

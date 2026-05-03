package fidelity

import (
	"testing"
)

func TestNewRefGraphEmpty(t *testing.T) {
	g := NewRefGraph()
	if len(g.Nodes) != 0 || len(g.Edges) != 0 {
		t.Fatal("expected empty graph")
	}
	g.Resolve()
	if !g.IsResolved() {
		t.Fatal("empty graph should be resolved")
	}
}

func TestAddNodeDeduplicates(t *testing.T) {
	g := NewRefGraph()
	g.AddNode("A", "doc1", "Title A")
	g.AddNode("A", "doc1", "Title A")
	g.AddNode("B", "doc1", "Title B")

	if len(g.Nodes) != 2 {
		t.Fatalf("expected 2 nodes (dedup), got %d", len(g.Nodes))
	}
}

func TestResolveDetectsBrokenRefs(t *testing.T) {
	g := NewRefGraph()
	g.AddNode("A", "doc1", "")
	g.AddNode("B", "doc1", "")
	g.AddEdge("A", "B", EdgeInternal, "", 1)
	g.AddEdge("A", "C", EdgeInternal, "missing", 5) // C not in graph

	g.Resolve()

	if g.IsResolved() {
		t.Fatal("expected broken refs")
	}
	if len(g.BrokenRefs) != 1 {
		t.Fatalf("expected 1 broken ref, got %d", len(g.BrokenRefs))
	}
	if g.BrokenRefs[0].TargetID != "C" {
		t.Fatalf("expected broken target C, got %s", g.BrokenRefs[0].TargetID)
	}
}

func TestResolveAllValid(t *testing.T) {
	g := NewRefGraph()
	g.AddNode("A", "doc1", "")
	g.AddNode("B", "doc1", "")
	g.AddEdge("A", "B", EdgeInternal, "", 1)
	g.AddEdge("B", "A", EdgeInternal, "", 2)

	g.Resolve()

	if !g.IsResolved() {
		t.Fatal("expected all resolved")
	}
}

func TestIncomingEdges(t *testing.T) {
	g := NewRefGraph()
	g.AddNode("A", "", "")
	g.AddNode("B", "", "")
	g.AddNode("C", "", "")
	g.AddEdge("A", "C", EdgeInternal, "", 0)
	g.AddEdge("B", "C", EdgeCrossDoc, "", 0)

	incoming := g.IncomingEdges("C")
	if len(incoming) != 2 {
		t.Fatalf("expected 2 incoming to C, got %d", len(incoming))
	}
}

func TestOutgoingEdges(t *testing.T) {
	g := NewRefGraph()
	g.AddNode("A", "", "")
	g.AddNode("B", "", "")
	g.AddNode("C", "", "")
	g.AddEdge("A", "B", EdgeInternal, "", 0)
	g.AddEdge("A", "C", EdgeExternal, "", 0)

	outgoing := g.OutgoingEdges("A")
	if len(outgoing) != 2 {
		t.Fatalf("expected 2 outgoing from A, got %d", len(outgoing))
	}
}

func TestOrphanNodes(t *testing.T) {
	g := NewRefGraph()
	g.AddNode("A", "", "")
	g.AddNode("B", "", "")
	g.AddNode("ORPHAN", "", "")
	g.AddEdge("A", "B", EdgeInternal, "", 0)

	orphans := g.OrphanNodes()
	if len(orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(orphans))
	}
	if orphans[0].ID != "ORPHAN" {
		t.Fatalf("expected ORPHAN, got %s", orphans[0].ID)
	}
}

func TestStats(t *testing.T) {
	g := NewRefGraph()
	g.AddNode("A", "", "")
	g.AddNode("B", "", "")
	g.AddEdge("A", "B", EdgeInternal, "", 0)
	g.AddEdge("A", "ext", EdgeExternal, "", 0)
	g.AddEdge("A", "other.md", EdgeCrossDoc, "", 0)
	g.Resolve()

	stats := g.Stats()
	if stats.TotalNodes != 2 {
		t.Fatalf("expected 2 nodes, got %d", stats.TotalNodes)
	}
	if stats.TotalEdges != 3 {
		t.Fatalf("expected 3 edges, got %d", stats.TotalEdges)
	}
	if stats.InternalEdges != 1 {
		t.Fatalf("expected 1 internal, got %d", stats.InternalEdges)
	}
	if stats.ExternalEdges != 1 {
		t.Fatalf("expected 1 external, got %d", stats.ExternalEdges)
	}
	if stats.CrossDocEdges != 1 {
		t.Fatalf("expected 1 cross-doc, got %d", stats.CrossDocEdges)
	}
	if stats.BrokenCount != 2 { // ext and other.md not in nodes
		t.Fatalf("expected 2 broken, got %d", stats.BrokenCount)
	}
}

func TestBuildFromCandidates(t *testing.T) {
	knownNodes := []GraphNode{
		{ID: "section-1", Document: "doc.md", Title: "Section 1"},
		{ID: "annex-a", Document: "doc.md", Title: "Annex A"},
	}
	candidates := []RefCandidateInput{
		{Type: "cross_reference", Target: "#section-1", Line: 5},
		{Type: "annex", Target: "A", Label: "Annex A", Line: 10},
		{Type: "bibliographic", Target: "RFC 2119", Line: 15},
	}

	g := BuildFromCandidates("doc.md", candidates, knownNodes)

	if len(g.Nodes) != 3 { // 2 known + doc.md
		t.Fatalf("expected 3 nodes, got %d", len(g.Nodes))
	}
	if len(g.Edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(g.Edges))
	}

	// section-1 ref should resolve (it's in known nodes).
	g.Resolve()
	// "A" (annex) and "RFC 2119" are not in known nodes → broken.
	brokenTargets := map[string]bool{}
	for _, b := range g.BrokenRefs {
		brokenTargets[b.TargetID] = true
	}
	if brokenTargets["section-1"] {
		t.Fatal("section-1 should resolve, not be broken")
	}
}

func TestBuildFromCandidatesEdgeTypes(t *testing.T) {
	candidates := []RefCandidateInput{
		{Type: "cross_reference", Target: "#internal", Line: 1},
		{Type: "cross_reference", Target: "./other.md", Line: 2},
		{Type: "bibliographic", Target: "ISO 27001", Line: 3},
	}

	g := BuildFromCandidates("src.md", candidates, nil)

	types := map[EdgeType]int{}
	for _, e := range g.Edges {
		types[e.Type]++
	}
	if types[EdgeInternal] != 1 {
		t.Fatalf("expected 1 internal, got %d", types[EdgeInternal])
	}
	if types[EdgeCrossDoc] != 1 {
		t.Fatalf("expected 1 cross-doc, got %d", types[EdgeCrossDoc])
	}
	if types[EdgeExternal] != 1 {
		t.Fatalf("expected 1 external, got %d", types[EdgeExternal])
	}
}

func TestTopologicalOrderAcyclic(t *testing.T) {
	g := NewRefGraph()
	g.AddNode("A", "", "")
	g.AddNode("B", "", "")
	g.AddNode("C", "", "")
	g.AddEdge("A", "B", EdgeInternal, "", 0)
	g.AddEdge("B", "C", EdgeInternal, "", 0)

	order := g.TopologicalOrder()
	if order == nil {
		t.Fatal("expected non-nil order for acyclic graph")
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 in order, got %d", len(order))
	}
	// A must come before B, B before C.
	posA, posB, posC := indexOf(order, "A"), indexOf(order, "B"), indexOf(order, "C")
	if posA > posB || posB > posC {
		t.Fatalf("wrong order: %v", order)
	}
}

func TestTopologicalOrderDetectsCycle(t *testing.T) {
	g := NewRefGraph()
	g.AddNode("A", "", "")
	g.AddNode("B", "", "")
	g.AddEdge("A", "B", EdgeInternal, "", 0)
	g.AddEdge("B", "A", EdgeInternal, "", 0)

	order := g.TopologicalOrder()
	if order != nil {
		t.Fatalf("expected nil for cyclic graph, got %v", order)
	}
}

func TestResolveExternalEdgesAlwaysBroken(t *testing.T) {
	g := NewRefGraph()
	g.AddNode("doc", "", "")
	g.AddEdge("doc", "https://example.com", EdgeExternal, "", 1)
	g.Resolve()

	// External targets are not registered as nodes → broken.
	if len(g.BrokenRefs) != 1 {
		t.Fatalf("expected 1 broken (external), got %d", len(g.BrokenRefs))
	}
}

func indexOf(slice []string, val string) int {
	for i, s := range slice {
		if s == val {
			return i
		}
	}
	return -1
}

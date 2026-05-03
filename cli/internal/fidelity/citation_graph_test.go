package fidelity

import (
	"testing"
)

func basicConfig() GraphConfig {
	return GraphConfig{
		KnownNodeIDs: []string{"NODE-A", "NODE-B", "NODE-C"},
		KnownAnchors: map[string]string{
			"section-1": "NODE-A",
			"section-2": "NODE-B",
		},
		KnownDocuments: []string{"DOC-EXTERNAL", "DOC-OTHER"},
	}
}

func TestBuildCitationGraphInternal(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "NODE-B", Label: "see B"},
	}
	g := BuildCitationGraph(links, basicConfig())

	if len(g.Citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(g.Citations))
	}
	if g.Citations[0].Kind != RefInternal {
		t.Fatalf("expected internal, got %s", g.Citations[0].Kind)
	}
	if g.Citations[0].Broken {
		t.Fatal("expected not broken")
	}
}

func TestBuildCitationGraphExternal(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "https://example.com/doc", Label: "external"},
	}
	g := BuildCitationGraph(links, basicConfig())

	if g.Citations[0].Kind != RefExternal {
		t.Fatalf("expected external, got %s", g.Citations[0].Kind)
	}
	if g.Citations[0].Broken {
		t.Fatal("external links should not be marked broken")
	}
}

func TestBuildCitationGraphAnchor(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "#section-1", Label: "anchor link"},
	}
	g := BuildCitationGraph(links, basicConfig())

	if g.Citations[0].Kind != RefAnchor {
		t.Fatalf("expected anchor, got %s", g.Citations[0].Kind)
	}
	if g.Citations[0].Broken {
		t.Fatal("known anchor should not be broken")
	}
}

func TestBuildCitationGraphBrokenAnchor(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "#nonexistent", Label: "bad anchor"},
	}
	g := BuildCitationGraph(links, basicConfig())

	if !g.Citations[0].Broken {
		t.Fatal("expected broken for unknown anchor")
	}
	if g.Citations[0].Kind != RefAnchor {
		t.Fatalf("expected anchor kind, got %s", g.Citations[0].Kind)
	}
}

func TestBuildCitationGraphBrokenNodeRef(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "NODE-MISSING", Label: "missing node"},
	}
	g := BuildCitationGraph(links, basicConfig())

	if !g.Citations[0].Broken {
		t.Fatal("expected broken for unknown node ref")
	}
	if g.Citations[0].Kind != RefInternal {
		t.Fatalf("expected internal kind, got %s", g.Citations[0].Kind)
	}
}

func TestBuildCitationGraphCrossDocument(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "DOC-EXTERNAL", Label: "cross-doc"},
	}
	g := BuildCitationGraph(links, basicConfig())

	if g.Citations[0].Kind != RefCrossDocument {
		t.Fatalf("expected cross_document, got %s", g.Citations[0].Kind)
	}
	if g.Citations[0].Broken {
		t.Fatal("known document should not be broken")
	}
}

func TestBuildCitationGraphBrokenCrossDoc(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "docs/unknown/path", Label: "bad path"},
	}
	g := BuildCitationGraph(links, basicConfig())

	if !g.Citations[0].Broken {
		t.Fatal("expected broken for unknown cross-doc ref")
	}
	if g.Citations[0].Kind != RefCrossDocument {
		t.Fatalf("expected cross_document kind, got %s", g.Citations[0].Kind)
	}
}

func TestBrokenRefs(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "NODE-B"},
		{SourceNodeID: "NODE-A", Target: "#nonexistent"},
		{SourceNodeID: "NODE-B", Target: "NODE-MISSING"},
		{SourceNodeID: "NODE-C", Target: "https://ok.com"},
	}
	g := BuildCitationGraph(links, basicConfig())

	broken := g.BrokenRefs()
	if len(broken) != 2 {
		t.Fatalf("expected 2 broken refs, got %d", len(broken))
	}
}

func TestOutgoingFrom(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "NODE-B"},
		{SourceNodeID: "NODE-A", Target: "NODE-C"},
		{SourceNodeID: "NODE-B", Target: "NODE-A"},
	}
	g := BuildCitationGraph(links, basicConfig())

	outgoing := g.OutgoingFrom("NODE-A")
	if len(outgoing) != 2 {
		t.Fatalf("expected 2 outgoing from NODE-A, got %d", len(outgoing))
	}
}

func TestIncomingTo(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "NODE-B"},
		{SourceNodeID: "NODE-C", Target: "NODE-B"},
	}
	g := BuildCitationGraph(links, basicConfig())

	incoming := g.IncomingTo("NODE-B")
	if len(incoming) != 2 {
		t.Fatalf("expected 2 incoming to NODE-B, got %d", len(incoming))
	}
}

func TestSummarize(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "NODE-B"},
		{SourceNodeID: "NODE-A", Target: "#section-1"},
		{SourceNodeID: "NODE-B", Target: "https://example.com"},
		{SourceNodeID: "NODE-B", Target: "DOC-EXTERNAL"},
		{SourceNodeID: "NODE-C", Target: "#broken-anchor"},
	}
	g := BuildCitationGraph(links, basicConfig())
	s := g.Summarize()

	if s.TotalCitations != 5 {
		t.Fatalf("expected 5 total, got %d", s.TotalCitations)
	}
	if s.InternalCount != 1 {
		t.Fatalf("expected 1 internal, got %d", s.InternalCount)
	}
	if s.AnchorCount != 2 {
		t.Fatalf("expected 2 anchor, got %d", s.AnchorCount)
	}
	if s.ExternalCount != 1 {
		t.Fatalf("expected 1 external, got %d", s.ExternalCount)
	}
	if s.CrossDocCount != 1 {
		t.Fatalf("expected 1 cross-doc, got %d", s.CrossDocCount)
	}
	if s.BrokenCount != 1 {
		t.Fatalf("expected 1 broken, got %d", s.BrokenCount)
	}
	if s.BySource["NODE-A"] != 2 {
		t.Fatalf("expected 2 from NODE-A, got %d", s.BySource["NODE-A"])
	}
}

func TestEmptyGraph(t *testing.T) {
	g := BuildCitationGraph(nil, GraphConfig{})

	if len(g.Citations) != 0 {
		t.Fatalf("expected 0 citations, got %d", len(g.Citations))
	}
	s := g.Summarize()
	if s.TotalCitations != 0 {
		t.Fatalf("expected 0 total, got %d", s.TotalCitations)
	}
	if s.BrokenCount != 0 {
		t.Fatalf("expected 0 broken, got %d", s.BrokenCount)
	}
}

func TestCitationLineNumber(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "NODE-B", Line: 42},
	}
	g := BuildCitationGraph(links, basicConfig())

	if g.Citations[0].Line != 42 {
		t.Fatalf("expected line 42, got %d", g.Citations[0].Line)
	}
}

func TestMultipleBrokenFromSameSource(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "#bad1"},
		{SourceNodeID: "NODE-A", Target: "#bad2"},
		{SourceNodeID: "NODE-A", Target: "NODE-B"},
	}
	g := BuildCitationGraph(links, basicConfig())

	broken := g.BrokenRefs()
	if len(broken) != 2 {
		t.Fatalf("expected 2 broken, got %d", len(broken))
	}
	outgoing := g.OutgoingFrom("NODE-A")
	if len(outgoing) != 3 {
		t.Fatalf("expected 3 outgoing, got %d", len(outgoing))
	}
}

func TestHttpsVsNodeRef(t *testing.T) {
	links := []LinkInput{
		{SourceNodeID: "NODE-A", Target: "http://example.com"},
		{SourceNodeID: "NODE-A", Target: "https://secure.com/path"},
	}
	g := BuildCitationGraph(links, basicConfig())

	for _, c := range g.Citations {
		if c.Kind != RefExternal {
			t.Fatalf("expected external for URL, got %s for %s", c.Kind, c.TargetID)
		}
	}
}

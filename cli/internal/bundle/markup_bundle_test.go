package bundle

import (
	"strings"
	"testing"
	"time"
)

// VRC-31 follow-through (#568): markup sources (XML/HTML) ride the bundle
// emitter — the full chain « connecteur → atomes ELI → bundle » closes. The
// ELI-shaped fixture follows the portable-corpus discipline (synthetic,
// license-safe, fake act number).
const eliXMLSource = `<?xml version="1.0" encoding="utf-8" ?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:jolux="http://data.legilux.public.lu/resource/ontology/jolux#">
  <rdf:Description rdf:about="https://fedlex.data.admin.ch/eli/cc/9999/0000_0000_0000">
    <jolux:title>Loi exemple sur les espaces construits</jolux:title>
    <jolux:dateDocument>2026-01-01</jolux:dateDocument>
  </rdf:Description>
  <rdf:Description rdf:about="https://fedlex.data.admin.ch/eli/cc/9999/0000_0000_0000/art_1">
    <jolux:title>Art. 1 Hauteurs et gabarits admissibles</jolux:title>
  </rdf:Description>
</rdf:RDF>`

func markupSources() []SourceFile {
	return []SourceFile{
		{RelPath: "fedlex/lat-entry.rdf.xml", Content: []byte(eliXMLSource)},
		{RelPath: "rules.md", Content: []byte("# Garanties\n\nToute réponse gouvernée doit citer la source applicable ou s'abstenir.\n")},
	}
}

func TestBuild_MarkupSourcesEmitELIAnchoredNodes(t *testing.T) {
	b, err := Build(BuildInput{
		BundleID:    "nomos-markup-bundle",
		Producer:    "nomos",
		Domain:      "built-environment",
		GeneratedAt: time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC),
		Sources:     markupSources(),
		Trace:       testTrace(t),
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// The emitted bundle passes the SAME in-engine gate as md-only bundles
	// (node invariants + facet vocabulary).
	if err := b.Validate(); err != nil {
		t.Fatalf("markup bundle invalid: %v", err)
	}

	var xmlNodes, mdNodes int
	for _, node := range b.Feeds[0].Nodes {
		if node.SourcePath == "fedlex/lat-entry.rdf.xml" {
			xmlNodes++
			// The ELI identity travels INSIDE the bundle via parent_chain.
			if len(node.ParentChain) != 1 || !strings.HasPrefix(node.ParentChain[0], "https://fedlex.data.admin.ch/eli/") {
				t.Fatalf("node %s lost the ELI identity: parent_chain=%v", node.NodeID, node.ParentChain)
			}
			if node.Span.StartLine <= 0 || node.Span.EndLine < node.Span.StartLine {
				t.Fatalf("node %s has a degenerate span: %+v", node.NodeID, node.Span)
			}
		}
		if node.SourcePath == "rules.md" {
			mdNodes++
		}
	}
	if xmlNodes != 3 {
		t.Fatalf("xml nodes = %d, want 3 (title + date + art title)", xmlNodes)
	}
	if mdNodes == 0 {
		t.Fatal("md nodes missing — the default path must stay untouched")
	}
	// rag_metadata stays fully joined (one entry per node, validated in-engine,
	// but assert the markup nodes are present there too).
	ragByNode := map[string]bool{}
	for _, r := range b.RAGMetadata {
		ragByNode[r.NodeID] = true
	}
	for _, node := range b.Feeds[0].Nodes {
		if !ragByNode[node.NodeID] {
			t.Fatalf("node %s missing from rag_metadata", node.NodeID)
		}
	}
}

func TestBuild_MalformedMarkupFailsTheWholeBuild(t *testing.T) {
	_, err := Build(BuildInput{
		BundleID:    "nomos-bad-markup",
		Producer:    "nomos",
		GeneratedAt: time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC),
		Sources: []SourceFile{
			{RelPath: "bad.xml", Content: []byte("<root><unclosed>")},
		},
		Trace: testTrace(t),
	})
	if err == nil {
		t.Fatal("a malformed markup source must fail the build — a bundle never silently drops a source")
	}
	if !strings.Contains(err.Error(), "bad.xml") {
		t.Fatalf("error does not name the offending source: %v", err)
	}
}

func TestBuild_MarkupBundleIsDeterministic(t *testing.T) {
	build := func() Bundle {
		b, err := Build(BuildInput{
			BundleID:    "nomos-markup-bundle",
			Producer:    "nomos",
			Domain:      "built-environment",
			GeneratedAt: time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC),
			Sources:     markupSources(),
			Trace:       testTrace(t),
		})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		return b
	}
	a, b := build(), build()
	aj, _ := a.Marshal()
	bj, _ := b.Marshal()
	if string(aj) != string(bj) {
		t.Fatal("two runs over identical markup input are not byte-identical")
	}
}

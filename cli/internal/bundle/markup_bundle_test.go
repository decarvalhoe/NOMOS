package bundle

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

// buildBundleTestPDF mirrors the fidelity/atomization test builder (fixtures
// don't cross package boundaries): minimal uncompressed PDF, exact xref.
func buildBundleTestPDF(pages [][]string) []byte {
	var buf bytes.Buffer
	offsets := []int{0}
	writeObj := func(body string) {
		offsets = append(offsets, buf.Len())
		buf.WriteString(body)
	}
	buf.WriteString("%PDF-1.4\n")
	kids := make([]string, len(pages))
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", 4+2*i)
	}
	writeObj("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	writeObj(fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", strings.Join(kids, " "), len(pages)))
	writeObj("3 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")
	for i, lines := range pages {
		pageID, contentID := 4+2*i, 5+2*i
		var content string
		if len(lines) == 0 {
			content = "q 1 0 0 1 0 0 cm Q"
		} else {
			var ops []string
			ops = append(ops, "BT /F1 12 Tf 72 720 Td")
			for j, line := range lines {
				if j > 0 {
					ops = append(ops, "0 -20 Td")
				}
				ops = append(ops, "("+line+") Tj")
			}
			ops = append(ops, "ET")
			content = strings.Join(ops, " ")
		}
		writeObj(fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>\nendobj\n", pageID, contentID))
		writeObj(fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", contentID, len(content), content))
	}
	xrefOff := buf.Len()
	buf.WriteString(fmt.Sprintf("xref\n0 %d\n", len(offsets)))
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets[1:] {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}
	buf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(offsets), xrefOff))
	return buf.Bytes()
}

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

// W23-1 (#590): a clean born-digital PDF rides the bundle; its structural
// locator (the canonical ref with the page/line path) travels in parent_chain.
func TestBuild_PDFSourceEmitsNodesWithPageLocators(t *testing.T) {
	b, err := Build(BuildInput{
		BundleID:    "nomos-pdf-bundle",
		Producer:    "nomos",
		Domain:      "built-environment",
		GeneratedAt: time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC),
		Sources: []SourceFile{
			{RelPath: "commune/rcc.pdf", Content: buildBundleTestPDF([][]string{{"Art. 1 Hauteur maximale 9 m", "Zone village"}})},
		},
		Trace: testTrace(t),
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("pdf bundle invalid: %v", err)
	}
	nodes := b.Feeds[0].Nodes
	if len(nodes) != 2 {
		t.Fatalf("nodes = %d, want 2", len(nodes))
	}
	if len(nodes[0].ParentChain) != 1 || !strings.Contains(nodes[0].ParentChain[0], "#/page[1]/line[1]") {
		t.Fatalf("pdf locator lost: parent_chain=%v", nodes[0].ParentChain)
	}
	if nodes[0].Span.StartLine != 1 || nodes[1].Span.StartLine != 2 {
		t.Fatalf("line spans wrong: %+v / %+v", nodes[0].Span, nodes[1].Span)
	}
}

// W23-1 (#590): a PDF with a page outside the born-digital-text claim is
// REFUSED, naming the pages — a bundle never silently drops content.
func TestBuild_PDFWithScannedPageIsRefused(t *testing.T) {
	_, err := Build(BuildInput{
		BundleID:    "nomos-pdf-scanned",
		Producer:    "nomos",
		GeneratedAt: time.Date(2026, 6, 11, 0, 0, 0, 0, time.UTC),
		Sources: []SourceFile{
			{RelPath: "commune/scan.pdf", Content: buildBundleTestPDF([][]string{{"Art. 1"}, {}})},
		},
		Trace: testTrace(t),
	})
	if err == nil {
		t.Fatal("a partially scanned PDF entered a bundle")
	}
	if !strings.Contains(err.Error(), "[2]") || !strings.Contains(err.Error(), "scan.pdf") {
		t.Fatalf("refusal must name the source and the pages: %v", err)
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

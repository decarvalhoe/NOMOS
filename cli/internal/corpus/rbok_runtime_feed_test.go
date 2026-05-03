package corpus

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

var rtTestTime = time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)

func makeLayerFeed(docID string, nodes []LawbookNode) LawbookFeed {
	return LawbookFeed{
		SchemaVersion: "0.1.0",
		FeedID:        strings.ToLower(docID) + "-feed",
		DocumentID:    docID,
		Domain:        "insurance",
		GeneratedAt:   "2026-05-03T12:00:00Z",
		SourcePath:    "sources/" + strings.ToLower(docID) + ".md",
		SourceHash:    "sha256:aaaa",
		NodeCount:     len(nodes),
		Nodes:         nodes,
	}
}

func node(id, ref, docID, domain string, nodeType LawbookNodeType) LawbookNode {
	return LawbookNode{
		NodeID: id, DocumentID: docID, NodeType: nodeType,
		CanonicalRef: ref, DisplayRef: string(nodeType) + ": " + id,
		Depth: nodeType.Depth(), OrdinalPath: "1",
		SourcePath: "sources/" + strings.ToLower(docID) + ".md",
		SourceHash: "sha256:bbbb", Status: StatusActive,
		Priority: PriorityMedium, Domain: domain, Title: id,
	}
}

func testLayers() ([]LayerInput, map[string]LawbookFeed) {
	layers := []LayerInput{
		{ID: "referentiel", Kind: LayerReferentiel, Path: "01_referentiel/", Domain: "insurance", Priority: 1},
		{ID: "domaine-habitation", Kind: LayerDomaine, Path: "02_domaines/habitation/", Domain: "insurance-home", Priority: 2},
		{ID: "meta", Kind: LayerMeta, Path: "00_meta/", Domain: "meta", Priority: 3},
	}

	feeds := map[string]LawbookFeed{
		"referentiel": makeLayerFeed("DOC-REF", []LawbookNode{
			node("REF-ART-1", "ref/article/garantie-eau", "DOC-REF", "insurance", NodeArticle),
			node("REF-ART-2", "ref/article/exclusion-toit", "DOC-REF", "insurance", NodeArticle),
		}),
		"domaine-habitation": makeLayerFeed("DOC-HAB", []LawbookNode{
			node("HAB-ART-1", "hab/article/franchise", "DOC-HAB", "insurance-home", NodeArticle),
			node("HAB-ART-2", "hab/article/plafond", "DOC-HAB", "insurance-home", NodeArticle),
		}),
		"meta": makeLayerFeed("DOC-META", []LawbookNode{
			node("META-GLOSS", "meta/glossary", "DOC-META", "meta", NodeDocument),
		}),
	}

	return layers, feeds
}

func TestAssembleRuntimeFeedBasic(t *testing.T) {
	layers, feeds := testLayers()
	feed := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})

	if feed.Format != RuntimeFeedFormat {
		t.Fatalf("format: %q", feed.Format)
	}
	if feed.LayerCount != 3 {
		t.Fatalf("expected 3 layers, got %d", feed.LayerCount)
	}
	if feed.NodeCount != 5 {
		t.Fatalf("expected 5 nodes, got %d", feed.NodeCount)
	}
	if feed.GeneratedAt != "2026-05-03T12:00:00Z" {
		t.Fatalf("timestamp: %q", feed.GeneratedAt)
	}
}

func TestAssembleRuntimeFeedProvenance(t *testing.T) {
	layers, feeds := testLayers()
	feed := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})

	if len(feed.Layers) != 3 {
		t.Fatalf("expected 3 layer provenances, got %d", len(feed.Layers))
	}

	// Check provenance order matches priority (sorted).
	if feed.Layers[0].ID != "referentiel" {
		t.Fatalf("first layer should be referentiel, got %q", feed.Layers[0].ID)
	}
	if feed.Layers[0].NodeCount != 2 {
		t.Fatalf("referentiel should have 2 nodes, got %d", feed.Layers[0].NodeCount)
	}
}

func TestAssembleRuntimeFeedNodeLayerIDs(t *testing.T) {
	layers, feeds := testLayers()
	feed := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})

	for _, n := range feed.Nodes {
		if n.LayerID == "" {
			t.Fatalf("node %s has empty layer_id", n.NodeID)
		}
		if n.LayerKind == "" {
			t.Fatalf("node %s has empty layer_kind", n.NodeID)
		}
	}
}

func TestAssembleRuntimeFeedConflictResolution(t *testing.T) {
	layers := []LayerInput{
		{ID: "primary", Kind: LayerReferentiel, Path: "a/", Domain: "ins", Priority: 1},
		{ID: "override", Kind: LayerOverride, Path: "b/", Domain: "ins", Priority: 0},
	}

	sharedRef := "shared/article/x"
	feeds := map[string]LawbookFeed{
		"primary":  makeLayerFeed("DOC-A", []LawbookNode{node("A-1", sharedRef, "DOC-A", "ins", NodeArticle)}),
		"override": makeLayerFeed("DOC-B", []LawbookNode{node("B-1", sharedRef, "DOC-B", "ins", NodeArticle)}),
	}

	feed := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})

	if len(feed.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(feed.Conflicts))
	}
	c := feed.Conflicts[0]
	if c.CanonicalRef != sharedRef {
		t.Fatalf("conflict ref: %q", c.CanonicalRef)
	}
	if c.WinnerLayer != "override" {
		t.Fatalf("override (priority 0) should win, got %q", c.WinnerLayer)
	}
	if c.Resolution != "priority" {
		t.Fatalf("resolution: %q", c.Resolution)
	}

	// Only 1 node should remain for the shared ref.
	if feed.NodeCount != 1 {
		t.Fatalf("expected 1 merged node, got %d", feed.NodeCount)
	}
}

func TestAssembleRuntimeFeedNoConflicts(t *testing.T) {
	layers, feeds := testLayers()
	feed := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})

	if len(feed.Conflicts) != 0 {
		t.Fatalf("expected no conflicts, got %d", len(feed.Conflicts))
	}
}

func TestAssembleRuntimeFeedContentHash(t *testing.T) {
	layers, feeds := testLayers()
	f1 := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})
	f2 := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})

	if !strings.HasPrefix(f1.ContentHash, "sha256:") {
		t.Fatalf("invalid hash: %q", f1.ContentHash)
	}
	if f1.ContentHash != f2.ContentHash {
		t.Fatal("hash should be deterministic")
	}
}

func TestAssembleRuntimeFeedDifferentTimesDifferentHash(t *testing.T) {
	layers, feeds := testLayers()
	f1 := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})
	f2 := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime.Add(time.Hour)})

	if f1.ContentHash == f2.ContentHash {
		t.Fatal("different timestamps should produce different hashes")
	}
}

func TestAssembleRuntimeFeedMissingLayer(t *testing.T) {
	layers := []LayerInput{
		{ID: "exists", Kind: LayerReferentiel, Path: "a/", Domain: "ins", Priority: 1},
		{ID: "missing", Kind: LayerDomaine, Path: "b/", Domain: "ins", Priority: 2},
	}
	feeds := map[string]LawbookFeed{
		"exists": makeLayerFeed("DOC-A", []LawbookNode{node("A-1", "a/art/1", "DOC-A", "ins", NodeArticle)}),
	}

	feed := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})

	if feed.LayerCount != 1 {
		t.Fatalf("expected 1 layer (missing skipped), got %d", feed.LayerCount)
	}
	if feed.NodeCount != 1 {
		t.Fatalf("expected 1 node, got %d", feed.NodeCount)
	}
}

func TestAssembleRuntimeFeedEmpty(t *testing.T) {
	feed := AssembleRuntimeFeed(nil, nil, RuntimeFeedOptions{Now: rtTestTime})

	if feed.LayerCount != 0 {
		t.Fatalf("expected 0 layers, got %d", feed.LayerCount)
	}
	if feed.NodeCount != 0 {
		t.Fatalf("expected 0 nodes, got %d", feed.NodeCount)
	}
}

func TestAssembleRuntimeFeedNodeOrder(t *testing.T) {
	layers, feeds := testLayers()
	feed := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})

	// Nodes should be sorted by canonical_ref.
	for i := 1; i < len(feed.Nodes); i++ {
		if feed.Nodes[i].CanonicalRef < feed.Nodes[i-1].CanonicalRef {
			t.Fatalf("nodes not sorted: %q before %q",
				feed.Nodes[i-1].CanonicalRef, feed.Nodes[i].CanonicalRef)
		}
	}
}

func TestWriteRuntimeFeedJSON(t *testing.T) {
	layers, feeds := testLayers()
	feed := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})

	var buf bytes.Buffer
	if err := WriteRuntimeFeed(&buf, feed); err != nil {
		t.Fatalf("write: %v", err)
	}

	var decoded RuntimeFeed
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Format != RuntimeFeedFormat {
		t.Fatalf("format: %q", decoded.Format)
	}
	if decoded.NodeCount != feed.NodeCount {
		t.Fatalf("node count: %d vs %d", decoded.NodeCount, feed.NodeCount)
	}
}

func TestRuntimeFeedSummarize(t *testing.T) {
	layers, feeds := testLayers()
	feed := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})

	summary := feed.Summarize()
	if summary.LayerCount != 3 {
		t.Fatalf("layer count: %d", summary.LayerCount)
	}
	if summary.NodeCount != 5 {
		t.Fatalf("node count: %d", summary.NodeCount)
	}
	if summary.ConflictCount != 0 {
		t.Fatalf("conflict count: %d", summary.ConflictCount)
	}
	if summary.ByLayer["referentiel"] != 2 {
		t.Fatalf("referentiel count: %d", summary.ByLayer["referentiel"])
	}
	if summary.ByDomain["insurance-home"] != 2 {
		t.Fatalf("insurance-home count: %d", summary.ByDomain["insurance-home"])
	}
}

func TestRuntimeFeedSummaryFormat(t *testing.T) {
	layers, feeds := testLayers()
	feed := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})

	s := feed.Summarize().FormatSummary()
	if !strings.Contains(s, "3 layers") {
		t.Fatalf("summary: %q", s)
	}
	if !strings.Contains(s, "5 nodes") {
		t.Fatalf("summary: %q", s)
	}
}

func TestAssembleRuntimeFeedMultiConflict(t *testing.T) {
	layers := []LayerInput{
		{ID: "l1", Kind: LayerReferentiel, Path: "a/", Domain: "ins", Priority: 1},
		{ID: "l2", Kind: LayerDomaine, Path: "b/", Domain: "ins", Priority: 2},
		{ID: "l3", Kind: LayerOverride, Path: "c/", Domain: "ins", Priority: 0},
	}
	ref := "shared/x"
	feeds := map[string]LawbookFeed{
		"l1": makeLayerFeed("D1", []LawbookNode{node("N1", ref, "D1", "ins", NodeArticle)}),
		"l2": makeLayerFeed("D2", []LawbookNode{node("N2", ref, "D2", "ins", NodeArticle)}),
		"l3": makeLayerFeed("D3", []LawbookNode{node("N3", ref, "D3", "ins", NodeArticle)}),
	}

	feed := AssembleRuntimeFeed(layers, feeds, RuntimeFeedOptions{Now: rtTestTime})

	if len(feed.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d", len(feed.Conflicts))
	}
	if len(feed.Conflicts[0].LayerIDs) != 3 {
		t.Fatalf("expected 3 layers in conflict, got %d", len(feed.Conflicts[0].LayerIDs))
	}
	if feed.Conflicts[0].WinnerLayer != "l3" {
		t.Fatalf("l3 (priority 0) should win, got %q", feed.Conflicts[0].WinnerLayer)
	}
	if feed.NodeCount != 1 {
		t.Fatalf("only winner should remain, got %d nodes", feed.NodeCount)
	}
}

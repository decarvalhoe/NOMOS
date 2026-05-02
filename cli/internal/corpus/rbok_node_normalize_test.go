package corpus

import (
	"os"
	"path/filepath"
	"testing"
)

var testDefaults = NodeDefaults{
	DocumentID: "DOC-TEST",
	SourcePath: "sources/test.md",
	SourceHash: "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	Domain:     "test-domain",
	Status:     StatusActive,
	Priority:   PriorityMedium,
}

func TestNormalizeNodeFillsDefaults(t *testing.T) {
	node := LawbookNode{
		NodeID:       "ART-001",
		NodeType:     NodeArticle,
		CanonicalRef: "test/article/one",
		DisplayRef:   "article: One",
	}

	errs := NormalizeNode(&node, testDefaults, "1.1.1.1.1")
	if len(errs) != 0 {
		t.Fatalf("expected valid after normalize, got: %v", errs)
	}

	if node.DocumentID != "DOC-TEST" {
		t.Fatalf("expected DocumentID DOC-TEST, got %q", node.DocumentID)
	}
	if node.SourcePath != "sources/test.md" {
		t.Fatalf("expected SourcePath, got %q", node.SourcePath)
	}
	if node.SourceHash != testDefaults.SourceHash {
		t.Fatalf("expected SourceHash, got %q", node.SourceHash)
	}
	if node.Domain != "test-domain" {
		t.Fatalf("expected Domain, got %q", node.Domain)
	}
	if node.Status != StatusActive {
		t.Fatalf("expected Status active, got %q", node.Status)
	}
	if node.Priority != PriorityMedium {
		t.Fatalf("expected Priority medium, got %q", node.Priority)
	}
	if node.OrdinalPath != "1.1.1.1.1" {
		t.Fatalf("expected OrdinalPath, got %q", node.OrdinalPath)
	}
}

func TestNormalizeNodeFixesDepth(t *testing.T) {
	node := LawbookNode{
		NodeID:       "DOC-001",
		NodeType:     NodeDocument,
		Depth:        99, // wrong
		CanonicalRef: "test/doc",
		DisplayRef:   "document: test",
	}

	NormalizeNode(&node, testDefaults, "1")

	if node.Depth != 0 {
		t.Fatalf("expected depth 0 for document, got %d", node.Depth)
	}
}

func TestNormalizeNodePreservesExisting(t *testing.T) {
	node := LawbookNode{
		NodeID:       "ART-001",
		DocumentID:   "DOC-EXISTING",
		NodeType:     NodeArticle,
		CanonicalRef: "test/article/one",
		DisplayRef:   "article: One",
		SourcePath:   "existing.md",
		SourceHash:   "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		Domain:       "existing-domain",
		Status:       StatusAmended,
		Priority:     PriorityCritical,
		OrdinalPath:  "2.3",
	}

	NormalizeNode(&node, testDefaults, "1.1")

	// Existing values should not be overwritten.
	if node.DocumentID != "DOC-EXISTING" {
		t.Fatalf("should preserve DocumentID, got %q", node.DocumentID)
	}
	if node.Domain != "existing-domain" {
		t.Fatalf("should preserve Domain, got %q", node.Domain)
	}
	if node.Status != StatusAmended {
		t.Fatalf("should preserve Status, got %q", node.Status)
	}
	if node.OrdinalPath != "2.3" {
		t.Fatalf("should preserve OrdinalPath, got %q", node.OrdinalPath)
	}
}

func TestNormalizeExtractResult(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "lawbook-sample.md"))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	result := ExtractMarkdown(string(src), "rga-2026")
	if len(result.Nodes) == 0 {
		t.Fatal("expected nodes")
	}

	errCount := NormalizeExtractResult(&result, testDefaults)

	// All nodes should now have required fields.
	for i, node := range result.Nodes {
		if node.DocumentID == "" {
			t.Fatalf("node[%d] %s: DocumentID still empty", i, node.NodeID)
		}
		if node.Domain == "" {
			t.Fatalf("node[%d] %s: Domain still empty", i, node.NodeID)
		}
		if !node.Status.IsValid() {
			t.Fatalf("node[%d] %s: invalid Status %q", i, node.NodeID, node.Status)
		}
		if !node.Priority.IsValid() {
			t.Fatalf("node[%d] %s: invalid Priority %q", i, node.NodeID, node.Priority)
		}
		if node.SourceHash == "" {
			t.Fatalf("node[%d] %s: SourceHash still empty", i, node.NodeID)
		}
		if node.OrdinalPath == "" {
			t.Fatalf("node[%d] %s: OrdinalPath still empty", i, node.NodeID)
		}
	}

	// Depth should match node type.
	for _, node := range result.Nodes {
		if node.NodeType.IsValid() && node.Depth != node.NodeType.Depth() {
			t.Fatalf("node %s: depth %d != type %s depth %d",
				node.NodeID, node.Depth, node.NodeType, node.NodeType.Depth())
		}
	}

	_ = errCount
}

func TestNormalizeExtractResultThenValidate(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "lawbook-sample.md"))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	result := ExtractMarkdown(string(src), "rga-2026")
	NormalizeExtractResult(&result, testDefaults)

	// Every node should now pass ValidateNode.
	for i, node := range result.Nodes {
		errs := ValidateNode(node)
		if len(errs) > 0 {
			t.Fatalf("node[%d] %s validation failed: %v", i, node.NodeID, errs)
		}
	}
}

func TestBuildNormalizedFeed(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "lawbook-sample.md"))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	result := ExtractMarkdown(string(src), "rga-2026")
	NormalizeExtractResult(&result, testDefaults)

	feed := BuildNormalizedFeed(result, "rga-2026-feed", testDefaults, "2026-05-02T10:00:00Z")

	feedErrs := ValidateFeed(feed)
	if len(feedErrs) > 0 {
		t.Fatalf("feed validation failed: %v", feedErrs)
	}

	if feed.FeedID != "rga-2026-feed" {
		t.Fatalf("expected feed_id rga-2026-feed, got %q", feed.FeedID)
	}
	if feed.DocumentID != "DOC-TEST" {
		t.Fatalf("expected DocumentID DOC-TEST, got %q", feed.DocumentID)
	}
	if feed.NodeCount != len(feed.Nodes) {
		t.Fatalf("node_count %d != len(nodes) %d", feed.NodeCount, len(feed.Nodes))
	}
}

func TestNormalizedFeedAssembly(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "lawbook-sample.md"))
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}

	result := ExtractMarkdown(string(src), "rga-2026")
	NormalizeExtractResult(&result, testDefaults)
	feed := BuildNormalizedFeed(result, "rga-2026-feed", testDefaults, "2026-05-02T10:00:00Z")

	// Assembly should work without errors.
	assembly := AssembleFeed(feed, AssembleOptions{Now: assemblyNow})

	if assembly.Format != FeedFormatVersion {
		t.Fatalf("expected format %s, got %s", FeedFormatVersion, assembly.Format)
	}
	if len(assembly.Index.NodesByID) != len(feed.Nodes) {
		t.Fatalf("index should have all %d nodes, got %d",
			len(feed.Nodes), len(assembly.Index.NodesByID))
	}
	if assembly.Governance.TotalNodes != len(feed.Nodes) {
		t.Fatalf("governance total %d != feed nodes %d",
			assembly.Governance.TotalNodes, len(feed.Nodes))
	}
	if len(assembly.EngineImport.Nodes) != len(feed.Nodes) {
		t.Fatalf("engine import should have all %d nodes", len(feed.Nodes))
	}
}

func TestExtractorDepthMatchesNodeType(t *testing.T) {
	src := "# Document Title\n\n## Chapter\n\n### Section\n\n#### Article\n"
	result := ExtractMarkdown(src, "test")

	expected := map[LawbookNodeType]int{
		NodeDocument: 0,
		NodeChapter:  1,
		NodeSection:  2,
		NodeArticle:  4,
	}

	for _, node := range result.Nodes {
		want, ok := expected[node.NodeType]
		if !ok {
			continue
		}
		if node.Depth != want {
			t.Fatalf("node type %s: expected depth %d, got %d",
				node.NodeType, want, node.Depth)
		}
	}
}

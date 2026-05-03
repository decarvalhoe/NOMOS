package corpus

import (
	"testing"
	"time"
)

var ragTestTime = time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)

func runtimeTestFeed() LawbookFeed {
	return LawbookFeed{
		SchemaVersion: "0.1.0",
		FeedID:        "feed-test-001",
		DocumentID:    "DOC-001",
		Domain:        "insurance",
		SourcePath:    "corpus/rules.md",
		SourceHash:    "sha256:feedhash",
		NodeCount:     5,
		Nodes: []LawbookNode{
			{NodeID: "N-001", DocumentID: "DOC-001", NodeType: NodeDocument, CanonicalRef: "doc/root", DisplayRef: "document: Rules", Depth: 0, Status: StatusActive, Priority: PriorityHigh, Domain: "insurance", Title: "Rules", Text: "Document root text.", SourceHash: "sha256:a1"},
			{NodeID: "N-002", DocumentID: "DOC-001", NodeType: NodeChapter, CanonicalRef: "doc/ch1", DisplayRef: "chapter: Ch1", Depth: 1, Status: StatusActive, Priority: PriorityHigh, Domain: "insurance", Title: "Chapter 1", Text: "Chapter content.", ParentID: "N-001", SourceHash: "sha256:a2"},
			{NodeID: "N-003", DocumentID: "DOC-001", NodeType: NodeArticle, CanonicalRef: "doc/art1", DisplayRef: "article: Art1", Depth: 4, Status: StatusActive, Priority: PriorityCritical, Domain: "insurance", Title: "Article 1", Text: "Article text with rules.", ParentID: "N-002", SourceHash: "sha256:a3"},
			{NodeID: "N-004", DocumentID: "DOC-001", NodeType: NodeParagraph, CanonicalRef: "doc/p1", DisplayRef: "paragraph 1", Depth: 5, Status: StatusActive, Priority: PriorityMedium, Domain: "insurance", Text: "Paragraph detail.", ParentID: "N-003", SourceHash: "sha256:a4"},
			{NodeID: "N-005", DocumentID: "DOC-001", NodeType: NodeAlinea, CanonicalRef: "doc/al1", DisplayRef: "alinea 1", Depth: 6, Status: StatusAmended, Priority: PriorityLow, Domain: "insurance", Text: "List item detail.", ParentID: "N-004", SourceHash: "sha256:a5"},
		},
	}
}

func runtimeRAGTestConfig() RuntimeRAGConfig {
	return RuntimeRAGConfig{
		Domain: "insurance",
		Layer:  "lawbook",
		Now:    ragTestTime,
	}
}

func TestGenerateRuntimeRAG_ProducesChunks(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())
	if result.TotalChunks != 5 {
		t.Fatalf("expected 5 chunks, got %d", result.TotalChunks)
	}
	if result.Domain != "insurance" {
		t.Fatalf("expected domain insurance, got %q", result.Domain)
	}
	if result.SchemaVersion != "0.1.0" {
		t.Fatalf("expected schema 0.1.0, got %q", result.SchemaVersion)
	}
}

func TestGenerateRuntimeRAG_SkipsEmptyText(t *testing.T) {
	feed := runtimeTestFeed()
	feed.Nodes = append(feed.Nodes, LawbookNode{
		NodeID: "N-006", NodeType: NodeParagraph, Text: "", SourceHash: "sha256:a6",
	})
	result := GenerateRuntimeRAG(feed, runtimeRAGTestConfig())
	if result.TotalChunks != 5 {
		t.Fatalf("expected 5 chunks (empty skipped), got %d", result.TotalChunks)
	}
}

func TestGenerateRuntimeRAG_AuthorityLevels(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())

	expected := map[string]AuthorityLevel{
		"N-001": AuthorityAuthoritative, // document
		"N-002": AuthorityAuthoritative, // chapter
		"N-003": AuthorityAuthoritative, // article, critical priority
		"N-004": AuthorityReference,     // paragraph
		"N-005": AuthorityDerived,       // alinea
	}
	for _, chunk := range result.Chunks {
		want, ok := expected[chunk.NodeID]
		if !ok {
			continue
		}
		if chunk.AuthorityLevel != want {
			t.Fatalf("node %s: expected authority %s, got %s", chunk.NodeID, want, chunk.AuthorityLevel)
		}
	}
}

func TestGenerateRuntimeRAG_ConfidenceLevels(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())

	for _, chunk := range result.Chunks {
		if chunk.Confidence == "" {
			t.Fatalf("chunk %s has empty confidence", chunk.NodeID)
		}
		if chunk.Confidence != "high" && chunk.Confidence != "medium" && chunk.Confidence != "low" {
			t.Fatalf("chunk %s has invalid confidence %q", chunk.NodeID, chunk.Confidence)
		}
	}

	// Amended alinea should have medium confidence.
	for _, chunk := range result.Chunks {
		if chunk.NodeID == "N-005" && chunk.Confidence != "medium" {
			t.Fatalf("amended alinea: expected medium confidence, got %q", chunk.Confidence)
		}
	}
}

func TestGenerateRuntimeRAG_ProvenanceChain(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())

	// Article N-003 should have provenance: feed -> document -> chapter(N-002 via parent) -> self
	var art *RuntimeRAGChunk
	for i := range result.Chunks {
		if result.Chunks[i].NodeID == "N-003" {
			art = &result.Chunks[i]
			break
		}
	}
	if art == nil {
		t.Fatal("article chunk not found")
	}
	if len(art.ProvenanceChain) < 3 {
		t.Fatalf("expected at least 3 provenance links, got %d", len(art.ProvenanceChain))
	}
	// First link should be the feed layer.
	if art.ProvenanceChain[0].Layer != "lawbook" {
		t.Fatalf("expected first provenance layer 'lawbook', got %q", art.ProvenanceChain[0].Layer)
	}
	// Last link should be the node itself.
	last := art.ProvenanceChain[len(art.ProvenanceChain)-1]
	if last.NodeID != "N-003" {
		t.Fatalf("expected last provenance node N-003, got %q", last.NodeID)
	}
}

func TestGenerateRuntimeRAG_ParentChain(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())

	// Alinea N-005 parent chain: N-001 -> N-002 -> N-003 -> N-004
	var alinea *RuntimeRAGChunk
	for i := range result.Chunks {
		if result.Chunks[i].NodeID == "N-005" {
			alinea = &result.Chunks[i]
			break
		}
	}
	if alinea == nil {
		t.Fatal("alinea chunk not found")
	}
	if len(alinea.ParentChain) != 4 {
		t.Fatalf("expected 4 parents, got %d: %v", len(alinea.ParentChain), alinea.ParentChain)
	}
	if alinea.ParentChain[0] != "N-001" {
		t.Fatalf("expected root parent N-001, got %q", alinea.ParentChain[0])
	}
}

func TestGenerateRuntimeRAG_ChunkIDStable(t *testing.T) {
	r1 := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())
	r2 := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())

	for i := range r1.Chunks {
		if r1.Chunks[i].ChunkID != r2.Chunks[i].ChunkID {
			t.Fatalf("chunk ID unstable: %q vs %q", r1.Chunks[i].ChunkID, r2.Chunks[i].ChunkID)
		}
	}
}

func TestGenerateRuntimeRAG_ChunkIDUnique(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())
	seen := map[string]bool{}
	for _, c := range result.Chunks {
		if seen[c.ChunkID] {
			t.Fatalf("duplicate chunk ID: %s", c.ChunkID)
		}
		seen[c.ChunkID] = true
	}
}

func TestGenerateRuntimeRAG_TokenCount(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())
	for _, c := range result.Chunks {
		if c.TokenCount <= 0 {
			t.Fatalf("chunk %s has non-positive token count %d", c.NodeID, c.TokenCount)
		}
		if c.CharCount <= 0 {
			t.Fatalf("chunk %s has non-positive char count %d", c.NodeID, c.CharCount)
		}
	}
}

func TestGenerateRuntimeRAG_ByAuthorityCounts(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())
	total := 0
	for _, count := range result.ByAuthority {
		total += count
	}
	if total != result.TotalChunks {
		t.Fatalf("by_authority sum %d != total_chunks %d", total, result.TotalChunks)
	}
}

func TestGenerateRuntimeRAG_ByConfidenceCounts(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())
	total := 0
	for _, count := range result.ByConfidence {
		total += count
	}
	if total != result.TotalChunks {
		t.Fatalf("by_confidence sum %d != total_chunks %d", total, result.TotalChunks)
	}
}

func TestGenerateRuntimeRAG_RepealedLowConfidence(t *testing.T) {
	feed := LawbookFeed{
		FeedID: "f", DocumentID: "D", Domain: "d", SourceHash: "sha256:x",
		Nodes: []LawbookNode{
			{NodeID: "N-R", NodeType: NodeArticle, Text: "Repealed rule.", Status: StatusRepealed, Priority: PriorityMedium, SourceHash: "sha256:r"},
		},
	}
	result := GenerateRuntimeRAG(feed, runtimeRAGTestConfig())
	if result.Chunks[0].Confidence != "low" {
		t.Fatalf("expected low confidence for repealed, got %q", result.Chunks[0].Confidence)
	}
}

func TestGenerateRuntimeRAG_DraftLowConfidence(t *testing.T) {
	feed := LawbookFeed{
		FeedID: "f", DocumentID: "D", Domain: "d", SourceHash: "sha256:x",
		Nodes: []LawbookNode{
			{NodeID: "N-D", NodeType: NodeArticle, Text: "Draft content.", Status: StatusNodeDraft, Priority: PriorityMedium, SourceHash: "sha256:d"},
		},
	}
	result := GenerateRuntimeRAG(feed, runtimeRAGTestConfig())
	if result.Chunks[0].Confidence != "low" {
		t.Fatalf("expected low confidence for draft, got %q", result.Chunks[0].Confidence)
	}
}

// --- Filters ---

func TestFilterByAuthority(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())
	auth := FilterByAuthority(result.Chunks, AuthorityAuthoritative)
	if len(auth) == 0 {
		t.Fatal("expected authoritative chunks")
	}
	for _, c := range auth {
		if c.AuthorityLevel != AuthorityAuthoritative {
			t.Fatalf("expected authoritative, got %s", c.AuthorityLevel)
		}
	}
}

func TestFilterByMinConfidence(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())
	high := FilterByMinConfidence(result.Chunks, "high")
	for _, c := range high {
		if c.Confidence != "high" {
			t.Fatalf("expected high confidence, got %q", c.Confidence)
		}
	}
}

func TestFilterByMinConfidence_Medium(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())
	medUp := FilterByMinConfidence(result.Chunks, "medium")
	for _, c := range medUp {
		if c.Confidence != "high" && c.Confidence != "medium" {
			t.Fatalf("expected medium+, got %q", c.Confidence)
		}
	}
}

// --- ContextWindow ---

func TestContextWindow(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())
	text, sources := ContextWindow(result.Chunks, 1000)
	if text == "" {
		t.Fatal("expected non-empty context text")
	}
	if len(sources) == 0 {
		t.Fatal("expected sources")
	}
}

func TestContextWindow_BudgetLimit(t *testing.T) {
	result := GenerateRuntimeRAG(runtimeTestFeed(), runtimeRAGTestConfig())
	_, sources := ContextWindow(result.Chunks, 5)
	// With only 5 tokens budget, should include at most 1 chunk.
	if len(sources) > 1 {
		t.Fatalf("expected at most 1 source with tiny budget, got %d", len(sources))
	}
}

func TestContextWindow_Empty(t *testing.T) {
	text, sources := ContextWindow(nil, 1000)
	if text != "" {
		t.Fatal("expected empty text for nil chunks")
	}
	if len(sources) != 0 {
		t.Fatal("expected no sources for nil chunks")
	}
}

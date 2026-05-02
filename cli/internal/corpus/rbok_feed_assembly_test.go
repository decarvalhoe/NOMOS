package corpus

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

var assemblyNow = time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)

func testFeed() LawbookFeed {
	return LawbookFeed{
		SchemaVersion: "0.1.0",
		FeedID:        "civil-code-feed",
		DocumentID:    "DOC-CIVIL-CODE",
		Domain:        "civil-law",
		GeneratedAt:   "2026-05-02T10:00:00Z",
		SourcePath:    "sources/civil-code.pdf",
		SourceHash:    "sha256:aaaa",
		NodeCount:     4,
		Nodes: []LawbookNode{
			{
				NodeID: "DOC-CIVIL-CODE", DocumentID: "DOC-CIVIL-CODE",
				NodeType: NodeDocument, CanonicalRef: "civil-code",
				DisplayRef: "Code civil", Depth: 0, OrdinalPath: "1",
				SourcePath: "sources/civil-code.pdf",
				SourceHash: "sha256:aaaa", Status: StatusActive,
				Priority: PriorityHigh, Domain: "civil-law",
			},
			{
				NodeID: "CHAP-3", DocumentID: "DOC-CIVIL-CODE",
				NodeType: NodeChapter, CanonicalRef: "civil-code/chap-3",
				DisplayRef: "Livre III", Depth: 1, OrdinalPath: "1.3",
				SourcePath: "sources/civil-code.pdf",
				SourceHash: "sha256:bbbb", Status: StatusActive,
				Priority: PriorityMedium, Domain: "civil-law",
				ParentID: "DOC-CIVIL-CODE",
			},
			{
				NodeID: "ART-1382", DocumentID: "DOC-CIVIL-CODE",
				NodeType: NodeArticle, CanonicalRef: "civil-code/chap-3/art-1382",
				DisplayRef: "Art. 1382", Depth: 4, OrdinalPath: "1.3.1.1.1",
				SourcePath: "sources/civil-code.pdf",
				SourceHash: "sha256:cccc", Status: StatusAmended,
				Priority: PriorityCritical, Domain: "civil-law",
				ParentID: "CHAP-3", Title: "Responsabilité",
			},
			{
				NodeID: "ART-1383", DocumentID: "DOC-CIVIL-CODE",
				NodeType: NodeArticle, CanonicalRef: "civil-code/chap-3/art-1383",
				DisplayRef: "Art. 1383", Depth: 4, OrdinalPath: "1.3.1.1.2",
				SourcePath: "sources/civil-code.pdf",
				SourceHash: "sha256:dddd", Status: StatusRepealed,
				Priority: PriorityLow, Domain: "civil-law",
				ParentID: "CHAP-3",
			},
		},
	}
}

func TestAssembleFeedFormat(t *testing.T) {
	assembly := AssembleFeed(testFeed(), AssembleOptions{Now: assemblyNow})

	if assembly.Format != FeedFormatVersion {
		t.Fatalf("expected format %s, got %s", FeedFormatVersion, assembly.Format)
	}
	if assembly.Version != "0.1.0" {
		t.Fatalf("expected version 0.1.0, got %s", assembly.Version)
	}
	if assembly.GeneratedAt == "" {
		t.Fatal("expected generated_at to be set")
	}
}

func TestAssembleFeedIndex(t *testing.T) {
	assembly := AssembleFeed(testFeed(), AssembleOptions{Now: assemblyNow})
	idx := assembly.Index

	if len(idx.NodesByID) != 4 {
		t.Fatalf("expected 4 nodes in index, got %d", len(idx.NodesByID))
	}
	if idx.NodesByID["ART-1382"] != 2 {
		t.Fatalf("expected ART-1382 at index 2, got %d", idx.NodesByID["ART-1382"])
	}
	if len(idx.RootNodes) != 1 || idx.RootNodes[0] != "DOC-CIVIL-CODE" {
		t.Fatalf("expected 1 root node DOC-CIVIL-CODE, got %v", idx.RootNodes)
	}
	if len(idx.NodesByType["article"]) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(idx.NodesByType["article"]))
	}
}

func TestAssembleFeedGovernance(t *testing.T) {
	assembly := AssembleFeed(testFeed(), AssembleOptions{Now: assemblyNow})
	gov := assembly.Governance

	if gov.TotalNodes != 4 {
		t.Fatalf("expected 4 total nodes, got %d", gov.TotalNodes)
	}
	if gov.ByStatus["active"] != 2 {
		t.Fatalf("expected 2 active, got %d", gov.ByStatus["active"])
	}
	if gov.ByStatus["amended"] != 1 {
		t.Fatalf("expected 1 amended, got %d", gov.ByStatus["amended"])
	}
	if gov.ByStatus["repealed"] != 1 {
		t.Fatalf("expected 1 repealed, got %d", gov.ByStatus["repealed"])
	}
	if gov.ActiveRatio != 0.5 {
		t.Fatalf("expected active ratio 0.5, got %f", gov.ActiveRatio)
	}
	if len(gov.StaleNodes) != 2 {
		t.Fatalf("expected 2 stale nodes, got %d: %v", len(gov.StaleNodes), gov.StaleNodes)
	}
}

func TestAssembleFeedCitationMap(t *testing.T) {
	assembly := AssembleFeed(testFeed(), AssembleOptions{Now: assemblyNow})
	cm := assembly.CitationMap

	if cm.ByCanonicalRef["civil-code/chap-3/art-1382"] != "ART-1382" {
		t.Fatalf("expected canonical ref mapping for ART-1382, got %s",
			cm.ByCanonicalRef["civil-code/chap-3/art-1382"])
	}
	if cm.ByDisplayRef["Art. 1382"] != "ART-1382" {
		t.Fatalf("expected display ref mapping for Art. 1382")
	}
}

func TestAssembleFeedParentChains(t *testing.T) {
	assembly := AssembleFeed(testFeed(), AssembleOptions{Now: assemblyNow})
	cm := assembly.CitationMap

	chain := cm.ParentChains["ART-1382"]
	if len(chain) != 2 {
		t.Fatalf("expected parent chain of length 2 for ART-1382, got %d: %v", len(chain), chain)
	}
	if chain[0] != "DOC-CIVIL-CODE" || chain[1] != "CHAP-3" {
		t.Fatalf("expected [DOC-CIVIL-CODE, CHAP-3], got %v", chain)
	}

	rootChain := cm.ParentChains["DOC-CIVIL-CODE"]
	if len(rootChain) != 0 {
		t.Fatalf("expected empty parent chain for root, got %v", rootChain)
	}
}

func TestAssembleFeedRAGMetadata(t *testing.T) {
	assembly := AssembleFeed(testFeed(), AssembleOptions{Now: assemblyNow})
	rag := assembly.RAGMetadata

	if len(rag) != 4 {
		t.Fatalf("expected 4 RAG chunks, got %d", len(rag))
	}

	art := rag[2] // ART-1382
	if art.ChunkID != "chunk:ART-1382" {
		t.Fatalf("expected chunk:ART-1382, got %s", art.ChunkID)
	}
	if art.CanonicalRef != "civil-code/chap-3/art-1382" {
		t.Fatalf("expected canonical ref, got %s", art.CanonicalRef)
	}
	if art.DisplayRef != "Art. 1382" {
		t.Fatalf("expected display ref, got %s", art.DisplayRef)
	}
	if art.NodeType != "article" {
		t.Fatalf("expected article, got %s", art.NodeType)
	}
	if len(art.ParentChain) != 2 {
		t.Fatalf("expected parent chain length 2, got %d", len(art.ParentChain))
	}
	if art.SourceHash != "sha256:cccc" {
		t.Fatalf("expected source hash, got %s", art.SourceHash)
	}
	if art.GovernanceStatus != "amended" {
		t.Fatalf("expected governance status amended, got %s", art.GovernanceStatus)
	}
	if art.Domain != "civil-law" {
		t.Fatalf("expected domain civil-law, got %s", art.Domain)
	}
	if art.Priority != "critical" {
		t.Fatalf("expected priority critical, got %s", art.Priority)
	}
	if art.Depth != 4 {
		t.Fatalf("expected depth 4, got %d", art.Depth)
	}
}

func TestAssembleFeedRAGEmptyParentChain(t *testing.T) {
	assembly := AssembleFeed(testFeed(), AssembleOptions{Now: assemblyNow})
	root := assembly.RAGMetadata[0]
	if root.ParentChain == nil {
		t.Fatal("expected non-nil parent chain (empty slice)")
	}
	if len(root.ParentChain) != 0 {
		t.Fatalf("expected empty parent chain for root, got %v", root.ParentChain)
	}
}

func TestAssembleFeedEngineImportDocuments(t *testing.T) {
	assembly := AssembleFeed(testFeed(), AssembleOptions{Now: assemblyNow})
	eng := assembly.EngineImport

	if len(eng.Documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(eng.Documents))
	}
	doc := eng.Documents[0]
	if doc.DocumentID != "DOC-CIVIL-CODE" {
		t.Fatalf("expected DOC-CIVIL-CODE, got %s", doc.DocumentID)
	}
	if doc.Domain != "civil-law" {
		t.Fatalf("expected civil-law, got %s", doc.Domain)
	}
	if doc.NodeCount != 4 {
		t.Fatalf("expected 4 nodes, got %d", doc.NodeCount)
	}
}

func TestAssembleFeedEngineImportNodes(t *testing.T) {
	assembly := AssembleFeed(testFeed(), AssembleOptions{Now: assemblyNow})
	eng := assembly.EngineImport

	if len(eng.Nodes) != 4 {
		t.Fatalf("expected 4 engine nodes, got %d", len(eng.Nodes))
	}

	node := eng.Nodes[2]
	if node.NodeID != "ART-1382" {
		t.Fatalf("expected ART-1382, got %s", node.NodeID)
	}
	if node.NodeType != "article" {
		t.Fatalf("expected article, got %s", node.NodeType)
	}
	if node.ParentID != "CHAP-3" {
		t.Fatalf("expected parent CHAP-3, got %s", node.ParentID)
	}
	if node.Status != "amended" {
		t.Fatalf("expected amended, got %s", node.Status)
	}
}

func TestAssembleFeedEngineImportRevisions(t *testing.T) {
	assembly := AssembleFeed(testFeed(), AssembleOptions{Now: assemblyNow})
	eng := assembly.EngineImport

	if len(eng.Revisions) != 4 {
		t.Fatalf("expected 4 revisions, got %d", len(eng.Revisions))
	}

	rev := eng.Revisions[0]
	if rev.NodeID != "DOC-CIVIL-CODE" {
		t.Fatalf("expected DOC-CIVIL-CODE, got %s", rev.NodeID)
	}
	if rev.SourceHash != "sha256:aaaa" {
		t.Fatalf("expected sha256:aaaa, got %s", rev.SourceHash)
	}
	if rev.Timestamp == "" {
		t.Fatal("expected timestamp on revision")
	}
}

func TestWriteFeedAssemblyJSON(t *testing.T) {
	assembly := AssembleFeed(testFeed(), AssembleOptions{Now: assemblyNow})

	var buf bytes.Buffer
	if err := WriteFeedAssembly(&buf, assembly); err != nil {
		t.Fatalf("write error: %v", err)
	}

	var decoded FeedAssembly
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode error: %v\n%s", err, buf.String())
	}
	if decoded.Format != FeedFormatVersion {
		t.Fatalf("expected format %s after round-trip, got %s", FeedFormatVersion, decoded.Format)
	}
	if len(decoded.RAGMetadata) != 4 {
		t.Fatalf("expected 4 RAG chunks after round-trip, got %d", len(decoded.RAGMetadata))
	}
}

func TestAssembleEmptyFeed(t *testing.T) {
	feed := LawbookFeed{
		SchemaVersion: "0.1.0",
		FeedID:        "empty-feed",
		DocumentID:    "DOC-EMPTY",
		Domain:        "test",
		GeneratedAt:   "2026-05-02T00:00:00Z",
		SourcePath:    "empty.pdf",
		SourceHash:    "sha256:0000",
		NodeCount:     0,
		Nodes:         []LawbookNode{},
	}

	assembly := AssembleFeed(feed, AssembleOptions{Now: assemblyNow})

	if assembly.Governance.TotalNodes != 0 {
		t.Fatalf("expected 0 total nodes, got %d", assembly.Governance.TotalNodes)
	}
	if len(assembly.RAGMetadata) != 0 {
		t.Fatalf("expected 0 RAG chunks, got %d", len(assembly.RAGMetadata))
	}
	if len(assembly.EngineImport.Nodes) != 0 {
		t.Fatalf("expected 0 engine nodes, got %d", len(assembly.EngineImport.Nodes))
	}
	if len(assembly.Index.RootNodes) != 0 {
		t.Fatalf("expected 0 root nodes, got %d", len(assembly.Index.RootNodes))
	}
}

func TestAssembleGovernancePriority(t *testing.T) {
	assembly := AssembleFeed(testFeed(), AssembleOptions{Now: assemblyNow})
	gov := assembly.Governance

	if gov.ByPriority["critical"] != 1 {
		t.Fatalf("expected 1 critical, got %d", gov.ByPriority["critical"])
	}
	if gov.ByPriority["high"] != 1 {
		t.Fatalf("expected 1 high, got %d", gov.ByPriority["high"])
	}
	if gov.ByPriority["medium"] != 1 {
		t.Fatalf("expected 1 medium, got %d", gov.ByPriority["medium"])
	}
	if gov.ByPriority["low"] != 1 {
		t.Fatalf("expected 1 low, got %d", gov.ByPriority["low"])
	}
}

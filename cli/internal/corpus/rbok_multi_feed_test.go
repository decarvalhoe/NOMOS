package corpus

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var multiFeedNow = time.Date(2026, 5, 2, 15, 0, 0, 0, time.UTC)

func feed1() LawbookFeed {
	return LawbookFeed{
		SchemaVersion: "0.1.0",
		FeedID:        "civil-code-feed",
		DocumentID:    "DOC-CIVIL-CODE",
		Domain:        "civil-law",
		GeneratedAt:   "2026-05-02T10:00:00Z",
		SourcePath:    "sources/civil-code.pdf",
		SourceHash:    "sha256:aaaa",
		NodeCount:     2,
		Nodes: []LawbookNode{
			{
				NodeID: "DOC-CIVIL-CODE", DocumentID: "DOC-CIVIL-CODE",
				NodeType: NodeDocument, CanonicalRef: "civil-code",
				DisplayRef: "Code civil", Depth: 0, OrdinalPath: "1",
				SourcePath: "sources/civil-code.pdf", SourceHash: "sha256:aaaa",
				Status: StatusActive, Priority: PriorityHigh, Domain: "civil-law",
			},
			{
				NodeID: "ART-1382", DocumentID: "DOC-CIVIL-CODE",
				NodeType: NodeArticle, CanonicalRef: "civil-code/art-1382",
				DisplayRef: "Art. 1382", Depth: 4, OrdinalPath: "1.1.1.1.1",
				SourcePath: "sources/civil-code.pdf", SourceHash: "sha256:bbbb",
				Status: StatusActive, Priority: PriorityCritical, Domain: "civil-law",
				ParentID: "DOC-CIVIL-CODE",
			},
		},
	}
}

func feed2() LawbookFeed {
	return LawbookFeed{
		SchemaVersion: "0.1.0",
		FeedID:        "labor-code-feed",
		DocumentID:    "DOC-LABOR-CODE",
		Domain:        "labor-law",
		GeneratedAt:   "2026-05-02T10:00:00Z",
		SourcePath:    "sources/labor-code.pdf",
		SourceHash:    "sha256:cccc",
		NodeCount:     2,
		Nodes: []LawbookNode{
			{
				NodeID: "DOC-LABOR-CODE", DocumentID: "DOC-LABOR-CODE",
				NodeType: NodeDocument, CanonicalRef: "labor-code",
				DisplayRef: "Code du travail", Depth: 0, OrdinalPath: "1",
				SourcePath: "sources/labor-code.pdf", SourceHash: "sha256:cccc",
				Status: StatusActive, Priority: PriorityHigh, Domain: "labor-law",
			},
			{
				NodeID: "ART-L1234", DocumentID: "DOC-LABOR-CODE",
				NodeType: NodeArticle, CanonicalRef: "labor-code/art-l1234",
				DisplayRef: "Art. L1234-1", Depth: 4, OrdinalPath: "1.1.1.1.1",
				SourcePath: "sources/labor-code.pdf", SourceHash: "sha256:dddd",
				Status: StatusAmended, Priority: PriorityMedium, Domain: "labor-law",
				ParentID: "DOC-LABOR-CODE",
			},
		},
	}
}

func TestAssembleMultiFeedBasic(t *testing.T) {
	assembly := AssembleMultiFeed([]LawbookFeed{feed1(), feed2()}, MultiAssembleOptions{Now: multiFeedNow})

	if assembly.Format != FeedFormatVersion {
		t.Fatalf("expected format %s, got %s", FeedFormatVersion, assembly.Format)
	}
	if assembly.DocumentCount != 2 {
		t.Fatalf("expected 2 documents, got %d", assembly.DocumentCount)
	}
	if assembly.TotalNodes != 4 {
		t.Fatalf("expected 4 total nodes, got %d", assembly.TotalNodes)
	}
}

func TestAssembleMultiFeedIndex(t *testing.T) {
	assembly := AssembleMultiFeed([]LawbookFeed{feed1(), feed2()}, MultiAssembleOptions{Now: multiFeedNow})

	if len(assembly.Index.NodesByID) != 4 {
		t.Fatalf("expected 4 nodes in index, got %d", len(assembly.Index.NodesByID))
	}
	if len(assembly.Index.RootNodes) != 2 {
		t.Fatalf("expected 2 root nodes, got %d: %v", len(assembly.Index.RootNodes), assembly.Index.RootNodes)
	}
	if len(assembly.Index.NodesByType["article"]) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(assembly.Index.NodesByType["article"]))
	}
}

func TestAssembleMultiFeedGovernance(t *testing.T) {
	assembly := AssembleMultiFeed([]LawbookFeed{feed1(), feed2()}, MultiAssembleOptions{Now: multiFeedNow})

	if assembly.Governance.TotalNodes != 4 {
		t.Fatalf("expected 4 total nodes, got %d", assembly.Governance.TotalNodes)
	}
	if assembly.Governance.ByStatus["active"] != 3 {
		t.Fatalf("expected 3 active, got %d", assembly.Governance.ByStatus["active"])
	}
	if assembly.Governance.ByStatus["amended"] != 1 {
		t.Fatalf("expected 1 amended, got %d", assembly.Governance.ByStatus["amended"])
	}
	if len(assembly.Governance.StaleNodes) != 1 || assembly.Governance.StaleNodes[0] != "ART-L1234" {
		t.Fatalf("expected stale [ART-L1234], got %v", assembly.Governance.StaleNodes)
	}
}

func TestAssembleMultiFeedCitations(t *testing.T) {
	assembly := AssembleMultiFeed([]LawbookFeed{feed1(), feed2()}, MultiAssembleOptions{Now: multiFeedNow})

	if assembly.CitationMap.ByCanonicalRef["civil-code/art-1382"] != "ART-1382" {
		t.Fatal("expected citation for ART-1382")
	}
	if assembly.CitationMap.ByCanonicalRef["labor-code/art-l1234"] != "ART-L1234" {
		t.Fatal("expected citation for ART-L1234")
	}
	chain := assembly.CitationMap.ParentChains["ART-1382"]
	if len(chain) != 1 || chain[0] != "DOC-CIVIL-CODE" {
		t.Fatalf("expected parent chain [DOC-CIVIL-CODE], got %v", chain)
	}
}

func TestAssembleMultiFeedRAGMetadata(t *testing.T) {
	assembly := AssembleMultiFeed([]LawbookFeed{feed1(), feed2()}, MultiAssembleOptions{Now: multiFeedNow})

	if len(assembly.RAGMetadata) != 4 {
		t.Fatalf("expected 4 RAG chunks, got %d", len(assembly.RAGMetadata))
	}
	// Check cross-document chunks
	domains := map[string]int{}
	for _, chunk := range assembly.RAGMetadata {
		domains[chunk.Domain]++
		if chunk.ParentChain == nil {
			t.Fatalf("expected non-nil parent chain for %s", chunk.NodeID)
		}
	}
	if domains["civil-law"] != 2 {
		t.Fatalf("expected 2 civil-law chunks, got %d", domains["civil-law"])
	}
	if domains["labor-law"] != 2 {
		t.Fatalf("expected 2 labor-law chunks, got %d", domains["labor-law"])
	}
}

func TestAssembleMultiFeedEngineImport(t *testing.T) {
	assembly := AssembleMultiFeed([]LawbookFeed{feed1(), feed2()}, MultiAssembleOptions{Now: multiFeedNow})

	if len(assembly.EngineImport.Documents) != 2 {
		t.Fatalf("expected 2 engine documents, got %d", len(assembly.EngineImport.Documents))
	}
	if len(assembly.EngineImport.Nodes) != 4 {
		t.Fatalf("expected 4 engine nodes, got %d", len(assembly.EngineImport.Nodes))
	}
	if len(assembly.EngineImport.Revisions) != 4 {
		t.Fatalf("expected 4 revisions, got %d", len(assembly.EngineImport.Revisions))
	}
	// Documents should be sorted by ID
	if assembly.EngineImport.Documents[0].DocumentID != "DOC-CIVIL-CODE" {
		t.Fatalf("expected first doc DOC-CIVIL-CODE, got %s", assembly.EngineImport.Documents[0].DocumentID)
	}
}

func TestAssembleMultiFeedEmpty(t *testing.T) {
	assembly := AssembleMultiFeed(nil, MultiAssembleOptions{Now: multiFeedNow})

	if assembly.DocumentCount != 0 {
		t.Fatalf("expected 0 documents, got %d", assembly.DocumentCount)
	}
	if assembly.TotalNodes != 0 {
		t.Fatalf("expected 0 nodes, got %d", assembly.TotalNodes)
	}
	if len(assembly.RAGMetadata) != 0 {
		t.Fatalf("expected 0 RAG chunks, got %d", len(assembly.RAGMetadata))
	}
}

func TestAssembleMultiFeedSingleDocument(t *testing.T) {
	assembly := AssembleMultiFeed([]LawbookFeed{feed1()}, MultiAssembleOptions{Now: multiFeedNow})

	if assembly.DocumentCount != 1 {
		t.Fatalf("expected 1 document, got %d", assembly.DocumentCount)
	}
	if assembly.TotalNodes != 2 {
		t.Fatalf("expected 2 nodes, got %d", assembly.TotalNodes)
	}
	if len(assembly.EngineImport.Documents) != 1 {
		t.Fatalf("expected 1 engine document, got %d", len(assembly.EngineImport.Documents))
	}
}

func TestWriteMultiFeedAssemblyJSON(t *testing.T) {
	assembly := AssembleMultiFeed([]LawbookFeed{feed1(), feed2()}, MultiAssembleOptions{Now: multiFeedNow})

	var buf bytes.Buffer
	if err := WriteMultiFeedAssembly(&buf, assembly); err != nil {
		t.Fatalf("write error: %v", err)
	}

	var decoded MultiFeedAssembly
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.DocumentCount != 2 {
		t.Fatalf("expected 2 documents after round-trip, got %d", decoded.DocumentCount)
	}
	if decoded.TotalNodes != 4 {
		t.Fatalf("expected 4 nodes after round-trip, got %d", decoded.TotalNodes)
	}
}

func TestWriteMultiFeedArtifacts(t *testing.T) {
	outDir := t.TempDir()
	assembly := AssembleMultiFeed([]LawbookFeed{feed1(), feed2()}, MultiAssembleOptions{Now: multiFeedNow})

	if err := WriteMultiFeedArtifacts(assembly, outDir); err != nil {
		t.Fatalf("write artifacts error: %v", err)
	}

	expectedFiles := []string{
		"rbok-lawbook-feed.json",
		"rbok-lawbook-index.json",
		"rbok-rag-metadata.json",
		"rbok-engine-import.json",
	}
	for _, name := range expectedFiles {
		path := filepath.Join(outDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("expected %s to be non-empty", name)
		}
	}

	// Verify index file content
	indexData, err := os.ReadFile(filepath.Join(outDir, "rbok-lawbook-index.json"))
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	var index LawbookIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		t.Fatalf("decode index: %v", err)
	}
	if len(index.NodesByID) != 4 {
		t.Fatalf("expected 4 nodes in index file, got %d", len(index.NodesByID))
	}

	// Verify RAG metadata file content
	ragData, err := os.ReadFile(filepath.Join(outDir, "rbok-rag-metadata.json"))
	if err != nil {
		t.Fatalf("read rag: %v", err)
	}
	var ragChunks []RAGChunk
	if err := json.Unmarshal(ragData, &ragChunks); err != nil {
		t.Fatalf("decode rag: %v", err)
	}
	if len(ragChunks) != 4 {
		t.Fatalf("expected 4 RAG chunks in file, got %d", len(ragChunks))
	}

	// Verify engine import file content
	engineData, err := os.ReadFile(filepath.Join(outDir, "rbok-engine-import.json"))
	if err != nil {
		t.Fatalf("read engine: %v", err)
	}
	var engine EngineImport
	if err := json.Unmarshal(engineData, &engine); err != nil {
		t.Fatalf("decode engine: %v", err)
	}
	if len(engine.Documents) != 2 {
		t.Fatalf("expected 2 engine documents in file, got %d", len(engine.Documents))
	}
}

func TestMultiFeedCrossDocumentParentChains(t *testing.T) {
	assembly := AssembleMultiFeed([]LawbookFeed{feed1(), feed2()}, MultiAssembleOptions{Now: multiFeedNow})

	// ART-L1234 parent is DOC-LABOR-CODE (same document)
	chain := assembly.CitationMap.ParentChains["ART-L1234"]
	if len(chain) != 1 || chain[0] != "DOC-LABOR-CODE" {
		t.Fatalf("expected ART-L1234 parent chain [DOC-LABOR-CODE], got %v", chain)
	}
	// Root nodes have empty chains
	rootChain := assembly.CitationMap.ParentChains["DOC-CIVIL-CODE"]
	if len(rootChain) != 0 {
		t.Fatalf("expected empty root chain, got %v", rootChain)
	}
}

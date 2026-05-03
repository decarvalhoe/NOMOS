package corpus

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// MultiFeedAssembly is the assembled output for N documents.
type MultiFeedAssembly struct {
	Format        string           `json:"format"`
	Version       string           `json:"version"`
	GeneratedAt   string           `json:"generated_at"`
	DocumentCount int              `json:"document_count"`
	TotalNodes    int              `json:"total_nodes"`
	Feeds         []LawbookFeed    `json:"feeds"`
	Index         LawbookIndex     `json:"index"`
	Governance    GovernanceReport `json:"governance"`
	CitationMap   CitationMap      `json:"citation_map"`
	RAGMetadata   []RAGChunk       `json:"rag_metadata"`
	EngineImport  EngineImport     `json:"engine_import"`
}

// MultiAssembleOptions configures multi-document feed assembly.
type MultiAssembleOptions struct {
	Now    time.Time
	OutDir string
}

// AssembleMultiFeed takes multiple validated LawbookFeeds and produces a
// unified MultiFeedAssembly with merged index, governance, citations,
// RAG metadata, and engine import across all documents.
func AssembleMultiFeed(feeds []LawbookFeed, opts MultiAssembleOptions) MultiFeedAssembly {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	generatedAt := now.Format(time.RFC3339)

	// Merge all nodes across feeds.
	var allNodes []LawbookNode
	for _, feed := range feeds {
		allNodes = append(allNodes, feed.Nodes...)
	}

	// Build unified structures from the merged node set.
	mergedFeed := LawbookFeed{
		Nodes: allNodes,
	}
	index := buildIndex(mergedFeed)
	governance := buildGovernanceFromNodes(allNodes)
	citations := buildCitationMapFromNodes(allNodes)
	ragMeta := buildRAGFromNodes(allNodes, citations)
	engineImport := buildMultiEngineImport(feeds, allNodes, generatedAt)

	version := "0.1.0"
	if len(feeds) > 0 && feeds[0].SchemaVersion != "" {
		version = feeds[0].SchemaVersion
	}

	return MultiFeedAssembly{
		Format:        FeedFormatVersion,
		Version:       version,
		GeneratedAt:   generatedAt,
		DocumentCount: len(feeds),
		TotalNodes:    len(allNodes),
		Feeds:         feeds,
		Index:         index,
		Governance:    governance,
		CitationMap:   citations,
		RAGMetadata:   ragMeta,
		EngineImport:  engineImport,
	}
}

// WriteMultiFeedArtifacts writes the assembly as separate artifact files:
// - rbok-lawbook-feed.json (full assembly)
// - rbok-lawbook-index.json (index only)
// - rbok-rag-metadata.json (RAG chunks only)
// - rbok-engine-import.json (engine projection only)
func WriteMultiFeedArtifacts(assembly MultiFeedAssembly, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	artifacts := map[string]any{
		"rbok-lawbook-feed.json":  assembly,
		"rbok-lawbook-index.json": assembly.Index,
		"rbok-rag-metadata.json":  assembly.RAGMetadata,
		"rbok-engine-import.json": assembly.EngineImport,
	}

	for name, value := range artifacts {
		path := filepath.Join(outDir, name)
		if err := writeJSONFile(path, value); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	return nil
}

// WriteMultiFeedAssembly writes the full assembly as indented JSON.
func WriteMultiFeedAssembly(w io.Writer, assembly MultiFeedAssembly) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(assembly)
}

func writeJSONFile(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func buildGovernanceFromNodes(nodes []LawbookNode) GovernanceReport {
	report := GovernanceReport{
		TotalNodes: len(nodes),
		ByStatus:   make(map[string]int),
		ByPriority: make(map[string]int),
	}

	activeCount := 0
	for _, node := range nodes {
		report.ByStatus[string(node.Status)]++
		report.ByPriority[string(node.Priority)]++
		if node.Status == StatusActive {
			activeCount++
		}
		if node.Status == StatusRepealed || node.Status == StatusAmended {
			report.StaleNodes = append(report.StaleNodes, node.NodeID)
		}
	}

	if report.TotalNodes > 0 {
		report.ActiveRatio = float64(activeCount) / float64(report.TotalNodes)
	}

	sort.Strings(report.StaleNodes)
	return report
}

func buildCitationMapFromNodes(nodes []LawbookNode) CitationMap {
	cm := CitationMap{
		ByCanonicalRef: make(map[string]string, len(nodes)),
		ByDisplayRef:   make(map[string]string, len(nodes)),
		ParentChains:   make(map[string][]string, len(nodes)),
	}

	nodeMap := make(map[string]*LawbookNode, len(nodes))
	for i := range nodes {
		nodeMap[nodes[i].NodeID] = &nodes[i]
	}

	for _, node := range nodes {
		cm.ByCanonicalRef[node.CanonicalRef] = node.NodeID
		cm.ByDisplayRef[node.DisplayRef] = node.NodeID
		cm.ParentChains[node.NodeID] = resolveParentChain(node.NodeID, nodeMap)
	}

	return cm
}

func buildRAGFromNodes(nodes []LawbookNode, citations CitationMap) []RAGChunk {
	chunks := make([]RAGChunk, 0, len(nodes))
	for _, node := range nodes {
		chunk := RAGChunk{
			ChunkID:          fmt.Sprintf("chunk:%s", node.NodeID),
			NodeID:           node.NodeID,
			CanonicalRef:     node.CanonicalRef,
			DisplayRef:       node.DisplayRef,
			NodeType:         string(node.NodeType),
			ParentChain:      citations.ParentChains[node.NodeID],
			SourcePath:       node.SourcePath,
			SourceHash:       node.SourceHash,
			SourceClass:      node.SourceClass,
			CorpusLayer:      node.CorpusLayer,
			Authority:        node.Authority,
			Locator:          node.Locator,
			GovernanceStatus: string(node.Status),
			Domain:           node.Domain,
			Priority:         string(node.Priority),
			Depth:            node.Depth,
		}
		if chunk.ParentChain == nil {
			chunk.ParentChain = []string{}
		}
		chunks = append(chunks, chunk)
	}
	return chunks
}

func buildMultiEngineImport(feeds []LawbookFeed, allNodes []LawbookNode, timestamp string) EngineImport {
	docs := make([]EngineDocument, 0, len(feeds))
	for _, feed := range feeds {
		docs = append(docs, EngineDocument{
			DocumentID: feed.DocumentID,
			Domain:     feed.Domain,
			SourcePath: feed.SourcePath,
			SourceHash: feed.SourceHash,
			NodeCount:  len(feed.Nodes),
		})
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].DocumentID < docs[j].DocumentID
	})

	nodes := make([]EngineNode, 0, len(allNodes))
	revisions := make([]EngineRevision, 0, len(allNodes))
	for _, n := range allNodes {
		nodes = append(nodes, EngineNode{
			NodeID:       n.NodeID,
			DocumentID:   n.DocumentID,
			NodeType:     string(n.NodeType),
			CanonicalRef: n.CanonicalRef,
			DisplayRef:   n.DisplayRef,
			Depth:        n.Depth,
			OrdinalPath:  n.OrdinalPath,
			ParentID:     n.ParentID,
			Status:       string(n.Status),
			Priority:     string(n.Priority),
		})
		revisions = append(revisions, EngineRevision{
			NodeID:     n.NodeID,
			SourceHash: n.SourceHash,
			Status:     string(n.Status),
			Timestamp:  timestamp,
		})
	}

	return EngineImport{
		Documents: docs,
		Nodes:     nodes,
		Revisions: revisions,
	}
}

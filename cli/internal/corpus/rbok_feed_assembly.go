package corpus

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
)

const FeedFormatVersion = "nomos.rbok-lawbook-feed.v1"

// FeedAssembly is the complete assembled output of a lawbook feed pipeline.
type FeedAssembly struct {
	Format       string           `json:"format"`
	Version      string           `json:"version"`
	GeneratedAt  string           `json:"generated_at"`
	Feed         LawbookFeed      `json:"feed"`
	Index        LawbookIndex     `json:"index"`
	Governance   GovernanceReport `json:"governance"`
	CitationMap  CitationMap      `json:"citation_map"`
	RAGMetadata  []RAGChunk       `json:"rag_metadata"`
	EngineImport EngineImport     `json:"engine_import"`
}

// LawbookIndex provides fast lookup structures for the feed.
type LawbookIndex struct {
	NodesByID   map[string]int      `json:"nodes_by_id"`
	NodesByType map[string][]string `json:"nodes_by_type"`
	RootNodes   []string            `json:"root_nodes"`
	Depth       map[string]int      `json:"depth"`
}

// GovernanceReport summarizes the governance status of the feed.
type GovernanceReport struct {
	TotalNodes  int            `json:"total_nodes"`
	ByStatus    map[string]int `json:"by_status"`
	ByPriority  map[string]int `json:"by_priority"`
	ActiveRatio float64        `json:"active_ratio"`
	StaleNodes  []string       `json:"stale_nodes,omitempty"`
}

// CitationMap maps canonical refs to their node IDs for cross-referencing.
type CitationMap struct {
	ByCanonicalRef map[string]string   `json:"by_canonical_ref"`
	ByDisplayRef   map[string]string   `json:"by_display_ref"`
	ParentChains   map[string][]string `json:"parent_chains"`
}

// RAGChunk represents metadata for a single vector-store chunk.
type RAGChunk struct {
	ChunkID          string   `json:"chunk_id"`
	NodeID           string   `json:"node_id"`
	CanonicalRef     string   `json:"canonical_ref"`
	DisplayRef       string   `json:"display_ref"`
	NodeType         string   `json:"node_type"`
	ParentChain      []string `json:"parent_chain"`
	SourceHash       string   `json:"source_hash"`
	GovernanceStatus string   `json:"governance_status"`
	Domain           string   `json:"domain"`
	Priority         string   `json:"priority"`
	Depth            int      `json:"depth"`
}

// EngineImport is the projection for the RBOK engine database import.
type EngineImport struct {
	Documents []EngineDocument `json:"documents"`
	Nodes     []EngineNode     `json:"nodes"`
	Revisions []EngineRevision `json:"revisions"`
}

// EngineDocument is a top-level document record for the engine.
type EngineDocument struct {
	DocumentID string `json:"document_id"`
	Domain     string `json:"domain"`
	SourcePath string `json:"source_path"`
	SourceHash string `json:"source_hash"`
	NodeCount  int    `json:"node_count"`
}

// EngineNode is a single node record for the engine.
type EngineNode struct {
	NodeID       string `json:"node_id"`
	DocumentID   string `json:"document_id"`
	NodeType     string `json:"node_type"`
	CanonicalRef string `json:"canonical_ref"`
	DisplayRef   string `json:"display_ref"`
	Depth        int    `json:"depth"`
	OrdinalPath  string `json:"ordinal_path"`
	ParentID     string `json:"parent_id,omitempty"`
	Status       string `json:"status"`
	Priority     string `json:"priority"`
}

// EngineRevision captures a snapshot revision for the engine.
type EngineRevision struct {
	NodeID     string `json:"node_id"`
	SourceHash string `json:"source_hash"`
	Status     string `json:"status"`
	Timestamp  string `json:"timestamp"`
}

// AssembleOptions configures feed assembly.
type AssembleOptions struct {
	Now time.Time
}

// AssembleFeed takes a validated LawbookFeed and produces the complete
// FeedAssembly with index, governance, citations, RAG metadata, and engine import.
func AssembleFeed(feed LawbookFeed, opts AssembleOptions) FeedAssembly {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	generatedAt := now.Format(time.RFC3339)

	index := buildIndex(feed)
	governance := buildGovernance(feed)
	citations := buildCitationMap(feed)
	ragMeta := buildLawbookRAGMetadata(feed, citations)
	engineImport := buildEngineImport(feed, generatedAt)

	return FeedAssembly{
		Format:       FeedFormatVersion,
		Version:      feed.SchemaVersion,
		GeneratedAt:  generatedAt,
		Feed:         feed,
		Index:        index,
		Governance:   governance,
		CitationMap:  citations,
		RAGMetadata:  ragMeta,
		EngineImport: engineImport,
	}
}

// WriteFeedAssembly writes the assembly as indented JSON.
func WriteFeedAssembly(w io.Writer, assembly FeedAssembly) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(assembly)
}

func buildIndex(feed LawbookFeed) LawbookIndex {
	idx := LawbookIndex{
		NodesByID:   make(map[string]int, len(feed.Nodes)),
		NodesByType: make(map[string][]string),
		Depth:       make(map[string]int, len(feed.Nodes)),
	}

	for i, node := range feed.Nodes {
		idx.NodesByID[node.NodeID] = i
		idx.NodesByType[string(node.NodeType)] = append(idx.NodesByType[string(node.NodeType)], node.NodeID)
		idx.Depth[node.NodeID] = node.Depth
		if node.ParentID == "" {
			idx.RootNodes = append(idx.RootNodes, node.NodeID)
		}
	}

	sort.Strings(idx.RootNodes)
	return idx
}

func buildGovernance(feed LawbookFeed) GovernanceReport {
	report := GovernanceReport{
		TotalNodes: len(feed.Nodes),
		ByStatus:   make(map[string]int),
		ByPriority: make(map[string]int),
	}

	activeCount := 0
	for _, node := range feed.Nodes {
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

func buildCitationMap(feed LawbookFeed) CitationMap {
	cm := CitationMap{
		ByCanonicalRef: make(map[string]string, len(feed.Nodes)),
		ByDisplayRef:   make(map[string]string, len(feed.Nodes)),
		ParentChains:   make(map[string][]string, len(feed.Nodes)),
	}

	nodeMap := make(map[string]*LawbookNode, len(feed.Nodes))
	for i := range feed.Nodes {
		nodeMap[feed.Nodes[i].NodeID] = &feed.Nodes[i]
	}

	for _, node := range feed.Nodes {
		cm.ByCanonicalRef[node.CanonicalRef] = node.NodeID
		cm.ByDisplayRef[node.DisplayRef] = node.NodeID
		cm.ParentChains[node.NodeID] = resolveParentChain(node.NodeID, nodeMap)
	}

	return cm
}

func resolveParentChain(nodeID string, nodeMap map[string]*LawbookNode) []string {
	var chain []string
	visited := map[string]bool{}
	current := nodeID

	for {
		node, ok := nodeMap[current]
		if !ok || node.ParentID == "" {
			break
		}
		if visited[node.ParentID] {
			break // prevent cycles
		}
		visited[node.ParentID] = true
		chain = append([]string{node.ParentID}, chain...)
		current = node.ParentID
	}

	return chain
}

func buildLawbookRAGMetadata(feed LawbookFeed, citations CitationMap) []RAGChunk {
	chunks := make([]RAGChunk, 0, len(feed.Nodes))

	for _, node := range feed.Nodes {
		chunk := RAGChunk{
			ChunkID:          fmt.Sprintf("chunk:%s", node.NodeID),
			NodeID:           node.NodeID,
			CanonicalRef:     node.CanonicalRef,
			DisplayRef:       node.DisplayRef,
			NodeType:         string(node.NodeType),
			ParentChain:      citations.ParentChains[node.NodeID],
			SourceHash:       node.SourceHash,
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

func buildEngineImport(feed LawbookFeed, timestamp string) EngineImport {
	doc := EngineDocument{
		DocumentID: feed.DocumentID,
		Domain:     feed.Domain,
		SourcePath: feed.SourcePath,
		SourceHash: feed.SourceHash,
		NodeCount:  len(feed.Nodes),
	}

	nodes := make([]EngineNode, 0, len(feed.Nodes))
	revisions := make([]EngineRevision, 0, len(feed.Nodes))

	for _, n := range feed.Nodes {
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
		Documents: []EngineDocument{doc},
		Nodes:     nodes,
		Revisions: revisions,
	}
}

package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// AuthorityLevel classifies how authoritative a chunk is.
type AuthorityLevel string

const (
	AuthorityAuthoritative AuthorityLevel = "authoritative"
	AuthorityReference     AuthorityLevel = "reference"
	AuthorityDerived       AuthorityLevel = "derived"
)

// ProvenanceLink is one hop in the provenance chain.
type ProvenanceLink struct {
	Layer      string `json:"layer"`
	DocumentID string `json:"document_id"`
	NodeID     string `json:"node_id"`
	NodeType   string `json:"node_type"`
}

// RuntimeRAGChunk is the enriched RAG metadata for a single chunk,
// adding authority and provenance to the base RAGChunk.
type RuntimeRAGChunk struct {
	ChunkID          string           `json:"chunk_id"`
	NodeID           string           `json:"node_id"`
	DocumentID       string           `json:"document_id"`
	CanonicalRef     string           `json:"canonical_ref"`
	DisplayRef       string           `json:"display_ref"`
	NodeType         string           `json:"node_type"`
	Domain           string           `json:"domain"`
	Text             string           `json:"text"`
	AuthorityLevel   AuthorityLevel   `json:"authority_level"`
	Confidence       string           `json:"confidence"`
	ProvenanceChain  []ProvenanceLink `json:"provenance_chain"`
	ParentChain      []string         `json:"parent_chain"`
	SourcePath       string           `json:"source_path"`
	SourceHash       string           `json:"source_hash"`
	Priority         string           `json:"priority"`
	Status           string           `json:"status"`
	Depth            int              `json:"depth"`
	TokenCount       int              `json:"token_count"`
	CharCount        int              `json:"char_count"`
	GeneratedAt      string           `json:"generated_at"`
}

// RuntimeRAGResult holds the full RAG generation output.
type RuntimeRAGResult struct {
	SchemaVersion string            `json:"schema_version"`
	Domain        string            `json:"domain"`
	GeneratedAt   string            `json:"generated_at"`
	TotalChunks   int               `json:"total_chunks"`
	ByAuthority   map[string]int    `json:"by_authority"`
	ByConfidence  map[string]int    `json:"by_confidence"`
	Chunks        []RuntimeRAGChunk `json:"chunks"`
}

// RuntimeRAGConfig controls RAG metadata generation.
type RuntimeRAGConfig struct {
	Domain             string
	Layer              string // e.g. "lawbook", "corpus", "regulation"
	TokenEstimateRatio float64
	Now                time.Time
}

func (c RuntimeRAGConfig) tokenRatio() float64 {
	if c.TokenEstimateRatio > 0 {
		return c.TokenEstimateRatio
	}
	return 4.0
}

func (c RuntimeRAGConfig) now() time.Time {
	if !c.Now.IsZero() {
		return c.Now
	}
	return time.Now().UTC()
}

// GenerateRuntimeRAG produces enriched RAG metadata from a lawbook feed.
func GenerateRuntimeRAG(feed LawbookFeed, config RuntimeRAGConfig) RuntimeRAGResult {
	now := config.now()
	nodeIndex := indexNodes(feed.Nodes)

	chunks := make([]RuntimeRAGChunk, 0, len(feed.Nodes))
	byAuthority := map[string]int{}
	byConfidence := map[string]int{}

	for _, node := range feed.Nodes {
		if strings.TrimSpace(node.Text) == "" {
			continue
		}

		authority := classifyAuthority(node)
		confidence := classifyConfidence(node, authority)
		provenance := buildProvenanceChain(node, feed, config.Layer, nodeIndex)
		parentChain := buildParentChain(node, nodeIndex)

		charCount := utf8.RuneCountInString(node.Text)
		tokenCount := int(float64(charCount)/config.tokenRatio() + 0.5)

		chunkID := computeRAGChunkID(feed.FeedID, node.NodeID, node.SourceHash)

		chunk := RuntimeRAGChunk{
			ChunkID:         chunkID,
			NodeID:          node.NodeID,
			DocumentID:      node.DocumentID,
			CanonicalRef:    node.CanonicalRef,
			DisplayRef:      node.DisplayRef,
			NodeType:        string(node.NodeType),
			Domain:          firstNonEmptyStr(node.Domain, config.Domain, feed.Domain),
			Text:            node.Text,
			AuthorityLevel:  authority,
			Confidence:      confidence,
			ProvenanceChain: provenance,
			ParentChain:     parentChain,
			SourcePath:      node.SourcePath,
			SourceHash:      node.SourceHash,
			Priority:        string(node.Priority),
			Status:          string(node.Status),
			Depth:           node.Depth,
			TokenCount:      tokenCount,
			CharCount:       charCount,
			GeneratedAt:     now.Format(time.RFC3339),
		}

		chunks = append(chunks, chunk)
		byAuthority[string(authority)]++
		byConfidence[confidence]++
	}

	return RuntimeRAGResult{
		SchemaVersion: "0.1.0",
		Domain:        firstNonEmptyStr(config.Domain, feed.Domain),
		GeneratedAt:   now.Format(time.RFC3339),
		TotalChunks:   len(chunks),
		ByAuthority:   byAuthority,
		ByConfidence:  byConfidence,
		Chunks:        chunks,
	}
}

func classifyAuthority(node LawbookNode) AuthorityLevel {
	switch node.NodeType {
	case NodeDocument, NodeChapter:
		return AuthorityAuthoritative
	case NodeSection, NodeArticle:
		if node.Priority == PriorityCritical || node.Priority == PriorityHigh {
			return AuthorityAuthoritative
		}
		return AuthorityReference
	case NodeParagraph:
		return AuthorityReference
	case NodeAlinea:
		return AuthorityDerived
	default:
		return AuthorityDerived
	}
}

func classifyConfidence(node LawbookNode, authority AuthorityLevel) string {
	if node.Status == StatusActive && authority == AuthorityAuthoritative {
		return "high"
	}
	if node.Status == StatusActive && authority == AuthorityReference {
		return "high"
	}
	if node.Status == StatusActive {
		return "medium"
	}
	if node.Status == StatusAmended {
		return "medium"
	}
	if node.Status == StatusPending || node.Status == StatusNodeDraft {
		return "low"
	}
	if node.Status == StatusRepealed {
		return "low"
	}
	return "medium"
}

func buildProvenanceChain(node LawbookNode, feed LawbookFeed, layer string, index map[string]LawbookNode) []ProvenanceLink {
	var chain []ProvenanceLink

	// Layer link.
	chain = append(chain, ProvenanceLink{
		Layer:      layer,
		DocumentID: feed.DocumentID,
		NodeID:     feed.FeedID,
		NodeType:   "feed",
	})

	// Document link.
	chain = append(chain, ProvenanceLink{
		Layer:      layer,
		DocumentID: node.DocumentID,
		NodeID:     node.DocumentID,
		NodeType:   "document",
	})

	// Walk up parent chain to build provenance.
	current := node
	var ancestors []ProvenanceLink
	seen := map[string]bool{current.NodeID: true}
	for current.ParentID != "" {
		if seen[current.ParentID] {
			break
		}
		seen[current.ParentID] = true
		parent, ok := index[current.ParentID]
		if !ok {
			break
		}
		ancestors = append(ancestors, ProvenanceLink{
			Layer:      layer,
			DocumentID: parent.DocumentID,
			NodeID:     parent.NodeID,
			NodeType:   string(parent.NodeType),
		})
		current = parent
	}
	// Reverse to get root-first order.
	for i := len(ancestors) - 1; i >= 0; i-- {
		chain = append(chain, ancestors[i])
	}

	// Self link.
	chain = append(chain, ProvenanceLink{
		Layer:      layer,
		DocumentID: node.DocumentID,
		NodeID:     node.NodeID,
		NodeType:   string(node.NodeType),
	})

	return chain
}

func buildParentChain(node LawbookNode, index map[string]LawbookNode) []string {
	var chain []string
	seen := map[string]bool{node.NodeID: true}
	current := node
	for current.ParentID != "" {
		if seen[current.ParentID] {
			break
		}
		seen[current.ParentID] = true
		chain = append([]string{current.ParentID}, chain...)
		parent, ok := index[current.ParentID]
		if !ok {
			break
		}
		current = parent
	}
	return chain
}

func indexNodes(nodes []LawbookNode) map[string]LawbookNode {
	idx := make(map[string]LawbookNode, len(nodes))
	for _, n := range nodes {
		idx[n.NodeID] = n
	}
	return idx
}

func computeRAGChunkID(feedID, nodeID, sourceHash string) string {
	h := sha256.New()
	h.Write([]byte(feedID))
	h.Write([]byte{0})
	h.Write([]byte(nodeID))
	h.Write([]byte{0})
	h.Write([]byte(sourceHash))
	return "rag-" + hex.EncodeToString(h.Sum(nil)[:12])
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// FilterByAuthority returns chunks matching the given authority level.
func FilterByAuthority(chunks []RuntimeRAGChunk, level AuthorityLevel) []RuntimeRAGChunk {
	var out []RuntimeRAGChunk
	for _, c := range chunks {
		if c.AuthorityLevel == level {
			out = append(out, c)
		}
	}
	return out
}

// FilterByMinConfidence returns chunks at or above the given confidence.
func FilterByMinConfidence(chunks []RuntimeRAGChunk, minConfidence string) []RuntimeRAGChunk {
	minRank := confidenceRank(minConfidence)
	var out []RuntimeRAGChunk
	for _, c := range chunks {
		if confidenceRank(c.Confidence) >= minRank {
			out = append(out, c)
		}
	}
	return out
}

func confidenceRank(c string) int {
	switch c {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// ContextWindow assembles a prompt context from chunks with provenance.
func ContextWindow(chunks []RuntimeRAGChunk, maxTokens int) (text string, sources []string) {
	var b strings.Builder
	tokenBudget := maxTokens
	seen := map[string]bool{}

	for _, c := range chunks {
		if c.TokenCount > tokenBudget {
			continue
		}
		fmt.Fprintf(&b, "[%s | %s | confidence:%s]\n%s\n\n",
			c.DisplayRef, c.AuthorityLevel, c.Confidence, c.Text)
		tokenBudget -= c.TokenCount
		if !seen[c.CanonicalRef] {
			sources = append(sources, c.CanonicalRef)
			seen[c.CanonicalRef] = true
		}
		if tokenBudget <= 0 {
			break
		}
	}
	return b.String(), sources
}

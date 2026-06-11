package atomization

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	DefaultMaxTokens = 512
	DefaultMinTokens = 10
)

var (
	ErrNoSourceHash   = errors.New("chunk rejected: missing source hash")
	ErrExceedsMaxLen  = errors.New("chunk rejected: exceeds max token limit")
	ErrEmptyContent   = errors.New("chunk rejected: empty content")
	ErrNoCanonicalRef = errors.New("chunk rejected: missing canonical_ref")
)

// RAGChunk is a projected chunk ready for vector embedding and retrieval.
type RAGChunk struct {
	ChunkID      string            `json:"chunk_id"`
	CanonicalRef string            `json:"canonical_ref"`
	DisplayRef   string            `json:"display_ref"`
	Content      string            `json:"content"`
	TokenCount   int               `json:"token_count"`
	SourceHash   string            `json:"source_hash"`
	SourcePath   string            `json:"source_path"`
	Confidence   string            `json:"confidence"`
	Citations    []string          `json:"citations,omitempty"`
	NodeType     string            `json:"node_type,omitempty"`
	Domain       string            `json:"domain,omitempty"`
	Depth        int               `json:"depth"`
	Metadata     map[string]string `json:"metadata,omitempty"`

	// Facets carries the CKM-01 classification forward from the source atom when
	// present. Additive: nil for atoms without facets (zero regression).
	Facets *Facets `json:"facets,omitempty"`
}


// ProjectionConfig controls RAG chunk projection behavior.
type ProjectionConfig struct {
	MaxTokens       int    // Maximum tokens per chunk (rejects above)
	MinTokens       int    // Minimum tokens to emit (skips below)
	DefaultDomain   string // Fallback domain
	ConfidenceMode  string // "high", "medium", "low" — based on source presence
}

// DefaultProjectionConfig returns production defaults.
func DefaultProjectionConfig() ProjectionConfig {
	return ProjectionConfig{
		MaxTokens:      DefaultMaxTokens,
		MinTokens:      DefaultMinTokens,
		DefaultDomain:  "unknown",
		ConfidenceMode: "source_based",
	}
}

// ProjectionResult holds the output of a RAG projection pass.
type ProjectionResult struct {
	Chunks   []RAGChunk       `json:"chunks"`
	Rejected []RejectedChunk  `json:"rejected"`
}

// RejectedChunk records an atom that failed safety gates.
type RejectedChunk struct {
	AtomID string `json:"atom_id"`
	Reason string `json:"reason"`
	Error  error  `json:"-"`
}

// ProjectAtoms converts atoms into RAG chunks with safety gates.
func ProjectAtoms(atoms []Atom, config ProjectionConfig) ProjectionResult {
	if config.MaxTokens <= 0 {
		config.MaxTokens = DefaultMaxTokens
	}
	if config.MinTokens <= 0 {
		config.MinTokens = DefaultMinTokens
	}

	var result ProjectionResult

	for _, atom := range atoms {
		chunk, err := projectOne(atom, config)
		if err != nil {
			result.Rejected = append(result.Rejected, RejectedChunk{
				AtomID: atom.ID,
				Reason: err.Error(),
				Error:  err,
			})
			continue
		}
		if chunk.TokenCount < config.MinTokens {
			// Skip too-small chunks silently (not a safety rejection).
			continue
		}
		result.Chunks = append(result.Chunks, chunk)
	}

	return result
}

func projectOne(atom Atom, config ProjectionConfig) (RAGChunk, error) {
	// Safety gate: content must not be empty.
	content := strings.TrimSpace(atom.Text)
	if content == "" {
		return RAGChunk{}, ErrEmptyContent
	}

	// Safety gate: source hash required.
	if atom.ContentHash == "" {
		return RAGChunk{}, ErrNoSourceHash
	}

	// Safety gate: canonical ref required.
	if atom.CanonicalRef == "" {
		return RAGChunk{}, ErrNoCanonicalRef
	}

	// Estimate token count (simple whitespace split approximation).
	tokenCount := estimateTokens(content)

	// Safety gate: max tokens.
	if tokenCount > config.MaxTokens {
		return RAGChunk{}, fmt.Errorf("%w: %d tokens (max %d)", ErrExceedsMaxLen, tokenCount, config.MaxTokens)
	}

	// Determine confidence based on source presence.
	confidence := determineConfidence(atom)

	// Build domain.
	domain := atom.Domain
	if domain == "" {
		domain = config.DefaultDomain
	}

	// Generate chunk ID.
	chunkID := generateChunkID(atom.ID, atom.ContentHash)

	return RAGChunk{
		ChunkID:      chunkID,
		CanonicalRef: atom.CanonicalRef,
		DisplayRef:   atom.CanonicalRef,
		Content:      content,
		TokenCount:   tokenCount,
		SourceHash:   atom.ContentHash,
		SourcePath:   atom.SourceSpan.File,
		Confidence:   confidence,
		Citations:    nil, // populated by reference projection
		NodeType:     string(atom.Type),
		Domain:       domain,
		Depth:        atom.Depth,
		Facets:       atom.Facets,
	}, nil
}

func estimateTokens(content string) int {
	// Rough estimate: ~4 chars per token for English/French text.
	// Use word count as more stable proxy.
	words := strings.Fields(content)
	return len(words)
}

func determineConfidence(atom Atom) string {
	if atom.ContentHash != "" && atom.SourceSpan.File != "" {
		return "high"
	}
	if atom.ContentHash != "" && atom.SourceSpan.File != "" {
		return "medium"
	}
	return "low"
}

func generateChunkID(atomID, sourceHash string) string {
	input := atomID + "|" + sourceHash
	h := sha256.Sum256([]byte(input))
	return "CHUNK-" + strings.ToUpper(hex.EncodeToString(h[:8]))
}

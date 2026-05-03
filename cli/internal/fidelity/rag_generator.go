package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// SourceBacking traces a RAG chunk to its original source.
type SourceBacking struct {
	SourceFile    string `json:"source_file"`
	StartLine     int    `json:"start_line"`
	EndLine       int    `json:"end_line"`
	SourceHash    string `json:"source_hash"`
	AtomID        string `json:"atom_id"`
	CanonicalRef  string `json:"canonical_ref"`
	ParentAtomID  string `json:"parent_atom_id,omitempty"`
	DocumentID    string `json:"document_id"`
	Profile       string `json:"profile"`
}

// SourceBackedChunk is a RAG chunk with full source traceability.
type SourceBackedChunk struct {
	ChunkID     string        `json:"chunk_id"`
	Content     string        `json:"content"`
	ContentHash string        `json:"content_hash"`
	TokenCount  int           `json:"token_count"`
	ChunkType   string        `json:"chunk_type"`
	Domain      string        `json:"domain"`
	Depth       int           `json:"depth"`
	Backing     SourceBacking `json:"backing"`
	Embeddable  bool          `json:"embeddable"`
	Reason      string        `json:"reason,omitempty"`
}

// RAGGeneratorConfig controls chunk generation.
type RAGGeneratorConfig struct {
	MaxTokens   int
	MinTokens   int
	DocumentID  string
	SourceFile  string
	SourceHash  string
	Profile     string
	Domain      string
}

// DefaultRAGGeneratorConfig returns production defaults.
func DefaultRAGGeneratorConfig() RAGGeneratorConfig {
	return RAGGeneratorConfig{
		MaxTokens: 512,
		MinTokens: 10,
	}
}

// RAGGeneratorResult holds the generator output.
type RAGGeneratorResult struct {
	DocumentID   string              `json:"document_id"`
	SourceFile   string              `json:"source_file"`
	SourceHash   string              `json:"source_hash"`
	TotalAtoms   int                 `json:"total_atoms"`
	TotalChunks  int                 `json:"total_chunks"`
	Embeddable   int                 `json:"embeddable"`
	Skipped      int                 `json:"skipped"`
	Rejected     int                 `json:"rejected"`
	ChainHash    string              `json:"chain_hash"`
	Chunks       []SourceBackedChunk `json:"chunks"`
}

// GenerateRAGChunks produces source-backed RAG chunks from PortableAtoms.
func GenerateRAGChunks(atoms []PortableAtom, config RAGGeneratorConfig) RAGGeneratorResult {
	if config.MaxTokens <= 0 {
		config.MaxTokens = 512
	}
	if config.MinTokens <= 0 {
		config.MinTokens = 10
	}

	result := RAGGeneratorResult{
		DocumentID: config.DocumentID,
		SourceFile: config.SourceFile,
		SourceHash: config.SourceHash,
		TotalAtoms: len(atoms),
	}

	h := sha256.New()
	h.Write([]byte(config.SourceHash))

	for _, atom := range atoms {
		text := strings.TrimSpace(atom.Text)
		if text == "" {
			result.Skipped++
			continue
		}

		tokens := ragEstimateTokens(text)
		contentHash := ragContentHash(text)

		chunk := SourceBackedChunk{
			ChunkID:     fmt.Sprintf("rag-%s", atom.ID),
			Content:     text,
			ContentHash: contentHash,
			TokenCount:  tokens,
			ChunkType:   atom.Type,
			Domain:      ragCoalesce(atom.Domain, config.Domain),
			Depth:       atom.Depth,
			Backing: SourceBacking{
				SourceFile:   config.SourceFile,
				StartLine:    atom.SourceLine,
				EndLine:      atom.SourceLine,
				SourceHash:   atom.ContentHash,
				AtomID:       atom.ID,
				CanonicalRef: atom.CanonicalRef,
				ParentAtomID: atom.ParentID,
				DocumentID:   config.DocumentID,
				Profile:      ragCoalesce(atom.Profile, config.Profile),
			},
		}

		if tokens < config.MinTokens {
			chunk.Embeddable = false
			chunk.Reason = fmt.Sprintf("below minimum tokens (%d < %d)", tokens, config.MinTokens)
			result.Skipped++
		} else if tokens > config.MaxTokens {
			chunk.Embeddable = false
			chunk.Reason = fmt.Sprintf("exceeds maximum tokens (%d > %d)", tokens, config.MaxTokens)
			result.Rejected++
		} else if atom.ContentHash == "" {
			chunk.Embeddable = false
			chunk.Reason = "missing source content hash"
			result.Rejected++
		} else {
			chunk.Embeddable = true
			result.Embeddable++
		}

		h.Write([]byte(contentHash))
		result.Chunks = append(result.Chunks, chunk)
	}

	result.TotalChunks = len(result.Chunks)
	result.ChainHash = "sha256:" + hex.EncodeToString(h.Sum(nil))
	return result
}

// GenerateRAGChunksFromCAST is a convenience that atomizes a CAST then generates chunks.
func GenerateRAGChunksFromCAST(cast CAST, config RAGGeneratorConfig) RAGGeneratorResult {
	var atoms []PortableAtom
	for _, node := range cast.Nodes {
		if node.Kind == KindDocument || node.Kind == KindThematicBreak {
			continue
		}
		text := node.Text
		if text == "" {
			text = node.RawText
		}
		atoms = append(atoms, PortableAtom{
			ID:           node.ID,
			CanonicalRef: fmt.Sprintf("%s#%s", config.DocumentID, node.ID),
			Type:         string(node.Kind),
			Text:         text,
			ContentHash:  node.Hash,
			Depth:        node.Level,
			ParentID:     node.ParentID,
			Domain:       config.Domain,
			Profile:      config.Profile,
			SourceLine:   node.Span.StartLine,
		})
	}
	if config.SourceHash == "" {
		config.SourceHash = cast.SourceHash
	}
	return GenerateRAGChunks(atoms, config)
}

// VerifyChunkBacking checks that a chunk's content hash matches its content.
func VerifyChunkBacking(chunk SourceBackedChunk) bool {
	return chunk.ContentHash == ragContentHash(chunk.Content)
}

// WriteRAGResultJSON serializes the result.
func WriteRAGResultJSON(w io.Writer, result RAGGeneratorResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func ragEstimateTokens(text string) int {
	words := len(strings.Fields(text))
	tokens := int(float64(words) * 1.33)
	if tokens < 1 && len(text) > 0 {
		tokens = 1
	}
	return tokens
}

func ragContentHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return "sha256:" + hex.EncodeToString(h[:])
}

func ragCoalesce(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

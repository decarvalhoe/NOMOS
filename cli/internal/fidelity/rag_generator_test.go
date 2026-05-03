package fidelity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func ragAtom(id, text, hash string, line int) PortableAtom {
	return PortableAtom{
		ID: id, CanonicalRef: "doc#" + id, Type: "paragraph",
		Text: text, ContentHash: hash, Depth: 1,
		Domain: "legal", Profile: "law-regulation", SourceLine: line,
	}
}

func ragConfig() RAGGeneratorConfig {
	return RAGGeneratorConfig{
		MaxTokens: 512, MinTokens: 5,
		DocumentID: "test-doc", SourceFile: "test.md",
		SourceHash: "sha256:source", Profile: "law-regulation", Domain: "legal",
	}
}

// --- Basic generation ---

func TestRAGGeneratorBasic(t *testing.T) {
	atoms := []PortableAtom{
		ragAtom("p1", "La garantie couvre les dégâts des eaux selon les conditions générales du contrat.", "sha256:a", 5),
		ragAtom("p2", "Les exclusions sont listées dans l'annexe A du contrat habitation.", "sha256:b", 10),
	}
	result := GenerateRAGChunks(atoms, ragConfig())
	if result.TotalAtoms != 2 {
		t.Fatalf("expected 2 atoms, got %d", result.TotalAtoms)
	}
	if result.Embeddable != 2 {
		t.Fatalf("expected 2 embeddable, got %d", result.Embeddable)
	}
	if result.TotalChunks != 2 {
		t.Fatalf("expected 2 chunks, got %d", result.TotalChunks)
	}
}

// --- Source backing ---

func TestRAGGeneratorSourceBacking(t *testing.T) {
	atoms := []PortableAtom{ragAtom("p1", "Content with full source backing and traceability.", "sha256:a", 42)}
	atoms[0].ParentID = "h1"
	result := GenerateRAGChunks(atoms, ragConfig())
	chunk := result.Chunks[0]

	if chunk.Backing.SourceFile != "test.md" {
		t.Fatalf("expected test.md, got %q", chunk.Backing.SourceFile)
	}
	if chunk.Backing.StartLine != 42 {
		t.Fatalf("expected line 42, got %d", chunk.Backing.StartLine)
	}
	if chunk.Backing.AtomID != "p1" {
		t.Fatalf("expected p1, got %q", chunk.Backing.AtomID)
	}
	if chunk.Backing.CanonicalRef != "doc#p1" {
		t.Fatalf("expected doc#p1, got %q", chunk.Backing.CanonicalRef)
	}
	if chunk.Backing.ParentAtomID != "h1" {
		t.Fatalf("expected h1, got %q", chunk.Backing.ParentAtomID)
	}
	if chunk.Backing.DocumentID != "test-doc" {
		t.Fatalf("expected test-doc, got %q", chunk.Backing.DocumentID)
	}
	if chunk.Backing.Profile != "law-regulation" {
		t.Fatalf("expected law-regulation, got %q", chunk.Backing.Profile)
	}
	if chunk.Backing.SourceHash != "sha256:a" {
		t.Fatalf("expected source hash, got %q", chunk.Backing.SourceHash)
	}
}

// --- Chunk ID ---

func TestRAGGeneratorChunkID(t *testing.T) {
	atoms := []PortableAtom{ragAtom("p1", "Some longer content for token counting purposes here.", "sha256:a", 1)}
	result := GenerateRAGChunks(atoms, ragConfig())
	if !strings.HasPrefix(result.Chunks[0].ChunkID, "rag-") {
		t.Fatalf("expected rag- prefix, got %q", result.Chunks[0].ChunkID)
	}
}

// --- Content hash ---

func TestRAGGeneratorContentHash(t *testing.T) {
	atoms := []PortableAtom{ragAtom("p1", "Stable content for hash verification test.", "sha256:a", 1)}
	result := GenerateRAGChunks(atoms, ragConfig())
	if !strings.HasPrefix(result.Chunks[0].ContentHash, "sha256:") {
		t.Fatalf("expected sha256, got %q", result.Chunks[0].ContentHash)
	}
}

func TestRAGGeneratorContentHashDeterministic(t *testing.T) {
	atoms := []PortableAtom{ragAtom("p1", "Same text for determinism check.", "sha256:a", 1)}
	r1 := GenerateRAGChunks(atoms, ragConfig())
	r2 := GenerateRAGChunks(atoms, ragConfig())
	if r1.Chunks[0].ContentHash != r2.Chunks[0].ContentHash {
		t.Fatal("content hash not deterministic")
	}
}

// --- Skipped: empty text ---

func TestRAGGeneratorSkipsEmpty(t *testing.T) {
	atoms := []PortableAtom{
		ragAtom("p1", "", "sha256:a", 1),
		ragAtom("p2", "   ", "sha256:b", 2),
	}
	result := GenerateRAGChunks(atoms, ragConfig())
	if result.TotalChunks != 0 {
		t.Fatalf("expected 0 chunks for empty, got %d", result.TotalChunks)
	}
	if result.Skipped != 2 {
		t.Fatalf("expected 2 skipped, got %d", result.Skipped)
	}
}

// --- Skipped: below min tokens ---

func TestRAGGeneratorSkipsBelowMin(t *testing.T) {
	atoms := []PortableAtom{ragAtom("p1", "Short", "sha256:a", 1)}
	cfg := ragConfig()
	cfg.MinTokens = 50
	result := GenerateRAGChunks(atoms, cfg)
	if result.Embeddable != 0 {
		t.Fatalf("expected 0 embeddable, got %d", result.Embeddable)
	}
	if !strings.Contains(result.Chunks[0].Reason, "below minimum") {
		t.Fatalf("expected min token reason, got %q", result.Chunks[0].Reason)
	}
}

// --- Rejected: exceeds max tokens ---

func TestRAGGeneratorRejectsOverMax(t *testing.T) {
	longText := strings.Repeat("word ", 1000) // ~1000 words
	atoms := []PortableAtom{ragAtom("p1", longText, "sha256:a", 1)}
	cfg := ragConfig()
	cfg.MaxTokens = 100
	result := GenerateRAGChunks(atoms, cfg)
	if result.Rejected != 1 {
		t.Fatalf("expected 1 rejected, got %d", result.Rejected)
	}
	if !strings.Contains(result.Chunks[0].Reason, "exceeds maximum") {
		t.Fatalf("expected max token reason, got %q", result.Chunks[0].Reason)
	}
}

// --- Rejected: missing source hash ---

func TestRAGGeneratorRejectsMissingHash(t *testing.T) {
	atoms := []PortableAtom{ragAtom("p1", "Content without a source content hash for verification.", "", 1)}
	result := GenerateRAGChunks(atoms, ragConfig())
	if result.Rejected != 1 {
		t.Fatalf("expected 1 rejected, got %d", result.Rejected)
	}
	if !strings.Contains(result.Chunks[0].Reason, "missing source") {
		t.Fatalf("expected missing hash reason, got %q", result.Chunks[0].Reason)
	}
}

// --- Mixed results ---

func TestRAGGeneratorMixed(t *testing.T) {
	atoms := []PortableAtom{
		ragAtom("ok", "Valid embeddable content with enough tokens for the minimum.", "sha256:a", 1),
		ragAtom("empty", "", "sha256:b", 2),
		ragAtom("nohash", "Content missing its source hash for traceability.", "", 3),
	}
	result := GenerateRAGChunks(atoms, ragConfig())
	if result.Embeddable != 1 {
		t.Fatalf("expected 1 embeddable, got %d", result.Embeddable)
	}
	if result.Skipped != 1 {
		t.Fatalf("expected 1 skipped, got %d", result.Skipped)
	}
	if result.Rejected != 1 {
		t.Fatalf("expected 1 rejected, got %d", result.Rejected)
	}
}

// --- Chain hash ---

func TestRAGGeneratorChainHash(t *testing.T) {
	atoms := []PortableAtom{ragAtom("p1", "Content for chain hash determinism verification.", "sha256:a", 1)}
	r1 := GenerateRAGChunks(atoms, ragConfig())
	r2 := GenerateRAGChunks(atoms, ragConfig())
	if r1.ChainHash != r2.ChainHash {
		t.Fatal("chain hash not deterministic")
	}
	if !strings.HasPrefix(r1.ChainHash, "sha256:") {
		t.Fatalf("expected sha256 prefix, got %q", r1.ChainHash)
	}
}

// --- From CAST ---

func TestGenerateRAGChunksFromCAST(t *testing.T) {
	cast := CAST{
		Root:       "doc-root",
		SourceHash: "sha256:castsrc",
		Nodes: []CNode{
			{ID: "doc-root", Kind: KindDocument, Span: Span{1, 10}},
			{ID: "h1", Kind: KindHeading, Text: "Introduction to the Legal Framework", Level: 1, Span: Span{1, 1}, ParentID: "doc-root", Hash: "sha256:h1"},
			{ID: "p1", Kind: KindParagraph, Text: "The regulation establishes baseline requirements for all participants in the market.", Span: Span{3, 5}, ParentID: "h1", Hash: "sha256:p1"},
		},
	}
	cfg := ragConfig()
	cfg.SourceHash = ""
	result := GenerateRAGChunksFromCAST(cast, cfg)
	if result.SourceHash != "sha256:castsrc" {
		t.Fatalf("expected cast source hash, got %q", result.SourceHash)
	}
	if result.TotalChunks < 2 {
		t.Fatalf("expected >= 2 chunks, got %d", result.TotalChunks)
	}
}

// --- Verify chunk backing ---

func TestVerifyChunkBacking(t *testing.T) {
	atoms := []PortableAtom{ragAtom("p1", "Verifiable content with proper hash chain.", "sha256:a", 1)}
	result := GenerateRAGChunks(atoms, ragConfig())
	if !VerifyChunkBacking(result.Chunks[0]) {
		t.Fatal("expected verification pass")
	}
}

func TestVerifyChunkBackingTampered(t *testing.T) {
	atoms := []PortableAtom{ragAtom("p1", "Original content for tamper detection.", "sha256:a", 1)}
	result := GenerateRAGChunks(atoms, ragConfig())
	result.Chunks[0].Content = "TAMPERED content"
	if VerifyChunkBacking(result.Chunks[0]) {
		t.Fatal("expected verification fail")
	}
}

// --- Empty input ---

func TestRAGGeneratorEmpty(t *testing.T) {
	result := GenerateRAGChunks(nil, ragConfig())
	if result.TotalChunks != 0 || result.Embeddable != 0 {
		t.Fatal("expected empty result")
	}
	if !strings.HasPrefix(result.ChainHash, "sha256:") {
		t.Fatal("expected chain hash even for empty")
	}
}

// --- Domain and type ---

func TestRAGGeneratorDomainAndType(t *testing.T) {
	atoms := []PortableAtom{ragAtom("p1", "Content carrying domain and type metadata for filtering.", "sha256:a", 1)}
	atoms[0].Type = "article"
	atoms[0].Domain = "insurance"
	result := GenerateRAGChunks(atoms, ragConfig())
	if result.Chunks[0].Domain != "insurance" {
		t.Fatalf("expected insurance, got %q", result.Chunks[0].Domain)
	}
	if result.Chunks[0].ChunkType != "article" {
		t.Fatalf("expected article, got %q", result.Chunks[0].ChunkType)
	}
}

// --- JSON roundtrip ---

func TestRAGGeneratorJSON(t *testing.T) {
	atoms := []PortableAtom{ragAtom("p1", "Content for JSON serialization roundtrip test.", "sha256:a", 1)}
	result := GenerateRAGChunks(atoms, ragConfig())
	var buf bytes.Buffer
	if err := WriteRAGResultJSON(&buf, result); err != nil {
		t.Fatal(err)
	}
	var decoded RAGGeneratorResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.TotalChunks != result.TotalChunks {
		t.Fatal("roundtrip mismatch")
	}
	if decoded.ChainHash != result.ChainHash {
		t.Fatal("chain hash mismatch")
	}
}

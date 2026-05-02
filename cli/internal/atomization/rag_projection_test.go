package atomization

import (
	"errors"
	"strings"
	"testing"
)

func validAtom() Atom {
	return Atom{
		ID:           "ATOM-001",
		CanonicalRef: "L.113-2",
		Text:      "L'assure est oblige de repondre exactement aux questions posees par l'assureur.",
		ContentHash:   "sha256:abc123def456",
		Type:     "article",
		Domain:       "assurance",
		Depth:        4,
	}
}


func TestProjectAtomsRejectsNoSourceHash(t *testing.T) {
	atom := validAtom()
	atom.ContentHash = ""

	result := ProjectAtoms([]Atom{atom}, DefaultProjectionConfig())

	if len(result.Chunks) != 0 {
		t.Fatal("expected no chunks for missing source hash")
	}
	if len(result.Rejected) != 1 {
		t.Fatalf("expected 1 rejected, got %d", len(result.Rejected))
	}
	if !errors.Is(result.Rejected[0].Error, ErrNoSourceHash) {
		t.Fatalf("expected ErrNoSourceHash, got: %v", result.Rejected[0].Error)
	}
}

func TestProjectAtomsRejectsExceedsMaxTokens(t *testing.T) {
	atom := validAtom()
	// Generate content exceeding default max (512 words).
	words := make([]string, 600)
	for i := range words {
		words[i] = "word"
	}
	atom.Text = strings.Join(words, " ")

	result := ProjectAtoms([]Atom{atom}, DefaultProjectionConfig())

	if len(result.Chunks) != 0 {
		t.Fatal("expected no chunks for oversized content")
	}
	if len(result.Rejected) != 1 {
		t.Fatalf("expected 1 rejected, got %d", len(result.Rejected))
	}
	if !errors.Is(result.Rejected[0].Error, ErrExceedsMaxLen) {
		t.Fatalf("expected ErrExceedsMaxLen, got: %v", result.Rejected[0].Error)
	}
}

func TestProjectAtomsRejectsEmptyContent(t *testing.T) {
	atom := validAtom()
	atom.Text = "   "

	result := ProjectAtoms([]Atom{atom}, DefaultProjectionConfig())

	if len(result.Chunks) != 0 {
		t.Fatal("expected no chunks for empty content")
	}
	if !errors.Is(result.Rejected[0].Error, ErrEmptyContent) {
		t.Fatalf("expected ErrEmptyContent, got: %v", result.Rejected[0].Error)
	}
}

func TestProjectAtomsRejectsNoCanonicalRef(t *testing.T) {
	atom := validAtom()
	atom.CanonicalRef = ""

	result := ProjectAtoms([]Atom{atom}, DefaultProjectionConfig())

	if len(result.Chunks) != 0 {
		t.Fatal("expected no chunks for missing canonical_ref")
	}
	if !errors.Is(result.Rejected[0].Error, ErrNoCanonicalRef) {
		t.Fatalf("expected ErrNoCanonicalRef, got: %v", result.Rejected[0].Error)
	}
}

func TestProjectAtomsSkipsBelowMinTokens(t *testing.T) {
	atom := validAtom()
	atom.Text = "Short." // 1 word, below min 10

	result := ProjectAtoms([]Atom{atom}, DefaultProjectionConfig())

	// Not rejected (not a safety issue), just skipped.
	if len(result.Chunks) != 0 {
		t.Fatal("expected chunk skipped below min tokens")
	}
	if len(result.Rejected) != 0 {
		t.Fatal("below-min should not be in rejected list")
	}
}


func TestProjectAtomsDefaultDomain(t *testing.T) {
	atom := validAtom()
	atom.Domain = ""

	config := DefaultProjectionConfig()
	config.DefaultDomain = "general"
	result := ProjectAtoms([]Atom{atom}, config)

	if result.Chunks[0].Domain != "general" {
		t.Fatalf("expected fallback domain 'general', got %s", result.Chunks[0].Domain)
	}
}


func TestProjectAtomsCustomMaxTokens(t *testing.T) {
	atom := validAtom()
	// 12 words in the default content.
	config := ProjectionConfig{MaxTokens: 5, MinTokens: 1}

	result := ProjectAtoms([]Atom{atom}, config)

	if len(result.Rejected) != 1 {
		t.Fatal("expected rejection at maxTokens=5")
	}
}

func TestProjectAtomsChunkIDDeterministic(t *testing.T) {
	atom := validAtom()
	r1 := ProjectAtoms([]Atom{atom}, DefaultProjectionConfig())
	r2 := ProjectAtoms([]Atom{atom}, DefaultProjectionConfig())

	if r1.Chunks[0].ChunkID != r2.Chunks[0].ChunkID {
		t.Fatal("chunk IDs should be deterministic")
	}
}

func TestProjectAtomsChunkIDUnique(t *testing.T) {
	a1 := validAtom()
	a2 := validAtom()
	a2.ID = "ATOM-002"

	result := ProjectAtoms([]Atom{a1, a2}, DefaultProjectionConfig())

	if len(result.Chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(result.Chunks))
	}
	if result.Chunks[0].ChunkID == result.Chunks[1].ChunkID {
		t.Fatal("different atoms should produce different chunk IDs")
	}
}


func TestProjectAtomsTokenCountPopulated(t *testing.T) {
	atom := validAtom()
	result := ProjectAtoms([]Atom{atom}, DefaultProjectionConfig())

	if result.Chunks[0].TokenCount <= 0 {
		t.Fatal("expected positive token count")
	}
}

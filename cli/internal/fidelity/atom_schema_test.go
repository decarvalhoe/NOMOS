package fidelity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// --- Deterministic IDs ---

func TestDeterministicIDStable(t *testing.T) {
	span := AtomSpan{File: "doc.md", StartLine: 10, EndLine: 15}
	id1 := DeterministicID("doc#node-1", span)
	id2 := DeterministicID("doc#node-1", span)
	if id1 != id2 {
		t.Fatalf("ID not stable: %q vs %q", id1, id2)
	}
}

func TestDeterministicIDPrefix(t *testing.T) {
	span := AtomSpan{File: "doc.md", StartLine: 1, EndLine: 1}
	id := DeterministicID("doc#h1", span)
	if !strings.HasPrefix(id, "atom-") {
		t.Fatalf("expected atom- prefix, got %q", id)
	}
}

func TestDeterministicIDLength(t *testing.T) {
	span := AtomSpan{File: "doc.md", StartLine: 1, EndLine: 1}
	id := DeterministicID("doc#h1", span)
	// "atom-" + 16 hex chars (8 bytes)
	if len(id) != 5+16 {
		t.Fatalf("expected length 21, got %d: %q", len(id), id)
	}
}

func TestDeterministicIDUnique(t *testing.T) {
	ids := map[string]bool{}
	for i := 1; i <= 100; i++ {
		span := AtomSpan{File: "doc.md", StartLine: i, EndLine: i}
		id := DeterministicID("doc#node", span)
		if ids[id] {
			t.Fatalf("duplicate ID at line %d: %q", i, id)
		}
		ids[id] = true
	}
}

func TestDeterministicIDDiffersForDiffRef(t *testing.T) {
	span := AtomSpan{File: "doc.md", StartLine: 1, EndLine: 1}
	id1 := DeterministicID("doc-a#node", span)
	id2 := DeterministicID("doc-b#node", span)
	if id1 == id2 {
		t.Fatal("different refs should produce different IDs")
	}
}

func TestDeterministicIDDiffersForDiffSpan(t *testing.T) {
	span1 := AtomSpan{File: "doc.md", StartLine: 1, EndLine: 1}
	span2 := AtomSpan{File: "doc.md", StartLine: 2, EndLine: 2}
	id1 := DeterministicID("doc#node", span1)
	id2 := DeterministicID("doc#node", span2)
	if id1 == id2 {
		t.Fatal("different spans should produce different IDs")
	}
}

// --- Content hash ---

func TestContentHashDeterministic(t *testing.T) {
	h1 := ContentHashFor("some text")
	h2 := ContentHashFor("some text")
	if h1 != h2 {
		t.Fatal("content hash not deterministic")
	}
}

func TestContentHashPrefix(t *testing.T) {
	h := ContentHashFor("test")
	if !strings.HasPrefix(h, "sha256:") {
		t.Fatalf("expected sha256 prefix, got %q", h)
	}
}

func TestContentHashDiffers(t *testing.T) {
	h1 := ContentHashFor("text a")
	h2 := ContentHashFor("text b")
	if h1 == h2 {
		t.Fatal("different text should have different hash")
	}
}

// --- AtomSpan ---

func TestAtomSpanStringSingleLine(t *testing.T) {
	s := AtomSpan{File: "doc.md", StartLine: 5, EndLine: 5}
	if s.String() != "doc.md:5" {
		t.Fatalf("expected doc.md:5, got %q", s.String())
	}
}

func TestAtomSpanStringRange(t *testing.T) {
	s := AtomSpan{File: "doc.md", StartLine: 5, EndLine: 10}
	if s.String() != "doc.md:5-10" {
		t.Fatalf("expected doc.md:5-10, got %q", s.String())
	}
}

// --- NewContextSafeAtom ---

func TestNewContextSafeAtom(t *testing.T) {
	span := AtomSpan{File: "doc.md", StartLine: 3, EndLine: 5}
	config := AtomSchemaConfig{DocumentID: "test-doc", Profile: "law-regulation", Domain: "legal", SourceFile: "doc.md"}
	atom := NewContextSafeAtom("test-doc#art-1", "article", "Content here.", span, config)

	if !strings.HasPrefix(atom.ID, "atom-") {
		t.Fatalf("expected atom- prefix, got %q", atom.ID)
	}
	if atom.CanonicalRef != "test-doc#art-1" {
		t.Fatalf("expected ref, got %q", atom.CanonicalRef)
	}
	if atom.DocumentID != "test-doc" {
		t.Fatalf("expected doc ID, got %q", atom.DocumentID)
	}
	if atom.Type != "article" {
		t.Fatalf("expected article, got %q", atom.Type)
	}
	if atom.Text != "Content here." {
		t.Fatalf("expected text, got %q", atom.Text)
	}
	if !strings.HasPrefix(atom.ContentHash, "sha256:") {
		t.Fatalf("expected hash, got %q", atom.ContentHash)
	}
	if atom.Profile != "law-regulation" {
		t.Fatalf("expected profile, got %q", atom.Profile)
	}
	if atom.Domain != "legal" {
		t.Fatalf("expected domain, got %q", atom.Domain)
	}
}

// --- BuildContextSafeAtomSet ---

func TestBuildAtomSetFromCNodes(t *testing.T) {
	nodes := []CNode{
		{ID: "doc-root", Kind: KindDocument, Span: Span{1, 10}},
		{ID: "h1", Kind: KindHeading, Text: "Title", Level: 1, Span: Span{1, 1}, ParentID: "doc-root", Hash: "h1"},
		{ID: "p1", Kind: KindParagraph, Text: "Paragraph content here.", Span: Span{3, 5}, ParentID: "h1", Hash: "p1"},
		{ID: "break", Kind: KindThematicBreak, Span: Span{6, 6}},
		{ID: "p2", Kind: KindParagraph, Text: "More content.", Span: Span{7, 8}, ParentID: "h1", Hash: "p2"},
	}
	config := AtomSchemaConfig{DocumentID: "test-doc", Profile: "default", SourceFile: "test.md"}
	set := BuildContextSafeAtomSet(nodes, "sha256:source", config)

	// Should skip document root and thematic break
	if set.AtomCount != 3 {
		t.Fatalf("expected 3 atoms, got %d", set.AtomCount)
	}
	if set.DocumentID != "test-doc" {
		t.Fatalf("expected test-doc, got %q", set.DocumentID)
	}
	if set.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %q", set.SchemaVersion)
	}
	if !strings.HasPrefix(set.SetHash, "sha256:") {
		t.Fatalf("expected set hash, got %q", set.SetHash)
	}
}

func TestBuildAtomSetIDsUnique(t *testing.T) {
	nodes := []CNode{
		{ID: "h1", Kind: KindHeading, Text: "A", Span: Span{1, 1}, Hash: "a"},
		{ID: "h2", Kind: KindHeading, Text: "B", Span: Span{2, 2}, Hash: "b"},
		{ID: "p1", Kind: KindParagraph, Text: "C", Span: Span{3, 3}, Hash: "c"},
	}
	config := AtomSchemaConfig{DocumentID: "doc", SourceFile: "f.md"}
	set := BuildContextSafeAtomSet(nodes, "sha256:s", config)
	seen := map[string]bool{}
	for _, a := range set.Atoms {
		if seen[a.ID] {
			t.Fatalf("duplicate atom ID: %q", a.ID)
		}
		seen[a.ID] = true
	}
}

func TestBuildAtomSetSkipsEmpty(t *testing.T) {
	nodes := []CNode{
		{ID: "h1", Kind: KindHeading, Text: "Title", Span: Span{1, 1}, Hash: "a"},
		{ID: "p1", Kind: KindParagraph, Text: "", RawText: "", Span: Span{2, 2}, Hash: "b"},
	}
	config := AtomSchemaConfig{DocumentID: "doc", SourceFile: "f.md"}
	set := BuildContextSafeAtomSet(nodes, "sha256:s", config)
	if set.AtomCount != 1 {
		t.Fatalf("expected 1 (skip empty), got %d", set.AtomCount)
	}
}

func TestBuildAtomSetDepth(t *testing.T) {
	nodes := []CNode{
		{ID: "root", Kind: KindDocument, Span: Span{1, 10}},
		{ID: "h1", Kind: KindHeading, Text: "Top", Span: Span{1, 1}, ParentID: "root"},
		{ID: "h2", Kind: KindHeading, Text: "Sub", Span: Span{2, 2}, ParentID: "h1"},
		{ID: "p1", Kind: KindParagraph, Text: "Deep", Span: Span{3, 3}, ParentID: "h2"},
	}
	config := AtomSchemaConfig{DocumentID: "doc", SourceFile: "f.md"}
	set := BuildContextSafeAtomSet(nodes, "sha256:s", config)
	// h1: depth 1 (parent=root), h2: depth 2, p1: depth 3
	for _, a := range set.Atoms {
		if a.CanonicalRef == "doc#p1" && a.Depth < 2 {
			t.Fatalf("expected p1 depth >= 2, got %d", a.Depth)
		}
	}
}

// --- Verify ---

func TestVerifyAtomSet(t *testing.T) {
	nodes := []CNode{
		{ID: "h1", Kind: KindHeading, Text: "Title", Span: Span{1, 1}},
	}
	set := BuildContextSafeAtomSet(nodes, "sha256:s", AtomSchemaConfig{DocumentID: "doc", SourceFile: "f.md"})
	if !VerifyAtomSet(set) {
		t.Fatal("expected verification pass")
	}
}

func TestVerifyAtomSetTampered(t *testing.T) {
	nodes := []CNode{
		{ID: "h1", Kind: KindHeading, Text: "Title", Span: Span{1, 1}},
	}
	set := BuildContextSafeAtomSet(nodes, "sha256:s", AtomSchemaConfig{DocumentID: "doc", SourceFile: "f.md"})
	set.Atoms[0].Text = "TAMPERED"
	if VerifyAtomSet(set) {
		t.Fatal("expected verification fail")
	}
}

// --- Set hash determinism ---

func TestSetHashDeterministic(t *testing.T) {
	nodes := []CNode{
		{ID: "h1", Kind: KindHeading, Text: "Title", Span: Span{1, 1}},
		{ID: "p1", Kind: KindParagraph, Text: "Body", Span: Span{2, 3}},
	}
	config := AtomSchemaConfig{DocumentID: "doc", SourceFile: "f.md", Profile: "law"}
	s1 := BuildContextSafeAtomSet(nodes, "sha256:s", config)
	s2 := BuildContextSafeAtomSet(nodes, "sha256:s", config)
	if s1.SetHash != s2.SetHash {
		t.Fatal("set hash not deterministic")
	}
}

// --- Empty ---

func TestBuildAtomSetEmpty(t *testing.T) {
	set := BuildContextSafeAtomSet(nil, "sha256:s", AtomSchemaConfig{DocumentID: "doc"})
	if set.AtomCount != 0 {
		t.Fatalf("expected 0, got %d", set.AtomCount)
	}
	if set.SetHash == "" {
		t.Fatal("expected hash even for empty")
	}
}

// --- JSON roundtrip ---

func TestAtomSetJSON(t *testing.T) {
	nodes := []CNode{
		{ID: "h1", Kind: KindHeading, Text: "Title", Span: Span{1, 1}},
	}
	set := BuildContextSafeAtomSet(nodes, "sha256:s", AtomSchemaConfig{DocumentID: "doc", SourceFile: "f.md"})
	var buf bytes.Buffer
	if err := WriteAtomSetJSON(&buf, set); err != nil {
		t.Fatal(err)
	}
	var decoded ContextSafeAtomSet
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.AtomCount != set.AtomCount {
		t.Fatal("roundtrip mismatch")
	}
	if decoded.SetHash != set.SetHash {
		t.Fatal("hash mismatch")
	}
}

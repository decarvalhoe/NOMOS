package fidelity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func tocSampleHeadings() []HeadingInput {
	return []HeadingInput{
		{ID: "h1", Title: "Introduction", Level: 1, Hash: "sha256:h1"},
		{ID: "h2a", Title: "Background", Level: 2, Hash: "sha256:h2a"},
		{ID: "h2b", Title: "Scope", Level: 2, Hash: "sha256:h2b"},
		{ID: "h3", Title: "In Scope", Level: 3, Hash: "sha256:h3"},
		{ID: "h1b", Title: "Methodology", Level: 1, Hash: "sha256:h1b"},
		{ID: "h2c", Title: "Approach", Level: 2, Hash: "sha256:h2c"},
	}
}

func tocSampleArtifact() TOCArtifact {
	return GenerateTOCFromHeadings(tocSampleHeadings(), "test-doc", "sha256:src", DefaultTOCConfig())
}

// --- Basic generation ---

func TestGenerateTOCEntryCount(t *testing.T) {
	toc := tocSampleArtifact()
	if toc.TotalEntries != 6 {
		t.Fatalf("expected 6 entries, got %d", toc.TotalEntries)
	}
}

func TestGenerateTOCDocumentID(t *testing.T) {
	toc := tocSampleArtifact()
	if toc.DocumentID != "test-doc" {
		t.Fatalf("expected test-doc, got %q", toc.DocumentID)
	}
}

func TestGenerateTOCSchemaVersion(t *testing.T) {
	toc := tocSampleArtifact()
	if toc.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %q", toc.SchemaVersion)
	}
}

func TestGenerateTOCArtifactType(t *testing.T) {
	toc := tocSampleArtifact()
	if toc.ArtifactType != "nomos.toc.v1" {
		t.Fatalf("expected nomos.toc.v1, got %q", toc.ArtifactType)
	}
}

func TestGenerateTOCCertified(t *testing.T) {
	toc := tocSampleArtifact()
	if !toc.Certified {
		t.Fatal("expected certified=true")
	}
}

// --- Entry properties ---

func TestTOCEntryNumbering(t *testing.T) {
	toc := tocSampleArtifact()
	for _, e := range toc.Entries {
		if e.Number == "" {
			t.Fatalf("entry %q has empty number", e.NodeID)
		}
	}
	// First h1 should be "1", second h1 should be "2"
	if toc.Entries[0].Number != "1" {
		t.Fatalf("expected first entry number '1', got %q", toc.Entries[0].Number)
	}
}

func TestTOCEntryDepth(t *testing.T) {
	toc := tocSampleArtifact()
	for _, e := range toc.Entries {
		if e.Depth < 1 {
			t.Fatalf("entry %q has depth %d, expected >= 1", e.NodeID, e.Depth)
		}
	}
}

func TestTOCEntryHashes(t *testing.T) {
	toc := tocSampleArtifact()
	for _, e := range toc.Entries {
		if e.Hash == "" {
			t.Fatalf("entry %q missing hash", e.NodeID)
		}
	}
}

func TestTOCEntryHashesOmittedWhenDisabled(t *testing.T) {
	config := DefaultTOCConfig()
	config.IncludeHashes = false
	toc := GenerateTOCFromHeadings(tocSampleHeadings(), "doc", "sha256:s", config)
	for _, e := range toc.Entries {
		if e.Hash != "" {
			t.Fatalf("entry %q should have no hash when disabled, got %q", e.NodeID, e.Hash)
		}
	}
}

func TestTOCEntryHasChildren(t *testing.T) {
	toc := tocSampleArtifact()
	// "Introduction" (h1) has children (h2a, h2b)
	intro := findTOCEntry(t, toc, "h1")
	if intro.ChildCount == 0 {
		t.Fatal("expected h1 to have children")
	}
	// "In Scope" (h3) has no children
	inScope := findTOCEntry(t, toc, "h3")
	if inScope.ChildCount > 0 {
		t.Fatal("expected h3 to have no children")
	}
}

func TestTOCEntryParentID(t *testing.T) {
	toc := tocSampleArtifact()
	_ = findTOCEntry(t, toc, "h2a")
		t.Fatal("expected h2a to have parent")
	}

// --- Max depth ---

func TestTOCMaxDepth(t *testing.T) {
	toc := tocSampleArtifact()
	if toc.MaxDepth < 3 {
		t.Fatalf("expected max depth >= 3, got %d", toc.MaxDepth)
	}
}

func TestTOCMaxDepthConfig(t *testing.T) {
	config := DefaultTOCConfig()
	config.MaxDepth = 1
	toc := GenerateTOCFromHeadings(tocSampleHeadings(), "doc", "sha256:s", config)
	for _, e := range toc.Entries {
		if e.Depth > 1 {
			t.Fatalf("entry %q at depth %d exceeds max 1", e.NodeID, e.Depth)
		}
	}
	if toc.TotalEntries != 2 {
		t.Fatalf("expected 2 entries at depth 1, got %d", toc.TotalEntries)
	}
}


// --- Artifact hash ---

func TestTOCArtifactHash(t *testing.T) {
	toc := tocSampleArtifact()
	if !strings.HasPrefix(toc.ArtifactHash, "sha256:") {
		t.Fatalf("expected sha256: prefix, got %q", toc.ArtifactHash)
	}
	if len(toc.ArtifactHash) != 7+64 {
		t.Fatalf("expected 71 char hash, got %d", len(toc.ArtifactHash))
	}
}

func TestTOCArtifactHashDeterministic(t *testing.T) {
	h := tocSampleHeadings()
	t1 := GenerateTOCFromHeadings(h, "doc", "sha256:s", DefaultTOCConfig())
	t2 := GenerateTOCFromHeadings(h, "doc", "sha256:s", DefaultTOCConfig())
	if t1.ArtifactHash != t2.ArtifactHash {
		t.Fatal("artifact hash not deterministic")
	}
}

func TestVerifyTOCArtifact(t *testing.T) {
	toc := tocSampleArtifact()
	if !VerifyTOCArtifact(toc) {
		t.Fatal("expected verification to pass")
	}
}

func TestVerifyTOCArtifactTampered(t *testing.T) {
	toc := tocSampleArtifact()
	toc.Entries[0].Title = "TAMPERED"
	if VerifyTOCArtifact(toc) {
		t.Fatal("expected verification to fail on tampered entry")
	}
}

// --- Empty ---

func TestGenerateTOCEmpty(t *testing.T) {
	toc := GenerateTOCFromHeadings(nil, "empty", "sha256:e", DefaultTOCConfig())
	if toc.TotalEntries != 0 {
		t.Fatalf("expected 0, got %d", toc.TotalEntries)
	}
	if toc.ArtifactHash == "" {
		t.Fatal("expected hash even for empty")
	}
}

// --- JSON roundtrip ---

func TestTOCJSON(t *testing.T) {
	toc := tocSampleArtifact()
	var buf bytes.Buffer
	if err := WriteTOCJSON(&buf, toc); err != nil {
		t.Fatal(err)
	}
	var decoded TOCArtifact
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.TotalEntries != toc.TotalEntries {
		t.Fatal("roundtrip mismatch")
	}
	if decoded.ArtifactHash != toc.ArtifactHash {
		t.Fatal("hash mismatch")
	}
}

// --- Markdown output ---

func TestTOCMarkdown(t *testing.T) {
	toc := tocSampleArtifact()
	var buf bytes.Buffer
	WriteTOCMarkdown(&buf, toc)
	md := buf.String()
	if !strings.Contains(md, "# Table of Contents") {
		t.Fatal("expected heading")
	}
	if !strings.Contains(md, "Introduction") {
		t.Fatal("expected entry title")
	}
	if !strings.Contains(md, "Artifact hash:") {
		t.Fatal("expected artifact hash footer")
	}
}


// --- Ordering ---

func TestTOCPreservesOrder(t *testing.T) {
	toc := tocSampleArtifact()
	titles := make([]string, 0, len(toc.Entries))
	for _, e := range toc.Entries {
		titles = append(titles, e.Title)
	}
	expected := []string{"Introduction", "Background", "Scope", "In Scope", "Methodology", "Approach"}
	if len(titles) != len(expected) {
		t.Fatalf("expected %d titles, got %d", len(expected), len(titles))
	}
	for i, title := range expected {
		if titles[i] != title {
			t.Fatalf("at %d: expected %q, got %q", i, title, titles[i])
		}
	}
}

// --- helper ---

func findTOCEntry(t *testing.T, toc TOCArtifact, id string) TOCEntry {
	t.Helper()
	for _, e := range toc.Entries {
		if e.NodeID == id {
			return e
		}
	}
	t.Fatalf("entry %q not found", id)
	return TOCEntry{}
}

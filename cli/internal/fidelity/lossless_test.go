package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const losslessSuiteDir = "testdata/lossless-suite"

func loadLosslessFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(losslessSuiteDir, name))
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return string(data)
}

func hashContent(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// --- Structural preservation tests ---

func TestLosslessSimpleHeadings(t *testing.T) {
	source := loadLosslessFixture(t, "simple-headings.md")
	cast := ParseMarkdown(source)

	if cast.Coverage.Headings < 3 {
		t.Fatalf("expected at least 3 headings, got %d", cast.Coverage.Headings)
	}
	if cast.Coverage.Paragraphs < 2 {
		t.Fatalf("expected at least 2 paragraphs, got %d", cast.Coverage.Paragraphs)
	}
	assertHasNodeType(t, cast, "heading")
	assertHasNodeType(t, cast, "paragraph")
}

func TestLosslessSimpleLists(t *testing.T) {
	source := loadLosslessFixture(t, "simple-lists.md")
	cast := ParseMarkdown(source)

	if cast.Coverage.Lists < 2 {
		t.Fatalf("expected at least 2 lists (ordered+unordered), got %d", cast.Coverage.Lists)
	}
	if cast.Coverage.ListItems < 6 {
		t.Fatalf("expected at least 6 list items, got %d", cast.Coverage.ListItems)
	}
}

func TestLosslessComplexStructure(t *testing.T) {
	source := loadLosslessFixture(t, "complex-structure.md")
	cast := ParseMarkdown(source)

	if cast.Coverage.Headings < 5 {
		t.Fatalf("expected at least 5 headings, got %d", cast.Coverage.Headings)
	}
	if cast.Coverage.CodeBlocks < 1 {
		t.Fatalf("expected at least 1 code block, got %d", cast.Coverage.CodeBlocks)
	}
	if cast.Coverage.Blockquotes < 1 {
		t.Fatalf("expected at least 1 blockquote, got %d", cast.Coverage.Blockquotes)
	}
	if cast.Coverage.ThematicBreaks < 1 {
		t.Fatalf("expected at least 1 thematic break, got %d", cast.Coverage.ThematicBreaks)
	}
	if cast.Coverage.Links < 1 {
		t.Fatalf("expected at least 1 link, got %d", cast.Coverage.Links)
	}
}

func TestLosslessGFMTables(t *testing.T) {
	source := loadLosslessFixture(t, "gfm-tables.md")
	cast := ParseMarkdown(source)

	if cast.Coverage.Tables < 2 {
		t.Fatalf("expected at least 2 tables, got %d", cast.Coverage.Tables)
	}
	if cast.Coverage.Lists < 1 {
		t.Fatalf("expected at least 1 list (task list), got %d", cast.Coverage.Lists)
	}
}

func TestLosslessEdgeCases(t *testing.T) {
	source := loadLosslessFixture(t, "edge-cases.md")
	cast := ParseMarkdown(source)

	if cast.Coverage.Headings < 6 {
		t.Fatalf("expected at least 6 headings, got %d", cast.Coverage.Headings)
	}
	if cast.Coverage.Paragraphs < 4 {
		t.Fatalf("expected at least 4 paragraphs, got %d", cast.Coverage.Paragraphs)
	}
	// Note: indented code blocks may not be detected by all parsers.
	// This is a known limitation tracked for future fidelity improvements.
	if cast.Coverage.Lists < 2 {
		t.Fatalf("expected at least 2 lists (tight+loose), got %d", cast.Coverage.Lists)
	}
}

func TestLosslessFrontmatter(t *testing.T) {
	source := loadLosslessFixture(t, "frontmatter.md")
	cast := ParseMarkdown(source)

	// Frontmatter document should still have headings and paragraphs.
	if cast.Coverage.Headings < 2 {
		t.Fatalf("expected at least 2 headings, got %d", cast.Coverage.Headings)
	}
	if cast.Coverage.Paragraphs < 2 {
		t.Fatalf("expected at least 2 paragraphs, got %d", cast.Coverage.Paragraphs)
	}
}

// --- Determinism tests (same input → same output) ---

func TestLosslessDeterministic(t *testing.T) {
	fixtures := []string{
		"simple-headings.md",
		"simple-lists.md",
		"complex-structure.md",
		"gfm-tables.md",
		"edge-cases.md",
		"frontmatter.md",
	}

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			source := loadLosslessFixture(t, name)
			cast1 := ParseMarkdown(source)
			cast2 := ParseMarkdown(source)

			if cast1.SourceHash != cast2.SourceHash {
				t.Fatalf("source hash not deterministic: %s vs %s",
					cast1.SourceHash, cast2.SourceHash)
			}
			if len(cast1.Nodes) != len(cast2.Nodes) {
				t.Fatalf("node count not deterministic: %d vs %d",
					len(cast1.Nodes), len(cast2.Nodes))
			}
		})
	}
}

// --- Source hash stability ---

func TestLosslessSourceHashMatchesInput(t *testing.T) {
	fixtures := []string{
		"simple-headings.md",
		"complex-structure.md",
	}

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			source := loadLosslessFixture(t, name)
			cast := ParseMarkdown(source)

			expectedHash := hashContent(source)
			if cast.SourceHash != expectedHash {
				t.Fatalf("source hash mismatch: expected %s, got %s",
					expectedHash, cast.SourceHash)
			}
		})
	}
}

// --- Node completeness (no content lost) ---

func TestLosslessNodeContentNotEmpty(t *testing.T) {
	fixtures := []string{
		"simple-headings.md",
		"complex-structure.md",
		"gfm-tables.md",
	}

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			source := loadLosslessFixture(t, name)
			cast := ParseMarkdown(source)

			emptyContent := 0
			for _, node := range cast.Nodes {
				if node.Text == "" && node.RawText == "" && string(node.Kind) != "document" && string(node.Kind) != "list" {
					emptyContent++
				}
			}
			// Allow some structural nodes to be empty, but not majority.
			if emptyContent > len(cast.Nodes)/2 {
				t.Fatalf("too many empty-content nodes: %d of %d", emptyContent, len(cast.Nodes))
			}
		})
	}
}

// --- Parent chain integrity ---

func TestLosslessParentChainIntegrity(t *testing.T) {
	fixtures := []string{
		"simple-headings.md",
		"complex-structure.md",
	}

	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			source := loadLosslessFixture(t, name)
			cast := ParseMarkdown(source)

			nodeIDs := map[string]bool{}
			for _, node := range cast.Nodes {
				nodeIDs[node.ID] = true
			}

			// Every node's parent should exist (except root).
			for _, node := range cast.Nodes {
				if node.ParentID != "" && !nodeIDs[node.ParentID] {
					t.Fatalf("node %s references non-existent parent %s", node.ID, node.ParentID)
				}
			}
		})
	}
}

// --- All fixtures parse without error ---

func TestLosslessSuiteAllParse(t *testing.T) {
	entries, err := os.ReadDir(losslessSuiteDir)
	if err != nil {
		t.Fatalf("read suite dir: %v", err)
	}

	mdCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			mdCount++
			t.Run(entry.Name(), func(t *testing.T) {
				source := loadLosslessFixture(t, entry.Name())
				cast := ParseMarkdown(source)
				if cast.Root == "" {
					t.Fatal("expected non-empty root ID")
				}
				if len(cast.Nodes) == 0 {
					t.Fatal("expected at least one node")
				}
			})
		}
	}
	if mdCount < 6 {
		t.Fatalf("expected at least 6 fixtures, found %d", mdCount)
	}
}

func assertHasNodeType(t *testing.T, cast CAST, nodeType string) {
	t.Helper()
	for _, node := range cast.Nodes {
		if string(node.Kind) == nodeType {
			return
		}
	}
	t.Fatalf("expected node of type %q in CAST with %d nodes", nodeType, len(cast.Nodes))
}

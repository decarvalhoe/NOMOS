package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// TOCArtifact is the certified Table of Contents document.
type TOCArtifact struct {
	SchemaVersion string     `json:"schema_version"`
	ArtifactType  string     `json:"artifact_type"`
	GeneratedAt   string     `json:"generated_at"`
	DocumentID    string     `json:"document_id"`
	SourceHash    string     `json:"source_hash"`
	TreeHash      string     `json:"tree_hash"`
	TotalEntries  int        `json:"total_entries"`
	MaxDepth      int        `json:"max_depth"`
	Certified     bool       `json:"certified"`
	Entries       []TOCEntry `json:"entries"`
	ArtifactHash  string     `json:"artifact_hash"`
}

// TOCEntry is a single line in the certified TOC.
type TOCEntry struct {
	ID       string `json:"id"`
	Number   string `json:"number"`
	Title    string `json:"title"`
	Depth    int    `json:"depth"`
	Hash     string `json:"hash"`
	ParentID string `json:"parent_id,omitempty"`
	HasChildren bool `json:"has_children"`
	PageRef  string `json:"page_ref,omitempty"`
}

// TOCGeneratorConfig controls TOC generation.
type TOCGeneratorConfig struct {
	MaxDepth      int    // 0 = unlimited
	IncludeHashes bool   // include per-entry hashes
	PageRefs      map[string]string // id → page reference (optional)
}

// DefaultTOCConfig returns standard config.
func DefaultTOCConfig() TOCGeneratorConfig {
	return TOCGeneratorConfig{
		MaxDepth:      0,
		IncludeHashes: true,
	}
}

// GenerateTOC produces a certified TOC artifact from a StructureTree.
func GenerateTOC(tree StructureTree, config TOCGeneratorConfig) TOCArtifact {
	entries := flattenToEntries(tree.Root, config, 0)

	artifact := TOCArtifact{
		SchemaVersion: "0.1.0",
		ArtifactType:  "nomos.toc.v1",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		DocumentID:    tree.DocumentID,
		SourceHash:    tree.SourceHash,
		TreeHash:      tree.TreeHash,
		TotalEntries:  len(entries),
		MaxDepth:      maxEntryDepth(entries),
		Certified:     tree.Verify(),
		Entries:       entries,
	}

	artifact.ArtifactHash = computeArtifactHash(artifact)
	return artifact
}

// GenerateTOCFromHeadings is a convenience that builds a tree then generates TOC.
func GenerateTOCFromHeadings(headings []HeadingInput, docID string, sourceHash string, config TOCGeneratorConfig) TOCArtifact {
	tree := BuildStructureTree(headings, TreeConfig{
		DocumentID: docID,
		SourceHash: sourceHash,
	})
	return GenerateTOC(tree, config)
}

// WriteTOCJSON serializes the artifact.
func WriteTOCJSON(w io.Writer, toc TOCArtifact) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(toc)
}

// WriteTOCMarkdown renders the TOC as a Markdown document.
func WriteTOCMarkdown(w io.Writer, toc TOCArtifact) error {
	fmt.Fprintf(w, "# Table of Contents\n\n")
	fmt.Fprintf(w, "Document: `%s`  \n", toc.DocumentID)
	fmt.Fprintf(w, "Source hash: `%s`  \n", toc.SourceHash)
	fmt.Fprintf(w, "Tree hash: `%s`  \n", toc.TreeHash)
	fmt.Fprintf(w, "Entries: %d  \n", toc.TotalEntries)
	fmt.Fprintf(w, "Certified: %v  \n\n", toc.Certified)

	for _, e := range toc.Entries {
		indent := strings.Repeat("  ", e.Depth)
		fmt.Fprintf(w, "%s- **%s** %s", indent, e.Number, e.Title)
		if e.PageRef != "" {
			fmt.Fprintf(w, " _(p. %s)_", e.PageRef)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "\n---\nArtifact hash: `%s`\n", toc.ArtifactHash)
	return nil
}

// VerifyTOCArtifact checks that the artifact hash matches its contents.
func VerifyTOCArtifact(toc TOCArtifact) bool {
	stored := toc.ArtifactHash
	toc.ArtifactHash = ""
	computed := computeArtifactHash(toc)
	return stored == computed
}

func flattenToEntries(node *TOCNode, config TOCGeneratorConfig, depth int) []TOCEntry {
	if node == nil {
		return nil
	}

	var entries []TOCEntry

	// Skip root node (depth 0, "Document Root"), include its children.
	if depth > 0 {
		if config.MaxDepth > 0 && depth > config.MaxDepth {
			return nil
		}

		entry := TOCEntry{
			ID:          node.ID,
			Number:      node.Number,
			Title:       node.Title,
			Depth:       node.Depth,
			ParentID:    node.ParentID,
			HasChildren: len(node.Children) > 0,
		}

		if config.IncludeHashes {
			entry.Hash = node.Hash
		}

		if config.PageRefs != nil {
			entry.PageRef = config.PageRefs[node.ID]
		}

		entries = append(entries, entry)
	}

	for _, child := range node.Children {
		entries = append(entries, flattenToEntries(child, config, depth+1)...)
	}

	return entries
}

func maxEntryDepth(entries []TOCEntry) int {
	max := 0
	for _, e := range entries {
		if e.Depth > max {
			max = e.Depth
		}
	}
	return max
}

func computeArtifactHash(toc TOCArtifact) string {
	h := sha256.New()
	h.Write([]byte(toc.DocumentID))
	h.Write([]byte(toc.SourceHash))
	h.Write([]byte(toc.TreeHash))
	for _, e := range toc.Entries {
		h.Write([]byte(e.ID))
		h.Write([]byte(e.Number))
		h.Write([]byte(e.Title))
		h.Write([]byte(fmt.Sprintf("%d", e.Depth)))
		h.Write([]byte(e.Hash))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// TOCNode represents one entry in the certified table of contents.
type TOCNode struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Depth     int        `json:"depth"`
	Number    string     `json:"number"`
	Hash      string     `json:"hash"`
	ParentID  string     `json:"parent_id,omitempty"`
	Children  []*TOCNode `json:"children,omitempty"`
	CrossRefs []string   `json:"cross_refs,omitempty"`
}

// StructureTree is the certified structure of a document.
type StructureTree struct {
	DocumentID string     `json:"document_id"`
	SourceHash string     `json:"source_hash"`
	Root       *TOCNode   `json:"root"`
	TotalNodes int        `json:"total_nodes"`
	MaxDepth   int        `json:"max_depth"`
	TreeHash   string     `json:"tree_hash"`
}

// HeadingInput is a flat heading extracted from a source document.
type HeadingInput struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Level     int      `json:"level"` // 1-6
	Hash      string   `json:"hash"`
	CrossRefs []string `json:"cross_refs,omitempty"`
}

// TreeConfig controls structure tree generation.
type TreeConfig struct {
	DocumentID string
	SourceHash string
}

// BuildStructureTree constructs a certified structure tree from flat headings.
// Headings must be in document order. The tree respects nesting: a heading at
// level N becomes a child of the nearest preceding heading at level N-1.
func BuildStructureTree(headings []HeadingInput, config TreeConfig) StructureTree {
	root := &TOCNode{
		ID:    config.DocumentID + ".ROOT",
		Title: "Document Root",
		Depth: 0,
	}

	// Stack tracks the current parent at each depth level.
	stack := make([]*TOCNode, 7) // levels 0-6
	stack[0] = root

	counters := make([]int, 7) // numbering counters per level
	maxDepth := 0
	totalNodes := 1 // root

	for _, h := range headings {
		level := h.Level
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}

		// Reset deeper counters when we go up.
		for i := level + 1; i <= 6; i++ {
			counters[i] = 0
		}
		counters[level]++

		// Build numbering string.
		number := buildNumber(counters, level)

		// Find parent: nearest stack entry at level-1.
		parent := findParent(stack, level)
		if parent == nil {
			parent = root
		}

		node := &TOCNode{
			ID:        h.ID,
			Title:     h.Title,
			Depth:     level,
			Number:    number,
			Hash:      h.Hash,
			ParentID:  parent.ID,
			CrossRefs: h.CrossRefs,
		}

		parent.Children = append(parent.Children, node)
		stack[level] = node
		totalNodes++

		if level > maxDepth {
			maxDepth = level
		}
	}

	// Compute tree hash from children (excludes root.Hash to avoid circular dependency).
	treeHash := computeTreeHash(root)

	return StructureTree{
		DocumentID: config.DocumentID,
		SourceHash: config.SourceHash,
		Root:       root,
		TotalNodes: totalNodes,
		MaxDepth:   maxDepth,
		TreeHash:   treeHash,
	}
}

// FlatTOC returns the tree as a flat ordered slice (depth-first).
func (t StructureTree) FlatTOC() []TOCNode {
	var result []TOCNode
	flattenNode(t.Root, &result)
	return result
}

// Verify checks that the tree hash matches the computed hash.
func (t StructureTree) Verify() bool {
	computed := computeTreeHash(t.Root)
	return computed == t.TreeHash
}

// FindByID searches the tree for a node with the given ID.
func (t StructureTree) FindByID(id string) *TOCNode {
	return searchNode(t.Root, id)
}

// CrossRefIndex builds a map of cross-reference target → source nodes.
func (t StructureTree) CrossRefIndex() map[string][]string {
	index := make(map[string][]string)
	collectCrossRefs(t.Root, index)
	// Sort for determinism.
	for k := range index {
		sort.Strings(index[k])
	}
	return index
}

func buildNumber(counters []int, level int) string {
	parts := make([]string, 0, level)
	for i := 1; i <= level; i++ {
		parts = append(parts, fmt.Sprintf("%d", counters[i]))
	}
	return strings.Join(parts, ".")
}

func findParent(stack []*TOCNode, level int) *TOCNode {
	for i := level - 1; i >= 0; i-- {
		if stack[i] != nil {
			return stack[i]
		}
	}
	return nil
}

func computeTreeHash(node *TOCNode) string {
	var sb strings.Builder
	hashNode(node, &sb)
	h := sha256.Sum256([]byte(sb.String()))
	return "sha256:" + hex.EncodeToString(h[:])
}

func hashNode(node *TOCNode, sb *strings.Builder) {
	sb.WriteString(node.ID)
	sb.WriteByte('|')
	sb.WriteString(node.Title)
	sb.WriteByte('|')
	sb.WriteString(fmt.Sprintf("%d", node.Depth))
	sb.WriteByte('|')
	sb.WriteString(node.Number)
	sb.WriteByte('|')
	sb.WriteString(node.Hash)
	sb.WriteByte('\n')
	for _, child := range node.Children {
		hashNode(child, sb)
	}
}

func flattenNode(node *TOCNode, result *[]TOCNode) {
	if node == nil {
		return
	}
	*result = append(*result, *node)
	for _, child := range node.Children {
		flattenNode(child, result)
	}
}

func searchNode(node *TOCNode, id string) *TOCNode {
	if node == nil {
		return nil
	}
	if node.ID == id {
		return node
	}
	for _, child := range node.Children {
		if found := searchNode(child, id); found != nil {
			return found
		}
	}
	return nil
}

func collectCrossRefs(node *TOCNode, index map[string][]string) {
	if node == nil {
		return
	}
	for _, ref := range node.CrossRefs {
		index[ref] = append(index[ref], node.ID)
	}
	for _, child := range node.Children {
		collectCrossRefs(child, index)
	}
}

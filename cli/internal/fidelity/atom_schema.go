package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ContextSafeAtom is an atom with a deterministic ID derived from its
// canonical reference and source span. The ID is stable across re-parses
// as long as the source content at that location does not change.
type ContextSafeAtom struct {
	ID           string            `json:"id"`
	CanonicalRef string            `json:"canonical_ref"`
	DocumentID   string            `json:"document_id"`
	Type         string            `json:"type"`
	Title        string            `json:"title,omitempty"`
	Text         string            `json:"text"`
	ContentHash  string            `json:"content_hash"`
	Span         AtomSpan          `json:"span"`
	Depth        int               `json:"depth"`
	ParentID     string            `json:"parent_id,omitempty"`
	Profile      string            `json:"profile"`
	Domain       string            `json:"domain,omitempty"`
	Props        map[string]string `json:"props,omitempty"`
}

// AtomSpan locates an atom in its source with file, line, and column.
type AtomSpan struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	StartCol  int    `json:"start_col,omitempty"`
	EndCol    int    `json:"end_col,omitempty"`
}

// String returns file:startLine-endLine.
func (s AtomSpan) String() string {
	if s.StartLine == s.EndLine {
		return fmt.Sprintf("%s:%d", s.File, s.StartLine)
	}
	return fmt.Sprintf("%s:%d-%d", s.File, s.StartLine, s.EndLine)
}

// AtomSchemaConfig controls context-safe atom generation.
type AtomSchemaConfig struct {
	DocumentID string
	Profile    string
	Domain     string
	SourceFile string
}

// DeterministicID computes a stable atom ID from canonical ref + source span.
// The ID is a truncated SHA-256 hash ensuring uniqueness within a document
// and stability across re-parses of unchanged content.
func DeterministicID(canonicalRef string, span AtomSpan) string {
	input := fmt.Sprintf("%s|%s|%d|%d", canonicalRef, span.File, span.StartLine, span.EndLine)
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("atom-%s", hex.EncodeToString(h[:8]))
}

// ContentHashFor computes the content hash for an atom's text.
func ContentHashFor(text string) string {
	h := sha256.Sum256([]byte(text))
	return "sha256:" + hex.EncodeToString(h[:])
}

// NewContextSafeAtom creates a context-safe atom with deterministic ID.
func NewContextSafeAtom(canonicalRef string, atomType string, text string, span AtomSpan, config AtomSchemaConfig) ContextSafeAtom {
	return ContextSafeAtom{
		ID:           DeterministicID(canonicalRef, span),
		CanonicalRef: canonicalRef,
		DocumentID:   config.DocumentID,
		Type:         atomType,
		Text:         text,
		ContentHash:  ContentHashFor(text),
		Span:         span,
		Profile:      config.Profile,
		Domain:       config.Domain,
	}
}

// ContextSafeAtomSet is the output of context-safe atomization.
type ContextSafeAtomSet struct {
	SchemaVersion string            `json:"schema_version"`
	DocumentID    string            `json:"document_id"`
	SourceFile    string            `json:"source_file"`
	SourceHash    string            `json:"source_hash"`
	Profile       string            `json:"profile"`
	AtomCount     int               `json:"atom_count"`
	SetHash       string            `json:"set_hash"`
	Atoms         []ContextSafeAtom `json:"atoms"`
}

// BuildContextSafeAtomSet converts CNodes into a ContextSafeAtomSet.
func BuildContextSafeAtomSet(nodes []CNode, sourceHash string, config AtomSchemaConfig) ContextSafeAtomSet {
	var atoms []ContextSafeAtom
	for _, node := range nodes {
		if node.Kind == KindDocument || node.Kind == KindThematicBreak {
			continue
		}
		if strings.TrimSpace(node.Text) == "" && strings.TrimSpace(node.RawText) == "" {
			continue
		}

		canonicalRef := fmt.Sprintf("%s#%s", config.DocumentID, node.ID)
		text := node.Text
		if text == "" {
			text = node.RawText
		}

		span := AtomSpan{
			File:      config.SourceFile,
			StartLine: node.Span.StartLine,
			EndLine:   node.Span.EndLine,
		}

		atom := NewContextSafeAtom(canonicalRef, string(node.Kind), text, span, config)
		atom.Title = extractAtomTitle(node)
		atom.Depth = nodeDepth(node, nodes)
		atom.ParentID = parentAtomID(node, config)
		atom.Props = node.Props

		atoms = append(atoms, atom)
	}

	set := ContextSafeAtomSet{
		SchemaVersion: "0.1.0",
		DocumentID:    config.DocumentID,
		SourceFile:    config.SourceFile,
		SourceHash:    sourceHash,
		Profile:       config.Profile,
		AtomCount:     len(atoms),
		Atoms:         atoms,
	}
	set.SetHash = computeSetHash(set)
	return set
}

// VerifyAtomSet checks that the set hash matches its contents and
// that each atom's content hash matches its text.
func VerifyAtomSet(set ContextSafeAtomSet) bool {
	stored := set.SetHash
	set.SetHash = ""
	if stored != computeSetHash(set) {
		return false
	}
	for _, atom := range set.Atoms {
		if atom.ContentHash != ContentHashFor(atom.Text) {
			return false
		}
	}
	return true
}

// WriteAtomSetJSON serializes the atom set.
func WriteAtomSetJSON(w io.Writer, set ContextSafeAtomSet) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(set)
}

func extractAtomTitle(node CNode) string {
	if node.Kind == KindHeading {
		return node.Text
	}
	text := node.Text
	if len(text) > 80 {
		text = text[:80] + "..."
	}
	return text
}

func nodeDepth(node CNode, nodes []CNode) int {
	depth := 0
	current := node.ParentID
	visited := map[string]bool{node.ID: true}
	for current != "" && !visited[current] {
		visited[current] = true
		depth++
		for _, n := range nodes {
			if n.ID == current {
				current = n.ParentID
				break
			}
		}
	}
	return depth
}

func parentAtomID(node CNode, config AtomSchemaConfig) string {
	if node.ParentID == "" {
		return ""
	}
	parentRef := fmt.Sprintf("%s#%s", config.DocumentID, node.ParentID)
	parentSpan := AtomSpan{File: config.SourceFile}
	return DeterministicID(parentRef, parentSpan)
}

func computeSetHash(set ContextSafeAtomSet) string {
	h := sha256.New()
	h.Write([]byte(set.DocumentID))
	h.Write([]byte(set.SourceHash))
	h.Write([]byte(set.Profile))
	for _, atom := range set.Atoms {
		h.Write([]byte(atom.ID))
		h.Write([]byte(atom.ContentHash))
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

package atomization

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// LawbookAtomResult holds the output of lawbook atomization.
type LawbookAtomResult struct {
	DocumentID string              `json:"document_id"`
	SourceFile string              `json:"source_file"`
	SourceHash string              `json:"source_hash"`
	TotalAtoms int                 `json:"total_atoms"`
	Atoms      []Atom              `json:"atoms"`
	ParentMap  map[string][]string `json:"parent_map"`
}

// LawbookEngineConfig controls atomization behavior.
type LawbookEngineConfig struct {
	DocumentID string // Document identifier prefix
	SourceFile string // Source file path for span tracking
	Domain     string // Business domain
	MinDepth   int    // Minimum heading level to emit (0 = all)
	MaxDepth   int    // Maximum heading level to emit (0 = all)
}

// DefaultEngineConfig returns sensible defaults.
func DefaultEngineConfig() LawbookEngineConfig {
	return LawbookEngineConfig{
		DocumentID: "DOC",
		SourceFile: "",
		Domain:     "unknown",
		MinDepth:   0,
		MaxDepth:   0,
	}
}

// AtomizeLawbook takes a parsed Markdown AST and produces structured atoms
// with source spans, content hashes, and parent chain linkage.
func AtomizeLawbook(ast AST, config LawbookEngineConfig) LawbookAtomResult {
	result := LawbookAtomResult{
		DocumentID: config.DocumentID,
		SourceFile: config.SourceFile,
		SourceHash: ast.SourceHash,
		ParentMap:  make(map[string][]string),
	}

	for _, block := range ast.Blocks {
		if block.Type == BlockBlankLine {
			continue
		}
		if shouldFilterBlock(block, config) {
			continue
		}

		atom := blockToAtom(block, config)
		result.Atoms = append(result.Atoms, atom)

		if atom.ParentID != "" {
			result.ParentMap[atom.ParentID] = append(result.ParentMap[atom.ParentID], atom.ID)
		}
	}

	result.TotalAtoms = len(result.Atoms)
	return result
}

// AtomizeSource is a convenience function that parses Markdown and atomizes in one step.
func AtomizeSource(source string, config LawbookEngineConfig) LawbookAtomResult {
	ast := ParseMarkdown(source)
	return AtomizeLawbook(ast, config)
}

func blockToAtom(block Block, config LawbookEngineConfig) Atom {
	atomID := buildAtomID(config.DocumentID, block)
	canonicalRef := buildCanonicalRef(block)
	depth := effectiveDepth(block)
	atomType := mapBlockTypeToAtomType(block.Type)

	return Atom{
		ID:           atomID,
		CanonicalRef: canonicalRef,
		Title:        buildDisplayRef(block),
		Text:         block.Text,
		ContentHash:  ensureAtomHash(block.Hash, block.Text),
		Type:         atomType,
		Domain:       config.Domain,
		Depth:        depth,
		ParentID:     block.ParentID,
		BlockID:      block.ID,
		ReviewState:  ReviewDraft,
		SourceSpan: SourceSpan{
			File:      config.SourceFile,
			StartLine: block.Span.StartLine,
			EndLine:   block.Span.EndLine,
		},
	}
}

func buildAtomID(docID string, block Block) string {
	prefix := strings.ToUpper(docID)
	suffix := strings.ToUpper(block.ID)
	return prefix + "." + suffix
}

func buildCanonicalRef(block Block) string {
	switch block.Type {
	case BlockHeading:
		return slugCanonical(block.Text)
	case BlockDocument:
		return "root"
	default:
		return block.ID
	}
}

func buildDisplayRef(block Block) string {
	if block.Type == BlockHeading {
		return block.Text
	}
	return ""
}

func effectiveDepth(block Block) int {
	if block.Type == BlockHeading {
		return block.Level
	}
	if block.Type == BlockDocument {
		return 0
	}
	return block.Level
}

func shouldFilterBlock(block Block, config LawbookEngineConfig) bool {
	if block.Type == BlockDocument {
		return false
	}
	depth := effectiveDepth(block)
	if config.MinDepth > 0 && depth < config.MinDepth {
		return true
	}
	if config.MaxDepth > 0 && depth > config.MaxDepth {
		return true
	}
	return false
}

func mapBlockTypeToAtomType(bt BlockType) AtomType {
	switch bt {
	case BlockHeading:
		return AtomClause
	case BlockParagraph:
		return AtomRule
	case BlockList, BlockListItem:
		return AtomListItem
	case BlockCodeBlock:
		return AtomCodeBlock
	case BlockTable, BlockTableRow:
		return AtomTable
	case BlockMetadata:
		return AtomMeta
	case BlockDocument:
		return AtomMeta
	default:
		return AtomRule
	}
}

func ensureAtomHash(existing, content string) string {
	if existing != "" {
		return existing
	}
	h := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(h[:])
}

func slugCanonical(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	lastDash := false
	for _, r := range text {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if len(result) > 64 {
		h := sha256.Sum256([]byte(text))
		result = result[:56] + "-" + fmt.Sprintf("%x", h[:4])
	}
	return result
}

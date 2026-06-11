package atomization

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// ReviewState tracks the governance lifecycle of an atom.
type ReviewState string

const (
	ReviewDraft    ReviewState = "draft"
	ReviewPending  ReviewState = "pending"
	ReviewApproved ReviewState = "approved"
	ReviewRejected ReviewState = "rejected"
	ReviewAmended  ReviewState = "amended"
)

// IsValid returns true if the review state is recognized.
func (s ReviewState) IsValid() bool {
	switch s {
	case ReviewDraft, ReviewPending, ReviewApproved, ReviewRejected, ReviewAmended:
		return true
	}
	return false
}

// AtomType classifies the semantic role of an atom.
type AtomType string

const (
	AtomRule      AtomType = "rule"
	AtomClause    AtomType = "clause"
	AtomDefinition AtomType = "definition"
	AtomListItem  AtomType = "list_item"
	AtomCodeBlock AtomType = "code_block"
	AtomTable     AtomType = "table"
	AtomMeta      AtomType = "meta"
)

// SourceSpan locates an atom in its source file.
type SourceSpan struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	StartCol  int    `json:"start_col"`
	EndCol    int    `json:"end_col"`
}

// String returns file:line:col format.
func (s SourceSpan) String() string {
	return fmt.Sprintf("%s:%d:%d", s.File, s.StartLine, s.StartCol)
}

// Atom is the smallest traceable unit extracted from a source document.
type Atom struct {
	ID           string      `json:"id"`
	CanonicalRef string      `json:"canonical_ref"`
	Type         AtomType    `json:"type"`
	Title        string      `json:"title,omitempty"`
	Text         string      `json:"text"`
	ContentHash  string      `json:"content_hash"`
	SourceSpan   SourceSpan  `json:"source_span"`
	ParentID     string      `json:"parent_id,omitempty"`
	BlockID      string      `json:"block_id"`
	Depth        int         `json:"depth"`
	ReviewState  ReviewState `json:"review_state"`
	Domain       string      `json:"domain,omitempty"`

	// Facets is the CKM-01 multidimensional classification of this atom. It is
	// additive and opt-in: nil unless atomization is asked to emit facets, so
	// existing atom output stays byte-identical (doctrine §2.1, zero regression).
	Facets *Facets `json:"facets,omitempty"`
}

// AtomSet is the output of atomization for a single document.
type AtomSet struct {
	DocumentRef string `json:"document_ref"`
	SourceFile  string `json:"source_file"`
	SourceHash  string `json:"source_hash"`
	AtomCount   int    `json:"atom_count"`
	Atoms       []Atom `json:"atoms"`
}

// AtomizeOptions configures atom generation.
type AtomizeOptions struct {
	DocumentRef  string
	SourceFile   string
	Domain       string
	DefaultState ReviewState

	// EmitFacets, when true, makes atomization derive and attach CKM-01 facets
	// to each atom (DeriveFacets). It defaults to false so existing callers and
	// golden outputs are unaffected.
	EmitFacets bool
}

// Atomize converts a block AST into a set of atoms.
// Each content-bearing block (heading, paragraph, list_item, code_block,
// table, metadata) produces one atom with a stable ID derived from its
// canonical reference.
func Atomize(ast AST, opts AtomizeOptions) AtomSet {
	defaultState := opts.DefaultState
	if !defaultState.IsValid() {
		defaultState = ReviewDraft
	}

	result := AtomSet{
		DocumentRef: opts.DocumentRef,
		SourceFile:  opts.SourceFile,
		SourceHash:  ast.SourceHash,
	}

	// Map block IDs to their atom IDs for parent resolution.
	blockToAtom := map[string]string{}

	// Track heading hierarchy for canonical ref construction.
	headingPath := map[int]string{} // level -> slug

	for _, blk := range ast.Blocks {
		if blk.Type == BlockDocument || blk.Type == BlockBlankLine {
			continue
		}

		atomType := blockTypeToAtomType(blk.Type)
		text := blk.Text
		title := ""

		if blk.Type == BlockHeading {
			title = text
			headingPath[blk.Level] = slugify(text)
			// Clear deeper levels.
			for l := blk.Level + 1; l <= 6; l++ {
				delete(headingPath, l)
			}
		}

		canonicalRef := buildAtomCanonicalRef(opts.DocumentRef, headingPath, blk)
		atomID := stableAtomID(canonicalRef)

		parentAtomID := ""
		if blk.ParentID != "" {
			if mapped, ok := blockToAtom[blk.ParentID]; ok {
				parentAtomID = mapped
			}
		}

		atom := Atom{
			ID:           atomID,
			CanonicalRef: canonicalRef,
			Type:         atomType,
			Title:        title,
			Text:         text,
			ContentHash:  hashContent(text),
			SourceSpan: SourceSpan{
				File:      opts.SourceFile,
				StartLine: blk.Span.StartLine,
				EndLine:   blk.Span.EndLine,
				StartCol:  1,
				EndCol:    len(lastLine(blk.RawText)) + 1,
			},
			ParentID:    parentAtomID,
			BlockID:     blk.ID,
			Depth:       blockDepth(blk),
			ReviewState: defaultState,
			Domain:      opts.Domain,
		}

		if opts.EmitFacets {
			f := DeriveFacets(atom)
			atom.Facets = &f
		}

		blockToAtom[blk.ID] = atomID
		result.Atoms = append(result.Atoms, atom)
	}

	result.AtomCount = len(result.Atoms)
	return result
}

func blockTypeToAtomType(bt BlockType) AtomType {
	switch bt {
	case BlockHeading:
		return AtomClause
	case BlockParagraph:
		return AtomRule
	case BlockList:
		return AtomRule
	case BlockListItem:
		return AtomListItem
	case BlockCodeBlock:
		return AtomCodeBlock
	case BlockTable, BlockTableRow:
		return AtomTable
	case BlockMetadata:
		return AtomMeta
	default:
		return AtomRule
	}
}

func buildAtomCanonicalRef(docRef string, headingPath map[int]string, blk Block) string {
	var parts []string
	if docRef != "" {
		parts = append(parts, docRef)
	}

	// Build path from heading hierarchy.
	for l := 1; l <= 6; l++ {
		slug, ok := headingPath[l]
		if !ok {
			break
		}
		parts = append(parts, slug)
	}

	// Add block-specific suffix.
	switch blk.Type {
	case BlockHeading:
		// Heading slug is already in the path.
	case BlockParagraph:
		parts = append(parts, fmt.Sprintf("p-%d", blk.Span.StartLine))
	case BlockList:
		parts = append(parts, fmt.Sprintf("list-%d", blk.Span.StartLine))
	case BlockListItem:
		parts = append(parts, fmt.Sprintf("item-%d", blk.Span.StartLine))
	case BlockCodeBlock:
		parts = append(parts, fmt.Sprintf("code-%d", blk.Span.StartLine))
	case BlockTable:
		parts = append(parts, fmt.Sprintf("table-%d", blk.Span.StartLine))
	case BlockTableRow:
		parts = append(parts, fmt.Sprintf("row-%d", blk.Span.StartLine))
	case BlockMetadata:
		parts = append(parts, "meta")
	}

	return strings.Join(parts, "/")
}

func blockDepth(blk Block) int {
	if blk.Type == BlockHeading {
		return blk.Level
	}
	// Non-heading blocks sit below their heading parent.
	return 7
}

func stableAtomID(canonicalRef string) string {
	h := sha256.Sum256([]byte(canonicalRef))
	return "A-" + strings.ToUpper(hex.EncodeToString(h[:8]))
}

func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func lastLine(s string) string {
	lines := strings.Split(s, "\n")
	return lines[len(lines)-1]
}

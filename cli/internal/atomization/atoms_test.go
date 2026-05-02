package atomization

import (
	"strings"
	"testing"
)

var atomOpts = AtomizeOptions{
	DocumentRef:  "doc-2026",
	SourceFile:   "sources/contract.md",
	Domain:       "insurance",
	DefaultState: ReviewDraft,
}

const atomSampleMD = `# Contract Title

| Référence | DOC-2026-001 |
| --- | --- |
| Statut | En vigueur |

Opening statement of the contract.

## Chapter 1 — Coverage

Coverage applies to all insured parties.

### Section 1.1 — Water Damage

Water damage is covered when:

- The event is accidental
- No exclusion applies
- The claim is filed within 30 days

` + "```yaml" + `
coverage:
  water_damage: true
` + "```" + `

## Chapter 2 — Exclusions

| Exclusion | Condition |
| --- | --- |
| Flood | Declared zone |
| Earthquake | Not covered |
`

func TestAtomizeBasic(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	if set.AtomCount == 0 {
		t.Fatal("expected atoms")
	}
	if set.DocumentRef != "doc-2026" {
		t.Fatalf("expected doc ref, got %q", set.DocumentRef)
	}
	if set.SourceFile != "sources/contract.md" {
		t.Fatalf("expected source file, got %q", set.SourceFile)
	}
	if !strings.HasPrefix(set.SourceHash, "sha256:") {
		t.Fatalf("expected source hash, got %q", set.SourceHash)
	}
	if set.AtomCount != len(set.Atoms) {
		t.Fatalf("atom_count %d != len(atoms) %d", set.AtomCount, len(set.Atoms))
	}
}

func TestAtomizeStableIDs(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	s1 := Atomize(ast, atomOpts)
	s2 := Atomize(ast, atomOpts)

	if len(s1.Atoms) != len(s2.Atoms) {
		t.Fatalf("atom count unstable: %d vs %d", len(s1.Atoms), len(s2.Atoms))
	}
	for i := range s1.Atoms {
		if s1.Atoms[i].ID != s2.Atoms[i].ID {
			t.Fatalf("atom[%d] ID unstable: %q vs %q", i, s1.Atoms[i].ID, s2.Atoms[i].ID)
		}
	}
}

func TestAtomizeIDFormat(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	for _, atom := range set.Atoms {
		if !strings.HasPrefix(atom.ID, "A-") {
			t.Fatalf("atom ID should start with A-, got %q", atom.ID)
		}
		if len(atom.ID) != 18 { // A- + 16 hex
			t.Fatalf("atom ID should be 18 chars, got %d: %q", len(atom.ID), atom.ID)
		}
	}
}

func TestAtomizeIDsUnique(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	seen := map[string]string{}
	for _, atom := range set.Atoms {
		if prev, dup := seen[atom.ID]; dup {
			t.Fatalf("duplicate ID %s: %q and %q", atom.ID, prev, atom.CanonicalRef)
		}
		seen[atom.ID] = atom.CanonicalRef
	}
}

func TestAtomizeContentHash(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	for _, atom := range set.Atoms {
		if !strings.HasPrefix(atom.ContentHash, "sha256:") {
			t.Fatalf("atom %s has invalid content hash: %q", atom.ID, atom.ContentHash)
		}
		// Hash should match text content.
		expected := hashContent(atom.Text)
		if atom.ContentHash != expected {
			t.Fatalf("atom %s content hash mismatch", atom.ID)
		}
	}
}

func TestAtomizeSourceSpans(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	for _, atom := range set.Atoms {
		sp := atom.SourceSpan
		if sp.File != "sources/contract.md" {
			t.Fatalf("atom %s file: %q", atom.ID, sp.File)
		}
		if sp.StartLine <= 0 {
			t.Fatalf("atom %s start_line: %d", atom.ID, sp.StartLine)
		}
		if sp.EndLine < sp.StartLine {
			t.Fatalf("atom %s end < start: %d < %d", atom.ID, sp.EndLine, sp.StartLine)
		}
		if sp.StartCol <= 0 {
			t.Fatalf("atom %s start_col: %d", atom.ID, sp.StartCol)
		}
	}
}

func TestAtomizeSourceSpanString(t *testing.T) {
	sp := SourceSpan{File: "test.md", StartLine: 10, EndLine: 12, StartCol: 1, EndCol: 40}
	s := sp.String()
	if s != "test.md:10:1" {
		t.Fatalf("expected test.md:10:1, got %q", s)
	}
}

func TestAtomizeReviewState(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	for _, atom := range set.Atoms {
		if atom.ReviewState != ReviewDraft {
			t.Fatalf("atom %s review state: %q (expected draft)", atom.ID, atom.ReviewState)
		}
	}
}

func TestAtomizeCustomReviewState(t *testing.T) {
	ast := ParseMarkdown("# Doc\n\nParagraph.\n")
	opts := atomOpts
	opts.DefaultState = ReviewPending
	set := Atomize(ast, opts)

	for _, atom := range set.Atoms {
		if atom.ReviewState != ReviewPending {
			t.Fatalf("expected pending, got %q", atom.ReviewState)
		}
	}
}

func TestAtomizeInvalidDefaultState(t *testing.T) {
	ast := ParseMarkdown("# Doc\n\nText.\n")
	opts := atomOpts
	opts.DefaultState = ReviewState("bogus")
	set := Atomize(ast, opts)

	for _, atom := range set.Atoms {
		if atom.ReviewState != ReviewDraft {
			t.Fatalf("invalid state should default to draft, got %q", atom.ReviewState)
		}
	}
}

func TestAtomizeParentChain(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	atomMap := map[string]*Atom{}
	for i := range set.Atoms {
		atomMap[set.Atoms[i].ID] = &set.Atoms[i]
	}

	for _, atom := range set.Atoms {
		if atom.ParentID == "" {
			continue
		}
		if _, ok := atomMap[atom.ParentID]; !ok {
			t.Fatalf("atom %s parent %s not found", atom.ID, atom.ParentID)
		}
	}
}

func TestAtomizeCanonicalRefs(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	for _, atom := range set.Atoms {
		if atom.CanonicalRef == "" {
			t.Fatalf("atom %s has empty canonical ref", atom.ID)
		}
		if !strings.HasPrefix(atom.CanonicalRef, "doc-2026/") {
			t.Fatalf("canonical ref should start with doc ref: %q", atom.CanonicalRef)
		}
	}
}

func TestAtomizeHeadingAtoms(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	headings := filterAtoms(set, AtomClause)
	// H1 + H2 + H2 + H3 = 4
	if len(headings) != 4 {
		t.Fatalf("expected 4 heading atoms, got %d", len(headings))
	}
	if headings[0].Title != "Contract Title" {
		t.Fatalf("first heading: %q", headings[0].Title)
	}
	if headings[0].Depth != 1 {
		t.Fatalf("H1 depth should be 1, got %d", headings[0].Depth)
	}
}

func TestAtomizeListItems(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	items := filterAtoms(set, AtomListItem)
	if len(items) != 3 {
		t.Fatalf("expected 3 list items, got %d", len(items))
	}
	if !strings.Contains(items[0].Text, "accidental") {
		t.Fatalf("first list item: %q", items[0].Text)
	}
}

func TestAtomizeCodeBlockAtom(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	codes := filterAtoms(set, AtomCodeBlock)
	if len(codes) != 1 {
		t.Fatalf("expected 1 code atom, got %d", len(codes))
	}
	if !strings.Contains(codes[0].Text, "water_damage") {
		t.Fatalf("code atom text: %q", codes[0].Text)
	}
}

func TestAtomizeTableAtom(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	tables := filterAtoms(set, AtomTable)
	if len(tables) == 0 {
		t.Fatal("expected table atoms")
	}
}

func TestAtomizeMetaAtom(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	metas := filterAtoms(set, AtomMeta)
	if len(metas) == 0 {
		t.Fatal("expected meta atom from H1 metadata table")
	}
}

func TestAtomizeDomain(t *testing.T) {
	ast := ParseMarkdown("# Doc\nText.\n")
	set := Atomize(ast, atomOpts)
	for _, atom := range set.Atoms {
		if atom.Domain != "insurance" {
			t.Fatalf("expected domain insurance, got %q", atom.Domain)
		}
	}
}

func TestAtomizeBlockIDRef(t *testing.T) {
	ast := ParseMarkdown(atomSampleMD)
	set := Atomize(ast, atomOpts)

	blockIDs := map[string]bool{}
	for _, blk := range ast.Blocks {
		blockIDs[blk.ID] = true
	}

	for _, atom := range set.Atoms {
		if !blockIDs[atom.BlockID] {
			t.Fatalf("atom %s references unknown block %s", atom.ID, atom.BlockID)
		}
	}
}

func TestAtomizeEmpty(t *testing.T) {
	ast := ParseMarkdown("")
	set := Atomize(ast, atomOpts)
	if set.AtomCount != 0 {
		t.Fatalf("expected 0 atoms for empty doc, got %d", set.AtomCount)
	}
}

func TestReviewStateIsValid(t *testing.T) {
	for _, s := range []ReviewState{ReviewDraft, ReviewPending, ReviewApproved, ReviewRejected, ReviewAmended} {
		if !s.IsValid() {
			t.Fatalf("%q should be valid", s)
		}
	}
	if ReviewState("bogus").IsValid() {
		t.Fatal("bogus should be invalid")
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Chapter 1 — Coverage": "chapter-1-coverage",
		"Section 1.1":          "section-1-1",
		"UPPER CASE":           "upper-case",
		"  spaces  ":           "spaces",
	}
	for input, want := range cases {
		got := slugify(input)
		if got != want {
			t.Fatalf("slugify(%q) = %q, want %q", input, got, want)
		}
	}
}

// --- helpers ---

func filterAtoms(set AtomSet, typ AtomType) []Atom {
	var result []Atom
	for _, a := range set.Atoms {
		if a.Type == typ {
			result = append(result, a)
		}
	}
	return result
}

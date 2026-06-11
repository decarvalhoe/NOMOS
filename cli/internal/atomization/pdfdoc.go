// W23-1 (#590) — the PDF leg of the atomization engine: born-digital PDF text
// (extracted by the SAME fidelity core as VRC-30 #589, one claim ladder, one
// behavior) becomes Atoms with page+line+Y locators, ready for `bundle`.
//
// Claim boundary: rung 1 only (born-digital text). Pages without extractable
// text are returned as UNSUPPORTED page numbers — the caller decides: the CLI
// warns, the bundle REFUSES the source (a bundle never silently drops content
// it was asked to carry). Byte offsets into the compressed container are not
// claimable: the locator is structural (`/page[2]/line[5]`), the integrity
// story stays at the file-level source hash.
package atomization

import (
	"fmt"

	"github.com/RBOKproject/Nomos/cli/internal/fidelity"
)

// AtomizePDF converts a born-digital PDF into an AtomSet. The second return
// value lists the pages that are OUT of the born-digital-text claim (scanned /
// image-only) — explicitly, never silently dropped.
func AtomizePDF(source []byte, opts AtomizeOptions) (AtomSet, []int, error) {
	lines, unsupported, err := fidelity.ExtractPDFLines(source)
	if err != nil {
		return AtomSet{}, nil, err
	}
	defaultState := opts.DefaultState
	if !defaultState.IsValid() {
		defaultState = ReviewDraft
	}
	set := AtomSet{
		DocumentRef: opts.DocumentRef,
		SourceFile:  opts.SourceFile,
		SourceHash:  hashContent(string(source)),
	}
	lineInPage := map[int]int{}
	globalLine := 0
	for _, ln := range lines {
		lineInPage[ln.Page]++
		globalLine++
		domPath := fmt.Sprintf("/page[%d]/line[%d]", ln.Page, lineInPage[ln.Page])
		canonicalRef := opts.DocumentRef + "#" + domPath
		atom := Atom{
			ID:           stableAtomID(canonicalRef),
			CanonicalRef: canonicalRef,
			Type:         AtomClause,
			Text:         ln.Text,
			ContentHash:  hashContent(ln.Text),
			SourceSpan: SourceSpan{
				File:      opts.SourceFile,
				StartLine: globalLine,
				EndLine:   globalLine,
				DomPath:   domPath,
			},
			BlockID:     fmt.Sprintf("P-%03d-%04d", ln.Page, lineInPage[ln.Page]),
			Depth:       1,
			ReviewState: defaultState,
			Domain:      opts.Domain,
		}
		if opts.EmitFacets {
			f := DeriveFacets(atom)
			atom.Facets = &f
		}
		set.Atoms = append(set.Atoms, atom)
	}
	set.AtomCount = len(set.Atoms)
	return set, unsupported, nil
}

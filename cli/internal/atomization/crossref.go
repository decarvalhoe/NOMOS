package atomization

// VRC-43 (#579, doc 45 §3 B5) — deterministic cross-reference graph.
//
// Legal text is not a flat list of rules. "Sous réserve de l'art. 12", "par
// dérogation à l'article 5", "au sens de l'art. 3 al. 2" are edges: they change
// what the carrying rule means. Until now those edges were invisible, so an
// answer could cite an atom while silently ignoring the article that qualifies
// it.
//
// This builds the graph. Three properties make it evidence rather than output:
//
//  1. PARSED, NEVER GENERATED. Every edge carries the verbatim text it was
//     built from and the half-open offsets it occupies in the atom. Verify()
//     re-slices the atom and refuses any edge whose bytes do not reconstruct.
//     No model is involved, at any point, by construction: this file has no
//     network, no scorer, and no source of text other than the atom itself.
//  2. UNPARSABLE IS RECORDED, NOT DROPPED. A cue with no parsable target —
//     "sous réserve des dispositions applicables" — becomes an explicit
//     Unsupported entry naming the cue and its span. Silence would let a reader
//     believe the rule stands alone.
//  3. UNRESOLVED IS NOT INVENTED. A target locator that matches no atom in the
//     set stays `unresolved_target`, with the locator recorded verbatim. The
//     graph never fabricates a destination to look complete.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// CrossRefKind is the relation a cue expresses. The kinds are deliberately few
// and legally distinct: each changes the carrying rule differently.
type CrossRefKind string

const (
	// CrossRefSubjectTo — the rule yields to the target ("sous réserve de").
	CrossRefSubjectTo CrossRefKind = "subject_to"
	// CrossRefWithoutPrejudiceTo — the target survives the rule ("sans préjudice de").
	CrossRefWithoutPrejudiceTo CrossRefKind = "without_prejudice_to"
	// CrossRefNotwithstanding — the rule overrides the target ("nonobstant").
	CrossRefNotwithstanding CrossRefKind = "notwithstanding"
	// CrossRefByDerogationFrom — the rule is an exception to the target ("par dérogation à").
	CrossRefByDerogationFrom CrossRefKind = "by_derogation_from"
	// CrossRefInAccordanceWith — the rule is applied as the target prescribes ("conformément à").
	CrossRefInAccordanceWith CrossRefKind = "in_accordance_with"
	// CrossRefWithinMeaningOf — the target supplies a definition ("au sens de").
	CrossRefWithinMeaningOf CrossRefKind = "within_meaning_of"
	// CrossRefSeeAlso — a pointer with no normative force ("voir", "cf.").
	CrossRefSeeAlso CrossRefKind = "see_also"
)

// Resolution states. `unresolved_target` is a first-class outcome, not an error:
// a corpus legitimately refers to articles outside it.
const (
	CrossRefResolved         = "resolved"
	CrossRefUnresolvedTarget = "unresolved_target"
)

// CrossRefGraphSchemaVersion identifies the emitted envelope.
const CrossRefGraphSchemaVersion = "nomos-crossref-graph-v1"

// CrossRef is one edge: an atom pointing at a target locator, with the bytes it
// was parsed from.
type CrossRef struct {
	FromAtomID string       `json:"from_atom_id"`
	Kind       CrossRefKind `json:"kind"`
	// Cue is the verbatim trigger phrase, as written in the source.
	Cue string `json:"cue"`
	// TargetRaw is the locator verbatim; TargetLocator is its normalised form,
	// which is what resolution compares. Both are kept so a reader can see what
	// the text said and what the engine made of it.
	TargetRaw     string `json:"target_raw"`
	TargetLocator string `json:"target_locator"`
	ToAtomID      string `json:"to_atom_id,omitempty"`
	Resolution    string `json:"resolution"`
	// MatchedText is the full verbatim match, and StartOffset/EndOffset its
	// half-open range in the atom's Text. Verify() reconstructs from these.
	MatchedText string `json:"matched_text"`
	StartOffset int    `json:"start_offset"`
	EndOffset   int    `json:"end_offset"`
	SourceFile  string `json:"source_file,omitempty"`
	StartLine   int    `json:"start_line,omitempty"`
}

// UnsupportedCrossRef is a cue the rules could not turn into an edge. It is
// emitted so the gap is visible; dropping it would misrepresent the rule as
// standing alone.
type UnsupportedCrossRef struct {
	FromAtomID  string       `json:"from_atom_id"`
	Kind        CrossRefKind `json:"kind"`
	Cue         string       `json:"cue"`
	MatchedText string       `json:"matched_text"`
	StartOffset int          `json:"start_offset"`
	EndOffset   int          `json:"end_offset"`
	Reason      string       `json:"reason"`
	SourceFile  string       `json:"source_file,omitempty"`
	StartLine   int          `json:"start_line,omitempty"`
}

// CrossRefStats is the published shape of the result: the three outcomes are
// reported separately, because "12 edges" alone would hide how many point
// nowhere and how many cues were not understood at all.
type CrossRefStats struct {
	Atoms            int `json:"atoms"`
	Edges            int `json:"edges"`
	Resolved         int `json:"resolved"`
	UnresolvedTarget int `json:"unresolved_target"`
	Unsupported      int `json:"unsupported"`
}

// CrossRefGraph is the emitted envelope.
type CrossRefGraph struct {
	SchemaVersion string                `json:"schema_version"`
	DocumentRef   string                `json:"document_ref,omitempty"`
	SourceFile    string                `json:"source_file,omitempty"`
	ClaimBoundary string                `json:"claim_boundary"`
	Stats         CrossRefStats         `json:"stats"`
	Edges         []CrossRef            `json:"edges"`
	Unsupported   []UnsupportedCrossRef `json:"unsupported"`
}

const crossRefClaimBoundary = "Cross-references parsed from source text by rules only. " +
	"An edge asserts that the text says it, not that the reference is legally correct, " +
	"in force, or exhaustive."

// cueRule pairs a trigger phrase with the relation it expresses.
type cueRule struct {
	kind CrossRefKind
	re   *regexp.Regexp
}

// The cue vocabulary. Each pattern captures ONLY the cue; the locator is parsed
// separately, immediately after it, so an unparsable target is detectable
// rather than swallowed by a greedy match.
var crossRefCues = []cueRule{
	{CrossRefSubjectTo, regexp.MustCompile(`(?i)sous r[ée]serve d(?:es|e la|e l'|e l’|u|e)`)},
	{CrossRefWithoutPrejudiceTo, regexp.MustCompile(`(?i)sans pr[ée]judice d(?:es|e la|e l'|e l’|u|e)`)},
	{CrossRefNotwithstanding, regexp.MustCompile(`(?i)nonobstant`)},
	{CrossRefByDerogationFrom, regexp.MustCompile(`(?i)par d[ée]rogation [àa]u?x?`)},
	{CrossRefInAccordanceWith, regexp.MustCompile(`(?i)conform[ée]ment [àa]u?x?`)},
	{CrossRefWithinMeaningOf, regexp.MustCompile(`(?i)au sens d(?:es|e la|e l'|e l’|u|e)`)},
	{CrossRefSeeAlso, regexp.MustCompile(`(?i)\b(?:voir|cf\.)`)},
}

// The target locator, anchored to the position right after a cue. Filler words
// ("l'", "les dispositions de", "à") are allowed between cue and locator, but
// only a bounded, enumerated set — an open `.*` would let the parser leap across
// a sentence and attach a locator that qualifies something else.
var crossRefTargetRe = regexp.MustCompile(
	`^(?:\s+|\s*(?:l['’]|d['’]|de\s+l['’]|des\s+|les\s+|la\s+|le\s+|aux?\s+|dispositions\s+(?:de\s+l['’]|des\s+)?)\s*)*` +
		`((?:art(?:icle)?s?\.?\s*\d+[a-z]?(?:\s*(?:al\.|alin[ée]a)\s*\d+)?)|(?:§\s*\d+)|(?:section\s+\d+)|(?:ch(?:apitre)?\.?\s*\d+))`,
)

// locatorNormalizeRe collapses the many spellings of the same locator.
var (
	locatorSpaceRe   = regexp.MustCompile(`\s+`)
	locatorArticleRe = regexp.MustCompile(`(?i)^articles?\.?\s*`)
	locatorArtRe     = regexp.MustCompile(`(?i)^arts?\.?\s*`)
	locatorAlineaRe  = regexp.MustCompile(`(?i)\s*(?:al\.|alin[ée]a)\s*`)
	locatorChapRe    = regexp.MustCompile(`(?i)^ch(?:apitre)?\.?\s*`)
	locatorSectionRe = regexp.MustCompile(`(?i)^section\s+`)
	locatorParaRe    = regexp.MustCompile(`^§\s*`)
)

// NormalizeCrossRefLocator maps a written locator to its comparison form:
// "Article 12", "art.12", "art 12" all become "art.12"; "art. 3 al. 2" becomes
// "art.3/al.2". Normalisation is only ever used for COMPARISON — the verbatim
// text is always kept beside it.
func NormalizeCrossRefLocator(raw string) string {
	s := strings.TrimSpace(locatorSpaceRe.ReplaceAllString(raw, " "))
	switch {
	case locatorParaRe.MatchString(s):
		return "§" + strings.TrimSpace(locatorParaRe.ReplaceAllString(s, ""))
	case locatorSectionRe.MatchString(s):
		return "section." + strings.TrimSpace(locatorSectionRe.ReplaceAllString(s, ""))
	case locatorChapRe.MatchString(s):
		return "ch." + strings.TrimSpace(locatorChapRe.ReplaceAllString(s, ""))
	}
	s = locatorArticleRe.ReplaceAllString(s, "")
	s = locatorArtRe.ReplaceAllString(s, "")
	if locatorAlineaRe.MatchString(s) {
		parts := locatorAlineaRe.Split(s, 2)
		if len(parts) == 2 {
			return "art." + strings.TrimSpace(parts[0]) + "/al." + strings.TrimSpace(parts[1])
		}
	}
	return "art." + strings.TrimSpace(s)
}

// crossRefLocatorInText finds a locator anywhere in an atom's own text, used to
// index atoms as possible targets.
var crossRefSelfLocatorRe = regexp.MustCompile(
	`(?i)(art(?:icle)?s?\.?\s*\d+[a-z]?(?:\s*(?:al\.|alin[ée]a)\s*\d+)?|§\s*\d+|section\s+\d+|ch(?:apitre)?\.?\s*\d+)`,
)

// buildTargetIndex maps normalised locators to atom IDs. An atom is indexed by
// the locator in its title (its heading) or, failing that, its canonical ref —
// never by a locator merely mentioned in its body, which would make every atom
// citing article 12 a candidate destination for "art. 12".
func buildTargetIndex(atoms []Atom) map[string]string {
	index := map[string]string{}
	for _, atom := range atoms {
		for _, candidate := range []string{atom.Title, atom.CanonicalRef} {
			if candidate == "" {
				continue
			}
			m := crossRefSelfLocatorRe.FindString(candidate)
			if m == "" {
				continue
			}
			key := NormalizeCrossRefLocator(m)
			// First atom wins, so the index is deterministic under atom order.
			if _, seen := index[key]; !seen {
				index[key] = atom.ID
			}
			break
		}
	}
	return index
}

// BuildCrossRefGraph parses the cross-reference graph of an atom set.
//
// It reads nothing but the atoms handed to it. The output is deterministic:
// edges are ordered by (atom id, offset), and the target index resolves ties by
// atom order.
func BuildCrossRefGraph(set AtomSet) CrossRefGraph {
	index := buildTargetIndex(set.Atoms)

	graph := CrossRefGraph{
		SchemaVersion: CrossRefGraphSchemaVersion,
		DocumentRef:   set.DocumentRef,
		SourceFile:    set.SourceFile,
		ClaimBoundary: crossRefClaimBoundary,
		Edges:         []CrossRef{},
		Unsupported:   []UnsupportedCrossRef{},
	}

	for _, atom := range set.Atoms {
		edges, unsupported := parseAtomCrossRefs(atom, index)
		graph.Edges = append(graph.Edges, edges...)
		graph.Unsupported = append(graph.Unsupported, unsupported...)
	}

	sort.SliceStable(graph.Edges, func(i, j int) bool {
		if graph.Edges[i].FromAtomID != graph.Edges[j].FromAtomID {
			return graph.Edges[i].FromAtomID < graph.Edges[j].FromAtomID
		}
		return graph.Edges[i].StartOffset < graph.Edges[j].StartOffset
	})
	sort.SliceStable(graph.Unsupported, func(i, j int) bool {
		if graph.Unsupported[i].FromAtomID != graph.Unsupported[j].FromAtomID {
			return graph.Unsupported[i].FromAtomID < graph.Unsupported[j].FromAtomID
		}
		return graph.Unsupported[i].StartOffset < graph.Unsupported[j].StartOffset
	})

	graph.Stats = CrossRefStats{
		Atoms:       len(set.Atoms),
		Edges:       len(graph.Edges),
		Unsupported: len(graph.Unsupported),
	}
	for _, e := range graph.Edges {
		if e.Resolution == CrossRefResolved {
			graph.Stats.Resolved++
		} else {
			graph.Stats.UnresolvedTarget++
		}
	}
	return graph
}

// parseAtomCrossRefs scans one atom. Overlapping cue matches are resolved by
// taking the leftmost, longest cue, so "sous réserve de" is never also reported
// as a bare "de".
func parseAtomCrossRefs(atom Atom, index map[string]string) ([]CrossRef, []UnsupportedCrossRef) {
	text := atom.Text
	var edges []CrossRef
	var unsupported []UnsupportedCrossRef

	type hit struct {
		kind       CrossRefKind
		start, end int
	}
	var hits []hit
	for _, rule := range crossRefCues {
		for _, loc := range rule.re.FindAllStringIndex(text, -1) {
			hits = append(hits, hit{rule.kind, loc[0], loc[1]})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].start != hits[j].start {
			return hits[i].start < hits[j].start
		}
		return hits[i].end > hits[j].end
	})

	lastEnd := -1
	for _, h := range hits {
		if h.start < lastEnd {
			continue // contained in a previous, longer cue
		}
		cue := text[h.start:h.end]
		rest := text[h.end:]

		loc := crossRefTargetRe.FindStringSubmatchIndex(rest)
		if loc == nil {
			unsupported = append(unsupported, UnsupportedCrossRef{
				FromAtomID:  atom.ID,
				Kind:        h.kind,
				Cue:         cue,
				MatchedText: cue,
				StartOffset: h.start,
				EndOffset:   h.end,
				Reason:      "no parsable target locator follows the cue",
				SourceFile:  atom.SourceSpan.File,
				StartLine:   atom.SourceSpan.StartLine,
			})
			lastEnd = h.end
			continue
		}

		matchEnd := h.end + loc[1]
		targetRaw := rest[loc[2]:loc[3]]
		normalized := NormalizeCrossRefLocator(targetRaw)

		edge := CrossRef{
			FromAtomID:    atom.ID,
			Kind:          h.kind,
			Cue:           cue,
			TargetRaw:     targetRaw,
			TargetLocator: normalized,
			Resolution:    CrossRefUnresolvedTarget,
			MatchedText:   text[h.start:matchEnd],
			StartOffset:   h.start,
			EndOffset:     matchEnd,
			SourceFile:    atom.SourceSpan.File,
			StartLine:     atom.SourceSpan.StartLine,
		}
		if to, ok := index[normalized]; ok && to != atom.ID {
			edge.ToAtomID = to
			edge.Resolution = CrossRefResolved
		}
		edges = append(edges, edge)
		lastEnd = matchEnd
	}
	return edges, unsupported
}

// VerifyCrossRefGraph re-derives every edge from the atoms and refuses any that
// does not reconstruct. This is the adversarial core of VRC-43: a fabricated
// edge — one whose text is not in the source at the offsets it claims — cannot
// survive this check, which is what makes "parsed, never generated" a property
// rather than a promise.
func VerifyCrossRefGraph(set AtomSet, graph CrossRefGraph) error {
	byID := make(map[string]Atom, len(set.Atoms))
	for _, atom := range set.Atoms {
		byID[atom.ID] = atom
	}

	slice := func(atomID string, start, end int, what string) (string, error) {
		atom, ok := byID[atomID]
		if !ok {
			return "", fmt.Errorf("%s: from_atom_id %q is not in the atom set", what, atomID)
		}
		if start < 0 || end > len(atom.Text) || start >= end {
			return "", fmt.Errorf("%s: offsets [%d,%d) are outside atom %q (len %d)",
				what, start, end, atomID, len(atom.Text))
		}
		return atom.Text[start:end], nil
	}

	for i, e := range graph.Edges {
		what := fmt.Sprintf("edge %d", i)
		got, err := slice(e.FromAtomID, e.StartOffset, e.EndOffset, what)
		if err != nil {
			return err
		}
		if got != e.MatchedText {
			return fmt.Errorf("%s: matched_text %q is not what atom %q holds at [%d,%d) (%q) — "+
				"the edge was not parsed from the source", what, e.MatchedText, e.FromAtomID,
				e.StartOffset, e.EndOffset, got)
		}
		if !strings.Contains(e.MatchedText, e.TargetRaw) {
			return fmt.Errorf("%s: target_raw %q does not appear in the matched text %q",
				what, e.TargetRaw, e.MatchedText)
		}
		if !strings.Contains(e.MatchedText, e.Cue) {
			return fmt.Errorf("%s: cue %q does not appear in the matched text %q",
				what, e.Cue, e.MatchedText)
		}
		switch e.Resolution {
		case CrossRefResolved:
			if e.ToAtomID == "" {
				return fmt.Errorf("%s: resolved with no to_atom_id", what)
			}
			if _, ok := byID[e.ToAtomID]; !ok {
				return fmt.Errorf("%s: to_atom_id %q is not in the atom set — "+
					"the destination was invented", what, e.ToAtomID)
			}
		case CrossRefUnresolvedTarget:
			if e.ToAtomID != "" {
				return fmt.Errorf("%s: unresolved but carries to_atom_id %q", what, e.ToAtomID)
			}
		default:
			return fmt.Errorf("%s: unknown resolution %q", what, e.Resolution)
		}
	}

	for i, u := range graph.Unsupported {
		what := fmt.Sprintf("unsupported %d", i)
		got, err := slice(u.FromAtomID, u.StartOffset, u.EndOffset, what)
		if err != nil {
			return err
		}
		if got != u.MatchedText {
			return fmt.Errorf("%s: matched_text %q is not what atom %q holds at [%d,%d) (%q)",
				what, u.MatchedText, u.FromAtomID, u.StartOffset, u.EndOffset, got)
		}
		if u.Reason == "" {
			return fmt.Errorf("%s: recorded without a reason", what)
		}
	}
	return nil
}

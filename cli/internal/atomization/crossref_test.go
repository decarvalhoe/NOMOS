package atomization

// VRC-43 (#579) — the cross-reference graph is parsed, never generated.
//
// Doctrine §2.3: the proof is the failure. The load-bearing tests here are the
// ones that FORGE an edge and prove Verify refuses it — that is what turns
// "no LLM is involved" from a promise into a property.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const crossRefDoc = `# Art. 3 Définitions

Au sens de la présente loi, un système est un ensemble de composants.

# Art. 12 Obligations du fournisseur

Sous réserve de l'art. 3, le fournisseur documente le système.
Conformément à l'article 5 al. 2, il conserve les journaux.
Nonobstant les dispositions applicables, une exception demeure.
Voir art. 99 pour les sanctions.
`

func buildGraph(t *testing.T, src string) (AtomSet, CrossRefGraph) {
	t.Helper()
	ast := ParseMarkdown(src)
	set := Atomize(ast, AtomizeOptions{DocumentRef: "demo", SourceFile: "demo.md"})
	graph := BuildCrossRefGraph(set)
	if err := VerifyCrossRefGraph(set, graph); err != nil {
		t.Fatalf("the graph the engine just built does not verify: %v", err)
	}
	return set, graph
}

func findEdge(g CrossRefGraph, kind CrossRefKind) *CrossRef {
	for i := range g.Edges {
		if g.Edges[i].Kind == kind {
			return &g.Edges[i]
		}
	}
	return nil
}

func TestCrossRef_KnownReferenceIsPresentWithItsSourceSpan(t *testing.T) {
	// The proof the issue asks for: a known cross-reference of the corpus is in
	// the graph, and it carries the span it was read from.
	set, graph := buildGraph(t, crossRefDoc)

	edge := findEdge(graph, CrossRefSubjectTo)
	if edge == nil {
		t.Fatalf("the 'sous réserve de' edge is missing: %+v", graph.Edges)
	}
	if edge.TargetLocator != "art.3" {
		t.Fatalf("target locator = %q, want art.3", edge.TargetLocator)
	}
	if edge.Resolution != CrossRefResolved || edge.ToAtomID == "" {
		t.Fatalf("the reference to art. 3 should resolve inside the document: %+v", edge)
	}

	// The span is not decoration: re-slicing the atom must give the same bytes.
	var carrying Atom
	for _, a := range set.Atoms {
		if a.ID == edge.FromAtomID {
			carrying = a
		}
	}
	if got := carrying.Text[edge.StartOffset:edge.EndOffset]; got != edge.MatchedText {
		t.Fatalf("span does not reproduce the match: %q vs %q", got, edge.MatchedText)
	}
	if !strings.Contains(edge.MatchedText, "art. 3") {
		t.Fatalf("matched text lost the locator: %q", edge.MatchedText)
	}
}

func TestCrossRef_EachCueYieldsItsOwnRelation(t *testing.T) {
	// The kinds are legally distinct; collapsing them would lose the reason the
	// edge exists.
	_, graph := buildGraph(t, crossRefDoc)
	for _, kind := range []CrossRefKind{
		CrossRefSubjectTo, CrossRefInAccordanceWith, CrossRefSeeAlso,
	} {
		if findEdge(graph, kind) == nil {
			t.Fatalf("no edge of kind %q: %+v", kind, graph.Edges)
		}
	}
	if e := findEdge(graph, CrossRefInAccordanceWith); e.TargetLocator != "art.5/al.2" {
		t.Fatalf("alinéa lost in normalisation: %q", e.TargetLocator)
	}
}

func TestCrossRef_UnparsableCueIsRecordedNotDropped(t *testing.T) {
	// "Nonobstant les dispositions applicables" names no article. Silence would
	// let a reader believe the rule stands alone.
	_, graph := buildGraph(t, crossRefDoc)
	if len(graph.Unsupported) == 0 {
		t.Fatal("the unparsable cue was dropped instead of recorded")
	}
	found := false
	for _, u := range graph.Unsupported {
		if u.Kind == CrossRefNotwithstanding {
			found = true
			if u.Reason == "" {
				t.Fatal("unsupported entry carries no reason")
			}
			if !strings.Contains(strings.ToLower(u.MatchedText), "nonobstant") {
				t.Fatalf("unsupported entry lost its cue: %q", u.MatchedText)
			}
		}
	}
	if !found {
		t.Fatalf("the 'nonobstant' cue is not in the unsupported list: %+v", graph.Unsupported)
	}
}

func TestCrossRef_TargetOutsideTheCorpusStaysUnresolved(t *testing.T) {
	// "Voir art. 99" points outside the document. The graph must say so rather
	// than attach the nearest plausible atom.
	_, graph := buildGraph(t, crossRefDoc)
	edge := findEdge(graph, CrossRefSeeAlso)
	if edge == nil {
		t.Fatal("the see_also edge is missing")
	}
	if edge.Resolution != CrossRefUnresolvedTarget {
		t.Fatalf("art. 99 is not in the document; resolution = %q", edge.Resolution)
	}
	if edge.ToAtomID != "" {
		t.Fatalf("an unresolved edge must carry no destination, got %q", edge.ToAtomID)
	}
	if edge.TargetRaw == "" {
		t.Fatal("the locator must still be recorded verbatim when unresolved")
	}
}

func TestCrossRef_ForgedEdgeIsRefused(t *testing.T) {
	// ADVERSARIAL, the central one. Hand-write an edge whose text is not in the
	// source — exactly what a generated graph would produce — and prove Verify
	// refuses it.
	set, graph := buildGraph(t, crossRefDoc)

	forged := graph
	forged.Edges = append([]CrossRef{}, graph.Edges...)
	forged.Edges[0].MatchedText = "sous réserve de l'art. 42"
	forged.Edges[0].TargetRaw = "art. 42"
	forged.Edges[0].TargetLocator = "art.42"

	err := VerifyCrossRefGraph(set, forged)
	if err == nil {
		t.Fatal("a forged edge passed verification")
	}
	if !strings.Contains(err.Error(), "not parsed from the source") {
		t.Fatalf("the failure should name the forgery: %v", err)
	}
}

func TestCrossRef_InventedDestinationIsRefused(t *testing.T) {
	// ADVERSARIAL: the text is genuine but the destination is made up.
	set, graph := buildGraph(t, crossRefDoc)
	forged := graph
	forged.Edges = append([]CrossRef{}, graph.Edges...)
	for i := range forged.Edges {
		forged.Edges[i].Resolution = CrossRefResolved
		forged.Edges[i].ToAtomID = "atom-that-does-not-exist"
	}
	err := VerifyCrossRefGraph(set, forged)
	if err == nil {
		t.Fatal("an invented destination passed verification")
	}
	if !strings.Contains(err.Error(), "invented") {
		t.Fatalf("the failure should name the invention: %v", err)
	}
}

func TestCrossRef_ShiftedSpanIsRefused(t *testing.T) {
	// ADVERSARIAL: right text, wrong offsets. The span must be load-bearing.
	set, graph := buildGraph(t, crossRefDoc)
	forged := graph
	forged.Edges = append([]CrossRef{}, graph.Edges...)
	forged.Edges[0].StartOffset++
	if err := VerifyCrossRefGraph(set, forged); err == nil {
		t.Fatal("a shifted span passed verification")
	}
}

func TestCrossRef_ResolvedWithoutDestinationIsRefused(t *testing.T) {
	set, graph := buildGraph(t, crossRefDoc)
	forged := graph
	forged.Edges = append([]CrossRef{}, graph.Edges...)
	for i := range forged.Edges {
		forged.Edges[i].Resolution = CrossRefResolved
		forged.Edges[i].ToAtomID = ""
	}
	if err := VerifyCrossRefGraph(set, forged); err == nil {
		t.Fatal("a resolved edge with no destination passed verification")
	}
}

func TestCrossRef_UnsupportedWithoutReasonIsRefused(t *testing.T) {
	set, graph := buildGraph(t, crossRefDoc)
	if len(graph.Unsupported) == 0 {
		t.Skip("fixture produced no unsupported entry")
	}
	forged := graph
	forged.Unsupported = append([]UnsupportedCrossRef{}, graph.Unsupported...)
	forged.Unsupported[0].Reason = ""
	if err := VerifyCrossRefGraph(set, forged); err == nil {
		t.Fatal("an unsupported entry with no reason passed verification")
	}
}

func TestCrossRef_NoEdgeWithoutACue(t *testing.T) {
	// Precision: a bare mention of an article is not a cross-reference. Only a
	// cue creates an edge, so the graph does not fill up with noise.
	_, graph := buildGraph(t, "# Titre\n\nL'art. 7 est mentionné sans aucune locution de renvoi.\n")
	if len(graph.Edges) != 0 {
		t.Fatalf("a bare article mention produced edges: %+v", graph.Edges)
	}
	if len(graph.Unsupported) != 0 {
		t.Fatalf("a bare article mention produced unsupported entries: %+v", graph.Unsupported)
	}
}

func TestCrossRef_CueDoesNotLeapAcrossASentence(t *testing.T) {
	// Precision: the locator must follow the cue closely. A cue whose sentence
	// ends before any article is unsupported, NOT attached to the article of the
	// next sentence — that would invent a qualification the text never made.
	_, graph := buildGraph(t,
		"# Titre\n\nSous réserve des dispositions applicables. L'art. 8 prévoit autre chose.\n")
	if len(graph.Edges) != 0 {
		t.Fatalf("the cue leapt to the next sentence: %+v", graph.Edges)
	}
	if len(graph.Unsupported) != 1 {
		t.Fatalf("the dangling cue should be recorded once: %+v", graph.Unsupported)
	}
}

func TestCrossRef_IsDeterministic(t *testing.T) {
	set, first := buildGraph(t, crossRefDoc)
	second := BuildCrossRefGraph(set)
	if len(first.Edges) != len(second.Edges) || len(first.Unsupported) != len(second.Unsupported) {
		t.Fatal("graph size varies between runs")
	}
	for i := range first.Edges {
		if first.Edges[i] != second.Edges[i] {
			t.Fatalf("edge %d differs between runs: %+v vs %+v", i, first.Edges[i], second.Edges[i])
		}
	}
}

func TestCrossRef_StatsReportTheThreeOutcomesSeparately(t *testing.T) {
	// One "edges" number would hide how many point nowhere and how many cues
	// were never understood.
	_, graph := buildGraph(t, crossRefDoc)
	if graph.Stats.Edges != len(graph.Edges) {
		t.Fatalf("edge count disagrees with the edge list")
	}
	if graph.Stats.Resolved+graph.Stats.UnresolvedTarget != graph.Stats.Edges {
		t.Fatalf("resolved + unresolved != edges: %+v", graph.Stats)
	}
	if graph.Stats.Unsupported != len(graph.Unsupported) {
		t.Fatalf("unsupported count disagrees with the list")
	}
	if graph.Stats.Unsupported == 0 || graph.Stats.UnresolvedTarget == 0 {
		t.Fatalf("the fixture should exercise both weak outcomes: %+v", graph.Stats)
	}
}

func TestNormalizeCrossRefLocator(t *testing.T) {
	cases := map[string]string{
		"art. 12":         "art.12",
		"Article 12":      "art.12",
		"art 12":          "art.12",
		"articles 12":     "art.12",
		"art. 3 al. 2":    "art.3/al.2",
		"art. 3 alinéa 2": "art.3/al.2",
		"§ 5":             "§5",
		"section 4":       "section.4",
		"chapitre 2":      "ch.2",
		"ch. 2":           "ch.2",
	}
	for raw, want := range cases {
		if got := NormalizeCrossRefLocator(raw); got != want {
			t.Errorf("NormalizeCrossRefLocator(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestCrossRef_EmptyAtomSetIsAnEmptyGraphNotAFailure(t *testing.T) {
	set := AtomSet{DocumentRef: "empty"}
	graph := BuildCrossRefGraph(set)
	if err := VerifyCrossRefGraph(set, graph); err != nil {
		t.Fatalf("empty graph does not verify: %v", err)
	}
	if graph.Stats.Edges != 0 || graph.Stats.Unsupported != 0 {
		t.Fatalf("empty set produced content: %+v", graph.Stats)
	}
	if graph.ClaimBoundary == "" {
		t.Fatal("even an empty graph carries its claim boundary")
	}
}

func TestCrossRef_KnownReferenceOfTheGoldenCorpusIsInTheGraph(t *testing.T) {
	// The proof VRC-43 (#579) asks for, on the REAL golden corpus rather than an
	// inline fixture: a known cross-reference is present, resolved, and carries
	// the span it was read from.
	path := filepath.Join("..", "..", "internal", "corpus", "testdata",
		"eu-ai-act-golden-corpus", "obligations-deployeur.md")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden corpus document missing: %v", err)
	}

	ast := ParseMarkdown(string(source))
	set := Atomize(ast, AtomizeOptions{DocumentRef: "eu-ai-act", SourceFile: path})
	graph := BuildCrossRefGraph(set)
	if err := VerifyCrossRefGraph(set, graph); err != nil {
		t.Fatalf("the golden-corpus graph does not verify: %v", err)
	}

	var known *CrossRef
	for i := range graph.Edges {
		if graph.Edges[i].Kind == CrossRefSubjectTo && graph.Edges[i].TargetLocator == "art.4" {
			known = &graph.Edges[i]
		}
	}
	if known == nil {
		t.Fatalf("the known 'sous réserve de l'art. 4' reference is absent: %+v", graph.Edges)
	}
	if known.Resolution != CrossRefResolved || known.ToAtomID == "" {
		t.Fatalf("the reference resolves inside the document; got %+v", known)
	}

	byID := map[string]Atom{}
	for _, a := range set.Atoms {
		byID[a.ID] = a
	}
	carrying := byID[known.FromAtomID]
	if got := carrying.Text[known.StartOffset:known.EndOffset]; got != known.MatchedText {
		t.Fatalf("the span does not reproduce the match: %q vs %q", got, known.MatchedText)
	}
	if carrying.SourceSpan.File == "" || carrying.SourceSpan.StartLine == 0 {
		t.Fatalf("the carrying atom has no source span: %+v", carrying.SourceSpan)
	}

	// The corpus also exercises the weak outcomes, so the gate would notice if
	// they silently stopped being reported.
	if graph.Stats.UnresolvedTarget == 0 {
		t.Fatal("the golden corpus should carry a reference pointing outside the document")
	}
	if graph.Stats.Unsupported == 0 {
		t.Fatal("the golden corpus should carry a cue with no parsable target")
	}
}

package atomization

import (
	"strings"
	"testing"
)

func TestFacetsValidate_AcceptsContractValidFacets(t *testing.T) {
	f := Facets{
		Nature:          "obligation",
		DisciplineRole:  []string{"legal.owner", "knowledge.engineer"},
		Activity:        []string{"rag.answering"},
		ScopeLevel:      "atom",
		TrustTier:       "certified",
		Provenance:      "source_backed",
		Confidentiality: "restricted",
		Applicability:   "applicable",
		VocabularyRefs:  []string{"nomos-facet-core"},
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("valid facets rejected: %v", err)
	}
}

func TestFacetsValidate_EmptyFacetsAreValid(t *testing.T) {
	// The no-facet case must remain valid (additive / opt-in).
	var f Facets
	if !f.IsZero() {
		t.Fatal("zero-value facets should report IsZero")
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("empty facets rejected: %v", err)
	}
}

// Adversarial: each scalar axis must reject a value outside its vocabulary.
// Without the in-engine vocabulary check these cases would pass — so a failure
// here without the fix is the proof (doctrine §2.3).
func TestFacetsValidate_RejectsEachOutOfVocabularyAxis(t *testing.T) {
	cases := map[string]Facets{
		"nature":          {Nature: "bogus"},
		"scope_level":     {ScopeLevel: "galaxy"},
		"trust_tier":      {TrustTier: "trust-me"},
		"provenance":      {Provenance: "vibes"},
		"confidentiality": {Confidentiality: "kinda-secret"},
		"applicability":   {Applicability: "maybe"},
	}
	for axis, f := range cases {
		t.Run(axis, func(t *testing.T) {
			err := f.Validate()
			if err == nil {
				t.Fatalf("invalid %s unexpectedly passed validation", axis)
			}
			if !strings.Contains(err.Error(), axis) {
				t.Fatalf("error %q does not mention offending axis %q", err, axis)
			}
		})
	}
}

func TestFacetsValidate_RejectsPresentButEmptyTermLists(t *testing.T) {
	// CUE: discipline_role?: [#FacetTermRef, ...#FacetTermRef] — present ⇒ ≥1.
	for _, axis := range []string{"discipline_role", "activity"} {
		t.Run(axis, func(t *testing.T) {
			var f Facets
			if axis == "discipline_role" {
				f.DisciplineRole = []string{}
			} else {
				f.Activity = []string{}
			}
			if err := f.Validate(); err == nil {
				t.Fatalf("present-but-empty %s unexpectedly passed", axis)
			}
		})
	}
}

func TestFacetsValidate_RejectsEmptyTerm(t *testing.T) {
	f := Facets{DisciplineRole: []string{"legal.owner", ""}}
	if err := f.Validate(); err == nil {
		t.Fatal("empty discipline_role term unexpectedly passed")
	}
	f = Facets{VocabularyRefs: []string{""}}
	if err := f.Validate(); err == nil {
		t.Fatal("empty vocabulary_ref unexpectedly passed")
	}
}

func TestDeriveFacets_IsHonest(t *testing.T) {
	// Approved + source-anchored atom: indicative (never certified), source_backed.
	approved := Atom{
		Type:        AtomRule,
		ContentHash: "abc",
		SourceSpan:  SourceSpan{File: "src.md"},
		ReviewState: ReviewApproved,
	}
	f := DeriveFacets(approved)
	if f.TrustTier == "certified" {
		t.Fatal("derivation must never claim certified (doctrine §2.2)")
	}
	if f.TrustTier != "indicative" {
		t.Fatalf("approved atom trust_tier = %q, want indicative", f.TrustTier)
	}
	if f.Provenance != "source_backed" {
		t.Fatalf("source-anchored atom provenance = %q, want source_backed", f.Provenance)
	}
	if f.ScopeLevel != "atom" {
		t.Fatalf("scope_level = %q, want atom", f.ScopeLevel)
	}
	if err := f.Validate(); err != nil {
		t.Fatalf("derived facets fail their own contract: %v", err)
	}

	// Draft, unanchored atom: unverified + derived.
	draft := Atom{Type: AtomRule, ReviewState: ReviewDraft}
	df := DeriveFacets(draft)
	if df.TrustTier != "unverified" {
		t.Fatalf("draft atom trust_tier = %q, want unverified", df.TrustTier)
	}
	if df.Provenance != "derived" {
		t.Fatalf("unanchored atom provenance = %q, want derived", df.Provenance)
	}
}

func TestDeriveFacets_NatureMapping(t *testing.T) {
	want := map[AtomType]FacetNature{
		AtomRule:       "rule",
		AtomClause:     "rule",
		AtomDefinition: "definition",
		AtomListItem:   "rule",
		AtomCodeBlock:  "evidence",
		AtomTable:      "evidence",
		AtomMeta:       "context",
	}
	for atomType, nature := range want {
		got := DeriveFacets(Atom{Type: atomType}).Nature
		if got != nature {
			t.Errorf("nature for %q = %q, want %q", atomType, got, nature)
		}
		if !facetNatures[got] {
			t.Errorf("derived nature %q is not in the contract vocabulary", got)
		}
	}
}

func TestAtomize_EmitsFacetsOnlyOnRequest(t *testing.T) {
	src := "# Title\n\nA governed answer must cite its source or abstain.\n"
	ast := ParseMarkdown(src)

	// Default: zero regression — no facets on the wire.
	plain := Atomize(ast, AtomizeOptions{SourceFile: "doc.md"})
	if len(plain.Atoms) == 0 {
		t.Fatal("expected atoms")
	}
	for _, a := range plain.Atoms {
		if a.Facets != nil {
			t.Fatalf("atom %s carried facets without EmitFacets (regression)", a.ID)
		}
	}

	// Opt-in: every atom carries valid facets.
	faceted := Atomize(ast, AtomizeOptions{SourceFile: "doc.md", EmitFacets: true})
	for _, a := range faceted.Atoms {
		if a.Facets == nil {
			t.Fatalf("atom %s missing facets with EmitFacets", a.ID)
		}
		if err := a.Facets.Validate(); err != nil {
			t.Fatalf("emitted facets invalid for atom %s: %v", a.ID, err)
		}
	}
}

func TestProjectAtoms_CarriesFacets(t *testing.T) {
	atom := Atom{
		ID:           "A-1",
		CanonicalRef: "doc/x",
		Type:         AtomRule,
		Text:         "A governed answer must cite its source or abstain entirely.",
		ContentHash:  "deadbeef",
		SourceSpan:   SourceSpan{File: "doc.md"},
		ReviewState:  ReviewApproved,
	}
	f := DeriveFacets(atom)
	atom.Facets = &f

	res := ProjectAtoms([]Atom{atom}, DefaultProjectionConfig())
	if len(res.Chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d (rejected: %v)", len(res.Chunks), res.Rejected)
	}
	if res.Chunks[0].Facets == nil {
		t.Fatal("projected chunk dropped facets")
	}
	if res.Chunks[0].Facets.Nature != "rule" {
		t.Fatalf("chunk facet nature = %q, want rule", res.Chunks[0].Facets.Nature)
	}
}

// --- CKM-02 lens, consumed in-engine ---------------------------------------

func TestApplyLens_NoPredicateKeepsEverything(t *testing.T) {
	d := ApplyLens(Facets{Nature: "rule"}, KnowledgeLens{ID: "empty"})
	if !d.Included {
		t.Fatalf("lens with no predicate should include all, got %+v", d)
	}
}

func TestApplyLens_ExcludeByFacet(t *testing.T) {
	lens := KnowledgeLens{
		ID: "drop-unverified",
		Exclude: &LensPredicate{
			AnyOf: []LensFacetSelection{{TrustTier: "unverified"}},
		},
	}
	out := ApplyLens(Facets{TrustTier: "unverified"}, lens)
	if out.Included {
		t.Fatal("unverified candidate should be excluded")
	}
	if out.Reason != "excluded_by_facets.trust_tier" {
		t.Fatalf("reason = %q, want excluded_by_facets.trust_tier", out.Reason)
	}
	keep := ApplyLens(Facets{TrustTier: "certified"}, lens)
	if !keep.Included {
		t.Fatal("certified candidate should survive an unverified-exclusion lens")
	}
}

func TestApplyLens_IncludeAllOfAndAnyOf(t *testing.T) {
	lens := KnowledgeLens{
		ID: "architect-permit-review",
		Include: &LensPredicate{
			AllOf: []LensFacetSelection{{Applicability: "applicable"}},
			AnyOf: []LensFacetSelection{
				{DisciplineRole: []string{"architecture.lead"}},
				{Activity: []string{"permit.review"}},
			},
		},
	}
	// Matches all_of (applicable) and one any_of (activity).
	in := ApplyLens(Facets{Applicability: "applicable", Activity: []string{"permit.review"}}, lens)
	if !in.Included {
		t.Fatalf("candidate matching all_of+any_of should be included, got %+v", in)
	}
	// Fails all_of (not applicable).
	out := ApplyLens(Facets{Applicability: "not_applicable", Activity: []string{"permit.review"}}, lens)
	if out.Included {
		t.Fatal("candidate failing all_of should be excluded")
	}
	// Passes all_of but matches no any_of branch.
	none := ApplyLens(Facets{Applicability: "applicable", Activity: []string{"unrelated.task"}}, lens)
	if none.Included {
		t.Fatal("candidate matching no any_of branch should be excluded")
	}
}

func TestApplyLens_IncludeNoneOf(t *testing.T) {
	lens := KnowledgeLens{
		ID: "no-secret",
		Include: &LensPredicate{
			NoneOf: []LensFacetSelection{{Confidentiality: "secret"}},
		},
	}
	if ApplyLens(Facets{Confidentiality: "secret"}, lens).Included {
		t.Fatal("secret candidate should be excluded by none_of")
	}
	if !ApplyLens(Facets{Confidentiality: "public"}, lens).Included {
		t.Fatal("public candidate should pass none_of")
	}
}

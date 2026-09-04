package atomization

// CKM-H3 — Facets in the Go engine.
//
// This file makes the CKM-01 facet contract (specs/facets.cue), the CKM-02
// knowledge lens (specs/knowledge-lens.cue), and CKM-03 promotion live INSIDE
// the Go engine rather than only in CUE specs and Python sidecars. The audit
// (#518) found that Atom/Chunk carried no Facets field and the engine never
// consumed faceting/lens/promotion.
//
// The Go types here mirror specs/facets.cue one-to-one. Validate() enforces the
// same vocabularies the CUE contract enforces, so the contract is checked
// in-engine (no `cue` binary required at runtime). A conformance test
// (facets_cue_conformance_test.go) cross-checks engine output against the real
// CUE contract when `cue` is available.
//
// Everything here is additive: facets live under the open metadata of #Atom /
// #Chunk, and atoms/chunks without facets remain byte-identical on the wire
// (pointer + omitempty). Zero regression is a non-negotiable (doctrine §2.1).

import (
	"fmt"
	"sort"
)

// FacetNature mirrors #FacetNature in specs/facets.cue.
type FacetNature string

// FacetScopeLevel mirrors #FacetScopeLevel.
type FacetScopeLevel string

// FacetTrustTier mirrors #FacetTrustTier.
type FacetTrustTier string

// FacetProvenance mirrors #FacetProvenance.
type FacetProvenance string

// FacetConfidentiality mirrors #FacetConfidentiality.
type FacetConfidentiality string

// FacetApplicability mirrors #FacetApplicability.
type FacetApplicability string

// Controlled vocabularies — kept in lockstep with specs/facets.cue. Any change
// here is a contract change and requires a schema_version bump + migration
// (doctrine §2.1).
var (
	facetNatures = map[FacetNature]bool{
		"rule": true, "definition": true, "obligation": true, "permission": true,
		"prohibition": true, "condition": true, "exception": true, "calculation": true,
		"evidence": true, "governance": true, "metier": true, "context": true,
	}
	facetScopeLevels = map[FacetScopeLevel]bool{
		"source": true, "structure": true, "atom": true, "chunk": true,
		"domain": true, "product": true, "release": true,
	}
	facetTrustTiers = map[FacetTrustTier]bool{
		"certified": true, "indicative": true, "unverified": true,
	}
	facetProvenances = map[FacetProvenance]bool{
		"source_backed": true, "derived": true, "inferred": true,
		"external_attested": true, "user_promoted": true,
	}
	facetConfidentialities = map[FacetConfidentiality]bool{
		"public": true, "internal": true, "restricted": true, "secret": true,
		"licensed_restricted": true, "customer_confidential": true,
	}
	facetApplicabilities = map[FacetApplicability]bool{
		"applicable": true, "partially_applicable": true, "not_applicable": true,
		"blocked": true, "unknown": true,
	}
)

// Facets mirrors #Facets in specs/facets.cue. All axes are optional; an atom or
// chunk with no facets is valid against the base atomization spine.
//
// RiskTier is the third open-term axis (VRC-22, #565). It grades how dangerous
// the regulated thing itself is — which TrustTier (how far an artifact may be
// trusted) and Applicability (whether a rule applies) do not say.
type Facets struct {
	Nature          FacetNature          `json:"nature,omitempty"`
	DisciplineRole  []string             `json:"discipline_role,omitempty"`
	Activity        []string             `json:"activity,omitempty"`
	RiskTier        []string             `json:"risk_tier,omitempty"`
	ScopeLevel      FacetScopeLevel      `json:"scope_level,omitempty"`
	TrustTier       FacetTrustTier       `json:"trust_tier,omitempty"`
	Provenance      FacetProvenance      `json:"provenance,omitempty"`
	Confidentiality FacetConfidentiality `json:"confidentiality,omitempty"`
	Applicability   FacetApplicability   `json:"applicability,omitempty"`
	VocabularyRefs  []string             `json:"vocabulary_refs,omitempty"`
	Extensions      map[string]any       `json:"extensions,omitempty"`
}

// IsZero reports whether no facet axis is set. Zero facets serialize to nothing
// and are valid (the no-facet case).
func (f Facets) IsZero() bool {
	return f.Nature == "" && len(f.DisciplineRole) == 0 && len(f.Activity) == 0 &&
		len(f.RiskTier) == 0 &&
		f.ScopeLevel == "" && f.TrustTier == "" && f.Provenance == "" &&
		f.Confidentiality == "" && f.Applicability == "" &&
		len(f.VocabularyRefs) == 0 && len(f.Extensions) == 0
}

// Validate enforces the #Facets contract in-engine: every set scalar axis must
// be a member of its controlled vocabulary, and the term-list axes
// (discipline_role, activity, risk_tier) must, when present, hold at least one
// non-empty term — exactly as `[#FacetTermRef, ...#FacetTermRef]` requires in CUE.
func (f Facets) Validate() error {
	if f.Nature != "" && !facetNatures[f.Nature] {
		return fmt.Errorf("facets: invalid nature %q", f.Nature)
	}
	if f.ScopeLevel != "" && !facetScopeLevels[f.ScopeLevel] {
		return fmt.Errorf("facets: invalid scope_level %q", f.ScopeLevel)
	}
	if f.TrustTier != "" && !facetTrustTiers[f.TrustTier] {
		return fmt.Errorf("facets: invalid trust_tier %q", f.TrustTier)
	}
	if f.Provenance != "" && !facetProvenances[f.Provenance] {
		return fmt.Errorf("facets: invalid provenance %q", f.Provenance)
	}
	if f.Confidentiality != "" && !facetConfidentialities[f.Confidentiality] {
		return fmt.Errorf("facets: invalid confidentiality %q", f.Confidentiality)
	}
	if f.Applicability != "" && !facetApplicabilities[f.Applicability] {
		return fmt.Errorf("facets: invalid applicability %q", f.Applicability)
	}
	// CUE: discipline_role?: [#FacetTermRef, ...#FacetTermRef] — present ⇒ ≥1 term.
	if f.DisciplineRole != nil {
		if err := validateTermList("discipline_role", f.DisciplineRole); err != nil {
			return err
		}
	}
	if f.Activity != nil {
		if err := validateTermList("activity", f.Activity); err != nil {
			return err
		}
	}
	if f.RiskTier != nil {
		if err := validateTermList("risk_tier", f.RiskTier); err != nil {
			return err
		}
	}
	// CUE: vocabulary_refs?: [...#FacetTermRef] — zero or more, each non-empty.
	for i, ref := range f.VocabularyRefs {
		if ref == "" {
			return fmt.Errorf("facets: vocabulary_refs[%d] is empty", i)
		}
	}
	return nil
}

func validateTermList(axis string, terms []string) error {
	if len(terms) == 0 {
		return fmt.Errorf("facets: %s is present but empty (contract requires at least one term)", axis)
	}
	for i, t := range terms {
		if t == "" {
			return fmt.Errorf("facets: %s[%d] is empty", axis, i)
		}
	}
	return nil
}

// natureForAtomType maps the engine's AtomType onto a facet nature. The mapping
// is intentionally conservative: ambiguous block kinds resolve to "context"
// rather than over-claiming a regulatory nature.
func natureForAtomType(t AtomType) FacetNature {
	switch t {
	case AtomRule:
		return "rule"
	case AtomClause:
		return "rule"
	case AtomDefinition:
		return "definition"
	case AtomListItem:
		return "rule"
	case AtomCodeBlock:
		return "evidence"
	case AtomTable:
		return "evidence"
	case AtomMeta:
		return "context"
	default:
		return "context"
	}
}

// DeriveFacets emits facets for an atom from what the engine can actually prove.
//
// Honesty discipline (doctrine §2.2/§2.4): derived atoms are never tagged
// `certified` — certification is an out-of-band act, not a side effect of
// atomization. The highest tier derivation will claim is `indicative` (for an
// approved atom); everything else is `unverified`. Provenance is `source_backed`
// only when the atom is anchored to a real source span + content hash.
func DeriveFacets(atom Atom) Facets {
	f := Facets{
		Nature:     natureForAtomType(atom.Type),
		ScopeLevel: "atom",
	}

	if atom.ContentHash != "" && atom.SourceSpan.File != "" {
		f.Provenance = "source_backed"
	} else {
		f.Provenance = "derived"
	}

	switch atom.ReviewState {
	case ReviewApproved:
		f.TrustTier = "indicative"
	default:
		f.TrustTier = "unverified"
	}

	return f
}

// --- CKM-02 knowledge lens, consumed in-engine -----------------------------

// LensFacetSelection mirrors #LensFacetSelection: a conjunction of axis
// constraints. A selection matches a candidate when, for every axis named in
// the selection, the candidate's facet values intersect the selection's values.
type LensFacetSelection struct {
	Nature          FacetNature          `json:"nature,omitempty"`
	DisciplineRole  []string             `json:"discipline_role,omitempty"`
	Activity        []string             `json:"activity,omitempty"`
	RiskTier        []string             `json:"risk_tier,omitempty"`
	ScopeLevel      FacetScopeLevel      `json:"scope_level,omitempty"`
	TrustTier       FacetTrustTier       `json:"trust_tier,omitempty"`
	Provenance      FacetProvenance      `json:"provenance,omitempty"`
	Confidentiality FacetConfidentiality `json:"confidentiality,omitempty"`
	Applicability   FacetApplicability   `json:"applicability,omitempty"`
}

// LensPredicate mirrors #KnowledgeLensPredicate.
type LensPredicate struct {
	AllOf  []LensFacetSelection `json:"all_of,omitempty"`
	AnyOf  []LensFacetSelection `json:"any_of,omitempty"`
	NoneOf []LensFacetSelection `json:"none_of,omitempty"`
}

// KnowledgeLens mirrors #KnowledgeLens: an inclusion/exclusion scope predicate
// over facets. With no lens the engine keeps every candidate (the documented
// default_behavior "include_all_when_no_lens").
type KnowledgeLens struct {
	ID          string         `json:"id"`
	Description string         `json:"description,omitempty"`
	Include     *LensPredicate `json:"include,omitempty"`
	Exclude     *LensPredicate `json:"exclude,omitempty"`
}

// LensDecision records the in/out verdict for one candidate.
type LensDecision struct {
	Included bool
	Reason   string
}

// axisValues returns the candidate's values on a given axis as a set. Scalar
// axes yield a single-element set; the term-list axes yield the whole list.
func (f Facets) axisValues(axis string) []string {
	switch axis {
	case "nature":
		return nonEmpty(string(f.Nature))
	case "scope_level":
		return nonEmpty(string(f.ScopeLevel))
	case "trust_tier":
		return nonEmpty(string(f.TrustTier))
	case "provenance":
		return nonEmpty(string(f.Provenance))
	case "confidentiality":
		return nonEmpty(string(f.Confidentiality))
	case "applicability":
		return nonEmpty(string(f.Applicability))
	case "discipline_role":
		return f.DisciplineRole
	case "activity":
		return f.Activity
	case "risk_tier":
		return f.RiskTier
	default:
		return nil
	}
}

// selectionAxes returns the (axis, expectedValues) pairs named by a selection.
func (s LensFacetSelection) axes() map[string][]string {
	out := map[string][]string{}
	if s.Nature != "" {
		out["nature"] = []string{string(s.Nature)}
	}
	if s.ScopeLevel != "" {
		out["scope_level"] = []string{string(s.ScopeLevel)}
	}
	if s.TrustTier != "" {
		out["trust_tier"] = []string{string(s.TrustTier)}
	}
	if s.Provenance != "" {
		out["provenance"] = []string{string(s.Provenance)}
	}
	if s.Confidentiality != "" {
		out["confidentiality"] = []string{string(s.Confidentiality)}
	}
	if s.Applicability != "" {
		out["applicability"] = []string{string(s.Applicability)}
	}
	if len(s.DisciplineRole) > 0 {
		out["discipline_role"] = s.DisciplineRole
	}
	if len(s.Activity) > 0 {
		out["activity"] = s.Activity
	}
	if len(s.RiskTier) > 0 {
		out["risk_tier"] = s.RiskTier
	}
	return out
}

func selectionMatches(f Facets, sel LensFacetSelection) bool {
	axes := sel.axes()
	if len(axes) == 0 {
		return false
	}
	for axis, expected := range axes {
		if !intersects(f.axisValues(axis), expected) {
			return false
		}
	}
	return true
}

// firstMatchingAxis returns the (sorted-deterministic) axis name of the first
// selection that matches, mirroring the Python sidecar's reason strings.
func firstMatchingAxis(f Facets, selections []LensFacetSelection) (string, bool) {
	for _, sel := range selections {
		if selectionMatches(f, sel) {
			axes := sel.axes()
			names := make([]string, 0, len(axes))
			for name := range axes {
				names = append(names, name)
			}
			sort.Strings(names)
			if len(names) > 0 {
				return names[0], true
			}
		}
	}
	return "", false
}

// ApplyLens decides whether a candidate's facets keep it in retrieval scope.
// The semantics mirror scripts/ckm_knowledge_lens_filter.py exactly so the
// engine and the CI sidecar agree:
//   - exclude.any_of match            → drop (excluded_by_facets.<axis>)
//   - include.all_of present, not all → drop (not_selected_by_lens)
//   - include.any_of present, none    → drop
//   - include.none_of present, any    → drop
//   - otherwise                       → keep
func ApplyLens(f Facets, lens KnowledgeLens) LensDecision {
	if lens.Exclude != nil {
		if axis, ok := firstMatchingAxis(f, lens.Exclude.AnyOf); ok {
			return LensDecision{Included: false, Reason: "excluded_by_facets." + axis}
		}
	}
	if lens.Include != nil {
		inc := lens.Include
		if len(inc.AllOf) > 0 {
			for _, sel := range inc.AllOf {
				if !selectionMatches(f, sel) {
					return LensDecision{Included: false, Reason: "not_selected_by_lens"}
				}
			}
		}
		if len(inc.AnyOf) > 0 {
			if _, ok := firstMatchingAxis(f, inc.AnyOf); !ok {
				return LensDecision{Included: false, Reason: "not_selected_by_lens"}
			}
		}
		if len(inc.NoneOf) > 0 {
			if _, ok := firstMatchingAxis(f, inc.NoneOf); ok {
				return LensDecision{Included: false, Reason: "not_selected_by_lens"}
			}
		}
	}
	return LensDecision{Included: true}
}

func nonEmpty(v string) []string {
	if v == "" {
		return nil
	}
	return []string{v}
}

func intersects(a, b []string) bool {
	set := make(map[string]bool, len(a))
	for _, x := range a {
		set[x] = true
	}
	for _, y := range b {
		if set[y] {
			return true
		}
	}
	return false
}

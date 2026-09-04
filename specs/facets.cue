package nomos

import "strings"

// CKM-01 optional multidimensional facets.
//
// Facets are intentionally validated through opt-in shapes and live under the
// open metadata field of #Atom/#Chunk. Existing atoms and chunks without facets
// remain valid against the base atomization spine.

#FacetTermRef: string & strings.MinRunes(1)

#FacetNature:
	"rule" |
	"definition" |
	"obligation" |
	"permission" |
	"prohibition" |
	"condition" |
	"exception" |
	"calculation" |
	"evidence" |
	"governance" |
	"metier" |
	"context"

#FacetScopeLevel:
	"source" |
	"structure" |
	"atom" |
	"chunk" |
	"domain" |
	"product" |
	"release"

#FacetTrustTier:
	"certified" |
	"indicative" |
	"unverified"

#FacetProvenance:
	"source_backed" |
	"derived" |
	"inferred" |
	"external_attested" |
	"user_promoted"

#FacetConfidentiality:
	"public" |
	"internal" |
	"restricted" |
	"secret" |
	"licensed_restricted" |
	"customer_confidential"

#FacetApplicability:
	"applicable" |
	"partially_applicable" |
	"not_applicable" |
	"blocked" |
	"unknown"

#FacetAxisVocabulary: {
	axis: "nature" | "discipline_role" | "activity" | "risk_tier" | "scope_level" | "trust_tier" | "provenance" | "confidentiality" | "applicability"
	terms: [#FacetVocabularyTerm, ...#FacetVocabularyTerm]
}

#FacetVocabularyTerm: {
	id: #FacetTermRef
	prefLabel: string & strings.MinRunes(1)
	definition?: string
	broader?: #FacetTermRef
	exactMatch?: [...#FacetTermRef]
}

#Facets: {
	nature?: #FacetNature
	discipline_role?: [#FacetTermRef, ...#FacetTermRef]
	activity?: [#FacetTermRef, ...#FacetTermRef]

	// risk_tier (VRC-22, #565) — the third OPEN-term axis. Added for the EU AI
	// Act vertical, whose central concept is a risk classification that none of
	// the closed axes carries: trust_tier grades how far an artifact may be
	// trusted, applicability grades whether a rule applies. Neither says how
	// dangerous the regulated thing itself is. Packs own the terms; the core
	// owns the axis, exactly as for discipline_role and activity. This is the
	// one core change VRC-22 records in its D6 measurement.
	risk_tier?: [#FacetTermRef, ...#FacetTermRef]

	scope_level?: #FacetScopeLevel
	trust_tier?: #FacetTrustTier
	provenance?: #FacetProvenance
	confidentiality?: #FacetConfidentiality
	applicability?: #FacetApplicability

	// Domain packs may add SKOS-backed axes without changing #Atom/#Chunk.
	vocabulary_refs?: [...#FacetTermRef]
	extensions?: {[string]: _}
}

#FacetedAtom: #Atom & {
	metadata: {
		facets: #Facets
		...
	}
}

#FacetedChunk: #Chunk & {
	metadata: {
		facets: #Facets
		...
	}
}

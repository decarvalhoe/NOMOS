package nomos

import "strings"

// CKM-02 optional knowledge lens.
//
// A lens is a base-level retrieval scope predicate over CKM-01 facets. Without
// a selected lens, the existing behavior is preserved and all candidate chunks
// remain in scope.

#LensRef: string & strings.MinRunes(1)

#LensFacetSelection: {
	nature?:            #FacetNature
	discipline_role?:   [#FacetTermRef, ...#FacetTermRef]
	activity?:          [#FacetTermRef, ...#FacetTermRef]
	risk_tier?:         [#FacetTermRef, ...#FacetTermRef]
	scope_level?:       #FacetScopeLevel
	trust_tier?:        #FacetTrustTier
	provenance?:        #FacetProvenance
	confidentiality?:   #FacetConfidentiality
	applicability?:     #FacetApplicability
}

#KnowledgeLensPredicate: {
	all_of?: [...#LensFacetSelection]
	any_of?: [...#LensFacetSelection]
	none_of?: [...#LensFacetSelection]
}

#KnowledgeLens: {
	id:               #LensRef
	description:      string & strings.MinRunes(1)
	default_behavior: "include_all_when_no_lens"
	include?:         #KnowledgeLensPredicate
	exclude?:         #KnowledgeLensPredicate
}

#KnowledgeLensPreset: {
	id:                #LensRef
	role:              #FacetTermRef
	activity:          #FacetTermRef
	lens_id:           #LensRef
	activation_reason: string & strings.MinRunes(1)
}

#KnowledgeLensBundle: {
	record_type:      "ckm_knowledge_lens_bundle"
	schema_version:   string & strings.MinRunes(1)
	default_behavior: "include_all_when_no_lens"
	epistemology: {
		// Applicability is represented as source-backed/objective metadata.
		applicability: "objective_fact"
		// Activation is represented as a user/customer lens choice.
		activation: "subjective_choice"
	}
	lenses:  [#KnowledgeLens, ...#KnowledgeLens]
	presets: [#KnowledgeLensPreset, ...#KnowledgeLensPreset]
}

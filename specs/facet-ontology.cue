package nomos

import "strings"

// CKM-12 ontology pattern for facet axes.
//
// This contract documents how core facet axes are grounded in BFO, IOF Core,
// and a domain pack. It is additive and does not tighten #Atom/#Chunk.

#OntologyIRI: string & strings.MinRunes(1)

#OntologyAnchor: {
	standard: string & strings.MinRunes(1)
	iri:      #OntologyIRI
	version?: string
	note?:    string
}

#OntologyTerm: {
	id:    string & strings.MinRunes(1)
	label: string & strings.MinRunes(1)
	iri:   #OntologyIRI
	maps_to: {
		bfo:      #OntologyIRI
		iof_core: #OntologyIRI
		domain?:  #OntologyIRI
	}
}

#FacetOntologyAxis: {
	id:        "nature" | "discipline_role" | "activity" | "risk_tier" | "scope_level" | "trust_tier" | "provenance" | "confidentiality" | "applicability"
	label:     string & strings.MinRunes(1)
	root:      #OntologyIRI
	iof_class: #OntologyIRI
	terms:     [#OntologyTerm, ...#OntologyTerm]
}

#ODPPattern: {
	kind: "obligation" | "process" | "evidence"
	intent: string & strings.MinRunes(1)
	bfo_anchor:      #OntologyIRI
	iof_core_anchor: #OntologyIRI
	facet_axis:      string & strings.MinRunes(1)
	required_facets: [string & strings.MinRunes(1), ...string]
}

#FacetOntology: {
	schema_version: "ckm-facet-ontology-v1"
	anchors: {
		bfo: #OntologyAnchor & {
			standard: "BFO"
			iri: =~"^http://purl\\.obolibrary\\.org/obo/BFO_"
		}
		iof_core: #OntologyAnchor & {
			standard: "IOF Core"
			iri: =~"^https://spec\\.industrialontologies\\.org/ontology/core/Core/"
		}
		domain_pack: #OntologyAnchor & {
			standard: "domain-pack"
		}
	}
	facet_axes: [#FacetOntologyAxis, ...#FacetOntologyAxis]
	odp_patterns: {
		obligation: #ODPPattern & {kind: "obligation"}
		process:    #ODPPattern & {kind: "process"}
		evidence:   #ODPPattern & {kind: "evidence"}
	}
	orthogonality: {
		owl_construct: "owl:disjointUnionOf"
		disjoint_axes: [string & strings.MinRunes(1), ...string]
		validation: "ckm_facet_ontology_validate.py"
	}
	claim_boundary: string & strings.MinRunes(1)
}

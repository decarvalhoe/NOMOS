package nomos

import "strings"

// Domain cartography — docs/49 §2.1.
//
// What a consumer domain ACTUALLY holds, sub-corpus by sub-corpus, layer by
// layer, each layer verified independently. The rule this contract enforces is
// the one learned from a neighbouring legal RAG (docs/49): a layer is never
// inferred from another. A text can be indexed and not enriched, enriched and
// not linked to the graph, present in the source manifest and absent from the
// index. A layer that was not verified says so and carries no number.
//
// Three kinds of domain are distinguished on purpose:
//   - own_corpus: the domain has its own sub-corpora (a code, a court's case law);
//   - phantom: a real legal/business domain with no technical collection of its
//     own, hosted entirely inside neighbouring domains — kept visible so it
//     never disappears from the commercial or legal field of view;
//   - transversal_base: a text shared by construction across the domains of a
//     jurisdiction (a civil code, a constitution), never duplicated.
//
// Claim boundary: a cartography is a verified inventory of presence and
// coverage. It measures no retrieval quality, no answer quality, and it does
// not make a domain "supported" — the support model and the domain pack do.

#CartographySchemaVersion: "nomos-domain-cartography-v1"

#CartographyLayerName: "source" | "index" | "enrichment" | "graph"

#CartographyDomainKind: "own_corpus" | "phantom" | "transversal_base"

// How a layer was verified. `not_verified` is a legitimate, loud state.
#CartographyVerification: "direct_count" | "direct_query" | "manifest" | "not_verified"

#CartographyDate: =~"^\\d{4}-\\d{2}-\\d{2}$"

#CartographyLayer: {
	layer:       #CartographyLayerName
	verified_by: #CartographyVerification
	// Where the number comes from: a manifest path, a query, a counter name.
	source_ref?: string & strings.MinRunes(1)
	note?:       string & strings.MinRunes(1)
} & ({
	// Not verified: no date, no count, no ratio — nothing to read as a fact.
	verified_by:     "not_verified"
	verified_at?:    _|_
	count?:          _|_
	coverage_ratio?: _|_
} | {
	verified_by:     "direct_count" | "direct_query" | "manifest"
	verified_at:     #CartographyDate
	count:           int & >=0
	coverage_ratio?: number & >=0 & <=1
})

#CartographyLayers: {
	source:     #CartographyLayer & {layer: "source"}
	index:      #CartographyLayer & {layer: "index"}
	enrichment: #CartographyLayer & {layer: "enrichment"}
	graph:      #CartographyLayer & {layer: "graph"}
}

#CartographySubcorpus: {
	id:               string & strings.MinRunes(1)
	title:            string & strings.MinRunes(1)
	source_authority: string & strings.MinRunes(1)
	// Units of the sub-corpus as the source counts them (texts, decisions...).
	unit_kind:  string & strings.MinRunes(1)
	unit_count: int & >=0
	layers:     #CartographyLayers
}

#CartographyDomain: {
	id:           string & strings.MinRunes(1)
	title:        string & strings.MinRunes(1)
	jurisdiction: string & strings.MinRunes(2)
	kind:         #CartographyDomainKind
	note?:        string & strings.MinRunes(1)
} & ({
	kind:      "own_corpus"
	subcorpora: [#CartographySubcorpus, ...#CartographySubcorpus]
	// A real domain fed by federation from separate domains names them.
	federated_with?: [...string & strings.MinRunes(1)]
} | {
	// A phantom domain owns nothing: it names where its content lives.
	kind:        "phantom"
	hosted_in: [string & strings.MinRunes(1), ...string & strings.MinRunes(1)]
	subcorpora?: _|_
} | {
	// A transversal base is shared, never duplicated, and says with whom.
	kind:             "transversal_base"
	shared_with: [string & strings.MinRunes(1), ...string & strings.MinRunes(1)]
	never_duplicated: true
	subcorpora: [#CartographySubcorpus, ...#CartographySubcorpus]
})

#DomainCartography: {
	schema_version: #CartographySchemaVersion
	cartography_id: string & strings.MinRunes(1)
	as_of:          #CartographyDate
	claim_boundary: string & strings.MinRunes(20)
	domains: [#CartographyDomain, ...#CartographyDomain]
}

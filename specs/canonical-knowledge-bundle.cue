package nomos

import "strings"

// #CanonicalKnowledgeBundle is the versioned downstream handoff contract for
// Canonical Knowledge Mesh consumers. It is deliberately decoupled from the
// NOMOS codebase and carries faceted feed nodes, RAG metadata, trace manifest,
// and attestation in one inspectable artifact.
#CanonicalKnowledgeBundle: {
	schema_version: "ckm-bundle-v1"
	bundle_id:      =~"^[a-z0-9][a-z0-9._-]*$"
	generated_at:   string & strings.MinRunes(1)
	producer:       string & strings.MinRunes(1)
	claim_boundary: string & strings.MinRunes(1)

	feeds: [#BundleFeed, ...#BundleFeed]
	rag_metadata: [...#BundleRAGMetadata]
	trace_manifest: #NomosTraceManifest
	attestation: #BundleAttestation
}

#BundleFeed: {
	feed_id: string & strings.MinRunes(1)
	format:  string & strings.MinRunes(1)

	// version is the OPTIONAL feed content version a consumer pins imports to.
	// Absent on a feed that does not version its payload; when present the emitter
	// derives it deterministically from bundle_id@generated_at so re-runs over the
	// same corpus produce the same version. Additive: a domain feed with no
	// version stays valid against this contract.
	version?: string & strings.MinRunes(1)

	// jurisdiction is the OPTIONAL legal scope a feed's nodes apply to. NOMOS has
	// no native jurisdiction concept, so this is populated only when the emitter
	// is told (--country/--canton/--commune). A domain corpus with no jurisdiction
	// (e.g. AI governance, GxP) omits it entirely and stays valid.
	jurisdiction?: #BundleJurisdiction

	nodes: [#BundleNode, ...#BundleNode]
}

// #BundleJurisdiction is the legal-scope facet a consumer (Aedifica) maps onto a
// commune/canton permit context. Every field is optional so a partially-known
// scope (canton only, no commune) is expressible.
#BundleJurisdiction: {
	country?: string & strings.MinRunes(1)
	canton?:  string & strings.MinRunes(1)
	commune?: string & strings.MinRunes(1)
}

#BundleNode: {
	node_id:      =~"^[A-Z0-9][A-Z0-9._-]*$"
	text:         string & strings.MinRunes(1)
	source_path:  string & strings.MinRunes(1)
	source_hash:  =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	span:         #BundleSpan
	parent_chain: [...string]
	facets:       #Facets
}

// #BundleSpan locates a node in its source. The CANONICAL line-span form NOMOS
// emits is `start_line`/`end_line` (1-based, inclusive). Consumers that model
// spans as `start`/`end` must read from start_line/end_line — they are the
// source of truth. Byte offsets and a free-form locator are optional refinements.
// (Aedifica's importer already accepts both `start`/`end` and
// `start_line`/`end_line`; NOMOS commits to the line-span names.)
#BundleSpan: {
	start_line?: int & >=0
	end_line?:   int & >=0
	start_byte?: int & >=0
	end_byte?:   int & >=0
	locator?:    string
}

#BundleRAGMetadata: {
	node_id:        =~"^[A-Z0-9][A-Z0-9._-]*$"
	chunk_id:       string & strings.MinRunes(1)
	source_path:    string & strings.MinRunes(1)
	source_hash:    =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	parent_chain:   [...string]
	facets?:        #Facets
	embedding_model?: string
	embedding_dim?:   int & >=1
}

#BundleAttestation: {
	"_type":       "https://in-toto.io/Statement/v1"
	subject:       [#BundleAttestationSubject, ...#BundleAttestationSubject]
	predicateType: string & strings.MinRunes(1)
	predicate:     {...}
}

#BundleAttestationSubject: {
	name:   string & strings.MinRunes(1)
	digest: [string]: =~"^[A-Fa-f0-9]+$"
}

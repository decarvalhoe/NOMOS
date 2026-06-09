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
	nodes:   [#BundleNode, ...#BundleNode]
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

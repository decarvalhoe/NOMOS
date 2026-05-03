package nomos

import "strings"

// #RBOKRuntimeImportContract defines the handoff schema from Nomos
// multi-layer runtime feed to the RBOK Engine database importer.
// The engine consumes this contract to populate its relational model.

// #LayerKind identifies the business purpose of a source layer.
#LayerKind:
	"referentiel" |
	"domaine" |
	"meta" |
	"override"

// #LayerProvenance records scan metadata for one source layer.
#LayerProvenance: {
	id:          =~"^[a-z0-9][a-z0-9._-]*$"
	kind:        #LayerKind
	path:        string & strings.MinRunes(1)
	domain:      string & strings.MinRunes(1)
	priority:    int & >=0
	node_count:  int & >=0
	source_hash: =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
}

// #RuntimeFeedNode is a single node in the unified feed with layer provenance.
#RuntimeFeedNode: {
	node_id:       =~"^[A-Z0-9][A-Z0-9._-]*$"
	document_id:   =~"^[A-Z0-9][A-Z0-9._-]*$"
	node_type:     #LawbookNodeType
	canonical_ref: string & strings.MinRunes(1)
	display_ref:   string & strings.MinRunes(1)
	depth:         int & >=0 & <=7
	ordinal_path:  =~"^[0-9]+(\\.[0-9]+)*$"
	source_path:   string
	source_hash:   =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	status:        #LawbookNodeStatus
	priority:      #LawbookPriority
	domain:        string & strings.MinRunes(1)
	title?:        string
	text?:         string
	parent_id?:    string
	layer_id:      =~"^[a-z0-9][a-z0-9._-]*$"
	layer_kind:    #LayerKind
}

// #MergeConflict records a canonical_ref claimed by multiple layers.
#MergeConflict: {
	canonical_ref: string & strings.MinRunes(1)
	layer_ids: [string, ...string] & [_, ...]
	resolution:   "priority" | "first"
	winner_layer: =~"^[a-z0-9][a-z0-9._-]*$"
}

// #RuntimeFeed is the top-level multi-layer feed document.
#RuntimeFeed: {
	format:       "nomos.rbok-runtime-feed.v1"
	generated_at: string & strings.MinRunes(1)
	content_hash: =~"^sha256:[A-Fa-f0-9]+$"
	layer_count:  int & >=0
	node_count:   int & >=0
	layers: [...#LayerProvenance]
	nodes: [...#RuntimeFeedNode]
	conflicts?: [...#MergeConflict]

	// Invariants.
	layer_count: len(layers)
	node_count:  len(nodes)
}

// #EngineImportBatch is the projection the RBOK Engine database
// importer expects. Nomos produces this from the RuntimeFeed.
#EngineImportBatch: {
	schema_version: string | *"0.1.0"
	feed_format:    "nomos.rbok-runtime-feed.v1"
	generated_at:   string & strings.MinRunes(1)
	content_hash:   =~"^sha256:[A-Fa-f0-9]+$"

	documents: [...#EngineImportDocument]
	nodes: [...#EngineImportNode]
	revisions: [...#EngineImportRevision]
	layer_metadata: [...#LayerProvenance]
}

// #EngineImportDocument is a top-level document record for the engine.
#EngineImportDocument: {
	document_id: =~"^[A-Z0-9][A-Z0-9._-]*$"
	domain:      string & strings.MinRunes(1)
	source_path: string
	source_hash: =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	node_count:  int & >=0
	layer_id:    =~"^[a-z0-9][a-z0-9._-]*$"
}

// #EngineImportNode is a single node for the engine relational model.
#EngineImportNode: {
	node_id:       =~"^[A-Z0-9][A-Z0-9._-]*$"
	document_id:   =~"^[A-Z0-9][A-Z0-9._-]*$"
	node_type:     #LawbookNodeType
	canonical_ref: string & strings.MinRunes(1)
	display_ref:   string & strings.MinRunes(1)
	depth:         int & >=0 & <=7
	ordinal_path:  =~"^[0-9]+(\\.[0-9]+)*$"
	source_path?:  string
	source_hash?:  =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	parent_id?:    string
	status:        #LawbookNodeStatus
	priority:      #LawbookPriority
	domain?:       string
	title?:        string
	text?:         string
	layer_id:      =~"^[a-z0-9][a-z0-9._-]*$"
	layer_kind:    #LayerKind
}

// #EngineImportRevision captures a snapshot revision for the engine.
#EngineImportRevision: {
	node_id:     =~"^[A-Z0-9][A-Z0-9._-]*$"
	source_hash: =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	status:      #LawbookNodeStatus
	timestamp:   string & strings.MinRunes(1)
	layer_id:    =~"^[a-z0-9][a-z0-9._-]*$"
}

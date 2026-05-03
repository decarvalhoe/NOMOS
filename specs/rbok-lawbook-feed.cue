package nomos

import "strings"

// #LawbookNodeType enumerates the structural levels of a lawbook.
#LawbookNodeType:
	"document" |
	"chapter" |
	"section" |
	"subsection" |
	"article" |
	"paragraph" |
	"alinea"

// #LawbookNodeStatus tracks the lifecycle of a node.
#LawbookNodeStatus:
	"active" |
	"amended" |
	"repealed" |
	"pending" |
	"draft"

// #LawbookPriority indicates processing priority for the node.
#LawbookPriority:
	"critical" |
	"high" |
	"medium" |
	"low"

// #LawbookNode is a single structural node in a lawbook feed.
#LawbookNode: {
	node_id:      =~"^[A-Z0-9][A-Z0-9._-]*$"
	document_id:  =~"^[A-Z0-9][A-Z0-9._-]*$"
	node_type:    #LawbookNodeType
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
	effective_date?: string
	metadata?: {...}
}

// #LawbookFeed is a batch of lawbook nodes forming a feed document.
#LawbookFeed: {
	schema_version: string | *"0.1.0"
	feed_id:        =~"^[a-z0-9][a-z0-9-]*$"
	document_id:    =~"^[A-Z0-9][A-Z0-9._-]*$"
	domain:         string & strings.MinRunes(1)
	generated_at:   string
	source_path:    string
	source_hash:    =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	node_count:     int & >=0
	nodes: [...#LawbookNode]

	// Invariant: node_count matches length.
	node_count: len(nodes)
}

// #LawbookDepthMap defines canonical depth for each node type.
#LawbookDepthMap: {
	document:   0
	chapter:    1
	section:    2
	subsection: 3
	article:    4
	paragraph:  5
	alinea:     6
}

// =============================================================================
// Multi-layer runtime feed extensions for realisons-business corpus.
// Layers: 01_rbok (lawbook), 02_parcours (process flows), 03_workbooks (references)
// =============================================================================

// #CorpusLayer identifies the source layer in realisons-business.
#CorpusLayer:
	"01_rbok" |
	"02_parcours" |
	"03_workbooks" |
	"04_doctrine" |
	"99_archive"

// #AuthorityLevel classifies the normative weight of a node.
#AuthorityLevel:
	"binding" |
	"regulatory" |
	"guidance" |
	"informational" |
	"internal" |
	"deprecated"

// #ParcoursNodeType enumerates process/parcours structural elements.
#ParcoursNodeType:
	"parcours" |
	"phase" |
	"step" |
	"decision" |
	"gate" |
	"output" |
	"reference"

// #WorkbookRefType classifies workbook reference entries.
#WorkbookRefType:
	"template" |
	"checklist" |
	"form" |
	"example" |
	"tool" |
	"standard"

// #RuntimeFeedNode extends the lawbook node with multi-layer metadata.
#RuntimeFeedNode: {
	// Core identity (same pattern as #LawbookNode).
	node_id:       =~"^[A-Z0-9][A-Z0-9._-]*$"
	document_id:   =~"^[A-Z0-9][A-Z0-9._-]*$"
	canonical_ref: string & strings.MinRunes(1)
	display_ref:   string & strings.MinRunes(1)
	source_path:   string
	source_hash:   =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	status:        #LawbookNodeStatus
	priority:      #LawbookPriority
	domain:        string & strings.MinRunes(1)

	// Multi-layer extensions.
	layer:           #CorpusLayer
	authority_level: #AuthorityLevel
	node_type:       #LawbookNodeType | #ParcoursNodeType | #WorkbookRefType
	depth:           int & >=0 & <=10

	// Optional structural fields.
	ordinal_path?: =~"^[0-9]+(\\.[0-9]+)*$"
	parent_id?:    string
	title?:        string
	text?:         string
	effective_date?: string

	// Parcours-specific fields (present when layer == "02_parcours").
	predecessor_ids?: [...string]
	successor_ids?:   [...string]
	gate_criteria?:   string

	// Workbook-specific fields (present when layer == "03_workbooks").
	ref_type?:     #WorkbookRefType
	target_url?:   string
	target_hash?:  =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"

	metadata?: {...}
}

// #RuntimeFeed is the multi-layer feed combining all corpus layers.
#RuntimeFeed: {
	schema_version: string | *"0.1.0"
	feed_format:    "nomos.rbok-runtime-feed.v1"
	feed_id:        =~"^[a-z0-9][a-z0-9-]*$"
	corpus_id:      string & strings.MinRunes(1)
	domain:         string & strings.MinRunes(1)
	generated_at:   string
	layers:         [...#CorpusLayer]
	node_count:     int & >=0
	nodes:          [...#RuntimeFeedNode]

	// Invariant.
	node_count: len(nodes)

	// Per-layer summary.
	layer_summary: [string]: {
		node_count:      int & >=0
		document_count:  int & >=0
		authority_breakdown: [string]: int
	}
}

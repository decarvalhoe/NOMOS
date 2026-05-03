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
	source_class?: string
	corpus_layer?: string
	authority?:    string
	allowed_uses?: [...string]
	locator?:      string
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

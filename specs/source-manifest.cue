package nomos

// #SourceManifest defines the authoritative source registry format.

#SourceManifest: {
	schema_version: string | *"0.1.0"
	sources: [#Source, ...#Source]
}

#Source: {
	id:              =~"^[A-Z0-9][A-Z0-9-]*$"
	path:            string
	type:            #SourceType
	domain:          string
	priority:        #SourcePriority
	status:          #SourceStatus
	hash:            string
	version?:        string
	owner:           string
	license:         string
	confidentiality: #Confidentiality
	allowed_uses: [#AllowedUse, ...#AllowedUse]
	redaction_policy?: "none" | "partial" | "full"
	notes?:            string

	// #610 — provenance of a source fetched from the web. Optional, so every
	// existing manifest stays valid byte-for-byte. When present the engine
	// validates it fail-closed (see cli/internal/corpus/web_source.go).
	web_source?: #WebSource
}

// #WebSource is a point-in-time capture. It is never the site's ongoing truth:
// the claim is "this is what canonical_url served at fetched_at, as seen by
// crawler_version". Field correspondence with the connector evidence
// (nomos-connector-evidence-v1) is documented on the Go type.
#WebSource: {
	canonical_url:            =~"^https?://[^/]+"
	fetched_url?:             =~"^https?://[^/]+"
	http_status:              int & >=100 & <=599
	content_type?:            string
	etag?:                    string
	last_modified?:           string
	content_hash:             #StableHash
	normalized_content_hash?: #StableHash
	fetched_at:               =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\\.[0-9]+)?(Z|[+-][0-9]{2}:[0-9]{2})$"
	crawler_version:          string & !=""
	scope_policy:             "seed" | "in_scope" | "out_scope"
	// `undecided` may be RECORDED; it can never be ADMITTED into a feed.
	robots_decision:  "allowed" | "disallowed" | "undecided"
	licence_decision: "allowed" | "disallowed" | "undecided"
	parent_url?:      =~"^https?://[^/]+"
	depth:            int & >=0
	claim_boundary?:  string
}

// A stable hash names its algorithm; a bare or placeholder digest cannot be
// re-verified.
#StableHash: =~"^(sha256|sha384|sha512):[0-9a-f]{64,128}$"

#SourceType:
	"markdown" |
	"pdf" |
	"html" |
	"php" |
	"source_code" |
	"csv" |
	"database_export" |
	"spreadsheet" |
	"image" |
	"audio" |
	"decision" |
	"api_export"

#SourcePriority: "primary" | "secondary" | "legacy" | "derived" | "reference"

#SourceStatus: "active" | "superseded" | "duplicate" | "out_of_scope" | "needs_review" | "blocked"

#Confidentiality: "public" | "internal" | "restricted" | "secret"

#AllowedUse:
	"structured_contract" |
	"vector_index" |
	"citation_internal" |
	"citation_external" |
	"golden_case" |
	"human_review_only"

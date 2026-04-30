package nomos

// #SourceManifest defines the authoritative source registry format.

#SourceManifest: {
	schema_version: string | *"0.1.0"
	sources: [...#Source]
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
	allowed_uses: [...(
		"structured_contract" |
		"vector_index" |
		"citation_internal" |
		"citation_external" |
		"golden_case" |
		"human_review_only"
	)]
	redaction_policy?: "none" | "partial" | "full"
	notes?:            string
}

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

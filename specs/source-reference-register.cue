package nomos

import "strings"

method_version: string & strings.MinRunes(1)
generated_at:    string & strings.MinRunes(1)
scope:           string & strings.MinRunes(1)
curator:         string & strings.MinRunes(1)

method: {
	objective: string & strings.MinRunes(1)
	selection_rules: [...string] & [_, ...]
	quality_rules:   [...string] & [_, ...]
	exclusion_rules?: [...string]
}

sources: [...#ExternalSource] & [_, ...]

#ExternalSource: {
	id:        =~"^[a-z0-9][a-z0-9-]*$"
	title:     string & strings.MinRunes(1)
	url:       =~"^https://[^\\s]+$"
	publisher: string & strings.MinRunes(1)
	status:    "official-standard" | "official-specification" | "official-documentation" | "peer-reviewed" | "technical-report" | "open-source-reference" | "licensed-standard-reference"
	domain:    "legal-structure" | "markdown-ast" | "document-ai" | "terminology" | "semantic-validation" | "provenance" | "rag" | "regulated-quality"
	authority: "primary" | "secondary"
	nomos_use: [...string] & [_, ...]
	license_status: "open" | "licensed-required" | "terms-review-required"
	accessed_at:    string & strings.MinRunes(1)
	notes?:         string
}

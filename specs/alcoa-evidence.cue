package nomos

import "strings"

// #ALCOAPlusAttribute enumerates the 8 ALCOA+ data integrity attributes.
// ALCOA: Attributable, Legible, Contemporaneous, Original, Accurate
// Plus: Complete, Consistent, Enduring, Available
#ALCOAPlusAttribute:
	"attributable" |
	"legible" |
	"contemporaneous" |
	"original" |
	"accurate" |
	"complete" |
	"consistent" |
	"enduring" |
	"available"

// #ALCOACompliance indicates how well an evidence record satisfies an attribute.
#ALCOACompliance:
	"satisfied" |
	"partial" |
	"not_satisfied" |
	"not_applicable"

// #ALCOAAttributeAssessment is the assessment of a single ALCOA+ attribute.
#ALCOAAttributeAssessment: {
	attribute:  #ALCOAPlusAttribute
	compliance: #ALCOACompliance
	evidence:   string & strings.MinRunes(1)
	assessor?:  string
	assessed_at?: string
	notes?:     string
}

// #ALCOAEvidenceRecord is an evidence record annotated with ALCOA+ integrity.
#ALCOAEvidenceRecord: {
	record_id:    =~"^[A-Z0-9][A-Z0-9._-]*$"
	evidence_ref: string & strings.MinRunes(1)
	owner:        string & strings.MinRunes(1)
	created_at:   string
	source_hash:  =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	domain:       string & strings.MinRunes(1)

	assessments: [...#ALCOAAttributeAssessment] & list.MinItems(1)

	overall_compliance: #ALCOACompliance
	review_status:      "pending" | "reviewed" | "approved" | "rejected"
	reviewer?:          string
	reviewed_at?:       string
	metadata?:          {...}
}

// #ALCOAReport is a batch of assessed evidence records.
#ALCOAReport: {
	schema_version: string | *"0.1.0"
	report_id:      =~"^[a-z0-9][a-z0-9-]*$"
	generated_at:   string
	domain:         string & strings.MinRunes(1)
	record_count:   int & >=0
	records: [...#ALCOAEvidenceRecord]

	// Invariant.
	record_count: len(records)

	summary: {
		satisfied_count:      int & >=0
		partial_count:        int & >=0
		not_satisfied_count:  int & >=0
		not_applicable_count: int & >=0
		compliance_ratio:     number & >=0 & <=1
	}
}

package nomos

import "strings"

// #EvidenceContract defines the shared evidence record format
// between Nomos (verification) and Praxis (execution).
// All evidence exchanged between the two systems must conform
// to this contract. Designed for ALCOA+ data integrity.

// #EvidenceRecord is a single piece of verifiable evidence.
// Each record is attributable, timestamped, hashed, and owned.
#EvidenceRecord: {
	// Identity
	record_id:   =~"^EV-[A-Z0-9][A-Z0-9._-]*$"
	category:    #EvidenceCategory
	title:       strings.MinRunes(1)
	description: strings.MinRunes(1)

	// Provenance (ALCOA+: attributable, contemporaneous)
	producer:    #EvidenceProducer
	owner:       strings.MinRunes(1)
	created_at:  =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}(T[0-9:.Z+-]+)?$"
	updated_at?: =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}(T[0-9:.Z+-]+)?$"

	// Integrity (ALCOA+: original, accurate, enduring)
	source_hash:  =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	content_type: strings.MinRunes(1)
	location: {
		type: "file" | "url" | "inline" | "external"
		path: strings.MinRunes(1)
	}

	// Status and lifecycle (ALCOA+: complete, consistent)
	status:          #EvidenceStatus
	claim_allowed:   strings.MinRunes(1)
	claim_boundary?: string

	// Review (ALCOA+: legible, available)
	review: {
		status:      "pending" | "reviewed" | "approved" | "rejected"
		reviewer?:   string
		reviewed_at?: string
		notes?:      string
	}

	// ALCOA+ assessment (optional, for records that need formal assessment)
	alcoa_assessment?: [...#ALCOAAttributeAssessment]

	// Traceability
	references?: [...#EvidenceReference]
	metadata?:   {[string]: _}
}

// #EvidenceReference links an evidence record to its source or consumer.
#EvidenceReference: {
	type: "source" | "control" | "requirement" | "test" | "decision" | "product"
	id:   strings.MinRunes(1)
	path?: string
}

// #EvidenceCategory classifies evidence by its function.
#EvidenceCategory:
	"external_reference_register" |
	"control_matrix" |
	"quality_system_document" |
	"lifecycle_document" |
	"validation_evidence" |
	"test_result" |
	"data_integrity_record" |
	"security_evidence" |
	"ai_governance_evidence" |
	"release_evidence" |
	"corpus_attestation" |
	"source_manifest" |
	"canonical_matrix" |
	"decision_record" |
	"training_record" |
	"audit_record" |
	"deviation_record" |
	"supplier_record"

// #EvidenceStatus tracks the lifecycle of evidence.
#EvidenceStatus:
	"planned" |
	"draft" |
	"present" |
	"generated" |
	"requires_evidence" |
	"effective" |
	"superseded" |
	"expired"

// #EvidenceProducer identifies who or what generated the evidence.
#EvidenceProducer:
	"nomos_cli" |
	"praxis_engine" |
	"human_author" |
	"ci_pipeline" |
	"external_tool" |
	"manual_review"

// #EvidenceLedger is the master index of all evidence for a product.
#EvidenceLedger: {
	schema_version: string | *"0.1.0"
	product_id:     =~"^[a-z0-9][a-z0-9-]*$"
	generated_at:   string
	status:         "draft" | "effective" | "superseded"
	claim_boundary: strings.MinRunes(1)

	categories: [...#EvidenceLedgerCategory]

	blocking_gaps?: [...#EvidenceGap]
}

// #EvidenceLedgerCategory is a row in the evidence ledger.
#EvidenceLedgerCategory: {
	id:                =~"^EV-[A-Z0-9][A-Z0-9._-]*$"
	category:          #EvidenceCategory
	expected_location: strings.MinRunes(1)
	current_status:    #EvidenceStatus
	claim_allowed:     strings.MinRunes(1)
	records?: [...#EvidenceRecord]
}

// #EvidenceGap documents a known gap blocking claims.
#EvidenceGap: {
	id:          =~"^GAP-[A-Z0-9][A-Z0-9._-]*$"
	description: strings.MinRunes(1)
	severity:    "minor" | "major" | "critical"
	status:      "open" | "mitigated" | "resolved"
	blocks_claims: [...strings.MinRunes(1)]
	owner?:      string
	target_date?: string
}

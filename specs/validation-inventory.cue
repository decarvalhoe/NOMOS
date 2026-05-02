package nomos

#ValidationInventory: {
	schema_version: string | *"0.1.0"
	document_id:    =~"^VI-[A-Z]+-[0-9]+$"
	status:         #ValidationDocStatus
	owner:          #NonEmptyString
	product:        =~"^[a-z][a-z0-9-]*$"
	generated_from?: string

	validations: [#ValidationEntry, ...#ValidationEntry]
}

#ValidationEntry: {
	id:                =~"^VAL-[0-9]+$"
	intended_use_ref?: string
	title:             #NonEmptyString
	risk_level:        #RiskLevel
	validation_type:   #ValidationType
	method:            #NonEmptyString
	evidence_artifact: #NonEmptyString
	acceptance_gate:   #AcceptanceGate
	status:            #ValidationStatus
	owner:             #NonEmptyString
	verification_command?: string
	last_verified?:    string
}

#ValidationDocStatus:
	"draft" |
	"effective" |
	"superseded" |
	"retired"

#ValidationType:
	"automated" |
	"manual" |
	"hybrid"

#ValidationStatus:
	"not_qualified" |
	"planned" |
	"implemented" |
	"verified" |
	"approved" |
	"waived" |
	"blocked"

#AcceptanceGate:
	"ci" |
	"corpus-scan" |
	"rbok-lawbook-e2e" |
	"rbok-lawbook-release-gate" |
	"regulated-documentation-gate" |
	"regulated-evidence-pack" |
	"manual-review"

#RiskLevel: "low" | "medium" | "high" | "critical"

#NonEmptyString: string & =~"\\S"

package nomos

import "strings"

// #IntendedUseStatement is the structured intended use declaration.
#IntendedUseStatement: {
	schema_version: string | *"0.1.0"
	document_id:    =~"^[A-Z][A-Z0-9-]*$"
	title:          string & strings.MinRunes(1)
	status:         "draft" | "effective" | "superseded"
	owner:          string & strings.MinRunes(1)
	effective_date: string | null
	last_reviewed:  string

	product: {
		id:             string & strings.MinRunes(1)
		name:           string & strings.MinRunes(1)
		version:        string & strings.MinRunes(1)
		classification: string & strings.MinRunes(1)
	}

	intended_use: {
		summary: string & strings.MinRunes(10)
		operating_environment: [...{
			id:           string & strings.MinRunes(1)
			description:  string & strings.MinRunes(1)
			risk_context: string & strings.MinRunes(1)
		}]
		user_roles: [...{
			id:             string & strings.MinRunes(1)
			role:           string & strings.MinRunes(1)
			access:         string & strings.MinRunes(1)
			responsibility: string & strings.MinRunes(1)
		}]
		exclusions: [...string] & [_, ...]
		risk_classification: {
			level:     "low" | "medium" | "high" | "critical"
			rationale: string & strings.MinRunes(10)
		}
	}

	constraints: [...{
		id:          string & strings.MinRunes(1)
		description: string & strings.MinRunes(1)
	}]
}

// #ValidationMasterPlan is the structured VMP.
#ValidationMasterPlan: {
	schema_version: string | *"0.1.0"
	document_id:    =~"^[A-Z][A-Z0-9-]*$"
	title:          string & strings.MinRunes(1)
	status:         "draft" | "effective" | "superseded"
	owner:          string & strings.MinRunes(1)
	approved_by:    string | null
	effective_date: string | null
	last_reviewed:  string

	claim_boundary: string & strings.MinRunes(10)

	scope: {
		in_scope:     [...string] & [_, ...]
		out_of_scope: [...string] & [_, ...]
	}

	validation_approach: {
		methodology:         string & strings.MinRunes(1)
		reference_standards: [...{
			id:            string & strings.MinRunes(1)
			title:         string & strings.MinRunes(1)
			applicability: string & strings.MinRunes(1)
		}]
		software_category: string & strings.MinRunes(1)
		rationale:         string & strings.MinRunes(10)
	}

	validation_activities: [...{
		id:            string & strings.MinRunes(1)
		activity:      string & strings.MinRunes(1)
		tool:          string & strings.MinRunes(1)
		frequency:     string & strings.MinRunes(1)
		evidence:      string & strings.MinRunes(1)
		risk_coverage: [...string] & [_, ...]
	}]

	acceptance_criteria: [...{
		id:        string & strings.MinRunes(1)
		criterion: string & strings.MinRunes(1)
		gate:      string & strings.MinRunes(1)
	}]

	traceability: {
		intended_use_ref:   string & strings.MinRunes(1)
		risk_assessment_ref: string
		test_protocol_refs: [...string]
		evidence_ledger_ref: string
	}
}

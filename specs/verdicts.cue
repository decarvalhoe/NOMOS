package nomos

// #VerdictName is the stable product scope verdict vocabulary.
#VerdictName:
	"in_scope" |
	"partial" |
	"blocked" |
	"out_of_scope"

// #ConfidenceLevel captures how strong the evidence is for a verdict.
#ConfidenceLevel: "low" | "medium" | "high"

// #EscalationLevel identifies who must resolve or approve a non-trivial verdict.
#EscalationLevel:
	"none" |
	"domain_owner" |
	"product_owner" |
	"compliance_owner"

#VerdictDefinition: {
	value:   #VerdictName
	label:   string
	summary: string
	entry_conditions: [string, ...string]
	gate_effect:              string
	default_confidence_floor: #ConfidenceLevel
	default_escalation:       #EscalationLevel
	required_evidence: [...string]
}

#ConfidenceDefinition: {
	value:   #ConfidenceLevel
	summary: string
	minimum_evidence: [string, ...string]
	escalation: #EscalationLevel
}

#VerdictTaxonomy: {
	schema_version: string | *"0.1.0"
	verdicts: {
		in_scope: #VerdictDefinition & {
			value:   "in_scope"
			label:   "In scope"
			summary: "The product surface is admitted into the Nomos scope with sufficient authority and no known blocking gap."
			entry_conditions: [
				"Authoritative sources are identified and active.",
				"Scope boundaries are explicit.",
				"No blocking source, ownership, or compliance gap is open.",
			]
			gate_effect:              "Eligible for canonical checks and strict release gates when confidence is high."
			default_confidence_floor: "high"
			default_escalation:       "none"
			required_evidence: [
				"source manifest entries",
				"scope boundaries",
				"owner approval when the project is regulated or critical",
			]
		}
		partial: #VerdictDefinition & {
			value:   "partial"
			label:   "Partial"
			summary: "The surface can enter Nomos with explicit gaps, assumptions, and a remediation trajectory."
			entry_conditions: [
				"At least one useful authoritative source or legacy behavior source exists.",
				"Known missing sources, gaps, or assumptions are documented.",
				"A remediation path exists for each blocking gap before strict release.",
			]
			gate_effect:              "Allowed for bootstrap and brownfield adoption; not sufficient for strict release on critical surfaces."
			default_confidence_floor: "medium"
			default_escalation:       "domain_owner"
			required_evidence: [
				"gap list",
				"remediation plan",
				"decision reference for accepted ambiguity",
			]
		}
		blocked: #VerdictDefinition & {
			value:   "blocked"
			label:   "Blocked"
			summary: "The surface must not be promoted because an authority, evidence, ownership, or compliance dependency is unresolved."
			entry_conditions: [
				"A required source is missing, inaccessible, contradictory, or legally unusable.",
				"A critical ambiguity has no accountable decision owner.",
				"A compliance or safety prerequisite is unresolved.",
			]
			gate_effect:              "Fails canonical checks until blockers are resolved or explicitly downgraded by governance."
			default_confidence_floor: "low"
			default_escalation:       "product_owner"
			required_evidence: [
				"blocker list",
				"owner",
				"remediation or stop decision",
			]
		}
		out_of_scope: #VerdictDefinition & {
			value:   "out_of_scope"
			label:   "Out of scope"
			summary: "The surface is intentionally excluded from the current Nomos scope."
			entry_conditions: [
				"The excluded boundary is explicit.",
				"The exclusion does not hide an active product behavior that needs canonical proof.",
				"Re-entry criteria are documented when the exclusion is temporary.",
			]
			gate_effect:              "Ignored by canonical gates except for boundary drift checks."
			default_confidence_floor: "medium"
			default_escalation:       "none"
			required_evidence: [
				"scope boundary",
				"exclusion rationale",
			]
		}
	}
	confidence_levels: {
		low: #ConfidenceDefinition & {
			value:   "low"
			summary: "Evidence is incomplete, stale, conflicting, or mostly inferred."
			minimum_evidence: [
				"one explicit uncertainty or gap",
			]
			escalation: "domain_owner"
		}
		medium: #ConfidenceDefinition & {
			value:   "medium"
			summary: "Evidence supports the verdict, but material assumptions or incomplete downstream coverage remain."
			minimum_evidence: [
				"source or legacy behavior reference",
				"documented assumptions",
			]
			escalation: "domain_owner"
		}
		high: #ConfidenceDefinition & {
			value:   "high"
			summary: "Evidence is current, sourced, owned, and consistent across the required Nomos chain."
			minimum_evidence: [
				"active source references",
				"owner or decision reference",
				"no open blocker for the admitted scope",
			]
			escalation: "none"
		}
	}
}

#VerdictCaseSet: {
	schema_version: string | *"0.1.0"
	cases: [#VerdictCase, ...#VerdictCase]
}

#VerdictCase: {
	id:         =~"^[A-Z0-9][A-Z0-9-]*$"
	title:      string
	verdict:    #VerdictName
	confidence: #ConfidenceLevel
	escalation: #EscalationLevel
	rationale:  string
	evidence: [string, ...string]
	expected_action: string
	remediations?: [...string]
	blockers?: [...string]
	out_of_scope_reason?: string

	if confidence == "low" {
		escalation: "domain_owner" | "product_owner" | "compliance_owner"
	}
	if verdict == "partial" {
		remediations: [string, ...string]
	}
	if verdict == "blocked" {
		blockers: [string, ...string]
		escalation: "domain_owner" | "product_owner" | "compliance_owner"
	}
	if verdict == "out_of_scope" {
		out_of_scope_reason: string
	}
}

package nomos

// #VerdictName is the stable product scope verdict vocabulary.
#VerdictName:
	"in_scope" |
	"partial" |
	"blocked" |
	"out_of_scope"

// #CorpusVerdictName is the corpus admission verdict vocabulary.
// Corpus admission determines whether a source corpus can feed the
// canonical chain, independent of the product surface verdict.
#CorpusVerdictName:
	"corpus_admissible" |
	"corpus_partial" |
	"corpus_blocked"

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

// #CorpusVerdictDefinition captures the admission rules for a source corpus.
#CorpusVerdictDefinition: {
	value:   #CorpusVerdictName
	label:   string
	summary: string
	entry_conditions: [string, ...string]
	gate_effect:              string
	default_confidence_floor: #ConfidenceLevel
	default_escalation:       #EscalationLevel
	required_evidence: [...string]
}

// #CorpusVerdictTaxonomy extends the base taxonomy with corpus admission verdicts.
#CorpusVerdictTaxonomy: {
	schema_version: string | *"0.1.0"
	corpus_verdicts: {
		corpus_admissible: #CorpusVerdictDefinition & {
			value:   "corpus_admissible"
			label:   "Corpus Admissible"
			summary: "The source corpus is complete, hashed, owned, and ready to feed the canonical chain without restrictions."
			entry_conditions: [
				"All corpus documents are indexed with provenance metadata.",
				"Every document has a valid hash and an identified owner.",
				"No contradictory or stale document is present without a supersession record.",
				"The corpus scope aligns with the declared Nomos project domain.",
			]
			gate_effect:              "Corpus can feed canonical contracts, read-models, and knowledge base without further human review."
			default_confidence_floor: "high"
			default_escalation:       "none"
			required_evidence: [
				"source manifest with all corpus entries hashed",
				"owner for each source",
				"no stale or contradictory document without decision record",
			]
		}
		corpus_partial: #CorpusVerdictDefinition & {
			value:   "corpus_partial"
			label:   "Corpus Partial"
			summary: "The corpus can begin feeding the canonical chain but has known gaps, stale documents, or missing provenance that require tracked remediation."
			entry_conditions: [
				"At least one authoritative source document is indexed and hashed.",
				"Known gaps or stale documents are listed explicitly.",
				"A remediation plan exists for each missing or stale source before strict gate.",
			]
			gate_effect:              "Corpus can feed non-critical surfaces and bootstrap contracts; strict gates on critical surfaces require remediation completion."
			default_confidence_floor: "medium"
			default_escalation:       "domain_owner"
			required_evidence: [
				"partial source manifest with gap annotations",
				"remediation plan for missing sources",
				"decision record for accepted staleness",
			]
		}
		corpus_blocked: #CorpusVerdictDefinition & {
			value:   "corpus_blocked"
			label:   "Corpus Blocked"
			summary: "The corpus cannot feed the canonical chain because critical sources are missing, inaccessible, legally restricted, or have no accountable owner."
			entry_conditions: [
				"A critical source document is missing or inaccessible.",
				"Legal, licensing, or confidentiality restrictions prevent indexing.",
				"No accountable owner can approve the corpus for canonical use.",
			]
			gate_effect:              "Corpus is excluded from canonical chain until blockers are resolved; product surfaces depending on this corpus inherit blocked status."
			default_confidence_floor: "low"
			default_escalation:       "product_owner"
			required_evidence: [
				"blocker description with source reference",
				"owner or legal contact",
				"remediation or stop decision",
			]
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

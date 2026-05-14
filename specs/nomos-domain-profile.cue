package nomos

// Domain profile contract for DOR-001.
//
// The profile enables domain-scoped planning only. Unsupported compliance,
// certification, validation, legal, and regulatory claims must be listed as
// blocked claims and must never appear as authorized claims.

#DomainProfile: {
	schema_version: string | *"0.1.0"
	domain_profile: #DomainID
	name:           #NonEmptyString
	summary:        #NonEmptyString
	intended_use:   #IntendedUse
	references: [#Reference, ...#Reference]
	applicability: #Applicability
	risk_class:    #RiskClass
	claim_ladder:  #ClaimLadder
	required_artifacts: [#RequiredArtifact, ...#RequiredArtifact]
	evidence_placeholders?: [#EvidencePlaceholder, ...#EvidencePlaceholder]
	validation_gates: [#ValidationGate, ...#ValidationGate]
	waiver?: #DomainWaiver
}

#DomainID: =~"^[a-z0-9][a-z0-9-]*$"

#IntendedUse: {
	statement: #NonEmptyString
	allowed_uses: [#NonEmptyString, ...#NonEmptyString]
	not_authorized: [#NonEmptyString, ...#NonEmptyString]
}

#Reference: {
	id:             =~"^[A-Z0-9][A-Z0-9-]*$"
	title:          #NonEmptyString
	authority_type: #AuthorityType
	access_policy:  #AccessPolicy
	status:         #ReferenceStatus
	purpose:        #NonEmptyString
}

#AuthorityType:
	"public_guidance" |
	"licensed_standard" |
	"private_source" |
	"customer_source" |
	"internal_policy"

#AccessPolicy:
	"repository_public" |
	"licensed_reference_only" |
	"private_reference_only" |
	"customer_confidential" |
	"internal_only"

#ReferenceStatus: "required" | "optional" | "deferred" | "blocked"

#Applicability: {
	status: #ApplicabilityStatus
	applies_when: [#NonEmptyString, ...#NonEmptyString]
	does_not_apply_when: [#NonEmptyString, ...#NonEmptyString]
}

#ApplicabilityStatus:
	"applicable" |
	"partially_applicable" |
	"not_applicable" |
	"blocked"

#RiskClass: {
	level:     #RiskLevel
	rationale: #NonEmptyString
}

#RiskLevel: "low" | "medium" | "high" | "critical"

#ClaimLadder: {
	current_level: #ClaimLevel
	authorized_claims: [#AuthorizedClaim, ...#AuthorizedClaim]
	blocked_claims: [#BlockedClaim, ...#BlockedClaim]
}

#ClaimLevel:
	"registered" |
	"mapped" |
	"evidence_ready" |
	"validated_by_customer" |
	"independent_review_ready"

#AuthorizedClaim: {
	id:        =~"^[A-Z0-9][A-Z0-9-]*$"
	level:     #ClaimLevel
	kind:      #AuthorizedClaimKind
	statement: #NonEmptyString
	evidence: [#NonEmptyString, ...#NonEmptyString]
}

#AuthorizedClaimKind:
	"planning" |
	"scope_boundary" |
	"evidence_available" |
	"workflow_support" |
	"readiness" |
	"integration_support"

#BlockedClaim: {
	id:           =~"^[A-Z0-9][A-Z0-9-]*$"
	kind:         #UnsupportedClaimKind
	statement:    #NonEmptyString
	reason:       #NonEmptyString
	remediation?: #NonEmptyString
}

#UnsupportedClaimKind:
	"certification" |
	"regulated_validation" |
	"gxp_compliance" |
	"part11_compliance" |
	"medical_device_compliance" |
	"financial_regulatory_compliance" |
	"legal_compliance" |
	"legal_sufficiency"

#RequiredArtifact: {
	id:                  #DomainID
	type:                #ArtifactType
	path:                #NonEmptyString
	required:            bool | *true
	minimum_claim_level: #ClaimLevel
}

#EvidencePlaceholder: {
	id:                  #DomainID
	kind:                #EvidencePlaceholderKind
	path:                #NonEmptyString
	status:              #EvidencePlaceholderStatus
	reason:              #NonEmptyString
	related_issue?:      =~"^#[0-9]+$"
	blocks_claim_levels: [#ClaimLevel, ...#ClaimLevel]
}

#EvidencePlaceholderKind:
	"clinical_evaluation" |
	"requirement_test_trace" |
	"licensed_reference_intake" |
	"source_processing_evidence" |
	"risk_assessment" |
	"lifecycle_evidence" |
	"other"

#EvidencePlaceholderStatus:
	"missing_evidence_blocked" |
	"deferred_until_licensed_intake" |
	"deferred_until_customer_execution" |
	"planned"

#ArtifactType:
	"source_manifest" |
	"canonical_matrix" |
	"evidence_pack" |
	"validation_plan" |
	"control_matrix" |
	"attestation" |
	"sbom" |
	"provenance" |
	"access_policy" |
	"change_control_ledger" |
	"release_bundle" |
	"supplier_pack" |
	"training_record" |
	"audit_log" |
	"workflow_config" |
	"decision_record" |
	"other"

#ValidationGate: {
	id:       #DomainID
	command:  #NonEmptyString
	required: bool | *true
	blocks_claim_levels: [#ClaimLevel, ...#ClaimLevel]
}

#DomainWaiver: {
	id:         =~"^[A-Z0-9][A-Z0-9-]*$"
	reason:     #NonEmptyString
	approver:   #NonEmptyString
	expires_on: =~"^[0-9]{4}-[0-9]{2}-[0-9]{2}$"
}

#NonEmptyString: string & =~".*\\S.*"

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
	validation_gates: [#ValidationGate, ...#ValidationGate]
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

#NonEmptyString: string & =~".*\\S.*"

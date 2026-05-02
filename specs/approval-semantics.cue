package nomos

import "strings"

// #ApprovalRole defines the roles in a controlled approval workflow.
#ApprovalRole:
	"author" |
	"reviewer" |
	"approver" |
	"quality_unit"

// #ApprovalDecision captures the outcome of a single approval action.
#ApprovalDecision:
	"approved" |
	"rejected" |
	"returned_for_revision" |
	"deferred"

// #ApprovalStatus is the aggregate lifecycle state of an evidence record.
#ApprovalStatus:
	"draft" |
	"in_review" |
	"approved" |
	"rejected" |
	"superseded" |
	"retired"

// #SignatureMethod indicates how the approval was authenticated.
#SignatureMethod:
	"git_commit_signature" |
	"github_review_approval" |
	"manual_attestation" |
	"system_generated" |
	"external_pki"

// #ApprovalSignature records one approval action with e-signature semantics.
// Compatible with 21 CFR Part 11 requirements for electronic signatures:
// - unique to the signer (identity)
// - meaning of signature stated (role + decision)
// - date/time recorded (signed_at)
// - linked to record (record_hash)
#ApprovalSignature: {
	signer_id:    string & strings.MinRunes(1)
	signer_name:  string & strings.MinRunes(1)
	role:         #ApprovalRole
	decision:     #ApprovalDecision
	meaning:      string & strings.MinRunes(1)
	signed_at:    string // RFC3339 datetime
	method:       #SignatureMethod
	record_hash:  =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	comment?:     string
	previous_hash?: =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
}

// #ApprovalRecord is the full approval envelope for an evidence record.
#ApprovalRecord: {
	schema_version: string | *"0.1.0"
	record_id:      =~"^[A-Z0-9][A-Z0-9._-]*$"
	record_type:    string & strings.MinRunes(1)
	record_path:    string & strings.MinRunes(1)
	record_hash:    =~"^(sha256|sha384|sha512):[A-Fa-f0-9]+$"
	status:         #ApprovalStatus
	created_at:     string
	updated_at:     string
	signatures:     [...#ApprovalSignature]
	chain_valid:    bool

	// At least author signature required for non-draft records.
	if status != "draft" {
		signatures: [_, ...]
	}
}

// #ApprovalPolicy defines required signatures for a record type.
#ApprovalPolicy: {
	record_type:        string & strings.MinRunes(1)
	required_roles:     [#ApprovalRole, ...#ApprovalRole]
	minimum_signatures: int & >=1
	require_chain:      bool | *true
	allow_self_approve: bool | *false
}

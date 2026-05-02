package compliance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var (
	ErrApprovalIncomplete  = errors.New("approval requirements not met")
	ErrChainBroken         = errors.New("approval hash chain is broken")
	ErrSelfApproval        = errors.New("self-approval not permitted")
	ErrDuplicateRole       = errors.New("role already signed by same signer")
	ErrInvalidTransition   = errors.New("invalid approval status transition")
)

// ApprovalRole defines roles in the controlled approval workflow.
type ApprovalRole string

const (
	RoleAuthor      ApprovalRole = "author"
	RoleReviewer    ApprovalRole = "reviewer"
	RoleApprover    ApprovalRole = "approver"
	RoleQualityUnit ApprovalRole = "quality_unit"
)

// ApprovalDecision captures the outcome of an approval action.
type ApprovalDecision string

const (
	DecisionApproved  ApprovalDecision = "approved"
	DecisionRejected  ApprovalDecision = "rejected"
	DecisionReturned  ApprovalDecision = "returned_for_revision"
	DecisionDeferred  ApprovalDecision = "deferred"
)

// ApprovalStatus is the aggregate state of an evidence record.
type ApprovalStatus string

const (
	ApprovalDraft      ApprovalStatus = "draft"
	ApprovalInReview   ApprovalStatus = "in_review"
	ApprovalApproved   ApprovalStatus = "approved"
	ApprovalRejected   ApprovalStatus = "rejected"
	ApprovalSuperseded ApprovalStatus = "superseded"
	ApprovalRetired    ApprovalStatus = "retired"
)

// SignatureMethod indicates how the approval was authenticated.
type SignatureMethod string

const (
	MethodGitCommit     SignatureMethod = "git_commit_signature"
	MethodGitHubReview  SignatureMethod = "github_review_approval"
	MethodManual        SignatureMethod = "manual_attestation"
	MethodSystem        SignatureMethod = "system_generated"
	MethodExternalPKI   SignatureMethod = "external_pki"
)

// ApprovalSignature records one approval action (Part 11 compliant).
type ApprovalSignature struct {
	SignerID     string           `json:"signer_id" yaml:"signer_id"`
	SignerName   string           `json:"signer_name" yaml:"signer_name"`
	Role        ApprovalRole     `json:"role" yaml:"role"`
	Decision    ApprovalDecision `json:"decision" yaml:"decision"`
	Meaning     string           `json:"meaning" yaml:"meaning"`
	SignedAt    time.Time        `json:"signed_at" yaml:"signed_at"`
	Method      SignatureMethod  `json:"method" yaml:"method"`
	RecordHash  string           `json:"record_hash" yaml:"record_hash"`
	Comment     string           `json:"comment,omitempty" yaml:"comment,omitempty"`
	PreviousHash string          `json:"previous_hash,omitempty" yaml:"previous_hash,omitempty"`
}

// ApprovalRecord is the full approval envelope for an evidence record.
type ApprovalRecord struct {
	SchemaVersion string              `json:"schema_version" yaml:"schema_version"`
	RecordID      string              `json:"record_id" yaml:"record_id"`
	RecordType    string              `json:"record_type" yaml:"record_type"`
	RecordPath    string              `json:"record_path" yaml:"record_path"`
	RecordHash    string              `json:"record_hash" yaml:"record_hash"`
	Status        ApprovalStatus      `json:"status" yaml:"status"`
	CreatedAt     time.Time           `json:"created_at" yaml:"created_at"`
	UpdatedAt     time.Time           `json:"updated_at" yaml:"updated_at"`
	Signatures    []ApprovalSignature `json:"signatures" yaml:"signatures"`
	ChainValid    bool                `json:"chain_valid" yaml:"chain_valid"`
}

// ApprovalPolicy defines required signatures for a record type.
type ApprovalPolicy struct {
	RecordType       string         `json:"record_type" yaml:"record_type"`
	RequiredRoles    []ApprovalRole `json:"required_roles" yaml:"required_roles"`
	MinSignatures   int            `json:"minimum_signatures" yaml:"minimum_signatures"`
	RequireChain    bool           `json:"require_chain" yaml:"require_chain"`
	AllowSelfApprove bool          `json:"allow_self_approve" yaml:"allow_self_approve"`
}

// NewApprovalRecord creates a new record in draft status.
func NewApprovalRecord(recordID, recordType, recordPath, recordHash string) ApprovalRecord {
	now := time.Now().UTC()
	return ApprovalRecord{
		SchemaVersion: "0.1.0",
		RecordID:      recordID,
		RecordType:    recordType,
		RecordPath:    recordPath,
		RecordHash:    recordHash,
		Status:        ApprovalDraft,
		CreatedAt:     now,
		UpdatedAt:     now,
		Signatures:    []ApprovalSignature{},
		ChainValid:    true,
	}
}

// Sign adds an approval signature to the record with hash-chain linkage.
func (r *ApprovalRecord) Sign(signerID, signerName string, role ApprovalRole, decision ApprovalDecision, meaning string, method SignatureMethod, comment string) error {
	// Build previous hash for chain.
	var previousHash string
	if len(r.Signatures) > 0 {
		previousHash = r.Signatures[len(r.Signatures)-1].RecordHash
	}

	// Compute current chain hash: sha256(record_hash + previous_hash + signer_id + role + decision + timestamp)
	now := time.Now().UTC()
	chainInput := fmt.Sprintf("%s|%s|%s|%s|%s|%s", r.RecordHash, previousHash, signerID, role, decision, now.Format(time.RFC3339Nano))
	hash := computeHash(chainInput)

	sig := ApprovalSignature{
		SignerID:      signerID,
		SignerName:    signerName,
		Role:         role,
		Decision:     decision,
		Meaning:      meaning,
		SignedAt:      now,
		Method:       method,
		RecordHash:   hash,
		Comment:      comment,
		PreviousHash: previousHash,
	}

	r.Signatures = append(r.Signatures, sig)
	r.UpdatedAt = now
	return nil
}

// ValidateChain verifies the hash-chain integrity of all signatures.
func (r *ApprovalRecord) ValidateChain() bool {
	for i, sig := range r.Signatures {
		if i == 0 {
			if sig.PreviousHash != "" {
				r.ChainValid = false
				return false
			}
		} else {
			if sig.PreviousHash != r.Signatures[i-1].RecordHash {
				r.ChainValid = false
				return false
			}
		}
	}
	r.ChainValid = true
	return true
}

// CheckPolicy verifies the record meets an approval policy's requirements.
func (r *ApprovalRecord) CheckPolicy(policy ApprovalPolicy) error {
	// Check minimum signatures.
	approvedSigs := 0
	for _, sig := range r.Signatures {
		if sig.Decision == DecisionApproved {
			approvedSigs++
		}
	}
	if approvedSigs < policy.MinSignatures {
		return fmt.Errorf("%w: need %d approved signatures, have %d", ErrApprovalIncomplete, policy.MinSignatures, approvedSigs)
	}

	// Check required roles.
	rolesSatisfied := map[ApprovalRole]bool{}
	for _, sig := range r.Signatures {
		if sig.Decision == DecisionApproved {
			rolesSatisfied[sig.Role] = true
		}
	}
	for _, required := range policy.RequiredRoles {
		if !rolesSatisfied[required] {
			return fmt.Errorf("%w: missing approved signature from role %s", ErrApprovalIncomplete, required)
		}
	}

	// Check self-approval.
	if !policy.AllowSelfApprove {
		authors := map[string]bool{}
		for _, sig := range r.Signatures {
			if sig.Role == RoleAuthor {
				authors[sig.SignerID] = true
			}
		}
		for _, sig := range r.Signatures {
			if sig.Role == RoleApprover && authors[sig.SignerID] {
				return fmt.Errorf("%w: %s is both author and approver", ErrSelfApproval, sig.SignerID)
			}
		}
	}

	// Check chain integrity.
	if policy.RequireChain && !r.ValidateChain() {
		return ErrChainBroken
	}

	return nil
}

// Transition moves the record to a new status with validation.
func (r *ApprovalRecord) Transition(newStatus ApprovalStatus) error {
	if !isValidApprovalTransition(r.Status, newStatus) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, r.Status, newStatus)
	}
	r.Status = newStatus
	r.UpdatedAt = time.Now().UTC()
	return nil
}

// DefaultRegulatedPolicy returns the standard policy for regulated evidence.
func DefaultRegulatedPolicy() ApprovalPolicy {
	return ApprovalPolicy{
		RecordType:       "regulated_evidence",
		RequiredRoles:    []ApprovalRole{RoleAuthor, RoleReviewer, RoleApprover},
		MinSignatures:   3,
		RequireChain:    true,
		AllowSelfApprove: false,
	}
}

// DefaultStandardPolicy returns a lighter policy for standard evidence.
func DefaultStandardPolicy() ApprovalPolicy {
	return ApprovalPolicy{
		RecordType:       "standard_evidence",
		RequiredRoles:    []ApprovalRole{RoleAuthor, RoleReviewer},
		MinSignatures:   2,
		RequireChain:    true,
		AllowSelfApprove: false,
	}
}

func isValidApprovalTransition(from, to ApprovalStatus) bool {
	switch from {
	case ApprovalDraft:
		return to == ApprovalInReview || to == ApprovalRetired
	case ApprovalInReview:
		return to == ApprovalApproved || to == ApprovalRejected || to == ApprovalDraft
	case ApprovalApproved:
		return to == ApprovalSuperseded || to == ApprovalRetired
	case ApprovalRejected:
		return to == ApprovalDraft || to == ApprovalRetired
	case ApprovalSuperseded:
		return to == ApprovalRetired
	default:
		return false
	}
}

func computeHash(input string) string {
	h := sha256.Sum256([]byte(input))
	return "sha256:" + hex.EncodeToString(h[:])
}

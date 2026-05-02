package compliance

import (
	"errors"
	"testing"
)

func TestNewApprovalRecord(t *testing.T) {
	r := NewApprovalRecord("REC-001", "regulated_evidence", "reports/evidence.json", "sha256:abcdef")
	if r.Status != ApprovalDraft {
		t.Fatalf("expected draft, got %s", r.Status)
	}
	if r.RecordID != "REC-001" {
		t.Fatalf("expected REC-001, got %s", r.RecordID)
	}
	if !r.ChainValid {
		t.Fatal("new record should have valid chain")
	}
	if len(r.Signatures) != 0 {
		t.Fatal("new record should have no signatures")
	}
}

func TestSignAddsSignature(t *testing.T) {
	r := NewApprovalRecord("REC-002", "test", "path.json", "sha256:111")
	err := r.Sign("alice", "Alice Smith", RoleAuthor, DecisionApproved, "I authored this record", MethodGitCommit, "")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if len(r.Signatures) != 1 {
		t.Fatalf("expected 1 signature, got %d", len(r.Signatures))
	}
	sig := r.Signatures[0]
	if sig.SignerID != "alice" || sig.Role != RoleAuthor {
		t.Fatalf("unexpected sig: %+v", sig)
	}
	if sig.RecordHash == "" {
		t.Fatal("expected non-empty record hash")
	}
	if sig.PreviousHash != "" {
		t.Fatal("first signature should have empty previous hash")
	}
}

func TestSignChainLinkage(t *testing.T) {
	r := NewApprovalRecord("REC-003", "test", "path.json", "sha256:222")
	r.Sign("alice", "Alice", RoleAuthor, DecisionApproved, "author", MethodGitCommit, "")
	r.Sign("bob", "Bob", RoleReviewer, DecisionApproved, "reviewed", MethodGitHubReview, "looks good")
	r.Sign("carol", "Carol", RoleApprover, DecisionApproved, "approved", MethodManual, "")

	if len(r.Signatures) != 3 {
		t.Fatalf("expected 3 signatures, got %d", len(r.Signatures))
	}
	// Second sig's previous should be first sig's hash.
	if r.Signatures[1].PreviousHash != r.Signatures[0].RecordHash {
		t.Fatal("chain broken between sig 0 and 1")
	}
	if r.Signatures[2].PreviousHash != r.Signatures[1].RecordHash {
		t.Fatal("chain broken between sig 1 and 2")
	}
}

func TestValidateChainPass(t *testing.T) {
	r := NewApprovalRecord("REC-004", "test", "path.json", "sha256:333")
	r.Sign("alice", "Alice", RoleAuthor, DecisionApproved, "author", MethodGitCommit, "")
	r.Sign("bob", "Bob", RoleReviewer, DecisionApproved, "review", MethodGitHubReview, "")

	if !r.ValidateChain() {
		t.Fatal("expected valid chain")
	}
}

func TestValidateChainBroken(t *testing.T) {
	r := NewApprovalRecord("REC-005", "test", "path.json", "sha256:444")
	r.Sign("alice", "Alice", RoleAuthor, DecisionApproved, "author", MethodGitCommit, "")
	r.Sign("bob", "Bob", RoleReviewer, DecisionApproved, "review", MethodGitHubReview, "")

	// Tamper with chain.
	r.Signatures[1].PreviousHash = "sha256:tampered"

	if r.ValidateChain() {
		t.Fatal("expected broken chain")
	}
	if r.ChainValid {
		t.Fatal("ChainValid should be false")
	}
}

func TestCheckPolicyRegulatedPass(t *testing.T) {
	r := NewApprovalRecord("REC-006", "regulated_evidence", "path.json", "sha256:555")
	r.Sign("alice", "Alice", RoleAuthor, DecisionApproved, "authored", MethodGitCommit, "")
	r.Sign("bob", "Bob", RoleReviewer, DecisionApproved, "reviewed", MethodGitHubReview, "")
	r.Sign("carol", "Carol", RoleApprover, DecisionApproved, "approved for release", MethodManual, "")

	err := r.CheckPolicy(DefaultRegulatedPolicy())
	if err != nil {
		t.Fatalf("expected pass, got: %v", err)
	}
}

func TestCheckPolicyMissingRole(t *testing.T) {
	r := NewApprovalRecord("REC-007", "regulated_evidence", "path.json", "sha256:666")
	r.Sign("alice", "Alice", RoleAuthor, DecisionApproved, "authored", MethodGitCommit, "")
	r.Sign("bob", "Bob", RoleReviewer, DecisionApproved, "reviewed", MethodGitHubReview, "")
	// No approver signature.

	err := r.CheckPolicy(DefaultRegulatedPolicy())
	if !errors.Is(err, ErrApprovalIncomplete) {
		t.Fatalf("expected ErrApprovalIncomplete, got: %v", err)
	}
}

func TestCheckPolicyMinSignatures(t *testing.T) {
	r := NewApprovalRecord("REC-008", "regulated_evidence", "path.json", "sha256:777")
	r.Sign("alice", "Alice", RoleAuthor, DecisionApproved, "authored", MethodGitCommit, "")
	// Only 1 approved sig, need 3.

	err := r.CheckPolicy(DefaultRegulatedPolicy())
	if !errors.Is(err, ErrApprovalIncomplete) {
		t.Fatalf("expected ErrApprovalIncomplete, got: %v", err)
	}
}

func TestCheckPolicySelfApprovalBlocked(t *testing.T) {
	r := NewApprovalRecord("REC-009", "regulated_evidence", "path.json", "sha256:888")
	r.Sign("alice", "Alice", RoleAuthor, DecisionApproved, "authored", MethodGitCommit, "")
	r.Sign("bob", "Bob", RoleReviewer, DecisionApproved, "reviewed", MethodGitHubReview, "")
	r.Sign("alice", "Alice", RoleApprover, DecisionApproved, "self-approved", MethodManual, "")

	err := r.CheckPolicy(DefaultRegulatedPolicy())
	if !errors.Is(err, ErrSelfApproval) {
		t.Fatalf("expected ErrSelfApproval, got: %v", err)
	}
}

func TestCheckPolicySelfApprovalAllowed(t *testing.T) {
	policy := ApprovalPolicy{
		RecordType:       "internal",
		RequiredRoles:    []ApprovalRole{RoleAuthor},
		MinSignatures:   1,
		RequireChain:    true,
		AllowSelfApprove: true,
	}
	r := NewApprovalRecord("REC-010", "internal", "path.json", "sha256:999")
	r.Sign("alice", "Alice", RoleAuthor, DecisionApproved, "authored", MethodGitCommit, "")

	err := r.CheckPolicy(policy)
	if err != nil {
		t.Fatalf("expected pass with self-approve allowed: %v", err)
	}
}

func TestCheckPolicyChainRequired(t *testing.T) {
	r := NewApprovalRecord("REC-011", "regulated_evidence", "path.json", "sha256:aaa")
	r.Sign("alice", "Alice", RoleAuthor, DecisionApproved, "authored", MethodGitCommit, "")
	r.Sign("bob", "Bob", RoleReviewer, DecisionApproved, "reviewed", MethodGitHubReview, "")
	r.Sign("carol", "Carol", RoleApprover, DecisionApproved, "approved", MethodManual, "")

	// Tamper chain.
	r.Signatures[2].PreviousHash = "sha256:wrong"

	err := r.CheckPolicy(DefaultRegulatedPolicy())
	if !errors.Is(err, ErrChainBroken) {
		t.Fatalf("expected ErrChainBroken, got: %v", err)
	}
}

func TestTransitionValidPaths(t *testing.T) {
	cases := []struct {
		from ApprovalStatus
		to   ApprovalStatus
	}{
		{ApprovalDraft, ApprovalInReview},
		{ApprovalInReview, ApprovalApproved},
		{ApprovalInReview, ApprovalRejected},
		{ApprovalInReview, ApprovalDraft},
		{ApprovalRejected, ApprovalDraft},
		{ApprovalApproved, ApprovalSuperseded},
		{ApprovalApproved, ApprovalRetired},
		{ApprovalSuperseded, ApprovalRetired},
	}
	for _, tc := range cases {
		r := NewApprovalRecord("T", "t", "p", "sha256:t")
		r.Status = tc.from
		if err := r.Transition(tc.to); err != nil {
			t.Fatalf("%s -> %s should be valid: %v", tc.from, tc.to, err)
		}
		if r.Status != tc.to {
			t.Fatalf("expected %s, got %s", tc.to, r.Status)
		}
	}
}

func TestTransitionInvalidPaths(t *testing.T) {
	cases := []struct {
		from ApprovalStatus
		to   ApprovalStatus
	}{
		{ApprovalDraft, ApprovalApproved},
		{ApprovalApproved, ApprovalDraft},
		{ApprovalApproved, ApprovalInReview},
		{ApprovalRetired, ApprovalDraft},
		{ApprovalRejected, ApprovalApproved},
	}
	for _, tc := range cases {
		r := NewApprovalRecord("T", "t", "p", "sha256:t")
		r.Status = tc.from
		err := r.Transition(tc.to)
		if !errors.Is(err, ErrInvalidTransition) {
			t.Fatalf("%s -> %s should be invalid, got: %v", tc.from, tc.to, err)
		}
	}
}

func TestDefaultPolicies(t *testing.T) {
	regulated := DefaultRegulatedPolicy()
	if len(regulated.RequiredRoles) != 3 {
		t.Fatalf("expected 3 required roles, got %d", len(regulated.RequiredRoles))
	}
	if regulated.MinSignatures != 3 {
		t.Fatalf("expected 3 min sigs, got %d", regulated.MinSignatures)
	}
	if regulated.AllowSelfApprove {
		t.Fatal("regulated should not allow self-approve")
	}

	standard := DefaultStandardPolicy()
	if len(standard.RequiredRoles) != 2 {
		t.Fatalf("expected 2 required roles, got %d", len(standard.RequiredRoles))
	}
}

func TestRejectedSignatureDoesNotCount(t *testing.T) {
	r := NewApprovalRecord("REC-012", "regulated_evidence", "path.json", "sha256:bbb")
	r.Sign("alice", "Alice", RoleAuthor, DecisionApproved, "authored", MethodGitCommit, "")
	r.Sign("bob", "Bob", RoleReviewer, DecisionRejected, "not ready", MethodGitHubReview, "needs work")
	r.Sign("carol", "Carol", RoleApprover, DecisionApproved, "approved", MethodManual, "")

	err := r.CheckPolicy(DefaultRegulatedPolicy())
	if !errors.Is(err, ErrApprovalIncomplete) {
		t.Fatalf("expected incomplete (reviewer rejected), got: %v", err)
	}
}

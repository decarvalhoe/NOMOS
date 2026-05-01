package corpus

import (
	"testing"
	"time"
)

var testNow = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

func TestNewUnitReviewStartsInDraft(t *testing.T) {
	r := NewUnitReview("UNIT-001", testNow)
	if r.State != StateDraft {
		t.Fatalf("expected draft, got %s", r.State)
	}
	if r.UnitID != "UNIT-001" {
		t.Fatalf("expected UNIT-001, got %s", r.UnitID)
	}
	if !r.CreatedAt.Equal(testNow) {
		t.Fatalf("expected created_at %v, got %v", testNow, r.CreatedAt)
	}
}

func TestSubmitForReview(t *testing.T) {
	r := NewUnitReview("UNIT-001", testNow)
	err := r.Apply(StatePendingReview, "alice", "ready for review", testNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.State != StatePendingReview {
		t.Fatalf("expected pending_review, got %s", r.State)
	}
	if r.Reviewer != "alice" {
		t.Fatalf("expected reviewer alice, got %s", r.Reviewer)
	}
	if len(r.Transitions) != 1 {
		t.Fatalf("expected 1 transition, got %d", len(r.Transitions))
	}
	if r.Transitions[0].Action != "submit_for_review" {
		t.Fatalf("expected action submit_for_review, got %s", r.Transitions[0].Action)
	}
}

func TestApproveFromPendingReview(t *testing.T) {
	r := NewUnitReview("UNIT-001", testNow)
	_ = r.Apply(StatePendingReview, "alice", "", testNow)
	err := r.Apply(StateApproved, "bob", "looks good", testNow.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.State != StateApproved {
		t.Fatalf("expected approved, got %s", r.State)
	}
	if len(r.Transitions) != 2 {
		t.Fatalf("expected 2 transitions, got %d", len(r.Transitions))
	}
}

func TestRejectFromPendingReview(t *testing.T) {
	r := NewUnitReview("UNIT-001", testNow)
	_ = r.Apply(StatePendingReview, "alice", "", testNow)
	err := r.Apply(StateRejected, "bob", "needs rework", testNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.State != StateRejected {
		t.Fatalf("expected rejected, got %s", r.State)
	}
}

func TestReviseFromRejected(t *testing.T) {
	r := NewUnitReview("UNIT-001", testNow)
	_ = r.Apply(StatePendingReview, "alice", "", testNow)
	_ = r.Apply(StateRejected, "bob", "bad", testNow)
	err := r.Apply(StateDraft, "alice", "will fix", testNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.State != StateDraft {
		t.Fatalf("expected draft, got %s", r.State)
	}
	if r.Transitions[2].Action != "revise" {
		t.Fatalf("expected action revise, got %s", r.Transitions[2].Action)
	}
}

func TestArchiveFromDraft(t *testing.T) {
	r := NewUnitReview("UNIT-001", testNow)
	err := r.Apply(StateArchived, "admin", "no longer needed", testNow)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.State != StateArchived {
		t.Fatalf("expected archived, got %s", r.State)
	}
}

func TestUnarchive(t *testing.T) {
	r := NewUnitReview("UNIT-001", testNow)
	_ = r.Apply(StateArchived, "admin", "", testNow)
	err := r.Apply(StateDraft, "admin", "reopening", testNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.State != StateDraft {
		t.Fatalf("expected draft, got %s", r.State)
	}
	if r.Transitions[1].Action != "unarchive" {
		t.Fatalf("expected action unarchive, got %s", r.Transitions[1].Action)
	}
}

func TestRequestReReview(t *testing.T) {
	r := NewUnitReview("UNIT-001", testNow)
	_ = r.Apply(StatePendingReview, "alice", "", testNow)
	_ = r.Apply(StateApproved, "bob", "", testNow)
	err := r.Apply(StatePendingReview, "carol", "source changed", testNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.State != StatePendingReview {
		t.Fatalf("expected pending_review, got %s", r.State)
	}
	if r.Transitions[2].Action != "request_re_review" {
		t.Fatalf("expected action request_re_review, got %s", r.Transitions[2].Action)
	}
}

func TestInvalidTransitionFromDraft(t *testing.T) {
	r := NewUnitReview("UNIT-001", testNow)
	err := r.Apply(StateApproved, "alice", "", testNow)
	if err == nil {
		t.Fatal("expected error for invalid transition draft->approved")
	}
	te, ok := err.(*TransitionError)
	if !ok {
		t.Fatalf("expected TransitionError, got %T", err)
	}
	if te.From != StateDraft || te.To != StateApproved {
		t.Fatalf("unexpected error states: %s -> %s", te.From, te.To)
	}
	// State should not change
	if r.State != StateDraft {
		t.Fatalf("state should remain draft, got %s", r.State)
	}
}

func TestInvalidTransitionFromApproved(t *testing.T) {
	r := NewUnitReview("UNIT-001", testNow)
	_ = r.Apply(StatePendingReview, "alice", "", testNow)
	_ = r.Apply(StateApproved, "bob", "", testNow)
	err := r.Apply(StateDraft, "alice", "", testNow)
	if err == nil {
		t.Fatal("expected error for invalid transition approved->draft")
	}
}

func TestInvalidTargetState(t *testing.T) {
	r := NewUnitReview("UNIT-001", testNow)
	err := r.Apply(ReviewState("invalid"), "alice", "", testNow)
	if err == nil {
		t.Fatal("expected error for invalid target state")
	}
}

func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to ReviewState
		want     bool
	}{
		{StateDraft, StatePendingReview, true},
		{StateDraft, StateApproved, false},
		{StateDraft, StateArchived, true},
		{StatePendingReview, StateApproved, true},
		{StatePendingReview, StateRejected, true},
		{StatePendingReview, StateDraft, true},
		{StateApproved, StateArchived, true},
		{StateApproved, StatePendingReview, true},
		{StateApproved, StateDraft, false},
		{StateRejected, StateDraft, true},
		{StateRejected, StateArchived, true},
		{StateRejected, StateApproved, false},
		{StateArchived, StateDraft, true},
		{StateArchived, StateApproved, false},
	}
	for _, tc := range cases {
		got := CanTransition(tc.from, tc.to)
		if got != tc.want {
			t.Errorf("CanTransition(%s, %s) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}

func TestAllowedTransitions(t *testing.T) {
	targets := AllowedTransitions(StateDraft)
	if len(targets) != 2 {
		t.Fatalf("expected 2 allowed from draft, got %d: %v", len(targets), targets)
	}
}

func TestAllStates(t *testing.T) {
	states := AllStates()
	if len(states) != 5 {
		t.Fatalf("expected 5 states, got %d", len(states))
	}
	for _, s := range states {
		if !s.IsValid() {
			t.Fatalf("expected %s to be valid", s)
		}
	}
}

func TestIsValid(t *testing.T) {
	if !StateDraft.IsValid() {
		t.Fatal("draft should be valid")
	}
	if ReviewState("bogus").IsValid() {
		t.Fatal("bogus should not be valid")
	}
}

func TestReturnToDraft(t *testing.T) {
	r := NewUnitReview("UNIT-001", testNow)
	_ = r.Apply(StatePendingReview, "alice", "", testNow)
	err := r.Apply(StateDraft, "alice", "not ready yet", testNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.State != StateDraft {
		t.Fatalf("expected draft, got %s", r.State)
	}
	if r.Transitions[1].Action != "return_to_draft" {
		t.Fatalf("expected return_to_draft, got %s", r.Transitions[1].Action)
	}
}

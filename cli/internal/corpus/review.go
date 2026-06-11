package corpus

import (
	"fmt"
	"time"
)

// ReviewState represents the lifecycle state of an extracted unit.
type ReviewState string

const (
	StateDraft         ReviewState = "draft"
	StatePendingReview ReviewState = "pending_review"
	StateApproved      ReviewState = "approved"
	StateRejected      ReviewState = "rejected"
	StateArchived      ReviewState = "archived"
)

// AllStates returns all valid review states.
func AllStates() []ReviewState {
	return []ReviewState{StateDraft, StatePendingReview, StateApproved, StateRejected, StateArchived}
}

// IsValid returns true if the state is a recognized review state.
func (s ReviewState) IsValid() bool {
	switch s {
	case StateDraft, StatePendingReview, StateApproved, StateRejected, StateArchived:
		return true
	default:
		return false
	}
}

// Transition represents a valid state transition with metadata.
type Transition struct {
	From      ReviewState `json:"from"`
	To        ReviewState `json:"to"`
	Action    string      `json:"action"`
	Actor     string      `json:"actor,omitempty"`
	Reason    string      `json:"reason,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// UnitReview tracks the review lifecycle of a single extracted unit.
type UnitReview struct {
	UnitID      string       `json:"unit_id"`
	State       ReviewState  `json:"state"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Reviewer    string       `json:"reviewer,omitempty"`
	Transitions []Transition `json:"transitions,omitempty"`
}

// NewUnitReview creates a new unit review in draft state.
func NewUnitReview(unitID string, now time.Time) UnitReview {
	return UnitReview{
		UnitID:    unitID,
		State:     StateDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// transitions defines the valid state machine transitions.
// Key: from state → allowed to states with their action names.
var transitions = map[ReviewState]map[ReviewState]string{
	StateDraft: {
		StatePendingReview: "submit_for_review",
		StateArchived:      "archive",
	},
	StatePendingReview: {
		StateApproved: "approve",
		StateRejected: "reject",
		StateDraft:    "return_to_draft",
	},
	StateApproved: {
		StateArchived:      "archive",
		StatePendingReview: "request_re_review",
	},
	StateRejected: {
		StateDraft:    "revise",
		StateArchived: "archive",
	},
	StateArchived: {
		StateDraft: "unarchive",
	},
}

// CanTransition returns true if the transition from current state to target is valid.
func CanTransition(from, to ReviewState) bool {
	allowed, ok := transitions[from]
	if !ok {
		return false
	}
	_, valid := allowed[to]
	return valid
}

// ActionForTransition returns the action name for a valid transition.
// Returns empty string if the transition is not valid.
func ActionForTransition(from, to ReviewState) string {
	allowed, ok := transitions[from]
	if !ok {
		return ""
	}
	return allowed[to]
}

// AllowedTransitions returns all valid target states from the given state.
func AllowedTransitions(from ReviewState) []ReviewState {
	allowed, ok := transitions[from]
	if !ok {
		return nil
	}
	var targets []ReviewState
	for to := range allowed {
		targets = append(targets, to)
	}
	return targets
}

// TransitionError is returned when an invalid state transition is attempted.
type TransitionError struct {
	From   ReviewState
	To     ReviewState
	Reason string
}

func (e *TransitionError) Error() string {
	return fmt.Sprintf("invalid transition from %s to %s: %s", e.From, e.To, e.Reason)
}

// Apply transitions the unit review to a new state.
// Returns an error if the transition is not valid.
func (r *UnitReview) Apply(to ReviewState, actor string, reason string, now time.Time) error {
	if !to.IsValid() {
		return &TransitionError{From: r.State, To: to, Reason: "unknown target state"}
	}
	if !CanTransition(r.State, to) {
		return &TransitionError{
			From:   r.State,
			To:     to,
			Reason: fmt.Sprintf("allowed transitions from %s: %v", r.State, AllowedTransitions(r.State)),
		}
	}

	action := ActionForTransition(r.State, to)
	transition := Transition{
		From:      r.State,
		To:        to,
		Action:    action,
		Actor:     actor,
		Reason:    reason,
		Timestamp: now,
	}

	r.State = to
	r.UpdatedAt = now
	r.Transitions = append(r.Transitions, transition)

	if to == StateApproved || to == StatePendingReview {
		r.Reviewer = actor
	}

	return nil
}

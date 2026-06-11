// Package pointintime is the point-in-time atom resolver in the Go engine
// (VRC-12 #558, A3). Given a work id and a project date, it selects the
// recorded expression in force on that date — and REFUSES (not_in_force) when
// no version applies, so Nomos never cites a stale or future expression.
//
// Ported faithfully from scripts/ckm_point_in_time_resolve.py +
// specs/point-in-time.cue. All dates are supplied by the caller (ISO
// YYYY-MM-DD); there is no wall-clock read, so resolution is deterministic.
package pointintime

import (
	"fmt"
	"sort"
	"time"
)

const dateLayout = "2006-01-02"

// Temporal is an atom's temporal metadata.
type Temporal struct {
	WorkID        string `json:"work_id"`
	ExpressionID  string `json:"expression_id"`
	EffectiveFrom string `json:"effective_from"`
	EffectiveTo   string `json:"effective_to"`
}

// SourceSpan carries the content hash of the selected expression.
type SourceSpan struct {
	Hash string `json:"hash"`
}

// Atom is one temporal atom version.
type Atom struct {
	AtomID     string     `json:"atom_id"`
	SourceSpan SourceSpan `json:"source_span"`
	Metadata   struct {
		Temporal Temporal `json:"temporal"`
	} `json:"metadata"`
}

// Result is the resolver verdict.
type Result struct {
	Status         string `json:"status"` // "resolved" | "not_in_force"
	WorkID         string `json:"work_id"`
	AsOf           string `json:"as_of"`
	SelectedAtomID string `json:"selected_atom_id,omitempty"`
	ExpressionID   string `json:"expression_id,omitempty"`
	EffectiveFrom  string `json:"effective_from,omitempty"`
	EffectiveTo    string `json:"effective_to,omitempty"`
	SourceHash     string `json:"source_hash,omitempty"`
	ClaimBoundary  string `json:"claim_boundary"`
}

func parseDay(s string) (time.Time, error) { return time.Parse(dateLayout, s) }

// inForce reports whether as_of falls within [effective_from, effective_to].
// An empty effective_to is open-ended (in force indefinitely).
func inForce(t Temporal, asOf time.Time) (time.Time, bool, error) {
	from, err := parseDay(t.EffectiveFrom)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("effective_from %q: %w", t.EffectiveFrom, err)
	}
	if asOf.Before(from) {
		return from, false, nil
	}
	if t.EffectiveTo != "" {
		to, err := parseDay(t.EffectiveTo)
		if err != nil {
			return from, false, fmt.Errorf("effective_to %q: %w", t.EffectiveTo, err)
		}
		if asOf.After(to) {
			return from, false, nil
		}
	}
	return from, true, nil
}

// ResolvePointInTime selects the expression of workID in force on asOf (the
// latest effective_from among the in-force candidates), or returns
// not_in_force. asOf is ISO YYYY-MM-DD.
func ResolvePointInTime(atoms []Atom, workID, asOf string) (Result, error) {
	asOfDay, err := parseDay(asOf)
	if err != nil {
		return Result{}, fmt.Errorf("as_of %q: %w", asOf, err)
	}

	type candidate struct {
		from time.Time
		atom Atom
	}
	candidates := []candidate{}
	for _, atom := range atoms {
		t := atom.Metadata.Temporal
		if t.WorkID != workID || t.EffectiveFrom == "" {
			continue
		}
		from, ok, err := inForce(t, asOfDay)
		if err != nil {
			return Result{}, err
		}
		if ok {
			candidates = append(candidates, candidate{from: from, atom: atom})
		}
	}

	if len(candidates) == 0 {
		return Result{
			Status:        "not_in_force",
			WorkID:        workID,
			AsOf:          asOf,
			ClaimBoundary: "No atom version is in force for the requested date; Nomos refuses to cite a stale or future expression.",
		}, nil
	}

	// Latest effective_from wins; ties break on atom id for determinism.
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].from.Equal(candidates[j].from) {
			return candidates[i].atom.AtomID > candidates[j].atom.AtomID
		}
		return candidates[i].from.After(candidates[j].from)
	})
	sel := candidates[0].atom
	t := sel.Metadata.Temporal
	return Result{
		Status:         "resolved",
		WorkID:         workID,
		AsOf:           asOf,
		SelectedAtomID: sel.AtomID,
		ExpressionID:   t.ExpressionID,
		EffectiveFrom:  t.EffectiveFrom,
		EffectiveTo:    t.EffectiveTo,
		SourceHash:     sel.SourceSpan.Hash,
		ClaimBoundary:  "Point-in-time resolver selects the recorded expression in force; it does not prove legal sufficiency.",
	}, nil
}

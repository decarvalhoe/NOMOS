package atomization

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	atomIDPattern   = regexp.MustCompile(`^ATOM-[A-Z0-9][A-Z0-9._-]*$`)
	hashPattern     = regexp.MustCompile(`^(sha256|sha384|sha512):[A-Fa-f0-9]+$`)
)

// AtomForVerification is the minimal atom representation needed for gate checks.
type AtomForVerification struct {
	AtomID      string `json:"atom_id"`
	Kind        string `json:"kind"`
	Hash        string `json:"hash"`
	ReviewState string `json:"review_state"`
	SourceSpan  *VerificationSourceSpan `json:"source_span,omitempty"`
}

// VerificationSourceSpan is the source span for verification purposes.
type VerificationSourceSpan struct {
	SourceID string `json:"source_id"`
	Path     string `json:"path"`
	Hash     string `json:"hash"`
}

// GateCheck is a single verification gate result.
type GateCheck struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"` // "passed", "failed", "warning"
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// AtomizationCertificate attests the completeness of an atom set.
type AtomizationCertificate struct {
	SchemaVersion string      `json:"schema_version"`
	CertID        string      `json:"cert_id"`
	IssuedAt      string      `json:"issued_at"`
	Issuer        string      `json:"issuer"`
	Domain        string      `json:"domain"`
	SetHash       string      `json:"set_hash"`
	AtomCount     int         `json:"atom_count"`
	GateResults   []GateCheck `json:"gate_results"`
	Passed        bool        `json:"passed"`
	Summary       CertSummary `json:"summary"`
}

// CertSummary provides aggregate gate results.
type CertSummary struct {
	TotalGates  int `json:"total_gates"`
	Passed      int `json:"passed"`
	Failed      int `json:"failed"`
	Warnings    int `json:"warnings"`
}

// CertificateOptions configures certificate generation.
type CertificateOptions struct {
	CertID string
	Issuer string
	Domain string
	Now    time.Time
}

// VerifyAndCertify runs all verification gates on a set of atoms and
// produces an AtomizationCertificate.
func VerifyAndCertify(atoms []AtomForVerification, opts CertificateOptions) AtomizationCertificate {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	gates := runGates(atoms)
	setHash := computeSetHash(atoms)

	passed := 0
	failed := 0
	warnings := 0
	allPassed := true

	for _, g := range gates {
		switch g.Status {
		case "passed":
			passed++
		case "failed":
			failed++
			allPassed = false
		case "warning":
			warnings++
		}
	}

	return AtomizationCertificate{
		SchemaVersion: "0.1.0",
		CertID:        opts.CertID,
		IssuedAt:      now.Format(time.RFC3339),
		Issuer:        opts.Issuer,
		Domain:        opts.Domain,
		SetHash:       setHash,
		AtomCount:     len(atoms),
		GateResults:   gates,
		Passed:        allPassed,
		Summary: CertSummary{
			TotalGates: len(gates),
			Passed:     passed,
			Failed:     failed,
			Warnings:   warnings,
		},
	}
}

// WriteCertificate writes the certificate as indented JSON.
func WriteCertificate(w io.Writer, cert AtomizationCertificate) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(cert)
}

func runGates(atoms []AtomForVerification) []GateCheck {
	return []GateCheck{
		gateStableIDs(atoms),
		gateHashes(atoms),
		gateSourceSpans(atoms),
		gateReviewStates(atoms),
		gateNoDuplicateIDs(atoms),
		gateMinimumAtoms(atoms),
	}
}

func gateStableIDs(atoms []AtomForVerification) GateCheck {
	var invalid []string
	for _, a := range atoms {
		if !atomIDPattern.MatchString(a.AtomID) {
			invalid = append(invalid, a.AtomID)
		}
	}
	if len(invalid) > 0 {
		return GateCheck{
			ID:       "atom.stable_ids",
			Name:     "Stable atom IDs",
			Status:   "failed",
			Message:  fmt.Sprintf("%d atom(s) have invalid IDs: %s", len(invalid), truncateList(invalid, 5)),
			Severity: "high",
		}
	}
	return GateCheck{
		ID:       "atom.stable_ids",
		Name:     "Stable atom IDs",
		Status:   "passed",
		Message:  fmt.Sprintf("All %d atom IDs match ATOM-* pattern.", len(atoms)),
		Severity: "info",
	}
}

func gateHashes(atoms []AtomForVerification) GateCheck {
	var missing []string
	var invalid []string
	for _, a := range atoms {
		if a.Hash == "" {
			missing = append(missing, a.AtomID)
		} else if !hashPattern.MatchString(a.Hash) {
			invalid = append(invalid, a.AtomID)
		}
	}
	if len(missing) > 0 || len(invalid) > 0 {
		msg := ""
		if len(missing) > 0 {
			msg += fmt.Sprintf("%d atom(s) missing hash. ", len(missing))
		}
		if len(invalid) > 0 {
			msg += fmt.Sprintf("%d atom(s) have invalid hash format.", len(invalid))
		}
		return GateCheck{
			ID:       "atom.hashes",
			Name:     "Content hashes",
			Status:   "failed",
			Message:  strings.TrimSpace(msg),
			Severity: "high",
		}
	}
	return GateCheck{
		ID:       "atom.hashes",
		Name:     "Content hashes",
		Status:   "passed",
		Message:  fmt.Sprintf("All %d atoms have valid content hashes.", len(atoms)),
		Severity: "info",
	}
}

func gateSourceSpans(atoms []AtomForVerification) GateCheck {
	var missing []string
	var noHash []string
	for _, a := range atoms {
		if a.SourceSpan == nil {
			missing = append(missing, a.AtomID)
		} else {
			if a.SourceSpan.SourceID == "" || a.SourceSpan.Path == "" {
				missing = append(missing, a.AtomID)
			}
			if a.SourceSpan.Hash == "" || !hashPattern.MatchString(a.SourceSpan.Hash) {
				noHash = append(noHash, a.AtomID)
			}
		}
	}
	if len(missing) > 0 {
		return GateCheck{
			ID:       "atom.source_spans",
			Name:     "Source span provenance",
			Status:   "failed",
			Message:  fmt.Sprintf("%d atom(s) missing source span: %s", len(missing), truncateList(missing, 5)),
			Severity: "high",
		}
	}
	if len(noHash) > 0 {
		return GateCheck{
			ID:       "atom.source_spans",
			Name:     "Source span provenance",
			Status:   "warning",
			Message:  fmt.Sprintf("%d atom(s) have source span without valid hash.", len(noHash)),
			Severity: "medium",
		}
	}
	return GateCheck{
		ID:       "atom.source_spans",
		Name:     "Source span provenance",
		Status:   "passed",
		Message:  fmt.Sprintf("All %d atoms have complete source spans with hashes.", len(atoms)),
		Severity: "info",
	}
}

func gateReviewStates(atoms []AtomForVerification) GateCheck {
	validStates := map[string]bool{
		"draft": true, "pending_review": true, "approved": true,
		"rejected": true, "archived": true,
	}
	var invalid []string
	draftCount := 0
	for _, a := range atoms {
		if !validStates[a.ReviewState] {
			invalid = append(invalid, a.AtomID)
		}
		if a.ReviewState == "draft" {
			draftCount++
		}
	}
	if len(invalid) > 0 {
		return GateCheck{
			ID:       "atom.review_states",
			Name:     "Review state validity",
			Status:   "failed",
			Message:  fmt.Sprintf("%d atom(s) have invalid review state.", len(invalid)),
			Severity: "high",
		}
	}
	if draftCount > 0 && draftCount == len(atoms) {
		return GateCheck{
			ID:       "atom.review_states",
			Name:     "Review state validity",
			Status:   "warning",
			Message:  fmt.Sprintf("All %d atoms are in draft state.", draftCount),
			Severity: "medium",
		}
	}
	return GateCheck{
		ID:       "atom.review_states",
		Name:     "Review state validity",
		Status:   "passed",
		Message:  fmt.Sprintf("All %d atoms have valid review states.", len(atoms)),
		Severity: "info",
	}
}

func gateNoDuplicateIDs(atoms []AtomForVerification) GateCheck {
	seen := map[string]int{}
	for _, a := range atoms {
		seen[a.AtomID]++
	}
	var dups []string
	for id, count := range seen {
		if count > 1 {
			dups = append(dups, fmt.Sprintf("%s(%d)", id, count))
		}
	}
	if len(dups) > 0 {
		sort.Strings(dups)
		return GateCheck{
			ID:       "atom.no_duplicates",
			Name:     "No duplicate IDs",
			Status:   "failed",
			Message:  fmt.Sprintf("%d duplicate ID(s): %s", len(dups), truncateList(dups, 5)),
			Severity: "high",
		}
	}
	return GateCheck{
		ID:       "atom.no_duplicates",
		Name:     "No duplicate IDs",
		Status:   "passed",
		Message:  fmt.Sprintf("All %d atom IDs are unique.", len(atoms)),
		Severity: "info",
	}
}

func gateMinimumAtoms(atoms []AtomForVerification) GateCheck {
	if len(atoms) == 0 {
		return GateCheck{
			ID:       "atom.minimum_count",
			Name:     "Minimum atom count",
			Status:   "failed",
			Message:  "No atoms provided.",
			Severity: "high",
		}
	}
	return GateCheck{
		ID:       "atom.minimum_count",
		Name:     "Minimum atom count",
		Status:   "passed",
		Message:  fmt.Sprintf("%d atom(s) present.", len(atoms)),
		Severity: "info",
	}
}

func computeSetHash(atoms []AtomForVerification) string {
	ids := make([]string, 0, len(atoms))
	for _, a := range atoms {
		ids = append(ids, a.AtomID+":"+a.Hash)
	}
	sort.Strings(ids)

	h := sha256.New()
	for _, id := range ids {
		h.Write([]byte(id))
		h.Write([]byte{'\n'})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func truncateList(items []string, max int) string {
	if len(items) <= max {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:max], ", ") + fmt.Sprintf(" (+%d more)", len(items)-max)
}

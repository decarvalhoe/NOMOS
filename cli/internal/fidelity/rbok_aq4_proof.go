package fidelity

import (
	"fmt"
	"strings"
)

// AQ4Criterion identifies one proof criterion in the AQ-4 gate.
type AQ4Criterion string

const (
	AQ4SemanticRoles AQ4Criterion = "semantic_roles_verified"
	AQ4CrossRefs     AQ4Criterion = "cross_refs_resolved"
	AQ4Lexicon       AQ4Criterion = "lexicon_governed"
)

// AQ4CheckResult records one criterion's outcome.
type AQ4CheckResult struct {
	Criterion AQ4Criterion `json:"criterion"`
	Pass      bool         `json:"pass"`
	Score     float64      `json:"score"` // 0.0-1.0
	Message   string       `json:"message"`
	Details   []string     `json:"details,omitempty"`
}

// AQ4ProofReport is the full AQ-4 semantic proof output.
type AQ4ProofReport struct {
	Pass       bool             `json:"pass"`
	Score      float64          `json:"score"`
	Criteria   []AQ4CheckResult `json:"criteria"`
	TotalAtoms int              `json:"total_atoms"`
	Summary    string           `json:"summary"`
}

// AQ4Input provides the data needed for AQ-4 proof evaluation.
type AQ4Input struct {
	Atoms       []AQ4Atom       `json:"atoms"`
	CrossRefs   []AQ4CrossRef   `json:"cross_refs"`
	Lexicon     []LexiconEntry  `json:"lexicon"`
}

// AQ4Atom is a minimal atom representation for proof purposes.
type AQ4Atom struct {
	ID           string `json:"id"`
	SemanticRole string `json:"semantic_role"` // from structured_paths mapping
	Text         string `json:"text"`
	HasRef       bool   `json:"has_ref"`
}

// AQ4CrossRef records a reference that needs resolution.
type AQ4CrossRef struct {
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Resolved bool   `json:"resolved"`
}

// LexiconEntry defines a governed term.
type LexiconEntry struct {
	Term       string   `json:"term"`
	Definition string   `json:"definition"`
	Aliases    []string `json:"aliases,omitempty"`
	Governed   bool     `json:"governed"`
}

// RunAQ4Proof executes the AQ-4 semantic proof and returns a report.
func RunAQ4Proof(input AQ4Input) AQ4ProofReport {
	criteria := []AQ4CheckResult{
		checkSemanticRoles(input.Atoms),
		checkCrossRefsResolved(input.CrossRefs),
		checkLexiconGoverned(input.Atoms, input.Lexicon),
	}

	totalScore := 0.0
	allPass := true
	for _, c := range criteria {
		totalScore += c.Score
		if !c.Pass {
			allPass = false
		}
	}
	avgScore := totalScore / float64(len(criteria))

	summary := "AQ-4 semantic proof passed."
	if !allPass {
		failed := 0
		for _, c := range criteria {
			if !c.Pass {
				failed++
			}
		}
		summary = fmt.Sprintf("AQ-4 semantic proof failed: %d/%d criteria not met.", failed, len(criteria))
	}

	return AQ4ProofReport{
		Pass:       allPass,
		Score:      avgScore,
		Criteria:   criteria,
		TotalAtoms: len(input.Atoms),
		Summary:    summary,
	}
}

func checkSemanticRoles(atoms []AQ4Atom) AQ4CheckResult {
	if len(atoms) == 0 {
		return AQ4CheckResult{
			Criterion: AQ4SemanticRoles,
			Pass:      false,
			Score:     0,
			Message:   "no atoms to verify",
		}
	}

	assigned := 0
	var unassigned []string
	for _, atom := range atoms {
		role := strings.TrimSpace(atom.SemanticRole)
		if role != "" && role != "unknown" {
			assigned++
		} else {
			if len(unassigned) < 5 {
				unassigned = append(unassigned, atom.ID)
			}
		}
	}

	ratio := float64(assigned) / float64(len(atoms))
	pass := ratio >= 0.8

	msg := fmt.Sprintf("%d/%d atoms have semantic roles (%.0f%%)", assigned, len(atoms), ratio*100)
	return AQ4CheckResult{
		Criterion: AQ4SemanticRoles,
		Pass:      pass,
		Score:     ratio,
		Message:   msg,
		Details:   unassigned,
	}
}

func checkCrossRefsResolved(refs []AQ4CrossRef) AQ4CheckResult {
	if len(refs) == 0 {
		return AQ4CheckResult{
			Criterion: AQ4CrossRefs,
			Pass:      true,
			Score:     1.0,
			Message:   "no cross-references to resolve",
		}
	}

	resolved := 0
	var broken []string
	for _, ref := range refs {
		if ref.Resolved {
			resolved++
		} else {
			if len(broken) < 5 {
				broken = append(broken, fmt.Sprintf("%s→%s", ref.SourceID, ref.TargetID))
			}
		}
	}

	ratio := float64(resolved) / float64(len(refs))
	pass := ratio >= 0.9

	msg := fmt.Sprintf("%d/%d cross-references resolved (%.0f%%)", resolved, len(refs), ratio*100)
	return AQ4CheckResult{
		Criterion: AQ4CrossRefs,
		Pass:      pass,
		Score:     ratio,
		Message:   msg,
		Details:   broken,
	}
}

func checkLexiconGoverned(atoms []AQ4Atom, lexicon []LexiconEntry) AQ4CheckResult {
	if len(lexicon) == 0 {
		return AQ4CheckResult{
			Criterion: AQ4Lexicon,
			Pass:      false,
			Score:     0,
			Message:   "no lexicon defined",
		}
	}

	governed := 0
	var ungoverned []string
	for _, entry := range lexicon {
		if entry.Governed {
			governed++
		} else {
			if len(ungoverned) < 5 {
				ungoverned = append(ungoverned, entry.Term)
			}
		}
	}

	// Also check that atoms use governed terms.
	termUsage := 0
	governedTerms := buildTermSet(lexicon)
	for _, atom := range atoms {
		text := strings.ToLower(atom.Text)
		for term := range governedTerms {
			if strings.Contains(text, strings.ToLower(term)) {
				termUsage++
				break
			}
		}
	}

	lexiconRatio := float64(governed) / float64(len(lexicon))
	usageRatio := 0.0
	if len(atoms) > 0 {
		usageRatio = float64(termUsage) / float64(len(atoms))
	}

	// Combined score: 60% lexicon governance, 40% usage in atoms.
	score := lexiconRatio*0.6 + usageRatio*0.4
	pass := lexiconRatio >= 0.8

	msg := fmt.Sprintf("lexicon: %d/%d governed (%.0f%%), usage: %d/%d atoms reference governed terms",
		governed, len(lexicon), lexiconRatio*100, termUsage, len(atoms))

	return AQ4CheckResult{
		Criterion: AQ4Lexicon,
		Pass:      pass,
		Score:     score,
		Message:   msg,
		Details:   ungoverned,
	}
}

func buildTermSet(lexicon []LexiconEntry) map[string]bool {
	terms := map[string]bool{}
	for _, entry := range lexicon {
		if entry.Governed {
			terms[entry.Term] = true
			for _, alias := range entry.Aliases {
				terms[alias] = true
			}
		}
	}
	return terms
}

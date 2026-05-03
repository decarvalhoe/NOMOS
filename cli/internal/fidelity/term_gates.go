package fidelity

import (
	"fmt"
	"sort"
	"strings"
)

// Gate codes for term checks.
const (
	CodeDefinitionConflict = "DEFINITION_CONFLICT"
	CodeUndefinedTerm      = "UNDEFINED_TERM"
	CodeDuplicateDefinition = "DUPLICATE_DEFINITION"
)

// TermDefinition is a single term definition from a source.
type TermDefinition struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
	Source     string `json:"source"`
	Line       int    `json:"line,omitempty"`
}

// TermUsage records where a term is used.
type TermUsage struct {
	Term     string `json:"term"`
	Location string `json:"location"`
	Line     int    `json:"line,omitempty"`
}

// TermFinding describes a term gate violation.
type TermFinding struct {
	Term       string   `json:"term"`
	Code       string   `json:"code"`
	Severity   string   `json:"severity"`
	Message    string   `json:"message"`
	Sources    []string `json:"sources,omitempty"`
	Blocking   bool     `json:"blocking"`
}

// TermGateResult holds the outcome of term gate checks.
type TermGateResult struct {
	Pass             bool          `json:"pass"`
	DefinitionCount  int           `json:"definition_count"`
	UsageCount       int           `json:"usage_count"`
	ConflictCount    int           `json:"conflict_count"`
	UndefinedCount   int           `json:"undefined_count"`
	Findings         []TermFinding `json:"findings,omitempty"`
}

// TermGateInput provides definitions and usages for checking.
type TermGateInput struct {
	Definitions []TermDefinition
	Usages      []TermUsage
}

// CheckTermGates detects definition conflicts and undefined terms.
func CheckTermGates(input TermGateInput) TermGateResult {
	result := TermGateResult{
		Pass:            true,
		DefinitionCount: len(input.Definitions),
		UsageCount:      len(input.Usages),
	}

	// Index definitions by normalized term.
	defsByTerm := map[string][]TermDefinition{}
	for _, d := range input.Definitions {
		key := normalize(d.Term)
		defsByTerm[key] = append(defsByTerm[key], d)
	}

	// Check for definition conflicts: same term, different definitions.
	for key, defs := range defsByTerm {
		if len(defs) < 2 {
			continue
		}

		// Group by unique definition text.
		uniqueDefs := map[string][]string{} // definition text → sources
		for _, d := range defs {
			normDef := strings.TrimSpace(d.Definition)
			uniqueDefs[normDef] = append(uniqueDefs[normDef], d.Source)
		}

		if len(uniqueDefs) > 1 {
			// Genuine conflict: same term, different definition texts.
			var sources []string
			for _, d := range defs {
				sources = appendUniq(sources, d.Source)
			}
			result.Findings = append(result.Findings, TermFinding{
				Term:     defs[0].Term,
				Code:     CodeDefinitionConflict,
				Severity: "high",
				Message:  fmt.Sprintf("term %q has %d conflicting definitions across %s", key, len(uniqueDefs), strings.Join(sources, ", ")),
				Sources:  sources,
				Blocking: true,
			})
			result.ConflictCount++
			result.Pass = false
		} else if len(defs) > 1 {
			// Same definition, multiple sources — duplicate warning.
			var sources []string
			for _, d := range defs {
				sources = appendUniq(sources, d.Source)
			}
			result.Findings = append(result.Findings, TermFinding{
				Term:     defs[0].Term,
				Code:     CodeDuplicateDefinition,
				Severity: "low",
				Message:  fmt.Sprintf("term %q defined identically in %d sources: %s", key, len(sources), strings.Join(sources, ", ")),
				Sources:  sources,
				Blocking: false,
			})
		}
	}

	// Check for undefined terms: used but not defined.
	definedTerms := map[string]bool{}
	for key := range defsByTerm {
		definedTerms[key] = true
	}

	undefinedSeen := map[string]bool{}
	for _, u := range input.Usages {
		key := normalize(u.Term)
		if definedTerms[key] || undefinedSeen[key] {
			continue
		}
		undefinedSeen[key] = true
		result.Findings = append(result.Findings, TermFinding{
			Term:     u.Term,
			Code:     CodeUndefinedTerm,
			Severity: "medium",
			Message:  fmt.Sprintf("term %q is used at %s but has no definition", u.Term, u.Location),
			Sources:  []string{u.Location},
			Blocking: true,
		})
		result.UndefinedCount++
		result.Pass = false
	}

	sort.Slice(result.Findings, func(i, j int) bool {
		if result.Findings[i].Code == result.Findings[j].Code {
			return result.Findings[i].Term < result.Findings[j].Term
		}
		return result.Findings[i].Code < result.Findings[j].Code
	})

	return result
}

func normalize(term string) string {
	return strings.ToLower(strings.TrimSpace(term))
}

func appendUniq(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

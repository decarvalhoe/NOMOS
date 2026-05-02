package atomization

import (
	"fmt"
	"regexp"
	"strings"
)

// CanonicalRef is a reference extracted from an atom to its source.
type CanonicalRef struct {
	AtomID    string `json:"atom_id"`
	SourceID  string `json:"source_id"`
	Locator   string `json:"locator"`
	Hash      string `json:"hash,omitempty"`
}

// CodeRef links an atom to its implementation.
type CodeRef struct {
	Module  string `json:"module"`
	Package string `json:"package,omitempty"`
	Symbol  string `json:"symbol,omitempty"`
}

// TestRef links an atom to its test.
type TestRef struct {
	Path   string `json:"path"`
	Symbol string `json:"symbol,omitempty"`
}

// TraceRow is a single row in the traceability matrix, linking
// atom -> source -> code -> test with coverage status.
type TraceRow struct {
	AtomID       string        `json:"atom_id"`
	AtomTitle    string        `json:"atom_title"`
	AtomType     string        `json:"atom_type"`
	Domain       string        `json:"domain"`
	Criticality  string        `json:"criticality"`
	BusinessRule string        `json:"business_rule"`
	SourceRefs   []CanonicalRef `json:"source_refs"`
	CodeRefs     []CodeRef      `json:"code_refs,omitempty"`
	TestRefs     []TestRef      `json:"test_refs,omitempty"`
	Gaps         []string       `json:"gaps,omitempty"`
	Status       string        `json:"status"`
}

// TraceMatrix is the full traceability matrix.
type TraceMatrix struct {
	SchemaVersion string     `json:"schema_version"`
	Domain        string     `json:"domain"`
	TotalRows     int        `json:"total_rows"`
	Covered       int        `json:"covered"`
	Partial       int        `json:"partial"`
	Missing       int        `json:"missing"`
	Rows          []TraceRow `json:"rows"`
}

// Atom is a minimal domain knowledge unit extracted from a parsed document.
type Atom struct {
	ID           string
	Title        string
	Type         string // rule, term, exception, formula, etc.
	Domain       string
	Criticality  string
	BusinessRule string
	SourceBlock  Block
}

// Reference patterns detected in atom text.
var (
	sourceRefRe = regexp.MustCompile(`(?i)\bsource[:=]\s*([A-Z0-9][A-Z0-9-]+)`)
	locatorRe   = regexp.MustCompile(`(?i)(?:\bp\.\s*\d+|\bart(?:icle)?\.?\s*\d+|\bs(?:ection)?\.?\s*\d+|\x{00a7}\s*\d+)`)
	codeRefRe   = regexp.MustCompile(`(?i)\b(?:implements?|see|ref)[:=]?\s*([A-Za-z0-9_./@:+-]+)`)
	testRefRe   = regexp.MustCompile(`(?i)\b(?:tested?\s+(?:by|in)|test[:=])\s*([A-Za-z0-9_./@:+-]+)`)
)

// ExtractReferences extracts canonical references from atom text.
func ExtractReferences(atom Atom) []CanonicalRef {
	text := atom.SourceBlock.Text
	if text == "" {
		text = atom.BusinessRule
	}

	var refs []CanonicalRef
	seen := map[string]bool{}

	// Extract explicit source references.
	for _, m := range sourceRefRe.FindAllStringSubmatch(text, -1) {
		sourceID := m[1]
		locator := extractLocator(text)
		key := sourceID + ":" + locator
		if seen[key] {
			continue
		}
		seen[key] = true
		refs = append(refs, CanonicalRef{
			AtomID:   atom.ID,
			SourceID: sourceID,
			Locator:  locator,
			Hash:     atom.SourceBlock.Hash,
		})
	}

	// If no explicit source ref, create one from block context.
	if len(refs) == 0 && atom.SourceBlock.ID != "" {
		refs = append(refs, CanonicalRef{
			AtomID:   atom.ID,
			SourceID: inferSourceID(atom),
			Locator:  fmt.Sprintf("line:%d", atom.SourceBlock.Span.StartLine),
			Hash:     atom.SourceBlock.Hash,
		})
	}

	return refs
}

// ExtractCodeRefs extracts code references from atom text.
func ExtractCodeRefs(atom Atom) []CodeRef {
	text := atom.SourceBlock.Text
	if text == "" {
		text = atom.BusinessRule
	}

	var refs []CodeRef
	seen := map[string]bool{}

	for _, m := range codeRefRe.FindAllStringSubmatch(text, -1) {
		path := m[1]
		if seen[path] || !looksLikeCodePath(path) {
			continue
		}
		seen[path] = true
		refs = append(refs, CodeRef{
			Module: path,
		})
	}
	return refs
}

// ExtractTestRefs extracts test references from atom text.
func ExtractTestRefs(atom Atom) []TestRef {
	text := atom.SourceBlock.Text
	if text == "" {
		text = atom.BusinessRule
	}

	var refs []TestRef
	seen := map[string]bool{}

	for _, m := range testRefRe.FindAllStringSubmatch(text, -1) {
		path := m[1]
		if seen[path] {
			continue
		}
		seen[path] = true
		refs = append(refs, TestRef{Path: path})
	}
	return refs
}

// ProjectTraceMatrix builds a traceability matrix from a list of atoms.
func ProjectTraceMatrix(atoms []Atom, domain string) TraceMatrix {
	var rows []TraceRow
	covered, partial, missing := 0, 0, 0

	for _, atom := range atoms {
		sourceRefs := ExtractReferences(atom)
		codeRefs := ExtractCodeRefs(atom)
		testRefs := ExtractTestRefs(atom)

		var gaps []string
		if len(sourceRefs) == 0 {
			gaps = append(gaps, "no source reference")
		}
		if len(codeRefs) == 0 {
			gaps = append(gaps, "no code reference")
		}
		if len(testRefs) == 0 {
			gaps = append(gaps, "no test reference")
		}

		status := computeStatus(sourceRefs, codeRefs, testRefs)
		switch status {
		case "covered":
			covered++
		case "partial":
			partial++
		case "missing":
			missing++
		}

		d := atom.Domain
		if d == "" {
			d = domain
		}

		rows = append(rows, TraceRow{
			AtomID:       atom.ID,
			AtomTitle:    atom.Title,
			AtomType:     atom.Type,
			Domain:       d,
			Criticality:  atom.Criticality,
			BusinessRule: atom.BusinessRule,
			SourceRefs:   sourceRefs,
			CodeRefs:     codeRefs,
			TestRefs:     testRefs,
			Gaps:         gaps,
			Status:       status,
		})
	}

	return TraceMatrix{
		SchemaVersion: "0.1.0",
		Domain:        domain,
		TotalRows:     len(rows),
		Covered:       covered,
		Partial:        partial,
		Missing:        missing,
		Rows:          rows,
	}
}

func computeStatus(sources []CanonicalRef, code []CodeRef, tests []TestRef) string {
	hasSource := len(sources) > 0
	hasCode := len(code) > 0
	hasTest := len(tests) > 0

	if hasSource && hasCode && hasTest {
		return "covered"
	}
	if hasSource || hasCode || hasTest {
		return "partial"
	}
	return "missing"
}

func extractLocator(text string) string {
	m := locatorRe.FindString(text)
	if m != "" {
		return strings.TrimSpace(m)
	}
	return ""
}

func inferSourceID(atom Atom) string {
	if atom.Domain != "" {
		return strings.ToUpper(strings.ReplaceAll(atom.Domain, "-", "")) + "-SRC"
	}
	return "UNKNOWN-SRC"
}

func looksLikeCodePath(s string) bool {
	return strings.Contains(s, "/") || strings.Contains(s, ".")
}

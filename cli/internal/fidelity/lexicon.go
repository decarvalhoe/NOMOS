package fidelity

import (
	"fmt"
	"sort"
	"strings"
)

// TermStatus tracks the governance lifecycle of a term.
type TermStatus string

const (
	TermCanonical  TermStatus = "canonical"
	TermSynonym    TermStatus = "synonym"
	TermDeprecated TermStatus = "deprecated"
)

// Term is a single entry in the controlled lexicon.
type Term struct {
	Canonical   string     `json:"canonical" yaml:"canonical"`
	Status      TermStatus `json:"status" yaml:"status"`
	Synonyms    []string   `json:"synonyms,omitempty" yaml:"synonyms,omitempty"`
	Deprecated  []string   `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	Domain      string     `json:"domain,omitempty" yaml:"domain,omitempty"`
	Definition  string     `json:"definition,omitempty" yaml:"definition,omitempty"`
	Reference   string     `json:"reference,omitempty" yaml:"reference,omitempty"`
}

// Lexicon is a controlled vocabulary for a domain.
type Lexicon struct {
	terms       map[string]*Term   // canonical lower → Term
	synonymMap  map[string]string  // synonym lower → canonical lower
	deprecated  map[string]string  // deprecated lower → canonical lower
}

// NewLexicon creates an empty lexicon.
func NewLexicon() *Lexicon {
	return &Lexicon{
		terms:      make(map[string]*Term),
		synonymMap: make(map[string]string),
		deprecated: make(map[string]string),
	}
}

// Add registers a term with its synonyms and deprecated forms.
func (l *Lexicon) Add(t Term) error {
	key := strings.ToLower(strings.TrimSpace(t.Canonical))
	if key == "" {
		return fmt.Errorf("canonical term is empty")
	}
	if _, exists := l.terms[key]; exists {
		return fmt.Errorf("term %q already registered", t.Canonical)
	}

	t.Status = TermCanonical
	l.terms[key] = &t

	for _, syn := range t.Synonyms {
		sk := strings.ToLower(strings.TrimSpace(syn))
		if sk != "" {
			l.synonymMap[sk] = key
		}
	}
	for _, dep := range t.Deprecated {
		dk := strings.ToLower(strings.TrimSpace(dep))
		if dk != "" {
			l.deprecated[dk] = key
		}
	}

	return nil
}

// Resolve looks up a word and returns its canonical form and status.
// Returns ("", "") if the word is not governed.
func (l *Lexicon) Resolve(word string) (canonical string, status TermStatus) {
	key := strings.ToLower(strings.TrimSpace(word))

	if t, ok := l.terms[key]; ok {
		return t.Canonical, TermCanonical
	}
	if canon, ok := l.synonymMap[key]; ok {
		return l.terms[canon].Canonical, TermSynonym
	}
	if canon, ok := l.deprecated[key]; ok {
		return l.terms[canon].Canonical, TermDeprecated
	}
	return "", ""
}

// IsGoverned returns true if the word is known to the lexicon.
func (l *Lexicon) IsGoverned(word string) bool {
	_, status := l.Resolve(word)
	return status != ""
}

// TermCount returns the number of canonical terms.
func (l *Lexicon) TermCount() int {
	return len(l.terms)
}

// AllTerms returns all canonical terms sorted alphabetically.
func (l *Lexicon) AllTerms() []Term {
	result := make([]Term, 0, len(l.terms))
	for _, t := range l.terms {
		result = append(result, *t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Canonical < result[j].Canonical
	})
	return result
}

// Finding describes a single lexicon gate violation.
type Finding struct {
	Word       string     `json:"word"`
	Status     TermStatus `json:"status,omitempty"`
	Canonical  string     `json:"canonical,omitempty"`
	Location   string     `json:"location,omitempty"`
	Code       string     `json:"code"`
	Message    string     `json:"message"`
	Blocking   bool       `json:"blocking"`
}

// Gate codes.
const (
	CodeUngoverned     = "UNGOVERNED_TERM"
	CodeDeprecatedTerm = "DEPRECATED_TERM"
)

// GateResult holds the outcome of a lexicon gate check.
type GateResult struct {
	Pass     bool      `json:"pass"`
	Checked  int       `json:"checked"`
	Findings []Finding `json:"findings,omitempty"`
}

// CheckText scans text for ungoverned or deprecated terms.
// It tokenizes the text into words and checks each against the lexicon.
// location is an optional label (e.g. "atom:A-1234" or "file:doc.md:15").
func (l *Lexicon) CheckText(text string, location string) GateResult {
	words := tokenize(text)
	result := GateResult{Pass: true, Checked: len(words)}

	seen := map[string]bool{}
	for _, word := range words {
		key := strings.ToLower(word)
		if seen[key] {
			continue
		}
		seen[key] = true

		canonical, status := l.Resolve(word)

		switch status {
		case TermDeprecated:
			result.Findings = append(result.Findings, Finding{
				Word:      word,
				Status:    TermDeprecated,
				Canonical: canonical,
				Location:  location,
				Code:      CodeDeprecatedTerm,
				Message:   fmt.Sprintf("term %q is deprecated; use %q instead", word, canonical),
				Blocking:  true,
			})
			result.Pass = false
		case "":
			// Skip short common words.
			if len(word) <= 2 {
				continue
			}
			result.Findings = append(result.Findings, Finding{
				Word:     word,
				Location: location,
				Code:     CodeUngoverned,
				Message:  fmt.Sprintf("term %q is not in the controlled lexicon", word),
				Blocking: false,
			})
		}
	}

	return result
}

// CheckTerms checks a list of explicit terms (not free text).
// Every term must be governed. Ungoverned terms are blocking.
func (l *Lexicon) CheckTerms(terms []string, location string) GateResult {
	result := GateResult{Pass: true, Checked: len(terms)}

	for _, term := range terms {
		canonical, status := l.Resolve(term)

		switch status {
		case TermDeprecated:
			result.Findings = append(result.Findings, Finding{
				Word:      term,
				Status:    TermDeprecated,
				Canonical: canonical,
				Location:  location,
				Code:      CodeDeprecatedTerm,
				Message:   fmt.Sprintf("term %q is deprecated; use %q", term, canonical),
				Blocking:  true,
			})
			result.Pass = false
		case "":
			result.Findings = append(result.Findings, Finding{
				Word:     term,
				Location: location,
				Code:     CodeUngoverned,
				Message:  fmt.Sprintf("term %q is not governed", term),
				Blocking: true,
			})
			result.Pass = false
		}
	}

	return result
}

func tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if isWordChar(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '-' || r == '_' ||
		r == '\'' || r >= 0x00C0 // accented chars
}

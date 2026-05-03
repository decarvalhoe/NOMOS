package fidelity

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// LexiconArtifact is the serializable output of lexicon extraction.
type LexiconArtifact struct {
	SchemaVersion string          `json:"schema_version" yaml:"schema_version"`
	Domain        string          `json:"domain" yaml:"domain"`
	SourceHash    string          `json:"source_hash" yaml:"source_hash"`
	TermCount     int             `json:"term_count" yaml:"term_count"`
	Terms         []ExtractedTerm `json:"terms" yaml:"terms"`
}

// ExtractedTerm is a term/definition pair extracted from source content.
type ExtractedTerm struct {
	Term       string   `json:"term" yaml:"term"`
	Definition string   `json:"definition" yaml:"definition"`
	SourceNode string   `json:"source_node,omitempty" yaml:"source_node,omitempty"`
	Line       int      `json:"line,omitempty" yaml:"line,omitempty"`
	Confidence string   `json:"confidence" yaml:"confidence"` // "high", "medium", "low"
	Patterns   []string `json:"patterns,omitempty" yaml:"patterns,omitempty"`
}

// ExtractionConfig configures lexicon extraction behavior.
type ExtractionConfig struct {
	Domain     string
	SourceHash string
	// MinDefinitionLength filters out very short definitions.
	MinDefinitionLength int
}

// Definition patterns commonly found in technical/legal documents.
var (
	// "Term: definition" or "Term — definition"
	colonDefPattern = regexp.MustCompile(`^([A-Z][A-Za-zÀ-ÿ\s'-]{2,50})\s*[:—–]\s*(.{10,})$`)
	// "**Term** definition" (bold term followed by text)
	boldDefPattern = regexp.MustCompile(`^\*\*([^*]{2,50})\*\*\s*[:—–]?\s*(.{10,})$`)
	// "- **Term**: definition" (list item with bold term)
	listDefPattern = regexp.MustCompile(`^[-*]\s+\*\*([^*]{2,50})\*\*\s*[:—–]?\s*(.{10,})$`)
	// Glossary heading detection
	glossaryHeadingPattern = regexp.MustCompile(`(?i)^#+\s*(glossaire|glossary|definitions|définitions|termes|terminology|vocabulary|vocabulaire)`)
)

// ExtractLexicon extracts terms and definitions from parsed CAST nodes.
func ExtractLexicon(cast CAST, config ExtractionConfig) LexiconArtifact {
	minLen := config.MinDefinitionLength
	if minLen == 0 {
		minLen = 10
	}

	var terms []ExtractedTerm
	inGlossarySection := false

	for _, node := range cast.Nodes {
		text := node.RawText
		if text == "" {
			text = node.Text
		}

		// Detect glossary sections.
		if string(node.Kind) == "heading" {
			inGlossarySection = glossaryHeadingPattern.MatchString(text)
			continue
		}

		// Extract from paragraph/list_item content.
		if string(node.Kind) != "paragraph" && string(node.Kind) != "list_item" {
			continue
		}

		extracted := extractFromText(text, node.ID, node.Span.StartLine, minLen, inGlossarySection)
		terms = append(terms, extracted...)
	}

	// Deduplicate by term (keep highest confidence).
	terms = deduplicateTerms(terms)

	sort.Slice(terms, func(i, j int) bool {
		return strings.ToLower(terms[i].Term) < strings.ToLower(terms[j].Term)
	})

	return LexiconArtifact{
		SchemaVersion: "0.1.0",
		Domain:        config.Domain,
		SourceHash:    config.SourceHash,
		TermCount:     len(terms),
		Terms:         terms,
	}
}

// WriteLexiconYAML writes the lexicon artifact as YAML.
func WriteLexiconYAML(w io.Writer, artifact LexiconArtifact) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(artifact)
}

// ToLexicon converts an extracted artifact to the runtime Lexicon type.
func (a LexiconArtifact) ToLexicon() *Lexicon {
	lex := NewLexicon()
	for _, et := range a.Terms {
		_ = lex.Add(Term{
			Canonical:  et.Term,
			Status:     TermCanonical,
			Domain:     a.Domain,
			Definition: et.Definition,
		})
	}
	return lex
}

func extractFromText(text string, nodeID string, line int, minLen int, inGlossary bool) []ExtractedTerm {
	var terms []ExtractedTerm

	// Try multi-line: split into lines for list-style definitions.
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		l = strings.TrimSpace(l)
		if len(l) < minLen {
			continue
		}

		if et := matchPattern(listDefPattern, l, "list_definition", nodeID, line+i, inGlossary); et != nil {
			terms = append(terms, *et)
			continue
		}
		if et := matchPattern(boldDefPattern, l, "bold_definition", nodeID, line+i, inGlossary); et != nil {
			terms = append(terms, *et)
			continue
		}
		if et := matchPattern(colonDefPattern, l, "colon_definition", nodeID, line+i, inGlossary); et != nil {
			terms = append(terms, *et)
			continue
		}
	}

	return terms
}

func matchPattern(pattern *regexp.Regexp, text string, patternName string, nodeID string, line int, inGlossary bool) *ExtractedTerm {
	matches := pattern.FindStringSubmatch(text)
	if matches == nil || len(matches) < 3 {
		return nil
	}

	term := strings.TrimSpace(matches[1])
	definition := strings.TrimSpace(matches[2])

	if len(term) < 2 || len(definition) < 5 {
		return nil
	}

	confidence := "medium"
	if inGlossary {
		confidence = "high"
	}

	return &ExtractedTerm{
		Term:       term,
		Definition: definition,
		SourceNode: nodeID,
		Line:       line,
		Confidence: confidence,
		Patterns:   []string{patternName},
	}
}

func deduplicateTerms(terms []ExtractedTerm) []ExtractedTerm {
	byTerm := map[string]ExtractedTerm{}
	for _, t := range terms {
		key := strings.ToLower(t.Term)
		existing, ok := byTerm[key]
		if !ok || confidenceRank(t.Confidence) > confidenceRank(existing.Confidence) {
			byTerm[key] = t
		}
	}

	result := make([]ExtractedTerm, 0, len(byTerm))
	for _, t := range byTerm {
		result = append(result, t)
	}
	return result
}

func confidenceRank(c string) int {
	switch c {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// ValidateLexiconArtifact checks structural validity.
func ValidateLexiconArtifact(a LexiconArtifact) []string {
	var errs []string
	if a.Domain == "" {
		errs = append(errs, "domain is required")
	}
	if a.TermCount != len(a.Terms) {
		errs = append(errs, fmt.Sprintf("term_count %d != len(terms) %d", a.TermCount, len(a.Terms)))
	}
	seen := map[string]bool{}
	for i, t := range a.Terms {
		if t.Term == "" {
			errs = append(errs, fmt.Sprintf("terms[%d].term is required", i))
		}
		if t.Definition == "" {
			errs = append(errs, fmt.Sprintf("terms[%d].definition is required", i))
		}
		key := strings.ToLower(t.Term)
		if seen[key] {
			errs = append(errs, fmt.Sprintf("terms[%d].term %q is duplicated", i, t.Term))
		}
		seen[key] = true
	}
	return errs
}

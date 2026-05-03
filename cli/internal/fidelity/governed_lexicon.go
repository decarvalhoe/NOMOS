package fidelity

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// GovernedTerm is a single term in the governed lexicon artifact.
type GovernedTerm struct {
	Term       string   `json:"term"       yaml:"term"`
	Definition string   `json:"definition" yaml:"definition"`
	Source     string   `json:"source"     yaml:"source"`
	SourceLine int      `json:"source_line,omitempty" yaml:"source_line,omitempty"`
	Status     string   `json:"status"     yaml:"status"` // defined, used, undefined
	Domain     string   `json:"domain,omitempty" yaml:"domain,omitempty"`
	Synonyms   []string `json:"synonyms,omitempty" yaml:"synonyms,omitempty"`
}

// GovernedLexicon is the governed lexicon artifact.
type GovernedLexicon struct {
	SchemaVersion string         `json:"schema_version" yaml:"schema_version"`
	Domain        string         `json:"domain"         yaml:"domain"`
	TotalDefined  int            `json:"total_defined"  yaml:"total_defined"`
	TotalUsed     int            `json:"total_used"     yaml:"total_used"`
	TotalUndefined int           `json:"total_undefined" yaml:"total_undefined"`
	Terms         []GovernedTerm `json:"terms"          yaml:"terms"`
}

// GovernedLexiconGate is the gate result for lexicon governance.
type GovernedLexiconGate struct {
	Pass          bool             `json:"pass"`
	TotalTerms    int              `json:"total_terms"`
	Defined       int              `json:"defined"`
	Undefined     int              `json:"undefined"`
	UndefinedList []string         `json:"undefined_list,omitempty"`
	Verdict       string           `json:"verdict"`
}

// Definition patterns — detect "Term : definition" or "Term means ..." etc.
var defPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^([A-Z][a-zéèêëàâäùûüïîôöç]+(?:\s+[a-zéèêëàâäùûüïîôöç]+){0,3})\s*:\s+(.{10,})`),
	regexp.MustCompile(`(?i)(?:on entend par|au sens d[ue])\s+[«"]?([^»",:]+)[»"]?\s*[:,]\s*(.{10,})`),
	regexp.MustCompile(`(?i)(?:the term|le terme)\s+[«"]?([^»",:]+)[»"]?\s+(?:means?|désigne|designe)\s+(.{10,})`),
}

// ExtractDefinitions scans CAST nodes for term definitions.
func ExtractDefinitions(cast CAST, domain string) []GovernedTerm {
	var terms []GovernedTerm
	seen := map[string]bool{}

	for _, node := range cast.Nodes {
		text := node.Text
		if text == "" {
			continue
		}

		for _, pat := range defPatterns {
			matches := pat.FindAllStringSubmatch(text, -1)
			for _, m := range matches {
				term := strings.TrimSpace(m[1])
				def := strings.TrimSpace(m[2])
				// Truncate definition at sentence end.
				if idx := strings.Index(def, ". "); idx > 0 {
					def = def[:idx+1]
				}
				key := strings.ToLower(term)
				if seen[key] {
					continue
				}
				seen[key] = true
				terms = append(terms, GovernedTerm{
					Term:       term,
					Definition: def,
					Source:     node.ID,
					SourceLine: node.Span.StartLine,
					Status:     "defined",
					Domain:     domain,
				})
			}
		}
	}

	sort.Slice(terms, func(i, j int) bool {
		return strings.ToLower(terms[i].Term) < strings.ToLower(terms[j].Term)
	})
	return terms
}

// BuildGovernedLexicon creates a governed lexicon from extracted definitions
// and checks which terms from usedTerms are not defined.
func BuildGovernedLexicon(defined []GovernedTerm, usedTerms []string, domain string) GovernedLexicon {
	definedSet := map[string]bool{}
	for _, d := range defined {
		definedSet[strings.ToLower(d.Term)] = true
	}

	var allTerms []GovernedTerm
	allTerms = append(allTerms, defined...)

	// Find used-but-undefined terms.
	seenUsed := map[string]bool{}
	totalUsed := 0
	totalUndefined := 0
	for _, u := range usedTerms {
		key := strings.ToLower(strings.TrimSpace(u))
		if key == "" || len(key) < 3 {
			continue
		}
		totalUsed++
		if definedSet[key] || seenUsed[key] {
			continue
		}
		seenUsed[key] = true
		if !definedSet[key] {
			totalUndefined++
			allTerms = append(allTerms, GovernedTerm{
				Term:   u,
				Status: "undefined",
				Domain: domain,
			})
		}
	}

	sort.Slice(allTerms, func(i, j int) bool {
		return strings.ToLower(allTerms[i].Term) < strings.ToLower(allTerms[j].Term)
	})

	return GovernedLexicon{
		SchemaVersion:  "0.1.0",
		Domain:         domain,
		TotalDefined:   len(defined),
		TotalUsed:      totalUsed,
		TotalUndefined: totalUndefined,
		Terms:          allTerms,
	}
}

// CheckGovernedLexicon runs the gate: fails if any term is used but undefined.
func CheckGovernedLexicon(lexicon GovernedLexicon) GovernedLexiconGate {
	var undefined []string
	for _, t := range lexicon.Terms {
		if t.Status == "undefined" {
			undefined = append(undefined, t.Term)
		}
	}

	verdict := "pass"
	pass := true
	if len(undefined) > 0 {
		verdict = "fail"
		pass = false
	}

	return GovernedLexiconGate{
		Pass:          pass,
		TotalTerms:    len(lexicon.Terms),
		Defined:       lexicon.TotalDefined,
		Undefined:     len(undefined),
		UndefinedList: undefined,
		Verdict:       verdict,
	}
}

// MarshalGovernedLexicon serializes to YAML bytes.
func MarshalGovernedLexicon(lexicon GovernedLexicon) ([]byte, error) {
	return yaml.Marshal(lexicon)
}

// MergeWithLexicon imports GovernedTerm entries into a Lexicon.
func MergeWithLexicon(lex *Lexicon, governed []GovernedTerm) int {
	added := 0
	for _, g := range governed {
		if g.Status != "defined" {
			continue
		}
		err := lex.Add(Term{
			Canonical:  g.Term,
			Definition: g.Definition,
			Domain:     g.Domain,
			Reference:  fmt.Sprintf("source:%s line:%d", g.Source, g.SourceLine),
			Synonyms:   g.Synonyms,
		})
		if err == nil {
			added++
		}
	}
	return added
}

// ExtractUsedTerms collects candidate domain terms from CAST node text.
// It returns capitalized multi-word phrases likely to be domain terms.
var domainTermRe = regexp.MustCompile(`\b([A-ZÉÈÊËÀÂÄÙÛÜ][a-zéèêëàâäùûüïîôöç]{2,})\b`)

func ExtractUsedTerms(cast CAST) []string {
	seen := map[string]bool{}
	var terms []string

	for _, node := range cast.Nodes {
		if node.Kind == KindHeading || node.Kind == KindDocument {
			continue
		}
		text := node.Text
		if text == "" {
			continue
		}
		for _, m := range domainTermRe.FindAllStringSubmatch(text, -1) {
			term := strings.TrimSpace(m[1])
			key := strings.ToLower(term)
			if len(key) < 3 || seen[key] {
				continue
			}
			// Skip common words.
			if isCommonWord(key) {
				continue
			}
			seen[key] = true
			terms = append(terms, term)
		}
	}
	sort.Strings(terms)
	return terms
}

var commonWords = map[string]bool{
	"les": true, "des": true, "une": true, "dans": true, "pour": true,
	"par": true, "sur": true, "avec": true, "sont": true, "est": true,
	"the": true, "and": true, "for": true, "with": true, "from": true,
	"this": true, "that": true, "not": true, "all": true, "can": true,
	"will": true, "has": true, "have": true, "had": true, "but": true,
	"also": true, "each": true, "every": true, "some": true, "any": true,
	"tout": true, "tous": true, "toute": true, "toutes": true,
	"autre": true, "autres": true, "entre": true, "comme": true,
	"plus": true, "moins": true, "donc": true, "mais": true,
}

func isCommonWord(word string) bool {
	return commonWords[word]
}

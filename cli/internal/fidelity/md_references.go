package fidelity

import (
	"regexp"
	"strconv"
	"strings"
)

// RefType classifies the kind of reference candidate.
type RefType string

const (
	RefAnnex          RefType = "annex"
	RefBibliographic  RefType = "bibliographic"
	RefCrossReference RefType = "cross_reference"
)

// RefCandidate represents a detected reference in parsed Markdown.
type RefCandidate struct {
	Type       RefType `json:"type"`
	Raw        string  `json:"raw"`
	Target     string  `json:"target"`
	Label      string  `json:"label,omitempty"`
	Line       int     `json:"line"`
	Confidence string  `json:"confidence"` // high, medium, low
}

// Reference detection patterns.
var (
	// Annex patterns: "Annexe A", "Annex 1", "Appendix B"
	annexPattern = regexp.MustCompile(`(?i)\b(annex[e]?|appendix|app\.)\s+([A-Z0-9][A-Z0-9.-]*)`)

	// Bibliographic patterns: "[1]", "[RFC 2119]", "(Author, 2024)"
	biblioSquarePattern = regexp.MustCompile(`\[([A-Za-z0-9][A-Za-z0-9 ._-]*)\]`)
	biblioParenPattern  = regexp.MustCompile(`\(([A-Z][a-z]+(?:\s+(?:et\s+al\.|&\s+[A-Z][a-z]+))?),?\s*((?:19|20)\d{2})\)`)

	// Cross-reference patterns: "see Section 3.2", "cf. Article 5", "per §12"
	crossRefPattern = regexp.MustCompile(`(?i)\b(see|cf\.|per|selon|voir|ref\.|as per)\s+(?:(?:section|article|clause|chapter|§|chapitre|titre|alinea)\s*)?([A-Z0-9][A-Z0-9._-]*)`)

	// Internal link pattern: [text](#anchor) or [text](./path)
	internalLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	// Section ref: "Section 3.2", "Article L.113-2", "Chapter IV"
	sectionRefPattern = regexp.MustCompile(`(?i)\b(section|article|clause|chapter|chapitre|titre|§)\s+([A-Z0-9][A-Z0-9._-]*)`)
)

// DetectReferences scans Markdown content line by line and returns reference candidates.
func DetectReferences(content string) []RefCandidate {
	lines := strings.Split(content, "\n")
	var candidates []RefCandidate

	for i, line := range lines {
		lineNum := i + 1
		candidates = append(candidates, detectAnnexes(line, lineNum)...)
		candidates = append(candidates, detectBibliographic(line, lineNum)...)
		candidates = append(candidates, detectCrossRefs(line, lineNum)...)
	}

	return deduplicateCandidates(candidates)
}

func detectAnnexes(line string, lineNum int) []RefCandidate {
	var results []RefCandidate
	matches := annexPattern.FindAllStringSubmatch(line, -1)
	for _, m := range matches {
		results = append(results, RefCandidate{
			Type:       RefAnnex,
			Raw:        m[0],
			Target:     strings.ToUpper(m[2]),
			Label:      m[1] + " " + m[2],
			Line:       lineNum,
			Confidence: "high",
		})
	}
	return results
}

func detectBibliographic(line string, lineNum int) []RefCandidate {
	var results []RefCandidate

	// Square bracket refs [RFC 2119], [1]
	squareMatches := biblioSquarePattern.FindAllStringSubmatch(line, -1)
	for _, m := range squareMatches {
		inner := m[1]
		// Skip markdown links [text](url) — these have a ( immediately after ]
		idx := strings.Index(line, m[0])
		if idx >= 0 && idx+len(m[0]) < len(line) && line[idx+len(m[0])] == '(' {
			continue
		}
		// Skip if it looks like a checkbox [ ] or [x]
		if inner == " " || inner == "x" || inner == "X" {
			continue
		}
		confidence := "medium"
		if isLikelyBiblioRef(inner) {
			confidence = "high"
		}
		results = append(results, RefCandidate{
			Type:       RefBibliographic,
			Raw:        m[0],
			Target:     inner,
			Line:       lineNum,
			Confidence: confidence,
		})
	}

	// Parenthetical refs (Author, 2024)
	parenMatches := biblioParenPattern.FindAllStringSubmatch(line, -1)
	for _, m := range parenMatches {
		results = append(results, RefCandidate{
			Type:       RefBibliographic,
			Raw:        m[0],
			Target:     m[1] + " " + m[2],
			Label:      m[1],
			Line:       lineNum,
			Confidence: "high",
		})
	}

	return results
}

func detectCrossRefs(line string, lineNum int) []RefCandidate {
	var results []RefCandidate

	// Explicit cross-references: "see Section X", "cf. Article Y"
	crossMatches := crossRefPattern.FindAllStringSubmatch(line, -1)
	for _, m := range crossMatches {
		results = append(results, RefCandidate{
			Type:       RefCrossReference,
			Raw:        m[0],
			Target:     m[2],
			Label:      m[1],
			Line:       lineNum,
			Confidence: "high",
		})
	}

	// Internal links [text](#anchor)
	linkMatches := internalLinkPattern.FindAllStringSubmatch(line, -1)
	for _, m := range linkMatches {
		target := m[2]
		if strings.HasPrefix(target, "#") || strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../") {
			results = append(results, RefCandidate{
				Type:       RefCrossReference,
				Raw:        m[0],
				Target:     target,
				Label:      m[1],
				Line:       lineNum,
				Confidence: "high",
			})
		}
	}

	// Standalone section refs: "Article L.113-2", "Section 3.2"
	sectionMatches := sectionRefPattern.FindAllStringSubmatch(line, -1)
	for _, m := range sectionMatches {
		// Avoid duplicates with crossRefPattern
		raw := m[0]
		alreadyFound := false
		for _, existing := range results {
			if strings.Contains(existing.Raw, raw) || strings.Contains(raw, existing.Raw) {
				alreadyFound = true
				break
			}
		}
		if !alreadyFound {
			results = append(results, RefCandidate{
				Type:       RefCrossReference,
				Raw:        raw,
				Target:     m[2],
				Label:      m[1],
				Line:       lineNum,
				Confidence: "medium",
			})
		}
	}

	return results
}

func isLikelyBiblioRef(s string) bool {
	s = strings.TrimSpace(s)
	// Numeric: [1], [23]
	if len(s) <= 3 {
		allDigit := true
		for _, r := range s {
			if r < '0' || r > '9' {
				allDigit = false
				break
			}
		}
		if allDigit {
			return true
		}
	}
	// RFC, ISO, etc.
	upper := strings.ToUpper(s)
	for _, prefix := range []string{"RFC", "ISO", "IEEE", "NIST", "ICH", "GAMP"} {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

func deduplicateCandidates(candidates []RefCandidate) []RefCandidate {
	seen := map[string]bool{}
	var result []RefCandidate
	for _, c := range candidates {
		key := string(c.Type) + "|" + c.Raw + "|" + strconv.Itoa(c.Line)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, c)
	}
	return result
}


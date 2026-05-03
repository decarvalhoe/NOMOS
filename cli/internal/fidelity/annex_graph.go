package fidelity

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// AnnexEntry represents a detected annex in the document.
type AnnexEntry struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Title    string `json:"title,omitempty"`
	Line     int    `json:"line"`
	RefCount int    `json:"ref_count"`
}

// BiblioEntry represents a detected bibliography item.
type BiblioEntry struct {
	ID       string `json:"id"`
	Citation string `json:"citation"`
	Line     int    `json:"line"`
	RefCount int    `json:"ref_count"`
}

// AnnexRef records where an annex or biblio item is referenced.
type AnnexRef struct {
	SourceLine int    `json:"source_line"`
	TargetID   string `json:"target_id"`
	Context    string `json:"context,omitempty"`
}

// AnnexBiblioGraph holds the full annex/bibliography structure.
type AnnexBiblioGraph struct {
	Annexes     []AnnexEntry  `json:"annexes"`
	Bibliography []BiblioEntry `json:"bibliography"`
	References  []AnnexRef    `json:"references"`
	Orphans     []string      `json:"orphans"`
	Unreferenced []string     `json:"unreferenced"`
}

// Detection patterns
var (
	agAnnexHeadingRe = regexp.MustCompile(`(?im)^#{1,3}\s+(annex[e]?|appendix|app\.)\s+([A-Z0-9][A-Z0-9.-]*)\s*[:\-—]?\s*(.*)$`)
	agAnnexInlineRe  = regexp.MustCompile(`(?i)\b(annex[e]?|appendix|app\.)\s+([A-Z0-9][A-Z0-9-]*)`)
	agBiblioDefRe    = regexp.MustCompile(`(?m)^\[([A-Za-z0-9][A-Za-z0-9 ._-]*)\]\s*[:.]?\s*(.+)$`)
	agBiblioRefRe    = regexp.MustCompile(`\[([A-Za-z0-9][A-Za-z0-9 ._-]*)\]`)
)

// BuildAnnexGraph scans Markdown content and builds the annex/bibliography graph.
func BuildAnnexGraph(content string) AnnexBiblioGraph {
	lines := strings.Split(content, "\n")

	annexes := detectAnnexDefinitions(lines)
	biblio := detectBibliographyDefinitions(lines)
	refs := detectAllReferences(lines, annexes, biblio)

	// Count references per target.
	refCounts := map[string]int{}
	for _, ref := range refs {
		refCounts[ref.TargetID]++
	}
	for i := range annexes {
		annexes[i].RefCount = refCounts[annexes[i].ID]
	}
	for i := range biblio {
		biblio[i].RefCount = refCounts[biblio[i].ID]
	}

	// Find orphans: references to undefined targets.
	defined := buildDefinedSet(annexes, biblio)
	orphanSet := map[string]bool{}
	for _, ref := range refs {
		if !defined[ref.TargetID] {
			orphanSet[ref.TargetID] = true
		}
	}
	orphans := sortedKeys(orphanSet)

	// Find unreferenced: defined but never cited.
	var unreferenced []string
	for _, a := range annexes {
		if a.RefCount == 0 {
			unreferenced = append(unreferenced, a.ID)
		}
	}
	for _, b := range biblio {
		if b.RefCount == 0 {
			unreferenced = append(unreferenced, b.ID)
		}
	}
	sort.Strings(unreferenced)

	return AnnexBiblioGraph{
		Annexes:      annexes,
		Bibliography: biblio,
		References:   refs,
		Orphans:      orphans,
		Unreferenced: unreferenced,
	}
}

// IsComplete returns true if there are no orphans (all refs resolve).
func (g AnnexBiblioGraph) IsComplete() bool {
	return len(g.Orphans) == 0
}

// Stats returns summary counts.
func (g AnnexBiblioGraph) Stats() AnnexGraphStats {
	return AnnexGraphStats{
		AnnexCount:       len(g.Annexes),
		BiblioCount:      len(g.Bibliography),
		ReferenceCount:   len(g.References),
		OrphanCount:      len(g.Orphans),
		UnreferencedCount: len(g.Unreferenced),
	}
}

// AnnexGraphStats summarizes the graph.
type AnnexGraphStats struct {
	AnnexCount        int `json:"annex_count"`
	BiblioCount       int `json:"biblio_count"`
	ReferenceCount    int `json:"reference_count"`
	OrphanCount       int `json:"orphan_count"`
	UnreferencedCount int `json:"unreferenced_count"`
}

func detectAnnexDefinitions(lines []string) []AnnexEntry {
	var annexes []AnnexEntry
	seen := map[string]bool{}

	for i, line := range lines {
		matches := agAnnexHeadingRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		id := strings.ToUpper(matches[2])
		if seen[id] {
			continue
		}
		seen[id] = true
		annexes = append(annexes, AnnexEntry{
			ID:    fmt.Sprintf("ANNEX-%s", id),
			Label: matches[1] + " " + matches[2],
			Title: strings.TrimSpace(matches[3]),
			Line:  i + 1,
		})
	}
	return annexes
}

func detectBibliographyDefinitions(lines []string) []BiblioEntry {
	var biblio []BiblioEntry
	seen := map[string]bool{}

	for i, line := range lines {
		matches := agBiblioDefRe.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		id := matches[1]
		if seen[id] {
			continue
		}
		// Skip if it looks like a markdown link [text](url).
		if i < len(lines) && strings.Contains(line, "](") {
			continue
		}
		seen[id] = true
		biblio = append(biblio, BiblioEntry{
			ID:       fmt.Sprintf("BIB-%s", strings.ReplaceAll(id, " ", "-")),
			Citation: strings.TrimSpace(matches[2]),
			Line:     i + 1,
		})
	}
	return biblio
}

func detectAllReferences(lines []string, annexes []AnnexEntry, biblio []BiblioEntry) []AnnexRef {
	var refs []AnnexRef
	annexIDs := map[string]bool{}
	for _, a := range annexes {
		annexIDs[a.ID] = true
	}
	biblioIDs := map[string]bool{}
	for _, b := range biblio {
		biblioIDs[b.ID] = true
	}

	for i, line := range lines {
		lineNum := i + 1

		// Detect inline annex references.
		annexMatches := agAnnexInlineRe.FindAllStringSubmatch(line, -1)
		for _, m := range annexMatches {
			id := fmt.Sprintf("ANNEX-%s", strings.ToUpper(m[2]))
			// Skip if this line is the annex heading definition itself.
			if agAnnexHeadingRe.MatchString(line) {
				continue
			}
			refs = append(refs, AnnexRef{
				SourceLine: lineNum,
				TargetID:   id,
				Context:    truncate(line, 60),
			})
		}

		// Detect bibliography references [key].
		biblioMatches := agBiblioRefRe.FindAllStringSubmatch(line, -1)
		for _, m := range biblioMatches {
			key := m[1]
			// Skip definitions (line starts with [key]).
			if strings.HasPrefix(strings.TrimSpace(line), "["+key+"]") && (strings.Contains(line, ":") || strings.Contains(line, ".")) {
				if agBiblioDefRe.MatchString(line) {
					continue
				}
			}
			// Skip markdown links.
			idx := strings.Index(line, m[0])
			if idx >= 0 && idx+len(m[0]) < len(line) && line[idx+len(m[0])] == '(' {
				continue
			}
			// Skip checkboxes.
			if key == " " || key == "x" || key == "X" {
				continue
			}
			id := fmt.Sprintf("BIB-%s", strings.ReplaceAll(key, " ", "-"))
			refs = append(refs, AnnexRef{
				SourceLine: lineNum,
				TargetID:   id,
				Context:    truncate(line, 60),
			})
		}
	}

	return refs
}

func buildDefinedSet(annexes []AnnexEntry, biblio []BiblioEntry) map[string]bool {
	defined := map[string]bool{}
	for _, a := range annexes {
		defined[a.ID] = true
	}
	for _, b := range biblio {
		defined[b.ID] = true
	}
	return defined
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func truncate(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

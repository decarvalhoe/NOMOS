package fidelity

import (
	"fmt"
	"sort"
	"strings"
)


// TOCDrift describes a single difference between source and Nomos TOC.
type TOCDrift struct {
	Kind     DriftKind `json:"kind"`
	Position int       `json:"position"`
	SourceEntry *TOCEntry `json:"source_entry,omitempty"`
	NomosEntry  *TOCEntry `json:"nomos_entry,omitempty"`
	Message  string    `json:"message"`
}

// DriftKind classifies the type of drift.
type DriftKind string

const (
	DriftMissing  DriftKind = "missing"   // in source but not in Nomos
	DriftExtra    DriftKind = "extra"     // in Nomos but not in source
	DriftRenamed  DriftKind = "renamed"   // same position, different title
	DriftReordered DriftKind = "reordered" // same title, different position
	DriftDepth    DriftKind = "depth"     // same title+position, different depth
)

// TOCCompareResult is the output of comparing two TOCs.
type TOCCompareResult struct {
	Aligned     bool       `json:"aligned"`
	SourceCount int        `json:"source_count"`
	NomosCount  int        `json:"nomos_count"`
	MatchCount  int        `json:"match_count"`
	DriftCount  int        `json:"drift_count"`
	Drifts      []TOCDrift `json:"drifts,omitempty"`
	Score       float64    `json:"score"` // 0-1, 1 = perfect match
}

// CompareTOC compares a source TOC against a Nomos-generated TOC.
// Source TOC is the ground truth extracted directly from the document.
// Nomos TOC is what the pipeline produced.
func CompareTOCLegacy(source, nomos []TOCEntry) TOCCompareResult {
	result := TOCCompareResult{
		SourceCount: len(source),
		NomosCount:  len(nomos),
	}

	if len(source) == 0 && len(nomos) == 0 {
		result.Aligned = true
		result.Score = 1.0
		return result
	}

	// Build lookup maps.
	nomosIndex := buildTitleIndex(nomos)
	sourceIndex := buildTitleIndex(source)

	var drifts []TOCDrift
	matched := 0

	// Check each source entry against Nomos.
	for i, sEntry := range source {
		normalizedTitle := normalizeTitle(sEntry.Title)

		if nEntries, ok := nomosIndex[normalizedTitle]; ok {
			// Found by title — check depth and position.
			nEntry := bestMatch(sEntry, nEntries)
			if nEntry.Depth != sEntry.Depth {
				drifts = append(drifts, TOCDrift{
					Kind:        DriftDepth,
					Position:    i,
					SourceEntry: ptr(sEntry),
					NomosEntry:  ptr(nEntry),
					Message:     fmt.Sprintf("Depth mismatch for %q: source=%d, nomos=%d", sEntry.Title, sEntry.Depth, nEntry.Depth),
				})
			} else {
				matched++
			}
		} else {
			// Not found in Nomos — missing.
			drifts = append(drifts, TOCDrift{
				Kind:        DriftMissing,
				Position:    i,
				SourceEntry: ptr(sEntry),
				Message:     fmt.Sprintf("Source entry %q (depth %d) not found in Nomos TOC.", sEntry.Title, sEntry.Depth),
			})
		}
	}

	// Check Nomos entries not in source.
	for i, nEntry := range nomos {
		normalizedTitle := normalizeTitle(nEntry.Title)
		if _, ok := sourceIndex[normalizedTitle]; !ok {
			drifts = append(drifts, TOCDrift{
				Kind:       DriftExtra,
				Position:   i,
				NomosEntry: ptr(nEntry),
				Message:    fmt.Sprintf("Nomos entry %q (depth %d) not found in source TOC.", nEntry.Title, nEntry.Depth),
			})
		}
	}

	// Detect reordering among matched entries.
	reorderDrifts := detectReordering(source, nomos)
	drifts = append(drifts, reorderDrifts...)

	sort.Slice(drifts, func(i, j int) bool {
		return drifts[i].Position < drifts[j].Position
	})

	result.Drifts = drifts
	result.DriftCount = len(drifts)
	result.MatchCount = matched

	total := max(len(source), len(nomos))
	if total > 0 {
		result.Score = float64(matched) / float64(total)
	}
	result.Aligned = len(drifts) == 0

	return result
}

// CompareTOCFromTree compares a source TOC against a StructureTree.
func CompareTOCFromTree(source []TOCEntry, tree StructureTree) TOCCompareResult {
	nomos := flattenTreeToEntries(tree)
	return CompareTOCLegacy(source, nomos)
}

func flattenTreeToEntries(tree StructureTree) []TOCEntry {
	flat := tree.FlatTOC()
	entries := make([]TOCEntry, 0, len(flat))
	for _, node := range flat {
		entries = append(entries, TOCEntry{
			NodeID:    node.ID,
			Title: node.Title,
			Depth: node.Depth,
		})
	}
	return entries
}

func buildTitleIndex(entries []TOCEntry) map[string][]TOCEntry {
	index := map[string][]TOCEntry{}
	for _, e := range entries {
		key := normalizeTitle(e.Title)
		index[key] = append(index[key], e)
	}
	return index
}

func normalizeTitle(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}

func bestMatch(source TOCEntry, candidates []TOCEntry) TOCEntry {
	// Prefer exact depth match.
	for _, c := range candidates {
		if c.Depth == source.Depth {
			return c
		}
	}
	return candidates[0]
}

func detectReordering(source, nomos []TOCEntry) []TOCDrift {
	// Build position map for Nomos entries by normalized title.
	nomosPos := map[string]int{}
	for i, n := range nomos {
		key := normalizeTitle(n.Title)
		if _, exists := nomosPos[key]; !exists {
			nomosPos[key] = i
		}
	}

	var drifts []TOCDrift
	prevNomosIdx := -1
	for i, s := range source {
		key := normalizeTitle(s.Title)
		nIdx, ok := nomosPos[key]
		if !ok {
			continue
		}
		if nIdx < prevNomosIdx {
			drifts = append(drifts, TOCDrift{
				Kind:        DriftReordered,
				Position:    i,
				SourceEntry: ptr(s),
				Message:     fmt.Sprintf("Entry %q appears at different position in Nomos TOC.", s.Title),
			})
		}
		prevNomosIdx = nIdx
	}

	return drifts
}

func ptr(e TOCEntry) *TOCEntry {
	return &e
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const CertifiedTOCFormat = "nomos.certified-toc.v1"

// TOCEntry is a single entry in the certified table of contents.
type TOCEntry struct {
	Number    string `json:"number"`
	Title     string `json:"title"`
	Depth     int    `json:"depth"`
	Line      int    `json:"line"`
	NodeID    string `json:"node_id,omitempty"`
	Hash      string `json:"hash"`
	ChildCount int   `json:"child_count"`
}

// CertifiedTOC is the certified structure proof artifact.
type CertifiedTOC struct {
	Format        string     `json:"format"`
	DocumentRef   string     `json:"document_ref"`
	StructureHash string     `json:"structure_hash"`
	EntryCount    int        `json:"entry_count"`
	MaxDepth      int        `json:"max_depth"`
	Entries       []TOCEntry `json:"entries"`
}

// TOCInput provides the extracted structure for TOC generation.
type TOCInput struct {
	DocumentRef string
	Headings    []TOCHeading
}

// TOCHeading is a heading extracted from the source document.
type TOCHeading struct {
	Title  string
	Level  int
	Line   int
	NodeID string
}

// BuildCertifiedTOC generates a certified TOC from extracted headings.
func BuildCertifiedTOC(input TOCInput) CertifiedTOC {
	entries := make([]TOCEntry, 0, len(input.Headings))

	// Numbering counters per depth.
	counters := make([]int, 7)
	maxDepth := 0

	for _, h := range input.Headings {
		depth := h.Level - 1 // H1=depth 0, H2=depth 1, etc.
		if depth < 0 {
			depth = 0
		}
		if depth > 6 {
			depth = 6
		}

		counters[depth]++
		// Reset deeper counters.
		for d := depth + 1; d < len(counters); d++ {
			counters[d] = 0
		}

		number := buildNumber(counters, depth)
		hash := tocEntryHash(number, h.Title, depth)

		entries = append(entries, TOCEntry{
			Number: number,
			Title:  h.Title,
			Depth:  depth,
			Line:   h.Line,
			NodeID: h.NodeID,
			Hash:   hash,
		})

		if depth > maxDepth {
			maxDepth = depth
		}
	}

	// Compute child counts.
	for i := range entries {
		entries[i].ChildCount = countChildren(entries, i)
	}

	structureHash := computeStructureHash(entries)

	return CertifiedTOC{
		Format:        CertifiedTOCFormat,
		DocumentRef:   input.DocumentRef,
		StructureHash: structureHash,
		EntryCount:    len(entries),
		MaxDepth:      maxDepth,
		Entries:       entries,
	}
}

// WriteCertifiedTOC writes the TOC as indented JSON.
func WriteCertifiedTOC(w io.Writer, toc CertifiedTOC) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(toc)
}

// --- TOC comparison / drift detection ---

// DriftEntry describes one structural difference between two TOCs.
type DriftEntry struct {
	Number    string `json:"number"`
	Code      string `json:"code"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	Blocking  bool   `json:"blocking"`
}

// Drift codes.
const (
	CodeTOCAdded      = "TOC_ENTRY_ADDED"
	CodeTOCRemoved    = "TOC_ENTRY_REMOVED"
	CodeTOCRenamed    = "TOC_ENTRY_RENAMED"
	CodeTOCReordered  = "TOC_ENTRY_REORDERED"
	CodeTOCDepthChanged = "TOC_DEPTH_CHANGED"
	CodeTOCHashChanged  = "TOC_HASH_CHANGED"
)

// DriftReport is the result of comparing two TOCs.
type DriftReport struct {
	SourceRef    string       `json:"source_ref"`
	ExtractedRef string       `json:"extracted_ref"`
	SourceHash   string       `json:"source_hash"`
	ExtractedHash string      `json:"extracted_hash"`
	Match        bool         `json:"match"`
	Drifts       []DriftEntry `json:"drifts,omitempty"`
}

// CompareTOC compares a source TOC against an extracted TOC and reports drift.
func CompareTOC(source, extracted CertifiedTOC) DriftReport {
	report := DriftReport{
		SourceRef:     source.DocumentRef,
		ExtractedRef:  extracted.DocumentRef,
		SourceHash:    source.StructureHash,
		ExtractedHash: extracted.StructureHash,
		Match:         source.StructureHash == extracted.StructureHash,
	}

	if report.Match {
		return report
	}

	// Index source entries by number.
	srcByNum := map[string]*TOCEntry{}
	for i := range source.Entries {
		srcByNum[source.Entries[i].Number] = &source.Entries[i]
	}
	extByNum := map[string]*TOCEntry{}
	for i := range extracted.Entries {
		extByNum[extracted.Entries[i].Number] = &extracted.Entries[i]
	}

	// Detect removed entries.
	for num, src := range srcByNum {
		if _, ok := extByNum[num]; !ok {
			report.Drifts = append(report.Drifts, DriftEntry{
				Number: num, Code: CodeTOCRemoved, Severity: "high",
				Message: fmt.Sprintf("entry %s %q removed", num, src.Title), Blocking: true,
			})
		}
	}

	// Detect added and changed entries.
	for num, ext := range extByNum {
		src, existed := srcByNum[num]
		if !existed {
			report.Drifts = append(report.Drifts, DriftEntry{
				Number: num, Code: CodeTOCAdded, Severity: "medium",
				Message: fmt.Sprintf("entry %s %q added", num, ext.Title),
			})
			continue
		}

		if src.Title != ext.Title {
			report.Drifts = append(report.Drifts, DriftEntry{
				Number: num, Code: CodeTOCRenamed, Severity: "high",
				Message: fmt.Sprintf("entry %s renamed: %q → %q", num, src.Title, ext.Title),
				Blocking: true,
			})
		}
		if src.Depth != ext.Depth {
			report.Drifts = append(report.Drifts, DriftEntry{
				Number: num, Code: CodeTOCDepthChanged, Severity: "high",
				Message: fmt.Sprintf("entry %s depth changed: %d → %d", num, src.Depth, ext.Depth),
				Blocking: true,
			})
		}
	}

	// Detect reordering: same entries but different sequence.
	if len(report.Drifts) == 0 && !report.Match {
		// Hashes differ but no add/remove/rename/depth — must be reorder or content change.
		for num, ext := range extByNum {
			src := srcByNum[num]
			if src != nil && src.Hash != ext.Hash {
				report.Drifts = append(report.Drifts, DriftEntry{
					Number: num, Code: CodeTOCHashChanged, Severity: "medium",
					Message: fmt.Sprintf("entry %s hash changed", num),
				})
			}
		}
	}

	sort.Slice(report.Drifts, func(i, j int) bool {
		return report.Drifts[i].Number < report.Drifts[j].Number
	})

	return report
}

// HasBlockingDrift returns true if any drift is blocking.
func (r DriftReport) HasBlockingDrift() bool {
	for _, d := range r.Drifts {
		if d.Blocking {
			return true
		}
	}
	return false
}

// --- helpers ---

func buildNumber(counters []int, depth int) string {
	parts := make([]string, 0, depth+1)
	for d := 0; d <= depth; d++ {
		parts = append(parts, fmt.Sprintf("%d", counters[d]))
	}
	return strings.Join(parts, ".")
}

func tocEntryHash(number, title string, depth int) string {
	raw := fmt.Sprintf("%s|%s|%d", number, title, depth)
	h := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(h[:16])
}

func computeStructureHash(entries []TOCEntry) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s|%s|%d\n", e.Number, e.Title, e.Depth)
	}
	h := sha256.Sum256([]byte(b.String()))
	return "sha256:" + hex.EncodeToString(h[:])
}

func countChildren(entries []TOCEntry, parentIdx int) int {
	parentDepth := entries[parentIdx].Depth
	count := 0
	for i := parentIdx + 1; i < len(entries); i++ {
		if entries[i].Depth <= parentDepth {
			break
		}
		if entries[i].Depth == parentDepth+1 {
			count++
		}
	}
	return count
}

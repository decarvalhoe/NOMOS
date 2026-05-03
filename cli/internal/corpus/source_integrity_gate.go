package corpus

import (
	"sort"
	"strings"
	"unicode"
)

// Stable finding codes emitted by the source integrity gate. These are part
// of the public contract: downstream consumers (CLI, gates, dashboards) key
// off these strings, so they MUST NOT change without coordination.
const (
	FindingSourceUncoveredRange      = "SOURCE_UNCOVERED_RANGE"
	FindingSourceDuplicateSemantic   = "SOURCE_DUPLICATE_SEMANTIC_SPAN"
	FindingSourceJunkSemantic        = "SOURCE_JUNK_SEMANTIC_ATOM"
	FindingSourceUnsupportedBlocking = "SOURCE_UNSUPPORTED_BLOCKING"
	FindingSourceSegmentInvalidRange = "SOURCE_SEGMENT_INVALID_RANGE"
	FindingSourceSegmentMissingHash  = "SOURCE_SEGMENT_MISSING_HASH"
)

// IntegrityFinding is a single rule violation emitted by the gate. The
// JSON encoding mirrors specs/corpus-integrity-report.cue and is the wire
// format consumed by SFI-09.
type IntegrityFinding struct {
	Code      string `json:"code"`
	SegmentID string `json:"segment_id,omitempty"`
	SourceID  string `json:"source_id,omitempty"`
	StartByte int    `json:"start_byte,omitempty"`
	EndByte   int    `json:"end_byte,omitempty"`
	Message   string `json:"message"`
}

// ByteRange is a half-open [start, end) byte interval inside a source.
type ByteRange struct {
	SourceID  string `json:"source_id"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
}

// IntegrityReport is the full, structured result of CheckSourceIntegrity.
// Status is "pass" iff Findings is empty.
type IntegrityReport struct {
	Status                      string             `json:"status"`
	SourceCount                 int                `json:"source_count"`
	SegmentCount                int                `json:"segment_count"`
	SemanticSegmentCount        int                `json:"semantic_segment_count"`
	UncoveredRanges             []ByteRange        `json:"uncovered_ranges"`
	DuplicateSemanticRanges     []ByteRange        `json:"duplicate_semantic_ranges"`
	JunkSemanticSegments        []string           `json:"junk_semantic_segments"`
	UnsupportedBlockingSegments []string           `json:"unsupported_blocking_segments"`
	Findings                    []IntegrityFinding `json:"findings"`
}

// SourceInput pairs a source identifier with its raw bytes. Content may be
// nil; in that case CheckSourceIntegrity validates the segment-level rules
// for that source's segments but skips byte-level rules (coverage and junk
// detection) which require the original text.
type SourceInput struct {
	SourceID string
	Path     string
	Content  []byte
}

// CheckSourceIntegrity runs the SFI-04 gate over a SourceSegment ledger
// plus the original source bytes. It is stateless and side-effect-free.
//
// The gate enforces, in order:
//  1. per-segment shape (range ordering, canonical-atom hashes);
//  2. duplicate canonical_atom spans across the ledger;
//  3. junk canonical_atom contents (whitespace, punctuation, table separators);
//  4. unsupported_blocking dispositions;
//  5. per-source byte coverage gaps.
//
// Status is "pass" iff Findings is empty.
func CheckSourceIntegrity(sources []SourceInput, segments []SourceSegment) IntegrityReport {
	report := IntegrityReport{
		SegmentCount:                len(segments),
		UncoveredRanges:             []ByteRange{},
		DuplicateSemanticRanges:     []ByteRange{},
		JunkSemanticSegments:        []string{},
		UnsupportedBlockingSegments: []string{},
		Findings:                    []IntegrityFinding{},
	}

	uniqSrc := map[string]struct{}{}
	for _, seg := range segments {
		uniqSrc[seg.SourceID] = struct{}{}
		if seg.Disposition == DispositionCanonicalAtom {
			report.SemanticSegmentCount++
		}
	}
	report.SourceCount = len(uniqSrc)

	sourceByID := make(map[string]SourceInput, len(sources))
	for _, src := range sources {
		sourceByID[src.SourceID] = src
	}

	report.Findings = append(report.Findings, segmentShapeFindings(segments)...)
	report.Findings = append(report.Findings, unsupportedBlockingFindings(segments, &report)...)
	report.Findings = append(report.Findings, duplicateSemanticFindings(segments, &report)...)
	report.Findings = append(report.Findings, junkSemanticFindings(segments, sourceByID, &report)...)
	report.Findings = append(report.Findings, uncoveredRangeFindings(segments, sourceByID, &report)...)

	if len(report.Findings) == 0 {
		report.Status = "pass"
	} else {
		report.Status = "fail"
	}
	return report
}

// segmentShapeFindings emits per-segment findings for invalid range ordering
// and missing hashes on canonical atoms. We do not call SourceSegment.Validate
// directly because we need to distinguish range issues from hash issues.
func segmentShapeFindings(segments []SourceSegment) []IntegrityFinding {
	var out []IntegrityFinding
	for _, seg := range segments {
		if msg, bad := invalidRangeReason(seg); bad {
			out = append(out, IntegrityFinding{
				Code:      FindingSourceSegmentInvalidRange,
				SegmentID: seg.SegmentID,
				SourceID:  seg.SourceID,
				StartByte: seg.StartByte,
				EndByte:   seg.EndByte,
				Message:   msg,
			})
		}
		if seg.Disposition == DispositionCanonicalAtom {
			if strings.TrimSpace(seg.RawTextHash) == "" || strings.TrimSpace(seg.NormalizedTextHash) == "" {
				out = append(out, IntegrityFinding{
					Code:      FindingSourceSegmentMissingHash,
					SegmentID: seg.SegmentID,
					SourceID:  seg.SourceID,
					StartByte: seg.StartByte,
					EndByte:   seg.EndByte,
					Message:   "canonical_atom segment is missing raw_text_hash or normalized_text_hash",
				})
			}
		}
	}
	return out
}

// invalidRangeReason returns a human-readable description and true when the
// segment's byte/line/column ordering is invalid.
func invalidRangeReason(s SourceSegment) (string, bool) {
	switch {
	case s.StartByte > s.EndByte:
		return "start_byte greater than end_byte", true
	case s.StartLine > s.EndLine:
		return "start_line greater than end_line", true
	case s.StartLine == s.EndLine && s.StartColumn > s.EndColumn:
		return "start_column greater than end_column on same line", true
	default:
		return "", false
	}
}

func unsupportedBlockingFindings(segments []SourceSegment, report *IntegrityReport) []IntegrityFinding {
	var out []IntegrityFinding
	for _, seg := range segments {
		if seg.Disposition != DispositionUnsupportedBlocking {
			continue
		}
		report.UnsupportedBlockingSegments = append(report.UnsupportedBlockingSegments, seg.SegmentID)
		msg := "segment classified as unsupported_blocking"
		if reason := strings.TrimSpace(seg.UnsupportedReason); reason != "" {
			msg = msg + ": " + reason
		}
		out = append(out, IntegrityFinding{
			Code:      FindingSourceUnsupportedBlocking,
			SegmentID: seg.SegmentID,
			SourceID:  seg.SourceID,
			StartByte: seg.StartByte,
			EndByte:   seg.EndByte,
			Message:   msg,
		})
	}
	return out
}

func duplicateSemanticFindings(segments []SourceSegment, report *IntegrityReport) []IntegrityFinding {
	type spanKey struct {
		src   string
		start int
		end   int
	}
	groups := map[spanKey][]SourceSegment{}
	order := []spanKey{}
	for _, seg := range segments {
		if seg.Disposition != DispositionCanonicalAtom {
			continue
		}
		k := spanKey{seg.SourceID, seg.StartByte, seg.EndByte}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], seg)
	}
	var out []IntegrityFinding
	for _, k := range order {
		group := groups[k]
		if len(group) < 2 {
			continue
		}
		report.DuplicateSemanticRanges = append(report.DuplicateSemanticRanges, ByteRange{
			SourceID:  k.src,
			StartByte: k.start,
			EndByte:   k.end,
		})
		for i := 1; i < len(group); i++ {
			out = append(out, IntegrityFinding{
				Code:      FindingSourceDuplicateSemantic,
				SegmentID: group[i].SegmentID,
				SourceID:  group[i].SourceID,
				StartByte: group[i].StartByte,
				EndByte:   group[i].EndByte,
				Message:   "duplicate canonical_atom span (already claimed by an earlier segment)",
			})
		}
	}
	return out
}

func junkSemanticFindings(segments []SourceSegment, sourceByID map[string]SourceInput, report *IntegrityReport) []IntegrityFinding {
	var out []IntegrityFinding
	for _, seg := range segments {
		if seg.Disposition != DispositionCanonicalAtom {
			continue
		}
		src, ok := sourceByID[seg.SourceID]
		if !ok || src.Content == nil {
			continue
		}
		if !rangeFitsContent(seg, src.Content) {
			continue
		}
		raw := src.Content[seg.StartByte:seg.EndByte]
		if !isJunkSemantic(raw) {
			continue
		}
		report.JunkSemanticSegments = append(report.JunkSemanticSegments, seg.SegmentID)
		out = append(out, IntegrityFinding{
			Code:      FindingSourceJunkSemantic,
			SegmentID: seg.SegmentID,
			SourceID:  seg.SourceID,
			StartByte: seg.StartByte,
			EndByte:   seg.EndByte,
			Message:   "canonical_atom contains only whitespace, punctuation, or layout markers",
		})
	}
	return out
}

func rangeFitsContent(s SourceSegment, content []byte) bool {
	return s.StartByte >= 0 && s.EndByte <= len(content) && s.StartByte <= s.EndByte
}

// junkRuneSet enumerates the punctuation and layout characters that, on
// their own, mark a canonical atom as junk. Backtick is included so that
// fenced fragments accidentally tagged as canonical atoms are caught.
var junkRuneSet = func() map[rune]struct{} {
	const junkRunes = "-_*~|.,;:!?()[]{}<>+=`"
	m := make(map[rune]struct{}, len(junkRunes))
	for _, r := range junkRunes {
		m[r] = struct{}{}
	}
	return m
}()

func isJunkSemantic(raw []byte) bool {
	str := string(raw)
	normalized := normalizeText(str)
	if strings.TrimSpace(normalized) == "" {
		return true
	}
	if onlyJunkRunes(normalized) {
		return true
	}
	if isTableSeparatorBlock(str) {
		return true
	}
	return false
}

// onlyJunkRunes returns true when every non-whitespace rune in s is a
// member of junkRuneSet.
func onlyJunkRunes(s string) bool {
	saw := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		if _, ok := junkRuneSet[r]; !ok {
			return false
		}
		saw = true
	}
	return saw
}

// isTableSeparatorBlock returns true when every non-empty line of s matches
// the Markdown table-separator regex (lines of :?-{3,}:? optionally split
// by pipes). Reuses mdScanTableSeparatorRe from markdown_scanner.go.
func isTableSeparatorBlock(s string) bool {
	saw := false
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !mdScanTableSeparatorRe.MatchString(line) {
			return false
		}
		saw = true
	}
	return saw
}

func uncoveredRangeFindings(segments []SourceSegment, sourceByID map[string]SourceInput, report *IntegrityReport) []IntegrityFinding {
	bySrc := map[string][]SourceSegment{}
	for _, seg := range segments {
		bySrc[seg.SourceID] = append(bySrc[seg.SourceID], seg)
	}
	srcIDs := make([]string, 0, len(sourceByID))
	for id := range sourceByID {
		srcIDs = append(srcIDs, id)
	}
	sort.Strings(srcIDs)

	var out []IntegrityFinding
	for _, id := range srcIDs {
		src := sourceByID[id]
		if src.Content == nil {
			continue
		}
		for _, gap := range computeGaps(src.Content, bySrc[id]) {
			report.UncoveredRanges = append(report.UncoveredRanges, ByteRange{
				SourceID:  id,
				StartByte: gap.start,
				EndByte:   gap.end,
			})
			msg := "uncovered whitespace-only source range"
			if hasNonWhitespaceBytes(src.Content[gap.start:gap.end]) {
				msg = "uncovered source range with non-whitespace bytes"
			}
			out = append(out, IntegrityFinding{
				Code:      FindingSourceUncoveredRange,
				SourceID:  id,
				StartByte: gap.start,
				EndByte:   gap.end,
				Message:   msg,
			})
		}
	}
	return out
}

type byteSpan struct{ start, end int }

// computeGaps returns the contiguous [start, end) byte ranges of content
// that are not covered by any segment. Segments with degenerate or
// out-of-bounds ranges are ignored; overlapping segments are merged.
func computeGaps(content []byte, segs []SourceSegment) []byteSpan {
	if len(content) == 0 {
		return nil
	}
	intervals := make([]byteSpan, 0, len(segs))
	for _, s := range segs {
		if !rangeFitsContent(s, content) {
			continue
		}
		if s.StartByte == s.EndByte {
			continue
		}
		intervals = append(intervals, byteSpan{s.StartByte, s.EndByte})
	}
	sort.Slice(intervals, func(i, j int) bool {
		if intervals[i].start != intervals[j].start {
			return intervals[i].start < intervals[j].start
		}
		return intervals[i].end < intervals[j].end
	})
	merged := intervals[:0]
	for _, v := range intervals {
		if len(merged) == 0 || v.start > merged[len(merged)-1].end {
			merged = append(merged, v)
			continue
		}
		if v.end > merged[len(merged)-1].end {
			merged[len(merged)-1].end = v.end
		}
	}
	var gaps []byteSpan
	cursor := 0
	for _, v := range merged {
		if v.start > cursor {
			gaps = append(gaps, byteSpan{cursor, v.start})
		}
		if v.end > cursor {
			cursor = v.end
		}
	}
	if cursor < len(content) {
		gaps = append(gaps, byteSpan{cursor, len(content)})
	}
	return gaps
}

func hasNonWhitespaceBytes(b []byte) bool {
	for _, r := range string(b) {
		if !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

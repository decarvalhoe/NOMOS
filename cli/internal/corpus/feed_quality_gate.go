package corpus

import (
	"fmt"
	"strings"
)

// Stable finding codes emitted by the SFI-07 feed quality gate. Downstream
// consumers (CLI, dashboards, the SFI-08 strict release gate) key off these
// strings; they MUST NOT change without coordination.
//
// The RAG_CHUNK_* codes intentionally reuse the SFI-06 constants from
// rag_metadata.go (RAGChunkNoUnit / RAGChunkNoSegment / RAGChunkNonSemanticSource)
// so that the build-time rejection codes and the after-the-fact gate findings
// share one source of truth. They already use the exact literal strings
// required by SFI-07, so no aliasing is needed.
const (
	FindingFeedUnitNoSegment = "FEED_UNIT_NO_SEGMENT"
	FindingFeedUnitNoSource  = "FEED_UNIT_NO_SOURCE"
	FindingFeedUnitNoSpan    = "FEED_UNIT_NO_SPAN"
	FindingFeedEmptyText     = "FEED_EMPTY_TEXT"
	FindingFeedJunkText      = "FEED_JUNK_TEXT"
	FindingFeedDuplicateSpan = "FEED_DUPLICATE_SPAN"
)

// FeedQualityInput is the read-only view of the artifacts to be validated.
// All slices may be nil; an empty corpus yields a passing report.
type FeedQualityInput struct {
	FeedUnits []FeedUnit
	Chunks    []ChunkMetadata
	Segments  []SourceSegment
}

// FeedQualityFinding is a single rule violation. The JSON shape is the wire
// format consumed by the SFI-08 release gate and the SFI-09 CUE schema.
type FeedQualityFinding struct {
	Code      string `json:"code"`
	UnitID    string `json:"unit_id,omitempty"`
	ChunkID   string `json:"chunk_id,omitempty"`
	SegmentID string `json:"segment_id,omitempty"`
	SourceID  string `json:"source_id,omitempty"`
	Message   string `json:"message"`
}

// FeedQualityReport summarises a CheckFeedQuality run. Status is "pass" iff
// Findings is empty.
type FeedQualityReport struct {
	Status                 string               `json:"status"`
	FeedUnitCount          int                  `json:"feed_unit_count"`
	SourceDerivedUnitCount int                  `json:"source_derived_unit_count"`
	ChunkCount             int                  `json:"chunk_count"`
	DuplicateSpanCount     int                  `json:"duplicate_span_count"`
	Findings               []FeedQualityFinding `json:"findings"`
}

// CheckFeedQuality validates final feed and RAG artifacts against the
// SourceSegment ledger they claim to be derived from. It is stateless,
// side-effect-free, and complementary to:
//
//   - the SFI-04 source integrity gate (which validates the ledger itself
//     and is a precondition, not a synonym);
//   - the SFI-06 build-time RAG rejection rules in BuildRAGMetadata
//     (which prevent junk chunks at construction time).
//
// SFI-07 is the consumer-facing check on artifacts that have already been
// produced. Matrix-derived feed units (those without source-segment evidence)
// are skipped — only source-derived units are scrutinised.
func CheckFeedQuality(input FeedQualityInput) FeedQualityReport {
	report := FeedQualityReport{
		FeedUnitCount: len(input.FeedUnits),
		ChunkCount:    len(input.Chunks),
		Findings:      []FeedQualityFinding{},
	}

	sourceDerived := make([]FeedUnit, 0, len(input.FeedUnits))
	for _, u := range input.FeedUnits {
		if isSourceDerivedFeedUnit(u) {
			sourceDerived = append(sourceDerived, u)
		}
	}
	report.SourceDerivedUnitCount = len(sourceDerived)

	for _, u := range sourceDerived {
		report.Findings = append(report.Findings, checkFeedUnitShape(u)...)
		report.Findings = append(report.Findings, checkFeedUnitText(u)...)
	}

	report.Findings = append(report.Findings, findDuplicateFeedSpans(sourceDerived, &report)...)

	segByID := make(map[string]SourceSegment, len(input.Segments))
	for _, s := range input.Segments {
		segByID[s.SegmentID] = s
	}
	for _, c := range input.Chunks {
		report.Findings = append(report.Findings, checkOneChunk(c, segByID)...)
	}

	if len(report.Findings) == 0 {
		report.Status = "pass"
	} else {
		report.Status = "fail"
	}
	return report
}

// isSourceDerivedFeedUnit returns true when a FeedUnit carries any of the
// SFI-05 source-segment markers introduced in #343. The discriminator is the
// union of those markers so that *any* missing single field still routes the
// unit through the gate (and triggers the matching FEED_UNIT_NO_* finding).
//
// Matrix-derived units (built from canonical-matrix YAML) carry only
// SourceIDs and never set SourceSegmentID/SourcePath/spans/NormalizedTextHash,
// so they fall on the not-source-derived side and are correctly skipped from
// these checks.
func isSourceDerivedFeedUnit(u FeedUnit) bool {
	return u.SourceSegmentID != "" ||
		u.SourcePath != "" ||
		u.NormalizedTextHash != "" ||
		u.StartByte != 0 ||
		u.EndByte != 0 ||
		len(u.HeadingPath) > 0
}

func checkFeedUnitShape(u FeedUnit) []FeedQualityFinding {
	var out []FeedQualityFinding
	if strings.TrimSpace(u.SourceSegmentID) == "" {
		out = append(out, FeedQualityFinding{
			Code:     FindingFeedUnitNoSegment,
			UnitID:   u.UnitID,
			SourceID: u.SourceID,
			Message:  "source-derived feed unit has empty source_segment_id",
		})
	}
	if strings.TrimSpace(u.SourceSegmentID) != "" && strings.TrimSpace(u.SourceID) == "" {
		out = append(out, FeedQualityFinding{
			Code:      FindingFeedUnitNoSource,
			UnitID:    u.UnitID,
			SegmentID: u.SourceSegmentID,
			Message:   "feed unit has source_segment_id but source_id is empty",
		})
	}
	if strings.TrimSpace(u.SourceSegmentID) != "" && u.StartByte == 0 && u.EndByte == 0 {
		out = append(out, FeedQualityFinding{
			Code:      FindingFeedUnitNoSpan,
			UnitID:    u.UnitID,
			SegmentID: u.SourceSegmentID,
			SourceID:  u.SourceID,
			Message:   "feed unit has source_segment_id but byte span is missing (start_byte=end_byte=0)",
		})
	}
	return out
}

func checkFeedUnitText(u FeedUnit) []FeedQualityFinding {
	text := u.BusinessRule
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []FeedQualityFinding{{
			Code:      FindingFeedEmptyText,
			UnitID:    u.UnitID,
			SegmentID: u.SourceSegmentID,
			SourceID:  u.SourceID,
			Message:   "feed unit business_rule is empty after trim",
		}}
	}
	if isJunkSemantic([]byte(text)) {
		return []FeedQualityFinding{{
			Code:      FindingFeedJunkText,
			UnitID:    u.UnitID,
			SegmentID: u.SourceSegmentID,
			SourceID:  u.SourceID,
			Message:   "feed unit business_rule is punctuation/layout-only or matches a markdown table separator",
		}}
	}
	return nil
}

// findDuplicateFeedSpans detects two or more source-derived feed units that
// share the same (SourceID, StartByte, EndByte). Implemented on the feed
// side independently of the SFI-04 ledger-side check (#342) so that even a
// passing ledger cannot smuggle a duplicated artifact past this gate.
func findDuplicateFeedSpans(units []FeedUnit, report *FeedQualityReport) []FeedQualityFinding {
	type spanKey struct {
		src   string
		start int
		end   int
	}
	groups := map[spanKey][]FeedUnit{}
	order := []spanKey{}
	for _, u := range units {
		if strings.TrimSpace(u.SourceSegmentID) == "" {
			continue
		}
		if u.StartByte == 0 && u.EndByte == 0 {
			continue
		}
		k := spanKey{u.SourceID, u.StartByte, u.EndByte}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], u)
	}
	var out []FeedQualityFinding
	for _, k := range order {
		group := groups[k]
		if len(group) < 2 {
			continue
		}
		report.DuplicateSpanCount++
		for i := 1; i < len(group); i++ {
			out = append(out, FeedQualityFinding{
				Code:      FindingFeedDuplicateSpan,
				UnitID:    group[i].UnitID,
				SegmentID: group[i].SourceSegmentID,
				SourceID:  group[i].SourceID,
				Message: fmt.Sprintf(
					"duplicate (source_id=%s, span=[%d,%d)) already claimed by feed unit %q",
					k.src, k.start, k.end, group[0].UnitID,
				),
			})
		}
	}
	return out
}

func checkOneChunk(c ChunkMetadata, segByID map[string]SourceSegment) []FeedQualityFinding {
	var out []FeedQualityFinding

	if strings.TrimSpace(c.CanonicalUnitID) == "" && len(c.UnitIDs) == 0 {
		out = append(out, FeedQualityFinding{
			Code:      RAGChunkNoUnit,
			ChunkID:   c.ChunkID,
			SegmentID: c.SourceSegmentID,
			SourceID:  c.SourceID,
			Message:   "chunk has no canonical_unit_id and no feed-unit linkage (unit_ids is empty)",
		})
	}

	segID := strings.TrimSpace(c.SourceSegmentID)
	if segID == "" {
		out = append(out, FeedQualityFinding{
			Code:     RAGChunkNoSegment,
			ChunkID:  c.ChunkID,
			SourceID: c.SourceID,
			Message:  "chunk has no source_segment_id",
		})
		return out
	}

	seg, ok := segByID[segID]
	if !ok {
		out = append(out, FeedQualityFinding{
			Code:      RAGChunkNoSegment,
			ChunkID:   c.ChunkID,
			SegmentID: segID,
			SourceID:  c.SourceID,
			Message:   "chunk source_segment_id does not resolve to any segment in the supplied ledger",
		})
		return out
	}

	if seg.Disposition != DispositionCanonicalAtom || isNonSemanticSegmentKindForRAG(seg.Kind) {
		out = append(out, FeedQualityFinding{
			Code:      RAGChunkNonSemanticSource,
			ChunkID:   c.ChunkID,
			SegmentID: seg.SegmentID,
			SourceID:  c.SourceID,
			Message: fmt.Sprintf(
				"chunk source segment is non-semantic (kind=%q disposition=%q)",
				seg.Kind, string(seg.Disposition),
			),
		})
	}
	return out
}

// isNonSemanticSegmentKindForRAG matches the explicit kind list in the
// SFI-07 dispatch: blank, decorative_separator, table_separator, metadata,
// and the literal "structure_only" (kept for parity with the dispatch even
// though current scanners use it only as a Disposition).
func isNonSemanticSegmentKindForRAG(kind string) bool {
	switch kind {
	case KindBlank,
		KindDecorativeSeparator,
		KindTableSeparator,
		KindMetadata,
		"structure_only":
		return true
	}
	return false
}

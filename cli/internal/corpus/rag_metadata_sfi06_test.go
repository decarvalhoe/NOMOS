package corpus

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// SFI-06 (#344) — source-backed RAG metadata with build-time rejection.

var sfi06TestNow = time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)

func sfi06Config() EnrichConfig {
	return EnrichConfig{Now: sfi06TestNow}
}

func sfi06ParagraphSegment() SourceSegment {
	return SourceSegment{
		SegmentID:          "seg-paragraph-001",
		SourceID:           "RBOK-RULE",
		SourcePath:         "docs/rule.md",
		Kind:               KindParagraph,
		Disposition:        DispositionCanonicalAtom,
		StartByte:          10,
		EndByte:            64,
		StartLine:          2,
		EndLine:            2,
		RawTextHash:        "rawhash",
		NormalizedTextHash: "normhash",
		CanonicalUnitID:    "RBOK-MD-RBOK-RULE-RULE-A",
	}
}

func sfi06ValidInput(seg SourceSegment) RAGBuildInput {
	return RAGBuildInput{
		Unit: FeedUnit{
			UnitID:             "RBOK-MD-RBOK-RULE-RULE-A",
			Name:               "Rule A",
			Domain:             "rbok",
			UnitType:           "rule",
			Status:             "partial",
			BusinessRule:       "Body of rule A explains the obligation.",
			SourceIDs:          []string{seg.SourceID},
			SourceSegmentID:    seg.SegmentID,
			SourceID:           seg.SourceID,
			SourcePath:         seg.SourcePath,
			StartByte:          seg.StartByte,
			EndByte:            seg.EndByte,
			StartLine:          seg.StartLine,
			EndLine:            seg.EndLine,
			NormalizedTextHash: seg.NormalizedTextHash,
			HeadingPath:        []string{"Rule A"},
		},
		Content:    "Body of rule A explains the obligation.",
		SourceHash: "sha256:abc123",
		Domain:     "rbok",
		Priority:   "primary",
		Status:     "active",
		Confidence: "medium",
		Tags:       []string{"rule"},
	}
}

func TestBuildRAGMetadata_ValidCanonicalAtom(t *testing.T) {
	seg := sfi06ParagraphSegment()
	in := sfi06ValidInput(seg)
	segs := map[string]SourceSegment{seg.SegmentID: seg}

	chunks, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	c := chunks[0]
	if c.ChunkID != "chunk:RBOK-RULE:10-64" {
		t.Fatalf("unexpected chunk_id %q", c.ChunkID)
	}
	if c.SourceSegmentID != seg.SegmentID {
		t.Fatalf("expected source_segment_id %q, got %q", seg.SegmentID, c.SourceSegmentID)
	}
	if c.CanonicalUnitID != seg.CanonicalUnitID {
		t.Fatalf("expected canonical_unit_id %q, got %q", seg.CanonicalUnitID, c.CanonicalUnitID)
	}
	if c.SegmentKind != KindParagraph {
		t.Fatalf("expected segment_kind paragraph, got %q", c.SegmentKind)
	}
	if c.NormalizedTextHash != "normhash" {
		t.Fatalf("expected normalized_text_hash, got %q", c.NormalizedTextHash)
	}
	if c.StartByte != 10 || c.EndByte != 64 || c.StartLine != 2 || c.EndLine != 2 {
		t.Fatalf("unexpected span: bytes [%d,%d) lines [%d,%d]",
			c.StartByte, c.EndByte, c.StartLine, c.EndLine)
	}
	if c.Locator != "docs/rule.md:L2-L2" {
		t.Fatalf("unexpected locator %q", c.Locator)
	}
	if c.IngestionVersion != RAGIngestionVersion {
		t.Fatalf("expected ingestion_version %q, got %q", RAGIngestionVersion, c.IngestionVersion)
	}
	if c.SourceHash != "sha256:abc123" {
		t.Fatalf("expected source_hash propagated, got %q", c.SourceHash)
	}
	if c.Domain != "rbok" || c.Priority != "primary" || c.Status != "active" || c.Confidence != "medium" {
		t.Fatalf("unexpected classification fields: %+v", c)
	}
	if c.CharCount <= 0 || c.TokenCount <= 0 {
		t.Fatalf("expected positive char/token counts, got %d/%d", c.CharCount, c.TokenCount)
	}
	if c.IngestedAt != "2026-05-01T10:00:00Z" {
		t.Fatalf("unexpected ingested_at %q", c.IngestedAt)
	}
	hasHeading := false
	for _, tag := range c.SemanticTags {
		if tag == "Rule A" {
			hasHeading = true
		}
	}
	if !hasHeading {
		t.Fatalf("expected heading_path entry in semantic_tags, got %v", c.SemanticTags)
	}
}

func TestBuildRAGMetadata_DecorativeSeparatorRejected(t *testing.T) {
	seg := sfi06ParagraphSegment()
	seg.SegmentID = "seg-decor-001"
	seg.Kind = KindDecorativeSeparator
	seg.Disposition = DispositionCoverageOnly
	in := sfi06ValidInput(seg)
	segs := map[string]SourceSegment{seg.SegmentID: seg}

	_, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err == nil {
		t.Fatal("expected error for decorative_separator segment")
	}
	assertRejectionCode(t, err, RAGChunkNonSemanticSource)
}

func TestBuildRAGMetadata_TableSeparatorRejected(t *testing.T) {
	seg := sfi06ParagraphSegment()
	seg.SegmentID = "seg-tablesep-001"
	seg.Kind = KindTableSeparator
	seg.Disposition = DispositionStructureOnly
	in := sfi06ValidInput(seg)
	segs := map[string]SourceSegment{seg.SegmentID: seg}

	_, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err == nil {
		t.Fatal("expected error for table_separator segment")
	}
	assertRejectionCode(t, err, RAGChunkNonSemanticSource)
}

func TestBuildRAGMetadata_BlankRejected(t *testing.T) {
	seg := sfi06ParagraphSegment()
	seg.SegmentID = "seg-blank-001"
	seg.Kind = KindBlank
	seg.Disposition = DispositionCoverageOnly
	in := sfi06ValidInput(seg)
	segs := map[string]SourceSegment{seg.SegmentID: seg}

	_, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err == nil {
		t.Fatal("expected error for blank segment")
	}
	assertRejectionCode(t, err, RAGChunkNonSemanticSource)
}

func TestBuildRAGMetadata_MetadataKindRejected(t *testing.T) {
	seg := sfi06ParagraphSegment()
	seg.SegmentID = "seg-meta-001"
	seg.Kind = KindMetadata
	seg.Disposition = DispositionStructureOnly
	in := sfi06ValidInput(seg)
	segs := map[string]SourceSegment{seg.SegmentID: seg}

	_, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err == nil {
		t.Fatal("expected error for metadata segment")
	}
	assertRejectionCode(t, err, RAGChunkNonSemanticSource)
}

func TestBuildRAGMetadata_StructureOnlyDispositionRejected(t *testing.T) {
	seg := sfi06ParagraphSegment()
	seg.SegmentID = "seg-structure-001"
	seg.Disposition = DispositionStructureOnly
	in := sfi06ValidInput(seg)
	segs := map[string]SourceSegment{seg.SegmentID: seg}

	_, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err == nil {
		t.Fatal("expected error for structure_only disposition")
	}
	assertRejectionCode(t, err, RAGChunkNonSemanticSource)
}

func TestBuildRAGMetadata_UnsupportedBlockingRejected(t *testing.T) {
	seg := sfi06ParagraphSegment()
	seg.SegmentID = "seg-unsupported-001"
	seg.Disposition = DispositionUnsupportedBlocking
	seg.UnsupportedReason = "html block not yet supported"
	in := sfi06ValidInput(seg)
	segs := map[string]SourceSegment{seg.SegmentID: seg}

	_, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err == nil {
		t.Fatal("expected error for unsupported_blocking disposition")
	}
	assertRejectionCode(t, err, RAGChunkUnsupportedBlocking)
}

func TestBuildRAGMetadata_MissingSegmentIDRejected(t *testing.T) {
	seg := sfi06ParagraphSegment()
	in := sfi06ValidInput(seg)
	in.Unit.SourceSegmentID = ""
	segs := map[string]SourceSegment{seg.SegmentID: seg}

	_, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err == nil {
		t.Fatal("expected error for unit without source_segment_id")
	}
	assertRejectionCode(t, err, RAGChunkNoSegment)
}

func TestBuildRAGMetadata_SegmentNotInLookupRejected(t *testing.T) {
	seg := sfi06ParagraphSegment()
	in := sfi06ValidInput(seg)
	segs := map[string]SourceSegment{}

	_, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err == nil {
		t.Fatal("expected error when segment lookup misses")
	}
	assertRejectionCode(t, err, RAGChunkNoSegment)
}

func TestBuildRAGMetadata_NoUnitIDRejected(t *testing.T) {
	seg := sfi06ParagraphSegment()
	seg.CanonicalUnitID = ""
	seg.SegmentID = ""
	// SourceSegmentID on the unit must reference the (empty) segment id; we
	// instead key the lookup map on a sentinel id and have the unit point
	// at it, then synthesize a segment whose own id and canonical unit id
	// are both empty — impossible via Validate(), but reachable as a guard.
	segKey := "seg-empty-ids"
	in := sfi06ValidInput(sfi06ParagraphSegment())
	in.Unit.SourceSegmentID = segKey
	segs := map[string]SourceSegment{segKey: seg}

	_, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err == nil {
		t.Fatal("expected error when neither canonical_unit_id nor segment_id is set")
	}
	assertRejectionCode(t, err, RAGChunkNoUnit)
}

func TestBuildRAGMetadata_EmptyContentRejected(t *testing.T) {
	seg := sfi06ParagraphSegment()
	in := sfi06ValidInput(seg)
	in.Content = "   \n  "
	segs := map[string]SourceSegment{seg.SegmentID: seg}

	_, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err == nil {
		t.Fatal("expected error when chunk content is empty after trim")
	}
	assertRejectionCode(t, err, RAGChunkEmptyText)
}

func TestBuildRAGMetadata_DegenerateSpanRejected(t *testing.T) {
	seg := sfi06ParagraphSegment()
	seg.StartByte = 50
	seg.EndByte = 50
	in := sfi06ValidInput(seg)
	segs := map[string]SourceSegment{seg.SegmentID: seg}

	_, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err == nil {
		t.Fatal("expected error for degenerate byte span")
	}
	assertRejectionCode(t, err, RAGChunkMissingSpan)
}

func TestBuildRAGMetadata_FallbackUnitIDFromSegmentID(t *testing.T) {
	seg := sfi06ParagraphSegment()
	seg.CanonicalUnitID = ""
	in := sfi06ValidInput(seg)
	segs := map[string]SourceSegment{seg.SegmentID: seg}

	chunks, err := BuildRAGMetadata([]RAGBuildInput{in}, segs, sfi06Config())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunks[0].CanonicalUnitID != seg.SegmentID {
		t.Fatalf("expected canonical_unit_id to fall back to segment_id, got %q",
			chunks[0].CanonicalUnitID)
	}
}

func TestBuildRAGMetadata_BatchStopsOnFirstRejection(t *testing.T) {
	good := sfi06ParagraphSegment()
	bad := sfi06ParagraphSegment()
	bad.SegmentID = "seg-decor-batch"
	bad.Kind = KindDecorativeSeparator
	bad.Disposition = DispositionCoverageOnly

	inGood := sfi06ValidInput(good)
	inBad := sfi06ValidInput(bad)
	segs := map[string]SourceSegment{
		good.SegmentID: good,
		bad.SegmentID:  bad,
	}

	_, err := BuildRAGMetadata([]RAGBuildInput{inGood, inBad}, segs, sfi06Config())
	if err == nil {
		t.Fatal("expected error from batch with one bad input")
	}
	if !strings.Contains(err.Error(), "rag chunk[1]") {
		t.Fatalf("expected error to reference rag chunk[1], got %v", err)
	}
	assertRejectionCode(t, err, RAGChunkNonSemanticSource)
}

func assertRejectionCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected RAGRejection with code %s, got nil", code)
	}
	var rej *RAGRejection
	if errors.As(err, &rej) {
		if rej.Code != code {
			t.Fatalf("expected rejection code %s, got %s (%v)", code, rej.Code, err)
		}
		return
	}
	if !strings.Contains(err.Error(), code) {
		t.Fatalf("expected error to contain %s, got %v", code, err)
	}
}

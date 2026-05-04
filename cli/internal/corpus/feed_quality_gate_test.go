package corpus

import (
	"strings"
	"testing"
	"time"
)

// SFI-07 (#345) — feed quality gate tests.

const fqgSourceID = "SRC-FQG"
const fqgSourcePath = "docs/fqg.md"

func fqgConfig() EnrichConfig {
	return EnrichConfig{Now: time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)}
}

// fqgValidUnit returns a fully populated source-derived feed unit. Tests
// mutate one field per case to isolate the rule under test.
func fqgValidUnit() FeedUnit {
	return FeedUnit{
		UnitID:             "RBOK-MD-FQG-RULE-A",
		Name:               "Rule A",
		Domain:             "rbok",
		UnitType:           "rule",
		Criticality:        "medium",
		Status:             "partial",
		BusinessRule:       "The system must validate all source bytes before producing a feed unit.",
		SourceIDs:          []string{fqgSourceID},
		SourceSegmentID:    "seg:SRC-FQG:10-80:paragraph",
		SourceID:           fqgSourceID,
		SourcePath:         fqgSourcePath,
		StartByte:          10,
		EndByte:            80,
		StartLine:          3,
		EndLine:            3,
		NormalizedTextHash: "normhash-rule-a",
		HeadingPath:        []string{"Rule A"},
	}
}

// fqgValidSegment is the canonical_atom segment that the fqgValidUnit
// claims provenance from. Negative chunk tests mutate this to break a rule.
func fqgValidSegment() SourceSegment {
	return SourceSegment{
		SegmentID:          "seg:SRC-FQG:10-80:paragraph",
		SourceID:           fqgSourceID,
		SourcePath:         fqgSourcePath,
		Kind:               KindParagraph,
		Disposition:        DispositionCanonicalAtom,
		StartByte:          10,
		EndByte:            80,
		StartLine:          3,
		EndLine:            3,
		RawTextHash:        "rawhash-rule-a",
		NormalizedTextHash: "normhash-rule-a",
		CanonicalUnitID:    "RBOK-MD-FQG-RULE-A",
	}
}

// fqgValidChunk pairs with fqgValidUnit/fqgValidSegment.
func fqgValidChunk() ChunkMetadata {
	return ChunkMetadata{
		ChunkID:            "chunk:SRC-FQG:10-80",
		SourceID:           fqgSourceID,
		SourcePath:         fqgSourcePath,
		SourceHash:         "sha256:abcdef",
		Domain:             "rbok",
		UnitIDs:            []string{"RBOK-MD-FQG-RULE-A"},
		Locator:            fqgSourcePath + ":L3-L3",
		Priority:           "primary",
		Status:             "active",
		Confidence:         "medium",
		SemanticTags:       []string{"rule"},
		TokenCount:         18,
		CharCount:          70,
		IngestedAt:         "2026-05-04T12:00:00Z",
		IngestionVersion:   "test",
		SourceSegmentID:    "seg:SRC-FQG:10-80:paragraph",
		CanonicalUnitID:    "RBOK-MD-FQG-RULE-A",
		SegmentKind:        KindParagraph,
		NormalizedTextHash: "normhash-rule-a",
		StartByte:          10,
		EndByte:            80,
		StartLine:          3,
		EndLine:            3,
	}
}

// containsCode returns true when at least one finding has the given code.
func containsCode(findings []FeedQualityFinding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// ----------------------------------------------------------------------------
// Positive end-to-end fixture: clean markdown corpus through the real
// pipeline (ScanMarkdown → markdownFeedUnitsFromBytes → BuildRAGMetadata).
// ----------------------------------------------------------------------------

func TestCheckFeedQuality_PositiveEndToEnd(t *testing.T) {
	t.Parallel()

	content := []byte("" +
		"# Title\n" +
		"\n" +
		"First clean paragraph that is meaningful enough to keep.\n" +
		"\n" +
		"## Subtitle\n" +
		"\n" +
		"Second clean paragraph in another section.\n",
	)
	source := ManifestSource{
		ID:     fqgSourceID,
		Path:   fqgSourcePath,
		Type:   "markdown",
		Domain: "rbok",
		Hash:   "sha256:fixture",
		Status: "active",
	}

	segments, err := ScanMarkdown(source.ID, source.Path, content)
	if err != nil {
		t.Fatalf("ScanMarkdown: %v", err)
	}

	extracted, err := markdownFeedUnitsFromBytes(content, source, map[string]int{})
	if err != nil {
		t.Fatalf("markdownFeedUnitsFromBytes: %v", err)
	}
	if len(extracted) == 0 {
		t.Fatal("expected at least one extracted feed unit from clean fixture")
	}

	segByID := map[string]SourceSegment{}
	for _, s := range segments {
		segByID[s.SegmentID] = s
	}

	feedUnits := make([]FeedUnit, 0, len(extracted))
	ragInputs := make([]RAGBuildInput, 0, len(extracted))
	for _, e := range extracted {
		feedUnits = append(feedUnits, e.FeedUnit)
		ragInputs = append(ragInputs, RAGBuildInput{
			Unit:       e.FeedUnit,
			Content:    e.Content,
			SourceHash: source.Hash,
			Domain:     "rbok",
			Priority:   "primary",
			Status:     "active",
			Confidence: "medium",
		})
	}
	chunks, err := BuildRAGMetadata(ragInputs, segByID, fqgConfig())
	if err != nil {
		t.Fatalf("BuildRAGMetadata: %v", err)
	}

	report := CheckFeedQuality(FeedQualityInput{
		FeedUnits: feedUnits,
		Chunks:    chunks,
		Segments:  segments,
	})

	if report.Status != "pass" {
		t.Fatalf("expected status=pass, got %q with findings: %+v", report.Status, report.Findings)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(report.Findings), report.Findings)
	}
	if report.SourceDerivedUnitCount == 0 {
		t.Fatal("expected SourceDerivedUnitCount > 0 on the clean fixture")
	}
	if report.ChunkCount == 0 {
		t.Fatal("expected ChunkCount > 0 on the clean fixture")
	}
}

// ----------------------------------------------------------------------------
// Matrix-derived units (no source-segment evidence) must not be flagged.
// ----------------------------------------------------------------------------

func TestCheckFeedQuality_MatrixDerivedUnitSkipped(t *testing.T) {
	t.Parallel()

	matrixOnly := FeedUnit{
		UnitID:       "RBOK-MATRIX-ONLY",
		Name:         "Matrix Unit",
		Domain:       "rbok",
		UnitType:     "rule",
		Criticality:  "medium",
		Status:       "active",
		BusinessRule: "Rule from canonical-matrix YAML; no source-segment evidence.",
		SourceIDs:    []string{"matrix-src"},
	}
	report := CheckFeedQuality(FeedQualityInput{FeedUnits: []FeedUnit{matrixOnly}})
	if report.Status != "pass" {
		t.Fatalf("matrix-derived unit should be skipped; got %q with %+v", report.Status, report.Findings)
	}
	if report.SourceDerivedUnitCount != 0 {
		t.Fatalf("expected SourceDerivedUnitCount=0, got %d", report.SourceDerivedUnitCount)
	}
}

// ----------------------------------------------------------------------------
// Negative tests: one per stable finding code.
// ----------------------------------------------------------------------------

func TestCheckFeedQuality_FeedUnitNoSegment(t *testing.T) {
	t.Parallel()
	u := fqgValidUnit()
	u.SourceSegmentID = "" // source-derived markers (SourcePath, span, hash) remain
	report := CheckFeedQuality(FeedQualityInput{FeedUnits: []FeedUnit{u}})
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %q", report.Status)
	}
	if !containsCode(report.Findings, FindingFeedUnitNoSegment) {
		t.Fatalf("expected %s, got findings: %+v", FindingFeedUnitNoSegment, report.Findings)
	}
}

func TestCheckFeedQuality_FeedUnitNoSource(t *testing.T) {
	t.Parallel()
	u := fqgValidUnit()
	u.SourceID = ""
	report := CheckFeedQuality(FeedQualityInput{FeedUnits: []FeedUnit{u}})
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %q", report.Status)
	}
	if !containsCode(report.Findings, FindingFeedUnitNoSource) {
		t.Fatalf("expected %s, got findings: %+v", FindingFeedUnitNoSource, report.Findings)
	}
}

func TestCheckFeedQuality_FeedUnitNoSpan(t *testing.T) {
	t.Parallel()
	u := fqgValidUnit()
	u.StartByte = 0
	u.EndByte = 0
	report := CheckFeedQuality(FeedQualityInput{FeedUnits: []FeedUnit{u}})
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %q", report.Status)
	}
	if !containsCode(report.Findings, FindingFeedUnitNoSpan) {
		t.Fatalf("expected %s, got findings: %+v", FindingFeedUnitNoSpan, report.Findings)
	}
}

func TestCheckFeedQuality_FeedEmptyText(t *testing.T) {
	t.Parallel()
	u := fqgValidUnit()
	u.BusinessRule = "    \n\t  "
	report := CheckFeedQuality(FeedQualityInput{FeedUnits: []FeedUnit{u}})
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %q", report.Status)
	}
	if !containsCode(report.Findings, FindingFeedEmptyText) {
		t.Fatalf("expected %s, got findings: %+v", FindingFeedEmptyText, report.Findings)
	}
}

func TestCheckFeedQuality_FeedJunkText(t *testing.T) {
	t.Parallel()
	u := fqgValidUnit()
	u.BusinessRule = "---|---|---" // markdown table separator
	report := CheckFeedQuality(FeedQualityInput{FeedUnits: []FeedUnit{u}})
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %q", report.Status)
	}
	if !containsCode(report.Findings, FindingFeedJunkText) {
		t.Fatalf("expected %s, got findings: %+v", FindingFeedJunkText, report.Findings)
	}

	// Punctuation-only text should also trip the rule.
	u2 := fqgValidUnit()
	u2.BusinessRule = "...... ;;; ---"
	report2 := CheckFeedQuality(FeedQualityInput{FeedUnits: []FeedUnit{u2}})
	if !containsCode(report2.Findings, FindingFeedJunkText) {
		t.Fatalf("expected punctuation-only %s, got findings: %+v", FindingFeedJunkText, report2.Findings)
	}
}

func TestCheckFeedQuality_FeedDuplicateSpan(t *testing.T) {
	t.Parallel()
	u1 := fqgValidUnit()
	u2 := fqgValidUnit()
	u2.UnitID = "RBOK-MD-FQG-RULE-A-DUP"
	// u2 keeps the same SourceID/StartByte/EndByte as u1 → duplicate span.
	report := CheckFeedQuality(FeedQualityInput{FeedUnits: []FeedUnit{u1, u2}})
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %q", report.Status)
	}
	if !containsCode(report.Findings, FindingFeedDuplicateSpan) {
		t.Fatalf("expected %s, got findings: %+v", FindingFeedDuplicateSpan, report.Findings)
	}
	if report.DuplicateSpanCount != 1 {
		t.Fatalf("expected DuplicateSpanCount=1, got %d", report.DuplicateSpanCount)
	}
}

func TestCheckFeedQuality_RAGChunkNoUnit(t *testing.T) {
	t.Parallel()
	c := fqgValidChunk()
	c.UnitIDs = nil
	c.CanonicalUnitID = ""
	seg := fqgValidSegment()
	report := CheckFeedQuality(FeedQualityInput{
		Chunks:   []ChunkMetadata{c},
		Segments: []SourceSegment{seg},
	})
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %q", report.Status)
	}
	if !containsCode(report.Findings, RAGChunkNoUnit) {
		t.Fatalf("expected %s, got findings: %+v", RAGChunkNoUnit, report.Findings)
	}
}

func TestCheckFeedQuality_RAGChunkNoSegment(t *testing.T) {
	t.Parallel()
	c := fqgValidChunk()
	c.SourceSegmentID = ""
	report := CheckFeedQuality(FeedQualityInput{Chunks: []ChunkMetadata{c}})
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %q", report.Status)
	}
	if !containsCode(report.Findings, RAGChunkNoSegment) {
		t.Fatalf("expected %s, got findings: %+v", RAGChunkNoSegment, report.Findings)
	}

	// Unresolved segment id is also reported as RAG_CHUNK_NO_SEGMENT.
	c2 := fqgValidChunk()
	c2.SourceSegmentID = "seg:DOES-NOT-EXIST"
	report2 := CheckFeedQuality(FeedQualityInput{Chunks: []ChunkMetadata{c2}})
	if !containsCode(report2.Findings, RAGChunkNoSegment) {
		t.Fatalf("expected unresolved-id %s, got findings: %+v", RAGChunkNoSegment, report2.Findings)
	}
}

func TestCheckFeedQuality_RAGChunkNonSemanticSource(t *testing.T) {
	t.Parallel()

	// Disposition-driven: chunk points at a structure_only heading.
	headingSeg := fqgValidSegment()
	headingSeg.SegmentID = "seg:SRC-FQG:0-7:heading"
	headingSeg.Kind = KindHeading
	headingSeg.Disposition = DispositionStructureOnly

	c := fqgValidChunk()
	c.SourceSegmentID = headingSeg.SegmentID

	report := CheckFeedQuality(FeedQualityInput{
		Chunks:   []ChunkMetadata{c},
		Segments: []SourceSegment{headingSeg},
	})
	if report.Status != "fail" {
		t.Fatalf("expected fail, got %q", report.Status)
	}
	if !containsCode(report.Findings, RAGChunkNonSemanticSource) {
		t.Fatalf("expected %s, got findings: %+v", RAGChunkNonSemanticSource, report.Findings)
	}

	// Kind-driven: a (theoretically canonical) chunk pointing at a blank
	// segment must still be rejected.
	blankSeg := fqgValidSegment()
	blankSeg.SegmentID = "seg:SRC-FQG:80-81:blank"
	blankSeg.Kind = KindBlank
	blankSeg.Disposition = DispositionCoverageOnly

	c2 := fqgValidChunk()
	c2.SourceSegmentID = blankSeg.SegmentID
	report2 := CheckFeedQuality(FeedQualityInput{
		Chunks:   []ChunkMetadata{c2},
		Segments: []SourceSegment{blankSeg},
	})
	if !containsCode(report2.Findings, RAGChunkNonSemanticSource) {
		t.Fatalf("expected blank-kind %s, got findings: %+v", RAGChunkNonSemanticSource, report2.Findings)
	}
}

// ----------------------------------------------------------------------------
// Smoke-test the JSON-shape fields — every finding must always have a Code
// and a Message.
// ----------------------------------------------------------------------------

func TestCheckFeedQuality_FindingsAlwaysHaveCodeAndMessage(t *testing.T) {
	t.Parallel()
	u := fqgValidUnit()
	u.SourceSegmentID = ""
	u.SourceID = ""
	u.BusinessRule = ""
	report := CheckFeedQuality(FeedQualityInput{FeedUnits: []FeedUnit{u}})
	if len(report.Findings) == 0 {
		t.Fatal("expected at least one finding")
	}
	for _, f := range report.Findings {
		if strings.TrimSpace(f.Code) == "" {
			t.Fatalf("finding has empty code: %+v", f)
		}
		if strings.TrimSpace(f.Message) == "" {
			t.Fatalf("finding has empty message: %+v", f)
		}
	}
}

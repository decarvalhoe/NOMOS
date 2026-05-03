package corpus

import (
	"strings"
	"testing"
)

const gateSourceID = "src-gate-test"
const gateSourcePath = "fixtures/gate.md"

// findingsByCode bins the report's Findings slice by code so individual
// sub-tests can assert on the codes they care about without depending on
// the order in which the gate emits them.
func findingsByCode(report IntegrityReport) map[string][]IntegrityFinding {
	out := map[string][]IntegrityFinding{}
	for _, f := range report.Findings {
		out[f.Code] = append(out[f.Code], f)
	}
	return out
}

func mustScan(t *testing.T, content string) []SourceSegment {
	t.Helper()
	segs, err := ScanMarkdown(gateSourceID, gateSourcePath, []byte(content))
	if err != nil {
		t.Fatalf("ScanMarkdown error: %v", err)
	}
	return segs
}

func TestSourceIntegrityGate_PositiveMixedCleanFixture(t *testing.T) {
	doc := strings.Join([]string{
		"# Heading One",
		"",
		"This is a clean paragraph with normal content.",
		"",
		"- first item",
		"- second item",
		"",
		"| col a | col b |",
		"| --- | --- |",
		"| a1 | b1 |",
		"",
		"```",
		"code goes here",
		"```",
		"",
		"Another final paragraph.",
		"",
	}, "\n")
	segs := mustScan(t, doc)
	report := CheckSourceIntegrity(
		[]SourceInput{{SourceID: gateSourceID, Path: gateSourcePath, Content: []byte(doc)}},
		segs,
	)
	if report.Status != "pass" {
		t.Fatalf("expected status=pass on clean fixture; got %s with findings: %+v",
			report.Status, report.Findings)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected 0 findings on clean fixture; got %d: %+v",
			len(report.Findings), report.Findings)
	}
	if report.SourceCount != 1 {
		t.Fatalf("expected source_count=1; got %d", report.SourceCount)
	}
	if report.SegmentCount != len(segs) {
		t.Fatalf("expected segment_count=%d; got %d", len(segs), report.SegmentCount)
	}
	if report.SemanticSegmentCount == 0 {
		t.Fatalf("expected at least one canonical_atom in mixed fixture")
	}
}

func TestSourceIntegrityGate_NegativeMissingParagraph(t *testing.T) {
	doc := strings.Join([]string{
		"# Heading",
		"",
		"This paragraph will be dropped from the ledger.",
		"",
		"Trailing paragraph stays.",
		"",
	}, "\n")
	segs := mustScan(t, doc)

	dropIdx := -1
	for i, seg := range segs {
		if seg.Kind == KindParagraph && seg.ParentSegmentID == "" &&
			strings.Contains(string([]byte(doc)[seg.StartByte:seg.EndByte]), "dropped") {
			dropIdx = i
			break
		}
	}
	if dropIdx == -1 {
		t.Fatalf("could not find target paragraph segment to drop in fixture")
	}
	dropped := segs[dropIdx]
	withoutPara := append([]SourceSegment{}, segs[:dropIdx]...)
	withoutPara = append(withoutPara, segs[dropIdx+1:]...)

	report := CheckSourceIntegrity(
		[]SourceInput{{SourceID: gateSourceID, Path: gateSourcePath, Content: []byte(doc)}},
		withoutPara,
	)
	if report.Status != "fail" {
		t.Fatalf("expected status=fail when a paragraph is missing; got %s", report.Status)
	}
	by := findingsByCode(report)
	uncovered := by[FindingSourceUncoveredRange]
	if len(uncovered) == 0 {
		t.Fatalf("expected at least one %s finding; got findings: %+v",
			FindingSourceUncoveredRange, report.Findings)
	}
	matched := false
	for _, f := range uncovered {
		if f.StartByte <= dropped.StartByte && f.EndByte >= dropped.EndByte {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("expected an uncovered finding covering bytes [%d,%d); got %+v",
			dropped.StartByte, dropped.EndByte, uncovered)
	}
	if len(report.UncoveredRanges) == 0 {
		t.Fatalf("expected uncovered_ranges to be populated alongside findings")
	}
}

func TestSourceIntegrityGate_NegativeDuplicateSpan(t *testing.T) {
	doc := strings.Join([]string{
		"# Title",
		"",
		"A simple paragraph that will be duplicated in the ledger.",
		"",
	}, "\n")
	segs := mustScan(t, doc)

	var paragraph SourceSegment
	found := false
	for _, seg := range segs {
		if seg.Kind == KindParagraph && seg.Disposition == DispositionCanonicalAtom {
			paragraph = seg
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("could not find canonical_atom paragraph in fixture")
	}
	dup := paragraph
	dup.SegmentID = paragraph.SegmentID + ":dup"
	withDup := append(append([]SourceSegment{}, segs...), dup)

	report := CheckSourceIntegrity(
		[]SourceInput{{SourceID: gateSourceID, Path: gateSourcePath, Content: []byte(doc)}},
		withDup,
	)
	if report.Status != "fail" {
		t.Fatalf("expected status=fail with duplicate span; got %s", report.Status)
	}
	by := findingsByCode(report)
	if len(by[FindingSourceDuplicateSemantic]) == 0 {
		t.Fatalf("expected %s finding; got %+v", FindingSourceDuplicateSemantic, report.Findings)
	}
	if len(report.DuplicateSemanticRanges) == 0 {
		t.Fatalf("expected duplicate_semantic_ranges to be populated")
	}
}

func TestSourceIntegrityGate_NegativeDashSeparatorAsSemantic(t *testing.T) {
	content := []byte("---\n")
	seg := SourceSegment{
		SegmentID:          "seg:junk:dash",
		SourceID:           gateSourceID,
		SourcePath:         gateSourcePath,
		Kind:               KindParagraph,
		Disposition:        DispositionCanonicalAtom,
		StartByte:          0,
		EndByte:            len(content),
		StartLine:          1,
		StartColumn:        1,
		EndLine:            1,
		EndColumn:          4,
		RawTextHash:        ComputeRawTextHash(content),
		NormalizedTextHash: ComputeNormalizedTextHash(string(content)),
		IncludeInFeed:      true,
		IncludeInRAG:       true,
	}
	report := CheckSourceIntegrity(
		[]SourceInput{{SourceID: gateSourceID, Path: gateSourcePath, Content: content}},
		[]SourceSegment{seg},
	)
	if report.Status != "fail" {
		t.Fatalf("expected status=fail for dash separator; got %s", report.Status)
	}
	by := findingsByCode(report)
	if len(by[FindingSourceJunkSemantic]) == 0 {
		t.Fatalf("expected %s finding; got %+v", FindingSourceJunkSemantic, report.Findings)
	}
	if len(report.JunkSemanticSegments) == 0 || report.JunkSemanticSegments[0] != seg.SegmentID {
		t.Fatalf("expected junk_semantic_segments to include %q; got %v",
			seg.SegmentID, report.JunkSemanticSegments)
	}
}

func TestSourceIntegrityGate_NegativeTableSeparatorAsSemantic(t *testing.T) {
	content := []byte("| --- | --- |\n")
	seg := SourceSegment{
		SegmentID:          "seg:junk:table-sep",
		SourceID:           gateSourceID,
		SourcePath:         gateSourcePath,
		Kind:               KindParagraph,
		Disposition:        DispositionCanonicalAtom,
		StartByte:          0,
		EndByte:            len(content),
		StartLine:          1,
		StartColumn:        1,
		EndLine:            1,
		EndColumn:          len(content),
		RawTextHash:        ComputeRawTextHash(content),
		NormalizedTextHash: ComputeNormalizedTextHash(string(content)),
		IncludeInFeed:      true,
		IncludeInRAG:       true,
	}
	report := CheckSourceIntegrity(
		[]SourceInput{{SourceID: gateSourceID, Path: gateSourcePath, Content: content}},
		[]SourceSegment{seg},
	)
	if report.Status != "fail" {
		t.Fatalf("expected status=fail for table separator; got %s", report.Status)
	}
	by := findingsByCode(report)
	if len(by[FindingSourceJunkSemantic]) == 0 {
		t.Fatalf("expected %s finding; got %+v", FindingSourceJunkSemantic, report.Findings)
	}
}

func TestSourceIntegrityGate_NegativeUnsupportedBlocking(t *testing.T) {
	content := []byte("<div>raw html</div>\n")
	seg := SourceSegment{
		SegmentID:         "seg:unsupported:html",
		SourceID:          gateSourceID,
		SourcePath:        gateSourcePath,
		Kind:              KindUnsupportedBlock,
		Disposition:       DispositionUnsupportedBlocking,
		StartByte:         0,
		EndByte:           len(content),
		StartLine:         1,
		StartColumn:       1,
		EndLine:           1,
		EndColumn:         len(content),
		RawTextHash:       ComputeRawTextHash(content),
		UnsupportedReason: "raw HTML block not yet classified",
	}
	report := CheckSourceIntegrity(
		[]SourceInput{{SourceID: gateSourceID, Path: gateSourcePath, Content: content}},
		[]SourceSegment{seg},
	)
	if report.Status != "fail" {
		t.Fatalf("expected status=fail for unsupported_blocking; got %s", report.Status)
	}
	by := findingsByCode(report)
	if len(by[FindingSourceUnsupportedBlocking]) == 0 {
		t.Fatalf("expected %s finding; got %+v", FindingSourceUnsupportedBlocking, report.Findings)
	}
	if len(report.UnsupportedBlockingSegments) == 0 {
		t.Fatalf("expected unsupported_blocking_segments to be populated")
	}
}

func TestSourceIntegrityGate_NegativeInvalidRangeAndMissingHash(t *testing.T) {
	t.Run("invalid byte range", func(t *testing.T) {
		seg := SourceSegment{
			SegmentID:   "seg:bad:byterange",
			SourceID:    gateSourceID,
			SourcePath:  gateSourcePath,
			Kind:        KindParagraph,
			Disposition: DispositionStructureOnly,
			StartByte:   20,
			EndByte:     5,
			StartLine:   1,
			StartColumn: 1,
			EndLine:     1,
			EndColumn:   1,
			RawTextHash: ComputeRawTextHash([]byte("x")),
		}
		report := CheckSourceIntegrity(nil, []SourceSegment{seg})
		if report.Status != "fail" {
			t.Fatalf("expected fail for invalid byte range; got %s", report.Status)
		}
		by := findingsByCode(report)
		if len(by[FindingSourceSegmentInvalidRange]) == 0 {
			t.Fatalf("expected %s finding; got %+v",
				FindingSourceSegmentInvalidRange, report.Findings)
		}
	})

	t.Run("invalid line range", func(t *testing.T) {
		seg := SourceSegment{
			SegmentID:   "seg:bad:linerange",
			SourceID:    gateSourceID,
			SourcePath:  gateSourcePath,
			Kind:        KindParagraph,
			Disposition: DispositionStructureOnly,
			StartByte:   0,
			EndByte:     5,
			StartLine:   10,
			StartColumn: 1,
			EndLine:     5,
			EndColumn:   1,
			RawTextHash: ComputeRawTextHash([]byte("x")),
		}
		report := CheckSourceIntegrity(nil, []SourceSegment{seg})
		by := findingsByCode(report)
		if len(by[FindingSourceSegmentInvalidRange]) == 0 {
			t.Fatalf("expected %s finding for swapped lines; got %+v",
				FindingSourceSegmentInvalidRange, report.Findings)
		}
	})

	t.Run("missing hash on canonical atom", func(t *testing.T) {
		seg := SourceSegment{
			SegmentID:   "seg:bad:nohash",
			SourceID:    gateSourceID,
			SourcePath:  gateSourcePath,
			Kind:        KindParagraph,
			Disposition: DispositionCanonicalAtom,
			StartByte:   0,
			EndByte:     3,
			StartLine:   1,
			StartColumn: 1,
			EndLine:     1,
			EndColumn:   4,
		}
		report := CheckSourceIntegrity(nil, []SourceSegment{seg})
		if report.Status != "fail" {
			t.Fatalf("expected fail for missing hash; got %s", report.Status)
		}
		by := findingsByCode(report)
		if len(by[FindingSourceSegmentMissingHash]) == 0 {
			t.Fatalf("expected %s finding; got %+v",
				FindingSourceSegmentMissingHash, report.Findings)
		}
	})

	t.Run("only raw hash present", func(t *testing.T) {
		seg := SourceSegment{
			SegmentID:   "seg:bad:onlyrawhash",
			SourceID:    gateSourceID,
			SourcePath:  gateSourcePath,
			Kind:        KindParagraph,
			Disposition: DispositionCanonicalAtom,
			StartByte:   0,
			EndByte:     3,
			StartLine:   1,
			StartColumn: 1,
			EndLine:     1,
			EndColumn:   4,
			RawTextHash: ComputeRawTextHash([]byte("foo")),
		}
		report := CheckSourceIntegrity(nil, []SourceSegment{seg})
		by := findingsByCode(report)
		if len(by[FindingSourceSegmentMissingHash]) == 0 {
			t.Fatalf("expected %s finding when normalized hash is missing; got %+v",
				FindingSourceSegmentMissingHash, report.Findings)
		}
	})
}

func TestSourceIntegrityGate_StatusPassRequiresEmptyFindings(t *testing.T) {
	report := CheckSourceIntegrity(nil, nil)
	if report.Status != "pass" {
		t.Fatalf("empty input should pass; got %s", report.Status)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("empty input must yield zero findings; got %+v", report.Findings)
	}
	if report.SourceCount != 0 || report.SegmentCount != 0 || report.SemanticSegmentCount != 0 {
		t.Fatalf("empty input counters expected zero; got src=%d seg=%d sem=%d",
			report.SourceCount, report.SegmentCount, report.SemanticSegmentCount)
	}
}

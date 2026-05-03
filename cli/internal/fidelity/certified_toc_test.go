package fidelity

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func sampleHeadings() []TOCHeading {
	return []TOCHeading{
		{Title: "Reglement General", Level: 1, Line: 1, NodeID: "N-001"},
		{Title: "Chapitre 1 - Garanties", Level: 2, Line: 5, NodeID: "N-002"},
		{Title: "Section 1.1 - Dommages des eaux", Level: 3, Line: 10, NodeID: "N-003"},
		{Title: "Section 1.2 - Incendie", Level: 3, Line: 20, NodeID: "N-004"},
		{Title: "Chapitre 2 - Exclusions", Level: 2, Line: 30, NodeID: "N-005"},
		{Title: "Section 2.1 - Toiture", Level: 3, Line: 35, NodeID: "N-006"},
	}
}

func TestBuildCertifiedTOCBasic(t *testing.T) {
	toc := BuildCertifiedTOC(TOCInput{DocumentRef: "rga-2026", Headings: sampleHeadings()})
	if toc.Format != CertifiedTOCFormat {
		t.Fatalf("format: %q", toc.Format)
	}
	if toc.DocumentRef != "rga-2026" {
		t.Fatalf("doc ref: %q", toc.DocumentRef)
	}
	if toc.EntryCount != 6 {
		t.Fatalf("entries: %d", toc.EntryCount)
	}
}

func TestBuildCertifiedTOCNumbering(t *testing.T) {
	toc := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()})
	expected := []string{"1", "1.1", "1.1.1", "1.1.2", "1.2", "1.2.1"}
	for i, e := range toc.Entries {
		if e.Number != expected[i] {
			t.Fatalf("entry[%d] number: expected %q, got %q", i, expected[i], e.Number)
		}
	}
}

func TestBuildCertifiedTOCDepths(t *testing.T) {
	toc := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()})
	expectedDepths := []int{0, 1, 2, 2, 1, 2}
	for i, e := range toc.Entries {
		if e.Depth != expectedDepths[i] {
			t.Fatalf("entry[%d] depth: expected %d, got %d", i, expectedDepths[i], e.Depth)
		}
	}
}

func TestBuildCertifiedTOCMaxDepth(t *testing.T) {
	toc := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()})
	if toc.MaxDepth != 2 {
		t.Fatalf("max depth: %d", toc.MaxDepth)
	}
}

func TestBuildCertifiedTOCHashes(t *testing.T) {
	toc := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()})
	if !strings.HasPrefix(toc.StructureHash, "sha256:") {
		t.Fatalf("structure hash: %q", toc.StructureHash)
	}
	for _, e := range toc.Entries {
		if !strings.HasPrefix(e.Hash, "sha256:") {
			t.Fatalf("entry %s hash: %q", e.Number, e.Hash)
		}
	}
}

func TestBuildCertifiedTOCDeterministic(t *testing.T) {
	t1 := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()})
	t2 := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()})
	if t1.StructureHash != t2.StructureHash {
		t.Fatal("structure hash not deterministic")
	}
	for i := range t1.Entries {
		if t1.Entries[i].Hash != t2.Entries[i].Hash {
			t.Fatalf("entry[%d] hash not deterministic", i)
		}
	}
}

func TestBuildCertifiedTOCChildCounts(t *testing.T) {
	toc := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()})
	// "1" (Reglement) has 2 direct children: "1.1" and "1.2".
	if toc.Entries[0].ChildCount != 2 {
		t.Fatalf("root child count: expected 2, got %d", toc.Entries[0].ChildCount)
	}
	// "1.1" (Chapitre 1) has 2 children: "1.1.1" and "1.1.2".
	if toc.Entries[1].ChildCount != 2 {
		t.Fatalf("chap1 child count: expected 2, got %d", toc.Entries[1].ChildCount)
	}
	// "1.1.1" (Section 1.1) is a leaf.
	if toc.Entries[2].ChildCount != 0 {
		t.Fatalf("section child count: expected 0, got %d", toc.Entries[2].ChildCount)
	}
}

func TestBuildCertifiedTOCNodeIDs(t *testing.T) {
	toc := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()})
	if toc.Entries[0].NodeID != "N-001" {
		t.Fatalf("node_id: %q", toc.Entries[0].NodeID)
	}
}

func TestBuildCertifiedTOCLines(t *testing.T) {
	toc := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()})
	if toc.Entries[0].Line != 1 {
		t.Fatalf("line: %d", toc.Entries[0].Line)
	}
	if toc.Entries[4].Line != 30 {
		t.Fatalf("line: %d", toc.Entries[4].Line)
	}
}

func TestBuildCertifiedTOCEmpty(t *testing.T) {
	toc := BuildCertifiedTOC(TOCInput{DocumentRef: "empty"})
	if toc.EntryCount != 0 {
		t.Fatalf("entries: %d", toc.EntryCount)
	}
	if toc.MaxDepth != 0 {
		t.Fatalf("max depth: %d", toc.MaxDepth)
	}
}

func TestBuildCertifiedTOCSingleHeading(t *testing.T) {
	toc := BuildCertifiedTOC(TOCInput{Headings: []TOCHeading{
		{Title: "Document", Level: 1, Line: 1},
	}})
	if toc.EntryCount != 1 {
		t.Fatalf("entries: %d", toc.EntryCount)
	}
	if toc.Entries[0].Number != "1" {
		t.Fatalf("number: %q", toc.Entries[0].Number)
	}
}

// --- CompareTOC / drift ---

func TestCompareTOCMatch(t *testing.T) {
	toc := BuildCertifiedTOC(TOCInput{DocumentRef: "doc", Headings: sampleHeadings()})
	report := CompareTOC(toc, toc)
	if !report.Match {
		t.Fatal("identical TOCs should match")
	}
	if len(report.Drifts) != 0 {
		t.Fatalf("no drifts expected: %v", report.Drifts)
	}
}

func TestCompareTOCAdded(t *testing.T) {
	source := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()[:3]})
	extracted := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()[:4]})
	report := CompareTOC(source, extracted)
	if report.Match {
		t.Fatal("should not match")
	}
	found := false
	for _, d := range report.Drifts {
		if d.Code == CodeTOCAdded {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected TOC_ENTRY_ADDED: %v", report.Drifts)
	}
}

func TestCompareTOCRemoved(t *testing.T) {
	source := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()})
	extracted := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()[:4]})
	report := CompareTOC(source, extracted)
	found := false
	for _, d := range report.Drifts {
		if d.Code == CodeTOCRemoved {
			found = true
			if !d.Blocking {
				t.Fatal("removed should be blocking")
			}
		}
	}
	if !found {
		t.Fatalf("expected TOC_ENTRY_REMOVED: %v", report.Drifts)
	}
}

func TestCompareTOCRenamed(t *testing.T) {
	h := sampleHeadings()
	source := BuildCertifiedTOC(TOCInput{Headings: h})
	h2 := make([]TOCHeading, len(h))
	copy(h2, h)
	h2[1].Title = "Chapitre 1 - Renamed"
	extracted := BuildCertifiedTOC(TOCInput{Headings: h2})
	report := CompareTOC(source, extracted)
	found := false
	for _, d := range report.Drifts {
		if d.Code == CodeTOCRenamed {
			found = true
			if !d.Blocking {
				t.Fatal("rename should be blocking")
			}
		}
	}
	if !found {
		t.Fatalf("expected TOC_ENTRY_RENAMED: %v", report.Drifts)
	}
}

func TestCompareTOCDepthChanged(t *testing.T) {
	// Manually construct TOCs with same numbering but different depths.
	source := CertifiedTOC{
		StructureHash: "sha256:aaa",
		Entries: []TOCEntry{
			{Number: "1", Title: "Doc", Depth: 0, Hash: "sha256:x1"},
			{Number: "1.1", Title: "Chapter", Depth: 1, Hash: "sha256:x2"},
		},
	}
	extracted := CertifiedTOC{
		StructureHash: "sha256:bbb",
		Entries: []TOCEntry{
			{Number: "1", Title: "Doc", Depth: 0, Hash: "sha256:x1"},
			{Number: "1.1", Title: "Chapter", Depth: 2, Hash: "sha256:x3"}, // depth changed
		},
	}
	report := CompareTOC(source, extracted)
	found := false
	for _, d := range report.Drifts {
		if d.Code == CodeTOCDepthChanged {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected TOC_DEPTH_CHANGED: %v", report.Drifts)
	}
}

func TestCompareTOCDriftsSorted(t *testing.T) {
	source := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()})
	extracted := BuildCertifiedTOC(TOCInput{Headings: sampleHeadings()[:2]})
	report := CompareTOC(source, extracted)
	for i := 1; i < len(report.Drifts); i++ {
		if report.Drifts[i].Number < report.Drifts[i-1].Number {
			t.Fatalf("drifts not sorted")
		}
	}
}

func TestHasBlockingDrift(t *testing.T) {
	r := DriftReport{Drifts: []DriftEntry{{Blocking: true}}}
	if !r.HasBlockingDrift() {
		t.Fatal("should have blocking drift")
	}
	r2 := DriftReport{Drifts: []DriftEntry{{Blocking: false}}}
	if r2.HasBlockingDrift() {
		t.Fatal("should not have blocking drift")
	}
	r3 := DriftReport{}
	if r3.HasBlockingDrift() {
		t.Fatal("empty should not have blocking drift")
	}
}

func TestWriteCertifiedTOCJSON(t *testing.T) {
	toc := BuildCertifiedTOC(TOCInput{DocumentRef: "test", Headings: sampleHeadings()})
	var buf bytes.Buffer
	if err := WriteCertifiedTOC(&buf, toc); err != nil {
		t.Fatalf("write: %v", err)
	}
	var decoded CertifiedTOC
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Format != CertifiedTOCFormat {
		t.Fatalf("format: %q", decoded.Format)
	}
	if decoded.EntryCount != toc.EntryCount {
		t.Fatal("round-trip count mismatch")
	}
	if decoded.StructureHash != toc.StructureHash {
		t.Fatal("round-trip hash mismatch")
	}
}

func TestBuildCertifiedTOCDeepHierarchy(t *testing.T) {
	headings := []TOCHeading{
		{Title: "Doc", Level: 1, Line: 1},
		{Title: "Part", Level: 2, Line: 2},
		{Title: "Chapter", Level: 3, Line: 3},
		{Title: "Section", Level: 4, Line: 4},
		{Title: "Sub", Level: 5, Line: 5},
		{Title: "Para", Level: 6, Line: 6},
	}
	toc := BuildCertifiedTOC(TOCInput{Headings: headings})
	if toc.MaxDepth != 5 {
		t.Fatalf("max depth: %d", toc.MaxDepth)
	}
	if toc.Entries[5].Number != "1.1.1.1.1.1" {
		t.Fatalf("deep number: %q", toc.Entries[5].Number)
	}
}

func TestStructureHashChangesOnContent(t *testing.T) {
	h1 := sampleHeadings()
	h2 := make([]TOCHeading, len(h1))
	copy(h2, h1)
	h2[0].Title = "Different Title"
	t1 := BuildCertifiedTOC(TOCInput{Headings: h1})
	t2 := BuildCertifiedTOC(TOCInput{Headings: h2})
	if t1.StructureHash == t2.StructureHash {
		t.Fatal("different titles should produce different hashes")
	}
}

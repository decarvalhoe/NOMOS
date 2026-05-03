package corpus

import (
	"reflect"
	"strings"
	"testing"
)

const testSourceID = "src-test"
const testSourcePath = "fixtures/test.md"

// scanOK is a small helper that runs ScanMarkdown and fails the test on
// any error.
func scanOK(t *testing.T, content string) []SourceSegment {
	t.Helper()
	segs, err := ScanMarkdown(testSourceID, testSourcePath, []byte(content))
	if err != nil {
		t.Fatalf("ScanMarkdown returned unexpected error: %v", err)
	}
	return segs
}

// assertCoverageAndIntegrity verifies the structural invariants documented
// on ScanMarkdown: every segment validates, root-level segments cover the
// full input without overlap, sibling segments do not overlap, and child
// spans fit within their parent.
func assertCoverageAndIntegrity(t *testing.T, content string, segs []SourceSegment) {
	t.Helper()
	byID := make(map[string]SourceSegment, len(segs))
	for _, s := range segs {
		if err := s.Validate(); err != nil {
			t.Fatalf("segment %q failed Validate(): %v\n  seg=%+v", s.SegmentID, err, s)
		}
		if _, dup := byID[s.SegmentID]; dup {
			t.Fatalf("duplicate SegmentID emitted: %q", s.SegmentID)
		}
		byID[s.SegmentID] = s
	}

	// Root-level segments must cover [0, len(content)) contiguously, in order.
	var roots []SourceSegment
	for _, s := range segs {
		if s.ParentSegmentID == "" {
			roots = append(roots, s)
		}
	}
	if len(roots) == 0 && len(content) == 0 {
		return
	}
	cursor := 0
	totalCovered := 0
	for i, r := range roots {
		if r.StartByte != cursor {
			t.Fatalf("root segment[%d] %q starts at %d; expected %d (gap or overlap)",
				i, r.SegmentID, r.StartByte, cursor)
		}
		if r.EndByte < r.StartByte {
			t.Fatalf("root segment[%d] %q has end_byte %d < start_byte %d",
				i, r.SegmentID, r.EndByte, r.StartByte)
		}
		cursor = r.EndByte
		totalCovered += r.EndByte - r.StartByte
	}
	if cursor != len(content) {
		t.Fatalf("root segments cover only %d bytes; input is %d bytes", cursor, len(content))
	}
	if totalCovered != len(content) {
		t.Fatalf("sum of root span sizes = %d, expected %d", totalCovered, len(content))
	}

	// Children must fit inside their parent's byte range.
	// Siblings sharing the same parent must not overlap each other.
	type spanKey struct {
		parent string
		idx    int
	}
	siblingsByParent := map[string][]SourceSegment{}
	for _, s := range segs {
		if s.ParentSegmentID == "" {
			continue
		}
		parent, ok := byID[s.ParentSegmentID]
		if !ok {
			t.Fatalf("segment %q references unknown parent %q", s.SegmentID, s.ParentSegmentID)
		}
		if s.StartByte < parent.StartByte || s.EndByte > parent.EndByte {
			t.Fatalf("child %q span [%d,%d) escapes parent %q span [%d,%d)",
				s.SegmentID, s.StartByte, s.EndByte,
				parent.SegmentID, parent.StartByte, parent.EndByte)
		}
		siblingsByParent[s.ParentSegmentID] = append(siblingsByParent[s.ParentSegmentID], s)
	}
	for parentID, sibs := range siblingsByParent {
		// Group siblings by kind for the overlap check; same-kind sibs must
		// be strictly ordered. Cross-kind siblings (e.g. table_cell vs
		// link inside the same row) may legitimately overlap, so we don't
		// constrain them.
		byKind := map[string][]SourceSegment{}
		for _, s := range sibs {
			byKind[s.Kind] = append(byKind[s.Kind], s)
		}
		_ = parentID
		for _, group := range byKind {
			for i := 1; i < len(group); i++ {
				prev := group[i-1]
				cur := group[i]
				if cur.StartByte < prev.EndByte {
					t.Fatalf("siblings of kind %q overlap: %q [%d,%d) and %q [%d,%d)",
						cur.Kind, prev.SegmentID, prev.StartByte, prev.EndByte,
						cur.SegmentID, cur.StartByte, cur.EndByte)
				}
			}
		}
	}
}

// assertSliceMatchesHash confirms that for every non-container kind, the
// RawTextHash equals sha256(content[StartByte:EndByte]). This is the
// machine-checkable form of "every segment raw text can be recovered by
// slicing source bytes".
func assertSliceMatchesHash(t *testing.T, content string, segs []SourceSegment) {
	t.Helper()
	containerKinds := map[string]bool{
		KindList:        true,
		KindBlockquote:  true,
		KindTable:       true,
		KindTableHeader: true,
		KindTableRow:    true,
	}
	for _, s := range segs {
		if s.RawTextHash == "" {
			continue
		}
		if containerKinds[s.Kind] {
			continue
		}
		want := ComputeRawTextHash([]byte(content[s.StartByte:s.EndByte]))
		if s.RawTextHash != want {
			t.Fatalf("segment %q (%s) RawTextHash %s does not match sha256 of slice %s",
				s.SegmentID, s.Kind, s.RawTextHash, want)
		}
	}
}

func collectKinds(segs []SourceSegment) map[string]int {
	out := map[string]int{}
	for _, s := range segs {
		out[s.Kind]++
	}
	return out
}

func firstOfKind(segs []SourceSegment, kind string) (SourceSegment, bool) {
	for _, s := range segs {
		if s.Kind == kind {
			return s, true
		}
	}
	return SourceSegment{}, false
}

func TestScanMarkdown_Headings_H1_to_H6(t *testing.T) {
	t.Parallel()
	content := "# H1\n## H2\n### H3\n#### H4\n##### H5\n###### H6\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)
	assertSliceMatchesHash(t, content, segs)

	kinds := collectKinds(segs)
	if kinds[KindHeading] != 6 {
		t.Fatalf("expected 6 heading segments, got %d (kinds=%v)", kinds[KindHeading], kinds)
	}
	for _, s := range segs {
		if s.Kind == KindHeading && s.Disposition != DispositionStructureOnly {
			t.Fatalf("heading %q must be structure_only, got %s", s.SegmentID, s.Disposition)
		}
	}
}

func TestScanMarkdown_ParagraphIsCanonicalAtom(t *testing.T) {
	t.Parallel()
	content := "Article 1.\nThe quick brown fox jumps over the lazy dog.\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)
	assertSliceMatchesHash(t, content, segs)

	p, ok := firstOfKind(segs, KindParagraph)
	if !ok {
		t.Fatalf("no paragraph emitted; got kinds=%v", collectKinds(segs))
	}
	if p.Disposition != DispositionCanonicalAtom {
		t.Fatalf("paragraph must be canonical_atom, got %s", p.Disposition)
	}
	if p.NormalizedTextHash == "" {
		t.Fatal("paragraph must have NormalizedTextHash")
	}
	if !p.IncludeInFeed || !p.IncludeInRAG {
		t.Fatal("paragraph must be flagged include_in_feed and include_in_rag")
	}
}

func TestScanMarkdown_BlankLines(t *testing.T) {
	t.Parallel()
	content := "alpha\n\n\nbeta\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)
	assertSliceMatchesHash(t, content, segs)

	if collectKinds(segs)[KindBlank] != 2 {
		t.Fatalf("expected 2 blank segments, got %v", collectKinds(segs))
	}
	for _, s := range segs {
		if s.Kind == KindBlank && s.Disposition != DispositionCoverageOnly {
			t.Fatalf("blank must be coverage_only, got %s", s.Disposition)
		}
	}
}

func TestScanMarkdown_DecorativeSeparators(t *testing.T) {
	t.Parallel()
	content := "para one.\n\n---\n\npara two.\n\n***\n\npara three.\n\n___\n\npara four.\n\n...\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)
	assertSliceMatchesHash(t, content, segs)

	if collectKinds(segs)[KindDecorativeSeparator] != 4 {
		t.Fatalf("expected 4 decorative separators, got %v", collectKinds(segs))
	}
	for _, s := range segs {
		if s.Kind == KindDecorativeSeparator && s.Disposition != DispositionCoverageOnly {
			t.Fatalf("decorative separator must be coverage_only, got %s", s.Disposition)
		}
	}
}

func TestScanMarkdown_TableEmitsRowsAndCells(t *testing.T) {
	t.Parallel()
	content := "" +
		"| Name | Role |\n" +
		"|------|------|\n" +
		"| Ada  | Lead |\n" +
		"| Lin  |      |\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)
	assertSliceMatchesHash(t, content, segs)

	kinds := collectKinds(segs)
	if kinds[KindTable] != 1 {
		t.Fatalf("expected 1 table, got %d (%v)", kinds[KindTable], kinds)
	}
	if kinds[KindTableHeader] != 1 {
		t.Fatalf("expected 1 table_header, got %d", kinds[KindTableHeader])
	}
	if kinds[KindTableSeparator] != 1 {
		t.Fatalf("expected 1 table_separator, got %d", kinds[KindTableSeparator])
	}
	if kinds[KindTableRow] != 2 {
		t.Fatalf("expected 2 table_row segments, got %d", kinds[KindTableRow])
	}
	// 2 cells per header row + 2 cells per body row = 6 total cells.
	if kinds[KindTableCell] != 6 {
		t.Fatalf("expected 6 table_cell segments, got %d", kinds[KindTableCell])
	}

	for _, s := range segs {
		if s.Kind == KindTableSeparator && s.Disposition != DispositionCoverageOnly {
			t.Fatalf("table_separator must be coverage_only, got %s", s.Disposition)
		}
	}

	// At least one non-empty cell must be canonical_atom; the empty cell must
	// fall back to coverage_only.
	var sawCanonical, sawEmptyCoverage bool
	for _, s := range segs {
		if s.Kind != KindTableCell {
			continue
		}
		txt := strings.TrimSpace(content[s.StartByte:s.EndByte])
		if txt == "" {
			if s.Disposition != DispositionCoverageOnly {
				t.Fatalf("empty cell %q must be coverage_only, got %s", s.SegmentID, s.Disposition)
			}
			sawEmptyCoverage = true
		} else {
			if s.Disposition == DispositionCanonicalAtom {
				sawCanonical = true
			}
		}
	}
	if !sawCanonical {
		t.Fatal("expected at least one canonical_atom table_cell")
	}
	if !sawEmptyCoverage {
		t.Fatal("expected at least one coverage_only empty table_cell")
	}
}

func TestScanMarkdown_FencedCodeBlock(t *testing.T) {
	t.Parallel()
	content := "intro paragraph.\n\n```python\nprint('hi')\n```\n\nfollow-up.\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)
	assertSliceMatchesHash(t, content, segs)

	cb, ok := firstOfKind(segs, KindCodeBlock)
	if !ok {
		t.Fatalf("no code_block emitted; got kinds=%v", collectKinds(segs))
	}
	if cb.Disposition != DispositionCoverageOnly {
		t.Fatalf("code_block must be coverage_only by default, got %s", cb.Disposition)
	}
	slice := content[cb.StartByte:cb.EndByte]
	if !strings.Contains(slice, "print('hi')") {
		t.Fatalf("code_block slice missing body; got %q", slice)
	}
}

func TestScanMarkdown_ListsAndNestedItems(t *testing.T) {
	t.Parallel()
	content := "" +
		"- top one\n" +
		"  - nested one\n" +
		"  - nested two\n" +
		"- top two\n" +
		"\n" +
		"1. first\n" +
		"2. second\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)
	assertSliceMatchesHash(t, content, segs)

	kinds := collectKinds(segs)
	if kinds[KindList] != 2 {
		t.Fatalf("expected 2 list containers (unordered + ordered), got %d (%v)", kinds[KindList], kinds)
	}
	if kinds[KindListItem] != 6 {
		t.Fatalf("expected 6 list_items, got %d", kinds[KindListItem])
	}
	for _, s := range segs {
		if s.Kind == KindListItem && s.Disposition != DispositionCanonicalAtom {
			t.Fatalf("list_item must be canonical_atom (non-empty), got %s for %q", s.Disposition, s.SegmentID)
		}
	}
}

func TestScanMarkdown_LinksAndImagesAsChildren(t *testing.T) {
	t.Parallel()
	content := "See [home](https://example.com) and ![logo](logo.png) here.\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)
	assertSliceMatchesHash(t, content, segs)

	kinds := collectKinds(segs)
	if kinds[KindLink] != 1 {
		t.Fatalf("expected 1 link, got %v", kinds)
	}
	if kinds[KindImageRef] != 1 {
		t.Fatalf("expected 1 image_ref, got %v", kinds)
	}
	for _, s := range segs {
		if s.Kind == KindLink || s.Kind == KindImageRef {
			if s.ParentSegmentID == "" {
				t.Fatalf("inline ref %q must have a parent paragraph", s.SegmentID)
			}
			if s.Disposition != DispositionCoverageOnly {
				t.Fatalf("inline %s must be coverage_only, got %s", s.Kind, s.Disposition)
			}
		}
	}
}

func TestScanMarkdown_BlockquoteAndCallout(t *testing.T) {
	t.Parallel()
	content := "" +
		"> a regular blockquote line.\n" +
		"> still in the blockquote.\n" +
		"\n" +
		"> [!NOTE]\n" +
		"> this is a callout body.\n" +
		"> across multiple lines.\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)
	assertSliceMatchesHash(t, content, segs)

	kinds := collectKinds(segs)
	if kinds[KindBlockquote] != 1 {
		t.Fatalf("expected 1 blockquote container, got %d (%v)", kinds[KindBlockquote], kinds)
	}
	if kinds[KindCallout] != 1 {
		t.Fatalf("expected 1 callout, got %d", kinds[KindCallout])
	}
	for _, s := range segs {
		switch s.Kind {
		case KindBlockquote:
			if s.Disposition != DispositionStructureOnly {
				t.Fatalf("blockquote must be structure_only, got %s", s.Disposition)
			}
		case KindCallout:
			if s.Disposition != DispositionCanonicalAtom {
				t.Fatalf("callout must be canonical_atom, got %s", s.Disposition)
			}
		}
	}
}

func TestScanMarkdown_FrontMatterIsExcludedByPolicy(t *testing.T) {
	t.Parallel()
	content := "---\ntitle: Doc\nauthor: Alice\n---\n\n# Body\n\nParagraph.\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)
	assertSliceMatchesHash(t, content, segs)

	meta, ok := firstOfKind(segs, KindMetadata)
	if !ok {
		t.Fatalf("no metadata segment emitted; got %v", collectKinds(segs))
	}
	if meta.Disposition != DispositionExcludedByPolicy {
		t.Fatalf("metadata must be excluded_by_policy, got %s", meta.Disposition)
	}
	if meta.StartByte != 0 {
		t.Fatalf("metadata must start at byte 0, got %d", meta.StartByte)
	}
}

func TestScanMarkdown_UnsupportedHTMLBlock(t *testing.T) {
	t.Parallel()
	content := "<div class=\"warn\">danger</div>\n\nplain paragraph.\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)
	assertSliceMatchesHash(t, content, segs)

	u, ok := firstOfKind(segs, KindUnsupportedBlock)
	if !ok {
		t.Fatalf("no unsupported_block emitted; got %v", collectKinds(segs))
	}
	if u.Disposition != DispositionUnsupportedBlocking {
		t.Fatalf("unsupported_block must be unsupported_blocking, got %s", u.Disposition)
	}
	if strings.TrimSpace(u.UnsupportedReason) == "" {
		t.Fatal("unsupported_block must carry a non-empty UnsupportedReason")
	}
}

func TestScanMarkdown_DeterministicOutput(t *testing.T) {
	t.Parallel()
	content := "" +
		"---\ntitle: Doc\n---\n\n" +
		"# H1\n\nBody paragraph.\n\n" +
		"- one\n- two\n\n" +
		"| A | B |\n|---|---|\n| 1 | 2 |\n\n" +
		"```go\nfmt.Println(\"x\")\n```\n\n" +
		"> [!TIP]\n> tip body.\n"
	a := scanOK(t, content)
	b := scanOK(t, content)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("scanner output is not deterministic across runs")
	}
	assertCoverageAndIntegrity(t, content, a)
	assertSliceMatchesHash(t, content, a)
}

func TestScanMarkdown_EmptyInput(t *testing.T) {
	t.Parallel()
	segs, err := ScanMarkdown(testSourceID, testSourcePath, nil)
	if err != nil {
		t.Fatalf("expected nil error on empty input, got %v", err)
	}
	if len(segs) != 0 {
		t.Fatalf("expected zero segments on empty input, got %d", len(segs))
	}
}

func TestScanMarkdown_RequiresSourceFields(t *testing.T) {
	t.Parallel()
	if _, err := ScanMarkdown("", "p.md", []byte("# h\n")); err == nil {
		t.Fatal("expected error when sourceID is empty")
	}
	if _, err := ScanMarkdown("src", "", []byte("# h\n")); err == nil {
		t.Fatal("expected error when sourcePath is empty")
	}
}

func TestScanMarkdown_DeterministicSegmentIDs(t *testing.T) {
	t.Parallel()
	content := "para a.\n\npara b.\n"
	segs := scanOK(t, content)
	for _, s := range segs {
		if !strings.HasPrefix(s.SegmentID, "seg:"+testSourceID+":") {
			t.Fatalf("segment id %q does not encode sourceID prefix", s.SegmentID)
		}
		if !strings.HasSuffix(s.SegmentID, ":"+s.Kind) {
			t.Fatalf("segment id %q does not end with kind %q", s.SegmentID, s.Kind)
		}
	}
}

func TestScanMarkdown_TableSeparatorNeverCanonical(t *testing.T) {
	t.Parallel()
	content := "" +
		"| h1 | h2 |\n" +
		"|----|----|\n" +
		"| a  | b  |\n"
	segs := scanOK(t, content)
	assertCoverageAndIntegrity(t, content, segs)
	for _, s := range segs {
		if s.Kind == KindTableSeparator && s.Disposition == DispositionCanonicalAtom {
			t.Fatalf("table_separator must never be canonical_atom; got %q", s.SegmentID)
		}
	}
}

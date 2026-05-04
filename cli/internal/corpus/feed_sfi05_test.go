package corpus

import (
	"strings"
	"testing"
)

// SFI-05 (#343): source-derived feed units must be produced from
// canonical_atom SourceSegments only, must carry segment linkage and
// the heading path, and feed generation must fail closed when the
// SFI-04 source integrity gate would have rejected the input.

const sfi05ManifestRule = `
schema_version: "0.1.0"
sources:
  - id: RBOK-RULE
    path: docs/rule.md
    type: markdown
    domain: rbok
    priority: primary
    status: active
    hash: "sha256:abc123"
    owner: Alice
    license: internal
    confidentiality: internal
    allowed_uses:
      - structured_contract
      - vector_index
`

func sfi05Feed(t *testing.T, body string) Feed {
	t.Helper()
	root := t.TempDir()
	writeFeedTestFile(t, root, "docs/rule.md", body)
	feed, err := GenerateFeed(FeedInput{
		ManifestYAML: []byte(sfi05ManifestRule),
		Root:         root,
		GeneratedAt:  fixedTime,
	})
	if err != nil {
		t.Fatalf("GenerateFeed: %v", err)
	}
	return feed
}

// 1. Headings + paragraphs: each canonical paragraph yields exactly one
//    feed unit; each unit carries SourceSegmentID, byte/line spans, and
//    the correct HeadingPath.
func TestSFI05FeedHeadingsAndParagraphs(t *testing.T) {
	feed := sfi05Feed(t, "# Rule A\nBody A\n\n## Rule B\nBody B\n")

	if feed.UnitCount != 2 {
		t.Fatalf("expected 2 feed units, got %d", feed.UnitCount)
	}

	u0 := feed.Units[0]
	if u0.SourceSegmentID == "" {
		t.Fatalf("unit 0 missing SourceSegmentID: %#v", u0)
	}
	if u0.SourceID != "RBOK-RULE" {
		t.Fatalf("unit 0 SourceID = %q, want RBOK-RULE", u0.SourceID)
	}
	if u0.SourcePath != "docs/rule.md" {
		t.Fatalf("unit 0 SourcePath = %q, want docs/rule.md", u0.SourcePath)
	}
	if u0.StartByte == 0 && u0.EndByte == 0 {
		t.Fatalf("unit 0 missing byte span: %#v", u0)
	}
	if u0.EndByte <= u0.StartByte {
		t.Fatalf("unit 0 EndByte %d must be > StartByte %d", u0.EndByte, u0.StartByte)
	}
	if u0.StartLine == 0 || u0.EndLine == 0 {
		t.Fatalf("unit 0 missing line span: %#v", u0)
	}
	if u0.NormalizedTextHash == "" {
		t.Fatalf("unit 0 missing NormalizedTextHash")
	}
	if len(u0.HeadingPath) == 0 || u0.HeadingPath[len(u0.HeadingPath)-1] != "Rule A" {
		t.Fatalf("unit 0 HeadingPath = %v, want last element 'Rule A'", u0.HeadingPath)
	}

	u1 := feed.Units[1]
	if len(u1.HeadingPath) == 0 || u1.HeadingPath[len(u1.HeadingPath)-1] != "Rule B" {
		t.Fatalf("unit 1 HeadingPath = %v, want last element 'Rule B'", u1.HeadingPath)
	}
	if u1.SourceSegmentID == u0.SourceSegmentID {
		t.Fatalf("units 0 and 1 share SourceSegmentID %q", u0.SourceSegmentID)
	}
	if u1.NormalizedTextHash == u0.NormalizedTextHash {
		t.Fatalf("distinct paragraphs collapsed to the same NormalizedTextHash")
	}
}

// 2. Markdown table: feed surfaces the data-row cell text but never the
//    "| --- | --- |" separator row.
func TestSFI05FeedExcludesTableSeparator(t *testing.T) {
	doc := "# Rules\n" +
		"| key | value |\n" +
		"|-----|-------|\n" +
		"| alpha | beta |\n"
	feed := sfi05Feed(t, doc)

	if feed.UnitCount == 0 {
		t.Fatalf("expected at least one feed unit, got 0")
	}
	for _, u := range feed.Units {
		body := strings.TrimSpace(u.BusinessRule)
		if body == "" {
			continue
		}
		if strings.Contains(body, "---") || strings.Contains(body, "-----") {
			t.Fatalf("feed unit %q contains table separator markers: %q", u.UnitID, body)
		}
	}
}

// 3. Decorative thematic breaks ("---" between paragraphs) never produce
//    a feed unit.
func TestSFI05FeedExcludesDecorativeSeparator(t *testing.T) {
	doc := "# Rules\n\nFirst paragraph.\n\n---\n\nSecond paragraph.\n"
	feed := sfi05Feed(t, doc)

	if feed.UnitCount != 2 {
		t.Fatalf("expected 2 paragraph feed units (separator excluded), got %d", feed.UnitCount)
	}
	for _, u := range feed.Units {
		if strings.TrimSpace(u.BusinessRule) == "---" {
			t.Fatalf("feed unit %q is the '---' decorative separator", u.UnitID)
		}
	}
}

// 4. Front-matter / metadata blocks and blank-line gaps never produce
//    feed units.
func TestSFI05FeedExcludesFrontMatterAndBlanks(t *testing.T) {
	doc := "---\ntitle: Spec\nowner: alice\n---\n\n# Rules\n\nOnly paragraph.\n"
	feed := sfi05Feed(t, doc)

	if feed.UnitCount != 1 {
		t.Fatalf("expected exactly 1 feed unit, got %d (units=%+v)", feed.UnitCount, feed.Units)
	}
	body := feed.Units[0].BusinessRule
	if strings.Contains(body, "title:") || strings.Contains(body, "owner:") {
		t.Fatalf("feed unit leaked front-matter content: %q", body)
	}
	if strings.TrimSpace(body) == "" {
		t.Fatalf("feed unit derived from blank line: %q", body)
	}
}

// 5. A synthetic canonical_atom whose raw text is "---" (i.e. junk) must
//    cause feed generation to fail. We exercise the helper directly so
//    we can construct the segment slice.
func TestSFI05FeedRejectsJunkCanonicalAtom(t *testing.T) {
	content := []byte("---\n")
	source := ManifestSource{ID: "S1", Path: "junk.md", Hash: "sha256:0", Domain: "rbok", Status: "active"}
	junkSeg := SourceSegment{
		SegmentID:          "seg:S1:0-4:paragraph",
		SourceID:           "S1",
		SourcePath:         "junk.md",
		Kind:               KindParagraph,
		Disposition:        DispositionCanonicalAtom,
		StartByte:          0,
		EndByte:            4,
		StartLine:          1,
		StartColumn:        1,
		EndLine:            1,
		EndColumn:          5,
		RawTextHash:        ComputeRawTextHash(content),
		NormalizedTextHash: ComputeNormalizedTextHash(string(content)),
		IncludeInFeed:      true,
	}
	report := CheckSourceIntegrity(
		[]SourceInput{{SourceID: "S1", Path: "junk.md", Content: content}},
		[]SourceSegment{junkSeg},
	)
	if report.Status == "pass" {
		t.Fatalf("integrity gate unexpectedly passed; cannot exercise SFI-05 fail-closed path")
	}

	_, err := feedUnitsFromSegments(content, []SourceSegment{junkSeg}, source, map[string]int{})
	if err == nil {
		t.Fatal("expected feed generation to error on junk canonical atom; got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, FindingSourceJunkSemantic) {
		t.Fatalf("error message missing %s code: %q", FindingSourceJunkSemantic, msg)
	}
}

// 6. A canonical_atom missing its hashes is an integrity-gate failure
//    and feed generation must surface it as an error.
func TestSFI05FeedRejectsCanonicalAtomMissingHash(t *testing.T) {
	content := []byte("Body text.\n")
	source := ManifestSource{ID: "S2", Path: "missing-hash.md", Hash: "sha256:0", Domain: "rbok", Status: "active"}
	bad := SourceSegment{
		SegmentID:   "seg:S2:0-11:paragraph",
		SourceID:    "S2",
		SourcePath:  "missing-hash.md",
		Kind:        KindParagraph,
		Disposition: DispositionCanonicalAtom,
		StartByte:   0,
		EndByte:     11,
		StartLine:   1,
		StartColumn: 1,
		EndLine:     1,
		EndColumn:   12,
		// RawTextHash + NormalizedTextHash intentionally empty.
		IncludeInFeed: true,
	}
	report := CheckSourceIntegrity(
		[]SourceInput{{SourceID: "S2", Path: "missing-hash.md", Content: content}},
		[]SourceSegment{bad},
	)
	if report.Status == "pass" {
		t.Fatalf("integrity gate unexpectedly passed for missing-hash canonical atom")
	}

	_, err := feedUnitsFromSegments(content, []SourceSegment{bad}, source, map[string]int{})
	if err == nil {
		t.Fatal("expected feed generation to error on canonical_atom missing hash; got nil")
	}
	if !strings.Contains(err.Error(), FindingSourceSegmentMissingHash) {
		t.Fatalf("error did not mention SOURCE_SEGMENT_MISSING_HASH: %q", err.Error())
	}
}

// 7. Source-derived feed units must always quote their canonical-atom
//    NormalizedTextHash (not the unrelated raw_text_hash).
func TestSFI05FeedUnitNormalizedHashMatchesSegment(t *testing.T) {
	feed := sfi05Feed(t, "# Rule A\nBody A\n")
	if feed.UnitCount != 1 {
		t.Fatalf("expected 1 feed unit, got %d", feed.UnitCount)
	}
	u := feed.Units[0]
	want := ComputeNormalizedTextHash("Body A")
	if u.NormalizedTextHash != want {
		t.Fatalf("NormalizedTextHash = %q, want %q (matching ComputeNormalizedTextHash of paragraph body)",
			u.NormalizedTextHash, want)
	}
}

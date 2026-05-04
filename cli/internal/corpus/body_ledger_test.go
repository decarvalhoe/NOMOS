package corpus

import (
	"encoding/json"
	"strings"
	"testing"
)

// FSQ-05 (#368): the corpus body ledger covers every byte of every
// admitted source; the curated feed (canonical_atom only) is a
// subordinate artifact. These tests pin the per-disposition byte
// accounting, the binary/reference handling, the deterministic
// JSON, and the feed-unit linkage.

const fsq05BodyLedgerNow = "2026-05-04T00:00:00Z"

func fsq05ScanFixture(t *testing.T, sourceID, sourcePath, body string) (BodyLedgerSourceInput, []byte) {
	t.Helper()
	content := []byte(body)
	segs, err := ScanMarkdown(sourceID, sourcePath, content)
	if err != nil {
		t.Fatalf("ScanMarkdown(%q): %v", sourcePath, err)
	}
	src := ManifestSource{
		ID:                sourceID,
		Path:              sourcePath,
		Type:              "markdown",
		Domain:            "rbok",
		Status:            "active",
		Hash:              "sha256:" + sourceID,
		AdmissionStatus:   AdmissionAdmitted,
		AtomizationStatus: AtomizationAtomized,
		SourceRole:        AdmissionRoleCanonical,
		FormatSupport:     FormatSupported,
	}
	return BodyLedgerSourceInput{Source: src, Content: content, Segments: segs}, content
}

// 1. Markdown source full coverage: a 2-paragraph markdown source
//    fully covers its bytes; semantic_bytes > 0; uncovered_bytes == 0.
func TestFSQ05BodyLedgerMarkdownFullCoverage(t *testing.T) {
	in, content := fsq05ScanFixture(t, "RULE", "docs/rule.md",
		"# Rule A\n\nFirst paragraph.\n\n# Rule B\n\nSecond paragraph.\n")
	ledger, err := BuildCorpusBodyLedger(BodyLedgerInput{
		CorpusRoot:  "/tmp/corpus",
		GeneratedAt: fsq05BodyLedgerNow,
		Sources:     []BodyLedgerSourceInput{in},
	})
	if err != nil {
		t.Fatalf("BuildCorpusBodyLedger: %v", err)
	}

	if ledger.Format != BodyLedgerFormat {
		t.Fatalf("Format = %q, want %q", ledger.Format, BodyLedgerFormat)
	}
	if ledger.SourceCount != 1 || ledger.AdmittedCount != 1 {
		t.Fatalf("source/admitted counts = %d/%d, want 1/1", ledger.SourceCount, ledger.AdmittedCount)
	}
	row := ledger.Sources[0]
	if row.SizeBytes != int64(len(content)) {
		t.Fatalf("SizeBytes = %d, want %d", row.SizeBytes, len(content))
	}
	if row.ByteCoverage.UncoveredBytes != 0 {
		t.Fatalf("UncoveredBytes = %d, want 0; coverage=%+v", row.ByteCoverage.UncoveredBytes, row.ByteCoverage)
	}
	if row.ByteCoverage.SemanticBytes == 0 {
		t.Fatalf("SemanticBytes = 0; expected paragraph bytes; coverage=%+v", row.ByteCoverage)
	}
	if ledger.CoverageSummary.UncoveredBytes != 0 {
		t.Fatalf("summary UncoveredBytes = %d, want 0", ledger.CoverageSummary.UncoveredBytes)
	}
	if ledger.CoverageSummary.BySourceRole[AdmissionRoleCanonical] == 0 {
		t.Fatalf("BySourceRole[canonical] = 0; expected = SizeBytes")
	}
}

// 2. Mixed admitted text + reference: both appear in the ledger; the
//    .pdf has zero segments and BinaryBytes == size; covers_full body
//    holds because no admitted text source has uncovered bytes.
func TestFSQ05BodyLedgerMixedTextAndReference(t *testing.T) {
	textIn, _ := fsq05ScanFixture(t, "RULE", "docs/rule.md",
		"# Rule\n\nBody paragraph.\n")
	pdfIn := BodyLedgerSourceInput{
		Source: ManifestSource{
			ID:                "REF",
			Path:              "refs/spec.pdf",
			Type:              "pdf",
			Hash:              "sha256:pdf",
			AdmissionStatus:   AdmissionAdmitted,
			AtomizationStatus: AtomizationUnsupported,
			SourceRole:        AdmissionRoleReference,
			FormatSupport:     FormatUnsupported,
			ExclusionReason:   "format not yet supported",
		},
		Content:   nil,
		Segments:  nil,
		SizeBytes: 12345,
	}
	ledger, err := BuildCorpusBodyLedger(BodyLedgerInput{
		GeneratedAt: fsq05BodyLedgerNow,
		Sources:     []BodyLedgerSourceInput{textIn, pdfIn},
	})
	if err != nil {
		t.Fatalf("BuildCorpusBodyLedger: %v", err)
	}
	if ledger.SourceCount != 2 || ledger.AdmittedCount != 2 {
		t.Fatalf("counts = %d/%d, want 2/2", ledger.SourceCount, ledger.AdmittedCount)
	}
	pdf := ledger.Sources[1]
	if pdf.ByteCoverage.BinaryBytes != 0 {
		t.Fatalf("PDF BinaryBytes = %d; expected 0 (admitted+unsupported -> UnsupportedBytes)", pdf.ByteCoverage.BinaryBytes)
	}
	if pdf.ByteCoverage.UnsupportedBytes != 12345 {
		t.Fatalf("PDF UnsupportedBytes = %d, want 12345", pdf.ByteCoverage.UnsupportedBytes)
	}
	if len(pdf.Segments) != 0 {
		t.Fatalf("PDF Segments = %d, want 0", len(pdf.Segments))
	}
	if ledger.CoverageSummary.UncoveredBytes != 0 {
		t.Fatalf("summary UncoveredBytes = %d, want 0", ledger.CoverageSummary.UncoveredBytes)
	}
	cc := computeClaimCoverage(ledger, 1)
	if !cc.CoversFullSourceBody {
		t.Fatalf("CoversFullSourceBody = false; expected true (no uncovered text bytes)")
	}
	if cc.SummaryStatus != "feed_and_body" {
		t.Fatalf("SummaryStatus = %q, want feed_and_body", cc.SummaryStatus)
	}
}

// 3. Uncovered text bytes: drop a segment so its byte range is not
//    covered → UncoveredBytes > 0 and CoversFullSourceBody == false.
func TestFSQ05BodyLedgerUncoveredTextBytes(t *testing.T) {
	in, _ := fsq05ScanFixture(t, "RULE", "docs/rule.md",
		"# Rule\n\nFirst.\n\n# Other\n\nSecond.\n")
	if len(in.Segments) < 2 {
		t.Fatalf("need at least 2 root-level segments; got %d", len(in.Segments))
	}
	// Drop one root-level segment to simulate scanner gap.
	var trimmed []SourceSegment
	dropped := false
	for _, s := range in.Segments {
		if !dropped && s.ParentSegmentID == "" && s.EndByte > s.StartByte {
			dropped = true
			continue
		}
		trimmed = append(trimmed, s)
	}
	in.Segments = trimmed

	ledger, err := BuildCorpusBodyLedger(BodyLedgerInput{
		GeneratedAt: fsq05BodyLedgerNow,
		Sources:     []BodyLedgerSourceInput{in},
	})
	if err != nil {
		t.Fatalf("BuildCorpusBodyLedger: %v", err)
	}
	if ledger.Sources[0].ByteCoverage.UncoveredBytes <= 0 {
		t.Fatalf("UncoveredBytes = 0; expected > 0 after dropping a root-level segment; coverage=%+v",
			ledger.Sources[0].ByteCoverage)
	}
	if ledger.CoverageSummary.UncoveredBytes <= 0 {
		t.Fatalf("summary UncoveredBytes = %d, want > 0", ledger.CoverageSummary.UncoveredBytes)
	}
	cc := computeClaimCoverage(ledger, 1)
	if cc.CoversFullSourceBody {
		t.Fatalf("CoversFullSourceBody = true; expected false")
	}
	if cc.SummaryStatus != "feed_only" {
		t.Fatalf("SummaryStatus = %q, want feed_only", cc.SummaryStatus)
	}
}

// 4. Excluded source: no segments expected; bytes counted under
//    BySourceStatus["excluded"] only.
func TestFSQ05BodyLedgerExcludedSource(t *testing.T) {
	excluded := BodyLedgerSourceInput{
		Source: ManifestSource{
			ID:              "DRAFT",
			Path:            "drafts/wip.md",
			Type:            "markdown",
			AdmissionStatus: AdmissionExcluded,
			SourceRole:      AdmissionRoleCanonical,
			FormatSupport:   FormatSupported,
			ExclusionReason: "not yet ready for canonical review",
		},
		Content:   nil,
		Segments:  nil,
		SizeBytes: 4096,
	}
	ledger, err := BuildCorpusBodyLedger(BodyLedgerInput{
		GeneratedAt: fsq05BodyLedgerNow,
		Sources:     []BodyLedgerSourceInput{excluded},
	})
	if err != nil {
		t.Fatalf("BuildCorpusBodyLedger: %v", err)
	}
	if ledger.AdmittedCount != 0 {
		t.Fatalf("AdmittedCount = %d, want 0", ledger.AdmittedCount)
	}
	row := ledger.Sources[0]
	if row.ByteCoverage.BinaryBytes != 4096 {
		t.Fatalf("excluded source BinaryBytes = %d, want 4096", row.ByteCoverage.BinaryBytes)
	}
	if got := ledger.CoverageSummary.BySourceStatus[AdmissionExcluded]; got != 4096 {
		t.Fatalf("BySourceStatus[excluded] = %d, want 4096", got)
	}
	if got := ledger.CoverageSummary.BySourceStatus[AdmissionAdmitted]; got != 0 {
		t.Fatalf("BySourceStatus[admitted] = %d, want 0", got)
	}
}

// 5. Feed unit linkage: a feed unit derived from a paragraph segment
//    must carry BodyLedgerSegmentIDs = [SourceSegmentID].
func TestFSQ05FeedUnitLinkage(t *testing.T) {
	root := t.TempDir()
	writeFeedTestFile(t, root, "docs/rule.md", "# Rule\n\nBody paragraph.\n")
	feed, err := GenerateFeed(FeedInput{
		ManifestYAML: []byte(sfi05ManifestRule),
		Root:         root,
		GeneratedAt:  fixedTime,
	})
	if err != nil {
		t.Fatalf("GenerateFeed: %v", err)
	}
	if feed.UnitCount != 1 {
		t.Fatalf("UnitCount = %d, want 1", feed.UnitCount)
	}
	u := feed.Units[0]
	if u.SourceSegmentID == "" {
		t.Fatalf("expected SourceSegmentID to be set: %#v", u)
	}
	if len(u.BodyLedgerSegmentIDs) != 1 || u.BodyLedgerSegmentIDs[0] != u.SourceSegmentID {
		t.Fatalf("BodyLedgerSegmentIDs = %v, want [%s]", u.BodyLedgerSegmentIDs, u.SourceSegmentID)
	}
}

// 5b. Composed table_row feed unit: BodyLedgerSegmentIDs lists the row
//     segment plus its child table_cell segments.
func TestFSQ05FeedUnitTableRowLinkage(t *testing.T) {
	root := t.TempDir()
	doc := "# Tarif\n" +
		"| Offre | Prix |\n" +
		"|-------|------|\n" +
		"| Programme Lancement | 980 CHF |\n"
	writeFeedTestFile(t, root, "docs/tarif.md", doc)
	feed, err := GenerateFeed(FeedInput{
		ManifestYAML: []byte(strings.Replace(sfi05ManifestRule,
			"path: docs/rule.md", "path: docs/tarif.md", 1)),
		Root:        root,
		GeneratedAt: fixedTime,
	})
	if err != nil {
		t.Fatalf("GenerateFeed: %v", err)
	}
	var rowUnit *FeedUnit
	for i := range feed.Units {
		if feed.Units[i].UnitType == "table_row" {
			rowUnit = &feed.Units[i]
			break
		}
	}
	if rowUnit == nil {
		t.Fatalf("no table_row feed unit produced; feed.Units=%+v", feed.Units)
	}
	if len(rowUnit.BodyLedgerSegmentIDs) < 2 {
		t.Fatalf("table_row BodyLedgerSegmentIDs = %v; expected row id + at least one cell id",
			rowUnit.BodyLedgerSegmentIDs)
	}
	if rowUnit.BodyLedgerSegmentIDs[0] != rowUnit.SourceSegmentID {
		t.Fatalf("first BodyLedgerSegmentID %q must equal SourceSegmentID %q",
			rowUnit.BodyLedgerSegmentIDs[0], rowUnit.SourceSegmentID)
	}
}

// 6. Determinism: building the ledger twice with the same inputs and
//    GeneratedAt produces byte-identical JSON.
func TestFSQ05BodyLedgerDeterministic(t *testing.T) {
	in, _ := fsq05ScanFixture(t, "RULE", "docs/rule.md",
		"# Rule A\n\nFirst paragraph.\n\n## Rule B\n\nSecond paragraph.\n")
	build := func() []byte {
		l, err := BuildCorpusBodyLedger(BodyLedgerInput{
			CorpusRoot:  "/tmp/corpus",
			GeneratedAt: "TEST",
			Sources:     []BodyLedgerSourceInput{in},
		})
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		out, err := MarshalCorpusBodyLedger(l)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return out
	}
	first := build()
	second := build()
	if string(first) != string(second) {
		t.Fatalf("non-deterministic output:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// 7. Attestation claim coverage: when a body ledger with zero uncovered
//    bytes is supplied, the predicate carries CoversFullSourceBody=true.
//    When no body ledger is supplied, ClaimCoverage is omitted.
func TestFSQ05AttestationClaimCoverage(t *testing.T) {
	in, _ := fsq05ScanFixture(t, "RULE", "docs/rule.md",
		"# Rule\n\nBody.\n")
	ledger, err := BuildCorpusBodyLedger(BodyLedgerInput{
		GeneratedAt: fsq05BodyLedgerNow,
		Sources:     []BodyLedgerSourceInput{in},
	})
	if err != nil {
		t.Fatalf("BuildCorpusBodyLedger: %v", err)
	}

	stmtWith, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID:       "rbok-poc",
		ProjectID:      "rbok",
		ScannerVersion: "test",
		Verdict:        VerdictAdmissible,
		FilesScanned:   1,
		UnitsExtracted: 1,
		ScannedFiles:   []string{"docs/rule.md"},
		Now:            attestNow,
		BodyLedger:     &ledger,
	})
	if err != nil {
		t.Fatalf("attest with ledger: %v", err)
	}
	var pWith CorpusPredicate
	if err := json.Unmarshal(stmtWith.Predicate, &pWith); err != nil {
		t.Fatalf("unmarshal predicate: %v", err)
	}
	if pWith.ClaimCoverage == nil {
		t.Fatalf("ClaimCoverage missing; expected populated when body ledger supplied")
	}
	if !pWith.ClaimCoverage.CoversFullSourceBody {
		t.Fatalf("CoversFullSourceBody = false, want true")
	}
	if !pWith.ClaimCoverage.CoversCuratedFeed {
		t.Fatalf("CoversCuratedFeed = false, want true")
	}
	if pWith.ClaimCoverage.SummaryStatus != "feed_and_body" {
		t.Fatalf("SummaryStatus = %q, want feed_and_body", pWith.ClaimCoverage.SummaryStatus)
	}

	stmtWithout, err := GenerateCorpusAttestation(CorpusAttestationOptions{
		CorpusID:       "rbok-poc",
		ProjectID:      "rbok",
		ScannerVersion: "test",
		Verdict:        VerdictAdmissible,
		FilesScanned:   1,
		UnitsExtracted: 1,
		ScannedFiles:   []string{"docs/rule.md"},
		Now:            attestNow,
	})
	if err != nil {
		t.Fatalf("attest without ledger: %v", err)
	}
	var pWithout CorpusPredicate
	if err := json.Unmarshal(stmtWithout.Predicate, &pWithout); err != nil {
		t.Fatalf("unmarshal predicate: %v", err)
	}
	if pWithout.ClaimCoverage != nil {
		t.Fatalf("ClaimCoverage = %+v; expected nil when no body ledger supplied", pWithout.ClaimCoverage)
	}
	// Verify the JSON itself omits the key (omitempty).
	if strings.Contains(string(stmtWithout.Predicate), "claim_coverage") {
		t.Fatalf("predicate JSON unexpectedly contains claim_coverage: %s", string(stmtWithout.Predicate))
	}
}

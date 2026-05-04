package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

var frozenAuditTime = time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)

func auditConfig(feedPath, corpusRoot string) FeedAuditConfig {
	return FeedAuditConfig{
		FeedPath:    feedPath,
		CorpusRoot:  corpusRoot,
		GeneratedAt: frozenAuditTime,
	}
}

// ----------------------------------------------------------------------------
// Empty feed: every counter is 0, the report serialises cleanly.
// ----------------------------------------------------------------------------

func TestRunAudit_EmptyFeed(t *testing.T) {
	t.Parallel()
	report := RunAudit(corpus.Feed{}, nil, auditConfig("/tmp/empty.json", ""))
	if report.SchemaVersion != FeedAuditSchemaVersion {
		t.Fatalf("expected schema_version %q, got %q", FeedAuditSchemaVersion, report.SchemaVersion)
	}
	if report.Totals.FeedUnitCount != 0 || report.Totals.ChunkCount != 0 {
		t.Fatalf("expected zero counters, got %+v", report.Totals)
	}
	if report.RAGPath != nil {
		t.Fatalf("expected rag_path null, got %v", *report.RAGPath)
	}
	if report.CorpusRoot != nil {
		t.Fatalf("expected corpus_root null, got %v", *report.CorpusRoot)
	}
	if _, err := json.Marshal(report); err != nil {
		t.Fatalf("empty report must serialise cleanly: %v", err)
	}
}

// ----------------------------------------------------------------------------
// Clean fixture: 5 distinct paragraphs, all source-backed, no offenders.
// ----------------------------------------------------------------------------

func cleanUnit(idx int, text string) corpus.FeedUnit {
	return corpus.FeedUnit{
		UnitID:             "RBOK-CLEAN-" + string(rune('A'+idx)),
		Name:               "Clean " + string(rune('A'+idx)),
		Domain:             "rbok",
		UnitType:           "rule",
		BusinessRule:       text,
		SourceIDs:          []string{"SRC-CLEAN"},
		SourceSegmentID:    fakeSegmentID(idx, "paragraph"),
		SourceID:           "SRC-CLEAN",
		SourcePath:         "docs/clean.md",
		StartByte:          10 + idx*100,
		EndByte:            10 + idx*100 + len(text),
		StartLine:          2 + idx*5,
		EndLine:            2 + idx*5,
		NormalizedTextHash: "norm-clean-" + string(rune('A'+idx)),
	}
}

func fakeSegmentID(idx int, kind string) string {
	return "seg:SRC-CLEAN:" + intToStr(10+idx*100) + "-" + intToStr(10+idx*100+50) + ":" + kind
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

func TestRunAudit_CleanFixture(t *testing.T) {
	t.Parallel()
	feed := corpus.Feed{
		Sources: []corpus.FeedSource{
			{ID: "SRC-CLEAN", Path: "docs/clean.md", Status: "active"},
		},
		Units: []corpus.FeedUnit{
			cleanUnit(0, "First clean paragraph that is meaningful and long enough to keep."),
			cleanUnit(1, "Second clean paragraph in another section, fully distinct."),
			cleanUnit(2, "Third clean paragraph, clearly different content."),
			cleanUnit(3, "Fourth clean paragraph, again distinct from the rest."),
			cleanUnit(4, "Fifth clean paragraph that closes the fixture."),
		},
	}
	report := RunAudit(feed, nil, auditConfig("/tmp/clean.json", ""))

	if report.DuplicateNormalizedText.DuplicatedUnitCount != 0 {
		t.Fatalf("expected zero duplicates, got %d", report.DuplicateNormalizedText.DuplicatedUnitCount)
	}
	if report.TableCellRatio.Ratio != 0 {
		t.Fatalf("expected table_cell ratio 0, got %v", report.TableCellRatio.Ratio)
	}
	if report.TableCellRatio.TableCellUnitCount != 0 {
		t.Fatalf("expected zero table_cell units, got %d", report.TableCellRatio.TableCellUnitCount)
	}
	if report.Totals.SourceBackedUnitCount != 5 {
		t.Fatalf("expected 5 source-backed units, got %d", report.Totals.SourceBackedUnitCount)
	}
	if report.Totals.SourcesWithZeroUnits != 0 {
		t.Fatalf("expected zero zero-unit sources, got %d", report.Totals.SourcesWithZeroUnits)
	}
	if got := report.UnitKindDistribution["paragraph"]; got != 5 {
		t.Fatalf("expected 5 paragraph units, got %d (dist=%v)", got, report.UnitKindDistribution)
	}
}

// ----------------------------------------------------------------------------
// Noisy fixture: synthetic units exercising every metric. Production-scale
// numbers (9500 / 3230 / 3344 / 2195 / 3704) are the audit-evidence target
// shape from the FSQ epic; a small synthetic corpus reproduces every metric
// type without needing the full 9500 rows.
// ----------------------------------------------------------------------------

func TestRunAudit_NoisyFixture(t *testing.T) {
	t.Parallel()
	feed := buildNoisyFeed()
	chunks := buildNoisyChunks()

	report := RunAudit(feed, chunks, auditConfig("/tmp/noisy.json", ""))

	if report.Totals.FeedUnitCount == 0 {
		t.Fatal("expected non-zero feed_unit_count")
	}
	if report.Totals.ChunkCount == 0 {
		t.Fatal("expected non-zero chunk_count")
	}
	if report.TableCellRatio.TableCellUnitCount == 0 {
		t.Fatal("expected at least one table_cell unit")
	}
	if report.TableCellRatio.Ratio <= 0 || report.TableCellRatio.Ratio > 1 {
		t.Fatalf("ratio out of range: %v", report.TableCellRatio.Ratio)
	}
	if report.LengthDistribution.Tokens.Le2 == 0 {
		t.Fatal("expected at least one ≤2-token unit")
	}
	if report.LengthDistribution.Characters.Le10 == 0 {
		t.Fatal("expected at least one ≤10-char unit")
	}
	if report.DuplicateNormalizedText.GroupCount == 0 {
		t.Fatal("expected at least one duplicate group")
	}
	if report.DuplicateNormalizedText.DuplicatedUnitCount == 0 {
		t.Fatal("expected non-zero duplicated unit count")
	}
	if len(report.TopOffenders.VeryShortUnits) == 0 {
		t.Fatal("expected very_short_units examples")
	}
	if len(report.TopOffenders.DuplicatedUnits) == 0 {
		t.Fatal("expected duplicated_units examples")
	}
	if got := report.UnitKindDistribution["table_cell"]; got == 0 {
		t.Fatalf("expected table_cell in unit_kind_distribution, got %v", report.UnitKindDistribution)
	}
	if got := report.ChunkKindDistribution["table_cell"]; got == 0 {
		t.Fatalf("expected table_cell in chunk_kind_distribution, got %v", report.ChunkKindDistribution)
	}
}

func buildNoisyFeed() corpus.Feed {
	var units []corpus.FeedUnit
	// 5 table_cell units, 2 of which share the same normalized hash (duplicate).
	for i := 0; i < 5; i++ {
		text := "x"
		hash := "norm-cell-" + intToStr(i)
		if i >= 3 {
			hash = "norm-cell-dup"
			text = "shared"
		}
		units = append(units, corpus.FeedUnit{
			UnitID:             "RBOK-CELL-" + intToStr(i),
			Domain:             "rbok",
			UnitType:           "rule",
			BusinessRule:       text,
			SourceIDs:          []string{"SRC-NOISY"},
			SourceSegmentID:    "seg:SRC-NOISY:" + intToStr(i*10) + "-" + intToStr(i*10+1) + ":table_cell",
			SourceID:           "SRC-NOISY",
			SourcePath:         "docs/noisy.md",
			StartByte:          i * 10,
			EndByte:            i*10 + 1,
			StartLine:          1 + i,
			EndLine:            1 + i,
			NormalizedTextHash: hash,
		})
	}
	// 3 short paragraphs (≤2 tokens, ≤10 chars).
	for i := 0; i < 3; i++ {
		units = append(units, corpus.FeedUnit{
			UnitID:             "RBOK-SHORT-" + intToStr(i),
			Domain:             "rbok",
			UnitType:           "rule",
			BusinessRule:       "ok " + intToStr(i),
			SourceIDs:          []string{"SRC-NOISY"},
			SourceSegmentID:    "seg:SRC-NOISY:" + intToStr(100+i*5) + "-" + intToStr(100+i*5+4) + ":paragraph",
			SourceID:           "SRC-NOISY",
			SourcePath:         "docs/noisy.md",
			StartByte:          100 + i*5,
			EndByte:            100 + i*5 + 4,
			StartLine:          20 + i,
			EndLine:            20 + i,
			NormalizedTextHash: "norm-short-" + intToStr(i),
		})
	}
	// 2 long paragraphs.
	for i := 0; i < 2; i++ {
		long := "This is a longer paragraph that exceeds the very-short bucket and provides meaningful semantic content for retrieval."
		units = append(units, corpus.FeedUnit{
			UnitID:             "RBOK-LONG-" + intToStr(i),
			Domain:             "rbok",
			UnitType:           "rule",
			BusinessRule:       long,
			SourceIDs:          []string{"SRC-NOISY"},
			SourceSegmentID:    "seg:SRC-NOISY:" + intToStr(200+i*200) + "-" + intToStr(200+i*200+len(long)) + ":paragraph",
			SourceID:           "SRC-NOISY",
			SourcePath:         "docs/noisy.md",
			StartByte:          200 + i*200,
			EndByte:            200 + i*200 + len(long),
			StartLine:          40 + i,
			EndLine:            40 + i,
			NormalizedTextHash: "norm-long-" + intToStr(i),
		})
	}
	// 1 declared source that produces zero units.
	return corpus.Feed{
		Sources: []corpus.FeedSource{
			{ID: "SRC-NOISY", Path: "docs/noisy.md", Status: "active"},
			{ID: "SRC-ORPHAN", Path: "docs/orphan.md", Status: "active"},
		},
		Units: units,
	}
}

func buildNoisyChunks() []corpus.ChunkMetadata {
	var chunks []corpus.ChunkMetadata
	for i := 0; i < 3; i++ {
		chunks = append(chunks, corpus.ChunkMetadata{
			ChunkID:         "chunk:SRC-NOISY:" + intToStr(i*10) + "-" + intToStr(i*10+1),
			SourceID:        "SRC-NOISY",
			SourceSegmentID: "seg:SRC-NOISY:" + intToStr(i*10) + "-" + intToStr(i*10+1) + ":table_cell",
			SegmentKind:     "table_cell",
		})
	}
	chunks = append(chunks, corpus.ChunkMetadata{
		ChunkID:         "chunk:SRC-NOISY:200-300",
		SourceID:        "SRC-NOISY",
		SourceSegmentID: "seg:SRC-NOISY:200-300:paragraph",
		SegmentKind:     "paragraph",
	})
	return chunks
}

// ----------------------------------------------------------------------------
// Determinism: byte-identical JSON across two runs with --frozen-time.
// ----------------------------------------------------------------------------

func TestRunAudit_Deterministic(t *testing.T) {
	t.Parallel()
	feed := buildNoisyFeed()
	chunks := buildNoisyChunks()
	cfg := auditConfig("/tmp/noisy.json", "")

	a := RunAudit(feed, chunks, cfg)
	b := RunAudit(feed, chunks, cfg)

	aj, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		t.Fatalf("marshal a: %v", err)
	}
	bj, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		t.Fatalf("marshal b: %v", err)
	}
	if !bytes.Equal(aj, bj) {
		t.Fatalf("audit output is not byte-deterministic across runs")
	}
}

// ----------------------------------------------------------------------------
// Source-coverage: walks the corpus dir and produces per-extension stats.
// ----------------------------------------------------------------------------

func TestRunAudit_SourceCoverageWithCorpusDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "docs", "covered.md")
	if err := os.MkdirAll(filepath.Dir(mdPath), 0o755); err != nil {
		t.Fatal(err)
	}
	mdBytes := []byte("# Title\n\nBody paragraph for coverage measurement.\n")
	if err := os.WriteFile(mdPath, mdBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	orphanPath := filepath.Join(dir, "docs", "orphan.md")
	if err := os.WriteFile(orphanPath, []byte("not referenced"), 0o644); err != nil {
		t.Fatal(err)
	}

	feed := corpus.Feed{
		Sources: []corpus.FeedSource{{ID: "SRC", Path: "docs/covered.md", Status: "active"}},
		Units: []corpus.FeedUnit{
			{
				UnitID:             "U-1",
				Domain:             "rbok",
				BusinessRule:       "Body paragraph for coverage measurement.",
				SourceIDs:          []string{"SRC"},
				SourceSegmentID:    "seg:SRC:9-50:paragraph",
				SourceID:           "SRC",
				SourcePath:         "docs/covered.md",
				StartByte:          9,
				EndByte:            50,
				StartLine:          3,
				EndLine:            3,
				NormalizedTextHash: "norm-1",
			},
		},
	}

	report := RunAudit(feed, nil, FeedAuditConfig{
		FeedPath:    "/tmp/feed.json",
		CorpusRoot:  dir,
		GeneratedAt: frozenAuditTime,
	})

	cov, ok := report.SourceCoverage.ByExtension[".md"]
	if !ok {
		t.Fatalf(".md extension missing from coverage; got %v", report.SourceCoverage.ByExtension)
	}
	if cov.Sources != 2 {
		t.Fatalf("expected 2 .md sources on disk, got %d", cov.Sources)
	}
	if cov.WithUnits != 1 {
		t.Fatalf("expected 1 .md source with units, got %d", cov.WithUnits)
	}
	if cov.ByteCoveragePct == nil || *cov.ByteCoveragePct <= 0 {
		t.Fatalf("expected non-nil positive byte_coverage_pct, got %v", cov.ByteCoveragePct)
	}

	// orphan.md must show up in zero-unit list.
	var sawOrphan bool
	for _, z := range report.SourceCoverage.SourcesWithZeroUnits {
		if z.Path == "docs/orphan.md" {
			sawOrphan = true
		}
	}
	if !sawOrphan {
		t.Fatalf("expected docs/orphan.md in sources_with_zero_units, got %+v",
			report.SourceCoverage.SourcesWithZeroUnits)
	}
}

// ----------------------------------------------------------------------------
// cmd-layer: malformed JSON exits 1, missing required flags exits 2.
// ----------------------------------------------------------------------------

func TestRun_MalformedFeedJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	feedPath := filepath.Join(dir, "feed.json")
	outPath := filepath.Join(dir, "audit.json")
	if err := os.WriteFile(feedPath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--feed", feedPath, "--out", outPath, "--frozen-time", "2026-05-04T12:00:00Z"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit 1 on malformed JSON, got %d (stderr=%q)", code, stderr.String())
	}
}

func TestRun_MissingRequiredFlags(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2 on missing flags, got %d", code)
	}
}

// ----------------------------------------------------------------------------
// cmd-layer: end-to-end success with --frozen-time produces deterministic JSON.
// ----------------------------------------------------------------------------

func TestRun_EndToEndDeterministic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	feedPath := filepath.Join(dir, "feed.json")
	outA := filepath.Join(dir, "a.json")
	outB := filepath.Join(dir, "b.json")

	feed := buildNoisyFeed()
	feed.RAGMetadata = buildNoisyChunks()
	feedBytes, err := json.MarshalIndent(feed, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(feedPath, feedBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	args := []string{
		"--feed", feedPath,
		"--out", outA,
		"--frozen-time", "2026-05-04T12:00:00Z",
	}
	if code := run(args, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("first run exit %d", code)
	}
	args[3] = outB
	if code := run(args, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("second run exit %d", code)
	}

	a, err := os.ReadFile(outA)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(outB)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("two runs of feed-audit on the same input produced different JSON")
	}
}

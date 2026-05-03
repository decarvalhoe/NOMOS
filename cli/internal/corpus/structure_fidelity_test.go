package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStructureFidelityReportPassesCoveredMarkdownBlocks(t *testing.T) {
	dir := t.TempDir()
	rel := "01_referentiel/doc.md"
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	source := "# Doc\n\n| Reference | DOC-1 |\n| --- | --- |\n\nIntro.\n\n| A | B |\n| --- | --- |\n| C | D |\n\n```go\nfmt.Println(\"x\")\n```\n\n> [!NOTE]\n> Note.\n"
	if err := os.WriteFile(abs, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	feed := normalizedMarkdownTestFeed(t, rel, source, abs)
	assembly := AssembleMultiFeed([]LawbookFeed{feed}, MultiAssembleOptions{Now: time.Date(2026, 5, 3, 9, 0, 0, 0, time.UTC)})

	report := BuildStructureFidelityReport("portable-test", "2026-05-03T09:00:00Z", dir, []RBOKSourceClassification{
		ClassifyRBOKSource(rel),
	}, assembly)

	if report.Blocking != 0 {
		t.Fatalf("expected no blocking findings, got %+v", report.Findings)
	}
	if report.CheckedSourceCount != 1 {
		t.Fatalf("expected 1 checked source, got %d", report.CheckedSourceCount)
	}
	if report.SourceBlockCount == 0 {
		t.Fatal("expected source blocks")
	}
	if report.CoveredSourceBlockCount != report.SourceBlockCount {
		t.Fatalf("expected full block coverage, got %d/%d", report.CoveredSourceBlockCount, report.SourceBlockCount)
	}
	if report.UnsupportedBlockCount != 0 {
		t.Fatalf("expected no unsupported blocks, got %d", report.UnsupportedBlockCount)
	}
}

func TestStructureFidelityReportBlocksMissingNodeSpan(t *testing.T) {
	dir := t.TempDir()
	rel := "01_referentiel/doc.md"
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	source := "# Doc\n\nGoverned paragraph.\n"
	if err := os.WriteFile(abs, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	feed := normalizedMarkdownTestFeed(t, rel, source, abs)
	feed.Nodes[1].SourceSpan = nil
	assembly := AssembleMultiFeed([]LawbookFeed{feed}, MultiAssembleOptions{})

	report := BuildStructureFidelityReport("portable-test", "2026-05-03T09:00:00Z", dir, []RBOKSourceClassification{
		ClassifyRBOKSource(rel),
	}, assembly)

	if report.Blocking == 0 {
		t.Fatalf("expected blocking finding, got %+v", report)
	}
	if !hasStructureFinding(report, "node.missing_source_span") {
		t.Fatalf("expected node.missing_source_span finding, got %+v", report.Findings)
	}
}

func TestStructureFidelityReportBlocksUncoveredSourceBlock(t *testing.T) {
	dir := t.TempDir()
	rel := "01_referentiel/doc.md"
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	source := "# Doc\n\nFirst paragraph.\n\nSecond paragraph.\n"
	if err := os.WriteFile(abs, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	feed := normalizedMarkdownTestFeed(t, rel, source, abs)
	feed.Nodes = feed.Nodes[:3]
	assembly := AssembleMultiFeed([]LawbookFeed{feed}, MultiAssembleOptions{})

	report := BuildStructureFidelityReport("portable-test", "2026-05-03T09:00:00Z", dir, []RBOKSourceClassification{
		ClassifyRBOKSource(rel),
	}, assembly)

	if report.Blocking == 0 {
		t.Fatalf("expected blocking finding, got %+v", report)
	}
	if !hasStructureFinding(report, "source_block.uncovered") {
		t.Fatalf("expected source_block.uncovered finding, got %+v", report.Findings)
	}
}

func TestStructureFidelityReportBlocksMissingStructuredSourceSpan(t *testing.T) {
	dir := t.TempDir()
	rel := "03_parcours/parcours.yaml"
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	source := "parcours:\n  code: PAR_TEST\n  modules: []\n"
	if err := os.WriteFile(abs, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, _, err := hashFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	feed := LawbookFeed{
		SchemaVersion: "0.1.0",
		FeedID:        "structured-feed",
		DocumentID:    "STRUCTURED-DOC",
		Domain:        "test",
		GeneratedAt:   "2026-05-03T09:00:00Z",
		SourcePath:    rel,
		SourceHash:    "sha256:" + hash,
		NodeCount:     1,
		Nodes: []LawbookNode{
			{
				NodeID:       "STRUCTURED-DOC",
				DocumentID:   "STRUCTURED-DOC",
				NodeType:     NodeDocument,
				CanonicalRef: "test/structured",
				DisplayRef:   "structured",
				Depth:        NodeDocument.Depth(),
				OrdinalPath:  "1",
				SourcePath:   rel,
				SourceHash:   "sha256:" + hash,
				Status:       StatusActive,
				Priority:     PriorityHigh,
				Domain:       "test",
			},
		},
	}
	assembly := AssembleMultiFeed([]LawbookFeed{feed}, MultiAssembleOptions{})

	report := BuildStructureFidelityReport("portable-test", "2026-05-03T09:00:00Z", dir, []RBOKSourceClassification{
		ClassifyRBOKSource(rel),
	}, assembly)

	if report.Blocking == 0 {
		t.Fatalf("expected missing structured source span to block, got %+v", report)
	}
	if !hasStructureFinding(report, "node.missing_source_span") {
		t.Fatalf("expected node.missing_source_span finding, got %+v", report.Findings)
	}
}

func normalizedMarkdownTestFeed(t *testing.T, rel string, source string, abs string) LawbookFeed {
	t.Helper()
	hash, _, err := hashFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	result := ExtractMarkdown(source, documentSlugForPath(rel))
	defaults := NodeDefaults{
		DocumentID: documentIDForPath(rel),
		SourcePath: rel,
		SourceHash: "sha256:" + hash,
		Domain:     "test",
		Status:     StatusActive,
		Priority:   PriorityHigh,
	}
	if count := NormalizeExtractResult(&result, defaults); count != 0 {
		t.Fatalf("normalization errors: %d", count)
	}
	for i := range result.Nodes {
		result.Nodes[i].Locator = locatorForNode(rel, result.Nodes[i])
	}
	return BuildNormalizedFeed(result, feedIDForPath(rel), defaults, "2026-05-03T09:00:00Z")
}

func hasStructureFinding(report StructureFidelityReport, code string) bool {
	for _, finding := range report.Findings {
		if finding.Code == code || strings.HasPrefix(finding.Code, code) {
			return true
		}
	}
	return false
}

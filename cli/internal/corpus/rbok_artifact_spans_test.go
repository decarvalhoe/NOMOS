package corpus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const aqFixtureDoc = `# Garanties habitation
| Champ | Valeur |
|---|---|
| Reference | GAR-001 |

## Chapitre 1 - Dégât des eaux

L'assuré est couvert pour les dégâts des eaux.

- Fuite de canalisation
- Infiltration par toiture
- Débordement d'appareil ménager

## Chapitre 2 - Incendie

La garantie incendie couvre les dommages causés par le feu.

### Section 2.1 - Exclusions

Les biens suivants sont exclus de la garantie incendie.
`

// TestIntegratedPipelineSpansPopulated verifies that the full pipeline
// (ExtractMarkdownWithSpans → NormalizeExtractResult → BuildNormalizedFeed)
// produces nodes with valid spans — the root fix for AQ #325.
func TestIntegratedPipelineSpansPopulated(t *testing.T) {
	extracted := ExtractMarkdownWithSpans(aqFixtureDoc, "garanties-habitation", "01_rbok/referentiel/garanties.md")
	defaults := NodeDefaults{
		DocumentID: "DOC-GARANTIES",
		SourcePath: "01_rbok/referentiel/garanties.md",
		SourceHash: "sha256:testfixture",
		Domain:     "insurance-home",
		Status:     StatusActive,
		Priority:   PriorityHigh,
	}
	NormalizeExtractResult(&extracted, defaults)
	feed := BuildNormalizedFeed(extracted, "garanties-feed", defaults, time.Now().UTC().Format(time.RFC3339))

	totalNodes := len(feed.Nodes)
	withSpan := 0
	withByteLen := 0

	for _, node := range feed.Nodes {
		if node.Span.IsValid() {
			withSpan++
		}
		if node.Span.ByteLength > 0 {
			withByteLen++
		}
	}

	t.Logf("total_nodes=%d with_span=%d with_byte_len=%d", totalNodes, withSpan, withByteLen)

	if totalNodes == 0 {
		t.Fatal("expected nodes")
	}
	if withSpan == 0 {
		t.Fatalf("CRITICAL: 0/%d nodes have spans — AQ #325 not fixed", totalNodes)
	}

	// All heading-level nodes must have spans.
	for _, node := range feed.Nodes {
		switch node.NodeType {
		case NodeDocument, NodeChapter, NodeSection, NodeArticle:
			if !node.Span.IsValid() {
				t.Fatalf("heading node %s (%s) has invalid span", node.NodeID, node.NodeType)
			}
			if node.Span.File != "01_rbok/referentiel/garanties.md" {
				t.Fatalf("heading node %s span.file=%q, expected source path", node.NodeID, node.Span.File)
			}
			if node.Span.ByteLength <= 0 {
				t.Fatalf("heading node %s has zero byte_length", node.NodeID)
			}
		}
	}
}

// TestIntegratedPipelineSpanOrdering verifies start_line ordering.
func TestIntegratedPipelineSpanOrdering(t *testing.T) {
	extracted := ExtractMarkdownWithSpans(aqFixtureDoc, "test", "test.md")
	var headingLines []int
	for _, node := range extracted.Nodes {
		switch node.NodeType {
		case NodeDocument, NodeChapter, NodeSection:
			headingLines = append(headingLines, node.Span.StartLine)
		}
	}
	for i := 1; i < len(headingLines); i++ {
		if headingLines[i] < headingLines[i-1] {
			t.Fatalf("heading lines not ordered: %d before %d", headingLines[i-1], headingLines[i])
		}
	}
}

// TestIntegratedPipelineSpanCoverage verifies ComputeSpanCoverage.
func TestIntegratedPipelineSpanCoverage(t *testing.T) {
	extracted := ExtractMarkdownWithSpans(aqFixtureDoc, "test", "test.md")
	cov := ComputeSpanCoverage(extracted.Nodes, len(aqFixtureDoc))
	t.Logf("coverage: total=%d with_span=%d without=%d ratio=%.2f",
		cov.TotalNodes, cov.WithSpan, cov.WithoutSpan, cov.CoverageRatio)
	if cov.WithSpan == 0 {
		t.Fatalf("CRITICAL: 0 nodes with spans in coverage report")
	}
	if cov.WithoutSpan > 0 {
		t.Logf("WARNING: %d nodes without spans", cov.WithoutSpan)
	}
}

// TestIntegratedPipelineAlineaSpans verifies sub-nodes (alineas) have spans.
func TestIntegratedPipelineAlineaSpans(t *testing.T) {
	extracted := ExtractMarkdownWithSpans(aqFixtureDoc, "test", "test.md")
	alineaCount := 0
	alineaWithSpan := 0
	for _, node := range extracted.Nodes {
		if node.NodeType == NodeAlinea {
			alineaCount++
			if node.Span.IsValid() {
				alineaWithSpan++
			}
		}
	}
	if alineaCount < 3 {
		t.Fatalf("expected >= 3 alineas from list items, got %d", alineaCount)
	}
	t.Logf("alineas: total=%d with_span=%d", alineaCount, alineaWithSpan)
	if alineaWithSpan == 0 {
		t.Fatal("no alineas have spans")
	}
}

// TestFeedJSONSpansSerialize verifies spans survive JSON serialization
// (this is what the fidelity proof script reads).
func TestFeedJSONSpansSerialize(t *testing.T) {
	extracted := ExtractMarkdownWithSpans(aqFixtureDoc, "test", "test.md")
	defaults := NodeDefaults{
		DocumentID: "DOC-TEST", SourcePath: "test.md",
		SourceHash: "sha256:test", Domain: "test",
		Status: StatusActive, Priority: PriorityMedium,
	}
	NormalizeExtractResult(&extracted, defaults)
	feed := BuildNormalizedFeed(extracted, "test-feed", defaults, "2026-01-01T00:00:00Z")

	data, err := json.Marshal(feed)
	if err != nil {
		t.Fatal(err)
	}

	// Parse back as generic JSON and check span fields exist.
	var parsed map[string]any
	json.Unmarshal(data, &parsed)
	nodes, ok := parsed["nodes"].([]any)
	if !ok || len(nodes) == 0 {
		t.Fatal("expected nodes in JSON")
	}

	spanCount := 0
	for _, n := range nodes {
		node, ok := n.(map[string]any)
		if !ok {
			continue
		}
		span, ok := node["span"].(map[string]any)
		if !ok {
			continue
		}
		sl, _ := span["start_line"].(float64)
		if sl > 0 {
			spanCount++
		}
	}
	t.Logf("JSON nodes with span.start_line > 0: %d/%d", spanCount, len(nodes))
	if spanCount == 0 {
		t.Fatal("CRITICAL: no nodes in JSON have span.start_line — proof script will report BYTE_SPANS_MISSING")
	}
}

// TestWriteRBOKArtifactPackWithSpans runs the full artifact pack and checks the
// written feed JSON files contain nodes with spans.
func TestWriteRBOKArtifactPackWithSpans(t *testing.T) {
	corpusRoot := t.TempDir()
	rbokDir := filepath.Join(corpusRoot, "01_rbok", "referentiel")
	refDir := filepath.Join(corpusRoot, "01_referentiel")
	os.MkdirAll(rbokDir, 0o755)
	os.MkdirAll(refDir, 0o755)
	os.WriteFile(filepath.Join(rbokDir, "doc.md"), []byte(aqFixtureDoc), 0o644)
	os.WriteFile(filepath.Join(refDir, "ref.md"), []byte("# Reference\n\nRef content.\n"), 0o644)

	outDir := t.TempDir()
	_, err := WriteRBOKLawbookArtifactPack(corpusRoot, outDir, RBOKLawbookArtifactPackOptions{
		Now: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("artifact pack: %v", err)
	}

	// Find the generated feed JSON.
	feedFiles, _ := filepath.Glob(filepath.Join(outDir, "*feed*.json"))
	if len(feedFiles) == 0 {
		// Try the lawbook feed file.
		feedFiles, _ = filepath.Glob(filepath.Join(outDir, "rbok-lawbook-feed.json"))
	}
	if len(feedFiles) == 0 {
		// List everything in outDir for debugging.
		entries, _ := os.ReadDir(outDir)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("no feed JSON found in %s: %v", outDir, names)
	}

	for _, feedFile := range feedFiles {
		data, err := os.ReadFile(feedFile)
		if err != nil {
			t.Fatal(err)
		}

		// Parse as generic JSON to check spans.
		var doc map[string]any
		json.Unmarshal(data, &doc)

		// Collect all nodes (top-level or nested in feeds).
		var allNodes []map[string]any
		if nodes, ok := doc["nodes"].([]any); ok {
			for _, n := range nodes {
				if m, ok := n.(map[string]any); ok {
					allNodes = append(allNodes, m)
				}
			}
		}
		if feeds, ok := doc["feeds"].([]any); ok {
			for _, f := range feeds {
				fm, _ := f.(map[string]any)
				if nodes, ok := fm["nodes"].([]any); ok {
					for _, n := range nodes {
						if m, ok := n.(map[string]any); ok {
							allNodes = append(allNodes, m)
						}
					}
				}
			}
		}

		spanCount := 0
		for _, node := range allNodes {
			span, ok := node["span"].(map[string]any)
			if !ok {
				continue
			}
			if sl, _ := span["start_line"].(float64); sl > 0 {
				spanCount++
			}
		}
		t.Logf("file=%s total_nodes=%d nodes_with_spans=%d", filepath.Base(feedFile), len(allNodes), spanCount)
		if len(allNodes) > 0 && spanCount == 0 {
			t.Fatalf("CRITICAL: %s has %d nodes but 0 with spans — BYTE_SPANS_MISSING will fire", filepath.Base(feedFile), len(allNodes))
		}
	}
}

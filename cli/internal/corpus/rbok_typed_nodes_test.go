package corpus

import (
	"testing"
)

func TestEmitTypedNodesFromExtractionLinks(t *testing.T) {
	source := "# Doc Title\n\nSee [example](https://example.com) and [docs](https://docs.io) for details.\n"
	result := ExtractMarkdown(source, "test-doc")
	EmitTypedNodesFromExtraction(&result)

	_, _, _, links, _ := CountSemanticNodes(result.Nodes)
	if links < 2 {
		t.Fatalf("expected at least 2 link nodes, got %d", links)
	}

	// Check link metadata.
	for _, n := range result.Nodes {
		if n.NodeType == NodeLink {
			if n.Metadata == nil || n.Metadata["href"] == nil {
				t.Fatalf("link node %s missing href metadata", n.NodeID)
			}
		}
	}
}

func TestEmitTypedNodesFromExtractionImages(t *testing.T) {
	source := "# Doc\n\n![diagram](arch.png)\n\n![photo](photo.jpg)\n"
	result := ExtractMarkdown(source, "test-doc")
	EmitTypedNodesFromExtraction(&result)

	_, _, _, _, images := CountSemanticNodes(result.Nodes)
	if images < 2 {
		t.Fatalf("expected at least 2 image nodes, got %d", images)
	}

	for _, n := range result.Nodes {
		if n.NodeType == NodeImage {
			if n.Metadata == nil || n.Metadata["src"] == nil {
				t.Fatalf("image node %s missing src metadata", n.NodeID)
			}
		}
	}
}

func TestEmitTypedNodesFromExtractionCodeBlock(t *testing.T) {
	source := "# Doc\n\n```python\ndef hello():\n    pass\n```\n\n```go\nfunc main() {}\n```\n"
	result := ExtractMarkdown(source, "test-doc")
	EmitTypedNodesFromExtraction(&result)

	_, codeBlocks, _, _, _ := CountSemanticNodes(result.Nodes)
	if codeBlocks < 2 {
		t.Fatalf("expected at least 2 code_block nodes, got %d", codeBlocks)
	}

	for _, n := range result.Nodes {
		if n.NodeType == NodeCodeBlock {
			if n.Metadata == nil || n.Metadata["language"] == nil {
				t.Fatalf("code_block %s missing language metadata", n.NodeID)
			}
		}
	}
}

func TestEmitTypedNodesFromExtractionCodeBlockWithoutLanguageIsPlainText(t *testing.T) {
	source := "# Doc\n\n```\nraw example\n```\n"
	result := ExtractMarkdown(source, "test-doc")
	EmitTypedNodesFromExtraction(&result)

	for _, n := range result.Nodes {
		if n.NodeType != NodeCodeBlock {
			continue
		}
		if n.Metadata["language"] != "plain_text" {
			t.Fatalf("expected plain_text fallback, got %v", n.Metadata["language"])
		}
		if n.Metadata["language_declared"] != false {
			t.Fatalf("expected language_declared=false, got %v", n.Metadata["language_declared"])
		}
		return
	}
	t.Fatal("expected code block node")
}

func TestEmitTypedNodesFromExtractionCallout(t *testing.T) {
	source := "# Doc\n\n> [!WARNING]\n> This is important.\n> Second line.\n"
	result := ExtractMarkdown(source, "test-doc")
	EmitTypedNodesFromExtraction(&result)

	_, _, callouts, _, _ := CountSemanticNodes(result.Nodes)
	if callouts < 1 {
		t.Fatalf("expected at least 1 callout node, got %d", callouts)
	}

	for _, n := range result.Nodes {
		if n.NodeType == NodeCallout {
			if n.Metadata == nil || n.Metadata["callout_type"] == nil {
				t.Fatalf("callout %s missing callout_type metadata", n.NodeID)
			}
			if n.Metadata["callout_type"] != "WARNING" {
				t.Fatalf("expected WARNING, got %v", n.Metadata["callout_type"])
			}
		}
	}
}

func TestEmitTypedNodesFromExtractionTable(t *testing.T) {
	// Table after introductory text (not consumed as heading metadata).
	source := "# Doc\n\nIntroduction paragraph.\n\n| Header A | Header B |\n|----------|----------|\n| Cell 1   | Cell 2   |\n| Cell 3   | Cell 4   |\n"
	result := ExtractMarkdown(source, "test-doc")
	EmitTypedNodesFromExtraction(&result)

	tables, _, _, _, _ := CountSemanticNodes(result.Nodes)
	if tables < 1 {
		t.Fatalf("expected at least 1 table node, got %d", tables)
	}

	for _, n := range result.Nodes {
		if n.NodeType != NodeTable {
			continue
		}
		if n.Metadata == nil {
			t.Fatalf("table node %s missing metadata", n.NodeID)
		}
		if n.Metadata["col_count"] != "2" {
			t.Fatalf("expected col_count=2, got %v", n.Metadata["col_count"])
		}
		if n.Metadata["row_count"] != "2" {
			t.Fatalf("expected row_count=2, got %v", n.Metadata["row_count"])
		}
		return
	}
	t.Fatal("expected table node metadata to be checked")
}

func TestEmitTypedNodesFromExtractionMixed(t *testing.T) {
	source := "# Mixed Document\n\n" +
		"Some text with [a link](http://a.com).\n\n" +
		"![img](pic.png)\n\n" +
		"```js\nconsole.log('hi')\n```\n\n" +
		"| X | Y |\n|---|---|\n| 1 | 2 |\n\n" +
		"> [!NOTE]\n> A note here.\n"
	result := ExtractMarkdown(source, "mixed")
	EmitTypedNodesFromExtraction(&result)

	tables, codeBlocks, callouts, links, images := CountSemanticNodes(result.Nodes)
	if tables < 1 {
		t.Fatalf("expected >= 1 table, got %d", tables)
	}
	if codeBlocks < 1 {
		t.Fatalf("expected >= 1 code_block, got %d", codeBlocks)
	}
	if callouts < 1 {
		t.Fatalf("expected >= 1 callout, got %d", callouts)
	}
	if links < 1 {
		t.Fatalf("expected >= 1 link, got %d", links)
	}
	if images < 1 {
		t.Fatalf("expected >= 1 image, got %d", images)
	}
}

func TestEmitTypedNodesDoesNotDuplicateImageAsLink(t *testing.T) {
	source := "# Doc\n\n![alt](img.png)\n"
	result := ExtractMarkdown(source, "test")
	EmitTypedNodesFromExtraction(&result)

	_, _, _, links, images := CountSemanticNodes(result.Nodes)
	if links != 0 {
		t.Fatalf("expected 0 links (image not link), got %d", links)
	}
	if images < 1 {
		t.Fatalf("expected >= 1 image, got %d", images)
	}
}

func TestEmitTypedNodesNilResult(t *testing.T) {
	// Should not panic.
	EmitTypedNodesFromExtraction(nil)
}

func TestEmitTypedNodesEmpty(t *testing.T) {
	source := "# Just a heading\n"
	result := ExtractMarkdown(source, "empty")
	EmitTypedNodesFromExtraction(&result)

	tables, codeBlocks, callouts, links, images := CountSemanticNodes(result.Nodes)
	if tables+codeBlocks+callouts+links+images != 0 {
		t.Fatalf("expected 0 semantic nodes for heading-only doc")
	}
}

func TestSemanticNodeTypeIsValid(t *testing.T) {
	semantics := []LawbookNodeType{NodeTable, NodeCodeBlock, NodeCallout, NodeLink, NodeImage}
	for _, nt := range semantics {
		if !nt.IsValid() {
			t.Fatalf("expected %s to be valid", nt)
		}
		if !nt.IsSemanticType() {
			t.Fatalf("expected %s to be semantic type", nt)
		}
	}
}

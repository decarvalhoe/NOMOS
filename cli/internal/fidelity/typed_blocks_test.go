package fidelity

import (
	"strings"
	"testing"
)

func TestEmitTypedBlocksLinks(t *testing.T) {
	source := "# Doc\n\nSee [example](https://example.com) and [docs](https://docs.io) here.\n"
	cast := ParseMarkdown(source)
	result := EmitTypedBlocks(&cast)

	if len(result.Links) < 2 {
		t.Fatalf("expected at least 2 links, got %d", len(result.Links))
	}
	found := false
	for _, l := range result.Links {
		if l.Href == "https://example.com" && l.Text == "example" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected link to https://example.com")
	}

	// Verify link nodes added to CAST.
	linkCount := 0
	for _, n := range cast.Nodes {
		if n.Kind == KindLink {
			linkCount++
			if n.Props["href"] == "" {
				t.Fatal("link node missing href prop")
			}
		}
	}
	if linkCount < 2 {
		t.Fatalf("expected at least 2 link nodes in CAST, got %d", linkCount)
	}
}

func TestEmitTypedBlocksImages(t *testing.T) {
	source := "# Doc\n\n![alt text](image.png)\n\n![diagram](./arch.svg)\n"
	cast := ParseMarkdown(source)
	result := EmitTypedBlocks(&cast)

	if len(result.Images) < 2 {
		t.Fatalf("expected at least 2 images, got %d", len(result.Images))
	}
	found := false
	for _, img := range result.Images {
		if img.Src == "image.png" && img.Alt == "alt text" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected image with src=image.png")
	}

	// Verify image nodes.
	imgCount := 0
	for _, n := range cast.Nodes {
		if n.Kind == KindImage {
			imgCount++
			if n.Props["src"] == "" {
				t.Fatal("image node missing src prop")
			}
			if n.Props["alt"] == "" {
				t.Fatal("image node missing alt prop")
			}
		}
	}
	if imgCount < 2 {
		t.Fatalf("expected at least 2 image nodes, got %d", imgCount)
	}
}

func TestEmitTypedBlocksCallout(t *testing.T) {
	source := "# Doc\n\n> [!WARNING]\n> This is a warning callout.\n> Second line.\n"
	cast := ParseMarkdown(source)
	result := EmitTypedBlocks(&cast)

	if len(result.Callouts) != 1 {
		t.Fatalf("expected 1 callout, got %d", len(result.Callouts))
	}
	callout := result.Callouts[0]
	if callout.CalloutType != "WARNING" {
		t.Fatalf("expected WARNING, got %s", callout.CalloutType)
	}
	if !strings.Contains(callout.Content, "warning callout") {
		t.Fatalf("expected callout content, got %q", callout.Content)
	}

	// Verify callout node in CAST.
	calloutCount := 0
	for _, n := range cast.Nodes {
		if n.Kind == KindCallout {
			calloutCount++
			if n.Props["callout_type"] != "WARNING" {
				t.Fatalf("expected callout_type=WARNING, got %s", n.Props["callout_type"])
			}
		}
	}
	if calloutCount != 1 {
		t.Fatalf("expected 1 callout node, got %d", calloutCount)
	}
}

func TestEmitTypedBlocksCalloutTypes(t *testing.T) {
	types := []string{"NOTE", "WARNING", "TIP", "IMPORTANT", "CAUTION"}
	for _, ct := range types {
		source := "# Doc\n\n> [!" + ct + "]\n> Content here.\n"
		cast := ParseMarkdown(source)
		result := EmitTypedBlocks(&cast)

		if len(result.Callouts) != 1 {
			t.Fatalf("[%s] expected 1 callout, got %d", ct, len(result.Callouts))
		}
		if result.Callouts[0].CalloutType != ct {
			t.Fatalf("expected %s, got %s", ct, result.Callouts[0].CalloutType)
		}
	}
}

func TestEmitTypedBlocksNoCalloutInRegularBlockquote(t *testing.T) {
	source := "# Doc\n\n> This is a regular blockquote.\n> No callout marker.\n"
	cast := ParseMarkdown(source)
	result := EmitTypedBlocks(&cast)

	if len(result.Callouts) != 0 {
		t.Fatalf("expected 0 callouts for regular blockquote, got %d", len(result.Callouts))
	}
}

func TestEmitTypedBlocksImagesNotDuplicatedAsLinks(t *testing.T) {
	source := "# Doc\n\n![alt](img.png)\n"
	cast := ParseMarkdown(source)
	result := EmitTypedBlocks(&cast)

	if len(result.Links) != 0 {
		t.Fatalf("expected 0 links (image should not be a link), got %d", len(result.Links))
	}
	if len(result.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(result.Images))
	}
}

func TestEmitTypedBlocksCodeBlockLanguage(t *testing.T) {
	source := "# Doc\n\n```python\nprint('hello')\n```\n\n```go\nfmt.Println(\"hi\")\n```\n"
	cast := ParseMarkdown(source)

	codeBlocks := 0
	for _, n := range cast.Nodes {
		if n.Kind == KindCodeBlock {
			codeBlocks++
			if n.Props["language"] == "" {
				t.Fatalf("code block missing language prop")
			}
		}
	}
	if codeBlocks < 2 {
		t.Fatalf("expected at least 2 code blocks, got %d", codeBlocks)
	}
}

func TestEmitTypedBlocksTableStructure(t *testing.T) {
	source := "# Doc\n\n| A | B |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |\n"
	cast := ParseMarkdown(source)

	tables, _, _, _, _ := CountTypedBlocks(cast)
	if tables < 1 {
		t.Fatalf("expected at least 1 table, got %d", tables)
	}

	// Check table has rows and cells.
	rows := 0
	cells := 0
	for _, n := range cast.Nodes {
		if n.Kind == KindTableRow {
			rows++
		}
		if n.Kind == KindTableCell {
			cells++
		}
	}
	if rows < 2 {
		t.Fatalf("expected at least 2 rows (header+data), got %d", rows)
	}
	if cells < 4 {
		t.Fatalf("expected at least 4 cells, got %d", cells)
	}
}

func TestCountTypedBlocks(t *testing.T) {
	source := "# Title\n\n[link](http://x.com)\n\n![img](y.png)\n\n```go\ncode\n```\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n> [!NOTE]\n> Note.\n"
	cast := ParseMarkdown(source)
	EmitTypedBlocks(&cast)

	tables, codeBlocks, links, images, callouts := CountTypedBlocks(cast)
	if tables < 1 {
		t.Fatalf("expected >= 1 table, got %d", tables)
	}
	if codeBlocks < 1 {
		t.Fatalf("expected >= 1 code block, got %d", codeBlocks)
	}
	if links < 1 {
		t.Fatalf("expected >= 1 link, got %d", links)
	}
	if images < 1 {
		t.Fatalf("expected >= 1 image, got %d", images)
	}
	if callouts < 1 {
		t.Fatalf("expected >= 1 callout, got %d", callouts)
	}
}

func TestEmitTypedBlocksLinkParentID(t *testing.T) {
	source := "# Doc\n\nText with [link](http://example.com).\n"
	cast := ParseMarkdown(source)
	EmitTypedBlocks(&cast)

	for _, n := range cast.Nodes {
		if n.Kind == KindLink {
			if n.ParentID == "" {
				t.Fatal("link node should have parent_id")
			}
			// Parent should be a paragraph.
			for _, p := range cast.Nodes {
				if p.ID == n.ParentID {
					if p.Kind != KindParagraph {
						t.Fatalf("link parent should be paragraph, got %s", p.Kind)
					}
				}
			}
		}
	}
}

func TestEmitTypedBlocksMultipleLinksInParagraph(t *testing.T) {
	source := "# Doc\n\n[A](http://a.com) text [B](http://b.com) more [C](http://c.com)\n"
	cast := ParseMarkdown(source)
	result := EmitTypedBlocks(&cast)

	if len(result.Links) < 3 {
		t.Fatalf("expected 3 links, got %d", len(result.Links))
	}
}

func TestEmitTypedBlocksEmpty(t *testing.T) {
	source := "# Just a heading\n"
	cast := ParseMarkdown(source)
	result := EmitTypedBlocks(&cast)

	if len(result.Links) != 0 {
		t.Fatalf("expected 0 links, got %d", len(result.Links))
	}
	if len(result.Images) != 0 {
		t.Fatalf("expected 0 images, got %d", len(result.Images))
	}
	if len(result.Callouts) != 0 {
		t.Fatalf("expected 0 callouts, got %d", len(result.Callouts))
	}
}

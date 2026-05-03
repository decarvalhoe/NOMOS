package fidelity

import (
	"testing"
)

const sampleHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8"/>
  <title>Test Page</title>
  <link rel="stylesheet" href="https://cdn.example.com/style.css"/>
  <link rel="icon" href="/favicon.ico"/>
  <script src="https://cdn.example.com/app.js"></script>
</head>
<body>
  <h1>Title</h1>
  <p>Paragraph with <a href="https://example.com">a link</a>.</p>
  <img src="images/photo.png" alt="Photo"/>
  <img src="https://remote.example.com/logo.svg" alt="Logo"/>
  <!-- A comment -->
  <div style="background: url('bg.jpg')">
    <video src="media/clip.mp4"></video>
  </div>
</body>
</html>`

func TestParseHTML_RootNode(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	if hast.Root == "" {
		t.Fatal("expected root ID")
	}
	if hast.TotalNodes == 0 {
		t.Fatal("expected nodes")
	}
	if hast.SourceHash == "" {
		t.Fatal("expected source hash")
	}
}

func TestParseHTML_Doctype(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	found := false
	for _, n := range hast.Nodes {
		if n.Kind == KindHTMLDoctype {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected doctype node")
	}
}

func TestParseHTML_Elements(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	tags := map[string]int{}
	for _, n := range hast.Nodes {
		if n.Kind == KindHTMLElement {
			tags[n.Tag]++
		}
	}
	for _, expected := range []string{"html", "head", "body", "h1", "p", "img", "div"} {
		if tags[expected] == 0 {
			t.Errorf("expected tag %q", expected)
		}
	}
}

func TestParseHTML_TextNodes(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	found := false
	for _, n := range hast.Nodes {
		if n.Kind == KindHTMLText && n.Text == "Title" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected text node 'Title'")
	}
}

func TestParseHTML_Comment(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	found := false
	for _, n := range hast.Nodes {
		if n.Kind == KindHTMLComment {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected comment node")
	}
}

func TestParseHTML_Attributes(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	for _, n := range hast.Nodes {
		if n.Tag == "html" && n.Attrs != nil {
			if n.Attrs["lang"] != "en" {
				t.Fatalf("expected lang=en, got %q", n.Attrs["lang"])
			}
			return
		}
	}
	t.Fatal("html element with lang attr not found")
}

func TestParseHTML_ExternalRefs_Images(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	if hast.RefSummary.Images < 2 {
		t.Fatalf("expected at least 2 images, got %d", hast.RefSummary.Images)
	}
}

func TestParseHTML_ExternalRefs_Stylesheet(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	if hast.RefSummary.Stylesheets != 1 {
		t.Fatalf("expected 1 stylesheet, got %d", hast.RefSummary.Stylesheets)
	}
	found := false
	for _, r := range hast.ExternalRefs {
		if r.Kind == ExtRefStylesheet && r.URL == "https://cdn.example.com/style.css" {
			found = true
			if !r.IsRemote {
				t.Fatal("expected remote stylesheet")
			}
		}
	}
	if !found {
		t.Fatal("stylesheet ref not found")
	}
}

func TestParseHTML_ExternalRefs_Script(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	if hast.RefSummary.Scripts != 1 {
		t.Fatalf("expected 1 script, got %d", hast.RefSummary.Scripts)
	}
}

func TestParseHTML_ExternalRefs_Favicon(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	found := false
	for _, r := range hast.ExternalRefs {
		if r.Kind == ExtRefFavicon {
			found = true
			if r.IsRemote {
				t.Fatal("favicon should be local")
			}
		}
	}
	if !found {
		t.Fatal("favicon ref not found")
	}
}

func TestParseHTML_ExternalRefs_RemoteVsLocal(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	if hast.RefSummary.TotalRemote < 3 {
		t.Fatalf("expected at least 3 remote refs, got %d", hast.RefSummary.TotalRemote)
	}
	if hast.RefSummary.TotalLocal < 2 {
		t.Fatalf("expected at least 2 local refs, got %d", hast.RefSummary.TotalLocal)
	}
}

func TestParseHTML_ExternalRefs_Link(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	found := false
	for _, r := range hast.ExternalRefs {
		if r.Kind == ExtRefLink && r.URL == "https://example.com" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected link ref for https://example.com")
	}
}

func TestParseHTML_ExternalRefs_BackgroundURL(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	found := false
	for _, r := range hast.ExternalRefs {
		if r.URL == "bg.jpg" && r.Attr == "style" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected background url ref")
	}
}

func TestParseHTML_ExternalRefs_Media(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	if hast.RefSummary.Media < 1 {
		t.Fatalf("expected at least 1 media ref, got %d", hast.RefSummary.Media)
	}
}

func TestParseHTML_ParentChain(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	for _, n := range hast.Nodes {
		if n.ID == hast.Root {
			continue
		}
		if n.ParentID == "" {
			t.Fatalf("node %s (%s) has no parent", n.ID, n.Kind)
		}
	}
}

func TestParseHTML_UniqueIDs(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	seen := map[string]bool{}
	for _, n := range hast.Nodes {
		if seen[n.ID] {
			t.Fatalf("duplicate ID: %s", n.ID)
		}
		seen[n.ID] = true
	}
}

func TestParseHTML_Empty(t *testing.T) {
	hast := ParseHTML("")
	if hast.TotalNodes != 1 {
		t.Fatalf("expected 1 (root), got %d", hast.TotalNodes)
	}
}

func TestParseHTML_Minimal(t *testing.T) {
	hast := ParseHTML("<p>Hello</p>")
	found := false
	for _, n := range hast.Nodes {
		if n.Kind == KindHTMLText && n.Text == "Hello" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected text node Hello")
	}
}

func TestHASTToCAST(t *testing.T) {
	hast := ParseHTML(sampleHTML)
	cast := HASTToCAST(hast)

	if cast.Root == "" {
		t.Fatal("expected root")
	}
	if len(cast.Nodes) != len(hast.Nodes) {
		t.Fatalf("expected %d CAST nodes, got %d", len(hast.Nodes), len(cast.Nodes))
	}
}

func TestParseHTMLToCAST(t *testing.T) {
	cast := ParseHTMLToCAST("<p>Test</p>")
	if len(cast.Nodes) == 0 {
		t.Fatal("expected nodes from ParseHTMLToCAST")
	}
	if cast.Root == "" {
		t.Fatal("expected root")
	}
}

func TestIsRemoteURL(t *testing.T) {
	cases := map[string]bool{
		"https://example.com":  true,
		"http://example.com":   true,
		"//cdn.example.com/a":  true,
		"/local/path.css":      false,
		"images/photo.png":     false,
		"data:image/png;base64": false,
	}
	for url, want := range cases {
		if isRemoteURL(url) != want {
			t.Errorf("isRemoteURL(%q) = %v, want %v", url, !want, want)
		}
	}
}

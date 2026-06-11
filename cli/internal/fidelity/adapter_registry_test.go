package fidelity

import (
	"strings"
	"testing"
)

func TestDefaultRegistryCount(t *testing.T) {
	r := DefaultRegistry()
	if r.Count() != 5 {
		t.Fatalf("expected 5 adapters, got %d", r.Count())
	}
}

func TestDefaultRegistryNames(t *testing.T) {
	r := DefaultRegistry()
	names := r.Names()
	expected := []string{"docx", "json", "markdown", "pdf", "yaml"}
	if len(names) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, names)
	}
	for i, n := range expected {
		if names[i] != n {
			t.Fatalf("names[%d]: expected %q, got %q", i, n, names[i])
		}
	}
}

func TestDefaultRegistryExtensions(t *testing.T) {
	r := DefaultRegistry()
	exts := r.SupportedExtensions()
	for _, want := range []string{".md", ".mdx", ".yaml", ".yml", ".json", ".docx", ".pdf"} {
		found := false
		for _, e := range exts {
			if e == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected extension %q in %v", want, exts)
		}
	}
}

func TestLookupByName(t *testing.T) {
	r := DefaultRegistry()
	a, ok := r.Lookup("markdown")
	if !ok {
		t.Fatal("markdown not found")
	}
	if a.Name() != "markdown" {
		t.Fatalf("name: %q", a.Name())
	}
}

func TestLookupUnknown(t *testing.T) {
	r := DefaultRegistry()
	_, ok := r.Lookup("latex")
	if ok {
		t.Fatal("latex should not exist")
	}
}

func TestForFileMarkdown(t *testing.T) {
	r := DefaultRegistry()
	a, ok := r.ForFile("docs/reglement.md")
	if !ok {
		t.Fatal("should resolve .md")
	}
	if a.Name() != "markdown" {
		t.Fatalf("expected markdown, got %q", a.Name())
	}
}

func TestForFileMDX(t *testing.T) {
	r := DefaultRegistry()
	a, ok := r.ForFile("page.mdx")
	if !ok {
		t.Fatal("should resolve .mdx")
	}
	if a.Name() != "markdown" {
		t.Fatalf("expected markdown, got %q", a.Name())
	}
}

func TestForFileYAML(t *testing.T) {
	r := DefaultRegistry()
	for _, name := range []string{"config.yaml", "data.yml"} {
		a, ok := r.ForFile(name)
		if !ok {
			t.Fatalf("should resolve %s", name)
		}
		if a.Name() != "yaml" {
			t.Fatalf("expected yaml for %s, got %q", name, a.Name())
		}
	}
}

func TestForFileJSON(t *testing.T) {
	r := DefaultRegistry()
	a, ok := r.ForFile("report.json")
	if !ok {
		t.Fatal("should resolve .json")
	}
	if a.Name() != "json" {
		t.Fatalf("expected json, got %q", a.Name())
	}
}

func TestForFileUnknown(t *testing.T) {
	r := DefaultRegistry()
	_, ok := r.ForFile("data.csv")
	if ok {
		t.Fatal("csv should not resolve")
	}
}

func TestForFileCaseInsensitive(t *testing.T) {
	r := DefaultRegistry()
	a, ok := r.ForFile("README.MD")
	if !ok {
		t.Fatal("should resolve .MD case-insensitively")
	}
	if a.Name() != "markdown" {
		t.Fatalf("got %q", a.Name())
	}
}

func TestRegisterDuplicateName(t *testing.T) {
	r := NewRegistry()
	r.Register(MarkdownAdapter{})
	err := r.Register(MarkdownAdapter{})
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestRegisterDuplicateExtension(t *testing.T) {
	r := NewRegistry()
	r.Register(MarkdownAdapter{})
	err := r.Register(PlaceholderAdapter{AdapterName: "other-md", Exts: []string{".md"}})
	if err == nil {
		t.Fatal("expected error for duplicate extension")
	}
}

func TestRegisterEmptyName(t *testing.T) {
	r := NewRegistry()
	err := r.Register(PlaceholderAdapter{AdapterName: "", Exts: []string{".x"}})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

// --- Markdown adapter ---

func TestMarkdownParse(t *testing.T) {
	a := MarkdownAdapter{}
	src := []byte("# Title\n\nParagraph text.\n\n- item\n")
	result, err := a.Parse(src, "test.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.Format != "markdown" {
		t.Fatalf("format: %q", result.Format)
	}
	if result.NodeCount == 0 {
		t.Fatal("expected nodes")
	}
}

func TestMarkdownSpans(t *testing.T) {
	a := MarkdownAdapter{}
	src := []byte("# H1\n\n## H2\n\nText.\n")
	spans, err := a.Spans(src, "test.md")
	if err != nil {
		t.Fatalf("spans: %v", err)
	}
	headings := 0
	for _, s := range spans {
		if s.NodeType == "heading" {
			headings++
		}
		if s.StartLine <= 0 {
			t.Fatalf("invalid start_line: %d", s.StartLine)
		}
	}
	if headings != 2 {
		t.Fatalf("expected 2 headings, got %d", headings)
	}
}

func TestMarkdownValidatePass(t *testing.T) {
	a := MarkdownAdapter{}
	r := a.Validate([]byte("# Doc\nContent.\n"), "test.md")
	if !r.Valid {
		t.Fatalf("expected valid: %v", r.Findings)
	}
}

func TestMarkdownValidateNoHeading(t *testing.T) {
	a := MarkdownAdapter{}
	r := a.Validate([]byte("Just text, no heading.\n"), "test.md")
	if r.Valid {
		t.Fatal("expected invalid without heading")
	}
}

func TestMarkdownValidateEmpty(t *testing.T) {
	a := MarkdownAdapter{}
	r := a.Validate([]byte{}, "empty.md")
	if !r.Valid {
		t.Fatal("empty should be valid")
	}
}

// --- YAML adapter ---

func TestYAMLParse(t *testing.T) {
	a := YAMLAdapter{}
	src := []byte("key: value\nitems:\n  - one\n  - two\n")
	result, err := a.Parse(src, "data.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.Format != "yaml" {
		t.Fatalf("format: %q", result.Format)
	}
	if result.NodeCount == 0 {
		t.Fatal("expected nodes")
	}
}

func TestYAMLValidateWithTabs(t *testing.T) {
	a := YAMLAdapter{}
	r := a.Validate([]byte("key:\n\tvalue\n"), "bad.yaml")
	if r.Valid {
		t.Fatal("tabs should be invalid in YAML")
	}
}

// --- JSON adapter ---

func TestJSONParse(t *testing.T) {
	a := JSONAdapter{}
	src := []byte(`{"key": "value"}`)
	result, err := a.Parse(src, "data.json")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.Format != "json" {
		t.Fatalf("format: %q", result.Format)
	}
}

func TestJSONValidateInvalid(t *testing.T) {
	a := JSONAdapter{}
	r := a.Validate([]byte("not json"), "bad.json")
	if r.Valid {
		t.Fatal("should be invalid")
	}
}

// --- Placeholder adapter ---

func TestPlaceholderParseErrors(t *testing.T) {
	// VRC-30/VRC-41 promoted pdf and docx to real adapters, so no placeholder
	// is registered by default; the PlaceholderAdapter TYPE still refuses
	// loudly when used for a not-yet-implemented format.
	a := PlaceholderAdapter{AdapterName: "rtf", Exts: []string{".rtf"}}
	_, err := a.Parse(nil, "file.rtf")
	if err == nil {
		t.Fatal("placeholder should error")
	}
	if !strings.Contains(err.Error(), "not yet implemented") {
		t.Fatalf("error: %v", err)
	}
}

func TestPlaceholderValidate(t *testing.T) {
	a := PlaceholderAdapter{AdapterName: "rtf", Exts: []string{".rtf"}}
	v := a.Validate(nil, "file.rtf")
	if v.Valid {
		t.Fatal("placeholder should be invalid")
	}
}

func TestPlaceholderSpansErrors(t *testing.T) {
	a := PlaceholderAdapter{AdapterName: "docx", Exts: []string{".docx"}}
	_, err := a.Spans(nil, "file.docx")
	if err == nil {
		t.Fatal("placeholder spans should error")
	}
}

// --- Byte offset correctness ---

func TestMarkdownSpansByteOffsets(t *testing.T) {
	a := MarkdownAdapter{}
	src := "# Title\nBody line.\n"
	spans, _ := a.Spans([]byte(src), "test.md")
	if len(spans) < 2 {
		t.Fatalf("expected >=2 spans, got %d", len(spans))
	}
	// First span: "# Title" at offset 0.
	if spans[0].ByteOff != 0 {
		t.Fatalf("first span offset: %d", spans[0].ByteOff)
	}
	// Second span: "Body line." at offset 8 (len("# Title\n") = 8).
	if spans[1].ByteOff != 8 {
		t.Fatalf("second span offset: expected 8, got %d", spans[1].ByteOff)
	}
}

package fidelity

import (
	"strings"
	"testing"
)

const sampleYAML = `# Schema version
schema_version: "0.1.0"

project:
  id: nomos
  name: Nomos
  owners:
    - name: Alice
      role: lead
    - name: Bob

surfaces:
  - name: cli
    type: cli
    path: cli/
  - name: docs
    type: docs
`

func TestParseYAMLASTNodeCount(t *testing.T) {
	ast := ParseYAMLAST([]byte(sampleYAML), "test.yaml")
	if ast.NodeCount == 0 {
		t.Fatal("expected nodes")
	}
	if ast.Root == "" {
		t.Fatal("expected root ID")
	}
}

func TestParseYAMLASTRoot(t *testing.T) {
	ast := ParseYAMLAST([]byte(sampleYAML), "test.yaml")
	root := findYAMLNode(ast, ast.Root)
	if root == nil {
		t.Fatal("root not found")
	}
	if root.Kind != YAMLDocument {
		t.Fatalf("root kind: %q", root.Kind)
	}
}

func TestParseYAMLASTMappingValue(t *testing.T) {
	ast := ParseYAMLAST([]byte(sampleYAML), "test.yaml")
	var found *YAMLASTNode
	for i := range ast.Nodes {
		if ast.Nodes[i].Kind == YAMLMappingValue && ast.Nodes[i].Key == "schema_version" {
			found = &ast.Nodes[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected schema_version mapping value")
	}
	if found.Value != `"0.1.0"` {
		t.Fatalf("value: %q", found.Value)
	}
}

func TestParseYAMLASTMappingContainer(t *testing.T) {
	ast := ParseYAMLAST([]byte(sampleYAML), "test.yaml")
	var found *YAMLASTNode
	for i := range ast.Nodes {
		if ast.Nodes[i].Kind == YAMLMapping && ast.Nodes[i].Key == "project" {
			found = &ast.Nodes[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected project mapping")
	}
	if len(found.Children) == 0 {
		t.Fatal("project should have children")
	}
}

func TestParseYAMLASTSequenceItems(t *testing.T) {
	ast := ParseYAMLAST([]byte(sampleYAML), "test.yaml")
	var items []YAMLASTNode
	for _, n := range ast.Nodes {
		if n.Kind == YAMLSequenceItem {
			items = append(items, n)
		}
	}
	// owners: 2 items, surfaces: 2 items (each with nested kv = multiple lines)
	if len(items) < 4 {
		t.Fatalf("expected at least 4 sequence items, got %d", len(items))
	}
}

func TestParseYAMLASTSequenceItemKeyValue(t *testing.T) {
	ast := ParseYAMLAST([]byte(sampleYAML), "test.yaml")
	for _, n := range ast.Nodes {
		if n.Kind == YAMLSequenceItem && n.Key == "name" && n.Value == "Alice" {
			return
		}
	}
	t.Fatal("expected sequence item with key=name value=Alice")
}

func TestParseYAMLASTComment(t *testing.T) {
	ast := ParseYAMLAST([]byte(sampleYAML), "test.yaml")
	var comments []YAMLASTNode
	for _, n := range ast.Nodes {
		if n.Kind == YAMLComment {
			comments = append(comments, n)
		}
	}
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	if !strings.Contains(comments[0].Value, "Schema version") {
		t.Fatalf("comment value: %q", comments[0].Value)
	}
}

func TestParseYAMLASTBlankLines(t *testing.T) {
	ast := ParseYAMLAST([]byte(sampleYAML), "test.yaml")
	blanks := 0
	for _, n := range ast.Nodes {
		if n.Kind == YAMLBlank {
			blanks++
		}
	}
	if blanks == 0 {
		t.Fatal("expected blank line nodes")
	}
}

func TestParseYAMLASTSpans(t *testing.T) {
	ast := ParseYAMLAST([]byte(sampleYAML), "test.yaml")
	for _, n := range ast.Nodes {
		if n.Kind == YAMLDocument {
			continue
		}
		if n.Span.StartLine <= 0 {
			t.Fatalf("node %s start_line: %d", n.ID, n.Span.StartLine)
		}
		if n.Span.EndLine < n.Span.StartLine {
			t.Fatalf("node %s end < start", n.ID)
		}
	}
}

func TestParseYAMLASTHashes(t *testing.T) {
	ast := ParseYAMLAST([]byte(sampleYAML), "test.yaml")
	if !strings.HasPrefix(ast.SourceHash, "sha256:") {
		t.Fatalf("source hash: %q", ast.SourceHash)
	}
	for _, n := range ast.Nodes {
		if !strings.HasPrefix(n.Hash, "sha256:") {
			t.Fatalf("node %s hash: %q", n.ID, n.Hash)
		}
	}
}

func TestParseYAMLASTParentChain(t *testing.T) {
	ast := ParseYAMLAST([]byte(sampleYAML), "test.yaml")
	nodeMap := map[string]bool{}
	for _, n := range ast.Nodes {
		nodeMap[n.ID] = true
	}
	for _, n := range ast.Nodes {
		if n.ID == ast.Root {
			if n.ParentID != "" {
				t.Fatal("root should have no parent")
			}
			continue
		}
		if n.ParentID == "" {
			t.Fatalf("non-root %s (%s) has no parent", n.ID, n.Kind)
		}
		if !nodeMap[n.ParentID] {
			t.Fatalf("node %s parent %s not found", n.ID, n.ParentID)
		}
	}
}

func TestParseYAMLASTDeterministic(t *testing.T) {
	src := []byte(sampleYAML)
	a1 := ParseYAMLAST(src, "test.yaml")
	a2 := ParseYAMLAST(src, "test.yaml")
	if a1.NodeCount != a2.NodeCount {
		t.Fatal("node count unstable")
	}
	for i := range a1.Nodes {
		if a1.Nodes[i].ID != a2.Nodes[i].ID {
			t.Fatalf("node[%d] ID unstable", i)
		}
	}
}

func TestParseYAMLASTEmpty(t *testing.T) {
	ast := ParseYAMLAST([]byte{}, "empty.yaml")
	if ast.NodeCount < 1 {
		t.Fatal("expected at least document root")
	}
}

func TestParseYAMLASTNodeIDs(t *testing.T) {
	ast := ParseYAMLAST([]byte(sampleYAML), "test.yaml")
	for _, n := range ast.Nodes {
		if !strings.HasPrefix(n.ID, "Y-") {
			t.Fatalf("ID should start with Y-: %q", n.ID)
		}
	}
}

func TestParseYAMLASTByteOffsets(t *testing.T) {
	src := "key: value\nlist:\n  - item\n"
	ast := ParseYAMLAST([]byte(src), "test.yaml")
	// Skip document root.
	for _, n := range ast.Nodes {
		if n.Kind == YAMLDocument || n.Kind == YAMLBlank {
			continue
		}
		if n.Span.ByteLen <= 0 {
			t.Fatalf("node %s byte_len: %d", n.ID, n.Span.ByteLen)
		}
	}
}

// --- YAMLASTAdapter interface ---

func TestYAMLASTAdapterParse(t *testing.T) {
	a := YAMLASTAdapter{}
	result, err := a.Parse([]byte(sampleYAML), "test.yaml")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if result.Format != "yaml-ast" {
		t.Fatalf("format: %q", result.Format)
	}
	if result.NodeCount == 0 {
		t.Fatal("expected nodes")
	}
}

func TestYAMLASTAdapterSpans(t *testing.T) {
	a := YAMLASTAdapter{}
	spans, err := a.Spans([]byte("key: val\n"), "test.yaml")
	if err != nil {
		t.Fatalf("spans: %v", err)
	}
	if len(spans) == 0 {
		t.Fatal("expected spans")
	}
}

func TestYAMLASTAdapterValidateTabs(t *testing.T) {
	a := YAMLASTAdapter{}
	r := a.Validate([]byte("key:\n\tvalue\n"), "bad.yaml")
	if r.Valid {
		t.Fatal("tabs should fail")
	}
}

func TestYAMLASTAdapterValidateEmpty(t *testing.T) {
	a := YAMLASTAdapter{}
	r := a.Validate([]byte{}, "empty.yaml")
	if !r.Valid {
		t.Fatal("empty should pass")
	}
}

func TestYAMLASTAdapterRegistration(t *testing.T) {
	r := NewRegistry()
	err := r.Register(YAMLASTAdapter{})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	a, ok := r.ForFile("data.yaml")
	if !ok {
		t.Fatal("should resolve .yaml")
	}
	if a.Name() != "yaml-ast" {
		t.Fatalf("name: %q", a.Name())
	}
}

// --- helper ---

func findYAMLNode(ast YAMLAST, id string) *YAMLASTNode {
	for i := range ast.Nodes {
		if ast.Nodes[i].ID == id {
			return &ast.Nodes[i]
		}
	}
	return nil
}

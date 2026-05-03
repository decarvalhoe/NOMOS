package fidelity

import (
	"strings"
	"testing"
)

const simpleJSON = `{
  "name": "Nomos",
  "version": "0.1.0",
  "active": true,
  "count": 42,
  "tags": ["cli", "go", "cue"],
  "config": {
    "mode": "strict",
    "verbose": false
  },
  "empty": null
}`

const nestedJSON = `{
  "project": {
    "id": "nomos",
    "surfaces": [
      {"name": "api", "critical": true},
      {"name": "ui", "critical": false}
    ]
  }
}`

func TestParseJSON_Simple(t *testing.T) {
	jast, err := ParseJSON(simpleJSON)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if jast.Root == "" {
		t.Fatal("expected root ID")
	}
	if jast.TotalNodes == 0 {
		t.Fatal("expected nodes")
	}
	if jast.SourceHash == "" {
		t.Fatal("expected source hash")
	}
}

func TestParseJSON_RootIsObject(t *testing.T) {
	jast, err := ParseJSON(simpleJSON)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	var root *JSONNode
	for i := range jast.Nodes {
		if jast.Nodes[i].ID == jast.Root {
			root = &jast.Nodes[i]
			break
		}
	}
	if root == nil {
		t.Fatal("root node not found")
	}
	if root.Kind != KindJSONObject {
		t.Fatalf("expected root kind json_object, got %s", root.Kind)
	}
	if root.Pointer != "" {
		t.Fatalf("expected empty pointer for root, got %q", root.Pointer)
	}
}

func TestParseJSON_ScalarValues(t *testing.T) {
	jast, err := ParseJSON(simpleJSON)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	scalars := map[string]struct{ value, valueType string }{
		"/name":    {"Nomos", "string"},
		"/version": {"0.1.0", "string"},
		"/active":  {"true", "boolean"},
		"/count":   {"42", "number"},
		"/empty":   {"null", "null"},
	}

	for _, node := range jast.Nodes {
		if expected, ok := scalars[node.Pointer]; ok {
			if node.Value != expected.value {
				t.Errorf("pointer %s: expected value %q, got %q", node.Pointer, expected.value, node.Value)
			}
			if node.ValueType != expected.valueType {
				t.Errorf("pointer %s: expected type %q, got %q", node.Pointer, expected.valueType, node.ValueType)
			}
			if node.Kind != KindJSONValue {
				t.Errorf("pointer %s: expected kind json_value, got %s", node.Pointer, node.Kind)
			}
		}
	}
}

func TestParseJSON_ArrayNode(t *testing.T) {
	jast, err := ParseJSON(simpleJSON)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	var tagsNode *JSONNode
	for i := range jast.Nodes {
		if jast.Nodes[i].Pointer == "/tags" {
			tagsNode = &jast.Nodes[i]
			break
		}
	}
	if tagsNode == nil {
		t.Fatal("tags array not found")
	}
	if tagsNode.Kind != KindJSONArray {
		t.Fatalf("expected json_array, got %s", tagsNode.Kind)
	}
	if len(tagsNode.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(tagsNode.Children))
	}
}

func TestParseJSON_NestedObject(t *testing.T) {
	jast, err := ParseJSON(simpleJSON)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	var configNode *JSONNode
	for i := range jast.Nodes {
		if jast.Nodes[i].Pointer == "/config" {
			configNode = &jast.Nodes[i]
			break
		}
	}
	if configNode == nil {
		t.Fatal("config object not found")
	}
	if configNode.Kind != KindJSONObject {
		t.Fatalf("expected json_object, got %s", configNode.Kind)
	}
	if configNode.Depth != 1 {
		t.Fatalf("expected depth 1, got %d", configNode.Depth)
	}
}

func TestParseJSON_DeepNesting(t *testing.T) {
	jast, err := ParseJSON(nestedJSON)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	// Find /project/surfaces/0/name
	found := false
	for _, node := range jast.Nodes {
		if node.Pointer == "/project/surfaces/0/name" {
			found = true
			if node.Value != "api" {
				t.Fatalf("expected value 'api', got %q", node.Value)
			}
			if node.Depth != 4 {
				t.Fatalf("expected depth 4, got %d", node.Depth)
			}
		}
	}
	if !found {
		t.Fatal("deep pointer /project/surfaces/0/name not found")
	}
}

func TestParseJSON_PointerPaths(t *testing.T) {
	jast, err := ParseJSON(simpleJSON)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	pointers := map[string]bool{}
	for _, node := range jast.Nodes {
		pointers[node.Pointer] = true
	}

	for _, expected := range []string{"", "/name", "/version", "/tags", "/tags/0", "/config", "/config/mode"} {
		if !pointers[expected] {
			t.Errorf("expected pointer %q not found", expected)
		}
	}
}

func TestParseJSON_UniqueIDs(t *testing.T) {
	jast, err := ParseJSON(simpleJSON)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	seen := map[string]bool{}
	for _, node := range jast.Nodes {
		if seen[node.ID] {
			t.Fatalf("duplicate ID: %s", node.ID)
		}
		seen[node.ID] = true
	}
}

func TestParseJSON_StableIDs(t *testing.T) {
	j1, _ := ParseJSON(simpleJSON)
	j2, _ := ParseJSON(simpleJSON)

	if len(j1.Nodes) != len(j2.Nodes) {
		t.Fatalf("node count changed: %d vs %d", len(j1.Nodes), len(j2.Nodes))
	}

	ids1 := map[string]string{}
	for _, n := range j1.Nodes {
		ids1[n.Pointer] = n.ID
	}
	for _, n := range j2.Nodes {
		if ids1[n.Pointer] != n.ID {
			t.Fatalf("pointer %s: ID changed %q vs %q", n.Pointer, ids1[n.Pointer], n.ID)
		}
	}
}

func TestParseJSON_ParentChain(t *testing.T) {
	jast, err := ParseJSON(nestedJSON)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	nodeByID := map[string]JSONNode{}
	for _, n := range jast.Nodes {
		nodeByID[n.ID] = n
	}

	for _, node := range jast.Nodes {
		if node.ID == jast.Root {
			continue
		}
		if node.ParentID == "" {
			t.Fatalf("node %s (%s) has no parent", node.Pointer, node.Kind)
		}
		if _, ok := nodeByID[node.ParentID]; !ok {
			t.Fatalf("node %s parent %s not found", node.Pointer, node.ParentID)
		}
	}
}

func TestParseJSON_KeySpan(t *testing.T) {
	jast, err := ParseJSON(simpleJSON)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	for _, node := range jast.Nodes {
		if node.Key != "" && node.KeySpan != nil {
			if node.KeySpan.StartByte >= node.KeySpan.EndByte {
				t.Fatalf("node %s key span invalid: [%d:%d]", node.Pointer, node.KeySpan.StartByte, node.KeySpan.EndByte)
			}
			// Verify key span contains the key text in source.
			if node.KeySpan.EndByte <= len(simpleJSON) {
				slice := simpleJSON[node.KeySpan.StartByte:node.KeySpan.EndByte]
				if !strings.Contains(slice, node.Key) {
					t.Errorf("key span for %s does not contain key %q: got %q", node.Pointer, node.Key, slice)
				}
			}
		}
	}
}

func TestParseJSON_ValueSpan(t *testing.T) {
	jast, err := ParseJSON(simpleJSON)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	for _, node := range jast.Nodes {
		if node.ValueSpan.StartByte > node.ValueSpan.EndByte {
			t.Fatalf("node %s value span invalid: [%d:%d]", node.Pointer, node.ValueSpan.StartByte, node.ValueSpan.EndByte)
		}
		if node.ValueSpan.StartLine < 1 {
			t.Fatalf("node %s has start_line < 1", node.Pointer)
		}
	}
}

func TestParseJSON_InvalidJSON(t *testing.T) {
	_, err := ParseJSON("not valid json {{{")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseJSON_EmptyObject(t *testing.T) {
	jast, err := ParseJSON("{}")
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if jast.TotalNodes != 1 {
		t.Fatalf("expected 1 node for empty object, got %d", jast.TotalNodes)
	}
}

func TestParseJSON_EmptyArray(t *testing.T) {
	jast, err := ParseJSON("[]")
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if jast.TotalNodes != 1 {
		t.Fatalf("expected 1 node for empty array, got %d", jast.TotalNodes)
	}
}

func TestParseJSON_EscapedPointer(t *testing.T) {
	src := `{"a/b": 1, "c~d": 2}`
	jast, err := ParseJSON(src)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	pointers := map[string]bool{}
	for _, n := range jast.Nodes {
		pointers[n.Pointer] = true
	}
	if !pointers["/a~1b"] {
		t.Fatal("expected escaped pointer /a~1b")
	}
	if !pointers["/c~0d"] {
		t.Fatal("expected escaped pointer /c~0d")
	}
}

// --- JAST to CAST conversion ---

func TestJASTToCAST(t *testing.T) {
	jast, err := ParseJSON(simpleJSON)
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	cast := jastToCAST(jast)

	if cast.Root == "" {
		t.Fatal("expected root")
	}
	if len(cast.Nodes) != len(jast.Nodes) {
		t.Fatalf("expected %d CAST nodes, got %d", len(jast.Nodes), len(cast.Nodes))
	}
	for _, n := range cast.Nodes {
		if _, hasPointer := n.Props["pointer"]; !hasPointer {
			t.Fatalf("expected pointer prop on node %s", n.ID)
		}
	}
}

// --- Adapter Registry ---

func TestAdapterRegistry_Markdown(t *testing.T) {
	a, ok := GetAdapter("markdown")
	if !ok {
		t.Fatal("markdown adapter not registered")
	}
	cast, err := a("# Title\n\nText.\n")
	if err != nil {
		t.Fatalf("markdown adapter: %v", err)
	}
	if len(cast.Nodes) == 0 {
		t.Fatal("expected nodes from markdown adapter")
	}
}

func TestAdapterRegistry_JSON(t *testing.T) {
	a, ok := GetAdapter("json")
	if !ok {
		t.Fatal("json adapter not registered")
	}
	cast, err := a(`{"key": "value"}`)
	if err != nil {
		t.Fatalf("json adapter: %v", err)
	}
	if len(cast.Nodes) == 0 {
		t.Fatal("expected nodes from json adapter")
	}
}

func TestAdapterRegistry_List(t *testing.T) {
	names := ListAdapters()
	if len(names) < 2 {
		t.Fatalf("expected at least 2 adapters, got %d", len(names))
	}
	has := map[string]bool{}
	for _, n := range names {
		has[n] = true
	}
	if !has["markdown"] || !has["json"] {
		t.Fatalf("expected markdown and json adapters, got %v", names)
	}
}

func TestAdapterRegistry_Unknown(t *testing.T) {
	_, ok := GetAdapter("xml")
	if ok {
		t.Fatal("expected xml adapter to not be registered")
	}
}

package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// JSON-specific node kinds.
const (
	KindJSONObject   NodeKind = "json_object"
	KindJSONArray    NodeKind = "json_array"
	KindJSONField    NodeKind = "json_field"
	KindJSONValue    NodeKind = "json_value"
)

// JSONSpan records byte-level position of a JSON element.
type JSONSpan struct {
	StartByte int `json:"start_byte"`
	EndByte   int `json:"end_byte"`
	StartLine int `json:"start_line"`
	EndLine   int `json:"end_line"`
}

// JSONNode is a node in the JSON AST with pointer path and byte spans.
type JSONNode struct {
	ID        string            `json:"id"`
	Kind      NodeKind          `json:"kind"`
	Pointer   string            `json:"pointer"`
	Key       string            `json:"key,omitempty"`
	Value     string            `json:"value,omitempty"`
	ValueType string            `json:"value_type"`
	Children  []string          `json:"children,omitempty"`
	ParentID  string            `json:"parent_id,omitempty"`
	KeySpan   *JSONSpan         `json:"key_span,omitempty"`
	ValueSpan JSONSpan          `json:"value_span"`
	Depth     int               `json:"depth"`
	Hash      string            `json:"hash"`
}

// JAST is the JSON AST output.
type JAST struct {
	Root       string     `json:"root"`
	Nodes      []JSONNode `json:"nodes"`
	SourceHash string     `json:"source_hash"`
	TotalNodes int        `json:"total_nodes"`
}

// ParseJSON parses JSON source into a JAST with pointer paths and byte spans.
func ParseJSON(source string) (JAST, error) {
	var raw any
	if err := json.Unmarshal([]byte(source), &raw); err != nil {
		return JAST{}, fmt.Errorf("invalid JSON: %w", err)
	}

	lineOffsets := jsonLineOffsets(source)
	h := sha256.Sum256([]byte(source))
	jast := JAST{SourceHash: hex.EncodeToString(h[:])}

	rootID := jsonNodeID("", 0)
	jast.Root = rootID

	walkJSON(&jast, raw, "", rootID, "", 0, source, lineOffsets)

	jast.TotalNodes = len(jast.Nodes)
	return jast, nil
}

func walkJSON(jast *JAST, val any, pointer string, parentID string, key string, depth int, source string, offsets []int) {
	nodeID := jsonNodeID(pointer, depth)

	switch v := val.(type) {
	case map[string]any:
		node := JSONNode{
			ID:        nodeID,
			Kind:      KindJSONObject,
			Pointer:   pointer,
			Key:       key,
			ValueType: "object",
			ParentID:  parentID,
			Depth:     depth,
			Hash:      jsonHash(val),
		}
		// Find span in source.
		node.ValueSpan = findJSONSpan(source, pointer, offsets)
		if key != "" {
			ks := findKeySpan(source, key, node.ValueSpan, offsets)
			node.KeySpan = &ks
		}

		var childIDs []string
		for k, child := range v {
			childPointer := pointer + "/" + escapePointer(k)
			childID := jsonNodeID(childPointer, depth+1)
			childIDs = append(childIDs, childID)
			walkJSON(jast, child, childPointer, nodeID, k, depth+1, source, offsets)
		}
		node.Children = childIDs
		jast.Nodes = append(jast.Nodes, node)

	case []any:
		node := JSONNode{
			ID:        nodeID,
			Kind:      KindJSONArray,
			Pointer:   pointer,
			Key:       key,
			ValueType: "array",
			ParentID:  parentID,
			Depth:     depth,
			Hash:      jsonHash(val),
		}
		node.ValueSpan = findJSONSpan(source, pointer, offsets)
		if key != "" {
			ks := findKeySpan(source, key, node.ValueSpan, offsets)
			node.KeySpan = &ks
		}

		var childIDs []string
		for i, child := range v {
			childPointer := pointer + "/" + strconv.Itoa(i)
			childID := jsonNodeID(childPointer, depth+1)
			childIDs = append(childIDs, childID)
			walkJSON(jast, child, childPointer, nodeID, "", depth+1, source, offsets)
		}
		node.Children = childIDs
		jast.Nodes = append(jast.Nodes, node)

	default:
		// Scalar value (string, number, bool, null).
		valueStr := formatScalar(v)
		valueType := scalarType(v)
		node := JSONNode{
			ID:        nodeID,
			Kind:      KindJSONValue,
			Pointer:   pointer,
			Key:       key,
			Value:     valueStr,
			ValueType: valueType,
			ParentID:  parentID,
			Depth:     depth,
			Hash:      jsonHash(val),
		}
		node.ValueSpan = findJSONSpan(source, pointer, offsets)
		if key != "" {
			ks := findKeySpan(source, key, node.ValueSpan, offsets)
			node.KeySpan = &ks
		}
		jast.Nodes = append(jast.Nodes, node)
	}
}

func jsonNodeID(pointer string, depth int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("json:%s:%d", pointer, depth)))
	return "jn-" + hex.EncodeToString(h[:6])
}

func jsonHash(val any) string {
	data, _ := json.Marshal(val)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func formatScalar(v any) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func scalarType(v any) string {
	if v == nil {
		return "null"
	}
	switch v.(type) {
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	default:
		return "unknown"
	}
}

func escapePointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

func jsonLineOffsets(source string) []int {
	offsets := []int{0}
	for i, ch := range source {
		if ch == '\n' {
			offsets = append(offsets, i+1)
		}
	}
	return offsets
}

func byteToLine(offset int, lineOffsets []int) int {
	for i := len(lineOffsets) - 1; i >= 0; i-- {
		if offset >= lineOffsets[i] {
			return i + 1
		}
	}
	return 1
}

// findJSONSpan locates a JSON pointer's approximate position in source.
func findJSONSpan(source string, pointer string, offsets []int) JSONSpan {
	if pointer == "" {
		return JSONSpan{StartByte: 0, EndByte: len(source), StartLine: 1, EndLine: byteToLine(len(source)-1, offsets)}
	}

	// Extract the last segment as a search key.
	parts := strings.Split(pointer, "/")
	lastSeg := parts[len(parts)-1]
	lastSeg = strings.ReplaceAll(lastSeg, "~1", "/")
	lastSeg = strings.ReplaceAll(lastSeg, "~0", "~")

	// Try to find the key in source.
	searchKey := `"` + lastSeg + `"`
	idx := strings.Index(source, searchKey)
	if idx >= 0 {
		// Find the value after the colon.
		colonIdx := strings.Index(source[idx:], ":")
		if colonIdx >= 0 {
			valStart := idx + colonIdx + 1
			return JSONSpan{
				StartByte: idx,
				EndByte:   minInt(valStart+50, len(source)),
				StartLine: byteToLine(idx, offsets),
				EndLine:   byteToLine(minInt(valStart+50, len(source)-1), offsets),
			}
		}
	}

	// Array index — search by position.
	if _, err := strconv.Atoi(lastSeg); err == nil {
		return JSONSpan{StartByte: 0, EndByte: len(source), StartLine: 1, EndLine: byteToLine(len(source)-1, offsets)}
	}

	return JSONSpan{StartByte: 0, EndByte: len(source), StartLine: 1, EndLine: byteToLine(len(source)-1, offsets)}
}

func findKeySpan(source string, key string, valueSpan JSONSpan, offsets []int) JSONSpan {
	searchKey := `"` + key + `"`
	// Search near the value span.
	searchStart := maxInt(0, valueSpan.StartByte-len(searchKey)-5)
	searchArea := source[searchStart:]
	idx := strings.Index(searchArea, searchKey)
	if idx >= 0 {
		absStart := searchStart + idx
		absEnd := absStart + len(searchKey)
		return JSONSpan{
			StartByte: absStart,
			EndByte:   absEnd,
			StartLine: byteToLine(absStart, offsets),
			EndLine:   byteToLine(absEnd, offsets),
		}
	}
	return JSONSpan{StartByte: valueSpan.StartByte, EndByte: valueSpan.StartByte, StartLine: valueSpan.StartLine, EndLine: valueSpan.StartLine}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// --- Adapter Registry ---

// FormatAdapter parses a source string into a CAST.
type FormatAdapter func(source string) (CAST, error)

var adapterRegistry = map[string]FormatAdapter{}

// RegisterAdapter registers a format adapter by name.
func RegisterAdapter(name string, adapter FormatAdapter) {
	adapterRegistry[name] = adapter
}

// GetAdapter returns a registered adapter by name.
func GetAdapter(name string) (FormatAdapter, bool) {
	a, ok := adapterRegistry[name]
	return a, ok
}

// ListAdapters returns the names of all registered adapters.
func ListAdapters() []string {
	names := make([]string, 0, len(adapterRegistry))
	for name := range adapterRegistry {
		names = append(names, name)
	}
	return names
}

func init() {
	RegisterAdapter("markdown", func(source string) (CAST, error) {
		return ParseMarkdown(source), nil
	})

	RegisterAdapter("json", func(source string) (CAST, error) {
		jast, err := ParseJSON(source)
		if err != nil {
			return CAST{}, err
		}
		return jastToCAST(jast), nil
	})
}

// jastToCAST converts a JAST to the generic CAST format.
func jastToCAST(jast JAST) CAST {
	nodes := make([]CNode, 0, len(jast.Nodes))
	for _, jn := range jast.Nodes {
		text := jn.Value
		if text == "" && jn.Key != "" {
			text = jn.Key
		}
		props := map[string]string{
			"pointer":    jn.Pointer,
			"value_type": jn.ValueType,
		}
		if jn.Key != "" {
			props["key"] = jn.Key
		}
		if jn.Value != "" {
			props["value"] = jn.Value
		}

		var children []string
		children = append(children, jn.Children...)

		nodes = append(nodes, CNode{
			ID:       jn.ID,
			Kind:     jn.Kind,
			Text:     text,
			ParentID: jn.ParentID,
			Children: children,
			Level:    jn.Depth,
			Hash:     jn.Hash,
			Props:    props,
			Span:     Span{StartLine: jn.ValueSpan.StartLine, EndLine: jn.ValueSpan.EndLine},
		})
	}
	return CAST{
		Root:       jast.Root,
		Nodes:      nodes,
		SourceHash: jast.SourceHash,
	}
}

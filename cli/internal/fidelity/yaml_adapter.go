package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// YAMLNodeKind classifies a YAML AST node.
type YAMLNodeKind string

const (
	YAMLDocument     YAMLNodeKind = "document"
	YAMLMapping      YAMLNodeKind = "mapping"
	YAMLMappingKey   YAMLNodeKind = "mapping_key"
	YAMLMappingValue YAMLNodeKind = "mapping_value"
	YAMLSequence     YAMLNodeKind = "sequence"
	YAMLSequenceItem YAMLNodeKind = "sequence_item"
	YAMLScalar       YAMLNodeKind = "scalar"
	YAMLComment      YAMLNodeKind = "comment"
	YAMLBlank        YAMLNodeKind = "blank"
)

// YAMLASTNode is a single node in the YAML AST.
type YAMLASTNode struct {
	ID       string       `json:"id"`
	Kind     YAMLNodeKind `json:"kind"`
	Key      string       `json:"key,omitempty"`
	Value    string       `json:"value,omitempty"`
	Indent   int          `json:"indent"`
	Span     SpanInfo     `json:"span"`
	Hash     string       `json:"hash"`
	ParentID string       `json:"parent_id,omitempty"`
	Children []string     `json:"children,omitempty"`
}

// YAMLAST is the parsed AST for a YAML file.
type YAMLAST struct {
	SourceHash string        `json:"source_hash"`
	NodeCount  int           `json:"node_count"`
	Nodes      []YAMLASTNode `json:"nodes"`
	Root       string        `json:"root"`
}

// ParseYAMLAST parses YAML source into an AST with per-node spans.
func ParseYAMLAST(source []byte, filename string) YAMLAST {
	content := string(source)
	lines := strings.Split(content, "\n")

	srcHash := hashBytes(source)
	rootID := yamlNodeID("document", 0, 0)

	ast := YAMLAST{
		SourceHash: srcHash,
		Root:       rootID,
	}

	root := YAMLASTNode{
		ID:   rootID,
		Kind: YAMLDocument,
		Span: SpanInfo{
			ID: rootID, NodeType: string(YAMLDocument),
			StartLine: 1, EndLine: len(lines),
			ByteOff: 0, ByteLen: len(source),
		},
		Hash: srcHash,
	}
	ast.Nodes = append(ast.Nodes, root)

	stack := []yamlStackEntry{{id: rootID, indent: -1}}

	offset := 0
	for i, line := range lines {
		lineLen := len(line)
		if i < len(lines)-1 {
			lineLen++ // newline
		}

		trimmed := strings.TrimSpace(line)
		indent := countIndent(line)

		// Blank line.
		if trimmed == "" {
			node := YAMLASTNode{
				ID:   yamlNodeID("blank", i, i),
				Kind: YAMLBlank,
				Span: SpanInfo{
					ID: yamlNodeID("blank", i, i), NodeType: string(YAMLBlank),
					StartLine: i + 1, EndLine: i + 1,
					ByteOff: offset, ByteLen: len(line),
				},
				Hash:     yamlHashStr(line),
				ParentID: yamlCurrentParent(stack, indent),
			}
			ast.Nodes = append(ast.Nodes, node)
			addYAMLChild(&ast, node.ParentID, node.ID)
			offset += lineLen
			continue
		}

		// Comment.
		if strings.HasPrefix(trimmed, "#") {
			node := YAMLASTNode{
				ID:    yamlNodeID("comment", i, i),
				Kind:  YAMLComment,
				Value: trimmed,
				Indent: indent,
				Span: SpanInfo{
					ID: yamlNodeID("comment", i, i), NodeType: string(YAMLComment),
					StartLine: i + 1, EndLine: i + 1,
					ByteOff: offset, ByteLen: len(line),
				},
				Hash:     yamlHashStr(line),
				ParentID: yamlCurrentParent(stack, indent),
			}
			ast.Nodes = append(ast.Nodes, node)
			addYAMLChild(&ast, node.ParentID, node.ID)
			offset += lineLen
			continue
		}

		// Sequence item: "- value" or "- key: value".
		if strings.HasPrefix(trimmed, "- ") {
			itemValue := strings.TrimPrefix(trimmed, "- ")
			parent := yamlCurrentParent(stack, indent)

			node := YAMLASTNode{
				ID:     yamlNodeID("seq_item", i, i),
				Kind:   YAMLSequenceItem,
				Value:  itemValue,
				Indent: indent,
				Span: SpanInfo{
					ID: yamlNodeID("seq_item", i, i), NodeType: string(YAMLSequenceItem),
					StartLine: i + 1, EndLine: i + 1,
					ByteOff: offset, ByteLen: len(line),
				},
				Hash:     yamlHashStr(line),
				ParentID: parent,
			}

			// If item contains "key: value", split.
			if colonIdx := strings.Index(itemValue, ": "); colonIdx >= 0 {
				node.Key = strings.TrimSpace(itemValue[:colonIdx])
				node.Value = strings.TrimSpace(itemValue[colonIdx+2:])
			}

			ast.Nodes = append(ast.Nodes, node)
			addYAMLChild(&ast, parent, node.ID)

			// Push as potential parent for nested content.
			stack = pushStack(stack, yamlStackEntry{id: node.ID, indent: indent + 2})
			offset += lineLen
			continue
		}

		// Mapping: "key: value" or "key:".
		if colonIdx := strings.Index(trimmed, ":"); colonIdx >= 0 {
			key := strings.TrimSpace(trimmed[:colonIdx])
			rest := ""
			if colonIdx+1 < len(trimmed) {
				rest = strings.TrimSpace(trimmed[colonIdx+1:])
			}

			parent := yamlCurrentParent(stack, indent)

			if rest == "" {
				// Mapping container (key with nested children).
				node := YAMLASTNode{
					ID:     yamlNodeID("mapping", i, i),
					Kind:   YAMLMapping,
					Key:    key,
					Indent: indent,
					Span: SpanInfo{
						ID: yamlNodeID("mapping", i, i), NodeType: string(YAMLMapping),
						StartLine: i + 1, EndLine: i + 1,
						ByteOff: offset, ByteLen: len(line),
					},
					Hash:     yamlHashStr(line),
					ParentID: parent,
				}
				ast.Nodes = append(ast.Nodes, node)
				addYAMLChild(&ast, parent, node.ID)
				stack = pushStack(stack, yamlStackEntry{id: node.ID, indent: indent})
			} else {
				// Key-value pair.
				kvID := yamlNodeID("kv", i, i)
				node := YAMLASTNode{
					ID:     kvID,
					Kind:   YAMLMappingValue,
					Key:    key,
					Value:  rest,
					Indent: indent,
					Span: SpanInfo{
						ID: kvID, NodeType: string(YAMLMappingValue),
						StartLine: i + 1, EndLine: i + 1,
						ByteOff: offset, ByteLen: len(line),
					},
					Hash:     yamlHashStr(line),
					ParentID: parent,
				}
				ast.Nodes = append(ast.Nodes, node)
				addYAMLChild(&ast, parent, node.ID)
				stack = pushStack(stack, yamlStackEntry{id: kvID, indent: indent})
			}

			offset += lineLen
			continue
		}

		// Scalar continuation or unstructured line.
		parent := yamlCurrentParent(stack, indent)
		node := YAMLASTNode{
			ID:     yamlNodeID("scalar", i, i),
			Kind:   YAMLScalar,
			Value:  trimmed,
			Indent: indent,
			Span: SpanInfo{
				ID: yamlNodeID("scalar", i, i), NodeType: string(YAMLScalar),
				StartLine: i + 1, EndLine: i + 1,
				ByteOff: offset, ByteLen: len(line),
			},
			Hash:     yamlHashStr(line),
			ParentID: parent,
		}
		ast.Nodes = append(ast.Nodes, node)
		addYAMLChild(&ast, parent, node.ID)
		offset += lineLen
	}

	ast.NodeCount = len(ast.Nodes)
	return ast
}

// YAMLASTAdapter wraps ParseYAMLAST into the Adapter interface.
type YAMLASTAdapter struct{}

func (YAMLASTAdapter) Name() string         { return "yaml-ast" }
func (YAMLASTAdapter) Extensions() []string { return []string{".yaml", ".yml"} }

func (y YAMLASTAdapter) Parse(source []byte, filename string) (ParseResult, error) {
	ast := ParseYAMLAST(source, filename)
	spans := make([]SpanInfo, 0, len(ast.Nodes))
	for _, n := range ast.Nodes {
		if n.Kind == YAMLBlank {
			continue
		}
		spans = append(spans, n.Span)
	}
	return ParseResult{Format: "yaml-ast", NodeCount: ast.NodeCount, Spans: spans}, nil
}

func (YAMLASTAdapter) Validate(source []byte, filename string) ValidationResult {
	s := strings.TrimSpace(string(source))
	if s == "" {
		return ValidationResult{Valid: true}
	}
	if strings.Contains(s, "\t") {
		return ValidationResult{Valid: false, Findings: []string{"YAML must not contain tabs"}}
	}
	return ValidationResult{Valid: true}
}

func (y YAMLASTAdapter) Spans(source []byte, filename string) ([]SpanInfo, error) {
	result, err := y.Parse(source, filename)
	return result.Spans, err
}

type yamlStackEntry struct {
	id     string
	indent int
}

func countIndent(line string) int {
	n := 0
	for _, r := range line {
		if r == ' ' {
			n++
		} else {
			break
		}
	}
	return n
}

func yamlCurrentParent(stack []yamlStackEntry, indent int) string {
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].indent < indent {
			return stack[i].id
		}
	}
	if len(stack) > 0 {
		return stack[0].id
	}
	return ""
}

func pushStack(stack []yamlStackEntry, entry yamlStackEntry) []yamlStackEntry {
	// Pop entries at same or deeper indent.
	result := stack
	for len(result) > 1 && result[len(result)-1].indent >= entry.indent {
		result = result[:len(result)-1]
	}
	return append(result, entry)
}

func addYAMLChild(ast *YAMLAST, parentID, childID string) {
	for i := range ast.Nodes {
		if ast.Nodes[i].ID == parentID {
			ast.Nodes[i].Children = append(ast.Nodes[i].Children, childID)
			return
		}
	}
}

func yamlNodeID(prefix string, startLine, endLine int) string {
	raw := fmt.Sprintf("yaml:%s:%d:%d", prefix, startLine, endLine)
	h := sha256.Sum256([]byte(raw))
	return "Y-" + strings.ToUpper(hex.EncodeToString(h[:6]))
}

func hashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

func yamlHashStr(s string) string {
	return hashBytes([]byte(s))
}

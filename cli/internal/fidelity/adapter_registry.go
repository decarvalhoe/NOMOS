package fidelity

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ParseResult is the output of an adapter's Parse method.
type ParseResult struct {
	Format    string     `json:"format"`
	NodeCount int        `json:"node_count"`
	Spans     []SpanInfo `json:"spans"`
	Errors    []string   `json:"errors,omitempty"`
}

// SpanInfo describes a source span produced by parsing.
type SpanInfo struct {
	ID        string `json:"id"`
	NodeType  string `json:"node_type"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	ByteOff   int    `json:"byte_offset"`
	ByteLen   int    `json:"byte_length"`
}

// ValidationResult is the output of an adapter's Validate method.
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Findings []string `json:"findings,omitempty"`
}

// AdapterKit is the MANDATORY capability kit every adapter ships (VRC-33
// #570, C5; doc 14 principle 4 « capability versionnée avec limites
// déclarées »): the claim boundary, the declared claim level, the honest
// taxonomy of what the adapter REFUSES, and the gate fixtures that prove the
// behavior. Registration without a complete kit fails closed — a format
// claim with no mapped evidence cannot enter the registry.
type AdapterKit struct {
	// ClaimBoundary states what this adapter's output may and may not be
	// used to claim — one honest sentence, never empty.
	ClaimBoundary string
	// ClaimLevel names the rung actually held (e.g. "structural-spans",
	// "born-digital-text", "declared-placeholder").
	ClaimLevel string
	// UnsupportedKinds is the refusal taxonomy: what the adapter does NOT
	// handle and how that surfaces. Nothing parses everything — an empty
	// list is a lie, not a feature.
	UnsupportedKinds []string
	// GateFixtures anchor the kit to executable evidence: either a
	// repo-relative fixture path, or "test://<file>#<TestSymbol>" naming
	// the gate test that generates/exercises the fixture in-test. The
	// registry conformance test resolves every reference.
	GateFixtures []string
}

// Adapter is the interface every format parser must implement.
type Adapter interface {
	// Name returns the adapter's unique identifier.
	Name() string
	// Extensions returns file extensions this adapter handles (e.g. ".md").
	Extensions() []string
	// Kit returns the adapter's mandatory capability kit (VRC-33).
	Kit() AdapterKit
	// Parse parses raw source content and returns structural nodes with spans.
	Parse(source []byte, filename string) (ParseResult, error)
	// Validate checks the source for structural correctness.
	Validate(source []byte, filename string) ValidationResult
	// Spans extracts source spans without full parsing (fast path).
	Spans(source []byte, filename string) ([]SpanInfo, error)
}

// validateKit is the C5 gate: every registered adapter carries a complete
// kit. Each missing piece is named — fail closed, never best-effort.
func validateKit(name string, kit AdapterKit) error {
	if strings.TrimSpace(kit.ClaimBoundary) == "" {
		return fmt.Errorf("adapter %q: kit has no claim boundary — an adapter without a boundary claims everything", name)
	}
	if strings.TrimSpace(kit.ClaimLevel) == "" {
		return fmt.Errorf("adapter %q: kit declares no claim level", name)
	}
	if len(kit.UnsupportedKinds) == 0 {
		return fmt.Errorf("adapter %q: kit declares nothing unsupported — nothing parses everything", name)
	}
	for _, kind := range kit.UnsupportedKinds {
		if strings.TrimSpace(kind) == "" {
			return fmt.Errorf("adapter %q: kit has a blank unsupported kind", name)
		}
	}
	if len(kit.GateFixtures) == 0 {
		return fmt.Errorf("adapter %q: kit names no gate fixtures — the claim is not mapped to evidence", name)
	}
	for _, ref := range kit.GateFixtures {
		if strings.TrimSpace(ref) == "" {
			return fmt.Errorf("adapter %q: kit has a blank gate-fixture reference", name)
		}
	}
	return nil
}

// Registry holds registered adapters and resolves them by extension.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Adapter // name → adapter
	extMap   map[string]string  // ".ext" → adapter name
}

// NewRegistry creates an empty adapter registry.
func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[string]Adapter),
		extMap:   make(map[string]string),
	}
}

// Register adds an adapter to the registry. Returns error if the name
// or any extension is already claimed.
func (r *Registry) Register(a Adapter) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := a.Name()
	if name == "" {
		return fmt.Errorf("adapter name is empty")
	}
	if _, exists := r.adapters[name]; exists {
		return fmt.Errorf("adapter %q already registered", name)
	}
	// VRC-33 (C5): no kit, no registration.
	if err := validateKit(name, a.Kit()); err != nil {
		return err
	}

	for _, ext := range a.Extensions() {
		ext = normalizeExt(ext)
		if owner, claimed := r.extMap[ext]; claimed {
			return fmt.Errorf("extension %q already claimed by %q", ext, owner)
		}
	}

	r.adapters[name] = a
	for _, ext := range a.Extensions() {
		r.extMap[normalizeExt(ext)] = name
	}
	return nil
}

// Lookup returns the adapter for a given name.
func (r *Registry) Lookup(name string) (Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[name]
	return a, ok
}

// ForFile returns the adapter that handles the given filename's extension.
func (r *Registry) ForFile(filename string) (Adapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ext := normalizeExt(filepath.Ext(filename))
	name, ok := r.extMap[ext]
	if !ok {
		return nil, false
	}
	return r.adapters[name], true
}

// Names returns all registered adapter names, sorted.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.adapters))
	for n := range r.adapters {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// SupportedExtensions returns all registered extensions, sorted.
func (r *Registry) SupportedExtensions() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	exts := make([]string, 0, len(r.extMap))
	for e := range r.extMap {
		exts = append(exts, e)
	}
	sort.Strings(exts)
	return exts
}

// Count returns the number of registered adapters.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.adapters)
}

func normalizeExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext != "" && ext[0] != '.' {
		ext = "." + ext
	}
	return ext
}

// --- Built-in adapters ---

// MarkdownAdapter parses Markdown files.
type MarkdownAdapter struct{}

func (MarkdownAdapter) Name() string           { return "markdown" }
func (MarkdownAdapter) Extensions() []string   { return []string{".md", ".mdx"} }

func (MarkdownAdapter) Kit() AdapterKit {
	return AdapterKit{
		ClaimBoundary:    "Line-level structural spans only — no claim of full CommonMark/GFM semantics; semantic atomization lives in the portable atomizer, not here.",
		ClaimLevel:       "structural-spans",
		UnsupportedKinds: []string{"nested_block_structure", "embedded_html_semantics", "reference_link_resolution"},
		GateFixtures: []string{
			"test://cli/internal/fidelity/adapter_registry_test.go#TestMarkdownSpans",
			"test://cli/internal/fidelity/adapter_registry_test.go#TestMarkdownValidateNoHeading",
		},
	}
}

func (m MarkdownAdapter) Parse(source []byte, filename string) (ParseResult, error) {
	spans, err := m.Spans(source, filename)
	if err != nil {
		return ParseResult{}, err
	}
	return ParseResult{
		Format:    "markdown",
		NodeCount: len(spans),
		Spans:     spans,
	}, nil
}

func (MarkdownAdapter) Validate(source []byte, filename string) ValidationResult {
	if len(source) == 0 {
		return ValidationResult{Valid: true}
	}
	// Basic structural check: at least one heading.
	hasHeading := false
	for _, line := range strings.Split(string(source), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			hasHeading = true
			break
		}
	}
	if !hasHeading {
		return ValidationResult{Valid: false, Findings: []string{"no heading found"}}
	}
	return ValidationResult{Valid: true}
}

func (MarkdownAdapter) Spans(source []byte, filename string) ([]SpanInfo, error) {
	lines := strings.Split(string(source), "\n")
	var spans []SpanInfo
	offset := 0
	for i, line := range lines {
		lineLen := len(line)
		if i < len(lines)-1 {
			lineLen++ // \n
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			offset += lineLen
			continue
		}
		nodeType := "paragraph"
		if strings.HasPrefix(trimmed, "#") {
			nodeType = "heading"
		} else if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "1.") {
			nodeType = "list_item"
		} else if strings.HasPrefix(trimmed, "```") {
			nodeType = "code_fence"
		} else if strings.HasPrefix(trimmed, "|") {
			nodeType = "table_row"
		}
		spans = append(spans, SpanInfo{
			ID:        fmt.Sprintf("md:%d", i+1),
			NodeType:  nodeType,
			StartLine: i + 1,
			EndLine:   i + 1,
			ByteOff:   offset,
			ByteLen:   len(line),
		})
		offset += lineLen
	}
	return spans, nil
}

// YAMLAdapter parses YAML files.
type YAMLAdapter struct{}

func (YAMLAdapter) Name() string           { return "yaml" }
func (YAMLAdapter) Extensions() []string   { return []string{".yaml", ".yml"} }

func (YAMLAdapter) Kit() AdapterKit {
	return AdapterKit{
		ClaimBoundary:    "Line-level structural spans and a tabs check only — no claim of YAML semantic parsing, anchor/alias resolution, or schema validation.",
		ClaimLevel:       "structural-spans",
		UnsupportedKinds: []string{"anchor_alias_resolution", "multi_document_streams", "semantic_schema_validation"},
		GateFixtures: []string{
			"test://cli/internal/fidelity/adapter_registry_test.go#TestYAMLParse",
			"test://cli/internal/fidelity/adapter_registry_test.go#TestYAMLValidateWithTabs",
		},
	}
}

func (y YAMLAdapter) Parse(source []byte, filename string) (ParseResult, error) {
	spans, err := y.Spans(source, filename)
	if err != nil {
		return ParseResult{}, err
	}
	return ParseResult{Format: "yaml", NodeCount: len(spans), Spans: spans}, nil
}

func (YAMLAdapter) Validate(source []byte, filename string) ValidationResult {
	s := strings.TrimSpace(string(source))
	if s == "" {
		return ValidationResult{Valid: true}
	}
	if strings.Contains(s, "\t") {
		return ValidationResult{Valid: false, Findings: []string{"YAML should not contain tabs"}}
	}
	return ValidationResult{Valid: true}
}

func (YAMLAdapter) Spans(source []byte, filename string) ([]SpanInfo, error) {
	lines := strings.Split(string(source), "\n")
	var spans []SpanInfo
	offset := 0
	for i, line := range lines {
		lineLen := len(line)
		if i < len(lines)-1 {
			lineLen++
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			offset += lineLen
			continue
		}
		nodeType := "mapping"
		if strings.HasPrefix(trimmed, "- ") {
			nodeType = "sequence_item"
		}
		spans = append(spans, SpanInfo{
			ID: fmt.Sprintf("yaml:%d", i+1), NodeType: nodeType,
			StartLine: i + 1, EndLine: i + 1, ByteOff: offset, ByteLen: len(line),
		})
		offset += lineLen
	}
	return spans, nil
}

// JSONAdapter parses JSON files.
type JSONAdapter struct{}

func (JSONAdapter) Name() string           { return "json" }
func (JSONAdapter) Extensions() []string   { return []string{".json"} }

func (JSONAdapter) Kit() AdapterKit {
	return AdapterKit{
		ClaimBoundary:    "Line-level structural spans and a root-token check only — no claim of JSON grammar validation or schema conformance.",
		ClaimLevel:       "structural-spans",
		UnsupportedKinds: []string{"grammar_validation", "schema_conformance", "streaming_documents"},
		GateFixtures: []string{
			"test://cli/internal/fidelity/adapter_registry_test.go#TestJSONParse",
			"test://cli/internal/fidelity/adapter_registry_test.go#TestJSONValidateInvalid",
		},
	}
}

func (j JSONAdapter) Parse(source []byte, filename string) (ParseResult, error) {
	spans, err := j.Spans(source, filename)
	if err != nil {
		return ParseResult{}, err
	}
	return ParseResult{Format: "json", NodeCount: len(spans), Spans: spans}, nil
}

func (JSONAdapter) Validate(source []byte, filename string) ValidationResult {
	s := strings.TrimSpace(string(source))
	if s == "" {
		return ValidationResult{Valid: true}
	}
	if (s[0] != '{' && s[0] != '[') {
		return ValidationResult{Valid: false, Findings: []string{"JSON must start with { or ["}}
	}
	return ValidationResult{Valid: true}
}

func (JSONAdapter) Spans(source []byte, filename string) ([]SpanInfo, error) {
	lines := strings.Split(string(source), "\n")
	var spans []SpanInfo
	offset := 0
	for i, line := range lines {
		lineLen := len(line)
		if i < len(lines)-1 {
			lineLen++
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			offset += lineLen
			continue
		}
		spans = append(spans, SpanInfo{
			ID: fmt.Sprintf("json:%d", i+1), NodeType: "value",
			StartLine: i + 1, EndLine: i + 1, ByteOff: offset, ByteLen: len(line),
		})
		offset += lineLen
	}
	return spans, nil
}

// PlaceholderAdapter is a stub for formats not yet implemented (DOCX, PDF).
type PlaceholderAdapter struct {
	AdapterName string
	Exts        []string
}

func (p PlaceholderAdapter) Name() string         { return p.AdapterName }
func (p PlaceholderAdapter) Extensions() []string { return p.Exts }

func (p PlaceholderAdapter) Kit() AdapterKit {
	return AdapterKit{
		ClaimBoundary:    "Nothing is claimable: every call refuses loudly. The placeholder exists so the extension is RESERVED, never silently mis-parsed.",
		ClaimLevel:       "declared-placeholder",
		UnsupportedKinds: []string{"all_content"},
		GateFixtures: []string{
			"test://cli/internal/fidelity/adapter_registry_test.go#TestPlaceholderParseErrors",
			"test://cli/internal/fidelity/adapter_registry_test.go#TestPlaceholderValidate",
		},
	}
}

func (p PlaceholderAdapter) Parse(_ []byte, _ string) (ParseResult, error) {
	return ParseResult{}, fmt.Errorf("adapter %q is not yet implemented", p.AdapterName)
}

func (p PlaceholderAdapter) Validate(_ []byte, _ string) ValidationResult {
	return ValidationResult{Valid: false, Findings: []string{fmt.Sprintf("adapter %q not implemented", p.AdapterName)}}
}

func (p PlaceholderAdapter) Spans(_ []byte, _ string) ([]SpanInfo, error) {
	return nil, fmt.Errorf("adapter %q is not yet implemented", p.AdapterName)
}

// DefaultRegistry returns a registry with all built-in adapters.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(MarkdownAdapter{})
	r.Register(YAMLAdapter{})
	r.Register(JSONAdapter{})
	r.Register(PlaceholderAdapter{AdapterName: "docx", Exts: []string{".docx"}})
	// VRC-30 (#567): the pdf placeholder is gone — born-digital PDF text is a
	// real adapter (claim ladder + unsupported records, see pdf_adapter.go).
	r.Register(PDFAdapter{})
	return r
}

package parse

import (
	"fmt"
	"regexp"
	"strings"
)

var hashPattern = regexp.MustCompile(`^(sha256|sha384|sha512):[A-Fa-f0-9]+$`)

// ParserInfo identifies the parser adapter that produced an AST.
type ParserInfo struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
}

// TextQuoteSelector anchors a source span by surrounding text.
type TextQuoteSelector struct {
	Exact  string `json:"exact" yaml:"exact"`
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	Suffix string `json:"suffix,omitempty" yaml:"suffix,omitempty"`
}

// SourceSpan locates an AST node in a source document.
type SourceSpan struct {
	SourceID    string             `json:"source_id" yaml:"source_id"`
	Path        string             `json:"path" yaml:"path"`
	Hash        string             `json:"hash" yaml:"hash"`
	StartLine   *int               `json:"start_line,omitempty" yaml:"start_line,omitempty"`
	EndLine     *int               `json:"end_line,omitempty" yaml:"end_line,omitempty"`
	StartColumn *int               `json:"start_column,omitempty" yaml:"start_column,omitempty"`
	EndColumn   *int               `json:"end_column,omitempty" yaml:"end_column,omitempty"`
	StartByte   *int               `json:"start_byte,omitempty" yaml:"start_byte,omitempty"`
	EndByte     *int               `json:"end_byte,omitempty" yaml:"end_byte,omitempty"`
	Locator     string             `json:"locator,omitempty" yaml:"locator,omitempty"`
	TextQuote   *TextQuoteSelector `json:"text_quote,omitempty" yaml:"text_quote,omitempty"`
}

// Validate checks whether the span is source-backed and precisely anchored.
func (s SourceSpan) Validate() []string {
	var errs []string
	if strings.TrimSpace(s.SourceID) == "" {
		errs = append(errs, "source_id is required")
	}
	if strings.TrimSpace(s.Path) == "" {
		errs = append(errs, "path is required")
	}
	if !hashPattern.MatchString(s.Hash) {
		errs = append(errs, fmt.Sprintf("hash %q must be sha256/sha384/sha512", s.Hash))
	}
	if !s.hasSelector() {
		errs = append(errs, "at least one source selector is required")
	}
	errs = append(errs, validateRange("line", s.StartLine, s.EndLine, 1)...)
	errs = append(errs, validateRange("column", s.StartColumn, s.EndColumn, 1)...)
	errs = append(errs, validateRange("byte", s.StartByte, s.EndByte, 0)...)
	if s.TextQuote != nil && strings.TrimSpace(s.TextQuote.Exact) == "" {
		errs = append(errs, "text_quote.exact is required when text_quote is present")
	}
	return errs
}

func (s SourceSpan) hasSelector() bool {
	return (s.StartLine != nil && s.EndLine != nil) ||
		(s.StartByte != nil && s.EndByte != nil) ||
		strings.TrimSpace(s.Locator) != "" ||
		(s.TextQuote != nil && strings.TrimSpace(s.TextQuote.Exact) != "")
}

func validateRange(name string, start *int, end *int, min int) []string {
	var errs []string
	if (start == nil) != (end == nil) {
		errs = append(errs, fmt.Sprintf("%s range requires both start and end", name))
		return errs
	}
	if start == nil {
		return errs
	}
	if *start < min || *end < min {
		errs = append(errs, fmt.Sprintf("%s range must be >= %d", name, min))
	}
	if *end < *start {
		errs = append(errs, fmt.Sprintf("%s range end must be >= start", name))
	}
	return errs
}

// ASTNodeType is a parser-neutral source AST node family.
type ASTNodeType string

const (
	NodeRoot             ASTNodeType = "root"
	NodeHeading          ASTNodeType = "heading"
	NodeParagraph        ASTNodeType = "paragraph"
	NodeList             ASTNodeType = "list"
	NodeListItem         ASTNodeType = "list_item"
	NodeTable            ASTNodeType = "table"
	NodeTableRow         ASTNodeType = "table_row"
	NodeTableCell        ASTNodeType = "table_cell"
	NodeBlockQuote       ASTNodeType = "block_quote"
	NodeCodeBlock        ASTNodeType = "code_block"
	NodeThematicBreak    ASTNodeType = "thematic_break"
	NodeLink             ASTNodeType = "link"
	NodeImage            ASTNodeType = "image"
	NodeRawHTML          ASTNodeType = "raw_html"
	NodeUnsupportedBlock ASTNodeType = "unsupported_block"
	NodeUnknownBlock     ASTNodeType = "unknown_block"
)

func (t ASTNodeType) valid() bool {
	switch t {
	case NodeRoot, NodeHeading, NodeParagraph, NodeList, NodeListItem,
		NodeTable, NodeTableRow, NodeTableCell, NodeBlockQuote, NodeCodeBlock,
		NodeThematicBreak, NodeLink, NodeImage, NodeRawHTML,
		NodeUnsupportedBlock, NodeUnknownBlock:
		return true
	default:
		return false
	}
}

// FindingSeverity classifies parse-time findings.
type FindingSeverity string

const (
	SeverityInfo     FindingSeverity = "info"
	SeverityWarning  FindingSeverity = "warning"
	SeverityBlocking FindingSeverity = "blocking"
)

func (s FindingSeverity) valid() bool {
	switch s {
	case SeverityInfo, SeverityWarning, SeverityBlocking:
		return true
	default:
		return false
	}
}

// Finding records parser ambiguity, unsupported content, or loss risk.
type Finding struct {
	Code     string          `json:"code" yaml:"code"`
	Severity FindingSeverity `json:"severity" yaml:"severity"`
	Message  string          `json:"message" yaml:"message"`
}

func (f Finding) validate() []string {
	var errs []string
	if strings.TrimSpace(f.Code) == "" {
		errs = append(errs, "finding code is required")
	}
	if !f.Severity.valid() {
		errs = append(errs, fmt.Sprintf("finding severity %q is invalid", f.Severity))
	}
	if strings.TrimSpace(f.Message) == "" {
		errs = append(errs, "finding message is required")
	}
	return errs
}

// ASTNode is a parser-neutral node produced before semantic structuring.
type ASTNode struct {
	NodeID   string      `json:"node_id" yaml:"node_id"`
	NodeType ASTNodeType `json:"node_type" yaml:"node_type"`
	ParentID string      `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
	Span     SourceSpan  `json:"span" yaml:"span"`
	Title    string      `json:"title,omitempty" yaml:"title,omitempty"`
	Text     string      `json:"text,omitempty" yaml:"text,omitempty"`
	Raw      string      `json:"raw,omitempty" yaml:"raw,omitempty"`
	Hash     string      `json:"hash,omitempty" yaml:"hash,omitempty"`
	Findings []Finding   `json:"findings,omitempty" yaml:"findings,omitempty"`
	Children []ASTNode   `json:"children,omitempty" yaml:"children,omitempty"`
}

func (n ASTNode) validate(path string, seen map[string]struct{}) []string {
	var errs []string
	if strings.TrimSpace(n.NodeID) == "" {
		errs = append(errs, path+": node_id is required")
	} else if _, ok := seen[n.NodeID]; ok {
		errs = append(errs, fmt.Sprintf("duplicate node_id %s", n.NodeID))
	} else {
		seen[n.NodeID] = struct{}{}
	}
	if !n.NodeType.valid() {
		errs = append(errs, fmt.Sprintf("%s: node_type %q is invalid", path, n.NodeType))
	}
	for _, err := range n.Span.Validate() {
		errs = append(errs, path+": span: "+err)
	}
	for i, finding := range n.Findings {
		for _, err := range finding.validate() {
			errs = append(errs, fmt.Sprintf("%s: findings[%d]: %s", path, i, err))
		}
	}
	if (n.NodeType == NodeUnsupportedBlock || n.NodeType == NodeUnknownBlock) && len(n.Findings) == 0 {
		errs = append(errs, fmt.Sprintf("unsupported node %s must include a finding", n.NodeID))
	}
	for i, child := range n.Children {
		childPath := fmt.Sprintf("%s.children[%d]", path, i)
		errs = append(errs, child.validate(childPath, seen)...)
	}
	return errs
}

// DocumentAST is the parser output for one source document.
type DocumentAST struct {
	SourceID string     `json:"source_id" yaml:"source_id"`
	Parser   ParserInfo `json:"parser" yaml:"parser"`
	Root     ASTNode    `json:"root" yaml:"root"`
}

// Validate checks the AST contract before structure or atomization runs.
func (d DocumentAST) Validate() []string {
	var errs []string
	if strings.TrimSpace(d.SourceID) == "" {
		errs = append(errs, "source_id is required")
	}
	if strings.TrimSpace(d.Parser.Name) == "" {
		errs = append(errs, "parser.name is required")
	}
	if strings.TrimSpace(d.Parser.Version) == "" {
		errs = append(errs, "parser.version is required")
	}
	errs = append(errs, d.Root.validate("root", map[string]struct{}{})...)
	if d.Root.NodeType != NodeRoot {
		errs = append(errs, "root node_type must be root")
	}
	return errs
}

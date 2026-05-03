package nomos

import "strings"

// #CanonicalAST defines the fidelity-grade abstract syntax tree
// for Markdown source documents. Every node carries exact source
// spans so that any transformation can be audited back to the
// original byte range.

// #SourceSpan locates a node in its source file with exact
// line, column, and byte offsets.
#SourceSpan: {
	file:        string & strings.MinRunes(1)
	start_line:  int & >=1
	start_col:   int & >=1
	end_line:    int & >=1
	end_col:     int & >=1
	byte_offset: int & >=0
	byte_length: int & >=0

	// Invariant: end must be at or after start.
	end_line: >=start_line
}

// #BlockType enumerates block-level elements.
#BlockType:
	"document" |
	"heading" |
	"paragraph" |
	"list" |
	"list_item" |
	"code_block" |
	"table" |
	"table_row" |
	"table_cell" |
	"metadata" |
	"blockquote" |
	"thematic_break" |
	"blank_line"

// #InlineType enumerates inline-level elements.
#InlineType:
	"text" |
	"emphasis" |
	"strong" |
	"code" |
	"link" |
	"image" |
	"line_break" |
	"html_entity"

// #SpanType enumerates source-span-only markers used for
// provenance tracking without semantic content.
#SpanType:
	"raw_html" |
	"comment" |
	"front_matter" |
	"directive"

// #ASTNodeKind is the union of all node kinds.
#ASTNodeKind: "block" | "inline" | "span"

// #ASTNode is a single node in the canonical AST.
#ASTNode: #BlockNode | #InlineNode | #SpanNode

// #BlockNode represents a block-level element.
#BlockNode: {
	id:          =~"^B-[A-F0-9]{12}$"
	kind:        "block"
	block_type:  #BlockType
	level?:      int & >=1 & <=6
	span:        #SourceSpan
	hash:        =~"^sha256:[A-Fa-f0-9]{64}$"
	parent_id?:  string
	children?:   [...string]
	text?:       string
	raw_text:    string
	props?: {[string]: string}
}

// #InlineNode represents an inline-level element within a block.
#InlineNode: {
	id:           =~"^I-[A-F0-9]{12}$"
	kind:         "inline"
	inline_type:  #InlineType
	span:         #SourceSpan
	hash:         =~"^sha256:[A-Fa-f0-9]{64}$"
	parent_id:    string & strings.MinRunes(1)
	text:         string
	raw_text:     string
	href?:        string
	alt?:         string
	title?:       string
}

// #SpanNode is a source-span marker for non-semantic content.
#SpanNode: {
	id:          =~"^S-[A-F0-9]{12}$"
	kind:        "span"
	span_type:   #SpanType
	span:        #SourceSpan
	hash:        =~"^sha256:[A-Fa-f0-9]{64}$"
	parent_id?:  string
	raw_text:    string
}

// #CanonicalASTDocument is the top-level AST for one source file.
#CanonicalASTDocument: {
	schema_version: string | *"0.1.0"
	format:         "nomos.fidelity-ast.v1"
	source_file:    string & strings.MinRunes(1)
	source_hash:    =~"^sha256:[A-Fa-f0-9]{64}$"
	source_bytes:   int & >=0
	node_count:     int & >=0
	nodes:          [...#ASTNode]

	// Coverage tracking.
	coverage: #CoverageReport

	// Invariant.
	node_count: len(nodes)
}

// #CoverageReport tracks how much of the source file is covered
// by AST nodes.
#CoverageReport: {
	total_bytes:   int & >=0
	covered_bytes: int & >=0
	lost_bytes:    int & >=0
	loss_ratio:    number & >=0 & <=1
	is_lossless:   bool
	lost_spans?:   [...#LostSpanEntry]
}

// #LostSpanEntry identifies a source range not covered by any node.
#LostSpanEntry: {
	start_line: int & >=1
	end_line:   int & >=1
	preview:    string
}

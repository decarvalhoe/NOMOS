package corpus

import (
	"fmt"
	"regexp"
	"strings"
)

// LawbookNodeType enumerates the structural levels of a lawbook.
type LawbookNodeType string

const (
	NodeDocument   LawbookNodeType = "document"
	NodeChapter    LawbookNodeType = "chapter"
	NodeSection    LawbookNodeType = "section"
	NodeSubsection LawbookNodeType = "subsection"
	NodeArticle    LawbookNodeType = "article"
	NodeParagraph  LawbookNodeType = "paragraph"
	NodeAlinea     LawbookNodeType = "alinea"

	// Semantic typed nodes (AQ-326).
	NodeTable     LawbookNodeType = "table"
	NodeCodeBlock LawbookNodeType = "code_block"
	NodeCallout   LawbookNodeType = "callout"
	NodeLink      LawbookNodeType = "link"
	NodeImage     LawbookNodeType = "image"
)

// AllNodeTypes returns all valid lawbook node types in depth order.
func AllNodeTypes() []LawbookNodeType {
	return []LawbookNodeType{
		NodeDocument, NodeChapter, NodeSection, NodeSubsection,
		NodeArticle, NodeParagraph, NodeAlinea,
	}
}

// Depth returns the canonical depth for a node type.
func (t LawbookNodeType) Depth() int {
	switch t {
	case NodeDocument:
		return 0
	case NodeChapter:
		return 1
	case NodeSection:
		return 2
	case NodeSubsection:
		return 3
	case NodeArticle:
		return 4
	case NodeParagraph:
		return 5
	case NodeAlinea:
		return 6
	default:
		return -1
	}
}

// IsSemanticType returns true for typed semantic nodes (no structural depth).
func (t LawbookNodeType) IsSemanticType() bool {
	switch t {
	case NodeTable, NodeCodeBlock, NodeCallout, NodeLink, NodeImage:
		return true
	default:
		return false
	}
}

// IsValid returns true if the node type is recognized.
func (t LawbookNodeType) IsValid() bool {
	return t.Depth() >= 0 || t.IsSemanticType()
}

// LawbookNodeStatus tracks the lifecycle of a node.
type LawbookNodeStatus string

const (
	StatusActive   LawbookNodeStatus = "active"
	StatusAmended  LawbookNodeStatus = "amended"
	StatusRepealed LawbookNodeStatus = "repealed"
	StatusPending  LawbookNodeStatus = "pending"
	StatusNodeDraft LawbookNodeStatus = "draft"
)

// IsValid returns true if the status is recognized.
func (s LawbookNodeStatus) IsValid() bool {
	switch s {
	case StatusActive, StatusAmended, StatusRepealed, StatusPending, StatusNodeDraft:
		return true
	default:
		return false
	}
}

// LawbookPriority indicates processing priority.
type LawbookPriority string

const (
	PriorityCritical LawbookPriority = "critical"
	PriorityHigh     LawbookPriority = "high"
	PriorityMedium   LawbookPriority = "medium"
	PriorityLow      LawbookPriority = "low"
)

// IsValid returns true if the priority is recognized.
func (p LawbookPriority) IsValid() bool {
	switch p {
	case PriorityCritical, PriorityHigh, PriorityMedium, PriorityLow:
		return true
	default:
		return false
	}
}

// LawbookSourceSpan locates a node precisely in its source file.
type LawbookSourceSpan struct {
	File       string `json:"file"        yaml:"file"`
	StartLine  int    `json:"start_line"  yaml:"start_line"`
	StartCol   int    `json:"start_col"   yaml:"start_col"`
	EndLine    int    `json:"end_line"    yaml:"end_line"`
	EndCol     int    `json:"end_col"     yaml:"end_col"`
	ByteOffset int    `json:"byte_offset" yaml:"byte_offset"`
	ByteLength int    `json:"byte_length" yaml:"byte_length"`
}

// IsValid returns true if the span has non-zero line range.
func (s LawbookSourceSpan) IsValid() bool {
	return s.StartLine > 0 && s.EndLine >= s.StartLine
}

// String returns a human-readable location.
func (s LawbookSourceSpan) String() string {
	if s.StartLine == s.EndLine {
		return fmt.Sprintf("%s:%d", s.File, s.StartLine)
	}
	return fmt.Sprintf("%s:%d-%d", s.File, s.StartLine, s.EndLine)
}

// LawbookNode is a single structural node in a lawbook feed.
type LawbookNode struct {
	NodeID        string            `json:"node_id" yaml:"node_id"`
	DocumentID    string            `json:"document_id" yaml:"document_id"`
	NodeType      LawbookNodeType   `json:"node_type" yaml:"node_type"`
	CanonicalRef  string            `json:"canonical_ref" yaml:"canonical_ref"`
	DisplayRef    string            `json:"display_ref" yaml:"display_ref"`
	Depth         int               `json:"depth" yaml:"depth"`
	OrdinalPath   string            `json:"ordinal_path" yaml:"ordinal_path"`
	SourcePath    string            `json:"source_path" yaml:"source_path"`
	SourceHash    string            `json:"source_hash" yaml:"source_hash"`
	Span          LawbookSourceSpan `json:"span" yaml:"span"`
	Status        LawbookNodeStatus `json:"status" yaml:"status"`
	Priority      LawbookPriority   `json:"priority" yaml:"priority"`
	Domain        string            `json:"domain" yaml:"domain"`
	Title         string            `json:"title,omitempty" yaml:"title,omitempty"`
	Text          string            `json:"text,omitempty" yaml:"text,omitempty"`
	ParentID      string            `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
	EffectiveDate string            `json:"effective_date,omitempty" yaml:"effective_date,omitempty"`
	Metadata      map[string]any    `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// SpanCoverageResult reports how well source spans cover the document.
type SpanCoverageResult struct {
	TotalNodes    int     `json:"total_nodes"`
	WithSpan      int     `json:"with_span"`
	WithoutSpan   int     `json:"without_span"`
	TotalBytes    int     `json:"total_bytes"`
	CoveredBytes  int     `json:"covered_bytes"`
	CoverageRatio float64 `json:"coverage_ratio"`
}

// ComputeSpanCoverage calculates span coverage for a set of nodes.
func ComputeSpanCoverage(nodes []LawbookNode, sourceLen int) SpanCoverageResult {
	result := SpanCoverageResult{TotalNodes: len(nodes), TotalBytes: sourceLen}
	coveredBytes := 0
	for _, n := range nodes {
		if n.Span.IsValid() {
			result.WithSpan++
			coveredBytes += n.Span.ByteLength
		} else {
			result.WithoutSpan++
		}
	}
	result.CoveredBytes = coveredBytes
	if sourceLen > 0 {
		result.CoverageRatio = float64(coveredBytes) / float64(sourceLen)
		if result.CoverageRatio > 1.0 {
			result.CoverageRatio = 1.0
		}
	}
	return result
}

// LawbookFeed is a batch of lawbook nodes forming a feed document.
type LawbookFeed struct {
	SchemaVersion string        `json:"schema_version" yaml:"schema_version"`
	FeedID        string        `json:"feed_id" yaml:"feed_id"`
	DocumentID    string        `json:"document_id" yaml:"document_id"`
	Domain        string        `json:"domain" yaml:"domain"`
	GeneratedAt   string        `json:"generated_at" yaml:"generated_at"`
	SourcePath    string        `json:"source_path" yaml:"source_path"`
	SourceHash    string        `json:"source_hash" yaml:"source_hash"`
	NodeCount     int           `json:"node_count" yaml:"node_count"`
	Nodes         []LawbookNode `json:"nodes" yaml:"nodes"`
}

var (
	nodeIDPattern    = regexp.MustCompile(`^[A-Z0-9][A-Z0-9._-]*$`)
	feedIDPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	ordinalPattern   = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)
	hashPattern      = regexp.MustCompile(`^(sha256|sha384|sha512):[A-Fa-f0-9]+$`)
)

// ValidateNode checks a LawbookNode for schema conformance.
func ValidateNode(n LawbookNode) []string {
	var errs []string

	if !nodeIDPattern.MatchString(n.NodeID) {
		errs = append(errs, fmt.Sprintf("node_id %q must match %s", n.NodeID, nodeIDPattern.String()))
	}
	if !nodeIDPattern.MatchString(n.DocumentID) {
		errs = append(errs, fmt.Sprintf("document_id %q must match %s", n.DocumentID, nodeIDPattern.String()))
	}
	if !n.NodeType.IsValid() {
		errs = append(errs, fmt.Sprintf("node_type %q is not valid", n.NodeType))
	}
	if strings.TrimSpace(n.CanonicalRef) == "" {
		errs = append(errs, "canonical_ref is required")
	}
	if strings.TrimSpace(n.DisplayRef) == "" {
		errs = append(errs, "display_ref is required")
	}
	if n.Depth < 0 || n.Depth > 7 {
		errs = append(errs, fmt.Sprintf("depth %d must be 0-7", n.Depth))
	}
	if n.NodeType.IsValid() && !n.NodeType.IsSemanticType() && n.Depth != n.NodeType.Depth() {
		errs = append(errs, fmt.Sprintf("depth %d does not match node_type %s (expected %d)", n.Depth, n.NodeType, n.NodeType.Depth()))
	}
	if !ordinalPattern.MatchString(n.OrdinalPath) {
		errs = append(errs, fmt.Sprintf("ordinal_path %q must match %s", n.OrdinalPath, ordinalPattern.String()))
	}
	if !hashPattern.MatchString(n.SourceHash) {
		errs = append(errs, fmt.Sprintf("source_hash %q must match %s", n.SourceHash, hashPattern.String()))
	}
	if !n.Status.IsValid() {
		errs = append(errs, fmt.Sprintf("status %q is not valid", n.Status))
	}
	if !n.Priority.IsValid() {
		errs = append(errs, fmt.Sprintf("priority %q is not valid", n.Priority))
	}
	if strings.TrimSpace(n.Domain) == "" {
		errs = append(errs, "domain is required")
	}

	return errs
}

// ValidateFeed checks a LawbookFeed for schema conformance.
func ValidateFeed(f LawbookFeed) []string {
	var errs []string

	if !feedIDPattern.MatchString(f.FeedID) {
		errs = append(errs, fmt.Sprintf("feed_id %q must match %s", f.FeedID, feedIDPattern.String()))
	}
	if !nodeIDPattern.MatchString(f.DocumentID) {
		errs = append(errs, fmt.Sprintf("document_id %q must match %s", f.DocumentID, nodeIDPattern.String()))
	}
	if strings.TrimSpace(f.Domain) == "" {
		errs = append(errs, "domain is required")
	}
	if strings.TrimSpace(f.GeneratedAt) == "" {
		errs = append(errs, "generated_at is required")
	}
	if !hashPattern.MatchString(f.SourceHash) {
		errs = append(errs, fmt.Sprintf("source_hash %q must match %s", f.SourceHash, hashPattern.String()))
	}
	if f.NodeCount != len(f.Nodes) {
		errs = append(errs, fmt.Sprintf("node_count %d does not match len(nodes) %d", f.NodeCount, len(f.Nodes)))
	}

	for i, node := range f.Nodes {
		nodeErrs := ValidateNode(node)
		for _, e := range nodeErrs {
			errs = append(errs, fmt.Sprintf("nodes[%d]: %s", i, e))
		}
	}

	return errs
}

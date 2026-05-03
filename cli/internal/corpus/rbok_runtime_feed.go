package corpus

import (
	"fmt"
	"strings"
)

// CorpusLayer identifies the source layer in realisons-business.
type CorpusLayer string

const (
	LayerRBOK      CorpusLayer = "01_rbok"
	LayerParcours  CorpusLayer = "02_parcours"
	LayerWorkbooks CorpusLayer = "03_workbooks"
	LayerDoctrine  CorpusLayer = "04_doctrine"
	LayerArchive   CorpusLayer = "99_archive"
)

// IsValid returns true if the layer is recognized.
func (l CorpusLayer) IsValid() bool {
	switch l {
	case LayerRBOK, LayerParcours, LayerWorkbooks, LayerDoctrine, LayerArchive:
		return true
	default:
		return false
	}
}

// AuthorityLevel classifies the normative weight of a node.
type AuthorityLevel string

const (
	AuthBinding       AuthorityLevel = "binding"
	AuthRegulatory    AuthorityLevel = "regulatory"
	AuthGuidance      AuthorityLevel = "guidance"
	AuthInformational AuthorityLevel = "informational"
	AuthInternal      AuthorityLevel = "internal"
	AuthDeprecated    AuthorityLevel = "deprecated"
)

// IsValid returns true if the authority level is recognized.
func (a AuthorityLevel) IsValid() bool {
	switch a {
	case AuthBinding, AuthRegulatory, AuthGuidance, AuthInformational, AuthInternal, AuthDeprecated:
		return true
	default:
		return false
	}
}

// RuntimeFeedNode extends the lawbook node with multi-layer metadata.
type RuntimeFeedNode struct {
	NodeID         string         `json:"node_id" yaml:"node_id"`
	DocumentID     string         `json:"document_id" yaml:"document_id"`
	CanonicalRef   string         `json:"canonical_ref" yaml:"canonical_ref"`
	DisplayRef     string         `json:"display_ref" yaml:"display_ref"`
	SourcePath     string         `json:"source_path" yaml:"source_path"`
	SourceHash     string         `json:"source_hash" yaml:"source_hash"`
	Status         LawbookNodeStatus `json:"status" yaml:"status"`
	Priority       LawbookPriority   `json:"priority" yaml:"priority"`
	Domain         string         `json:"domain" yaml:"domain"`
	Layer          CorpusLayer    `json:"layer" yaml:"layer"`
	AuthorityLevel AuthorityLevel `json:"authority_level" yaml:"authority_level"`
	NodeType       string         `json:"node_type" yaml:"node_type"`
	Depth          int            `json:"depth" yaml:"depth"`
	OrdinalPath    string         `json:"ordinal_path,omitempty" yaml:"ordinal_path,omitempty"`
	ParentID       string         `json:"parent_id,omitempty" yaml:"parent_id,omitempty"`
	Title          string         `json:"title,omitempty" yaml:"title,omitempty"`
	Text           string         `json:"text,omitempty" yaml:"text,omitempty"`
	EffectiveDate  string         `json:"effective_date,omitempty" yaml:"effective_date,omitempty"`

	// Parcours-specific.
	PredecessorIDs []string `json:"predecessor_ids,omitempty" yaml:"predecessor_ids,omitempty"`
	SuccessorIDs   []string `json:"successor_ids,omitempty" yaml:"successor_ids,omitempty"`
	GateCriteria   string   `json:"gate_criteria,omitempty" yaml:"gate_criteria,omitempty"`

	// Workbook-specific.
	RefType    string `json:"ref_type,omitempty" yaml:"ref_type,omitempty"`
	TargetURL  string `json:"target_url,omitempty" yaml:"target_url,omitempty"`
	TargetHash string `json:"target_hash,omitempty" yaml:"target_hash,omitempty"`

	Metadata map[string]any `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// LayerSummary provides per-layer aggregate counts.
type LayerSummary struct {
	NodeCount          int            `json:"node_count" yaml:"node_count"`
	DocumentCount      int            `json:"document_count" yaml:"document_count"`
	AuthorityBreakdown map[string]int `json:"authority_breakdown" yaml:"authority_breakdown"`
}

// RuntimeFeed is the multi-layer feed combining all corpus layers.
type RuntimeFeed struct {
	SchemaVersion string                  `json:"schema_version" yaml:"schema_version"`
	FeedFormat    string                  `json:"feed_format" yaml:"feed_format"`
	FeedID        string                  `json:"feed_id" yaml:"feed_id"`
	CorpusID      string                  `json:"corpus_id" yaml:"corpus_id"`
	Domain        string                  `json:"domain" yaml:"domain"`
	GeneratedAt   string                  `json:"generated_at" yaml:"generated_at"`
	Layers        []CorpusLayer           `json:"layers" yaml:"layers"`
	NodeCount     int                     `json:"node_count" yaml:"node_count"`
	Nodes         []RuntimeFeedNode       `json:"nodes" yaml:"nodes"`
	LayerSummary  map[string]LayerSummary `json:"layer_summary" yaml:"layer_summary"`
}

const RuntimeFeedFormat = "nomos.rbok-runtime-feed.v1"

// ValidateRuntimeFeedNode checks a node for structural validity.
func ValidateRuntimeFeedNode(n RuntimeFeedNode) []string {
	var errs []string

	if !nodeIDPattern.MatchString(n.NodeID) {
		errs = append(errs, fmt.Sprintf("node_id %q invalid", n.NodeID))
	}
	if !nodeIDPattern.MatchString(n.DocumentID) {
		errs = append(errs, fmt.Sprintf("document_id %q invalid", n.DocumentID))
	}
	if strings.TrimSpace(n.CanonicalRef) == "" {
		errs = append(errs, "canonical_ref required")
	}
	if strings.TrimSpace(n.DisplayRef) == "" {
		errs = append(errs, "display_ref required")
	}
	if !hashPattern.MatchString(n.SourceHash) {
		errs = append(errs, fmt.Sprintf("source_hash %q invalid", n.SourceHash))
	}
	if !n.Layer.IsValid() {
		errs = append(errs, fmt.Sprintf("layer %q invalid", n.Layer))
	}
	if !n.AuthorityLevel.IsValid() {
		errs = append(errs, fmt.Sprintf("authority_level %q invalid", n.AuthorityLevel))
	}
	if strings.TrimSpace(n.Domain) == "" {
		errs = append(errs, "domain required")
	}
	if n.Depth < 0 || n.Depth > 10 {
		errs = append(errs, fmt.Sprintf("depth %d out of range 0-10", n.Depth))
	}

	return errs
}

// ValidateRuntimeFeed checks the feed envelope for consistency.
func ValidateRuntimeFeed(f RuntimeFeed) []string {
	var errs []string

	if f.FeedFormat != RuntimeFeedFormat {
		errs = append(errs, fmt.Sprintf("feed_format must be %q, got %q", RuntimeFeedFormat, f.FeedFormat))
	}
	if strings.TrimSpace(f.FeedID) == "" {
		errs = append(errs, "feed_id required")
	}
	if strings.TrimSpace(f.CorpusID) == "" {
		errs = append(errs, "corpus_id required")
	}
	if strings.TrimSpace(f.Domain) == "" {
		errs = append(errs, "domain required")
	}
	if f.NodeCount != len(f.Nodes) {
		errs = append(errs, fmt.Sprintf("node_count %d != len(nodes) %d", f.NodeCount, len(f.Nodes)))
	}

	for i, node := range f.Nodes {
		nodeErrs := ValidateRuntimeFeedNode(node)
		for _, e := range nodeErrs {
			errs = append(errs, fmt.Sprintf("nodes[%d]: %s", i, e))
		}
	}

	return errs
}

// ComputeLayerSummary builds the per-layer summary from nodes.
func ComputeLayerSummary(nodes []RuntimeFeedNode) map[string]LayerSummary {
	summaries := map[string]*LayerSummary{}
	docSets := map[string]map[string]bool{}

	for _, n := range nodes {
		layer := string(n.Layer)
		s, ok := summaries[layer]
		if !ok {
			s = &LayerSummary{AuthorityBreakdown: map[string]int{}}
			summaries[layer] = s
			docSets[layer] = map[string]bool{}
		}
		s.NodeCount++
		s.AuthorityBreakdown[string(n.AuthorityLevel)]++
		docSets[layer][n.DocumentID] = true
	}

	result := make(map[string]LayerSummary, len(summaries))
	for layer, s := range summaries {
		s.DocumentCount = len(docSets[layer])
		result[layer] = *s
	}
	return result
}

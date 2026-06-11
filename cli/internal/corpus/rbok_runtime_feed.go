package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

const RuntimeFeedFormat = "nomos.rbok-runtime-feed.v1"

// LayerKind identifies the business purpose of a source layer.
type LayerKind string

const (
	LayerReferentiel LayerKind = "referentiel"
	LayerDomaine     LayerKind = "domaine"
	LayerMeta        LayerKind = "meta"
	LayerOverride    LayerKind = "override"
)

// LayerInput describes one source layer to be scanned and merged.
type LayerInput struct {
	ID       string    `json:"id"`
	Kind     LayerKind `json:"kind"`
	Path     string    `json:"path"`
	Domain   string    `json:"domain"`
	Priority int       `json:"priority"` // lower = higher priority in merge
}

// LayerProvenance records the scan result for one layer.
type LayerProvenance struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Path       string `json:"path"`
	Domain     string `json:"domain"`
	Priority   int    `json:"priority"`
	NodeCount  int    `json:"node_count"`
	SourceHash string `json:"source_hash"`
}

// RuntimeFeedNode is a node in the unified feed with layer provenance.
type RuntimeFeedNode struct {
	NodeID       string            `json:"node_id"`
	DocumentID   string            `json:"document_id"`
	NodeType     LawbookNodeType   `json:"node_type"`
	CanonicalRef string            `json:"canonical_ref"`
	DisplayRef   string            `json:"display_ref"`
	Depth        int               `json:"depth"`
	OrdinalPath  string            `json:"ordinal_path"`
	SourcePath   string            `json:"source_path"`
	SourceHash   string            `json:"source_hash"`
	Status       LawbookNodeStatus `json:"status"`
	Priority     LawbookPriority   `json:"priority"`
	Domain       string            `json:"domain"`
	Title        string            `json:"title,omitempty"`
	Text         string            `json:"text,omitempty"`
	ParentID     string            `json:"parent_id,omitempty"`
	LayerID      string            `json:"layer_id"`
	LayerKind    string            `json:"layer_kind"`
}

// RuntimeFeed is the unified multi-layer output.
type RuntimeFeed struct {
	Format      string            `json:"format"`
	GeneratedAt string            `json:"generated_at"`
	ContentHash string            `json:"content_hash"`
	LayerCount  int               `json:"layer_count"`
	NodeCount   int               `json:"node_count"`
	Layers      []LayerProvenance `json:"layers"`
	Nodes       []RuntimeFeedNode `json:"nodes"`
	Conflicts   []MergeConflict   `json:"conflicts,omitempty"`
}

// MergeConflict records when the same canonical_ref appears in multiple layers.
type MergeConflict struct {
	CanonicalRef string   `json:"canonical_ref"`
	LayerIDs     []string `json:"layer_ids"`
	Resolution   string   `json:"resolution"` // "priority" or "first"
	WinnerLayer  string   `json:"winner_layer"`
}

// RuntimeFeedOptions configures feed assembly.
type RuntimeFeedOptions struct {
	Now time.Time
}

// AssembleRuntimeFeed scans multiple layers, classifies their nodes,
// merges them into a unified feed with per-node layer provenance,
// and detects cross-layer conflicts.
func AssembleRuntimeFeed(layers []LayerInput, feeds map[string]LawbookFeed, opts RuntimeFeedOptions) RuntimeFeed {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	// Sort layers by priority (lower = higher priority).
	sorted := make([]LayerInput, len(layers))
	copy(sorted, layers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	var provenances []LayerProvenance
	var allNodes []RuntimeFeedNode
	refToLayers := map[string][]string{}      // canonical_ref → layer IDs
	refToNode := map[string]RuntimeFeedNode{} // canonical_ref → winning node
	refToPriority := map[string]int{}         // canonical_ref → winning priority

	for _, layer := range sorted {
		feed, ok := feeds[layer.ID]
		if !ok {
			continue
		}

		prov := LayerProvenance{
			ID:         layer.ID,
			Kind:       string(layer.Kind),
			Path:       layer.Path,
			Domain:     layer.Domain,
			Priority:   layer.Priority,
			NodeCount:  len(feed.Nodes),
			SourceHash: feed.SourceHash,
		}
		provenances = append(provenances, prov)

		for _, node := range feed.Nodes {
			rn := RuntimeFeedNode{
				NodeID:       node.NodeID,
				DocumentID:   node.DocumentID,
				NodeType:     node.NodeType,
				CanonicalRef: node.CanonicalRef,
				DisplayRef:   node.DisplayRef,
				Depth:        node.Depth,
				OrdinalPath:  node.OrdinalPath,
				SourcePath:   node.SourcePath,
				SourceHash:   node.SourceHash,
				Status:       node.Status,
				Priority:     node.Priority,
				Domain:       node.Domain,
				Title:        node.Title,
				Text:         node.Text,
				ParentID:     node.ParentID,
				LayerID:      layer.ID,
				LayerKind:    string(layer.Kind),
			}

			ref := node.CanonicalRef
			refToLayers[ref] = appendUnique(refToLayers[ref], layer.ID)

			// Priority merge: lower priority number wins.
			if _, exists := refToNode[ref]; !exists {
				refToNode[ref] = rn
				refToPriority[ref] = layer.Priority
			} else if layer.Priority < refToPriority[ref] {
				refToNode[ref] = rn
				refToPriority[ref] = layer.Priority
			}
		}
	}

	// Collect winning nodes in deterministic order.
	var refs []string
	for ref := range refToNode {
		refs = append(refs, ref)
	}
	sort.Strings(refs)

	for _, ref := range refs {
		allNodes = append(allNodes, refToNode[ref])
	}

	// Detect conflicts.
	var conflicts []MergeConflict
	for ref, layerIDs := range refToLayers {
		if len(layerIDs) > 1 {
			conflicts = append(conflicts, MergeConflict{
				CanonicalRef: ref,
				LayerIDs:     layerIDs,
				Resolution:   "priority",
				WinnerLayer:  refToNode[ref].LayerID,
			})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].CanonicalRef < conflicts[j].CanonicalRef
	})

	result := RuntimeFeed{
		Format:      RuntimeFeedFormat,
		GeneratedAt: now.Format(time.RFC3339),
		LayerCount:  len(provenances),
		NodeCount:   len(allNodes),
		Layers:      provenances,
		Nodes:       allNodes,
		Conflicts:   conflicts,
	}

	result.ContentHash = computeRuntimeFeedHash(result)
	return result
}

// WriteRuntimeFeed serialises the feed as indented JSON.
func WriteRuntimeFeed(w io.Writer, feed RuntimeFeed) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(feed)
}

func computeRuntimeFeedHash(feed RuntimeFeed) string {
	tmp := feed
	tmp.ContentHash = ""
	data, _ := json.Marshal(tmp)
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

func appendUnique(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}

// RuntimeFeedSummary provides a quick overview for governance.
type RuntimeFeedSummary struct {
	LayerCount    int            `json:"layer_count"`
	NodeCount     int            `json:"node_count"`
	ConflictCount int            `json:"conflict_count"`
	ByLayer       map[string]int `json:"by_layer"`
	ByDomain      map[string]int `json:"by_domain"`
	ByNodeType    map[string]int `json:"by_node_type"`
}

// Summarize produces a governance summary of the runtime feed.
func (f RuntimeFeed) Summarize() RuntimeFeedSummary {
	s := RuntimeFeedSummary{
		LayerCount:    f.LayerCount,
		NodeCount:     f.NodeCount,
		ConflictCount: len(f.Conflicts),
		ByLayer:       map[string]int{},
		ByDomain:      map[string]int{},
		ByNodeType:    map[string]int{},
	}
	for _, n := range f.Nodes {
		s.ByLayer[n.LayerID]++
		s.ByDomain[n.Domain]++
		s.ByNodeType[string(n.NodeType)]++
	}
	return s
}

// FormatSummary returns a human-readable summary string.
func (s RuntimeFeedSummary) FormatSummary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Runtime feed: %d layers, %d nodes, %d conflicts\n", s.LayerCount, s.NodeCount, s.ConflictCount)
	for layer, count := range s.ByLayer {
		fmt.Fprintf(&b, "  layer %s: %d nodes\n", layer, count)
	}
	return b.String()
}

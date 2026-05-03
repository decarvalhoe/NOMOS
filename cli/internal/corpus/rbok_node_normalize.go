package corpus

import (
	"fmt"
	"strings"
)

// NodeDefaults provides document-level values that the extractor cannot
// infer from Markdown alone. These are applied by NormalizeNode to every
// node missing the field, making LawbookNode the single source of truth
// across extractor, schema, feed, and import.
type NodeDefaults struct {
	DocumentID string
	SourcePath string
	SourceHash string
	Domain     string
	Status     LawbookNodeStatus
	Priority   LawbookPriority
}

// NormalizeNode fills missing fields from defaults, fixes depth to match
// node_type, and assigns ordinal_path when absent. Returns validation
// errors (empty slice = valid).
func NormalizeNode(n *LawbookNode, defaults NodeDefaults, ordinal string) []string {
	// Apply defaults for fields the extractor cannot know.
	if n.DocumentID == "" {
		n.DocumentID = defaults.DocumentID
	}
	if n.SourcePath == "" {
		n.SourcePath = defaults.SourcePath
	}
	if n.SourceHash == "" {
		n.SourceHash = defaults.SourceHash
	}
	if n.Domain == "" {
		n.Domain = defaults.Domain
	}
	if !n.Status.IsValid() {
		n.Status = defaults.Status
	}
	if !n.Priority.IsValid() {
		n.Priority = defaults.Priority
	}

	// Fix depth to match node_type canonical depth when that type has a
	// canonical lawbook depth. Portable block nodes keep their AST depth.
	if n.NodeType.IsValid() && n.NodeType.hasFixedDepth() {
		n.Depth = n.NodeType.Depth()
	}

	// Assign ordinal_path if missing.
	if n.OrdinalPath == "" && ordinal != "" {
		n.OrdinalPath = ordinal
	}

	if n.SourceSpan != nil {
		if strings.TrimSpace(n.SourceSpan.SourceID) == "" {
			n.SourceSpan.SourceID = defaults.DocumentID
		}
		if strings.TrimSpace(n.SourceSpan.Path) == "" {
			n.SourceSpan.Path = defaults.SourcePath
		}
		if strings.TrimSpace(n.SourceSpan.Hash) == "" {
			n.SourceSpan.Hash = defaults.SourceHash
		}
		if strings.TrimSpace(n.SourceSpan.Locator) == "" {
			n.SourceSpan.Locator = sourceSpanLocator(defaults.SourcePath, n.SourceSpan)
		}
	}

	return ValidateNode(*n)
}

// NormalizeExtractResult applies defaults to all nodes from an extraction,
// assigning sequential ordinal paths. Returns the total error count.
func NormalizeExtractResult(result *MDExtractResult, defaults NodeDefaults) int {
	errorCount := 0
	ordinalCounters := map[string]int{} // parent_id -> child counter

	for i := range result.Nodes {
		node := &result.Nodes[i]

		// Compute ordinal path based on parent's ordinal and child index.
		ordinalCounters[node.ParentID]++
		ordinal := computeOrdinal(node, result.Nodes[:i], ordinalCounters[node.ParentID])

		errs := NormalizeNode(node, defaults, ordinal)
		errorCount += len(errs)
	}

	return errorCount
}

func computeOrdinal(node *LawbookNode, prior []LawbookNode, childIndex int) string {
	if node.ParentID == "" {
		return fmt.Sprintf("%d", childIndex)
	}

	// Find parent's ordinal.
	parentOrdinal := ""
	for i := range prior {
		if prior[i].NodeID == node.ParentID {
			parentOrdinal = prior[i].OrdinalPath
			break
		}
	}

	if parentOrdinal == "" {
		return fmt.Sprintf("%d", childIndex)
	}
	return fmt.Sprintf("%s.%d", parentOrdinal, childIndex)
}

// BuildNormalizedFeed creates a LawbookFeed from an extraction result
// with all nodes normalized. This is the canonical pipeline entry point:
// ExtractMarkdown → NormalizeExtractResult → BuildNormalizedFeed.
func BuildNormalizedFeed(result MDExtractResult, feedID string, defaults NodeDefaults, generatedAt string) LawbookFeed {
	return LawbookFeed{
		SchemaVersion: "0.1.0",
		FeedID:        feedID,
		DocumentID:    defaults.DocumentID,
		Domain:        defaults.Domain,
		GeneratedAt:   generatedAt,
		SourcePath:    defaults.SourcePath,
		SourceHash:    defaults.SourceHash,
		NodeCount:     len(result.Nodes),
		Nodes:         result.Nodes,
	}
}

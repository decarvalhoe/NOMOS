package ragexport

import (
	"fmt"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/bundle"
	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

// InputsFromBundle projects the faceted nodes of a Canonical Knowledge Bundle
// into export inputs. The bundle is the faceted artifact: its nodes carry the
// engine-derived closed facet axes (nature, scope_level, trust_tier,
// provenance), which is what a Knowledge Lens needs to scope an export; the
// open axes come from pack document facets (Options.DocumentFacets).
//
// source_id is the node's source_path: a bundle has no separate source
// identity, and pack document facets are keyed by that path.
func InputsFromBundle(b *bundle.Bundle) []Input {
	if b == nil {
		return nil
	}
	chunkByNode := make(map[string]bundle.RAGMetadata, len(b.RAGMetadata))
	for _, r := range b.RAGMetadata {
		chunkByNode[r.NodeID] = r
	}
	var inputs []Input
	for _, feed := range b.Feeds {
		for _, n := range feed.Nodes {
			chunkID := "chunk:" + n.NodeID
			if r, ok := chunkByNode[n.NodeID]; ok && strings.TrimSpace(r.ChunkID) != "" {
				chunkID = r.ChunkID
			}
			locator := n.Span.Locator
			if locator == "" && n.Span.StartLine > 0 {
				locator = fmt.Sprintf("%s:L%d-L%d", n.SourcePath, n.Span.StartLine, n.Span.EndLine)
			}
			facets := n.Facets
			inputs = append(inputs, Input{
				Chunk: corpus.ChunkMetadata{
					ChunkID:                  chunkID,
					SourceID:                 n.SourcePath,
					SourcePath:               n.SourcePath,
					SourceHash:               n.SourceHash,
					Locator:                  locator,
					UnitIDs:                  []string{n.NodeID},
					CanonicalUnitID:          n.NodeID,
					StartByte:                n.Span.StartByte,
					EndByte:                  n.Span.EndByte,
					StartLine:                n.Span.StartLine,
					EndLine:                  n.Span.EndLine,
					ChunkCompositionStrategy: "bundle_node",
					IngestedAt:               b.GeneratedAt,
					IngestionVersion:         b.SchemaVersion,
					ChunkText:                n.Text,
				},
				Facets: &facets,
			})
		}
	}
	return inputs
}

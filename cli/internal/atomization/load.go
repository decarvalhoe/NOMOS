package atomization

import (
	"fmt"

	"github.com/RBOKproject/Nomos/cli/internal/docload"
)

// LoadKnowledgeLens is the engine's loader for one knowledge lens
// (specs/knowledge-lens.cue #KnowledgeLens), YAML or JSON. A lens needs an id
// and at least one predicate: a lens with neither include nor exclude would
// silently keep everything. `nomos atomize`, `nomos pack` and the contract
// registry read lenses through it.
func LoadKnowledgeLens(path string) (*KnowledgeLens, error) {
	var lens KnowledgeLens
	if err := docload.Load(path, &lens); err != nil {
		return nil, err
	}
	if lens.ID == "" {
		return nil, fmt.Errorf("%s: lens is missing an id", path)
	}
	if lens.Include == nil && lens.Exclude == nil {
		return nil, fmt.Errorf("%s: lens %q has no include/exclude predicate", path, lens.ID)
	}
	return &lens, nil
}

// FacetedDocument is the facets-bearing shape of a faceted atom or chunk
// document (specs/facets.cue #FacetedAtom / #FacetedChunk): the facets live
// under metadata.facets; the rest of the atom is the atomizer's.
type FacetedDocument struct {
	AtomID   string `json:"atom_id"`
	ChunkID  string `json:"chunk_id"`
	Kind     string `json:"kind"`
	Metadata struct {
		Facets *Facets `json:"facets"`
	} `json:"metadata"`
}

// LoadFacetedDocument reads a faceted atom/chunk document and validates its
// facets with the engine's own rule set (Facets.Validate).
func LoadFacetedDocument(path string) (FacetedDocument, error) {
	var doc FacetedDocument
	if err := docload.Load(path, &doc); err != nil {
		return FacetedDocument{}, err
	}
	if doc.Metadata.Facets == nil {
		return FacetedDocument{}, fmt.Errorf("%s: document carries no metadata.facets block", path)
	}
	if err := doc.Metadata.Facets.Validate(); err != nil {
		return FacetedDocument{}, fmt.Errorf("%s: facets: %w", path, err)
	}
	return doc, nil
}

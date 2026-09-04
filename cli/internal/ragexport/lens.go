package ragexport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/RBOKproject/Nomos/cli/internal/atomization"
)

// LensBinding records which Knowledge Lens an export was scoped with. The
// digest covers the whole lens document, so a consumer can prove its index
// was built for exactly this scope — not a lens of the same name edited
// since. Two manifests with different bindings describe different indexes.
type LensBinding struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// LensDigest fingerprints a lens by its canonical JSON form.
func LensDigest(lens atomization.KnowledgeLens) string {
	// A lens is plain data (strings and string lists); encoding cannot fail.
	raw, _ := json.Marshal(lens)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// resolveFacets merges engine-derived facets with the pack's per-document
// enrichment — the same order the reference retrieval kit applies before its
// lens runs: pack values override node values on the axes they set. A chunk
// with neither yields nil, which a lens treats as "cannot prove membership".
func resolveFacets(in Input, opts Options) *atomization.Facets {
	var out *atomization.Facets
	if in.Facets != nil {
		f := *in.Facets
		out = &f
	}
	doc, ok := opts.DocumentFacets[in.Chunk.SourcePath]
	if !ok {
		doc, ok = opts.DocumentFacets[in.Chunk.SourceID]
	}
	if ok {
		base := atomization.Facets{}
		if out != nil {
			base = *out
		}
		merged := mergeFacets(base, doc)
		out = &merged
	}
	return out
}

// mergeFacets overlays over onto base, axis by axis, only where over is set.
func mergeFacets(base, over atomization.Facets) atomization.Facets {
	out := base
	if over.Nature != "" {
		out.Nature = over.Nature
	}
	if over.ScopeLevel != "" {
		out.ScopeLevel = over.ScopeLevel
	}
	if over.TrustTier != "" {
		out.TrustTier = over.TrustTier
	}
	if over.Provenance != "" {
		out.Provenance = over.Provenance
	}
	if over.Confidentiality != "" {
		out.Confidentiality = over.Confidentiality
	}
	if over.Applicability != "" {
		out.Applicability = over.Applicability
	}
	if len(over.DisciplineRole) > 0 {
		out.DisciplineRole = append([]string(nil), over.DisciplineRole...)
	}
	if len(over.Activity) > 0 {
		out.Activity = append([]string(nil), over.Activity...)
	}
	if len(over.VocabularyRefs) > 0 {
		out.VocabularyRefs = append([]string(nil), over.VocabularyRefs...)
	}
	if len(over.Extensions) > 0 {
		ext := make(map[string]any, len(base.Extensions)+len(over.Extensions))
		for k, v := range base.Extensions {
			ext[k] = v
		}
		for k, v := range over.Extensions {
			ext[k] = v
		}
		out.Extensions = ext
	}
	return out
}

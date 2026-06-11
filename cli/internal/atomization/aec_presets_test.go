package atomization

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// W23-4 (#571, partial) — the AEC lens presets are REAL pack artifacts (the
// exact files `nomos atomize chunks --lens …` consumes), and they DISCRIMINATE:
// a conception atom never leaks through the permit lens, a confidential atom
// never leaves through the public-enquête lens, blocked applicability is out
// everywhere. This is the distractor proof (doc 45 B1) played at unit level —
// the full retrieval harness stays VRC-35.

func loadPackLens(t *testing.T, name string) KnowledgeLens {
	t.Helper()
	path := filepath.Join("..", "..", "..", "docs", "regulated", "domain-packs",
		"built-environment", "aec-lens-presets", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var generic any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	bridged, err := json.Marshal(generic)
	if err != nil {
		t.Fatalf("normalize %s: %v", path, err)
	}
	var lens KnowledgeLens
	if err := json.Unmarshal(bridged, &lens); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if lens.ID == "" || (lens.Include == nil && lens.Exclude == nil) {
		t.Fatalf("%s is not a usable lens: %+v", path, lens)
	}
	return lens
}

func aecFacets(activity string, confidentiality FacetConfidentiality, applicability FacetApplicability) Facets {
	return Facets{
		Nature:          "rule",
		ScopeLevel:      "atom",
		TrustTier:       "indicative",
		Provenance:      "source_backed",
		Confidentiality: confidentiality,
		Applicability:   applicability,
		Activity:        []string{activity},
		DisciplineRole:  []string{"aec.architecte"},
	}
}

func TestAECPresets_DiscriminateAcrossPhases(t *testing.T) {
	conception := loadPackLens(t, "archi-conception.lens.yaml")
	chantier := loadPackLens(t, "dt-chantier.lens.yaml")
	permis := loadPackLens(t, "permis.lens.yaml")

	conceptionAtom := aecFacets("aec.conception", "public", "applicable")
	permisAtom := aecFacets("aec.permis", "public", "applicable")
	chantierAtom := aecFacets("aec.direction_travaux", "public", "applicable")

	// Each preset includes ITS phase…
	if d := ApplyLens(conceptionAtom, conception); !d.Included {
		t.Fatalf("conception lens rejected its own atom: %+v", d)
	}
	if d := ApplyLens(permisAtom, permis); !d.Included {
		t.Fatalf("permis lens rejected its own atom: %+v", d)
	}
	if d := ApplyLens(chantierAtom, chantier); !d.Included {
		t.Fatalf("chantier lens rejected its own atom: %+v", d)
	}
	// …and rejects the others (the distractor pairs).
	for name, pair := range map[string]struct {
		atom Facets
		lens KnowledgeLens
	}{
		"conception ∉ permis":   {conceptionAtom, permis},
		"permis ∉ conception":   {permisAtom, conception},
		"chantier ∉ permis":     {chantierAtom, permis},
		"conception ∉ chantier": {conceptionAtom, chantier},
	} {
		if d := ApplyLens(pair.atom, pair.lens); d.Included {
			t.Fatalf("distractor leaked: %s", name)
		}
	}
}

func TestAECPresets_PermitLensNeverLeaksConfidential(t *testing.T) {
	permis := loadPackLens(t, "permis.lens.yaml")
	confidential := aecFacets("aec.permis", "confidential", "applicable")
	if d := ApplyLens(confidential, permis); d.Included {
		t.Fatal("the public-enquête lens leaked a confidential atom")
	}
}

func TestAECPresets_BlockedApplicabilityIsOutEverywhere(t *testing.T) {
	for _, name := range []string{"archi-conception.lens.yaml", "dt-chantier.lens.yaml", "permis.lens.yaml"} {
		lens := loadPackLens(t, name)
		blocked := aecFacets("aec.conception", "public", "blocked")
		blocked.Activity = []string{"aec.conception", "aec.permis", "aec.direction_travaux"}
		if d := ApplyLens(blocked, lens); d.Included {
			t.Fatalf("%s included a blocked-applicability atom", name)
		}
	}
}

// Package canon is the canon-promotion validator in the Go engine (VRC-11
// #557, A2). A user-promoted atom may enter the project knowledge silo only
// under the certified-promotion rules — and a customer-confidential source or
// atom may NEVER leak to the shared catalog. Ported faithfully from
// scripts/ckm_canon_promotion_validate.py + specs/canon-promotion.cue so the
// Go verdict and the Python sidecar agree.
//
// Doctrine: a promoted source never usurps the official certified
// (trust_tier=certified is refused on a user_promoted atom); confidentiality
// is one-way (confidential stays siloed); every promotion binds to a
// non-revoked certificate.
package canon

import "sort"

// Source is the promoted source's identity + access policy.
type Source struct {
	SourceID     string `json:"source_id"`
	AccessPolicy string `json:"access_policy"`
	SiloID       string `json:"silo_id"`
}

// SharedCatalog lists the source ids exported to the shared (cross-project) pool.
type SharedCatalog struct {
	ExportedSourceIDs []string `json:"exported_source_ids"`
}

// SiloCatalog lists the source ids kept project-private.
type SiloCatalog struct {
	SourceIDs []string `json:"source_ids"`
}

// Certificate is a promotion certificate (may be revoked).
type Certificate struct {
	CertID  string `json:"cert_id"`
	Revoked bool   `json:"revoked"`
}

// Facets is the controlled-facet subset the promotion rules read.
type Facets struct {
	Provenance      string `json:"provenance"`
	TrustTier       string `json:"trust_tier"`
	Confidentiality string `json:"confidentiality"`
}

// CanonPromotion is the per-atom promotion record.
type CanonPromotion struct {
	CertificateID string `json:"certificate_id"`
	SourceID      string `json:"source_id"`
	SiloID        string `json:"silo_id"`
	Surfacing     string `json:"surfacing"`
	SharedCatalog *bool  `json:"shared_catalog"`
}

// Atom is one promotion-candidate atom.
type Atom struct {
	AtomID      string `json:"atom_id"`
	ReviewState string `json:"review_state"`
	Metadata    struct {
		Facets         Facets         `json:"facets"`
		CanonPromotion CanonPromotion `json:"canon_promotion"`
	} `json:"metadata"`
}

// PromotionBundle is the document the gate validates.
type PromotionBundle struct {
	Source        Source        `json:"source"`
	SharedCatalog SharedCatalog `json:"shared_catalog"`
	SiloCatalog   SiloCatalog   `json:"silo_catalog"`
	Certificates  []Certificate `json:"certificates"`
	Atoms         []Atom        `json:"atoms"`
}

// Finding is one promotion-rule violation.
type Finding struct {
	Code   string `json:"code"`
	Target string `json:"target"`
}

// PromotedAtom is the per-atom outcome echoed back.
type PromotedAtom struct {
	Provenance      string `json:"provenance"`
	TrustTier       string `json:"trust_tier"`
	Confidentiality string `json:"confidentiality"`
	CertificateID   string `json:"certificate_id"`
}

// Report is the gate verdict.
type Report struct {
	Status                       string                  `json:"status"` // "pass" | "fail"
	Findings                     []Finding               `json:"findings"`
	SharedCatalogExposedSourceIDs []string               `json:"shared_catalog_exposed_source_ids"`
	SiloedSourceIDs              []string                `json:"siloed_source_ids"`
	PromotedAtoms                map[string]PromotedAtom `json:"promoted_atoms"`
}

// ValidateCanonPromotion enforces the canon-promotion rules and fails closed on
// any violation. The verdict is rendered by the engine, not declared.
func ValidateCanonPromotion(b PromotionBundle) Report {
	findings := []Finding{}
	add := func(code, target string) { findings = append(findings, Finding{Code: code, Target: target}) }

	shared := map[string]bool{}
	for _, id := range b.SharedCatalog.ExportedSourceIDs {
		shared[id] = true
	}
	certs := map[string]Certificate{}
	for _, c := range b.Certificates {
		certs[c.CertID] = c
	}

	if b.Source.AccessPolicy == "customer_confidential" && shared[b.Source.SourceID] {
		add("customer_confidential_source_exposed", b.Source.SourceID)
	}

	promoted := map[string]PromotedAtom{}
	for _, atom := range b.Atoms {
		f := atom.Metadata.Facets
		p := atom.Metadata.CanonPromotion
		cert, certPresent := certs[p.CertificateID]

		if atom.ReviewState != "approved" {
			add("review_state_must_be_approved", atom.AtomID)
		}
		if f.Provenance != "user_promoted" {
			add("promoted_atom_requires_user_promoted_provenance", atom.AtomID)
		}
		if f.TrustTier == "certified" {
			add("user_promoted_cannot_be_certified", atom.AtomID)
		}
		if f.Confidentiality == "customer_confidential" {
			// Must remain siloed: surfacing silo_only AND shared_catalog
			// explicitly false. Missing / true both leak.
			if p.Surfacing != "silo_only" || p.SharedCatalog == nil || *p.SharedCatalog != false {
				add("customer_confidential_atom_must_remain_siloed", atom.AtomID)
			}
		}
		if p.SourceID != b.Source.SourceID {
			add("promotion_source_mismatch", atom.AtomID)
		}
		if p.SiloID != b.Source.SiloID {
			add("promotion_silo_mismatch", atom.AtomID)
		}
		if !certPresent {
			add("certificate_required", atom.AtomID)
		} else if cert.Revoked {
			add("certificate_revoked", p.CertificateID)
		}

		promoted[atom.AtomID] = PromotedAtom{
			Provenance:      f.Provenance,
			TrustTier:       f.TrustTier,
			Confidentiality: f.Confidentiality,
			CertificateID:   p.CertificateID,
		}
	}

	exposed := []string{}
	for id := range shared {
		if id != "" {
			exposed = append(exposed, id)
		}
	}
	sort.Strings(exposed)
	siloed := append([]string{}, b.SiloCatalog.SourceIDs...)

	status := "pass"
	if len(findings) > 0 {
		status = "fail"
	}
	return Report{
		Status:                        status,
		Findings:                      findings,
		SharedCatalogExposedSourceIDs: exposed,
		SiloedSourceIDs:               siloed,
		PromotedAtoms:                 promoted,
	}
}

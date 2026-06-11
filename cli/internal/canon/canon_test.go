package canon

import "testing"

// VRC-11 (#557, A2) — the canon-promotion gate proves itself by REFUSING the
// doctrine violations: a promoted atom claiming `certified`, a confidential
// source/atom leaking to the shared catalog, a missing/revoked certificate.

func boolp(b bool) *bool { return &b }

func validBundle() PromotionBundle {
	b := PromotionBundle{
		Source:        Source{SourceID: "SRC-1", AccessPolicy: "customer_source", SiloID: "SILO-1"},
		SharedCatalog: SharedCatalog{ExportedSourceIDs: []string{}},
		SiloCatalog:   SiloCatalog{SourceIDs: []string{"SRC-1"}},
		Certificates:  []Certificate{{CertID: "CERT-1", Revoked: false}},
	}
	atom := Atom{AtomID: "AT-1", ReviewState: "approved"}
	atom.Metadata.Facets = Facets{Provenance: "user_promoted", TrustTier: "indicative", Confidentiality: "internal"}
	atom.Metadata.CanonPromotion = CanonPromotion{CertificateID: "CERT-1", SourceID: "SRC-1", SiloID: "SILO-1"}
	b.Atoms = []Atom{atom}
	return b
}

func has(r Report, code string) bool {
	for _, f := range r.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func TestCanon_ValidBundlePasses(t *testing.T) {
	if r := ValidateCanonPromotion(validBundle()); r.Status != "pass" {
		t.Fatalf("valid promotion must pass, got findings %+v", r.Findings)
	}
}

func TestCanon_CertifiedTrustTierIsRefused(t *testing.T) {
	b := validBundle()
	b.Atoms[0].Metadata.Facets.TrustTier = "certified"
	r := ValidateCanonPromotion(b)
	if r.Status != "fail" || !has(r, "user_promoted_cannot_be_certified") {
		t.Fatalf("a user_promoted atom claiming certified must be refused: %+v", r.Findings)
	}
}

func TestCanon_ConfidentialSourceLeakIsRefused(t *testing.T) {
	b := validBundle()
	b.Source.AccessPolicy = "customer_confidential"
	b.SharedCatalog.ExportedSourceIDs = []string{"SRC-1"}
	r := ValidateCanonPromotion(b)
	if !has(r, "customer_confidential_source_exposed") {
		t.Fatalf("a confidential source in the shared catalog must be refused: %+v", r.Findings)
	}
}

func TestCanon_ConfidentialAtomMustStaySiloed(t *testing.T) {
	b := validBundle()
	b.Atoms[0].Metadata.Facets.Confidentiality = "customer_confidential"
	// surfacing not silo_only, shared_catalog missing → leak
	r := ValidateCanonPromotion(b)
	if !has(r, "customer_confidential_atom_must_remain_siloed") {
		t.Fatalf("a confidential atom must be forced to stay siloed: %+v", r.Findings)
	}
	// Properly siloed → passes that rule.
	b.Atoms[0].Metadata.CanonPromotion.Surfacing = "silo_only"
	b.Atoms[0].Metadata.CanonPromotion.SharedCatalog = boolp(false)
	if has(ValidateCanonPromotion(b), "customer_confidential_atom_must_remain_siloed") {
		t.Fatal("a correctly siloed confidential atom must not be flagged")
	}
}

func TestCanon_MissingAndRevokedCertificateRefused(t *testing.T) {
	b := validBundle()
	b.Certificates = nil // certificate missing
	if !has(ValidateCanonPromotion(b), "certificate_required") {
		t.Fatal("a promotion with no certificate must be refused")
	}
	b2 := validBundle()
	b2.Certificates = []Certificate{{CertID: "CERT-1", Revoked: true}}
	if !has(ValidateCanonPromotion(b2), "certificate_revoked") {
		t.Fatal("a promotion on a revoked certificate must be refused")
	}
}

func TestCanon_ProvenanceAndReviewStateEnforced(t *testing.T) {
	b := validBundle()
	b.Atoms[0].Metadata.Facets.Provenance = "official"
	b.Atoms[0].ReviewState = "draft"
	r := ValidateCanonPromotion(b)
	if !has(r, "promoted_atom_requires_user_promoted_provenance") || !has(r, "review_state_must_be_approved") {
		t.Fatalf("provenance + review_state must be enforced: %+v", r.Findings)
	}
}

func TestCanon_SourceAndSiloMismatchRefused(t *testing.T) {
	b := validBundle()
	b.Atoms[0].Metadata.CanonPromotion.SourceID = "OTHER"
	b.Atoms[0].Metadata.CanonPromotion.SiloID = "OTHER-SILO"
	r := ValidateCanonPromotion(b)
	if !has(r, "promotion_source_mismatch") || !has(r, "promotion_silo_mismatch") {
		t.Fatalf("source/silo binding must be enforced: %+v", r.Findings)
	}
}

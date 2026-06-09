package nomos

import "strings"

// CKM-03 optional user-promoted canon.
//
// A customer source may be promoted into that customer's canon only with
// explicit review approval and a certificate. Customer-confidential promoted
// canon remains siloed and must not be surfaced as a shared source.

#CanonPromotionSource: {
	source_id:           string & strings.MinRunes(1)
	authority_type:      "customer_source"
	access_policy:       "customer_confidential"
	silo_id:             string & strings.MinRunes(1)
	committed_full_text: false
}

#CanonPromotionAtom: #FacetedAtom & {
	review_state: "approved"
	metadata: {
		facets: {
			provenance:      "user_promoted"
			confidentiality: "customer_confidential"
			trust_tier:      "indicative" | "unverified"
		}
		canon_promotion: {
			promotion_id:   string & strings.MinRunes(1)
			source_id:      string & strings.MinRunes(1)
			certificate_id: #CertID
			silo_id:        string & strings.MinRunes(1)
			surfacing:      "silo_only"
			shared_catalog: false
		}
		...
	}
}

#CanonPromotionCertificate: #Certificate & {
	cert_type: "review_approval"
	revoked:   false
	metadata: {
		promotion_id:          string & strings.MinRunes(1)
		source_authority_type: "customer_source"
		confidentiality:       "customer_confidential"
		promotion_status:      "active"
		...
	}
}

#CanonPromotionBundle: {
	record_type:    "ckm_canon_promotion_bundle"
	schema_version: string & strings.MinRunes(1)
	source:         #CanonPromotionSource
	atoms:          [#CanonPromotionAtom, ...#CanonPromotionAtom]
	certificates:   [#CanonPromotionCertificate, ...#CanonPromotionCertificate]
	shared_catalog: {
		exported_source_ids: [...string] | *[]
		exported_atom_ids:   [...#AtomID] | *[]
	}
	silo_catalog: {
		silo_id:    string & strings.MinRunes(1)
		source_ids: [string, ...string]
		atom_ids:   [#AtomID, ...#AtomID]
	}
	revocations?: [...{
		certificate_id: #CertID
		reason:         string & strings.MinRunes(1)
		revoked_at:     string & strings.MinRunes(1)
	}]
}

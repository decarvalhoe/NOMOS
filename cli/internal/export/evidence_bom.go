package export

import (
	"fmt"
	"sort"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

// VRC-23 (#566, A5) — evidence packs as supply-chain BOMs (CycloneDX 1.5 /
// SPDX 2.3). Each admitted corpus source becomes a BOM component carrying its
// sha256, and the BOM is bound to a Merkle-VERIFIED body ledger: both
// generators FIRST recompute every leaf from the ledger rows and walk the
// inclusion proofs (corpus.VerifyCorpusBodyLedgerProofs). A tampered source
// hash — or size, or admission status — changes the recomputed leaf, breaks
// verification, and refuses emission. The BOM hashes are therefore
// cross-checked against the COMPUTED corpus integrity, never declared.

const (
	evidenceBOMTool    = "nomos"
	evidenceBOMVendor  = "RBOK"
	evidenceBOMVersion = "0.1.0-ALPHA"
)

// splitDigest separates an "alg:hex" hash into its parts; a bare hex string is
// treated as sha256 (the body-ledger default).
func splitDigest(h string) (alg, hex string) {
	if i := strings.Index(h, ":"); i >= 0 {
		return strings.ToLower(h[:i]), h[i+1:]
	}
	return "sha256", h
}

func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// admittedSources returns the ledger's admitted sources (the evidence pack's
// content), sorted by SourceID for deterministic BOM output.
func admittedSources(ledger corpus.CorpusBodyLedger) []corpus.BodyLedgerSource {
	out := make([]corpus.BodyLedgerSource, 0, len(ledger.Sources))
	for _, s := range ledger.Sources {
		if s.AdmissionStatus == "admitted" {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SourceID < out[j].SourceID })
	return out
}

// verifyForBOM is the shared cross-check: the ledger must pass Merkle
// verification and every admitted source must carry a real sha256.
func verifyForBOM(ledger corpus.CorpusBodyLedger) ([]corpus.BodyLedgerSource, error) {
	if err := corpus.VerifyCorpusBodyLedgerProofs(ledger); err != nil {
		return nil, fmt.Errorf("evidence bom: body ledger failed merkle verification: %w", err)
	}
	sources := admittedSources(ledger)
	if len(sources) == 0 {
		return nil, fmt.Errorf("evidence bom: ledger has no admitted sources to attest")
	}
	for _, s := range sources {
		alg, hex := splitDigest(s.Hash)
		if alg != "sha256" || !isSHA256Hex(hex) {
			return nil, fmt.Errorf("evidence bom: source %s carries no real sha256 (%q)", s.SourceID, s.Hash)
		}
	}
	return sources, nil
}

// GenerateEvidenceCycloneDX verifies the ledger then emits a CycloneDX 1.5 BOM
// whose components carry each admitted source's sha256.
func GenerateEvidenceCycloneDX(ledger corpus.CorpusBodyLedger, serialNumber string) (CDXBom, error) {
	sources, err := verifyForBOM(ledger)
	if err != nil {
		return CDXBom{}, err
	}
	comps := make([]CDXComponent, 0, len(sources))
	for _, s := range sources {
		_, hex := splitDigest(s.Hash)
		comps = append(comps, CDXComponent{
			Type:    "file",
			BomRef:  s.SourceID,
			Name:    s.Path,
			Version: ledger.GeneratedAt,
			Hashes:  []CDXHash{{Alg: "SHA-256", Content: hex}},
			Properties: []CDXProperty{
				{Name: "nomos:admission_status", Value: s.AdmissionStatus},
				{Name: "nomos:source_role", Value: s.SourceRole},
				{Name: "nomos:size_bytes", Value: fmt.Sprintf("%d", s.SizeBytes)},
			},
		})
	}
	return CDXBom{
		BomFormat:    "CycloneDX",
		SpecVersion:  "1.5",
		SerialNumber: serialNumber,
		Version:      1,
		Metadata: CDXMetadata{
			Timestamp: ledger.GeneratedAt,
			Tools:     []CDXTool{{Vendor: evidenceBOMVendor, Name: evidenceBOMTool, Version: evidenceBOMVersion}},
			Component: &CDXComponent{
				Type:    "data",
				Name:    "nomos-evidence-pack",
				Version: ledger.GeneratedAt,
				Properties: []CDXProperty{
					{Name: "nomos:merkle_root", Value: ledger.Merkle.Root},
					{Name: "nomos:merkle_algorithm", Value: ledger.Merkle.Algorithm},
					{Name: "nomos:leaf_count", Value: fmt.Sprintf("%d", ledger.Merkle.LeafCount)},
				},
			},
		},
		Components: comps,
	}, nil
}

// GenerateEvidenceSPDX verifies the ledger then emits an SPDX 2.3 document
// whose packages carry each admitted source's SHA256 checksum.
func GenerateEvidenceSPDX(ledger corpus.CorpusBodyLedger, namespace string) (SPDXDocument, error) {
	sources, err := verifyForBOM(ledger)
	if err != nil {
		return SPDXDocument{}, err
	}
	rootID := "SPDXRef-evidence-pack"
	packages := []SPDXPackage{{
		SPDXID:                rootID,
		Name:                  "nomos-evidence-pack",
		VersionInfo:           ledger.GeneratedAt,
		DownloadLocation:      "NOASSERTION",
		FilesAnalyzed:         false,
		PrimaryPackagePurpose: "DATA",
		Comment:               fmt.Sprintf("merkle_root=%s leaves=%d", ledger.Merkle.Root, ledger.Merkle.LeafCount),
	}}
	relationships := []SPDXRelationship{{
		SPDXElementID:      "SPDXRef-DOCUMENT",
		RelationshipType:   "DESCRIBES",
		RelatedSPDXElement: rootID,
	}}
	for _, s := range sources {
		_, hex := splitDigest(s.Hash)
		pkgID := "SPDXRef-Package-" + sanitizeID(s.SourceID)
		packages = append(packages, SPDXPackage{
			SPDXID:           pkgID,
			Name:             s.Path,
			VersionInfo:      ledger.GeneratedAt,
			DownloadLocation: "NOASSERTION",
			FilesAnalyzed:    false,
			Checksums:        []SPDXChecksum{{Algorithm: "SHA256", ChecksumValue: hex}},
			PrimaryPackagePurpose: "FILE",
			Comment:          fmt.Sprintf("admission=%s role=%s", s.AdmissionStatus, s.SourceRole),
		})
		relationships = append(relationships, SPDXRelationship{
			SPDXElementID:      rootID,
			RelationshipType:   "CONTAINS",
			RelatedSPDXElement: pkgID,
		})
	}
	return SPDXDocument{
		SPDXVersion:       "SPDX-2.3",
		DataLicense:       "CC0-1.0",
		SPDXID:            "SPDXRef-DOCUMENT",
		Name:              "nomos-evidence-pack",
		DocumentNamespace: namespace,
		CreationInfo: SPDXCreationInfo{
			Created:  ledger.GeneratedAt,
			Creators: []string{"Tool: " + evidenceBOMTool + "-" + evidenceBOMVersion},
		},
		Packages:      packages,
		Relationships: relationships,
	}, nil
}

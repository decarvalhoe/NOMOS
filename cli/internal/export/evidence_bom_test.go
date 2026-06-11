package export

import (
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

// VRC-23 (#566, A5) — the evidence BOM binds each component hash to a
// Merkle-VERIFIED body ledger. Doctrine §2.3: the cross-check proves itself by
// REFUSING emission on a tampered hash, not by passing the happy path.

func buildEvidenceLedger(t *testing.T) corpus.CorpusBodyLedger {
	t.Helper()
	content := []byte("# Rule\n\nBody paragraph one.\n\n## Sub\n\nBody paragraph two.\n")
	segments, err := corpus.ScanMarkdown("SRC-1", "01_rbok/rule.md", content)
	if err != nil {
		t.Fatalf("scan markdown: %v", err)
	}
	source := corpus.ManifestSource{
		ID:    "SRC-1",
		Path:  "01_rbok/rule.md",
		Hash:  corpus.ComputeRawTextHash(content),
		Owner: "domain-owner@example.com",
	}
	adm := source.Admission()
	corpus.BackfillAdmission(&adm, source.Path)
	source.AdmissionStatus = adm.AdmissionStatus
	source.AtomizationStatus = adm.AtomizationStatus
	source.SourceRole = adm.SourceRole
	source.FormatSupport = adm.FormatSupport

	ledger, err := corpus.BuildCorpusBodyLedger(corpus.BodyLedgerInput{
		CorpusRoot: "corpus",
		Sources: []corpus.BodyLedgerSourceInput{{
			Source:    source,
			Content:   content,
			Segments:  segments,
			SizeBytes: int64(len(content)),
		}},
	})
	if err != nil {
		t.Fatalf("build ledger: %v", err)
	}
	if ledger.Merkle == nil {
		t.Fatal("expected a merkle summary on the built ledger")
	}
	return ledger
}

func TestEvidenceCycloneDX_CarriesVerifiedHashes(t *testing.T) {
	ledger := buildEvidenceLedger(t)
	bom, err := GenerateEvidenceCycloneDX(ledger, "urn:uuid:test")
	if err != nil {
		t.Fatalf("verified ledger must emit a BOM: %v", err)
	}
	if bom.BomFormat != "CycloneDX" || bom.SpecVersion != "1.5" {
		t.Fatalf("unexpected BOM header: %+v", bom)
	}
	if len(bom.Components) != 1 {
		t.Fatalf("expected one admitted component, got %d", len(bom.Components))
	}
	comp := bom.Components[0]
	if len(comp.Hashes) != 1 || comp.Hashes[0].Alg != "SHA-256" {
		t.Fatalf("component carries no SHA-256 hash: %+v", comp.Hashes)
	}
	_, wantHex := splitDigest(ledger.Sources[0].Hash)
	if comp.Hashes[0].Content != wantHex {
		t.Fatalf("component hash %q != ledger hash %q", comp.Hashes[0].Content, wantHex)
	}
	// The Merkle root the BOM was bound to is recorded on the primary component.
	foundRoot := false
	for _, p := range bom.Metadata.Component.Properties {
		if p.Name == "nomos:merkle_root" && p.Value == ledger.Merkle.Root {
			foundRoot = true
		}
	}
	if !foundRoot {
		t.Fatal("BOM does not record the verified merkle root")
	}
}

func TestEvidenceSPDX_CarriesVerifiedChecksums(t *testing.T) {
	ledger := buildEvidenceLedger(t)
	doc, err := GenerateEvidenceSPDX(ledger, "https://nomos.dev/spdx/test")
	if err != nil {
		t.Fatalf("verified ledger must emit SPDX: %v", err)
	}
	if doc.SPDXVersion != "SPDX-2.3" {
		t.Fatalf("unexpected spdx version: %s", doc.SPDXVersion)
	}
	// One root package + one per source.
	var filePkgs int
	_, wantHex := splitDigest(ledger.Sources[0].Hash)
	for _, p := range doc.Packages {
		if len(p.Checksums) == 1 && p.Checksums[0].Algorithm == "SHA256" {
			filePkgs++
			if p.Checksums[0].ChecksumValue != wantHex {
				t.Fatalf("package checksum %q != ledger hash %q", p.Checksums[0].ChecksumValue, wantHex)
			}
		}
	}
	if filePkgs != 1 {
		t.Fatalf("expected one source package with a checksum, got %d", filePkgs)
	}
}

func TestEvidenceBOM_FailsClosedOnTamperedHash(t *testing.T) {
	// The cross-check: alter one recorded source hash. The recomputed Merkle
	// leaf no longer matches its proof → verification fails → NO BOM is emitted
	// (neither format), so a forged hash can never ship in a BOM.
	for _, format := range []string{"cyclonedx", "spdx"} {
		t.Run(format, func(t *testing.T) {
			ledger := buildEvidenceLedger(t)
			ledger.Sources[0].Hash = strings.Repeat("a", 64) // forged but well-formed
			var err error
			if format == "cyclonedx" {
				_, err = GenerateEvidenceCycloneDX(ledger, "urn:uuid:test")
			} else {
				_, err = GenerateEvidenceSPDX(ledger, "https://nomos.dev/spdx/test")
			}
			if err == nil {
				t.Fatal("a tampered source hash produced a BOM — the cross-check is broken")
			}
			if !strings.Contains(err.Error(), "merkle verification") {
				t.Fatalf("expected a merkle-verification refusal, got: %v", err)
			}
		})
	}
}

func TestEvidenceBOM_RejectsLedgerWithoutMerkle(t *testing.T) {
	ledger := buildEvidenceLedger(t)
	ledger.Merkle = nil // a pre-CKM-06 ledger is unverifiable, never best-effort
	if _, err := GenerateEvidenceCycloneDX(ledger, "urn:uuid:test"); err == nil {
		t.Fatal("a ledger without a merkle summary must be refused")
	}
}

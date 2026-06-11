package corpus

// VRC-07 (#553): adversarial proofs for the body-ledger Merkle verifier.
// Doctrine §2.3 — crypto/integrity mechanics prove themselves by FAILING on a
// 1-byte alteration, not by passing on the happy path.

import (
	"strings"
	"testing"
)

func buildVerifiableLedger(t *testing.T) CorpusBodyLedger {
	t.Helper()
	content := []byte("# Rule\n\nBody paragraph one.\n\n## Sub\n\nBody paragraph two.\n")
	segments, err := ScanMarkdown("SRC-1", "01_rbok/rule.md", content)
	if err != nil {
		t.Fatalf("scan markdown: %v", err)
	}
	source := ManifestSource{
		ID:    "SRC-1",
		Path:  "01_rbok/rule.md",
		Hash:  ComputeRawTextHash(content),
		Owner: "domain-owner@example.com",
	}
	adm := source.Admission()
	BackfillAdmission(&adm, source.Path)
	source.AdmissionStatus = adm.AdmissionStatus
	source.AtomizationStatus = adm.AtomizationStatus
	source.SourceRole = adm.SourceRole
	source.FormatSupport = adm.FormatSupport

	ledger, err := BuildCorpusBodyLedger(BodyLedgerInput{
		CorpusRoot: "corpus",
		Sources: []BodyLedgerSourceInput{{
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
		t.Fatal("expected merkle summary on built ledger")
	}
	return ledger
}

func TestVerifyCorpusBodyLedgerProofs_BuiltLedgerVerifies(t *testing.T) {
	ledger := buildVerifiableLedger(t)
	if err := VerifyCorpusBodyLedgerProofs(ledger); err != nil {
		t.Fatalf("built ledger must verify: %v", err)
	}
}

func TestVerifyCorpusBodyLedgerProofs_FailsWhenSourceRowTampered(t *testing.T) {
	// Alter one recorded source hash AFTER proof generation: the recomputed
	// leaf no longer matches the proof's leaf and verification must go red.
	ledger := buildVerifiableLedger(t)
	orig := ledger.Sources[0].Hash
	flipped := "0" + orig[1:]
	if flipped == orig {
		flipped = "1" + orig[1:]
	}
	ledger.Sources[0].Hash = flipped
	err := VerifyCorpusBodyLedgerProofs(ledger)
	if err == nil {
		t.Fatal("tampered source row VERIFIED — the verifier is not real")
	}
	if !strings.Contains(err.Error(), "leaf hash mismatch") {
		t.Fatalf("expected leaf hash mismatch, got: %v", err)
	}
}

func TestVerifyCorpusBodyLedgerProofs_FailsWhenSegmentTampered(t *testing.T) {
	ledger := buildVerifiableLedger(t)
	tampered := false
	for i := range ledger.Sources[0].Segments {
		if ledger.Sources[0].Segments[i].ParentSegmentID == "" {
			ledger.Sources[0].Segments[i].RawTextHash = "deadbeef"
			tampered = true
			break
		}
	}
	if !tampered {
		t.Fatal("fixture has no root segment to tamper")
	}
	if err := VerifyCorpusBodyLedgerProofs(ledger); err == nil {
		t.Fatal("tampered segment VERIFIED — the verifier is not real")
	}
}

func TestVerifyCorpusBodyLedgerProofs_FailsWhenProofHopTampered(t *testing.T) {
	ledger := buildVerifiableLedger(t)
	proof := ledger.Sources[0].MerkleProof
	if proof == nil || len(proof.Path) == 0 {
		t.Fatal("fixture source proof has no path hops")
	}
	proof.Path[0].Hash = "0" + proof.Path[0].Hash[1:]
	err := VerifyCorpusBodyLedgerProofs(ledger)
	if err == nil {
		t.Fatal("tampered proof hop VERIFIED — the verifier is not real")
	}
	if !strings.Contains(err.Error(), "root mismatch") {
		t.Fatalf("expected root mismatch, got: %v", err)
	}
}

func TestVerifyCorpusBodyLedgerProofs_FailsWhenRootTampered(t *testing.T) {
	ledger := buildVerifiableLedger(t)
	ledger.Merkle.Root = "0" + ledger.Merkle.Root[1:]
	if err := VerifyCorpusBodyLedgerProofs(ledger); err == nil {
		t.Fatal("tampered root VERIFIED — the verifier is not real")
	}
}

func TestVerifyCorpusBodyLedgerProofs_FailsWhenLeafCountLies(t *testing.T) {
	ledger := buildVerifiableLedger(t)
	ledger.Merkle.LeafCount++
	if err := VerifyCorpusBodyLedgerProofs(ledger); err == nil {
		t.Fatal("ledger declaring extra leaves VERIFIED — count check is not real")
	}
}

func TestVerifyCorpusBodyLedgerProofs_RejectsLedgerWithoutMerkle(t *testing.T) {
	ledger := buildVerifiableLedger(t)
	ledger.Merkle = nil
	err := VerifyCorpusBodyLedgerProofs(ledger)
	if err == nil {
		t.Fatal("ledger without merkle summary must be unverifiable, not silently green")
	}
	if !strings.Contains(err.Error(), "no merkle summary") {
		t.Fatalf("expected no-merkle-summary error, got: %v", err)
	}
}

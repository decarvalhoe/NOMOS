package atomization

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

var certNow = time.Date(2026, 5, 2, 16, 0, 0, 0, time.UTC)

func validAtoms() []AtomForVerification {
	return []AtomForVerification{
		{
			AtomID: "ATOM-ART-1382", Kind: "rule",
			Hash: "sha256:aaaa", ReviewState: "approved",
			SourceSpan: &VerificationSourceSpan{
				SourceID: "SRC-001", Path: "sources/code.pdf",
				Hash: "sha256:bbbb",
			},
		},
		{
			AtomID: "ATOM-ART-1383", Kind: "rule",
			Hash: "sha256:cccc", ReviewState: "approved",
			SourceSpan: &VerificationSourceSpan{
				SourceID: "SRC-001", Path: "sources/code.pdf",
				Hash: "sha256:dddd",
			},
		},
	}
}

func certOpts() CertificateOptions {
	return CertificateOptions{
		CertID: "CERT-TEST-001",
		Issuer: "test-runner",
		Domain: "civil-law",
		Now:    certNow,
	}
}

func TestVerifyAndCertifyAllPassed(t *testing.T) {
	cert := VerifyAndCertify(validAtoms(), certOpts())

	if !cert.Passed {
		t.Fatalf("expected passed, got failed. Gates: %v", cert.GateResults)
	}
	if cert.Summary.Failed != 0 {
		t.Fatalf("expected 0 failed gates, got %d", cert.Summary.Failed)
	}
	if cert.Summary.TotalGates != 6 {
		t.Fatalf("expected 6 gates, got %d", cert.Summary.TotalGates)
	}
	if cert.AtomCount != 2 {
		t.Fatalf("expected 2 atoms, got %d", cert.AtomCount)
	}
	if cert.CertID != "CERT-TEST-001" {
		t.Fatalf("expected CERT-TEST-001, got %s", cert.CertID)
	}
	if cert.Domain != "civil-law" {
		t.Fatalf("expected civil-law, got %s", cert.Domain)
	}
}

func TestVerifyAndCertifyInvalidID(t *testing.T) {
	atoms := []AtomForVerification{
		{AtomID: "bad-id", Kind: "rule", Hash: "sha256:aa", ReviewState: "approved",
			SourceSpan: &VerificationSourceSpan{SourceID: "S", Path: "p", Hash: "sha256:bb"}},
	}
	cert := VerifyAndCertify(atoms, certOpts())

	if cert.Passed {
		t.Fatal("expected failed for invalid ID")
	}
	assertGateFailed(t, cert, "atom.stable_ids")
}

func TestVerifyAndCertifyMissingHash(t *testing.T) {
	atoms := []AtomForVerification{
		{AtomID: "ATOM-TEST-001", Kind: "rule", Hash: "", ReviewState: "approved",
			SourceSpan: &VerificationSourceSpan{SourceID: "S", Path: "p", Hash: "sha256:bb"}},
	}
	cert := VerifyAndCertify(atoms, certOpts())

	if cert.Passed {
		t.Fatal("expected failed for missing hash")
	}
	assertGateFailed(t, cert, "atom.hashes")
}

func TestVerifyAndCertifyMissingSourceSpan(t *testing.T) {
	atoms := []AtomForVerification{
		{AtomID: "ATOM-TEST-001", Kind: "rule", Hash: "sha256:aa", ReviewState: "approved",
			SourceSpan: nil},
	}
	cert := VerifyAndCertify(atoms, certOpts())

	if cert.Passed {
		t.Fatal("expected failed for missing source span")
	}
	assertGateFailed(t, cert, "atom.source_spans")
}

func TestVerifyAndCertifySourceSpanNoHash(t *testing.T) {
	atoms := []AtomForVerification{
		{AtomID: "ATOM-TEST-001", Kind: "rule", Hash: "sha256:aa", ReviewState: "approved",
			SourceSpan: &VerificationSourceSpan{SourceID: "S", Path: "p", Hash: ""}},
	}
	cert := VerifyAndCertify(atoms, certOpts())

	// Warning, not failure
	assertGateStatus(t, cert, "atom.source_spans", "warning")
}

func TestVerifyAndCertifyInvalidReviewState(t *testing.T) {
	atoms := []AtomForVerification{
		{AtomID: "ATOM-TEST-001", Kind: "rule", Hash: "sha256:aa", ReviewState: "bogus",
			SourceSpan: &VerificationSourceSpan{SourceID: "S", Path: "p", Hash: "sha256:bb"}},
	}
	cert := VerifyAndCertify(atoms, certOpts())

	if cert.Passed {
		t.Fatal("expected failed for invalid review state")
	}
	assertGateFailed(t, cert, "atom.review_states")
}

func TestVerifyAndCertifyAllDraft(t *testing.T) {
	atoms := []AtomForVerification{
		{AtomID: "ATOM-TEST-001", Kind: "rule", Hash: "sha256:aa", ReviewState: "draft",
			SourceSpan: &VerificationSourceSpan{SourceID: "S", Path: "p", Hash: "sha256:bb"}},
	}
	cert := VerifyAndCertify(atoms, certOpts())

	// Warning for all-draft
	assertGateStatus(t, cert, "atom.review_states", "warning")
	if !cert.Passed {
		t.Fatal("all-draft is a warning, not a failure")
	}
}

func TestVerifyAndCertifyDuplicateIDs(t *testing.T) {
	atoms := []AtomForVerification{
		{AtomID: "ATOM-DUP", Kind: "rule", Hash: "sha256:aa", ReviewState: "approved",
			SourceSpan: &VerificationSourceSpan{SourceID: "S", Path: "p", Hash: "sha256:bb"}},
		{AtomID: "ATOM-DUP", Kind: "rule", Hash: "sha256:cc", ReviewState: "approved",
			SourceSpan: &VerificationSourceSpan{SourceID: "S", Path: "p", Hash: "sha256:dd"}},
	}
	cert := VerifyAndCertify(atoms, certOpts())

	if cert.Passed {
		t.Fatal("expected failed for duplicate IDs")
	}
	assertGateFailed(t, cert, "atom.no_duplicates")
}

func TestVerifyAndCertifyEmptyAtoms(t *testing.T) {
	cert := VerifyAndCertify(nil, certOpts())

	if cert.Passed {
		t.Fatal("expected failed for empty atoms")
	}
	assertGateFailed(t, cert, "atom.minimum_count")
}

func TestSetHashDeterministic(t *testing.T) {
	atoms := validAtoms()
	hash1 := computeSetHash(atoms)

	// Reverse order — should produce same hash (sorted internally)
	reversed := []AtomForVerification{atoms[1], atoms[0]}
	hash2 := computeSetHash(reversed)

	if hash1 != hash2 {
		t.Fatalf("expected deterministic hash regardless of order: %s != %s", hash1, hash2)
	}
}

func TestSetHashChangesWithContent(t *testing.T) {
	atoms1 := validAtoms()
	hash1 := computeSetHash(atoms1)

	atoms2 := validAtoms()
	atoms2[0].Hash = "sha256:different"
	hash2 := computeSetHash(atoms2)

	if hash1 == hash2 {
		t.Fatal("expected different hash for different atom content")
	}
}

func TestWriteCertificate(t *testing.T) {
	cert := VerifyAndCertify(validAtoms(), certOpts())

	var buf bytes.Buffer
	if err := WriteCertificate(&buf, cert); err != nil {
		t.Fatalf("write error: %v", err)
	}

	var decoded AtomizationCertificate
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.CertID != "CERT-TEST-001" {
		t.Fatalf("expected CERT-TEST-001 after round-trip, got %s", decoded.CertID)
	}
	if !decoded.Passed {
		t.Fatal("expected passed after round-trip")
	}
}

func TestCertificateSchemaVersion(t *testing.T) {
	cert := VerifyAndCertify(validAtoms(), certOpts())
	if cert.SchemaVersion != "0.1.0" {
		t.Fatalf("expected 0.1.0, got %s", cert.SchemaVersion)
	}
}

func TestCertificateSetHashFormat(t *testing.T) {
	cert := VerifyAndCertify(validAtoms(), certOpts())
	if !hashPattern.MatchString(cert.SetHash) {
		t.Fatalf("expected valid hash format, got %s", cert.SetHash)
	}
}

func assertGateFailed(t *testing.T, cert AtomizationCertificate, gateID string) {
	t.Helper()
	assertGateStatus(t, cert, gateID, "failed")
}

func assertGateStatus(t *testing.T, cert AtomizationCertificate, gateID string, expected string) {
	t.Helper()
	for _, g := range cert.GateResults {
		if g.ID == gateID {
			if g.Status != expected {
				t.Fatalf("gate %s: expected %s, got %s (%s)", gateID, expected, g.Status, g.Message)
			}
			return
		}
	}
	t.Fatalf("gate %s not found in results", gateID)
}

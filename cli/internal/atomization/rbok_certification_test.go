package atomization

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func goodAtom(id string) Atom {
	return Atom{
		ID: id, CanonicalRef: "doc#" + id, Type: AtomRule,
		Text: "Some rule content.", ContentHash: "sha256:" + id + "hash",
		SourceSpan: SourceSpan{File: "test.md", StartLine: 1, EndLine: 3},
		BlockID: "b-" + id, ReviewState: ReviewDraft,
	}
}

func goodAtomSet() AtomSet {
	return AtomSet{
		DocumentRef: "test-doc", SourceFile: "test.md",
		SourceHash: "sha256:sourcehash", AtomCount: 3,
		Atoms: []Atom{goodAtom("A1"), goodAtom("A2"), goodAtom("A3")},
	}
}

func losslessAST() AST {
	return AST{
		Root: "root", SourceHash: "sha256:sourcehash", SourceLen: 100,
		Blocks: []Block{{ID: "root", Type: BlockDocument}},
		LossReport: LossReport{
			TotalSourceBytes: 100, CoveredBytes: 100, LostBytes: 0,
			LossRatio: 0.0, IsLossless: true,
		},
	}
}

func lossyAST(lossRatio float64, lostBytes int) AST {
	total := 1000
	covered := total - lostBytes
	return AST{
		Root: "root", SourceHash: "sha256:sourcehash", SourceLen: total,
		Blocks: []Block{{ID: "root", Type: BlockDocument}},
		LossReport: LossReport{
			TotalSourceBytes: total, CoveredBytes: covered, LostBytes: lostBytes,
			LossRatio: lossRatio, IsLossless: false,
			LostSpans: []LostSpan{{StartLine: 10, EndLine: 15, Preview: "..."}},
		},
	}
}

// --- Certified ---

func TestCertifyCertified(t *testing.T) {
	cert := Certify(goodAtomSet(), losslessAST(), DefaultThresholds())
	if cert.Level != CertCertified {
		t.Fatalf("expected certified, got %q (findings: %v)", cert.Level, cert.Findings)
	}
	if cert.BlockingCount != 0 {
		t.Fatalf("expected 0 blocking, got %d", cert.BlockingCount)
	}
	if !cert.StableIDs {
		t.Fatal("expected stable IDs")
	}
	if !cert.SourceSpans {
		t.Fatal("expected source spans valid")
	}
	if !cert.HashChain {
		t.Fatal("expected hash chain valid")
	}
	if cert.ChainHash == "" {
		t.Fatal("expected non-empty chain hash")
	}
	if !cert.IsLossless {
		t.Fatal("expected lossless")
	}
	if cert.CoveragePercent != 100.0 {
		t.Fatalf("expected 100%%, got %.1f%%", cert.CoveragePercent)
	}
}

// --- Low coverage ---

func TestCertifyLowCoverage(t *testing.T) {
	ast := lossyAST(0.20, 200)
	cert := Certify(goodAtomSet(), ast, DefaultThresholds())
	if cert.Level != CertFailed {
		t.Fatalf("expected failed, got %q", cert.Level)
	}
	assertCertFinding(t, cert, "LOW_COVERAGE")
}

// --- High loss ---

func TestCertifyHighLoss(t *testing.T) {
	ast := lossyAST(0.10, 100)
	cert := Certify(goodAtomSet(), ast, DefaultThresholds())
	assertCertFinding(t, cert, "HIGH_LOSS")
}

// --- Require lossless ---

func TestCertifyRequireLossless(t *testing.T) {
	thresholds := DefaultThresholds()
	thresholds.RequireLossless = true
	ast := lossyAST(0.01, 10)
	cert := Certify(goodAtomSet(), ast, thresholds)
	assertCertFinding(t, cert, "NOT_LOSSLESS")
}

// --- Duplicate IDs ---

func TestCertifyDuplicateIDs(t *testing.T) {
	as := AtomSet{
		DocumentRef: "doc", SourceFile: "test.md", SourceHash: "sha256:src",
		AtomCount: 2,
		Atoms: []Atom{goodAtom("DUP"), goodAtom("DUP")},
	}
	cert := Certify(as, losslessAST(), DefaultThresholds())
	if cert.Level != CertFailed {
		t.Fatalf("expected failed, got %q", cert.Level)
	}
	if cert.StableIDs {
		t.Fatal("expected stable IDs to be false")
	}
	assertCertFinding(t, cert, "DUPLICATE_ATOM_ID")
}

// --- Empty atom ID ---

func TestCertifyEmptyAtomID(t *testing.T) {
	atom := goodAtom("")
	as := AtomSet{
		DocumentRef: "doc", SourceFile: "test.md", SourceHash: "sha256:src",
		AtomCount: 1, Atoms: []Atom{atom},
	}
	cert := Certify(as, losslessAST(), DefaultThresholds())
	assertCertFinding(t, cert, "EMPTY_ATOM_ID")
}

// --- Missing source span ---

func TestCertifyMissingSourceSpan(t *testing.T) {
	atom := goodAtom("A1")
	atom.SourceSpan = SourceSpan{}
	as := AtomSet{
		DocumentRef: "doc", SourceFile: "test.md", SourceHash: "sha256:src",
		AtomCount: 1, Atoms: []Atom{atom},
	}
	cert := Certify(as, losslessAST(), DefaultThresholds())
	assertCertFinding(t, cert, "MISSING_SOURCE_SPAN")
	// Non-blocking, so should be provisional
	if cert.Level == CertFailed {
		t.Fatal("missing span should be provisional, not failed")
	}
}

// --- Inverted source span ---

func TestCertifyInvertedSourceSpan(t *testing.T) {
	atom := goodAtom("A1")
	atom.SourceSpan = SourceSpan{File: "t.md", StartLine: 10, EndLine: 5}
	as := AtomSet{
		DocumentRef: "doc", SourceFile: "test.md", SourceHash: "sha256:src",
		AtomCount: 1, Atoms: []Atom{atom},
	}
	cert := Certify(as, losslessAST(), DefaultThresholds())
	assertCertFinding(t, cert, "INVALID_SOURCE_SPAN")
}

// --- Missing content hash ---

func TestCertifyMissingContentHash(t *testing.T) {
	atom := goodAtom("A1")
	atom.ContentHash = ""
	as := AtomSet{
		DocumentRef: "doc", SourceFile: "test.md", SourceHash: "sha256:src",
		AtomCount: 1, Atoms: []Atom{atom},
	}
	cert := Certify(as, losslessAST(), DefaultThresholds())
	assertCertFinding(t, cert, "MISSING_CONTENT_HASH")
	if cert.HashChain {
		t.Fatal("expected hash chain invalid")
	}
}

// --- Empty atoms ---

func TestCertifyEmptyAtoms(t *testing.T) {
	atom := goodAtom("A1")
	atom.Text = ""
	as := AtomSet{
		DocumentRef: "doc", SourceFile: "test.md", SourceHash: "sha256:src",
		AtomCount: 1, Atoms: []Atom{atom},
	}
	cert := Certify(as, losslessAST(), DefaultThresholds())
	assertCertFinding(t, cert, "EXCESSIVE_EMPTY_ATOMS")
}

// --- Zero atoms ---

func TestCertifyZeroAtoms(t *testing.T) {
	cert := Certify(AtomSet{AtomCount: 0}, losslessAST(), DefaultThresholds())
	if cert.Level != CertFailed {
		t.Fatalf("expected failed for zero atoms, got %q", cert.Level)
	}
	assertCertFinding(t, cert, "NO_ATOMS")
}

// --- Hash chain determinism ---

func TestCertifyHashChainDeterministic(t *testing.T) {
	as := goodAtomSet()
	ast := losslessAST()
	c1 := Certify(as, ast, DefaultThresholds())
	c2 := Certify(as, ast, DefaultThresholds())
	if c1.ChainHash != c2.ChainHash {
		t.Fatalf("chain hash not deterministic: %q vs %q", c1.ChainHash, c2.ChainHash)
	}
	if !strings.HasPrefix(c1.ChainHash, "sha256:") {
		t.Fatalf("expected sha256: prefix, got %q", c1.ChainHash)
	}
}

// --- Provisional (non-blocking findings only) ---

func TestCertifyProvisional(t *testing.T) {
	atom := goodAtom("A1")
	atom.SourceSpan = SourceSpan{} // missing span = non-blocking
	as := AtomSet{
		DocumentRef: "doc", SourceFile: "test.md", SourceHash: "sha256:src",
		AtomCount: 1, Atoms: []Atom{atom},
	}
	cert := Certify(as, losslessAST(), DefaultThresholds())
	if cert.Level != CertProvisional {
		t.Fatalf("expected provisional, got %q", cert.Level)
	}
}

// --- Certificate JSON ---

func TestCertifyWriteJSON(t *testing.T) {
	cert := Certify(goodAtomSet(), losslessAST(), DefaultThresholds())
	var buf bytes.Buffer
	if err := WriteCertificateJSON(&buf, cert); err != nil {
		t.Fatal(err)
	}
	var decoded CertificationCertificate
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded.Level != CertCertified {
		t.Fatalf("roundtrip level: expected certified, got %q", decoded.Level)
	}
	if decoded.ChainHash != cert.ChainHash {
		t.Fatal("chain hash mismatch on roundtrip")
	}
}

// --- Level constants ---

func TestCertificationLevelConstants(t *testing.T) {
	if CertCertified != "certified" || CertProvisional != "provisional" || CertFailed != "failed" {
		t.Fatal("level constants wrong")
	}
}

// --- helper ---

func assertCertFinding(t *testing.T, cert CertificationCertificate, code string) {
	t.Helper()
	for _, f := range cert.Findings {
		if f.Code == code {
			return
		}
	}
	var codes []string
	for _, f := range cert.Findings {
		codes = append(codes, f.Code)
	}
	t.Fatalf("expected finding %q in %v", code, codes)
}

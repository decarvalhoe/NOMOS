package connector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const sampleCSV = "HistoricalCode,BfsCode,Name\n1,1,Zürich\n2,2,Genève\n261,261,Zürich\n"

func sha256Prefixed(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// A real HTTP fetch over a real socket (httptest) — the hash is computed from
// the bytes that actually came back, never fabricated.
func TestFetch_RealHashAndProvenance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("ETag", "\"abc123\"")
		_, _ = w.Write([]byte(sampleCSV))
	}))
	defer srv.Close()

	content, result, err := Fetch(context.Background(), srv.URL, FetchOptions{Now: time.Unix(0, 0).UTC()})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(content) != sampleCSV {
		t.Fatal("fetched content does not match served content")
	}
	if result.SHA256 != sha256Prefixed([]byte(sampleCSV)) {
		t.Fatalf("recorded hash %s != real hash %s", result.SHA256, sha256Prefixed([]byte(sampleCSV)))
	}
	if result.ByteCount != len(sampleCSV) {
		t.Fatalf("byte count %d != %d", result.ByteCount, len(sampleCSV))
	}
	if result.StatusCode != 200 {
		t.Fatalf("status %d", result.StatusCode)
	}
	if !strings.HasPrefix(result.ContentType, "text/plain") {
		t.Fatalf("content type %q", result.ContentType)
	}
	if result.ETag == "" {
		t.Fatal("ETag not captured")
	}
}

func TestBuildLineLedger_TilesExactlyWithZeroUncovered(t *testing.T) {
	ledger := BuildLineLedger([]byte(sampleCSV))
	if !ledger.IsFullyCovered() {
		t.Fatalf("ledger not fully covered: %d uncovered", ledger.UncoveredBytes)
	}
	if ledger.TotalBytes != len(sampleCSV) {
		t.Fatalf("total %d != %d", ledger.TotalBytes, len(sampleCSV))
	}
	// Spans must be contiguous and non-overlapping: seg[i].EndByte == seg[i+1].StartByte.
	prevEnd := 0
	for i, seg := range ledger.Segments {
		if seg.StartByte != prevEnd {
			t.Fatalf("segment %d starts at %d, expected %d (gap/overlap)", i, seg.StartByte, prevEnd)
		}
		if seg.EndByte <= seg.StartByte {
			t.Fatalf("segment %d has non-positive span", i)
		}
		prevEnd = seg.EndByte
	}
	if prevEnd != len(sampleCSV) {
		t.Fatalf("segments end at %d, expected %d", prevEnd, len(sampleCSV))
	}
}

func TestBuildLineLedger_NoTrailingNewline(t *testing.T) {
	ledger := BuildLineLedger([]byte("alpha\nbeta"))
	if !ledger.IsFullyCovered() {
		t.Fatalf("non-newline-terminated content left %d bytes uncovered", ledger.UncoveredBytes)
	}
	if ledger.SegmentCount != 2 {
		t.Fatalf("expected 2 segments, got %d", ledger.SegmentCount)
	}
}

func TestBuildEvidence_ValidatesAndCarriesNoFullText(t *testing.T) {
	content := []byte(sampleCSV)
	fetch := FetchResult{
		URL:        "https://example.test/communes.csv",
		FetchedAt:  "2026-06-10T00:00:00Z",
		StatusCode: 200,
		ByteCount:  len(content),
		SHA256:     sha256Prefixed(content),
	}
	ledger := BuildLineLedger(content)
	ev := BuildEvidence("ch-ofs-commune-register", content, fetch, ledger, 2)

	if err := ev.Validate(); err != nil {
		t.Fatalf("evidence invalid: %v", err)
	}
	if !ev.NoFullText {
		t.Fatal("evidence must declare no_full_text")
	}
	if ev.Source.Authority == "" || ev.Source.Jurisdiction != "CH" {
		t.Fatalf("source descriptor not resolved: %+v", ev.Source)
	}
	if ev.AtomCount != 4 {
		t.Fatalf("atom count = %d, want 4 (header + 3 rows)", ev.AtomCount)
	}
	if len(ev.AtomSample) != 2 {
		t.Fatalf("sample length = %d, want 2", len(ev.AtomSample))
	}
	// The committed ledger drops the per-line segment list (compact, no text map).
	if ev.BodyLedger.Segments != nil {
		t.Fatal("evidence ledger should not embed the full segment list")
	}
}

// Adversarial: the audit's complaint was *synthetic* hashes. Evidence with a
// digest that is not a real sha256 must be rejected.
func TestEvidence_RejectsSyntheticHash(t *testing.T) {
	content := []byte(sampleCSV)
	fetch := FetchResult{URL: "https://x", ByteCount: len(content), SHA256: "sha256:deadbeef"}
	ledger := BuildLineLedger(content)
	ev := BuildEvidence("ch-ofs-commune-register", content, fetch, ledger, 1)
	if err := ev.Validate(); err == nil {
		t.Fatal("evidence with a synthetic (short) hash passed validation")
	}
}

// Adversarial: any uncovered byte must fail validation.
func TestEvidence_RejectsUncoveredBytes(t *testing.T) {
	content := []byte(sampleCSV)
	fetch := FetchResult{URL: "https://x", ByteCount: len(content), SHA256: sha256Prefixed(content)}
	ledger := BuildLineLedger(content)
	ledger.CoveredBytes -= 5
	ledger.UncoveredBytes += 5
	ev := BuildEvidence("ch-ofs-commune-register", content, fetch, ledger, 1)
	if err := ev.Validate(); err == nil {
		t.Fatal("evidence with uncovered bytes passed validation")
	}
}

func TestVerifyContentHash(t *testing.T) {
	content := []byte(sampleCSV)
	fetch := FetchResult{URL: "https://x", ByteCount: len(content), SHA256: sha256Prefixed(content)}
	ev := BuildEvidence("ch-ofs-commune-register", content, fetch, BuildLineLedger(content), 1)
	if err := ev.VerifyContentHash(content); err != nil {
		t.Fatalf("VerifyContentHash on real content failed: %v", err)
	}
	if err := ev.VerifyContentHash([]byte("tampered")); err == nil {
		t.Fatal("VerifyContentHash accepted tampered content")
	}
}

func TestFetch_RejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	if _, _, err := Fetch(context.Background(), srv.URL, FetchOptions{}); err == nil {
		t.Fatal("Fetch accepted a 404 response")
	}
}

func TestFetch_RejectsNonHTTP(t *testing.T) {
	if _, _, err := Fetch(context.Background(), "ftp://example.test/x", FetchOptions{}); err == nil {
		t.Fatal("Fetch accepted a non-http(s) URL")
	}
}

func TestFetch_EnforcesMaxBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("x", 1000)))
	}))
	defer srv.Close()
	if _, _, err := Fetch(context.Background(), srv.URL, FetchOptions{MaxBytes: 100}); err == nil {
		t.Fatal("Fetch did not enforce max bytes")
	}
}

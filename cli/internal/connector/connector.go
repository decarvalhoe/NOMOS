// Package connector implements a read-only source connector that performs a REAL
// fetch of an open Swiss source, hashes the real bytes, atomizes them to spans,
// and proves byte-level coverage (CKM-H5 / #523).
//
// The audit (#518) found CKM-10 carried a CUE manifest + tests but only
// *synthetic* hashes and no live fetch. This package closes that: it does a real
// HTTP GET, computes the real sha256 of the retrieved bytes (never a fabricated
// digest), splits the content into line segments with exact byte spans, and
// asserts that zero bytes are left uncovered.
//
// Scope discipline (doctrine §2.6): targets OPEN government data (OFS commune
// register, Fedlex/ELI) — never the paid SIA norms. The evidence records the
// hash, byte count, and span coverage; it does not redistribute full text.
package connector

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultUserAgent identifies the connector to upstream servers.
const DefaultUserAgent = "nomos-connector/0.1 (+https://nomos.dev; read-only)"

// DefaultMaxBytes caps a fetch so a connector cannot be steered into an
// unbounded download.
const DefaultMaxBytes = 32 << 20 // 32 MiB

// EvidenceSchemaVersion is the connector evidence contract version.
const EvidenceSchemaVersion = "nomos-connector-evidence-v1"

// FetchResult records the real provenance of a fetch.
type FetchResult struct {
	URL          string `json:"url"`
	FetchedAt    string `json:"fetched_at"`
	StatusCode   int    `json:"status_code"`
	ContentType  string `json:"content_type"`
	ByteCount    int    `json:"byte_count"`
	SHA256       string `json:"sha256"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
	// Accept records the content negotiation REQUESTED, when any. ELI endpoints
	// (Fedlex) serve an Angular shell on a plain GET and the machine
	// representation (RDF/XML) only under negotiation — provenance must say
	// which representation was asked for, or the hash is not reproducible.
	Accept string `json:"accept,omitempty"`
}

// FetchOptions configures a fetch.
type FetchOptions struct {
	Client    *http.Client
	UserAgent string
	MaxBytes  int64
	Now       time.Time
	// Accept, when non-empty, is sent as the Accept header (ELI content
	// negotiation). Empty keeps the historical behavior byte-identical.
	Accept string
}

// Fetch performs a read-only HTTP GET and returns the raw bytes plus real
// provenance (status, content-type, byte count, sha256). It never writes
// anywhere and never fabricates a digest.
func Fetch(ctx context.Context, url string, opts FetchOptions) ([]byte, FetchResult, error) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return nil, FetchResult{}, fmt.Errorf("connector: url must be http(s): %q", url)
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, FetchResult{}, fmt.Errorf("connector: build request: %w", err)
	}
	req.Header.Set("User-Agent", ua)
	if strings.TrimSpace(opts.Accept) != "" {
		req.Header.Set("Accept", opts.Accept)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, FetchResult{}, fmt.Errorf("connector: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, FetchResult{}, fmt.Errorf("connector: %s returned status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, FetchResult{}, fmt.Errorf("connector: read body: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, FetchResult{}, fmt.Errorf("connector: response exceeds max %d bytes", maxBytes)
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sum := sha256.Sum256(body)
	return body, FetchResult{
		URL:          url,
		FetchedAt:    now.UTC().Format(time.RFC3339),
		StatusCode:   resp.StatusCode,
		ContentType:  resp.Header.Get("Content-Type"),
		ByteCount:    len(body),
		SHA256:       "sha256:" + hex.EncodeToString(sum[:]),
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		Accept:       strings.TrimSpace(opts.Accept),
	}, nil
}

// Segment is one line-bounded byte span of the fetched content. Spans are
// half-open [StartByte, EndByte) and tile the content exactly.
type Segment struct {
	Index       int    `json:"index"`
	Line        int    `json:"line"`
	StartByte   int    `json:"start_byte"`
	EndByte     int    `json:"end_byte"`
	ByteLen     int    `json:"byte_len"`
	ContentHash string `json:"content_hash"`
}

// BodyLedger proves byte-level coverage of the fetched content.
type BodyLedger struct {
	Method        string    `json:"method"`
	TotalBytes    int       `json:"total_bytes"`
	CoveredBytes  int       `json:"covered_bytes"`
	UncoveredBytes int      `json:"uncovered_bytes"`
	SegmentCount  int       `json:"segment_count"`
	Segments      []Segment `json:"segments,omitempty"`
}

// BuildLineLedger splits content into line segments whose half-open byte spans
// tile the whole document. Every byte — including each newline — belongs to
// exactly one segment, so a correct ledger has UncoveredBytes == 0.
func BuildLineLedger(content []byte) BodyLedger {
	var segments []Segment
	start := 0
	line := 1
	idx := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			end := i + 1 // newline belongs to this segment
			segments = append(segments, makeSegment(idx, line, start, end, content[start:end]))
			idx++
			line++
			start = end
		}
	}
	if start < len(content) {
		segments = append(segments, makeSegment(idx, line, start, len(content), content[start:len(content)]))
	}

	covered := 0
	for _, s := range segments {
		covered += s.ByteLen
	}
	return BodyLedger{
		Method:         "line_segments",
		TotalBytes:     len(content),
		CoveredBytes:   covered,
		UncoveredBytes: len(content) - covered,
		SegmentCount:   len(segments),
		Segments:       segments,
	}
}

func makeSegment(idx, line, start, end int, b []byte) Segment {
	sum := sha256.Sum256(b)
	return Segment{
		Index:       idx,
		Line:        line,
		StartByte:   start,
		EndByte:     end,
		ByteLen:     end - start,
		ContentHash: "sha256:" + hex.EncodeToString(sum[:]),
	}
}

// IsFullyCovered reports whether the ledger leaves zero bytes uncovered.
func (l BodyLedger) IsFullyCovered() bool {
	return l.TotalBytes >= 0 && l.UncoveredBytes == 0 && l.CoveredBytes == l.TotalBytes
}

// VRC-30 / #567 — the born-digital PDF adapter (plan C1), claim ladder:
//
//	(1) born-digital TEXT — this file: positioned text runs are extracted and
//	    grouped into lines; locators carry page + line + Y position.
//	(2) tagged-PDF structure — NOT claimed yet.
//	(3) OCR / scanned content — explicitly OUT OF CLAIM: a page without
//	    extractable text becomes an UNSUPPORTED record (the body-ledger
//	    pattern), never silently dropped and never fabricated.
//
// Library decision (doctrine §2.6, registre licences): github.com/ledongthuc/pdf
// (BSD-3, the maintained rsc.io/pdf lineage) — permissive license, pure Go
// (zero cgo), and positioned text runs (X/Y per run), which is exactly what
// page+position locators require. Rejected: unipdf / MuPDF / poppler bindings
// (AGPL/GPL — the doctrine isolates them out of process), pdfcpu (Apache, but
// its text extraction is not position-faithful enough for span locators),
// go-pdfium (license-clean BSD via WASM but a heavyweight runtime — the
// industrialization candidate, not the first slice).
//
// Byte offsets: a PDF is a compressed binary container; raw-file byte offsets
// for DECODED text are not claimable, so SpanInfo.ByteOff/ByteLen stay 0 and
// the locator lives in the span ID ("pdf:p2:l5:y712.1"). The file-level sha256
// drift discipline (one mutated byte → different parse or different source
// hash) is proven in the tests instead.
package fidelity

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

import ledongthuc "github.com/ledongthuc/pdf"

// PDFAdapter parses born-digital PDF text with page+position locators.
type PDFAdapter struct{}

// Name returns the adapter identifier.
func (PDFAdapter) Name() string { return "pdf" }

// Extensions returns the extensions this adapter claims.
func (PDFAdapter) Extensions() []string { return []string{".pdf"} }

// pdfLine is one reconstructed text line of one page.
type pdfLine struct {
	page int
	y    float64
	text string
}

// extractPDF walks every page and returns the reconstructed text lines plus
// one unsupported record per page WITHOUT extractable born-digital text. The
// parser library can panic on malformed input, so the whole walk is fenced
// and converted into a fail-closed error.
func extractPDF(source []byte) (lines []pdfLine, unsupported []int, err error) {
	defer func() {
		if r := recover(); r != nil {
			lines, unsupported = nil, nil
			err = fmt.Errorf("pdf: malformed document: %v", r)
		}
	}()

	reader, err := ledongthuc.NewReader(bytes.NewReader(source), int64(len(source)))
	if err != nil {
		return nil, nil, fmt.Errorf("pdf: %w", err)
	}

	for pageNum := 1; pageNum <= reader.NumPage(); pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			unsupported = append(unsupported, pageNum)
			continue
		}
		content := page.Content()
		if len(content.Text) == 0 {
			// Image-only / scanned / empty page: out of the born-digital-text
			// claim — recorded, never dropped.
			unsupported = append(unsupported, pageNum)
			continue
		}
		// Group positioned runs into lines by their Y coordinate (PDF Y grows
		// upward; reading order = descending Y, then ascending X).
		type runKey struct{ y float64 }
		byY := map[float64][]ledongthuc.Text{}
		for _, t := range content.Text {
			byY[t.Y] = append(byY[t.Y], t)
		}
		ys := make([]float64, 0, len(byY))
		for y := range byY {
			ys = append(ys, y)
		}
		sort.Sort(sort.Reverse(sort.Float64Slice(ys)))
		pageHadText := false
		for _, y := range ys {
			runs := byY[y]
			sort.Slice(runs, func(i, j int) bool { return runs[i].X < runs[j].X })
			var b bytes.Buffer
			for _, r := range runs {
				b.WriteString(r.S)
			}
			text := b.String()
			if len(bytes.TrimSpace([]byte(text))) == 0 {
				continue
			}
			pageHadText = true
			lines = append(lines, pdfLine{page: pageNum, y: y, text: text})
		}
		if !pageHadText {
			unsupported = append(unsupported, pageNum)
		}
	}
	return lines, unsupported, nil
}

// Parse extracts born-digital text lines as spans. Every page yields either
// text_line spans or an explicit unsupported record — zero silent loss.
func (p PDFAdapter) Parse(source []byte, filename string) (ParseResult, error) {
	lines, unsupported, err := extractPDF(source)
	if err != nil {
		return ParseResult{}, err
	}
	result := ParseResult{Format: "pdf"}
	lineNo := 0
	for _, ln := range lines {
		lineNo++
		// The span ID is content-addressed (page + line + Y position + a short
		// hash of the line text): mutating ONE byte of carried text drifts the
		// parse output itself — the C1 drift bar, proven in the tests.
		result.Spans = append(result.Spans, SpanInfo{
			ID:        fmt.Sprintf("pdf:p%d:l%d:y%.1f:h%s", ln.page, lineNo, ln.y, shortTextHash(ln.text)),
			NodeType:  "text_line",
			StartLine: lineNo,
			EndLine:   lineNo,
		})
	}
	for _, pageNum := range unsupported {
		result.Spans = append(result.Spans, SpanInfo{
			ID:       fmt.Sprintf("pdf:p%d:unsupported", pageNum),
			NodeType: "unsupported",
		})
		result.Errors = append(result.Errors,
			fmt.Sprintf("page %d: no extractable born-digital text (image-only/scanned is out of claim — OCR is never claimed without proof)", pageNum))
	}
	result.NodeCount = len(result.Spans)
	if result.NodeCount == 0 {
		return ParseResult{}, fmt.Errorf("pdf: document has no pages")
	}
	return result, nil
}

// Validate checks the document parses as a PDF and reports, as findings, any
// page outside the born-digital-text claim.
func (p PDFAdapter) Validate(source []byte, filename string) ValidationResult {
	lines, unsupported, err := extractPDF(source)
	if err != nil {
		return ValidationResult{Valid: false, Findings: []string{err.Error()}}
	}
	res := ValidationResult{Valid: true}
	if len(lines) == 0 {
		res.Findings = append(res.Findings,
			"no born-digital text anywhere in the document — scanned/image PDF is out of claim (ladder rung 3, OCR unproven)")
	}
	for _, pageNum := range unsupported {
		res.Findings = append(res.Findings, fmt.Sprintf("page %d: out of born-digital-text claim", pageNum))
	}
	return res
}

// Spans is the fast path — same extraction, spans only.
func (p PDFAdapter) Spans(source []byte, filename string) ([]SpanInfo, error) {
	parsed, err := p.Parse(source, filename)
	if err != nil {
		return nil, err
	}
	return parsed.Spans, nil
}

// shortTextHash content-addresses one extracted line (8 hex chars suffice for
// drift visibility; the full integrity story stays at the file-level sha256).
func shortTextHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:4])
}

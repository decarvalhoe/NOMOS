package fidelity

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// buildPDF assembles a minimal, uncompressed, born-digital PDF with EXACT xref
// offsets — generated deterministically in-test (license-safe, no binary
// committed; same discipline as the portable golden corpus). Each entry of
// pages is the list of text lines of one page; an empty list produces an
// image-only page (no text operators — the scanned/out-of-claim case).
func buildPDF(pages [][]string) []byte {
	var buf bytes.Buffer
	offsets := []int{0} // object 0 is the free head
	writeObj := func(body string) {
		offsets = append(offsets, buf.Len())
		buf.WriteString(body)
	}

	buf.WriteString("%PDF-1.4\n")

	kids := make([]string, len(pages))
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", 4+2*i)
	}
	writeObj("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")
	writeObj(fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n",
		strings.Join(kids, " "), len(pages)))
	writeObj("3 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	for i, lines := range pages {
		pageID, contentID := 4+2*i, 5+2*i
		var content string
		if len(lines) == 0 {
			content = "q 1 0 0 1 0 0 cm Q"
		} else {
			var ops []string
			ops = append(ops, "BT /F1 12 Tf 72 720 Td")
			for j, line := range lines {
				if j > 0 {
					ops = append(ops, "0 -20 Td")
				}
				ops = append(ops, "("+line+") Tj")
			}
			ops = append(ops, "ET")
			content = strings.Join(ops, " ")
		}
		writeObj(fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>\nendobj\n",
			pageID, contentID))
		writeObj(fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n",
			contentID, len(content), content))
	}

	xrefOff := buf.Len()
	buf.WriteString(fmt.Sprintf("xref\n0 %d\n", len(offsets)))
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets[1:] {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}
	buf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF",
		len(offsets), xrefOff))
	return buf.Bytes()
}

func TestPDFAdapter_BornDigitalTextWithPageLocators(t *testing.T) {
	pdf := buildPDF([][]string{
		{"Art. 1 Hauteur maximale 9 m", "Zone village"},
		{"Art. 2 Indice 0.4"},
	})
	res, err := PDFAdapter{}.Parse(pdf, "fixture.pdf")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.Format != "pdf" || res.NodeCount != 3 {
		t.Fatalf("format=%q nodes=%d, want pdf/3", res.Format, res.NodeCount)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("clean born-digital fixture produced errors: %v", res.Errors)
	}
	// Locators carry page + line + Y position + a content hash.
	if !strings.HasPrefix(res.Spans[0].ID, "pdf:p1:l1:y720.0:h") {
		t.Fatalf("first locator = %q", res.Spans[0].ID)
	}
	if !strings.HasPrefix(res.Spans[2].ID, "pdf:p2:l3:y720.0:h") {
		t.Fatalf("third locator = %q (page 2 lost?)", res.Spans[2].ID)
	}
	for _, s := range res.Spans {
		if s.NodeType != "text_line" {
			t.Fatalf("unexpected node type %q", s.NodeType)
		}
	}
	v := PDFAdapter{}.Validate(pdf, "fixture.pdf")
	if !v.Valid || len(v.Findings) != 0 {
		t.Fatalf("validate: %+v", v)
	}
}

func TestPDFAdapter_ScannedPageBecomesUnsupportedRecord(t *testing.T) {
	pdf := buildPDF([][]string{
		{"Art. 1 Hauteur maximale 9 m"},
		{}, // image-only page — out of the born-digital-text claim
	})
	res, err := PDFAdapter{}.Parse(pdf, "fixture.pdf")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var unsupported []SpanInfo
	for _, s := range res.Spans {
		if s.NodeType == "unsupported" {
			unsupported = append(unsupported, s)
		}
	}
	if len(unsupported) != 1 || unsupported[0].ID != "pdf:p2:unsupported" {
		t.Fatalf("unsupported records = %+v, want exactly pdf:p2:unsupported", unsupported)
	}
	if len(res.Errors) != 1 || !strings.Contains(res.Errors[0], "page 2") || !strings.Contains(res.Errors[0], "out of claim") {
		t.Fatalf("errors = %v — the unsupported page must be RECORDED, never dropped", res.Errors)
	}
	v := PDFAdapter{}.Validate(pdf, "fixture.pdf")
	if !v.Valid {
		t.Fatalf("a structurally valid PDF must validate; findings carry the claim boundary: %+v", v)
	}
	joined := strings.Join(v.Findings, " | ")
	if !strings.Contains(joined, "page 2") {
		t.Fatalf("validate findings missing the out-of-claim page: %v", v.Findings)
	}
}

func TestPDFAdapter_FullyScannedStaysOutOfClaim(t *testing.T) {
	pdf := buildPDF([][]string{{}})
	res, err := PDFAdapter{}.Parse(pdf, "scan.pdf")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if res.NodeCount != 1 || res.Spans[0].NodeType != "unsupported" {
		t.Fatalf("fully scanned doc must yield only unsupported records: %+v", res.Spans)
	}
	v := PDFAdapter{}.Validate(pdf, "scan.pdf")
	if len(v.Findings) == 0 || !strings.Contains(strings.Join(v.Findings, " "), "OCR") {
		t.Fatalf("the OCR-out-of-claim ladder rung must be stated: %+v", v)
	}
}

// The C1 drift bar: mutating ONE byte of carried text changes the parse output
// itself (content-addressed span IDs); corrupting the structure fails closed.
func TestPDFAdapter_ByteMutationDriftsTheParse(t *testing.T) {
	pdf := buildPDF([][]string{{"Art. 1 Hauteur maximale 9 m"}})
	before, err := PDFAdapter{}.Parse(pdf, "a.pdf")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	mutated := bytes.Replace(pdf, []byte("Hauteur"), []byte("Xauteur"), 1)
	if bytes.Equal(mutated, pdf) {
		t.Fatal("mutation did not apply")
	}
	after, err := PDFAdapter{}.Parse(mutated, "a.pdf")
	if err != nil {
		t.Fatalf("Parse mutated: %v", err)
	}
	if before.Spans[0].ID == after.Spans[0].ID {
		t.Fatalf("one mutated byte left the parse output identical: %s", after.Spans[0].ID)
	}

	corrupted := bytes.Replace(pdf, []byte("xref"), []byte("xrEf"), 1)
	if _, err := (PDFAdapter{}).Parse(corrupted, "a.pdf"); err == nil {
		t.Fatal("structurally corrupted PDF parsed instead of failing closed")
	}
}

// The « suppression de l'adapter → gate rouge » bar: the DEFAULT registry must
// hand out a REAL pdf adapter. Re-registering the placeholder would make this
// test fail with "not yet implemented".
func TestDefaultRegistry_PDFAdapterIsReal(t *testing.T) {
	a, ok := DefaultRegistry().ForFile("reglement.pdf")
	if !ok {
		t.Fatal("no adapter registered for .pdf")
	}
	res, err := a.Parse(buildPDF([][]string{{"Zone village"}}), "reglement.pdf")
	if err != nil {
		t.Fatalf("the registered pdf adapter is not real: %v", err)
	}
	if res.NodeCount != 1 || res.Spans[0].NodeType != "text_line" {
		t.Fatalf("unexpected parse from the registered adapter: %+v", res)
	}
}

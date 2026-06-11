package atomization

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// buildTestPDF mirrors the fidelity test builder (test fixtures don't cross
// package boundaries): a minimal uncompressed born-digital PDF with EXACT xref
// offsets, generated deterministically — license-safe, no binary committed.
func buildTestPDF(pages [][]string) []byte {
	var buf bytes.Buffer
	offsets := []int{0}
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
	writeObj(fmt.Sprintf("2 0 obj\n<< /Type /Pages /Kids [%s] /Count %d >>\nendobj\n", strings.Join(kids, " "), len(pages)))
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
		writeObj(fmt.Sprintf("%d 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>\nendobj\n", pageID, contentID))
		writeObj(fmt.Sprintf("%d 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", contentID, len(content), content))
	}
	xrefOff := buf.Len()
	buf.WriteString(fmt.Sprintf("xref\n0 %d\n", len(offsets)))
	buf.WriteString("0000000000 65535 f \n")
	for _, off := range offsets[1:] {
		buf.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}
	buf.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF", len(offsets), xrefOff))
	return buf.Bytes()
}

func pdfOpts(ref string) AtomizeOptions {
	return AtomizeOptions{DocumentRef: ref, SourceFile: ref + ".pdf", Domain: "built-environment"}
}

func TestAtomizePDF_PageLineLocators(t *testing.T) {
	pdf := buildTestPDF([][]string{
		{"Art. 1 Hauteur maximale 9 m", "Zone village"},
		{"Art. 2 Indice 0.4"},
	})
	set, unsupported, err := AtomizePDF(pdf, pdfOpts("rcc-fixture"))
	if err != nil {
		t.Fatalf("AtomizePDF: %v", err)
	}
	if len(unsupported) != 0 {
		t.Fatalf("clean fixture reported unsupported pages: %v", unsupported)
	}
	if set.AtomCount != 3 {
		t.Fatalf("atoms = %d, want 3", set.AtomCount)
	}
	first, third := set.Atoms[0], set.Atoms[2]
	if first.CanonicalRef != "rcc-fixture#/page[1]/line[1]" {
		t.Fatalf("first canonical_ref = %q", first.CanonicalRef)
	}
	if first.SourceSpan.DomPath != "/page[1]/line[1]" || first.Text != "Art. 1 Hauteur maximale 9 m" {
		t.Fatalf("first atom locator/text wrong: %+v", first)
	}
	if third.CanonicalRef != "rcc-fixture#/page[2]/line[1]" {
		t.Fatalf("page restart lost: %q", third.CanonicalRef)
	}
	// Determinism: stable IDs and hashes across runs.
	again, _, _ := AtomizePDF(pdf, pdfOpts("rcc-fixture"))
	for i := range set.Atoms {
		if set.Atoms[i].ID != again.Atoms[i].ID || set.Atoms[i].ContentHash != again.Atoms[i].ContentHash {
			t.Fatalf("non-deterministic atom %d", i)
		}
	}
}

func TestAtomizePDF_UnsupportedPagesAreReturnedNeverDropped(t *testing.T) {
	pdf := buildTestPDF([][]string{
		{"Art. 1 Hauteur maximale 9 m"},
		{}, // scanned/image-only — out of claim
	})
	set, unsupported, err := AtomizePDF(pdf, pdfOpts("rcc-mixte"))
	if err != nil {
		t.Fatalf("AtomizePDF: %v", err)
	}
	if len(unsupported) != 1 || unsupported[0] != 2 {
		t.Fatalf("unsupported = %v, want [2]", unsupported)
	}
	if set.AtomCount != 1 {
		t.Fatalf("atoms = %d, want 1 (page 1 only)", set.AtomCount)
	}
}

func TestAtomizePDF_FacetsOptIn(t *testing.T) {
	pdf := buildTestPDF([][]string{{"Zone village"}})
	o := pdfOpts("rcc")
	o.EmitFacets = true
	set, _, err := AtomizePDF(pdf, o)
	if err != nil {
		t.Fatalf("AtomizePDF: %v", err)
	}
	if set.Atoms[0].Facets == nil {
		t.Fatal("facets requested but absent")
	}
	off, _, _ := AtomizePDF(pdf, pdfOpts("rcc"))
	if off.Atoms[0].Facets != nil {
		t.Fatal("facets emitted without opt-in")
	}
}

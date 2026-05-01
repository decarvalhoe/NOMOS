package corpus

import (
	"os"
	"path/filepath"
	"testing"
)

// --- Classify tests ---

func TestClassifyTextByExtension(t *testing.T) {
	f := writeTempFile(t, "hello.go", []byte("package main\n"))
	class, err := Classify(f)
	assertNoErr(t, err)
	assertEqual(t, ClassText, class)
}

func TestClassifyPDFByExtension(t *testing.T) {
	f := writeTempFile(t, "doc.pdf", []byte("%PDF-1.4 fake pdf"))
	class, err := Classify(f)
	assertNoErr(t, err)
	assertEqual(t, ClassPDF, class)
}

func TestClassifyDocxByExtension(t *testing.T) {
	f := writeTempFile(t, "report.docx", []byte("PK\x03\x04 fake docx"))
	class, err := Classify(f)
	assertNoErr(t, err)
	assertEqual(t, ClassOffice, class)
}

func TestClassifyXlsxByExtension(t *testing.T) {
	f := writeTempFile(t, "data.xlsx", []byte("PK\x03\x04 fake xlsx"))
	class, err := Classify(f)
	assertNoErr(t, err)
	assertEqual(t, ClassOffice, class)
}

func TestClassifyPptxByExtension(t *testing.T) {
	f := writeTempFile(t, "slides.pptx", []byte("PK\x03\x04 fake pptx"))
	class, err := Classify(f)
	assertNoErr(t, err)
	assertEqual(t, ClassOffice, class)
}

func TestClassifyPNGByExtension(t *testing.T) {
	f := writeTempFile(t, "icon.png", []byte("\x89PNG\r\n\x1a\n fake"))
	class, err := Classify(f)
	assertNoErr(t, err)
	assertEqual(t, ClassImage, class)
}

func TestClassifyBinaryByContent(t *testing.T) {
	// Unknown extension, binary content with null bytes.
	content := make([]byte, 100)
	content[0] = 0x00
	content[50] = 0x00
	f := writeTempFile(t, "data.bin", content)
	class, err := Classify(f)
	assertNoErr(t, err)
	assertEqual(t, ClassBinary, class)
}

func TestClassifyTextByContentSniff(t *testing.T) {
	// Unknown extension but text content.
	f := writeTempFile(t, "readme", []byte("This is a plain text file.\n"))
	class, err := Classify(f)
	assertNoErr(t, err)
	assertEqual(t, ClassText, class)
}

func TestClassifyPDFByMagicBytes(t *testing.T) {
	// No .pdf extension but PDF magic.
	f := writeTempFile(t, "mystery", []byte("%PDF-1.7 content"))
	class, err := Classify(f)
	assertNoErr(t, err)
	assertEqual(t, ClassPDF, class)
}

func TestClassifyZIPAsMaybeOffice(t *testing.T) {
	// No office extension but ZIP magic (PK header).
	f := writeTempFile(t, "archive", []byte("PK\x03\x04 some content"))
	class, err := Classify(f)
	assertNoErr(t, err)
	assertEqual(t, ClassOffice, class)
}

func TestClassifyJPEGByMagic(t *testing.T) {
	f := writeTempFile(t, "photo", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10})
	class, err := Classify(f)
	assertNoErr(t, err)
	assertEqual(t, ClassImage, class)
}

func TestClassifyEmptyFile(t *testing.T) {
	f := writeTempFile(t, "empty", []byte{})
	class, err := Classify(f)
	assertNoErr(t, err)
	assertEqual(t, ClassText, class)
}

func TestClassifyNonexistent(t *testing.T) {
	_, err := Classify("/nonexistent/file.xyz")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// --- Policy.Apply tests ---

func TestDefaultPolicyActions(t *testing.T) {
	p := DefaultPolicy()

	cases := []struct {
		class  FileClass
		expect Action
	}{
		{ClassText, ActionAllow},
		{ClassPDF, ActionExtractMetadata},
		{ClassOffice, ActionExtractMetadata},
		{ClassImage, ActionSkip},
		{ClassBinary, ActionBlock},
	}
	for _, tc := range cases {
		action, _ := p.Apply(tc.class)
		if action != tc.expect {
			t.Fatalf("class %q: expected %q, got %q", tc.class, tc.expect, action)
		}
	}
}

func TestCustomPolicy(t *testing.T) {
	p := Policy{
		PDF:    ActionBlock,
		Office: ActionBlock,
		Image:  ActionBlock,
		Binary: ActionBlock,
	}
	for _, class := range []FileClass{ClassPDF, ClassOffice, ClassImage, ClassBinary} {
		action, _ := p.Apply(class)
		if action != ActionBlock {
			t.Fatalf("class %q: expected block, got %q", class, action)
		}
	}
}

// --- EvaluateFile tests ---

func TestEvaluateFilePDF(t *testing.T) {
	f := writeTempFile(t, "spec.pdf", []byte("%PDF-1.5"))
	p := DefaultPolicy()
	result, err := p.EvaluateFile(f)
	assertNoErr(t, err)
	assertEqual(t, ClassPDF, result.Class)
	assertEqual(t, ActionExtractMetadata, result.Action)
}

func TestEvaluateFileBlocksBinary(t *testing.T) {
	content := make([]byte, 64)
	content[0] = 0x00
	f := writeTempFile(t, "blob.dat", content)
	p := DefaultPolicy()
	result, err := p.EvaluateFile(f)
	assertNoErr(t, err)
	assertEqual(t, ClassBinary, result.Class)
	assertEqual(t, ActionBlock, result.Action)
}

// --- EvaluateDir tests ---

func TestEvaluateDir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "readme.md", []byte("# docs\n"))
	writeFile(t, dir, "contract.pdf", []byte("%PDF-1.4"))
	writeFile(t, dir, "report.docx", []byte("PK\x03\x04 docx"))
	writeFile(t, dir, "logo.png", []byte("\x89PNG\r\n\x1a\n"))

	binContent := make([]byte, 64)
	binContent[0] = 0x00
	writeFile(t, dir, "firmware.bin", binContent)

	p := DefaultPolicy()
	results, err := p.EvaluateDir(dir)
	assertNoErr(t, err)

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	m := map[string]PolicyResult{}
	for _, r := range results {
		m[r.Path] = r
	}

	assertEqual(t, ActionAllow, m["readme.md"].Action)
	assertEqual(t, ActionExtractMetadata, m["contract.pdf"].Action)
	assertEqual(t, ActionExtractMetadata, m["report.docx"].Action)
	assertEqual(t, ActionSkip, m["logo.png"].Action)
	assertEqual(t, ActionBlock, m["firmware.bin"].Action)
}

func TestEvaluateDirSubdirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "sub/nested.go", []byte("package main\n"))
	writeFile(t, dir, "sub/data.xlsx", []byte("PK\x03\x04 xlsx"))

	p := DefaultPolicy()
	results, err := p.EvaluateDir(dir)
	assertNoErr(t, err)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

// --- helpers ---

func writeTempFile(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFile(t *testing.T, root, rel string, content []byte) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertEqual[T comparable](t *testing.T, expected, actual T) {
	t.Helper()
	if expected != actual {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
}

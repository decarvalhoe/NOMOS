package fidelity

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, root, rel string, content []byte) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanFormatsAllSupported(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "doc.md", []byte("# Title\n"))
	writeTestFile(t, dir, "data.yaml", []byte("key: val\n"))
	writeTestFile(t, dir, "config.json", []byte(`{"a":1}`))

	r := DefaultRegistry()
	result, err := ScanFormats(dir, r)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if result.ScannedFiles != 3 {
		t.Fatalf("scanned: %d", result.ScannedFiles)
	}
	if result.SupportedFiles != 3 {
		t.Fatalf("supported: %d", result.SupportedFiles)
	}
	if !result.Pass {
		t.Fatalf("expected pass, findings: %v", result.Findings)
	}
}

func TestScanFormatsProprietary(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "report.doc", []byte("fake doc content"))

	r := DefaultRegistry()
	result, _ := ScanFormats(dir, r)
	if result.Pass {
		t.Fatal("proprietary should block")
	}
	assertHasFindingCode(t, result.Findings, CodeProprietaryFormat)
}

func TestScanFormatsImage(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "logo.png", []byte{0x89, 0x50, 0x4E, 0x47})

	r := DefaultRegistry()
	result, _ := ScanFormats(dir, r)
	assertHasFindingCode(t, result.Findings, CodeImageNoAlt)
	// Images are non-blocking warnings.
	if !result.Pass {
		t.Fatal("image warning should not block")
	}
}

func TestScanFormatsBinary(t *testing.T) {
	dir := t.TempDir()
	bin := make([]byte, 64)
	bin[0] = 0x00
	bin[10] = 0x00
	writeTestFile(t, dir, "firmware.elf", bin)

	r := DefaultRegistry()
	result, _ := ScanFormats(dir, r)
	if result.Pass {
		t.Fatal("binary should block")
	}
	assertHasFindingCode(t, result.Findings, CodeBinaryDetected)
}

func TestScanFormatsUnsupportedText(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "notes.rst", []byte("Title\n=====\n"))

	r := DefaultRegistry()
	result, _ := ScanFormats(dir, r)
	assertHasFindingCode(t, result.Findings, CodeUnsupportedFormat)
	// Unknown text is warning, not blocking.
	if !result.Pass {
		t.Fatal("unsupported text should not block")
	}
}

func TestScanFormatsEmptyFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "empty.md", []byte{})

	r := DefaultRegistry()
	result, _ := ScanFormats(dir, r)
	assertHasFindingCode(t, result.Findings, CodeEmptyFile)
}

func TestScanFormatsSkipsGitDir(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, ".git/config", []byte("gitconfig"))
	writeTestFile(t, dir, "doc.md", []byte("# OK\n"))

	r := DefaultRegistry()
	result, _ := ScanFormats(dir, r)
	if result.ScannedFiles != 1 {
		t.Fatalf("should skip .git, scanned: %d", result.ScannedFiles)
	}
}

func TestScanFormatsSkipsNodeModules(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "node_modules/pkg/index.js", []byte("module.exports = {}"))
	writeTestFile(t, dir, "readme.md", []byte("# Hi\n"))

	r := DefaultRegistry()
	result, _ := ScanFormats(dir, r)
	if result.ScannedFiles != 1 {
		t.Fatalf("should skip node_modules, scanned: %d", result.ScannedFiles)
	}
}

func TestScanFormatsMixed(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "doc.md", []byte("# Title\n"))
	writeTestFile(t, dir, "data.yaml", []byte("key: val\n"))
	writeTestFile(t, dir, "legacy.xls", []byte("fake excel"))
	writeTestFile(t, dir, "photo.jpg", []byte{0xFF, 0xD8})
	bin := make([]byte, 32)
	bin[0] = 0x00
	writeTestFile(t, dir, "blob.dat", bin)
	writeTestFile(t, dir, "notes.txt", []byte("just text"))

	r := DefaultRegistry()
	result, _ := ScanFormats(dir, r)
	if result.Pass {
		t.Fatal("should fail: proprietary + binary present")
	}
	if result.ScannedFiles != 6 {
		t.Fatalf("scanned: %d", result.ScannedFiles)
	}
	if result.SupportedFiles != 2 {
		t.Fatalf("supported: %d", result.SupportedFiles)
	}
}

func TestScanFormatsEmpty(t *testing.T) {
	dir := t.TempDir()
	r := DefaultRegistry()
	result, _ := ScanFormats(dir, r)
	if result.ScannedFiles != 0 {
		t.Fatalf("scanned: %d", result.ScannedFiles)
	}
	if !result.Pass {
		t.Fatal("empty dir should pass")
	}
}

func TestScanFormatsFindingsSorted(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "z.doc", []byte("doc"))
	writeTestFile(t, dir, "a.xls", []byte("xls"))

	r := DefaultRegistry()
	result, _ := ScanFormats(dir, r)
	if len(result.Findings) < 2 {
		t.Fatalf("expected >=2 findings, got %d", len(result.Findings))
	}
	for i := 1; i < len(result.Findings); i++ {
		if result.Findings[i].Path < result.Findings[i-1].Path {
			t.Fatalf("findings not sorted: %q before %q",
				result.Findings[i-1].Path, result.Findings[i].Path)
		}
	}
}

// --- CheckFile ---

func TestCheckFileSupported(t *testing.T) {
	r := DefaultRegistry()
	f := CheckFile("doc.md", r)
	if f.Code != "" {
		t.Fatalf("supported file should have no code, got %q", f.Code)
	}
}

func TestCheckFileProprietary(t *testing.T) {
	r := DefaultRegistry()
	f := CheckFile("report.doc", r)
	if f.Code != CodeProprietaryFormat {
		t.Fatalf("expected PROPRIETARY_FORMAT, got %q", f.Code)
	}
	if !f.Blocking {
		t.Fatal("proprietary should be blocking")
	}
}

func TestCheckFileImage(t *testing.T) {
	r := DefaultRegistry()
	f := CheckFile("photo.png", r)
	if f.Code != CodeImageNoAlt {
		t.Fatalf("expected IMAGE_NO_ALT, got %q", f.Code)
	}
}

func TestCheckFileUnknown(t *testing.T) {
	r := DefaultRegistry()
	f := CheckFile("data.csv", r)
	if f.Code != CodeUnsupportedFormat {
		t.Fatalf("expected UNSUPPORTED_FORMAT, got %q", f.Code)
	}
}

func TestCheckFileAllProprietary(t *testing.T) {
	r := DefaultRegistry()
	for ext, desc := range proprietaryExts {
		f := CheckFile("file"+ext, r)
		if f.Code != CodeProprietaryFormat {
			t.Fatalf("%s (%s): expected PROPRIETARY_FORMAT, got %q", ext, desc, f.Code)
		}
	}
}

func TestCheckFileAllImages(t *testing.T) {
	r := DefaultRegistry()
	for ext := range imageExts {
		f := CheckFile("img"+ext, r)
		if f.Code != CodeImageNoAlt {
			t.Fatalf("%s: expected IMAGE_NO_ALT, got %q", ext, f.Code)
		}
	}
}

// --- helpers ---

func assertHasFindingCode(t *testing.T, findings []FormatFinding, code string) {
	t.Helper()
	for _, f := range findings {
		if f.Code == code {
			return
		}
	}
	t.Fatalf("expected finding code %q in %v", code, findings)
}

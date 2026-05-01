package corpus

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixtureRoot() string {
	return filepath.Join("testdata", "sample-corpus")
}

func TestScanFindsAllFiles(t *testing.T) {
	snap, err := Scan(fixtureRoot(), ScanOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.TotalFiles != 4 {
		t.Fatalf("expected 4 files, got %d", snap.TotalFiles)
	}
	if snap.Format != "nomos.corpus-snapshot.v1" {
		t.Fatalf("expected format nomos.corpus-snapshot.v1, got %q", snap.Format)
	}
	if snap.TotalBytes == 0 {
		t.Fatal("expected non-zero total bytes")
	}
}

func TestScanHashesAreStable(t *testing.T) {
	snap1, err := Scan(fixtureRoot(), ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	snap2, err := Scan(fixtureRoot(), ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap1.Sources) != len(snap2.Sources) {
		t.Fatal("source count mismatch")
	}
	for i := range snap1.Sources {
		if snap1.Sources[i].Hash != snap2.Sources[i].Hash {
			t.Fatalf("hash mismatch for %s", snap1.Sources[i].Path)
		}
	}
}

func TestScanHashFormat(t *testing.T) {
	snap, err := Scan(fixtureRoot(), ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range snap.Sources {
		if !strings.HasPrefix(src.Hash, "sha256:") {
			t.Fatalf("expected sha256: prefix, got %q", src.Hash)
		}
		if len(src.Hash) != 7+64 {
			t.Fatalf("expected 71 char hash, got %d for %s", len(src.Hash), src.Path)
		}
	}
}

func TestScanSortedByPath(t *testing.T) {
	snap, err := Scan(fixtureRoot(), ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(snap.Sources); i++ {
		if snap.Sources[i].Path < snap.Sources[i-1].Path {
			t.Fatalf("sources not sorted: %q before %q", snap.Sources[i-1].Path, snap.Sources[i].Path)
		}
	}
}

func TestScanFilterByExtension(t *testing.T) {
	snap, err := Scan(fixtureRoot(), ScanOptions{Extensions: []string{".pdf"}})
	if err != nil {
		t.Fatal(err)
	}
	if snap.TotalFiles != 2 {
		t.Fatalf("expected 2 pdf files, got %d", snap.TotalFiles)
	}
	for _, src := range snap.Sources {
		if src.Extension != ".pdf" {
			t.Fatalf("expected .pdf extension, got %q", src.Extension)
		}
	}
}

func TestScanFilterMultipleExtensions(t *testing.T) {
	snap, err := Scan(fixtureRoot(), ScanOptions{Extensions: []string{".pdf", ".csv"}})
	if err != nil {
		t.Fatal(err)
	}
	if snap.TotalFiles != 3 {
		t.Fatalf("expected 3 files (.pdf + .csv), got %d", snap.TotalFiles)
	}
}

func TestScanExtensionWithoutDot(t *testing.T) {
	snap, err := Scan(fixtureRoot(), ScanOptions{Extensions: []string{"csv"}})
	if err != nil {
		t.Fatal(err)
	}
	if snap.TotalFiles != 1 {
		t.Fatalf("expected 1 csv file, got %d", snap.TotalFiles)
	}
}

func TestScanNonexistentRoot(t *testing.T) {
	_, err := Scan("/nonexistent-corpus", ScanOptions{})
	if err == nil {
		t.Fatal("expected error for nonexistent root")
	}
}

func TestScanFileNotDir(t *testing.T) {
	_, err := Scan(filepath.Join(fixtureRoot(), "README.md"), ScanOptions{})
	if err == nil {
		t.Fatal("expected error for file (not dir)")
	}
}

func TestWriteJSON(t *testing.T) {
	snap, err := Scan(fixtureRoot(), ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, snap); err != nil {
		t.Fatalf("write json: %v", err)
	}
	var decoded Snapshot
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	if decoded.TotalFiles != snap.TotalFiles {
		t.Fatalf("expected %d files, got %d", snap.TotalFiles, decoded.TotalFiles)
	}
}

func TestGuardOutputInsideCorpus(t *testing.T) {
	err := GuardOutput(filepath.Join(fixtureRoot(), "output.json"), fixtureRoot())
	if err == nil {
		t.Fatal("expected error for output inside corpus")
	}
	if !strings.Contains(err.Error(), "inside corpus root") {
		t.Fatalf("expected inside corpus root error, got %q", err.Error())
	}
}

func TestGuardOutputOutsideCorpus(t *testing.T) {
	err := GuardOutput("/tmp/output.json", fixtureRoot())
	if err != nil {
		t.Fatalf("expected no error for output outside corpus, got %v", err)
	}
}

func TestGuardOutputToTempDir(t *testing.T) {
	dir := t.TempDir()
	err := GuardOutput(filepath.Join(dir, "snapshot.json"), fixtureRoot())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestScanWriteToFile(t *testing.T) {
	snap, err := Scan(fixtureRoot(), ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	outPath := filepath.Join(dir, "snapshot.json")

	if err := GuardOutput(outPath, fixtureRoot()); err != nil {
		t.Fatalf("guard: %v", err)
	}

	f, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := WriteJSON(f, snap); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "nomos.corpus-snapshot.v1") {
		t.Fatal("expected snapshot format in output file")
	}
}

func TestGuardGitCleanNonGitRepo(t *testing.T) {
	// Non-git directory should pass without error
	err := GuardGitClean(fixtureRoot())
	if err != nil {
		t.Fatalf("expected no error for non-git repo, got %v", err)
	}
}

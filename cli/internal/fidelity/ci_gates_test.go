package fidelity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckReadOnlyPass(t *testing.T) {
	before := CorpusSnapshot{Files: map[string]string{
		"a.txt": "sha256:aaa",
		"b.txt": "sha256:bbb",
	}}
	after := CorpusSnapshot{Files: map[string]string{
		"a.txt": "sha256:aaa",
		"b.txt": "sha256:bbb",
	}}

	result := CheckReadOnly(before, after)
	if !result.Pass {
		t.Fatalf("expected pass, got: %s", result.Message)
	}
}

func TestCheckReadOnlyDetectsModified(t *testing.T) {
	before := CorpusSnapshot{Files: map[string]string{"a.txt": "sha256:aaa"}}
	after := CorpusSnapshot{Files: map[string]string{"a.txt": "sha256:changed"}}

	result := CheckReadOnly(before, after)
	if result.Pass {
		t.Fatal("expected fail for modified file")
	}
	if result.Gate != "read_only_corpus" {
		t.Fatalf("expected gate read_only_corpus, got %s", result.Gate)
	}
}

func TestCheckReadOnlyDetectsCreated(t *testing.T) {
	before := CorpusSnapshot{Files: map[string]string{"a.txt": "sha256:aaa"}}
	after := CorpusSnapshot{Files: map[string]string{"a.txt": "sha256:aaa", "new.txt": "sha256:new"}}

	result := CheckReadOnly(before, after)
	if result.Pass {
		t.Fatal("expected fail for created file")
	}
}

func TestCheckReadOnlyDetectsDeleted(t *testing.T) {
	before := CorpusSnapshot{Files: map[string]string{"a.txt": "sha256:aaa", "b.txt": "sha256:bbb"}}
	after := CorpusSnapshot{Files: map[string]string{"a.txt": "sha256:aaa"}}

	result := CheckReadOnly(before, after)
	if result.Pass {
		t.Fatal("expected fail for deleted file")
	}
}

func TestCheckArtifactPresencePass(t *testing.T) {
	root := t.TempDir()
	writeTestFileCI(t, root, "report.json", "{}")
	writeTestFileCI(t, root, "coverage.md", "# Coverage")

	specs := []ArtifactSpec{
		{Path: "report.json", Required: true},
		{Path: "coverage.md", Required: true},
	}
	result := CheckArtifactPresence(root, specs)
	if !result.Pass {
		t.Fatalf("expected pass: %s", result.Message)
	}
}

func TestCheckArtifactPresenceMissing(t *testing.T) {
	root := t.TempDir()
	writeTestFileCI(t, root, "report.json", "{}")

	specs := []ArtifactSpec{
		{Path: "report.json", Required: true},
		{Path: "missing.md", Required: true, Description: "coverage report"},
	}
	result := CheckArtifactPresence(root, specs)
	if result.Pass {
		t.Fatal("expected fail for missing artifact")
	}
	if result.Gate != "artifact_presence" {
		t.Fatalf("expected gate artifact_presence, got %s", result.Gate)
	}
}

func TestCheckArtifactPresenceOptionalSkipped(t *testing.T) {
	root := t.TempDir()
	specs := []ArtifactSpec{
		{Path: "optional.txt", Required: false},
	}
	result := CheckArtifactPresence(root, specs)
	if !result.Pass {
		t.Fatal("optional artifacts should not cause failure")
	}
}

func TestCheckArtifactIntegrityPass(t *testing.T) {
	root := t.TempDir()
	content := "verified content"
	writeTestFileCI(t, root, "data.txt", content)
	hash := computeFileSHA256(t, filepath.Join(root, "data.txt"))

	specs := []ArtifactSpec{
		{Path: "data.txt", ExpectedHash: hash},
	}
	result := CheckArtifactIntegrity(root, specs)
	if !result.Pass {
		t.Fatalf("expected pass: %s", result.Message)
	}
}

func TestCheckArtifactIntegrityFail(t *testing.T) {
	root := t.TempDir()
	writeTestFileCI(t, root, "data.txt", "actual content")

	specs := []ArtifactSpec{
		{Path: "data.txt", ExpectedHash: "sha256:wrong"},
	}
	result := CheckArtifactIntegrity(root, specs)
	if result.Pass {
		t.Fatal("expected fail for hash mismatch")
	}
	if result.Gate != "artifact_integrity" {
		t.Fatalf("expected gate artifact_integrity, got %s", result.Gate)
	}
}

func TestCheckArtifactIntegritySkipsNoHash(t *testing.T) {
	root := t.TempDir()
	specs := []ArtifactSpec{
		{Path: "nocheck.txt", ExpectedHash: ""},
	}
	result := CheckArtifactIntegrity(root, specs)
	if !result.Pass {
		t.Fatal("specs without hash should pass")
	}
}

func TestRunCIGatesAllPass(t *testing.T) {
	root := t.TempDir()
	writeTestFileCI(t, root, "report.json", "{}")
	writeTestFileCI(t, root, "data.txt", "content")
	hash := computeFileSHA256(t, filepath.Join(root, "data.txt"))

	before, _ := TakeCorpusSnapshot(root)
	specs := []ArtifactSpec{
		{Path: "report.json", Required: true},
		{Path: "data.txt", Required: true, ExpectedHash: hash},
	}

	report, err := RunCIGates(root, &before, specs)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !report.Pass {
		t.Fatalf("expected pass, failed %d gates", report.Failed)
	}
	if len(report.Gates) != 3 {
		t.Fatalf("expected 3 gates, got %d", len(report.Gates))
	}
}

func TestRunCIGatesCorpusModified(t *testing.T) {
	root := t.TempDir()
	writeTestFileCI(t, root, "corpus.md", "original")

	before, _ := TakeCorpusSnapshot(root)
	// Modify after snapshot.
	writeTestFileCI(t, root, "corpus.md", "tampered")

	report, err := RunCIGates(root, &before, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if report.Pass {
		t.Fatal("expected fail for modified corpus")
	}
}

func TestRunCIGatesWithoutSnapshot(t *testing.T) {
	root := t.TempDir()
	writeTestFileCI(t, root, "report.json", "{}")

	specs := []ArtifactSpec{{Path: "report.json", Required: true}}
	report, err := RunCIGates(root, nil, specs)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	// Should skip read-only check (no before snapshot).
	if len(report.Gates) != 2 {
		t.Fatalf("expected 2 gates (no read-only), got %d", len(report.Gates))
	}
}

func TestTakeCorpusSnapshot(t *testing.T) {
	root := t.TempDir()
	writeTestFileCI(t, root, "a.txt", "hello")
	writeTestFileCI(t, root, "sub/b.txt", "world")

	snap, err := TakeCorpusSnapshot(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(snap.Files))
	}
	if snap.Files["a.txt"] == "" || snap.Files["sub/b.txt"] == "" {
		t.Fatalf("missing expected file hashes: %v", snap.Files)
	}
}

func writeTestFileCI(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func computeFileSHA256(t *testing.T, path string) string {
	t.Helper()
	hash, err := hashFileSHA256(path)
	if err != nil {
		t.Fatalf("hash %s: %v", path, err)
	}
	return hash
}

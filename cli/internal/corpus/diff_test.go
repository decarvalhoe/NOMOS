package corpus

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiffIdenticalSnapshots(t *testing.T) {
	entries := []SourceEntry{
		{Path: "a.go", Hash: "sha256:aaa"},
		{Path: "b.go", Hash: "sha256:bbb"},
	}
	old := Snapshot{Sources: entries}
	new := Snapshot{Sources: entries}

	report := Diff(old, new)

	if report.TotalChanges() != 0 {
		t.Fatalf("expected 0 changes, got %d", report.TotalChanges())
	}
	if len(report.Unchanged) != 2 {
		t.Fatalf("expected 2 unchanged, got %d", len(report.Unchanged))
	}
}

func TestDiffAddedFiles(t *testing.T) {
	old := Snapshot{Sources: []SourceEntry{
		{Path: "existing.go", Hash: "sha256:111"},
	}}
	new := Snapshot{Sources: []SourceEntry{
		{Path: "existing.go", Hash: "sha256:111"},
		{Path: "new-file.go", Hash: "sha256:222"},
	}}

	report := Diff(old, new)

	if len(report.Added) != 1 {
		t.Fatalf("expected 1 added, got %d", len(report.Added))
	}
	if report.Added[0].Path != "new-file.go" {
		t.Fatalf("unexpected added path: %s", report.Added[0].Path)
	}
	if report.Added[0].NewHash != "sha256:222" {
		t.Fatalf("unexpected new hash: %s", report.Added[0].NewHash)
	}
	if report.Added[0].OldHash != "" {
		t.Fatalf("added file should have empty old hash")
	}
}

func TestDiffChangedFiles(t *testing.T) {
	old := Snapshot{Sources: []SourceEntry{
		{Path: "main.go", Hash: "sha256:old"},
	}}
	new := Snapshot{Sources: []SourceEntry{
		{Path: "main.go", Hash: "sha256:new"},
	}}

	report := Diff(old, new)

	if len(report.Changed) != 1 {
		t.Fatalf("expected 1 changed, got %d", len(report.Changed))
	}
	if report.Changed[0].OldHash != "sha256:old" || report.Changed[0].NewHash != "sha256:new" {
		t.Fatalf("unexpected hashes: %+v", report.Changed[0])
	}
}

func TestDiffRemovedFiles(t *testing.T) {
	old := Snapshot{Sources: []SourceEntry{
		{Path: "deleted.go", Hash: "sha256:del"},
		{Path: "kept.go", Hash: "sha256:keep"},
	}}
	new := Snapshot{Sources: []SourceEntry{
		{Path: "kept.go", Hash: "sha256:keep"},
	}}

	report := Diff(old, new)

	if len(report.Removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(report.Removed))
	}
	if report.Removed[0].Path != "deleted.go" {
		t.Fatalf("unexpected removed path: %s", report.Removed[0].Path)
	}
	if report.Removed[0].OldHash != "sha256:del" {
		t.Fatalf("unexpected old hash: %s", report.Removed[0].OldHash)
	}
}

func TestDiffArchivedFiles(t *testing.T) {
	old := Snapshot{Sources: []SourceEntry{
		{Path: "archive/old-policy.yaml", Hash: "sha256:arc"},
		{Path: "docs/archived/legacy.md", Hash: "sha256:leg"},
		{Path: "src/normal.go", Hash: "sha256:norm"},
	}}
	new := Snapshot{Sources: []SourceEntry{
		{Path: "src/normal.go", Hash: "sha256:norm"},
	}}

	report := Diff(old, new)

	if len(report.Archived) != 2 {
		t.Fatalf("expected 2 archived, got %d: %+v", len(report.Archived), report.Archived)
	}
	if len(report.Removed) != 0 {
		t.Fatalf("expected 0 removed, got %d", len(report.Removed))
	}
}

func TestDiffMixedChanges(t *testing.T) {
	old := Snapshot{Sources: []SourceEntry{
		{Path: "a.go", Hash: "sha256:a1"},
		{Path: "b.go", Hash: "sha256:b1"},
		{Path: "c.go", Hash: "sha256:c1"},
		{Path: "archive/d.go", Hash: "sha256:d1"},
	}}
	new := Snapshot{Sources: []SourceEntry{
		{Path: "a.go", Hash: "sha256:a1"},
		{Path: "b.go", Hash: "sha256:b2"},
		{Path: "e.go", Hash: "sha256:e1"},
	}}

	report := Diff(old, new)

	if len(report.Unchanged) != 1 || report.Unchanged[0].Path != "a.go" {
		t.Fatalf("unexpected unchanged: %+v", report.Unchanged)
	}
	if len(report.Changed) != 1 || report.Changed[0].Path != "b.go" {
		t.Fatalf("unexpected changed: %+v", report.Changed)
	}
	if len(report.Removed) != 1 || report.Removed[0].Path != "c.go" {
		t.Fatalf("unexpected removed: %+v", report.Removed)
	}
	if len(report.Archived) != 1 || report.Archived[0].Path != "archive/d.go" {
		t.Fatalf("unexpected archived: %+v", report.Archived)
	}
	if len(report.Added) != 1 || report.Added[0].Path != "e.go" {
		t.Fatalf("unexpected added: %+v", report.Added)
	}
	if report.TotalChanges() != 4 {
		t.Fatalf("expected 4 total changes, got %d", report.TotalChanges())
	}
}

func TestDiffEmptySnapshots(t *testing.T) {
	report := Diff(Snapshot{}, Snapshot{})

	if report.TotalChanges() != 0 {
		t.Fatalf("expected 0 changes for empty snapshots")
	}
	if len(report.Unchanged) != 0 {
		t.Fatalf("expected 0 unchanged for empty snapshots")
	}
}

func TestDiffOldEmptyNewPopulated(t *testing.T) {
	new := Snapshot{Sources: []SourceEntry{
		{Path: "x.go", Hash: "sha256:x"},
		{Path: "y.go", Hash: "sha256:y"},
	}}

	report := Diff(Snapshot{}, new)

	if len(report.Added) != 2 {
		t.Fatalf("expected 2 added, got %d", len(report.Added))
	}
}

func TestSnapshotFromDir(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "src/main.go", "package main\n")
	writeTestFile(t, root, "src/util.go", "package main\n// util\n")
	writeTestFile(t, root, "README.md", "# Hello\n")
	// .git should be skipped
	writeTestFile(t, root, ".git/config", "[core]\n")

	snap, err := SnapshotFromDir(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	paths := make(map[string]bool)
	for _, e := range snap.Sources {
		paths[e.Path] = true
		if e.Hash == "" {
			t.Fatalf("empty hash for %s", e.Path)
		}
		if len(e.Hash) < 10 {
			t.Fatalf("hash too short for %s: %s", e.Path, e.Hash)
		}
	}

	if !paths["src/main.go"] || !paths["src/util.go"] || !paths["README.md"] {
		t.Fatalf("missing expected files: %v", paths)
	}
	if paths[".git/config"] {
		t.Fatal(".git should be skipped")
	}
}

func TestSnapshotFromDirHashConsistency(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "file.txt", "deterministic content\n")

	snap1, err := SnapshotFromDir(root)
	if err != nil {
		t.Fatalf("snapshot 1: %v", err)
	}
	snap2, err := SnapshotFromDir(root)
	if err != nil {
		t.Fatalf("snapshot 2: %v", err)
	}

	if snap1.Sources[0].Hash != snap2.Sources[0].Hash {
		t.Fatalf("hashes should be deterministic: %s vs %s", snap1.Sources[0].Hash, snap2.Sources[0].Hash)
	}
}

func TestSnapshotFromDirDetectsChange(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "data.txt", "version 1\n")

	snap1, _ := SnapshotFromDir(root)

	writeTestFile(t, root, "data.txt", "version 2\n")
	snap2, _ := SnapshotFromDir(root)

	report := Diff(snap1, snap2)
	if len(report.Changed) != 1 {
		t.Fatalf("expected 1 changed after content modification, got %d", len(report.Changed))
	}
}

func TestSnapshotFromDirNonexistent(t *testing.T) {
	_, err := SnapshotFromDir("/nonexistent-xyz-1801")
	if err == nil {
		t.Fatal("expected error for nonexistent dir")
	}
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

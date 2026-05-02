package guard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTakeHashSnapshotEmpty(t *testing.T) {
	root := t.TempDir()
	snap, err := TakeHashSnapshot(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(snap.Files))
	}
}

func TestTakeHashSnapshotWithFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "a.txt", "alpha\n")
	writeFile(t, root, "b.txt", "bravo\n")

	snap, err := TakeHashSnapshot(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if len(snap.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(snap.Files))
	}
	// Should be sorted.
	if snap.Files[0].Path != "a.txt" {
		t.Fatalf("expected a.txt first, got %q", snap.Files[0].Path)
	}
}

func TestTakeHashSnapshotDeterministic(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "file.txt", "content\n")

	s1, _ := TakeHashSnapshot(root)
	s2, _ := TakeHashSnapshot(root)

	if s1.Files[0].Hash != s2.Files[0].Hash {
		t.Fatal("hash should be deterministic")
	}
}

func TestTakeHashSnapshotSkipsGitDir(t *testing.T) {
	root := initTestRepo(t)
	writeFile(t, root, "data.txt", "content\n")
	git(t, root, "add", "data.txt")
	git(t, root, "commit", "-m", "add data")

	snap, err := TakeHashSnapshot(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	for _, f := range snap.Files {
		if f.Path == ".git" || strings.HasPrefix(f.Path, ".git/") {
			t.Fatalf("should skip .git directory entries, found %q", f.Path)
		}
	}
}

func TestGuardHashIntegrityPasses(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "file.txt", "original\n")

	before, _ := TakeHashSnapshot(root)
	err := GuardHashIntegrity(before)
	if err != nil {
		t.Fatalf("expected pass, got: %v", err)
	}
}

func TestGuardHashIntegrityDetectsModification(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "file.txt", "original\n")

	before, _ := TakeHashSnapshot(root)
	writeFile(t, root, "file.txt", "tampered\n")

	err := GuardHashIntegrity(before)
	if !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("expected ErrHashMismatch, got: %v", err)
	}
}

func TestGuardHashIntegrityDetectsAddition(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "file.txt", "original\n")

	before, _ := TakeHashSnapshot(root)
	writeFile(t, root, "extra.txt", "injected\n")

	err := GuardHashIntegrity(before)
	if !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("expected ErrHashMismatch, got: %v", err)
	}
}

func TestGuardHashIntegrityDetectsRemoval(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "file.txt", "original\n")

	before, _ := TakeHashSnapshot(root)
	os.Remove(filepath.Join(root, "file.txt"))

	err := GuardHashIntegrity(before)
	if !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("expected ErrHashMismatch, got: %v", err)
	}
}

func TestGuardHashIntegritySubdirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "sub/deep/file.txt", "nested\n")

	before, _ := TakeHashSnapshot(root)

	err := GuardHashIntegrity(before)
	if err != nil {
		t.Fatalf("expected pass, got: %v", err)
	}

	writeFile(t, root, "sub/deep/file.txt", "changed\n")
	err = GuardHashIntegrity(before)
	if !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("expected ErrHashMismatch for nested file, got: %v", err)
	}
}

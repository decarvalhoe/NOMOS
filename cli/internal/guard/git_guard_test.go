package guard

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTakeSnapshotCleanRepo(t *testing.T) {
	root := initTestRepo(t)

	snap, err := TakeSnapshot(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if !snap.IsClean {
		t.Fatalf("expected clean repo, got status: %v", snap.Status)
	}
}

func TestTakeSnapshotDirtyRepo(t *testing.T) {
	root := initTestRepo(t)
	writeFile(t, root, "dirty.txt", "uncommitted\n")

	snap, err := TakeSnapshot(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.IsClean {
		t.Fatal("expected dirty repo")
	}
	if _, ok := snap.Status["dirty.txt"]; !ok {
		t.Fatalf("expected dirty.txt in status, got: %v", snap.Status)
	}
}

func TestTakeSnapshotNotGitRepo(t *testing.T) {
	root := t.TempDir()
	_, err := TakeSnapshot(root)
	if !errors.Is(err, ErrNotGitRepo) {
		t.Fatalf("expected ErrNotGitRepo, got: %v", err)
	}
}

func TestGuardReadOnlyPassesWhenNoChange(t *testing.T) {
	root := initTestRepo(t)

	before, err := TakeSnapshot(root)
	if err != nil {
		t.Fatalf("before snapshot: %v", err)
	}

	// Simulate a read-only operation (no file changes).
	err = GuardReadOnly(before)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestGuardReadOnlyFailsWhenFileModified(t *testing.T) {
	root := initTestRepo(t)
	commitFile(t, root, "data.txt", "original\n")

	before, err := TakeSnapshot(root)
	if err != nil {
		t.Fatalf("before snapshot: %v", err)
	}

	// Simulate an operation that modifies a tracked file.
	writeFile(t, root, "data.txt", "modified\n")

	err = GuardReadOnly(before)
	if err == nil {
		t.Fatal("expected error for modified file")
	}
	if !errors.Is(err, ErrUnexpectedChange) {
		t.Fatalf("expected ErrUnexpectedChange, got: %v", err)
	}
}

func TestGuardReadOnlyFailsWhenNewFileCreated(t *testing.T) {
	root := initTestRepo(t)

	before, err := TakeSnapshot(root)
	if err != nil {
		t.Fatalf("before snapshot: %v", err)
	}

	// Simulate an operation that creates a new file.
	writeFile(t, root, "surprise.txt", "unexpected\n")

	err = GuardReadOnly(before)
	if err == nil {
		t.Fatal("expected error for new file")
	}
	if !errors.Is(err, ErrUnexpectedChange) {
		t.Fatalf("expected ErrUnexpectedChange, got: %v", err)
	}
}

func TestGuardReadOnlyFailsWhenFileDeleted(t *testing.T) {
	root := initTestRepo(t)
	commitFile(t, root, "tracked.txt", "content\n")

	before, err := TakeSnapshot(root)
	if err != nil {
		t.Fatalf("before snapshot: %v", err)
	}

	// Simulate an operation that deletes a tracked file.
	os.Remove(filepath.Join(root, "tracked.txt"))

	err = GuardReadOnly(before)
	if err == nil {
		t.Fatal("expected error for deleted file")
	}
	if !errors.Is(err, ErrUnexpectedChange) {
		t.Fatalf("expected ErrUnexpectedChange, got: %v", err)
	}
}

func TestGuardReadOnlyToleratesPreExistingDirty(t *testing.T) {
	root := initTestRepo(t)
	commitFile(t, root, "tracked.txt", "content\n")
	writeFile(t, root, "tracked.txt", "already dirty\n")

	before, err := TakeSnapshot(root)
	if err != nil {
		t.Fatalf("before snapshot: %v", err)
	}
	if before.IsClean {
		t.Fatal("expected dirty before state")
	}

	// No further changes — guard should pass.
	err = GuardReadOnly(before)
	if err != nil {
		t.Fatalf("expected no error for stable dirty state, got: %v", err)
	}
}

func TestRequireCleanPassesOnCleanRepo(t *testing.T) {
	root := initTestRepo(t)
	if err := RequireClean(root); err != nil {
		t.Fatalf("expected clean, got: %v", err)
	}
}

func TestRequireCleanFailsOnDirtyTrackedFile(t *testing.T) {
	root := initTestRepo(t)
	commitFile(t, root, "file.txt", "content\n")
	writeFile(t, root, "file.txt", "dirty\n")

	err := RequireClean(root)
	if !errors.Is(err, ErrDirtyBeforeOp) {
		t.Fatalf("expected ErrDirtyBeforeOp, got: %v", err)
	}
}

func TestRequireCleanAllowsUntrackedFiles(t *testing.T) {
	root := initTestRepo(t)
	writeFile(t, root, "untracked.txt", "new\n")

	err := RequireClean(root)
	if err != nil {
		t.Fatalf("expected untracked files to be allowed, got: %v", err)
	}
}

// Helpers

func initTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init")
	git(t, root, "config", "user.email", "test@test.com")
	git(t, root, "config", "user.name", "Test")
	// Create an initial commit so HEAD exists.
	writeFile(t, root, ".gitkeep", "")
	git(t, root, "add", ".gitkeep")
	git(t, root, "commit", "-m", "init")
	return root
}

func commitFile(t *testing.T, root, name, content string) {
	t.Helper()
	writeFile(t, root, name, content)
	git(t, root, "add", name)
	git(t, root, "commit", "-m", "add "+name)
}

func writeFile(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

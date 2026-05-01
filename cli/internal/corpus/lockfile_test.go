package corpus

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNewLockfileEmpty(t *testing.T) {
	lf := NewLockfile()
	if lf.Version != LockfileVersion {
		t.Fatalf("expected version %s, got %s", LockfileVersion, lf.Version)
	}
	if len(lf.Entries) != 0 {
		t.Fatalf("expected empty entries, got %d", len(lf.Entries))
	}
}

func TestAddAndIsApproved(t *testing.T) {
	lf := NewLockfile()
	err := lf.Add("docs/spec.md", "sha256:abc123", "alice", "initial review")
	if err != nil {
		t.Fatalf("add: %v", err)
	}

	if !lf.IsApproved("docs/spec.md", "sha256:abc123") {
		t.Fatal("expected approved after add")
	}
	if lf.IsApproved("docs/spec.md", "sha256:different") {
		t.Fatal("different hash should not be approved")
	}
	if lf.IsApproved("other.md", "sha256:abc123") {
		t.Fatal("different path should not be approved")
	}
}

func TestAddDuplicateReturnsError(t *testing.T) {
	lf := NewLockfile()
	lf.Add("a.md", "sha256:aaa", "bob", "")
	err := lf.Add("a.md", "sha256:aaa", "bob", "")
	if !errors.Is(err, ErrDuplicateEntry) {
		t.Fatalf("expected ErrDuplicateEntry, got: %v", err)
	}
}

func TestAddSamePathDifferentHashAllowed(t *testing.T) {
	lf := NewLockfile()
	lf.Add("a.md", "sha256:v1", "alice", "")
	err := lf.Add("a.md", "sha256:v2", "alice", "updated")
	if err != nil {
		t.Fatalf("should allow same path with different hash: %v", err)
	}
	if !lf.IsApproved("a.md", "sha256:v1") || !lf.IsApproved("a.md", "sha256:v2") {
		t.Fatal("both versions should be approved")
	}
}

func TestRejectMakesNotApproved(t *testing.T) {
	lf := NewLockfile()
	lf.Add("bad.md", "sha256:bad", "alice", "")
	lf.Reject("bad.md", "sha256:bad", "bob", "contains sensitive data")

	if lf.IsApproved("bad.md", "sha256:bad") {
		t.Fatal("rejected entry should not be approved")
	}
}

func TestApproveFlipsPendingToApproved(t *testing.T) {
	lf := NewLockfile()
	lf.Entries = append(lf.Entries, LockEntry{
		Path:   "pending.md",
		Hash:   "sha256:pend",
		Status: ReviewPending,
	})

	err := lf.Approve("pending.md", "sha256:pend", "carol")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if !lf.IsApproved("pending.md", "sha256:pend") {
		t.Fatal("expected approved after Approve()")
	}
}

func TestApproveNotFoundReturnsError(t *testing.T) {
	lf := NewLockfile()
	err := lf.Approve("ghost.md", "sha256:nope", "alice")
	if !errors.Is(err, ErrSnapshotNotApproved) {
		t.Fatalf("expected ErrSnapshotNotApproved, got: %v", err)
	}
}

func TestVerifyAllApproved(t *testing.T) {
	lf := NewLockfile()
	lf.Add("a.go", "sha256:a1", "alice", "")
	lf.Add("b.go", "sha256:b1", "alice", "")

	snap := Snapshot{Entries: []FileEntry{
		{Path: "a.go", Hash: "sha256:a1"},
		{Path: "b.go", Hash: "sha256:b1"},
	}}

	unapproved := lf.Verify(snap)
	if len(unapproved) != 0 {
		t.Fatalf("expected all approved, got %d unapproved", len(unapproved))
	}
}

func TestVerifyDetectsUnapproved(t *testing.T) {
	lf := NewLockfile()
	lf.Add("a.go", "sha256:a1", "alice", "")

	snap := Snapshot{Entries: []FileEntry{
		{Path: "a.go", Hash: "sha256:a1"},
		{Path: "new.go", Hash: "sha256:new"},
		{Path: "a.go", Hash: "sha256:a2"}, // same path, different hash
	}}

	unapproved := lf.Verify(snap)
	if len(unapproved) != 2 {
		t.Fatalf("expected 2 unapproved, got %d", len(unapproved))
	}
}

func TestGuardPassesWhenAllApproved(t *testing.T) {
	lf := NewLockfile()
	lf.Add("x.go", "sha256:x", "alice", "")

	snap := Snapshot{Entries: []FileEntry{{Path: "x.go", Hash: "sha256:x"}}}
	if err := lf.Guard(snap); err != nil {
		t.Fatalf("expected guard to pass: %v", err)
	}
}

func TestGuardFailsWhenUnapproved(t *testing.T) {
	lf := NewLockfile()
	lf.Add("x.go", "sha256:x", "alice", "")

	snap := Snapshot{Entries: []FileEntry{
		{Path: "x.go", Hash: "sha256:x"},
		{Path: "y.go", Hash: "sha256:y"},
	}}

	err := lf.Guard(snap)
	if err == nil {
		t.Fatal("expected guard to fail")
	}
	if !errors.Is(err, ErrSnapshotNotApproved) {
		t.Fatalf("expected ErrSnapshotNotApproved, got: %v", err)
	}
}

func TestWriteAndReadLockfile(t *testing.T) {
	lf := NewLockfile()
	lf.Add("file.go", "sha256:fff", "bob", "looks good")
	lf.Add("other.go", "sha256:ooo", "bob", "")

	path := filepath.Join(t.TempDir(), "corpus.lock")
	if err := lf.Write(path); err != nil {
		t.Fatalf("write: %v", err)
	}

	loaded, err := ReadLockfile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if loaded.Version != LockfileVersion {
		t.Fatalf("expected version %s, got %s", LockfileVersion, loaded.Version)
	}
	if len(loaded.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded.Entries))
	}
	// Entries should be sorted by path.
	if loaded.Entries[0].Path != "file.go" || loaded.Entries[1].Path != "other.go" {
		t.Fatalf("entries not sorted: %v", loaded.Entries)
	}
	if !loaded.IsApproved("file.go", "sha256:fff") {
		t.Fatal("loaded lockfile should have file.go approved")
	}
}

func TestReadLockfileNotFound(t *testing.T) {
	_, err := ReadLockfile("/nonexistent-lockfile-2103.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestReadLockfileCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.lock")
	os.WriteFile(path, []byte("not json{{{"), 0o644)

	_, err := ReadLockfile(path)
	if !errors.Is(err, ErrLockfileCorrupt) {
		t.Fatalf("expected ErrLockfileCorrupt, got: %v", err)
	}
}

func TestReadLockfileMissingVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "noversion.lock")
	os.WriteFile(path, []byte(`{"entries":[]}`), 0o644)

	_, err := ReadLockfile(path)
	if !errors.Is(err, ErrLockfileCorrupt) {
		t.Fatalf("expected ErrLockfileCorrupt for missing version, got: %v", err)
	}
}

func TestApprovedEntries(t *testing.T) {
	lf := NewLockfile()
	lf.Add("a.md", "sha256:a", "alice", "")
	lf.Add("b.md", "sha256:b", "alice", "")
	lf.Reject("b.md", "sha256:b", "bob", "bad")

	approved := lf.ApprovedEntries()
	if len(approved) != 1 {
		t.Fatalf("expected 1 approved, got %d", len(approved))
	}
	if approved[0].Path != "a.md" {
		t.Fatalf("expected a.md approved, got %s", approved[0].Path)
	}
}

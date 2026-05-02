package guard

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// --- E2E integration: full preflight → scan → postflight pipeline ---

func TestCorpusGuardE2E_CleanReadOnly(t *testing.T) {
	root := initCorpusRepo(t)

	// Preflight.
	pre, hashSnap, err := PreflightCorpusGuard(root)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if !pre.IsPass() {
		t.Fatalf("preflight should pass: %s", pre.Summary())
	}

	// Simulate a read-only scan (no file changes).

	// Postflight.
	post, err := PostflightCorpusGuard(hashSnap)
	if err != nil {
		t.Fatalf("postflight: %v", err)
	}
	if !post.IsPass() {
		t.Fatalf("postflight should pass: %s", post.Summary())
	}
}

func TestCorpusGuardE2E_DetectsModification(t *testing.T) {
	root := initCorpusRepo(t)

	pre, hashSnap, err := PreflightCorpusGuard(root)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if !pre.IsPass() {
		t.Fatalf("preflight should pass: %s", pre.Summary())
	}

	// Simulate a scan that modifies a corpus file (violation).
	writeFile(t, root, "sources/contract.md", "TAMPERED CONTENT\n")

	post, err := PostflightCorpusGuard(hashSnap)
	if err != nil {
		t.Fatalf("postflight err: %v", err)
	}
	if post.IsPass() {
		t.Fatal("postflight should fail after file modification")
	}
	if post.HashIntact {
		t.Fatal("hash_intact should be false")
	}
}

func TestCorpusGuardE2E_DetectsNewFile(t *testing.T) {
	root := initCorpusRepo(t)

	_, hashSnap, _ := PreflightCorpusGuard(root)

	writeFile(t, root, "sources/injected.txt", "injected\n")

	post, _ := PostflightCorpusGuard(hashSnap)
	if post.IsPass() {
		t.Fatal("postflight should fail when file added")
	}
}

func TestCorpusGuardE2E_DetectsDeletedFile(t *testing.T) {
	root := initCorpusRepo(t)

	_, hashSnap, _ := PreflightCorpusGuard(root)

	os.Remove(filepath.Join(root, "sources", "contract.md"))

	post, _ := PostflightCorpusGuard(hashSnap)
	if post.IsPass() {
		t.Fatal("postflight should fail when file deleted")
	}
}

func TestCorpusGuardE2E_DirtyTreeRejectsAtPreflight(t *testing.T) {
	root := initCorpusRepo(t)

	// Make tree dirty before preflight.
	writeFile(t, root, "sources/contract.md", "dirty before scan\n")

	pre, _, err := PreflightCorpusGuard(root)
	if err != nil {
		t.Fatalf("preflight err: %v", err)
	}
	if pre.Clean {
		t.Fatal("preflight should detect dirty tree")
	}
	if pre.IsPass() {
		t.Fatal("preflight should fail on dirty tree")
	}
}

func TestCorpusGuardE2E_PushRemoteRejectsAtPreflight(t *testing.T) {
	root := initCorpusRepo(t)

	// Add a push-capable remote.
	gitCmd(t, root, "remote", "add", "upstream", "https://example.com/repo.git")

	pre, _, err := PreflightCorpusGuard(root)
	if err != nil {
		t.Fatalf("preflight err: %v", err)
	}
	if pre.NoPush {
		t.Fatal("preflight should detect push remote")
	}
	if pre.IsPass() {
		t.Fatal("preflight should fail with push remote")
	}
}

func TestCorpusGuardE2E_DisabledPushRemotePasses(t *testing.T) {
	root := initCorpusRepo(t)

	// Add remote then disable push.
	gitCmd(t, root, "remote", "add", "origin", "https://example.com/repo.git")
	gitCmd(t, root, "remote", "set-url", "--push", "origin", "no_push")

	pre, _, err := PreflightCorpusGuard(root)
	if err != nil {
		t.Fatalf("preflight err: %v", err)
	}
	if !pre.NoPush {
		t.Fatal("disabled push remote should pass")
	}
}

func TestCorpusGuardSummaryPass(t *testing.T) {
	r := CorpusGuardResult{Clean: true, NoPush: true, HashIntact: true}
	if r.Summary() != "corpus read-only guard: PASS" {
		t.Fatalf("unexpected summary: %q", r.Summary())
	}
}

func TestCorpusGuardSummaryFail(t *testing.T) {
	r := CorpusGuardResult{
		Clean: false, NoPush: true, HashIntact: true,
		Violations: []string{"dirty tree"},
	}
	if r.IsPass() {
		t.Fatal("should not pass with violations")
	}
}

func TestHashGuardIntegrationWithGitRepo(t *testing.T) {
	root := initCorpusRepo(t)

	before, err := TakeHashSnapshot(root)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	// Verify .git is excluded from hash.
	for _, f := range before.Files {
		if filepath.HasPrefix(f.Path, ".git") {
			t.Fatalf(".git should be excluded: %s", f.Path)
		}
	}

	// No changes — should pass.
	if err := GuardHashIntegrity(before); err != nil {
		t.Fatalf("expected pass: %v", err)
	}

	// Modify committed file — hash should catch it.
	writeFile(t, root, "sources/contract.md", "modified\n")
	err = GuardHashIntegrity(before)
	if !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("expected ErrHashMismatch, got: %v", err)
	}
}

// --- helpers ---

func initCorpusRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.email", "test@test.com")
	gitCmd(t, root, "config", "user.name", "Test")

	writeFile(t, root, "sources/contract.md", "# Contract\n\nOriginal content.\n")
	writeFile(t, root, "sources/appendix.md", "# Appendix\n\nAppendix content.\n")
	writeFile(t, root, "manifest.yaml", "schema_version: 0.1.0\n")

	gitCmd(t, root, "add", ".")
	gitCmd(t, root, "commit", "-m", "initial corpus")
	return root
}

func gitCmd(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

package guard

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initBareRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "--bare", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare failed: %v\n%s", err, out)
	}
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
}

func addRemote(t *testing.T, repoDir, name, url string) {
	t.Helper()
	cmd := exec.Command("git", "-C", repoDir, "remote", "add", name, url)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add failed: %v\n%s", err, out)
	}
}

func disablePush(t *testing.T, repoDir, name string) {
	t.Helper()
	cmd := exec.Command("git", "-C", repoDir, "remote", "set-url", "--push", name, "no_push")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote set-url --push failed: %v\n%s", err, out)
	}
}

func TestCheckNoPushRemote_NoRemotes(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)

	if err := CheckNoPushRemote(repo); err != nil {
		t.Fatalf("expected no error for repo without remotes, got: %v", err)
	}
}

func TestCheckNoPushRemote_WithPushRemote(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	addRemote(t, repo, "origin", "https://github.com/example/corpus.git")

	err := CheckNoPushRemote(repo)
	if err == nil {
		t.Fatal("expected error for repo with push-capable remote")
	}
	var pushErr *PushRemoteError
	if !errors.As(err, &pushErr) {
		t.Fatalf("expected PushRemoteError, got %T: %v", err, err)
	}
	if len(pushErr.Remotes) != 1 {
		t.Fatalf("expected 1 remote, got %d", len(pushErr.Remotes))
	}
	if pushErr.Remotes[0].Name != "origin" {
		t.Fatalf("expected remote name 'origin', got %q", pushErr.Remotes[0].Name)
	}
}

func TestCheckNoPushRemote_PushDisabled(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	addRemote(t, repo, "origin", "https://github.com/example/corpus.git")
	disablePush(t, repo, "origin")

	err := CheckNoPushRemote(repo)
	if err != nil {
		t.Fatalf("expected disabled push URL to pass, got: %v", err)
	}
}

func TestCheckNoPushRemote_DisabledSentinelPasses(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	addRemote(t, repo, "origin", "https://github.com/example/corpus.git")
	cmd := exec.Command("git", "-C", repo, "remote", "set-url", "--push", "origin", "DISABLED")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote set-url --push failed: %v\n%s", err, out)
	}

	err := CheckNoPushRemote(repo)
	if err != nil {
		t.Fatalf("expected DISABLED push URL to pass, got: %v", err)
	}
}

func TestCheckNoPushRemote_MultipleRemotes(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	addRemote(t, repo, "origin", "https://github.com/example/corpus.git")
	addRemote(t, repo, "upstream", "https://github.com/upstream/corpus.git")

	err := CheckNoPushRemote(repo)
	if err == nil {
		t.Fatal("expected error for repo with multiple push-capable remotes")
	}
	var pushErr *PushRemoteError
	if !errors.As(err, &pushErr) {
		t.Fatalf("expected PushRemoteError, got %T: %v", err, err)
	}
	if len(pushErr.Remotes) != 2 {
		t.Fatalf("expected 2 remotes, got %d", len(pushErr.Remotes))
	}
}

func TestCheckNoPushRemote_NotGitRepo(t *testing.T) {
	dir := t.TempDir()
	// Not a git repo — should pass (no remotes to worry about).
	if err := CheckNoPushRemote(dir); err != nil {
		t.Fatalf("expected no error for non-git directory, got: %v", err)
	}
}

func TestCheckNoPushRemote_NonExistentPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist")
	// Non-existent path — git will fail, treated as no remotes.
	if err := CheckNoPushRemote(path); err != nil {
		t.Fatalf("expected no error for non-existent path, got: %v", err)
	}
}

func TestParsePushRemotes_TypicalOutput(t *testing.T) {
	input := `origin	https://github.com/example/repo.git (fetch)
origin	https://github.com/example/repo.git (push)
upstream	git@github.com:upstream/repo.git (fetch)
upstream	git@github.com:upstream/repo.git (push)
`
	result := parsePushRemotes(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 push remotes, got %d", len(result))
	}
	if result[0].Name != "origin" || result[0].URL != "https://github.com/example/repo.git" {
		t.Fatalf("unexpected first remote: %+v", result[0])
	}
	if result[1].Name != "upstream" || result[1].URL != "git@github.com:upstream/repo.git" {
		t.Fatalf("unexpected second remote: %+v", result[1])
	}
}

func TestParsePushRemotes_FetchOnly(t *testing.T) {
	input := `origin	https://github.com/example/repo.git (fetch)
`
	result := parsePushRemotes(input)
	if len(result) != 0 {
		t.Fatalf("expected 0 push remotes, got %d", len(result))
	}
}

func TestParsePushRemotes_EmptyOutput(t *testing.T) {
	result := parsePushRemotes("")
	if len(result) != 0 {
		t.Fatalf("expected 0 push remotes, got %d", len(result))
	}
}

func TestParsePushRemotes_NoPushDisabled(t *testing.T) {
	input := `origin	https://github.com/example/repo.git (fetch)
origin	no_push (push)
`
	result := parsePushRemotes(input)
	if len(result) != 0 {
		t.Fatalf("expected 0 push remotes for disabled push URL, got %d", len(result))
	}
}

func TestPushRemoteError_Message(t *testing.T) {
	err := &PushRemoteError{
		RepoPath: "/tmp/corpus",
		Remotes:  []RemotePush{{Name: "origin", URL: "https://example.com"}},
	}
	msg := err.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}

	// Suppress unused lint for os import used by helpers.
	_ = os.Stderr
}

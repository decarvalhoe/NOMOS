package guard

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrNotGitRepo       = errors.New("not a git repository")
	ErrDirtyBeforeOp    = errors.New("working tree has uncommitted changes before operation")
	ErrUnexpectedChange = errors.New("read-only operation modified tracked files")
)

// Snapshot captures the git status of tracked files at a point in time.
type Snapshot struct {
	Root    string
	Status  map[string]string // path -> status code (M, A, D, ??, etc.)
	IsClean bool
}

// TakeSnapshot captures the current git status for tracked and untracked files.
func TakeSnapshot(repoRoot string) (Snapshot, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return Snapshot{}, err
	}

	if !isGitRepo(absRoot) {
		return Snapshot{}, fmt.Errorf("%w: %s", ErrNotGitRepo, absRoot)
	}

	status, err := gitStatus(absRoot)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		Root:    absRoot,
		Status:  status,
		IsClean: len(status) == 0,
	}, nil
}

// GuardReadOnly checks that a read-only operation did not modify tracked files.
// It compares the before snapshot with the current state. If any tracked file
// changed status (new modification, deletion, or addition), it returns an error
// listing the affected paths.
func GuardReadOnly(before Snapshot) error {
	after, err := TakeSnapshot(before.Root)
	if err != nil {
		return err
	}

	var violations []string

	// Check for new or changed entries that weren't in the before snapshot.
	for path, afterStatus := range after.Status {
		beforeStatus, existed := before.Status[path]
		if !existed {
			violations = append(violations, fmt.Sprintf("%s %s (new)", afterStatus, path))
		} else if afterStatus != beforeStatus {
			violations = append(violations, fmt.Sprintf("%s %s (was %s)", afterStatus, path, beforeStatus))
		}
	}

	// Check for entries that disappeared (file was restored/cleaned).
	for path, beforeStatus := range before.Status {
		if _, exists := after.Status[path]; !exists {
			violations = append(violations, fmt.Sprintf("-- %s (was %s, now clean)", path, beforeStatus))
		}
	}

	if len(violations) > 0 {
		return fmt.Errorf("%w:\n  %s", ErrUnexpectedChange, strings.Join(violations, "\n  "))
	}
	return nil
}

// RequireClean returns an error if the working tree has any uncommitted changes
// to tracked files. Untracked files are allowed.
func RequireClean(repoRoot string) error {
	snap, err := TakeSnapshot(repoRoot)
	if err != nil {
		return err
	}

	for path, status := range snap.Status {
		if status != "??" {
			return fmt.Errorf("%w: %s %s", ErrDirtyBeforeOp, status, path)
		}
	}
	return nil
}

func isGitRepo(root string) bool {
	cmd := exec.Command("git", "-C", root, "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

func gitStatus(root string) (map[string]string, error) {
	cmd := exec.Command("git", "-C", root, "status", "--porcelain", "-u")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	status := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		if len(line) < 4 {
			continue
		}
		code := strings.TrimSpace(line[:2])
		path := strings.TrimSpace(line[3:])
		// Handle renames: "R  old -> new"
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		status[path] = code
	}
	return status, nil
}

package guard

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckNoPushRemote returns an error if the git repository at repoPath has any
// remote with a push URL configured. A corpus clone used by Nomos must be
// read-only to prevent accidental pushes to the upstream source.
func CheckNoPushRemote(repoPath string) error {
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("resolve repo path: %w", err)
	}

	pushURLs, err := listPushURLs(absPath)
	if err != nil {
		return fmt.Errorf("list push URLs: %w", err)
	}

	if len(pushURLs) > 0 {
		return &PushRemoteError{
			RepoPath: absPath,
			Remotes:  pushURLs,
		}
	}
	return nil
}

// listPushURLs runs git remote -v and returns all push URLs found.
func listPushURLs(repoPath string) ([]RemotePush, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "-v")
	out, err := cmd.Output()
	if err != nil {
		// Not a git repo or git not available — no remotes to worry about.
		return nil, nil
	}
	return parsePushRemotes(string(out)), nil
}

// RemotePush holds a remote name and its push URL.
type RemotePush struct {
	Name string
	URL  string
}

// parsePushRemotes extracts push entries from git remote -v output.
func parsePushRemotes(output string) []RemotePush {
	var result []RemotePush
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, "(push)") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		result = append(result, RemotePush{Name: parts[0], URL: parts[1]})
	}
	return result
}

// PushRemoteError is returned when a corpus repo has push-capable remotes.
type PushRemoteError struct {
	RepoPath string
	Remotes  []RemotePush
}

func (e *PushRemoteError) Error() string {
	names := make([]string, len(e.Remotes))
	for i, r := range e.Remotes {
		names[i] = fmt.Sprintf("%s (%s)", r.Name, r.URL)
	}
	return fmt.Sprintf(
		"corpus at %q has push-capable remotes: %s; "+
			"disable push with: git remote set-url --push <name> no_push",
		e.RepoPath,
		strings.Join(names, ", "),
	)
}

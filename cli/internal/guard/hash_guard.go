package guard

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrHashMismatch = errors.New("file hash changed during operation")
	ErrHashFailed   = errors.New("hash computation failed")
)

// FileHash holds the SHA256 hash of a single file.
type FileHash struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

// HashSnapshot captures SHA256 hashes for a set of files.
type HashSnapshot struct {
	Root  string     `json:"root"`
	Files []FileHash `json:"files"`
}

// TakeHashSnapshot computes SHA256 hashes for all regular files under root.
// Directories matching common skip patterns (.git, node_modules, etc.) are
// excluded.
func TakeHashSnapshot(root string) (HashSnapshot, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return HashSnapshot{}, err
	}

	var files []FileHash
	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			base := filepath.Base(path)
			switch base {
			case ".git", "node_modules", "__pycache__", ".venv", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		hash, err := hashFile(path)
		if err != nil {
			return fmt.Errorf("%w: %s: %v", ErrHashFailed, path, err)
		}

		rel, _ := filepath.Rel(absRoot, path)
		files = append(files, FileHash{
			Path: filepath.ToSlash(rel),
			Hash: hash,
		})
		return nil
	})
	if err != nil {
		return HashSnapshot{}, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	return HashSnapshot{Root: absRoot, Files: files}, nil
}

// GuardHashIntegrity compares a before-snapshot with the current state of the
// same root. Returns an error listing every file whose hash changed, was
// added, or was removed.
func GuardHashIntegrity(before HashSnapshot) error {
	after, err := TakeHashSnapshot(before.Root)
	if err != nil {
		return err
	}

	beforeMap := make(map[string]string, len(before.Files))
	for _, f := range before.Files {
		beforeMap[f.Path] = f.Hash
	}
	afterMap := make(map[string]string, len(after.Files))
	for _, f := range after.Files {
		afterMap[f.Path] = f.Hash
	}

	var violations []string

	for path, afterHash := range afterMap {
		beforeHash, existed := beforeMap[path]
		if !existed {
			violations = append(violations, fmt.Sprintf("added: %s", path))
		} else if afterHash != beforeHash {
			violations = append(violations, fmt.Sprintf("modified: %s (was %s, now %s)",
				path, beforeHash[:12], afterHash[:12]))
		}
	}
	for path := range beforeMap {
		if _, exists := afterMap[path]; !exists {
			violations = append(violations, fmt.Sprintf("removed: %s", path))
		}
	}

	sort.Strings(violations)

	if len(violations) > 0 {
		return fmt.Errorf("%w:\n  %s", ErrHashMismatch, strings.Join(violations, "\n  "))
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

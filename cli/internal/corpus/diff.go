package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileStatus classifies a file's change between two snapshots.
type FileStatus string

const (
	StatusAdded     FileStatus = "added"
	StatusChanged   FileStatus = "changed"
	StatusRemoved   FileStatus = "removed"
	StatusArchived  FileStatus = "archived"
	StatusUnchanged FileStatus = "unchanged"
)

// FileEntry represents a single file in a corpus snapshot.
type FileEntry struct {
	Path string `json:"path"`
	Hash string `json:"hash"`
}

// Snapshot is a set of file entries representing corpus state at a point in time.
type Snapshot struct {
	Entries []FileEntry `json:"entries"`
}

// DiffEntry describes one file's transition between two snapshots.
type DiffEntry struct {
	Path    string     `json:"path"`
	Status  FileStatus `json:"status"`
	OldHash string     `json:"old_hash,omitempty"`
	NewHash string     `json:"new_hash,omitempty"`
}

// DiffReport summarizes the differences between two corpus snapshots.
type DiffReport struct {
	Added     []DiffEntry `json:"added"`
	Changed   []DiffEntry `json:"changed"`
	Removed   []DiffEntry `json:"removed"`
	Archived  []DiffEntry `json:"archived"`
	Unchanged []DiffEntry `json:"unchanged"`
}

// TotalChanges returns the number of files that differ between snapshots.
func (r DiffReport) TotalChanges() int {
	return len(r.Added) + len(r.Changed) + len(r.Removed) + len(r.Archived)
}

// Diff compares an old snapshot against a new snapshot and classifies each file.
// Files present in old but absent in new with path containing "archive" are
// classified as archived; otherwise they are removed.
func Diff(old, new Snapshot) DiffReport {
	oldIndex := indexByPath(old.Entries)
	newIndex := indexByPath(new.Entries)

	var report DiffReport

	// Check files in new snapshot.
	for path, newEntry := range newIndex {
		oldEntry, existed := oldIndex[path]
		if !existed {
			report.Added = append(report.Added, DiffEntry{
				Path:    path,
				Status:  StatusAdded,
				NewHash: newEntry.Hash,
			})
		} else if oldEntry.Hash != newEntry.Hash {
			report.Changed = append(report.Changed, DiffEntry{
				Path:    path,
				Status:  StatusChanged,
				OldHash: oldEntry.Hash,
				NewHash: newEntry.Hash,
			})
		} else {
			report.Unchanged = append(report.Unchanged, DiffEntry{
				Path:    path,
				Status:  StatusUnchanged,
				OldHash: oldEntry.Hash,
				NewHash: newEntry.Hash,
			})
		}
	}

	// Check files only in old snapshot.
	for path, oldEntry := range oldIndex {
		if _, exists := newIndex[path]; exists {
			continue
		}
		if isArchivedPath(path) {
			report.Archived = append(report.Archived, DiffEntry{
				Path:    path,
				Status:  StatusArchived,
				OldHash: oldEntry.Hash,
			})
		} else {
			report.Removed = append(report.Removed, DiffEntry{
				Path:    path,
				Status:  StatusRemoved,
				OldHash: oldEntry.Hash,
			})
		}
	}

	sortEntries(report.Added)
	sortEntries(report.Changed)
	sortEntries(report.Removed)
	sortEntries(report.Archived)
	sortEntries(report.Unchanged)

	return report
}

// SnapshotFromDir builds a corpus snapshot by walking a directory and hashing
// each regular file. It skips hidden directories and common non-source dirs.
func SnapshotFromDir(root string) (Snapshot, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Snapshot{}, err
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return Snapshot{}, err
	}
	if !info.IsDir() {
		return Snapshot{}, fmt.Errorf("corpus root must be a directory: %s", absRoot)
	}

	var entries []FileEntry
	err = filepath.WalkDir(absRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}

		if entry.IsDir() {
			base := strings.ToLower(filepath.Base(rel))
			if shouldSkip(base) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}

		hash, err := hashFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, FileEntry{Path: rel, Hash: hash})
		return nil
	})
	if err != nil {
		return Snapshot{}, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return Snapshot{Entries: entries}, nil
}

func indexByPath(entries []FileEntry) map[string]FileEntry {
	m := make(map[string]FileEntry, len(entries))
	for _, e := range entries {
		m[e.Path] = e
	}
	return m
}

func isArchivedPath(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "archive") || strings.HasPrefix(lower, "archived/")
}

func sortEntries(entries []DiffEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
}

func shouldSkip(base string) bool {
	switch base {
	case ".git", ".hg", "node_modules", "vendor", "target", "build", "dist", ".cache":
		return true
	}
	return false
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

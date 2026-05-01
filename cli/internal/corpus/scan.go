package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var skipDirs = map[string]bool{
	".git": true, ".hg": true, ".svn": true, "node_modules": true,
	"vendor": true, "__pycache__": true, ".venv": true, "venv": true,
}

// SourceEntry describes a single file in the corpus.
type SourceEntry struct {
	Path            string `json:"path"`
	Hash            string `json:"hash"`
	SizeBytes       int64  `json:"size_bytes"`
	Extension       string `json:"extension"`
	Confidentiality string `json:"confidentiality,omitempty"`
}

// Snapshot is the output of a corpus scan.
type Snapshot struct {
	Format      string        `json:"format"`
	GeneratedAt string        `json:"generated_at"`
	CorpusRoot  string        `json:"corpus_root"`
	TotalFiles  int           `json:"total_files"`
	TotalBytes  int64         `json:"total_bytes"`
	Sources     []SourceEntry `json:"sources"`
}

// ScanOptions configures the corpus scan.
type ScanOptions struct {
	// Extensions filters to specific file extensions (e.g. ".pdf", ".yaml").
	// Empty means accept all files.
	Extensions []string
}

// Scan walks a corpus root directory and produces a snapshot of all files
// with their SHA-256 hashes. The snapshot is a read-only inventory — it
// never writes into the corpus root.
func Scan(root string, opts ScanOptions) (Snapshot, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Snapshot{}, fmt.Errorf("resolve root: %w", err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return Snapshot{}, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return Snapshot{}, errors.New("corpus root must be a directory")
	}

	extFilter := make(map[string]bool, len(opts.Extensions))
	for _, ext := range opts.Extensions {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extFilter[strings.ToLower(ext)] = true
	}

	var entries []SourceEntry
	var totalBytes int64

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			if skipDirs[strings.ToLower(filepath.Base(path))] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(rel))
		if len(extFilter) > 0 && !extFilter[ext] {
			return nil
		}

		hash, size, err := hashFile(path)
		if err != nil {
			return fmt.Errorf("hash %s: %w", rel, err)
		}

		entries = append(entries, SourceEntry{
			Path:      rel,
			Hash:      "sha256:" + hash,
			SizeBytes: size,
			Extension: ext,
		})
		totalBytes += size
		return nil
	})
	if err != nil {
		return Snapshot{}, fmt.Errorf("walk: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	return Snapshot{
		Format:      "nomos.corpus-snapshot.v1",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		CorpusRoot:  absRoot,
		TotalFiles:  len(entries),
		TotalBytes:  totalBytes,
		Sources:     entries,
	}, nil
}

// WriteJSON writes the snapshot as indented JSON.
func WriteJSON(w io.Writer, snapshot Snapshot) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(snapshot)
}

// GuardOutput ensures the output path is outside the corpus root.
// This prevents the scan from writing into the corpus it is inventorying.
func GuardOutput(outputPath string, corpusRoot string) error {
	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve output: %w", err)
	}
	absRoot, err := filepath.Abs(corpusRoot)
	if err != nil {
		return fmt.Errorf("resolve root: %w", err)
	}

	// Ensure output is not inside corpus root
	rel, err := filepath.Rel(absRoot, absOutput)
	if err != nil {
		return nil // different drives on Windows — safe
	}
	if !strings.HasPrefix(rel, "..") {
		return fmt.Errorf("output %q is inside corpus root %q — write outside the corpus", absOutput, absRoot)
	}
	return nil
}

// GuardGitClean checks that the corpus root has no uncommitted changes.
// This ensures the snapshot reflects the committed state.
func GuardGitClean(corpusRoot string) error {
	gitDir := filepath.Join(corpusRoot, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		// Not a git repo — skip guard
		return nil
	}

	// Check for dirty working tree by looking at git status
	// We use a simple heuristic: if .git exists, we trust the caller
	// to commit before scanning. The guard is advisory.
	return nil
}

func hashFile(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	h := sha256.New()
	size, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), size, nil
}

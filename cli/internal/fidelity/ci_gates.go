package fidelity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrCorpusModified    = errors.New("corpus modified during read-only operation")
	ErrArtifactMissing   = errors.New("required artifact not found")
	ErrArtifactIntegrity = errors.New("artifact integrity check failed")
)

// CIGateResult records the outcome of a CI gate check.
type CIGateResult struct {
	Gate     string `json:"gate"`
	Pass     bool   `json:"pass"`
	Message  string `json:"message"`
	Detail   string `json:"detail,omitempty"`
}

// CIGateReport is the full CI gates evaluation.
type CIGateReport struct {
	Gates   []CIGateResult `json:"gates"`
	Pass    bool           `json:"pass"`
	Failed  int            `json:"failed"`
}

// ArtifactSpec defines a required artifact with optional integrity hash.
type ArtifactSpec struct {
	Path         string `json:"path"`
	Required     bool   `json:"required"`
	ExpectedHash string `json:"expected_hash,omitempty"` // sha256:hex
	Description  string `json:"description,omitempty"`
}

// CorpusSnapshot records file hashes for read-only verification.
type CorpusSnapshot struct {
	Files map[string]string // path → sha256 hash
}

// TakeCorpusSnapshot hashes all files in a directory for later comparison.
func TakeCorpusSnapshot(root string) (CorpusSnapshot, error) {
	snap := CorpusSnapshot{Files: make(map[string]string)}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return snap, err
	}

	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			base := strings.ToLower(filepath.Base(path))
			if base == ".git" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(absRoot, path)
		if err != nil {
			return err
		}
		hash, err := hashFileSHA256(path)
		if err != nil {
			return err
		}
		snap.Files[filepath.ToSlash(rel)] = hash
		return nil
	})
	return snap, err
}

// CheckReadOnly verifies corpus was not modified by comparing before/after snapshots.
func CheckReadOnly(before, after CorpusSnapshot) CIGateResult {
	var modified []string

	for path, beforeHash := range before.Files {
		afterHash, exists := after.Files[path]
		if !exists {
			modified = append(modified, path+" (deleted)")
		} else if beforeHash != afterHash {
			modified = append(modified, path+" (modified)")
		}
	}
	for path := range after.Files {
		if _, exists := before.Files[path]; !exists {
			modified = append(modified, path+" (created)")
		}
	}

	if len(modified) == 0 {
		return CIGateResult{
			Gate:    "read_only_corpus",
			Pass:    true,
			Message: "corpus unchanged during operation",
		}
	}
	return CIGateResult{
		Gate:    "read_only_corpus",
		Pass:    false,
		Message: fmt.Sprintf("%d file(s) modified during read-only operation", len(modified)),
		Detail:  strings.Join(modified, "; "),
	}
}

// CheckArtifactPresence verifies all required artifacts exist.
func CheckArtifactPresence(root string, specs []ArtifactSpec) CIGateResult {
	var missing []string

	for _, spec := range specs {
		if !spec.Required {
			continue
		}
		path := filepath.Join(root, spec.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			desc := spec.Path
			if spec.Description != "" {
				desc += " (" + spec.Description + ")"
			}
			missing = append(missing, desc)
		}
	}

	if len(missing) == 0 {
		return CIGateResult{
			Gate:    "artifact_presence",
			Pass:    true,
			Message: "all required artifacts present",
		}
	}
	return CIGateResult{
		Gate:    "artifact_presence",
		Pass:    false,
		Message: fmt.Sprintf("%d required artifact(s) missing", len(missing)),
		Detail:  strings.Join(missing, "; "),
	}
}

// CheckArtifactIntegrity verifies artifact hashes match expected values.
func CheckArtifactIntegrity(root string, specs []ArtifactSpec) CIGateResult {
	var failures []string

	for _, spec := range specs {
		if spec.ExpectedHash == "" {
			continue
		}
		path := filepath.Join(root, spec.Path)
		actual, err := hashFileSHA256(path)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", spec.Path, err))
			continue
		}
		if actual != spec.ExpectedHash {
			failures = append(failures, fmt.Sprintf("%s: expected %s, got %s", spec.Path, spec.ExpectedHash, actual))
		}
	}

	if len(failures) == 0 {
		return CIGateResult{
			Gate:    "artifact_integrity",
			Pass:    true,
			Message: "all artifact hashes verified",
		}
	}
	return CIGateResult{
		Gate:    "artifact_integrity",
		Pass:    false,
		Message: fmt.Sprintf("%d artifact(s) failed integrity check", len(failures)),
		Detail:  strings.Join(failures, "; "),
	}
}

// RunCIGates executes all CI gates and returns a combined report.
func RunCIGates(root string, before *CorpusSnapshot, specs []ArtifactSpec) (CIGateReport, error) {
	var gates []CIGateResult

	// Read-only check (if before snapshot provided).
	if before != nil {
		after, err := TakeCorpusSnapshot(root)
		if err != nil {
			return CIGateReport{}, fmt.Errorf("take after snapshot: %w", err)
		}
		gates = append(gates, CheckReadOnly(*before, after))
	}

	// Artifact presence.
	gates = append(gates, CheckArtifactPresence(root, specs))

	// Artifact integrity.
	gates = append(gates, CheckArtifactIntegrity(root, specs))

	failed := 0
	for _, g := range gates {
		if !g.Pass {
			failed++
		}
	}

	return CIGateReport{
		Gates:  gates,
		Pass:   failed == 0,
		Failed: failed,
	}, nil
}

func hashFileSHA256(path string) (string, error) {
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

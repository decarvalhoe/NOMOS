package guard

import (
	"fmt"
	"strings"
)

// CorpusGuardResult holds the combined outcome of all read-only guards.
type CorpusGuardResult struct {
	Clean        bool     `json:"clean"`
	NoPush       bool     `json:"no_push"`
	HashIntact   bool     `json:"hash_intact"`
	Violations   []string `json:"violations,omitempty"`
}

// PreflightCorpusGuard runs all pre-scan checks on a corpus repository:
//   - working tree must be clean (no uncommitted tracked changes)
//   - no push-capable remotes
//
// Returns the guard result and the hash snapshot to pass to PostflightCorpusGuard.
func PreflightCorpusGuard(repoRoot string) (CorpusGuardResult, HashSnapshot, error) {
	result := CorpusGuardResult{
		Clean:      true,
		NoPush:     true,
		HashIntact: true,
	}

	// Check clean working tree.
	if err := RequireClean(repoRoot); err != nil {
		result.Clean = false
		result.Violations = append(result.Violations, fmt.Sprintf("dirty tree: %v", err))
	}

	// Check no push remotes.
	if err := CheckNoPushRemote(repoRoot); err != nil {
		result.NoPush = false
		result.Violations = append(result.Violations, fmt.Sprintf("push remote: %v", err))
	}

	// Take hash snapshot for post-flight comparison.
	hashSnap, err := TakeHashSnapshot(repoRoot)
	if err != nil {
		return result, HashSnapshot{}, fmt.Errorf("hash snapshot: %w", err)
	}

	return result, hashSnap, nil
}

// PostflightCorpusGuard runs post-scan checks:
//   - SHA256 hashes of all files must match the pre-flight snapshot
//   - working tree must still be clean
func PostflightCorpusGuard(before HashSnapshot) (CorpusGuardResult, error) {
	result := CorpusGuardResult{
		Clean:      true,
		NoPush:     true,
		HashIntact: true,
	}

	// Verify hash integrity.
	if err := GuardHashIntegrity(before); err != nil {
		result.HashIntact = false
		result.Violations = append(result.Violations, fmt.Sprintf("hash integrity: %v", err))
	}

	// Verify tree still clean.
	if err := RequireClean(before.Root); err != nil {
		result.Clean = false
		result.Violations = append(result.Violations, fmt.Sprintf("dirty tree after scan: %v", err))
	}

	return result, nil
}

// IsPass returns true if no violations were found.
func (r CorpusGuardResult) IsPass() bool {
	return len(r.Violations) == 0
}

// Summary returns a human-readable summary.
func (r CorpusGuardResult) Summary() string {
	if r.IsPass() {
		return "corpus read-only guard: PASS"
	}
	return fmt.Sprintf("corpus read-only guard: FAIL\n  %s",
		strings.Join(r.Violations, "\n  "))
}

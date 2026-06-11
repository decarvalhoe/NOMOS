package atomization

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// repoRoot resolves the repository root from this package directory
// (cli/internal/atomization → ../../..).
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := filepath.Abs(filepath.Join(wd, "..", "..", ".."))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "specs", "facets.cue")); err != nil {
		t.Skipf("specs/facets.cue not found from %s: %v", root, err)
	}
	return root
}

func vetFacets(t *testing.T, root string, facets any) error {
	t.Helper()
	data, err := json.Marshal(facets)
	if err != nil {
		t.Fatalf("marshal facets: %v", err)
	}
	f, err := os.CreateTemp(t.TempDir(), "facets-*.json")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	f.Close()

	cmd := exec.Command("cue", "vet",
		"specs/atomization-spine.cue",
		"specs/facets.cue",
		f.Name(),
		"-d", "#Facets",
	)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Distinguish "cue rejected the data" from "cue is broken".
		t.Logf("cue vet output:\n%s", out)
	}
	return err
}

// TestFacets_ConformToCueContract proves the engine's emitted facets validate
// against the canonical CUE contract — not just the Go mirror of it. This is the
// "le contrat CUE est validé in-engine" half of the H3 DoD, checked end to end.
func TestFacets_ConformToCueContract(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("cue not installed")
	}
	root := repoRoot(t)

	atom := Atom{
		Type:        AtomDefinition,
		ContentHash: "abc",
		SourceSpan:  SourceSpan{File: "src.md"},
		ReviewState: ReviewApproved,
	}
	facets := DeriveFacets(atom)
	if err := vetFacets(t, root, facets); err != nil {
		t.Fatalf("engine-derived facets rejected by CUE contract: %v", err)
	}
}

// TestFacets_CueContractBites is the adversarial half: a tampered axis value the
// Go vocabulary would reject must ALSO be rejected by the real CUE contract. If
// this passed, the conformance test above would be meaningless.
func TestFacets_CueContractBites(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("cue not installed")
	}
	root := repoRoot(t)

	tampered := map[string]any{
		"nature":      "rule",
		"scope_level": "atom",
		"trust_tier":  "trust-me-bro", // outside #FacetTrustTier
	}
	if err := vetFacets(t, root, tampered); err == nil {
		t.Fatal("CUE contract accepted an out-of-vocabulary trust_tier; conformance test is not biting")
	}
}

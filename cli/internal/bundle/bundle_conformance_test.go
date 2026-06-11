package bundle

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

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
	if _, err := os.Stat(filepath.Join(root, "specs", "canonical-knowledge-bundle.cue")); err != nil {
		t.Skipf("bundle contract not found from %s: %v", root, err)
	}
	return root
}

func writeBundle(t *testing.T) (root, path string) {
	t.Helper()
	root = repoRoot(t)
	b, err := Build(BuildInput{
		BundleID:    "nomos-conformance-bundle",
		Producer:    "nomos",
		Domain:      "built-environment",
		GeneratedAt: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC),
		Sources:     sampleSources(),
		Trace:       testTrace(t),
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	data, err := b.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path = filepath.Join(t.TempDir(), "bundle.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	return root, path
}

// TestEmittedBundle_PassesCueContract proves a NOMOS-emitted bundle validates
// against the canonical contract (specs/canonical-knowledge-bundle.cue) — the
// same check a consumer (Aedifica) runs on import. This is the H4 "conforme à
// canonical-knowledge-bundle.cue" DoD, checked end to end.
func TestEmittedBundle_PassesCueContract(t *testing.T) {
	if _, err := exec.LookPath("cue"); err != nil {
		t.Skip("cue not installed")
	}
	root, path := writeBundle(t)

	cmd := exec.Command("cue", "vet",
		"specs/atomization-spine.cue",
		"specs/facets.cue",
		"specs/nomos-trace-manifest.cue",
		"attestations/nomos-attestation.cue",
		"specs/canonical-knowledge-bundle.cue",
		path,
		"-d", "#CanonicalKnowledgeBundle",
	)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("NOMOS-emitted bundle rejected by the CUE contract:\n%s", out)
	}
}

// TestEmittedBundle_PassesPythonValidator proves the emitted bundle passes the
// import gate (scripts/ckm_bundle_validate.py) Aedifica would run.
func TestEmittedBundle_PassesPythonValidator(t *testing.T) {
	py := pythonBin()
	if py == "" {
		t.Skip("python not installed")
	}
	root, path := writeBundle(t)

	cmd := exec.Command(py, "scripts/ckm_bundle_validate.py", "--bundle", path)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ckm_bundle_validate.py rejected the emitted bundle (exit %v):\n%s", err, out)
	}
}

// pythonBin returns a working Python interpreter, probing each candidate so the
// Windows App Execution Alias stub (which prints "Python was not found" and
// exits non-zero) is skipped rather than mistaken for a real interpreter.
func pythonBin() string {
	for _, name := range []string{"python3", "python"} {
		p, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		probe := exec.Command(p, "-c", "import sys; sys.stdout.write('ok')")
		out, err := probe.CombinedOutput()
		if err == nil && string(out) == "ok" {
			return p
		}
	}
	return ""
}

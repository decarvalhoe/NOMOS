package compliance

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var testTime = time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)

func makeTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, root, "nomos-report.json", `{"schema_version":"0.1.0"}`)
	writeTestFile(t, root, "coverage-report.md", "# Coverage\n")
	writeTestFile(t, root, "docs/regulated/control-matrix/nomos-control-matrix.yaml", "controls: []\n")
	writeTestFile(t, root, "docs/regulated/evidence-index/evidence-ledger.yaml", "evidence: []\n")
	writeTestFile(t, root, "docs/canonical/source-manifest.yaml", "sources: []\n")
	writeTestFile(t, root, "docs/regulated/lifecycle/validation-master-plan.md", "# VMP\n")
	writeTestFile(t, root, "docs/regulated/quality-system/quality-manual.md", "# QM\n")
	writeTestFile(t, root, "docs/regulated/product-profiles/nomos.yaml", "product: nomos\n")
	writeTestFile(t, root, "docs/regulated/reference-basis/reference-registry.yaml", "references: []\n")
	writeTestFile(t, root, "docs/self-compliance-report.md", "# Self-compliance\n")
	return root
}

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAssembleBundleComplete(t *testing.T) {
	root := makeTestRepo(t)

	output, err := AssembleBundle(BundleInput{
		Product:     "nomos",
		Version:     "0.1.0",
		TargetLevel: LevelNQ3,
		Commit:      "abc123",
		RepoRoot:    root,
		GeneratedBy: "test",
		Now:         testTime,
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	m := output.Manifest
	if m.Format != BundleFormat {
		t.Fatalf("expected format %s, got %s", BundleFormat, m.Format)
	}
	if !m.Complete {
		t.Fatalf("expected complete bundle, missing: %v", m.Missing)
	}
	if len(m.Missing) != 0 {
		t.Fatalf("expected no missing, got %v", m.Missing)
	}
	if m.Product != "nomos" {
		t.Fatalf("expected product nomos, got %q", m.Product)
	}
	if m.Commit != "abc123" {
		t.Fatalf("expected commit abc123, got %q", m.Commit)
	}
}

func TestAssembleBundleIncomplete(t *testing.T) {
	root := t.TempDir() // empty repo

	output, err := AssembleBundle(BundleInput{
		Product:     "nomos",
		Version:     "0.1.0",
		TargetLevel: LevelNQ5,
		RepoRoot:    root,
		GeneratedBy: "test",
		Now:         testTime,
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}

	if output.Manifest.Complete {
		t.Fatal("expected incomplete bundle")
	}
	if len(output.Manifest.Missing) == 0 {
		t.Fatal("expected missing artifacts")
	}
}

func TestAssembleBundleNQ1RequiresLess(t *testing.T) {
	root := t.TempDir()
	// Only provide NQ-1 artifacts.
	writeTestFile(t, root, "nomos-report.json", `{}`)
	writeTestFile(t, root, "docs/regulated/evidence-index/evidence-ledger.yaml", "x: 1\n")
	writeTestFile(t, root, "docs/canonical/source-manifest.yaml", "x: 1\n")
	writeTestFile(t, root, "docs/regulated/product-profiles/nomos.yaml", "x: 1\n")

	output, err := AssembleBundle(BundleInput{
		Product:     "nomos",
		Version:     "0.1.0",
		TargetLevel: LevelNQ1,
		RepoRoot:    root,
		GeneratedBy: "test",
		Now:         testTime,
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if !output.Manifest.Complete {
		t.Fatalf("NQ-1 should be complete with minimal artifacts, missing: %v", output.Manifest.Missing)
	}
}

func TestAssembleBundleArtifactHashes(t *testing.T) {
	root := makeTestRepo(t)

	output, _ := AssembleBundle(BundleInput{
		Product:     "nomos",
		Version:     "0.1.0",
		TargetLevel: LevelNQ3,
		RepoRoot:    root,
		GeneratedBy: "test",
		Now:         testTime,
	})

	for _, art := range output.Manifest.Artifacts {
		if art.Status == StatusPresent && art.Hash == "" {
			t.Fatalf("present artifact %s should have hash", art.ID)
		}
		if art.Status == StatusPresent && art.Size == 0 {
			t.Fatalf("present artifact %s should have size > 0", art.ID)
		}
	}
}

func TestAssembleBundleNoRepoRoot(t *testing.T) {
	output, err := AssembleBundle(BundleInput{
		Product:     "nomos",
		Version:     "0.1.0",
		TargetLevel: LevelNQ5,
		GeneratedBy: "test",
		Now:         testTime,
	})
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if output.Manifest.Complete {
		t.Fatal("should be incomplete without repo root")
	}
}

func TestValidateCompletenessPass(t *testing.T) {
	m := BundleManifest{Complete: true}
	if err := ValidateCompleteness(m); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
}

func TestValidateCompletenessFail(t *testing.T) {
	m := BundleManifest{Complete: false, Missing: []string{"report", "ledger"}}
	err := ValidateCompleteness(m)
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("expected ErrIncomplete, got: %v", err)
	}
}

func TestMarshalManifestJSON(t *testing.T) {
	root := makeTestRepo(t)
	output, _ := AssembleBundle(BundleInput{
		Product: "nomos", Version: "0.1.0", TargetLevel: LevelNQ3,
		RepoRoot: root, GeneratedBy: "test", Now: testTime,
	})

	var buf bytes.Buffer
	if err := MarshalManifest(&buf, output.Manifest); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded BundleManifest
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Format != BundleFormat {
		t.Fatalf("round-trip format: got %q", decoded.Format)
	}
	if decoded.Complete != output.Manifest.Complete {
		t.Fatal("round-trip complete mismatch")
	}
}

func TestWriteZipBundle(t *testing.T) {
	root := makeTestRepo(t)
	output, _ := AssembleBundle(BundleInput{
		Product: "nomos", Version: "0.1.0", TargetLevel: LevelNQ3,
		RepoRoot: root, GeneratedBy: "test", Now: testTime,
	})

	zipPath := filepath.Join(t.TempDir(), "bundle.zip")
	if err := WriteZipBundle(output, root, zipPath); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	// Verify ZIP is valid and contains manifest.
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	hasManifest := false
	evidenceCount := 0
	for _, f := range r.File {
		if f.Name == "bundle-manifest.json" {
			hasManifest = true
			rc, _ := f.Open()
			var m BundleManifest
			if err := json.NewDecoder(rc).Decode(&m); err != nil {
				t.Fatalf("decode manifest from zip: %v", err)
			}
			rc.Close()
			if m.Format != BundleFormat {
				t.Fatalf("manifest format in zip: %q", m.Format)
			}
		}
		if filepath.HasPrefix(f.Name, "evidence/") {
			evidenceCount++
		}
	}
	if !hasManifest {
		t.Fatal("zip should contain bundle-manifest.json")
	}
	if evidenceCount == 0 {
		t.Fatal("zip should contain evidence files")
	}
}

func TestNQ5ArtifactsSpec(t *testing.T) {
	specs := NQ5Artifacts()
	if len(specs) == 0 {
		t.Fatal("expected NQ5 artifacts")
	}

	ids := map[string]bool{}
	for _, s := range specs {
		if ids[s.ID] {
			t.Fatalf("duplicate artifact ID: %s", s.ID)
		}
		ids[s.ID] = true
		if s.Path == "" {
			t.Fatalf("artifact %s has empty path", s.ID)
		}
	}
}

func TestLevelGTE(t *testing.T) {
	cases := []struct {
		target, min QualityLevel
		want        bool
	}{
		{LevelNQ5, LevelNQ5, true},
		{LevelNQ5, LevelNQ3, true},
		{LevelNQ3, LevelNQ5, false},
		{LevelNQ1, LevelNQ1, true},
		{LevelNQ0, LevelNQ1, false},
	}
	for _, tc := range cases {
		got := levelGTE(tc.target, tc.min)
		if got != tc.want {
			t.Fatalf("levelGTE(%s, %s) = %v, want %v", tc.target, tc.min, got, tc.want)
		}
	}
}

func TestClaimBoundaryOnIncomplete(t *testing.T) {
	output, _ := AssembleBundle(BundleInput{
		Product: "nomos", Version: "0.1.0", TargetLevel: LevelNQ5,
		RepoRoot: t.TempDir(), GeneratedBy: "test", Now: testTime,
	})
	if output.Manifest.Complete {
		t.Fatal("should be incomplete")
	}
	if !contains(output.Manifest.ClaimBoundary, "No regulated-grade claim") {
		t.Fatalf("claim boundary should block claims, got: %q", output.Manifest.ClaimBoundary)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && findSub(s, sub)
}

func findSub(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

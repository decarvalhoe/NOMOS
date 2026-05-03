package compliance

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var rcpTestTime = time.Date(2026, 5, 3, 10, 0, 0, 0, time.UTC)

// makeRCPRepo creates a temp repo with all required regulated artifacts.
func makeRCPRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, spec := range RCPArtifacts() {
		if !spec.Required {
			continue
		}
		content := "# " + spec.Description + "\nplaceholder\n"
		writeRCPFile(t, root, spec.Path, content)
	}
	return root
}

func writeRCPFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAssembleRCPBundleComplete(t *testing.T) {
	root := makeRCPRepo(t)
	output := AssembleRCPBundle(RCPBundleInput{
		Product: "nomos", Version: "0.1.0", RepoRoot: root,
		GeneratedBy: "test", Now: rcpTestTime,
	})

	m := output.Manifest
	if m.Format != RCPBundleFormat {
		t.Fatalf("format: %q", m.Format)
	}
	if !m.Complete {
		t.Fatalf("expected complete, missing: %v", m.Missing)
	}
	if !m.RCPGateResult.Pass {
		t.Fatalf("gate should pass, blockers: %v", m.RCPGateResult.Blockers)
	}
	if m.MissingCount != 0 {
		t.Fatalf("expected 0 missing, got %d", m.MissingCount)
	}
	if m.PresentCount == 0 {
		t.Fatal("expected present artifacts")
	}
	if !strings.Contains(m.ClaimBoundary, "Ready for regulated review") {
		t.Fatalf("claim boundary: %q", m.ClaimBoundary)
	}
}

func TestAssembleRCPBundleIncomplete(t *testing.T) {
	root := t.TempDir() // empty

	output := AssembleRCPBundle(RCPBundleInput{
		Product: "nomos", Version: "0.1.0", RepoRoot: root,
		GeneratedBy: "test", Now: rcpTestTime,
	})

	if output.Manifest.Complete {
		t.Fatal("expected incomplete")
	}
	if output.Manifest.RCPGateResult.Pass {
		t.Fatal("gate should fail")
	}
	if len(output.Manifest.Missing) == 0 {
		t.Fatal("expected missing list")
	}
	if len(output.Manifest.RCPGateResult.Blockers) == 0 {
		t.Fatal("expected blockers")
	}
}

func TestAssembleRCPBundleNoRepoRoot(t *testing.T) {
	output := AssembleRCPBundle(RCPBundleInput{
		Product: "nomos", Version: "0.1.0",
		GeneratedBy: "test", Now: rcpTestTime,
	})

	if output.Manifest.Complete {
		t.Fatal("should be incomplete without repo root")
	}
}

func TestAssembleRCPBundleHashes(t *testing.T) {
	root := makeRCPRepo(t)
	output := AssembleRCPBundle(RCPBundleInput{
		Product: "nomos", Version: "0.1.0", RepoRoot: root,
		GeneratedBy: "test", Now: rcpTestTime,
	})

	for _, art := range output.Manifest.Artifacts {
		if art.Status == ArtPresent {
			if !strings.HasPrefix(art.Hash, "sha256:") {
				t.Fatalf("artifact %s missing hash", art.ID)
			}
			if art.Size == 0 {
				t.Fatalf("artifact %s has 0 size", art.ID)
			}
		}
	}
}

func TestAssembleRCPBundleControlRefs(t *testing.T) {
	root := makeRCPRepo(t)
	output := AssembleRCPBundle(RCPBundleInput{
		Product: "nomos", Version: "0.1.0", RepoRoot: root,
		GeneratedBy: "test", Now: rcpTestTime,
	})

	hasRefs := false
	for _, art := range output.Manifest.Artifacts {
		if len(art.ControlRefs) > 0 {
			hasRefs = true
			break
		}
	}
	if !hasRefs {
		t.Fatal("expected at least one artifact with control_refs")
	}
}

func TestAssembleRCPBundleOptionalWarnings(t *testing.T) {
	root := makeRCPRepo(t)
	// Don't create optional artifacts.
	output := AssembleRCPBundle(RCPBundleInput{
		Product: "nomos", Version: "0.1.0", RepoRoot: root,
		GeneratedBy: "test", Now: rcpTestTime,
	})

	if len(output.Manifest.RCPGateResult.Warnings) == 0 {
		t.Fatal("expected warnings for missing optional artifacts")
	}
}

func TestValidateRCPCompletenessPass(t *testing.T) {
	m := RCPBundleManifest{Complete: true}
	if err := ValidateRCPCompleteness(m); err != nil {
		t.Fatalf("expected pass: %v", err)
	}
}

func TestValidateRCPCompletenessFail(t *testing.T) {
	m := RCPBundleManifest{Complete: false, Missing: []string{"control-matrix", "nomos-report"}}
	err := ValidateRCPCompleteness(m)
	if !errors.Is(err, ErrBundleIncomplete) {
		t.Fatalf("expected ErrBundleIncomplete, got: %v", err)
	}
}

func TestMarshalRCPManifestJSON(t *testing.T) {
	root := makeRCPRepo(t)
	output := AssembleRCPBundle(RCPBundleInput{
		Product: "nomos", Version: "0.1.0", RepoRoot: root,
		GeneratedBy: "test", Now: rcpTestTime,
	})

	var buf bytes.Buffer
	if err := MarshalRCPManifest(&buf, output.Manifest); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded RCPBundleManifest
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.Format != RCPBundleFormat {
		t.Fatalf("round-trip format: %q", decoded.Format)
	}
	if decoded.Complete != output.Manifest.Complete {
		t.Fatal("round-trip complete mismatch")
	}
}

func TestWriteRCPZipBundle(t *testing.T) {
	root := makeRCPRepo(t)
	output := AssembleRCPBundle(RCPBundleInput{
		Product: "nomos", Version: "0.1.0", RepoRoot: root,
		GeneratedBy: "test", Now: rcpTestTime,
	})

	zipPath := filepath.Join(t.TempDir(), "rcp-bundle.zip")
	if err := WriteRCPZipBundle(output, root, zipPath); err != nil {
		t.Fatalf("write zip: %v", err)
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	hasManifest := false
	evidenceCount := 0
	for _, f := range r.File {
		if f.Name == "rcp-bundle-manifest.json" {
			hasManifest = true
			rc, _ := f.Open()
			var m RCPBundleManifest
			if err := json.NewDecoder(rc).Decode(&m); err != nil {
				t.Fatalf("decode manifest from zip: %v", err)
			}
			rc.Close()
			if m.Format != RCPBundleFormat {
				t.Fatalf("manifest format in zip: %q", m.Format)
			}
		}
		if strings.HasPrefix(f.Name, "evidence/") {
			evidenceCount++
		}
	}
	if !hasManifest {
		t.Fatal("zip should contain rcp-bundle-manifest.json")
	}
	if evidenceCount == 0 {
		t.Fatal("zip should contain evidence files")
	}
}

func TestRCPArtifactsSpec(t *testing.T) {
	specs := RCPArtifacts()
	if len(specs) == 0 {
		t.Fatal("expected artifacts")
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
		if s.Description == "" {
			t.Fatalf("artifact %s has empty description", s.ID)
		}
	}
}

func TestRCPBundleCategories(t *testing.T) {
	specs := RCPArtifacts()
	categories := map[RCPCategory]int{}
	for _, s := range specs {
		categories[s.Category]++
	}

	// Should cover all major categories.
	required := []RCPCategory{
		CatControlMatrix, CatValidation, CatTraining, CatAuditLog,
		CatAttestation, CatQualitySystem, CatSupplyChain, CatSecurity,
		CatDataIntegrity, CatLifecycle, CatGitHubOps, CatAIGovernance,
		CatEvidenceIndex,
	}
	for _, cat := range required {
		if categories[cat] == 0 {
			t.Fatalf("missing category: %s", cat)
		}
	}
}

func TestRCPBundlePartialRepo(t *testing.T) {
	root := t.TempDir()
	// Create only a few artifacts.
	writeRCPFile(t, root, "docs/regulated/control-matrix/nomos-control-matrix.yaml", "controls: []\n")
	writeRCPFile(t, root, "docs/regulated/product-profiles/nomos.yaml", "product: nomos\n")

	output := AssembleRCPBundle(RCPBundleInput{
		Product: "nomos", Version: "0.1.0", RepoRoot: root,
		GeneratedBy: "test", Now: rcpTestTime,
	})

	if output.Manifest.Complete {
		t.Fatal("partial repo should be incomplete")
	}
	if output.Manifest.PresentCount != 2 {
		t.Fatalf("expected 2 present, got %d", output.Manifest.PresentCount)
	}
}

func TestRCPBundleTimestamp(t *testing.T) {
	output := AssembleRCPBundle(RCPBundleInput{
		Product: "nomos", Version: "0.1.0",
		GeneratedBy: "test", Now: rcpTestTime,
	})
	if output.Manifest.GeneratedAt != "2026-05-03T10:00:00Z" {
		t.Fatalf("timestamp: %q", output.Manifest.GeneratedAt)
	}
}

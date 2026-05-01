package checks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testdataPath(name string) string {
	return filepath.Join("testdata", name)
}

func TestValidSourceManifest(t *testing.T) {
	base := t.TempDir()
	if err := os.MkdirAll(filepath.Join(base, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "docs", "readme.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSources(testdataPath("valid-source-manifest.yaml"), base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Sources[0].Errors)
	}
	if len(result.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(result.Sources))
	}
	if result.Sources[0].ID != "SRC-001" {
		t.Fatalf("expected SRC-001, got %q", result.Sources[0].ID)
	}
}

func TestMissingOwner(t *testing.T) {
	result, err := CheckSources(testdataPath("missing-owner.yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertCheckError(t, result.Sources[0], "NO_OWNER")
}

func TestInvalidHash(t *testing.T) {
	result, err := CheckSources(testdataPath("invalid-hash.yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertCheckError(t, result.Sources[0], "INVALID_HASH")
}

func TestInvalidStatus(t *testing.T) {
	result, err := CheckSources(testdataPath("invalid-status.yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertCheckError(t, result.Sources[0], "INVALID_STATUS")
}

func TestNoAllowedUses(t *testing.T) {
	result, err := CheckSources(testdataPath("no-allowed-uses.yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertCheckError(t, result.Sources[0], "NO_ALLOWED_USES")
}

func TestMissingSourceFile(t *testing.T) {
	base := t.TempDir()
	result, err := CheckSources(testdataPath("missing-source-file.yaml"), base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertCheckError(t, result.Sources[0], "MISSING_SOURCE")
}

func TestMissingSourceFileSkippedWithoutBaseDir(t *testing.T) {
	result, err := CheckSources(testdataPath("missing-source-file.yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid when baseDir is empty (skip file check), got errors: %v", result.Sources[0].Errors)
	}
}

func TestMultipleErrors(t *testing.T) {
	result, err := CheckSources(testdataPath("multiple-errors.yaml"), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}

	src := result.Sources[0]
	assertCheckError(t, src, "INVALID_ID")
	assertCheckError(t, src, "MISSING_SOURCE")
	assertCheckError(t, src, "INVALID_HASH")
	assertCheckError(t, src, "NO_OWNER")
	assertCheckError(t, src, "INVALID_STATUS")
	assertCheckError(t, src, "INVALID_TYPE")
	assertCheckError(t, src, "INVALID_PRIORITY")
	assertCheckError(t, src, "INVALID_CONFIDENTIALITY")
	assertCheckError(t, src, "NO_ALLOWED_USES")
}

func TestCheckSourcesFromBytes(t *testing.T) {
	data := []byte(`
sources:
  - id: SRC-OK
    path: any/path.pdf
    type: pdf
    domain: test
    priority: primary
    status: active
    hash: "sha256:abc123"
    owner: someone@example.com
    license: internal
    confidentiality: public
    allowed_uses:
      - vector_index
`)
	result, err := CheckSourcesFromBytes(data, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Sources[0].Errors)
	}
}

func TestInvalidYAML(t *testing.T) {
	_, err := CheckSourcesFromBytes([]byte(`{not valid yaml`), "")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestNonexistentManifest(t *testing.T) {
	_, err := CheckSources("/nonexistent/manifest.yaml", "")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestOfficialExamplePasses(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	examplePath := filepath.Join(repoRoot, "examples", "insurance", "source-manifest.example.yaml")
	result, err := CheckSources(examplePath, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		for _, src := range result.Sources {
			for _, e := range src.Errors {
				t.Errorf("[%s] %s: %s", e.Code, e.SourceID, e.Message)
			}
		}
		t.Fatal("expected official example to pass checks")
	}
}

func assertCheckError(t *testing.T, sc SourceCheck, code string) {
	t.Helper()
	for _, e := range sc.Errors {
		if e.Code == code {
			return
		}
	}
	var codes []string
	for _, e := range sc.Errors {
		codes = append(codes, e.Code)
	}
	t.Fatalf("expected error code %s in [%s]", code, strings.Join(codes, ", "))
}

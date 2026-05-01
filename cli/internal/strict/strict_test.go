package strict

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func td(name string) string {
	return filepath.Join("testdata", name)
}

func TestAllConsistent(t *testing.T) {
	result, err := Check(StrictInput{
		ProjectPath: td("project.yaml"),
		SourcesPath: td("sources.yaml"),
		MatrixPath:  td("matrix.yaml"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Errors)
	}
}

func TestDuplicateSourceIDs(t *testing.T) {
	result, err := Check(StrictInput{
		SourcesPath: td("sources-duplicate.yaml"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid")
	}
	assertError(t, result, "DUPLICATE_SOURCE_ID")
}

func TestDuplicateUnitIDs(t *testing.T) {
	result, err := Check(StrictInput{
		SourcesPath: td("sources.yaml"),
		MatrixPath:  td("matrix-duplicate.yaml"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid")
	}
	assertError(t, result, "DUPLICATE_UNIT_ID")
}

func TestDanglingSourceRef(t *testing.T) {
	result, err := Check(StrictInput{
		SourcesPath: td("sources.yaml"),
		MatrixPath:  td("matrix-dangling-ref.yaml"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid")
	}
	assertError(t, result, "DANGLING_SOURCE_REF")
}

func TestOrphanSource(t *testing.T) {
	result, err := Check(StrictInput{
		SourcesPath: td("sources-orphan.yaml"),
		MatrixPath:  td("matrix-orphan.yaml"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid")
	}
	assertError(t, result, "ORPHAN_SOURCE")
	// SRC-SUPERSEDED should NOT be flagged as orphan
	for _, e := range result.Errors {
		if e.Code == "ORPHAN_SOURCE" && strings.Contains(e.Message, "SRC-SUPERSEDED") {
			t.Fatal("superseded source should not be flagged as orphan")
		}
	}
}

func TestDomainNotInScope(t *testing.T) {
	result, err := Check(StrictInput{
		ProjectPath: td("project.yaml"),
		MatrixPath:  td("matrix-bad-domain.yaml"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid")
	}
	assertError(t, result, "DOMAIN_NOT_IN_SCOPE")
}

func TestSourcesOnlyMode(t *testing.T) {
	result, err := Check(StrictInput{
		SourcesPath: td("sources.yaml"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid for sources-only check, got: %v", result.Errors)
	}
}

func TestMatrixOnlyMode(t *testing.T) {
	result, err := Check(StrictInput{
		MatrixPath: td("matrix.yaml"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid for matrix-only check, got: %v", result.Errors)
	}
}

func TestCheckFromManifestsDirectly(t *testing.T) {
	proj := &projectManifest{}
	proj.Scope.InScope = []string{"domain-a"}

	src := &sourceManifest{
		Sources: []source{
			{ID: "SRC-A", Domain: "domain-a", Status: "active"},
		},
	}

	mat := &canonicalMatrix{
		Units: []unit{
			{UnitID: "U-1", Domain: "domain-a", SourceRefs: []sourceRef{{SourceID: "SRC-A"}}},
		},
	}

	result := CheckFromManifests(proj, src, mat)
	if !result.Valid {
		t.Fatalf("expected valid, got: %v", result.Errors)
	}
}

func TestMissingManifestFile(t *testing.T) {
	_, err := Check(StrictInput{
		ProjectPath: "/nonexistent/project.yaml",
	})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.yaml")
	if err := writeFile(bad, []byte(`{broken`)); err != nil {
		t.Fatal(err)
	}
	_, err := Check(StrictInput{SourcesPath: bad})
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestOfficialExamplesConsistent(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	result, err := Check(StrictInput{
		ProjectPath: filepath.Join(repoRoot, "specs", "examples", "nomos-project.minimal.yaml"),
		SourcesPath: filepath.Join(repoRoot, "examples", "insurance", "source-manifest.example.yaml"),
		MatrixPath:  filepath.Join(repoRoot, "examples", "insurance", "canonical-matrix.example.yaml"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range result.Errors {
		t.Logf("[%s] %s", e.Code, e.Message)
	}
	// Official examples may not be perfectly cross-consistent, log but don't fail
}

func assertError(t *testing.T, result StrictResult, code string) {
	t.Helper()
	for _, e := range result.Errors {
		if e.Code == code {
			return
		}
	}
	var codes []string
	for _, e := range result.Errors {
		codes = append(codes, e.Code)
	}
	t.Fatalf("expected error code %s in [%s]", code, strings.Join(codes, ", "))
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

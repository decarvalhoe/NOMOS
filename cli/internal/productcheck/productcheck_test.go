package productcheck

import (
	"path/filepath"
	"strings"
	"testing"
)

func testdataPath(name string) string {
	return filepath.Join("testdata", name)
}

func TestValidProject(t *testing.T) {
	result, err := CheckProduct(testdataPath("valid-project.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Errors)
	}
}

func TestMissingProjectID(t *testing.T) {
	result, err := CheckProduct(testdataPath("missing-project-id.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertError(t, result, "MISSING_PROJECT_ID")
}

func TestInvalidProjectID(t *testing.T) {
	result, err := CheckProduct(testdataPath("invalid-project-id.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertError(t, result, "INVALID_PROJECT_ID")
}

func TestNoOwners(t *testing.T) {
	result, err := CheckProduct(testdataPath("no-owners.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertError(t, result, "NO_OWNERS")
}

func TestNoSurfaces(t *testing.T) {
	result, err := CheckProduct(testdataPath("no-surfaces.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertError(t, result, "NO_SURFACES")
}

func TestInvalidSurfaceType(t *testing.T) {
	result, err := CheckProduct(testdataPath("invalid-surface-type.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertError(t, result, "INVALID_SURFACE_TYPE")
}

func TestInvalidBlocker(t *testing.T) {
	result, err := CheckProduct(testdataPath("invalid-blocker.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	assertError(t, result, "MISSING_BLOCKER_ID")
	assertError(t, result, "INVALID_BLOCKER_SEVERITY")
	assertError(t, result, "MISSING_BLOCKER_DESCRIPTION")
}

func TestMultipleErrors(t *testing.T) {
	result, err := CheckProduct(testdataPath("multiple-errors.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}

	assertError(t, result, "MISSING_PROJECT_ID")
	assertError(t, result, "MISSING_PROJECT_NAME")
	assertError(t, result, "MISSING_DOMAIN")
	assertError(t, result, "INVALID_LIFECYCLE")
	assertError(t, result, "INVALID_RISK_LEVEL")
	assertError(t, result, "NO_OWNERS")
	assertError(t, result, "INVALID_SCOPE_VERDICT")
	assertError(t, result, "INVALID_CONFIDENCE")
	assertError(t, result, "NO_IN_SCOPE")
	assertError(t, result, "NO_SURFACES")
	assertError(t, result, "INVALID_DATA_SENSITIVITY")
	assertError(t, result, "INVALID_REPORT_TYPE")
	assertError(t, result, "INVALID_ATTESTATION_LEVEL")
}

func TestCheckProductFromBytes(t *testing.T) {
	data := []byte(`
project:
  id: ok-project
  name: OK
  domain: test
  lifecycle: brownfield
  risk_level: low
  owners:
    - name: Bob
scope:
  in_scope:
    - billing
surfaces:
  - name: web
    type: ui
`)
	result, err := CheckProductFromBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Errors)
	}
}

func TestInvalidYAML(t *testing.T) {
	_, err := CheckProductFromBytes([]byte(`{not valid`))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestNonexistentManifest(t *testing.T) {
	_, err := CheckProduct("/nonexistent/project.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestOfficialExamplesPassed(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	examples := []string{
		filepath.Join(repoRoot, "specs", "examples", "nomos-project.minimal.yaml"),
		filepath.Join(repoRoot, "specs", "examples", "nomos-project.brownfield.yaml"),
		filepath.Join(repoRoot, "specs", "examples", "nomos-project.regulated.yaml"),
	}

	for _, path := range examples {
		t.Run(filepath.Base(path), func(t *testing.T) {
			result, err := CheckProduct(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, e := range result.Errors {
				t.Errorf("[%s] %s: %s", e.Code, e.Path, e.Message)
			}
			if !result.Valid {
				t.Fatal("expected official example to pass checks")
			}
		})
	}
}

func assertError(t *testing.T, result CheckResult, code string) {
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

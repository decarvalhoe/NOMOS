package validate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfficialExamplesAreValid(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	paths := []string{
		filepath.Join(repoRoot, "specs", "examples", "adapter-manifest.node-typescript.yaml"),
		filepath.Join(repoRoot, "specs", "examples", "nomos-project.minimal.yaml"),
		filepath.Join(repoRoot, "specs", "examples", "nomos-project.brownfield.yaml"),
		filepath.Join(repoRoot, "specs", "examples", "nomos-project.regulated.yaml"),
		filepath.Join(repoRoot, "examples", "insurance", "source-manifest.example.yaml"),
		filepath.Join(repoRoot, "examples", "insurance", "canonical-matrix.example.yaml"),
	}

	result := ValidateFiles(paths)
	if !result.Valid {
		t.Fatalf("expected official examples to be valid, got %#v", result)
	}
	for _, file := range result.Files {
		if file.ManifestType == "" {
			t.Fatalf("expected manifest type for %s", file.Path)
		}
	}
}

func TestInvalidSourceManifestReturnsStructuredErrors(t *testing.T) {
	result := ValidateBytes("source.yaml", []byte(`
sources:
  - id: lower-case
    path: docs/source.pdf
    type: pdf
    domain: insurance
    priority: primary
    status: active
    owner: domain@example.com
    license: internal
    confidentiality: restricted
    allowed_uses:
      - vector_index
`))

	if result.Valid {
		t.Fatalf("expected invalid source manifest")
	}
	assertHasError(t, result.Errors, "sources[0].id", "pattern")
	assertHasError(t, result.Errors, "sources[0].hash", "required")
}

func TestUnknownFieldsAreDecodeErrors(t *testing.T) {
	result := ValidateBytes("project.yaml", []byte(`
project:
  id: demo
  name: Demo
  domain: internal
  lifecycle: greenfield
  risk_level: low
  owners:
    - name: Alice
scope:
  in_scope:
    - demo
surfaces:
  - name: api
    type: api
unexpected: true
`))

	if result.Valid {
		t.Fatalf("expected unknown field to fail")
	}
	assertHasError(t, result.Errors, "", "decode_error")
}

func TestInvalidAdapterManifestReturnsStructuredErrors(t *testing.T) {
	result := ValidateBytes("adapter.nomos.yaml", []byte(`
adapter:
  id: Bad_ID
  name: Demo Adapter
  version: nope
  status: stable
  owners: []
compatibility:
  nomos_core:
    min_version: "0.1.0"
  manifest_contract:
    version: "0.1.0"
stack_support:
  - language: typescript
    file_globs:
      - "src/**/*.ts"
    surfaces:
      - invalid_surface
capabilities:
  provides: []
test_contract:
  required_checks:
    - unknown-check
`))

	if result.Valid {
		t.Fatalf("expected invalid adapter manifest")
	}
	if result.ManifestType != "adapter-manifest" {
		t.Fatalf("expected adapter-manifest, got %q", result.ManifestType)
	}
	assertHasError(t, result.Errors, "adapter.id", "pattern")
	assertHasError(t, result.Errors, "adapter.version", "pattern")
	assertHasError(t, result.Errors, "adapter.status", "enum")
	assertHasError(t, result.Errors, "capabilities.provides", "required")
	assertHasError(t, result.Errors, "test_contract.required_checks[0]", "enum")
}

func TestCommandWritesJSONResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nomos.project.yaml")
	if err := os.WriteFile(path, []byte(`
project:
  id: demo
  name: Demo
  domain: internal
  lifecycle: greenfield
  risk_level: medium
  owners:
    - name: Alice
scope:
  in_scope:
    - eligibility
surfaces:
  - name: public-api
    type: api
`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Command([]string{"--format", "json", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}

	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON result: %v\n%s", err, stdout.String())
	}
	if !result.Valid || len(result.Files) != 1 || result.Files[0].ManifestType != "nomos-project" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestCommandHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Command([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "nomos validate") {
		t.Fatalf("expected validate usage, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func assertHasError(t *testing.T, errors []ValidationError, path string, code string) {
	t.Helper()
	for _, validationErr := range errors {
		if validationErr.Path == path && validationErr.Code == code {
			return
		}
	}
	var rendered []string
	for _, validationErr := range errors {
		rendered = append(rendered, validationErr.Path+":"+validationErr.Code)
	}
	t.Fatalf("expected %s:%s in %s", path, code, strings.Join(rendered, ", "))
}

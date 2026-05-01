package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// --- sources check ---

func TestSourcesCheckValid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := SourcesCheckCommand([]string{"../checks/testdata/valid-source-manifest.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Fatalf("expected ok output, got %q", stdout.String())
	}
}

func TestSourcesCheckInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := SourcesCheckCommand([]string{"../checks/testdata/missing-owner.yaml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "NO_OWNER") {
		t.Fatalf("expected NO_OWNER, got %q", stdout.String())
	}
}

func TestSourcesCheckJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := SourcesCheckCommand([]string{"--format", "json", "../checks/testdata/valid-source-manifest.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result["valid"] != true {
		t.Fatalf("expected valid=true, got %v", result["valid"])
	}
}

func TestSourcesCheckNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := SourcesCheckCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

func TestSourcesCheckMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := SourcesCheckCommand([]string{"/nonexistent.yaml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
}

// --- contracts check ---

func TestContractsCheckValid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ContractsCheckCommand([]string{"../checks/contracts/testdata/valid-matrix.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Fatalf("expected ok, got %q", stdout.String())
	}
}

func TestContractsCheckInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ContractsCheckCommand([]string{"../checks/contracts/testdata/no-contract.yaml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "NO_CONTRACT") {
		t.Fatalf("expected NO_CONTRACT, got %q", stdout.String())
	}
}

func TestContractsCheckJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ContractsCheckCommand([]string{"--format", "json", "../checks/contracts/testdata/valid-matrix.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
}

func TestContractsCheckNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ContractsCheckCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

// --- product-check ---

func TestProductCheckValid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ProductCheckCommand([]string{"../productcheck/testdata/valid-project.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Fatalf("expected ok, got %q", stdout.String())
	}
}

func TestProductCheckInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ProductCheckCommand([]string{"../productcheck/testdata/no-owners.yaml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "NO_OWNERS") {
		t.Fatalf("expected NO_OWNERS, got %q", stdout.String())
	}
}

func TestProductCheckJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ProductCheckCommand([]string{"--format", "json", "../productcheck/testdata/valid-project.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
}

func TestProductCheckNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ProductCheckCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

// --- matrix check ---

func TestMatrixCheckValid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := MatrixCheckCommand([]string{"../checks/testdata/matrix-valid.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Fatalf("expected ok, got %q", stdout.String())
	}
}

func TestMatrixCheckInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := MatrixCheckCommand([]string{"../checks/testdata/matrix-broken-contract.yaml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "FAILED") {
		t.Fatalf("expected FAILED, got %q", stdout.String())
	}
}

func TestMatrixCheckNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := MatrixCheckCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

// --- strict ---

func TestStrictValid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictCommand([]string{
		"--project", "../strict/testdata/project.yaml",
		"--sources", "../strict/testdata/sources.yaml",
		"--matrix", "../strict/testdata/matrix.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Fatalf("expected ok, got %q", stdout.String())
	}
}

func TestStrictDanglingRef(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictCommand([]string{
		"--sources", "../strict/testdata/sources.yaml",
		"--matrix", "../strict/testdata/matrix-dangling-ref.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "DANGLING_SOURCE_REF") {
		t.Fatalf("expected DANGLING_SOURCE_REF, got %q", stdout.String())
	}
}

func TestStrictJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictCommand([]string{
		"--format", "json",
		"--sources", "../strict/testdata/sources.yaml",
		"--matrix", "../strict/testdata/matrix.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
}

func TestStrictNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

// --- exceptions check ---

func TestExceptionsCheckValid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ExceptionsCheckCommand([]string{"../exceptions/testdata/valid.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Fatalf("expected ok, got %q", stdout.String())
	}
}

func TestExceptionsCheckInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ExceptionsCheckCommand([]string{"../exceptions/testdata/no-owner.yaml"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "NO_OWNER") {
		t.Fatalf("expected NO_OWNER, got %q", stdout.String())
	}
}

func TestExceptionsCheckJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ExceptionsCheckCommand([]string{"--format", "json", "../exceptions/testdata/valid.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
}

func TestExceptionsCheckNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ExceptionsCheckCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestStrictGateAllGreen(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--project", "testdata/gate-project.yaml",
		"--sources", "testdata/gate-sources.yaml",
		"--matrix", "testdata/gate-matrix.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "PASS") {
		t.Fatalf("expected PASS, got %q", stdout.String())
	}
}

func TestStrictGateAllGreenWithExceptions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--project", "testdata/gate-project.yaml",
		"--sources", "testdata/gate-sources.yaml",
		"--matrix", "testdata/gate-matrix.yaml",
		"--exceptions", "testdata/gate-exceptions.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "PASS") {
		t.Fatalf("expected PASS, got %q", stdout.String())
	}
}

func TestStrictGateSourcesInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--sources", "../checks/testdata/missing-owner.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "FAIL") {
		t.Fatalf("expected FAIL, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "NO_OWNER") {
		t.Fatalf("expected NO_OWNER, got %q", stdout.String())
	}
}

func TestStrictGateCrossCheckDangling(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--sources", "testdata/gate-sources.yaml",
		"--matrix", "../strict/testdata/matrix-dangling-ref.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "DANGLING_SOURCE_REF") {
		t.Fatalf("expected DANGLING_SOURCE_REF, got %q", stdout.String())
	}
}

func TestStrictGateProductInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--project", "../productcheck/testdata/no-owners.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "NO_OWNERS") {
		t.Fatalf("expected NO_OWNERS, got %q", stdout.String())
	}
}

func TestStrictGateExceptionsInvalid(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--sources", "testdata/gate-sources.yaml",
		"--exceptions", "../exceptions/testdata/no-owner.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "NO_OWNER") {
		t.Fatalf("expected NO_OWNER, got %q", stdout.String())
	}
}

func TestStrictGateJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--project", "testdata/gate-project.yaml",
		"--sources", "testdata/gate-sources.yaml",
		"--matrix", "testdata/gate-matrix.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v\n%s", err, stdout.String())
	}
	if !result.Valid {
		t.Fatalf("expected valid=true")
	}
	if len(result.Sections) == 0 {
		t.Fatal("expected sections")
	}
}

func TestStrictGateJSONFailed(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--sources", "../checks/testdata/missing-owner.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result.Valid {
		t.Fatal("expected valid=false")
	}
}

func TestStrictGateNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

func TestStrictGateMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--project", "/nonexistent.yaml",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
}

func TestStrictGateSectionsPresent(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--project", "testdata/gate-project.yaml",
		"--sources", "testdata/gate-sources.yaml",
		"--matrix", "testdata/gate-matrix.yaml",
		"--exceptions", "testdata/gate-exceptions.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stdout=%q", code, stdout.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}

	names := make(map[string]bool)
	for _, s := range result.Sections {
		names[s.Name] = true
	}
	for _, expected := range []string{"product-check", "sources-check", "matrix-check", "contracts-check", "cross-check", "exceptions-check"} {
		if !names[expected] {
			t.Fatalf("expected section %q, got %v", expected, names)
		}
	}
}

func TestStrictGateProjectOnlyRuns(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := StrictGateCommand([]string{
		"--format", "json",
		"--project", "testdata/gate-project.yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stdout=%q", code, stdout.String())
	}
	var result GateResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if len(result.Sections) != 2 {
		t.Fatalf("expected 2 sections for project-only, got %d", len(result.Sections))
	}
}

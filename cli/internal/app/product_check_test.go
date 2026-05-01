package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/productcheck"
)

func TestProductCheckValidManifest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := ProductCheckCommand(
		[]string{"../productcheck/testdata/valid-project.yaml"},
		&stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok") {
		t.Fatalf("expected ok output, got %q", stdout.String())
	}
}

func TestProductCheckInvalidManifest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := ProductCheckCommand(
		[]string{"../productcheck/testdata/missing-project-id.yaml"},
		&stdout, &stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "MISSING_PROJECT_ID") {
		t.Fatalf("expected MISSING_PROJECT_ID in output, got %q", stdout.String())
	}
}

func TestProductCheckJSONFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := ProductCheckCommand(
		[]string{"--format", "json", "../productcheck/testdata/valid-project.yaml"},
		&stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}

	var result productcheck.CheckResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode json: %v\n%s", err, stdout.String())
	}
	if !result.Valid {
		t.Fatalf("expected valid result, got errors: %v", result.Errors)
	}
}

func TestProductCheckJSONInvalid(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := ProductCheckCommand(
		[]string{"--format", "json", "../productcheck/testdata/multiple-errors.yaml"},
		&stdout, &stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}

	var result productcheck.CheckResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode json: %v\n%s", err, stdout.String())
	}
	if result.Valid {
		t.Fatal("expected invalid result")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected errors in result")
	}
}

func TestProductCheckNoArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := ProductCheckCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "manifest path is required") {
		t.Fatalf("expected usage error, got %q", stderr.String())
	}
}

func TestProductCheckMissingFile(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := ProductCheckCommand(
		[]string{"/nonexistent/project.yaml"},
		&stdout, &stderr,
	)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "product-check:") {
		t.Fatalf("expected error message, got %q", stderr.String())
	}
}

func TestProductCheckUnknownOption(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := ProductCheckCommand(
		[]string{"--unknown", "file.yaml"},
		&stdout, &stderr,
	)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestProductCheckUnsupportedFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := ProductCheckCommand(
		[]string{"--format", "xml", "file.yaml"},
		&stdout, &stderr,
	)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unsupported format") {
		t.Fatalf("expected format error, got %q", stderr.String())
	}
}

func TestProductCheckMultipleManifests(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := ProductCheckCommand(
		[]string{"file1.yaml", "file2.yaml"},
		&stdout, &stderr,
	)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "only one manifest") {
		t.Fatalf("expected single manifest error, got %q", stderr.String())
	}
}

func TestProductCheckFormatEquals(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := ProductCheckCommand(
		[]string{"--format=json", "../productcheck/testdata/valid-project.yaml"},
		&stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}

	var result productcheck.CheckResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode json: %v\n%s", err, stdout.String())
	}
	if !result.Valid {
		t.Fatalf("expected valid result")
	}
}

func TestProductCheckOfficialExamples(t *testing.T) {
	examples := []string{
		"../../../specs/examples/nomos-project.minimal.yaml",
		"../../../specs/examples/nomos-project.brownfield.yaml",
		"../../../specs/examples/nomos-project.regulated.yaml",
	}

	for _, path := range examples {
		t.Run(path, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := ProductCheckCommand([]string{path}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("expected exit code 0 for %s, got %d; out=%q err=%q",
					path, code, stdout.String(), stderr.String())
			}
		})
	}
}

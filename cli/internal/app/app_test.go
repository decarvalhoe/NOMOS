package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/output"
)

func TestRunHelpByDefault(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Available commands:") {
		t.Fatalf("expected help output, got %q", stdout.String())
	}
}

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
	if strings.TrimSpace(stdout.String()) != Version {
		t.Fatalf("expected version %q, got %q", Version, stdout.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"unknown"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 after help fallback, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("expected unknown command error, got %q", stderr.String())
	}
}

func TestRunScaffoldedCommand(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"validate"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not implemented yet") {
		t.Fatalf("expected not implemented message, got %q", stderr.String())
	}
}

func TestRunDiagnoseJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"diagnose", "--root", "../diagnose/testdata/corpus/nomos-ready", "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}

	var report output.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode diagnose json: %v\n%s", err, stdout.String())
	}
	if report.Run.Mode != "admission" {
		t.Fatalf("expected admission report, got %q", report.Run.Mode)
	}
	if report.Verdict.Status != "pass" {
		t.Fatalf("expected pass verdict, got %#v", report.Verdict)
	}
}

func TestRunDiagnoseMarkdown(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"diagnose", "--format", "markdown", "../diagnose/testdata/corpus/docs-only"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Preliminary verdict out_of_scope") {
		t.Fatalf("expected markdown diagnose verdict, got %q", stdout.String())
	}
}

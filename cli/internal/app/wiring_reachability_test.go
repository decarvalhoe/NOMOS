package app

// VRC-09 (#555): ten *Command functions were implemented and tested but never
// registered in the command map nor called from production code — the exact
// class PR #543 fixed for `atomize`, caught here by the wiring matrix
// (scripts/vrc_wiring_matrix.py). These tests pin REACHABILITY through Run():
// they fail if any of these commands is dropped from the command map again.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCheckSubcommandsAreReachable(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"sources", []string{"check", "sources", "../checks/testdata/valid-source-manifest.yaml"}},
		{"contracts", []string{"check", "contracts", "../checks/contracts/testdata/valid-matrix.yaml"}},
		{"matrix", []string{"check", "matrix", "../checks/testdata/matrix-valid.yaml"}},
		{"exceptions", []string{"check", "exceptions", "../exceptions/testdata/valid.yaml"}},
		{"strict", []string{
			"check", "strict",
			"--project", "../strict/testdata/project.yaml",
			"--sources", "../strict/testdata/sources.yaml",
			"--matrix", "../strict/testdata/matrix.yaml",
		}},
	}
	for _, tc := range cases {
		var stdout, stderr bytes.Buffer
		if code := Run(tc.args, &stdout, &stderr); code != 0 {
			t.Fatalf("check %s: expected 0, got %d; stderr=%q", tc.name, code, stderr.String())
		}
	}
}

func TestRunCheckUnknownSubcommandFails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"check", "nope"}, &stdout, &stderr); code != 2 {
		t.Fatalf("expected 2 for unknown check subcommand, got %d", code)
	}
}

func TestRunReportIsReachable(t *testing.T) {
	out := filepath.Join(t.TempDir(), "report.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"report", "--root", ".", "--project-id", "vrc", "--output", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("report: expected 0, got %d; stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("report output missing: %v", err)
	}
}

func TestRunExportSubcommandsAreReachable(t *testing.T) {
	for _, sub := range []string{"spdx", "cyclonedx"} {
		out := filepath.Join(t.TempDir(), sub+".json")
		var stdout, stderr bytes.Buffer
		code := Run([]string{"export", sub, "--root", ".", "--project-id", "vrc", "--output", out}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("export %s: expected 0, got %d; stderr=%q", sub, code, stderr.String())
		}
		if _, err := os.Stat(out); err != nil {
			t.Fatalf("export %s output missing: %v", sub, err)
		}
	}
}

func TestRunExportUnknownSubcommandFails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"export", "nope"}, &stdout, &stderr); code != 2 {
		t.Fatalf("expected 2 for unknown export subcommand, got %d", code)
	}
}

func TestRunProductCheckIsReachable(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"product-check", "../productcheck/testdata/valid-project.yaml"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("product-check: expected 0, got %d; stderr=%q", code, stderr.String())
	}
}

func TestRunAttestCreateIsReachable(t *testing.T) {
	// One-shot create+sign with an ephemeral key: the envelope and the
	// verifying public key must both be written.
	dir := t.TempDir()
	subject := filepath.Join(dir, "artifact.json")
	if err := os.WriteFile(subject, []byte(`{"k":"v"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "envelope.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"attest", "create", "--project-id", "vrc", "--verdict", "pass", "--subject", subject, "--output", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("attest create: expected 0, got %d; stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("attest create envelope missing: %v", err)
	}
	if _, err := os.Stat(out + ".pub.pem"); err != nil {
		t.Fatalf("attest create public key missing (signature would be unverifiable): %v", err)
	}
}

func TestHelpAdvertisesRegisteredCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := Run(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("help: expected 0, got %d", code)
	}
	for _, name := range []string{"  check", "  report", "  export", "  product-check"} {
		if !strings.Contains(stdout.String(), name) {
			t.Fatalf("help text does not advertise %q", strings.TrimSpace(name))
		}
	}
}

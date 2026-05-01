package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- report ---

func TestReportCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ReportCommand([]string{
		"--root", "../detect/testdata/corpus/fullstack",
		"--project-id", "test-project",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result["report_type"] != "nomos-report" {
		t.Fatalf("expected nomos-report, got %v", result["report_type"])
	}
}

func TestReportCommandToFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "report.json")
	var stdout, stderr bytes.Buffer
	code := ReportCommand([]string{
		"--root", "../detect/testdata/corpus/fullstack",
		"--output", out,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), "nomos-report") {
		t.Fatalf("expected nomos-report in file, got %q", string(data)[:100])
	}
}

func TestReportCommandBadRoot(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ReportCommand([]string{"--root", "/nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
}

// --- export spdx ---

func TestExportSPDXCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ExportSPDXCommand([]string{
		"--root", "../detect/testdata/corpus/fullstack",
		"--project-id", "test",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result["spdxVersion"] != "SPDX-2.3" {
		t.Fatalf("expected SPDX-2.3, got %v", result["spdxVersion"])
	}
}

func TestExportSPDXToFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "sbom.spdx.json")
	var stdout, stderr bytes.Buffer
	code := ExportSPDXCommand([]string{
		"--root", "../detect/testdata/corpus/fullstack",
		"--output", out,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), "SPDX-2.3") {
		t.Fatalf("expected SPDX content")
	}
}

func TestExportSPDXBadRoot(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ExportSPDXCommand([]string{"--root", "/nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
}

// --- export cyclonedx ---

func TestExportCycloneDXCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ExportCycloneDXCommand([]string{
		"--root", "../detect/testdata/corpus/fullstack",
		"--project-id", "test",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result["bomFormat"] != "CycloneDX" {
		t.Fatalf("expected CycloneDX, got %v", result["bomFormat"])
	}
}

func TestExportCycloneDXToFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "sbom.cdx.json")
	var stdout, stderr bytes.Buffer
	code := ExportCycloneDXCommand([]string{
		"--root", "../detect/testdata/corpus/fullstack",
		"--output", out,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), "CycloneDX") {
		t.Fatalf("expected CycloneDX content")
	}
}

// --- attest ---

func TestAttestCommand(t *testing.T) {
	// Create a temp subject file
	dir := t.TempDir()
	subject := filepath.Join(dir, "artifact.txt")
	if err := os.WriteFile(subject, []byte("test artifact"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := AttestCommand([]string{
		"--project-id", "test-project",
		"--verdict", "pass",
		"--subject", subject,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON: %v", err)
	}
	if result["payloadType"] == nil {
		t.Fatal("expected payloadType in cosign envelope")
	}
}

func TestAttestCommandToFile(t *testing.T) {
	dir := t.TempDir()
	subject := filepath.Join(dir, "artifact.txt")
	if err := os.WriteFile(subject, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "attestation.json")

	var stdout, stderr bytes.Buffer
	code := AttestCommand([]string{
		"--project-id", "test",
		"--verdict", "pass",
		"--subject", subject,
		"--output", out,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected 0, got %d; stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), "payloadType") {
		t.Fatalf("expected attestation content")
	}
}

func TestAttestCommandNoProjectID(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := AttestCommand([]string{"--verdict", "pass"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

func TestAttestCommandNoVerdict(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := AttestCommand([]string{"--project-id", "test"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
}

func TestAttestCommandNoSubject(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := AttestCommand([]string{
		"--project-id", "test",
		"--verdict", "pass",
	}, &stdout, &stderr)
	// No subject → attestation.GenerateStatement will error (needs at least one subject)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
}

func TestAttestCommandBadSubjectPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := AttestCommand([]string{
		"--project-id", "test",
		"--verdict", "pass",
		"--subject", "/nonexistent",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
}

//go:build generate

package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateReleaseArtifacts(t *testing.T) {
	repoRoot := filepath.Clean(filepath.Join("..", "..", ".."))
	outDir := filepath.Join(repoRoot, "reports")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var devNull devNullWriter

	// 1. Report
	code := ReportCommand([]string{
		"--root", repoRoot,
		"--project-id", "nomos",
		"--project-name", "Nomos",
		"--domain", "product-intelligence",
		"--risk-level", "medium",
		"--output", filepath.Join(outDir, "nomos-report.json"),
	}, &devNull, os.Stderr)
	if code != 0 {
		t.Fatalf("report failed: %d", code)
	}

	// 2. SPDX
	code = ExportSPDXCommand([]string{
		"--root", repoRoot,
		"--project-id", "nomos",
		"--output", filepath.Join(outDir, "nomos-spdx.json"),
	}, &devNull, os.Stderr)
	if code != 0 {
		t.Fatalf("spdx failed: %d", code)
	}

	// 3. CycloneDX
	code = ExportCycloneDXCommand([]string{
		"--root", repoRoot,
		"--project-id", "nomos",
		"--output", filepath.Join(outDir, "nomos-cyclonedx.json"),
	}, &devNull, os.Stderr)
	if code != 0 {
		t.Fatalf("cyclonedx failed: %d", code)
	}

	// 4. Attestation (subject = the report)
	code = AttestCommand([]string{
		"--project-id", "nomos",
		"--verdict", "pass",
		"--subject", filepath.Join(outDir, "nomos-report.json"),
		"--key-id", "nomos-dev",
		"--output", filepath.Join(outDir, "nomos-attestation.json"),
	}, &devNull, os.Stderr)
	if code != 0 {
		t.Fatalf("attestation failed: %d", code)
	}
}

type devNullWriter struct{}

func (devNullWriter) Write(p []byte) (int, error) { return len(p), nil }

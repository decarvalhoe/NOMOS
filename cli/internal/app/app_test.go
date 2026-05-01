package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
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

	code := Run([]string{"diagnose"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not implemented yet") {
		t.Fatalf("expected not implemented message, got %q", stderr.String())
	}
}

func TestRunInitMinimalCreatesBaselineProject(t *testing.T) {
	target := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"init",
		"--mode", "minimal",
		"--project-id", "demo-greenfield",
		"--project-name", "Demo Greenfield",
		"--domain", "internal-ops",
		"--owner-name", "Alice Example",
		"--owner-email", "alice@example.com",
		target,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", code, stderr.String())
	}

	manifest := readTestFile(t, target, "nomos.project.yaml")
	for _, expected := range []string{
		"id: demo-greenfield",
		`name: "Demo Greenfield"`,
		`domain: "internal-ops"`,
		"risk_level: medium",
		"regulated: false",
		"attestation_level: none",
	} {
		if !strings.Contains(manifest, expected) {
			t.Fatalf("expected manifest to contain %q:\n%s", expected, manifest)
		}
	}

	assertPathExists(t, target, "docs/canonical/source-manifest.yaml")
	assertPathExists(t, target, "docs/canonical/internal-ops-matrix.yaml")
	assertPathExists(t, target, "docs/governance/domain-risk-profile.md")
	assertPathExists(t, target, "data/canonical/.gitkeep")
	assertPathExists(t, target, "src/.gitkeep")
	assertPathExists(t, target, "tests/golden/.gitkeep")
}

func TestRunInitRegulatedCreatesStrictBaselineProject(t *testing.T) {
	target := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{
		"init",
		"--mode", "regulated",
		"--project-id", "regulated-benefits-core",
		"--project-name", "Regulated Benefits Core",
		"--domain", "public-benefits",
		target,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", code, stderr.String())
	}

	manifest := readTestFile(t, target, "nomos.project.yaml")
	for _, expected := range []string{
		"risk_level: critical",
		"regulated: true",
		"data_sensitivity: secret",
		"attestation_level: signed",
		"- sbom",
		"- provenance",
		"name: canonical-data",
	} {
		if !strings.Contains(manifest, expected) {
			t.Fatalf("expected regulated manifest to contain %q:\n%s", expected, manifest)
		}
	}

	assertPathExists(t, target, "docs/compliance/evidence-policy.md")
	assertPathExists(t, target, "src/api/.gitkeep")
}

func TestRunInitRejectsUnknownMode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"init", "--mode", "brownfield", t.TempDir()}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unsupported mode") {
		t.Fatalf("expected unsupported mode error, got %q", stderr.String())
	}
}

func TestRunInitRejectsNonEmptyWorkspace(t *testing.T) {
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "README.md"), []byte("# existing\n"), 0o644); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"init", target}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "not empty") {
		t.Fatalf("expected non-empty workspace error, got %q", stderr.String())
	}
}

func TestRunInitRejectsInvalidOwnerEmail(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"init", "--owner-email", "invalid", t.TempDir()}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "valid email") {
		t.Fatalf("expected email validation error, got %q", stderr.String())
	}
}

func TestRunInitAllowsGitOnlyWorkspace(t *testing.T) {
	target := t.TempDir()
	if err := os.Mkdir(filepath.Join(target, ".git"), 0o755); err != nil {
		t.Fatalf("create .git directory: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"init", target}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d: %s", code, stderr.String())
	}
	assertPathExists(t, target, ".git")
	assertPathExists(t, target, "nomos.project.yaml")
}

func readTestFile(t *testing.T, root string, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

func assertPathExists(t *testing.T, root string, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("expected %s to exist: %v", rel, err)
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
func TestRunValidateRequiresManifestPath(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run([]string{"validate"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "at least one manifest path is required") {
		t.Fatalf("expected usage error, got %q", stderr.String())
	}
}

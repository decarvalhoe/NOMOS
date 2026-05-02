package app

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
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
	if code != 2 {
		t.Fatalf("expected exit code 2 for unknown command, got %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("expected unknown command error, got %q", stderr.String())
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

func readFileAbs(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func gitStatusText(t *testing.T, root string) string {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "status", "--porcelain", "-u")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git status failed: %v\n%s", err, out)
	}
	return string(out)
}

func assertPathExists(t *testing.T, root string, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
		t.Fatalf("expected %s to exist: %v", rel, err)
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

func TestRunDiagnoseCanonicalCorpusMode(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "nomos.project.yaml", `schema_version: "0.1.0"
mode: canonical_corpus
project:
  id: rbok-corpus
  name: RBOK Corpus
  domain: rbok
source_inventory:
  manifest_path: source-manifest.yaml
  hash_required: true
  owner_required: true
  confidentiality_required: true
corpus_policy:
  execution: read_only
`)
	writeTestFile(t, root, "source-manifest.yaml", `schema_version: "0.1.0"
sources:
  - id: RBOK-RULE
    path: 01_rbok/rule.md
    type: markdown
    domain: rbok
    priority: primary
    status: active
    hash: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    owner: domain-owner@example.com
    license: internal
    confidentiality: internal
    allowed_uses:
      - structured_contract
`)
	writeTestFile(t, root, "01_rbok/rule.md", "# Rule\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"diagnose", "--mode", "canonical_corpus", "--root", root, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}

	var report output.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode diagnose json: %v\n%s", err, stdout.String())
	}
	raw := report.Metadata["diagnose"].(map[string]any)
	if raw["preliminary_verdict"] != "corpus_admissible" {
		t.Fatalf("expected corpus_admissible, got %#v", raw)
	}
	if !strings.Contains(report.Verdict.Summary, "corpus_admissible") {
		t.Fatalf("expected corpus verdict summary, got %#v", report.Verdict)
	}
}

func TestRunDiagnoseCanonicalCorpusSidecarManifest(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "01_rbok/rule.md", "# Rule\n")
	sidecar := t.TempDir()
	projectManifest := filepath.Join(sidecar, "nomos.project.yaml")
	sourceManifest := filepath.Join(sidecar, "source-manifest.yaml")
	if err := os.WriteFile(projectManifest, []byte(`schema_version: "0.1.0"
mode: canonical_corpus
project:
  id: rbok-corpus
  name: RBOK Corpus
  domain: rbok
`), 0o644); err != nil {
		t.Fatalf("write project sidecar: %v", err)
	}
	if err := os.WriteFile(sourceManifest, []byte(`schema_version: "0.1.0"
sources:
  - id: RBOK-RULE
    path: 01_rbok/rule.md
    type: markdown
    domain: rbok
    priority: primary
    status: active
    hash: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    owner: domain-owner@example.com
    license: internal
    confidentiality: internal
    allowed_uses:
      - structured_contract
`), 0o644); err != nil {
		t.Fatalf("write source sidecar: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"diagnose",
		"--root", root,
		"--project-manifest", projectManifest,
		"--source-manifest", sourceManifest,
		"--format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}

	var report output.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode diagnose json: %v\n%s", err, stdout.String())
	}
	raw := report.Metadata["diagnose"].(map[string]any)
	if raw["preliminary_verdict"] != "corpus_admissible" {
		t.Fatalf("expected corpus_admissible from sidecar mode, got %#v", raw)
	}
	if report.Project.ManifestPath != projectManifest {
		t.Fatalf("expected project manifest sidecar path, got %q", report.Project.ManifestPath)
	}
}

func TestRunCorpusScanWritesSnapshotOutsideSourceRoot(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "01_rbok/rule.md", "# Rule\n")
	writeTestFile(t, root, "03_catalogue_services/service.yaml", "id: service\n")
	initGitRepo(t, root)

	out := filepath.Join(t.TempDir(), "snapshot.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{
		"corpus", "scan",
		"--root", root,
		"--out", out,
		"--format", "json",
		"--ext", ".md",
		"--ext", ".yaml",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d; stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "scanned 2 files") {
		t.Fatalf("expected scan summary, got %q", stdout.String())
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var snapshot corpus.Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v\n%s", err, string(data))
	}
	if snapshot.TotalFiles != 2 {
		t.Fatalf("expected 2 files, got %#v", snapshot)
	}
	if snapshot.Commit == "" {
		t.Fatalf("expected git commit metadata in snapshot: %#v", snapshot)
	}
	for _, source := range snapshot.Sources {
		if source.Hash == "" || source.Extension == "" || source.Classification == "" {
			t.Fatalf("expected hash, extension, classification for source: %#v", source)
		}
	}
}

func TestRunCorpusScanRejectsOutputInsideSourceRoot(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "01_rbok/rule.md", "# Rule\n")
	initGitRepo(t, root)

	out := filepath.Join(root, "snapshot.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"corpus", "scan", "--root", root, "--out", out}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "inside source root") {
		t.Fatalf("expected source-root guard error, got %q", stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("expected no output file inside source root, stat err=%v", err)
	}
}

func TestRunCorpusManifestAndValidateSidecar(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "01_rbok/rule.md", "# Rule\n")
	initGitRepo(t, root)

	outDir := t.TempDir()
	snapshotPath := filepath.Join(outDir, "snapshot.json")
	manifestPath := filepath.Join(outDir, "source-manifest.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"corpus", "scan", "--root", root, "--out", snapshotPath, "--ext", ".md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("scan failed: code=%d stderr=%q", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"corpus", "manifest",
		"--snapshot", snapshotPath,
		"--out", manifestPath,
		"--domain", "rbok",
		"--owner", "domain-owner@example.com",
		"--confidentiality", "internal",
		"--id-prefix", "RBOK",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("manifest failed: code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(readTestFile(t, outDir, "source-manifest.yaml"), "RBOK-RULE") {
		t.Fatalf("expected generated manifest to contain RBOK-RULE")
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"corpus", "validate-sidecar", "--root", root, "--manifest", manifestPath, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("validate-sidecar failed: code=%d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"valid": true`) {
		t.Fatalf("expected valid sidecar json, got %s", stdout.String())
	}
}

func TestRunCorpusFeedBuildsRealBundleWithoutMutatingCorpus(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "01_rbok/rule.md", "# Rule\nBody\n")
	initGitRepo(t, root)

	outDir := t.TempDir()
	snapshotPath := filepath.Join(outDir, "snapshot.json")
	manifestPath := filepath.Join(outDir, "source-manifest.yaml")
	lockfilePath := filepath.Join(outDir, "corpus.lock.json")
	feedPath := filepath.Join(outDir, "rbok-feed.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run([]string{"corpus", "scan", "--root", root, "--out", snapshotPath, "--ext", ".md"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("scan failed: code=%d stderr=%q", code, stderr.String())
	}
	var snapshot corpus.Snapshot
	if err := json.Unmarshal([]byte(readFileAbs(t, snapshotPath)), &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	lf := corpus.NewLockfile()
	for _, source := range snapshot.Sources {
		if err := lf.Add(source.Path, source.Hash, "alice", "accepted"); err != nil {
			t.Fatalf("lock add: %v", err)
		}
	}
	if err := lf.Write(lockfilePath); err != nil {
		t.Fatalf("write lockfile: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"corpus", "manifest",
		"--snapshot", snapshotPath,
		"--out", manifestPath,
		"--domain", "rbok",
		"--owner", "domain-owner@example.com",
		"--id-prefix", "RBOK",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("manifest failed: code=%d stderr=%q", code, stderr.String())
	}

	before := gitStatusText(t, root)
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{
		"corpus", "feed",
		"--root", root,
		"--manifest", manifestPath,
		"--snapshot", snapshotPath,
		"--lockfile", lockfilePath,
		"--corpus-id", "rbok",
		"--project-id", "nomos-rbok",
		"--out", feedPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("feed failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	after := gitStatusText(t, root)
	if before != after {
		t.Fatalf("corpus mutated: before=%q after=%q", before, after)
	}

	raw := readFileAbs(t, feedPath)
	for _, expected := range []string{`"unit_count": 1`, `"corpus_index"`, `"rag_metadata"`, `"attestation"`, `"lockfile"`, `"accepted": true`} {
		if !strings.Contains(raw, expected) {
			t.Fatalf("expected feed to contain %s, got %s", expected, raw)
		}
	}
}

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

func writeTestFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func initGitRepo(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "nomos@example.com")
	runGit(t, root, "config", "user.name", "Nomos Test")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "seed corpus")
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

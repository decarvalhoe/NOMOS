package detect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectFullStackCorpus(t *testing.T) {
	report, err := Detect(filepath.Join("testdata", "corpus", "fullstack"))
	if err != nil {
		t.Fatalf("detect fullstack corpus: %v", err)
	}

	assertHasLanguage(t, report, "Go")
	assertHasLanguage(t, report, "TypeScript")
	assertHasLanguage(t, report, "Python")
	assertHasTreeSitterLanguage(t, report, "Go")
	assertHasTreeSitterLanguage(t, report, "Java")
	assertHasTreeSitterLanguage(t, report, "Python")
	assertHasTreeSitterLanguage(t, report, "TSX")
	assertHasTool(t, report, "Go modules")
	assertHasTool(t, report, "React")
	assertHasTool(t, report, "GitHub Actions")
	assertHasCI(t, report, "GitHub Actions")
	if report.TreeSitter.ParsedFiles == 0 {
		t.Fatalf("expected Tree-sitter parsed files in %#v", report.TreeSitter)
	}
	if len(report.TreeSitter.MissingGrammars) != 0 {
		t.Fatalf("did not expect missing grammars: %#v", report.TreeSitter.MissingGrammars)
	}

	for _, surface := range []string{"api", "ui", "worker", "data", "infra", "docs"} {
		assertHasSurface(t, report, surface)
	}
}

func TestDetectNodeTypeScriptOfficialFixture(t *testing.T) {
	report, err := Detect(filepath.Join("..", "..", "..", "adapters", "node-typescript", "fixtures", "nextjs-api-ui"))
	if err != nil {
		t.Fatalf("detect Node/TypeScript fixture: %v", err)
	}

	assertHasLanguage(t, report, "TypeScript")
	assertHasTreeSitterLanguage(t, report, "TypeScript")
	assertHasTreeSitterLanguage(t, report, "TSX")
	if len(report.TreeSitter.ParseErrors) != 0 {
		t.Fatalf("expected clean Tree-sitter parse for official fixture, got %#v", report.TreeSitter.ParseErrors)
	}
	assertHasSurface(t, report, "api")
	assertHasSurface(t, report, "ui")
	assertHasSurface(t, report, "data")

	assertHasNodeTypeScriptFinding(t, report, "language_detection", "Node package manifest", "")
	assertHasNodeTypeScriptFinding(t, report, "dependency_detection", "Next.js", "ui")
	assertHasNodeTypeScriptFinding(t, report, "route_detection", "Next.js API route", "api")
	assertHasNodeTypeScriptFinding(t, report, "route_detection", "Next.js UI route", "ui")
	assertHasNodeTypeScriptFinding(t, report, "route_detection", "Node route handler", "api")
	assertHasNodeTypeScriptFinding(t, report, "service_detection", "Service module", "")
	assertHasNodeTypeScriptFinding(t, report, "mock_detection", "Test mock", "")
	assertHasNodeTypeScriptFinding(t, report, "fixture_detection", "Test fixture", "")
	assertHasNodeTypeScriptFinding(t, report, "hardcoded_catalog_detection", "Hardcoded catalogue", "data")
}

func TestDetectSkipsDependencyAndBuildDirectories(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "node_modules/react/package.json", `{"dependencies":{"react":"latest"}}`)
	writeTestFile(t, root, "vendor/server.go", "package vendor\nimport \"net/http\"\n")
	writeTestFile(t, root, ".tools/go/src/tool.go", "package tool\n")
	writeTestFile(t, root, ".cache/generated/server.go", "package cache\n")
	writeTestFile(t, root, "README.md", "# docs\n")

	report, err := Detect(root)
	if err != nil {
		t.Fatalf("detect temp corpus: %v", err)
	}

	if hasTool(report, "React") {
		t.Fatalf("did not expect dependency directory package.json to be scanned")
	}
	if hasSurface(report, "api") {
		t.Fatalf("did not expect vendor source to create an API surface")
	}
	if hasLanguage(report, "Go") {
		t.Fatalf("did not expect ignored tool/cache directories to be scanned")
	}
	assertHasSurface(t, report, "docs")
}

func TestDetectReportsClearMissingTreeSitterGrammar(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "src/Program.cs", "namespace Demo { class Program {} }\n")

	report, err := Detect(root)
	if err != nil {
		t.Fatalf("detect temp corpus: %v", err)
	}

	if len(report.TreeSitter.MissingGrammars) != 1 {
		t.Fatalf("expected one missing grammar diagnostic, got %#v", report.TreeSitter.MissingGrammars)
	}
	diagnostic := report.TreeSitter.MissingGrammars[0]
	if diagnostic.Language != "C#" {
		t.Fatalf("expected C# diagnostic, got %#v", diagnostic)
	}
	for _, expected := range []string{
		"tree-sitter grammar missing for language C#",
		"registered grammars: Go, Java, JavaScript, Python, TSX, TypeScript",
	} {
		if !strings.Contains(diagnostic.Message, expected) {
			t.Fatalf("expected diagnostic %q to contain %q", diagnostic.Message, expected)
		}
	}
}

func TestDetectTreeSitterMediumRepoPerformance(t *testing.T) {
	root := t.TempDir()
	const filesPerLanguage = 80
	for i := 0; i < filesPerLanguage; i++ {
		writeTestFile(t, root, fmt.Sprintf("go/pkg_%03d/service.go", i), fmt.Sprintf(
			"package pkg_%03d\nfunc Value() int { return %d }\n",
			i,
			i,
		))
		writeTestFile(t, root, fmt.Sprintf("web/src/component_%03d.ts", i), fmt.Sprintf(
			"export function component%d(): number { return %d; }\n",
			i,
			i,
		))
		writeTestFile(t, root, fmt.Sprintf("workers/task_%03d.py", i), fmt.Sprintf(
			"def task_%03d():\n    return %d\n",
			i,
			i,
		))
	}

	started := time.Now()
	report, err := Detect(root)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("detect medium corpus: %v", err)
	}

	const parsedFiles = filesPerLanguage * 3
	if report.TreeSitter.ParsedFiles != parsedFiles {
		t.Fatalf("expected %d parsed files, got %d", parsedFiles, report.TreeSitter.ParsedFiles)
	}
	if len(report.TreeSitter.MissingGrammars) != 0 || len(report.TreeSitter.ParseErrors) != 0 {
		t.Fatalf("expected clean Tree-sitter parse, got %#v", report.TreeSitter)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("medium corpus detection took %s, expected <= 10s", elapsed)
	}
}

func TestWriteJSONExportsStableDetectionReport(t *testing.T) {
	report, err := Detect(filepath.Join("testdata", "corpus", "fullstack"))
	if err != nil {
		t.Fatalf("detect fullstack corpus: %v", err)
	}

	var out bytes.Buffer
	if err := WriteJSON(&out, report); err != nil {
		t.Fatalf("write json: %v", err)
	}

	var decoded Report
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decode json report: %v\n%s", err, out.String())
	}
	if decoded.Format != ReportFormat {
		t.Fatalf("expected format %q, got %q", ReportFormat, decoded.Format)
	}
	if decoded.FilesScanned == 0 {
		t.Fatalf("expected filesScanned to be populated")
	}
	if !decoded.NodeTypeScript.Enabled {
		t.Fatalf("expected Node/TypeScript adapter report to be enabled")
	}
	assertHasSurface(t, decoded, "api")
}

func writeTestFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	filePath := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("create test directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", rel, err)
	}
}

func assertHasLanguage(t *testing.T, report Report, name string) {
	t.Helper()
	if !hasLanguage(report, name) {
		t.Fatalf("expected language %q in %#v", name, report.Languages)
	}
}

func assertHasTreeSitterLanguage(t *testing.T, report Report, name string) {
	t.Helper()
	for _, item := range report.TreeSitter.Languages {
		if item.Name == name {
			if item.Files == 0 || len(item.Evidence) == 0 {
				t.Fatalf("Tree-sitter language %q has incomplete evidence: %#v", name, item)
			}
			return
		}
	}
	t.Fatalf("expected Tree-sitter language %q in %#v", name, report.TreeSitter.Languages)
}

func hasLanguage(report Report, name string) bool {
	for _, item := range report.Languages {
		if item.Name != name {
			continue
		}
		return len(item.Evidence) > 0
	}
	return false
}

func assertHasTool(t *testing.T, report Report, name string) {
	t.Helper()
	if !hasTool(report, name) {
		t.Fatalf("expected tool %q in %#v", name, report.Tools)
	}
}

func hasTool(report Report, name string) bool {
	for _, item := range report.Tools {
		if item.Name == name {
			return true
		}
	}
	return false
}

func assertHasCI(t *testing.T, report Report, provider string) {
	t.Helper()
	for _, item := range report.CI {
		if item.Provider == provider {
			if len(item.Evidence) == 0 {
				t.Fatalf("CI provider %q has no evidence", provider)
			}
			return
		}
	}
	t.Fatalf("expected CI provider %q in %#v", provider, report.CI)
}

func assertHasSurface(t *testing.T, report Report, name string) {
	t.Helper()
	if !hasSurface(report, name) {
		t.Fatalf("expected surface %q in %#v", name, report.Surfaces)
	}
}

func hasSurface(report Report, name string) bool {
	for _, item := range report.Surfaces {
		if item.Name == name {
			return true
		}
	}
	return false
}

func assertHasNodeTypeScriptFinding(t *testing.T, report Report, kind string, name string, surface string) {
	t.Helper()
	for _, item := range report.NodeTypeScript.Findings {
		if item.Kind == kind && item.Name == name && item.Surface == surface {
			if item.Confidence == "" || len(item.Evidence) == 0 {
				t.Fatalf("Node/TypeScript finding %s/%s/%s has incomplete evidence: %#v", kind, name, surface, item)
			}
			return
		}
	}
	t.Fatalf(
		"expected Node/TypeScript finding %s/%s/%s in %#v",
		kind,
		name,
		surface,
		report.NodeTypeScript.Findings,
	)
}

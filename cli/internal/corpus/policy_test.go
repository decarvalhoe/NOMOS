package corpus

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultPolicyIgnoresCommonDirs(t *testing.T) {
	p := DefaultPolicy()

	ignored := []string{
		".git/config",
		".git/objects/pack/abc",
		"node_modules/react/index.js",
		"vendor/github.com/pkg/errors/errors.go",
		".venv/lib/python3.11/site.py",
		"dist/bundle.js",
		"__pycache__/module.cpython-311.pyc",
		"target/classes/Main.class",
	}
	for _, rel := range ignored {
		if p.Match(rel) {
			t.Errorf("expected %q to be ignored by default policy", rel)
		}
	}
}

func TestDefaultPolicyIgnoresBinaries(t *testing.T) {
	p := DefaultPolicy()

	binaries := []string{
		"bin/server.exe",
		"lib/libcrypto.so",
		"build/app.dll",
		"out/module.wasm",
		"pkg/archive.a",
		"deep/path/to/file.pyc",
	}
	for _, rel := range binaries {
		if p.Match(rel) {
			t.Errorf("expected %q to be ignored by default policy", rel)
		}
	}
}

func TestDefaultPolicyAllowsSources(t *testing.T) {
	p := DefaultPolicy()

	allowed := []string{
		"src/main.go",
		"cli/internal/detect/detect.go",
		"docs/README.md",
		"pyproject.toml",
		"adapters/python/adapter.nomos.yaml",
		"tests/test_policy.py",
	}
	for _, rel := range allowed {
		if !p.Match(rel) {
			t.Errorf("expected %q to be allowed by default policy", rel)
		}
	}
}

func TestCustomAllowPattern(t *testing.T) {
	p := Policy{
		Allow:  []string{"docs/**"},
		Ignore: []string{},
	}

	if !p.Match("docs/README.md") {
		t.Error("expected docs/README.md to match allow docs/**")
	}
	if p.Match("src/main.go") {
		t.Error("expected src/main.go to not match allow docs/**")
	}
}

func TestIgnoreOverridesAllow(t *testing.T) {
	p := Policy{
		Allow:  []string{"**/*"},
		Ignore: []string{"secret/**"},
	}

	if p.Match("secret/keys.pem") {
		t.Error("expected secret/keys.pem to be ignored")
	}
	if !p.Match("public/index.html") {
		t.Error("expected public/index.html to be allowed")
	}
}

func TestParsePolicyFromYAML(t *testing.T) {
	yaml := []byte(`
schema_version: "0.1.0"
corpus:
  allow:
    - "src/**"
    - "docs/**"
  ignore:
    - "**/*.tmp"
    - "docs/draft/**"
`)
	p, err := ParsePolicy(yaml)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(p.Allow) != 2 {
		t.Fatalf("expected 2 allow patterns, got %d", len(p.Allow))
	}
	if len(p.Ignore) != 2 {
		t.Fatalf("expected 2 ignore patterns, got %d", len(p.Ignore))
	}

	if !p.Match("src/main.go") {
		t.Error("expected src/main.go to be allowed")
	}
	if !p.Match("docs/README.md") {
		t.Error("expected docs/README.md to be allowed")
	}
	if p.Match("docs/draft/wip.md") {
		t.Error("expected docs/draft/wip.md to be ignored")
	}
	if p.Match("lib/other.go") {
		t.Error("expected lib/other.go to not match allow patterns")
	}
}

func TestParsePolicyDefaultAllow(t *testing.T) {
	yaml := []byte(`
schema_version: "0.1.0"
corpus:
  ignore:
    - "tmp/**"
`)
	p, err := ParsePolicy(yaml)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(p.Allow) != 1 || p.Allow[0] != "**/*" {
		t.Fatalf("expected default allow [**/*], got %v", p.Allow)
	}
}

func TestParsePolicyInvalidYAML(t *testing.T) {
	_, err := ParsePolicy([]byte(`{not valid`))
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadPolicyFromFile(t *testing.T) {
	dir := t.TempDir()
	config := `
schema_version: "0.1.0"
corpus:
  allow:
    - "contracts/**"
  ignore:
    - "**/*.bak"
`
	if err := os.WriteFile(filepath.Join(dir, ".nomos-corpus.yaml"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	p, err := LoadPolicy(dir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(p.Allow) != 1 || p.Allow[0] != "contracts/**" {
		t.Fatalf("expected allow [contracts/**], got %v", p.Allow)
	}
	if !p.Match("contracts/benefits.yaml") {
		t.Error("expected contracts/benefits.yaml to be allowed")
	}
	if p.Match("contracts/old.bak") {
		t.Error("expected .bak file to be ignored")
	}
}

func TestLoadPolicyDefaultWhenMissing(t *testing.T) {
	dir := t.TempDir()
	p, err := LoadPolicy(dir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	// Should return default policy.
	if len(p.Ignore) == 0 {
		t.Fatal("expected default ignore patterns")
	}
	if !p.Match("src/main.go") {
		t.Error("expected default policy to allow source files")
	}
}

func TestMergePolicies(t *testing.T) {
	base := Policy{
		Allow:  []string{"**/*"},
		Ignore: []string{".git/**"},
	}
	override := Policy{
		Ignore: []string{"tmp/**", "**/*.log"},
	}

	merged := base.Merge(override)

	if len(merged.Allow) != 1 || merged.Allow[0] != "**/*" {
		t.Fatalf("expected base allow preserved, got %v", merged.Allow)
	}
	if len(merged.Ignore) != 3 {
		t.Fatalf("expected 3 ignore patterns, got %d: %v", len(merged.Ignore), merged.Ignore)
	}
	if p := merged; p.Match(".git/config") || p.Match("tmp/data") || p.Match("app/debug.log") {
		t.Error("expected merged ignores to apply")
	}
}

func TestMergeOverrideAllow(t *testing.T) {
	base := Policy{
		Allow:  []string{"**/*"},
		Ignore: []string{".git/**"},
	}
	override := Policy{
		Allow: []string{"src/**"},
	}

	merged := base.Merge(override)

	if len(merged.Allow) != 1 || merged.Allow[0] != "src/**" {
		t.Fatalf("expected override allow to replace, got %v", merged.Allow)
	}
}

func TestFilter(t *testing.T) {
	p := Policy{
		Allow:  []string{"**/*"},
		Ignore: []string{"node_modules/**", "**/*.exe"},
	}

	paths := []string{
		"src/main.go",
		"node_modules/express/index.js",
		"bin/tool.exe",
		"docs/guide.md",
	}

	result := p.Filter(paths)

	if len(result) != 2 {
		t.Fatalf("expected 2 filtered paths, got %d: %v", len(result), result)
	}
	if result[0] != "src/main.go" || result[1] != "docs/guide.md" {
		t.Fatalf("unexpected filtered result: %v", result)
	}
}

func TestGlobMatchExtensionAnywhere(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"**/*.go", "main.go", true},
		{"**/*.go", "cmd/api/main.go", true},
		{"**/*.go", "main.py", false},
		{"src/**", "src/lib/utils.ts", true},
		{"src/**", "test/lib/utils.ts", false},
		{".git/**", ".git/HEAD", true},
		{".git/**", ".git/objects/ab/cd", true},
		{".git/**", "src/.git/fake", false},
	}

	for _, tc := range cases {
		got := globMatch(tc.pattern, tc.path)
		if got != tc.want {
			t.Errorf("globMatch(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

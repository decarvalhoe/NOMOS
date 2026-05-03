package detect

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxEvidenceEntries = 20
	maxContentBytes    = 128 * 1024
	maxDiagnostics     = 20
)

type aggregate struct {
	format         string
	root           string
	filesScanned   int
	languages      map[string]*languageAggregate
	tools          map[string]*toolAggregate
	ci             map[string]*ciAggregate
	surfaces       map[string]*surfaceAggregate
	treeSitter     *treeSitterAggregate
	nodeTypeScript *nodeTypeScriptAggregate
}

type languageAggregate struct {
	name     string
	files    int
	evidence []Evidence
}

type toolAggregate struct {
	name     string
	kind     string
	evidence []Evidence
}

type ciAggregate struct {
	provider string
	evidence []Evidence
}

type surfaceAggregate struct {
	name       string
	confidence string
	evidence   []Evidence
}

type treeSitterAggregate struct {
	enabled         bool
	parsedFiles     int
	languages       map[string]*languageAggregate
	missingGrammars []TreeSitterDiagnostic
	parseErrors     []TreeSitterDiagnostic
	registry        *treeSitterRegistry
}

type nodeTypeScriptAggregate struct {
	enabled  bool
	findings map[string]*nodeTypeScriptFindingAggregate
}

type nodeTypeScriptFindingAggregate struct {
	kind       string
	name       string
	surface    string
	confidence string
	evidence   []Evidence
}

type packageManifest struct {
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
}

func Detect(root string) (Report, error) {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	info, err := os.Stat(cleanRoot)
	if err != nil {
		return Report{}, err
	}
	if !info.IsDir() {
		return Report{}, errors.New("detect root must be a directory")
	}

	a := aggregate{
		format:         ReportFormat,
		root:           cleanRoot,
		languages:      map[string]*languageAggregate{},
		tools:          map[string]*toolAggregate{},
		ci:             map[string]*ciAggregate{},
		surfaces:       map[string]*surfaceAggregate{},
		treeSitter:     newTreeSitterAggregate(),
		nodeTypeScript: newNodeTypeScriptAggregate(),
	}

	err = filepath.WalkDir(cleanRoot, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(cleanRoot, filePath)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if entry.IsDir() {
			if shouldSkipDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}

		a.filesScanned++
		a.detectPath(rel)
		a.detectNodeTypeScriptPath(rel)
		if shouldReadContent(rel) {
			content, err := readFilePrefix(filePath)
			if err != nil {
				return err
			}
			a.detectContent(rel, content)
			a.detectNodeTypeScriptContent(rel, content)
			a.detectTreeSitter(rel, []byte(content))
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}

	return a.report(), nil
}

func shouldSkipDir(rel string) bool {
	base := strings.ToLower(path.Base(rel))
	switch base {
	case ".git", ".hg", ".svn", ".tools", ".cache", "node_modules", "vendor", "dist", "build", ".next",
		"coverage", ".venv", "venv", "__pycache__", "target", ".terraform":
		return true
	default:
		return false
	}
}

func shouldReadContent(rel string) bool {
	base := strings.ToLower(path.Base(rel))
	ext := strings.ToLower(path.Ext(rel))
	if base == "package.json" || base == "go.mod" || base == "pyproject.toml" {
		return true
	}
	switch ext {
	case ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".rb", ".php", ".java", ".kt",
		".cs", ".rs", ".sql", ".yaml", ".yml", ".toml", ".json":
		return true
	default:
		return false
	}
}

func readFilePrefix(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxContentBytes))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *aggregate) detectPath(rel string) {
	lower := strings.ToLower(rel)
	base := path.Base(rel)
	lowerBase := strings.ToLower(base)
	ext := strings.ToLower(path.Ext(rel))

	if language := languageForPath(rel); language != "" {
		a.addLanguage(language, rel, "file extension or well-known filename")
	}

	switch lowerBase {
	case "go.mod":
		a.addTool("Go modules", "language-manifest", rel, "go.mod")
	case "package.json":
		a.addTool("Node package manifest", "language-manifest", rel, "package.json")
	case "package-lock.json":
		a.addTool("npm", "package-manager", rel, "package-lock.json")
	case "pnpm-lock.yaml":
		a.addTool("pnpm", "package-manager", rel, "pnpm-lock.yaml")
	case "yarn.lock":
		a.addTool("Yarn", "package-manager", rel, "yarn.lock")
	case "pyproject.toml":
		a.addTool("Python pyproject", "language-manifest", rel, "pyproject.toml")
	case "requirements.txt":
		a.addTool("pip requirements", "language-manifest", rel, "requirements.txt")
	case "poetry.lock":
		a.addTool("Poetry", "package-manager", rel, "poetry.lock")
	case "pipfile":
		a.addTool("Pipenv", "package-manager", rel, "Pipfile")
	case "cargo.toml":
		a.addTool("Cargo", "language-manifest", rel, "Cargo.toml")
	case "pom.xml":
		a.addTool("Maven", "build-tool", rel, "pom.xml")
	case "build.gradle", "build.gradle.kts":
		a.addTool("Gradle", "build-tool", rel, base)
	case "makefile":
		a.addTool("Make", "build-tool", rel, "Makefile")
	case "taskfile.yml", "taskfile.yaml":
		a.addTool("Task", "build-tool", rel, base)
	case "dockerfile":
		a.addTool("Docker", "container", rel, "Dockerfile")
		a.addSurface("infra", "high", rel, "container build file")
	case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
		a.addTool("Docker Compose", "container", rel, base)
		a.addSurface("infra", "high", rel, "compose file")
	}

	if strings.HasPrefix(lower, ".github/workflows/") && (ext == ".yml" || ext == ".yaml") {
		a.addCI("GitHub Actions", rel, "workflow file")
		a.addTool("GitHub Actions", "ci", rel, "workflow file")
	}
	if lowerBase == ".gitlab-ci.yml" || lowerBase == ".gitlab-ci.yaml" {
		a.addCI("GitLab CI", rel, "GitLab CI config")
		a.addTool("GitLab CI", "ci", rel, "GitLab CI config")
	}
	if lowerBase == "circle.yml" || strings.HasPrefix(lower, ".circleci/") {
		a.addCI("CircleCI", rel, "CircleCI config")
		a.addTool("CircleCI", "ci", rel, "CircleCI config")
	}

	if isDocsPath(lower, ext) {
		a.addSurface("docs", "high", rel, "documentation path")
	}
	if isInfraPath(lower, lowerBase, ext) {
		a.addSurface("infra", "high", rel, "infrastructure path or file")
	}
	if isDataPath(lower, lowerBase, ext) {
		a.addSurface("data", "high", rel, "database or migration path")
	}
	if isAPIPath(lower, lowerBase, ext) {
		a.addSurface("api", "medium", rel, "API-oriented path or contract")
	}
	if isUIPath(lower, ext) {
		a.addSurface("ui", "medium", rel, "UI-oriented path")
	}
	if isWorkerPath(lower, lowerBase) {
		a.addSurface("worker", "medium", rel, "worker or job path")
	}
}

func (a *aggregate) detectContent(rel string, content string) {
	lowerContent := strings.ToLower(content)
	lowerRel := strings.ToLower(rel)
	base := strings.ToLower(path.Base(rel))

	if base == "package.json" {
		a.detectPackageJSON(rel, content)
	}

	for _, pattern := range []string{
		"fastapi(", "apirouter(", "@app.route", "@router.", "express()",
		"express.router", "router()", "app.get(", "app.post(", "app.put(",
		"app.delete(", "http.handlefunc(", "http.handle(", "gin.default(",
		"chi.newrouter(", "router.get(",
	} {
		if strings.Contains(lowerContent, pattern) {
			a.addSurface("api", "high", rel, "API route or server pattern")
			break
		}
	}

	for _, pattern := range []string{
		"celery(", "sidekiq", "bullmq", "queue.process", "worker(",
		"cron.schedule", "background job",
	} {
		if strings.Contains(lowerContent, pattern) {
			a.addSurface("worker", "high", rel, "worker or queue pattern")
			break
		}
	}

	if strings.HasSuffix(lowerRel, ".sql") ||
		strings.Contains(lowerContent, "create table ") ||
		strings.Contains(lowerContent, "alter table ") {
		a.addSurface("data", "high", rel, "SQL schema or migration content")
	}
}

func (a *aggregate) detectNodeTypeScriptPath(rel string) {
	lower := strings.ToLower(rel)
	lowerBase := strings.ToLower(path.Base(rel))
	ext := strings.ToLower(path.Ext(rel))

	if lowerBase == "package.json" {
		a.addNodeTypeScriptFinding(
			"language_detection",
			"Node package manifest",
			"",
			"high",
			rel,
			"Node/TypeScript adapter package manifest",
		)
	}

	if isNodeTypeScriptFixturePath(lower, lowerBase) {
		a.addNodeTypeScriptFinding(
			"fixture_detection",
			"Test fixture",
			"",
			"high",
			rel,
			"Node/TypeScript fixture path convention",
		)
	}
	if !isNodeTypeScriptSourceExt(ext) {
		return
	}

	switch {
	case isNextAPIRoutePath(lower, lowerBase):
		a.addSurface("api", "high", rel, "Next.js API route convention")
		a.addNodeTypeScriptFinding(
			"route_detection",
			"Next.js API route",
			"api",
			"high",
			rel,
			"Next.js app/pages API route path",
		)
	case isNextUIRoutePath(lower, lowerBase):
		a.addSurface("ui", "high", rel, "Next.js UI route convention")
		a.addNodeTypeScriptFinding(
			"route_detection",
			"Next.js UI route",
			"ui",
			"high",
			rel,
			"Next.js app/pages UI route path",
		)
	case strings.Contains(lower, "/routes/") ||
		strings.Contains(lower, "/controllers/") ||
		strings.HasSuffix(lowerBase, "routes.ts") ||
		strings.HasSuffix(lowerBase, "routes.js") ||
		strings.HasSuffix(lowerBase, ".route.ts") ||
		strings.HasSuffix(lowerBase, ".route.js"):
		a.addSurface("api", "medium", rel, "Node route path convention")
		a.addNodeTypeScriptFinding(
			"route_detection",
			"Node route module",
			"api",
			"medium",
			rel,
			"Node route/controller path convention",
		)
	}

	if strings.Contains(lower, "/services/") ||
		strings.HasSuffix(lowerBase, ".service.ts") ||
		strings.HasSuffix(lowerBase, ".service.js") ||
		strings.HasSuffix(lowerBase, "service.ts") ||
		strings.HasSuffix(lowerBase, "service.js") {
		a.addNodeTypeScriptFinding(
			"service_detection",
			"Service module",
			"",
			"high",
			rel,
			"Node/TypeScript service path convention",
		)
	}

	if isNodeTypeScriptMockPath(lower, lowerBase) {
		a.addNodeTypeScriptFinding(
			"mock_detection",
			"Test mock",
			"",
			"high",
			rel,
			"Node/TypeScript mock path convention",
		)
	}

	if strings.Contains(lower, "/catalogs/") ||
		strings.Contains(lower, "/catalogues/") ||
		strings.Contains(lower, "/catalog/") ||
		strings.Contains(lowerBase, "catalog") ||
		strings.Contains(lowerBase, "catalogue") {
		a.addSurface("data", "medium", rel, "Node/TypeScript catalogue path")
		a.addNodeTypeScriptFinding(
			"hardcoded_catalog_detection",
			"Hardcoded catalogue",
			"data",
			"medium",
			rel,
			"Node/TypeScript catalogue path convention",
		)
	}
}

func (a *aggregate) detectNodeTypeScriptContent(rel string, content string) {
	lowerContent := strings.ToLower(content)
	lowerBase := strings.ToLower(path.Base(rel))
	ext := strings.ToLower(path.Ext(rel))

	if lowerBase == "package.json" {
		a.detectNodeTypeScriptPackageJSON(rel, content)
		return
	}
	if !isNodeTypeScriptSourceExt(ext) {
		return
	}

	for _, pattern := range []string{
		"app.get(", "app.post(", "app.put(", "app.patch(", "app.delete(",
		"router.get(", "router.post(", "router.put(", "router.patch(", "router.delete(",
		"routes.get(", "routes.post(", "routes.put(", "routes.patch(", "routes.delete(",
		"fastify.get(", "fastify.post(", "fastify.put(", "fastify.patch(", "fastify.delete(",
		"export function get(", "export function post(", "export function put(",
		"export function patch(", "export function delete(",
		"export async function get(", "export async function post(", "export async function put(",
		"export async function patch(", "export async function delete(",
	} {
		if strings.Contains(lowerContent, pattern) {
			a.addSurface("api", "high", rel, "Node/TypeScript route handler content")
			a.addNodeTypeScriptFinding(
				"route_detection",
				"Node route handler",
				"api",
				"high",
				rel,
				"Node/TypeScript route handler pattern "+pattern,
			)
			break
		}
	}

	if (strings.Contains(lowerContent, "export class ") && strings.Contains(lowerContent, "service")) ||
		(strings.Contains(lowerContent, "export const ") && strings.Contains(lowerContent, "service =")) ||
		(strings.Contains(lowerContent, "export function ") && strings.Contains(lowerContent, "service(")) {
		a.addNodeTypeScriptFinding(
			"service_detection",
			"Service module",
			"",
			"medium",
			rel,
			"Node/TypeScript service symbol pattern",
		)
	}

	for _, pattern := range []string{
		"jest.mock(", "vi.mock(", "setupserver(", "mockserviceworker",
		" from \"msw\"", " from 'msw'", "rest.get(", "http.get(",
	} {
		if strings.Contains(lowerContent, pattern) {
			a.addNodeTypeScriptFinding(
				"mock_detection",
				"Test mock",
				"",
				"medium",
				rel,
				"Node/TypeScript mock content pattern "+pattern,
			)
			break
		}
	}

	if looksLikeHardcodedCatalogue(lowerContent) {
		a.addSurface("data", "medium", rel, "hardcoded Node/TypeScript catalogue content")
		a.addNodeTypeScriptFinding(
			"hardcoded_catalog_detection",
			"Hardcoded catalogue",
			"data",
			"medium",
			rel,
			"Node/TypeScript hardcoded catalogue constant",
		)
	}
}

func (a *aggregate) detectTreeSitter(rel string, content []byte) {
	language := treeSitterLanguageForPath(rel)
	if language == "" {
		return
	}

	result, err := a.treeSitter.registry.parse(rel, language, content)
	if err != nil {
		a.treeSitter.addDiagnostic(&a.treeSitter.missingGrammars, rel, language, err)
		return
	}

	a.treeSitter.parsedFiles++
	item, ok := a.treeSitter.languages[result.Language]
	if !ok {
		item = &languageAggregate{name: result.Language}
		a.treeSitter.languages[result.Language] = item
	}
	item.files++
	addEvidence(
		&item.evidence,
		rel,
		"Tree-sitter parsed "+result.Language+" AST root "+result.RootType,
	)

	if result.HasError {
		a.treeSitter.addDiagnostic(
			&a.treeSitter.parseErrors,
			rel,
			result.Language,
			errors.New("tree-sitter parse completed with syntax errors"),
		)
	}
}

func (a *aggregate) detectPackageJSON(rel string, content string) {
	var manifest packageManifest
	if err := json.Unmarshal([]byte(content), &manifest); err != nil {
		return
	}
	deps := map[string]string{}
	for key, value := range manifest.Dependencies {
		deps[strings.ToLower(key)] = value
	}
	for key, value := range manifest.DevDependencies {
		deps[strings.ToLower(key)] = value
	}
	for key, value := range manifest.OptionalDependencies {
		deps[strings.ToLower(key)] = value
	}

	toolRules := map[string]struct {
		name    string
		kind    string
		surface string
	}{
		"@angular/core":        {"Angular", "ui-framework", "ui"},
		"@sveltejs/kit":        {"SvelteKit", "ui-framework", "ui"},
		"@vitejs/plugin-react": {"Vite React", "ui-build-tool", "ui"},
		"bullmq":               {"BullMQ", "queue", "worker"},
		"express":              {"Express", "api-framework", "api"},
		"fastify":              {"Fastify", "api-framework", "api"},
		"next":                 {"Next.js", "ui-framework", "ui"},
		"prisma":               {"Prisma", "database-tool", "data"},
		"react":                {"React", "ui-framework", "ui"},
		"svelte":               {"Svelte", "ui-framework", "ui"},
		"vue":                  {"Vue", "ui-framework", "ui"},
		"vite":                 {"Vite", "ui-build-tool", "ui"},
	}
	for dependency, rule := range toolRules {
		if _, ok := deps[dependency]; !ok {
			continue
		}
		a.addTool(rule.name, rule.kind, rel, "package.json dependency "+dependency)
		a.addSurface(rule.surface, "high", rel, "package.json dependency "+dependency)
	}
}

func (a *aggregate) detectNodeTypeScriptPackageJSON(rel string, content string) {
	var manifest packageManifest
	if err := json.Unmarshal([]byte(content), &manifest); err != nil {
		return
	}
	deps := map[string]string{}
	for key, value := range manifest.Dependencies {
		deps[strings.ToLower(key)] = value
	}
	for key, value := range manifest.DevDependencies {
		deps[strings.ToLower(key)] = value
	}
	for key, value := range manifest.OptionalDependencies {
		deps[strings.ToLower(key)] = value
	}

	frameworkRules := map[string]struct {
		name    string
		surface string
	}{
		"@nestjs/core":  {"NestJS", "api"},
		"@sveltejs/kit": {"SvelteKit", "ui"},
		"express":       {"Express", "api"},
		"fastify":       {"Fastify", "api"},
		"next":          {"Next.js", "ui"},
		"react":         {"React", "ui"},
		"vue":           {"Vue", "ui"},
	}
	for dependency, rule := range frameworkRules {
		if _, ok := deps[dependency]; !ok {
			continue
		}
		a.addNodeTypeScriptFinding(
			"dependency_detection",
			rule.name,
			rule.surface,
			"high",
			rel,
			"package.json dependency "+dependency,
		)
	}

	for _, dependency := range []string{"@types/node", "typescript", "ts-node", "tsx"} {
		if _, ok := deps[dependency]; !ok {
			continue
		}
		a.addNodeTypeScriptFinding(
			"language_detection",
			"TypeScript toolchain",
			"",
			"high",
			rel,
			"package.json dependency "+dependency,
		)
	}

	for _, dependency := range []string{"jest", "vitest", "msw", "@testing-library/react"} {
		if _, ok := deps[dependency]; !ok {
			continue
		}
		a.addNodeTypeScriptFinding(
			"test_surface_detection",
			"Node/TypeScript test tooling",
			"",
			"medium",
			rel,
			"package.json dependency "+dependency,
		)
	}
}

func languageForPath(rel string) string {
	base := path.Base(rel)
	lowerBase := strings.ToLower(base)
	if lowerBase == "dockerfile" || strings.HasPrefix(lowerBase, "dockerfile.") {
		return "Dockerfile"
	}
	switch strings.ToLower(path.Ext(rel)) {
	case ".adoc":
		return "AsciiDoc"
	case ".cs":
		return "C#"
	case ".cue":
		return "CUE"
	case ".go":
		return "Go"
	case ".java":
		return "Java"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "JavaScript"
	case ".json":
		return "JSON"
	case ".kt", ".kts":
		return "Kotlin"
	case ".md", ".mdx":
		return "Markdown"
	case ".php":
		return "PHP"
	case ".py":
		return "Python"
	case ".rb":
		return "Ruby"
	case ".rs":
		return "Rust"
	case ".sql":
		return "SQL"
	case ".swift":
		return "Swift"
	case ".toml":
		return "TOML"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".yaml", ".yml":
		return "YAML"
	default:
		return ""
	}
}

func isDocsPath(lower string, ext string) bool {
	if lower == "readme.md" || lower == "readme.mdx" {
		return true
	}
	if !(ext == ".md" || ext == ".mdx" || ext == ".rst" || ext == ".adoc") {
		return false
	}
	return strings.HasPrefix(lower, "docs/") ||
		strings.HasPrefix(lower, "documentation/") ||
		strings.HasPrefix(lower, "adr/") ||
		strings.Contains(lower, "/docs/")
}

func isInfraPath(lower string, lowerBase string, ext string) bool {
	if strings.HasPrefix(lowerBase, "dockerfile") ||
		lowerBase == "terraform.tf" ||
		strings.HasSuffix(lower, ".tf") ||
		strings.HasSuffix(lower, ".tfvars") ||
		lowerBase == "helmfile.yaml" ||
		lowerBase == "helmfile.yml" ||
		lowerBase == "nginx.conf" {
		return true
	}
	if strings.HasPrefix(lower, "infra/") ||
		strings.HasPrefix(lower, "deploy/") ||
		strings.HasPrefix(lower, "deployment/") ||
		strings.HasPrefix(lower, "terraform/") ||
		strings.HasPrefix(lower, "k8s/") ||
		strings.HasPrefix(lower, "kubernetes/") ||
		strings.HasPrefix(lower, "helm/") ||
		strings.Contains(lower, "/k8s/") ||
		strings.Contains(lower, "/helm/") {
		return true
	}
	return (ext == ".yaml" || ext == ".yml") &&
		(strings.Contains(lower, "deployment") || strings.Contains(lower, "service"))
}

func isDataPath(lower string, lowerBase string, ext string) bool {
	if ext == ".sql" || lowerBase == "schema.prisma" {
		return true
	}
	return strings.HasPrefix(lower, "db/") ||
		strings.HasPrefix(lower, "database/") ||
		strings.HasPrefix(lower, "data/") ||
		strings.HasPrefix(lower, "migrations/") ||
		strings.HasPrefix(lower, "prisma/") ||
		strings.Contains(lower, "/migrations/") ||
		strings.Contains(lower, "/schema/")
}

func isAPIPath(lower string, lowerBase string, ext string) bool {
	if lowerBase == "openapi.yaml" ||
		lowerBase == "openapi.yml" ||
		lowerBase == "swagger.yaml" ||
		lowerBase == "swagger.yml" {
		return true
	}
	if strings.HasPrefix(lower, "api/") ||
		strings.HasPrefix(lower, "cmd/api/") ||
		strings.HasPrefix(lower, "internal/api/") ||
		strings.HasPrefix(lower, "server/") ||
		strings.Contains(lower, "/api/") ||
		strings.Contains(lower, "/controllers/") ||
		strings.Contains(lower, "/handlers/") ||
		strings.Contains(lower, "/routes/") ||
		strings.Contains(lower, "/routers/") {
		return true
	}
	return ext == ".proto" && strings.Contains(lower, "api")
}

func isUIPath(lower string, ext string) bool {
	if strings.HasPrefix(lower, "frontend/") ||
		strings.HasPrefix(lower, "web/") ||
		strings.HasPrefix(lower, "ui/") ||
		strings.HasPrefix(lower, "client/") ||
		strings.Contains(lower, "/components/") ||
		strings.Contains(lower, "/pages/") ||
		strings.Contains(lower, "/app/") && (ext == ".tsx" || ext == ".jsx") {
		return true
	}
	return ext == ".tsx" || ext == ".jsx" || ext == ".vue" || ext == ".svelte"
}

func isWorkerPath(lower string, lowerBase string) bool {
	if strings.Contains(lowerBase, "worker") ||
		strings.Contains(lowerBase, "consumer") ||
		strings.Contains(lowerBase, "queue") {
		return true
	}
	return strings.HasPrefix(lower, "worker/") ||
		strings.HasPrefix(lower, "workers/") ||
		strings.HasPrefix(lower, "jobs/") ||
		strings.HasPrefix(lower, "queues/") ||
		strings.HasPrefix(lower, "consumers/") ||
		strings.HasPrefix(lower, "cron/") ||
		strings.Contains(lower, "/workers/") ||
		strings.Contains(lower, "/jobs/") ||
		strings.Contains(lower, "/queues/") ||
		strings.Contains(lower, "/consumers/")
}

func isNodeTypeScriptSourceExt(ext string) bool {
	switch ext {
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx":
		return true
	default:
		return false
	}
}

func isNextAPIRoutePath(lower string, lowerBase string) bool {
	inAppAPI := strings.Contains(lower, "/app/api/") || strings.HasPrefix(lower, "app/api/")
	isAppRoute := strings.HasSuffix(lower, "/route.ts") || strings.HasSuffix(lower, "/route.js")
	isPagesAPI := strings.HasPrefix(lower, "pages/api/") || strings.Contains(lower, "/pages/api/")
	isFrameworkBootstrap := lowerBase == "_app.tsx" || lowerBase == "_document.tsx"
	return inAppAPI && isAppRoute || isPagesAPI && !isFrameworkBootstrap
}

func isNextUIRoutePath(lower string, lowerBase string) bool {
	if strings.HasSuffix(lower, "/page.tsx") ||
		strings.HasSuffix(lower, "/page.ts") ||
		strings.HasSuffix(lower, "/page.jsx") ||
		strings.HasSuffix(lower, "/page.js") {
		return strings.HasPrefix(lower, "app/") ||
			strings.HasPrefix(lower, "src/app/") ||
			strings.Contains(lower, "/app/")
	}
	if !(strings.HasPrefix(lower, "pages/") || strings.HasPrefix(lower, "src/pages/") || strings.Contains(lower, "/pages/")) {
		return false
	}
	return lowerBase != "_app.tsx" &&
		lowerBase != "_app.ts" &&
		lowerBase != "_app.jsx" &&
		lowerBase != "_app.js" &&
		lowerBase != "_document.tsx" &&
		lowerBase != "_document.ts" &&
		lowerBase != "_document.jsx" &&
		lowerBase != "_document.js" &&
		!strings.HasPrefix(lowerBase, "api.")
}

func isNodeTypeScriptMockPath(lower string, lowerBase string) bool {
	return strings.HasPrefix(lower, "__mocks__/") ||
		strings.HasPrefix(lower, "mocks/") ||
		strings.HasPrefix(lower, "mock-data/") ||
		strings.HasPrefix(lower, "msw/") ||
		strings.Contains(lower, "/__mocks__/") ||
		strings.Contains(lower, "/mocks/") ||
		strings.Contains(lower, "/mock-data/") ||
		strings.Contains(lower, "/msw/") ||
		strings.Contains(lowerBase, ".mock.") ||
		strings.Contains(lowerBase, ".mocks.") ||
		strings.HasSuffix(lowerBase, "mock.ts") ||
		strings.HasSuffix(lowerBase, "mock.tsx") ||
		strings.HasSuffix(lowerBase, "mock.js") ||
		strings.HasSuffix(lowerBase, "mock.jsx")
}

func isNodeTypeScriptFixturePath(lower string, lowerBase string) bool {
	return strings.HasPrefix(lower, "__fixtures__/") ||
		strings.HasPrefix(lower, "fixtures/") ||
		strings.HasPrefix(lower, "testdata/") ||
		strings.HasPrefix(lower, "cypress/fixtures/") ||
		strings.Contains(lower, "/__fixtures__/") ||
		strings.Contains(lower, "/fixtures/") ||
		strings.Contains(lower, "/testdata/") ||
		strings.Contains(lower, "/cypress/fixtures/") ||
		strings.Contains(lowerBase, ".fixture.") ||
		strings.Contains(lowerBase, ".fixtures.")
}

func looksLikeHardcodedCatalogue(lowerContent string) bool {
	for _, line := range strings.Split(lowerContent, "\n") {
		if !(strings.Contains(line, "const ") || strings.Contains(line, "readonly ")) {
			continue
		}
		if !(strings.Contains(line, " = [") ||
			strings.Contains(line, " = {") ||
			strings.Contains(line, " as const")) {
			continue
		}
		for _, name := range []string{
			"catalog", "catalogue", "status", "statuses", "tier", "tiers",
			"bracket", "brackets", "option", "options",
		} {
			if strings.Contains(line, name) {
				return true
			}
		}
	}
	return false
}

func (a *aggregate) addLanguage(name string, rel string, reason string) {
	item, ok := a.languages[name]
	if !ok {
		item = &languageAggregate{name: name}
		a.languages[name] = item
	}
	item.files++
	addEvidence(&item.evidence, rel, reason)
}

func (a *aggregate) addTool(name string, kind string, rel string, reason string) {
	item, ok := a.tools[name]
	if !ok {
		item = &toolAggregate{name: name, kind: kind}
		a.tools[name] = item
	}
	addEvidence(&item.evidence, rel, reason)
}

func (a *aggregate) addCI(provider string, rel string, reason string) {
	item, ok := a.ci[provider]
	if !ok {
		item = &ciAggregate{provider: provider}
		a.ci[provider] = item
	}
	addEvidence(&item.evidence, rel, reason)
}

func (a *aggregate) addSurface(name string, confidence string, rel string, reason string) {
	item, ok := a.surfaces[name]
	if !ok {
		item = &surfaceAggregate{name: name, confidence: confidence}
		a.surfaces[name] = item
	} else if item.confidence != "high" && confidence == "high" {
		item.confidence = confidence
	}
	addEvidence(&item.evidence, rel, reason)
}

func (a *aggregate) addNodeTypeScriptFinding(
	kind string,
	name string,
	surface string,
	confidence string,
	rel string,
	reason string,
) {
	key := strings.Join([]string{kind, name, surface}, "\x00")
	item, ok := a.nodeTypeScript.findings[key]
	if !ok {
		item = &nodeTypeScriptFindingAggregate{
			kind:       kind,
			name:       name,
			surface:    surface,
			confidence: confidence,
		}
		a.nodeTypeScript.findings[key] = item
	} else if confidenceRank(confidence) > confidenceRank(item.confidence) {
		item.confidence = confidence
	}
	addEvidence(&item.evidence, rel, reason)
}

func confidenceRank(confidence string) int {
	switch confidence {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func addEvidence(target *[]Evidence, rel string, reason string) {
	for _, existing := range *target {
		if existing.Path == rel && existing.Reason == reason {
			return
		}
	}
	if len(*target) >= maxEvidenceEntries {
		return
	}
	*target = append(*target, Evidence{Path: rel, Reason: reason})
}

func newTreeSitterAggregate() *treeSitterAggregate {
	return &treeSitterAggregate{
		enabled:   treeSitterSupported(),
		languages: map[string]*languageAggregate{},
		registry:  newTreeSitterRegistry(),
	}
}

func newNodeTypeScriptAggregate() *nodeTypeScriptAggregate {
	return &nodeTypeScriptAggregate{
		enabled:  true,
		findings: map[string]*nodeTypeScriptFindingAggregate{},
	}
}

func (a *treeSitterAggregate) addDiagnostic(
	target *[]TreeSitterDiagnostic,
	rel string,
	language string,
	err error,
) {
	if len(*target) >= maxDiagnostics {
		return
	}
	for _, existing := range *target {
		if existing.Path == rel && existing.Language == language && existing.Message == err.Error() {
			return
		}
	}
	*target = append(*target, TreeSitterDiagnostic{
		Path:     rel,
		Language: language,
		Message:  err.Error(),
	})
}

func (a *aggregate) report() Report {
	report := Report{
		Format:       a.format,
		Root:         a.root,
		FilesScanned: a.filesScanned,
		Languages:    make([]LanguageFinding, 0, len(a.languages)),
		Tools:        make([]ToolFinding, 0, len(a.tools)),
		CI:           make([]CIFinding, 0, len(a.ci)),
		Surfaces:     make([]SurfaceFinding, 0, len(a.surfaces)),
		TreeSitter: TreeSitterReport{
			Enabled:         a.treeSitter.enabled,
			ParsedFiles:     a.treeSitter.parsedFiles,
			Languages:       make([]TreeSitterLanguageReport, 0, len(a.treeSitter.languages)),
			MissingGrammars: append([]TreeSitterDiagnostic(nil), a.treeSitter.missingGrammars...),
			ParseErrors:     append([]TreeSitterDiagnostic(nil), a.treeSitter.parseErrors...),
		},
		NodeTypeScript: NodeTypeScriptReport{
			Enabled:  a.nodeTypeScript.enabled,
			Findings: make([]NodeTypeScriptAdapterSignal, 0, len(a.nodeTypeScript.findings)),
		},
	}

	for _, item := range a.languages {
		sortEvidence(item.evidence)
		report.Languages = append(report.Languages, LanguageFinding{
			Name:     item.name,
			Files:    item.files,
			Evidence: item.evidence,
		})
	}
	sort.Slice(report.Languages, func(i, j int) bool {
		if report.Languages[i].Files == report.Languages[j].Files {
			return report.Languages[i].Name < report.Languages[j].Name
		}
		return report.Languages[i].Files > report.Languages[j].Files
	})

	for _, item := range a.tools {
		sortEvidence(item.evidence)
		report.Tools = append(report.Tools, ToolFinding{
			Name:     item.name,
			Kind:     item.kind,
			Evidence: item.evidence,
		})
	}
	sort.Slice(report.Tools, func(i, j int) bool {
		return report.Tools[i].Name < report.Tools[j].Name
	})

	for _, item := range a.ci {
		sortEvidence(item.evidence)
		report.CI = append(report.CI, CIFinding{
			Provider: item.provider,
			Evidence: item.evidence,
		})
	}
	sort.Slice(report.CI, func(i, j int) bool {
		return report.CI[i].Provider < report.CI[j].Provider
	})

	for _, item := range a.surfaces {
		sortEvidence(item.evidence)
		report.Surfaces = append(report.Surfaces, SurfaceFinding{
			Name:       item.name,
			Confidence: item.confidence,
			Evidence:   item.evidence,
		})
	}
	sort.Slice(report.Surfaces, func(i, j int) bool {
		return report.Surfaces[i].Name < report.Surfaces[j].Name
	})

	for _, item := range a.treeSitter.languages {
		sortEvidence(item.evidence)
		report.TreeSitter.Languages = append(report.TreeSitter.Languages, TreeSitterLanguageReport{
			Name:     item.name,
			Files:    item.files,
			Evidence: item.evidence,
		})
	}
	sort.Slice(report.TreeSitter.Languages, func(i, j int) bool {
		if report.TreeSitter.Languages[i].Files == report.TreeSitter.Languages[j].Files {
			return report.TreeSitter.Languages[i].Name < report.TreeSitter.Languages[j].Name
		}
		return report.TreeSitter.Languages[i].Files > report.TreeSitter.Languages[j].Files
	})
	sortTreeSitterDiagnostics(report.TreeSitter.MissingGrammars)
	sortTreeSitterDiagnostics(report.TreeSitter.ParseErrors)

	for _, item := range a.nodeTypeScript.findings {
		sortEvidence(item.evidence)
		report.NodeTypeScript.Findings = append(report.NodeTypeScript.Findings, NodeTypeScriptAdapterSignal{
			Kind:       item.kind,
			Name:       item.name,
			Surface:    item.surface,
			Confidence: item.confidence,
			Evidence:   item.evidence,
		})
	}
	sort.Slice(report.NodeTypeScript.Findings, func(i, j int) bool {
		left := report.NodeTypeScript.Findings[i]
		right := report.NodeTypeScript.Findings[j]
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return left.Surface < right.Surface
	})

	return report
}

func sortEvidence(items []Evidence) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Reason < items[j].Reason
		}
		return items[i].Path < items[j].Path
	})
}

func sortTreeSitterDiagnostics(items []TreeSitterDiagnostic) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			if items[i].Language == items[j].Language {
				return items[i].Message < items[j].Message
			}
			return items[i].Language < items[j].Language
		}
		return items[i].Path < items[j].Path
	})
}

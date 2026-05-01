package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type initConfig struct {
	mode        string
	projectID   string
	projectName string
	domain      string
	ownerName   string
	ownerEmail  string
	repository  string
	target      string
}

type initProfile struct {
	riskLevel       string
	scopeConfidence string
	dataSensitivity string
	attestation     string
	regulated       bool
	requiredReports []string
	standards       []string
	surfaces        []initSurface
	extraDirs       []string
}

type initSurface struct {
	name     string
	kind     string
	path     string
	stack    string
	critical bool
}

var (
	projectIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	emailPattern     = regexp.MustCompile(`^[^@\s]+@[^@\s]+[.][^@\s]+$`)
)

func initCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	config := initConfig{
		mode:        "minimal",
		projectID:   "nomos-project",
		projectName: "Nomos Project",
		domain:      "example-domain",
		ownerName:   "Project Owner",
		ownerEmail:  "owner@example.com",
		target:      ".",
	}

	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&config.mode, "mode", config.mode, "initialization mode: minimal or regulated")
	flags.StringVar(&config.projectID, "project-id", config.projectID, "project slug used in Nomos reports")
	flags.StringVar(&config.projectName, "project-name", config.projectName, "human-readable project name")
	flags.StringVar(&config.domain, "domain", config.domain, "business domain name")
	flags.StringVar(&config.ownerName, "owner-name", config.ownerName, "accountable owner name")
	flags.StringVar(&config.ownerEmail, "owner-email", config.ownerEmail, "accountable owner email")
	flags.StringVar(&config.repository, "repository", config.repository, "optional repository URL")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	switch flags.NArg() {
	case 0:
	case 1:
		config.target = flags.Arg(0)
	default:
		fmt.Fprintln(stderr, "init accepts at most one target directory")
		return 2
	}

	if err := runInit(config); err != nil {
		fmt.Fprintf(stderr, "init: %v\n", err)
		return 2
	}

	fmt.Fprintf(stdout, "initialized Nomos %s project in %s\n", config.mode, filepath.Clean(config.target))
	return 0
}

func runInit(config initConfig) error {
	profile, err := profileForMode(config.mode)
	if err != nil {
		return err
	}
	if err := validateInitConfig(config); err != nil {
		return err
	}

	root := filepath.Clean(config.target)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}
	if err := requireEmptyWorkspace(root); err != nil {
		return err
	}

	dirs := []string{
		"docs/canonical",
		"docs/decisions",
		"docs/governance",
		"docs/sources",
		"data/canonical",
		"schemas",
		"src",
		"tests/golden",
		"reports",
		"policies",
		"attestations",
	}
	dirs = append(dirs, profile.extraDirs...)
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	matrixPath := "docs/canonical/" + slugForPath(config.domain) + "-matrix.yaml"
	files := map[string]string{
		"nomos.project.yaml":                     projectManifest(config, profile),
		"docs/canonical/source-manifest.yaml":    sourceManifest(),
		matrixPath:                               canonicalMatrix(),
		"docs/canonical/glossary.md":             glossary(config.domain),
		"docs/governance/domain-risk-profile.md": riskProfile(config, profile),
		"docs/decisions/README.md":               decisionsReadme(),
		"reports/README.md":                      reportsReadme(profile),
		"policies/README.md":                     policiesReadme(profile),
		"attestations/README.md":                 attestationsReadme(profile),
	}
	if profile.regulated {
		files["docs/compliance/evidence-policy.md"] = evidencePolicy(profile)
	}

	for path, content := range files {
		if err := writeNewFile(root, path, content); err != nil {
			return err
		}
	}

	for _, dir := range dirs {
		if hasSeedFile(files, dir) {
			continue
		}
		if err := writeNewFile(root, filepath.ToSlash(filepath.Join(dir, ".gitkeep")), ""); err != nil {
			return err
		}
	}

	return nil
}

func profileForMode(mode string) (initProfile, error) {
	switch mode {
	case "minimal":
		return initProfile{
			riskLevel:       "medium",
			scopeConfidence: "medium",
			dataSensitivity: "internal",
			attestation:     "none",
			requiredReports: []string{"nomos-report.json", "coverage-report.md"},
			surfaces: []initSurface{
				{name: "product-api", kind: "api", path: "src/", stack: "unspecified", critical: true},
			},
		}, nil
	case "regulated":
		return initProfile{
			riskLevel:       "critical",
			scopeConfidence: "high",
			dataSensitivity: "secret",
			attestation:     "signed",
			regulated:       true,
			requiredReports: []string{"nomos-report.json", "coverage-report.md", "attestation", "sbom", "provenance"},
			standards:       []string{"internal-control", "external-audit"},
			surfaces: []initSurface{
				{name: "product-api", kind: "api", path: "src/api/", stack: "unspecified", critical: true},
				{name: "canonical-data", kind: "data", path: "data/canonical/", stack: "yaml", critical: true},
			},
			extraDirs: []string{"docs/compliance", "src/api"},
		}, nil
	default:
		return initProfile{}, fmt.Errorf("unsupported mode %q, expected minimal or regulated", mode)
	}
}

func validateInitConfig(config initConfig) error {
	if !projectIDPattern.MatchString(config.projectID) {
		return fmt.Errorf("project-id %q must match ^[a-z0-9][a-z0-9-]*$", config.projectID)
	}
	for name, value := range map[string]string{
		"project-name": config.projectName,
		"domain":       config.domain,
		"owner-name":   config.ownerName,
		"owner-email":  config.ownerEmail,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s must not be empty", name)
		}
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must be a single line", name)
		}
	}
	if config.repository != "" && strings.ContainsAny(config.repository, "\r\n") {
		return fmt.Errorf("repository must be a single line")
	}
	if !emailPattern.MatchString(config.ownerEmail) {
		return fmt.Errorf("owner-email %q must be a valid email address", config.ownerEmail)
	}
	return nil
}

func requireEmptyWorkspace(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("read target directory: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == ".git" && entry.IsDir() {
			continue
		}
		return fmt.Errorf("target directory %s is not empty", root)
	}
	return nil
}

func writeNewFile(root string, rel string, content string) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", rel)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", rel, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create parent for %s: %w", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	return nil
}

func hasSeedFile(files map[string]string, dir string) bool {
	prefix := strings.TrimSuffix(filepath.ToSlash(dir), "/") + "/"
	for path := range files {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func slugForPath(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, item := range strings.ToLower(value) {
		if (item >= 'a' && item <= 'z') || (item >= '0' && item <= '9') {
			builder.WriteRune(item)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}
	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "domain"
	}
	return slug
}

func yamlString(value string) string {
	return fmt.Sprintf("%q", value)
}

func projectManifest(config initConfig, profile initProfile) string {
	var builder strings.Builder
	fmt.Fprintln(&builder, `schema_version: "0.1.0"`)
	fmt.Fprintln(&builder, "project:")
	fmt.Fprintf(&builder, "  id: %s\n", config.projectID)
	fmt.Fprintf(&builder, "  name: %s\n", yamlString(config.projectName))
	fmt.Fprintln(&builder, "  description: Initial Nomos project manifest.")
	if config.repository != "" {
		fmt.Fprintf(&builder, "  repository: %s\n", yamlString(config.repository))
	}
	fmt.Fprintf(&builder, "  domain: %s\n", yamlString(config.domain))
	fmt.Fprintln(&builder, "  lifecycle: greenfield")
	fmt.Fprintf(&builder, "  risk_level: %s\n", profile.riskLevel)
	fmt.Fprintln(&builder, "  owners:")
	fmt.Fprintf(&builder, "    - name: %s\n", yamlString(config.ownerName))
	fmt.Fprintln(&builder, "      role: product-owner")
	fmt.Fprintf(&builder, "      email: %s\n", yamlString(config.ownerEmail))
	if profile.regulated {
		fmt.Fprintln(&builder, "    - name: Compliance Owner")
		fmt.Fprintln(&builder, "      role: compliance-owner")
		fmt.Fprintln(&builder, "      email: compliance@example.com")
	}
	fmt.Fprintln(&builder, "scope:")
	fmt.Fprintln(&builder, "  verdict: in_scope")
	fmt.Fprintf(&builder, "  confidence: %s\n", profile.scopeConfidence)
	if profile.regulated {
		fmt.Fprintln(&builder, "  bounded_contexts:")
		fmt.Fprintf(&builder, "    - %s\n", yamlString(config.domain))
	}
	fmt.Fprintln(&builder, "  in_scope:")
	fmt.Fprintf(&builder, "    - %s\n", yamlString(config.domain))
	fmt.Fprintln(&builder, "surfaces:")
	for _, surface := range profile.surfaces {
		fmt.Fprintf(&builder, "  - name: %s\n", surface.name)
		fmt.Fprintf(&builder, "    type: %s\n", surface.kind)
		fmt.Fprintf(&builder, "    path: %s\n", yamlString(surface.path))
		fmt.Fprintf(&builder, "    stack: %s\n", yamlString(surface.stack))
		if surface.critical {
			fmt.Fprintln(&builder, "    critical: true")
		}
	}
	fmt.Fprintln(&builder, "toolchain:")
	fmt.Fprintln(&builder, "  build:")
	fmt.Fprintln(&builder, "    - make build")
	fmt.Fprintln(&builder, "  test:")
	fmt.Fprintln(&builder, "    - make test")
	fmt.Fprintln(&builder, "  lint:")
	fmt.Fprintln(&builder, "    - make lint")
	if profile.regulated {
		fmt.Fprintln(&builder, "  typecheck:")
		fmt.Fprintln(&builder, "    - make typecheck")
	}
	fmt.Fprintln(&builder, "compliance:")
	fmt.Fprintf(&builder, "  regulated: %t\n", profile.regulated)
	if len(profile.standards) > 0 {
		fmt.Fprintln(&builder, "  standards:")
		for _, standard := range profile.standards {
			fmt.Fprintf(&builder, "    - %s\n", standard)
		}
	}
	fmt.Fprintf(&builder, "  data_sensitivity: %s\n", profile.dataSensitivity)
	fmt.Fprintln(&builder, "  exceptions_allowed: false")
	fmt.Fprintln(&builder, "evidence:")
	fmt.Fprintln(&builder, "  required_reports:")
	for _, report := range profile.requiredReports {
		fmt.Fprintf(&builder, "    - %s\n", report)
	}
	fmt.Fprintf(&builder, "  attestation_level: %s\n", profile.attestation)
	return builder.String()
}

func sourceManifest() string {
	return `schema_version: "0.1.0"
sources: []
`
}

func canonicalMatrix() string {
	return `schema_version: "0.1.0"
units: []
`
}

func glossary(domain string) string {
	return fmt.Sprintf(`# Glossary

Domain: %s

Add the domain vocabulary Nomos must preserve when reading sources, creating
canonical units, and reporting gaps.
`, domain)
}

func riskProfile(config initConfig, profile initProfile) string {
	return fmt.Sprintf(`# Domain Risk Profile

Project: %s
Domain: %s
Risk level: %s
Regulated: %t

Document the product decisions, human authority boundaries, release gates, and
LLM restrictions before promoting this project beyond initial admission.
`, config.projectName, config.domain, profile.riskLevel, profile.regulated)
}

func decisionsReadme() string {
	return `# Decisions

Record source conflicts, admissible exceptions, and governance decisions here.
`
}

func reportsReadme(profile initProfile) string {
	var builder strings.Builder
	fmt.Fprintln(&builder, "# Reports")
	fmt.Fprintln(&builder, "")
	fmt.Fprintln(&builder, "Expected Nomos evidence reports:")
	fmt.Fprintln(&builder, "")
	for _, report := range profile.requiredReports {
		fmt.Fprintf(&builder, "- `%s`\n", report)
	}
	return builder.String()
}

func policiesReadme(profile initProfile) string {
	if profile.regulated {
		return `# Policies

Regulated mode starts fail-closed: strict gates require coverage, provenance,
signed attestation, and explicit exception records.
`
	}
	return `# Policies

Add local admission and release policies here as the project matures.
`
}

func attestationsReadme(profile initProfile) string {
	return fmt.Sprintf(`# Attestations

Required attestation level: %s
`, profile.attestation)
}

func evidencePolicy(profile initProfile) string {
	var builder strings.Builder
	fmt.Fprintln(&builder, "# Evidence Policy")
	fmt.Fprintln(&builder, "")
	fmt.Fprintln(&builder, "Regulated admission requires:")
	fmt.Fprintln(&builder, "")
	for _, report := range profile.requiredReports {
		fmt.Fprintf(&builder, "- `%s`\n", report)
	}
	fmt.Fprintln(&builder, "- signed human attestation before release promotion")
	return builder.String()
}

package app

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/diagnose"
	"github.com/RBOKproject/Nomos/cli/internal/output"
	"github.com/RBOKproject/Nomos/cli/internal/validate"
)

const Version = "0.1.0-ALPHA"

type commandFunc func(args []string, stdout io.Writer, stderr io.Writer) int

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	commands := map[string]commandFunc{
		"help":          helpCommand,
		"version":       versionCommand,
		"init":          initCommand,
		"validate":      validate.Command,
		"diagnose":      diagnoseCommand,
		"corpus":        corpusCommand,
		"connector":     connectorCommand,
		"atomize":       AtomizeCommand,
		"bundle":        bundleCommand,
		"pack":          packCommand,
		"strict":        StrictGateCommand,
		"check":         checkCommand,
		"report":        ReportCommand,
		"export":        exportCommand,
		"product-check": ProductCheckCommand,
		"github":        githubCommand,
		"evidence":      evidenceCommand,
		"attest":        attestCommand,
	}

	if len(args) == 0 {
		return helpCommand(nil, stdout, stderr)
	}

	name := args[0]
	command, ok := commands[name]
	if !ok {
		fmt.Fprintf(stderr, "unknown command %q\n\n", name)
		helpCommand(nil, stderr, stderr)
		return 2
	}

	return command(args[1:], stdout, stderr)
}

func helpCommand(_ []string, stdout io.Writer, _ io.Writer) int {
	fmt.Fprintln(stdout, "nomos")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Canonical Product Intelligence CLI")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  nomos init [--mode minimal|regulated] [directory]")
	fmt.Fprintln(stdout, "  nomos <command> [options]")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Available commands:")
	fmt.Fprintln(stdout, "  init       Initialize Nomos manifests in a repository")
	fmt.Fprintln(stdout, "  validate   Validate Nomos manifests and schemas")
	fmt.Fprintln(stdout, "  diagnose   Inspect a repository and emit an admission pre-report")
	fmt.Fprintln(stdout, "  corpus     Scan, manifest, validate, diff, feed, and attest source corpora")
	fmt.Fprintln(stdout, "  connector  Read-only live fetch of an open source with hash + span coverage")
	fmt.Fprintln(stdout, "  atomize    Atomize Markdown into atoms/chunks (facets, knowledge-lens scoping)")
	fmt.Fprintln(stdout, "  bundle     Emit a Canonical Knowledge Bundle from a real corpus run")
	fmt.Fprintln(stdout, "  pack       Validate a domain pack against its declarative contract (golden corpus included)")
	fmt.Fprintln(stdout, "  strict     Run the strict release/integrity gate")
	fmt.Fprintln(stdout, "  check      Granular manifest checks (sources, contracts, matrix, exceptions, strict)")
	fmt.Fprintln(stdout, "  report     Generate the project detection report (JSON)")
	fmt.Fprintln(stdout, "  export     Export the project report as an SPDX or CycloneDX BOM")
	fmt.Fprintln(stdout, "  product-check  Validate nomos.project.yaml against product rules")
	fmt.Fprintln(stdout, "  github     GitHub workflow integration (plan scoped diffs)")
	fmt.Fprintln(stdout, "  evidence   Hash, prepare/sign, and verify evidence bundles")
	fmt.Fprintln(stdout, "  attest     Sign and verify attestation predicates (ECDSA P-256 DSSE)")
	fmt.Fprintln(stdout, "  version    Print CLI version")
	fmt.Fprintln(stdout, "  help       Print this help")
	return 0
}

func versionCommand(_ []string, stdout io.Writer, _ io.Writer) int {
	fmt.Fprintln(stdout, Version)
	return 0
}

func notImplemented(name string) commandFunc {
	return func(_ []string, _ io.Writer, stderr io.Writer) int {
		fmt.Fprintf(stderr, "command %q is scaffolded but not implemented yet\n", name)
		return 2
	}
}

func diagnoseCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root to inspect")
	format := flags.String("format", "json", "output format: json or markdown")
	mode := flags.String("mode", "auto", "diagnosis mode: auto, product, or canonical_corpus")
	projectManifest := flags.String("project-manifest", "", "sidecar nomos.project.yaml path outside the inspected root")
	sourceManifest := flags.String("source-manifest", "", "sidecar source-manifest.yaml path outside the inspected root")
	canonicalMatrix := flags.String("canonical-matrix", "", "sidecar canonical-matrix.yaml path outside the inspected root")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if flags.NArg() > 1 {
		fmt.Fprintln(stderr, "diagnose accepts at most one positional root")
		return 2
	}
	if flags.NArg() == 1 {
		if *root != "." {
			fmt.Fprintln(stderr, "use either --root or a positional root, not both")
			return 2
		}
		*root = flags.Arg(0)
	}

	command := append([]string{"nomos", "diagnose"}, args...)
	report, err := diagnose.Diagnose(*root, diagnose.Options{
		Now:                 time.Now().UTC(),
		ToolVersion:         Version,
		Command:             command,
		Mode:                *mode,
		ProjectManifestPath: *projectManifest,
		SourceManifestPath:  *sourceManifest,
		CanonicalMatrixPath: *canonicalMatrix,
	})
	if err != nil {
		fmt.Fprintf(stderr, "diagnose failed: %v\n", err)
		return 1
	}

	switch strings.ToLower(strings.TrimSpace(*format)) {
	case "json":
		if err := output.WriteJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "write json report: %v\n", err)
			return 1
		}
	case "markdown", "md":
		if err := output.WriteMarkdown(stdout, report); err != nil {
			fmt.Fprintf(stderr, "write markdown report: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "unknown diagnose format %q\n", *format)
		return 2
	}
	return 0
}

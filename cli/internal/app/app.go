package app

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/diagnose"
	"github.com/RBOKproject/Nomos/cli/internal/output"
)

const Version = "0.1.0-dev"

type commandFunc func(args []string, stdout io.Writer, stderr io.Writer) int

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	commands := map[string]commandFunc{
		"help":     helpCommand,
		"version":  versionCommand,
		"init":     initCommand,
		"validate": notImplemented("validate"),
		"diagnose": diagnoseCommand,
	}

	if len(args) == 0 {
		return helpCommand(nil, stdout, stderr)
	}

	name := args[0]
	command, ok := commands[name]
	if !ok {
		fmt.Fprintf(stderr, "unknown command %q\n\n", name)
		return helpCommand(nil, stdout, stderr)
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
		Now:         time.Now().UTC(),
		ToolVersion: Version,
		Command:     command,
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

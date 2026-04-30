package app

import (
	"fmt"
	"io"
)

const Version = "0.1.0-dev"

type commandFunc func(stdout io.Writer, stderr io.Writer) int

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	commands := map[string]commandFunc{
		"help":     helpCommand,
		"version":  versionCommand,
		"init":     notImplemented("init"),
		"validate": notImplemented("validate"),
		"diagnose": notImplemented("diagnose"),
	}

	if len(args) == 0 {
		return helpCommand(stdout, stderr)
	}

	name := args[0]
	command, ok := commands[name]
	if !ok {
		fmt.Fprintf(stderr, "unknown command %q\n\n", name)
		return helpCommand(stdout, stderr)
	}

	return command(stdout, stderr)
}

func helpCommand(stdout io.Writer, _ io.Writer) int {
	fmt.Fprintln(stdout, "nomos")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Canonical Product Intelligence CLI")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Usage:")
	fmt.Fprintln(stdout, "  nomos <command>")
	fmt.Fprintln(stdout, "")
	fmt.Fprintln(stdout, "Available commands:")
	fmt.Fprintln(stdout, "  init       Initialize Nomos manifests in a repository")
	fmt.Fprintln(stdout, "  validate   Validate Nomos manifests and schemas")
	fmt.Fprintln(stdout, "  diagnose   Inspect a repository and emit an admission pre-report")
	fmt.Fprintln(stdout, "  version    Print CLI version")
	fmt.Fprintln(stdout, "  help       Print this help")
	return 0
}

func versionCommand(stdout io.Writer, _ io.Writer) int {
	fmt.Fprintln(stdout, Version)
	return 0
}

func notImplemented(name string) commandFunc {
	return func(_ io.Writer, stderr io.Writer) int {
		fmt.Fprintf(stderr, "command %q is scaffolded but not implemented yet\n", name)
		return 2
	}
}

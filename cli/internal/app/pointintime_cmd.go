package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/RBOKproject/Nomos/cli/internal/pointintime"
)

// pointInTimeCommand is `nomos pointintime`: the point-in-time resolver
// (VRC-12 #558, A3). `pointintime resolve` selects the recorded expression in
// force on a project date, or exits 1 (not_in_force) — the engine refuses to
// cite a stale or future version.
func pointInTimeCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: nomos pointintime resolve --atoms <yaml> --work-id <id> --as-of <YYYY-MM-DD>")
		return 2
	}
	switch args[0] {
	case "resolve":
		return pointInTimeResolveCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown pointintime subcommand %q (try: resolve)\n", args[0])
		return 2
	}
}

type pitDoc struct {
	Atoms []pointintime.Atom `json:"atoms"`
}

func pointInTimeResolveCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("pointintime resolve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	atomsPath := flags.String("atoms", "", "YAML atom set (required)")
	workID := flags.String("work-id", "", "FRBR/ELI work identifier (required)")
	asOf := flags.String("as-of", "", "project date YYYY-MM-DD (required)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*atomsPath) == "" || strings.TrimSpace(*workID) == "" || strings.TrimSpace(*asOf) == "" {
		fmt.Fprintln(stderr, "pointintime resolve: --atoms, --work-id and --as-of are required")
		return 2
	}
	raw, err := os.ReadFile(*atomsPath)
	if err != nil {
		fmt.Fprintf(stderr, "pointintime resolve: read atoms: %v\n", err)
		return 1
	}
	var generic any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		fmt.Fprintf(stderr, "pointintime resolve: parse atoms: %v\n", err)
		return 1
	}
	bridged, err := json.Marshal(normalizeYAML(generic))
	if err != nil {
		fmt.Fprintf(stderr, "pointintime resolve: normalize atoms: %v\n", err)
		return 1
	}
	var doc pitDoc
	if err := json.Unmarshal(bridged, &doc); err != nil {
		fmt.Fprintf(stderr, "pointintime resolve: decode atoms: %v\n", err)
		return 1
	}

	result, err := pointintime.ResolvePointInTime(doc.Atoms, *workID, *asOf)
	if err != nil {
		fmt.Fprintf(stderr, "pointintime resolve: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(stderr, "pointintime resolve: write: %v\n", err)
		return 1
	}
	if result.Status != "resolved" {
		return 1
	}
	return 0
}

package app

// versionCommand is `nomos version`: the bare core version, or with --json the
// announcement of what this core reads and writes (from the contract registry),
// the formats it emits and the adapters' verdicts against it (NRT-024 #677).
// The announcement reports; `nomos contracts status` is where an incompatible
// adapter is a refusal.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/RBOKproject/Nomos/cli/internal/contracts"
)

func versionCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "announce core version, schema versions read/written, formats and adapter compatibility")
	repoRoot := flags.String("repo-root", ".", "repository root holding specs/contract-registry.yaml and adapters/")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !*asJSON {
		fmt.Fprintln(stdout, Version)
		return 0
	}
	ann, err := contracts.Announce(*repoRoot, Version)
	if err != nil && ann.CoreVersion == "" {
		fmt.Fprintf(stderr, "version: the contract registry is unavailable at %s — %v; the bare version is %s\n", *repoRoot, err, Version)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ann); err != nil {
		return 1
	}
	if err != nil {
		fmt.Fprintf(stderr, "version: WARNING — %v\n", err)
	}
	return 0
}

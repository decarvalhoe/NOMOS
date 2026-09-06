package app

// contractsCommand is `nomos contracts` (NRT-023 #676): the contract stability
// registry, verified against the tree. A view with teeth: it refuses what
// contradicts the declared stability; it declares nothing itself.

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/contracts"
)

func contractsCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, contractsUsage)
		return 2
	}
	switch args[0] {
	case "status":
		return runContractsStatus(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, contractsUsage)
		return 0
	default:
		fmt.Fprintf(stderr, "contracts: unknown subcommand %q\n%s\n", args[0], contractsUsage)
		return 2
	}
}

const contractsUsage = `usage:
  nomos contracts status --repo-root <dir> [--out <report.json>] [--accept <id> --new-version <v>] [--emit-docs] [--check-docs]
      Verify specs/contract-registry.yaml against the tree: every specs/*.cue and *.schema.json registered,
      hash-at-version intact (a changed stable contract needs an accepted bump), stable contracts carry a
      version and a valid fixture, deprecations carry dates, compatibility fixtures are READ by their Go reader.
      --accept records a deliberate bump (the file must already declare the new version when it declares one).
      Adapters (adapters/*/adapter.nomos.yaml) must include the current core in their declared range and
      demand no schema newer than the core ships — otherwise red. Deprecated contracts are WARNED with their
      removal date. --emit-docs regenerates the compatibility matrix of docs/16; --check-docs exits 4 on drift.`

func runContractsStatus(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("contracts status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	repoRoot := flags.String("repo-root", ".", "repository root")
	out := flags.String("out", "", "write the report here (default: stdout)")
	accept := flags.String("accept", "", "contract id whose deliberate change is accepted")
	newVersion := flags.String("new-version", "", "the new schema_version recorded with --accept")
	emitDocs := flags.Bool("emit-docs", false, "regenerate the compatibility matrix section of "+contracts.MatrixDoc)
	checkDocs := flags.Bool("check-docs", false, "exit 4 when the compatibility matrix section of "+contracts.MatrixDoc+" is not the generated one")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *accept != "" {
		if err := contracts.AcceptBump(*repoRoot, *accept, *newVersion); err != nil {
			fmt.Fprintf(stderr, "contracts status: REFUSED — %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "contracts status: bump accepted for %s → %s (registry updated)\n", *accept, *newVersion)
	}
	rep, err := contracts.Verify(*repoRoot, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "contracts status: RED — %v\n", err)
		return 1
	}
	for _, w := range rep.Warnings {
		fmt.Fprintf(stderr, "contracts status: WARNING — %s\n", w)
	}
	ann, err := contracts.Announce(*repoRoot, Version)
	if err != nil {
		fmt.Fprintf(stderr, "contracts status: RED — %v\n", err)
		return 1
	}
	if *emitDocs {
		if err := contracts.EmitDocs(*repoRoot, ann); err != nil {
			fmt.Fprintf(stderr, "contracts status: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "contracts status: wrote the compatibility matrix into %s\n", contracts.MatrixDoc)
	}
	if *checkDocs {
		if err := contracts.CheckDocs(*repoRoot, ann); err != nil {
			fmt.Fprintf(stderr, "contracts status: DRIFT — %v\n", err)
			return 4
		}
	}
	write := func(w io.Writer) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}
	if *out == "" {
		if err := write(stdout); err != nil {
			return 1
		}
		return 0
	}
	if err := writeFile(*out, write); err != nil {
		fmt.Fprintf(stderr, "contracts status: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "contracts status: OK — %d contract(s): %v; %d compatibility read(s); %d adapter(s) compatible with core %s → %s\n", rep.Total, rep.ByStability, rep.CompatReads, len(ann.Adapters), Version, *out)
	return 0
}

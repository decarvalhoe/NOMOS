package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/RBOKproject/Nomos/cli/internal/canon"
)

// canonCommand is `nomos canon`: the canon-promotion validator (VRC-11 #557,
// A2). `canon validate` renders the verdict in the engine — a user-promoted
// atom enters the silo only under the certified rules, and confidential
// content never leaks to the shared catalog. Exits 1 on any violation.
func canonCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: nomos canon validate --bundle <promotion.yaml>")
		return 2
	}
	switch args[0] {
	case "validate":
		return canonValidateCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown canon subcommand %q (try: validate)\n", args[0])
		return 2
	}
}

func canonValidateCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("canon validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	bundlePath := flags.String("bundle", "", "canon-promotion bundle YAML (required)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if strings.TrimSpace(*bundlePath) == "" {
		fmt.Fprintln(stderr, "canon validate: --bundle is required")
		return 2
	}
	raw, err := os.ReadFile(*bundlePath)
	if err != nil {
		fmt.Fprintf(stderr, "canon validate: read bundle: %v\n", err)
		return 1
	}
	var generic any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		fmt.Fprintf(stderr, "canon validate: parse bundle: %v\n", err)
		return 1
	}
	bridged, err := json.Marshal(normalizeYAML(generic))
	if err != nil {
		fmt.Fprintf(stderr, "canon validate: normalize bundle: %v\n", err)
		return 1
	}
	var bundle canon.PromotionBundle
	if err := json.Unmarshal(bridged, &bundle); err != nil {
		fmt.Fprintf(stderr, "canon validate: decode bundle: %v\n", err)
		return 1
	}

	report := canon.ValidateCanonPromotion(bundle)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(stderr, "canon validate: write: %v\n", err)
		return 1
	}
	if report.Status == "fail" {
		return 1
	}
	return 0
}

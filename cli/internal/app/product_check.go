package app

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/RBOKproject/Nomos/cli/internal/productcheck"
)

// ProductCheckCommand implements the "product-check" CLI command.
// It validates a nomos.project.yaml manifest against product rules.
func ProductCheckCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseProductCheckOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "product-check: %s\n\n", err)
		printProductCheckUsage(stderr)
		return 2
	}

	result, err := productcheck.CheckProduct(opts.manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "product-check: %v\n", err)
		return 1
	}

	writeProductCheckResult(result, opts.format, stdout)
	if result.Valid {
		return 0
	}
	return 1
}

type productCheckOptions struct {
	manifestPath string
	format       string
}

func parseProductCheckOptions(args []string) (productCheckOptions, error) {
	opts := productCheckOptions{format: "text"}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("--format requires a value")
			}
			i++
			opts.format = args[i]
		case strings.HasPrefix(arg, "--format="):
			opts.format = strings.TrimPrefix(arg, "--format=")
		case arg == "--help" || arg == "-h":
			return opts, fmt.Errorf("usage requested")
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown option %q", arg)
		default:
			if opts.manifestPath != "" {
				return opts, fmt.Errorf("only one manifest path is accepted")
			}
			opts.manifestPath = arg
		}
	}

	if opts.format != "text" && opts.format != "json" {
		return opts, fmt.Errorf("unsupported format %q, expected text or json", opts.format)
	}
	if opts.manifestPath == "" {
		return opts, fmt.Errorf("a manifest path is required")
	}

	return opts, nil
}

func printProductCheckUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  nomos product-check [--format text|json] <nomos.project.yaml>")
}

func writeProductCheckResult(result productcheck.CheckResult, format string, w io.Writer) {
	if format == "json" {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(result)
		return
	}

	if result.Valid {
		fmt.Fprintln(w, "ok: product manifest is valid")
		return
	}

	fmt.Fprintf(w, "invalid: %d error(s) found\n", len(result.Errors))
	for _, e := range result.Errors {
		fmt.Fprintf(w, "  - [%s] %s: %s\n", e.Code, e.Path, e.Message)
	}
}

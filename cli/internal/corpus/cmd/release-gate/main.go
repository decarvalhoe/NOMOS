package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/RBOKproject/Nomos/cli/internal/corpus"
)

func main() {
	artifacts := flag.String("artifacts", "", "path to evidence artifacts directory")
	profile := flag.String("profile", "rbok-lawbook", "release gate profile")
	flag.Parse()

	if *artifacts == "" {
		fmt.Fprintln(os.Stderr, "usage: release-gate --artifacts <dir> [--profile rbok-lawbook]")
		os.Exit(2)
	}

	var config corpus.ReleaseGateConfig
	switch *profile {
	case "rbok-lawbook":
		config = corpus.DefaultRBOKLawbookGateConfig(*artifacts)
	default:
		fmt.Fprintf(os.Stderr, "unknown profile: %s\n", *profile)
		os.Exit(2)
	}

	result, err := corpus.EvaluateReleaseGate(config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "release gate error: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(result)

	if result.Verdict == corpus.GateFail {
		fmt.Fprintf(os.Stderr, "\nRELEASE GATE FAILED: %d blocking check(s)\n", result.Blocking)
		for _, c := range result.Checks {
			if c.Verdict == corpus.GateFail {
				fmt.Fprintf(os.Stderr, "  FAIL: %s — %s\n", c.Name, c.Detail)
			}
		}
		os.Exit(1)
	}

	if result.Verdict == corpus.GateWarn {
		fmt.Fprintf(os.Stderr, "\nRELEASE GATE PASSED WITH WARNINGS: %d warning(s)\n", result.Warnings)
	} else {
		fmt.Fprintln(os.Stderr, "\nRELEASE GATE PASSED")
	}
}

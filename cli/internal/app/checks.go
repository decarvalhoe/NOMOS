package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/checks"
	"github.com/RBOKproject/Nomos/cli/internal/checks/contracts"
	"github.com/RBOKproject/Nomos/cli/internal/exceptions"
	"github.com/RBOKproject/Nomos/cli/internal/strict"
)

// SourcesCheckCommand implements "nomos sources check".
func SourcesCheckCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("sources check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "output format: text or json")
	baseDir := flags.String("base-dir", "", "base directory for resolving source paths")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() < 1 {
		fmt.Fprintln(stderr, "sources check: at least one manifest path is required")
		return 2
	}

	manifestPath := flags.Arg(0)
	result, err := checks.CheckSources(manifestPath, *baseDir)
	if err != nil {
		fmt.Fprintf(stderr, "sources check: %v\n", err)
		return 1
	}

	writeSourcesResult(result, *format, stdout)
	if result.Valid {
		return 0
	}
	return 1
}

// ContractsCheckCommand implements "nomos contracts check".
func ContractsCheckCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("contracts check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "output format: text or json")
	baseDir := flags.String("base-dir", "", "base directory for resolving contract paths")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() < 1 {
		fmt.Fprintln(stderr, "contracts check: at least one manifest path is required")
		return 2
	}

	manifestPath := flags.Arg(0)
	result, err := contracts.CheckContracts(manifestPath, *baseDir)
	if err != nil {
		fmt.Fprintf(stderr, "contracts check: %v\n", err)
		return 1
	}

	writeContractsResult(result, *format, stdout)
	if result.Valid {
		return 0
	}
	return 1
}


// MatrixCheckCommand implements "nomos matrix check".
func MatrixCheckCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("matrix check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "output format: text or json")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() < 1 {
		fmt.Fprintln(stderr, "matrix check: at least one manifest path is required")
		return 2
	}

	manifestPath := flags.Arg(0)
	matrix, err := checks.ParseMatrixFile(manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "matrix check: %v\n", err)
		return 1
	}

	result := checks.CheckMatrix(matrix)
	writeMatrixResult(result, *format, stdout)
	if len(result.Findings) == 0 {
		return 0
	}
	return 1
}

// StrictCommand implements "nomos strict".
func StrictCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("strict", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "output format: text or json")
	projectPath := flags.String("project", "", "path to nomos.project.yaml")
	sourcesPath := flags.String("sources", "", "path to source-manifest.yaml")
	matrixPath := flags.String("matrix", "", "path to canonical-matrix.yaml")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *projectPath == "" && *sourcesPath == "" && *matrixPath == "" {
		fmt.Fprintln(stderr, "strict: at least one of --project, --sources, or --matrix is required")
		return 2
	}

	result, err := strict.Check(strict.StrictInput{
		ProjectPath: *projectPath,
		SourcesPath: *sourcesPath,
		MatrixPath:  *matrixPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "strict: %v\n", err)
		return 1
	}

	writeStrictResult(result, *format, stdout)
	if result.Valid {
		return 0
	}
	return 1
}

// ExceptionsCheckCommand implements "nomos exceptions check".
func ExceptionsCheckCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("exceptions check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "output format: text or json")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() < 1 {
		fmt.Fprintln(stderr, "exceptions check: at least one manifest path is required")
		return 2
	}

	manifestPath := flags.Arg(0)
	result, err := exceptions.CheckExceptions(manifestPath, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "exceptions check: %v\n", err)
		return 1
	}

	writeExceptionsResult(result, *format, stdout)
	if result.Valid {
		return 0
	}
	return 1
}

// --- output writers ---

func writeSourcesResult(result checks.CheckResult, format string, w io.Writer) {
	if format == "json" {
		writeJSON(w, result)
		return
	}
	if result.Valid {
		fmt.Fprintln(w, "sources check: ok")
		return
	}
	fmt.Fprintln(w, "sources check: FAILED")
	for _, src := range result.Sources {
		for _, e := range src.Errors {
			fmt.Fprintf(w, "  [%s] %s: %s\n", e.Code, e.SourceID, e.Message)
		}
	}
}

func writeContractsResult(result contracts.CheckResult, format string, w io.Writer) {
	if format == "json" {
		writeJSON(w, result)
		return
	}
	if result.Valid {
		fmt.Fprintln(w, "contracts check: ok")
		return
	}
	fmt.Fprintln(w, "contracts check: FAILED")
	for _, u := range result.Units {
		for _, e := range u.Errors {
			fmt.Fprintf(w, "  [%s] %s: %s\n", e.Code, e.UnitID, e.Message)
		}
	}
}


func writeMatrixResult(result checks.MatrixCheckResult, format string, w io.Writer) {
	if format == "json" {
		writeJSON(w, result)
		return
	}
	if len(result.Findings) == 0 {
		fmt.Fprintln(w, "matrix check: ok")
		return
	}
	fmt.Fprintln(w, "matrix check: FAILED")
	for _, f := range result.Findings {
		fmt.Fprintf(w, "  [%s] %s: %s\n", f.Code, f.UnitID, f.Message)
	}
}

func writeStrictResult(result strict.StrictResult, format string, w io.Writer) {
	if format == "json" {
		writeJSON(w, result)
		return
	}
	if result.Valid {
		fmt.Fprintln(w, "strict: ok")
		return
	}
	fmt.Fprintln(w, "strict: FAILED")
	for _, e := range result.Errors {
		fmt.Fprintf(w, "  [%s] %s\n", e.Code, e.Message)
	}
}

func writeExceptionsResult(result exceptions.CheckResult, format string, w io.Writer) {
	if format == "json" {
		writeJSON(w, result)
		return
	}
	if result.Valid {
		fmt.Fprintln(w, "exceptions check: ok")
		return
	}
	fmt.Fprintln(w, "exceptions check: FAILED")
	for _, exc := range result.Exceptions {
		for _, e := range exc.Errors {
			fmt.Fprintf(w, "  [%s] %s: %s\n", e.Code, e.ExceptionID, e.Message)
		}
	}
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

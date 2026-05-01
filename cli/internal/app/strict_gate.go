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
	"github.com/RBOKproject/Nomos/cli/internal/productcheck"
	"github.com/RBOKproject/Nomos/cli/internal/strict"
)

// GateResult aggregates all check results for a release gate.
type GateResult struct {
	Valid      bool            `json:"valid"`
	Sections  []GateSection   `json:"sections"`
}

// GateSection represents one check phase in the gate.
type GateSection struct {
	Name   string      `json:"name"`
	Valid  bool        `json:"valid"`
	Errors []GateError `json:"errors,omitempty"`
}

// GateError is a single error within a gate section.
type GateError struct {
	Code    string `json:"code"`
	ID      string `json:"id,omitempty"`
	Message string `json:"message"`
}

// StrictGateCommand implements the aggregated "nomos strict" release gate.
// It runs all blocking checks in sequence and reports a unified result.
func StrictGateCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	flags := flag.NewFlagSet("strict", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "text", "output format: text or json")
	projectPath := flags.String("project", "", "path to nomos.project.yaml")
	sourcesPath := flags.String("sources", "", "path to source-manifest.yaml")
	matrixPath := flags.String("matrix", "", "path to canonical-matrix.yaml")
	exceptionsPath := flags.String("exceptions", "", "path to exceptions.yaml")
	baseDir := flags.String("base-dir", "", "base directory for resolving file paths")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	if *projectPath == "" && *sourcesPath == "" && *matrixPath == "" {
		fmt.Fprintln(stderr, "strict: at least one of --project, --sources, or --matrix is required")
		return 2
	}

	result := runGate(*projectPath, *sourcesPath, *matrixPath, *exceptionsPath, *baseDir)

	writeGateResult(result, *format, stdout)
	if result.Valid {
		return 0
	}
	return 1
}

func runGate(projectPath, sourcesPath, matrixPath, exceptionsPath, baseDir string) GateResult {
	gate := GateResult{Valid: true}

	if projectPath != "" {
		section := runProductCheck(projectPath)
		gate.Sections = append(gate.Sections, section)
		if !section.Valid {
			gate.Valid = false
		}
	}

	if sourcesPath != "" {
		section := runSourcesCheck(sourcesPath, baseDir)
		gate.Sections = append(gate.Sections, section)
		if !section.Valid {
			gate.Valid = false
		}
	}

	if matrixPath != "" {
		section := runMatrixCheck(matrixPath)
		gate.Sections = append(gate.Sections, section)
		if !section.Valid {
			gate.Valid = false
		}

		section = runContractsCheck(matrixPath, baseDir)
		gate.Sections = append(gate.Sections, section)
		if !section.Valid {
			gate.Valid = false
		}
	}

	if projectPath != "" || sourcesPath != "" || matrixPath != "" {
		section := runCrossCheck(projectPath, sourcesPath, matrixPath)
		gate.Sections = append(gate.Sections, section)
		if !section.Valid {
			gate.Valid = false
		}
	}

	if exceptionsPath != "" {
		section := runExceptionsCheck(exceptionsPath)
		gate.Sections = append(gate.Sections, section)
		if !section.Valid {
			gate.Valid = false
		}
	}

	return gate
}

func runProductCheck(path string) GateSection {
	section := GateSection{Name: "product-check", Valid: true}
	result, err := productcheck.CheckProduct(path)
	if err != nil {
		section.Valid = false
		section.Errors = append(section.Errors, GateError{
			Code: "LOAD_ERROR", Message: err.Error(),
		})
		return section
	}
	if !result.Valid {
		section.Valid = false
		for _, e := range result.Errors {
			section.Errors = append(section.Errors, GateError{
				Code: e.Code, ID: e.Path, Message: e.Message,
			})
		}
	}
	return section
}

func runSourcesCheck(path string, baseDir string) GateSection {
	section := GateSection{Name: "sources-check", Valid: true}
	result, err := checks.CheckSources(path, baseDir)
	if err != nil {
		section.Valid = false
		section.Errors = append(section.Errors, GateError{
			Code: "LOAD_ERROR", Message: err.Error(),
		})
		return section
	}
	if !result.Valid {
		section.Valid = false
		for _, src := range result.Sources {
			for _, e := range src.Errors {
				section.Errors = append(section.Errors, GateError{
					Code: e.Code, ID: e.SourceID, Message: e.Message,
				})
			}
		}
	}
	return section
}

func runMatrixCheck(path string) GateSection {
	section := GateSection{Name: "matrix-check", Valid: true}
	matrix, err := checks.ParseMatrixFile(path)
	if err != nil {
		section.Valid = false
		section.Errors = append(section.Errors, GateError{
			Code: "LOAD_ERROR", Message: err.Error(),
		})
		return section
	}
	result := checks.CheckMatrix(matrix)
	if len(result.Findings) > 0 {
		section.Valid = false
		for _, f := range result.Findings {
			section.Errors = append(section.Errors, GateError{
				Code: f.Code, ID: f.UnitID, Message: f.Message,
			})
		}
	}
	return section
}

func runContractsCheck(path string, baseDir string) GateSection {
	section := GateSection{Name: "contracts-check", Valid: true}
	result, err := contracts.CheckContracts(path, baseDir)
	if err != nil {
		section.Valid = false
		section.Errors = append(section.Errors, GateError{
			Code: "LOAD_ERROR", Message: err.Error(),
		})
		return section
	}
	if !result.Valid {
		section.Valid = false
		for _, u := range result.Units {
			for _, e := range u.Errors {
				section.Errors = append(section.Errors, GateError{
					Code: e.Code, ID: e.UnitID, Message: e.Message,
				})
			}
		}
	}
	return section
}

func runCrossCheck(projectPath, sourcesPath, matrixPath string) GateSection {
	section := GateSection{Name: "cross-check", Valid: true}
	result, err := strict.Check(strict.StrictInput{
		ProjectPath: projectPath,
		SourcesPath: sourcesPath,
		MatrixPath:  matrixPath,
	})
	if err != nil {
		section.Valid = false
		section.Errors = append(section.Errors, GateError{
			Code: "LOAD_ERROR", Message: err.Error(),
		})
		return section
	}
	if !result.Valid {
		section.Valid = false
		for _, e := range result.Errors {
			section.Errors = append(section.Errors, GateError{
				Code: e.Code, Message: e.Message,
			})
		}
	}
	return section
}

func runExceptionsCheck(path string) GateSection {
	section := GateSection{Name: "exceptions-check", Valid: true}
	result, err := exceptions.CheckExceptions(path, time.Now().UTC())
	if err != nil {
		section.Valid = false
		section.Errors = append(section.Errors, GateError{
			Code: "LOAD_ERROR", Message: err.Error(),
		})
		return section
	}
	if !result.Valid {
		section.Valid = false
		for _, exc := range result.Exceptions {
			for _, e := range exc.Errors {
				section.Errors = append(section.Errors, GateError{
					Code: e.Code, ID: e.ExceptionID, Message: e.Message,
				})
			}
		}
	}
	return section
}

func writeGateResult(result GateResult, format string, w io.Writer) {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(result)
		return
	}

	for _, section := range result.Sections {
		if section.Valid {
			fmt.Fprintf(w, "  %-20s ok\n", section.Name)
			continue
		}
		fmt.Fprintf(w, "  %-20s FAILED\n", section.Name)
		for _, e := range section.Errors {
			if e.ID != "" {
				fmt.Fprintf(w, "    [%s] %s: %s\n", e.Code, e.ID, e.Message)
			} else {
				fmt.Fprintf(w, "    [%s] %s\n", e.Code, e.Message)
			}
		}
	}

	fmt.Fprintln(w)
	if result.Valid {
		fmt.Fprintln(w, "strict: PASS — release gate green")
	} else {
		fmt.Fprintln(w, "strict: FAIL — release blocked")
	}
}

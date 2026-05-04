package app

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/checks"
	"github.com/RBOKproject/Nomos/cli/internal/checks/contracts"
	"github.com/RBOKproject/Nomos/cli/internal/corpus"
	"github.com/RBOKproject/Nomos/cli/internal/exceptions"
	"github.com/RBOKproject/Nomos/cli/internal/productcheck"
	"github.com/RBOKproject/Nomos/cli/internal/strict"
)

// GateResult aggregates all check results for a release gate.
//
// SFI-08 (#346): CorpusIntegrityCheck is an optional, additive section. It
// is omitted from the JSON entirely when none of the --corpus-integrity-*
// flags are passed, so the existing wire format is preserved for callers
// that do not opt into the corpus checks.
type GateResult struct {
	Valid                bool                  `json:"valid"`
	Sections             []GateSection         `json:"sections"`
	CorpusIntegrityCheck *CorpusIntegrityCheck `json:"corpus_integrity_check,omitempty"`
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

// CorpusIntegrityCheck is the strict-gate facing summary of the SFI-04 source
// integrity gate (#342) and the SFI-07 feed quality gate (#345). Either
// sub-report may be nil if its inputs were not supplied. Status is "pass"
// when every supplied sub-report passed, "fail" otherwise.
//
// FSQ-05 (#368): when a body ledger is supplied via --corpus-body-ledger,
// any uncovered text bytes for an admitted+atomized source surface here
// as BodyLedgerFindings. The ledger itself is NOT attached as a separate
// section; per dispatch, the ledger is evidence consumed by this section.
type CorpusIntegrityCheck struct {
	Status             string                    `json:"status"`
	SourceIntegrity    *corpus.IntegrityReport   `json:"source_integrity"`
	FeedQuality        *corpus.FeedQualityReport `json:"feed_quality"`
	BodyLedgerFindings []corpus.IntegrityFinding `json:"body_ledger_findings,omitempty"`
	Summary            string                    `json:"summary"`
}

// FindingBodyLedgerUncoveredTextSource is the stable code emitted when a
// body ledger reports uncovered bytes for a source that the operator
// declared admitted+atomized (or admitted+coverage_only). FSQ-05 (#368):
// the dispatch fixes this exact code, distinct from the SFI-04
// SOURCE_UNCOVERED_RANGE finding so consumers can tell ledger-vs-segment
// fidelity defects apart.
const FindingBodyLedgerUncoveredTextSource = "BODY_LEDGER_UNCOVERED_TEXT_SOURCE"

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
	integrityReportPath := flags.String("corpus-integrity-report", "", "path to a precomputed JSON integrity/quality report (single or aggregate)")
	integritySourceDir := flags.String("corpus-integrity-source", "", "path to a directory of *.md source files; computes the source integrity report on the fly")
	integrityFeedPath := flags.String("corpus-integrity-feed", "", "path to a JSON []FeedUnit; combined with --corpus-integrity-source to compute the feed quality report")
	integrityRAGPath := flags.String("corpus-integrity-rag", "", "path to a JSON []ChunkMetadata; combined with --corpus-integrity-source to compute the feed quality report")
	bodyLedgerPath := flags.String("corpus-body-ledger", "", "path to a JSON CorpusBodyLedger (FSQ-05 #368); when supplied, uncovered text bytes for admitted+atomized sources fail the integrity check")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	integrityRequested := *integrityReportPath != "" ||
		*integritySourceDir != "" ||
		*integrityFeedPath != "" ||
		*integrityRAGPath != "" ||
		*bodyLedgerPath != ""

	if *projectPath == "" && *sourcesPath == "" && *matrixPath == "" && !integrityRequested {
		fmt.Fprintln(stderr, "strict: at least one of --project, --sources, --matrix, or --corpus-integrity-* is required")
		return 2
	}

	result := runGate(*projectPath, *sourcesPath, *matrixPath, *exceptionsPath, *baseDir)

	if integrityRequested {
		check := runCorpusIntegrityCheck(
			*integrityReportPath,
			*integritySourceDir,
			*integrityFeedPath,
			*integrityRAGPath,
		)
		if *bodyLedgerPath != "" {
			applyBodyLedgerToIntegrityCheck(check, *bodyLedgerPath)
		}
		result.CorpusIntegrityCheck = check
		if check.Status == "fail" {
			result.Valid = false
		}
	}

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

// runCorpusIntegrityCheck runs whichever combination of the SFI-04 source
// integrity gate and the SFI-07 feed quality gate is requested via the
// --corpus-integrity-* flags. It never returns nil. On a load/parse failure
// it produces a fail-status check with the error captured in Summary, so
// the caller can simply read check.Status to decide the gate outcome.
func runCorpusIntegrityCheck(reportPath, sourceDir, feedPath, ragPath string) *CorpusIntegrityCheck {
	check := &CorpusIntegrityCheck{Status: "pass"}

	if reportPath != "" {
		si, fq, err := loadIntegrityReportFile(reportPath)
		if err != nil {
			return &CorpusIntegrityCheck{
				Status:  "fail",
				Summary: fmt.Sprintf("corpus-integrity-report load failed: %v", err),
			}
		}
		check.SourceIntegrity = si
		check.FeedQuality = fq
	}

	if sourceDir != "" {
		si, fq, err := computeIntegrityFromSources(sourceDir, feedPath, ragPath)
		if err != nil {
			return &CorpusIntegrityCheck{
				Status:  "fail",
				Summary: fmt.Sprintf("corpus-integrity-source compute failed: %v", err),
			}
		}
		if si != nil {
			check.SourceIntegrity = si
		}
		if fq != nil {
			check.FeedQuality = fq
		}
	}

	parts := []string{}
	if check.SourceIntegrity != nil {
		if check.SourceIntegrity.Status != "pass" {
			check.Status = "fail"
		}
		parts = append(parts, fmt.Sprintf("source_integrity=%s (%d findings)",
			check.SourceIntegrity.Status, len(check.SourceIntegrity.Findings)))
	}
	if check.FeedQuality != nil {
		if check.FeedQuality.Status != "pass" {
			check.Status = "fail"
		}
		parts = append(parts, fmt.Sprintf("feed_quality=%s (%d findings)",
			check.FeedQuality.Status, len(check.FeedQuality.Findings)))
	}
	if len(parts) == 0 {
		check.Summary = "no integrity sub-reports produced"
	} else {
		check.Summary = strings.Join(parts, "; ")
	}
	return check
}

// applyBodyLedgerToIntegrityCheck loads the FSQ-05 body ledger from path,
// then appends a BODY_LEDGER_UNCOVERED_TEXT_SOURCE finding for every source
// that has admission_status=admitted, atomization_status in {atomized,
// coverage_only}, and uncovered_bytes > 0. The check's Status is set to
// "fail" if any such finding is appended (or if the ledger fails to parse).
// The Summary string is augmented with a body-ledger sentence so the
// strict-gate text output explains the failure cause.
func applyBodyLedgerToIntegrityCheck(check *CorpusIntegrityCheck, path string) {
	if check == nil {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		check.Status = "fail"
		check.Summary = appendSummary(check.Summary, fmt.Sprintf("body_ledger=fail (read %s: %v)", path, err))
		return
	}
	var ledger corpus.CorpusBodyLedger
	if err := json.Unmarshal(raw, &ledger); err != nil {
		check.Status = "fail"
		check.Summary = appendSummary(check.Summary, fmt.Sprintf("body_ledger=fail (parse %s: %v)", path, err))
		return
	}
	for _, src := range ledger.Sources {
		if src.AdmissionStatus != corpus.AdmissionAdmitted {
			continue
		}
		if src.AtomizationStatus != corpus.AtomizationAtomized &&
			src.AtomizationStatus != corpus.AtomizationCoverageOnly {
			continue
		}
		if src.ByteCoverage.UncoveredBytes <= 0 {
			continue
		}
		check.BodyLedgerFindings = append(check.BodyLedgerFindings, corpus.IntegrityFinding{
			Code:     FindingBodyLedgerUncoveredTextSource,
			SourceID: src.SourceID,
			Message: fmt.Sprintf(
				"source %q has %d uncovered byte(s); admission=%s atomization=%s",
				src.SourceID, src.ByteCoverage.UncoveredBytes,
				src.AdmissionStatus, src.AtomizationStatus,
			),
		})
	}
	if len(check.BodyLedgerFindings) > 0 {
		check.Status = "fail"
		check.Summary = appendSummary(check.Summary, fmt.Sprintf(
			"body_ledger=fail (%d uncovered text source(s))", len(check.BodyLedgerFindings),
		))
	} else {
		check.Summary = appendSummary(check.Summary, "body_ledger=pass")
	}
}

// appendSummary joins a new summary fragment to an existing summary
// string with the same "; "-separator the source/feed sub-reports use.
func appendSummary(existing, fragment string) string {
	if strings.TrimSpace(existing) == "" {
		return fragment
	}
	return existing + "; " + fragment
}

// loadIntegrityReportFile parses the JSON file produced by an upstream run
// of the SFI-04 / SFI-07 gates. Three shapes are accepted:
//
//   - aggregate: {"source_integrity": ..., "feed_quality": ...}
//   - single source-integrity report (carries source_count)
//   - single feed-quality report (carries feed_unit_count)
//
// Either or both report pointers may be nil if a key is absent.
func loadIntegrityReportFile(path string) (*corpus.IntegrityReport, *corpus.FeedQualityReport, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", path, err)
	}
	var agg struct {
		SourceIntegrity *corpus.IntegrityReport   `json:"source_integrity"`
		FeedQuality     *corpus.FeedQualityReport `json:"feed_quality"`
	}
	if err := json.Unmarshal(raw, &agg); err == nil {
		if agg.SourceIntegrity != nil || agg.FeedQuality != nil {
			return agg.SourceIntegrity, agg.FeedQuality, nil
		}
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if _, ok := probe["source_count"]; ok {
		var r corpus.IntegrityReport
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, nil, fmt.Errorf("parse source integrity report %s: %w", path, err)
		}
		return &r, nil, nil
	}
	if _, ok := probe["feed_unit_count"]; ok {
		var r corpus.FeedQualityReport
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, nil, fmt.Errorf("parse feed quality report %s: %w", path, err)
		}
		return nil, &r, nil
	}
	return nil, nil, fmt.Errorf("integrity report %s: shape not recognised (no source_integrity, feed_quality, source_count, or feed_unit_count keys)", path)
}

// computeIntegrityFromSources walks sourceDir for *.md files, runs the typed
// markdown scanner over each, and produces a SFI-04 IntegrityReport. When
// feedPath or ragPath is also supplied, it additionally runs CheckFeedQuality
// against the resulting segment ledger.
func computeIntegrityFromSources(sourceDir, feedPath, ragPath string) (*corpus.IntegrityReport, *corpus.FeedQualityReport, error) {
	info, err := os.Stat(sourceDir)
	if err != nil {
		return nil, nil, fmt.Errorf("stat source dir %s: %w", sourceDir, err)
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("--corpus-integrity-source %s: not a directory", sourceDir)
	}
	var sources []corpus.SourceInput
	var allSegments []corpus.SourceSegment
	walkErr := filepath.WalkDir(sourceDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(d.Name()), ".md") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		rel, relErr := filepath.Rel(sourceDir, path)
		if relErr != nil {
			rel = filepath.Base(path)
		}
		sourceID := filepath.ToSlash(rel)
		segs, err := corpus.ScanMarkdown(sourceID, path, content)
		if err != nil {
			return fmt.Errorf("scan %s: %w", path, err)
		}
		sources = append(sources, corpus.SourceInput{
			SourceID: sourceID,
			Path:     path,
			Content:  content,
		})
		allSegments = append(allSegments, segs...)
		return nil
	})
	if walkErr != nil {
		return nil, nil, walkErr
	}
	si := corpus.CheckSourceIntegrity(sources, allSegments)

	var fq *corpus.FeedQualityReport
	if feedPath != "" || ragPath != "" {
		var feedUnits []corpus.FeedUnit
		var chunks []corpus.ChunkMetadata
		if feedPath != "" {
			data, err := os.ReadFile(feedPath)
			if err != nil {
				return &si, nil, fmt.Errorf("read feed %s: %w", feedPath, err)
			}
			if err := json.Unmarshal(data, &feedUnits); err != nil {
				return &si, nil, fmt.Errorf("parse feed %s: %w", feedPath, err)
			}
		}
		if ragPath != "" {
			data, err := os.ReadFile(ragPath)
			if err != nil {
				return &si, nil, fmt.Errorf("read rag %s: %w", ragPath, err)
			}
			if err := json.Unmarshal(data, &chunks); err != nil {
				return &si, nil, fmt.Errorf("parse rag %s: %w", ragPath, err)
			}
		}
		r := corpus.CheckFeedQuality(corpus.FeedQualityInput{
			FeedUnits: feedUnits,
			Chunks:    chunks,
			Segments:  allSegments,
		})
		fq = &r
	}
	return &si, fq, nil
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

	if result.CorpusIntegrityCheck != nil {
		c := result.CorpusIntegrityCheck
		marker := "ok"
		if c.Status != "pass" {
			marker = "FAILED"
		}
		fmt.Fprintf(w, "  %-20s %s\n", "corpus-integrity", marker)
		if c.Summary != "" {
			fmt.Fprintf(w, "    %s\n", c.Summary)
		}
	}

	fmt.Fprintln(w)
	if result.Valid {
		fmt.Fprintln(w, "strict: PASS — release gate green")
	} else {
		fmt.Fprintln(w, "strict: FAIL — release blocked")
	}
}

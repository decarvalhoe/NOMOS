package report

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/detect"
)

const (
	SchemaVersion = "0.1.0"
	ReportType    = "nomos-report"
)

// Options configures report generation.
type Options struct {
	ProjectID   string
	ProjectName string
	Domain      string
	RiskLevel   string
	Mode        string
	ToolVersion string
	Command     []string
}

// Generate produces a NomosReport from a detect.Report.
func Generate(dr detect.Report, opts Options) NomosReport {
	now := time.Now().UTC()
	runID := fmt.Sprintf("run-%s", now.Format("20060102-150405"))

	mode := opts.Mode
	if mode == "" {
		mode = "report"
	}

	checks, findings, evidence := buildChecks(dr, now)

	blockingCount := 0
	for _, f := range findings {
		if f.Blocking {
			blockingCount++
		}
	}

	verdictStatus, verdictSeverity, verdictBlocking, verdictSummary := computeVerdict(checks, findings)

	return NomosReport{
		SchemaVersion: SchemaVersion,
		ReportType:    ReportType,
		GeneratedAt:   now.Format(time.RFC3339),
		Run: Run{
			ID:   runID,
			Mode: mode,
			Tool: Tool{
				Name:    "nomos",
				Version: opts.ToolVersion,
			},
			Command: opts.Command,
			Environment: &Environment{
				OS: runtime.GOOS,
			},
		},
		Project: Project{
			ID:        opts.ProjectID,
			Name:      opts.ProjectName,
			Domain:    opts.Domain,
			RiskLevel: opts.RiskLevel,
		},
		Summary: Summary{
			CheckCount:           len(checks),
			FindingCount:         len(findings),
			BlockingFindingCount: blockingCount,
			EvidenceCount:        len(evidence),
			Coverage: CoverageSummary{
				CoverageRatio: 1,
			},
		},
		Verdict: Verdict{
			Status:   verdictStatus,
			Severity: verdictSeverity,
			Blocking: verdictBlocking,
			Summary:  verdictSummary,
		},
		Checks:   checks,
		Findings: findings,
		Evidence: evidence,
	}
}

func buildChecks(dr detect.Report, now time.Time) ([]CheckResult, []Finding, []EvidenceItem) {
	var checks []CheckResult
	var findings []Finding
	var evidence []EvidenceItem

	evidenceIdx := 0
	findingIdx := 0

	// Check: language detection
	{
		status := "passed"
		msg := fmt.Sprintf("Detected %d language(s).", len(dr.Languages))
		if len(dr.Languages) == 0 {
			status = "warning"
			msg = "No programming languages detected."
		}
		checks = append(checks, CheckResult{
			ID:       "sources.languages",
			Name:     "Language detection",
			Category: "sources",
			Status:   status,
			Severity: "info",
			Message:  msg,
		})
	}

	// Check: surface detection
	{
		status := "passed"
		msg := fmt.Sprintf("Detected %d surface(s).", len(dr.Surfaces))
		if len(dr.Surfaces) == 0 {
			status = "warning"
			msg = "No surfaces detected."
		}
		checks = append(checks, CheckResult{
			ID:       "sources.surfaces",
			Name:     "Surface detection",
			Category: "sources",
			Status:   status,
			Severity: "info",
			Message:  msg,
		})
	}

	// Check: hardcoded catalogue detection via node-typescript adapter
	for _, signal := range dr.NodeTypeScript.Findings {
		if signal.Kind != "hardcoded_catalog_detection" {
			continue
		}

		evidenceIdx++
		evidenceID := fmt.Sprintf("E-%03d", evidenceIdx)
		evPath := ""
		if len(signal.Evidence) > 0 {
			evPath = signal.Evidence[0].Path
		}
		evidence = append(evidence, EvidenceItem{
			ID:          evidenceID,
			Type:        "code_reference",
			Description: fmt.Sprintf("Hardcoded catalogue detected: %s", signal.Name),
			Target: &Target{
				Type: "code",
				Path: evPath,
			},
			CollectedAt: now.Format(time.RFC3339),
			Producer:    "nomos-adapter-node-typescript",
		})

		findingIdx++
		findingID := fmt.Sprintf("F-%03d", findingIdx)
		findings = append(findings, Finding{
			ID:       findingID,
			Code:     "NOMOS_PRODUCT_HARDCODED_CATALOG",
			Severity: "medium",
			Status:   "open",
			Blocking: false,
			Message:  fmt.Sprintf("Hardcoded catalogue %q found at %s.", signal.Name, evPath),
			Remediation: "Replace with a canonical read-model or link to an authoritative source.",
			Target: Target{
				Type: "code",
				Path: evPath,
			},
			EvidenceIDs: []string{evidenceID},
		})
	}

	if len(findings) > 0 {
		var findingIDs []string
		var evidenceIDs []string
		for _, f := range findings {
			findingIDs = append(findingIDs, f.ID)
			evidenceIDs = append(evidenceIDs, f.EvidenceIDs...)
		}
		checks = append(checks, CheckResult{
			ID:          "product.hardcoded-catalogs",
			Name:        "Hardcoded catalogue check",
			Category:    "product",
			Status:      "warning",
			Severity:    "medium",
			FindingIDs:  findingIDs,
			EvidenceIDs: evidenceIDs,
			Message:     fmt.Sprintf("Found %d hardcoded catalogue(s).", len(findings)),
		})
	}

	return checks, findings, evidence
}

func computeVerdict(checks []CheckResult, findings []Finding) (status, severity string, blocking bool, summary string) {
	hasBlocking := false
	hasWarning := false
	for _, f := range findings {
		if f.Blocking {
			hasBlocking = true
		}
	}
	for _, c := range checks {
		if c.Status == "failed" || c.Status == "blocked" {
			hasBlocking = true
		}
		if c.Status == "warning" {
			hasWarning = true
		}
	}

	switch {
	case hasBlocking:
		return "fail", "high", true, "Report gate failed due to blocking findings."
	case hasWarning || len(findings) > 0:
		return "warn", "medium", false, "Report gate passed with warnings."
	default:
		return "pass", "info", false, "All checks passed."
	}
}

// WriteJSON writes the report as indented JSON.
func WriteJSON(w io.Writer, report NomosReport) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

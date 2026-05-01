package output

import (
	"fmt"
	"io"
	"strings"
)

func WriteMarkdown(w io.Writer, report Report) error {
	report = Normalize(report)

	title := report.Project.Name
	if strings.TrimSpace(title) == "" {
		title = report.Project.ID
	}
	if _, err := fmt.Fprintf(w, "# Nomos Report: %s\n\n", markdownText(title)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Schema: `%s`\n", markdownText(report.SchemaVersion)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Report type: `%s`\n", markdownText(report.ReportType)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Generated at: `%s`\n", markdownText(report.GeneratedAt)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Run: `%s` (`%s`)\n", markdownText(report.Run.ID), markdownText(report.Run.Mode)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w, "## Verdict"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Field | Value |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- |"); err != nil {
		return err
	}
	verdictRows := []markdownRow{
		{Label: "Status", Value: report.Verdict.Status},
		{Label: "Severity", Value: report.Verdict.Severity},
		{Label: "Blocking", Value: fmt.Sprintf("%t", report.Verdict.Blocking)},
		{Label: "Summary", Value: report.Verdict.Summary},
	}
	for _, action := range report.Verdict.NextActions {
		verdictRows = append(verdictRows, markdownRow{Label: "Next action", Value: action})
	}
	for _, row := range verdictRows {
		if _, err := fmt.Fprintf(w, "| %s | %s |\n", tableCell(row.Label), tableCell(row.Value)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if err := writeSummary(w, report.Summary); err != nil {
		return err
	}
	if err := writeChecks(w, report.Checks); err != nil {
		return err
	}
	if err := writeFindings(w, report.Findings); err != nil {
		return err
	}
	if err := writeEvidence(w, report.Evidence); err != nil {
		return err
	}

	return nil
}

func writeSummary(w io.Writer, summary Summary) error {
	if _, err := fmt.Fprintln(w, "## Summary"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Metric | Value |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | ---: |"); err != nil {
		return err
	}
	rows := []markdownRow{
		{Label: "Checks", Value: fmt.Sprintf("%d", summary.CheckCount)},
		{Label: "Findings", Value: fmt.Sprintf("%d", summary.FindingCount)},
		{Label: "Blocking findings", Value: fmt.Sprintf("%d", summary.BlockingFindingCount)},
		{Label: "Evidence", Value: fmt.Sprintf("%d", summary.EvidenceCount)},
		{Label: "Coverage ratio", Value: fmt.Sprintf("%.2f", summary.Coverage.CoverageRatio)},
		{Label: "Units covered", Value: fmt.Sprintf("%d/%d", summary.Coverage.UnitCovered, summary.Coverage.UnitTotal)},
		{Label: "Units partial", Value: fmt.Sprintf("%d", summary.Coverage.UnitPartial)},
		{Label: "Units missing", Value: fmt.Sprintf("%d", summary.Coverage.UnitMissing)},
		{Label: "Units not applicable", Value: fmt.Sprintf("%d", summary.Coverage.UnitNotApplicable)},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "| %s | %s |\n", tableCell(row.Label), tableCell(row.Value)); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func writeChecks(w io.Writer, checks []Check) error {
	if _, err := fmt.Fprintln(w, "## Checks"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if len(checks) == 0 {
		if _, err := fmt.Fprintln(w, "No checks recorded."); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w)
		return err
	}
	if _, err := fmt.Fprintln(w, "| ID | Status | Severity | Category | Name |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | --- | --- | --- |"); err != nil {
		return err
	}
	for _, check := range checks {
		if _, err := fmt.Fprintf(
			w,
			"| `%s` | `%s` | `%s` | `%s` | %s |\n",
			tableCell(check.ID),
			tableCell(check.Status),
			tableCell(check.Severity),
			tableCell(check.Category),
			tableCell(check.Name),
		); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func writeFindings(w io.Writer, findings []Finding) error {
	if _, err := fmt.Fprintln(w, "## Findings"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if len(findings) == 0 {
		if _, err := fmt.Fprintln(w, "No findings recorded."); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w)
		return err
	}
	for _, finding := range findings {
		if _, err := fmt.Fprintf(w, "### %s\n\n", markdownText(finding.ID)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Code: `%s`\n", markdownText(finding.Code)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Severity: `%s`\n", markdownText(finding.Severity)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Status: `%s`\n", markdownText(finding.Status)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Blocking: `%t`\n", finding.Blocking); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Target: `%s%s`\n", markdownText(finding.Target.Type), targetSuffix(finding.Target)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Message: %s\n", markdownText(finding.Message)); err != nil {
			return err
		}
		if strings.TrimSpace(finding.Remediation) != "" {
			if _, err := fmt.Fprintf(w, "- Remediation: %s\n", markdownText(finding.Remediation)); err != nil {
				return err
			}
		}
		if len(finding.EvidenceIDs) > 0 {
			if _, err := fmt.Fprintf(w, "- Evidence: `%s`\n", strings.Join(finding.EvidenceIDs, "`, `")); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}

func writeEvidence(w io.Writer, evidence []Evidence) error {
	if _, err := fmt.Fprintln(w, "## Evidence"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if len(evidence) == 0 {
		if _, err := fmt.Fprintln(w, "No evidence recorded."); err != nil {
			return err
		}
		_, err := fmt.Fprintln(w)
		return err
	}
	if _, err := fmt.Fprintln(w, "| ID | Type | Description | Target |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | --- | --- |"); err != nil {
		return err
	}
	for _, item := range evidence {
		target := ""
		if item.Target != nil {
			target = item.Target.Type + targetSuffix(*item.Target)
		}
		if _, err := fmt.Fprintf(
			w,
			"| `%s` | `%s` | %s | `%s` |\n",
			tableCell(item.ID),
			tableCell(item.Type),
			tableCell(item.Description),
			tableCell(target),
		); err != nil {
			return err
		}
	}
	return nil
}

func markdownText(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "\r\n", "\n")
}

func tableCell(value string) string {
	value = markdownText(value)
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", "<br>")
	return value
}

func targetSuffix(target Target) string {
	switch {
	case target.Path != "":
		return ":" + target.Path
	case target.ID != "":
		return ":" + target.ID
	case target.Symbol != "":
		return ":" + target.Symbol
	case target.Locator != "":
		return ":" + target.Locator
	default:
		return ""
	}
}

type markdownRow struct {
	Label string
	Value string
}

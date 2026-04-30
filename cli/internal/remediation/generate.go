package remediation

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/RBOKproject/Nomos/cli/internal/report"
)

const SchemaVersion = "0.1.0"

// Generate builds a remediation Backlog from a NomosReport.
func Generate(nr report.NomosReport) Backlog {
	now := time.Now().UTC()
	var items []RemediationItem

	for i, f := range nr.Findings {
		item := RemediationItem{
			ID:          fmt.Sprintf("REM-%03d", i+1),
			FindingID:   f.ID,
			Code:        f.Code,
			Severity:    f.Severity,
			Blocking:    f.Blocking,
			Priority:    severityPriority(f.Severity, f.Blocking),
			Title:       titleForCode(f.Code),
			Description: f.Message,
			Remediation: f.Remediation,
			Target: Target{
				Type:    f.Target.Type,
				ID:      f.Target.ID,
				Path:    f.Target.Path,
				Locator: f.Target.Locator,
				Symbol:  f.Target.Symbol,
			},
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Priority != items[j].Priority {
			return items[i].Priority < items[j].Priority
		}
		return items[i].ID < items[j].ID
	})

	blockingCount := 0
	for _, item := range items {
		if item.Blocking {
			blockingCount++
		}
	}

	return Backlog{
		SchemaVersion: SchemaVersion,
		ProjectID:     nr.Project.ID,
		GeneratedAt:   now.Format(time.RFC3339),
		TotalItems:    len(items),
		BlockingItems: blockingCount,
		Items:         items,
	}
}

// severityPriority returns a sort key (lower = more critical).
func severityPriority(severity string, blocking bool) int {
	base := 0
	switch severity {
	case "critical":
		base = 1
	case "high":
		base = 2
	case "medium":
		base = 3
	case "low":
		base = 4
	case "info":
		base = 5
	default:
		base = 6
	}
	if blocking {
		base -= 1
		if base < 0 {
			base = 0
		}
	}
	return base
}

// titleForCode returns a human-readable title for a NOMOS_ error code.
func titleForCode(code string) string {
	titles := map[string]string{
		"NOMOS_SCHEMA_INVALID":         "Invalid schema",
		"NOMOS_SOURCE_MISSING":         "Missing source",
		"NOMOS_SOURCE_HASH_MISMATCH":   "Source hash mismatch",
		"NOMOS_SOURCE_OUT_OF_SCOPE":    "Source out of scope",
		"NOMOS_MATRIX_SOURCE_UNKNOWN":  "Unknown matrix source",
		"NOMOS_MATRIX_UNIT_INVALID":    "Invalid matrix unit",
		"NOMOS_CONTRACT_SCHEMA_INVALID": "Invalid contract schema",
		"NOMOS_CONTRACT_SOURCE_MISSING": "Missing contract source",
		"NOMOS_READ_MODEL_STALE":       "Stale read model",
		"NOMOS_KB_CHUNK_STALE":         "Stale knowledge base chunk",
		"NOMOS_PRODUCT_SAMPLE_LEAK":    "Sample data leak in product",
		"NOMOS_PRODUCT_HARDCODED_CATALOG": "Hardcoded catalogue in product code",
		"NOMOS_POLICY_DENIED":          "Policy denied",
		"NOMOS_EVIDENCE_MISSING":       "Missing evidence",
		"NOMOS_ATTESTATION_INVALID":    "Invalid attestation",
		"NOMOS_EXECUTION_ERROR":        "Execution error",
	}
	if title, ok := titles[code]; ok {
		return title
	}
	return code
}

// WriteJSON writes the backlog as indented JSON.
func WriteJSON(w io.Writer, b Backlog) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(b)
}

// WriteMarkdown writes the backlog as a Markdown document.
func WriteMarkdown(w io.Writer, b Backlog) error {
	var sb strings.Builder

	sb.WriteString("# Remediation Backlog\n\n")
	sb.WriteString(fmt.Sprintf("**Project:** %s  \n", b.ProjectID))
	sb.WriteString(fmt.Sprintf("**Generated:** %s  \n", b.GeneratedAt))
	sb.WriteString(fmt.Sprintf("**Total items:** %d  \n", b.TotalItems))
	sb.WriteString(fmt.Sprintf("**Blocking items:** %d\n\n", b.BlockingItems))

	if len(b.Items) == 0 {
		sb.WriteString("No remediation items. All checks passed.\n")
		_, err := io.WriteString(w, sb.String())
		return err
	}

	sb.WriteString("## Items\n\n")
	sb.WriteString("| # | Severity | Blocking | Code | Target | Description |\n")
	sb.WriteString("|---|----------|----------|------|--------|-------------|\n")

	for _, item := range b.Items {
		blocking := ""
		if item.Blocking {
			blocking = "YES"
		}
		target := item.Target.Path
		if target == "" {
			target = item.Target.ID
		}
		if target == "" {
			target = item.Target.Type
		}
		desc := item.Description
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | `%s` | `%s` | %s |\n",
			item.ID, item.Severity, blocking, item.Code, target, desc))
	}

	sb.WriteString("\n## Details\n\n")
	for _, item := range b.Items {
		sb.WriteString(fmt.Sprintf("### %s: %s\n\n", item.ID, item.Title))
		sb.WriteString(fmt.Sprintf("- **Finding:** %s\n", item.FindingID))
		sb.WriteString(fmt.Sprintf("- **Severity:** %s\n", item.Severity))
		if item.Blocking {
			sb.WriteString("- **Blocking:** yes\n")
		}
		sb.WriteString(fmt.Sprintf("- **Code:** `%s`\n", item.Code))
		if item.Target.Path != "" {
			sb.WriteString(fmt.Sprintf("- **Target:** `%s`\n", item.Target.Path))
		}
		sb.WriteString(fmt.Sprintf("\n%s\n\n", item.Description))
		if item.Remediation != "" {
			sb.WriteString(fmt.Sprintf("**Remediation:** %s\n\n", item.Remediation))
		}
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

package admit

import (
	"encoding/json"
	"io"

	"github.com/RBOKproject/Nomos/cli/internal/detect"
)

const ResultFormat = "nomos.admit.v1"

type Verdict string

const (
	VerdictAdmitted   Verdict = "admitted"
	VerdictRefused    Verdict = "refused"
	VerdictPartial    Verdict = "partial"
	VerdictOutOfScope Verdict = "out_of_scope"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type Remediation struct {
	Gap         string `json:"gap"`
	Description string `json:"description"`
}

type AdmissionResult struct {
	Format       string        `json:"format"`
	Verdict      Verdict       `json:"verdict"`
	Confidence   Confidence    `json:"confidence"`
	Reason       string        `json:"reason"`
	Remediations []Remediation `json:"remediations,omitempty"`
}

func Admit(report detect.Report) AdmissionResult {
	if report.FilesScanned == 0 {
		return AdmissionResult{
			Format:     ResultFormat,
			Verdict:    VerdictOutOfScope,
			Confidence: ConfidenceHigh,
			Reason:     "no files scanned; directory is empty or inaccessible",
		}
	}

	if len(report.Languages) == 0 {
		return AdmissionResult{
			Format:     ResultFormat,
			Verdict:    VerdictOutOfScope,
			Confidence: ConfidenceHigh,
			Reason:     "no programming languages detected; not a software repository",
		}
	}

	if len(report.Surfaces) == 0 {
		return AdmissionResult{
			Format:     ResultFormat,
			Verdict:    VerdictOutOfScope,
			Confidence: ConfidenceMedium,
			Reason:     "no product surfaces detected; repository has code but no identifiable product structure",
		}
	}

	var gaps []Remediation

	if len(report.CI) == 0 {
		gaps = append(gaps, Remediation{
			Gap:         "missing_ci",
			Description: "no CI/CD pipeline detected; add a CI configuration to enable automated gates",
		})
	}

	if len(report.Tools) == 0 {
		gaps = append(gaps, Remediation{
			Gap:         "missing_tools",
			Description: "no build or package tools detected; add a manifest (go.mod, package.json, etc.)",
		})
	}

	hasHighConfidenceSurface := false
	for _, s := range report.Surfaces {
		if s.Confidence == "high" {
			hasHighConfidenceSurface = true
			break
		}
	}
	if !hasHighConfidenceSurface {
		gaps = append(gaps, Remediation{
			Gap:         "low_surface_confidence",
			Description: "all detected surfaces have medium or low confidence; add clearer project structure",
		})
	}

	if len(gaps) > 0 {
		confidence := ConfidenceMedium
		if len(gaps) >= 3 {
			confidence = ConfidenceLow
		}
		return AdmissionResult{
			Format:       ResultFormat,
			Verdict:      VerdictPartial,
			Confidence:   confidence,
			Reason:       "repository is partially admissible; gaps must be addressed",
			Remediations: gaps,
		}
	}

	confidence := ConfidenceHigh
	if len(report.Surfaces) == 1 {
		confidence = ConfidenceMedium
	}

	return AdmissionResult{
		Format:     ResultFormat,
		Verdict:    VerdictAdmitted,
		Confidence: confidence,
		Reason:     "repository meets admission criteria",
	}
}

func WriteJSON(w io.Writer, result AdmissionResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

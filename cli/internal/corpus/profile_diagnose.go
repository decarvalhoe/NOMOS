package corpus

import (
	"fmt"
	"strings"
)

// DiagnoseVerdict is the outcome of a profile-aware diagnose.
type DiagnoseVerdict struct {
	Profile    string   `json:"profile"`
	Verdict    string   `json:"verdict"`
	Confidence string   `json:"confidence"`
	Blockers   []string `json:"blockers,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	Summary    string   `json:"summary"`
}

// DiagnoseProfile runs a profile-aware diagnosis on a corpus root.
// For rbok-lawbook: verdict is based on governance classification.
func DiagnoseProfile(profileName string, corpusRoot string) (DiagnoseVerdict, error) {
	profile, err := LookupProfile(profileName)
	if err != nil {
		return DiagnoseVerdict{}, err
	}

	classifications, scanErrors := classifyCorpusSources(corpusRoot)

	result := DiagnoseVerdict{
		Profile: profile.Name,
	}

	// Collect governance signals.
	var primaryCount, referenceCount, derivedCount, outOfScopeCount, archiveCount, blockedCount int
	var runtimeCount, supportingCount, evidenceCount, operationalCount int
	for _, c := range classifications {
		switch c.Priority {
		case "primary":
			primaryCount++
		case "reference":
			referenceCount++
		case "derived":
			derivedCount++
		case "archive":
			archiveCount++
		case "out_of_scope":
			outOfScopeCount++
		}
		switch c.SourceClass {
		case "runtime_binding":
			runtimeCount++
		case "supporting_context":
			supportingCount++
		case "experience_evidence":
			evidenceCount++
		case "operational_context":
			operationalCount++
		}
	}

	// Count blocked binaries from scan errors.
	for _, e := range scanErrors {
		if strings.HasPrefix(e, "blocked binary:") {
			blockedCount++
			result.Blockers = append(result.Blockers, e)
		} else {
			result.Warnings = append(result.Warnings, e)
		}
	}

	// Empty corpus.
	if len(classifications) == 0 {
		result.Verdict = "corpus_blocked"
		result.Confidence = "low"
		result.Summary = "No files found in corpus."
		if len(scanErrors) > 0 {
			result.Blockers = append(result.Blockers, scanErrors...)
		}
		return result, nil
	}

	// Determine verdict from governance report.
	switch {
	case blockedCount > 0:
		result.Verdict = "corpus_blocked"
		result.Confidence = "low"
		result.Summary = fmt.Sprintf(
			"Corpus has %d blocked binary file(s). Remove or declare them before admission.",
			blockedCount,
		)
	case primaryCount == 0:
		result.Verdict = "corpus_blocked"
		result.Confidence = "low"
		result.Blockers = append(result.Blockers, "no primary sources found (expected 01_rbok/00_meta, 01_rbok/01_referentiel, 01_rbok/02_domaines, 01_rbok/03_parcours, or legacy equivalent)")
		result.Summary = "No primary lawbook sources detected."
	case derivedCount > 0 && referenceCount == 0:
		result.Verdict = "corpus_partial"
		result.Confidence = "medium"
		result.Warnings = append(result.Warnings, "corpus has derived files but no reference originals")
		result.Summary = fmt.Sprintf(
			"Corpus has %d primary, %d runtime, and %d derived sources but no reference originals.",
			primaryCount, runtimeCount, derivedCount,
		)
	case primaryCount > 0 && referenceCount > 0:
		result.Verdict = "corpus_admissible"
		result.Confidence = "high"
		result.Summary = fmt.Sprintf(
			"Corpus has %d primary, %d runtime, %d reference, %d supporting, %d evidence, %d operational, %d derived sources. Ready for canonical processing.",
			primaryCount, runtimeCount, referenceCount, supportingCount, evidenceCount, operationalCount, derivedCount,
		)
	default:
		result.Verdict = "corpus_partial"
		result.Confidence = "medium"
		result.Summary = fmt.Sprintf(
			"Corpus has %d primary and %d runtime sources. Consider adding reference originals.",
			primaryCount, runtimeCount,
		)
	}

	if outOfScopeCount > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d out-of-scope file(s) detected (scripts, OS artifacts)", outOfScopeCount))
	}
	if archiveCount > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d archive file(s) excluded by default", archiveCount))
	}

	return result, nil
}

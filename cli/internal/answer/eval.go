package answer

import "sort"

// VRC-13 (#559, B2) — the regulated RAG evaluation harness. Eval runs the
// cite-or-abstain gate over a golden corpus and checks the AGGREGATE metrics
// against versioned thresholds, so a citation-recall (or faithfulness)
// regression below the recorded floor turns the CI job red. The per-answer
// gate (any blocking finding fails) still applies on top.
//
// #620 adds the retrieval-side metrics (context_recall, context_precision,
// noise_sensitivity) over the answers that declare expected_chunk_ids.

// EvalThresholds are the versioned floors a non-regressive corpus must hold.
type EvalThresholds struct {
	MinMeanCitationRecall    float64 `json:"min_mean_citation_recall"`
	MinMeanCitationPrecision float64 `json:"min_mean_citation_precision"`
	MinMeanFaithfulness      float64 `json:"min_mean_faithfulness"`
	MaxFailedAnswers         int     `json:"max_failed_answers"`
	// Context thresholds are optional: nil means reported, not gated. When
	// one is set, at least one answer must declare expected_chunk_ids, or the
	// harness fails closed — a floor nobody measures against is false comfort.
	MinMeanContextRecall    *float64 `json:"min_mean_context_recall,omitempty"`
	MinMeanContextPrecision *float64 `json:"min_mean_context_precision,omitempty"`
	MaxMeanNoiseSensitivity *float64 `json:"max_mean_noise_sensitivity,omitempty"`
}

// EvalResult is the harness verdict over a corpus.
type EvalResult struct {
	Status                string         `json:"status"` // "pass" | "fail"
	Checked               int            `json:"checked"`
	FailedAnswers         int            `json:"failed_answers"`
	MeanCitationRecall    float64        `json:"mean_citation_recall"`
	MeanCitationPrecision float64        `json:"mean_citation_precision"`
	MeanFaithfulness      float64        `json:"mean_faithfulness"`
	ContextEvaluated      int            `json:"context_evaluated"`
	MeanContextRecall     float64        `json:"mean_context_recall"`
	MeanContextPrecision  float64        `json:"mean_context_precision"`
	MeanNoiseSensitivity  float64        `json:"mean_noise_sensitivity"`
	Thresholds            EvalThresholds `json:"thresholds"`
	Regressions           []string       `json:"regressions"`
	Verdicts              []Verdict      `json:"verdicts"`
}

// Eval evaluates a corpus and fails closed on any per-answer finding or any
// aggregate metric outside its versioned bound.
func Eval(answers []Answer, cfg Config, th EvalThresholds) EvalResult {
	res := EvalResult{Status: "pass", Thresholds: th}
	if len(answers) == 0 {
		res.Status = "fail"
		res.Regressions = []string{"empty eval corpus: nothing to evaluate"}
		return res
	}
	var sumR, sumP, sumF float64
	var sumCR, sumCP, sumNS float64
	failed := 0
	for _, a := range answers {
		v := Evaluate(a, cfg)
		res.Verdicts = append(res.Verdicts, v)
		sumR += v.CitationRecall
		sumP += v.CitationPrecision
		sumF += v.Faithfulness
		if len(v.Findings) > 0 {
			failed++
		}
		if v.Context != nil {
			res.ContextEvaluated++
			sumCR += v.Context.ContextRecall
			sumCP += v.Context.ContextPrecision
			sumNS += v.Context.NoiseSensitivity
		}
	}
	n := float64(len(answers))
	res.Checked = len(answers)
	res.FailedAnswers = failed
	res.MeanCitationRecall = round4(sumR / n)
	res.MeanCitationPrecision = round4(sumP / n)
	res.MeanFaithfulness = round4(sumF / n)
	if res.ContextEvaluated > 0 {
		c := float64(res.ContextEvaluated)
		res.MeanContextRecall = round4(sumCR / c)
		res.MeanContextPrecision = round4(sumCP / c)
		res.MeanNoiseSensitivity = round4(sumNS / c)
	}

	if failed > th.MaxFailedAnswers {
		res.Regressions = append(res.Regressions,
			"failed_answers above the versioned maximum")
	}
	if res.MeanCitationRecall < th.MinMeanCitationRecall {
		res.Regressions = append(res.Regressions, "mean_citation_recall below the versioned floor")
	}
	if res.MeanCitationPrecision < th.MinMeanCitationPrecision {
		res.Regressions = append(res.Regressions, "mean_citation_precision below the versioned floor")
	}
	if res.MeanFaithfulness < th.MinMeanFaithfulness {
		res.Regressions = append(res.Regressions, "mean_faithfulness below the versioned floor")
	}

	contextGated := th.MinMeanContextRecall != nil || th.MinMeanContextPrecision != nil || th.MaxMeanNoiseSensitivity != nil
	switch {
	case contextGated && res.ContextEvaluated == 0:
		res.Regressions = append(res.Regressions,
			"context thresholds are set but no answer declares expected_chunk_ids: nothing was measured, refusing to pass")
	case res.ContextEvaluated > 0:
		if th.MinMeanContextRecall != nil && res.MeanContextRecall < *th.MinMeanContextRecall {
			res.Regressions = append(res.Regressions, "mean_context_recall below the versioned floor")
		}
		if th.MinMeanContextPrecision != nil && res.MeanContextPrecision < *th.MinMeanContextPrecision {
			res.Regressions = append(res.Regressions, "mean_context_precision below the versioned floor")
		}
		if th.MaxMeanNoiseSensitivity != nil && res.MeanNoiseSensitivity > *th.MaxMeanNoiseSensitivity {
			res.Regressions = append(res.Regressions, "mean_noise_sensitivity above the versioned ceiling")
		}
	}
	sort.Strings(res.Regressions)
	if len(res.Regressions) > 0 {
		res.Status = "fail"
	}
	sort.SliceStable(res.Verdicts, func(i, j int) bool { return res.Verdicts[i].AnswerID < res.Verdicts[j].AnswerID })
	return res
}
